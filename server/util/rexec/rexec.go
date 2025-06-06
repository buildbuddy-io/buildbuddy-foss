// package rexec provides utility functions for remote execution clients.
package rexec

import (
	"bytes"
	"context"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/buildbuddy-io/buildbuddy/server/environment"
	"github.com/buildbuddy-io/buildbuddy/server/interfaces"
	"github.com/buildbuddy-io/buildbuddy/server/remote_cache/cachetools"
	"github.com/buildbuddy-io/buildbuddy/server/remote_cache/digest"
	"github.com/buildbuddy-io/buildbuddy/server/util/proto"
	"github.com/buildbuddy-io/buildbuddy/server/util/retry"
	"github.com/buildbuddy-io/buildbuddy/server/util/status"
	"golang.org/x/sync/errgroup"
	"google.golang.org/genproto/googleapis/longrunning"

	repb "github.com/buildbuddy-io/buildbuddy/proto/remote_execution"
	rspb "github.com/buildbuddy-io/buildbuddy/proto/resource"
	gstatus "google.golang.org/grpc/status"
)

// MakeEnv assembles a list of EnvironmentVariable protos from a list of
// NAME=VALUE pairs. If the same name is specified more than once, the last one
// wins. The entries are sorted by name, so that the environment variables are
// cache-friendly.
func MakeEnv(pairs ...string) ([]*repb.Command_EnvironmentVariable, error) {
	m, err := parsePairs(pairs)
	if err != nil {
		return nil, err
	}
	names := slices.Sorted(maps.Keys(m))
	out := make([]*repb.Command_EnvironmentVariable, 0, len(m))
	for _, name := range names {
		out = append(out, &repb.Command_EnvironmentVariable{
			Name:  name,
			Value: m[name],
		})
	}
	return out, nil
}

// MakePlatform assembles a Platform proto from a list of NAME=VALUE pairs. If
// the same name is specified more than once, the last one wins. The entries are
// sorted by name, so that the platform is cache-friendly.
func MakePlatform(pairs ...string) (*repb.Platform, error) {
	m, err := parsePairs(pairs)
	if err != nil {
		return nil, err
	}
	names := slices.Sorted(maps.Keys(m))
	p := &repb.Platform{Properties: make([]*repb.Platform_Property, 0, len(names))}
	for _, name := range names {
		p.Properties = append(p.Properties, &repb.Platform_Property{
			Name:  name,
			Value: m[name],
		})
	}
	return p, nil
}

// LookupEnv returns the value of the environment variable with the given name,
// or an empty string if the variable is not present. The second return value
// indicates whether the variable was found.
func LookupEnv(envs []*repb.Command_EnvironmentVariable, name string) (string, bool) {
	for _, e := range envs {
		if e.Name == name {
			return e.Value, true
		}
	}
	return "", false
}

// NormalizeCommand produces a canonicalized version of the given command
// that is suitable for caching, without altering the semantics of the command.
func NormalizeCommand(cmd *repb.Command) {
	if cmd == nil {
		return
	}
	cmd.EnvironmentVariables = normalizedEnv(cmd.EnvironmentVariables)
	if cmd.Platform != nil {
		cmd.Platform.Properties = normalizedPlatform(cmd.Platform.Properties)
	}
}

// normalizedEnv returns environment variables sorted alphabetically by name. If
// the same name is specified more than once in the original list, the last one
// wins.
func normalizedEnv(envs []*repb.Command_EnvironmentVariable) []*repb.Command_EnvironmentVariable {
	m := make(map[string]string, len(envs))
	for _, e := range envs {
		m[e.Name] = e.Value
	}
	names := slices.Sorted(maps.Keys(m))
	out := make([]*repb.Command_EnvironmentVariable, 0, len(m))
	for _, name := range names {
		out = append(out, &repb.Command_EnvironmentVariable{
			Name:  name,
			Value: m[name],
		})
	}
	return out
}

// normalizedPlatform returns platform properties sorted alphabetically by name.
// If the same name is specified more than once in the original list, the last
// one wins.
func normalizedPlatform(props []*repb.Platform_Property) []*repb.Platform_Property {
	m := make(map[string]string, len(props))
	for _, p := range props {
		m[p.Name] = p.Value
	}

	uniqueProps := make([]*repb.Platform_Property, 0, len(m))
	for k, v := range m {
		uniqueProps = append(uniqueProps, &repb.Platform_Property{
			Name:  k,
			Value: v,
		})
	}

	sort.Slice(uniqueProps, func(i, j int) bool {
		return uniqueProps[i].Name < uniqueProps[j].Name
	})
	return uniqueProps
}

