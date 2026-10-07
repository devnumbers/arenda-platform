package objectstorage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/nambers/arenda-planform/apps/backend/internal/shared/storage"
)

// FakeStorage is the in-memory storage adapter for local development and
// tests (ADR 0005: no MinIO in docker compose): provider=fake, the objects
// live in the process memory and stream through the same backend endpoints
// as the S3 ones.
type FakeStorage struct {
	mu      sync.RWMutex
	objects map[string]fakeObject
}

type fakeObject struct {
	data        []byte
	contentType string
}

// NewFakeStorage creates a new in-memory storage adapter.
func NewFakeStorage() *FakeStorage {
	return &FakeStorage{objects: make(map[string]fakeObject)}
}

// Put stores the object in memory. The bytes are copied: the caller's
// reader may be a request-scoped buffer.
func (s *FakeStorage) Put(_ context.Context, key string, data io.Reader, contentType string, _ int64) error {
	b, err := io.ReadAll(data)
	if err != nil {
		return fmt.Errorf("read upload data: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = fakeObject{data: b, contentType: contentType}
	return nil
}

// Open returns the object's body, size and content type; a missing key is
// storage.ErrNotFound.
func (s *FakeStorage) Open(_ context.Context, key string) (body io.ReadCloser, size int64, contentType string, err error) {
	s.mu.RLock()
	obj, ok := s.objects[key]
	s.mu.RUnlock()
	if !ok {
		return nil, 0, "", storage.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(obj.data)), int64(len(obj.data)), obj.contentType, nil
}

// Delete removes the object; deleting an absent key succeeds.
func (s *FakeStorage) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}

// HeadBucket is a no-op for the in-memory adapter.
func (s *FakeStorage) HeadBucket(_ context.Context) error {
	return nil
}

// Object returns a stored object's bytes for test assertions.
func (s *FakeStorage) Object(key string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	obj, ok := s.objects[key]
	if !ok {
		return nil, false
	}
	return obj.data, true
}

// compile-time conformance of both adapters with the shared port.
var (
	_ storage.PhotoStorage = (*S3Storage)(nil)
	_ storage.PhotoStorage = (*FakeStorage)(nil)
)
