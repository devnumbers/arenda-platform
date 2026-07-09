package application

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

var fixedNow = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

type fakeClock struct{ now time.Time }

func newFakeClock(t time.Time) *fakeClock { return &fakeClock{now: t} }
func (c *fakeClock) Now() time.Time       { return c.now }

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

// --- transaction fake ---

type fakeTx struct {
	b    *fakeBeginner
	done bool
}

func (tx *fakeTx) Commit(context.Context) error {
	if !tx.done {
		tx.b.committed++
		tx.b.open--
		tx.done = true
	}
	return nil
}

func (tx *fakeTx) Rollback(context.Context) error {
	if !tx.done {
		tx.b.rolledBack++
		tx.b.open--
		tx.done = true
	}
	return nil
}

type fakeBeginner struct {
	begun, committed, rolledBack, open int
}

func (b *fakeBeginner) Begin(context.Context) (transaction.Tx, error) {
	b.begun++
	b.open++
	return &fakeTx{b: b}, nil
}

// --- repository fakes ---

type fakeTariffRepo struct {
	byName           map[domain.TariffName]domain.Tariff
	list             []domain.Tariff
	failGetByIDCount int
	failGetByIDFor   *uuid.UUID
	failGetByIDErr   error
}

func (r *fakeTariffRepo) GetByID(_ context.Context, id uuid.UUID) (domain.Tariff, error) {
	if r.failGetByIDFor != nil && *r.failGetByIDFor == id {
		if r.failGetByIDErr != nil {
			return domain.Tariff{}, r.failGetByIDErr
		}
		return domain.Tariff{}, ErrNotFound
	}
	if r.failGetByIDCount > 0 {
		r.failGetByIDCount--
		if r.failGetByIDErr != nil {
			return domain.Tariff{}, r.failGetByIDErr
		}
		return domain.Tariff{}, ErrNotFound
	}
	for _, t := range r.byName {
		if t.ID == id {
			return t, nil
		}
	}
	return domain.Tariff{}, ErrNotFound
}

func (r *fakeTariffRepo) GetByName(_ context.Context, name domain.TariffName) (domain.Tariff, error) {
	t, ok := r.byName[name]
	if !ok {
		return domain.Tariff{}, ErrNotFound
	}
	return t, nil
}

func (r *fakeTariffRepo) List(_ context.Context) ([]domain.Tariff, error) {
	out := append([]domain.Tariff(nil), r.list...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].MonthlyPriceKopecks != out[j].MonthlyPriceKopecks {
			return out[i].MonthlyPriceKopecks < out[j].MonthlyPriceKopecks
		}
		return out[i].ID.String() < out[j].ID.String()
	})
	return out, nil
}

func (r *fakeTariffRepo) WithTx(transaction.Tx) TariffRepository { return r }

type fakeSubscriptionRepo struct {
	subs map[uuid.UUID]domain.Subscription
}

func (r *fakeSubscriptionRepo) GetByID(_ context.Context, id uuid.UUID) (domain.Subscription, error) {
	for _, sub := range r.subs {
		if sub.ID == id {
			return sub, nil
		}
	}
	return domain.Subscription{}, ErrNotFound
}

func (r *fakeSubscriptionRepo) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Subscription, error) {
	return r.GetByID(ctx, id)
}

func (r *fakeSubscriptionRepo) GetByUserID(_ context.Context, userID uuid.UUID) (domain.Subscription, error) {
	sub, ok := r.subs[userID]
	if !ok {
		return domain.Subscription{}, ErrNotFound
	}
	return sub, nil
}

func (r *fakeSubscriptionRepo) GetByUserIDForUpdate(ctx context.Context, userID uuid.UUID) (domain.Subscription, error) {
	return r.GetByUserID(ctx, userID)
}

func (r *fakeSubscriptionRepo) Create(_ context.Context, sub domain.Subscription) (domain.Subscription, error) {
	r.subs[sub.UserID] = sub
	return sub, nil
}

func (r *fakeSubscriptionRepo) Update(_ context.Context, sub domain.Subscription) error {
	r.subs[sub.UserID] = sub
	return nil
}

func (r *fakeSubscriptionRepo) ListUpForRenewal(_ context.Context, now time.Time, limit int32) ([]domain.Subscription, error) {
	var out []domain.Subscription
	for _, sub := range r.subs {
		if sub.Status == domain.SubscriptionStatusActive && sub.AutoRenewEnabled && sub.ValidUntil != nil && !sub.ValidUntil.After(now) {
			out = append(out, sub)
		}
	}
	if len(out) > int(limit) {
		out = out[:limit]
	}
	return out, nil
}

func (r *fakeSubscriptionRepo) ListInExpiredGrace(_ context.Context, now time.Time, limit int32) ([]domain.Subscription, error) {
	var out []domain.Subscription
	for _, sub := range r.subs {
		if sub.Status == domain.SubscriptionStatusGrace && sub.ValidUntil != nil && !sub.ValidUntil.After(now) {
			out = append(out, sub)
		}
	}
	if len(out) > int(limit) {
		out = out[:limit]
	}
	return out, nil
}

func (r *fakeSubscriptionRepo) ListExpiredNonRenewing(_ context.Context, now time.Time, limit int32) ([]domain.Subscription, error) {
	var out []domain.Subscription
	for _, sub := range r.subs {
		if sub.Status == domain.SubscriptionStatusActive && !sub.AutoRenewEnabled && sub.ValidUntil != nil && !sub.ValidUntil.After(now) {
			out = append(out, sub)
		}
	}
	if len(out) > int(limit) {
		out = out[:limit]
	}
	return out, nil
}

func (r *fakeSubscriptionRepo) ListExpiredCancelled(_ context.Context, now time.Time, limit int32) ([]domain.Subscription, error) {
	var out []domain.Subscription
	for _, sub := range r.subs {
		if sub.Status == domain.SubscriptionStatusCancelled && sub.ValidUntil != nil && !sub.ValidUntil.After(now) {
			out = append(out, sub)
		}
	}
	if len(out) > int(limit) {
		out = out[:limit]
	}
	return out, nil
}

func (r *fakeSubscriptionRepo) ListPendingChanges(_ context.Context, now time.Time, limit int32) ([]domain.Subscription, error) {
	var out []domain.Subscription
	for _, sub := range r.subs {
		if sub.Status == domain.SubscriptionStatusActive &&
			sub.PendingTariffID != nil && sub.PendingChangeAt != nil &&
			!sub.PendingChangeAt.After(now) {
			out = append(out, sub)
		}
	}
	if len(out) > int(limit) {
		out = out[:limit]
	}
	return out, nil
}

func (r *fakeSubscriptionRepo) WithTx(transaction.Tx) SubscriptionRepository { return r }

type fakePaymentMethodRepo struct {
	methods map[uuid.UUID]domain.PaymentMethod
}

func (r *fakePaymentMethodRepo) Create(_ context.Context, pm domain.PaymentMethod) (domain.PaymentMethod, error) {
	r.methods[pm.ID] = pm
	return pm, nil
}

func (r *fakePaymentMethodRepo) UpsertByTokenHash(_ context.Context, pm domain.PaymentMethod) (domain.PaymentMethod, error) {
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

func (r *fakePaymentMethodRepo) GetByID(_ context.Context, id uuid.UUID) (domain.PaymentMethod, error) {
	pm, ok := r.methods[id]
	if !ok {
		return domain.PaymentMethod{}, ErrNotFound
	}
	return pm, nil
}

func (r *fakePaymentMethodRepo) ListByUserID(_ context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error) {
	var out []domain.PaymentMethod
	for _, pm := range r.methods {
		if pm.UserID == userID {
			out = append(out, pm)
		}
	}
	return out, nil
}

func (r *fakePaymentMethodRepo) SetActive(_ context.Context, userID, methodID uuid.UUID) error {
	pm, ok := r.methods[methodID]
	if !ok || pm.UserID != userID {
		return ErrNotFound
	}
	for id, m := range r.methods {
		if m.UserID == userID {
			m.IsActive = false
			r.methods[id] = m
		}
	}
	pm.IsActive = true
	r.methods[methodID] = pm
	return nil
}

func (r *fakePaymentMethodRepo) Delete(_ context.Context, userID, methodID uuid.UUID) error {
	pm, ok := r.methods[methodID]
	if !ok || pm.UserID != userID {
		return ErrNotFound
	}
	delete(r.methods, methodID)
	return nil
}

func (r *fakePaymentMethodRepo) WithTx(transaction.Tx) PaymentMethodRepository { return r }

type fakeSubscriptionPaymentRepo struct {
	payments                 map[uuid.UUID]domain.SubscriptionPayment
	pendingUpgradePaymentIDs map[uuid.UUID]bool
	listPendingErr           error
	listPendingLimit         int32
	listPendingCalls         int
	forUpdateStatus          map[uuid.UUID]domain.PaymentStatus
}

func (r *fakeSubscriptionPaymentRepo) Create(_ context.Context, p domain.SubscriptionPayment) (domain.SubscriptionPayment, error) {
	for _, existing := range r.payments {
		if existing.UserID == p.UserID && existing.TariffID == p.TariffID && existing.Period == p.Period && existing.Status == domain.PaymentStatusPending {
			return domain.SubscriptionPayment{}, ErrAlreadyExists
		}
	}
	r.payments[p.ID] = p
	return p, nil
}

func (r *fakeSubscriptionPaymentRepo) GetByID(_ context.Context, id uuid.UUID) (domain.SubscriptionPayment, error) {
	p, ok := r.payments[id]
	if !ok {
		return domain.SubscriptionPayment{}, ErrNotFound
	}
	return p, nil
}

func (r *fakeSubscriptionPaymentRepo) GetByIDAdmin(_ context.Context, id uuid.UUID) (SubscriptionPaymentWithUser, error) {
	p, ok := r.payments[id]
	if !ok {
		return SubscriptionPaymentWithUser{}, ErrNotFound
	}
	return SubscriptionPaymentWithUser{Payment: p, UserPhone: ""}, nil
}

func (r *fakeSubscriptionPaymentRepo) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.SubscriptionPayment, error) {
	p, err := r.GetByID(ctx, id)
	if err != nil {
		return p, err
	}
	if status, ok := r.forUpdateStatus[id]; ok {
		p.Status = status
	}
	return p, nil
}

