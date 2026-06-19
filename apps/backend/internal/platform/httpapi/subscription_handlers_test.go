package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

var testNow = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

type fakeClock struct{}

func (fakeClock) Now() time.Time { return testNow }

// --- transaction fake ---

type testTx struct{}

func (testTx) Commit(context.Context) error   { return nil }
func (testTx) Rollback(context.Context) error { return nil }

type testBeginner struct{}

func (testBeginner) Begin(context.Context) (transaction.Tx, error) { return testTx{}, nil }

// --- repository fakes ---

type testTariffRepo struct {
	byID   map[uuid.UUID]domain.Tariff
	byName map[domain.TariffName]domain.Tariff
	list   []domain.Tariff
}

func newTestTariffRepo() *testTariffRepo {
	return &testTariffRepo{
		byID:   make(map[uuid.UUID]domain.Tariff),
		byName: make(map[domain.TariffName]domain.Tariff),
	}
}

func (r *testTariffRepo) add(t domain.Tariff) {
	r.byID[t.ID] = t
	r.byName[t.Name] = t
	r.list = append(r.list, t)
}

func (r *testTariffRepo) GetByID(_ context.Context, id uuid.UUID) (domain.Tariff, error) {
	if t, ok := r.byID[id]; ok {
		return t, nil
	}
	return domain.Tariff{}, billingapp.ErrNotFound
}

func (r *testTariffRepo) GetByName(_ context.Context, name domain.TariffName) (domain.Tariff, error) {
	if t, ok := r.byName[name]; ok {
		return t, nil
	}
	return domain.Tariff{}, billingapp.ErrNotFound
}

func (r *testTariffRepo) List(_ context.Context) ([]domain.Tariff, error) {
	return append([]domain.Tariff(nil), r.list...), nil
}

func (r *testTariffRepo) WithTx(transaction.Tx) billingapp.TariffRepository { return r }

type testSubscriptionRepo struct{ subs map[uuid.UUID]domain.Subscription }

func newTestSubscriptionRepo() *testSubscriptionRepo {
	return &testSubscriptionRepo{subs: make(map[uuid.UUID]domain.Subscription)}
}

func (r *testSubscriptionRepo) add(sub domain.Subscription) { r.subs[sub.UserID] = sub }

func (r *testSubscriptionRepo) GetByID(_ context.Context, id uuid.UUID) (domain.Subscription, error) {
	for _, s := range r.subs {
		if s.ID == id {
			return s, nil
		}
	}
	return domain.Subscription{}, billingapp.ErrNotFound
}

func (r *testSubscriptionRepo) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Subscription, error) {
	return r.GetByID(ctx, id)
}

func (r *testSubscriptionRepo) GetByUserID(_ context.Context, userID uuid.UUID) (domain.Subscription, error) {
	if s, ok := r.subs[userID]; ok {
		return s, nil
	}
	return domain.Subscription{}, billingapp.ErrNotFound
}

func (r *testSubscriptionRepo) GetByUserIDForUpdate(ctx context.Context, userID uuid.UUID) (domain.Subscription, error) {
	return r.GetByUserID(ctx, userID)
}

func (r *testSubscriptionRepo) Create(_ context.Context, sub domain.Subscription) (domain.Subscription, error) {
	r.subs[sub.UserID] = sub
	return sub, nil
}

func (r *testSubscriptionRepo) Update(_ context.Context, sub domain.Subscription) error {
	r.subs[sub.UserID] = sub
	return nil
}

func (r *testSubscriptionRepo) ListUpForRenewal(_ context.Context, _ time.Time, _ int32) ([]domain.Subscription, error) {
	return nil, nil
}

func (r *testSubscriptionRepo) ListInExpiredGrace(_ context.Context, _ time.Time, _ int32) ([]domain.Subscription, error) {
	return nil, nil
}

func (r *testSubscriptionRepo) ListExpiredNonRenewing(_ context.Context, _ time.Time, _ int32) ([]domain.Subscription, error) {
	return nil, nil
}

func (r *testSubscriptionRepo) ListExpiredCancelled(_ context.Context, _ time.Time, _ int32) ([]domain.Subscription, error) {
	return nil, nil
}

