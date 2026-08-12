package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

func TestParseOptionalEmail(t *testing.T) {
	t.Parallel()

	// Absent-input cases: nil pointer and whitespace-only strings are both
	// treated as "no email" and must yield (nil, true) without touching the
	// response writer.
	absentCases := []struct {
		name string
		raw  *string
	}{
		{"nil pointer", nil},
		{"whitespace only", new("   ")},
		{"empty string", new("")},
		{"tab and newline", new("\t\n ")},
	}
	for _, tc := range absentCases {
		t.Run(tc.name+" returns (nil, true) without writing response", func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)

			got, ok := parseOptionalEmail(w, r, tc.raw)
			if !ok {
				t.Fatalf("ok = false, want true")
			}
			if got != nil {
				t.Fatalf("email = %v, want nil", got)
			}
			if w.Code != http.StatusOK || w.Body.Len() != 0 {
				t.Fatalf("response was written: code=%d body=%q, want untouched", w.Code, w.Body.String())
			}
		})
	}

	t.Run("valid email returns parsed value", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
		raw := "owner@example.com"

		got, ok := parseOptionalEmail(w, r, &raw)
		if !ok {
			t.Fatalf("ok = false, want true")
		}
		if got == nil {
			t.Fatal("email = nil, want parsed value")
		}
		if got.String() != "owner@example.com" {
			t.Fatalf("email = %q, want %q", got.String(), "owner@example.com")
		}
		if w.Code != http.StatusOK || w.Body.Len() != 0 {
			t.Fatalf("response was written: code=%d body=%q, want untouched", w.Code, w.Body.String())
		}
	})

	t.Run("mixed-case email is lowercased", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
		raw := "Owner@Example.COM"

		got, ok := parseOptionalEmail(w, r, &raw)
		if !ok || got == nil {
			t.Fatalf("ok=%v got=%v", ok, got)
		}
		if got.String() != "owner@example.com" {
			t.Fatalf("email = %q, want lowercased", got.String())
		}
	})

	t.Run("invalid email writes 400 and returns false", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
		raw := "not-an-email"

		got, ok := parseOptionalEmail(w, r, &raw)
		if ok {
			t.Fatalf("ok = true, want false")
		}
		if got != nil {
			t.Fatalf("email = %v, want nil", got)
		}
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}

		var problem struct {
			Title  string `json:"title"`
			Detail string `json:"detail"`
			Status int    `json:"status"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if problem.Detail != "Некорректная почта" {
			t.Fatalf("detail = %q, want %q", problem.Detail, "Некорректная почта")
		}
	})
}

// Compile-time guard that domain.Email remains the return type, so a future
// signature change does not silently break callers of parseOptionalEmail.
var _ domain.Email
