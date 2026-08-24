//go:build integration

package application_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpserver"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
)

// The per-user binding-session limit composition test (ticket #427, spec
// #419): POST /subscription/payment-methods is abuse-restricted per user on
// top of the global IP limit. The test drives the production router
// (httpserver.New — the full session middleware chain) over the real
// repositories and the fake provider: five binding sessions inside the
// sliding hour are the allowed burst, the sixth answers 429 with Retry-After,
// and once the harness clock slides past the window binding works again.

// bindingLimitHarness extends the base harness with the production HTTP
// router and one authenticated owner session.
type bindingLimitHarness struct {
	*integrationHarness
	router  http.Handler
	ownerID uuid.UUID
}

func newBindingLimitHarness(t *testing.T) *bindingLimitHarness {
	t.Helper()
	base := newIntegrationHarness(t)
	ownerID := base.seedUser()
	loader := gateSessionLoader{actors: map[string]gateActor{
		gateOwnerSessionCookie: {userID: ownerID, role: actor.RoleOwner},
	}}
	router := httpserver.New(httpserver.Deps{
		Sessions:       loader,
		PaymentMethods: base.services.PaymentMethods,
		Logger:         slog.New(slog.DiscardHandler),
		Clock:          base.clock,
	})
	return &bindingLimitHarness{integrationHarness: base, router: router, ownerID: ownerID}
}

// addCard fires one POST /subscription/payment-methods as the owner.
func (h *bindingLimitHarness) addCard(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/subscription/payment-methods", strings.NewReader("{}"))
	req.AddCookie(&http.Cookie{Name: httpsupport.SessionCookieName(false), Value: gateOwnerSessionCookie})
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	return w
}

// TestBindingSessionLimit_FivePerSlidingHourOnHTTP pins the whole HTTP
// behaviour: the ordinary flows stay unrestricted, the excess session is a
// 429 with Retry-After, and the limit window expires.
func TestBindingSessionLimit_FivePerSlidingHourOnHTTP(t *testing.T) {
	t.Parallel()
	h := newBindingLimitHarness(t)

	// Five sessions inside the window are the allowed burst — far above the
	// ordinary one-or-two-bindings and retry-after-failure flows.
	for i := 1; i <= 5; i++ {
		w := h.addCard(t)
		if w.Code != http.StatusOK {
			t.Fatalf("session %d inside the window: status = %d, want 200; body: %s", i, w.Code, w.Body.String())
		}
	}

	// The sixth inside the window answers 429 with the cooldown header and a
	// user-facing problem detail.
	w := h.addCard(t)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("sixth session inside the window: status = %d, want 429; body: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Retry-After"); got == "" {
		t.Errorf("Retry-After header is empty, want the cooldown seconds")
	}
	if detail := w.Body.String(); !strings.Contains(detail, "привязки карты") {
		t.Errorf("problem detail = %q, want a user-facing note about the card binding limit", detail)
	}
	// The refused request persisted no session row.
	if n := h.countRows("SELECT COUNT(*) FROM card_binding_sessions WHERE user_id = $1", h.ownerID); n != 5 {
		t.Fatalf("stored sessions = %d, want 5 (the refused request persists nothing)", n)
	}

	// Once the window has slid past the five sessions, binding works again.
	h.clock.now = h.clock.now.Add(time.Hour + time.Minute)
	w = h.addCard(t)
	if w.Code != http.StatusOK {
		t.Fatalf("after the window slid past: status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}