// parsePairs parses a list of "NAME=VALUE" pairs into a map. If the same NAME
// appears more than once, the last one wins.
func parsePairs(pairs []string) (map[string]string, error) {
	m := map[string]string{}
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, status.InvalidArgumentErrorf("invalid environment variable %q (expected NAME=VALUE)", pair)
		}
		m[parts[0]] = parts[1]
	}
	return m, nil
}

// Prepare uploads an Action and all of its dependencies to cache, populating
// uploaded digests into the Action.
//
// The `Action.command_digest` field is optional. If it is set, the command
// digest is expected to already have been uploaded. If it is not set, the given
// Command is uploaded to cache, and the resulting command digest is set on the
// Action.
//
// The `Action.input_root_digest` field is optional. If it is set, the input
// root digest is expected to already have been uploaded. If it is not set, the
// given local inputRootDir path is walked and recursively uploaded to cache,
// and the resulting root directory digest is set on the Action. An empty string
// for inputRootDir is treated as an empty directory, meaning the action will
// run from an empty input root.
//
// TODO: The REAPI spec says platform properties and environment variables
// "MUST" be sorted alphabetically by name. We should either automatically
// normalize here, or return an error if input is not normalized.
func Prepare(ctx context.Context, env environment.Env, instanceName string, digestFunction repb.DigestFunction_Value, action *repb.Action, cmd *repb.Command, inputRootDir string) (*rspb.ResourceName, error) {
	var commandDigest, inputRootDigest *repb.Digest
	eg, egctx := errgroup.WithContext(ctx)
	if action.CommandDigest != nil && cmd != nil {
		return nil, status.InvalidArgumentErrorf("cannot specify both an Action.command_digest and a Command")
	}
	if action.CommandDigest == nil && cmd == nil {
		return nil, status.InvalidArgumentErrorf("must specify either Action.command_digest or a Command")
	}
	if inputRootDir != "" && action.InputRootDigest != nil {
		return nil, status.InvalidArgumentErrorf("cannot specify both an input root directory and Action.input_root_digest")
	}
	if cmd != nil {
		eg.Go(func() error {
			d, err := cachetools.UploadProto(egctx, env.GetByteStreamClient(), instanceName, digestFunction, cmd)
			if err != nil {
				return err
			}
			commandDigest = d
			return nil
		})
	}
	if inputRootDir != "" {
		eg.Go(func() error {
			d, _, err := cachetools.UploadDirectoryToCAS(egctx, env, instanceName, digestFunction, inputRootDir)
			if err != nil {
				return err
			}
			inputRootDigest = d
			return nil
		})
	} else if action.InputRootDigest == nil {
		// If running an action without an input root, set it to the digest of
		// an empty directory, since the input_root_digest field is required.
		d, err := digest.Compute(bytes.NewReader(nil), digestFunction)
		if err != nil {
			return nil, err
		}
		inputRootDigest = d
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	if commandDigest != nil {
		action.CommandDigest = commandDigest
	}
	if inputRootDigest != nil {
		action.InputRootDigest = inputRootDigest
	}
	actionDigest, err := cachetools.UploadProto(ctx, env.GetByteStreamClient(), instanceName, digestFunction, action)
	if err != nil {
		return nil, err
	}
	actionResourceName := digest.NewCASResourceName(actionDigest, instanceName, digestFunction).ToProto()
	return actionResourceName, nil
}

// Start begins an Execute stream for the given remote action.
func Start(ctx context.Context, env environment.Env, actionResourceName *rspb.ResourceName) (*RetryingStream, error) {
	req := &repb.ExecuteRequest{
		InstanceName:    actionResourceName.GetInstanceName(),
		ActionDigest:    actionResourceName.GetDigest(),
		DigestFunction:  actionResourceName.GetDigestFunction(),
		SkipCacheLookup: true,
	}
	stream, err := env.GetRemoteExecutionClient().Execute(ctx, req)
	if err != nil {
		return nil, err
	}
	return NewRetryingStream(ctx, env.GetRemoteExecutionClient(), stream, ""), nil
}

// Wait waits for command execution to complete, and returns the COMPLETE stage
// operation response.
func Wait(stream *RetryingStream) (*Response, error) {
	for {
		op, err := stream.Recv()
		if err != nil {
			return nil, err
		}
		if op.Done {
			return op, nil
		}
	}
}

// Result runs the command and returns the result. If the command has already
// been started, it waits for the existing execution to complete.
func GetResult(ctx context.Context, env environment.Env, instanceName string, digestFunction repb.DigestFunction_Value, res *repb.ActionResult) (*interfaces.CommandResult, error) {
	var stdout, stderr bytes.Buffer
	eg, egctx := errgroup.WithContext(ctx)
	if res.GetStdoutDigest() != nil {
		eg.Go(func() error {
			rn := digest.NewCASResourceName(res.GetStdoutDigest(), instanceName, digestFunction)
			return cachetools.GetBlob(egctx, env.GetByteStreamClient(), rn, &stdout)
		})
	}
	if res.GetStderrDigest() != nil {
		eg.Go(func() error {
			rn := digest.NewCASResourceName(res.GetStderrDigest(), instanceName, digestFunction)
			return cachetools.GetBlob(egctx, env.GetByteStreamClient(), rn, &stderr)
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return &interfaces.CommandResult{
		ExitCode: int(res.GetExitCode()),
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
	}, nil
}

// RetryingStream implements a reliable operation stream.
//
// It keeps track of the operation name internally, and provides a Recv() func
// which re-establishes the stream transparently if the operation name has been
// established.
type RetryingStream struct {
	ctx    context.Context
	cancel context.CancelFunc
	client repb.ExecutionClient
	stream repb.Execution_ExecuteClient
	name   string
}

func NewRetryingStream(ctx context.Context, client repb.ExecutionClient, stream repb.Execution_ExecuteClient, name string) *RetryingStream {
	ctx, cancel := context.WithCancel(ctx)
	return &RetryingStream{
		ctx:    ctx,
		cancel: cancel,
		client: client,
		stream: stream,
		name:   name,
	}
}

// Name returns the operation name, if known.
func (s *RetryingStream) Name() string {
	return s.name
}

// Recv attempts to reliably return the next operation on the named stream.
//
// If the stream is disconnected and the operation name has been received, it
// will attempt to reconnect with WaitExecution.
func (s *RetryingStream) Recv() (*Response, error) {
	r := retry.DefaultWithContext(s.ctx)
	for {
		op, err := s.stream.Recv()
		if err == nil {
			if op.GetName() != "" {
				s.name = op.GetName()
			}
			return UnpackOperation(op)
		}
		if !status.IsUnavailableError(err) || s.name == "" {
			return nil, err
		}
		if !r.Next() {
			return nil, s.ctx.Err()
		}
		req := &repb.WaitExecutionRequest{Name: s.name}
		next, err := s.client.WaitExecution(s.ctx, req)
		if err != nil {
			return nil, err
		}
		s.stream.CloseSend()
		s.stream = next
	}
}

func (s *RetryingStream) CloseSend() error {
	var err error
	if s.stream != nil {
		err = s.stream.CloseSend()
		s.stream = nil
	}
	s.client = nil
	s.cancel()
	return err
}

// Response contains an operation along with its execution-specific payload.
type Response struct {
	*longrunning.Operation

	// ExecuteOperationMetadata contains any metadata unpacked from the
	// operation.
	ExecuteOperationMetadata *repb.ExecuteOperationMetadata
	// ExecuteResponse contains any response unpacked from the operation.
	ExecuteResponse *repb.ExecuteResponse
	// Err contains any error parsed from the ExecuteResponse status field.
	Err error
}

// UnpackOperation unmarshals all expected execution-specific fields from the
// given operationn.
func UnpackOperation(op *longrunning.Operation) (*Response, error) {
	msg := &Response{Operation: op}
	if op.GetResponse() != nil {
		msg.ExecuteResponse = &repb.ExecuteResponse{}
		if err := op.GetResponse().UnmarshalTo(msg.ExecuteResponse); err != nil {
			return nil, err
		}
	}
	if op.GetMetadata() != nil {
		msg.ExecuteOperationMetadata = &repb.ExecuteOperationMetadata{}
		if err := op.GetMetadata().UnmarshalTo(msg.ExecuteOperationMetadata); err != nil {
			return nil, err
		}
	}
	msg.Err = gstatus.FromProto(msg.ExecuteResponse.GetStatus()).Err()
	return msg, nil
}

// AuxiliaryMetadata searches for auxiliary metadata for a type matching the
// full name of the given proto message descriptor. If one is found, it
// unmarshals the type into the given message.
// It returns whether the type was found as well as whether there was an error
// unmarshaling.
func AuxiliaryMetadata(md *repb.ExecutedActionMetadata, pb proto.Message) (ok bool, err error) {
	typeURL := "type.googleapis.com/" + string(pb.ProtoReflect().Descriptor().FullName())
	for _, m := range md.GetAuxiliaryMetadata() {
		if m.TypeUrl == typeURL {
			if err := m.UnmarshalTo(pb); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}

// Retryable returns false if the error is a configuration error
// that will persist, even despite retries.
func Retryable(err error) bool {
	taskMisconfigured := status.IsInvalidArgumentError(err) ||
		status.IsFailedPreconditionError(err) ||
		status.IsUnauthenticatedError(err)
	return !taskMisconfigured
}
