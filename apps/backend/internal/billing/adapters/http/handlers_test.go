package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
)

// fakeTariffLister is a func-backed TariffLister (the consumer-side port of
// these handlers, ADR 0035).
type fakeTariffLister struct {
	list func(ctx context.Context) ([]domain.Tariff, error)
}

func (f *fakeTariffLister) ListTariffs(ctx context.Context) ([]domain.Tariff, error) {
	if f.list != nil {
		return f.list(ctx)
	}
	return nil, errors.New("unexpected ListTariffs call")
}

// fakeSubscriptionViewer is a func-backed SubscriptionViewer.
type fakeSubscriptionViewer struct {
	get func(ctx context.Context, userID uuid.UUID) (billingapp.SubscriptionView, error)
}

func (f *fakeSubscriptionViewer) GetSubscription(ctx context.Context, userID uuid.UUID) (billingapp.SubscriptionView, error) {
	if f.get != nil {
		return f.get(ctx, userID)
	}
	return billingapp.SubscriptionView{}, errors.New("unexpected GetSubscription call")
}

// newTestHandlers builds handlers over the given fakes; a nil fake is
// replaced by a stub that fails the test when called.
func newTestHandlers(tariffs TariffLister, subs SubscriptionViewer) *BillingHandlers {
	if tariffs == nil {
		tariffs = &fakeTariffLister{}
	}
	if subs == nil {
		subs = &fakeSubscriptionViewer{}
	}
	return NewBillingHandlers(tariffs, subs, nil)
}

// ownerRequest builds a request authenticated as the given owner.
func ownerRequest(t *testing.T, method, target string, userID uuid.UUID) *http.Request {
	t.Helper()
	return httptest.NewRequestWithContext(
		httpsupport.WithUserID(t.Context(), userID),
		method, target, http.NoBody,
	)
}