func (r *testSubscriptionRepo) ListPendingChanges(_ context.Context, _ time.Time, _ int32) ([]domain.Subscription, error) {
	return nil, nil
}

func (r *testSubscriptionRepo) WithTx(transaction.Tx) billingapp.SubscriptionRepository { return r }

type testPaymentMethodRepo struct{ methods map[uuid.UUID]domain.PaymentMethod }

func newTestPaymentMethodRepo() *testPaymentMethodRepo {
	return &testPaymentMethodRepo{methods: make(map[uuid.UUID]domain.PaymentMethod)}
}

func (r *testPaymentMethodRepo) Create(_ context.Context, pm domain.PaymentMethod) (domain.PaymentMethod, error) {
	r.methods[pm.ID] = pm
	return pm, nil
}

func (r *testPaymentMethodRepo) GetByID(_ context.Context, id uuid.UUID) (domain.PaymentMethod, error) {
	if pm, ok := r.methods[id]; ok {
		return pm, nil
	}
	return domain.PaymentMethod{}, billingapp.ErrNotFound
}

func (r *testPaymentMethodRepo) ListByUserID(_ context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error) {
	var list []domain.PaymentMethod
	for _, pm := range r.methods {
		if pm.UserID == userID {
			list = append(list, pm)
		}
	}
	return list, nil
}

func (r *testPaymentMethodRepo) SetActive(_ context.Context, userID, methodID uuid.UUID) error {
	for id, pm := range r.methods {
		if pm.UserID == userID {
			pm.IsActive = id == methodID
			r.methods[id] = pm
		}
	}
	return nil
}

func (r *testPaymentMethodRepo) Delete(_ context.Context, userID, methodID uuid.UUID) error {
	for id, pm := range r.methods {
		if pm.UserID == userID && id == methodID {
			if pm.IsActive {
				return billingapp.ErrPaymentMethodInUse
			}
			delete(r.methods, id)
			return nil
		}
	}
	return billingapp.ErrNotFound
}

func (r *testPaymentMethodRepo) WithTx(transaction.Tx) billingapp.PaymentMethodRepository { return r }

type testSubscriptionPaymentRepo struct{ payments map[uuid.UUID]domain.SubscriptionPayment }

func newTestSubscriptionPaymentRepo() *testSubscriptionPaymentRepo {
	return &testSubscriptionPaymentRepo{payments: make(map[uuid.UUID]domain.SubscriptionPayment)}
}

func (r *testSubscriptionPaymentRepo) add(p domain.SubscriptionPayment) { r.payments[p.ID] = p }

func (r *testSubscriptionPaymentRepo) Create(_ context.Context, p domain.SubscriptionPayment) (domain.SubscriptionPayment, error) {
	r.payments[p.ID] = p
	return p, nil
}

func (r *testSubscriptionPaymentRepo) GetByID(_ context.Context, id uuid.UUID) (domain.SubscriptionPayment, error) {
	if p, ok := r.payments[id]; ok {
		return p, nil
	}
	return domain.SubscriptionPayment{}, billingapp.ErrNotFound
}

func (r *testSubscriptionPaymentRepo) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.SubscriptionPayment, error) {
	return r.GetByID(ctx, id)
}

func (r *testSubscriptionPaymentRepo) ListByUserID(_ context.Context, userID uuid.UUID) ([]domain.SubscriptionPayment, error) {
	var list []domain.SubscriptionPayment
	for _, p := range r.payments {
		if p.UserID == userID {
			list = append(list, p)
		}
	}
	return list, nil
}

func (r *testSubscriptionPaymentRepo) ListPendingSubscriptionPaymentsByUserID(_ context.Context, userID uuid.UUID) ([]domain.SubscriptionPayment, error) {
	var list []domain.SubscriptionPayment
	for _, p := range r.payments {
		if p.UserID == userID && p.Status == domain.PaymentStatusPending {
			list = append(list, p)
		}
	}
	return list, nil
}

func (r *testSubscriptionPaymentRepo) GetLastSucceededBySubscriptionID(_ context.Context, subscriptionID uuid.UUID) (domain.SubscriptionPayment, error) {
	var latest *domain.SubscriptionPayment
	for _, p := range r.payments {
		if p.SubscriptionID == subscriptionID && p.Status == domain.PaymentStatusSucceeded {
			if latest == nil || p.CreatedAt.After(latest.CreatedAt) {
				payment := p
				latest = &payment
			}
		}
	}
	if latest == nil {
		return domain.SubscriptionPayment{}, billingapp.ErrNotFound
	}
	return *latest, nil
}

