package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
)

// TestHandlePropertyError_AccessOutcomes verifies the wire contract of the
// property page-entry privacy policy (issue #156 T3, T9): a suspended
// membership maps to a distinguishable 403 with the membership_suspended code,
// while the privacy-preserving 404 carries no code field at all.
func TestHandlePropertyError_AccessOutcomes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		err                error
		wantStatus         int
		wantTitle          string
		wantCode           string // empty means the "code" key must be absent from the JSON body
		wantDetailNonEmpty bool
	}{
		{
			name:               "suspended membership is a coded 403",
			err:                propertiesapp.ErrAccessSuspended,
			wantStatus:         http.StatusForbidden,
			wantTitle:          "Forbidden",
			wantCode:           "membership_suspended",
			wantDetailNonEmpty: true,
		},
		{
			name:               "not found stays an uncoded 404",
			err:                propertiesapp.ErrNotFound,
			wantStatus:         http.StatusNotFound,
			wantTitle:          "Not found",
			wantCode:           "",
			wantDetailNonEmpty: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := newPropertyHandlersForMapping(t)
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			rr := httptest.NewRecorder()

			h.handlePropertyError(rr, req, tt.err)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}
			if got := rr.Header().Get("Content-Type"); got != "application/problem+json" {
				t.Fatalf("Content-Type = %q, want application/problem+json", got)
			}

			var prob openapi.Problem
			if err := json.Unmarshal(rr.Body.Bytes(), &prob); err != nil {
				t.Fatalf("decode problem: %v\nbody: %s", err, rr.Body.String())
			}
			if prob.Title != tt.wantTitle {
				t.Errorf("Problem.Title = %q, want %q", prob.Title, tt.wantTitle)
			}
			if tt.wantDetailNonEmpty && (prob.Detail == nil || *prob.Detail == "") {
				t.Errorf("Problem.Detail = %v, want non-empty", prob.Detail)
			}

			// Assert on the raw JSON so a regression in the omitempty tag is
			// caught: the 404 body must not carry a "code" key at all.
			var wire map[string]json.RawMessage
			if err := json.Unmarshal(rr.Body.Bytes(), &wire); err != nil {
				t.Fatalf("decode wire body: %v", err)
			}
			raw, hasCode := wire["code"]
			if tt.wantCode == "" {
				if hasCode {
					t.Errorf("body carries \"code\" = %s, want the key absent", string(raw))
				}
				return
			}
			if !hasCode {
				t.Fatalf("body has no \"code\" key, want %q", tt.wantCode)
			}
			var code string
			if err := json.Unmarshal(raw, &code); err != nil {
				t.Fatalf("decode code: %v", err)
			}
			if code != tt.wantCode {
				t.Errorf("code = %q, want %q", code, tt.wantCode)
			}
		})
	}
}
