package build_event_server

import (
	"context"
	"io"
	"sort"
	"sync"

	"github.com/buildbuddy-io/buildbuddy/server/environment"
	"github.com/buildbuddy-io/buildbuddy/server/interfaces"
	"github.com/buildbuddy-io/buildbuddy/server/metrics"
	"github.com/buildbuddy-io/buildbuddy/server/real_environment"
	"github.com/buildbuddy-io/buildbuddy/server/util/log"
	"github.com/buildbuddy-io/buildbuddy/server/util/status"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	bepb "github.com/buildbuddy-io/buildbuddy/proto/build_events"
	pepb "github.com/buildbuddy-io/buildbuddy/proto/publish_build_event"
)

type BuildEventProtocolServer struct {
	env environment.Env
	// If true, wait until orwarding clients acknowledge.
	synchronous bool
}

func Register(env *real_environment.RealEnv) error {
	// Register to handle build event protocol messages.
	buildEventServer, err := NewBuildEventProtocolServer(env, false)
	if err != nil {
		return status.InternalErrorf("Error initializing BuildEventProtocolServer: %s", err)
	}
	env.SetBuildEventServer(buildEventServer)
	return nil
}

func NewBuildEventProtocolServer(env environment.Env, synchronous bool) (*BuildEventProtocolServer, error) {
	return &BuildEventProtocolServer{
		env:         env,
		synchronous: synchronous,
	}, nil
}

func (s *BuildEventProtocolServer) PublishLifecycleEvent(ctx context.Context, req *pepb.PublishLifecycleEventRequest) (*emptypb.Empty, error) {
	eg, ctx := errgroup.WithContext(ctx)
	for _, c := range s.env.GetBuildEventProxyClients() {
		client := c
		eg.Go(func() error {
			client.PublishLifecycleEvent(ctx, req)
			return nil
		})
	}
	if s.synchronous {
		eg.Wait()
	}
	return &emptypb.Empty{}, nil
}

func closeForwardingStreams(clients []pepb.PublishBuildEvent_PublishBuildToolEventStreamClient) {
	for _, c := range clients {
		c.CloseSend()
	}
}

// Handles Streaming BuildToolEvent
// From the bazel client: (Read more in BuildEventServiceUploader.java.)
// {@link BuildEventServiceUploaderCommands#OPEN_STREAM} is the first event and opens a
// bidi streaming RPC for sending build events and receiving ACKs.
// {@link BuildEventServiceUploaderCommands#SEND_REGULAR_BUILD_EVENT} sends a build event to
// the server. Sending of the Nth build event does
// does not wait for the ACK of the N-1th build event to have been received.
// {@link BuildEventServiceUploaderCommands#SEND_LAST_BUILD_EVENT} sends the last build event
// and half closes the RPC.
// {@link BuildEventServiceUploaderCommands#ACK_RECEIVED} is executed for every ACK from
// the server and checks that the ACKs are in the correct order.
// {@link BuildEventServiceUploaderCommands#STREAM_COMPLETE} checks that all build events
// have been sent and all ACKs have been received. If not it invokes a retry logic that may
// decide to re-send every build event for which an ACK has not been received. If so, it
// adds an OPEN_STREAM event.
func (s *BuildEventProtocolServer) PublishBuildToolEventStream(stream pepb.PublishBuildEvent_PublishBuildToolEventStreamServer) error {
	metrics.InvocationOpenStreams.Inc()
	defer metrics.InvocationOpenStreams.Dec()

	ctx := stream.Context()
	// Semantically, the protocol requires we ack events in order.
	acks := make([]int, 0)
	var streamID *bepb.StreamId
	var channel interfaces.BuildEventChannel

	eg, ctx := errgroup.WithContext(ctx)
	forwardingStreams := make([]pepb.PublishBuildEvent_PublishBuildToolEventStreamClient, 0)
	var closeStreamsOnce sync.Once
	defer closeStreamsOnce.Do(func() { closeForwardingStreams(forwardingStreams) })

	for _, client := range s.env.GetBuildEventProxyClients() {
		fwdStream, err := client.PublishBuildToolEventStream(ctx, grpc.WaitForReady(false))
		if err != nil {
			log.CtxWarningf(ctx, "Failed to proxy build event stream: %s", err)
			if s.synchronous {
				return err
			} else {
				continue
			}
		}
		eg.Go(func() error {
			for {
				_, err := fwdStream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					if !s.synchronous {
						log.CtxWarningf(ctx, "Proxying build event stream failed: recv: %s", err)
						return nil
					}
					return status.WrapError(err, "recv from proxy stream")
				}
			}
			return nil
		})
		forwardingStreams = append(forwardingStreams, fwdStream)
	}

	if len(forwardingStreams) > 0 {
		log.CtxInfof(ctx, "Started build event forwarding streams")
	}

	disconnectWithErr := func(e error) error {
		if channel != nil && streamID != nil {
			log.CtxWarningf(ctx, "Disconnecting invocation %q: %s", streamID.GetInvocationId(), e)
			if err := channel.FinalizeInvocation(streamID.GetInvocationId()); err != nil {
				log.CtxWarningf(ctx, "Error finalizing invocation %q during disconnect: %s", streamID.GetInvocationId(), err)
			}
		}
		return e
	}

	errCh := make(chan error, 1)
	inCh := make(chan *pepb.PublishBuildToolEventStreamRequest)

	// Listen on request stream in the background
	go func() {
		for {
			in, err := stream.Recv()
			if err != nil {
				errCh <- err
				return
			}
			// Recv can return multiple messages back to back so rather than
			// worrying about buffering the messages bail out if the context
			// is done.
			select {
			case inCh <- in:
			case <-stream.Context().Done():
				return
			}

		}
	}()

	channelCtx := ctx
	for {
		select {
		case <-channelCtx.Done():
			return disconnectWithErr(status.FromContextError(channelCtx))
		case err := <-errCh:
			if err == io.EOF {
				if s.synchronous {
					// Close the streams early so that we can Wait() for any forwarding errors.
					log.CtxInfof(ctx, "Closing build event forwarding stream")
					closeStreamsOnce.Do(func() { closeForwardingStreams(forwardingStreams) })
					if err := eg.Wait(); err != nil {
						return disconnectWithErr(err)
					}
				}
				if channel == nil {
					log.CtxInfo(ctx, "Closing empty channel.")
					return nil
				}
				return postProcessStream(ctx, channel, streamID, acks, stream)
			}
			log.CtxWarningf(ctx, "Error receiving build event stream %+v: %s", streamID, err)
			return disconnectWithErr(err)
		case in := <-inCh:
			if streamID == nil {
				if in.GetOrderedBuildEvent().GetStreamId().GetInvocationId() == "" {
					return status.FailedPreconditionError("Missing invocation ID")
				}

				streamID = in.GetOrderedBuildEvent().GetStreamId()
				ctx = log.EnrichContext(ctx, log.InvocationIDKey, streamID.GetInvocationId())
				newChannel, err := s.env.GetBuildEventHandler().OpenChannel(ctx, streamID.GetInvocationId())
				if err != nil {
					log.CtxWarningf(ctx, "Failed to open invocation channel: %s", err)
					return err
				}
				log.CtxInfo(ctx, "Opened invocation channel")
				channel = newChannel
				channelCtx = channel.Context()
				defer channel.Close()
			}

			if err := channel.HandleEvent(in); err != nil {
				log.CtxWarningf(ctx, "Error handling event; this means a broken build command: %s", err)
				return disconnectWithErr(err)
			}
			for _, stream := range forwardingStreams {
				err := stream.Send(in)
				// Async proxying is best effort--only handle errors in synchronous mode
				if s.synchronous && err != nil {
					return disconnectWithErr(err)
				}
			}
			acks = append(acks, int(in.GetOrderedBuildEvent().GetSequenceNumber()))
		}
	}
}

