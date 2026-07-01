package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	identitydomain "github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
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

type testSubscriptionRepo struct {
	subs map[uuid.UUID]domain.Subscription
}

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

type testPaymentMethodRepo struct {
	methods map[uuid.UUID]domain.PaymentMethod
}

func newTestPaymentMethodRepo() *testPaymentMethodRepo {
	return &testPaymentMethodRepo{methods: make(map[uuid.UUID]domain.PaymentMethod)}
}

func (r *testPaymentMethodRepo) Create(_ context.Context, pm domain.PaymentMethod) (domain.PaymentMethod, error) {
	r.methods[pm.ID] = pm
	return pm, nil
}

func (r *testPaymentMethodRepo) UpsertByTokenHash(_ context.Context, pm domain.PaymentMethod) (domain.PaymentMethod, error) {
	for id, existing := range r.methods {
		if existing.UserID == pm.UserID && existing.ProviderToken == pm.ProviderToken {
			existing.ProviderToken = pm.ProviderToken
			existing.ProviderCardID = pm.ProviderCardID
			existing.DisplayMask = pm.DisplayMask
			existing.ExpDate = pm.ExpDate
			existing.UpdatedAt = pm.UpdatedAt
			r.methods[id] = existing
			return existing, nil
		}
	}
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

type testSubscriptionPaymentRepo struct {
	payments map[uuid.UUID]domain.SubscriptionPayment
}

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

func (r *testSubscriptionPaymentRepo) GetByIDAdmin(_ context.Context, id uuid.UUID) (billingapp.SubscriptionPaymentWithUser, error) {
	if p, ok := r.payments[id]; ok {
		return billingapp.SubscriptionPaymentWithUser{Payment: p, UserPhone: ""}, nil
	}
	return billingapp.SubscriptionPaymentWithUser{}, billingapp.ErrNotFound
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

func (r *testSubscriptionPaymentRepo) ListPendingUpgradePayments(_ context.Context, _ time.Time, _ int32) ([]domain.SubscriptionPayment, error) {
	return nil, nil
}

func (r *testSubscriptionPaymentRepo) ListPendingPayments(_ context.Context, _ time.Time, _ int32) ([]domain.SubscriptionPayment, error) {
	return nil, nil
}

func (r *testSubscriptionPaymentRepo) ListAll(_ context.Context, _ string, _ uuid.UUID, _, _ int) ([]billingapp.SubscriptionPaymentWithUser, int64, error) {
	return nil, 0, nil
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

func (r *testSubscriptionPaymentRepo) MarkRefunded(_ context.Context, id uuid.UUID, _ domain.PaymentStatus, amountKopecks int64, now time.Time) error {
	if p, ok := r.payments[id]; ok {
		_ = p.MarkRefunded(amountKopecks, now)
		r.payments[id] = p
	}
	return nil
}

func (r *testSubscriptionPaymentRepo) UpdateProviderPaymentID(_ context.Context, id uuid.UUID, providerPaymentID string) (domain.SubscriptionPayment, error) {
	if p, ok := r.payments[id]; ok {
		p.ProviderPaymentID = &providerPaymentID
		r.payments[id] = p
		return p, nil
	}
	return domain.SubscriptionPayment{}, billingapp.ErrNotFound
}

func (r *testSubscriptionPaymentRepo) UpdatePaymentURL(_ context.Context, id uuid.UUID, paymentURL string) (domain.SubscriptionPayment, error) {
	if p, ok := r.payments[id]; ok {
		p.PaymentURL = &paymentURL
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

func (r *testSubscriptionPaymentRepo) UpdatePaymentMethodID(_ context.Context, id, paymentMethodID uuid.UUID) (domain.SubscriptionPayment, error) {
	if p, ok := r.payments[id]; ok {
		p.PaymentMethodID = &paymentMethodID
		r.payments[id] = p
		return p, nil
	}
	return domain.SubscriptionPayment{}, billingapp.ErrNotFound
}

func (r *testSubscriptionPaymentRepo) WithTx(transaction.Tx) billingapp.SubscriptionPaymentRepository {
	return r
}

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
	webhookResponse []byte

	cancelRes    billingapp.CancelResult
	cancelErr    error
	cancelCalled bool
	cancelReq    billingapp.CancelRequest

	statusRes    domain.PaymentStatus
	statusErr    error
	statusCalled bool
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

func (p *testProvider) Status(_ context.Context, _ uuid.UUID, _ string) (domain.PaymentStatus, error) {
	p.statusCalled = true
	if p.statusErr != nil {
		return domain.PaymentStatusPending, p.statusErr
	}
	if p.statusRes != "" {
		return p.statusRes, nil
	}
	return domain.PaymentStatusSucceeded, nil
}

func (p *testProvider) ParseWebhook(_ context.Context, _ []byte) (billingapp.WebhookPayload, error) {
	p.parseCalled = true
	return billingapp.WebhookPayload{}, nil
}

func (p *testProvider) InitAddCard(_ context.Context, _ billingapp.InitAddCardRequest) (billingapp.InitAddCardResult, error) {
	return billingapp.InitAddCardResult{}, errors.New("test provider: InitAddCard not supported")
}

func (p *testProvider) RemoveCard(_ context.Context, _, _ string) error {
	return nil
}

func (p *testProvider) WebhookResponse() []byte {
	if len(p.webhookResponse) > 0 {
		return p.webhookResponse
	}
	return []byte(`{"status":"ok"}`)
}

func (p *testProvider) ConfirmPayment(_ context.Context, internalPaymentID string) (billingapp.WebhookPayload, error) {
	p.confirmCalledID = internalPaymentID
	return p.confirmResult, p.confirmErr
}

func (p *testProvider) Cancel(_ context.Context, req billingapp.CancelRequest) (billingapp.CancelResult, error) {
	p.cancelCalled = true
	p.cancelReq = req
	return p.cancelRes, p.cancelErr
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
		"http://localhost",
		nil,
	)
	d.handlers = NewSubscriptionHandlers(d.service, slog.New(slog.DiscardHandler), true)
	return d
}

func withUserID(r *http.Request, userID uuid.UUID) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userIDKey{}, userID))
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("expected status %d, got %d: %s", want, rec.Code, rec.Body.String())
	}
}