func (r *fakeSubscriptionPaymentRepo) ListByUserID(_ context.Context, userID uuid.UUID) ([]domain.SubscriptionPayment, error) {
	var out []domain.SubscriptionPayment
	for _, p := range r.payments {
		if p.UserID == userID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r *fakeSubscriptionPaymentRepo) ListPendingSubscriptionPaymentsByUserID(_ context.Context, userID uuid.UUID) ([]domain.SubscriptionPayment, error) {
	var out []domain.SubscriptionPayment
	for _, p := range r.payments {
		if p.UserID == userID && p.Status == domain.PaymentStatusPending {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r *fakeSubscriptionPaymentRepo) ListPendingUpgradePayments(_ context.Context, _ time.Time, limit int32) ([]domain.SubscriptionPayment, error) {
	var out []domain.SubscriptionPayment
	for _, p := range r.payments {
		if r.pendingUpgradePaymentIDs[p.ID] && p.Status == domain.PaymentStatusPending && p.ProviderPaymentID != nil {
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	if len(out) > int(limit) {
		out = out[:limit]
	}
	return out, nil
}

func (r *fakeSubscriptionPaymentRepo) ListPendingPayments(_ context.Context, createdBefore time.Time, limit int32) ([]domain.SubscriptionPayment, error) {
	r.listPendingCalls++
	if r.listPendingErr != nil {
		return nil, r.listPendingErr
	}
	if r.listPendingLimit > 0 && limit > r.listPendingLimit {
		limit = r.listPendingLimit
	}
	var out []domain.SubscriptionPayment
	for _, p := range r.payments {
		if p.Status != domain.PaymentStatusPending || !p.CreatedAt.Before(createdBefore) {
			continue
		}
		if p.ProviderPaymentID == nil || *p.ProviderPaymentID == "" {
			continue
		}
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	if len(out) > int(limit) {
		out = out[:limit]
	}
	return out, nil
}

func (r *fakeSubscriptionPaymentRepo) ListAll(_ context.Context, _ string, _ uuid.UUID, _, _ int) ([]SubscriptionPaymentWithUser, int64, error) {
	return nil, 0, nil
}

func (r *fakeSubscriptionPaymentRepo) GetLastSucceededBySubscriptionID(_ context.Context, subscriptionID uuid.UUID) (domain.SubscriptionPayment, error) {
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
		return domain.SubscriptionPayment{}, ErrNotFound
	}
	return *latest, nil
}

func (r *fakeSubscriptionPaymentRepo) MarkSucceeded(_ context.Context, id uuid.UUID, now time.Time) error {
	p, ok := r.payments[id]
	if !ok {
		return ErrNotFound
	}
	if err := p.MarkSucceeded(now); err != nil {
		return err
	}
	r.payments[id] = p
	return nil
}

func (r *fakeSubscriptionPaymentRepo) MarkFailed(_ context.Context, id uuid.UUID, errorCode *string, now time.Time) error {
	p, ok := r.payments[id]
	if !ok {
		return ErrNotFound
	}
	if err := p.MarkFailed(errorCode, now); err != nil {
		return err
	}
	r.payments[id] = p
	return nil
}

func (r *fakeSubscriptionPaymentRepo) MarkRefunded(_ context.Context, id uuid.UUID, now time.Time) error {
	p, ok := r.payments[id]
	if !ok {
		return ErrNotFound
	}
	if err := p.MarkRefunded(now); err != nil {
		return err
	}
	r.payments[id] = p
	return nil
}

func (r *fakeSubscriptionPaymentRepo) UpdateProviderPaymentID(_ context.Context, id uuid.UUID, providerPaymentID string) (domain.SubscriptionPayment, error) {
	p, ok := r.payments[id]
	if !ok {
		return domain.SubscriptionPayment{}, ErrNotFound
	}
	p.ProviderPaymentID = &providerPaymentID
	r.payments[id] = p
	return p, nil
}

func (r *fakeSubscriptionPaymentRepo) UpdatePaymentURL(_ context.Context, id uuid.UUID, paymentURL string) (domain.SubscriptionPayment, error) {
	p, ok := r.payments[id]
	if !ok {
		return domain.SubscriptionPayment{}, ErrNotFound
	}
	p.PaymentURL = &paymentURL
	r.payments[id] = p
	return p, nil
}

func (r *fakeSubscriptionPaymentRepo) UpdatePaymentMethodAndProviderID(_ context.Context, id, paymentMethodID uuid.UUID, providerPaymentID string) (domain.SubscriptionPayment, error) {
	p, ok := r.payments[id]
	if !ok {
		return domain.SubscriptionPayment{}, ErrNotFound
	}
	p.PaymentMethodID = &paymentMethodID
	p.ProviderPaymentID = &providerPaymentID
	r.payments[id] = p
	return p, nil
}

func (r *fakeSubscriptionPaymentRepo) UpdatePaymentMethodID(_ context.Context, id, paymentMethodID uuid.UUID) (domain.SubscriptionPayment, error) {
	p, ok := r.payments[id]
	if !ok {
		return domain.SubscriptionPayment{}, ErrNotFound
	}
	p.PaymentMethodID = &paymentMethodID
	r.payments[id] = p
	return p, nil
}

func (r *fakeSubscriptionPaymentRepo) WithTx(transaction.Tx) SubscriptionPaymentRepository { return r }

// --- property archiver fake ---

type fakePropertyArchiver struct {
	calls []fakeArchiveCall
	err   error
}

type fakeArchiveCall struct {
	userID uuid.UUID
	limit  int
}

func (a *fakePropertyArchiver) ArchiveExcessProperties(_ context.Context, _ transaction.Tx, userID uuid.UUID, limit int) error {
	if a.err != nil {
		return a.err
	}
	if a.calls == nil {
		a.calls = make([]fakeArchiveCall, 0)
	}
	a.calls = append(a.calls, fakeArchiveCall{userID: userID, limit: limit})
	return nil
}

// --- provider fake ---

type stubProvider struct {
	name       domain.PaymentProvider
	initCalled bool
	initReq    InitRequest
	initRes    InitResult
	initErr    error
	initFunc   func(InitRequest)

	chargeRes    ChargeResult
	chargeErr    error
	chargeReq    ChargeRequest
	chargeFunc   func(ChargeRequest)
	chargeCalled bool

	statusRes    domain.PaymentStatus
	statusErr    error
	statusFunc   func(uuid.UUID, string)
	statusCalled bool

	parseWebhook func([]byte) (WebhookPayload, error)

	initAddCardRes    InitAddCardResult
	initAddCardErr    error
	initAddCardReq    InitAddCardRequest
	initAddCardCalled bool

	removeCardErr      error
	removeCardCalled   bool
	removeCardCustomer string
	removeCardCardID   string

	confirmPaymentRes        WebhookPayload
	confirmPaymentErr        error
	confirmPaymentInternalID string

	cancelRes    CancelResult
	cancelErr    error
	cancelCalled bool
	cancelReq    CancelRequest
}

func (p *stubProvider) Name() domain.PaymentProvider {
	if p.name == "" {
		return domain.ProviderFake
	}
	return p.name
}

func (p *stubProvider) Init(_ context.Context, req InitRequest) (InitResult, error) {
	p.initCalled = true
	p.initReq = req
	if p.initFunc != nil {
		p.initFunc(req)
	}
	if p.initErr != nil {
		return InitResult{}, p.initErr
	}
	res := p.initRes
	if res.ProviderPaymentID == "" {
		res.ProviderPaymentID = "stub_" + uuid.NewString()
	}
	if res.SavedToken == "" && p.Name() == domain.ProviderFake {
		res.SavedToken = "stub_token_" + uuid.NewString()
	}
	if res.PaymentURL == "" {
		res.PaymentURL = "http://localhost/confirm/" + res.ProviderPaymentID
	}
	if res.Status == "" {
		res.Status = domain.PaymentStatusPending
	}
	return res, nil
}

func (p *stubProvider) Charge(_ context.Context, req ChargeRequest) (ChargeResult, error) {
	p.chargeCalled = true
	p.chargeReq = req
	if p.chargeFunc != nil {
		p.chargeFunc(req)
	}
	return p.chargeRes, p.chargeErr
}

func (p *stubProvider) Status(_ context.Context, paymentID uuid.UUID, providerPaymentID string) (domain.PaymentStatus, error) {
	p.statusCalled = true
	if p.statusFunc != nil {
		p.statusFunc(paymentID, providerPaymentID)
	}
	return p.statusRes, p.statusErr
}

func (p *stubProvider) ParseWebhook(_ context.Context, payload []byte) (WebhookPayload, error) {
	if p.parseWebhook != nil {
		return p.parseWebhook(payload)
	}
	return WebhookPayload{}, nil
}

func (p *stubProvider) InitAddCard(_ context.Context, req InitAddCardRequest) (InitAddCardResult, error) {
	p.initAddCardCalled = true
	p.initAddCardReq = req
	if p.initAddCardErr != nil {
		return InitAddCardResult{}, p.initAddCardErr
	}
	res := p.initAddCardRes
	if res.PaymentURL == "" {
		res.PaymentURL = "http://localhost/add-card/confirm"
	}
	return res, nil
}

func (p *stubProvider) RemoveCard(_ context.Context, customerKey, cardID string) error {
	p.removeCardCalled = true
	p.removeCardCustomer = customerKey
	p.removeCardCardID = cardID
	return p.removeCardErr
}

func (p *stubProvider) WebhookResponse() []byte {
	return []byte(`{"status":"ok"}`)
}

func (p *stubProvider) ConfirmPayment(_ context.Context, internalPaymentID string) (WebhookPayload, error) {
	p.confirmPaymentInternalID = internalPaymentID
	return p.confirmPaymentRes, p.confirmPaymentErr
}

func (p *stubProvider) Cancel(_ context.Context, req CancelRequest) (CancelResult, error) {
	p.cancelCalled = true
	p.cancelReq = req
	return p.cancelRes, p.cancelErr
}

// --- test fixtures ---

type testDeps struct {
	tariffs              *fakeTariffRepo
	subscriptions        *fakeSubscriptionRepo
	paymentMethods       *fakePaymentMethodRepo
	subscriptionPayments *fakeSubscriptionPaymentRepo
	propertyArchiver     *fakePropertyArchiver
	provider             *stubProvider
	beginner             *fakeBeginner
	clock                *fakeClock
	service              *BillingService
}

func newTestDeps(t *testing.T) *testDeps {
	t.Helper()
	d := &testDeps{
		tariffs:        &fakeTariffRepo{byName: make(map[domain.TariffName]domain.Tariff)},
		subscriptions:  &fakeSubscriptionRepo{subs: make(map[uuid.UUID]domain.Subscription)},
		paymentMethods: &fakePaymentMethodRepo{methods: make(map[uuid.UUID]domain.PaymentMethod)},
		subscriptionPayments: &fakeSubscriptionPaymentRepo{
			payments:                 make(map[uuid.UUID]domain.SubscriptionPayment),
			pendingUpgradePaymentIDs: make(map[uuid.UUID]bool),
		},
		propertyArchiver: &fakePropertyArchiver{},
		provider:         &stubProvider{},
		beginner:         &fakeBeginner{},
		clock:            newFakeClock(fixedNow),
	}
	d.service = NewBillingService(
		d.tariffs,
		d.subscriptions,
		d.paymentMethods,
		d.subscriptionPayments,
		d.provider,
		d.beginner,
		d.clock,
		discardLogger(),
		"http://localhost",
		d.propertyArchiver,
		nil,
	)
	return d
}

func (d *testDeps) addTariff(t domain.Tariff) {
	d.tariffs.byName[t.Name] = t
	d.tariffs.list = append(d.tariffs.list, t)
}

func (d *testDeps) addSubscription(sub domain.Subscription) {
	d.subscriptions.subs[sub.UserID] = sub
}

// --- tests ---

func TestBillingService_ListTariffsOrderedByPrice(t *testing.T) {
	d := newTestDeps(t)
	d.addTariff(domain.Tariff{
		ID:                  uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Name:                domain.TariffPro,
		ActivePropertyLimit: 50,
		MonthlyPriceKopecks: 5000,
	})
	d.addTariff(domain.Tariff{
		ID:                  uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Name:                domain.TariffBasic,
		ActivePropertyLimit: 5,
		MonthlyPriceKopecks: 1000,
	})

	list, err := d.service.ListTariffs(context.Background())
	if err != nil {
		t.Fatalf("ListTariffs error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 tariffs, got %d", len(list))
	}
	if list[0].Name != domain.TariffBasic || list[1].Name != domain.TariffPro {
		t.Errorf("expected [basic, pro], got %v", []domain.TariffName{list[0].Name, list[1].Name})
	}
}

func TestBillingService_GetSubscriptionWithActivePaymentMethod(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	methodID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	tariffID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")

	d.addTariff(domain.Tariff{
		ID: tariffID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.addSubscription(domain.Subscription{
		ID:                    uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd"),
		UserID:                userID,
		TariffID:              tariffID,
		Source:                domain.SubscriptionSourcePaid,
		Status:                domain.SubscriptionStatusActive,
		AutoRenewEnabled:      true,
		ActivePaymentMethodID: &methodID,
	})
	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:       methodID,
		UserID:   userID,
		Provider: domain.ProviderFake,
		IsActive: true,
	}

	view, err := d.service.GetSubscription(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetSubscription error: %v", err)
	}
	if view.Subscription.UserID != userID {
		t.Errorf("expected user id %s, got %s", userID, view.Subscription.UserID)
	}
	if view.ActivePaymentMethod == nil || view.ActivePaymentMethod.ID != methodID {
		t.Errorf("expected active payment method %s, got %v", methodID, view.ActivePaymentMethod)
	}
}

func TestBillingService_ChangeTariff_UpgradeCreatesPendingPayment(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.addTariff(domain.Tariff{
		ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		UserID:           userID,
		TariffID:         basicID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	})

	resp, err := d.service.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: string(domain.TariffPro),
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff error: %v", err)
	}
	if resp.PaymentID == uuid.Nil {
		t.Error("expected non-nil payment id")
	}
	if resp.ConfirmURL == "" {
		t.Error("expected non-empty confirm url")
	}
	if !d.provider.initCalled {
		t.Error("expected provider.Init to be called")
	}

	// Subscription must stay unchanged until payment is confirmed.
	sub := d.subscriptions.subs[userID]
	if sub.TariffID != basicID {
		t.Errorf("expected subscription tariff to remain basic, got %s", sub.TariffID)
	}
	if sub.PendingTariffID != nil {
		t.Error("expected no pending tariff for upgrade")
	}

	payment, ok := d.subscriptionPayments.payments[resp.PaymentID]
	if !ok {
		t.Fatalf("payment not saved")
	}
	if payment.Status != domain.PaymentStatusPending {
		t.Errorf("expected pending status, got %s", payment.Status)
	}
	if payment.TariffID != proID {
		t.Errorf("expected payment tariff pro, got %s", payment.TariffID)
	}
	if payment.AmountKopecks != 5000 {
		t.Errorf("expected amount 5000, got %d", payment.AmountKopecks)
	}
	if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID == "" {
		t.Error("expected provider payment id set")
	}
	if payment.PaymentMethodID == nil {
		t.Fatal("expected payment method id set")
	}

	pm, ok := d.paymentMethods.methods[*payment.PaymentMethodID]
	if !ok {
		t.Fatalf("payment method not saved")
	}
	if pm.IsActive {
		t.Error("expected saved payment method to remain inactive until confirmation")
	}
	if pm.ProviderToken == "" {
		t.Error("expected saved payment method token")
	}
	if d.beginner.begun != 2 || d.beginner.committed != 2 {
		t.Errorf("expected two committed transactions, got begun=%d committed=%d", d.beginner.begun, d.beginner.committed)
	}
}

func TestBillingService_ChangeTariff_UpgradeCreatesPaymentBeforeProviderCall(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.addTariff(domain.Tariff{
		ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		UserID:           userID,
		TariffID:         basicID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	})

	var seenPaymentID uuid.UUID
	d.provider.initFunc = func(req InitRequest) {
		seenPaymentID = req.PaymentID
		payment, ok := d.subscriptionPayments.payments[req.PaymentID]
		if !ok {
			t.Errorf("provider.Init called before payment record was created")
			return
		}
		if payment.Status != domain.PaymentStatusPending {
			t.Errorf("expected payment status pending before init, got %s", payment.Status)
		}
		if payment.ProviderPaymentID != nil {
			t.Error("expected no provider payment id before init")
		}
		if payment.PaymentMethodID != nil {
			t.Error("expected no payment method id before init")
		}
	}

	resp, err := d.service.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: string(domain.TariffPro),
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff error: %v", err)
	}
	if seenPaymentID != resp.PaymentID {
		t.Errorf("expected provider.Init to receive payment %s, got %s", resp.PaymentID, seenPaymentID)
	}
}

func TestBillingService_ChangeTariff_DowngradeSchedulesPendingChange(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	basicID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	proID := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.addTariff(domain.Tariff{
		ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               uuid.MustParse("88888888-8888-8888-8888-888888888888"),
		UserID:           userID,
		TariffID:         proID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	})

	resp, err := d.service.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: string(domain.TariffBasic),
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff error: %v", err)
	}
	if resp.PaymentID != uuid.Nil || resp.ConfirmURL != "" {
		t.Error("expected empty response for downgrade")
	}
	if d.provider.initCalled {
		t.Error("expected no provider call for downgrade")
	}

	sub := d.subscriptions.subs[userID]
	if sub.TariffID != proID {
		t.Errorf("expected current tariff to remain pro, got %s", sub.TariffID)
	}
	if sub.PendingTariffID == nil || *sub.PendingTariffID != basicID {
		t.Errorf("expected pending tariff basic, got %v", sub.PendingTariffID)
	}
	if sub.PendingChangeAt == nil || !sub.PendingChangeAt.Equal(validUntil) {
		t.Errorf("expected pending change at %v, got %v", validUntil, sub.PendingChangeAt)
	}
	if sub.PendingPeriod == nil || *sub.PendingPeriod != domain.PeriodMonth {
		t.Errorf("expected pending period month, got %v", sub.PendingPeriod)
	}
	if sub.AutoRenewEnabled {
		t.Error("downgrade scheduling should not change auto-renew")
	}
}

func TestBillingService_ChangeTariff_AlreadyOnTariff(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	basicID := uuid.MustParse("00000000-0000-0000-0000-000000000000")

	d.addTariff(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.addSubscription(domain.Subscription{
		ID:       uuid.MustParse("11111111-1111-1111-1111-111111111112"),
		UserID:   userID,
		TariffID: basicID,
		Source:   domain.SubscriptionSourcePaid,
		Status:   domain.SubscriptionStatusActive,
	})

	_, err := d.service.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: string(domain.TariffBasic),
		Period:     domain.PeriodMonth,
	})
	if !errors.Is(err, ErrAlreadyOnTariff) {
		t.Fatalf("expected ErrAlreadyOnTariff, got %v", err)
	}
}

func TestBillingService_ToggleAutoRenew(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaab")
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addSubscription(domain.Subscription{
		ID:               uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbc"),
		UserID:           userID,
		TariffID:         uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	})

	if err := d.service.ToggleAutoRenew(context.Background(), userID, true); err != nil {
		t.Fatalf("ToggleAutoRenew error: %v", err)
	}
	if !d.subscriptions.subs[userID].AutoRenewEnabled {
		t.Error("expected auto-renew enabled")
	}

	if err := d.service.ToggleAutoRenew(context.Background(), userID, false); err != nil {
		t.Fatalf("ToggleAutoRenew error: %v", err)
	}
	if d.subscriptions.subs[userID].AutoRenewEnabled {
		t.Error("expected auto-renew disabled")
	}
}

func TestBillingService_AddPaymentMethod(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccd")

	resp, err := d.service.AddPaymentMethod(context.Background(), userID, AddPaymentMethodRequest{
		ProviderToken: "raw_token_1234",
	})
	if err != nil {
		t.Fatalf("AddPaymentMethod error: %v", err)
	}
	if resp.ConfirmURL != "" {
		t.Errorf("expected no confirm url for fake provider, got %q", resp.ConfirmURL)
	}
	if resp.PaymentMethod == nil {
		t.Fatal("expected payment method")
	}
	pm := *resp.PaymentMethod
	if pm.UserID != userID {
		t.Errorf("expected user id %s, got %s", userID, pm.UserID)
	}
	if pm.IsActive {
		t.Error("expected new payment method to be inactive")
	}
	if pm.DisplayMask != "****1234" {
		t.Errorf("expected display mask ****1234, got %s", pm.DisplayMask)
	}
}

func TestBillingService_SetActivePaymentMethodUsesTransaction(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("dddddddd-dddd-dddd-dddd-ddddddddddde")
	methodID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5})
	d.addSubscription(domain.Subscription{
		ID:       uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
		UserID:   userID,
		TariffID: basicID,
		Source:   domain.SubscriptionSourcePaid,
		Status:   domain.SubscriptionStatusActive,
	})
	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:       methodID,
		UserID:   userID,
		Provider: domain.ProviderFake,
		IsActive: false,
	}

	if err := d.service.SetActivePaymentMethod(context.Background(), userID, methodID); err != nil {
		t.Fatalf("SetActivePaymentMethod error: %v", err)
	}

	sub := d.subscriptions.subs[userID]
	if sub.ActivePaymentMethodID == nil || *sub.ActivePaymentMethodID != methodID {
		t.Errorf("expected subscription active payment method %s, got %v", methodID, sub.ActivePaymentMethodID)
	}
	if d.beginner.begun != 1 || d.beginner.committed != 1 {
		t.Errorf("expected transaction begin/commit, got begun=%d committed=%d", d.beginner.begun, d.beginner.committed)
	}
	if !d.paymentMethods.methods[methodID].IsActive {
		t.Error("expected method activated")
	}
}

func TestBillingService_DeletePaymentMethodUsesTransaction(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	methodID := uuid.MustParse("11111111-1111-1111-1111-111111111113")

	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:       methodID,
		UserID:   userID,
		Provider: domain.ProviderFake,
	}

	if err := d.service.DeletePaymentMethod(context.Background(), userID, methodID); err != nil {
		t.Fatalf("DeletePaymentMethod error: %v", err)
	}
	// Validation and deletion run atomically in a single transaction.
	if d.beginner.begun != 1 || d.beginner.committed != 1 {
		t.Errorf("expected one committed transaction, got begun=%d committed=%d", d.beginner.begun, d.beginner.committed)
	}
	if _, ok := d.paymentMethods.methods[methodID]; ok {
		t.Error("expected method deleted")
	}
}

func TestBillingService_DeletePaymentMethod_RejectsActiveMethodInSingleTransaction(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	methodID := uuid.MustParse("11111111-1111-1111-1111-111111111113")

	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:       methodID,
		UserID:   userID,
		Provider: domain.ProviderFake,
		IsActive: true,
	}

	err := d.service.DeletePaymentMethod(context.Background(), userID, methodID)
	if !errors.Is(err, ErrPaymentMethodInUse) {
		t.Fatalf("expected ErrPaymentMethodInUse, got %v", err)
	}
	if _, ok := d.paymentMethods.methods[methodID]; !ok {
		t.Error("expected active method to remain (not deleted)")
	}
	if d.provider.removeCardCalled {
		t.Error("expected provider.RemoveCard not to be called for active method")
	}
	// The in-use check runs inside the single transaction, which is rolled back
	// on rejection; nothing is committed.
	if d.beginner.begun != 1 || d.beginner.committed != 0 {
		t.Errorf("expected one rolled-back transaction, got begun=%d committed=%d", d.beginner.begun, d.beginner.committed)
	}
	if d.beginner.rolledBack != 1 {
		t.Errorf("expected the transaction to be rolled back, got rolledBack=%d", d.beginner.rolledBack)
	}
}

func TestBillingService_ConfirmFakePayment_AppliesUpgrade(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("22222222-2222-2222-2222-222222222223")
	basicID := uuid.MustParse("33333333-3333-3333-3333-333333333334")
	proID := uuid.MustParse("44444444-4444-4444-4444-444444444445")
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.addTariff(domain.Tariff{
		ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               uuid.MustParse("66666666-6666-6666-6666-666666666667"),
		UserID:           userID,
		TariffID:         basicID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	})

	resp, err := d.service.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: string(domain.TariffPro),
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff error: %v", err)
	}

	payment := d.subscriptionPayments.payments[resp.PaymentID]
	if payment.PaymentMethodID == nil {
		t.Fatal("expected payment method id set by upgrade flow")
	}
	methodID := *payment.PaymentMethodID

	d.provider.confirmPaymentRes = WebhookPayload{
		ProviderPaymentID: *payment.ProviderPaymentID,
		InternalPaymentID: resp.PaymentID,
		Status:            domain.PaymentStatusSucceeded,
	}

	if err := d.service.ConfirmFakePayment(context.Background(), resp.PaymentID); err != nil {
		t.Fatalf("ConfirmFakePayment error: %v", err)
	}

	if d.provider.confirmPaymentInternalID != resp.PaymentID.String() {
		t.Errorf("expected provider.ConfirmPayment to receive internal payment id %s, got %q", resp.PaymentID, d.provider.confirmPaymentInternalID)
	}

	if d.subscriptionPayments.payments[resp.PaymentID].Status != domain.PaymentStatusSucceeded {
		t.Errorf("expected payment succeeded, got %s", d.subscriptionPayments.payments[resp.PaymentID].Status)
	}

	sub := d.subscriptions.subs[userID]
	if sub.TariffID != proID {
		t.Errorf("expected tariff upgraded to pro, got %s", sub.TariffID)
	}
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(fixedNow.AddDate(0, 1, 0)) {
		t.Errorf("expected valid until %v, got %v", fixedNow.AddDate(0, 1, 0), sub.ValidUntil)
	}
	if !sub.AutoRenewEnabled {
		t.Error("expected auto-renew enabled after upgrade")
	}
	if sub.ActivePaymentMethodID == nil || *sub.ActivePaymentMethodID != methodID {
		t.Errorf("expected active payment method %s, got %v", methodID, sub.ActivePaymentMethodID)
	}
	if sub.PendingTariffID != nil || sub.PendingChangeAt != nil {
		t.Error("expected pending change cleared")
	}
	// ChangeTariff uses two transactions; ConfirmFakePayment uses one critical
	// transaction, one best-effort subscription-renewal transaction and one
	// best-effort archiving transaction.
	if d.beginner.begun != 5 || d.beginner.committed != 5 {
		t.Errorf("expected five committed transactions, got begun=%d committed=%d", d.beginner.begun, d.beginner.committed)
	}
}

func TestBillingService_HandleWebhook_AppliesFailedUpgradePayment(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777778")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-888888888889")
	currentTariffID := uuid.MustParse("99999999-9999-9999-9999-99999999999a")
	newTariffID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaac")
	providerPaymentID := "stub_webhook_1"
	errCode := "card_declined"

	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaab")
	d.addSubscription(domain.Subscription{
		ID:       subscriptionID,
		UserID:   userID,
		TariffID: currentTariffID,
		Source:   domain.SubscriptionSourcePaid,
		Status:   domain.SubscriptionStatusActive,
	})
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    subscriptionID,
		TariffID:          newTariffID,
		AmountKopecks:     1000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
	}

	d.provider.parseWebhook = func(_ []byte) (WebhookPayload, error) {
		return WebhookPayload{
			ProviderPaymentID: providerPaymentID,
			InternalPaymentID: paymentID,
			Status:            domain.PaymentStatusFailed,
			ErrorCode:         &errCode,
		}, nil
	}

	if err := d.service.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook error: %v", err)
	}

	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusFailed {
		t.Errorf("expected payment failed, got %s", payment.Status)
	}
	if payment.ErrorCode == nil || *payment.ErrorCode != errCode {
		t.Errorf("expected error code %s, got %v", errCode, payment.ErrorCode)
	}

	sub := d.subscriptions.subs[userID]
	if sub.Status != domain.SubscriptionStatusActive {
		t.Errorf("failed upgrade webhook should not change subscription status, got %s", sub.Status)
	}
}

