package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAddCardReturnHandler(t *testing.T) {
	cases := []struct {
		name         string
		success      bool
		wantStatus   int
		wantLocation string
	}{
		{
			name:         "success redirects with addCard=success",
			success:      true,
			wantStatus:   http.StatusFound,
			wantLocation: "https://example.com/profile/tariff/payment-methods?addCard=success",
		},
		{
			name:         "failure redirects with addCard=fail",
			success:      false,
			wantStatus:   http.StatusFound,
			wantLocation: "https://example.com/profile/tariff/payment-methods?addCard=fail",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/subscription/payment-methods/add-card/success", nil)
			rec := httptest.NewRecorder()

			addCardReturnHandler("https://example.com", tc.success)(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d", rec.Code, tc.wantStatus)
			}
			if got := rec.Header().Get("Location"); got != tc.wantLocation {
				t.Errorf("Location header: got %q, want %q", got, tc.wantLocation)
			}
		})
	}
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	healthHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type: got %q, want application/json", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control: got %q, want no-store", got)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"status":"ok"}` {
		t.Errorf("body: got %q, want health JSON", got)
	}
}
