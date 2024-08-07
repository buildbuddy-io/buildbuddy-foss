package target_tracker

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/buildbuddy-io/buildbuddy/proto/build_event_stream"
	"github.com/buildbuddy-io/buildbuddy/server/build_event_protocol/accumulator"
	"github.com/buildbuddy-io/buildbuddy/server/environment"
	"github.com/buildbuddy-io/buildbuddy/server/tables"
	"github.com/buildbuddy-io/buildbuddy/server/util/background"
	"github.com/buildbuddy-io/buildbuddy/server/util/clickhouse/schema"
	"github.com/buildbuddy-io/buildbuddy/server/util/db"
	"github.com/buildbuddy-io/buildbuddy/server/util/log"
	"github.com/buildbuddy-io/buildbuddy/server/util/perms"
	"github.com/buildbuddy-io/buildbuddy/server/util/query_builder"
	"github.com/buildbuddy-io/buildbuddy/server/util/status"
	"github.com/buildbuddy-io/buildbuddy/server/util/timeutil"
	"github.com/buildbuddy-io/buildbuddy/server/util/uuid"
	"golang.org/x/sync/errgroup"

	cmpb "github.com/buildbuddy-io/buildbuddy/proto/api/v1/common"
)

var (
	enableTargetTracking                   = flag.Bool("app.enable_target_tracking", false, "Cloud-Only")
	writeTestTargetStatusesToOLAPDBEnabled = flag.Bool("app.enable_write_test_target_statuses_to_olap_db", false, "If enabled, test target statuses will be flushed to OLAP DB")
)

const (
	writeTestTargetStatusesTimeout = 15 * time.Second
)

type targetClosure func(event *build_event_stream.BuildEvent)
type targetState int

const (
	targetStateExpanded targetState = iota
	targetStateConfigured
	targetStateCompleted
	targetStateResult
	targetStateSummary
	targetStateAborted
)

type target struct {
	label          string
	aspect         string
	ruleType       string
	firstStartTime time.Time
	totalDuration  time.Duration
	state          targetState
	id             int64
	overallStatus  build_event_stream.TestStatus
	cached         bool
	targetType     cmpb.TargetType
	testSize       build_event_stream.TestSize
	buildSuccess   bool
}

func md5Int64(text string) int64 {
	hash := md5.Sum([]byte(text))
	return int64(binary.BigEndian.Uint64(hash[:8]))
}

func newTarget(label string, aspect string) *target {
	return &target{
		label:  label,
		aspect: aspect,
		state:  targetStateExpanded,
	}
}

func getTestStatus(aborted *build_event_stream.Aborted) build_event_stream.TestStatus {
	switch aborted.GetReason() {
	case build_event_stream.Aborted_USER_INTERRUPTED:
		return build_event_stream.TestStatus_TOOL_HALTED_BEFORE_TESTING
	default:
		return build_event_stream.TestStatus_FAILED_TO_BUILD
	}
}

func (t *target) updateFromEvent(event *build_event_stream.BuildEvent) {
	switch p := event.Payload.(type) {
	case *build_event_stream.BuildEvent_Configured:
		t.ruleType = p.Configured.GetTargetKind()
		t.targetType = targetTypeFromRuleType(t.ruleType)
		t.testSize = p.Configured.GetTestSize()
		t.state = targetStateConfigured
	case *build_event_stream.BuildEvent_Completed:
		t.buildSuccess = p.Completed.GetSuccess()
		if !p.Completed.GetSuccess() {
			t.overallStatus = build_event_stream.TestStatus_FAILED_TO_BUILD
		}
		t.state = targetStateCompleted
	case *build_event_stream.BuildEvent_TestResult:
		t.cached = p.TestResult.GetCachedLocally() || p.TestResult.GetExecutionInfo().GetCachedRemotely()
		t.state = targetStateResult
	case *build_event_stream.BuildEvent_TestSummary:
		ts := p.TestSummary
		t.overallStatus = ts.GetOverallStatus()
		t.firstStartTime = timeutil.GetTimeWithFallback(ts.GetFirstStartTime(), ts.GetFirstStartTimeMillis())
		t.totalDuration = timeutil.GetDurationWithFallback(ts.GetTotalRunDuration(), ts.GetTotalRunDurationMillis())
		t.state = targetStateSummary
	case *build_event_stream.BuildEvent_Aborted:
		t.buildSuccess = false
		t.state = targetStateAborted
		t.overallStatus = getTestStatus(p.Aborted)
	}
}

