package storage

import (
	"context"
	"fmt"
	"io"
	"sync"
)

// FakeStorage is an in-memory storage adapter for local development and tests.
type FakeStorage struct {
	mu            sync.RWMutex
	publicBaseURL string
	objects       map[string][]byte
}

// NewFakeStorage creates a new in-memory storage adapter.
func NewFakeStorage(publicBaseURL string) *FakeStorage {
	return &FakeStorage{
		publicBaseURL: publicBaseURL,
		objects:       make(map[string][]byte),
	}
}

// Upload stores the object in memory and returns a deterministic public URL.
func (s *FakeStorage) Upload(_ context.Context, key, _ string, _ int64, data io.Reader) (string, error) {
	b, err := io.ReadAll(data)
	if err != nil {
		return "", fmt.Errorf("read upload data: %w", err)
	}

	s.mu.Lock()
	s.objects[key] = b
	s.mu.Unlock()

	return fmt.Sprintf("%s/%s", s.publicBaseURL, key), nil
}

// Delete removes the object from the in-memory store.
func (s *FakeStorage) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}

// HeadBucket is a no-op for the in-memory storage adapter.
func (s *FakeStorage) HeadBucket(_ context.Context) error {
	return nil
}

// Object returns a stored object for test assertions.
func (s *FakeStorage) Object(key string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.objects[key]
	return b, ok
}
