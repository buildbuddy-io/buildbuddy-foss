package mockgcs

import (
	"bytes"
	"context"
	"io"
	"sync"
	"time"

	"github.com/buildbuddy-io/buildbuddy/server/interfaces"
	"github.com/buildbuddy-io/buildbuddy/server/util/ioutil"
	"github.com/buildbuddy-io/buildbuddy/server/util/status"
	"github.com/jonboulle/clockwork"
)

type timestampedBlob struct {
	data       []byte
	customTime time.Time
}

func New(clock clockwork.Clock) *mockGCS {
	return &mockGCS{
		clock:     clock,
		ageInDays: 0,
		items:     make(map[string]*timestampedBlob),
		mu:        sync.Mutex{},
	}
}

// N.B. This implementation only mocks out the bits of GCS needed
// to implement the pebble.PebbleGCSStorage interface.
type mockGCS struct {
	clock     clockwork.Clock
	ageInDays int64
	items     map[string]*timestampedBlob
	mu        sync.Mutex
}

func (m *mockGCS) expired(blobName string) bool {
	if blob, ok := m.items[blobName]; ok {
		if m.ageInDays > 0 {
			if m.clock.Since(blob.customTime) > time.Duration(m.ageInDays)*24*time.Hour {
				return true
			}
		}
	}
	return false
}

func (m *mockGCS) SetBucketCustomTimeTTL(ctx context.Context, ageInDays int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ageInDays = ageInDays
	return nil
}

func (m *mockGCS) Reader(ctx context.Context, blobName string, offset, limit int64) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	blob, ok := m.items[blobName]
	if !ok {
		return nil, status.NotFoundError("mock gcs blob not found")
	}
	if m.expired(blobName) {
		return nil, status.InternalError("mock gcs blob expired")
	}
	data := blob.data[offset:]
	if limit > 0 && limit < int64(len(data)) {
		data = data[:limit]
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *mockGCS) ConditionalWriter(ctx context.Context, blobName string, overwriteExisting bool, customTime time.Time, estimatedSize int64) (interfaces.CommittedWriteCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.items[blobName]
	exists = exists && !m.expired(blobName)
	if exists && !overwriteExisting {
		cwc := ioutil.NewCustomCommitWriteCloser(ioutil.DiscardWriteCloser())
		cwc.CommitFn = func(int64) error {
			return status.AlreadyExistsError("mock gcs blob already exists")
		}
		return cwc, nil
	}
	var buf bytes.Buffer
	cwc := ioutil.NewCustomCommitWriteCloser(&buf)
	cwc.CommitFn = func(int64) error {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.items[blobName] = &timestampedBlob{
			data:       buf.Bytes(),
			customTime: customTime,
		}
		return nil
	}
	return cwc, nil
}

func (m *mockGCS) DeleteBlob(ctx context.Context, blobName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, blobName)
	return nil
}

func (m *mockGCS) UpdateCustomTime(ctx context.Context, blobName string, t time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	blob, ok := m.items[blobName]
	if !ok {
		return status.NotFoundError("mock gcs blob not found")
	}
	if m.expired(blobName) {
		return status.NotFoundError("mock gcs blob expired")
	}
	if t.Before(blob.customTime) {
		return status.FailedPreconditionError("custom time can only move forward")
	}
	blob.customTime = t
	return nil
}
