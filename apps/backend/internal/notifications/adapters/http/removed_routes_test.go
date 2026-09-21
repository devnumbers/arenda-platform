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

// removedReminderDetailPath is a removed reminder detail path with a fixed id;
// shared by the method cases below.
const removedReminderDetailPath = "/reminders/ffffffff-0000-0000-0000-000000000001"

// TestRemovedRoutes_NotFound pins the code contract of the route removals:
// the generated router no longer registers these paths, so any request to
// them — including old links and stale push deep links — falls through to the
// router's NotFound (404). The reminders removal is issue #438. The
// per-event-type notification preferences removal was the settings reset of
// the notifications map (решение #738, ADR 0058); the per-category contract
// of #743 re-registered the path, so it is no longer part of this list —
// the surviving-notifications control below pins its return.
func TestRemovedRoutes_NotFound(t *testing.T) {
	t.Parallel()

	r := chi.NewRouter()
	_ = openapi.HandlerWithOptions(struct{ openapi.Unimplemented }{}, openapi.ChiServerOptions{
		BaseRouter:       r,
		ErrorHandlerFunc: httpsupport.OpenAPIErrorHandler,
	})
	srv := httptest.NewServer(r)
	// Shutdown goes through t.Cleanup, not defer: the parallel subtests below
	// share this server and only start once this function's body has returned.
	t.Cleanup(srv.Close)

	propertyID := "11111111-0000-0000-0000-000000000001"
	removed := []struct{ method, path string }{
		{http.MethodGet, "/reminders"},
		{http.MethodGet, "/reminders/calendar"},
		{http.MethodGet, removedReminderDetailPath},
		{http.MethodPatch, removedReminderDetailPath},
		{http.MethodDelete, removedReminderDetailPath},
		{http.MethodGet, fmt.Sprintf("/leases/%s/reminders", propertyID)},
	}
	for _, tc := range removed {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			t.Parallel()
			req, err := http.NewRequestWithContext(t.Context(), tc.method, srv.URL+tc.path, nil)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			httpResp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatalf("do request: %v", err)
			}
			if err := httpResp.Body.Close(); err != nil {
				t.Errorf("close response body: %v", err)
			}
			if httpResp.StatusCode != http.StatusNotFound {
				t.Errorf("%s %s: status = %d, want 404", tc.method, tc.path, httpResp.StatusCode)
			}
		})
	}

	// Controls: the surviving notifications routes are still registered —
	// the Unimplemented stub answers 501, not the router's 404. The
	// re-registered per-category preferences route of #743 is among them.
	for _, path := range []string{"/push/vapid-public-key", "/notification-preferences"} {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+path, nil)
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatalf("do request: %v", err)
		}
		if err := resp.Body.Close(); err != nil {
			t.Errorf("close response body: %v", err)
		}
		if resp.StatusCode != http.StatusNotImplemented {
			t.Errorf("GET %s: status = %d, want 501 (route registered, stub answer)", path, resp.StatusCode)
		}
	}
}