func TestBillingService_HandleWebhook_RefundDowngradesToBasic(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-77777777777a")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-88888888888b")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaae")
	basicID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	proID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	providerPaymentID := "stub_webhook_refund"
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.addTariff(domain.Tariff{
		ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               subscriptionID,
		UserID:           userID,
		TariffID:         proID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: true,
	})
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    subscriptionID,
		TariffID:          proID,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusSucceeded,
		SucceededAt:       &validUntil,
	}

	d.provider.parseWebhook = func(_ []byte) (WebhookPayload, error) {
		return WebhookPayload{
			ProviderPaymentID: providerPaymentID,
			InternalPaymentID: paymentID,
			Status:            domain.PaymentStatusRefunded,
			AmountKopecks:     5000,
		}, nil
	}

	if err := d.service.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook error: %v", err)
	}

	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusRefunded {
		t.Errorf("expected payment refunded, got %s", payment.Status)
	}
	if payment.RefundedAmountKopecks == nil || *payment.RefundedAmountKopecks != 5000 {
		t.Errorf("expected refunded amount 5000, got %v", payment.RefundedAmountKopecks)
	}

	sub := d.subscriptions.subs[userID]
	if sub.TariffID != basicID {
		t.Errorf("expected subscription downgraded to basic, got tariff %s", sub.TariffID)
	}
	if sub.Status != domain.SubscriptionStatusActive {
		t.Errorf("expected subscription status active, got %s", sub.Status)
	}
	if sub.ValidUntil != nil {
		t.Errorf("expected valid_until nil after refund, got %v", sub.ValidUntil)
	}
	if sub.AutoRenewEnabled {
		t.Error("expected auto_renew disabled after refund")
	}
	if len(d.propertyArchiver.calls) != 1 || d.propertyArchiver.calls[0].limit != 5 {
		t.Errorf("expected property archiver called with limit 5, got %v", d.propertyArchiver.calls)
	}
}

func TestBillingService_HandleWebhook_FailedRenewalMovesToGrace(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777779")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-88888888888a")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaad")
	tariffID := uuid.MustParse("99999999-9999-9999-9999-99999999999b")
	providerPaymentID := "stub_webhook_renewal_failed"
	validUntil := fixedNow.AddDate(0, 0, -1)
	errCode := "card_declined"

	d.addTariff(domain.Tariff{
		ID: tariffID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               subscriptionID,
		UserID:           userID,
		TariffID:         tariffID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: true,
	})
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    subscriptionID,
		TariffID:          tariffID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
	}

	d.provider.parseWebhook = func(_ []byte) (WebhookPayload, error) {
		return WebhookPayload{
			ProviderPaymentID: providerPaymentID,
			InternalPaymentID: paymentID,
			Status:            domain.PaymentStatusFailed,
			ErrorCode:         &errCode,
		}, nil
	}

	if err := d.service.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook error: %v", err)
	}

	sub := d.subscriptions.subs[userID]
	if sub.Status != domain.SubscriptionStatusGrace {
		t.Errorf("expected subscription moved to grace, got %s", sub.Status)
	}
	if sub.ValidUntil == nil || !sub.ValidUntil.After(fixedNow) {
		t.Errorf("expected grace valid_until in the future, got %v", sub.ValidUntil)
	}
}

func TestBillingService_HandleWebhook_FailedRenewalDoesNotShortenFutureValidUntil(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777779")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-88888888888a")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaad")
	tariffID := uuid.MustParse("99999999-9999-9999-9999-99999999999b")
	providerPaymentID := "stub_webhook_renewal_failed_future"
	futureValidUntil := fixedNow.AddDate(0, 0, 30)
	errCode := "card_declined"

	d.addTariff(domain.Tariff{
		ID: tariffID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               subscriptionID,
		UserID:           userID,
		TariffID:         tariffID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &futureValidUntil,
		AutoRenewEnabled: true,
	})
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    subscriptionID,
		TariffID:          tariffID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
	}

	d.provider.parseWebhook = func(_ []byte) (WebhookPayload, error) {
		return WebhookPayload{
			ProviderPaymentID: providerPaymentID,
			InternalPaymentID: paymentID,
			Status:            domain.PaymentStatusFailed,
			ErrorCode:         &errCode,
		}, nil
	}

	if err := d.service.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook error: %v", err)
	}

	sub := d.subscriptions.subs[userID]
	if sub.Status != domain.SubscriptionStatusGrace {
		t.Errorf("expected subscription moved to grace, got %s", sub.Status)
	}
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(futureValidUntil) {
		t.Errorf("expected valid_until preserved at %v, got %v", futureValidUntil, sub.ValidUntil)
	}
}

func TestBillingService_ChangeTariff_UpgradeDefersPaymentMethodActivation(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	oldMethodID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.addTariff(domain.Tariff{
		ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		UserID:           userID,
		TariffID:         basicID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	})
	d.paymentMethods.methods[oldMethodID] = domain.PaymentMethod{
		ID:       oldMethodID,
		UserID:   userID,
		Provider: domain.ProviderFake,
		IsActive: true,
	}

	resp, err := d.service.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: string(domain.TariffPro),
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff error: %v", err)
	}

	payment := d.subscriptionPayments.payments[resp.PaymentID]
	if payment.PaymentMethodID == nil {
		t.Fatal("expected payment method id set")
	}
	newMethodID := *payment.PaymentMethodID

	// Activation is deferred until the payment is confirmed, so the previously
	// active method should remain active and the new method should be inactive.
	if !d.paymentMethods.methods[oldMethodID].IsActive {
		t.Error("expected old payment method to remain active until confirmation")
	}
	if d.paymentMethods.methods[newMethodID].IsActive {
		t.Error("expected new payment method to be inactive until confirmation")
	}
}

func TestBillingService_ChangeTariff_UpgradeInitFailureMarksPaymentFailed(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.addTariff(domain.Tariff{
		ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		UserID:           userID,
		TariffID:         basicID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	})

	sensitive := "token=secret123 card 1234-5678-9012-3456 phone +79991234567 token deadbeefcafebabe0011223344556677"
	wantErr := errors.New("provider init failed: " + sensitive)
	d.provider.initErr = wantErr

	resp, err := d.service.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: string(domain.TariffPro),
		Period:     domain.PeriodMonth,
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected provider init error, got %v", err)
	}
	for _, s := range []string{"token=secret123", "1234-5678-9012-3456", "+79991234567", "deadbeefcafebabe0011223344556677"} {
		if strings.Contains(err.Error(), s) {
			t.Errorf("returned error contains sensitive substring %q: %q", s, err.Error())
		}
	}
	if resp.PaymentID != uuid.Nil {
		t.Errorf("expected empty response on error, got %v", resp.PaymentID)
	}

	var failedPayment *domain.SubscriptionPayment
	for _, p := range d.subscriptionPayments.payments {
		if p.UserID == userID && p.TariffID == proID && p.Status == domain.PaymentStatusFailed {
			failedPayment = &p
			break
		}
	}
	if failedPayment == nil {
		t.Fatal("expected a failed payment record for the user")
	}
}

func TestBillingService_ChangeTariff_UpgradeReturnsExistingPendingPayment(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	validUntil := fixedNow.AddDate(0, 1, 0)
	existingPaymentID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	d.addTariff(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.addTariff(domain.Tariff{
		ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		UserID:           userID,
		TariffID:         basicID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	})
	providerPaymentID := "existing_provider_payment_1"
	d.subscriptionPayments.payments[existingPaymentID] = domain.SubscriptionPayment{
		ID:                existingPaymentID,
		UserID:            userID,
		SubscriptionID:    uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		TariffID:          proID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
	}

	resp, err := d.service.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: string(domain.TariffPro),
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff error: %v", err)
	}
	if resp.PaymentID != existingPaymentID {
		t.Errorf("expected existing payment id %s, got %s", existingPaymentID, resp.PaymentID)
	}
	// The test stub provider does not implement PaymentURLProvider, so reusing an
	// existing pending payment leaves ConfirmURL empty.
	if resp.ConfirmURL != "" {
		t.Errorf("expected empty confirm url, got %q", resp.ConfirmURL)
	}
	if d.provider.initCalled {
		t.Error("expected no provider call when reusing existing pending payment with provider reference")
	}
	// A transaction is begun to load the subscription for update and check for
	// the existing pending payment; it is rolled back when returning early.
	if d.beginner.begun != 1 {
		t.Errorf("expected one transaction begun, got begun=%d", d.beginner.begun)
	}
}

func TestBillingService_ChangeTariff_DowngradeBlockedInvalidState(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	basicID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	proID := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.addTariff(domain.Tariff{
		ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               uuid.MustParse("88888888-8888-8888-8888-888888888888"),
		UserID:           userID,
		TariffID:         proID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusBlocked,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	})

	_, err := d.service.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: string(domain.TariffBasic),
		Period:     domain.PeriodMonth,
	})
	if !errors.Is(err, domain.ErrInvalidSubscriptionState) {
		t.Fatalf("expected ErrInvalidSubscriptionState, got %v", err)
	}
}

func TestBillingService_ToggleAutoRenewUsesTransaction(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaab")
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addSubscription(domain.Subscription{
		ID:               uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbc"),
		UserID:           userID,
		TariffID:         uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	})

	if err := d.service.ToggleAutoRenew(context.Background(), userID, true); err != nil {
		t.Fatalf("ToggleAutoRenew error: %v", err)
	}
	if d.beginner.begun != 1 || d.beginner.committed != 1 {
		t.Errorf("expected one committed transaction, got begun=%d committed=%d", d.beginner.begun, d.beginner.committed)
	}
}

func TestBillingService_AddPaymentMethodUsesTransaction(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccd")

	if _, err := d.service.AddPaymentMethod(context.Background(), userID, AddPaymentMethodRequest{
		ProviderToken: "raw_token_1234",
	}); err != nil {
		t.Fatalf("AddPaymentMethod error: %v", err)
	}
	if d.beginner.begun != 1 || d.beginner.committed != 1 {
		t.Errorf("expected one committed transaction, got begun=%d committed=%d", d.beginner.begun, d.beginner.committed)
	}
}

func TestBillingService_HandleWebhook_SucceededPersistsProviderPaymentID(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777778")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-888888888889")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaab")
	tariffID := uuid.MustParse("99999999-9999-9999-9999-99999999999a")
	providerPaymentID := "stub_webhook_provider_id"
	existingValidUntil := fixedNow.AddDate(0, 0, 15)

	d.addTariff(domain.Tariff{
		ID: tariffID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               subscriptionID,
		UserID:           userID,
		TariffID:         tariffID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &existingValidUntil,
		AutoRenewEnabled: true,
	})
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:             paymentID,
		UserID:         userID,
		SubscriptionID: subscriptionID,
		TariffID:       tariffID,
		Period:         domain.PeriodMonth,
		AmountKopecks:  5000,
		Provider:       domain.ProviderFake,
		// ProviderPaymentID intentionally nil to simulate the webhook arriving
		// before the Init response was persisted.
		ProviderPaymentID: nil,
		Status:            domain.PaymentStatusPending,
	}

	d.provider.parseWebhook = func(_ []byte) (WebhookPayload, error) {
		return WebhookPayload{
			ProviderPaymentID: providerPaymentID,
			InternalPaymentID: paymentID,
			Status:            domain.PaymentStatusSucceeded,
		}, nil
	}

	if err := d.service.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook error: %v", err)
	}

	p := d.subscriptionPayments.payments[paymentID]
	if p.ProviderPaymentID == nil || *p.ProviderPaymentID != providerPaymentID {
		t.Errorf("expected provider payment id %q persisted, got %v", providerPaymentID, p.ProviderPaymentID)
	}
	if p.Status != domain.PaymentStatusSucceeded {
		t.Errorf("expected payment succeeded, got %s", p.Status)
	}
}

func TestBillingService_HandleWebhook_AppliesRenewal(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777778")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-888888888889")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaab")
	tariffID := uuid.MustParse("99999999-9999-9999-9999-99999999999a")
	providerPaymentID := "stub_webhook_renewal"
	existingValidUntil := fixedNow.AddDate(0, 0, 15)

	d.addTariff(domain.Tariff{
		ID: tariffID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               subscriptionID,
		UserID:           userID,
		TariffID:         tariffID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &existingValidUntil,
		AutoRenewEnabled: true,
	})
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    subscriptionID,
		TariffID:          tariffID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
	}

	d.provider.parseWebhook = func(_ []byte) (WebhookPayload, error) {
		return WebhookPayload{
			ProviderPaymentID: providerPaymentID,
			InternalPaymentID: paymentID,
			Status:            domain.PaymentStatusSucceeded,
		}, nil
	}

	if err := d.service.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook error: %v", err)
	}

	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusSucceeded {
		t.Errorf("expected payment succeeded, got %s", payment.Status)
	}

	sub := d.subscriptions.subs[userID]
	wantValidUntil := existingValidUntil.AddDate(0, 1, 0)
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(wantValidUntil) {
		t.Errorf("expected valid until extended to %v, got %v", wantValidUntil, sub.ValidUntil)
	}
	if sub.TariffID != tariffID {
		t.Errorf("expected tariff unchanged, got %s", sub.TariffID)
	}
}

func TestBillingService_HandleWebhook_DuplicateRenewalWebhookIsIdempotent(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777778")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-888888888889")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaab")
	tariffID := uuid.MustParse("99999999-9999-9999-9999-99999999999a")
	providerPaymentID := "stub_webhook_duplicate_renewal"
	existingValidUntil := fixedNow.AddDate(0, 0, 15)

	d.addTariff(domain.Tariff{
		ID: tariffID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               subscriptionID,
		UserID:           userID,
		TariffID:         tariffID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &existingValidUntil,
		AutoRenewEnabled: true,
	})
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    subscriptionID,
		TariffID:          tariffID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
	}

	d.provider.parseWebhook = func(_ []byte) (WebhookPayload, error) {
		return WebhookPayload{
			ProviderPaymentID: providerPaymentID,
			InternalPaymentID: paymentID,
			Status:            domain.PaymentStatusSucceeded,
		}, nil
	}

	if err := d.service.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("first HandleWebhook error: %v", err)
	}

	wantValidUntil := existingValidUntil.AddDate(0, 1, 0)
	sub := d.subscriptions.subs[userID]
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(wantValidUntil) {
		t.Fatalf("expected valid until extended to %v, got %v", wantValidUntil, sub.ValidUntil)
	}

	// Simulate a retry webhook arriving later; the subscription must not be
	// extended a second time.
	d.clock.now = fixedNow.Add(time.Hour)
	if err := d.service.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("second HandleWebhook error: %v", err)
	}

	sub = d.subscriptions.subs[userID]
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(wantValidUntil) {
		t.Errorf("duplicate webhook extended valid until: expected %v, got %v", wantValidUntil, sub.ValidUntil)
	}
}

func TestBillingService_ChangeTariff_RejectServiceSubscription(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	basicID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	proID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")

	d.addTariff(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.addTariff(domain.Tariff{
		ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:       uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd"),
		UserID:   userID,
		TariffID: basicID,
		Source:   domain.SubscriptionSourceService,
		Status:   domain.SubscriptionStatusActive,
	})

	_, err := d.service.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: string(domain.TariffPro),
		Period:     domain.PeriodMonth,
	})
	if !errors.Is(err, domain.ErrInvalidSubscriptionState) {
		t.Fatalf("expected ErrInvalidSubscriptionState, got %v", err)
	}
}

func TestBillingService_ConfirmFakePayment_Idempotent(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("22222222-2222-2222-2222-222222222223")
	basicID := uuid.MustParse("33333333-3333-3333-3333-333333333334")
	proID := uuid.MustParse("44444444-4444-4444-4444-444444444445")
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.addTariff(domain.Tariff{
		ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               uuid.MustParse("66666666-6666-6666-6666-666666666667"),
		UserID:           userID,
		TariffID:         basicID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	})

	resp, err := d.service.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: string(domain.TariffPro),
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff error: %v", err)
	}

	payment := d.subscriptionPayments.payments[resp.PaymentID]
	providerPaymentID := ""
	if payment.ProviderPaymentID != nil {
		providerPaymentID = *payment.ProviderPaymentID
	}

	d.provider.confirmPaymentRes = WebhookPayload{
		ProviderPaymentID: providerPaymentID,
		InternalPaymentID: resp.PaymentID,
		Status:            domain.PaymentStatusSucceeded,
	}

	if err := d.service.ConfirmFakePayment(context.Background(), resp.PaymentID); err != nil {
		t.Fatalf("first ConfirmFakePayment error: %v", err)
	}
	if d.subscriptionPayments.payments[resp.PaymentID].Status != domain.PaymentStatusSucceeded {
		t.Fatalf("expected payment succeeded after first confirmation")
	}

	// Second confirmation should be a no-op and not return an error.
	if err := d.service.ConfirmFakePayment(context.Background(), resp.PaymentID); err != nil {
		t.Fatalf("second ConfirmFakePayment error: %v", err)
	}
}

func TestBillingService_ProcessRenewals_Success(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	methodID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	validUntil := fixedNow

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addSubscription(domain.Subscription{
		ID:                    uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		UserID:                userID,
		TariffID:              proID,
		Source:                domain.SubscriptionSourcePaid,
		Status:                domain.SubscriptionStatusActive,
		ValidUntil:            &validUntil,
		AutoRenewEnabled:      true,
		ActivePaymentMethodID: &methodID,
	})
	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:            methodID,
		UserID:        userID,
		Provider:      domain.ProviderFake,
		ProviderToken: "fake_token_1234",
		IsActive:      true,
	}
	d.provider.chargeRes = ChargeResult{ProviderPaymentID: "charge_1", Status: domain.PaymentStatusSucceeded}

	count, err := d.service.ProcessRenewals(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("ProcessRenewals error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 renewal, got %d", count)
	}

	sub := d.subscriptions.subs[userID]
	if sub.Status != domain.SubscriptionStatusActive {
		t.Errorf("expected status active, got %s", sub.Status)
	}
	if sub.ValidUntil == nil || !sub.ValidUntil.After(fixedNow) {
		t.Errorf("expected valid_until extended, got %v", sub.ValidUntil)
	}
}

