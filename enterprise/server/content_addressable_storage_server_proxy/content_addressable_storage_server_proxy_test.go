package content_addressable_storage_server_proxy

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/buildbuddy-io/buildbuddy/enterprise/server/byte_stream_server_proxy"
	"github.com/buildbuddy-io/buildbuddy/server/remote_cache/byte_stream_server"
	"github.com/buildbuddy-io/buildbuddy/server/remote_cache/cachetools"
	"github.com/buildbuddy-io/buildbuddy/server/remote_cache/content_addressable_storage_server"
	"github.com/buildbuddy-io/buildbuddy/server/testutil/cas"
	"github.com/buildbuddy-io/buildbuddy/server/testutil/testenv"
	"github.com/buildbuddy-io/buildbuddy/server/util/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	repb "github.com/buildbuddy-io/buildbuddy/proto/remote_execution"
	bspb "google.golang.org/genproto/googleapis/bytestream"
)

const (
	fooDigest   = "2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"
	foofDigest  = "ecebed81d223de4ccfbcf9cee4e19e1872165b8a142c2d6ee6fb1d29617d0e8e"
	barDigest   = "fcde2b2edba56bf408601fb721fe9b5c338d10ee429ea04fae5511b68fbf8fb9"
	barrDigest  = "8fa319f9b487d6ae32862c952d708b192a999b2f96bda081e8a49a0c3fb99265"
	barrrDigest = "39938f9489bc9b0f9d7308be111b90a615942ebc4530f0bf5c98e6083af29ee8"
	bazDigest   = "baa5a0964d3320fbc0c6a922140453c8513ea24ab8fd0577034804a967248096"
	quxDigest   = "21f58d27f827d295ffcd860c65045685e3baf1ad4506caa0140113b316647534"
)

func requestCountingUnaryInterceptor(count *atomic.Int32) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		count.Add(1)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func requestCountingStreamInterceptor(count *atomic.Int32) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		count.Add(1)
		return streamer(ctx, desc, cc, method, opts...)
	}
}

func runRemoteCASS(ctx context.Context, env *testenv.TestEnv, t *testing.T) (*grpc.ClientConn, *atomic.Int32, *atomic.Int32) {
	casServer, err := content_addressable_storage_server.NewContentAddressableStorageServer(env)
	require.NoError(t, err)
	bsServer, err := byte_stream_server.NewByteStreamServer(env)
	require.NoError(t, err)
	grpcServer, runFunc := testenv.RegisterLocalGRPCServer(t, env)
	repb.RegisterContentAddressableStorageServer(grpcServer, casServer)
	bspb.RegisterByteStreamServer(grpcServer, bsServer)
	go runFunc()
	unaryRequestCounter := atomic.Int32{}
	streamRequestCounter := atomic.Int32{}
	conn, err := testenv.LocalGRPCConn(ctx, env,
		grpc.WithUnaryInterceptor(requestCountingUnaryInterceptor(&unaryRequestCounter)),
		grpc.WithStreamInterceptor(requestCountingStreamInterceptor(&streamRequestCounter)))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn, &unaryRequestCounter, &streamRequestCounter
}

func runLocalCASS(ctx context.Context, env *testenv.TestEnv, t *testing.T) (bspb.ByteStreamClient, repb.ContentAddressableStorageClient) {
	bs, err := byte_stream_server.NewByteStreamServer(env)
	require.NoError(t, err)
	cas, err := content_addressable_storage_server.NewContentAddressableStorageServer(env)
	require.NoError(t, err)
	grpcServer, runFunc := testenv.RegisterLocalInternalGRPCServer(t, env)
	repb.RegisterContentAddressableStorageServer(grpcServer, cas)
	bspb.RegisterByteStreamServer(grpcServer, bs)
	go runFunc()
	conn, err := testenv.LocalInternalGRPCConn(ctx, env)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return bspb.NewByteStreamClient(conn), repb.NewContentAddressableStorageClient(conn)
}

func runCASProxy(ctx context.Context, clientConn *grpc.ClientConn, env *testenv.TestEnv, t *testing.T) *grpc.ClientConn {
	env.SetByteStreamClient(bspb.NewByteStreamClient(clientConn))
	env.SetContentAddressableStorageClient(repb.NewContentAddressableStorageClient(clientConn))
	bs, cas := runLocalCASS(ctx, env, t)
	env.SetLocalByteStreamClient(bs)
	env.SetLocalCASClient(cas)
	casServer, err := New(env)
	require.NoError(t, err)
	bsServer, err := byte_stream_server_proxy.New(env)
	require.NoError(t, err)
	grpcServer, runFunc := testenv.RegisterLocalGRPCServer(t, env)
	repb.RegisterContentAddressableStorageServer(grpcServer, casServer)
	bspb.RegisterByteStreamServer(grpcServer, bsServer)
	go runFunc()
	conn, err := testenv.LocalGRPCConn(ctx, env, grpc.WithDefaultCallOptions())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
}

func digestProto(hash string, size int64) *repb.Digest {
	return &repb.Digest{Hash: hash, SizeBytes: size}
}