const targetIdSeparator string = "|"

func getTargetIdWithAspectFromEventId(beid *build_event_stream.BuildEventId) string {
	switch beid.Id.(type) {
	case *build_event_stream.BuildEventId_TargetConfigured:
		if aspect := beid.GetTargetConfigured().GetAspect(); aspect != "" {
			return beid.GetTargetConfigured().GetLabel() + targetIdSeparator + aspect
		} else {
			return beid.GetTargetConfigured().GetLabel()
		}
	case *build_event_stream.BuildEventId_TargetCompleted:
		if aspect := beid.GetTargetCompleted().GetAspect(); aspect != "" {
			return beid.GetTargetCompleted().GetLabel() + targetIdSeparator + aspect
		} else {
			return beid.GetTargetCompleted().GetLabel()
		}
	case *build_event_stream.BuildEventId_TestResult:
		// Aspects can't fire TestResult events.
		return beid.GetTestResult().GetLabel()
	case *build_event_stream.BuildEventId_TestSummary:
		// Aspects can't fire TestSummary events.
		return beid.GetTestSummary().GetLabel()
	}
	return ""
}

func targetTypeFromRuleType(ruleType string) cmpb.TargetType {
	ruleType = strings.TrimSuffix(ruleType, " rule")
	switch {
	case strings.HasSuffix(ruleType, "application"):
		return cmpb.TargetType_APPLICATION
	case strings.HasSuffix(ruleType, "binary"):
		return cmpb.TargetType_BINARY
	case strings.HasSuffix(ruleType, "library"):
		return cmpb.TargetType_LIBRARY
	case strings.HasSuffix(ruleType, "package"):
		return cmpb.TargetType_PACKAGE
	case strings.HasSuffix(ruleType, "test"):
		return cmpb.TargetType_TEST
	default:
		return cmpb.TargetType_TARGET_TYPE_UNSPECIFIED
	}
}

type TargetTracker struct {
	env                   environment.Env
	buildEventAccumulator accumulator.Accumulator
	targets               map[string]*target
	errGroup              *errgroup.Group
}

func NewTargetTracker(env environment.Env, buildEventAccumulator accumulator.Accumulator) *TargetTracker {
	return &TargetTracker{
		env:                   env,
		buildEventAccumulator: buildEventAccumulator,
		targets:               make(map[string]*target, 0),
	}
}

func (t *TargetTracker) handleEvent(event *build_event_stream.BuildEvent) {
	id := getTargetIdWithAspectFromEventId(event.GetId())
	if id == "" {
		return
	}
	target, ok := t.targets[id]
	if !ok {
		// TODO(jdhollen): bazel doesn't include a separate targetConfigured
		// event id for each aspect in the expanded event, so it's possible
		// to receive targetConfigured and targetCompleted events for
		// previously-unannounced target+aspect pairs.  If we ever want to
		// track aspects, we'll need to add them to t.targets here.
		return
	}
	target.updateFromEvent(event)
}

func isTest(t *target) bool {
	if t.aspect != "" {
		return false
	}
	if t.testSize != build_event_stream.TestSize_UNKNOWN {
		return true
	}
	return strings.HasSuffix(strings.ToLower(t.ruleType), "test")
}

func (t *TargetTracker) testTargetsInAtLeastState(state targetState) bool {
	for _, t := range t.targets {
		if isTest(t) && t.state < state {
			return false
		}
	}
	return true
}

func (t *TargetTracker) permissionsFromContext(ctx context.Context) (*perms.UserGroupPerm, error) {
	if u, err := t.env.GetAuthenticator().AuthenticatedUser(ctx); err == nil && u.GetGroupID() != "" {
		return perms.DefaultPermissions(u), nil
	}
	return nil, status.UnauthenticatedError("Context did not contain auth information")
}

func (t *TargetTracker) invocationID() string {
	return t.buildEventAccumulator.Invocation().GetInvocationId()
}

