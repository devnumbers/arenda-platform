package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseEmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		// WantEmail is the expected normalized address; "" expects the zero
		// email accompanying a failed parse.
		wantEmail string
		wantOK    bool
		// WantStatus is the expected response status; 0 expects the response
		// writer to stay untouched.
		wantStatus int
		wantDetail string
	}{
		{
			name:      "valid email returns parsed value",
			raw:       testOwnerEmail,
			wantEmail: testOwnerEmail,
			wantOK:    true,
		},
		{
			name:      "surrounding whitespace is trimmed",
			raw:       "  owner@example.com  ",
			wantEmail: testOwnerEmail,
			wantOK:    true,
		},
		{
			name:      "mixed-case email is lowercased",
			raw:       "Owner@Example.COM",
			wantEmail: testOwnerEmail,
			wantOK:    true,
		},
		{
			name:       "empty value writes 400 and returns false",
			raw:        "",
			wantStatus: http.StatusBadRequest,
			wantDetail: "Некорректная почта",
		},
		{
			name:       "invalid email writes 400 and returns false",
			raw:        "not-an-email",
			wantStatus: http.StatusBadRequest,
			wantDetail: "Некорректная почта",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)

			got, ok := parseEmail(w, r, tc.raw)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			assertParsedEmail(t, got.String(), tc.wantEmail)
			assertEmailResponse(t, w, tc.wantStatus, tc.wantDetail)
		})
	}
}

// assertParsedEmail checks the email result of parseEmail: the zero value when
// wantEmail is empty, otherwise the normalized address itself.
func assertParsedEmail(t *testing.T, got, wantEmail string) {
	t.Helper()
	if wantEmail == "" {
		if got != "" {
			t.Fatalf("email = %q, want empty", got)
		}
		return
	}
	if got != wantEmail {
		t.Fatalf("email = %q, want %q", got, wantEmail)
	}
}

// assertEmailResponse checks the response writer after parseEmail: a zero
// wantStatus means the writer must be untouched, otherwise the problem
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