func findMissingBlobsRequest(digests []*repb.Digest) *repb.FindMissingBlobsRequest {
	return &repb.FindMissingBlobsRequest{
		BlobDigests:    digests,
		DigestFunction: repb.DigestFunction_SHA256,
	}
}

func readBlobsRequest(digests []*repb.Digest) *repb.BatchReadBlobsRequest {
	return &repb.BatchReadBlobsRequest{
		Digests:               digests,
		AcceptableCompressors: []repb.Compressor_Value{repb.Compressor_IDENTITY},
		DigestFunction:        repb.DigestFunction_SHA256,
	}
}

func updateBlobsRequest(blobs map[*repb.Digest]string) *repb.BatchUpdateBlobsRequest {
	request := repb.BatchUpdateBlobsRequest{DigestFunction: repb.DigestFunction_SHA256}
	for digest, data := range blobs {
		request.Requests = append(request.Requests, &repb.BatchUpdateBlobsRequest_Request{
			Digest: digest,
			Data:   []byte(data),
		})
	}
	return &request
}

func findMissing(ctx context.Context, client repb.ContentAddressableStorageClient, digests []*repb.Digest, missing []*repb.Digest, t *testing.T) {
	resp, err := client.FindMissingBlobs(ctx, findMissingBlobsRequest(digests))
	require.NoError(t, err)
	require.Equal(t, len(missing), len(resp.MissingBlobDigests))
	for i := range missing {
		require.Equal(t, missing[i].Hash, resp.MissingBlobDigests[i].Hash)
		require.Equal(t, missing[i].SizeBytes, resp.MissingBlobDigests[0].SizeBytes)
	}
}

func read(ctx context.Context, client repb.ContentAddressableStorageClient, digests []*repb.Digest, blobs map[string]string, t *testing.T) {
	resp, err := client.BatchReadBlobs(ctx, readBlobsRequest(digests))
	require.NoError(t, err)
	require.Equal(t, len(digests), len(resp.Responses))
	expectedCount := map[string]int{}
	for _, digest := range digests {
		if _, ok := expectedCount[digest.Hash]; ok {
			expectedCount[digest.Hash] = expectedCount[digest.Hash] + 1
		} else {
			expectedCount[digest.Hash] = 1
		}
	}
	actualCount := map[string]int{}
	for _, response := range resp.Responses {
		hash := response.Digest.Hash
		if _, ok := actualCount[hash]; ok {
			actualCount[hash] = actualCount[hash] + 1
		} else {
			actualCount[hash] = 1
		}
		if _, ok := blobs[hash]; ok {
			require.Equal(t, int32(codes.OK), response.Status.Code)
			require.Equal(t, blobs[hash], string(response.Data))
		} else {
			require.Equal(t, int32(codes.NotFound), response.Status.Code)
		}
	}
	require.Equal(t, expectedCount, actualCount)
}

func update(ctx context.Context, client repb.ContentAddressableStorageClient, blobs map[*repb.Digest]string, t *testing.T) {
	resp, err := client.BatchUpdateBlobs(ctx, updateBlobsRequest(blobs))
	require.NoError(t, err)
	require.Equal(t, len(blobs), len(resp.Responses))
	for i := 0; i < len(blobs); i++ {
		require.Equal(t, int32(codes.OK), resp.Responses[i].Status.Code)
	}
}

func TestFindMissingBlobs(t *testing.T) {
	ctx := context.Background()
	conn, requestCount, _ := runRemoteCASS(ctx, testenv.GetTestEnv(t), t)
	proxyConn := runCASProxy(ctx, conn, testenv.GetTestEnv(t), t)
	proxy := repb.NewContentAddressableStorageClient(proxyConn)

	fooDigestProto := digestProto(fooDigest, 3)
	barDigestProto := digestProto(barDigest, 3)

	for i := 1; i < 10; i++ {
		findMissing(ctx, proxy, []*repb.Digest{fooDigestProto}, []*repb.Digest{fooDigestProto}, t)
		require.Equal(t, int32(i), requestCount.Load())
	}

	update(ctx, proxy, map[*repb.Digest]string{barDigestProto: "bar"}, t)

	requestCount.Store(0)
	for i := 1; i < 10; i++ {
		findMissing(ctx, proxy, []*repb.Digest{barDigestProto}, []*repb.Digest{}, t)
		require.Equal(t, int32(0), requestCount.Load())
	}

	requestCount.Store(0)
	for i := 1; i < 10; i++ {
		findMissing(ctx, proxy, []*repb.Digest{fooDigestProto, barDigestProto}, []*repb.Digest{fooDigestProto}, t)
		require.Equal(t, int32(i), requestCount.Load())
	}
}