// postProcessStream checks to ensure that we have received all build events by
// sorting `acks` in-place and iterating over it, checking to ensure it is a
// list of // consecutive ascending ints starting with the channel's initial
// sequence number. If it is not, it returns an error to force the client to
// open a new channel that resends everything; otherwise, it finalizes the
// channel and then sends a stream of ACKs to the client which ACKs each build
// event.
func postProcessStream(ctx context.Context, channel interfaces.BuildEventChannel, streamID *bepb.StreamId, acks []int, stream pepb.PublishBuildEvent_PublishBuildToolEventStreamServer) error {
	if channel.GetNumDroppedEvents() > 0 {
		log.CtxWarningf(ctx, "We got over 100 build events before an event with options for invocation %s. Dropped the %d earliest event(s).",
			streamID.GetInvocationId(), channel.GetNumDroppedEvents())
	}

	// Check that we have received all acks! If we haven't bail out since we
	// don't want to ack *anything*. This forces the client to retransmit
	// everything all at once, which means we don't need to worry about
	// cross-server consistency of messages in an invocation.
	sort.Ints(acks)

	expectedSeqNo := channel.GetInitialSequenceNumber()
	for _, ack := range acks {
		if ack != int(expectedSeqNo) {
			log.CtxWarningf(ctx, "Missing ack: saw %d and wanted %d. Bailing!", ack, expectedSeqNo)
			return status.UnknownErrorf("event sequence number mismatch: received %d, wanted %d", ack, expectedSeqNo)
		}
		expectedSeqNo++
	}

	if err := channel.FinalizeInvocation(streamID.GetInvocationId()); err != nil {
		log.CtxWarningf(ctx, "Error finalizing invocation %q: %s", streamID.GetInvocationId(), err)
		return err
	}

	// Finally, ack everything.
	for _, ack := range acks {
		rsp := &pepb.PublishBuildToolEventStreamResponse{
			StreamId:       streamID,
			SequenceNumber: int64(ack),
		}
		if err := stream.Send(rsp); err != nil {
			log.CtxWarningf(ctx, "Error sending ack stream for invocation %q: %s", streamID.GetInvocationId(), err)
			return err
		}
	}
	return nil
}