func (t *TargetTracker) writeTestTargets(ctx context.Context, permissions *perms.UserGroupPerm) error {
	if t.WriteToOLAPDBEnabled() {
		// Stop write test targets to MySQL when writes to OLAP DB is enabled
		return nil
	}
	repoURL := t.buildEventAccumulator.Invocation().GetRepoUrl()
	knownTargets, err := readRepoTargets(ctx, t.env, repoURL)
	if err != nil {
		return err
	}
	knownTargetsByLabel := make(map[string]*tables.Target, len(knownTargets))
	for _, knownTarget := range knownTargets {
		knownTargetsByLabel[knownTarget.Label] = knownTarget
	}
	newTargets := make([]*tables.Target, 0)
	updatedTargets := make([]*tables.Target, 0)
	for label, target := range t.targets {
		if target.aspect != "" {
			continue
		}
		if target.targetType != cmpb.TargetType_TEST {
			continue
		}
		tableTarget := &tables.Target{
			RepoURL:  repoURL,
			TargetID: md5Int64(repoURL + target.label),
			UserID:   permissions.UserID,
			GroupID:  permissions.GroupID,
			Perms:    permissions.Perms,
			Label:    target.label,
			RuleType: target.ruleType,
		}
		knownTarget, ok := knownTargetsByLabel[label]
		if !ok {
			newTargets = append(newTargets, tableTarget)
			continue
		}
		if knownTarget.RuleType != target.ruleType {
			updatedTargets = append(updatedTargets, tableTarget)
			continue
		}
	}
	if len(updatedTargets) > 0 {
		if err := updateTargets(ctx, t.env, updatedTargets); err != nil {
			log.Warningf("Error updating %q targets: %s", t.invocationID(), err.Error())
			return err
		}
	}
	if len(newTargets) > 0 {
		if err := insertTargets(ctx, t.env, newTargets); err != nil {
			log.Warningf("Error inserting %q targets: %s", t.invocationID(), err.Error())
			return err
		}
	}
	return nil
}

func (t *TargetTracker) writeTestTargetStatuses(ctx context.Context, permissions *perms.UserGroupPerm) error {
	if t.WriteToOLAPDBEnabled() {
		// Stop write test targets to MySQL when writes to OLAP DB is enabled
		return nil
	}
	repoURL := t.buildEventAccumulator.Invocation().GetRepoUrl()
	invocationUUID, err := uuid.StringToBytes(t.invocationID())
	if err != nil {
		return err
	}
	newTargetStatuses := make([]*tables.TargetStatus, 0)
	for _, target := range t.targets {
		if !isTest(target) {
			continue
		}
		newTargetStatuses = append(newTargetStatuses, &tables.TargetStatus{
			TargetID:       md5Int64(repoURL + target.label),
			InvocationUUID: invocationUUID,
			TargetType:     int32(target.targetType),
			TestSize:       int32(target.testSize),
			Status:         int32(target.overallStatus),
			Cached:         target.cached,
			StartTimeUsec:  target.firstStartTime.UnixMicro(),
			DurationUsec:   target.totalDuration.Microseconds(),
		})
	}
	if err := insertOrUpdateTargetStatuses(ctx, t.env, newTargetStatuses); err != nil {
		log.Warningf("Error inserting %q target statuses: %s", t.invocationID(), err.Error())
		return err
	}
	return nil
}

