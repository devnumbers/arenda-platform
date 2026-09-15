package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
)

// The stand-only time-travel endpoints (issue #665): the time-shift forwards
// the raw shift or the preset with the acting admin, maps the application
// sentinels to problems, and the tick endpoint triggers one worker pass.

// fakeTimeShiftBackend records the forwarded time-shift request.
type fakeTimeShiftBackend struct {
	called  bool
	adminID uuid.UUID
	userID  uuid.UUID
	req     billingapp.TimeShiftRequest
	err     error
}

func (f *fakeTimeShiftBackend) ShiftSubscriptionTime(
	_ context.Context, adminID, userID uuid.UUID, req billingapp.TimeShiftRequest,
) error {
	f.called = true
	f.adminID, f.userID, f.req = adminID, userID, req
	return f.err
}

// fakeTickRunner records the tick trigger.
type fakeTickRunner struct {
	called bool
	err    error
}

func (f *fakeTickRunner) TickOnce(context.Context) error {
	f.called = true
	return f.err
}

// newTimeTravelRouter mounts the rig's routes the way the composition root
// does, over the given fakes.
func newTimeTravelRouter(backend TimeShiftBackend, tick BillingTickRunner) http.Handler {
	router := chi.NewRouter()
	NewTimeTravelHandlers(backend, tick, nil).MountRoutes(router)
	return router
}

// timeTravelRequest builds an admin-authenticated POST request with a JSON body.
func timeTravelRequest(t *testing.T, target string, adminID uuid.UUID, body string) *http.Request {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequestWithContext(
		httpsupport.WithActor(t.Context(), adminID, actor.RoleAdmin),
		http.MethodPost, target, reader,
	)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// timeTravelGetRequest builds an admin-authenticated GET request.
func timeTravelGetRequest(t *testing.T, target string, adminID uuid.UUID) *http.Request {
	t.Helper()
	return httptest.NewRequestWithContext(
		httpsupport.WithActor(t.Context(), adminID, actor.RoleAdmin),
		http.MethodGet, target, http.NoBody,
	)
}

func TestTimeShift_ForwardsRawShiftAndAdmin(t *testing.T) {
	t.Parallel()
	adminID := uuid.Must(uuid.NewV7())
	userID := uuid.Must(uuid.NewV7())
	backend := &fakeTimeShiftBackend{}

	w := httptest.NewRecorder()
	newTimeTravelRouter(backend, &fakeTickRunner{}).ServeHTTP(w,
		timeTravelRequest(t, "/admin/users/"+userID.String()+"/subscription/time-shift", adminID, `{"shiftHours":-25}`))

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", w.Code, w.Body.String())
	}
	if !backend.called {
		t.Fatal("service was not called")
	}
	if backend.adminID != adminID || backend.userID != userID {
		t.Errorf("ids = %v/%v, want admin %v user %v", backend.adminID, backend.userID, adminID, userID)
	}
	if backend.req.Shift != -25*time.Hour || backend.req.Preset != "" {
		t.Errorf("request = %+v, want shift −25h without a preset", backend.req)
	}
}

func TestTimeShift_ForwardsPreset(t *testing.T) {
	t.Parallel()
	adminID := uuid.Must(uuid.NewV7())
	userID := uuid.Must(uuid.NewV7())
	backend := &fakeTimeShiftBackend{}

	w := httptest.NewRecorder()
	newTimeTravelRouter(backend, &fakeTickRunner{}).ServeHTTP(w,
		timeTravelRequest(t, "/admin/users/"+userID.String()+"/subscription/time-shift", adminID,
			`{"preset":"retry_24h_due"}`))

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", w.Code, w.Body.String())
	}
	if backend.req.Preset != billingapp.TimeShiftPresetRetryFirstDue || backend.req.Shift != 0 {
		t.Errorf("request = %+v, want the retry preset without a raw shift", backend.req)
	}
}