func TestBillingService_ProcessRenewals_UpdatesStalePaymentMethodID(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	oldMethodID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	newMethodID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	paymentID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	providerPaymentID := "provider_1"
	validUntil := fixedNow

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addSubscription(domain.Subscription{
		ID:                    uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		UserID:                userID,
		TariffID:              proID,
		Source:                domain.SubscriptionSourcePaid,
		Status:                domain.SubscriptionStatusActive,
		ValidUntil:            &validUntil,
		AutoRenewEnabled:      true,
		ActivePaymentMethodID: &newMethodID,
	})
	d.paymentMethods.methods[oldMethodID] = domain.PaymentMethod{
		ID:            oldMethodID,
		UserID:        userID,
		Provider:      domain.ProviderFake,
		ProviderToken: "old_token",
		IsActive:      false,
	}
	d.paymentMethods.methods[newMethodID] = domain.PaymentMethod{
		ID:            newMethodID,
		UserID:        userID,
		Provider:      domain.ProviderFake,
		ProviderToken: "new_token",
		IsActive:      true,
	}
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		TariffID:          proID,
		PaymentMethodID:   &oldMethodID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
	}
	d.provider.chargeRes = ChargeResult{ProviderPaymentID: "charge_1", Status: domain.PaymentStatusSucceeded}

	count, err := d.service.ProcessRenewals(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("ProcessRenewals error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 renewal, got %d", count)
	}

	p := d.subscriptionPayments.payments[paymentID]
	if p.PaymentMethodID == nil || *p.PaymentMethodID != newMethodID {
		t.Errorf("expected payment method id updated to %s, got %v", newMethodID, p.PaymentMethodID)
	}
}

func TestBillingService_ProcessRenewals_FailedChargeMovesToGrace(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	methodID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	validUntil := fixedNow

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addSubscription(domain.Subscription{
		ID:                    uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		UserID:                userID,
		TariffID:              proID,
		Source:                domain.SubscriptionSourcePaid,
		Status:                domain.SubscriptionStatusActive,
		ValidUntil:            &validUntil,
		AutoRenewEnabled:      true,
		ActivePaymentMethodID: &methodID,
	})
	//nolint:gosec // test token, not a real credential
	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:            methodID,
		UserID:        userID,
		Provider:      domain.ProviderFake,
		ProviderToken: "fake_fail_test_token",
		IsActive:      true,
	}
	d.provider.chargeRes = ChargeResult{ProviderPaymentID: "charge_1", Status: domain.PaymentStatusFailed}

	count, err := d.service.ProcessRenewals(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("ProcessRenewals error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 renewal processed, got %d", count)
	}

	sub := d.subscriptions.subs[userID]
	if sub.Status != domain.SubscriptionStatusGrace {
		t.Errorf("expected status grace, got %s", sub.Status)
	}
	if sub.ValidUntil == nil || sub.ValidUntil.Equal(fixedNow) {
		t.Errorf("expected grace valid_until set, got %v", sub.ValidUntil)
	}
}

func TestBillingService_ProcessRenewals_SkipsDuplicateChargeWhenProviderSucceeded(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	methodID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	paymentID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	providerPaymentID := "provider_renewal_1"
	validUntil := fixedNow

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addSubscription(domain.Subscription{
		ID:                    uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		UserID:                userID,
		TariffID:              proID,
		Source:                domain.SubscriptionSourcePaid,
		Status:                domain.SubscriptionStatusActive,
		ValidUntil:            &validUntil,
		AutoRenewEnabled:      true,
		ActivePaymentMethodID: &methodID,
	})
	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:            methodID,
		UserID:        userID,
		Provider:      domain.ProviderFake,
		ProviderToken: "fake_token_1234",
		IsActive:      true,
	}
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		TariffID:          proID,
		PaymentMethodID:   &methodID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
	}
	d.provider.statusRes = domain.PaymentStatusSucceeded

	count, err := d.service.ProcessRenewals(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("ProcessRenewals error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 renewal processed, got %d", count)
	}
	if d.provider.chargeCalled {
		t.Error("expected provider.Charge not to be called when status already succeeded")
	}

	sub := d.subscriptions.subs[userID]
	if sub.Status != domain.SubscriptionStatusActive {
		t.Errorf("expected status active, got %s", sub.Status)
	}
	if sub.ValidUntil == nil || !sub.ValidUntil.After(fixedNow) {
		t.Errorf("expected valid_until extended, got %v", sub.ValidUntil)
	}

	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusSucceeded {
		t.Errorf("expected payment succeeded, got %s", payment.Status)
	}
}

func TestBillingService_ProcessRenewals_FinalizesFailedPaymentWithoutCharge(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	methodID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	paymentID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	providerPaymentID := "provider_renewal_1"
	validUntil := fixedNow

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addSubscription(domain.Subscription{
		ID:                    uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		UserID:                userID,
		TariffID:              proID,
		Source:                domain.SubscriptionSourcePaid,
		Status:                domain.SubscriptionStatusActive,
		ValidUntil:            &validUntil,
		AutoRenewEnabled:      true,
		ActivePaymentMethodID: &methodID,
	})
	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:            methodID,
		UserID:        userID,
		Provider:      domain.ProviderFake,
		ProviderToken: "fake_token_1234",
		IsActive:      true,
	}
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		TariffID:          proID,
		PaymentMethodID:   &methodID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
	}
	d.provider.statusRes = domain.PaymentStatusFailed

	count, err := d.service.ProcessRenewals(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("ProcessRenewals error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 renewal processed, got %d", count)
	}
	if d.provider.chargeCalled {
		t.Error("expected provider.Charge not to be called when status already failed")
	}

	sub := d.subscriptions.subs[userID]
	if sub.Status != domain.SubscriptionStatusGrace {
		t.Errorf("expected status grace, got %s", sub.Status)
	}

	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusFailed {
		t.Errorf("expected payment failed, got %s", payment.Status)
	}
}

type fakeProviderError struct {
	code string
	msg  string
}

func (e *fakeProviderError) Error() string             { return e.msg }
func (e *fakeProviderError) ProviderErrorCode() string { return e.code }

func TestBillingService_ProcessRenewals_ProviderErrorCodeSavedOnFailedCharge(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	methodID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	paymentID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	providerPaymentID := "provider_renewal_1"
	validUntil := fixedNow

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addSubscription(domain.Subscription{
		ID:                    uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		UserID:                userID,
		TariffID:              proID,
		Source:                domain.SubscriptionSourcePaid,
		Status:                domain.SubscriptionStatusActive,
		ValidUntil:            &validUntil,
		AutoRenewEnabled:      true,
		ActivePaymentMethodID: &methodID,
	})
	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:            methodID,
		UserID:        userID,
		Provider:      domain.ProviderFake,
		ProviderToken: "fake_token_1234",
		IsActive:      true,
	}
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		TariffID:          proID,
		PaymentMethodID:   &methodID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
	}

	// First status query must be pending so Charge is attempted; recovery then
	// sees failed and finalizes the payment with the provider error code.
	statusCalls := 0
	d.provider.statusFunc = func(uuid.UUID, string) {
		statusCalls++
		if statusCalls == 1 {
			d.provider.statusRes = domain.PaymentStatusPending
		} else {
			d.provider.statusRes = domain.PaymentStatusFailed
		}
	}
	d.provider.chargeErr = &fakeProviderError{code: "card_declined", msg: "charge failed"}

	count, err := d.service.ProcessRenewals(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("ProcessRenewals error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 renewal processed, got %d", count)
	}
	if !d.provider.chargeCalled {
		t.Error("expected provider.Charge to be called")
	}

	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusFailed {
		t.Errorf("expected payment failed, got %s", payment.Status)
	}
	if payment.ErrorCode == nil || *payment.ErrorCode != "card_declined" {
		t.Errorf("expected error code card_declined, got %v", payment.ErrorCode)
	}
}

func TestBillingService_ProcessExpiredGrace_DowngradesToBasic(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	graceUntil := fixedNow

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addSubscription(domain.Subscription{
		ID:               uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		UserID:           userID,
		TariffID:         proID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusGrace,
		ValidUntil:       &graceUntil,
		AutoRenewEnabled: true,
	})

	count, err := d.service.ProcessExpiredGrace(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("ProcessExpiredGrace error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 downgrade, got %d", count)
	}

	sub := d.subscriptions.subs[userID]
	if sub.TariffID != basicID {
		t.Errorf("expected tariff basic, got %s", sub.TariffID)
	}
	if sub.Status != domain.SubscriptionStatusActive {
		t.Errorf("expected status active, got %s", sub.Status)
	}
	if sub.ValidUntil != nil {
		t.Errorf("expected nil valid_until, got %v", sub.ValidUntil)
	}
	if sub.AutoRenewEnabled {
		t.Error("expected auto-renew disabled")
	}
	if len(d.propertyArchiver.calls) != 1 || d.propertyArchiver.calls[0].limit != 5 {
		t.Errorf("expected property archive call with limit 5, got %v", d.propertyArchiver.calls)
	}
}

func TestBillingService_renewSubscription_ChargeCalledOutsideTransaction(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	methodID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	validUntil := fixedNow

	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	subID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	d.addSubscription(domain.Subscription{
		ID:                    subID,
		UserID:                userID,
		TariffID:              proID,
		Source:                domain.SubscriptionSourcePaid,
		Status:                domain.SubscriptionStatusActive,
		ValidUntil:            &validUntil,
		AutoRenewEnabled:      true,
		ActivePaymentMethodID: &methodID,
	})
	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:            methodID,
		UserID:        userID,
		Provider:      domain.ProviderFake,
		ProviderToken: "fake_token_1234",
		IsActive:      true,
	}
	d.provider.chargeRes = ChargeResult{ProviderPaymentID: "charge_1", Status: domain.PaymentStatusSucceeded}
	d.provider.chargeFunc = func(_ ChargeRequest) {
		if d.beginner.open != 0 {
			t.Errorf("provider.Charge called with %d open transaction(s); expected none", d.beginner.open)
		}
	}

	if err := d.service.renewSubscription(context.Background(), d.subscriptions.subs[userID], fixedNow); err != nil {
		t.Fatalf("renewSubscription error: %v", err)
	}

	sub := d.subscriptions.subs[userID]
	if sub.Status != domain.SubscriptionStatusActive {
		t.Errorf("expected status active, got %s", sub.Status)
	}
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(fixedNow.AddDate(0, 1, 0)) {
		t.Errorf("expected valid_until extended by one month, got %v", sub.ValidUntil)
	}
}

func newTestDepsWithLogger(t *testing.T, log *slog.Logger) *testDeps {
	t.Helper()
	d := newTestDeps(t)
	d.service = NewBillingService(
		d.tariffs,
		d.subscriptions,
		d.paymentMethods,
		d.subscriptionPayments,
		d.provider,
		d.beginner,
		d.clock,
		log,
		"http://localhost",
		d.propertyArchiver,
		nil,
	)
	return d
}

func captureLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewTextHandler(&buf, nil)), &buf
}

func TestBillingService_ProcessRenewals_RecoverAfterChargeTimeoutProviderSucceeds(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	methodID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	validUntil := fixedNow

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addSubscription(domain.Subscription{
		ID:                    uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		UserID:                userID,
		TariffID:              proID,
		Source:                domain.SubscriptionSourcePaid,
		Status:                domain.SubscriptionStatusActive,
		ValidUntil:            &validUntil,
		AutoRenewEnabled:      true,
		ActivePaymentMethodID: &methodID,
	})
	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:            methodID,
		UserID:        userID,
		Provider:      domain.ProviderFake,
		ProviderToken: "fake_token_1234",
		IsActive:      true,
	}
	// provider.Charge returns an error, but also a provider payment id that
	// recovery can use to query Status. Make the first Status query (before
	// Charge) return Pending so the Charge attempt still happens, then have
	// recovery see Succeeded.
	d.provider.chargeRes = ChargeResult{ProviderPaymentID: "charge_timeout_1", Status: domain.PaymentStatusSucceeded}
	d.provider.chargeErr = errors.New("provider timeout")
	statusCalls := 0
	d.provider.statusFunc = func(uuid.UUID, string) {
		statusCalls++
		if statusCalls == 1 {
			d.provider.statusRes = domain.PaymentStatusPending
		} else {
			d.provider.statusRes = domain.PaymentStatusSucceeded
		}
	}

	count, err := d.service.ProcessRenewals(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("ProcessRenewals error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 renewal, got %d", count)
	}

	sub := d.subscriptions.subs[userID]
	if sub.Status != domain.SubscriptionStatusActive {
		t.Errorf("expected status active, got %s", sub.Status)
	}
	if sub.ValidUntil == nil || !sub.ValidUntil.After(fixedNow) {
		t.Errorf("expected valid_until extended, got %v", sub.ValidUntil)
	}

	var payment *domain.SubscriptionPayment
	for _, p := range d.subscriptionPayments.payments {
		if p.SubscriptionID == sub.ID {
			payment = &p
			break
		}
	}
	if payment == nil {
		t.Fatal("expected renewal payment to be created")
	}
	if payment.Status != domain.PaymentStatusSucceeded {
		t.Errorf("expected payment succeeded, got %s", payment.Status)
	}
	if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID != "charge_timeout_1" {
		t.Errorf("expected provider payment id charge_timeout_1, got %v", payment.ProviderPaymentID)
	}
}

func TestBillingService_ProcessRenewals_RecoverAfterChargeTimeoutProviderUnknownKeepsActive(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111112")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222223")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333334")
	methodID := uuid.MustParse("44444444-4444-4444-4444-444444444445")
	validUntil := fixedNow

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addSubscription(domain.Subscription{
		ID:                    uuid.MustParse("55555555-5555-5555-5555-555555555556"),
		UserID:                userID,
		TariffID:              proID,
		Source:                domain.SubscriptionSourcePaid,
		Status:                domain.SubscriptionStatusActive,
		ValidUntil:            &validUntil,
		AutoRenewEnabled:      true,
		ActivePaymentMethodID: &methodID,
	})
	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:            methodID,
		UserID:        userID,
		Provider:      domain.ProviderFake,
		ProviderToken: "fake_token_1234",
		IsActive:      true,
	}

	// Charge times out, leaving the payment pending. Provider status cannot be
	// determined, so the subscription must remain active and the payment must
	// stay pending, not transition to grace.
	d.provider.chargeRes = ChargeResult{ProviderPaymentID: "charge_timeout_unknown", Status: domain.PaymentStatusSucceeded}
	d.provider.chargeErr = errors.New("provider timeout")

	tests := []struct {
		name      string
		statusRes domain.PaymentStatus
		statusErr error
	}{
		{
			name:      "pending status",
			statusRes: domain.PaymentStatusPending,
		},
		{
			name:      "status error",
			statusRes: domain.PaymentStatusPending,
			statusErr: errors.New("provider unreachable: token=secret123 card 1234-5678-9012-3456 phone +79991234567 token deadbeefcafebabe0011223344556677"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, logBuf := captureLogger()
			d := newTestDepsWithLogger(t, log)
			basicID := uuid.MustParse("22222222-2222-2222-2222-222222222223")
			proID := uuid.MustParse("33333333-3333-3333-3333-333333333334")
			methodID := uuid.MustParse("44444444-4444-4444-4444-444444444445")
			validUntil := fixedNow
			userID := uuid.MustParse("11111111-1111-1111-1111-111111111112")

			d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
			d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
			d.addSubscription(domain.Subscription{
				ID:                    uuid.MustParse("55555555-5555-5555-5555-555555555556"),
				UserID:                userID,
				TariffID:              proID,
				Source:                domain.SubscriptionSourcePaid,
				Status:                domain.SubscriptionStatusActive,
				ValidUntil:            &validUntil,
				AutoRenewEnabled:      true,
				ActivePaymentMethodID: &methodID,
			})
			d.paymentMethods.methods[methodID] = domain.PaymentMethod{
				ID:            methodID,
				UserID:        userID,
				Provider:      domain.ProviderFake,
				ProviderToken: "fake_token_1234",
				IsActive:      true,
			}
			d.provider.chargeRes = ChargeResult{ProviderPaymentID: "charge_timeout_unknown", Status: domain.PaymentStatusSucceeded}
			d.provider.chargeErr = errors.New("provider timeout")
			d.provider.statusRes = tt.statusRes
			d.provider.statusErr = tt.statusErr

			count, err := d.service.ProcessRenewals(context.Background(), fixedNow)
			if err != nil {
				t.Fatalf("ProcessRenewals error: %v", err)
			}
			if count != 1 {
				t.Fatalf("expected 1 renewal attempt, got %d", count)
			}

			sub := d.subscriptions.subs[userID]
			if sub.Status != domain.SubscriptionStatusActive {
				t.Errorf("expected status active, got %s", sub.Status)
			}
			if sub.ValidUntil == nil || !sub.ValidUntil.Equal(validUntil) {
				t.Errorf("expected valid_until unchanged at %v, got %v", validUntil, sub.ValidUntil)
			}

			var payment *domain.SubscriptionPayment
			for _, p := range d.subscriptionPayments.payments {
				if p.SubscriptionID == sub.ID {
					payment = &p
					break
				}
			}
			if payment == nil {
				t.Fatal("expected renewal payment to be created")
			}
			if payment.Status != domain.PaymentStatusPending {
				t.Errorf("expected payment pending, got %s", payment.Status)
			}
			if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID != "charge_timeout_unknown" {
				t.Errorf("expected provider payment id charge_timeout_unknown, got %v", payment.ProviderPaymentID)
			}

			if tt.statusErr != nil {
				logs := logBuf.String()
				for _, s := range []string{"token=secret123", "1234-5678-9012-3456", "+79991234567", "deadbeefcafebabe0011223344556677"} {
					if strings.Contains(logs, s) {
						t.Errorf("log contains sensitive substring %q:\n%s", s, logs)
					}
				}
				if !strings.Contains(logs, "provider status unknown during renewal recovery") {
					t.Errorf("expected provider status unknown log, got:\n%s", logs)
				}
			}
		})
	}
}

func TestBillingService_ProcessRenewals_ArchivingFailureDoesNotRollbackPayment(t *testing.T) {
	log, logBuf := captureLogger()
	d := newTestDepsWithLogger(t, log)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	businessID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	methodID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	validUntil := fixedNow

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addTariff(domain.Tariff{ID: businessID, Name: domain.TariffBusiness, ActivePropertyLimit: 10, MonthlyPriceKopecks: 3000})
	d.addSubscription(domain.Subscription{
		ID:                    uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		UserID:                userID,
		TariffID:              proID,
		Source:                domain.SubscriptionSourcePaid,
		Status:                domain.SubscriptionStatusActive,
		ValidUntil:            &validUntil,
		AutoRenewEnabled:      true,
		ActivePaymentMethodID: &methodID,
		PendingTariffID:       &businessID,
		PendingPeriod:         func(p domain.SubscriptionPeriod) *domain.SubscriptionPeriod { return &p }(domain.PeriodMonth),
	})
	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:            methodID,
		UserID:        userID,
		Provider:      domain.ProviderFake,
		ProviderToken: "fake_token_1234",
		IsActive:      true,
	}
	d.provider.chargeRes = ChargeResult{ProviderPaymentID: "charge_1", Status: domain.PaymentStatusSucceeded}
	d.propertyArchiver.err = errors.New("archive failed")

	count, err := d.service.ProcessRenewals(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("ProcessRenewals error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 renewal, got %d", count)
	}

	sub := d.subscriptions.subs[userID]
	if sub.Status != domain.SubscriptionStatusActive {
		t.Errorf("expected status active, got %s", sub.Status)
	}
	if sub.ValidUntil == nil || !sub.ValidUntil.After(fixedNow) {
		t.Errorf("expected valid_until extended, got %v", sub.ValidUntil)
	}

	var payment *domain.SubscriptionPayment
	for _, p := range d.subscriptionPayments.payments {
		if p.SubscriptionID == sub.ID {
			payment = &p
			break
		}
	}
	if payment == nil {
		t.Fatal("expected renewal payment to be created")
	}
	if payment.Status != domain.PaymentStatusSucceeded {
		t.Errorf("expected payment succeeded, got %s", payment.Status)
	}

	logs := logBuf.String()
	if !strings.Contains(logs, "best-effort property archiving failed") {
		t.Errorf("expected archiving error to be logged, got:\n%s", logs)
	}
}

