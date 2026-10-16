package objectstorage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/shared/storage"
)

func newTestS3(t *testing.T, endpoint string) *S3Storage {
	t.Helper()
	s, err := NewS3Storage(t.Context(), endpoint, "us-east-1", "test-bucket", "access", "secret")
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}
	return s
}

func TestS3Storage_Put_RequestShape(t *testing.T) {
	t.Parallel()

	var gotMethod, gotPath, gotContentType, gotACL string
	var gotBody []byte
	var readErr error
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		gotACL = r.Header.Get("X-Amz-Acl")
		gotBody, readErr = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newTestS3(t, srv.URL)
	err := s.Put(context.Background(), "photos/01987654-3210-7abc-9def-0123456789ab.jpg",
		strings.NewReader("data"), "image/jpeg", int64(len("data")))
	if err != nil {
		t.Fatalf("put failed: %v", err)
	}
	if readErr != nil {
		t.Fatalf("read uploaded body: %v", readErr)
	}

	if gotMethod != http.MethodPut {
		t.Errorf("method = %q, want PUT", gotMethod)
	}
	// Path-style addressing (Рег.ру, research #1218): bucket in the path.
	wantPath := "/test-bucket/photos/01987654-3210-7abc-9def-0123456789ab.jpg"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}
	if gotContentType != "image/jpeg" {
		t.Errorf("content-type = %q, want image/jpeg", gotContentType)
	}
	// ADR 0065: canned-ACL снят — бакет приватный, выдача только через бэкенд.
	if gotACL != "" {
		t.Errorf("x-amz-acl = %q, want no ACL header", gotACL)
	}
	if string(gotBody) != "data" {
		t.Errorf("body = %q, want data", gotBody)
	}
}

func TestS3Storage_Put_Error(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := newTestS3(t, srv.URL)
	err := s.Put(context.Background(), "photos/key.jpg", strings.NewReader("data"), "image/jpeg", 4)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestS3Storage_Open_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		wantPath := "/test-bucket/photos/key.jpg"
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", "4")
		if _, err := w.Write([]byte("data")); err != nil {
			t.Errorf("write body: %v", err)
		}
	}))
	defer srv.Close()

	s := newTestS3(t, srv.URL)
	rc, size, ct, err := s.Open(context.Background(), "photos/key.jpg")
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	defer func() {
		if closeErr := rc.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	}()
	body, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "data" {
		t.Errorf("body = %q, want data", body)
	}
	if size != 4 {
		t.Errorf("size = %d, want 4", size)
	}
	if ct != "image/png" {
		t.Errorf("content-type = %q, want image/png", ct)
	}
}

func TestS3Storage_Open_MissingKeyIsNotFound(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	s := newTestS3(t, srv.URL)
	rc, _, _, err := s.Open(context.Background(), "photos/gone.jpg")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("err = %v, want storage.ErrNotFound", err)
	}
	if rc != nil {
		t.Error("a not-found error must not carry a body")
	}
}

func TestS3Storage_Open_OtherErrorsPassThrough(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := newTestS3(t, srv.URL)
	rc, _, _, err := s.Open(context.Background(), "photos/key.jpg")
	if err == nil || errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("err = %v, want a non-404 error", err)
	}
	if rc != nil {
		t.Error("an error result must not carry a body")
	}
}

func TestS3Storage_Delete(t *testing.T) {
	t.Parallel()

	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	s := newTestS3(t, srv.URL)
	if err := s.Delete(context.Background(), "photos/key.jpg"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	wantPath := "/test-bucket/photos/key.jpg"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}
}

func TestS3Storage_HeadBucket_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("method = %q, want HEAD", r.Method)
		}
		wantPath := "/test-bucket"
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newTestS3(t, srv.URL)
	if err := s.HeadBucket(context.Background()); err != nil {
		t.Fatalf("HeadBucket failed: %v", err)
	}
}

func TestS3Storage_HeadBucket_Error(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	s := newTestS3(t, srv.URL)
	if err := s.HeadBucket(context.Background()); err == nil {
		t.Fatal("expected error for 404 response")
	}
}
