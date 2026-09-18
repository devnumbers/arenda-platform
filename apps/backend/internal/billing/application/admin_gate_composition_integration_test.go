//go:build integration

package application_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpserver"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
)

// The composition security test of issue #424 (spec #419): the billing admin
// gate holds on the composition root re-registering every /admin route over
// the generated OpenAPI router with AdminOnlyMiddleware (chi matches the last
// registration). No handler re-checks the role, so a routing refactor that
// silently drops a re-registration would expose the operation to regular
// owners. These tests drive the production router built by httpserver.New —
// the full middleware chain (session → actor role → admin gate → handler) —
// and pin both directions: an owner session is refused on every billing
// admin operation, and an admin session still passes through.

// Session cookie values the stub loader resolves; fixed values keep the
// routing under test deterministic.
const (
	gateOwnerSessionCookie = "gate-owner-session-cookie"
	gateAdminSessionCookie = "gate-admin-session-cookie"
)

// gateActor is the identity one stub session resolves to.
type gateActor struct {
	userID uuid.UUID
	role   actor.Role
}

// gateSessionLoader is the test stand-in for the identity-backed
// httpsupport.SessionLoader: it resolves the two fixed tokens to an owner and
// an admin session. The session middleware runs unmodified, so the actor role
// reaches the admin gate exactly as in production.
type gateSessionLoader struct{ actors map[string]gateActor }

func (l gateSessionLoader) Load(_ context.Context, token string, _ time.Time) (uuid.UUID, actor.Role, error) {
	a, ok := l.actors[token]
	if !ok {
		return uuid.Nil, "", httpsupport.SessionNotFound(nil)
	}
	return a.userID, a.role, nil
}

func (gateSessionLoader) Touch(context.Context, string, string, time.Time) (string, *time.Time, error) {
	return "", nil, nil
}

// adminGateHarness extends the refund harness (it already knows how to grow a
// succeeded payment) with the production HTTP router and the two sessions.
type adminGateHarness struct {
	*refundIntegrationHarness
	router http.Handler
	// The acting admin of the gate tests (gateAdminID) — a real admin-role
	// users row, unlike the refund harness's own adminID (an owner-role row
	// its service-level audit assertions run against).
	gateAdminID uuid.UUID
	// The succeeded payment every payment-addressed route targets; its payer
	// is also the owner session — the sharpest scenario is the payer aiming
	// the admin operations at their own money and subscription.
	payment domain.SubscriptionPayment
	// A subscription of another owner inside an open grace window, the
	// extend-grace target.
	graceSub domain.Subscription
}

func newAdminGateHarness(t *testing.T) *adminGateHarness {
	t.Helper()
	base := newRefundIntegrationHarness(t)

	payment := base.succeededUpgradePayment(t)
	graceSub := base.seedGraceSubscription(t, 48*time.Hour)

	adminID := base.seedUserWithRole(actor.RoleAdmin)
	loader := gateSessionLoader{actors: map[string]gateActor{
		gateOwnerSessionCookie: {userID: payment.UserID, role: actor.RoleOwner},
		gateAdminSessionCookie: {userID: adminID, role: actor.RoleAdmin},
	}}

	// The production composition root over the harness services — the same
	// billing Deps fields the API wiring fills. Everything outside billing
	// stays nil: those routes are never hit, and the constructors only store
	// their dependencies.
	router := httpserver.New(httpserver.Deps{
		Sessions:             nil,
		SessionLoader:        loader,
		Tariffs:              base.services.Tariffs,
		AdminTariffs:         base.services.Tariffs,
		Subscriptions:        base.services.Subscriptions,
		SubscriptionManagers: base.services.Subscriptions,
		Payments:             base.services.Payments,
		PaymentMethods:       base.services.PaymentMethods,
		Webhooks:             base.services.Payments,
		AdminPayments:        base.services.Payments,
		AdminSubscriptions:   base.services.Subscriptions,
		Logger:               slog.New(slog.DiscardHandler),
		Clock:                base.clock,
	})

	return &adminGateHarness{
		refundIntegrationHarness: base,
		router:                   router,
		gateAdminID:              adminID,
		payment:                  payment,
		graceSub:                 graceSub,
	}
}

// doAs fires one request at the full router under the given session cookie.
func (h *adminGateHarness) doAs(t *testing.T, cookie, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload io.Reader = http.NoBody
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal %s body: %v", method, err)
		}
		payload = bytes.NewReader(encoded)
	}
	req := httptest.NewRequestWithContext(t.Context(), method, path, payload)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.AddCookie(&http.Cookie{Name: httpsupport.SessionCookieName(false), Value: cookie})
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	return w
}