func (t *TargetTracker) writeTestTargetStatusesToOLAPDB(ctx context.Context, permissions *perms.UserGroupPerm) error {
	if !t.WriteToOLAPDBEnabled() {
		return nil
	}
	ctx, cancel := background.ExtendContextForFinalization(ctx, writeTestTargetStatusesTimeout)
	defer cancel()

	entries := make([]*schema.TestTargetStatus, 0)
	invocation := t.buildEventAccumulator.Invocation()
	if invocation == nil {
		return status.InternalError("failed to write test target statuses: no invocation from build accumulator")
	}
	if permissions.GroupID == "" {
		log.CtxInfo(ctx, "skip writing test target statuses to OLAPDB because group_id is empty")
		return nil
	}
	repoURL := t.buildEventAccumulator.Invocation().GetRepoUrl()
	if repoURL == "" {
		log.CtxInfo(ctx, "skip writing test target status because repo_url is empty")
		return nil
	}
	commitSHA := t.buildEventAccumulator.Invocation().GetCommitSha()
	if commitSHA == "" {
		log.CtxInfo(ctx, "skip writing test target status because commit_sha is empty")
		return nil
	}
	invocationStartTime := t.buildEventAccumulator.StartTime()
	if invocationStartTime.IsZero() {
		log.CtxInfo(ctx, "skip writing test target status because invocationStartTime is unavailable")
	}

	invocationUUID := strings.Replace(t.invocationID(), "-", "", -1)

	for _, target := range t.targets {
		if !isTest(target) {
			continue
		}

		if target.overallStatus == build_event_stream.TestStatus_NO_STATUS {
			continue
		}
		testStartTimeUsec := int64(0)
		if !target.firstStartTime.IsZero() {
			testStartTimeUsec = target.firstStartTime.UnixMicro()
		}

		entries = append(entries, &schema.TestTargetStatus{
			GroupID:                 permissions.GroupID,
			RepoURL:                 repoURL,
			CommitSHA:               commitSHA,
			Label:                   target.label,
			InvocationStartTimeUsec: invocationStartTime.UnixMicro(),

			RuleType:       target.ruleType,
			UserID:         permissions.UserID,
			InvocationUUID: invocationUUID,
			TargetType:     int32(target.targetType),
			TestSize:       int32(target.testSize),
			Status:         int32(target.overallStatus),
			Cached:         target.cached,
			StartTimeUsec:  testStartTimeUsec,
			DurationUsec:   target.totalDuration.Microseconds(),
			BranchName:     t.buildEventAccumulator.Invocation().GetBranchName(),
			Role:           t.buildEventAccumulator.Invocation().GetRole(),
			Command:        t.buildEventAccumulator.Invocation().GetCommand(),
		})
	}
	err := t.env.GetOLAPDBHandle().FlushTestTargetStatuses(ctx, entries)
	if err == nil {
		log.CtxInfof(ctx, "successfully wrote %d test target statuses", len(entries))
	}
	return err
}

func (t *TargetTracker) TrackTargetsForEvent(ctx context.Context, event *build_event_stream.BuildEvent) {
	if !*enableTargetTracking {
		return
	}
	// Depending on the event type we will either:
	//  - read the set of targets for this repo
	//  - update the set of targets for this repo
	//  - write statuses for the targets at this invocation
	switch event.Payload.(type) {
	case *build_event_stream.BuildEvent_Expanded:
		t.handleExpandedEvent(event)
	case *build_event_stream.BuildEvent_Configured:
		t.handleEvent(event)
	case *build_event_stream.BuildEvent_WorkspaceStatus:
		t.handleWorkspaceStatusEvent(ctx)
	case *build_event_stream.BuildEvent_Completed:
		t.handleEvent(event)
	case *build_event_stream.BuildEvent_TestResult:
		t.handleEvent(event)
	case *build_event_stream.BuildEvent_TestSummary:
		t.handleEvent(event)
	case *build_event_stream.BuildEvent_Aborted:
		t.handleEvent(event)
	}

	if event.GetLastMessage() {
		t.handleLastEvent(ctx)
	}
}

func (t *TargetTracker) handleExpandedEvent(event *build_event_stream.BuildEvent) {
	// This event announces all the upcoming targets we'll get information about.
	// For each target, we'll create a "target" object, keyed by the target label,
	// and each target will "listen" for followup events on the specified ID.
	for _, child := range event.GetChildren() {
		label := child.GetTargetConfigured().GetLabel()
		aspect := child.GetTargetConfigured().GetAspect()
		if label == "" {
			continue
		}
		id := getTargetIdWithAspectFromEventId(child)
		childTarget := newTarget(label, aspect)
		t.targets[id] = childTarget
	}
}