func (r *testSubscriptionPaymentRepo) MarkSucceeded(_ context.Context, id uuid.UUID, now time.Time) error {
	if p, ok := r.payments[id]; ok {
		_ = p.MarkSucceeded(now)
		r.payments[id] = p
		return nil
	}
	return billingapp.ErrNotFound
}

func (r *testSubscriptionPaymentRepo) MarkFailed(_ context.Context, id uuid.UUID, errorCode *string, now time.Time) error {
	if p, ok := r.payments[id]; ok {
		_ = p.MarkFailed(errorCode, now)
		r.payments[id] = p
		return nil
	}
	return billingapp.ErrNotFound
}

func (r *testSubscriptionPaymentRepo) UpdateProviderPaymentID(_ context.Context, id uuid.UUID, providerPaymentID string) (domain.SubscriptionPayment, error) {
	if p, ok := r.payments[id]; ok {
		p.ProviderPaymentID = &providerPaymentID
		r.payments[id] = p
		return p, nil
	}
	return domain.SubscriptionPayment{}, billingapp.ErrNotFound
}

func (r *testSubscriptionPaymentRepo) UpdatePaymentMethodAndProviderID(_ context.Context, id, paymentMethodID uuid.UUID, providerPaymentID string) (domain.SubscriptionPayment, error) {
	if p, ok := r.payments[id]; ok {
		p.PaymentMethodID = &paymentMethodID
		p.ProviderPaymentID = &providerPaymentID
		r.payments[id] = p
		return p, nil
	}
	return domain.SubscriptionPayment{}, billingapp.ErrNotFound
}

func (r *testSubscriptionPaymentRepo) WithTx(transaction.Tx) billingapp.SubscriptionPaymentRepository { return r }

// --- provider stub ---

type testProvider struct {
	name            domain.PaymentProvider
	initURL         string
	initToken       string
	initErr         error
	confirmResult   billingapp.WebhookPayload
	confirmErr      error
	confirmCalledID string
	parseCalled     bool
}

func (p *testProvider) Name() domain.PaymentProvider { return p.name }

func (p *testProvider) Init(_ context.Context, req billingapp.InitRequest) (billingapp.InitResult, error) {
	if p.initErr != nil {
		return billingapp.InitResult{}, p.initErr
	}
	return billingapp.InitResult{
		ProviderPaymentID: "provider_" + req.PaymentID.String(),
		PaymentURL:        p.initURL,
		SavedToken:        p.initToken,
		Status:            domain.PaymentStatusPending,
	}, nil
}

func (p *testProvider) Charge(_ context.Context, _ billingapp.ChargeRequest) (billingapp.ChargeResult, error) {
	return billingapp.ChargeResult{}, nil
}

func (p *testProvider) ParseWebhook(_ context.Context, _ []byte) (billingapp.WebhookPayload, error) {
	p.parseCalled = true
	return billingapp.WebhookPayload{}, nil
}

func (p *testProvider) ConfirmPayment(_ context.Context, internalPaymentID string) (billingapp.WebhookPayload, error) {
	p.confirmCalledID = internalPaymentID
	return p.confirmResult, p.confirmErr
}

// --- test helpers ---

type handlerTestDeps struct {
	tariffs              *testTariffRepo
	subscriptions        *testSubscriptionRepo
	paymentMethods       *testPaymentMethodRepo
	subscriptionPayments *testSubscriptionPaymentRepo
	provider             *testProvider
	service              *billingapp.BillingService
	handlers             *SubscriptionHandlers
}

