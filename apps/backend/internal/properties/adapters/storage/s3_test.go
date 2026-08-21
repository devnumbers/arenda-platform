package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestS3Storage_Upload_RequestShape(t *testing.T) {
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

	storage, err := NewS3Storage(t.Context(), srv.URL, "us-east-1", "test-bucket", "access", "secret", "https://cdn.example.com", true)
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}

	url, err := storage.Upload(context.Background(), "properties/123/image.jpg", "image/jpeg", int64(len("data")), strings.NewReader("data"))
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}
	if readErr != nil {
		t.Fatalf("read uploaded body: %v", readErr)
	}

	if gotMethod != http.MethodPut {
		t.Errorf("method = %q, want PUT", gotMethod)
	}
	wantPath := "/test-bucket/properties/123/image.jpg"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}
	if gotContentType != "image/jpeg" {
		t.Errorf("content-type = %q, want image/jpeg", gotContentType)
	}
	if gotACL != "public-read" {
		t.Errorf("x-amz-acl = %q, want public-read", gotACL)
	}
	if string(gotBody) != "data" {
		t.Errorf("body = %q, want data", gotBody)
	}
	wantURL := "https://cdn.example.com/properties/123/image.jpg"
	if url != wantURL {
		t.Errorf("url = %q, want %q", url, wantURL)
	}
}

func TestS3Storage_Upload_Error(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	storage, err := NewS3Storage(t.Context(), srv.URL, "us-east-1", "test-bucket", "access", "secret", "https://cdn.example.com", true)
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}

	_, err = storage.Upload(context.Background(), "properties/123/image.jpg", "image/jpeg", int64(len("data")), strings.NewReader("data"))
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestS3Storage_Upload_SetsContentLength(t *testing.T) {
	t.Parallel()

	var gotContentLength int64 = -1
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentLength = r.ContentLength
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	storage, err := NewS3Storage(t.Context(), srv.URL+"/", "us-east-1", "test-bucket", "access", "secret", "https://cdn.example.com", true)
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}

	wantSize := int64(42)
	_, err = storage.Upload(context.Background(), "properties/123/image.jpg",
		"image/jpeg", wantSize, strings.NewReader(strings.Repeat("x", int(wantSize))))
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	if gotContentLength != wantSize {
		t.Errorf("Content-Length = %d, want %d", gotContentLength, wantSize)
	}
	wantPath := "/test-bucket/properties/123/image.jpg"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}
}

func TestS3Storage_HeadBucket(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
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

		storage, err := NewS3Storage(t.Context(), srv.URL, "us-east-1", "test-bucket", "access", "secret", "https://cdn.example.com", true)
		if err != nil {
			t.Fatalf("create storage: %v", err)
		}

		if err := storage.HeadBucket(context.Background()); err != nil {
			t.Fatalf("HeadBucket failed: %v", err)
		}
	})

	t.Run("error on non-2xx", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		storage, err := NewS3Storage(t.Context(), srv.URL, "us-east-1", "test-bucket", "access", "secret", "https://cdn.example.com", true)
		if err != nil {
			t.Fatalf("create storage: %v", err)
		}

		if err := storage.HeadBucket(context.Background()); err == nil {
			t.Fatal("expected error for 404 response")
		}
	})
}