func TestTimeShift_ErrorMapping(t *testing.T) {
	t.Parallel()
	userID := uuid.Must(uuid.NewV7())
	adminID := uuid.Must(uuid.NewV7())

	cases := []struct {
		name       string
		body       string
		serviceErr error
		want       int
	}{
		{
			name:       "invalid shift answers 400",
			body:       `{"shiftHours":0}`,
			serviceErr: fmt.Errorf("wrapped: %w", domain.ErrInvalidTimeShift),
			want:       http.StatusBadRequest,
		},
		{
			name:       "state guard answers 409",
			body:       `{"preset":"period_expired"}`,
			serviceErr: fmt.Errorf("already past: %w", domain.ErrInvalidSubscriptionState),
			want:       http.StatusConflict,
		},
		{
			name:       "unknown user answers 404",
			body:       `{"shiftHours":-1}`,
			serviceErr: billingapp.ErrSubscriptionNotFound,
			want:       http.StatusNotFound,
		},
		{
			name:       "rig-less service answers 403",
			body:       `{"shiftHours":-1}`,
			serviceErr: billingapp.ErrTimeTravelDisabled,
			want:       http.StatusForbidden,
		},
	}
	for _, tc := range cases {
		backend := &fakeTimeShiftBackend{err: tc.serviceErr}
		w := httptest.NewRecorder()
		newTimeTravelRouter(backend, &fakeTickRunner{}).ServeHTTP(w,
			timeTravelRequest(t, "/admin/users/"+userID.String()+"/subscription/time-shift", adminID, tc.body))
		if w.Code != tc.want {
			t.Errorf("%s: status = %d, want %d; body: %s", tc.name, w.Code, tc.want, w.Body.String())
		}
	}

	// A malformed body is a 400 before the service is reached.
	backend := &fakeTimeShiftBackend{}
	w := httptest.NewRecorder()
	newTimeTravelRouter(backend, &fakeTickRunner{}).ServeHTTP(w,
		timeTravelRequest(t, "/admin/users/"+userID.String()+"/subscription/time-shift", adminID, `{oops`))
	if w.Code != http.StatusBadRequest {
		t.Errorf("malformed body: status = %d, want 400", w.Code)
	}
	if backend.called {
		t.Error("service was called with a malformed body")
	}

	// An absurd wire shift is refused at the edge before the duration
	// multiplication could overflow past the application cap.
	w = httptest.NewRecorder()
	newTimeTravelRouter(backend, &fakeTickRunner{}).ServeHTTP(w,
		timeTravelRequest(t, "/admin/users/"+userID.String()+"/subscription/time-shift", adminID, `{"shiftHours":9223372036854775807}`))
	if w.Code != http.StatusBadRequest {
		t.Errorf("overflowing shift: status = %d, want 400", w.Code)
	}
	if backend.called {
		t.Error("service was called with an overflowing shift")
	}
}

// TestRunTick_TriggersOnePass proves POST /admin/billing/tick runs one
// worker pass; a failing tick answers a 500 problem.
func TestRunTick_TriggersOnePass(t *testing.T) {
	t.Parallel()
	adminID := uuid.Must(uuid.NewV7())
	tick := &fakeTickRunner{}

	w := httptest.NewRecorder()
	newTimeTravelRouter(&fakeTimeShiftBackend{}, tick).ServeHTTP(w,
		timeTravelRequest(t, "/admin/billing/tick", adminID, ""))

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", w.Code, w.Body.String())
	}
	if !tick.called {
		t.Error("the tick was not triggered")
	}

	// A failing tick answers a 500 problem, not a silent 204.
	failing := &fakeTickRunner{err: errors.New("advisory lock unavailable")}
	w = httptest.NewRecorder()
	newTimeTravelRouter(&fakeTimeShiftBackend{}, failing).ServeHTTP(w,
		timeTravelRequest(t, "/admin/billing/tick", adminID, ""))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("failing tick: status = %d, want 500", w.Code)
	}
	if !failing.called {
		t.Error("the failing tick was not triggered")
	}
}

// TestTimeTravelStatus_ProbesEnabledRig proves GET /admin/billing/time-travel
// answers 200 with the enabled marker — the discovery call the admin panel
// (issue #666) uses to show itself only where the rig exists. In a rig-less
// build the route is not mounted at all (404), so the panel stays hidden.
func TestTimeTravelStatus_ProbesEnabledRig(t *testing.T) {
	t.Parallel()
	adminID := uuid.Must(uuid.NewV7())

	w := httptest.NewRecorder()
	newTimeTravelRouter(&fakeTimeShiftBackend{}, &fakeTickRunner{}).ServeHTTP(w,
		timeTravelGetRequest(t, "/admin/billing/time-travel", adminID))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if got := strings.TrimSpace(w.Body.String()); got != `{"enabled":true}` {
		t.Errorf(`body = %q, want {"enabled":true}`, got)
	}
}

func TestTimeTravelRoutes_RequireAdminRole(t *testing.T) {
	t.Parallel()
	userID := uuid.Must(uuid.NewV7())
	adminID := uuid.Must(uuid.NewV7())

	// Every rig route sits behind the mounted middleware: a non-admin session
	// is refused before the handlers run, an anonymous one too.
	routes := []string{
		"/admin/billing/time-travel",
		"/admin/billing/tick",
		"/admin/users/" + userID.String() + "/subscription/time-shift",
	}
	for _, route := range routes {
		method := http.MethodPost
		if route == TimeTravelStatusRoute {
			method = http.MethodGet
		}
		w := httptest.NewRecorder()
		newTimeTravelRouter(&fakeTimeShiftBackend{}, &fakeTickRunner{}).ServeHTTP(w,
			httptest.NewRequestWithContext(
				httpsupport.WithActor(t.Context(), adminID, actor.RoleOwner),
				method, route, http.NoBody,
			))
		if w.Code != http.StatusForbidden {
			t.Errorf("owner on %s: status = %d, want 403", route, w.Code)
		}

		w = httptest.NewRecorder()
		newTimeTravelRouter(&fakeTimeShiftBackend{}, &fakeTickRunner{}).ServeHTTP(w,
			httptest.NewRequestWithContext(t.Context(), method, route, http.NoBody))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("anonymous on %s: status = %d, want 401", route, w.Code)
		}
	}
}
