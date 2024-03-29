package mockstore

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/buildbuddy-io/buildbuddy/server/interfaces"
	"github.com/buildbuddy-io/buildbuddy/server/util/status"
)

type Context struct{}

func (m *Context) Deadline() (time.Time, bool) {
	return time.Unix(0, 0), false
}

func (m *Context) Done() <-chan struct{} {
	return nil
}

func (m *Context) Err() error {
	return nil
}

func (m *Context) Value(key interface{}) interface{} {
	return nil
}

type Mockstore struct {
	mu      sync.Mutex
	BlobMap map[string][]byte
}

func New() *Mockstore {
	return &Mockstore{BlobMap: make(map[string][]byte)}
}

func (m *Mockstore) BlobExists(_ context.Context, blobName string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.BlobMap[blobName]
	return ok, nil
}
func (m *Mockstore) ReadBlob(_ context.Context, blobName string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if value, ok := m.BlobMap[blobName]; !ok {
		return nil, fmt.Errorf("%s not present in mockstore map: %w", blobName, os.ErrNotExist)
	} else {
		return value, nil
	}
}

func (m *Mockstore) Set(blobName string, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.BlobMap[blobName] = data
}

func (m *Mockstore) GetBlobMap() map[string][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := make(map[string][]byte, len(m.BlobMap))
	for k, v := range m.BlobMap {
		r[k] = v
	}
	return r
}

func (m *Mockstore) WriteBlob(_ context.Context, blobName string, data []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.BlobMap[blobName] = make([]byte, len(data))
	return copy(m.BlobMap[blobName], data), nil
}

func (m *Mockstore) DeleteBlob(_ context.Context, blobName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.BlobMap, blobName)
	return nil
}

func (m *Mockstore) Writer(_ context.Context, blobName string) (interfaces.CommittedWriteCloser, error) {
	return &WriteCloser{&bytes.Buffer{}, m, blobName}, nil
}

type WriteCloser struct {
	buf      *bytes.Buffer
	m        *Mockstore
	blobName string
}

func (w *WriteCloser) Write(p []byte) (int, error) {
	n, err := w.buf.Write(p)
	return n, err
}

func (w *WriteCloser) Commit() error {
	if w.buf == nil {
		return status.FailedPreconditionError("Writer was already closed.")
	}
	w.m.mu.Lock()
	defer w.m.mu.Unlock()
	w.m.BlobMap[w.blobName] = w.buf.Bytes()
	return nil
}

func (w *WriteCloser) Close() error {
	w.buf = nil
	return nil
}
