package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
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
