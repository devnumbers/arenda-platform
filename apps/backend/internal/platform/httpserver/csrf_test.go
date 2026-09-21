package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCrossOriginProtection_RejectsCrossSiteMutations(t *testing.T) {
	t.Parallel()
	handler := crossOriginProtection()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/properties", nil)
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, r)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for a cross-site POST", rr.Code)
	}
}

func TestCrossOriginProtection_AllowsSameSiteAndBypasses(t *testing.T) {
	t.Parallel()
	const secFetchSite = "Sec-Fetch-Site"
	const crossSite = "cross-site"

	tests := []struct {
		name    string
		method  string
		target  string
		headers map[string]string
		want    int
	}{
		{
			name:   "same-origin POST passes",
			method: http.MethodPost,
			target: "/properties",
			headers: map[string]string{
				secFetchSite:   "same-origin",
				"Content-Type": "application/json",
			},
			want: http.StatusOK,
		},
		{
			name:   "the T-Kassa webhook path is bypassed despite cross-site",
			method: http.MethodPost,
			target: "/webhooks/payment/tkassa",
			headers: map[string]string{
				secFetchSite:   crossSite,
				"Content-Type": "application/json",
			},
			want: http.StatusOK,
		},
		{
			name:    "the internal perf path is bypassed",
			method:  http.MethodPost,
			target:  "/internal/perf/db-pool",
			headers: map[string]string{"Sec-Fetch-Site": "cross-site"},
			want:    http.StatusOK,
		},
		{
			name:    "safe GET navigation passes even cross-site",
			method:  http.MethodGet,
			target:  "/subscription/payment-methods/add-card/success",
			headers: map[string]string{"Sec-Fetch-Site": "cross-site"},
			want:    http.StatusOK,
		},
		{
			// Non-browser callers send neither Sec-Fetch-Site nor Origin.
			name:   "requests without browser headers pass (server-to-server)",
			method: http.MethodPost,
			target: "/webhooks/payment/tkassa",
			want:   http.StatusOK,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			called := false
			h := crossOriginProtection()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))

			r := httptest.NewRequestWithContext(context.Background(), tc.method, tc.target, nil)
			for k, v := range tc.headers {
				r.Header.Set(k, v)
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, r)

			if !called || rr.Code != tc.want {
				t.Fatalf("status = %d called = %v, want %d called", rr.Code, called, tc.want)
			}
		})
	}
}
