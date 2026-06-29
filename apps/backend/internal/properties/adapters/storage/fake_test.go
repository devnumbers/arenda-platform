package storage

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestFakeStorage_Upload(t *testing.T) {
	ctx := context.Background()
	s := NewFakeStorage("http://localhost:8080/uploads")

	url, err := s.Upload(ctx, "properties/123/image.jpg", "image/jpeg", int64(len("fake-image")), bytes.NewReader([]byte("fake-image")))
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	want := "http://localhost:8080/uploads/properties/123/image.jpg"
	if url != want {
		t.Errorf("url = %q, want %q", url, want)
	}

	got, ok := s.Object("properties/123/image.jpg")
	if !ok {
		t.Fatal("object not stored")
	}
	if !bytes.Equal(got, []byte("fake-image")) {
		t.Errorf("stored bytes = %q, want %q", got, "fake-image")
	}
}

func TestFakeStorage_Upload_Overwrites(t *testing.T) {
	ctx := context.Background()
	s := NewFakeStorage("http://localhost:8080/uploads")

	_, err := s.Upload(ctx, "properties/123/image.jpg", "image/jpeg", int64(len("first")), bytes.NewReader([]byte("first")))
	if err != nil {
		t.Fatalf("first upload failed: %v", err)
	}
	_, err = s.Upload(ctx, "properties/123/image.jpg", "image/jpeg", int64(len("second")), bytes.NewReader([]byte("second")))
	if err != nil {
		t.Fatalf("second upload failed: %v", err)
	}

	got, _ := s.Object("properties/123/image.jpg")
	if !bytes.Equal(got, []byte("second")) {
		t.Errorf("stored bytes = %q, want %q", got, "second")
	}
}

func TestFakeStorage_Upload_LargeFile(t *testing.T) {
	ctx := context.Background()
	s := NewFakeStorage("http://localhost:8080/uploads")

	data := bytes.Repeat([]byte("x"), 1024*1024) // 1 MiB
	url, err := s.Upload(ctx, "properties/123/large.jpg", "image/jpeg", int64(len(data)), bytes.NewReader(data))
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}
	if !strings.HasPrefix(url, "http://localhost:8080/uploads/") {
		t.Errorf("unexpected url prefix: %q", url)
	}

	got, ok := s.Object("properties/123/large.jpg")
	if !ok {
		t.Fatal("object not stored")
	}
	if len(got) != len(data) {
		t.Errorf("stored bytes length = %d, want %d", len(got), len(data))
	}
}

func TestFakeStorage_HeadBucket(t *testing.T) {
	ctx := context.Background()
	s := NewFakeStorage("http://localhost:8080/uploads")

	if err := s.HeadBucket(ctx); err != nil {
		t.Fatalf("HeadBucket failed: %v", err)
	}
}