func TestReadUpdateBlobs(t *testing.T) {
	ctx := context.Background()
	conn, requestCount, _ := runRemoteCASS(ctx, testenv.GetTestEnv(t), t)
	casClient := repb.NewContentAddressableStorageClient(conn)
	proxyConn := runCASProxy(ctx, conn, testenv.GetTestEnv(t), t)
	proxy := repb.NewContentAddressableStorageClient(proxyConn)

	fooDigestProto := digestProto(fooDigest, 3)
	foofDigestProto := digestProto(foofDigest, 4)
	barDigestProto := digestProto(barDigest, 3)
	barrDigestProto := digestProto(barrDigest, 4)
	barrrDigestProto := digestProto(barrrDigest, 5)
	bazDigestProto := digestProto(bazDigest, 3)
	quxDigestProto := digestProto(quxDigest, 3)

	read(ctx, proxy, []*repb.Digest{fooDigestProto, foofDigestProto, barDigestProto}, map[string]string{}, t)
	require.Equal(t, int32(1), requestCount.Load())

	update(ctx, proxy, map[*repb.Digest]string{fooDigestProto: "foo"}, t)
	require.Equal(t, int32(2), requestCount.Load())

	read(ctx, casClient, []*repb.Digest{fooDigestProto}, map[string]string{fooDigest: "foo"}, t)
	requestCount.Store(0)
	read(ctx, proxy, []*repb.Digest{fooDigestProto}, map[string]string{fooDigest: "foo"}, t)
	require.Equal(t, int32(0), requestCount.Load())
	read(ctx, proxy, []*repb.Digest{fooDigestProto, fooDigestProto}, map[string]string{fooDigest: "foo"}, t)
	require.Equal(t, int32(0), requestCount.Load())

	read(ctx, proxy, []*repb.Digest{barrDigestProto, barrrDigestProto, bazDigestProto}, map[string]string{}, t)
	require.Equal(t, int32(1), requestCount.Load())
	update(ctx, casClient, map[*repb.Digest]string{bazDigestProto: "baz"}, t)
	require.Equal(t, int32(2), requestCount.Load())

	read(ctx, proxy, []*repb.Digest{barrDigestProto, barrrDigestProto, bazDigestProto}, map[string]string{bazDigest: "baz"}, t)
	require.Equal(t, int32(3), requestCount.Load())
	read(ctx, proxy, []*repb.Digest{fooDigestProto, bazDigestProto}, map[string]string{fooDigest: "foo", bazDigest: "baz"}, t)

	update(ctx, casClient, map[*repb.Digest]string{quxDigestProto: "qux"}, t)
	read(ctx, proxy, []*repb.Digest{quxDigestProto, quxDigestProto}, map[string]string{quxDigest: "qux"}, t)
}

func makeTree(ctx context.Context, client bspb.ByteStreamClient, t *testing.T) (*repb.Digest, []string) {
	child1 := uuid.New()
	digest1, files1 := cas.MakeTree(ctx, t, client, "", 2, 2)
	child2 := uuid.New()
	digest2, files2 := cas.MakeTree(ctx, t, client, "", 2, 2)

	// Upload a root directory containing both child directories.
	root := &repb.Directory{
		Directories: []*repb.DirectoryNode{
			&repb.DirectoryNode{
				Name:   child1,
				Digest: digest1,
			},
			&repb.DirectoryNode{
				Name:   child2,
				Digest: digest2,
			},
		},
	}
	rootDigest, err := cachetools.UploadProto(ctx, client, "", repb.DigestFunction_SHA256, root)
	require.NoError(t, err)
	children := append(files1, files2...)
	children = append(children, child1, child2)
	return rootDigest, children
}

func TestGetTree(t *testing.T) {
	ctx := context.Background()
	conn, _, requestCounter := runRemoteCASS(ctx, testenv.GetTestEnv(t), t)
	casClient := repb.NewContentAddressableStorageClient(conn)
	bsClient := bspb.NewByteStreamClient(conn)
	proxyConn := runCASProxy(ctx, conn, testenv.GetTestEnv(t), t)
	casProxy := repb.NewContentAddressableStorageClient(proxyConn)
	bsProxy := bspb.NewByteStreamClient(proxyConn)

	// Read some random digest that's not there
	digest := &repb.Digest{
		Hash:      strings.Repeat("a", 64),
		SizeBytes: 15,
	}

	_, err := casClient.GetTree(ctx, &repb.GetTreeRequest{RootDigest: digest})
	require.NoError(t, err)
	_, err = casProxy.GetTree(ctx, &repb.GetTreeRequest{RootDigest: digest})
	require.NoError(t, err)
	require.Equal(t, int32(1), requestCounter.Load())

	rootDigest, files := makeTree(ctx, bsClient, t)
	treeFiles := cas.ReadTree(ctx, t, casClient, "", rootDigest)
	require.ElementsMatch(t, files, treeFiles)
	requestCounter.Store(0)
	treeFiles = cas.ReadTree(ctx, t, casProxy, "", rootDigest)
	require.ElementsMatch(t, files, treeFiles)
	require.Equal(t, int32(1), requestCounter.Load())

	rootDigest, files = makeTree(ctx, bsProxy, t)
	treeFiles = cas.ReadTree(ctx, t, casClient, "", rootDigest)
	require.ElementsMatch(t, files, treeFiles)
	requestCounter.Store(0)
	treeFiles = cas.ReadTree(ctx, t, casProxy, "", rootDigest)
	require.ElementsMatch(t, files, treeFiles)
	// TODO(iain): change this to 0 once tree caching support is added
	require.Equal(t, int32(1), requestCounter.Load())
}
