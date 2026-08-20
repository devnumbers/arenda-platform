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

	tests := []struct {
		name string
		raw  *string
		// WantEmail is the expected normalized address; "" expects a nil email.
		wantEmail string
		wantOK    bool
		// WantStatus is the expected response status; 0 expects the response
		// writer to stay untouched.
		wantStatus int
		wantDetail string
	}{
		// Absent-input cases: nil pointer and whitespace-only strings are both
		// treated as "no email" and must yield (nil, true) without touching the
		// response writer.
		{name: "nil pointer returns (nil, true) without writing response", raw: nil, wantOK: true},
		{name: "whitespace only returns (nil, true) without writing response", raw: new("   "), wantOK: true},
		{name: "empty string returns (nil, true) without writing response", raw: new(""), wantOK: true},
		{name: "tab and newline returns (nil, true) without writing response", raw: new("\t\n "), wantOK: true},
		{
			name:      "valid email returns parsed value",
			raw:       new(testOwnerEmail),
			wantEmail: testOwnerEmail,
			wantOK:    true,
		},
		{
			name:      "mixed-case email is lowercased",
			raw:       new("Owner@Example.COM"),
			wantEmail: testOwnerEmail,
			wantOK:    true,
		},
		{
			name:       "invalid email writes 400 and returns false",
			raw:        new("not-an-email"),
			wantStatus: http.StatusBadRequest,
			wantDetail: "Некорректная почта",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)

			got, ok := parseOptionalEmail(w, r, tc.raw)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			assertParsedEmail(t, got, tc.wantEmail)
			assertEmailResponse(t, w, tc.wantStatus, tc.wantDetail)
		})
	}
}

// assertParsedEmail checks the email result of parseOptionalEmail: nil when
// wantEmail is empty, otherwise the normalized address itself.
func assertParsedEmail(t *testing.T, got *domain.Email, wantEmail string) {
	t.Helper()
	if wantEmail == "" {
		if got != nil {
			t.Fatalf("email = %v, want nil", got)
		}
		return
	}
	if got == nil || got.String() != wantEmail {
		t.Fatalf("email = %v, want %q", got, wantEmail)
	}
}

// assertEmailResponse checks the response writer after parseOptionalEmail: a
// zero wantStatus means the writer must be untouched, otherwise the problem
// response must carry the given status and detail.
func assertEmailResponse(t *testing.T, w *httptest.ResponseRecorder, wantStatus int, wantDetail string) {
	t.Helper()
	if wantStatus == 0 {
		if w.Code != http.StatusOK || w.Body.Len() != 0 {
			t.Fatalf("response was written: code=%d body=%q, want untouched", w.Code, w.Body.String())
		}
		return
	}
	if w.Code != wantStatus {
		t.Fatalf("status = %d, want %d", w.Code, wantStatus)
	}

	var problem struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
		Status int    `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if problem.Detail != wantDetail {
		t.Fatalf("detail = %q, want %q", problem.Detail, wantDetail)
	}
}

// Compile-time guard that domain.Email remains the return type, so a future
// signature change does not silently break callers of parseOptionalEmail.
var _ domain.Email