// TestListTariffs_MapsDomainTariffsToContract proves GET /tariffs answers
// with the frozen OpenAPI shape: items with name, kopeck prices as integers
// and the property limit (issue #245).
func TestListTariffs_MapsDomainTariffsToContract(t *testing.T) {
	ownerID := uuid.New()
	h := newTestHandlers(&fakeTariffLister{list: func(context.Context) ([]domain.Tariff, error) {
		return []domain.Tariff{
			{Name: domain.TariffBasic, ActivePropertyLimit: 1, MonthlyPriceKopecks: 0, YearlyPriceKopecks: 0, IsActive: true},
			{Name: domain.TariffPro, ActivePropertyLimit: 5, MonthlyPriceKopecks: 49000, YearlyPriceKopecks: 440000, IsActive: true},
			{Name: domain.TariffBusiness, ActivePropertyLimit: -1, MonthlyPriceKopecks: 99000, YearlyPriceKopecks: 890000, IsActive: true},
		}, nil
	}}, nil)

	w := httptest.NewRecorder()
	h.ListTariffs(w, ownerRequest(t, http.MethodGet, "/tariffs", ownerID))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Items []struct {
			Name                string `json:"name"`
			ActivePropertyLimit int    `json:"activePropertyLimit"`
			MonthlyPriceKopecks int    `json:"monthlyPriceKopecks"`
			YearlyPriceKopecks  int    `json:"yearlyPriceKopecks"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(resp.Items))
	}
	pro := resp.Items[1]
	if pro.Name != "pro" || pro.ActivePropertyLimit != 5 || pro.MonthlyPriceKopecks != 49000 || pro.YearlyPriceKopecks != 440000 {
		t.Errorf("pro item = %+v, want name=pro limit=5 monthly=49000 yearly=440000", pro)
	}
}

// TestListTariffs_RequiresOwner proves the endpoint rejects unauthenticated
// calls with 401 before touching the service.
func TestListTariffs_RequiresOwner(t *testing.T) {
	h := newTestHandlers(nil, nil)

	w := httptest.NewRecorder()
	h.ListTariffs(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/tariffs", http.NoBody))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

// TestGetSubscription_MapsViewToContract proves GET /subscription answers with
// the frozen OpenAPI shape: required tariff/status/autoRenewEnabled plus the
// optional lifecycle fields (validUntil, currentPeriod, pending tariff trio).
func TestGetSubscription_MapsViewToContract(t *testing.T) {
	ownerID := uuid.New()
	validUntil := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	pendingChangeAt := validUntil
	period := domain.PeriodMonth
	pendingPeriod := domain.PeriodYear
	h := newTestHandlers(nil, &fakeSubscriptionViewer{get: func(_ context.Context, userID uuid.UUID) (billingapp.SubscriptionView, error) {
		if userID != ownerID {
			t.Errorf("GetSubscription called with user %v, want %v", userID, ownerID)
		}
		return billingapp.SubscriptionView{
			Subscription: domain.Subscription{
				ID:               uuid.New(),
				UserID:           userID,
				Status:           domain.SubscriptionStatusActive,
				ValidUntil:       &validUntil,
				AutoRenewEnabled: true,
				CurrentPeriod:    &period,
				PendingTariffID:  new(uuid.UUID),
				PendingChangeAt:  &pendingChangeAt,
				PendingPeriod:    &pendingPeriod,
			},
			Tariff:        domain.Tariff{Name: domain.TariffPro, ActivePropertyLimit: 5, MonthlyPriceKopecks: 49000, YearlyPriceKopecks: 440000},
			PendingTariff: &domain.Tariff{Name: domain.TariffBasic, ActivePropertyLimit: 1},
		}, nil
	}})

	w := httptest.NewRecorder()
	h.GetSubscription(w, ownerRequest(t, http.MethodGet, "/subscription", ownerID))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Tariff struct {
			Name string `json:"name"`
		} `json:"tariff"`
		Status           string  `json:"status"`
		ValidUntil       *string `json:"validUntil"`
		AutoRenewEnabled bool    `json:"autoRenewEnabled"`
		CurrentPeriod    *string `json:"currentPeriod"`
		PendingTariff    *struct {
			Name string `json:"name"`
		} `json:"pendingTariff"`
		PendingChangeAt *string `json:"pendingChangeAt"`
		PendingPeriod   *string `json:"pendingPeriod"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Tariff.Name != "pro" {
		t.Errorf("tariff.name = %q, want pro", resp.Tariff.Name)
	}
	if resp.Status != "active" {
		t.Errorf("status = %q, want active", resp.Status)
	}
	if !resp.AutoRenewEnabled {
		t.Error("autoRenewEnabled = false, want true")
	}
	if resp.ValidUntil == nil {
		t.Error("validUntil = nil, want set")
	}
	if resp.CurrentPeriod == nil || *resp.CurrentPeriod != "month" {
		t.Errorf("currentPeriod = %v, want month", resp.CurrentPeriod)
	}
	if resp.PendingTariff == nil || resp.PendingTariff.Name != "basic" {
		t.Errorf("pendingTariff = %+v, want basic", resp.PendingTariff)
	}
	if resp.PendingPeriod == nil || *resp.PendingPeriod != "year" {
		t.Errorf("pendingPeriod = %v, want year", resp.PendingPeriod)
	}
	if resp.PendingChangeAt == nil {
		t.Error("pendingChangeAt = nil, want set")
	}
}

// TestGetSubscription_NotFoundIsProblem proves a missing subscription maps to
// the contract's 404 with an RFC 7807 problem body.
func TestGetSubscription_NotFoundIsProblem(t *testing.T) {
	h := newTestHandlers(nil, &fakeSubscriptionViewer{get: func(context.Context, uuid.UUID) (billingapp.SubscriptionView, error) {
		return billingapp.SubscriptionView{}, billingapp.ErrSubscriptionNotFound
	}})

	w := httptest.NewRecorder()
	h.GetSubscription(w, ownerRequest(t, http.MethodGet, "/subscription", uuid.New()))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", w.Code, w.Body.String())
	}
	var problem struct {
		Status int    `json:"status"`
		Title  string `json:"title"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem.Status != http.StatusNotFound {
		t.Errorf("problem.status = %d, want 404", problem.Status)
	}
}

// TestGetSubscription_InfrastructureErrorIs500 proves an unexpected service
// error answers 500 without leaking the cause into the response body.
func TestGetSubscription_InfrastructureErrorIs500(t *testing.T) {
	h := newTestHandlers(nil, &fakeSubscriptionViewer{get: func(context.Context, uuid.UUID) (billingapp.SubscriptionView, error) {
		return billingapp.SubscriptionView{}, errors.New("connection reset by peer")
	}})

	w := httptest.NewRecorder()
	h.GetSubscription(w, ownerRequest(t, http.MethodGet, "/subscription", uuid.New()))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	// The body is a sanitized problem; the internal cause must not leak.
	if body := w.Body.String(); strings.Contains(body, "connection reset by peer") {
		t.Errorf("body leaks the internal error cause: %s", body)
	}
}

// TestDeferredEndpoints_Answer501 proves the endpoints whose flows return with
// later tickets (#249–#254) answer 501 instead of pretending to work.
func TestDeferredEndpoints_Answer501(t *testing.T) {
	h := newTestHandlers(nil, nil)
	ownerID := uuid.New()

	cases := []struct {
		name   string
		call   func(w http.ResponseWriter, r *http.Request)
		method string
		path   string
	}{
		{name: "cancel", call: h.CancelSubscription, method: http.MethodPost, path: "/subscription/cancel"},
		{name: "auto-renew", call: h.ToggleAutoRenew, method: http.MethodPatch, path: "/subscription/auto-renew"},
		{name: "change", call: h.ChangeTariff, method: http.MethodPost, path: "/subscription/change"},
		{name: "payments", call: h.ListSubscriptionPayments, method: http.MethodGet, path: "/subscription/payments"},
		{name: "payment-methods", call: h.ListPaymentMethods, method: http.MethodGet, path: "/subscription/payment-methods"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tc.call(w, ownerRequest(t, tc.method, tc.path, ownerID))
			if w.Code != http.StatusNotImplemented {
				t.Fatalf("status = %d, want 501; body: %s", w.Code, w.Body.String())
			}
		})
	}
}
