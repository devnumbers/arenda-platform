package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
)

// Shared wire fixtures of the handler tests: tariff names, the fake provider
// identity, the binding id, and the admin-tariff payload keys.
const (
	testTariffPro    = "pro"
	testTariffBiz    = "business"
	testProviderFake = "fake"
	testBindingID    = "fake_bind_1"
	// Admin-tariff JSON payload keys shared by the request fixtures.
	testKeyName                = "name"
	testKeyActivePropertyLimit = "activePropertyLimit"
	testKeyMonthlyPrice        = "monthlyPriceKopecks"
	testKeyYearlyPrice         = "yearlyPriceKopecks"
)

// fakeTariffLister is a func-backed TariffLister (the consumer-side port of
// these handlers, ADR 0035).
type fakeTariffLister struct {
	list    func(ctx context.Context) ([]domain.Tariff, error)
	listAll func(ctx context.Context) ([]domain.Tariff, error)
}

func (f *fakeTariffLister) ListTariffs(ctx context.Context) ([]domain.Tariff, error) {
	if f.list != nil {
		return f.list(ctx)
	}
	return nil, errors.New("unexpected ListTariffs call")
}

func (f *fakeTariffLister) ListAllTariffs(ctx context.Context) ([]domain.Tariff, error) {
	if f.listAll != nil {
		return f.listAll(ctx)
	}
	return nil, errors.New("unexpected ListAllTariffs call")
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

// fakeSubscriptionManager is a func-backed SubscriptionManager (the
// consumer-side port of these handlers, ADR 0035).
type fakeSubscriptionManager struct {
	cancel       func(ctx context.Context, userID uuid.UUID) error
	toggleRenew  func(ctx context.Context, userID uuid.UUID, enabled bool) error
	changeTariff func(ctx context.Context, userID uuid.UUID, req billingapp.ChangeTariffRequest) (billingapp.ChangeTariffResult, error)
}

func (f *fakeSubscriptionManager) CancelSubscription(ctx context.Context, userID uuid.UUID) error {
	if f.cancel != nil {
		return f.cancel(ctx, userID)
	}
	return errors.New("unexpected CancelSubscription call")
}

func (f *fakeSubscriptionManager) ToggleAutoRenew(ctx context.Context, userID uuid.UUID, enabled bool) error {
	if f.toggleRenew != nil {
		return f.toggleRenew(ctx, userID, enabled)
	}
	return errors.New("unexpected ToggleAutoRenew call")
}

func (f *fakeSubscriptionManager) ChangeTariff(
	ctx context.Context, userID uuid.UUID, req billingapp.ChangeTariffRequest,
) (billingapp.ChangeTariffResult, error) {
	if f.changeTariff != nil {
		return f.changeTariff(ctx, userID, req)
	}
	return billingapp.ChangeTariffResult{}, errors.New("unexpected ChangeTariff call")
}

// fakePaymentManager is a func-backed PaymentManager (the consumer-side port
// of these handlers, ADR 0035).
type fakePaymentManager struct {
	list func(ctx context.Context, userID uuid.UUID) ([]billingapp.SubscriptionPaymentView, error)
}

func (f *fakePaymentManager) ListPayments(ctx context.Context, userID uuid.UUID) ([]billingapp.SubscriptionPaymentView, error) {
	if f.list != nil {
		return f.list(ctx, userID)
	}
	return nil, errors.New("unexpected ListPayments call")
}

// fakePaymentMethodManager is a func-backed PaymentMethodManager (the
// consumer-side port of these handlers, ADR 0035).
type fakePaymentMethodManager struct {
	list     func(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error)
	add      func(ctx context.Context, userID uuid.UUID, req billingapp.AddPaymentMethodRequest) (billingapp.AddPaymentMethodResult, error)
	del      func(ctx context.Context, userID, methodID uuid.UUID) error
	activate func(ctx context.Context, userID, methodID uuid.UUID) error
	sync     func(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error)
}

func (f *fakePaymentMethodManager) ListPaymentMethods(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error) {
	if f.list != nil {
		return f.list(ctx, userID)
	}
	return nil, errors.New("unexpected ListPaymentMethods call")
}

func (f *fakePaymentMethodManager) AddPaymentMethod(
	ctx context.Context, userID uuid.UUID, req billingapp.AddPaymentMethodRequest,
) (billingapp.AddPaymentMethodResult, error) {
	if f.add != nil {
		return f.add(ctx, userID, req)
	}
	return billingapp.AddPaymentMethodResult{}, errors.New("unexpected AddPaymentMethod call")
}

func (f *fakePaymentMethodManager) DeletePaymentMethod(ctx context.Context, userID, methodID uuid.UUID) error {
	if f.del != nil {
		return f.del(ctx, userID, methodID)
	}
	return errors.New("unexpected DeletePaymentMethod call")
}

func (f *fakePaymentMethodManager) ActivatePaymentMethod(ctx context.Context, userID, methodID uuid.UUID) error {
	if f.activate != nil {
		return f.activate(ctx, userID, methodID)
	}
	return errors.New("unexpected ActivatePaymentMethod call")
}

func (f *fakePaymentMethodManager) SyncPaymentMethods(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error) {
	if f.sync != nil {
		return f.sync(ctx, userID)
	}
	return nil, errors.New("unexpected SyncPaymentMethods call")
}

// fakeWebhookProcessor is a func-backed WebhookProcessor.
type fakeWebhookProcessor struct {
	handle func(ctx context.Context, providerName string, payload []byte) error
	ack    []byte
}

func (f *fakeWebhookProcessor) HandleWebhook(ctx context.Context, providerName string, payload []byte) error {
	if f.handle != nil {
		return f.handle(ctx, providerName, payload)
	}
	return errors.New("unexpected HandleWebhook call")
}

func (f *fakeWebhookProcessor) WebhookAck() []byte { return f.ack }

// fakeAdminTariffManager fails the test when an admin tariff endpoint is
// called without one being wired; the admin-tariff tests override it.
type fakeAdminTariffManager struct {
	create func(ctx context.Context, adminID uuid.UUID, req billingapp.CreateTariffRequest) (domain.Tariff, error)
	update func(ctx context.Context, adminID, tariffID uuid.UUID, req billingapp.UpdateTariffRequest) (domain.Tariff, error)
}

func (f *fakeAdminTariffManager) CreateTariff(
	ctx context.Context, adminID uuid.UUID, req billingapp.CreateTariffRequest,
) (domain.Tariff, error) {
	if f.create != nil {
		return f.create(ctx, adminID, req)
	}
	return domain.Tariff{}, errors.New("unexpected CreateTariff call")
}

func (f *fakeAdminTariffManager) UpdateTariff(
	ctx context.Context, adminID, tariffID uuid.UUID, req billingapp.UpdateTariffRequest,
) (domain.Tariff, error) {
	if f.update != nil {
		return f.update(ctx, adminID, tariffID, req)
	}
	return domain.Tariff{}, errors.New("unexpected UpdateTariff call")
}

// newTestHandlers builds handlers over the given fakes; a nil fake is
// replaced by a stub that fails the test when called.
func newTestHandlers(tariffs TariffLister, subs SubscriptionViewer, managers SubscriptionManager) *BillingHandlers {
	return newTestHandlersOpts(tariffs, subs, managers, nil, nil, nil)
}

// newTestHandlersOpts builds handlers with explicit payment,
// payment-method and webhook options.
func newTestHandlersOpts(
	tariffs TariffLister, subs SubscriptionViewer, managers SubscriptionManager,
	payments PaymentManager, methods PaymentMethodManager, webhooks WebhookProcessor,
) *BillingHandlers {
	if tariffs == nil {
		tariffs = &fakeTariffLister{}
	}
	if subs == nil {
		subs = &fakeSubscriptionViewer{}
	}
	if managers == nil {
		managers = &fakeSubscriptionManager{}
	}
	if payments == nil {
		payments = &fakePaymentManager{}
	}
	if methods == nil {
		methods = &fakePaymentMethodManager{}
	}
	if webhooks == nil {
		webhooks = &fakeWebhookProcessor{}
	}
	return NewBillingHandlers(
		tariffs, &fakeAdminTariffManager{}, subs, managers, payments, methods, webhooks,
		&fakeAdminPaymentManager{}, &fakeAdminSubscriptionManager{}, nil,
	)
}

// fakeAdminPaymentManager fails the test when an admin payment endpoint is
// called without one being wired; the admin-endpoint tests override it.
type fakeAdminPaymentManager struct {
	get    func(ctx context.Context, paymentID uuid.UUID) (billingapp.AdminSubscriptionPaymentView, error)
	list   func(ctx context.Context, filters billingapp.AdminPaymentFilters) ([]billingapp.AdminSubscriptionPaymentView, int64, error)
	refund func(ctx context.Context, adminID, paymentID uuid.UUID) error
	sync   func(ctx context.Context, adminID, paymentID uuid.UUID) error
}

func (f *fakeAdminPaymentManager) GetAdminPayment(
	ctx context.Context, paymentID uuid.UUID,
) (billingapp.AdminSubscriptionPaymentView, error) {
	if f.get != nil {
		return f.get(ctx, paymentID)
	}
	return billingapp.AdminSubscriptionPaymentView{}, errors.New("unexpected GetAdminPayment call")
}

func (f *fakeAdminPaymentManager) ListAdminPayments(
	ctx context.Context, filters billingapp.AdminPaymentFilters,
) ([]billingapp.AdminSubscriptionPaymentView, int64, error) {
	if f.list != nil {
		return f.list(ctx, filters)
	}
	return nil, 0, errors.New("unexpected ListAdminPayments call")
}

func (f *fakeAdminPaymentManager) RefundPayment(ctx context.Context, adminID, paymentID uuid.UUID) error {
	if f.refund != nil {
		return f.refund(ctx, adminID, paymentID)
	}
	return errors.New("unexpected RefundPayment call")
}

func (f *fakeAdminPaymentManager) SyncPayment(ctx context.Context, adminID, paymentID uuid.UUID) error {
	if f.sync != nil {
		return f.sync(ctx, adminID, paymentID)
	}
	return errors.New("unexpected SyncPayment call")
}

// fakeAdminSubscriptionManager fails the test when an admin subscription
// endpoint is called without one being wired; the admin-endpoint tests
// override it.
type fakeAdminSubscriptionManager struct {
	assignService func(ctx context.Context, adminID, userID uuid.UUID, req billingapp.AssignServiceSubscriptionRequest) error
	forceChange   func(ctx context.Context, adminID, userID uuid.UUID, req billingapp.ForceChangeTariffRequest) error
	extendGrace   func(ctx context.Context, adminID, userID uuid.UUID, days int) error
	cancel        func(ctx context.Context, adminID, userID uuid.UUID) error
	transitions   func(ctx context.Context, userID uuid.UUID) ([]billingapp.SubscriptionTransitionView, error)
}

func (f *fakeAdminSubscriptionManager) AssignServiceSubscription(
	ctx context.Context, adminID, userID uuid.UUID, req billingapp.AssignServiceSubscriptionRequest,
) error {
	if f.assignService != nil {
		return f.assignService(ctx, adminID, userID, req)
	}
	return errors.New("unexpected AssignServiceSubscription call")
}

func (f *fakeAdminSubscriptionManager) ForceChangeTariff(
	ctx context.Context, adminID, userID uuid.UUID, req billingapp.ForceChangeTariffRequest,
) error {
	if f.forceChange != nil {
		return f.forceChange(ctx, adminID, userID, req)
	}
	return errors.New("unexpected ForceChangeTariff call")
}

func (f *fakeAdminSubscriptionManager) ExtendGrace(ctx context.Context, adminID, userID uuid.UUID, days int) error {
	if f.extendGrace != nil {
		return f.extendGrace(ctx, adminID, userID, days)
	}
	return errors.New("unexpected ExtendGrace call")
}

func (f *fakeAdminSubscriptionManager) CancelSubscriptionAsAdmin(ctx context.Context, adminID, userID uuid.UUID) error {
	if f.cancel != nil {
		return f.cancel(ctx, adminID, userID)
	}
	return errors.New("unexpected CancelSubscriptionAsAdmin call")
}

func (f *fakeAdminSubscriptionManager) ListTransitions(
	ctx context.Context, userID uuid.UUID,
) ([]billingapp.SubscriptionTransitionView, error) {
	if f.transitions != nil {
		return f.transitions(ctx, userID)
	}
	return nil, errors.New("unexpected ListTransitions call")
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
	ownerID := uuid.Must(uuid.NewV7())
	h := newTestHandlers(&fakeTariffLister{list: func(context.Context) ([]domain.Tariff, error) {
		return []domain.Tariff{
			{Name: domain.TariffBasic, ActivePropertyLimit: 1, MonthlyPriceKopecks: 0, YearlyPriceKopecks: 0, IsActive: true},
			{Name: domain.TariffPro, ActivePropertyLimit: 5, MonthlyPriceKopecks: 49000, YearlyPriceKopecks: 440000, IsActive: true},
			{Name: domain.TariffBusiness, ActivePropertyLimit: -1, MonthlyPriceKopecks: 99000, YearlyPriceKopecks: 890000, IsActive: true},
		}, nil
	}}, nil, nil)

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
	if pro.Name != testTariffPro || pro.ActivePropertyLimit != 5 || pro.MonthlyPriceKopecks != 49000 || pro.YearlyPriceKopecks != 440000 {
		t.Errorf("pro item = %+v, want name=pro limit=5 monthly=49000 yearly=440000", pro)
	}
}

// TestListTariffs_RequiresOwner proves the endpoint rejects unauthenticated
// calls with 401 before touching the service.
func TestListTariffs_RequiresOwner(t *testing.T) {
	h := newTestHandlers(nil, nil, nil)

	w := httptest.NewRecorder()
	h.ListTariffs(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/tariffs", http.NoBody))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

// TestListAdminTariffs_MapsDomainTariffsToContract proves GET /admin/tariffs
// answers with the frozen OpenAPI shape: every tariff including hidden ones,
// with id, isActive and the kopeck prices as integers (issue #247).
func TestListAdminTariffs_MapsDomainTariffsToContract(t *testing.T) {
	hiddenID := uuid.Must(uuid.NewV7())
	h := newTestHandlers(&fakeTariffLister{listAll: func(context.Context) ([]domain.Tariff, error) {
		return []domain.Tariff{
			{
				ID: uuid.Must(uuid.NewV7()), Name: domain.TariffBasic, ActivePropertyLimit: 1,
				MonthlyPriceKopecks: 0, YearlyPriceKopecks: 0, IsActive: true,
			},
			{
				ID: uuid.Must(uuid.NewV7()), Name: domain.TariffPro, ActivePropertyLimit: 5,
				MonthlyPriceKopecks: 49000, YearlyPriceKopecks: 440000, IsActive: true,
			},
			{
				ID: hiddenID, Name: domain.TariffBusiness, ActivePropertyLimit: -1,
				MonthlyPriceKopecks: 99000, YearlyPriceKopecks: 890000, IsActive: false,
			},
		}, nil
	}}, nil, nil)

	w := httptest.NewRecorder()
	h.ListAdminTariffs(w, ownerRequest(t, http.MethodGet, "/admin/tariffs", uuid.Must(uuid.NewV7())))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Items []struct {
			ID                  string `json:"id"`
			Name                string `json:"name"`
			IsActive            bool   `json:"isActive"`
			ActivePropertyLimit int    `json:"activePropertyLimit"`
			MonthlyPriceKopecks int    `json:"monthlyPriceKopecks"`
			YearlyPriceKopecks  int    `json:"yearlyPriceKopecks"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Items) != 3 {
		t.Fatalf("items = %d, want 3 (hidden tariff included)", len(resp.Items))
	}
	hidden := resp.Items[2]
	if hidden.ID != hiddenID.String() {
		t.Errorf("hidden item id = %q, want %q", hidden.ID, hiddenID)
	}
	if hidden.Name != testTariffBiz || hidden.IsActive || hidden.ActivePropertyLimit != -1 ||
		hidden.MonthlyPriceKopecks != 99000 || hidden.YearlyPriceKopecks != 890000 {
		t.Errorf("hidden item = %+v, want name=business isActive=false limit=-1 monthly=99000 yearly=890000", hidden)
	}
}

// TestListAdminTariffs_InfrastructureErrorIs500 proves an unexpected service
// error answers 500 without leaking the cause into the response body.
func TestListAdminTariffs_InfrastructureErrorIs500(t *testing.T) {
	h := newTestHandlers(&fakeTariffLister{listAll: func(context.Context) ([]domain.Tariff, error) {
		return nil, errors.New("connection reset by peer")
	}}, nil, nil)

	w := httptest.NewRecorder()
	h.ListAdminTariffs(w, ownerRequest(t, http.MethodGet, "/admin/tariffs", uuid.Must(uuid.NewV7())))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	if body := w.Body.String(); strings.Contains(body, "connection reset by peer") {
		t.Errorf("body leaks the internal error cause: %s", body)
	}
}

// adminJSONRequest builds an admin-authenticated request with a JSON body.
func adminJSONRequest(t *testing.T, method, target string, adminID uuid.UUID, body any) *http.Request {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequestWithContext(
		httpsupport.WithActor(t.Context(), adminID, actor.RoleAdmin),
		method, target, bytes.NewReader(payload),
	)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// TestCreateAdminTariff_CreatesAndAnswers201 proves POST /admin/tariffs
// (issue #256): the body maps to the application request, the acting admin is
// attributed, and the created plan answers 201 with the contract shape.
func TestCreateAdminTariff_CreatesAndAnswers201(t *testing.T) {
	adminID := uuid.Must(uuid.NewV7())
	created := domain.Tariff{
		ID: uuid.Must(uuid.NewV7()), Name: domain.TariffPro, ActivePropertyLimit: 5,
		MonthlyPriceKopecks: 59000, YearlyPriceKopecks: 540000, IsActive: true,
	}
	h := NewBillingHandlers(nil, &fakeAdminTariffManager{create: func(
		_ context.Context, gotAdmin uuid.UUID, req billingapp.CreateTariffRequest,
	) (domain.Tariff, error) {
		if gotAdmin != adminID {
			t.Errorf("CreateTariff admin = %v, want %v", gotAdmin, adminID)
		}
		if req.Name != domain.TariffPro || req.ActivePropertyLimit != 5 ||
			req.MonthlyPriceKopecks != 59000 || req.YearlyPriceKopecks != 540000 {
			t.Errorf("CreateTariff request = %+v, want the decoded body", req)
		}
		return created, nil
	}}, nil, nil, nil, nil, nil, &fakeAdminPaymentManager{}, &fakeAdminSubscriptionManager{}, nil)

	w := httptest.NewRecorder()
	h.CreateAdminTariff(w, adminJSONRequest(t, http.MethodPost, "/admin/tariffs", adminID, map[string]any{
		testKeyName: testTariffPro, testKeyActivePropertyLimit: 5, testKeyMonthlyPrice: 59000, testKeyYearlyPrice: 540000,
	}))

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		ID                  string `json:"id"`
		Name                string `json:"name"`
		IsActive            bool   `json:"isActive"`
		ActivePropertyLimit int    `json:"activePropertyLimit"`
		MonthlyPriceKopecks int    `json:"monthlyPriceKopecks"`
		YearlyPriceKopecks  int    `json:"yearlyPriceKopecks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID != created.ID.String() || resp.Name != testTariffPro || !resp.IsActive ||
		resp.ActivePropertyLimit != 5 || resp.MonthlyPriceKopecks != 59000 || resp.YearlyPriceKopecks != 540000 {
		t.Errorf("response = %+v, want the created tariff", resp)
	}
}

// TestCreateAdminTariff_ErrorMapping proves the create endpoint's error
// contract (issue #256): a duplicate name answers 409, invalid pricing 400,
// an unknown name 400, and no session 401 before the service is touched.
func TestCreateAdminTariff_ErrorMapping(t *testing.T) {
	validBody := func() map[string]any {
		return map[string]any{testKeyName: testTariffPro, testKeyActivePropertyLimit: 5, testKeyMonthlyPrice: 59000, testKeyYearlyPrice: 540000}
	}
	tests := []struct {
		name   string
		body   map[string]any
		svcErr error
		want   int
	}{
		{"duplicate name", validBody(), billingapp.ErrTariffAlreadyExists, http.StatusConflict},
		{
			"invalid pricing",
			map[string]any{testKeyName: testTariffPro, testKeyActivePropertyLimit: 5, testKeyMonthlyPrice: -1, testKeyYearlyPrice: 540000},
			domain.ErrInvalidTariffPricing, http.StatusBadRequest,
		},
		{
			"unknown name",
			map[string]any{testKeyName: "premium", testKeyActivePropertyLimit: 5, testKeyMonthlyPrice: 59000, testKeyYearlyPrice: 540000},
			nil, http.StatusBadRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewBillingHandlers(nil, &fakeAdminTariffManager{create: func(
				context.Context, uuid.UUID, billingapp.CreateTariffRequest,
			) (domain.Tariff, error) {
				return domain.Tariff{}, tt.svcErr
			}}, nil, nil, nil, nil, nil, &fakeAdminPaymentManager{}, &fakeAdminSubscriptionManager{}, nil)

			w := httptest.NewRecorder()
			h.CreateAdminTariff(w, adminJSONRequest(t, http.MethodPost, "/admin/tariffs", uuid.Must(uuid.NewV7()), tt.body))

			if w.Code != tt.want {
				t.Fatalf("status = %d, want %d; body: %s", w.Code, tt.want, w.Body.String())
			}
		})
	}

	t.Run("requires admin session", func(t *testing.T) {
		h := NewBillingHandlers(nil, &fakeAdminTariffManager{}, nil, nil, nil, nil, nil,
			&fakeAdminPaymentManager{}, &fakeAdminSubscriptionManager{}, nil)

		w := httptest.NewRecorder()
		payload, err := json.Marshal(validBody())
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		h.CreateAdminTariff(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/tariffs", bytes.NewReader(payload)))

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
	})
}

// TestUpdateAdminTariff_UpdatesAndAnswers200 proves PUT /admin/tariffs/{id}
// (issue #256): the full editable state maps to the application request with
// the acting admin and the path id, and the saved plan answers 200.
func TestUpdateAdminTariff_UpdatesAndAnswers200(t *testing.T) {
	adminID := uuid.Must(uuid.NewV7())
	tariffID := uuid.Must(uuid.NewV7())
	updated := domain.Tariff{
		ID: tariffID, Name: domain.TariffPro, ActivePropertyLimit: 7,
		MonthlyPriceKopecks: 59000, YearlyPriceKopecks: 540000, IsActive: false,
	}
	h := NewBillingHandlers(nil, &fakeAdminTariffManager{update: func(
		_ context.Context, gotAdmin, gotID uuid.UUID, req billingapp.UpdateTariffRequest,
	) (domain.Tariff, error) {
		if gotAdmin != adminID {
			t.Errorf("UpdateTariff admin = %v, want %v", gotAdmin, adminID)
		}
		if gotID != tariffID {
			t.Errorf("UpdateTariff id = %v, want %v", gotID, tariffID)
		}
		if req.ActivePropertyLimit != 7 || req.MonthlyPriceKopecks != 59000 ||
			req.YearlyPriceKopecks != 540000 || req.IsActive {
			t.Errorf("UpdateTariff request = %+v, want the decoded body", req)
		}
		return updated, nil
	}}, nil, nil, nil, nil, nil, &fakeAdminPaymentManager{}, &fakeAdminSubscriptionManager{}, nil)

	w := httptest.NewRecorder()
	h.UpdateAdminTariff(w, adminJSONRequest(t, http.MethodPut, "/admin/tariffs/"+tariffID.String(), adminID, map[string]any{
		testKeyActivePropertyLimit: 7, testKeyMonthlyPrice: 59000, testKeyYearlyPrice: 540000, "isActive": false,
	}), tariffID)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		ID       string `json:"id"`
		IsActive bool   `json:"isActive"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID != tariffID.String() || resp.IsActive {
		t.Errorf("response = %+v, want the hidden updated tariff", resp)
	}
}

// TestUpdateAdminTariff_ErrorMapping proves the edit endpoint's error
// contract (issue #256): a miss answers 404, invalid pricing 400.
func TestUpdateAdminTariff_ErrorMapping(t *testing.T) {
	tests := []struct {
		name   string
		svcErr error
		want   int
	}{
		{"missing tariff", billingapp.ErrTariffNotFound, http.StatusNotFound},
		{"invalid pricing", domain.ErrInvalidTariffPricing, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewBillingHandlers(nil, &fakeAdminTariffManager{update: func(
				context.Context, uuid.UUID, uuid.UUID, billingapp.UpdateTariffRequest,
			) (domain.Tariff, error) {
				return domain.Tariff{}, tt.svcErr
			}}, nil, nil, nil, nil, nil, &fakeAdminPaymentManager{}, &fakeAdminSubscriptionManager{}, nil)

			tariffID := uuid.Must(uuid.NewV7())
			w := httptest.NewRecorder()
			h.UpdateAdminTariff(w, adminJSONRequest(t, http.MethodPut, "/admin/tariffs/"+tariffID.String(), uuid.Must(uuid.NewV7()), map[string]any{
				testKeyActivePropertyLimit: 7, testKeyMonthlyPrice: 59000, testKeyYearlyPrice: 540000, "isActive": false,
			}), tariffID)

			if w.Code != tt.want {
				t.Fatalf("status = %d, want %d; body: %s", w.Code, tt.want, w.Body.String())
			}
		})
	}
}

// TestGetSubscription_MapsViewToContract proves GET /subscription answers with
// the frozen OpenAPI shape: required tariff/status/autoRenewEnabled plus the
// optional lifecycle fields (validUntil, currentPeriod, pending tariff trio).
func TestGetSubscription_MapsViewToContract(t *testing.T) {
	ownerID := uuid.Must(uuid.NewV7())
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
				ID:               uuid.Must(uuid.NewV7()),
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
	}}, nil)

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
	if resp.Tariff.Name != testTariffPro {
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
	}}, nil)

	w := httptest.NewRecorder()
	h.GetSubscription(w, ownerRequest(t, http.MethodGet, "/subscription", uuid.Must(uuid.NewV7())))

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
	}}, nil)

	w := httptest.NewRecorder()
	h.GetSubscription(w, ownerRequest(t, http.MethodGet, "/subscription", uuid.Must(uuid.NewV7())))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	// The body is a sanitized problem; the internal cause must not leak.
	if body := w.Body.String(); strings.Contains(body, "connection reset by peer") {
		t.Errorf("body leaks the internal error cause: %s", body)
	}
}

// adminRequest builds an admin-authenticated request (the AdminOnlyMiddleware
// stores the actor the handlers attribute refunds and syncs with).
func adminRequest(t *testing.T, method, target string, adminID uuid.UUID) *http.Request {
	t.Helper()
	return httptest.NewRequestWithContext(
		httpsupport.WithActor(t.Context(), adminID, actor.RoleAdmin),
		method, target, nil,
	)
}

// adminPaymentView builds a view the get/list tests can round-trip.
func adminPaymentView(t *testing.T) billingapp.AdminSubscriptionPaymentView {
	t.Helper()
	providerPaymentID := "prov_1"
	succeededAt := time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC)
	refunded := int64(99000)
	payment, err := domain.NewSubscriptionPayment(
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		domain.PeriodMonth, 99000, testProviderFake, time.Date(2026, 8, 14, 8, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewSubscriptionPayment() error = %v", err)
	}
	payment.ProviderPaymentID = &providerPaymentID
	payment.Status = domain.PaymentStatusRefunded
	payment.RefundedAmountKopecks = &refunded
	payment.SucceededAt = &succeededAt
	return billingapp.AdminSubscriptionPaymentView{
		Payment:   payment,
		Tariff:    domain.Tariff{ID: payment.TariffID, Name: domain.TariffBusiness},
		UserPhone: "+79990000001",
	}
}

// TestGetAdminSubscriptionPayment_AnswersView proves GET
// /admin/subscription/payments/{id} maps the view to the contract DTO
// (issue #254).
func TestGetAdminSubscriptionPayment_AnswersView(t *testing.T) {
	view := adminPaymentView(t)
	h := newTestHandlers(nil, nil, nil)
	h.adminPayments = &fakeAdminPaymentManager{get: func(
		_ context.Context, paymentID uuid.UUID,
	) (billingapp.AdminSubscriptionPaymentView, error) {
		if paymentID != view.Payment.ID {
			t.Errorf("GetAdminPayment called with %v, want %v", paymentID, view.Payment.ID)
		}
		return view, nil
	}}

	w := httptest.NewRecorder()
	h.GetAdminSubscriptionPayment(w,
		adminRequest(t, http.MethodGet, "/admin/subscription/payments/"+view.Payment.ID.String(), uuid.Must(uuid.NewV7())), view.Payment.ID)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp openapi.AdminSubscriptionPayment
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Id != view.Payment.ID {
		t.Errorf("id = %v, want %v", resp.Id, view.Payment.ID)
	}
	if resp.UserPhone != view.UserPhone {
		t.Errorf("userPhone = %q, want %q", resp.UserPhone, view.UserPhone)
	}
	if resp.RefundedAmountKopecks == nil || *resp.RefundedAmountKopecks != 99000 {
		t.Errorf("refundedAmountKopecks = %v, want 99000", resp.RefundedAmountKopecks)
	}
	if resp.SucceededAt == nil {
		t.Error("succeededAt = nil, want the timestamp")
	}
}

// TestRefundSubscriptionPayment_AttributesAdmin proves POST
// /admin/subscription/payments/{id}/refund answers 204 and forwards the acting
// admin from the session (issue #254).
func TestRefundSubscriptionPayment_AttributesAdmin(t *testing.T) {
	adminID := uuid.Must(uuid.NewV7())
	paymentID := uuid.Must(uuid.NewV7())
	check, called := requireAdminAttribution(t, "RefundPayment", adminID, paymentID)
	h := newTestHandlers(nil, nil, nil)
	h.adminPayments = &fakeAdminPaymentManager{refund: check}

	w := httptest.NewRecorder()
	h.RefundSubscriptionPayment(w,
		adminRequest(t, http.MethodPost, "/admin/subscription/payments/"+paymentID.String()+"/refund", adminID), paymentID)

	requireNoContentAndCalled(t, w, called)
}

// requireAdminAttribution builds a manager callback for the *AttributesAdmin
// tests: it records the call and asserts the acting admin and the target entity
// were forwarded unchanged (issues #254, #255).
func requireAdminAttribution(
	t *testing.T, what string, wantAdmin, wantEntity uuid.UUID,
) (check func(context.Context, uuid.UUID, uuid.UUID) error, called *bool) {
	t.Helper()
	called = new(bool)
	return func(_ context.Context, gotAdmin, gotEntity uuid.UUID) error {
		*called = true
		if gotAdmin != wantAdmin {
			t.Errorf("%s admin = %v, want %v", what, gotAdmin, wantAdmin)
		}
		if gotEntity != wantEntity {
			t.Errorf("%s entity = %v, want %v", what, gotEntity, wantEntity)
		}
		return nil
	}, called
}

// requireNoContentAndCalled asserts an admin action answered 204 and actually
// reached the manager callback.
func requireNoContentAndCalled(t *testing.T, w *httptest.ResponseRecorder, called *bool) {
	t.Helper()
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", w.Code, w.Body.String())
	}
	if !*called {
		t.Error("service was not called")
	}
}

// TestSyncSubscriptionPayment_AttributesAdmin proves POST
// /admin/subscription/payments/{id}/sync answers 204 and forwards the acting
// admin (issue #254).
func TestSyncSubscriptionPayment_AttributesAdmin(t *testing.T) {
	adminID := uuid.Must(uuid.NewV7())
	paymentID := uuid.Must(uuid.NewV7())
	h := newTestHandlers(nil, nil, nil)
	h.adminPayments = &fakeAdminPaymentManager{sync: func(_ context.Context, gotAdmin, gotPayment uuid.UUID) error {
		if gotAdmin != adminID {
			t.Errorf("SyncPayment admin = %v, want %v", gotAdmin, adminID)
		}
		if gotPayment != paymentID {
			t.Errorf("SyncPayment payment = %v, want %v", gotPayment, paymentID)
		}
		return nil
	}}

	w := httptest.NewRecorder()
	h.SyncSubscriptionPayment(w,
		adminRequest(t, http.MethodPost, "/admin/subscription/payments/"+paymentID.String()+"/sync", adminID), paymentID)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", w.Code, w.Body.String())
	}
}

// TestListAdminSubscriptionPayments_MapsFilters proves GET
// /admin/subscription/payments forwards every contract filter to the service
// and answers the items with the total (issue #254).
func TestListAdminSubscriptionPayments_MapsFilters(t *testing.T) {
	userID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	limit, offset := 50, 10
	h := newTestHandlers(nil, nil, nil)
	h.adminPayments = &fakeAdminPaymentManager{list: func(
		_ context.Context, filters billingapp.AdminPaymentFilters,
	) ([]billingapp.AdminSubscriptionPaymentView, int64, error) {
		if filters.UserID == nil || *filters.UserID != userID {
			t.Errorf("userID filter = %v, want %v", filters.UserID, userID)
		}
		if filters.Status != string(domain.PaymentStatusRefunding) {
			t.Errorf("status filter = %q, want refunding", filters.Status)
		}
		if filters.UserPhone != "+79990000001" {
			t.Errorf("userPhone filter = %q", filters.UserPhone)
		}
		if filters.SubscriptionStatus != string(domain.SubscriptionStatusGrace) {
			t.Errorf("subscriptionStatus filter = %q, want grace", filters.SubscriptionStatus)
		}
		if filters.Sort != "amountKopecks" || filters.Order != "asc" {
			t.Errorf("sort/order = %q/%q, want amountKopecks/asc", filters.Sort, filters.Order)
		}
		if filters.Limit != limit || filters.Offset != offset {
			t.Errorf("limit/offset = %d/%d, want %d/%d", filters.Limit, filters.Offset, limit, offset)
		}
		return []billingapp.AdminSubscriptionPaymentView{adminPaymentView(t)}, 7, nil
	}}

	target := "/admin/subscription/payments?user_id=" + userID.String() +
		"&status=refunding&user_phone=%2B79990000001&subscription_status=grace" +
		"&sort=amountKopecks&order=asc&limit=50&offset=10"
	w := httptest.NewRecorder()
	statusParam := openapi.SubscriptionPaymentStatus("refunding")
	phoneParam := "+79990000001"
	subscriptionStatusParam := openapi.SubscriptionStatus("grace")
	sortParam := "amountKopecks"
	orderParam := openapi.ListAdminSubscriptionPaymentsParamsOrder("asc")
	h.ListAdminSubscriptionPayments(w,
		adminRequest(t, http.MethodGet, target, uuid.Must(uuid.NewV7())), openapi.ListAdminSubscriptionPaymentsParams{
			UserId:             &userID,
			Status:             &statusParam,
			UserPhone:          &phoneParam,
			SubscriptionStatus: &subscriptionStatusParam,
			Sort:               &sortParam,
			Order:              &orderParam,
			Limit:              &limit,
			Offset:             &offset,
		})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp openapi.AdminSubscriptionPaymentsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Total != 7 || len(resp.Items) != 1 {
		t.Fatalf("total/items = %d/%d, want 7/1", resp.Total, len(resp.Items))
	}
	if resp.Items[0].UserPhone != "+79990000001" {
		t.Errorf("item userPhone = %q", resp.Items[0].UserPhone)
	}
}

// TestListAdminSubscriptionPayments_InvalidFilterAnswers400 proves a sort
// field outside the whitelist answers 400, not 500 (issue #254).
func TestListAdminSubscriptionPayments_InvalidFilterAnswers400(t *testing.T) {
	h := newTestHandlers(nil, nil, nil)
	h.adminPayments = &fakeAdminPaymentManager{list: func(
		_ context.Context, filters billingapp.AdminPaymentFilters,
	) ([]billingapp.AdminSubscriptionPaymentView, int64, error) {
		return nil, 0, billingapp.ErrInvalidFilter
	}}

	w := httptest.NewRecorder()
	h.ListAdminSubscriptionPayments(w,
		adminRequest(t, http.MethodGet, "/admin/subscription/payments", uuid.Must(uuid.NewV7())), openapi.ListAdminSubscriptionPaymentsParams{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
}

// ownerJSONRequest builds an owner-authenticated request with a JSON body.
func ownerJSONRequest(t *testing.T, method, target string, userID uuid.UUID, body string) *http.Request {
	t.Helper()
	return httptest.NewRequestWithContext(
		httpsupport.WithUserID(t.Context(), userID),
		method, target, strings.NewReader(body),
	)
}

// TestCancelSubscription_Returns204 proves POST /subscription/cancel answers
// 204 and forwards the owner id to the service (issue #249).
func TestCancelSubscription_Returns204(t *testing.T) {
	ownerID := uuid.Must(uuid.NewV7())
	called := false
	h := newTestHandlers(nil, nil, &fakeSubscriptionManager{cancel: func(_ context.Context, got uuid.UUID) error {
		called = true
		if got != ownerID {
			t.Errorf("CancelSubscription called with %v, want %v", got, ownerID)
		}
		return nil
	}})

	w := httptest.NewRecorder()
	h.CancelSubscription(w, ownerRequest(t, http.MethodPost, "/subscription/cancel", ownerID))

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", w.Code, w.Body.String())
	}
	if !called {
		t.Error("service was not called")
	}
}

// TestCancelSubscription_RequiresOwner proves the endpoint rejects
// unauthenticated calls with 401 before touching the service.
func TestCancelSubscription_RequiresOwner(t *testing.T) {
	h := newTestHandlers(nil, nil, nil)

	w := httptest.NewRecorder()
	h.CancelSubscription(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/subscription/cancel", http.NoBody))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

// TestCancelSubscription_ErrorMapping proves the service sentinels map to the
// frozen contract statuses: 404 for a missing subscription, 409 for an invalid
// state.
func TestCancelSubscription_ErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{name: "no subscription", err: billingapp.ErrSubscriptionNotFound, want: http.StatusNotFound},
		{name: "already cancelled", err: domain.ErrInvalidSubscriptionState, want: http.StatusConflict},
		{name: "infrastructure", err: errors.New("connection reset"), want: http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandlers(nil, nil, &fakeSubscriptionManager{cancel: func(context.Context, uuid.UUID) error {
				return tc.err
			}})

			w := httptest.NewRecorder()
			h.CancelSubscription(w, ownerRequest(t, http.MethodPost, "/subscription/cancel", uuid.Must(uuid.NewV7())))

			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d; body: %s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

// TestToggleAutoRenew_Returns204AndForwardsBody proves PATCH
// /subscription/auto-renew decodes the contract body and answers 204 (issue
// #249).
func TestToggleAutoRenew_Returns204AndForwardsBody(t *testing.T) {
	ownerID := uuid.Must(uuid.NewV7())
	var gotEnabled bool
	h := newTestHandlers(nil, nil, &fakeSubscriptionManager{toggleRenew: func(_ context.Context, got uuid.UUID, enabled bool) error {
		if got != ownerID {
			t.Errorf("ToggleAutoRenew called with %v, want %v", got, ownerID)
		}
		gotEnabled = enabled
		return nil
	}})

	w := httptest.NewRecorder()
	h.ToggleAutoRenew(w, ownerJSONRequest(t, http.MethodPatch, "/subscription/auto-renew", ownerID, `{"enabled": true}`))

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", w.Code, w.Body.String())
	}
	if !gotEnabled {
		t.Error("service received enabled = false, want true")
	}
}

// TestToggleAutoRenew_RejectsBadBody proves a malformed body answers 400
// before the service is reached.
func TestToggleAutoRenew_RejectsBadBody(t *testing.T) {
	h := newTestHandlers(nil, nil, &fakeSubscriptionManager{toggleRenew: func(context.Context, uuid.UUID, bool) error {
		t.Error("service must not be called on a malformed body")
		return nil
	}})

	w := httptest.NewRecorder()
	h.ToggleAutoRenew(w, ownerJSONRequest(t, http.MethodPatch, "/subscription/auto-renew", uuid.Must(uuid.NewV7()), `{enabled`))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
}

// TestToggleAutoRenew_CannotEnableMapsTo409 proves the domain rule "no
// auto-renew without a validity period" answers 409 with a user-facing
// detail.
func TestToggleAutoRenew_CannotEnableMapsTo409(t *testing.T) {
	h := newTestHandlers(nil, nil, &fakeSubscriptionManager{toggleRenew: func(context.Context, uuid.UUID, bool) error {
		return domain.ErrCannotEnableAutoRenew
	}})

	w := httptest.NewRecorder()
	h.ToggleAutoRenew(w, ownerJSONRequest(t, http.MethodPatch, "/subscription/auto-renew", uuid.Must(uuid.NewV7()), `{"enabled": true}`))

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "автопродление") {
		t.Errorf("body = %s, want a user-facing detail about auto-renew", w.Body.String())
	}
}

// TestChangeTariff_DowngradeReturnsEmptyResult proves POST
// /subscription/change answers 200 with the contract shape on the free
// downgrade path: paymentId and confirmUrl are null (issue #249).
func TestChangeTariff_DowngradeReturnsEmptyResult(t *testing.T) {
	ownerID := uuid.Must(uuid.NewV7())
	var gotReq billingapp.ChangeTariffRequest
	h := newTestHandlers(nil, nil, &fakeSubscriptionManager{changeTariff: func(
		_ context.Context, got uuid.UUID, req billingapp.ChangeTariffRequest,
	) (billingapp.ChangeTariffResult, error) {
		if got != ownerID {
			t.Errorf("ChangeTariff called with %v, want %v", got, ownerID)
		}
		gotReq = req
		return billingapp.ChangeTariffResult{}, nil
	}})

	w := httptest.NewRecorder()
	h.ChangeTariff(w, ownerJSONRequest(t, http.MethodPost, "/subscription/change", ownerID, `{"tariffName":"pro","period":"year"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if gotReq.TariffName != domain.TariffPro || gotReq.Period != domain.PeriodYear {
		t.Errorf("service request = %+v, want pro/year", gotReq)
	}
	var resp struct {
		PaymentID  *uuid.UUID `json:"paymentId"`
		ConfirmURL *string    `json:"confirmUrl"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.PaymentID != nil || resp.ConfirmURL != nil {
		t.Errorf("response = %+v, want null paymentId and confirmUrl", resp)
	}
}

// TestChangeTariff_UpgradeTemporarilyUnavailable proves an upgrade answers
// 503 with an explicit payment-unavailable problem until the payment flow
// lands (issue #250).
func TestChangeTariff_UpgradeTemporarilyUnavailable(t *testing.T) {
	h := newTestHandlers(nil, nil, &fakeSubscriptionManager{changeTariff: func(
		context.Context, uuid.UUID, billingapp.ChangeTariffRequest,
	) (billingapp.ChangeTariffResult, error) {
		return billingapp.ChangeTariffResult{}, billingapp.ErrPaymentUnavailable
	}})

	w := httptest.NewRecorder()
	h.ChangeTariff(w,
		ownerJSONRequest(t, http.MethodPost, "/subscription/change", uuid.Must(uuid.NewV7()), `{"tariffName":"business","period":"month"}`))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Оплата временно недоступна") {
		t.Errorf("body = %s, want the explicit payment-unavailable detail", w.Body.String())
	}
}

// TestChangeTariff_ErrorMapping proves the request-validation and state
// sentinels map to the frozen contract statuses.
func TestChangeTariff_ErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{name: "same tariff", err: domain.ErrAlreadyOnTariff, want: http.StatusConflict},
		{name: "invalid state", err: domain.ErrInvalidSubscriptionState, want: http.StatusConflict},
		{name: "unknown tariff", err: billingapp.ErrTariffNotFound, want: http.StatusNotFound},
		{name: "infrastructure", err: errors.New("disk full"), want: http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandlers(nil, nil, &fakeSubscriptionManager{changeTariff: func(
				context.Context, uuid.UUID, billingapp.ChangeTariffRequest,
			) (billingapp.ChangeTariffResult, error) {
				return billingapp.ChangeTariffResult{}, tc.err
			}})

			w := httptest.NewRecorder()
			h.ChangeTariff(w,
				ownerJSONRequest(t, http.MethodPost, "/subscription/change", uuid.Must(uuid.NewV7()), `{"tariffName":"pro","period":"month"}`))

			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d; body: %s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

// TestChangeTariff_RejectsInvalidInput proves a malformed body, an unknown
// tariff name and an unknown period answer 400 before the service is
// reached.
func TestChangeTariff_RejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "malformed body", body: `{"tariffName":`},
		{name: "unknown tariff name", body: `{"tariffName":"gold","period":"month"}`},
		{name: "unknown period", body: `{"tariffName":"pro","period":"week"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandlers(nil, nil, &fakeSubscriptionManager{changeTariff: func(
				context.Context, uuid.UUID, billingapp.ChangeTariffRequest,
			) (billingapp.ChangeTariffResult, error) {
				t.Error("service must not be called on invalid input")
				return billingapp.ChangeTariffResult{}, nil
			}})

			w := httptest.NewRecorder()
			h.ChangeTariff(w, ownerJSONRequest(t, http.MethodPost, "/subscription/change", uuid.Must(uuid.NewV7()), tc.body))

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body: %s", w.Code, w.Body.String())
			}
		})
	}
}

// TestListSubscriptionPayments_MapsViewToContract proves GET
// /subscription/payments answers with the frozen OpenAPI shape: items with
// id, tariff, period, status, kopeck amount as integer, provider and the
// payment URL of an unfinished payment (issue #250).
func TestListSubscriptionPayments_MapsViewToContract(t *testing.T) {
	ownerID := uuid.Must(uuid.NewV7())
	paymentID := uuid.Must(uuid.NewV7())
	url := "http://localhost:8080/internal/fake-subscription-payment/x/confirm"
	createdAt := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	h := newTestHandlersOpts(nil, nil, nil, &fakePaymentManager{list: func(
		_ context.Context, got uuid.UUID,
	) ([]billingapp.SubscriptionPaymentView, error) {
		if got != ownerID {
			t.Errorf("ListPayments called with %v, want %v", got, ownerID)
		}
		return []billingapp.SubscriptionPaymentView{{
			Payment: domain.SubscriptionPayment{
				ID:                paymentID,
				UserID:            ownerID,
				TariffID:          uuid.Must(uuid.NewV7()),
				Period:            domain.PeriodMonth,
				AmountKopecks:     49000,
				Provider:          testProviderFake,
				ProviderPaymentID: new(string),
				PaymentURL:        &url,
				Status:            domain.PaymentStatusPending,
				CreatedAt:         createdAt,
			},
			Tariff: domain.Tariff{
				Name: domain.TariffPro, ActivePropertyLimit: 5,
				MonthlyPriceKopecks: 49000, YearlyPriceKopecks: 440000, IsActive: true,
			},
		}}, nil
	}}, nil, nil)

	w := httptest.NewRecorder()
	h.ListSubscriptionPayments(w, ownerRequest(t, http.MethodGet, "/subscription/payments", ownerID))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Items []struct {
			ID     string `json:"id"`
			Tariff struct {
				Name string `json:"name"`
			} `json:"tariff"`
			Period        string    `json:"period"`
			Status        string    `json:"status"`
			AmountKopecks int       `json:"amountKopecks"`
			Provider      string    `json:"provider"`
			PaymentURL    *string   `json:"paymentUrl"`
			CreatedAt     time.Time `json:"createdAt"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(resp.Items))
	}
	item := resp.Items[0]
	if item.ID != paymentID.String() || item.Tariff.Name != testTariffPro || item.Period != "month" ||
		item.Status != "pending" || item.AmountKopecks != 49000 || item.Provider != testProviderFake {
		t.Errorf("item = %+v, want id/tariff=pro/month/pending/49000/fake", item)
	}
	if item.PaymentURL == nil || *item.PaymentURL != url {
		t.Errorf("paymentUrl = %v, want %q", item.PaymentURL, url)
	}
}

// TestListSubscriptionPayments_RequiresOwner proves the endpoint rejects
// unauthenticated calls with 401 before touching the service.
func TestListSubscriptionPayments_RequiresOwner(t *testing.T) {
	h := newTestHandlers(nil, nil, nil)

	w := httptest.NewRecorder()
	h.ListSubscriptionPayments(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/subscription/payments", http.NoBody))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

// webhookRequest builds a webhook delivery request with the given payload.
func webhookRequest(t *testing.T, provider, payload string) *http.Request {
	t.Helper()
	return httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhooks/payment/"+provider, strings.NewReader(payload))
}

// TestHandlePaymentWebhook_AnswersAckOnSuccess proves a fully processed
// notification answers 200 with the provider's fixed acknowledgement body
// (issue #250, synchronous webhooks).
func TestHandlePaymentWebhook_AnswersAckOnSuccess(t *testing.T) {
	var gotPayload []byte
	h := newTestHandlersOpts(nil, nil, nil, nil, nil, &fakeWebhookProcessor{
		handle: func(_ context.Context, providerName string, payload []byte) error {
			if providerName != testProviderFake {
				t.Errorf("provider = %q, want fake", providerName)
			}
			gotPayload = payload
			return nil
		},
		ack: []byte(`{"status":"ok"}`),
	})

	w := httptest.NewRecorder()
	h.HandlePaymentWebhook(w,
		webhookRequest(t, testProviderFake, `{"internal_payment_id":"11111111-1111-1111-1111-111111111111"}`), testProviderFake)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != `{"status":"ok"}` {
		t.Errorf("body = %s, want the provider ack body", w.Body.String())
	}
	if string(gotPayload) != `{"internal_payment_id":"11111111-1111-1111-1111-111111111111"}` {
		t.Errorf("payload forwarded = %s", gotPayload)
	}
}

// TestHandlePaymentWebhook_ErrorMapping proves processing failures answer
// non-200 so the provider retries: request defects answer 400, a missing
// payment 404, anything else 500 (issue #250).
func TestHandlePaymentWebhook_ErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{name: "provider mismatch", err: billingapp.ErrWebhookProviderMismatch, want: http.StatusBadRequest},
		{name: "unsupported event", err: billingapp.ErrWebhookUnsupported, want: http.StatusBadRequest},
		{name: "payment id mismatch", err: billingapp.ErrWebhookPaymentMismatch, want: http.StatusBadRequest},
		{name: "unverified payload", err: billingapp.ErrWebhookRejected, want: http.StatusBadRequest},
		{name: "payment not found", err: billingapp.ErrPaymentNotFound, want: http.StatusNotFound},
		{name: "processing failure", err: errors.New("connection refused"), want: http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandlersOpts(nil, nil, nil, nil, nil, &fakeWebhookProcessor{
				handle: func(context.Context, string, []byte) error { return tc.err },
			})

			w := httptest.NewRecorder()
			h.HandlePaymentWebhook(w, webhookRequest(t, testProviderFake, `{}`), testProviderFake)

			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d; body: %s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

// TestHandlePaymentWebhook_RejectsOversizedBody proves a body over the size
// cap answers 413 without reaching the processor.
func TestHandlePaymentWebhook_RejectsOversizedBody(t *testing.T) {
	h := newTestHandlersOpts(nil, nil, nil, nil, nil, &fakeWebhookProcessor{
		handle: func(context.Context, string, []byte) error {
			t.Error("processor must not be called on an oversized body")
			return nil
		},
	})

	big := strings.Repeat("x", maxWebhookBody+1)
	w := httptest.NewRecorder()
	h.HandlePaymentWebhook(w, webhookRequest(t, testProviderFake, big), testProviderFake)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413; body: %s", w.Code, w.Body.String())
	}
}

// TestListPaymentMethods_MapsDomainMethodsToContract proves GET
// /subscription/payment-methods answers with the frozen OpenAPI shape:
// display fields only (no charge token), newest first (issue #251).
func TestListPaymentMethods_MapsDomainMethodsToContract(t *testing.T) {
	ownerID := uuid.Must(uuid.NewV7())
	createdAt := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	methodID := uuid.Must(uuid.NewV7())
	h := newTestHandlersOpts(nil, nil, nil, nil, &fakePaymentMethodManager{list: func(
		_ context.Context, got uuid.UUID,
	) ([]domain.PaymentMethod, error) {
		if got != ownerID {
			t.Errorf("ListPaymentMethods called with %v, want %v", got, ownerID)
		}
		return []domain.PaymentMethod{{
			ID:            methodID,
			UserID:        ownerID,
			Provider:      testProviderFake,
			ProviderToken: "secret-token",
			DisplayMask:   "4111********1111",
			IsActive:      true,
			CreatedAt:     createdAt,
		}}, nil
	}}, nil)

	w := httptest.NewRecorder()
	h.ListPaymentMethods(w, ownerRequest(t, http.MethodGet, "/subscription/payment-methods", ownerID))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "secret-token") {
		t.Errorf("response leaks the charge token: %s", body)
	}
	var resp struct {
		Items []struct {
			ID          string `json:"id"`
			Provider    string `json:"provider"`
			DisplayMask string `json:"displayMask"`
			IsActive    bool   `json:"isActive"`
			CreatedAt   string `json:"createdAt"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(resp.Items))
	}
	item := resp.Items[0]
	if item.ID != methodID.String() || item.Provider != testProviderFake ||
		item.DisplayMask != "4111********1111" || !item.IsActive || item.CreatedAt == "" {
		t.Errorf("item = %+v, want the full display shape", item)
	}
}

// TestListPaymentMethods_RequiresOwner proves the endpoint rejects
// unauthenticated calls with 401.
func TestListPaymentMethods_RequiresOwner(t *testing.T) {
	h := newTestHandlers(nil, nil, nil)

	w := httptest.NewRecorder()
	h.ListPaymentMethods(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/subscription/payment-methods", http.NoBody))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

// TestAddPaymentMethod_BindingReturnsConfirmURL proves the bank-form branch
// answers with the confirmation URL and no method (issue #251).
func TestAddPaymentMethod_BindingReturnsConfirmURL(t *testing.T) {
	ownerID := uuid.Must(uuid.NewV7())
	h := newTestHandlersOpts(nil, nil, nil, nil, &fakePaymentMethodManager{add: func(
		_ context.Context, got uuid.UUID, req billingapp.AddPaymentMethodRequest,
	) (billingapp.AddPaymentMethodResult, error) {
		if got != ownerID {
			t.Errorf("AddPaymentMethod called with %v, want %v", got, ownerID)
		}
		if req.ProviderToken != "" {
			t.Errorf("request token = %q, want empty for the binding branch", req.ProviderToken)
		}
		return billingapp.AddPaymentMethodResult{ConfirmURL: "https://pay.example/bind/1"}, nil
	}}, nil)

	w := httptest.NewRecorder()
	h.AddPaymentMethod(w, ownerJSONRequest(t, http.MethodPost, "/subscription/payment-methods", ownerID, `{}`))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		ConfirmURL    *string `json:"confirmUrl"`
		PaymentMethod *struct {
			ID string `json:"id"`
		} `json:"paymentMethod"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ConfirmURL == nil || *resp.ConfirmURL != "https://pay.example/bind/1" {
		t.Errorf("confirmUrl = %v, want the binding form URL", resp.ConfirmURL)
	}
	if resp.PaymentMethod != nil {
		t.Errorf("paymentMethod = %+v, want none on the binding branch", resp.PaymentMethod)
	}
}

// TestAddPaymentMethod_TokenReturnsMethod proves the synchronous branch
// forwards the raw token and answers with the created method (issue #251).
func TestAddPaymentMethod_TokenReturnsMethod(t *testing.T) {
	ownerID := uuid.Must(uuid.NewV7())
	methodID := uuid.Must(uuid.NewV7())
	h := newTestHandlersOpts(nil, nil, nil, nil, &fakePaymentMethodManager{add: func(
		_ context.Context, _ uuid.UUID, req billingapp.AddPaymentMethodRequest,
	) (billingapp.AddPaymentMethodResult, error) {
		if req.ProviderToken != "raw-token" {
			t.Errorf("request token = %q, want the raw token forwarded", req.ProviderToken)
		}
		return billingapp.AddPaymentMethodResult{PaymentMethod: &domain.PaymentMethod{
			ID: methodID, UserID: ownerID, Provider: testProviderFake, ProviderToken: "raw-token", IsActive: true,
			CreatedAt: time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC),
		}}, nil
	}}, nil)

	w := httptest.NewRecorder()
	h.AddPaymentMethod(w, ownerJSONRequest(t, http.MethodPost, "/subscription/payment-methods", ownerID, `{"providerToken":"raw-token"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		ConfirmURL    *string `json:"confirmUrl"`
		PaymentMethod *struct {
			ID       string `json:"id"`
			IsActive bool   `json:"isActive"`
		} `json:"paymentMethod"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ConfirmURL != nil {
		t.Errorf("confirmUrl = %v, want none on the token branch", resp.ConfirmURL)
	}
	if resp.PaymentMethod == nil || resp.PaymentMethod.ID != methodID.String() || !resp.PaymentMethod.IsActive {
		t.Errorf("paymentMethod = %+v, want the created active method", resp.PaymentMethod)
	}
}

// TestDeletePaymentMethod_MapsServiceErrors proves DELETE maps the in-use
// guard to the contract's 409 and misses to 404, success to 204 (issue #251).
func TestDeletePaymentMethod_MapsServiceErrors(t *testing.T) {
	ownerID := uuid.Must(uuid.NewV7())
	methodID := uuid.Must(uuid.NewV7())

	cases := []struct {
		name   string
		err    error
		status int
	}{
		{name: "in use", err: billingapp.ErrPaymentMethodInUse, status: http.StatusConflict},
		{name: "not found", err: billingapp.ErrPaymentMethodNotFound, status: http.StatusNotFound},
		{name: "deleted", err: nil, status: http.StatusNoContent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandlersOpts(nil, nil, nil, nil, &fakePaymentMethodManager{del: func(_ context.Context, gotUser, gotMethod uuid.UUID) error {
				if gotUser != ownerID || gotMethod != methodID {
					t.Errorf("DeletePaymentMethod called with %v/%v, want %v/%v", gotUser, gotMethod, ownerID, methodID)
				}
				return tc.err
			}}, nil)

			w := httptest.NewRecorder()
			h.DeletePaymentMethod(w, ownerRequest(t, http.MethodDelete, "/subscription/payment-methods/"+methodID.String(), ownerID), methodID)

			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d; body: %s", w.Code, tc.status, w.Body.String())
			}
		})
	}
}

// TestActivatePaymentMethod_Returns204 proves POST activate answers 204 and
// forwards owner and method ids (issue #251).
func TestActivatePaymentMethod_Returns204(t *testing.T) {
	ownerID := uuid.Must(uuid.NewV7())
	methodID := uuid.Must(uuid.NewV7())
	called := false
	h := newTestHandlersOpts(nil, nil, nil, nil, &fakePaymentMethodManager{activate: func(
		_ context.Context, gotUser, gotMethod uuid.UUID,
	) error {
		called = true
		if gotUser != ownerID || gotMethod != methodID {
			t.Errorf("ActivatePaymentMethod called with %v/%v, want %v/%v", gotUser, gotMethod, ownerID, methodID)
		}
		return nil
	}}, nil)

	w := httptest.NewRecorder()
	h.ActivatePaymentMethod(w,
		ownerRequest(t, http.MethodPost, "/subscription/payment-methods/"+methodID.String()+"/activate", ownerID), methodID)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", w.Code, w.Body.String())
	}
	if !called {
		t.Error("service was not called")
	}
}

// TestSyncPaymentMethods_ReturnsList proves POST sync answers with the
// up-to-date method list (issue #251).
func TestSyncPaymentMethods_ReturnsList(t *testing.T) {
	ownerID := uuid.Must(uuid.NewV7())
	h := newTestHandlersOpts(nil, nil, nil, nil, &fakePaymentMethodManager{sync: func(
		_ context.Context, got uuid.UUID,
	) ([]domain.PaymentMethod, error) {
		if got != ownerID {
			t.Errorf("SyncPaymentMethods called with %v, want %v", got, ownerID)
		}
		return []domain.PaymentMethod{
			{
				ID: uuid.Must(uuid.NewV7()), UserID: ownerID, Provider: testProviderFake, ProviderToken: "t",
				IsActive: true, CreatedAt: time.Now(),
			},
		}, nil
	}}, nil)

	w := httptest.NewRecorder()
	h.SyncPaymentMethods(w, ownerRequest(t, http.MethodPost, "/subscription/payment-methods/sync", ownerID))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(resp.Items))
	}
}

// adminBodyRequest builds an admin POST request carrying a JSON body.
func adminBodyRequest(t *testing.T, target string, adminID uuid.UUID, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequestWithContext(
		httpsupport.WithActor(t.Context(), adminID, actor.RoleAdmin),
		http.MethodPost, target, strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// subscriptionTransitionView builds a transition view the listing test can
// round-trip (issue #255).
func subscriptionTransitionView(t *testing.T) billingapp.SubscriptionTransitionView {
	t.Helper()
	subID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	fromTariff := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	toTariff := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	adminID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	paymentID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	fromStatus := domain.SubscriptionStatusActive
	fromName := testTariffPro
	return billingapp.SubscriptionTransitionView{
		Transition: domain.Transition{
			ID:             uuid.MustParse("66666666-6666-6666-6666-666666666666"),
			SubscriptionID: subID,
			FromStatus:     &fromStatus,
			ToStatus:       domain.SubscriptionStatusActive,
			FromTariffID:   &fromTariff,
			ToTariffID:     toTariff,
			Reason:         domain.TransitionReasonServiceAssigned,
			Initiator:      domain.InitiatorAdmin,
			InitiatorID:    &adminID,
			PaymentID:      &paymentID,
			CreatedAt:      time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC),
		},
		FromTariffName: &fromName,
		ToTariffName:   testTariffBiz,
	}
}

// assertAdminTransitionItem checks one mapped contract item against the
// transition view it must round-trip (issue #255).
func assertAdminTransitionItem(t *testing.T, item openapi.AdminSubscriptionTransition, view billingapp.SubscriptionTransitionView) {
	t.Helper()
	if item.Id != view.Transition.ID {
		t.Errorf("id = %v, want %v", item.Id, view.Transition.ID)
	}
	if item.ToTariffName != "business" || item.FromTariffName == nil || *item.FromTariffName != "pro" {
		t.Errorf("tariff names = %q/%v, want business/pro", item.ToTariffName, item.FromTariffName)
	}
	if item.Reason != string(domain.TransitionReasonServiceAssigned) {
		t.Errorf("reason = %q, want service_assigned", item.Reason)
	}
	if item.Initiator != openapi.AdminSubscriptionTransitionInitiatorAdmin {
		t.Errorf("initiator = %q, want admin", item.Initiator)
	}
	if item.InitiatorId == nil || *item.InitiatorId != *view.Transition.InitiatorID {
		t.Errorf("initiatorId = %v, want %v", item.InitiatorId, *view.Transition.InitiatorID)
	}
	if item.PaymentId == nil || *item.PaymentId != *view.Transition.PaymentID {
		t.Errorf("paymentId = %v, want the referenced payment", item.PaymentId)
	}
	if item.FromStatus == nil || *item.FromStatus != openapi.SubscriptionStatusActive {
		t.Errorf("fromStatus = %v, want active", item.FromStatus)
	}
}

// TestAssignAdminServiceSubscription_MapsBodyAndAttributesAdmin proves POST
// /admin/users/{id}/subscription/service answers 204, forwards the acting
// admin and maps the contract body to the application input (issue #255).
func TestAssignAdminServiceSubscription_MapsBodyAndAttributesAdmin(t *testing.T) {
	adminID := uuid.Must(uuid.NewV7())
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	called := false
	h := newTestHandlers(nil, nil, nil)
	h.adminSubscriptions = &fakeAdminSubscriptionManager{assignService: func(
		_ context.Context, gotAdmin, gotUser uuid.UUID, req billingapp.AssignServiceSubscriptionRequest,
	) error {
		called = true
		if gotAdmin != adminID {
			t.Errorf("AssignServiceSubscription admin = %v, want %v", gotAdmin, adminID)
		}
		if gotUser != userID {
			t.Errorf("AssignServiceSubscription user = %v, want %v", gotUser, userID)
		}
		if req.TariffName != domain.TariffBusiness {
			t.Errorf("tariffName = %q, want business", req.TariffName)
		}
		if req.TermType != billingapp.ServiceTermDate {
			t.Errorf("termType = %q, want date", req.TermType)
		}
		wantDate := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
		if req.UntilDate == nil || !req.UntilDate.Equal(wantDate) {
			t.Errorf("untilDate = %v, want %v", req.UntilDate, wantDate)
		}
		return nil
	}}

	w := httptest.NewRecorder()
	h.AssignAdminServiceSubscription(w, adminBodyRequest(t, "/admin/users/"+userID.String()+"/subscription/service", adminID,
		`{"tariffName":"business","termType":"date","untilDate":"2026-09-01"}`), userID)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", w.Code, w.Body.String())
	}
	if !called {
		t.Error("service was not called")
	}
}

// TestAssignAdminServiceSubscription_ErrorMapping proves the service
// assignment's error mapping: a past until date answers 400, a state conflict
// answers 409 (issue #255).
func TestAssignAdminServiceSubscription_ErrorMapping(t *testing.T) {
	userID := uuid.Must(uuid.NewV7())
	h := newTestHandlers(nil, nil, nil)
	h.adminSubscriptions = &fakeAdminSubscriptionManager{assignService: func(
		_ context.Context, _, _ uuid.UUID, _ billingapp.AssignServiceSubscriptionRequest,
	) error {
		return fmt.Errorf("%w: untilDate is in the past", domain.ErrInvalidTerm)
	}}
	w := httptest.NewRecorder()
	h.AssignAdminServiceSubscription(w, adminBodyRequest(t, "/admin/users/"+userID.String()+"/subscription/service", uuid.Must(uuid.NewV7()),
		`{"tariffName":"business","termType":"month"}`), userID)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}

	h.adminSubscriptions = &fakeAdminSubscriptionManager{assignService: func(
		_ context.Context, _, _ uuid.UUID, _ billingapp.AssignServiceSubscriptionRequest,
	) error {
		return domain.ErrInvalidSubscriptionState
	}}
	w = httptest.NewRecorder()
	h.AssignAdminServiceSubscription(w, adminBodyRequest(t, "/admin/users/"+userID.String()+"/subscription/service", uuid.Must(uuid.NewV7()),
		`{"tariffName":"business","termType":"month"}`), userID)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body: %s", w.Code, w.Body.String())
	}
}

// TestForceChangeAdminSubscriptionTariff_MapsBodyAndAttributesAdmin proves
// POST /admin/users/{id}/subscription/force-change answers 204 with the body
// and the acting admin forwarded (issue #255).
func TestForceChangeAdminSubscriptionTariff_MapsBodyAndAttributesAdmin(t *testing.T) {
	adminID := uuid.Must(uuid.NewV7())
	userID := uuid.Must(uuid.NewV7())
	h := newTestHandlers(nil, nil, nil)
	h.adminSubscriptions = &fakeAdminSubscriptionManager{forceChange: func(
		_ context.Context, gotAdmin, gotUser uuid.UUID, req billingapp.ForceChangeTariffRequest,
	) error {
		if gotAdmin != adminID {
			t.Errorf("ForceChangeTariff admin = %v, want %v", gotAdmin, adminID)
		}
		if gotUser != userID {
			t.Errorf("ForceChangeTariff user = %v, want %v", gotUser, userID)
		}
		if req.TariffName != domain.TariffPro {
			t.Errorf("tariffName = %q, want pro", req.TariffName)
		}
		if req.Period != domain.PeriodYear {
			t.Errorf("period = %q, want year", req.Period)
		}
		return nil
	}}

	w := httptest.NewRecorder()
	h.ForceChangeAdminSubscriptionTariff(w, adminBodyRequest(t, "/admin/users/"+userID.String()+"/subscription/force-change", adminID,
		`{"tariffName":"pro","period":"year"}`), userID)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", w.Code, w.Body.String())
	}
}

// TestExtendAdminSubscriptionGrace_MapsBodyAndAttributesAdmin proves POST
// /admin/users/{id}/subscription/grace-extension answers 204 with the day
// count and the acting admin forwarded (issue #255).
func TestExtendAdminSubscriptionGrace_MapsBodyAndAttributesAdmin(t *testing.T) {
	adminID := uuid.Must(uuid.NewV7())
	userID := uuid.Must(uuid.NewV7())
	h := newTestHandlers(nil, nil, nil)
	h.adminSubscriptions = &fakeAdminSubscriptionManager{extendGrace: func(_ context.Context, gotAdmin, gotUser uuid.UUID, days int) error {
		if gotAdmin != adminID {
			t.Errorf("ExtendGrace admin = %v, want %v", gotAdmin, adminID)
		}
		if gotUser != userID {
			t.Errorf("ExtendGrace user = %v, want %v", gotUser, userID)
		}
		if days != 5 {
			t.Errorf("days = %d, want 5", days)
		}
		return nil
	}}

	w := httptest.NewRecorder()
	h.ExtendAdminSubscriptionGrace(w, adminBodyRequest(t, "/admin/users/"+userID.String()+"/subscription/grace-extension", adminID,
		`{"days":5}`), userID)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", w.Code, w.Body.String())
	}
}

// TestCancelAdminSubscription_AttributesAdmin proves POST
// /admin/users/{id}/subscription/cancel answers 204 and forwards the acting
// admin (issue #255).
func TestCancelAdminSubscription_AttributesAdmin(t *testing.T) {
	adminID := uuid.Must(uuid.NewV7())
	userID := uuid.Must(uuid.NewV7())
	check, called := requireAdminAttribution(t, "CancelSubscriptionAsAdmin", adminID, userID)
	h := newTestHandlers(nil, nil, nil)
	h.adminSubscriptions = &fakeAdminSubscriptionManager{cancel: check}

	w := httptest.NewRecorder()
	h.CancelAdminSubscription(w, adminRequest(t, http.MethodPost, "/admin/users/"+userID.String()+"/subscription/cancel", adminID), userID)

	requireNoContentAndCalled(t, w, called)
}

// TestListAdminSubscriptionTransitions_MapsView proves GET
// /admin/users/{id}/subscription/transitions answers the newest-first history
// with the tariff names and the initiator vocabulary of the contract
// (issue #255).
func TestListAdminSubscriptionTransitions_MapsView(t *testing.T) {
	userID := uuid.Must(uuid.NewV7())
	view := subscriptionTransitionView(t)
	h := newTestHandlers(nil, nil, nil)
	h.adminSubscriptions = &fakeAdminSubscriptionManager{transitions: func(
		_ context.Context, gotUser uuid.UUID,
	) ([]billingapp.SubscriptionTransitionView, error) {
		if gotUser != userID {
			t.Errorf("ListTransitions user = %v, want %v", gotUser, userID)
		}
		return []billingapp.SubscriptionTransitionView{view}, nil
	}}

	w := httptest.NewRecorder()
	h.ListAdminSubscriptionTransitions(w,
		adminRequest(t, http.MethodGet, "/admin/users/"+userID.String()+"/subscription/transitions", uuid.Must(uuid.NewV7())), userID)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp openapi.AdminSubscriptionTransitionsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Total != 1 || len(resp.Items) != 1 {
		t.Fatalf("total/items = %d/%d, want 1/1", resp.Total, len(resp.Items))
	}
	assertAdminTransitionItem(t, resp.Items[0], view)
}