func TestBillingService_ChangeTariff_UpgradeRecoversProviderReferenceAfterCrash(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	existingPaymentID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.addTariff(domain.Tariff{
		ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		UserID:           userID,
		TariffID:         basicID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	})
	// Simulate the state after a crash between provider.Init and the save:
	// a pending payment exists, but it has no provider reference.
	d.subscriptionPayments.payments[existingPaymentID] = domain.SubscriptionPayment{
		ID:             existingPaymentID,
		UserID:         userID,
		SubscriptionID: uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		TariffID:       proID,
		Period:         domain.PeriodMonth,
		AmountKopecks:  5000,
		Provider:       domain.ProviderFake,
		Status:         domain.PaymentStatusPending,
	}

	resp, err := d.service.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: string(domain.TariffPro),
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff error: %v", err)
	}
	if resp.PaymentID != existingPaymentID {
		t.Errorf("expected existing payment id %s, got %s", existingPaymentID, resp.PaymentID)
	}
	if resp.ConfirmURL == "" {
		t.Error("expected non-empty confirm url after recovery")
	}
	if !d.provider.initCalled {
		t.Error("expected provider.Init to be called to recover provider reference")
	}

	payment := d.subscriptionPayments.payments[existingPaymentID]
	if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID == "" {
		t.Error("expected provider payment id recovered and persisted")
	}
	if payment.PaymentMethodID == nil {
		t.Error("expected payment method id recovered and persisted")
	}
}

func TestBillingService_ProcessRenewals_ApplyTariffChangeFailureDoesNotRollbackPayment(t *testing.T) {
	log, logBuf := captureLogger()
	d := newTestDepsWithLogger(t, log)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	methodID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	validUntil := fixedNow

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addSubscription(domain.Subscription{
		ID:                    uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		UserID:                userID,
		TariffID:              basicID,
		Source:                domain.SubscriptionSourcePaid,
		Status:                domain.SubscriptionStatusActive,
		ValidUntil:            &validUntil,
		AutoRenewEnabled:      true,
		ActivePaymentMethodID: &methodID,
		PendingTariffID:       &proID,
		PendingPeriod:         func(p domain.SubscriptionPeriod) *domain.SubscriptionPeriod { return &p }(domain.PeriodMonth),
	})
	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:            methodID,
		UserID:        userID,
		Provider:      domain.ProviderFake,
		ProviderToken: "fake_token_1234",
		IsActive:      true,
	}
	d.provider.chargeRes = ChargeResult{ProviderPaymentID: "charge_1", Status: domain.PaymentStatusSucceeded}

	// First renewal attempt: charge succeeds, but the best-effort tariff change
	// fails because the current tariff cannot be loaded.
	d.tariffs.failGetByIDFor = &basicID

	count, err := d.service.ProcessRenewals(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("ProcessRenewals error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 renewal processed, got %d", count)
	}

	sub := d.subscriptions.subs[userID]
	if sub.TariffID != basicID {
		t.Errorf("expected tariff to remain basic after failed best-effort step, got %s", sub.TariffID)
	}
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(validUntil) {
		t.Errorf("expected valid_until unchanged after failed best-effort step, got %v", sub.ValidUntil)
	}

	var payment *domain.SubscriptionPayment
	for _, p := range d.subscriptionPayments.payments {
		if p.SubscriptionID == sub.ID {
			payment = &p
			break
		}
	}
	if payment == nil {
		t.Fatal("expected renewal payment to be created")
	}
	if payment.Status != domain.PaymentStatusSucceeded {
		t.Errorf("expected payment succeeded despite subscription update failure, got %s", payment.Status)
	}

	logs := logBuf.String()
	if !strings.Contains(logs, "best-effort subscription renewal failed") {
		t.Errorf("expected subscription renewal error to be logged, got:\n%s", logs)
	}

	// Second renewal attempt: the previous succeeded payment is reconciled and
	// the subscription is now updated.
	d.tariffs.failGetByIDFor = nil

	count, err = d.service.ProcessRenewals(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("second ProcessRenewals error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 renewal reconciled, got %d", count)
	}

	sub = d.subscriptions.subs[userID]
	if sub.TariffID != proID {
		t.Errorf("expected tariff upgraded to pro after reconciliation, got %s", sub.TariffID)
	}
	if sub.ValidUntil == nil || !sub.ValidUntil.After(fixedNow) {
		t.Errorf("expected valid_until extended after reconciliation, got %v", sub.ValidUntil)
	}
}

func TestBillingService_HandleWebhook_ApplyTariffChangeFailureDoesNotRollbackPayment(t *testing.T) {
	log, logBuf := captureLogger()
	d := newTestDepsWithLogger(t, log)
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777778")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-888888888889")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaab")
	basicID := uuid.MustParse("99999999-9999-9999-9999-99999999999a")
	proID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	methodID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	providerPaymentID := "stub_webhook_upgrade"

	d.addTariff(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.addTariff(domain.Tariff{
		ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               subscriptionID,
		UserID:           userID,
		TariffID:         basicID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		AutoRenewEnabled: false,
	})
	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:       methodID,
		UserID:   userID,
		Provider: domain.ProviderFake,
		IsActive: false,
	}
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    subscriptionID,
		TariffID:          proID,
		PaymentMethodID:   &methodID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
	}

	d.provider.parseWebhook = func(_ []byte) (WebhookPayload, error) {
		return WebhookPayload{
			ProviderPaymentID: providerPaymentID,
			InternalPaymentID: paymentID,
			Status:            domain.PaymentStatusSucceeded,
		}, nil
	}

	// First webhook: payment is marked succeeded, but the best-effort tariff
	// change fails because the current tariff cannot be loaded.
	d.tariffs.failGetByIDFor = &basicID

	if err := d.service.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("first HandleWebhook error: %v", err)
	}

	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusSucceeded {
		t.Errorf("expected payment succeeded after webhook, got %s", payment.Status)
	}

	sub := d.subscriptions.subs[userID]
	if sub.TariffID != basicID {
		t.Errorf("expected tariff to remain basic after failed best-effort step, got %s", sub.TariffID)
	}

	logs := logBuf.String()
	if !strings.Contains(logs, "best-effort subscription renewal failed") {
		t.Errorf("expected subscription renewal error to be logged, got:\n%s", logs)
	}

	// Second webhook: the already-succeeded payment is reconciled.
	d.tariffs.failGetByIDFor = nil

	if err := d.service.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("second HandleWebhook error: %v", err)
	}

	sub = d.subscriptions.subs[userID]
	if sub.TariffID != proID {
		t.Errorf("expected tariff upgraded to pro after reconciliation, got %s", sub.TariffID)
	}
	if sub.ValidUntil == nil || !sub.ValidUntil.After(fixedNow) {
		t.Errorf("expected valid_until extended after reconciliation, got %v", sub.ValidUntil)
	}
	if sub.ActivePaymentMethodID == nil || *sub.ActivePaymentMethodID != methodID {
		t.Errorf("expected active payment method %s after reconciliation, got %v", methodID, sub.ActivePaymentMethodID)
	}
}

func TestBillingService_ConfirmFakePayment_ArchivesExcessPropertiesAfterTariffChange(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("22222222-2222-2222-2222-222222222223")
	basicID := uuid.MustParse("33333333-3333-3333-3333-333333333334")
	proID := uuid.MustParse("44444444-4444-4444-4444-444444444445")
	businessID := uuid.MustParse("55555555-5555-5555-5555-555555555556")
	methodID := uuid.MustParse("66666666-6666-6666-6666-666666666667")
	paymentID := uuid.MustParse("77777777-7777-7777-7777-777777777778")
	providerPaymentID := "fake_downgrade_1"
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.addTariff(domain.Tariff{
		ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addTariff(domain.Tariff{
		ID: businessID, Name: domain.TariffBusiness, ActivePropertyLimit: 10, MonthlyPriceKopecks: 3000,
	})
	d.addSubscription(domain.Subscription{
		ID:               uuid.MustParse("88888888-8888-8888-8888-888888888889"),
		UserID:           userID,
		TariffID:         proID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	})
	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:       methodID,
		UserID:   userID,
		Provider: domain.ProviderFake,
		IsActive: false,
	}
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    uuid.MustParse("88888888-8888-8888-8888-888888888889"),
		TariffID:          businessID,
		PaymentMethodID:   &methodID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     3000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
	}

	d.provider.confirmPaymentRes = WebhookPayload{
		ProviderPaymentID: providerPaymentID,
		InternalPaymentID: paymentID,
		Status:            domain.PaymentStatusSucceeded,
	}

	if err := d.service.ConfirmFakePayment(context.Background(), paymentID); err != nil {
		t.Fatalf("ConfirmFakePayment error: %v", err)
	}

	sub := d.subscriptions.subs[userID]
	if sub.TariffID != businessID {
		t.Errorf("expected tariff changed to business, got %s", sub.TariffID)
	}
	if len(d.propertyArchiver.calls) != 1 || d.propertyArchiver.calls[0].limit != 10 {
		t.Errorf("expected one archive call with limit 10, got %v", d.propertyArchiver.calls)
	}
}

func TestBillingService_HandleWebhook_ArchivesExcessPropertiesAfterTariffChange(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777779")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-88888888888a")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaac")
	basicID := uuid.MustParse("99999999-9999-9999-9999-99999999999b")
	proID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	businessID := uuid.MustParse("66666666-7777-8888-9999-000000000000")
	methodID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	providerPaymentID := "stub_webhook_downgrade"

	d.addTariff(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.addTariff(domain.Tariff{
		ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addTariff(domain.Tariff{
		ID: businessID, Name: domain.TariffBusiness, ActivePropertyLimit: 10, MonthlyPriceKopecks: 3000,
	})
	d.addSubscription(domain.Subscription{
		ID:               subscriptionID,
		UserID:           userID,
		TariffID:         proID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		AutoRenewEnabled: false,
	})
	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:       methodID,
		UserID:   userID,
		Provider: domain.ProviderFake,
		IsActive: false,
	}
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    subscriptionID,
		TariffID:          businessID,
		PaymentMethodID:   &methodID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     3000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
	}

	d.provider.parseWebhook = func(_ []byte) (WebhookPayload, error) {
		return WebhookPayload{
			ProviderPaymentID: providerPaymentID,
			InternalPaymentID: paymentID,
			Status:            domain.PaymentStatusSucceeded,
		}, nil
	}

	if err := d.service.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook error: %v", err)
	}

	sub := d.subscriptions.subs[userID]
	if sub.TariffID != businessID {
		t.Errorf("expected tariff changed to business, got %s", sub.TariffID)
	}
	if len(d.propertyArchiver.calls) != 1 || d.propertyArchiver.calls[0].limit != 10 {
		t.Errorf("expected one archive call with limit 10, got %v", d.propertyArchiver.calls)
	}
}

func TestBillingService_ProcessRenewals_ProviderErrorSanitizedInLogs(t *testing.T) {
	log, logBuf := captureLogger()
	d := newTestDepsWithLogger(t, log)

	userID := uuid.MustParse("11111111-1111-1111-1111-11111111111a")
	basicID := uuid.MustParse("22222222-2222-2222-2222-22222222222b")
	proID := uuid.MustParse("33333333-3333-3333-3333-33333333333c")
	methodID := uuid.MustParse("44444444-4444-4444-4444-44444444444d")
	validUntil := fixedNow

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addSubscription(domain.Subscription{
		ID:                    uuid.MustParse("55555555-5555-5555-5555-55555555555e"),
		UserID:                userID,
		TariffID:              proID,
		Source:                domain.SubscriptionSourcePaid,
		Status:                domain.SubscriptionStatusActive,
		ValidUntil:            &validUntil,
		AutoRenewEnabled:      true,
		ActivePaymentMethodID: &methodID,
	})
	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:            methodID,
		UserID:        userID,
		Provider:      domain.ProviderFake,
		ProviderToken: "fake_token_1234",
		IsActive:      true,
	}

	d.provider.chargeRes = ChargeResult{ProviderPaymentID: "charge_sensitive", Status: domain.PaymentStatusSucceeded}
	d.provider.chargeErr = errors.New("charge rejected: token=secret123 card 1234-5678-9012-3456 phone +79991234567")
	// Keep the pre-charge status check from short-circuiting so the Charge error
	// path is exercised and logged.
	d.provider.statusRes = domain.PaymentStatusPending

	_, err := d.service.ProcessRenewals(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("ProcessRenewals error: %v", err)
	}

	logs := logBuf.String()
	forbidden := []string{
		"token=secret123",
		"1234-5678-9012-3456",
		"+79991234567",
	}
	for _, s := range forbidden {
		if strings.Contains(logs, s) {
			t.Errorf("log contains sensitive substring %q:\n%s", s, logs)
		}
	}
	if !strings.Contains(logs, "provider charge failed") {
		t.Errorf("expected provider charge failure log, got:\n%s", logs)
	}
}

// --- T-Kassa flow tests ---

func TestBillingService_AddPaymentMethod_TkassaReturnsConfirmURL(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccd")

	d.provider.initAddCardRes = InitAddCardResult{PaymentURL: "https://bank.example/add-card"}

	resp, err := d.service.AddPaymentMethod(context.Background(), userID, AddPaymentMethodRequest{
		ProviderToken: "ignored_for_tkassa",
	})
	if err != nil {
		t.Fatalf("AddPaymentMethod error: %v", err)
	}
	if resp.PaymentMethod != nil {
		t.Error("expected no payment method for tkassa add card flow")
	}
	if resp.ConfirmURL != "https://bank.example/add-card" {
		t.Errorf("expected confirm url https://bank.example/add-card, got %q", resp.ConfirmURL)
	}
	if !d.provider.initAddCardCalled {
		t.Error("expected provider.InitAddCard to be called")
	}
	if d.provider.initAddCardReq.CustomerKey != userID.String() {
		t.Errorf("expected customer key %s, got %s", userID.String(), d.provider.initAddCardReq.CustomerKey)
	}
	if d.provider.initAddCardReq.CheckType != "3DSHOLD" {
		t.Errorf("expected check type 3DSHOLD, got %s", d.provider.initAddCardReq.CheckType)
	}
}

func TestBillingService_ChangeTariff_TkassaFirstPaymentInitFields(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.addTariff(domain.Tariff{
		ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		UserID:           userID,
		TariffID:         basicID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	})

	d.provider.initRes = InitResult{
		ProviderPaymentID: "tkassa_payment_1",
		PaymentURL:        "https://bank.example/pay",
		SavedToken:        "",
	}

	resp, err := d.service.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: string(domain.TariffPro),
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff error: %v", err)
	}
	if resp.ConfirmURL != "https://bank.example/pay" {
		t.Errorf("expected confirm url https://bank.example/pay, got %q", resp.ConfirmURL)
	}

	req := d.provider.initReq
	if req.CustomerKey != userID.String() {
		t.Errorf("expected customer key %s, got %s", userID.String(), req.CustomerKey)
	}
	if !req.Recurrent {
		t.Error("expected recurrent true")
	}
	if req.OperationInitiatorType != "1" {
		t.Errorf("expected initiator type 1, got %s", req.OperationInitiatorType)
	}
	if req.NotificationURL == "" || req.SuccessURL == "" || req.FailURL == "" {
		t.Errorf("expected callback urls set, got notification=%q success=%q fail=%q", req.NotificationURL, req.SuccessURL, req.FailURL)
	}
	if !strings.Contains(req.SuccessURL, resp.PaymentID.String()) {
		t.Errorf("expected success url to contain payment id, got %q", req.SuccessURL)
	}

	payment := d.subscriptionPayments.payments[resp.PaymentID]
	if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID != "tkassa_payment_1" {
		t.Errorf("expected provider payment id set, got %v", payment.ProviderPaymentID)
	}
	if payment.PaymentMethodID != nil {
		t.Errorf("expected no local payment method until webhook, got %v", payment.PaymentMethodID)
	}
}

func TestBillingService_ChangeTariff_TkassaReturnsExistingPendingPaymentURL(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	existingPaymentID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	validUntil := fixedNow.AddDate(0, 1, 0)
	providerPaymentID := "existing_tkassa_payment_1"
	paymentURL := "https://bank.example/pay-existing"

	d.addTariff(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.addTariff(domain.Tariff{
		ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		UserID:           userID,
		TariffID:         basicID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	})
	d.subscriptionPayments.payments[existingPaymentID] = domain.SubscriptionPayment{
		ID:                existingPaymentID,
		UserID:            userID,
		SubscriptionID:    uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		TariffID:          proID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderTkassa,
		ProviderPaymentID: &providerPaymentID,
		PaymentURL:        &paymentURL,
		Status:            domain.PaymentStatusPending,
	}

	resp, err := d.service.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: string(domain.TariffPro),
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff error: %v", err)
	}
	if resp.PaymentID != existingPaymentID {
		t.Errorf("expected existing payment id %s, got %s", existingPaymentID, resp.PaymentID)
	}
	if resp.ConfirmURL != paymentURL {
		t.Errorf("expected confirm url %q, got %q", paymentURL, resp.ConfirmURL)
	}
	if d.provider.initCalled {
		t.Error("expected no provider call when reusing existing pending payment with provider reference")
	}
}

func TestBillingService_ProcessRenewals_TkassaInitChargeFlow(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	methodID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	validUntil := fixedNow

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addSubscription(domain.Subscription{
		ID:                    uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		UserID:                userID,
		TariffID:              proID,
		Source:                domain.SubscriptionSourcePaid,
		Status:                domain.SubscriptionStatusActive,
		ValidUntil:            &validUntil,
		AutoRenewEnabled:      true,
		ActivePaymentMethodID: &methodID,
	})
	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:            methodID,
		UserID:        userID,
		Provider:      domain.ProviderTkassa,
		ProviderToken: "rebill_1234",
		IsActive:      true,
	}
	d.provider.initRes = InitResult{
		ProviderPaymentID: "tkassa_renewal_1",
		PaymentURL:        "https://bank.example/pay",
		SavedToken:        "",
	}
	d.provider.chargeRes = ChargeResult{ProviderPaymentID: "tkassa_renewal_1", Status: domain.PaymentStatusSucceeded}

	count, err := d.service.ProcessRenewals(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("ProcessRenewals error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 renewal, got %d", count)
	}

	initReq := d.provider.initReq
	if initReq.OperationInitiatorType != "R" {
		t.Errorf("expected initiator type R, got %s", initReq.OperationInitiatorType)
	}
	if initReq.CustomerKey != userID.String() {
		t.Errorf("expected customer key %s, got %s", userID.String(), initReq.CustomerKey)
	}
	if initReq.AmountKopecks != 5000 {
		t.Errorf("expected amount 5000, got %d", initReq.AmountKopecks)
	}
	if !initReq.Recurrent {
		t.Errorf("expected Recurrent true for renewal, got false")
	}

	if d.provider.chargeReq.ProviderPaymentID != "tkassa_renewal_1" {
		t.Errorf("expected charge provider payment id tkassa_renewal_1, got %q", d.provider.chargeReq.ProviderPaymentID)
	}
	if d.provider.chargeReq.Token != "rebill_1234" {
		t.Errorf("expected charge token rebill_1234, got %q", d.provider.chargeReq.Token)
	}

	sub := d.subscriptions.subs[userID]
	if sub.Status != domain.SubscriptionStatusActive {
		t.Errorf("expected status active, got %s", sub.Status)
	}
	if sub.ValidUntil == nil || !sub.ValidUntil.After(fixedNow) {
		t.Errorf("expected valid_until extended, got %v", sub.ValidUntil)
	}
}

