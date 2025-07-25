package build_event_handler

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/buildbuddy-io/buildbuddy/proto/build_event_stream"
	"github.com/buildbuddy-io/buildbuddy/proto/command_line"
	"github.com/buildbuddy-io/buildbuddy/server/backends/invocationdb"
	"github.com/buildbuddy-io/buildbuddy/server/build_event_protocol/accumulator"
	"github.com/buildbuddy-io/buildbuddy/server/build_event_protocol/build_status_reporter"
	"github.com/buildbuddy-io/buildbuddy/server/build_event_protocol/invocation_format"
	"github.com/buildbuddy-io/buildbuddy/server/build_event_protocol/target_tracker"
	"github.com/buildbuddy-io/buildbuddy/server/endpoint_urls/build_buddy_url"
	"github.com/buildbuddy-io/buildbuddy/server/endpoint_urls/cache_api_url"
	"github.com/buildbuddy-io/buildbuddy/server/environment"
	"github.com/buildbuddy-io/buildbuddy/server/eventlog"
	"github.com/buildbuddy-io/buildbuddy/server/interfaces"
	"github.com/buildbuddy-io/buildbuddy/server/metrics"
	"github.com/buildbuddy-io/buildbuddy/server/olapdbconfig"
	"github.com/buildbuddy-io/buildbuddy/server/remote_cache/digest"
	"github.com/buildbuddy-io/buildbuddy/server/remote_cache/hit_tracker"
	"github.com/buildbuddy-io/buildbuddy/server/remote_cache/scorecard"
	"github.com/buildbuddy-io/buildbuddy/server/tables"
	"github.com/buildbuddy-io/buildbuddy/server/util/alert"
	"github.com/buildbuddy-io/buildbuddy/server/util/authutil"
	"github.com/buildbuddy-io/buildbuddy/server/util/background"
	"github.com/buildbuddy-io/buildbuddy/server/util/db"
	"github.com/buildbuddy-io/buildbuddy/server/util/log"
	"github.com/buildbuddy-io/buildbuddy/server/util/paging"
	"github.com/buildbuddy-io/buildbuddy/server/util/perms"
	"github.com/buildbuddy-io/buildbuddy/server/util/proto"
	"github.com/buildbuddy-io/buildbuddy/server/util/protofile"
	"github.com/buildbuddy-io/buildbuddy/server/util/redact"
	"github.com/buildbuddy-io/buildbuddy/server/util/status"
	"github.com/buildbuddy-io/buildbuddy/server/util/subdomain"
	"github.com/buildbuddy-io/buildbuddy/server/util/terminal"
	"github.com/buildbuddy-io/buildbuddy/server/util/urlutil"
	"github.com/buildbuddy-io/buildbuddy/server/util/usageutil"
	"github.com/buildbuddy-io/buildbuddy/server/util/uuid"
	"github.com/google/shlex"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	apipb "github.com/buildbuddy-io/buildbuddy/proto/api/v1"
	bepb "github.com/buildbuddy-io/buildbuddy/proto/build_events"
	capb "github.com/buildbuddy-io/buildbuddy/proto/cache"
	csinpb "github.com/buildbuddy-io/buildbuddy/proto/index"
	inpb "github.com/buildbuddy-io/buildbuddy/proto/invocation"
	inspb "github.com/buildbuddy-io/buildbuddy/proto/invocation_status"
	pgpb "github.com/buildbuddy-io/buildbuddy/proto/pagination"
	pepb "github.com/buildbuddy-io/buildbuddy/proto/publish_build_event"
	rspb "github.com/buildbuddy-io/buildbuddy/proto/resource"
	sipb "github.com/buildbuddy-io/buildbuddy/proto/stored_invocation"
	api_common "github.com/buildbuddy-io/buildbuddy/server/api/common"
	gitutil "github.com/buildbuddy-io/buildbuddy/server/util/git"
	gstatus "google.golang.org/grpc/status"
)

const (
	defaultChunkFileSizeBytes = 1000 * 100 // 100KB

	// How many workers to spin up for writing cache stats to the DB.
	numStatsRecorderWorkers = 8

	// How many workers to spin up for looking up invocations before
	// webhooks are notified.
	numWebhookInvocationLookupWorkers = 8
	// How many workers to spin up for notifying webhooks.
	numWebhookNotifyWorkers = 16

	// How long to wait before giving up on webhook requests.
	webhookNotifyTimeout = 1 * time.Minute

	// Default number of actions shown by bazel
	defaultActionsShown = 8

	// Exit code in Finished event indicating that the build was interrupted
	// (i.e. killed by user).
	InterruptedExitCode = 8

	// First sequence number that we expect to see in the ordered build
	// event stream.
	firstExpectedSequenceNumber = 1

	// Max total pattern length to include in the Expanded event returned to the
	// UI.
	maxPatternLengthBytes = 10_000

	// Rather than immediately deleting executions data from Redis after flushing
	// finalized data to Clickhouse, expire it after this TTL so that even if Clickhouse
	// has replication lag, clients will still be able to read the data from Redis.
	expireRedisExecutionsTTL = 5 * time.Minute
)

var (
	chunkFileSizeBytes      = flag.Int("storage.chunk_file_size_bytes", 3_000_000 /* 3 MB */, "How many bytes to buffer in memory before flushing a chunk of build protocol data to disk.")
	enableChunkedEventLogs  = flag.Bool("storage.enable_chunked_event_logs", true, "If true, Event logs will be stored separately from the invocation proto in chunks.")
	disablePersistArtifacts = flag.Bool("storage.disable_persist_cache_artifacts", false, "If disabled, buildbuddy will not persist cache artifacts in the blobstore. This may make older invocations not display properly.")
	writeToOLAPDBEnabled    = flag.Bool("app.enable_write_to_olap_db", true, "If enabled, complete invocations will be flushed to OLAP DB")

	buildEventFilterStartThreshold = flag.Int("app.build_event_filter_start_threshold", 100_000, "When looking up an invocation, start filtering out unimportant events after this many events have been processed.")
	cacheStatsFinalizationDelay    = flag.Duration("cache_stats_finalization_delay", 500*time.Millisecond, "The time allowed for all metrics collectors across all apps to flush their local cache stats to the backing storage, before finalizing stats in the DB.")
)

var cacheArtifactsBlobstorePath = path.Join("artifacts", "cache")

type PersistArtifacts struct {
	URIs              []*url.URL
	TestActionOutputs bool
}

type BuildEventHandler struct {
	env              environment.Env
	statsRecorder    *statsRecorder
	webhookNotifier  *webhookNotifier
	openChannels     *sync.WaitGroup
	cancelFnsByInvID sync.Map // map of string invocationID => context.CancelFunc

	mu           sync.Mutex
	shuttingDown bool
}

func NewBuildEventHandler(env environment.Env) *BuildEventHandler {
	openChannels := &sync.WaitGroup{}
	onStatsRecorded := make(chan *invocationInfo, 4096)
	statsRecorder := newStatsRecorder(env, openChannels, onStatsRecorded)
	webhookNotifier := newWebhookNotifier(env, onStatsRecorded)

	statsRecorder.Start()
	webhookNotifier.Start()

	h := &BuildEventHandler{
		env:              env,
		statsRecorder:    statsRecorder,
		webhookNotifier:  webhookNotifier,
		openChannels:     openChannels,
		cancelFnsByInvID: sync.Map{},
	}
	env.GetHealthChecker().RegisterShutdownFunction(func(ctx context.Context) error {
		h.Stop()
		return nil
	})
	return h
}

