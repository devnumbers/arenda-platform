package objectstorage

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/shared/storage"
)

// openFake reads an object through the port and closes the body, so the
// tests keep named results and a checked Close.
func openFake(t *testing.T, s *FakeStorage, key string) (data []byte, size int64, contentType string, err error) {
	t.Helper()
	rc, size, contentType, err := s.Open(context.Background(), key)
	if err != nil {
		return nil, 0, "", err
	}
	defer func() {
		if closeErr := rc.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	}()
	data, err = io.ReadAll(rc)
	if err != nil {
		return nil, 0, "", err
	}
	return data, size, contentType, nil
}

func TestFakeStorage_PutOpenRoundtrip(t *testing.T) {
	t.Parallel()

	s := NewFakeStorage()
	err := s.Put(t.Context(), "photos/a.jpg", strings.NewReader("jpeg-bytes"), "image/jpeg", int64(len("jpeg-bytes")))
	if err != nil {
		t.Fatalf("put: %v", err)
	}

	data, size, ct, err := openFake(t, s, "photos/a.jpg")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if string(data) != "jpeg-bytes" {
		t.Errorf("body = %q, want jpeg-bytes", data)
	}
	if size != int64(len("jpeg-bytes")) {
		t.Errorf("size = %d, want %d", size, len("jpeg-bytes"))
	}
	if ct != "image/jpeg" {
		t.Errorf("content-type = %q, want image/jpeg", ct)
	}
}

func TestFakeStorage_OpenMissing(t *testing.T) {
	t.Parallel()

	s := NewFakeStorage()
	data, _, _, err := openFake(t, s, "photos/gone.jpg")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("err = %v, want storage.ErrNotFound", err)
	}
	if data != nil {
		t.Error("a not-found read must not return data")
	}
}

func TestFakeStorage_PutOverwrites(t *testing.T) {
	t.Parallel()

	s := NewFakeStorage()
	if err := s.Put(t.Context(), "photos/a.png", strings.NewReader("first"), "image/png", 5); err != nil {
		t.Fatalf("put first: %v", err)
	}
	if err := s.Put(t.Context(), "photos/a.png", strings.NewReader("second-generation"), "image/png", 17); err != nil {
		t.Fatalf("put second: %v", err)
	}

	data, size, _, err := openFake(t, s, "photos/a.png")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if string(data) != "second-generation" || size != 17 {
		t.Errorf("body = %q size = %d, want second-generation/17", data, size)
	}
}

func TestFakeStorage_Delete(t *testing.T) {
	t.Parallel()

	t.Run("removes the object", func(t *testing.T) {
		t.Parallel()
		s := NewFakeStorage()
		if err := s.Put(t.Context(), "photos/a.jpg", strings.NewReader("x"), "image/jpeg", 1); err != nil {
			t.Fatalf("put: %v", err)
		}
		if err := s.Delete(t.Context(), "photos/a.jpg"); err != nil {
			t.Fatalf("delete: %v", err)
		}
		data, _, _, err := openFake(t, s, "photos/a.jpg")
		if !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("err = %v, want storage.ErrNotFound", err)
		}
		if data != nil {
			t.Error("a deleted object must not be readable")
		}
	})

	// Best-effort orphan cleanup must not fail on an already-gone object.
	t.Run("absent key succeeds", func(t *testing.T) {
		t.Parallel()
		s := NewFakeStorage()
		if err := s.Delete(t.Context(), "photos/gone.jpg"); err != nil {
			t.Fatalf("delete absent: %v", err)
		}
	})
}

func TestFakeStorage_Object(t *testing.T) {
	t.Parallel()

	s := NewFakeStorage()
	if err := s.Put(t.Context(), "photos/a.jpg", strings.NewReader("bytes"), "image/jpeg", 5); err != nil {
		t.Fatalf("put: %v", err)
	}
	obj, ok := s.Object("photos/a.jpg")
	if !ok || string(obj) != "bytes" {
		t.Errorf("object = %q %v, want bytes/true", obj, ok)
	}
	if _, ok := s.Object("photos/gone.jpg"); ok {
		t.Error("missing key reported as present")
	}
}