// TestAdminGateComposition_OwnerForbiddenOnEveryBillingAdminOperation drives
// every billing admin route of the production router with a regular owner
// session. Each row carries real fixtures and a valid body, so if a refactor
// dropped the route's admin re-registration the operation would actually
// execute for the owner — the test demands exactly 403 instead.
func TestAdminGateComposition_OwnerForbiddenOnEveryBillingAdminOperation(t *testing.T) {
	t.Parallel()
	h := newAdminGateHarness(t)

	paymentPath := fmt.Sprintf("/admin/subscription/payments/%s", h.payment.ID)
	subscriptionPath := fmt.Sprintf("/admin/users/%s/subscription", h.payment.UserID)
	gracePath := fmt.Sprintf("/admin/users/%s/subscription/grace-extension", h.graceSub.UserID)

	cases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"list tariffs", http.MethodGet, "/admin/tariffs", nil},
		{"create tariff", http.MethodPost, "/admin/tariffs", openapi.AdminCreateTariffRequest{
			Name:                openapi.TariffName(domain.TariffBusiness),
			ActivePropertyLimit: -1,
			MonthlyPriceKopecks: 99000,
			YearlyPriceKopecks:  890000,
		}},
		{"update tariff", http.MethodPut, "/admin/tariffs/" + h.tariffIDByName(t, domain.TariffPro).String(), openapi.AdminUpdateTariffRequest{
			ActivePropertyLimit: 5,
			IsActive:            true,
			MonthlyPriceKopecks: 49000,
			YearlyPriceKopecks:  440000,
		}},
		{"list payments", http.MethodGet, "/admin/subscription/payments", nil},
		{"get payment", http.MethodGet, paymentPath, nil},
		{"refund payment", http.MethodPost, paymentPath + "/refund", nil},
		{"sync payment", http.MethodPost, paymentPath + "/sync", nil},
		{"assign service subscription", http.MethodPost, subscriptionPath + "/service", openapi.AdminAssignServiceSubscriptionRequest{
			TariffName: openapi.TariffName(domain.TariffBusiness),
			TermType:   openapi.AdminAssignServiceSubscriptionRequestTermType(billingapp.ServiceTermMonth),
		}},
		{"force change tariff", http.MethodPost, subscriptionPath + "/force-change", openapi.AdminForceChangeTariffRequest{
			TariffName: openapi.TariffName(domain.TariffPro),
			Period:     openapi.AdminSubscriptionPaymentPeriod(domain.PeriodYear),
		}},
		{"extend grace", http.MethodPost, gracePath, openapi.AdminExtendGraceRequest{
			Days: 3,
		}},
		{"cancel subscription", http.MethodPost, subscriptionPath + "/cancel", nil},
		{"list transitions", http.MethodGet, subscriptionPath + "/transitions", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := h.doAs(t, gateOwnerSessionCookie, tc.method, tc.path, tc.body)
			if w.Code != http.StatusForbidden {
				t.Fatalf("owner %s %s: status = %d, want 403; body: %s", tc.method, tc.path, w.Code, w.Body.String())
			}
		})
	}
}

// TestAdminGateComposition_AdminSessionPassesRefundAndForceChange is the
// positive control: through the same full routing, the admin session executes
// the two money-shaping operations named by the spec — the refund saga and
// the force tariff change — and the stored state proves the requests truly
// ran, attributed to the acting admin.
func TestAdminGateComposition_AdminSessionPassesRefundAndForceChange(t *testing.T) {
	t.Parallel()
	h := newAdminGateHarness(t)

	// The refund executes and is attributed to the acting admin.
	w := h.doAs(t, gateAdminSessionCookie, http.MethodPost,
		fmt.Sprintf("/admin/subscription/payments/%s/refund", h.payment.ID), nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("admin refund: status = %d, want 204; body: %s", w.Code, w.Body.String())
	}
	h.requireRefundedPaymentShape(t, h.payment)
	refundedSub, err := h.subscriptions.GetByUserID(h.ctx(), h.payment.UserID)
	if err != nil {
		t.Fatalf("GetByUserID(refunded): %v", err)
	}
	refundTransitions := h.transitionsOf(t, refundedSub.ID)
	if len(refundTransitions) == 0 ||
		refundTransitions[0].Reason != domain.TransitionReasonRefunded ||
		refundTransitions[0].Initiator != domain.InitiatorAdmin ||
		refundTransitions[0].InitiatorID == nil || *refundTransitions[0].InitiatorID != h.gateAdminID {
		t.Fatalf("refund transition = %+v, want the acting admin's refunded entry", refundTransitions)
	}

	// The force change executes without payment and applies the new tariff.
	sub := h.seedPaidSubscription(t, domain.TariffBusiness)
	w = h.doAs(t, gateAdminSessionCookie, http.MethodPost,
		fmt.Sprintf("/admin/users/%s/subscription/force-change", sub.UserID),
		openapi.AdminForceChangeTariffRequest{
			TariffName: openapi.TariffName(domain.TariffPro),
			Period:     openapi.AdminSubscriptionPaymentPeriod(domain.PeriodYear),
		})
	if w.Code != http.StatusNoContent {
		t.Fatalf("admin force change: status = %d, want 204; body: %s", w.Code, w.Body.String())
	}
	stored, err := h.subscriptions.GetByUserID(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID(force-changed): %v", err)
	}
	if stored.TariffID != h.tariffIDByName(t, domain.TariffPro) {
		t.Fatalf("tariff after force change = %v, want pro", stored.TariffID)
	}
	wantUntil := h.clock.Now().AddDate(1, 0, 0)
	if stored.ValidUntil == nil || !stored.ValidUntil.Equal(wantUntil) {
		t.Fatalf("valid until = %v, want %v", stored.ValidUntil, wantUntil)
	}
}