func (b *BuildEventHandler) OpenChannel(ctx context.Context, iid string) (interfaces.BuildEventChannel, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.shuttingDown {
		return nil, status.UnavailableErrorf("Server shutting down, cannot open channel for %s", iid)
	}

	invocation := &inpb.Invocation{InvocationId: iid}
	buildEventAccumulator := accumulator.NewBEValues(invocation)
	val, ok := b.cancelFnsByInvID.Load(iid)
	if ok {
		cancelFn := val.(context.CancelFunc)
		cancelFn()
	}

	ctx, cancel := context.WithCancel(ctx)
	b.cancelFnsByInvID.Store(iid, cancel)

	b.openChannels.Add(1)
	onClose := func() {
		b.openChannels.Done()
		b.cancelFnsByInvID.Delete(iid)
	}

	return &EventChannel{
		env:            b.env,
		statsRecorder:  b.statsRecorder,
		ctx:            ctx,
		pw:             nil,
		beValues:       buildEventAccumulator,
		redactor:       redact.NewStreamingRedactor(b.env),
		statusReporter: build_status_reporter.NewBuildStatusReporter(b.env, buildEventAccumulator),
		targetTracker:  target_tracker.NewTargetTracker(b.env, buildEventAccumulator),
		collector:      b.env.GetMetricsCollector(),
		apiTargetMap:   api_common.NewTargetMap( /* TargetSelector */ nil),

		hasReceivedEventWithOptions: false,
		hasReceivedStartedEvent:     false,
		bufferedEvents:              make([]*inpb.InvocationEvent, 0),
		logWriter:                   nil,
		onClose:                     onClose,
		attempt:                     1,
	}, nil
}

func (b *BuildEventHandler) Stop() {
	b.mu.Lock()
	b.shuttingDown = true
	b.mu.Unlock()
	b.cancelFnsByInvID.Range(func(key, val interface{}) bool {
		iid := key.(string)
		cancelFn := val.(context.CancelFunc)
		log.Infof("Cancelling invocation %q because server received shutdown signal", iid)
		cancelFn()
		return true
	})
	b.statsRecorder.Stop()
	b.webhookNotifier.Stop()
}

// invocationInfo represents an invocation ID as well as the JWT granting access
// to it. It should only be used for background tasks that need access to the
// JWT after the build event stream is already closed.
type invocationInfo struct {
	id      string
	jwt     string
	attempt uint64
}

// recordStatsTask contains the info needed to record the stats for an
// invocation. These tasks are enqueued to statsRecorder and executed in the
// background.
type recordStatsTask struct {
	*invocationInfo
	// createdAt is the time at which this task was created.
	createdAt time.Time
	// files contains a mapping of file digests to file name metadata for files
	// referenced in the BEP.
	files                    map[string]*build_event_stream.File
	persist                  *PersistArtifacts
	kytheSSTableResourceName *rspb.ResourceName
	invocationStatus         inspb.InvocationStatus
}

// statsRecorder listens for finalized invocations and copies cache stats from
// the metrics collector to the DB.
type statsRecorder struct {
	env          environment.Env
	openChannels *sync.WaitGroup
	// onStatsRecorded is a channel for this statsRecorder to notify after
	// recording stats for each invocation. Invocations sent on this channel are
	// considered "finalized".
	onStatsRecorded chan<- *invocationInfo
	eg              errgroup.Group

	mu      sync.Mutex // protects(tasks, stopped)
	tasks   chan *recordStatsTask
	stopped bool
}

func newStatsRecorder(env environment.Env, openChannels *sync.WaitGroup, onStatsRecorded chan<- *invocationInfo) *statsRecorder {
	return &statsRecorder{
		env:             env,
		openChannels:    openChannels,
		onStatsRecorded: onStatsRecorded,
		tasks:           make(chan *recordStatsTask, 4096),
	}
}

// Enqueue enqueues a task for the given invocation's stats to be recorded
// once they are available.
func (r *statsRecorder) Enqueue(ctx context.Context, beValues *accumulator.BEValues) {
	persist := &PersistArtifacts{}
	if !*disablePersistArtifacts {
		persist.URIs = slices.Concat(
			beValues.BuildToolLogURIs(),
			beValues.FailedTestOutputURIs(),
			beValues.PassedTestOutputURIs(),
		)
	}

	invocation := beValues.Invocation()

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.stopped {
		alert.UnexpectedEvent(
			"stats_recorder_finalize_after_shutdown",
			"Invocation %q was marked finalized after the stats recorder was shut down.",
			invocation.GetInvocationId())
		return
	}
	jwt := r.env.GetAuthenticator().TrustedJWTFromAuthContext(ctx)
	req := &recordStatsTask{
		invocationInfo: &invocationInfo{
			id:      invocation.GetInvocationId(),
			attempt: invocation.GetAttempt(),
			jwt:     jwt,
		},
		createdAt:                time.Now(),
		files:                    beValues.OutputFiles(),
		invocationStatus:         invocation.GetInvocationStatus(),
		persist:                  persist,
		kytheSSTableResourceName: beValues.KytheSSTableResourceName(),
	}
	select {
	case r.tasks <- req:
		break
	default:
		alert.UnexpectedEvent(
			"stats_recorder_channel_buffer_full",
			"Failed to write cache stats: stats recorder task buffer is full")
	}
}

func (r *statsRecorder) Start() {
	ctx := r.env.GetServerContext()
	for i := 0; i < numStatsRecorderWorkers; i++ {
		metrics.StatsRecorderWorkers.Inc()
		r.eg.Go(func() error {
			defer metrics.StatsRecorderWorkers.Dec()
			for task := range r.tasks {
				r.handleTask(ctx, task)
			}
			return nil
		})
	}
}

func (r *statsRecorder) lookupInvocation(ctx context.Context, ij *invocationInfo) (*tables.Invocation, error) {
	ctx = r.env.GetAuthenticator().AuthContextFromTrustedJWT(ctx, ij.jwt)
	return r.env.GetInvocationDB().LookupInvocation(ctx, ij.id)
}

func (r *statsRecorder) flushInvocationStatsToOLAPDB(ctx context.Context, ij *invocationInfo) error {
	if r.env.GetOLAPDBHandle() == nil || !*writeToOLAPDBEnabled {
		return nil
	}
	inv, err := r.lookupInvocation(ctx, ij)
	if err != nil {
		return status.InternalErrorf("failed to look up invocation for invocation id %q: %s", ij.id, err)
	}

	err = r.env.GetOLAPDBHandle().FlushInvocationStats(ctx, inv)
	if err != nil {
		return err
	}
	// Temporary logging for debugging clickhouse missing data.
	log.CtxInfo(ctx, "Successfully wrote invocation to clickhouse")

	if r.env.GetExecutionCollector() == nil {
		return nil
	}
	const batchSize = 50_000
	var startIndex int64 = 0
	var endIndex int64 = batchSize - 1

	// Always clean up executions in Collector because we are not retrying
	defer func() {
		// Clickhouse ReplicatedMergeTree tables can have replication lag, which can cause
		// reads of executions data to fail.
		// Rather than immediately deleting executions data from Redis after flushing
		// finalized data to Clickhouse, keep the data in Redis a little bit
		// longer, so clients can read executions data from Redis in these cases.
		err := r.env.GetExecutionCollector().ExpireExecutions(ctx, inv.InvocationID, expireRedisExecutionsTTL)
		if err != nil {
			log.CtxErrorf(ctx, "failed to soft delete executions in collector: %s", err)
		}
	}()

	if !olapdbconfig.WriteExecutionsToOLAPDBEnabled() {
		return nil
	}

	// Add the invocation to redis to signal to the executors that they can flush
	// complete Executions into clickhouse directly, in case the PublishOperation
	// is received after the Invocation is complete.
	storedInv := toStoredInvocation(inv)
	if err = r.env.GetExecutionCollector().AddInvocation(ctx, storedInv); err != nil {
		log.CtxErrorf(ctx, "failed to write the complete Invocation to redis: %s", err)
	} else {
		log.CtxInfo(ctx, "Successfully wrote invocation to redis")
	}

	// Once we've flushed execution stats to ClickHouse for this invocation,
	// clean up the invocation => execution links, since these are only needed
	// for listing in-progress executions linked to an invocation, and this
	// listing will now be queryable using ClickHouse.
	defer func() {
		if err := r.env.GetExecutionCollector().DeleteInvocationExecutionLinks(ctx, inv.InvocationID); err != nil {
			log.CtxErrorf(ctx, "Failed to clean up reverse invocation links for invocation %q: %s", inv.InvocationID, err)
		}
	}()
	for {
		endIndex = startIndex + batchSize - 1
		executions, err := r.env.GetExecutionCollector().GetExecutions(ctx, inv.InvocationID, int64(startIndex), int64(endIndex))
		if err != nil {
			return status.InternalErrorf("failed to read executions for invocation_id = %q, startIndex = %d, endIndex = %d from Redis: %s", inv.InvocationID, startIndex, endIndex, err)
		}
		if len(executions) == 0 {
			break
		}
		if err := r.env.GetOLAPDBHandle().FlushExecutionStats(ctx, storedInv, executions); err != nil {
			log.CtxErrorf(ctx, "Failed to flush executions to OLAP DB: %s", err)
			break
		}
		log.CtxInfof(ctx, "successfully wrote %d executions", len(executions))
		// Flush executions to OLAP
		size := len(executions)
		if size < batchSize {
			break
		}
		startIndex += batchSize
	}

	return nil
}