func newHandlerTestDeps(t *testing.T) *handlerTestDeps {
	t.Helper()
	d := &handlerTestDeps{
		tariffs:              newTestTariffRepo(),
		subscriptions:        newTestSubscriptionRepo(),
		paymentMethods:       newTestPaymentMethodRepo(),
		subscriptionPayments: newTestSubscriptionPaymentRepo(),
		provider: &testProvider{
			name:      domain.ProviderFake,
			initURL:   "http://localhost/internal/fake-subscription-payment/test/confirm",
			initToken: "fake_token_1234",
		},
	}
	d.service = billingapp.NewBillingService(
		d.tariffs,
		d.subscriptions,
		d.paymentMethods,
		d.subscriptionPayments,
		d.provider,
		testBeginner{},
		fakeClock{},
		nil,
		nil,
	)
	d.handlers = NewSubscriptionHandlers(d.service, slog.New(slog.DiscardHandler), true)
	return d
}

func withUserID(r *http.Request, userID uuid.UUID) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userIDKey, userID))
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("expected status %d, got %d: %s", want, rec.Code, rec.Body.String())
	}
}

func TestSubscriptionHandlers_GetSubscription(t *testing.T) {
	d := newHandlerTestDeps(t)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	basicID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	d.tariffs.add(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.subscriptions.add(domain.Subscription{
		ID:               uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
		UserID:           userID,
		TariffID:         basicID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		AutoRenewEnabled: false,
	})

	req := withUserID(httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/subscription", nil), userID)
	rec := httptest.NewRecorder()
	d.handlers.GetSubscription(rec, req)

	assertStatus(t, rec, http.StatusOK)

	var resp openapi.Subscription
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Tariff.Name != openapi.TariffNameBasic {
		t.Errorf("expected tariff basic, got %s", resp.Tariff.Name)
	}
	if resp.Status != openapi.SubscriptionStatusActive {
		t.Errorf("expected status active, got %s", resp.Status)
	}
}

func TestSubscriptionHandlers_ChangeTariff(t *testing.T) {
	d := newHandlerTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	validUntil := testNow.AddDate(0, 1, 0)

	d.tariffs.add(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.tariffs.add(domain.Tariff{
		ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.subscriptions.add(domain.Subscription{
		ID:         uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		UserID:     userID,
		TariffID:   basicID,
		Source:     domain.SubscriptionSourcePaid,
		Status:     domain.SubscriptionStatusActive,
		ValidUntil: &validUntil,
	})

	body := `{"tariffName":"pro","period":"month"}`
	req := withUserID(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/subscription/change", strings.NewReader(body)), userID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	d.handlers.ChangeTariff(rec, req)

	assertStatus(t, rec, http.StatusOK)

	var resp openapi.ChangeTariffResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.PaymentId == nil {
		t.Fatal("expected payment id in response")
	}
	if resp.ConfirmUrl == nil || *resp.ConfirmUrl == "" {
		t.Fatal("expected confirm url in response")
	}
}

func TestSubscriptionHandlers_ConfirmFakeSubscriptionPayment(t *testing.T) {
	d := newHandlerTestDeps(t)
	userID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	basicID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	proID := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	subID := uuid.MustParse("88888888-8888-8888-8888-888888888888")
	paymentID := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	validUntil := testNow.AddDate(0, 1, 0)

	d.tariffs.add(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.tariffs.add(domain.Tariff{
		ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.subscriptions.add(domain.Subscription{
		ID:         subID,
		UserID:     userID,
		TariffID:   basicID,
		Source:     domain.SubscriptionSourcePaid,
		Status:     domain.SubscriptionStatusActive,
		ValidUntil: &validUntil,
	})
	d.subscriptionPayments.add(domain.SubscriptionPayment{
		ID:             paymentID,
		UserID:         userID,
		SubscriptionID: subID,
		TariffID:       proID,
		Period:         domain.PeriodMonth,
		AmountKopecks:  5000,
		Provider:       domain.ProviderFake,
		Status:         domain.PaymentStatusPending,
		CreatedAt:      testNow,
		UpdatedAt:      testNow,
	})
	d.provider.confirmResult = billingapp.WebhookPayload{
		InternalPaymentID: paymentID,
		Status:            domain.PaymentStatusSucceeded,
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/internal/fake-subscription-payment/"+paymentID.String()+"/confirm", nil)
	rec := httptest.NewRecorder()
	d.handlers.ConfirmFakeSubscriptionPayment(rec, req, paymentID)

	assertStatus(t, rec, http.StatusOK)

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %v", resp["status"])
	}

	if d.provider.confirmCalledID != paymentID.String() {
		t.Errorf("expected confirm called with %s, got %s", paymentID.String(), d.provider.confirmCalledID)
	}

	sub, _ := d.subscriptions.GetByUserID(context.Background(), userID)
	if sub.TariffID != proID {
		t.Errorf("expected subscription tariff upgraded to pro, got %s", sub.TariffID)
	}
}


func TestSubscriptionHandlers_ListTariffs(t *testing.T) {
	d := newHandlerTestDeps(t)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	d.tariffs.add(domain.Tariff{
		ID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Name: domain.TariffBasic,
		ActivePropertyLimit: 5, MonthlyPriceKopecks: 0, YearlyPriceKopecks: 0,
	})
	d.tariffs.add(domain.Tariff{
		ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Name: domain.TariffPro,
		ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000, YearlyPriceKopecks: 50000,
	})

	req := withUserID(httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/tariffs", nil), userID)
	rec := httptest.NewRecorder()
	d.handlers.ListTariffs(rec, req)

	assertStatus(t, rec, http.StatusOK)

	var resp openapi.TariffsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 tariffs, got %d", len(resp.Items))
	}
}

func TestSubscriptionHandlers_ToggleAutoRenew(t *testing.T) {
	d := newHandlerTestDeps(t)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	basicID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	validUntil := testNow.AddDate(0, 1, 0)

	d.tariffs.add(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5})
	d.subscriptions.add(domain.Subscription{
		ID:               uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
		UserID:           userID,
		TariffID:         basicID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	})

	body := `{"enabled":true}`
	req := withUserID(httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/subscription/auto-renew", strings.NewReader(body)), userID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	d.handlers.ToggleAutoRenew(rec, req)

	assertStatus(t, rec, http.StatusNoContent)

	sub, _ := d.subscriptions.GetByUserID(context.Background(), userID)
	if !sub.AutoRenewEnabled {
		t.Errorf("expected auto-renew enabled")
	}
}

func TestSubscriptionHandlers_ListSubscriptionPayments(t *testing.T) {
	d := newHandlerTestDeps(t)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	basicID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	d.tariffs.add(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5})
	d.subscriptionPayments.add(domain.SubscriptionPayment{
		ID:            uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		UserID:        userID,
		TariffID:      basicID,
		Period:        domain.PeriodMonth,
		AmountKopecks: 1000,
		Provider:      domain.ProviderFake,
		Status:        domain.PaymentStatusSucceeded,
		CreatedAt:     testNow,
		UpdatedAt:     testNow,
	})

	req := withUserID(httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/subscription/payments", nil), userID)
	rec := httptest.NewRecorder()
	d.handlers.ListSubscriptionPayments(rec, req)

	assertStatus(t, rec, http.StatusOK)

	var resp openapi.SubscriptionPaymentsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 payment, got %d", len(resp.Items))
	}
}

func TestSubscriptionHandlers_ListPaymentMethods(t *testing.T) {
	d := newHandlerTestDeps(t)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	d.paymentMethods.methods[uuid.MustParse("11111111-1111-1111-1111-111111111111")] = domain.PaymentMethod{
		ID:          uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		UserID:      userID,
		Provider:    domain.ProviderFake,
		DisplayMask: "****1234",
		IsActive:    true,
	}

	req := withUserID(httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/subscription/payment-methods", nil), userID)
	rec := httptest.NewRecorder()
	d.handlers.ListPaymentMethods(rec, req)

	assertStatus(t, rec, http.StatusOK)

	var resp openapi.PaymentMethodsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 payment method, got %d", len(resp.Items))
	}
}

func TestSubscriptionHandlers_AddPaymentMethod(t *testing.T) {
	d := newHandlerTestDeps(t)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	body := `{"providerToken":"fake_token_1234"}`
	req := withUserID(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/subscription/payment-methods", strings.NewReader(body)), userID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	d.handlers.AddPaymentMethod(rec, req)

	assertStatus(t, rec, http.StatusCreated)

	var resp openapi.PaymentMethod
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Provider != string(domain.ProviderFake) {
		t.Errorf("expected provider fake, got %s", resp.Provider)
	}
}

func TestSubscriptionHandlers_DeletePaymentMethod(t *testing.T) {
	d := newHandlerTestDeps(t)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	methodID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID: methodID, UserID: userID, Provider: domain.ProviderFake, IsActive: false,
	}

	req := withUserID(httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/subscription/payment-methods/"+methodID.String(), nil), userID)
	rec := httptest.NewRecorder()
	d.handlers.DeletePaymentMethod(rec, req, methodID)

	assertStatus(t, rec, http.StatusNoContent)
}

func TestSubscriptionHandlers_DeletePaymentMethod_Active(t *testing.T) {
	d := newHandlerTestDeps(t)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	methodID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID: methodID, UserID: userID, Provider: domain.ProviderFake, IsActive: true,
	}

	req := withUserID(httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/subscription/payment-methods/"+methodID.String(), nil), userID)
	rec := httptest.NewRecorder()
	d.handlers.DeletePaymentMethod(rec, req, methodID)

	assertStatus(t, rec, http.StatusConflict)
}

func TestSubscriptionHandlers_ActivatePaymentMethod(t *testing.T) {
	d := newHandlerTestDeps(t)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	methodID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	d.tariffs.add(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5})
	d.subscriptions.add(domain.Subscription{
		ID:       uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
		UserID:   userID,
		TariffID: basicID,
		Source:   domain.SubscriptionSourcePaid,
		Status:   domain.SubscriptionStatusActive,
	})
	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID: methodID, UserID: userID, Provider: domain.ProviderFake, IsActive: false,
	}

	req := withUserID(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/subscription/payment-methods/"+methodID.String()+"/activate", nil), userID)
	rec := httptest.NewRecorder()
	d.handlers.ActivatePaymentMethod(rec, req, methodID)

	assertStatus(t, rec, http.StatusNoContent)

	pm, _ := d.paymentMethods.GetByID(context.Background(), methodID)
	if !pm.IsActive {
		t.Errorf("expected method active")
	}

	sub, _ := d.subscriptions.GetByUserID(context.Background(), userID)
	if sub.ActivePaymentMethodID == nil || *sub.ActivePaymentMethodID != methodID {
		t.Errorf("expected subscription active payment method %s, got %v", methodID, sub.ActivePaymentMethodID)
	}
}

func TestSubscriptionHandlers_HandlePaymentWebhook(t *testing.T) {
	d := newHandlerTestDeps(t)

	body := `{"test":"payload"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/webhooks/payment/fake", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	d.handlers.HandlePaymentWebhook(rec, req, "fake")

	assertStatus(t, rec, http.StatusOK)
	if !d.provider.parseCalled {
		t.Errorf("expected provider ParseWebhook to be called")
	}
}

func TestSubscriptionHandlers_HandlePaymentWebhook_Oversized(t *testing.T) {
	d := newHandlerTestDeps(t)

	body := bytes.Repeat([]byte("x"), maxWebhookBody+1)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/webhooks/payment/fake", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	d.handlers.HandlePaymentWebhook(rec, req, "fake")

	assertStatus(t, rec, http.StatusOK)
	if d.provider.parseCalled {
		t.Errorf("expected provider ParseWebhook not to be called for oversized body")
	}
}

func TestSubscriptionHandlers_ConfirmFakeSubscriptionPayment_NotDevMode(t *testing.T) {
	d := newHandlerTestDeps(t)
	d.handlers.DevMode = false
	paymentID := uuid.MustParse("99999999-9999-9999-9999-999999999999")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/internal/fake-subscription-payment/"+paymentID.String()+"/confirm", nil)
	rec := httptest.NewRecorder()
	d.handlers.ConfirmFakeSubscriptionPayment(rec, req, paymentID)

	assertStatus(t, rec, http.StatusNotFound)
}

func TestSubscriptionHandlers_ChangeTariff_AlreadyOnTariff(t *testing.T) {
	d := newHandlerTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	d.tariffs.add(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5})
	d.subscriptions.add(domain.Subscription{
		ID:       uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		UserID:   userID,
		TariffID: basicID,
		Source:   domain.SubscriptionSourcePaid,
		Status:   domain.SubscriptionStatusActive,
	})

	body := `{"tariffName":"basic","period":"month"}`
	req := withUserID(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/subscription/change", strings.NewReader(body)), userID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	d.handlers.ChangeTariff(rec, req)

	assertStatus(t, rec, http.StatusConflict)
}
