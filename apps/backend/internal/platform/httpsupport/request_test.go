package httpsupport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONBody_RejectsNonJSONContentType(t *testing.T) {
	t.Parallel()

	for _, ct := range []string{
		"application/x-www-form-urlencoded",
		"text/plain",
		"multipart/form-data; boundary=xyz",
		"application/jsonx",
	} {
		r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/x", strings.NewReader(`{"a":1}`))
		r.Header.Set("Content-Type", ct)
		var dst struct {
			A int `json:"a"`
		}
		err := DecodeJSONBody(httptest.NewRecorder(), r, &dst)
		if !IsContentTypeNotJSON(err) {
			t.Errorf("Content-Type %q: err = %v, want the Content-Type sentinel", ct, err)
		}
	}
}

func TestDecodeJSONBody_AcceptsJSONWithParametersAndEmptyBodies(t *testing.T) {
	t.Parallel()

	t.Run("plain application/json", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/x", strings.NewReader(`{"a":1}`))
		r.Header.Set("Content-Type", "application/json")
		var dst struct {
			A int `json:"a"`
		}
		if err := DecodeJSONBody(httptest.NewRecorder(), r, &dst); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if dst.A != 1 {
			t.Fatalf("a = %d, want 1", dst.A)
		}
	})

	t.Run("charset parameter tolerated", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/x", strings.NewReader(`{"a":2}`))
		r.Header.Set("Content-Type", "application/json; charset=utf-8")
		var dst struct {
			A int `json:"a"`
		}
		if err := DecodeJSONBody(httptest.NewRecorder(), r, &dst); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
	})

	t.Run("no header with an empty body is an EOF (optional-body endpoints)", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/x", http.NoBody)
		var dst struct{}
		if err := DecodeJSONBody(httptest.NewRecorder(), r, &dst); !errors.Is(err, io.EOF) {
			t.Fatalf("err = %v, want io.EOF", err)
		}
	})

	t.Run("no header with a body is rejected", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/x", strings.NewReader(`{"a":1}`))
		var dst struct{}
		if err := DecodeJSONBody(httptest.NewRecorder(), r, &dst); !IsContentTypeNotJSON(err) {
			t.Fatalf("err = %v, want the Content-Type sentinel", err)
		}
	})
}