func (r *statsRecorder) maybeIngestKytheSST(ctx context.Context, ij *invocationInfo, sstableResource *rspb.ResourceName) error {
	// first check that css is enabled
	codesearchService := r.env.GetCodesearchService()
	if codesearchService == nil {
		return nil
	}

	if sstableResource == nil {
		return nil
	}

	ctx = r.env.GetAuthenticator().AuthContextFromTrustedJWT(ctx, ij.jwt)
	_, err := codesearchService.IngestAnnotations(ctx, &csinpb.IngestAnnotationsRequest{
		SstableName: sstableResource,
		Async:       true, // don't wait for an answer.
	})
	return err
}

func (r *statsRecorder) handleTask(ctx context.Context, task *recordStatsTask) {
	start := time.Now()
	defer func() {
		metrics.StatsRecorderDuration.Observe(float64(time.Since(start).Microseconds()))
	}()

	// Apply the finalization delay relative to when the invocation was marked
	// finalized, rather than relative to now. Otherwise each worker would be
	// unnecessarily throttled.
	time.Sleep(time.Until(task.createdAt.Add(*cacheStatsFinalizationDelay)))
	ti := &tables.Invocation{InvocationID: task.invocationInfo.id, Attempt: task.invocationInfo.attempt}
	ctx = log.EnrichContext(ctx, log.InvocationIDKey, task.invocationInfo.id)
	if stats := hit_tracker.CollectCacheStats(ctx, r.env, task.invocationInfo.id); stats != nil {
		fillInvocationFromCacheStats(stats, ti)
	} else {
		log.CtxInfo(ctx, "cache stats is not available.")
	}
	if sc := hit_tracker.ScoreCard(ctx, r.env, task.invocationInfo.id); sc != nil {
		scorecard.FillBESMetadata(sc, task.files)
		if err := scorecard.Write(ctx, r.env, task.invocationInfo.id, task.invocationInfo.attempt, sc); err != nil {
			log.CtxErrorf(ctx, "Error writing scorecard blob: %s", err)
		}
	}

	if err := r.maybeIngestKytheSST(ctx, task.invocationInfo, task.kytheSSTableResourceName); err != nil {
		log.CtxWarningf(ctx, "Failed to ingest kythe sst: %s", err)
	}

	updated, err := r.env.GetInvocationDB().UpdateInvocation(ctx, ti)
	if err != nil {
		log.CtxErrorf(ctx, "Failed to write cache stats to primaryDB: %s", err)
	}

	if task.invocationStatus == inspb.InvocationStatus_COMPLETE_INVOCATION_STATUS {
		// only flush complete invocation to clickhouse.
		err = r.flushInvocationStatsToOLAPDB(ctx, task.invocationInfo)
		if err != nil {
			log.CtxErrorf(ctx, "Failed to flush stats to clickhouse: %s", err)
		}
	} else {
		log.CtxInfof(ctx, "skipped writing stats to clickhouse, invocationStatus = %s", task.invocationStatus)
	}
	// Cleanup regardless of whether the stats are flushed successfully to
	// the DB (since we won't retry the flush and we don't need these stats
	// for any other purpose).
	hit_tracker.CleanupCacheStats(ctx, r.env, task.invocationInfo.id)
	if !updated {
		log.CtxWarningf(ctx, "Attempt %d of invocation pre-empted by more recent attempt, no cache stats flushed.", task.invocationInfo.attempt)
		// Don't notify the webhook; the more recent attempt should trigger
		// the notification when it is finalized.
		return
	}

	// Once cache stats are populated, notify the onStatsRecorded channel in
	// a non-blocking fashion.
	select {
	case r.onStatsRecorded <- task.invocationInfo:
		break
	default:
		alert.UnexpectedEvent(
			"webhook_channel_buffer_full",
			"Failed to notify webhook: channel buffer is full",
		)
	}

	ctx = r.env.GetAuthenticator().AuthContextFromTrustedJWT(ctx, task.invocationInfo.jwt)
	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(50) // Max concurrency when copying files from cache->blobstore.

	artifactsUploaded := make(map[string]struct{}, 0)
	for _, uri := range task.persist.URIs {
		uri := uri
		rn, err := digest.ParseDownloadResourceName(uri.Path)
		if err != nil {
			log.CtxErrorf(ctx, "Unparseable artifact URI: %s", err)
			continue
		}
		if rn.IsEmpty() {
			continue
		}
		if _, seen := artifactsUploaded[rn.GetDigest().GetHash()]; seen {
			continue
		}
		artifactsUploaded[rn.GetDigest().GetHash()] = struct{}{}
		eg.Go(func() error {
			// When persisting artifacts, make sure we associate the cache
			// requests with the app, not bazel.
			ctx := usageutil.WithLocalServerLabels(ctx)

			fullPath := path.Join(task.invocationInfo.id, cacheArtifactsBlobstorePath, uri.Path)
			// Only persist artifacts from caches that are hosted on the BuildBuddy
			// domain (but only if we know it).
			if cache_api_url.String() == "" || urlutil.GetDomain(uri.Hostname()) == urlutil.GetDomain(cache_api_url.WithPath("").Hostname()) {
				if err := persistArtifact(ctx, r.env, uri, fullPath); err != nil {
					log.CtxError(ctx, err.Error())
				}
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		log.CtxErrorf(ctx, "Failed to persist cache artifacts to blobstore: %s", err)
	}
}

func (r *statsRecorder) Stop() {
	// Wait for all EventHandler channels to be closed to ensure there will be no
	// more calls to Enqueue.
	// TODO(bduffany): This has a race condition where the server can be shutdown
	// just after the stream request is accepted by the server but before calling
	// openChannels.Add(1). Can fix this by explicitly waiting for the gRPC server
	// shutdown to finish, which ensures all streaming requests have terminated.
	log.Info("StatsRecorder: waiting for EventChannels to be closed before shutting down")
	r.openChannels.Wait()

	log.Info("StatsRecorder: shutting down")
	r.mu.Lock()
	r.stopped = true
	close(r.tasks)
	r.mu.Unlock()

	if err := r.eg.Wait(); err != nil {
		log.Error(err.Error())
	}

	close(r.onStatsRecorded)
}

func persistArtifact(ctx context.Context, env environment.Env, uri *url.URL, path string) error {
	w, err := env.GetBlobstore().Writer(ctx, path)
	if err != nil {
		return status.WrapErrorf(
			err,
			"Failed to open writer to blobstore for path %s to persist cache artifact at %s",
			path,
			uri.String(),
		)
	}
	if err := env.GetPooledByteStreamClient().StreamBytestreamFile(ctx, uri, w); err != nil {
		w.Close()
		return status.WrapErrorf(
			err,
			"Failed to stream to blobstore for path %s to persist cache artifact at %s",
			path,
			uri.String(),
		)
	}
	if err := w.Commit(); err != nil {
		w.Close()
		return status.WrapErrorf(
			err,
			"Failed to commit to blobstore for path %s to persist cache artifact at %s",
			path,
			uri.String(),
		)
	}
	if err := w.Close(); err != nil {
		return status.WrapErrorf(
			err,
			"Failed to close blobstore writer for path %s to persist cache artifact at %s",
			path,
			uri.String(),
		)
	}
	return nil
}

type notifyWebhookTask struct {
	// hook is the webhook to notify of a completed invocation.
	hook interfaces.Webhook
	// invocationInfo contains the invocation ID and JWT for the invocation.
	*invocationInfo
	// invocation is the complete invocation looked up from the invocationInfo.
	invocation *inpb.Invocation
}

func notifyWithTimeout(ctx context.Context, env environment.Env, t *notifyWebhookTask) error {
	start := time.Now()
	defer func() {
		metrics.WebhookNotifyDuration.Observe(float64(time.Since(start).Microseconds()))
	}()

	ctx, cancel := context.WithTimeout(ctx, webhookNotifyTimeout)
	defer cancel()
	// Run the webhook using the authenticated user from the build event stream.
	ij := t.invocationInfo
	ctx = env.GetAuthenticator().AuthContextFromTrustedJWT(ctx, ij.jwt)
	return t.hook.NotifyComplete(ctx, t.invocation)
}

// webhookNotifier listens for invocations to be finalized (including stats)
// and notifies webhooks.
type webhookNotifier struct {
	env environment.Env
	// invocations is a channel of finalized invocations. On each invocation
	// sent to this channel, we notify all configured webhooks.
	invocations <-chan *invocationInfo

	tasks       chan *notifyWebhookTask
	lookupGroup errgroup.Group
	notifyGroup errgroup.Group
}

func newWebhookNotifier(env environment.Env, invocations <-chan *invocationInfo) *webhookNotifier {
	return &webhookNotifier{
		env:         env,
		invocations: invocations,
		tasks:       make(chan *notifyWebhookTask, 4096),
	}
}

func (w *webhookNotifier) Start() {
	ctx := w.env.GetServerContext()

	w.lookupGroup = errgroup.Group{}
	for i := 0; i < numWebhookInvocationLookupWorkers; i++ {
		metrics.WebhookInvocationLookupWorkers.Inc()
		w.lookupGroup.Go(func() error {
			defer metrics.WebhookInvocationLookupWorkers.Dec()
			// Listen for invocations that have been finalized and start a notify
			// webhook task for each webhook.
			for ij := range w.invocations {
				if err := w.lookupAndCreateTask(ctx, ij); err != nil {
					log.Warningf("Failed to lookup invocation before notifying webhook: %s", err)
				}
			}
			return nil
		})
	}

	w.notifyGroup = errgroup.Group{}
	for i := 0; i < numWebhookNotifyWorkers; i++ {
		metrics.WebhookNotifyWorkers.Inc()
		w.notifyGroup.Go(func() error {
			defer metrics.WebhookNotifyWorkers.Dec()
			for task := range w.tasks {
				ctx := log.EnrichContext(ctx, log.InvocationIDKey, task.invocation.GetInvocationId())
				if err := notifyWithTimeout(ctx, w.env, task); err != nil {
					log.CtxWarningf(ctx, "Failed to notify invocation webhook: %s", err)
				}
			}
			return nil
		})
	}
}

func (w *webhookNotifier) lookupAndCreateTask(ctx context.Context, ij *invocationInfo) error {
	start := time.Now()
	defer func() {
		metrics.WebhookInvocationLookupDuration.Observe(float64(time.Since(start).Microseconds()))
	}()

	invocation, err := w.lookupInvocation(ctx, ij)
	if err != nil {
		return err
	}

	// Don't call webhooks for disconnected invocations.
	if invocation.GetInvocationStatus() == inspb.InvocationStatus_DISCONNECTED_INVOCATION_STATUS {
		return nil
	}

	for _, hook := range w.env.GetWebhooks() {
		w.tasks <- &notifyWebhookTask{
			hook:           hook,
			invocationInfo: ij,
			invocation:     invocation,
		}
	}

	return nil
}

func (w *webhookNotifier) Stop() {
	// Make sure we are done sending tasks on the task channel before we close it.
	if err := w.lookupGroup.Wait(); err != nil {
		log.Error(err.Error())
	}
	close(w.tasks)

	if err := w.notifyGroup.Wait(); err != nil {
		log.Error(err.Error())
	}
}

func (w *webhookNotifier) lookupInvocation(ctx context.Context, ij *invocationInfo) (*inpb.Invocation, error) {
	ctx = w.env.GetAuthenticator().AuthContextFromTrustedJWT(ctx, ij.jwt)
	inv, err := LookupInvocation(w.env, ctx, ij.id)
	if err != nil {
		return nil, err
	}
	// If detailed cache stats are enabled, the invocation will be missing the
	// scorecard misses field (with only AC misses) that we used to populate.
	// Populate these here for backwards compatibility.
	if hit_tracker.DetailedStatsEnabled() {
		tok, err := paging.EncodeOffsetLimit(&pgpb.OffsetLimit{Limit: hit_tracker.CacheMissScoreCardLimit})
		if err != nil {
			return nil, status.InternalErrorf("failed to encode page token: %s", err)
		}
		req := &capb.GetCacheScoreCardRequest{
			InvocationId: ij.id,
			PageToken:    tok,
			Filter: &capb.GetCacheScoreCardRequest_Filter{
				Mask: &fieldmaskpb.FieldMask{
					Paths: []string{
						"cache_type",
						"request_type",
						"response_type",
					},
				},
				CacheType:    rspb.CacheType_AC,
				RequestType:  capb.RequestType_READ,
				ResponseType: capb.ResponseType_NOT_FOUND,
			},
		}
		sc, err := scorecard.GetCacheScoreCard(ctx, w.env, req)
		if err != nil {
			log.Warningf("Failed to read cache scorecard for invocation %q: %s", req.GetInvocationId(), err)
		} else {
			inv.ScoreCard = &capb.ScoreCard{Misses: sc.GetResults()}
		}
	}
	return inv, nil
}

func isFinalEvent(obe *pepb.OrderedBuildEvent) bool {
	switch obe.GetEvent().GetEvent().(type) {
	case *bepb.BuildEvent_ComponentStreamFinished:
		return true
	}
	return false
}

func (e *EventChannel) isFirstStartedEvent(bazelBuildEvent *build_event_stream.BuildEvent) bool {
	if e.hasReceivedStartedEvent {
		return false
	}
	_, ok := bazelBuildEvent.GetPayload().(*build_event_stream.BuildEvent_Started)
	return ok
}

func (e *EventChannel) isFirstEventWithOptions(bazelBuildEvent *build_event_stream.BuildEvent) bool {
	switch p := bazelBuildEvent.GetPayload().(type) {
	case *build_event_stream.BuildEvent_Started:
		return p.Started.GetOptionsDescription() != "" && !e.hasReceivedEventWithOptions
	case *build_event_stream.BuildEvent_OptionsParsed:
		return !e.hasReceivedEventWithOptions
	}
	return false
}

func isWorkspaceStatusEvent(bazelBuildEvent *build_event_stream.BuildEvent) bool {
	switch bazelBuildEvent.GetPayload().(type) {
	case *build_event_stream.BuildEvent_WorkspaceStatus:
		return true
	}
	return false
}

func isChildInvocationsConfiguredEvent(bazelBuildEvent *build_event_stream.BuildEvent) bool {
	switch bazelBuildEvent.GetPayload().(type) {
	case *build_event_stream.BuildEvent_ChildInvocationsConfigured:
		return true
	}
	return false
}

func readBazelEvent(obe *pepb.OrderedBuildEvent, out *build_event_stream.BuildEvent) error {
	switch buildEvent := obe.GetEvent().GetEvent().(type) {
	case *bepb.BuildEvent_BazelEvent:
		return buildEvent.BazelEvent.UnmarshalTo(out)
	}
	return fmt.Errorf("Not a bazel event %s", obe)
}

type EventChannel struct {
	ctx            context.Context
	env            environment.Env
	pw             *protofile.BufferedProtoWriter
	beValues       *accumulator.BEValues
	redactor       *redact.StreamingRedactor
	statusReporter *build_status_reporter.BuildStatusReporter
	targetTracker  *target_tracker.TargetTracker
	statsRecorder  *statsRecorder
	collector      interfaces.MetricsCollector
	apiTargetMap   *api_common.TargetMap

	startedEvent                     *build_event_stream.BuildEvent_Started
	bufferedEvents                   []*inpb.InvocationEvent
	wroteBuildMetadata               bool
	numDroppedEventsBeforeProcessing uint64
	initialSequenceNumber            int64
	hasReceivedEventWithOptions      bool
	hasReceivedStartedEvent          bool
	logWriter                        *eventlog.EventLogWriter
	onClose                          func()
	attempt                          uint64

	// isVoid determines whether all EventChannel operations are NOPs. This is set
	// when we're retrying an invocation that is already complete, or is
	// incomplete but was created too far in the past.
	isVoid bool
}

func (e *EventChannel) Context() context.Context {
	return e.ctx
}

func (e *EventChannel) Close() {
	e.onClose()
}

func (e *EventChannel) FinalizeInvocation(iid string) error {
	if e.isVoid || !e.hasReceivedEventWithOptions {
		return nil
	}

	ctx, cancel := background.ExtendContextForFinalization(e.ctx, 10*time.Second)
	defer cancel()

	e.beValues.Finalize(ctx)

	invocation := e.beValues.Invocation()
	invocation.Attempt = e.attempt
	invocation.HasChunkedEventLogs = e.logWriter != nil

	if e.pw != nil {
		if err := e.pw.Flush(ctx); err != nil {
			return err
		}
	}

	if e.logWriter != nil {
		if err := e.logWriter.Close(ctx); err != nil {
			return err
		}
		invocation.LastChunkId = e.logWriter.GetLastChunkId(ctx)
	}

	ti, err := e.tableInvocationFromProto(invocation, iid)
	if err != nil {
		return err
	}

	e.recordInvocationMetrics(ti)
	updated, err := e.env.GetInvocationDB().UpdateInvocation(ctx, ti)
	if err != nil {
		return err
	}
	if !updated {
		e.isVoid = true
		return status.CanceledErrorf("Attempt %d of invocation %s pre-empted by more recent attempt, invocation not finalized.", e.attempt, iid)
	}

	e.flushAPIFacets(iid)

	// Report a disconnect only if we successfully updated the invocation.
	// This reduces the likelihood that the disconnected invocation's status
	// will overwrite any statuses written by a more recent attempt.
	if invocation.GetInvocationStatus() == inspb.InvocationStatus_DISCONNECTED_INVOCATION_STATUS {
		log.CtxWarning(ctx, "Reporting disconnected status for invocation")
		e.statusReporter.ReportDisconnect(ctx)
	}

	e.statsRecorder.Enqueue(ctx, e.beValues)
	log.CtxInfof(ctx, "Finalized invocation in primary DB and enqueued for stats recording (status: %s)", invocation.GetInvocationStatus())
	return nil
}

func fillInvocationFromCacheStats(cacheStats *capb.CacheStats, ti *tables.Invocation) {
	ti.ActionCacheHits = cacheStats.GetActionCacheHits()
	ti.ActionCacheMisses = cacheStats.GetActionCacheMisses()
	ti.ActionCacheUploads = cacheStats.GetActionCacheUploads()
	ti.CasCacheHits = cacheStats.GetCasCacheHits()
	ti.CasCacheMisses = cacheStats.GetCasCacheMisses()
	ti.CasCacheUploads = cacheStats.GetCasCacheUploads()
	ti.TotalDownloadSizeBytes = cacheStats.GetTotalDownloadSizeBytes()
	ti.TotalUploadSizeBytes = cacheStats.GetTotalUploadSizeBytes()
	ti.TotalDownloadTransferredSizeBytes = cacheStats.GetTotalDownloadTransferredSizeBytes()
	ti.TotalUploadTransferredSizeBytes = cacheStats.GetTotalUploadTransferredSizeBytes()
	ti.TotalDownloadUsec = cacheStats.GetTotalDownloadUsec()
	ti.TotalUploadUsec = cacheStats.GetTotalUploadUsec()
	ti.DownloadThroughputBytesPerSecond = cacheStats.GetDownloadThroughputBytesPerSecond()
	ti.UploadThroughputBytesPerSecond = cacheStats.GetUploadThroughputBytesPerSecond()
	ti.TotalCachedActionExecUsec = cacheStats.GetTotalCachedActionExecUsec()
	ti.TotalUncachedActionExecUsec = cacheStats.GetTotalUncachedActionExecUsec()
}

func invocationStatusLabel(ti *tables.Invocation) string {
	if ti.InvocationStatus == int64(inspb.InvocationStatus_COMPLETE_INVOCATION_STATUS) {
		if ti.Success {
			return "success"
		}
		return "failure"
	}
	if ti.InvocationStatus == int64(inspb.InvocationStatus_DISCONNECTED_INVOCATION_STATUS) {
		return "disconnected"
	}
	return "unknown"
}

func (e *EventChannel) getGroupIDForMetrics() string {
	userInfo, err := e.env.GetAuthenticator().AuthenticatedUser(e.ctx)
	if err != nil {
		return interfaces.AuthAnonymousUser
	}
	return userInfo.GetGroupID()
}

func (e *EventChannel) recordInvocationMetrics(ti *tables.Invocation) {
	statusLabel := invocationStatusLabel(ti)
	metrics.InvocationCount.With(prometheus.Labels{
		metrics.InvocationStatusLabel: statusLabel,
		metrics.BazelExitCode:         ti.BazelExitCode,
		metrics.BazelCommand:          ti.Command,
	}).Inc()
	metrics.InvocationDurationUs.With(prometheus.Labels{
		metrics.InvocationStatusLabel: statusLabel,
		metrics.BazelCommand:          ti.Command,
	}).Observe(float64(ti.DurationUsec))
	metrics.InvocationDurationUsExported.With(prometheus.Labels{
		metrics.InvocationStatusLabel: statusLabel,
		metrics.GroupID:               e.getGroupIDForMetrics(),
	}).Observe(float64(ti.DurationUsec))
}

func md5Int64(text string) int64 {
	hash := md5.Sum([]byte(text))
	return int64(binary.BigEndian.Uint64(hash[:8]))
}

func (e *EventChannel) HandleEvent(event *pepb.PublishBuildToolEventStreamRequest) error {
	tStart := time.Now()
	err := e.handleEvent(event)
	duration := time.Since(tStart)
	labels := prometheus.Labels{
		metrics.StatusLabel: fmt.Sprintf("%d", gstatus.Code(err)),
	}
	metrics.BuildEventCount.With(labels).Inc()
	metrics.BuildEventHandlerDurationUs.With(labels).Observe(float64(duration.Microseconds()))
	return err
}

func (e *EventChannel) handleEvent(event *pepb.PublishBuildToolEventStreamRequest) error {
	if e.isVoid {
		return nil
	}

	if event.GetOrderedBuildEvent() == nil {
		return status.InvalidArgumentError("Missing OrderedBuildEvent")
	}

	seqNo := event.GetOrderedBuildEvent().GetSequenceNumber()
	streamID := event.GetOrderedBuildEvent().GetStreamId()
	iid := streamID.GetInvocationId()

	if e.initialSequenceNumber == 0 {
		e.initialSequenceNumber = seqNo
	}
	// We only allow initial sequence numbers greater than one in the case where
	// Bazel failed to receive all of our ACKs after we finalized an invocation
	// (marking it complete). In that case we just void the channel and ACK all
	// events without doing any work.
	if e.initialSequenceNumber > firstExpectedSequenceNumber {
		// TODO: once https://github.com/bazelbuild/bazel/pull/18437 lands in
		// Bazel, log an error if the client attempt number is 1 in this case,
		// since today we're relying on Bazel to always start sending events
		// starting from sequence number 1 in the first attempt.
		log.Infof("Voiding EventChannel for invocation %s: build event stream starts with sequence number > %d (%d), which likely means Bazel is retrying an invocation that we already finalized.", iid, firstExpectedSequenceNumber, e.initialSequenceNumber)
		e.isVoid = true
		return nil
	}

	if isFinalEvent(event.GetOrderedBuildEvent()) {
		return nil
	}

	var bazelBuildEvent build_event_stream.BuildEvent
	if err := readBazelEvent(event.GetOrderedBuildEvent(), &bazelBuildEvent); err != nil {
		log.CtxWarningf(e.ctx, "error reading bazel event: %s", err)
		return err
	}

	invocationEvent := &inpb.InvocationEvent{
		EventTime:      event.GetOrderedBuildEvent().GetEvent().GetEventTime(),
		BuildEvent:     &bazelBuildEvent,
		SequenceNumber: event.GetOrderedBuildEvent().GetSequenceNumber(),
	}

	// Bazel sends an Interrupted exit code in the finished event if the user cancelled the build.
	// Use that signal to cancel any actions that are currently in the remote execution system.
	if f, ok := bazelBuildEvent.GetPayload().(*build_event_stream.BuildEvent_Finished); ok {
		if f.Finished.GetExitCode().GetCode() == InterruptedExitCode && e.env.GetRemoteExecutionService() != nil {
			if err := e.env.GetRemoteExecutionService().Cancel(e.ctx, iid); err != nil {
				log.CtxWarningf(e.ctx, "Could not cancel executions for invocation %q: %s", iid, err)
			}
		}
	}
	if seqNo == 1 {
		log.CtxDebugf(e.ctx, "First event! sequence: %d invocation_id: %s, project_id: %s, notification_keywords: %s", seqNo, iid, event.GetProjectId(), event.GetNotificationKeywords())
	}

	if e.isFirstStartedEvent(&bazelBuildEvent) {
		started, _ := bazelBuildEvent.GetPayload().(*build_event_stream.BuildEvent_Started)

		parsedVersion, err := semver.NewVersion(started.Started.GetBuildToolVersion())
		version := "unknown"
		if err == nil {
			version = fmt.Sprintf("%d.%d", parsedVersion.Major(), parsedVersion.Minor())
		}
		metrics.InvocationsByBazelVersionCount.With(
			prometheus.Labels{metrics.BazelVersion: version}).Inc()

		e.hasReceivedStartedEvent = true
		e.beValues.SetExpectedMetadataEvents(bazelBuildEvent.GetChildren())
	}
	// If this is the first event with options, keep track of the project ID and save any notification keywords.
	if e.isFirstEventWithOptions(&bazelBuildEvent) {
		e.hasReceivedEventWithOptions = true
		log.CtxDebugf(e.ctx, "Received options! sequence: %d invocation_id: %s", seqNo, iid)

		authenticated, err := e.authenticateEvent(&bazelBuildEvent)
		if err != nil {
			return err
		}

		if authenticated {
			if irs := e.env.GetIPRulesService(); irs != nil {
				if err := irs.Authorize(e.ctx); err != nil {
					return err
				}
			}
			baseBBURL, err := subdomain.ReplaceURLSubdomain(e.ctx, e.env, build_buddy_url.String())
			if err != nil {
				return err
			}
			e.statusReporter.SetBaseBuildBuddyURL(baseBBURL)
		}

		invocationUUID, err := uuid.StringToBytes(iid)
		if err != nil {
			return err
		}
		ti := &tables.Invocation{
			InvocationID:     iid,
			InvocationUUID:   invocationUUID,
			InvocationStatus: int64(inspb.InvocationStatus_PARTIAL_INVOCATION_STATUS),
			RedactionFlags:   redact.RedactionFlagStandardRedactions,
			Attempt:          e.attempt,
		}
		if *enableChunkedEventLogs {
			ti.LastChunkId = eventlog.EmptyId
		}

		created, err := e.env.GetInvocationDB().CreateInvocation(e.ctx, ti)
		if err != nil {
			return err
		}
		if !created {
			// We failed to retry an existing invocation
			log.CtxWarningf(e.ctx, "Voiding EventChannel for invocation %s: invocation already exists and is either completed or was last updated over 4 hours ago, so may not be retried.", iid)
			e.isVoid = true
			return nil
		}
		e.attempt = ti.Attempt
		e.ctx = log.EnrichContext(e.ctx, "invocation_attempt", fmt.Sprintf("%d", e.attempt))
		log.CtxInfof(e.ctx, "Created invocation %q, attempt %d", ti.InvocationID, ti.Attempt)
		chunkFileSizeBytes := *chunkFileSizeBytes
		if chunkFileSizeBytes == 0 {
			chunkFileSizeBytes = defaultChunkFileSizeBytes
		}
		e.pw = protofile.NewBufferedProtoWriter(
			e.env.GetBlobstore(),
			GetStreamIdFromInvocationIdAndAttempt(iid, e.attempt),
			chunkFileSizeBytes,
		)
		if *enableChunkedEventLogs {
			numLinesToRetain := getNumActionsFromOptions(&bazelBuildEvent)
			if numLinesToRetain != 0 {
				// the number of lines curses can overwrite is 3 + the ui_actions shown:
				// 1 for the progress tracker, 1 for each action, and 2 blank lines.
				// 0 indicates that curses is not being used.
				numLinesToRetain += 3
			}
			var err error
			e.logWriter, err = eventlog.NewEventLogWriter(
				e.ctx,
				e.env.GetBlobstore(),
				e.env.GetKeyValStore(),
				e.env.GetPubSub(),
				eventlog.GetEventLogPubSubChannel(iid),
				eventlog.GetEventLogPathFromInvocationIdAndAttempt(iid, e.attempt),
				numLinesToRetain,
			)
			if err != nil {
				return err
			}
		}
		// Since this is the first event with options and we just parsed the API key,
		// now is a good time to record invocation usage for the group. Check that
		// this is the first attempt of this invocation, to guarantee that we
		// don't increment the usage on invocation retries.
		if ut := e.env.GetUsageTracker(); ut != nil && ti.Attempt == 1 {
			incrementInvocationUsage(e.ctx, ut)
		}
	} else if !e.hasReceivedEventWithOptions || !e.hasReceivedStartedEvent {
		e.bufferedEvents = append(e.bufferedEvents, invocationEvent)
		if len(e.bufferedEvents) > 100 {
			e.numDroppedEventsBeforeProcessing++
			e.bufferedEvents = e.bufferedEvents[1:]
		}
		return nil
	}

	// Process buffered events.
	for _, event := range e.bufferedEvents {
		if err := e.processSingleEvent(event, iid); err != nil {
			return err
		}
	}
	e.bufferedEvents = nil

	// Process regular events.
	return e.processSingleEvent(invocationEvent, iid)
}

func (e *EventChannel) authenticateEvent(bazelBuildEvent *build_event_stream.BuildEvent) (bool, error) {
	auth := e.env.GetAuthenticator()
	if user, err := auth.AuthenticatedUser(e.ctx); err == nil && user != nil {
		return true, nil
	}
	options, err := extractOptions(bazelBuildEvent)
	if err != nil {
		return false, err
	}
	apiKey, err := authutil.ParseAPIKeyFromString(options)
	if err != nil {
		return false, err
	}
	if apiKey == "" {
		return false, nil
	}
	e.ctx = auth.AuthContextFromAPIKey(e.ctx, apiKey)
	authError := e.ctx.Value(interfaces.AuthContextUserErrorKey)
	if authError != nil {
		if err, ok := authError.(error); ok {
			return false, err
		}
		return false, status.UnknownError(fmt.Sprintf("%v", authError))
	}
	return true, nil
}

func (e *EventChannel) processSingleEvent(event *inpb.InvocationEvent, iid string) error {
	if err := e.redactor.RedactAPIKey(e.ctx, event.GetBuildEvent()); err != nil {
		return err
	}
	if err := e.redactor.RedactMetadata(event.GetBuildEvent()); err != nil {
		return err
	}
	// Accumulate a subset of invocation fields in memory.
	if err := e.beValues.AddEvent(event.GetBuildEvent()); err != nil {
		return err
	}

	switch p := event.GetBuildEvent().GetPayload().(type) {
	case *build_event_stream.BuildEvent_Progress:
		if e.logWriter != nil {
			if _, err := e.logWriter.Write(e.ctx, append([]byte(p.Progress.GetStderr()), []byte(p.Progress.GetStdout())...)); err != nil && err != context.Canceled {
				log.CtxWarningf(e.ctx, "Failed to write build logs for event: %s", err)
			}
			// Don't store the log in the protostream if we're
			// writing it separately to blobstore
			p.Progress.Stderr = ""
			p.Progress.Stdout = ""
		}
	}

	e.targetTracker.TrackTargetsForEvent(e.ctx, event.GetBuildEvent())
	e.statusReporter.ReportStatusForEvent(e.ctx, event.GetBuildEvent())

	if err := e.collectAPIFacets(iid, event.GetBuildEvent()); err != nil {
		log.CtxWarningf(e.ctx, "Error collecting API facets: %s", err)
	}

	// For everything else, just save the event to our buffer and keep on chugging.
	if e.pw != nil {
		if err := e.pw.WriteProtoToStream(e.ctx, event); err != nil {
			return err
		}

		// Small optimization: For certain event types, flush the event stream
		// immediately to show things to the user faster when fetching status
		// of an incomplete build.
		/// Also flush if we haven't in over a minute.
		if shouldFlushImmediately(event.GetBuildEvent()) || e.pw.TimeSinceLastWrite().Minutes() > 1 {
			if err := e.pw.Flush(e.ctx); err != nil {
				return err
			}
		}
	}

	// When we have processed all invocation-level metadata events, update the
	// invocation in the DB so that it can be searched by its commit SHA, user
	// name, etc. even while the invocation is still in progress.
	if !e.wroteBuildMetadata && e.beValues.MetadataIsLoaded() {
		if err := e.writeBuildMetadata(e.ctx, iid); err != nil {
			return err
		}
		e.wroteBuildMetadata = true
	}

	return nil
}

func shouldFlushImmediately(bazelBuildEvent *build_event_stream.BuildEvent) bool {
	// Workspace status event: Most of the command line options and workspace info
	// has come through by then, so we have a good amount of info to show the user
	// about the in-progress build
	//
	// Child invocations configured event: If a child invocation starts, flush
	// the event stream so we can link to the child invocation in the UI
	return isWorkspaceStatusEvent(bazelBuildEvent) || isChildInvocationsConfiguredEvent(bazelBuildEvent)
}

const apiFacetsExpiration = 1 * time.Hour

func (e *EventChannel) flushAPIFacets(iid string) error {
	if e.collector == nil || e.env.GetAPIService() == nil || !e.env.GetAPIService().CacheEnabled() {
		return nil
	}

	userInfo, err := e.env.GetAuthenticator().AuthenticatedUser(e.ctx)
	if userInfo == nil || err != nil {
		return nil
	}

	for label, target := range e.apiTargetMap.Targets {
		b, err := proto.Marshal(target)
		if err != nil {
			return err
		}
		key := api_common.TargetLabelKey(userInfo.GetGroupID(), iid, label)
		if err := e.collector.Set(e.ctx, key, string(b), apiFacetsExpiration); err != nil {
			return err
		}
	}
	return nil
}

func (e *EventChannel) collectAPIFacets(iid string, event *build_event_stream.BuildEvent) error {
	if e.collector == nil || e.env.GetAPIService() == nil || !e.env.GetAPIService().CacheEnabled() {
		return nil
	}

	userInfo, err := e.env.GetAuthenticator().AuthenticatedUser(e.ctx)
	if userInfo == nil || err != nil {
		return nil
	}

	e.apiTargetMap.ProcessEvent(iid, event)

	action := &apipb.Action{
		Id: &apipb.Action_Id{
			InvocationId: iid,
		},
	}
	action = api_common.FillActionFromBuildEvent(event, action)
	if action != nil {
		action = api_common.FillActionOutputFilesFromBuildEvent(event, action)
	} else {
		// early exit if this isn't an action event.
		return nil
	}
	b, err := proto.Marshal(action)
	if err != nil {
		return err
	}
	key := api_common.ActionLabelKey(userInfo.GetGroupID(), iid, action.GetTargetLabel())
	if err := e.collector.ListAppend(e.ctx, key, string(b)); err != nil {
		return err
	}
	if err := e.collector.Expire(e.ctx, key, apiFacetsExpiration); err != nil {
		return err
	}
	return nil
}

func (e *EventChannel) writeBuildMetadata(ctx context.Context, invocationID string) error {
	db := e.env.GetInvocationDB()
	invocationProto := e.beValues.Invocation()
	if e.logWriter != nil {
		invocationProto.LastChunkId = e.logWriter.GetLastChunkId(ctx)
	}
	ti, err := e.tableInvocationFromProto(invocationProto, "" /*=blobID*/)
	if err != nil {
		return err
	}
	ti.Attempt = e.attempt
	updated, err := db.UpdateInvocation(ctx, ti)
	if err != nil {
		return err
	}
	if !updated {
		e.isVoid = true
		return status.CanceledErrorf("Attempt %d of invocation %s pre-empted by more recent attempt, no build metadata written.", e.attempt, invocationID)
	}
	return nil
}

func (e *EventChannel) GetNumDroppedEvents() uint64 {
	return e.numDroppedEventsBeforeProcessing
}

func (e *EventChannel) GetInitialSequenceNumber() int64 {
	return e.initialSequenceNumber
}

func extractOptions(event *build_event_stream.BuildEvent) (string, error) {
	switch p := event.GetPayload().(type) {
	case *build_event_stream.BuildEvent_Started:
		return p.Started.GetOptionsDescription(), nil
	case *build_event_stream.BuildEvent_OptionsParsed:
		return strings.Join(p.OptionsParsed.GetCmdLine(), " "), nil
	}
	return "", nil
}

func getNumActionsFromOptions(event *build_event_stream.BuildEvent) int {
	options, err := extractOptions(event)
	if err != nil {
		log.Warningf("Could not extract options for ui_actions_shown, defaulting to %d: %d", defaultActionsShown, err)
		return defaultActionsShown
	}
	optionsList, err := shlex.Split(options)
	if err != nil {
		log.Warningf("Could not shlex split options '%s' for ui_actions_shown, defaulting to %d: %v", options, defaultActionsShown, err)
		return defaultActionsShown
	}
	actionsShownValues := getOptionValues(optionsList, "ui_actions_shown")
	cursesValues := getOptionValues(optionsList, "curses")
	if len(cursesValues) > 0 {
		curses := cursesValues[len(cursesValues)-1]
		if curses == "no" {
			return 0
		} else if curses != "yes" && curses != "auto" {
			log.Warningf("Unrecognized argument to curses, assuming auto: %v", curses)
		}
	}
	if len(actionsShownValues) > 0 {
		n, err := strconv.Atoi(actionsShownValues[len(actionsShownValues)-1])
		if err != nil {
			log.Warningf("Invalid argument to ui_actions_shown, defaulting to %d: %v", defaultActionsShown, err)
		} else if n < 1 {
			return 1
		} else {
			return n
		}
	}
	return defaultActionsShown
}

func getOptionValues(options []string, optionName string) []string {
	values := []string{}
	flag := "--" + optionName
	for _, option := range options {
		if option == "--" {
			break
		}
		if strings.HasPrefix(option, flag+"=") {
			values = append(values, strings.TrimPrefix(option, flag+"="))
		}
	}
	return values
}

type invocationEventCB func(*inpb.InvocationEvent) error

func streamRawInvocationEvents(env environment.Env, ctx context.Context, streamID string, callback invocationEventCB) error {
	eventAllocator := func() proto.Message { return &inpb.InvocationEvent{} }
	pr := protofile.NewBufferedProtoReader(env.GetBlobstore(), streamID, eventAllocator)
	for {
		event, err := pr.ReadProto(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err := callback(event.(*inpb.InvocationEvent)); err != nil {
			return err
		}
	}
	return nil
}

// LookupInvocation looks up the invocation, including all events. Prefer to use
// LookupInvocationWithCallback whenever possible, which avoids buffering events
// in memory.
func LookupInvocation(env environment.Env, ctx context.Context, iid string) (*inpb.Invocation, error) {
	var events []*inpb.InvocationEvent
	inv, err := LookupInvocationWithCallback(ctx, env, iid, func(event *inpb.InvocationEvent) error {
		// Certain buggy rulesets will mark intermediate output files as
		// important-outputs. This can result in very large BES streams which
		// use a ton of memory and are not displayable by the browser. If we
		// detect a large number of events coming through, begin dropping non-
		// important events so that this invocation can be displayed.
		if len(events) >= *buildEventFilterStartThreshold && !accumulator.IsImportantEvent(event.GetBuildEvent()) {
			return nil
		}
		events = append(events, event)
		return nil
	})
	if err != nil {
		return nil, err
	}
	inv.Event = events
	return inv, nil
}

// LookupInvocationWithCallback looks up an invocation but uses a callback for
// events instead of buffering events into the events list.
//
// TODO: switch to using this API wherever possible.
func LookupInvocationWithCallback(ctx context.Context, env environment.Env, iid string, cb invocationEventCB) (*inpb.Invocation, error) {
	ti, err := env.GetInvocationDB().LookupInvocation(ctx, iid)
	if err != nil {
		if db.IsRecordNotFound(err) {
			return nil, status.NotFoundError("invocation not found")
		}
		return nil, err
	}

	// If this is an incomplete invocation, attempt to fill cache stats
	// from counters rather than trying to read them from invocation b/c
	// they won't be set yet.
	if ti.InvocationStatus == int64(inspb.InvocationStatus_PARTIAL_INVOCATION_STATUS) {
		if cacheStats := hit_tracker.CollectCacheStats(ctx, env, iid); cacheStats != nil {
			fillInvocationFromCacheStats(cacheStats, ti)
		}
	}

	invocation := invocationdb.TableInvocationToProto(ti)

	var scoreCard *capb.ScoreCard
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		// When detailed stats are enabled, the scorecard is not inlined in the
		// invocation.
		if !hit_tracker.DetailedStatsEnabled() {
			// The cache ScoreCard is not stored in the table invocation, so we do this lookup
			// after converting the table invocation to a proto invocation.
			if ti.InvocationStatus == int64(inspb.InvocationStatus_PARTIAL_INVOCATION_STATUS) {
				scoreCard = hit_tracker.ScoreCard(ctx, env, iid)
			} else {
				sc, err := scorecard.Read(ctx, env, iid, ti.Attempt)
				if err != nil {
					log.Warningf("Failed to read scorecard for invocation %s: %s", iid, err)
				} else {
					scoreCard = sc
				}
			}
		}
		return nil
	})

	eg.Go(func() error {
		return FetchAllInvocationEventsWithCallback(ctx, env, invocation, ti.RedactionFlags, cb)
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	invocation.ScoreCard = scoreCard
	return invocation, nil
}

func FetchAllInvocationEventsWithCallback(ctx context.Context, env environment.Env, inv *inpb.Invocation, invRedactionFlags int32, cb invocationEventCB) error {
	var screenWriter *terminal.ScreenWriter
	if !inv.GetHasChunkedEventLogs() {
		var err error
		screenWriter, err = terminal.NewScreenWriter(0)
		if err != nil {
			return err
		}
	}
	var redactor *redact.StreamingRedactor
	if invRedactionFlags&redact.RedactionFlagStandardRedactions != redact.RedactionFlagStandardRedactions {
		// only redact if we hadn't redacted enough, only parse again if we redact
		redactor = redact.NewStreamingRedactor(env)
	}
	beValues := accumulator.NewBEValues(inv)
	structuredCommandLines := []*command_line.CommandLine{}
	streamID := GetStreamIdFromInvocationIdAndAttempt(inv.GetInvocationId(), inv.GetAttempt())
	err := streamRawInvocationEvents(env, ctx, streamID, func(event *inpb.InvocationEvent) error {
		if redactor != nil {
			if err := redactor.RedactAPIKeysWithSlowRegexp(ctx, event.GetBuildEvent()); err != nil {
				return err
			}
			if err := redactor.RedactMetadata(event.GetBuildEvent()); err != nil {
				return err
			}
			if err := beValues.AddEvent(event.GetBuildEvent()); err != nil {
				return err
			}
		}

		switch p := event.GetBuildEvent().GetPayload().(type) {
		case *build_event_stream.BuildEvent_Started:
			// Drop child pattern expanded events since this list can be
			// very long and we don't render these currently.
			event.BuildEvent.Children = nil
		case *build_event_stream.BuildEvent_Expanded:
			if len(event.GetBuildEvent().GetId().GetPattern().GetPattern()) > 0 {
				pattern, truncated := TruncateStringSlice(event.GetBuildEvent().GetId().GetPattern().GetPattern(), maxPatternLengthBytes)
				inv.PatternsTruncated = truncated
				event.BuildEvent.GetId().GetPattern().Pattern = pattern
			}
			// Don't return child TargetConfigured events to the UI; the UI
			// only cares about the actual TargetConfigured event payloads.
			event.BuildEvent.Children = nil
			// UI doesn't render TestSuiteExpansions yet (though we probably
			// should at some point?) So don't return these either.
			p.Expanded.TestSuiteExpansions = nil
		case *build_event_stream.BuildEvent_Progress:
			if screenWriter != nil {
				screenWriter.Write([]byte(p.Progress.GetStderr()))
				screenWriter.Write([]byte(p.Progress.GetStdout()))
			}
			// Don't serve progress event contents to the UI since they are too
			// large. Instead, logs are available either via the
			// console_buffer field or the separate logs RPC.
			p.Progress.Stderr = ""
			p.Progress.Stdout = ""
		case *build_event_stream.BuildEvent_StructuredCommandLine:
			structuredCommandLines = append(structuredCommandLines, p.StructuredCommandLine)
		}

		if err := cb(event); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	// TODO: Can we remove this StructuredCommandLine field? These are
	// already available in the events list.
	inv.StructuredCommandLine = structuredCommandLines
	if screenWriter != nil {
		inv.ConsoleBuffer = screenWriter.Render()
	}
	return nil
}

func (e *EventChannel) tableInvocationFromProto(p *inpb.Invocation, blobID string) (*tables.Invocation, error) {
	uuid, err := uuid.StringToBytes(p.GetInvocationId())
	if err != nil {
		return nil, err
	}

	i := &tables.Invocation{}
	i.InvocationID = p.GetInvocationId() // Required.
	i.InvocationUUID = uuid
	i.Success = p.GetSuccess()
	i.User = p.GetUser()
	i.DurationUsec = p.GetDurationUsec()
	i.Host = p.GetHost()
	i.RepoURL = p.GetRepoUrl()
	if norm, err := gitutil.NormalizeRepoURL(p.GetRepoUrl()); err == nil {
		i.RepoURL = norm.String()
	}
	i.BranchName = p.GetBranchName()
	i.CommitSHA = p.GetCommitSha()
	i.Role = p.GetRole()
	i.Command = p.GetCommand()
	if p.Pattern != nil {
		i.Pattern = invocation_format.ShortFormatPatterns(p.GetPattern())
	}
	i.ActionCount = p.GetActionCount()
	i.BlobID = blobID
	i.InvocationStatus = int64(p.GetInvocationStatus())
	i.LastChunkId = p.GetLastChunkId()
	i.RedactionFlags = redact.RedactionFlagStandardRedactions
	i.Attempt = p.GetAttempt()
	i.BazelExitCode = p.GetBazelExitCode()
	tags, err := invocation_format.JoinTags(p.GetTags())
	if err != nil {
		return nil, err
	}
	i.Tags = tags
	i.ParentRunID = p.GetParentRunId()
	i.RunID = p.GetRunId()

	userGroupPerms, err := perms.ForAuthenticatedGroup(e.ctx, e.env)
	if err != nil {
		return nil, err
	} else {
		i.Perms = userGroupPerms.Perms
	}
	if p.GetReadPermission() == inpb.InvocationPermission_PUBLIC {
		i.Perms |= perms.OTHERS_READ
	}
	i.DownloadOutputsOption = int64(p.GetDownloadOutputsOption())
	i.RemoteExecutionEnabled = p.GetRemoteExecutionEnabled()
	i.UploadLocalResultsEnabled = p.GetUploadLocalResultsEnabled()
	return i, nil
}

func GetStreamIdFromInvocationIdAndAttempt(iid string, attempt uint64) string {
	if attempt == 0 {
		// This invocation predates the attempt-tracking functionality, so its
		// streamId does not contain the attempt number.
		return iid
	}
	return iid + "/" + strconv.FormatUint(attempt, 10)
}

func toStoredInvocation(inv *tables.Invocation) *sipb.StoredInvocation {
	return &sipb.StoredInvocation{
		InvocationId:     inv.InvocationID,
		User:             inv.User,
		Host:             inv.Host,
		Pattern:          inv.Pattern,
		Role:             inv.Role,
		BranchName:       inv.BranchName,
		CommitSha:        inv.CommitSHA,
		RepoUrl:          inv.RepoURL,
		Command:          inv.Command,
		InvocationStatus: inv.InvocationStatus,
		Success:          inv.Success,
		Tags:             inv.Tags,
	}
}

func incrementInvocationUsage(ctx context.Context, ut interfaces.UsageTracker) {
	labels, err := usageutil.LabelsForUsageRecording(ctx, usageutil.ServerName())
	if err != nil {
		log.CtxWarningf(ctx, "Failed to compute invocation usage labels: %s", err)
		return
	}
	if err := ut.Increment(ctx, labels, &tables.UsageCounts{Invocations: 1}); err != nil {
		log.CtxWarningf(ctx, "Failed to increment invocation usage: %s", err)
		return
	}
}

// TruncateStringSlice truncates the given string slice so that when the strings
// are joined with a space (" "), the total byte length of the resulting string
// does not exceed the given character limit.
func TruncateStringSlice(strs []string, charLimit int) (truncatedList []string, truncated bool) {
	length := 0
	for i, s := range strs {
		if i > 0 {
			// When rendered in the UI, each arg except the first will be
			// preceded by a space. Count this towards the char limit.
			length += 1
		}
		if length+len(s) > charLimit {
			return strs[:i], true
		}
		length += len(s)
	}
	return strs, false
}