func TestBillingService_ProcessRenewals_TkassaInitFailureMovesToGrace(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	methodID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	validUntil := fixedNow

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addSubscription(domain.Subscription{
		ID:                    uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		UserID:                userID,
		TariffID:              proID,
		Source:                domain.SubscriptionSourcePaid,
		Status:                domain.SubscriptionStatusActive,
		ValidUntil:            &validUntil,
		AutoRenewEnabled:      true,
		ActivePaymentMethodID: &methodID,
	})
	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:            methodID,
		UserID:        userID,
		Provider:      domain.ProviderTkassa,
		ProviderToken: "rebill_1234",
		IsActive:      true,
	}
	d.provider.initErr = errors.New("tkassa init failed")

	count, err := d.service.ProcessRenewals(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("ProcessRenewals error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 renewal processed, got %d", count)
	}

	sub := d.subscriptions.subs[userID]
	if sub.Status != domain.SubscriptionStatusGrace {
		t.Errorf("expected status grace, got %s", sub.Status)
	}
}

func TestBillingService_HandleWebhook_TkassaAuthorizedThenConfirmed(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777778")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-888888888889")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaab")
	tariffID := uuid.MustParse("99999999-9999-9999-9999-99999999999a")
	providerPaymentID := "tkassa_webhook_1"
	existingValidUntil := fixedNow.AddDate(0, 0, 15)

	d.addTariff(domain.Tariff{
		ID: tariffID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               subscriptionID,
		UserID:           userID,
		TariffID:         tariffID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &existingValidUntil,
		AutoRenewEnabled: true,
	})
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:             paymentID,
		UserID:         userID,
		SubscriptionID: subscriptionID,
		TariffID:       tariffID,
		Period:         domain.PeriodMonth,
		AmountKopecks:  5000,
		Provider:       domain.ProviderTkassa,
		Status:         domain.PaymentStatusPending,
	}

	// AUTHORIZED webhook: creates inactive payment method, keeps payment pending.
	d.provider.parseWebhook = func(_ []byte) (WebhookPayload, error) {
		return WebhookPayload{
			ProviderPaymentID: providerPaymentID,
			InternalPaymentID: paymentID,
			Status:            domain.PaymentStatusPending,
			RebillID:          "rebill_auth",
			CardID:            "card_1",
			Pan:               "430000******0777",
			ExpDate:           "12/30",
			CustomerKey:       userID.String(),
		}, nil
	}

	if err := d.service.HandleWebhook(context.Background(), "tkassa", []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook authorized error: %v", err)
	}

	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusPending {
		t.Errorf("expected payment still pending, got %s", payment.Status)
	}
	if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID != providerPaymentID {
		t.Errorf("expected provider payment id %s, got %v", providerPaymentID, payment.ProviderPaymentID)
	}
	if payment.PaymentMethodID == nil {
		t.Fatal("expected payment method id set by authorized webhook")
	}
	pm := d.paymentMethods.methods[*payment.PaymentMethodID]
	if pm.ProviderToken != "rebill_auth" {
		t.Errorf("expected rebill auth, got %s", pm.ProviderToken)
	}
	if pm.ProviderCardID != "card_1" {
		t.Errorf("expected card id card_1, got %s", pm.ProviderCardID)
	}
	if pm.DisplayMask != "430000******0777" {
		t.Errorf("expected display mask 430000******0777, got %s", pm.DisplayMask)
	}
	if pm.IsActive {
		t.Error("expected payment method to remain inactive after authorized webhook")
	}

	// CONFIRMED webhook: activates method and extends subscription.
	d.provider.parseWebhook = func(_ []byte) (WebhookPayload, error) {
		return WebhookPayload{
			ProviderPaymentID: providerPaymentID,
			InternalPaymentID: paymentID,
			Status:            domain.PaymentStatusSucceeded,
			RebillID:          "rebill_auth",
			CardID:            "card_1",
			Pan:               "430000******0777",
			ExpDate:           "12/30",
			CustomerKey:       userID.String(),
		}, nil
	}

	if err := d.service.HandleWebhook(context.Background(), "tkassa", []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook confirmed error: %v", err)
	}

	payment = d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusSucceeded {
		t.Errorf("expected payment succeeded, got %s", payment.Status)
	}
	if !d.paymentMethods.methods[*payment.PaymentMethodID].IsActive {
		t.Error("expected payment method active after confirmed webhook")
	}

	sub := d.subscriptions.subs[userID]
	wantValidUntil := existingValidUntil.AddDate(0, 1, 0)
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(wantValidUntil) {
		t.Errorf("expected valid until extended to %v, got %v", wantValidUntil, sub.ValidUntil)
	}
}

func TestBillingService_HandleWebhook_TkassaAddCard(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777778")

	d.provider.parseWebhook = func(_ []byte) (WebhookPayload, error) {
		return WebhookPayload{
			NotificationType: "NotificationAddCard",
			CustomerKey:      userID.String(),
			RequestKey:       "req_1",
			RebillID:         "rebill_addcard",
			CardID:           "card_add",
			Pan:              "430000******0777",
			ExpDate:          "12/30",
		}, nil
	}

	if err := d.service.HandleWebhook(context.Background(), "tkassa", []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook AddCard error: %v", err)
	}

	var found bool
	for _, pm := range d.paymentMethods.methods {
		if pm.UserID == userID && pm.ProviderToken == "rebill_addcard" {
			found = true
			if pm.ProviderCardID != "card_add" {
				t.Errorf("expected card id card_add, got %s", pm.ProviderCardID)
			}
			if pm.DisplayMask != "430000******0777" {
				t.Errorf("expected display mask 430000******0777, got %s", pm.DisplayMask)
			}
			if !pm.IsActive {
				t.Error("expected add card payment method to be active")
			}
		}
	}
	if !found {
		t.Error("expected payment method created from AddCard webhook")
	}
}

func TestBillingService_HandleWebhook_AddCardLinksActivePaymentMethodToSubscription(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("77777777-7777-7777-7777-77777777777a")
	tariffID := uuid.MustParse("99999999-9999-9999-9999-99999999997b")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaa7c")
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{
		ID: tariffID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               subscriptionID,
		UserID:           userID,
		TariffID:         tariffID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: true,
	})

	d.provider.parseWebhook = func(_ []byte) (WebhookPayload, error) {
		return WebhookPayload{
			NotificationType: "NotificationAddCard",
			CustomerKey:      userID.String(),
			RequestKey:       "req_link",
			RebillID:         "rebill_link",
			CardID:           "card_link",
			Pan:              "430000******0777",
			ExpDate:          "12/30",
		}, nil
	}

	if err := d.service.HandleWebhook(t.Context(), "tkassa", []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook AddCard error: %v", err)
	}

	var linkedID uuid.UUID
	var found bool
	for _, pm := range d.paymentMethods.methods {
		if pm.UserID == userID && pm.ProviderToken == "rebill_link" {
			linkedID = pm.ID
			found = true
			if !pm.IsActive {
				t.Error("expected add card payment method to be active")
			}
		}
	}
	if !found {
		t.Fatal("expected payment method created from AddCard webhook")
	}

	sub, ok := d.subscriptions.subs[userID]
	if !ok {
		t.Fatal("expected subscription to exist")
	}
	if sub.ActivePaymentMethodID == nil {
		t.Fatalf("expected subscription active payment method to be set to %s, got nil", linkedID)
	}
	if *sub.ActivePaymentMethodID != linkedID {
		t.Errorf("expected subscription active payment method %s, got %s", linkedID, *sub.ActivePaymentMethodID)
	}
}

func TestBillingService_HandleWebhook_TkassaAddCardLegacy(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777779")

	d.provider.parseWebhook = func(_ []byte) (WebhookPayload, error) {
		return WebhookPayload{
			NotificationType: "AddCard",
			CustomerKey:      userID.String(),
			RequestKey:       "req_legacy",
			RebillID:         "rebill_addcard_legacy",
			CardID:           "card_add_legacy",
			Pan:              "430000******0777",
			ExpDate:          "12/30",
		}, nil
	}

	if err := d.service.HandleWebhook(context.Background(), "tkassa", []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook legacy AddCard error: %v", err)
	}

	var found bool
	for _, pm := range d.paymentMethods.methods {
		if pm.UserID == userID && pm.ProviderToken == "rebill_addcard_legacy" {
			found = true
			if pm.ProviderCardID != "card_add_legacy" {
				t.Errorf("expected card id card_add_legacy, got %s", pm.ProviderCardID)
			}
			if !pm.IsActive {
				t.Error("expected legacy add card payment method to be active")
			}
		}
	}
	if !found {
		t.Error("expected payment method created from legacy AddCard webhook")
	}
}

func TestBillingService_HandleWebhook_DuplicateConfirmedIsIdempotent(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777778")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-888888888889")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaab")
	tariffID := uuid.MustParse("99999999-9999-9999-9999-99999999999a")
	providerPaymentID := "tkassa_webhook_dup"
	existingValidUntil := fixedNow.AddDate(0, 0, 15)

	d.addTariff(domain.Tariff{
		ID: tariffID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               subscriptionID,
		UserID:           userID,
		TariffID:         tariffID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &existingValidUntil,
		AutoRenewEnabled: true,
	})
	methodID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:             methodID,
		UserID:         userID,
		Provider:       domain.ProviderTkassa,
		ProviderToken:  "rebill_dup",
		ProviderCardID: "card_dup",
		IsActive:       false,
	}
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    subscriptionID,
		TariffID:          tariffID,
		PaymentMethodID:   &methodID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderTkassa,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
	}

	webhook := func() WebhookPayload {
		return WebhookPayload{
			ProviderPaymentID: providerPaymentID,
			InternalPaymentID: paymentID,
			Status:            domain.PaymentStatusSucceeded,
			RebillID:          "rebill_dup",
			CardID:            "card_dup",
			Pan:               "430000******0777",
			ExpDate:           "12/30",
			CustomerKey:       userID.String(),
		}
	}
	d.provider.parseWebhook = func(_ []byte) (WebhookPayload, error) { return webhook(), nil }

	if err := d.service.HandleWebhook(context.Background(), "tkassa", []byte(`{}`)); err != nil {
		t.Fatalf("first HandleWebhook error: %v", err)
	}
	firstValidUntil := d.subscriptions.subs[userID].ValidUntil

	if err := d.service.HandleWebhook(context.Background(), "tkassa", []byte(`{}`)); err != nil {
		t.Fatalf("second HandleWebhook error: %v", err)
	}
	if !d.subscriptions.subs[userID].ValidUntil.Equal(*firstValidUntil) {
		t.Error("expected duplicate webhook to be idempotent")
	}
}

func TestBillingService_ProcessPendingUpgradePayments_SucceededFinalizesUpgrade(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	methodID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	paymentID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	subscriptionID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	providerPaymentID := "upgrade_1"
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addSubscription(domain.Subscription{
		ID:               subscriptionID,
		UserID:           userID,
		TariffID:         basicID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	})
	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:       methodID,
		UserID:   userID,
		Provider: domain.ProviderFake,
		IsActive: false,
	}
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    subscriptionID,
		TariffID:          proID,
		PaymentMethodID:   &methodID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
		CreatedAt:         fixedNow.Add(-10 * time.Minute),
		UpdatedAt:         fixedNow.Add(-10 * time.Minute),
	}
	d.subscriptionPayments.pendingUpgradePaymentIDs[paymentID] = true
	d.provider.statusRes = domain.PaymentStatusSucceeded

	count, err := d.service.ProcessPendingUpgradePayments(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("ProcessPendingUpgradePayments error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 upgrade processed, got %d", count)
	}

	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusSucceeded {
		t.Errorf("expected payment succeeded, got %s", payment.Status)
	}

	sub := d.subscriptions.subs[userID]
	if sub.TariffID != proID {
		t.Errorf("expected tariff upgraded to pro, got %s", sub.TariffID)
	}
	if sub.ValidUntil == nil || !sub.ValidUntil.After(fixedNow) {
		t.Errorf("expected valid_until extended, got %v", sub.ValidUntil)
	}
	if !sub.AutoRenewEnabled {
		t.Error("expected auto-renew enabled after upgrade")
	}
	if sub.ActivePaymentMethodID == nil || *sub.ActivePaymentMethodID != methodID {
		t.Errorf("expected active payment method %s, got %v", methodID, sub.ActivePaymentMethodID)
	}
}

func TestBillingService_ProcessPendingUpgradePayments_FailedMarksFailed(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111112")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222223")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333334")
	paymentID := uuid.MustParse("55555555-5555-5555-5555-555555555556")
	subscriptionID := uuid.MustParse("66666666-6666-6666-6666-666666666667")
	providerPaymentID := "upgrade_2"
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addSubscription(domain.Subscription{
		ID:               subscriptionID,
		UserID:           userID,
		TariffID:         basicID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	})
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    subscriptionID,
		TariffID:          proID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
		CreatedAt:         fixedNow.Add(-10 * time.Minute),
		UpdatedAt:         fixedNow.Add(-10 * time.Minute),
	}
	d.subscriptionPayments.pendingUpgradePaymentIDs[paymentID] = true
	d.provider.statusRes = domain.PaymentStatusFailed

	count, err := d.service.ProcessPendingUpgradePayments(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("ProcessPendingUpgradePayments error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 upgrade processed, got %d", count)
	}

	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusFailed {
		t.Errorf("expected payment failed, got %s", payment.Status)
	}

	sub := d.subscriptions.subs[userID]
	if sub.TariffID != basicID {
		t.Errorf("expected subscription tariff unchanged, got %s", sub.TariffID)
	}
}

func TestBillingService_ProcessPendingUpgradePayments_PendingLeavesAlone(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111113")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222224")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333335")
	paymentID := uuid.MustParse("55555555-5555-5555-5555-555555555557")
	subscriptionID := uuid.MustParse("66666666-6666-6666-6666-666666666668")
	providerPaymentID := "upgrade_3"
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addSubscription(domain.Subscription{
		ID:               subscriptionID,
		UserID:           userID,
		TariffID:         basicID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	})
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    subscriptionID,
		TariffID:          proID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
		CreatedAt:         fixedNow.Add(-10 * time.Minute),
		UpdatedAt:         fixedNow.Add(-10 * time.Minute),
	}
	d.subscriptionPayments.pendingUpgradePaymentIDs[paymentID] = true
	d.provider.statusRes = domain.PaymentStatusPending

	count, err := d.service.ProcessPendingUpgradePayments(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("ProcessPendingUpgradePayments error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 upgrade checked, got %d", count)
	}
	if d.provider.chargeCalled {
		t.Error("expected provider.Charge not to be called")
	}

	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusPending {
		t.Errorf("expected payment still pending, got %s", payment.Status)
	}

	sub := d.subscriptions.subs[userID]
	if sub.TariffID != basicID {
		t.Errorf("expected subscription tariff unchanged, got %s", sub.TariffID)
	}
}

func TestBillingService_DeletePaymentMethod_TkassaRemoveCardFailureIsBestEffort(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	methodID := uuid.MustParse("11111111-1111-1111-1111-111111111113")

	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:             methodID,
		UserID:         userID,
		Provider:       domain.ProviderTkassa,
		ProviderCardID: "card_123",
	}
	d.provider.removeCardErr = errors.New("provider remove failed")

	if err := d.service.DeletePaymentMethod(context.Background(), userID, methodID); err != nil {
		t.Fatalf("expected RemoveCard failure to be best-effort, got error: %v", err)
	}
	if !d.provider.removeCardCalled {
		t.Error("expected provider.RemoveCard to be called")
	}
	if d.provider.removeCardCustomer != userID.String() {
		t.Errorf("expected customer key %s, got %s", userID.String(), d.provider.removeCardCustomer)
	}
	if d.provider.removeCardCardID != "card_123" {
		t.Errorf("expected card id card_123, got %s", d.provider.removeCardCardID)
	}
	if _, ok := d.paymentMethods.methods[methodID]; ok {
		t.Error("expected local payment method to be deleted even when provider remove fails")
	}
	if d.beginner.begun != 1 || d.beginner.committed != 1 {
		t.Errorf("expected one committed transaction, got begun=%d committed=%d", d.beginner.begun, d.beginner.committed)
	}
}

func TestBillingService_RefundPayment_SucceedsAndDowngradesToBasic(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-77777777777b")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-88888888888c")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaf")
	basicID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	proID := uuid.MustParse("00000000-0000-0000-0000-000000000004")
	providerPaymentID := "stub_refund"
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.addTariff(domain.Tariff{
		ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               subscriptionID,
		UserID:           userID,
		TariffID:         proID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: true,
	})
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    subscriptionID,
		TariffID:          proID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusSucceeded,
		SucceededAt:       &validUntil,
	}
	d.provider.cancelRes = CancelResult{
		ProviderPaymentID:     providerPaymentID,
		Status:                domain.PaymentStatusRefunded,
		RefundedAmountKopecks: 5000,
	}

	if err := d.service.RefundPayment(t.Context(), paymentID); err != nil {
		t.Fatalf("RefundPayment error: %v", err)
	}

	if !d.provider.cancelCalled {
		t.Error("expected provider.Cancel to be called")
	}
	if d.provider.cancelReq.ProviderPaymentID != providerPaymentID {
		t.Errorf("expected cancel provider payment id %s, got %s", providerPaymentID, d.provider.cancelReq.ProviderPaymentID)
	}
	if d.provider.cancelReq.AmountKopecks != 5000 {
		t.Errorf("expected cancel amount 5000, got %d", d.provider.cancelReq.AmountKopecks)
	}

	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusRefunded {
		t.Errorf("expected payment refunded, got %s", payment.Status)
	}
	if payment.RefundedAmountKopecks == nil || *payment.RefundedAmountKopecks != 5000 {
		t.Errorf("expected refunded amount 5000, got %v", payment.RefundedAmountKopecks)
	}

	sub := d.subscriptions.subs[userID]
	if sub.TariffID != basicID {
		t.Errorf("expected subscription downgraded to basic, got tariff %s", sub.TariffID)
	}
	if sub.Status != domain.SubscriptionStatusActive {
		t.Errorf("expected subscription status active, got %s", sub.Status)
	}
	if sub.ValidUntil != nil {
		t.Errorf("expected valid_until nil after refund, got %v", sub.ValidUntil)
	}
	if sub.AutoRenewEnabled {
		t.Error("expected auto_renew disabled after refund")
	}
	if len(d.propertyArchiver.calls) != 1 || d.propertyArchiver.calls[0].limit != 5 {
		t.Errorf("expected property archiver called with limit 5, got %v", d.propertyArchiver.calls)
	}
}

