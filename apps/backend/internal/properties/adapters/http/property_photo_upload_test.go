package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
)

// Builds a POST request with a multipart/form-data body: an optional leading
// text field, then a "file" part carrying filename, content type and payload.
// The contentType argument is empty for a part without one.
func buildPhotoUploadRequest(t *testing.T, leadingTextField bool, filename, contentType string, payload []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if leadingTextField {
		if err := mw.WriteField("caption", "ignored"); err != nil {
			t.Fatalf("write text field: %v", err)
		}
	}
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, filename))
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	part, err := mw.CreatePart(header)
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}
	if _, err := part.Write(payload); err != nil {
		t.Fatalf("write file part: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/upload", &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	return r
}

func TestReadPhotoUpload(t *testing.T) {
	t.Parallel()

	t.Run("returns the file part", func(t *testing.T) {
		t.Parallel()
		r := buildPhotoUploadRequest(t, false, "cottage.jpg", "image/jpeg", []byte("jpeg-bytes"))

		filename, contentType, data, err := readPhotoUpload(r)
		if err != nil {
			t.Fatalf("readPhotoUpload: %v", err)
		}
		if filename != "cottage.jpg" {
			t.Errorf("filename = %q, want cottage.jpg", filename)
		}
		if contentType != "image/jpeg" {
			t.Errorf("contentType = %q, want image/jpeg", contentType)
		}
		if !bytes.Equal(data, []byte("jpeg-bytes")) {
			t.Errorf("data = %q, want jpeg-bytes", data)
		}
	})

	t.Run("skips a leading text field", func(t *testing.T) {
		t.Parallel()
		r := buildPhotoUploadRequest(t, true, "cottage.png", "image/png", []byte("png-bytes"))

		_, _, data, err := readPhotoUpload(r)
		if err != nil {
			t.Fatalf("readPhotoUpload: %v", err)
		}
		if !bytes.Equal(data, []byte("png-bytes")) {
			t.Errorf("data = %q, want png-bytes", data)
		}
	})

	t.Run("accepts a file of exactly the size limit", func(t *testing.T) {
		t.Parallel()
		r := buildPhotoUploadRequest(t, false, "edge.webp", "image/webp", bytes.Repeat([]byte{0xab}, propertiesapp.MaxPhotoSize))

		if _, _, _, err := readPhotoUpload(r); err != nil {
			t.Fatalf("readPhotoUpload at the limit: %v", err)
		}
	})

	t.Run("rejects an oversized file as invalid input", func(t *testing.T) {
		t.Parallel()
		r := buildPhotoUploadRequest(t, false, "huge.jpg", "image/jpeg", bytes.Repeat([]byte{0xcd}, propertiesapp.MaxPhotoSize+1))

		_, _, _, err := readPhotoUpload(r)
		if !errors.Is(err, propertiesapp.ErrInvalidInput) {
			t.Fatalf("error = %v, want propertiesapp.ErrInvalidInput", err)
		}
	})

	t.Run("missing file field", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		if err := mw.WriteField("caption", "no file here"); err != nil {
			t.Fatalf("write text field: %v", err)
		}
		if err := mw.Close(); err != nil {
			t.Fatalf("close multipart writer: %v", err)
		}
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/upload", &buf)
		r.Header.Set("Content-Type", mw.FormDataContentType())

		if _, _, _, err := readPhotoUpload(r); !errors.Is(err, errPhotoUploadMissing) {
			t.Fatalf("error = %v, want errPhotoUploadMissing", err)
		}
	})

	t.Run("non-multipart body", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/upload", strings.NewReader("plain"))
		r.Header.Set("Content-Type", "application/json")

		if _, _, _, err := readPhotoUpload(r); !errors.Is(err, errPhotoUploadForm) {
			t.Fatalf("error = %v, want errPhotoUploadForm", err)
		}
	})
}

// TestRejectPhotoUpload_WireContract pins the HTTP responses for each
// readPhotoUpload failure so the streaming rewrite keeps the previous
// ParseMultipartForm/FormFile contract.
func TestRejectPhotoUpload_WireContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantDetail string
	}{
		{
			name:       "oversized file maps like the service size check",
			err:        propertiesapp.NewPhotoTooLargeError(propertiesapp.MaxPhotoSize + 1),
			wantDetail: "Некорректные данные объекта",
		},
		{
			name:       "missing file field",
			err:        errPhotoUploadMissing,
			wantDetail: "Требуется файл",
		},
		{
			name:       "malformed form",
			err:        errPhotoUploadForm,
			wantDetail: "Некорректная форма загрузки файла",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := newPropertyHandlersForMapping(t)
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/upload", nil)
			rr := httptest.NewRecorder()

			h.rejectPhotoUpload(rr, req, tt.err)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
			}
			var prob openapi.Problem
			if err := json.Unmarshal(rr.Body.Bytes(), &prob); err != nil {
				t.Fatalf("decode problem: %v\nbody: %s", err, rr.Body.String())
			}
			if prob.Detail == nil || *prob.Detail != tt.wantDetail {
				t.Fatalf("Problem.Detail = %v, want %q", prob.Detail, tt.wantDetail)
			}
		})
	}
}