func (t *TargetTracker) handleWorkspaceStatusEvent(ctx context.Context) {
	ctx = log.EnrichContext(ctx, log.InvocationIDKey, t.invocationID())
	if !t.testTargetsInAtLeastState(targetStateConfigured) {
		// This should not happen, but it seems it can happen with certain targets.
		// For now, we will log the targets that do not meet the required state
		// so we can better understand whats happening to them.
		log.CtxWarningf(ctx, "Not all targets for %q reached state: %d, targets: %+v", t.invocationID(), targetStateConfigured, t.targets)
		return
	}
	if !isTestCommand(t.buildEventAccumulator.Invocation().GetCommand()) {
		log.CtxDebugf(ctx, "Not tracking targets for %q because it's not a test", t.invocationID())
		return
	}
	if t.buildEventAccumulator.Invocation().GetRole() != "CI" {
		log.CtxDebugf(ctx, "Not tracking targets for %q because it's not a CI build", t.invocationID())
		return
	}
	if t.buildEventAccumulator.DisableTargetTracking() {
		log.CtxDebugf(ctx, "Not tracking targets for %q because DISABLE_TARGET_TRACKING is set", t.invocationID())
		return
	}
	permissions, err := t.permissionsFromContext(ctx)
	if err != nil {
		log.CtxDebugf(ctx, "Not tracking targets for %q because it's not authenticated: %s", t.invocationID(), err.Error())
		return
	}
	eg, gctx := errgroup.WithContext(ctx)
	t.errGroup = eg
	t.errGroup.Go(func() error { return t.writeTestTargets(gctx, permissions) })
}

func (t *TargetTracker) handleLastEvent(ctx context.Context) {
	ctx = log.EnrichContext(ctx, log.InvocationIDKey, t.invocationID())
	if !isTestCommand(t.buildEventAccumulator.Invocation().GetCommand()) {
		log.Debugf("Not tracking targets statuses for %q because it's not a test", t.invocationID())
		return
	}
	if t.buildEventAccumulator.Invocation().GetRole() != "CI" {
		log.CtxDebugf(ctx, "Not tracking target statuses for %q because it's not a CI build", t.invocationID())
		return
	}
	if t.buildEventAccumulator.DisableTargetTracking() {
		log.CtxDebugf(ctx, "Not tracking targets for %q because DISABLE_TARGET_TRACKING is set", t.invocationID())
		return
	}
	permissions, err := t.permissionsFromContext(ctx)
	if err != nil {
		log.CtxDebugf(ctx, "Not tracking targets for %q because it's not authenticated: %s", t.invocationID(), err.Error())
		return
	}
	if t.errGroup == nil {
		log.CtxWarningf(ctx, "Not tracking target statuses for %q because targets were not reported", t.invocationID())
		return
	}
	// Synchronization point: make sure that all targets were read (or written).
	if err := t.errGroup.Wait(); err != nil {
		log.CtxWarningf(ctx, "Error getting %q targets: %s", t.invocationID(), err.Error())
		return
	}
	if err := t.writeTestTargetStatuses(ctx, permissions); err != nil {
		log.CtxDebugf(ctx, "Error writing %q target statuses: %s", t.invocationID(), err.Error())
	}
	if err := t.writeTestTargetStatusesToOLAPDB(ctx, permissions); err != nil {
		log.CtxErrorf(ctx, "Error writing %q target statuses: %s", t.invocationID(), err.Error())
	}
}

func isTestCommand(command string) bool {
	return command == "test" || command == "coverage"
}

func readRepoTargets(ctx context.Context, env environment.Env, repoURL string) ([]*tables.Target, error) {
	if env.GetDBHandle() == nil {
		return nil, status.FailedPreconditionError("database not configured")
	}
	var err error
	q := query_builder.NewQuery(`SELECT * FROM "Targets" as t`)
	q = q.AddWhereClause(`t.repo_url = ?`, repoURL)
	if err := perms.AddPermissionsCheckToQueryWithTableAlias(ctx, env, q, "t"); err != nil {
		return nil, err
	}
	queryStr, args := q.Build()
	rq := env.GetDBHandle().NewQuery(ctx, "target_tracker_get_targets_by_url").Raw(queryStr, args...)
	rsp, err := db.ScanAll(rq, &tables.Target{})
	if err != nil {
		return nil, err
	}
	return rsp, nil
}

func chunkTargetsBy(items []*tables.Target, chunkSize int) (chunks [][]*tables.Target) {
	for chunkSize < len(items) {
		items, chunks = items[chunkSize:], append(chunks, items[0:chunkSize:chunkSize])
	}
	return append(chunks, items)
}