func TestBillingService_RefundPayment_PendingPaymentSucceeds(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-77777777777b")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-88888888888c")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaf")
	basicID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	proID := uuid.MustParse("00000000-0000-0000-0000-000000000004")
	providerPaymentID := "stub_refund_pending"

	d.addTariff(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.addTariff(domain.Tariff{
		ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:       subscriptionID,
		UserID:   userID,
		TariffID: proID,
		Source:   domain.SubscriptionSourcePaid,
		Status:   domain.SubscriptionStatusActive,
	})
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    subscriptionID,
		TariffID:          proID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
	}
	d.provider.cancelRes = CancelResult{
		ProviderPaymentID:     providerPaymentID,
		Status:                domain.PaymentStatusRefunded,
		RefundedAmountKopecks: 5000,
	}

	if err := d.service.RefundPayment(t.Context(), paymentID); err != nil {
		t.Fatalf("RefundPayment error: %v", err)
	}

	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusRefunded {
		t.Errorf("expected payment refunded, got %s", payment.Status)
	}
	if payment.RefundedAmountKopecks == nil || *payment.RefundedAmountKopecks != 5000 {
		t.Errorf("expected refunded amount 5000, got %v", payment.RefundedAmountKopecks)
	}

	sub := d.subscriptions.subs[userID]
	if sub.TariffID != basicID {
		t.Errorf("expected subscription downgraded to basic, got tariff %s", sub.TariffID)
	}
}

func TestBillingService_RefundPayment_FullRefund(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-77777777777c")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-88888888888d")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaf")
	basicID := uuid.MustParse("00000000-0000-0000-0000-000000000005")
	proID := uuid.MustParse("00000000-0000-0000-0000-000000000006")
	providerPaymentID := "stub_full_refund"
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{
		ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000,
	})
	d.addTariff(domain.Tariff{
		ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               subscriptionID,
		UserID:           userID,
		TariffID:         proID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: true,
	})
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    subscriptionID,
		TariffID:          proID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusSucceeded,
		SucceededAt:       &validUntil,
	}
	d.provider.cancelRes = CancelResult{
		ProviderPaymentID:     providerPaymentID,
		Status:                domain.PaymentStatusRefunded,
		RefundedAmountKopecks: 5000,
	}

	if err := d.service.RefundPayment(t.Context(), paymentID); err != nil {
		t.Fatalf("RefundPayment error: %v", err)
	}

	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusRefunded {
		t.Errorf("expected payment refunded, got %s", payment.Status)
	}
	if payment.RefundedAmountKopecks == nil || *payment.RefundedAmountKopecks != 5000 {
		t.Errorf("expected refunded amount 5000, got %v", payment.RefundedAmountKopecks)
	}

	sub := d.subscriptions.subs[userID]
	if sub.TariffID != basicID {
		t.Errorf("expected subscription downgraded to basic, got tariff %s", sub.TariffID)
	}
}

func TestBillingService_RefundPayment_RejectedForNonSucceededOrPendingPayment(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-77777777777d")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-88888888888e")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaab0")
	proID := uuid.MustParse("00000000-0000-0000-0000-000000000007")
	providerPaymentID := "stub_refund_pending"

	d.addSubscription(domain.Subscription{
		ID:       subscriptionID,
		UserID:   userID,
		TariffID: proID,
		Source:   domain.SubscriptionSourcePaid,
		Status:   domain.SubscriptionStatusActive,
	})
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    subscriptionID,
		TariffID:          proID,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusFailed,
	}

	err := d.service.RefundPayment(t.Context(), paymentID)
	if !errors.Is(err, domain.ErrInvalidPaymentStatus) {
		t.Fatalf("expected ErrInvalidPaymentStatus, got %v", err)
	}
	if d.provider.cancelCalled {
		t.Error("expected provider.Cancel not to be called for non-succeeded/non-pending payment")
	}
}

func TestBillingService_RefundPayment_ProviderCancelError(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-77777777777f")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-88888888888a")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaab2")
	proID := uuid.MustParse("00000000-0000-0000-0000-000000000009")
	providerPaymentID := "stub_refund_cancel_error"
	cancelErr := errors.New("provider refused refund")

	d.addSubscription(domain.Subscription{
		ID:       subscriptionID,
		UserID:   userID,
		TariffID: proID,
		Source:   domain.SubscriptionSourcePaid,
		Status:   domain.SubscriptionStatusActive,
	})
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    subscriptionID,
		TariffID:          proID,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusSucceeded,
	}
	d.provider.cancelErr = cancelErr

	err := d.service.RefundPayment(t.Context(), paymentID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, cancelErr) {
		t.Errorf("expected provider cancel error, got %v", err)
	}
	if d.subscriptionPayments.payments[paymentID].Status != domain.PaymentStatusSucceeded {
		t.Errorf("expected payment status to remain succeeded, got %s", d.subscriptionPayments.payments[paymentID].Status)
	}
}

func TestBillingService_RefundPayment_RejectedWhenProviderPaymentIDMissing(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-77777777777e")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-88888888888b")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaab3")
	proID := uuid.MustParse("00000000-0000-0000-0000-00000000000a")

	d.addSubscription(domain.Subscription{
		ID:       subscriptionID,
		UserID:   userID,
		TariffID: proID,
		Source:   domain.SubscriptionSourcePaid,
		Status:   domain.SubscriptionStatusActive,
	})
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:             paymentID,
		UserID:         userID,
		SubscriptionID: subscriptionID,
		TariffID:       proID,
		AmountKopecks:  5000,
		Provider:       domain.ProviderFake,
		Status:         domain.PaymentStatusSucceeded,
	}

	err := d.service.RefundPayment(t.Context(), paymentID)
	if !errors.Is(err, domain.ErrInvalidPaymentStatus) {
		t.Fatalf("expected ErrInvalidPaymentStatus, got %v", err)
	}
	if d.provider.cancelCalled {
		t.Error("expected provider.Cancel not to be called when provider payment id is missing")
	}
}

func TestBillingService_SyncPendingPayment(t *testing.T) {
	type syncCase struct {
		name               string
		setup              func(*testDeps) uuid.UUID
		providerStatus     domain.PaymentStatus
		providerErr        error
		wantProviderCalled bool
		wantErr            func(error) bool
		assert             func(*testing.T, *testDeps, uuid.UUID)
	}

	runSyncCases := func(t *testing.T, cases []syncCase) {
		t.Helper()
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				d := newTestDeps(t)
				paymentID := tc.setup(d)
				d.provider.statusRes = tc.providerStatus
				d.provider.statusErr = tc.providerErr

				err := d.service.SyncPendingPayment(context.Background(), paymentID)

				if tc.wantErr != nil {
					if !tc.wantErr(err) {
						t.Fatalf("unexpected error: %v", err)
					}
				} else if err != nil {
					t.Fatalf("SyncPendingPayment error: %v", err)
				}
				if d.provider.statusCalled != tc.wantProviderCalled {
					t.Errorf("provider.Status called = %v, want %v", d.provider.statusCalled, tc.wantProviderCalled)
				}
				tc.assert(t, d, paymentID)
			})
		}
	}

	t.Run("succeeded", func(t *testing.T) {
		runSyncCases(t, []syncCase{
			{
				name: "finalizes upgrade",
				setup: func(d *testDeps) uuid.UUID {
					userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
					basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
					proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
					methodID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
					paymentID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
					subscriptionID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
					providerPaymentID := "sync_upgrade_1"
					validUntil := fixedNow.AddDate(0, 1, 0)

					d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000})
					d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
					d.addSubscription(domain.Subscription{
						ID:               subscriptionID,
						UserID:           userID,
						TariffID:         basicID,
						Source:           domain.SubscriptionSourcePaid,
						Status:           domain.SubscriptionStatusActive,
						ValidUntil:       &validUntil,
						AutoRenewEnabled: false,
					})
					d.paymentMethods.methods[methodID] = domain.PaymentMethod{ID: methodID, UserID: userID, Provider: domain.ProviderFake}
					d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
						ID:                paymentID,
						UserID:            userID,
						SubscriptionID:    subscriptionID,
						TariffID:          proID,
						PaymentMethodID:   &methodID,
						Period:            domain.PeriodMonth,
						AmountKopecks:     5000,
						Provider:          domain.ProviderFake,
						ProviderPaymentID: &providerPaymentID,
						Status:            domain.PaymentStatusPending,
						CreatedAt:         fixedNow.Add(-10 * time.Minute),
						UpdatedAt:         fixedNow.Add(-10 * time.Minute),
					}
					return paymentID
				},
				providerStatus:     domain.PaymentStatusSucceeded,
				wantProviderCalled: true,
				assert: func(t *testing.T, d *testDeps, paymentID uuid.UUID) {
					payment := d.subscriptionPayments.payments[paymentID]
					if payment.Status != domain.PaymentStatusSucceeded {
						t.Errorf("expected payment succeeded, got %s", payment.Status)
					}
					sub := d.subscriptions.subs[payment.UserID]
					proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
					methodID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
					if sub.TariffID != proID {
						t.Errorf("expected tariff upgraded to pro, got %s", sub.TariffID)
					}
					if sub.ValidUntil == nil || !sub.ValidUntil.After(fixedNow) {
						t.Errorf("expected valid_until extended, got %v", sub.ValidUntil)
					}
					if !sub.AutoRenewEnabled {
						t.Error("expected auto-renew enabled after upgrade")
					}
					if sub.ActivePaymentMethodID == nil || *sub.ActivePaymentMethodID != methodID {
						t.Errorf("expected active payment method %s, got %v", methodID, sub.ActivePaymentMethodID)
					}
				},
			},
			{
				name: "finalizes renewal",
				setup: func(d *testDeps) uuid.UUID {
					userID := uuid.MustParse("11111111-1111-1111-1111-111111111112")
					proID := uuid.MustParse("22222222-2222-2222-2222-222222222223")
					paymentID := uuid.MustParse("33333333-3333-3333-3333-333333333334")
					subscriptionID := uuid.MustParse("44444444-4444-4444-4444-444444444445")
					providerPaymentID := "sync_renewal_1"
					validUntil := fixedNow.Add(-time.Hour)

					d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
					d.addSubscription(domain.Subscription{
						ID:               subscriptionID,
						UserID:           userID,
						TariffID:         proID,
						Source:           domain.SubscriptionSourcePaid,
						Status:           domain.SubscriptionStatusActive,
						ValidUntil:       &validUntil,
						AutoRenewEnabled: true,
					})
					d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
						ID:                paymentID,
						UserID:            userID,
						SubscriptionID:    subscriptionID,
						TariffID:          proID,
						Period:            domain.PeriodMonth,
						AmountKopecks:     5000,
						Provider:          domain.ProviderFake,
						ProviderPaymentID: &providerPaymentID,
						Status:            domain.PaymentStatusPending,
						CreatedAt:         fixedNow.Add(-10 * time.Minute),
						UpdatedAt:         fixedNow.Add(-10 * time.Minute),
					}
					return paymentID
				},
				providerStatus:     domain.PaymentStatusSucceeded,
				wantProviderCalled: true,
				assert: func(t *testing.T, d *testDeps, paymentID uuid.UUID) {
					payment := d.subscriptionPayments.payments[paymentID]
					if payment.Status != domain.PaymentStatusSucceeded {
						t.Errorf("expected payment succeeded, got %s", payment.Status)
					}
					sub := d.subscriptions.subs[payment.UserID]
					proID := uuid.MustParse("22222222-2222-2222-2222-222222222223")
					if sub.TariffID != proID {
						t.Errorf("expected tariff unchanged, got %s", sub.TariffID)
					}
					if sub.ValidUntil == nil || !sub.ValidUntil.Equal(fixedNow.AddDate(0, 1, 0)) {
						t.Errorf("expected valid_until extended to %v, got %v", fixedNow.AddDate(0, 1, 0), sub.ValidUntil)
					}
				},
			},
		})
	})

	t.Run("failed", func(t *testing.T) {
		runSyncCases(t, []syncCase{
			{
				name: "moves renewal to grace",
				setup: func(d *testDeps) uuid.UUID {
					userID := uuid.MustParse("11111111-1111-1111-1111-111111111113")
					proID := uuid.MustParse("22222222-2222-2222-2222-222222222224")
					paymentID := uuid.MustParse("33333333-3333-3333-3333-333333333335")
					subscriptionID := uuid.MustParse("44444444-4444-4444-4444-444444444446")
					providerPaymentID := "sync_renewal_failed"
					validUntil := fixedNow.AddDate(0, 1, 0)

					d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
					d.addSubscription(domain.Subscription{
						ID:               subscriptionID,
						UserID:           userID,
						TariffID:         proID,
						Source:           domain.SubscriptionSourcePaid,
						Status:           domain.SubscriptionStatusActive,
						ValidUntil:       &validUntil,
						AutoRenewEnabled: true,
					})
					d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
						ID:                paymentID,
						UserID:            userID,
						SubscriptionID:    subscriptionID,
						TariffID:          proID,
						Period:            domain.PeriodMonth,
						AmountKopecks:     5000,
						Provider:          domain.ProviderFake,
						ProviderPaymentID: &providerPaymentID,
						Status:            domain.PaymentStatusPending,
						CreatedAt:         fixedNow.Add(-10 * time.Minute),
						UpdatedAt:         fixedNow.Add(-10 * time.Minute),
					}
					return paymentID
				},
				providerStatus:     domain.PaymentStatusFailed,
				wantProviderCalled: true,
				assert: func(t *testing.T, d *testDeps, paymentID uuid.UUID) {
					payment := d.subscriptionPayments.payments[paymentID]
					if payment.Status != domain.PaymentStatusFailed {
						t.Errorf("expected payment failed, got %s", payment.Status)
					}
					sub := d.subscriptions.subs[payment.UserID]
					if sub.Status != domain.SubscriptionStatusGrace {
						t.Errorf("expected subscription in grace, got %s", sub.Status)
					}
					if sub.ValidUntil == nil || !sub.ValidUntil.After(fixedNow) {
						t.Errorf("expected grace valid_until in the future, got %v", sub.ValidUntil)
					}
				},
			},
			{
				name: "leaves subscription unchanged for upgrade",
				setup: func(d *testDeps) uuid.UUID {
					userID := uuid.MustParse("11111111-1111-1111-1111-111111111114")
					basicID := uuid.MustParse("22222222-2222-2222-2222-222222222225")
					proID := uuid.MustParse("33333333-3333-3333-3333-333333333336")
					paymentID := uuid.MustParse("44444444-4444-4444-4444-444444444447")
					subscriptionID := uuid.MustParse("55555555-5555-5555-5555-555555555558")
					providerPaymentID := "sync_upgrade_failed"
					validUntil := fixedNow.AddDate(0, 1, 0)

					d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000})
					d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
					d.addSubscription(domain.Subscription{
						ID:               subscriptionID,
						UserID:           userID,
						TariffID:         basicID,
						Source:           domain.SubscriptionSourcePaid,
						Status:           domain.SubscriptionStatusActive,
						ValidUntil:       &validUntil,
						AutoRenewEnabled: false,
					})
					d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
						ID:                paymentID,
						UserID:            userID,
						SubscriptionID:    subscriptionID,
						TariffID:          proID,
						Period:            domain.PeriodMonth,
						AmountKopecks:     5000,
						Provider:          domain.ProviderFake,
						ProviderPaymentID: &providerPaymentID,
						Status:            domain.PaymentStatusPending,
						CreatedAt:         fixedNow.Add(-10 * time.Minute),
						UpdatedAt:         fixedNow.Add(-10 * time.Minute),
					}
					return paymentID
				},
				providerStatus:     domain.PaymentStatusFailed,
				wantProviderCalled: true,
				assert: func(t *testing.T, d *testDeps, paymentID uuid.UUID) {
					payment := d.subscriptionPayments.payments[paymentID]
					if payment.Status != domain.PaymentStatusFailed {
						t.Errorf("expected payment failed, got %s", payment.Status)
					}
					sub := d.subscriptions.subs[payment.UserID]
					basicID := uuid.MustParse("22222222-2222-2222-2222-222222222225")
					if sub.TariffID != basicID {
						t.Errorf("expected subscription tariff unchanged, got %s", sub.TariffID)
					}
					if sub.Status != domain.SubscriptionStatusActive {
						t.Errorf("expected subscription active, got %s", sub.Status)
					}
				},
			},
		})
	})

	t.Run("refunded", func(t *testing.T) {
		runSyncCases(t, []syncCase{
			{
				name: "downgrades subscription to basic for full refund",
				setup: func(d *testDeps) uuid.UUID {
					userID := uuid.MustParse("11111111-1111-1111-1111-11111111111a")
					basicID := uuid.MustParse("22222222-2222-2222-2222-22222222222c")
					proID := uuid.MustParse("33333333-3333-3333-3333-33333333333d")
					paymentID := uuid.MustParse("44444444-4444-4444-4444-44444444444e")
					subscriptionID := uuid.MustParse("55555555-5555-5555-5555-55555555555f")
					providerPaymentID := "sync_refunded"
					validUntil := fixedNow.AddDate(0, 1, 0)

					d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000})
					d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
					d.addSubscription(domain.Subscription{
						ID:               subscriptionID,
						UserID:           userID,
						TariffID:         proID,
						Source:           domain.SubscriptionSourcePaid,
						Status:           domain.SubscriptionStatusActive,
						ValidUntil:       &validUntil,
						AutoRenewEnabled: true,
					})
					d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
						ID:                paymentID,
						UserID:            userID,
						SubscriptionID:    subscriptionID,
						TariffID:          proID,
						Period:            domain.PeriodMonth,
						AmountKopecks:     5000,
						Provider:          domain.ProviderFake,
						ProviderPaymentID: &providerPaymentID,
						Status:            domain.PaymentStatusPending,
						CreatedAt:         fixedNow.Add(-10 * time.Minute),
						UpdatedAt:         fixedNow.Add(-10 * time.Minute),
					}
					return paymentID
				},
				providerStatus:     domain.PaymentStatusRefunded,
				wantProviderCalled: true,
				assert: func(t *testing.T, d *testDeps, paymentID uuid.UUID) {
					payment := d.subscriptionPayments.payments[paymentID]
					if payment.Status != domain.PaymentStatusRefunded {
						t.Errorf("expected payment refunded, got %s", payment.Status)
					}
					if payment.RefundedAmountKopecks == nil || *payment.RefundedAmountKopecks != 5000 {
						t.Errorf("expected refunded amount 5000, got %v", payment.RefundedAmountKopecks)
					}
					sub := d.subscriptions.subs[payment.UserID]
					basicID := uuid.MustParse("22222222-2222-2222-2222-22222222222c")
					if sub.TariffID != basicID {
						t.Errorf("expected subscription downgraded to basic, got %s", sub.TariffID)
					}
					if sub.Status != domain.SubscriptionStatusActive {
						t.Errorf("expected subscription active after downgrade, got %s", sub.Status)
					}
					if sub.AutoRenewEnabled {
						t.Error("expected auto-renew disabled after refund downgrade")
					}
					if len(d.propertyArchiver.calls) != 1 {
						t.Errorf("expected property archiver called once, got %d", len(d.propertyArchiver.calls))
					}
				},
			},
		})
	})

	t.Run("partial_refunded recorded as full refund", func(t *testing.T) {
		runSyncCases(t, []syncCase{
			{
				name: "downgrades subscription to basic and records external partial refund as full refund",
				setup: func(d *testDeps) uuid.UUID {
					userID := uuid.MustParse("11111111-1111-1111-1111-11111111111b")
					basicID := uuid.MustParse("22222222-2222-2222-2222-22222222222d")
					proID := uuid.MustParse("33333333-3333-3333-3333-33333333333e")
					paymentID := uuid.MustParse("44444444-4444-4444-4444-44444444444f")
					subscriptionID := uuid.MustParse("55555555-5555-5555-5555-555555555550")
					providerPaymentID := "sync_partial_refunded"
					validUntil := fixedNow.AddDate(0, 1, 0)

					d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000})
					d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
					d.addSubscription(domain.Subscription{
						ID:               subscriptionID,
						UserID:           userID,
						TariffID:         proID,
						Source:           domain.SubscriptionSourcePaid,
						Status:           domain.SubscriptionStatusActive,
						ValidUntil:       &validUntil,
						AutoRenewEnabled: true,
					})
					d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
						ID:                paymentID,
						UserID:            userID,
						SubscriptionID:    subscriptionID,
						TariffID:          proID,
						Period:            domain.PeriodMonth,
						AmountKopecks:     5000,
						Provider:          domain.ProviderFake,
						ProviderPaymentID: &providerPaymentID,
						Status:            domain.PaymentStatusPending,
						CreatedAt:         fixedNow.Add(-10 * time.Minute),
						UpdatedAt:         fixedNow.Add(-10 * time.Minute),
					}
					return paymentID
				},
				providerStatus:     domain.PaymentStatusPartialRefunded,
				wantProviderCalled: true,
				assert: func(t *testing.T, d *testDeps, paymentID uuid.UUID) {
					payment := d.subscriptionPayments.payments[paymentID]
					// Without a real refund amount from GetState, the synthetic payload
					// uses the full payment amount, so the payment is recorded as refunded.
					if payment.Status != domain.PaymentStatusRefunded {
						t.Errorf("expected payment recorded as refunded, got %s", payment.Status)
					}
					if payment.RefundedAmountKopecks == nil || *payment.RefundedAmountKopecks != 5000 {
						t.Errorf("expected refunded amount 5000, got %v", payment.RefundedAmountKopecks)
					}
					sub := d.subscriptions.subs[payment.UserID]
					basicID := uuid.MustParse("22222222-2222-2222-2222-22222222222d")
					if sub.TariffID != basicID {
						t.Errorf("expected subscription downgraded to basic, got %s", sub.TariffID)
					}
					if sub.Status != domain.SubscriptionStatusActive {
						t.Errorf("expected subscription active after downgrade, got %s", sub.Status)
					}
					if sub.AutoRenewEnabled {
						t.Error("expected auto-renew disabled after refund downgrade")
					}
					if len(d.propertyArchiver.calls) != 1 {
						t.Errorf("expected property archiver called once, got %d", len(d.propertyArchiver.calls))
					}
				},
			},
		})
	})

	t.Run("pending or already finalized", func(t *testing.T) {
		runSyncCases(t, []syncCase{
			{
				name: "leaves pending payment alone",
				setup: func(d *testDeps) uuid.UUID {
					userID := uuid.MustParse("11111111-1111-1111-1111-111111111115")
					basicID := uuid.MustParse("22222222-2222-2222-2222-222222222226")
					paymentID := uuid.MustParse("33333333-3333-3333-3333-333333333337")
					subscriptionID := uuid.MustParse("44444444-4444-4444-4444-444444444448")
					providerPaymentID := "sync_pending"
					validUntil := fixedNow.AddDate(0, 1, 0)

					d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000})
					d.addSubscription(domain.Subscription{
						ID:               subscriptionID,
						UserID:           userID,
						TariffID:         basicID,
						Source:           domain.SubscriptionSourcePaid,
						Status:           domain.SubscriptionStatusActive,
						ValidUntil:       &validUntil,
						AutoRenewEnabled: false,
					})
					d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
						ID:                paymentID,
						UserID:            userID,
						SubscriptionID:    subscriptionID,
						TariffID:          basicID,
						Period:            domain.PeriodMonth,
						AmountKopecks:     1000,
						Provider:          domain.ProviderFake,
						ProviderPaymentID: &providerPaymentID,
						Status:            domain.PaymentStatusPending,
						CreatedAt:         fixedNow.Add(-10 * time.Minute),
						UpdatedAt:         fixedNow.Add(-10 * time.Minute),
					}
					return paymentID
				},
				providerStatus:     domain.PaymentStatusPending,
				wantProviderCalled: true,
				assert: func(t *testing.T, d *testDeps, paymentID uuid.UUID) {
					payment := d.subscriptionPayments.payments[paymentID]
					if payment.Status != domain.PaymentStatusPending {
						t.Errorf("expected payment still pending, got %s", payment.Status)
					}
				},
			},
			{
				name: "already finalized returns invalid status error",
				setup: func(d *testDeps) uuid.UUID {
					userID := uuid.MustParse("11111111-1111-1111-1111-111111111117")
					basicID := uuid.MustParse("22222222-2222-2222-2222-222222222228")
					paymentID := uuid.MustParse("33333333-3333-3333-3333-333333333339")
					subscriptionID := uuid.MustParse("44444444-4444-4444-4444-44444444444a")
					providerPaymentID := "sync_finalized"

					d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000})
					d.addSubscription(domain.Subscription{
						ID:       subscriptionID,
						UserID:   userID,
						TariffID: basicID,
						Source:   domain.SubscriptionSourcePaid,
						Status:   domain.SubscriptionStatusActive,
					})
					d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
						ID:                paymentID,
						UserID:            userID,
						SubscriptionID:    subscriptionID,
						TariffID:          basicID,
						Period:            domain.PeriodMonth,
						AmountKopecks:     1000,
						Provider:          domain.ProviderFake,
						ProviderPaymentID: &providerPaymentID,
						Status:            domain.PaymentStatusSucceeded,
						CreatedAt:         fixedNow.Add(-10 * time.Minute),
						UpdatedAt:         fixedNow.Add(-10 * time.Minute),
					}
					return paymentID
				},
				providerStatus:     domain.PaymentStatusPending,
				wantProviderCalled: false,
				wantErr:            func(err error) bool { return errors.Is(err, domain.ErrInvalidPaymentStatus) },
				assert: func(t *testing.T, d *testDeps, paymentID uuid.UUID) {
					payment := d.subscriptionPayments.payments[paymentID]
					if payment.Status != domain.PaymentStatusSucceeded {
						t.Errorf("expected payment status unchanged, got %s", payment.Status)
					}
				},
			},
		})
	})

	t.Run("missing provider id", func(t *testing.T) {
		runSyncCases(t, []syncCase{
			{
				name: "returns invalid status error",
				setup: func(d *testDeps) uuid.UUID {
					userID := uuid.MustParse("11111111-1111-1111-1111-111111111116")
					basicID := uuid.MustParse("22222222-2222-2222-2222-222222222227")
					paymentID := uuid.MustParse("33333333-3333-3333-3333-333333333338")
					subscriptionID := uuid.MustParse("44444444-4444-4444-4444-444444444449")
					validUntil := fixedNow.AddDate(0, 1, 0)

					d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000})
					d.addSubscription(domain.Subscription{
						ID:               subscriptionID,
						UserID:           userID,
						TariffID:         basicID,
						Source:           domain.SubscriptionSourcePaid,
						Status:           domain.SubscriptionStatusActive,
						ValidUntil:       &validUntil,
						AutoRenewEnabled: false,
					})
					d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
						ID:             paymentID,
						UserID:         userID,
						SubscriptionID: subscriptionID,
						TariffID:       basicID,
						Period:         domain.PeriodMonth,
						AmountKopecks:  1000,
						Provider:       domain.ProviderFake,
						Status:         domain.PaymentStatusPending,
						CreatedAt:      fixedNow.Add(-10 * time.Minute),
						UpdatedAt:      fixedNow.Add(-10 * time.Minute),
					}
					return paymentID
				},
				wantProviderCalled: false,
				wantErr:            func(err error) bool { return errors.Is(err, domain.ErrInvalidPaymentStatus) },
				assert:             func(*testing.T, *testDeps, uuid.UUID) {},
			},
		})
	})
}

