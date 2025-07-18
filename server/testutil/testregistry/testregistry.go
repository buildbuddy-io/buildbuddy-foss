package testregistry

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
	"github.com/buildbuddy-io/buildbuddy/server/testutil/testdigest"
	"github.com/buildbuddy-io/buildbuddy/server/testutil/testport"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/require"

	gcr "github.com/google/go-containerregistry/pkg/v1"
)

const headerDockerContentDigest = "Docker-Content-Digest"

var blobsPathRegexp = regexp.MustCompile("/v2/(.+?)/blobs/(.+)")

type Opts struct {
	// An interceptor applied to HTTP calls. Returns true if the request
	// should be processed post-interception, or false if not.
	HttpInterceptor func(w http.ResponseWriter, r *http.Request) bool
}

type Registry struct {
	host   string
	port   int
	server *http.Server
}

func Run(t *testing.T, opts Opts) *Registry {
	handler := registry.New()
	mux := http.NewServeMux()
	mux.Handle("/", handler)

	// Docker Hub returns a Docker-Content-Digest header for manifests,
	// but not for blobs (layers).
	// Make sure the test registry does not return a Docker-Content-Digest
	// header for blobs (layers), so that clients do not accidentally
	// depend on this header.
	f := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			rw := w
			if blobsPathRegexp.MatchString(r.URL.Path) {
				rw = &discardingResponseWriter{w}
			}
			mux.ServeHTTP(rw, r)
		})
	if opts.HttpInterceptor != nil {
		f = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := w
			if blobsPathRegexp.MatchString(r.URL.Path) {
				rw = &discardingResponseWriter{w}
			}
			if opts.HttpInterceptor(rw, r) {
				mux.ServeHTTP(rw, r)
			}
		})
	}

	server := &http.Server{Handler: f}
	registry := Registry{
		host:   "localhost",
		port:   testport.FindFree(t),
		server: server,
	}
	lis, err := net.Listen("tcp", registry.Address())
	require.NoError(t, err)
	go func() { _ = server.Serve(lis) }()
	return &registry
}

func (r *Registry) Address() string {
	return fmt.Sprintf("%s:%d", r.host, r.port)
}

func (r *Registry) ImageAddress(imageName string) string {
	return fmt.Sprintf("%s:%d/%s", r.host, r.port, imageName)
}

func (r *Registry) Push(t *testing.T, image gcr.Image, imageName string) string {
	fullImageName := r.ImageAddress(imageName)
	ref, err := name.ParseReference(fullImageName)
	require.NoError(t, err)
	err = remote.Write(ref, image)
	require.NoError(t, err)
	return fullImageName
}

func (r *Registry) PushIndex(t *testing.T, idx gcr.ImageIndex, imageName string) string {
	fullImageName := r.ImageAddress(imageName)
	ref, err := name.ParseReference(fullImageName)
	require.NoError(t, err)
	err = remote.WriteIndex(ref, idx)
	require.NoError(t, err)
	return fullImageName
}

func (r *Registry) PushRandomImage(t *testing.T) (string, gcr.Image) {
	files := map[string][]byte{}
	buffer := bytes.Buffer{}
	buffer.Grow(1024)
	for i := 0; i < 1024; i++ {
		_, err := buffer.WriteString("0")
		require.NoError(t, err)
	}
	for i := 0; i < 100000; i++ {
		files[fmt.Sprintf("/tmp/%d", i)] = buffer.Bytes()
	}
	image, err := crane.Image(files)
	require.NoError(t, err)
	return r.Push(t, image, "test"), image
}

func (r *Registry) PushNamedImage(t *testing.T, imageName string) (string, gcr.Image) {
	files := map[string][]byte{
		"/tmp/" + imageName: []byte(imageName),
	}
	image, err := crane.Image(files)
	require.NoError(t, err)
	return r.Push(t, image, imageName), image
}

func (r *Registry) PushNamedImageWithFiles(t *testing.T, imageName string, files map[string][]byte) (string, gcr.Image) {
	image, err := crane.Image(files)
	require.NoError(t, err)
	return r.Push(t, image, imageName), image
}

func (r *Registry) PushNamedImageWithMultipleLayers(t *testing.T, imageName string) (string, gcr.Image) {
	base := empty.Image
	layers := make([]gcr.Layer, 0, 9)
	for i := 0; i < 9; i++ {
		rn, buf := testdigest.RandomCASResourceBuf(t, 128)
		layer, err := crane.Layer(map[string][]byte{
			"/layer/" + rn.Digest.Hash: buf,
		})
		require.NoError(t, err)
		layers = append(layers, layer)
	}
	image, err := mutate.AppendLayers(base, layers...)
	require.NoError(t, err)
	return r.Push(t, image, imageName), image
}

func (r *Registry) Shutdown(ctx context.Context) error {
	if r.server != nil {
		return r.server.Shutdown(ctx)
	}
	return nil
}

// ImageFromRlocationpath returns an Image from an rlocationpath.
// The rlocationpath should be set via x_defs in the BUILD file, and the
// rlocationpath target should be an OCI image target (e.g. oci.pull)
func ImageFromRlocationpath(t *testing.T, rlocationpath string) gcr.Image {
	indexPath, err := runfiles.Rlocation(rlocationpath)
	require.NoError(t, err)
	idx, err := layout.ImageIndexFromPath(indexPath)
	require.NoError(t, err)
	m, err := idx.IndexManifest()
	require.NoError(t, err)
	require.Len(t, m.Manifests, 1)
	require.True(t, m.Manifests[0].MediaType.IsImage())
	img, err := idx.Image(m.Manifests[0].Digest)
	require.NoError(t, err)
	return img
}

// bytesLayer implements partial.UncompressedLayer from raw bytes.
type bytesLayer struct {
	content   []byte
	diffID    gcr.Hash
	mediaType types.MediaType
}

// NewBytesLayer returns an image layer representing the given bytes.
//
// testtar.EntryBytes may be useful for constructing tarball contents.
func NewBytesLayer(t *testing.T, b []byte) gcr.Layer {
	sha := sha256.Sum256(b)
	layer, err := partial.UncompressedToLayer(&bytesLayer{
		mediaType: types.OCILayer,
		diffID: gcr.Hash{
			Algorithm: "sha256",
			Hex:       hex.EncodeToString(sha[:]),
		},
		content: b,
	})
	require.NoError(t, err)
	return layer
}

// DiffID implements partial.UncompressedLayer
func (ul *bytesLayer) DiffID() (gcr.Hash, error) {
	return ul.diffID, nil
}

// Uncompressed implements partial.UncompressedLayer
func (ul *bytesLayer) Uncompressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewBuffer(ul.content)), nil
}

// MediaType returns the media type of the layer
func (ul *bytesLayer) MediaType() (types.MediaType, error) {
	return ul.mediaType, nil
}

type discardingResponseWriter struct {
	http.ResponseWriter
}

func (rw *discardingResponseWriter) Header() http.Header {
	return rw.ResponseWriter.Header()
}

func (rw *discardingResponseWriter) Write(p []byte) (int, error) {
	rw.ResponseWriter.Header().Del(headerDockerContentDigest)
	return rw.ResponseWriter.Write(p)
}

func (rw *discardingResponseWriter) WriteHeader(statusCode int) {
	rw.ResponseWriter.Header().Del(headerDockerContentDigest)
	rw.ResponseWriter.WriteHeader(statusCode)
}