func updateTargets(ctx context.Context, env environment.Env, targets []*tables.Target) error {
	if env.GetDBHandle() == nil {
		return status.FailedPreconditionError("database not configured")
	}
	for _, t := range targets {
		result := env.GetDBHandle().GORM(ctx, "target_tracker_update_target").Where(
			"target_id = ?", t.TargetID).Updates(t)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return status.NotFoundErrorf("Failed to update target, no target exists with target_id %d", t.TargetID)
		}
	}
	return nil
}

func insertTargets(ctx context.Context, env environment.Env, targets []*tables.Target) error {
	if env.GetDBHandle() == nil {
		return status.FailedPreconditionError("database not configured")
	}
	chunkList := chunkTargetsBy(targets, 100)
	for _, chunk := range chunkList {
		valueStrings := []string{}
		valueArgs := []interface{}{}
		for _, t := range chunk {
			nowUsec := time.Now().UnixMicro()
			valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?, ?, ?, ?)")
			valueArgs = append(valueArgs, t.RepoURL)
			valueArgs = append(valueArgs, t.TargetID)
			valueArgs = append(valueArgs, t.UserID)
			valueArgs = append(valueArgs, t.GroupID)
			valueArgs = append(valueArgs, t.Perms)
			valueArgs = append(valueArgs, t.Label)
			valueArgs = append(valueArgs, t.RuleType)
			valueArgs = append(valueArgs, nowUsec)
			valueArgs = append(valueArgs, nowUsec)
		}
		stmt := fmt.Sprintf(`INSERT INTO "Targets" (repo_url, target_id, user_id, group_id, perms, label, rule_type, created_at_usec, updated_at_usec) VALUES %s`, strings.Join(valueStrings, ","))
		err := env.GetDBHandle().NewQuery(ctx, "target_tracker_insert_targets").Raw(stmt, valueArgs...).Exec().Error
		if err != nil {
			return err
		}
	}
	return nil
}

func chunkStatusesBy(items []*tables.TargetStatus, chunkSize int) (chunks [][]*tables.TargetStatus) {
	if len(items) == 0 {
		return nil
	}
	for chunkSize < len(items) {
		items, chunks = items[chunkSize:], append(chunks, items[0:chunkSize:chunkSize])
	}
	return append(chunks, items)
}

func insertOrUpdateTargetStatuses(ctx context.Context, env environment.Env, statuses []*tables.TargetStatus) error {
	if env.GetDBHandle() == nil {
		return status.FailedPreconditionError("database not configured")
	}
	chunkList := chunkStatusesBy(statuses, 100)
	for _, chunk := range chunkList {
		valueStrings := []string{}
		valueArgs := []interface{}{}
		for _, t := range chunk {
			nowUsec := time.Now().UnixMicro()
			valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
			valueArgs = append(valueArgs, t.TargetID)
			valueArgs = append(valueArgs, t.InvocationUUID)
			valueArgs = append(valueArgs, t.TargetType)
			valueArgs = append(valueArgs, t.TestSize)
			valueArgs = append(valueArgs, t.Status)
			valueArgs = append(valueArgs, t.Cached)
			valueArgs = append(valueArgs, t.StartTimeUsec)
			valueArgs = append(valueArgs, t.DurationUsec)
			valueArgs = append(valueArgs, nowUsec)
			valueArgs = append(valueArgs, nowUsec)
		}
		stmt := fmt.Sprintf(`INSERT INTO "TargetStatuses" (target_id, invocation_uuid, target_type, test_size, status, cached, start_time_usec, duration_usec, created_at_usec, updated_at_usec) VALUES %s`, strings.Join(valueStrings, ","))
		err := env.GetDBHandle().NewQuery(ctx, "target_tracker_insert_target_statuses").Raw(stmt, valueArgs...).Exec().Error
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *TargetTracker) WriteToOLAPDBEnabled() bool {
	return *writeTestTargetStatusesToOLAPDBEnabled && t.env.GetOLAPDBHandle() != nil
}

func TargetTrackingEnabled() bool {
	return *enableTargetTracking
}