func TestBillingService_SyncPendingPayment_UnexpectedProviderStatus(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111118")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222229")
	paymentID := uuid.MustParse("33333333-3333-3333-3333-33333333333a")
	subscriptionID := uuid.MustParse("44444444-4444-4444-4444-44444444444b")
	providerPaymentID := "sync_unexpected"
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000})
	d.addSubscription(domain.Subscription{
		ID:               subscriptionID,
		UserID:           userID,
		TariffID:         basicID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	})
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    subscriptionID,
		TariffID:          basicID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     1000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
		CreatedAt:         fixedNow.Add(-10 * time.Minute),
		UpdatedAt:         fixedNow.Add(-10 * time.Minute),
	}
	d.provider.statusRes = domain.PaymentStatus("unknown")

	err := d.service.SyncPendingPayment(context.Background(), paymentID)
	if err == nil {
		t.Fatal("expected error for unexpected provider status")
	}
	if !strings.Contains(err.Error(), "unexpected provider status") {
		t.Fatalf("expected unexpected provider status error, got %v", err)
	}
	if !d.provider.statusCalled {
		t.Error("expected provider.Status to be called")
	}
}

func TestBillingService_SyncPendingPayment_DoubleFinalizeGuard(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111119")
	basicID := uuid.MustParse("22222222-2222-2222-2222-22222222222a")
	paymentID := uuid.MustParse("33333333-3333-3333-3333-33333333333b")
	subscriptionID := uuid.MustParse("44444444-4444-4444-4444-44444444444c")
	providerPaymentID := "double_finalize"
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000})
	d.addSubscription(domain.Subscription{
		ID:               subscriptionID,
		UserID:           userID,
		TariffID:         basicID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	})
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    subscriptionID,
		TariffID:          basicID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     1000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
		CreatedAt:         fixedNow.Add(-10 * time.Minute),
		UpdatedAt:         fixedNow.Add(-10 * time.Minute),
	}
	d.subscriptionPayments.forUpdateStatus = map[uuid.UUID]domain.PaymentStatus{paymentID: domain.PaymentStatusSucceeded}
	d.provider.statusRes = domain.PaymentStatusSucceeded

	if err := d.service.SyncPendingPayment(context.Background(), paymentID); err != nil {
		t.Fatalf("SyncPendingPayment error: %v", err)
	}
	if !d.provider.statusCalled {
		t.Error("expected provider.Status to be called")
	}
	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusPending {
		t.Errorf("expected payment to remain pending after guard, got %s", payment.Status)
	}
}

func TestBillingService_ReconcilePendingPayments_SyncsInBatches(t *testing.T) {
	basicID := uuid.MustParse("22222222-2222-2222-2222-22222222222b")
	addBasicPayment := func(d *testDeps, userID, subscriptionID, paymentID uuid.UUID, providerPaymentID string, createdAt time.Time) {
		validUntil := fixedNow.Add(-time.Hour)
		d.addSubscription(domain.Subscription{
			ID:               subscriptionID,
			UserID:           userID,
			TariffID:         basicID,
			Source:           domain.SubscriptionSourcePaid,
			Status:           domain.SubscriptionStatusActive,
			ValidUntil:       &validUntil,
			AutoRenewEnabled: false,
		})
		d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
			ID:                paymentID,
			UserID:            userID,
			SubscriptionID:    subscriptionID,
			TariffID:          basicID,
			Period:            domain.PeriodMonth,
			AmountKopecks:     1000,
			Provider:          domain.ProviderFake,
			ProviderPaymentID: &providerPaymentID,
			Status:            domain.PaymentStatusPending,
			CreatedAt:         createdAt,
			UpdatedAt:         createdAt,
		}
	}

	t.Run("multiple payments across pages", func(t *testing.T) {
		d := newTestDeps(t)
		d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000})

		const totalPayments = 101
		paymentIDs := make([]uuid.UUID, 0, totalPayments)
		for i := 0; i < totalPayments; i++ {
			userID := uuid.MustParse(fmt.Sprintf("11111111-1111-1111-1111-%012d", i))
			subscriptionID := uuid.MustParse(fmt.Sprintf("22222222-2222-2222-2222-%012d", i))
			paymentID := uuid.MustParse(fmt.Sprintf("33333333-3333-3333-3333-%012d", i))
			providerPaymentID := fmt.Sprintf("reconcile_page_%d", i)
			createdAt := fixedNow.Add(-time.Hour - time.Duration(i)*time.Second)
			addBasicPayment(d, userID, subscriptionID, paymentID, providerPaymentID, createdAt)
			paymentIDs = append(paymentIDs, paymentID)
		}

		d.provider.statusRes = domain.PaymentStatusSucceeded

		count, err := d.service.ReconcilePendingPayments(context.Background(), fixedNow)
		if err != nil {
			t.Fatalf("ReconcilePendingPayments error: %v", err)
		}
		if count != totalPayments {
			t.Fatalf("expected %d payments synced, got %d", totalPayments, count)
		}
		if d.subscriptionPayments.listPendingCalls != 2 {
			t.Errorf("expected ListPendingPayments called twice, got %d", d.subscriptionPayments.listPendingCalls)
		}
		for _, paymentID := range paymentIDs {
			payment := d.subscriptionPayments.payments[paymentID]
			if payment.Status != domain.PaymentStatusSucceeded {
				t.Errorf("expected payment %s succeeded, got %s", paymentID, payment.Status)
			}
		}
	})

	t.Run("continues after a failing sync", func(t *testing.T) {
		d := newTestDeps(t)
		d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000})

		success1 := struct{ user, sub, payment uuid.UUID }{uuid.MustParse("11111111-1111-1111-1111-111111111123"), uuid.MustParse("22222222-2222-2222-2222-22222222222f"), uuid.MustParse("33333333-3333-3333-3333-33333333333f")}
		fail := struct{ user, sub, payment uuid.UUID }{uuid.MustParse("11111111-1111-1111-1111-111111111124"), uuid.MustParse("22222222-2222-2222-2222-222222222230"), uuid.MustParse("33333333-3333-3333-3333-333333333330")}
		success2 := struct{ user, sub, payment uuid.UUID }{uuid.MustParse("11111111-1111-1111-1111-111111111125"), uuid.MustParse("22222222-2222-2222-2222-222222222231"), uuid.MustParse("33333333-3333-3333-3333-333333333331")}
		missing := struct{ user, sub, payment uuid.UUID }{uuid.MustParse("11111111-1111-1111-1111-111111111126"), uuid.MustParse("22222222-2222-2222-2222-222222222232"), uuid.MustParse("33333333-3333-3333-3333-333333333332")}

		addBasicPayment(d, success1.user, success1.sub, success1.payment, "reconcile_success_1", fixedNow.Add(-10*time.Minute))
		addBasicPayment(d, fail.user, fail.sub, fail.payment, "reconcile_fail", fixedNow.Add(-9*time.Minute))
		addBasicPayment(d, success2.user, success2.sub, success2.payment, "reconcile_success_2", fixedNow.Add(-8*time.Minute))

		validUntil := fixedNow.AddDate(0, 1, 0)
		d.addSubscription(domain.Subscription{
			ID:               missing.sub,
			UserID:           missing.user,
			TariffID:         basicID,
			Source:           domain.SubscriptionSourcePaid,
			Status:           domain.SubscriptionStatusActive,
			ValidUntil:       &validUntil,
			AutoRenewEnabled: false,
		})
		d.subscriptionPayments.payments[missing.payment] = domain.SubscriptionPayment{
			ID:             missing.payment,
			UserID:         missing.user,
			SubscriptionID: missing.sub,
			TariffID:       basicID,
			Period:         domain.PeriodMonth,
			AmountKopecks:  1000,
			Provider:       domain.ProviderFake,
			Status:         domain.PaymentStatusPending,
			CreatedAt:      fixedNow.Add(-7 * time.Minute),
			UpdatedAt:      fixedNow.Add(-7 * time.Minute),
		}

		d.provider.statusRes = domain.PaymentStatusSucceeded
		d.provider.statusFunc = func(_ uuid.UUID, providerPaymentID string) {
			if providerPaymentID == "reconcile_fail" {
				d.provider.statusErr = errors.New("provider status boom")
			} else {
				d.provider.statusErr = nil
			}
		}

		count, err := d.service.ReconcilePendingPayments(context.Background(), fixedNow)
		if err != nil {
			t.Fatalf("ReconcilePendingPayments error: %v", err)
		}
		if count != 2 {
			t.Fatalf("expected 2 payments synced, got %d", count)
		}

		for _, id := range []uuid.UUID{success1.payment, success2.payment} {
			payment := d.subscriptionPayments.payments[id]
			if payment.Status != domain.PaymentStatusSucceeded {
				t.Errorf("expected payment %s succeeded, got %s", id, payment.Status)
			}
		}

		failPayment := d.subscriptionPayments.payments[fail.payment]
		if failPayment.Status != domain.PaymentStatusPending {
			t.Errorf("expected failing payment %s to remain pending, got %s", fail.payment, failPayment.Status)
		}
		failSub := d.subscriptions.subs[fail.user]
		if failSub.Status != domain.SubscriptionStatusActive {
			t.Errorf("expected failing subscription %s to stay active, got %s", fail.sub, failSub.Status)
		}

		missingPayment := d.subscriptionPayments.payments[missing.payment]
		if missingPayment.Status != domain.PaymentStatusPending {
			t.Errorf("expected missing provider id payment %s to remain pending, got %s", missing.payment, missingPayment.Status)
		}
	})

	t.Run("returns list error", func(t *testing.T) {
		d := newTestDeps(t)
		d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000})
		userID := uuid.MustParse("11111111-1111-1111-1111-111111111127")
		subscriptionID := uuid.MustParse("22222222-2222-2222-2222-222222222233")
		paymentID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
		addBasicPayment(d, userID, subscriptionID, paymentID, "reconcile_list_err", fixedNow.Add(-10*time.Minute))

		d.subscriptionPayments.listPendingErr = errors.New("list failed")

		count, err := d.service.ReconcilePendingPayments(context.Background(), fixedNow)
		if err == nil {
			t.Fatal("expected error from ListPendingPayments")
		}
		if count != 0 {
			t.Errorf("expected 0 payments synced on list error, got %d", count)
		}
	})
}
