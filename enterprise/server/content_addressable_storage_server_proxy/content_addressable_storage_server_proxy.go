package content_addressable_storage_server_proxy

import (
	"context"
	"fmt"
	"io"

	"github.com/buildbuddy-io/buildbuddy/server/environment"
	"github.com/buildbuddy-io/buildbuddy/server/real_environment"
	"github.com/buildbuddy-io/buildbuddy/server/remote_cache/digest"
	"github.com/buildbuddy-io/buildbuddy/server/util/log"
	"github.com/buildbuddy-io/buildbuddy/server/util/status"

	"google.golang.org/grpc/codes"

	repb "github.com/buildbuddy-io/buildbuddy/proto/remote_execution"
)

type CASServerProxy struct {
	env    environment.Env
	local  repb.ContentAddressableStorageClient
	remote repb.ContentAddressableStorageClient
}

func Register(env *real_environment.RealEnv) error {
	casServer, err := New(env)
	if err != nil {
		return status.InternalErrorf("Error initializing ContentAddressableStorageServerProxy: %s", err)
	}
	env.SetCASServer(casServer)
	return nil
}

func New(env environment.Env) (*CASServerProxy, error) {
	local := env.GetLocalCASClient()
	if local == nil {
		return nil, fmt.Errorf("A local ContentAddressableStorageClient is required to enable the ContentAddressableStorageServerProxy")
	}
	remote := env.GetContentAddressableStorageClient()
	if remote == nil {
		return nil, fmt.Errorf("A remote ContentAddressableStorageClient is required to enable the ContentAddressableStorageServerProxy")
	}
	return &CASServerProxy{
		env:    env,
		local:  local,
		remote: remote,
	}, nil
}

// TODO(iain): update remote atimes.
func (s *CASServerProxy) FindMissingBlobs(ctx context.Context, req *repb.FindMissingBlobsRequest) (*repb.FindMissingBlobsResponse, error) {
	resp, err := s.local.FindMissingBlobs(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(resp.MissingBlobDigests) == 0 {
		return resp, nil
	}
	remoteReq := repb.FindMissingBlobsRequest{
		InstanceName:   req.InstanceName,
		BlobDigests:    resp.MissingBlobDigests,
		DigestFunction: req.DigestFunction,
	}
	return s.remote.FindMissingBlobs(ctx, &remoteReq)
}

func (s *CASServerProxy) BatchUpdateBlobs(ctx context.Context, req *repb.BatchUpdateBlobsRequest) (*repb.BatchUpdateBlobsResponse, error) {
	_, err := s.local.BatchUpdateBlobs(context.Background(), req)
	if err != nil {
		log.Warningf("Local BatchUpdateBlobs error: %s", err)
	}
	return s.remote.BatchUpdateBlobs(ctx, req)
}

func (s *CASServerProxy) BatchReadBlobs(ctx context.Context, req *repb.BatchReadBlobsRequest) (*repb.BatchReadBlobsResponse, error) {
	mergedResp := repb.BatchReadBlobsResponse{}
	mergedDigests := []*repb.Digest{}
	localResp, err := s.local.BatchReadBlobs(ctx, req)
	if err != nil {
		return s.batchReadBlobsRemote(ctx, req)
	}
	for _, resp := range localResp.Responses {
		if resp.Status.Code == int32(codes.OK) {
			mergedResp.Responses = append(mergedResp.Responses, resp)
			mergedDigests = append(mergedDigests, resp.Digest)
		}
	}
	if len(mergedResp.Responses) == len(req.Digests) {
		return &mergedResp, nil
	}

	// digest.Diff returns a set of differences between two sets of digests,
	// but the protocol requires the server return multiple responses if the
	// same digest is requested multiple times. Count the number of client
	// requests per blob so we can duplicate responses that many times before
	// returning to the client.
	_, missing := digest.Diff(req.Digests, mergedDigests)
	cardinality := make(map[digest.Key]int)
	for _, d := range req.Digests {
		k := digest.NewKey(d)
		if _, ok := cardinality[k]; ok {
			cardinality[k] = cardinality[k] + 1
		} else {
			cardinality[k] = 1
		}
	}
	remoteReq := repb.BatchReadBlobsRequest{
		InstanceName:          req.InstanceName,
		Digests:               missing,
		AcceptableCompressors: req.AcceptableCompressors,
		DigestFunction:        req.DigestFunction,
	}
	remoteResp, err := s.batchReadBlobsRemote(ctx, &remoteReq)
	if err != nil {
		return nil, err
	}

	// Now go through and duplicate each response as many times as the client
	// requested it.
	for _, response := range remoteResp.Responses {
		c, ok := cardinality[digest.NewKey(response.Digest)]
		if !ok {
			log.Warningf("Received unexpected digest from remote CAS.BatchReadBlobs: %s/%d", response.Digest.Hash, response.Digest.SizeBytes)
		}
		for i := 0; i < c; i++ {
			mergedResp.Responses = append(mergedResp.Responses, response)
		}
	}

	return &mergedResp, nil
}

func (s *CASServerProxy) batchReadBlobsRemote(ctx context.Context, readReq *repb.BatchReadBlobsRequest) (*repb.BatchReadBlobsResponse, error) {
	readResp, err := s.remote.BatchReadBlobs(ctx, readReq)
	if err != nil {
		return nil, err
	}
	updateReq := repb.BatchUpdateBlobsRequest{
		InstanceName:   readReq.InstanceName,
		DigestFunction: readReq.DigestFunction,
	}
	for _, response := range readResp.Responses {
		if response.Status.Code != int32(codes.OK) {
			continue
		}
		updateReq.Requests = append(updateReq.Requests, &repb.BatchUpdateBlobsRequest_Request{
			Digest:     response.Digest,
			Data:       response.Data,
			Compressor: response.Compressor,
		})
	}
	if _, err := s.local.BatchUpdateBlobs(context.Background(), &updateReq); err != nil {
		log.Warningf("Error locally updating blobs: %s", err)
	}
	return readResp, nil
}

func (s *CASServerProxy) GetTree(req *repb.GetTreeRequest, stream repb.ContentAddressableStorage_GetTreeServer) error {
	// TODO(iain): cache these
	remoteStream, err := s.remote.GetTree(stream.Context(), req)
	if err != nil {
		return err
	}
	for {
		rsp, err := remoteStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err = stream.Send(rsp); err != nil {
			return err
		}
	}
	return nil
}