// --- session fake for server-level auth tests ---

type testSessionRepo struct {
	sessions map[string]identitydomain.Session
	users    map[string]identitydomain.User
}

func newTestSessionRepo() *testSessionRepo {
	return &testSessionRepo{
		sessions: make(map[string]identitydomain.Session),
		users:    make(map[string]identitydomain.User),
	}
}

func (r *testSessionRepo) add(token string, user identitydomain.User, expiresAt time.Time) {
	sum := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(sum[:])
	r.sessions[hash] = identitydomain.Session{
		UserID:    user.ID,
		TokenHash: hash,
		ExpiresAt: expiresAt,
		CreatedAt: expiresAt.Add(-time.Hour),
	}
	r.users[hash] = user
}

func (r *testSessionRepo) Create(_ context.Context, _ identitydomain.Session) error { return nil }
func (r *testSessionRepo) Update(_ context.Context, _ identitydomain.Session) error { return nil }
func (r *testSessionRepo) DeleteByTokenHash(_ context.Context, _ string) error      { return nil }
func (r *testSessionRepo) DeleteByUserID(_ context.Context, _ uuid.UUID) error      { return nil }
func (r *testSessionRepo) DeleteByUserIDExcept(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (r *testSessionRepo) DeleteExpiredBefore(_ context.Context, _ time.Time) error { return nil }
func (r *testSessionRepo) DeleteExpiredBeforeBatch(_ context.Context, _ time.Time, _ int32) (int64, error) {
	return 0, nil
}
func (r *testSessionRepo) WithTx(_ transaction.Tx) identityapp.SessionRepository { return r }
func (r *testSessionRepo) GetByTokenHash(_ context.Context, tokenHash string, _ time.Time) (identitydomain.Session, identitydomain.User, error) {
	session, ok := r.sessions[tokenHash]
	if !ok {
		return identitydomain.Session{}, identitydomain.User{}, identityapp.ErrNotFound
	}
	return session, r.users[tokenHash], nil
}

func newTestServerHandler(t *testing.T, d *handlerTestDeps, sessions identityapp.SessionRepository) http.Handler {
	t.Helper()
	return New(Deps{
		Auth:                  nil,
		Billing:               d.service,
		Sessions:              sessions,
		Properties:            nil,
		AddressSuggester:      nil,
		Leases:                nil,
		TenantContacts:        nil,
		Operations:            nil,
		RecurringOperations:   nil,
		Reminders:             nil,
		AppBaseURL:            "https://example.com",
		CookieSecure:          false,
		Logger:                slog.New(slog.DiscardHandler),
		Clock:                 fakeClock{},
		LogSuccessfulRequests: false,
		IPRateLimiter:         nil,
		PhoneSendLimiter:      nil,
		PhoneVerifyLimiter:    nil,
		DBPoolStats:           nil,
		DevMode:               true,
	})
}

func withSessionCookie(req *http.Request, token string) *http.Request {
	req.AddCookie(&http.Cookie{Name: "session_id", Value: token, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	return req
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
	if resp.Tariff.Name != openapi.Basic {
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

	assertStatus(t, rec, http.StatusOK)

	var resp openapi.AddPaymentMethodResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.ConfirmUrl != nil {
		t.Errorf("expected no confirm url, got %v", resp.ConfirmUrl)
	}
	if resp.PaymentMethod == nil {
		t.Fatal("expected payment method in response")
	}
	if resp.PaymentMethod.Provider != string(domain.ProviderFake) {
		t.Errorf("expected provider fake, got %s", resp.PaymentMethod.Provider)
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

	// The handler now acknowledges the webhook synchronously and processes it in
	// the background, so we wait briefly for the goroutine to call ParseWebhook.
	deadline := time.Now().Add(2 * time.Second)
	for !d.provider.parseCalled {
		if time.Now().After(deadline) {
			t.Errorf("expected provider ParseWebhook to be called")
			return
		}
		time.Sleep(10 * time.Millisecond)
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

func TestSubscriptionHandlers_HandlePaymentWebhook_TkassaResponseBody(t *testing.T) {
	d := newHandlerTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	d.provider.webhookResponse = []byte("OK")

	body := `{"test":"payload"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/webhooks/payment/tkassa", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	d.handlers.HandlePaymentWebhook(rec, req, "tkassa")

	assertStatus(t, rec, http.StatusOK)
	if rec.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Errorf("expected content-type text/plain; charset=utf-8, got %s", rec.Header().Get("Content-Type"))
	}
	if rec.Body.String() != "OK" {
		t.Errorf("expected response body OK, got %q", rec.Body.String())
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

func TestRefundSubscriptionPayment_AdminReturnsNoContent(t *testing.T) {
	d := newHandlerTestDeps(t)
	adminID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaab")
	ownerID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbc")
	basicID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccd")
	proID := uuid.MustParse("dddddddd-dddd-dddd-dddd-ddddddddddde")
	subID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeef")
	paymentID := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	validUntil := testNow.AddDate(0, 1, 0)
	providerPaymentID := "provider_refund_1"

	d.tariffs.add(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5})
	d.tariffs.add(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50})
	d.subscriptions.add(domain.Subscription{
		ID:               subID,
		UserID:           ownerID,
		TariffID:         proID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: true,
	})
	d.subscriptionPayments.add(domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            ownerID,
		SubscriptionID:    subID,
		TariffID:          proID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusSucceeded,
		CreatedAt:         testNow,
		UpdatedAt:         testNow,
	})
	d.provider.cancelRes = billingapp.CancelResult{
		ProviderPaymentID:     providerPaymentID,
		Status:                domain.PaymentStatusRefunded,
		RefundedAmountKopecks: 5000,
	}

	sessions := newTestSessionRepo()
	sessions.add("admin-token", identitydomain.User{ID: adminID, Role: identitydomain.RoleAdmin}, testNow.Add(time.Hour))

	handler := newTestServerHandler(t, d, sessions)
	req := withSessionCookie(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/subscription/payments/"+paymentID.String()+"/refund", nil), "admin-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusNoContent)
	if !d.provider.cancelCalled {
		t.Error("expected provider.Cancel to be called")
	}
	sub, _ := d.subscriptions.GetByUserID(context.Background(), ownerID)
	if sub.TariffID != basicID {
		t.Errorf("expected subscription downgraded to basic, got %s", sub.TariffID)
	}
}

func TestRefundSubscriptionPayment_AdminReturnsNoContentForPending(t *testing.T) {
	d := newHandlerTestDeps(t)
	adminID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaab")
	ownerID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbc")
	basicID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccd")
	proID := uuid.MustParse("dddddddd-dddd-dddd-dddd-ddddddddddde")
	subID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeef")
	paymentID := uuid.MustParse("ffffffff-ffff-ffff-ffff-fffffffffff1")
	providerPaymentID := "provider_refund_pending"

	d.tariffs.add(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5})
	d.tariffs.add(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50})
	d.subscriptions.add(domain.Subscription{
		ID:       subID,
		UserID:   ownerID,
		TariffID: proID,
		Source:   domain.SubscriptionSourcePaid,
		Status:   domain.SubscriptionStatusActive,
	})
	d.subscriptionPayments.add(domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            ownerID,
		SubscriptionID:    subID,
		TariffID:          proID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
		CreatedAt:         testNow,
		UpdatedAt:         testNow,
	})
	d.provider.cancelRes = billingapp.CancelResult{
		ProviderPaymentID:     providerPaymentID,
		Status:                domain.PaymentStatusRefunded,
		RefundedAmountKopecks: 5000,
	}

	sessions := newTestSessionRepo()
	sessions.add("admin-token", identitydomain.User{ID: adminID, Role: identitydomain.RoleAdmin}, testNow.Add(time.Hour))

	handler := newTestServerHandler(t, d, sessions)
	req := withSessionCookie(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/subscription/payments/"+paymentID.String()+"/refund", nil), "admin-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusNoContent)
	sub, _ := d.subscriptions.GetByUserID(context.Background(), ownerID)
	if sub.TariffID != basicID {
		t.Errorf("expected subscription downgraded to basic, got %s", sub.TariffID)
	}
}

func TestRefundSubscriptionPayment_NonAdminForbidden(t *testing.T) {
	d := newHandlerTestDeps(t)
	ownerID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbd")
	paymentID := uuid.MustParse("ffffffff-ffff-ffff-ffff-fffffffffff1")

	sessions := newTestSessionRepo()
	sessions.add("owner-token", identitydomain.User{ID: ownerID, Role: identitydomain.RoleOwner}, testNow.Add(time.Hour))

	handler := newTestServerHandler(t, d, sessions)
	req := withSessionCookie(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/subscription/payments/"+paymentID.String()+"/refund", nil), "owner-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusForbidden)
	if d.provider.cancelCalled {
		t.Error("expected provider.Cancel not to be called for non-admin")
	}
}

func TestRefundSubscriptionPayment_AnonymousUnauthorized(t *testing.T) {
	d := newHandlerTestDeps(t)
	paymentID := uuid.MustParse("ffffffff-ffff-ffff-ffff-fffffffffff2")

	handler := newTestServerHandler(t, d, newTestSessionRepo())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/subscription/payments/"+paymentID.String()+"/refund", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusUnauthorized)
	if d.provider.cancelCalled {
		t.Error("expected provider.Cancel not to be called for anonymous")
	}
}

func TestRefundSubscriptionPayment_AdminPartialAmount(t *testing.T) {
	d := newHandlerTestDeps(t)
	adminID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaac")
	ownerID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbe")
	basicID := uuid.MustParse("cccccccc-cccc-cccc-cccc-ccccccccccce")
	proID := uuid.MustParse("dddddddd-dddd-dddd-dddd-ddddddddddd1")
	subID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeee2")
	paymentID := uuid.MustParse("ffffffff-ffff-ffff-ffff-fffffffffff3")
	validUntil := testNow.AddDate(0, 1, 0)
	providerPaymentID := "provider_refund_partial"

	d.tariffs.add(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5})
	d.tariffs.add(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50})
	d.subscriptions.add(domain.Subscription{
		ID:               subID,
		UserID:           ownerID,
		TariffID:         proID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: true,
	})
	d.subscriptionPayments.add(domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            ownerID,
		SubscriptionID:    subID,
		TariffID:          proID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusSucceeded,
		CreatedAt:         testNow,
		UpdatedAt:         testNow,
	})
	d.provider.cancelRes = billingapp.CancelResult{
		ProviderPaymentID:     providerPaymentID,
		Status:                domain.PaymentStatusPartialRefunded,
		RefundedAmountKopecks: 2000,
	}

	sessions := newTestSessionRepo()
	sessions.add("admin-token", identitydomain.User{ID: adminID, Role: identitydomain.RoleAdmin}, testNow.Add(time.Hour))

	handler := newTestServerHandler(t, d, sessions)
	body := `{"amount_kopecks":2000}`
	req := withSessionCookie(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/subscription/payments/"+paymentID.String()+"/refund", strings.NewReader(body)), "admin-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusNoContent)
	if !d.provider.cancelCalled {
		t.Fatal("expected provider.Cancel to be called")
	}
	if d.provider.cancelReq.AmountKopecks != 2000 {
		t.Errorf("expected cancel amount 2000, got %d", d.provider.cancelReq.AmountKopecks)
	}
	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusPartialRefunded {
		t.Errorf("expected payment partial_refunded, got %s", payment.Status)
	}
}

func TestRefundSubscriptionPayment_AdminPartialAmount_ChunkedBody(t *testing.T) {
	d := newHandlerTestDeps(t)
	adminID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaad")
	ownerID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbf")
	basicID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccf")
	proID := uuid.MustParse("dddddddd-dddd-dddd-dddd-ddddddddddd2")
	subID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeee3")
	paymentID := uuid.MustParse("ffffffff-ffff-ffff-ffff-fffffffffff4")
	validUntil := testNow.AddDate(0, 1, 0)
	providerPaymentID := "provider_refund_chunked"

	d.tariffs.add(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5})
	d.tariffs.add(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50})
	d.subscriptions.add(domain.Subscription{
		ID:               subID,
		UserID:           ownerID,
		TariffID:         proID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: true,
	})
	d.subscriptionPayments.add(domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            ownerID,
		SubscriptionID:    subID,
		TariffID:          proID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusSucceeded,
		CreatedAt:         testNow,
		UpdatedAt:         testNow,
	})
	d.provider.cancelRes = billingapp.CancelResult{
		ProviderPaymentID:     providerPaymentID,
		Status:                domain.PaymentStatusPartialRefunded,
		RefundedAmountKopecks: 2000,
	}

	sessions := newTestSessionRepo()
	sessions.add("admin-token", identitydomain.User{ID: adminID, Role: identitydomain.RoleAdmin}, testNow.Add(time.Hour))

	handler := newTestServerHandler(t, d, sessions)
	body := `{"amount_kopecks":2000}`
	req := withSessionCookie(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/subscription/payments/"+paymentID.String()+"/refund", strings.NewReader(body)), "admin-token")
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = -1
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusNoContent)
	if !d.provider.cancelCalled {
		t.Fatal("expected provider.Cancel to be called for chunked body")
	}
	if d.provider.cancelReq.AmountKopecks != 2000 {
		t.Errorf("expected cancel amount 2000 for chunked body, got %d", d.provider.cancelReq.AmountKopecks)
	}
}

func TestRefundSubscriptionPayment_AdminInvalidBody(t *testing.T) {
	d := newHandlerTestDeps(t)
	adminID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaae")
	paymentID := uuid.MustParse("ffffffff-ffff-ffff-ffff-fffffffffff5")

	sessions := newTestSessionRepo()
	sessions.add("admin-token", identitydomain.User{ID: adminID, Role: identitydomain.RoleAdmin}, testNow.Add(time.Hour))

	handler := newTestServerHandler(t, d, sessions)
	body := `{"amount_kopecks": not_a_number}`
	req := withSessionCookie(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/subscription/payments/"+paymentID.String()+"/refund", strings.NewReader(body)), "admin-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusBadRequest)
	if d.provider.cancelCalled {
		t.Error("expected provider.Cancel not to be called for invalid body")
	}
}

func TestRefundSubscriptionPayment_UnknownPaymentReturnsNotFound(t *testing.T) {
	d := newHandlerTestDeps(t)
	adminID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaf")
	paymentID := uuid.MustParse("ffffffff-ffff-ffff-ffff-fffffffffff6")

	sessions := newTestSessionRepo()
	sessions.add("admin-token", identitydomain.User{ID: adminID, Role: identitydomain.RoleAdmin}, testNow.Add(time.Hour))

	handler := newTestServerHandler(t, d, sessions)
	req := withSessionCookie(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/subscription/payments/"+paymentID.String()+"/refund", nil), "admin-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusNotFound)
	if d.provider.cancelCalled {
		t.Error("expected provider.Cancel not to be called for unknown payment")
	}
}

func TestRefundSubscriptionPayment_AlreadyRefundedReturnsConflict(t *testing.T) {
	d := newHandlerTestDeps(t)
	adminID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaab0")
	ownerID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb0")
	basicID := uuid.MustParse("cccccccc-cccc-cccc-cccc-ccccccccccc0")
	proID := uuid.MustParse("dddddddd-dddd-dddd-dddd-ddddddddddd0")
	subID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeee0")
	paymentID := uuid.MustParse("ffffffff-ffff-ffff-ffff-fffffffffff7")
	validUntil := testNow.AddDate(0, 1, 0)
	providerPaymentID := "provider_refund_conflict"

	d.tariffs.add(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5})
	d.tariffs.add(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50})
	d.subscriptions.add(domain.Subscription{
		ID:               subID,
		UserID:           ownerID,
		TariffID:         proID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: true,
	})
	d.subscriptionPayments.add(domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            ownerID,
		SubscriptionID:    subID,
		TariffID:          proID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusRefunded,
		CreatedAt:         testNow,
		UpdatedAt:         testNow,
	})

	sessions := newTestSessionRepo()
	sessions.add("admin-token", identitydomain.User{ID: adminID, Role: identitydomain.RoleAdmin}, testNow.Add(time.Hour))

	handler := newTestServerHandler(t, d, sessions)
	req := withSessionCookie(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/subscription/payments/"+paymentID.String()+"/refund", nil), "admin-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusConflict)
	if d.provider.cancelCalled {
		t.Error("expected provider.Cancel not to be called for already-refunded payment")
	}
}

func TestRefundSubscriptionPayment_ExcessiveAmountReturnsBadRequest(t *testing.T) {
	d := newHandlerTestDeps(t)
	adminID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaab1")
	ownerID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb1")
	basicID := uuid.MustParse("cccccccc-cccc-cccc-cccc-ccccccccccc1")
	proID := uuid.MustParse("dddddddd-dddd-dddd-dddd-ddddddddddd1")
	subID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeee1")
	paymentID := uuid.MustParse("ffffffff-ffff-ffff-ffff-fffffffffff8")
	validUntil := testNow.AddDate(0, 1, 0)
	providerPaymentID := "provider_refund_excessive"

	d.tariffs.add(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5})
	d.tariffs.add(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50})
	d.subscriptions.add(domain.Subscription{
		ID:               subID,
		UserID:           ownerID,
		TariffID:         proID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: true,
	})
	d.subscriptionPayments.add(domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            ownerID,
		SubscriptionID:    subID,
		TariffID:          proID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusSucceeded,
		CreatedAt:         testNow,
		UpdatedAt:         testNow,
	})

	sessions := newTestSessionRepo()
	sessions.add("admin-token", identitydomain.User{ID: adminID, Role: identitydomain.RoleAdmin}, testNow.Add(time.Hour))

	handler := newTestServerHandler(t, d, sessions)
	body := `{"amount_kopecks":5001}`
	req := withSessionCookie(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/subscription/payments/"+paymentID.String()+"/refund", strings.NewReader(body)), "admin-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusBadRequest)
	if d.provider.cancelCalled {
		t.Error("expected provider.Cancel not to be called for excessive amount")
	}
}

func TestSyncSubscriptionPayment_AdminReturnsNoContent(t *testing.T) {
	d := newHandlerTestDeps(t)
	adminID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaac0")
	ownerID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbc0")
	basicID := uuid.MustParse("cccccccc-cccc-cccc-cccc-ccccccccccc0")
	paymentID := uuid.MustParse("ffffffff-ffff-ffff-ffff-fffffffffff9")
	providerPaymentID := "provider_sync_1"

	d.tariffs.add(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5})
	d.subscriptions.add(domain.Subscription{
		ID:       uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeee9"),
		UserID:   ownerID,
		TariffID: basicID,
		Source:   domain.SubscriptionSourcePaid,
		Status:   domain.SubscriptionStatusActive,
	})
	d.subscriptionPayments.add(domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            ownerID,
		SubscriptionID:    uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeee9"),
		TariffID:          basicID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     1000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
		CreatedAt:         testNow,
		UpdatedAt:         testNow,
	})

	sessions := newTestSessionRepo()
	sessions.add("admin-token", identitydomain.User{ID: adminID, Role: identitydomain.RoleAdmin}, testNow.Add(time.Hour))

	handler := newTestServerHandler(t, d, sessions)
	req := withSessionCookie(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/subscription/payments/"+paymentID.String()+"/sync", nil), "admin-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusNoContent)
	if !d.provider.statusCalled {
		t.Error("expected provider.Status to be called")
	}
}

func TestSyncSubscriptionPayment_AlreadyFinalizedReturnsConflict(t *testing.T) {
	d := newHandlerTestDeps(t)
	adminID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaac1")
	ownerID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbc1")
	basicID := uuid.MustParse("cccccccc-cccc-cccc-cccc-ccccccccccc1")
	paymentID := uuid.MustParse("ffffffff-ffff-ffff-ffff-fffffffffffa")
	providerPaymentID := "provider_sync_finalized"

	d.tariffs.add(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5})
	d.subscriptions.add(domain.Subscription{
		ID:       uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeea"),
		UserID:   ownerID,
		TariffID: basicID,
		Source:   domain.SubscriptionSourcePaid,
		Status:   domain.SubscriptionStatusActive,
	})
	d.subscriptionPayments.add(domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            ownerID,
		SubscriptionID:    uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeea"),
		TariffID:          basicID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     1000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusSucceeded,
		CreatedAt:         testNow,
		UpdatedAt:         testNow,
	})

	sessions := newTestSessionRepo()
	sessions.add("admin-token", identitydomain.User{ID: adminID, Role: identitydomain.RoleAdmin}, testNow.Add(time.Hour))

	handler := newTestServerHandler(t, d, sessions)
	req := withSessionCookie(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/subscription/payments/"+paymentID.String()+"/sync", nil), "admin-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusConflict)
	if d.provider.statusCalled {
		t.Error("expected provider.Status not to be called for finalized payment")
	}
}

func TestSyncSubscriptionPayment_MissingProviderIDReturnsConflict(t *testing.T) {
	d := newHandlerTestDeps(t)
	adminID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaac2")
	ownerID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbc2")
	basicID := uuid.MustParse("cccccccc-cccc-cccc-cccc-ccccccccccc2")
	paymentID := uuid.MustParse("ffffffff-ffff-ffff-ffff-fffffffffffb")

	d.tariffs.add(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5})
	d.subscriptions.add(domain.Subscription{
		ID:       uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeeb"),
		UserID:   ownerID,
		TariffID: basicID,
		Source:   domain.SubscriptionSourcePaid,
		Status:   domain.SubscriptionStatusActive,
	})
	d.subscriptionPayments.add(domain.SubscriptionPayment{
		ID:             paymentID,
		UserID:         ownerID,
		SubscriptionID: uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeeb"),
		TariffID:       basicID,
		Period:         domain.PeriodMonth,
		AmountKopecks:  1000,
		Provider:       domain.ProviderFake,
		Status:         domain.PaymentStatusPending,
		CreatedAt:      testNow,
		UpdatedAt:      testNow,
	})

	sessions := newTestSessionRepo()
	sessions.add("admin-token", identitydomain.User{ID: adminID, Role: identitydomain.RoleAdmin}, testNow.Add(time.Hour))

	handler := newTestServerHandler(t, d, sessions)
	req := withSessionCookie(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/subscription/payments/"+paymentID.String()+"/sync", nil), "admin-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusConflict)
	if d.provider.statusCalled {
		t.Error("expected provider.Status not to be called when provider payment id is missing")
	}
}

func TestSyncSubscriptionPayment_NotFoundReturnsNotFound(t *testing.T) {
	d := newHandlerTestDeps(t)
	adminID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaac3")
	paymentID := uuid.MustParse("ffffffff-ffff-ffff-ffff-fffffffffffc")

	sessions := newTestSessionRepo()
	sessions.add("admin-token", identitydomain.User{ID: adminID, Role: identitydomain.RoleAdmin}, testNow.Add(time.Hour))

	handler := newTestServerHandler(t, d, sessions)
	req := withSessionCookie(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/subscription/payments/"+paymentID.String()+"/sync", nil), "admin-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusNotFound)
	if d.provider.statusCalled {
		t.Error("expected provider.Status not to be called for unknown payment")
	}
}

func TestSyncSubscriptionPayment_NonAdminForbidden(t *testing.T) {
	d := newHandlerTestDeps(t)
	ownerID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbc4")
	paymentID := uuid.MustParse("ffffffff-ffff-ffff-ffff-fffffffffffd")

	sessions := newTestSessionRepo()
	sessions.add("owner-token", identitydomain.User{ID: ownerID, Role: identitydomain.RoleOwner}, testNow.Add(time.Hour))

	handler := newTestServerHandler(t, d, sessions)
	req := withSessionCookie(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/subscription/payments/"+paymentID.String()+"/sync", nil), "owner-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusForbidden)
	if d.provider.statusCalled {
		t.Error("expected provider.Status not to be called for non-admin")
	}
}

func TestSyncSubscriptionPayment_AnonymousUnauthorized(t *testing.T) {
	d := newHandlerTestDeps(t)
	paymentID := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")

	handler := newTestServerHandler(t, d, newTestSessionRepo())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/subscription/payments/"+paymentID.String()+"/sync", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusUnauthorized)
	if d.provider.statusCalled {
		t.Error("expected provider.Status not to be called for anonymous")
	}
}
