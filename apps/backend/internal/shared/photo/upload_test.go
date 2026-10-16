package photo

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// buildMultipart assembles a multipart/form-data body: the optional "file"
// field with value plus any extra text fields. Returns the body and the
// content type header value to attach to the request.
func buildMultipart(t *testing.T, fieldName, value string, extraFields map[string]string) (body *bytes.Buffer, contentType string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if fieldName != "" {
		fw, err := w.CreateFormFile(fieldName, "photo.jpg")
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := fw.Write([]byte(value)); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	for k, v := range extraFields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return &buf, w.FormDataContentType()
}

func TestReadMultipartUpload(t *testing.T) {
	t.Parallel()

	t.Run("reads the file field", func(t *testing.T) {
		t.Parallel()
		body, ct := buildMultipart(t, "file", "jpeg-bytes", map[string]string{"name": "x"})
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/photo", body)
		req.Header.Set("Content-Type", ct)

		data, err := ReadMultipartUpload(req)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if string(data) != "jpeg-bytes" {
			t.Errorf("data = %q, want jpeg-bytes", data)
		}
	})

	t.Run("no file field", func(t *testing.T) {
		t.Parallel()
		body, ct := buildMultipart(t, "", "", map[string]string{"name": "x"})
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/photo", body)
		req.Header.Set("Content-Type", ct)

		if _, err := ReadMultipartUpload(req); !errors.Is(err, ErrUploadMissing) {
			t.Fatalf("err = %v, want ErrUploadMissing", err)
		}
	})

	t.Run("not a multipart body", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/photo", strings.NewReader("hello"))
		req.Header.Set("Content-Type", "application/json")

		if _, err := ReadMultipartUpload(req); !errors.Is(err, ErrUploadForm) {
			t.Fatalf("err = %v, want ErrUploadForm", err)
		}
	})

	t.Run("file over the size bound", func(t *testing.T) {
		t.Parallel()
		big := strings.Repeat("x", int(MaxSize)+1)
		body, ct := buildMultipart(t, "file", big, nil)
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/photo", body)
		req.Header.Set("Content-Type", ct)

		if _, err := ReadMultipartUpload(req); !errors.Is(err, ErrTooLarge) {
			t.Fatalf("err = %v, want ErrTooLarge", err)
		}
	})
}
