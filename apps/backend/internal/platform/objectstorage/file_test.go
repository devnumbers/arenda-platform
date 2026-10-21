package objectstorage

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/shared/storage"
)

// testJPEGContentType pins the wire content type of the objectstorage tests;
// a shared const keeps goconst quiet across the package's three test files.
const testJPEGContentType = "image/jpeg"

// The file adapter serves local development (ADR 0065): the in-memory fake
// loses every object on a backend restart while the DB keys keep pointing
// into it — the dangling-key drift the self-heal then wipes. The file
// adapter keeps the same port semantics but survives the restart.

func TestFileStorage_PutOpenDelete_Roundtrip(t *testing.T) {
	t.Parallel()
	s := NewFileStorage(t.TempDir())

	err := s.Put(t.Context(), "photos/a.jpg", strings.NewReader("jpeg-bytes"), testJPEGContentType, 10)
	if err != nil {
		t.Fatalf("put: %v", err)
	}

	body, size, contentType, err := s.Open(t.Context(), "photos/a.jpg")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	}()

	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "jpeg-bytes" {
		t.Errorf("body = %q, want %q", data, "jpeg-bytes")
	}
	if size != 10 {
		t.Errorf("size = %d, want 10", size)
	}
	if contentType != testJPEGContentType {
		t.Errorf("contentType = %q, want %q", contentType, testJPEGContentType)
	}

	if err := s.Delete(t.Context(), "photos/a.jpg"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, _, _, err := s.Open(t.Context(), "photos/a.jpg"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("open after delete: err = %v, want storage.ErrNotFound", err)
	}
}

func TestFileStorage_Open_MissingKeyIsErrNotFound(t *testing.T) {
	t.Parallel()
	s := NewFileStorage(t.TempDir())

	if _, _, _, err := s.Open(t.Context(), "photos/absent.jpg"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("err = %v, want storage.ErrNotFound", err)
	}
}

// The restart scenario: a fresh adapter instance over the same directory
// must see the objects a previous process put there.
func TestFileStorage_ObjectsSurviveNewInstance(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	first := NewFileStorage(dir)
	if err := first.Put(t.Context(), "photos/b.png", strings.NewReader("png-bytes"), "image/png", 9); err != nil {
		t.Fatalf("put: %v", err)
	}

	second := NewFileStorage(dir)
	body, _, contentType, err := second.Open(t.Context(), "photos/b.png")
	if err != nil {
		t.Fatalf("open via new instance: %v", err)
	}
	defer func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	}()

	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "png-bytes" || contentType != "image/png" {
		t.Errorf("body = %q (%q), want png-bytes (image/png)", data, contentType)
	}
}

func TestFileStorage_Open_KeyEscapingDirRejected(t *testing.T) {
	t.Parallel()
	s := NewFileStorage(t.TempDir())

	if _, _, _, err := s.Open(t.Context(), "../escape.jpg"); err == nil {
		t.Error("open with a key escaping the root must fail")
	}
}

// An object file without the content-type header is a format the adapter
// never wrote (a foreign or torn file): the honest ErrNotFound — never a
// false 404 the serving self-heal would treat as a dangling key.
func TestFileStorage_Open_HeaderlessFileIsErrNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewFileStorage(dir)

	if err := os.MkdirAll(filepath.Join(dir, "photos"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "photos", "x.jpg"), []byte("headerless-bytes"), 0o600); err != nil {
		t.Fatalf("seed headerless file: %v", err)
	}
	if _, _, _, err := s.Open(t.Context(), "photos/x.jpg"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("err = %v, want storage.ErrNotFound", err)
	}
}

// compile-time conformance with the shared port.
var _ storage.PhotoStorage = (*FileStorage)(nil)
