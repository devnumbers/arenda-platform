package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

// TestRemovedReminderRoutes_NotFound pins the code contract of the
// free-reminders removal (issue #382): the generated router no longer
// registers the removed paths, so any request to them — including old links
// and stale push deep links — falls through to the router's NotFound (404).
// The surviving notifications routes stay registered: routed to the
// Unimplemented stub they answer 501, proving the route still exists.
func TestRemovedReminderRoutes_NotFound(t *testing.T) {
	r := chi.NewRouter()
	_ = openapi.HandlerWithOptions(struct{ openapi.Unimplemented }{}, openapi.ChiServerOptions{
		BaseRouter:       r,
		ErrorHandlerFunc: httpsupport.OpenAPIErrorHandler,
	})
	srv := httptest.NewServer(r)
	defer srv.Close()

	removed := []struct{ method, path string }{
		{http.MethodPost, fmt.Sprintf("/properties/%s/free-reminders", handlerPropertyID)},
		{http.MethodGet, fmt.Sprintf("/properties/%s/free-reminders", handlerPropertyID)},
		{http.MethodGet, fmt.Sprintf("/properties/%s/free-reminders/upcoming", handlerPropertyID)},
		{http.MethodGet, "/free-reminders"},
		{http.MethodGet, "/free-reminders/ffffffff-0000-0000-0000-000000000001"},
		{http.MethodPatch, "/free-reminders/ffffffff-0000-0000-0000-000000000001"},
		{http.MethodDelete, "/free-reminders/ffffffff-0000-0000-0000-000000000001"},
	}
	for _, tc := range removed {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), tc.method, srv.URL+tc.path, nil)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			httpResp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatalf("do request: %v", err)
			}
			defer func() { _ = httpResp.Body.Close() }()
			if httpResp.StatusCode != http.StatusNotFound {
				t.Errorf("%s %s: status = %d, want 404", tc.method, tc.path, httpResp.StatusCode)
			}
		})
	}

	// Control: a surviving notifications route is still registered — the
	// Unimplemented stub answers 501, not the router's 404.
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/reminders", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("GET /reminders: status = %d, want 501 (route registered, stub answer)", resp.StatusCode)
	}
}
