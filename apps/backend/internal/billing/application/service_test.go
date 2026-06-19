package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

var fixedNow = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

type fakeClock struct{ now time.Time }

func newFakeClock(t time.Time) *fakeClock { return &fakeClock{now: t} }
func (c *fakeClock) Now() time.Time      { return c.now }

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

// --- transaction fake ---

type fakeTx struct{ b *fakeBeginner }

func (tx *fakeTx) Commit(context.Context) error   { tx.b.committed++; return nil }
func (tx *fakeTx) Rollback(context.Context) error { tx.b.rolledBack++; return nil }

type fakeBeginner struct {
	begun, committed, rolledBack int
}

func (b *fakeBeginner) Begin(context.Context) (transaction.Tx, error) {
	b.begun++
	return &fakeTx{b: b}, nil
}

// --- repository fakes ---

type fakeTariffRepo struct {
	byName map[domain.TariffName]domain.Tariff
	list   []domain.Tariff
}

func (r *fakeTariffRepo) GetByID(_ context.Context, id uuid.UUID) (domain.Tariff, error) {
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
	return append([]domain.Tariff(nil), r.list...), nil
}

func (r *fakeTariffRepo) WithTx(transaction.Tx) TariffRepository { return r }

type fakeSubscriptionRepo struct{ subs map[uuid.UUID]domain.Subscription }

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

type fakePaymentMethodRepo struct{ methods map[uuid.UUID]domain.PaymentMethod }

func (r *fakePaymentMethodRepo) Create(_ context.Context, pm domain.PaymentMethod) (domain.PaymentMethod, error) {
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

type fakeSubscriptionPaymentRepo struct{ payments map[uuid.UUID]domain.SubscriptionPayment }

func (r *fakeSubscriptionPaymentRepo) Create(_ context.Context, p domain.SubscriptionPayment) (domain.SubscriptionPayment, error) {
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

func (r *fakeSubscriptionPaymentRepo) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.SubscriptionPayment, error) {
	return r.GetByID(ctx, id)
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

func (r *fakeSubscriptionPaymentRepo) UpdateProviderPaymentID(_ context.Context, id uuid.UUID, providerPaymentID string) (domain.SubscriptionPayment, error) {
	p, ok := r.payments[id]
	if !ok {
		return domain.SubscriptionPayment{}, ErrNotFound
	}
	p.ProviderPaymentID = &providerPaymentID
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

func (r *fakeSubscriptionPaymentRepo) WithTx(transaction.Tx) SubscriptionPaymentRepository { return r }

// --- property archiver fake ---

type fakePropertyArchiver struct {
	calls []fakeArchiveCall
}

type fakeArchiveCall struct {
	userID uuid.UUID
	limit  int
}

func (a *fakePropertyArchiver) ArchiveExcessProperties(_ context.Context, userID uuid.UUID, limit int) error {
	if a.calls == nil {
		a.calls = make([]fakeArchiveCall, 0)
	}
	a.calls = append(a.calls, fakeArchiveCall{userID: userID, limit: limit})
	return nil
}

// --- provider fake ---

type stubProvider struct {
	name             domain.PaymentProvider
	initCalled       bool
	initReq          InitRequest
	initRes          InitResult
	initErr          error
	initFunc         func(InitRequest)

	chargeRes    ChargeResult
	chargeErr    error
	parseWebhook func([]byte) (WebhookPayload, error)

	confirmPaymentRes        WebhookPayload
	confirmPaymentErr        error
	confirmPaymentInternalID string
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
	if res.SavedToken == "" {
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

func (p *stubProvider) Charge(_ context.Context, _ ChargeRequest) (ChargeResult, error) {
	return p.chargeRes, p.chargeErr
}

func (p *stubProvider) ParseWebhook(_ context.Context, payload []byte) (WebhookPayload, error) {
	if p.parseWebhook != nil {
		return p.parseWebhook(payload)
	}
	return WebhookPayload{}, nil
}

func (p *stubProvider) ConfirmPayment(_ context.Context, internalPaymentID string) (WebhookPayload, error) {
	p.confirmPaymentInternalID = internalPaymentID
	return p.confirmPaymentRes, p.confirmPaymentErr
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
		tariffs:              &fakeTariffRepo{byName: make(map[domain.TariffName]domain.Tariff)},
		subscriptions:        &fakeSubscriptionRepo{subs: make(map[uuid.UUID]domain.Subscription)},
		paymentMethods:       &fakePaymentMethodRepo{methods: make(map[uuid.UUID]domain.PaymentMethod)},
		subscriptionPayments: &fakeSubscriptionPaymentRepo{payments: make(map[uuid.UUID]domain.SubscriptionPayment)},
		propertyArchiver:     &fakePropertyArchiver{},
		provider:             &stubProvider{},
		beginner:             &fakeBeginner{},
		clock:                newFakeClock(fixedNow),
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
		d.propertyArchiver,
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

	pm, err := d.service.AddPaymentMethod(context.Background(), userID, AddPaymentMethodRequest{
		ProviderToken: "raw_token_1234",
	})
	if err != nil {
		t.Fatalf("AddPaymentMethod error: %v", err)
	}
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
	if d.beginner.begun != 1 || d.beginner.committed != 1 {
		t.Errorf("expected transaction begin/commit, got begun=%d committed=%d", d.beginner.begun, d.beginner.committed)
	}
	if _, ok := d.paymentMethods.methods[methodID]; ok {
		t.Error("expected method deleted")
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
	// ChangeTariff uses two transactions; ConfirmFakePayment uses one.
	if d.beginner.begun != 3 || d.beginner.committed != 3 {
		t.Errorf("expected three committed transactions, got begun=%d committed=%d", d.beginner.begun, d.beginner.committed)
	}
}

func TestBillingService_HandleWebhook_AppliesFailedPayment(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777778")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-888888888889")
	tariffID := uuid.MustParse("99999999-9999-9999-9999-99999999999a")
	providerPaymentID := "stub_webhook_1"
	errCode := "card_declined"

	d.addSubscription(domain.Subscription{
		ID:       uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaab"),
		UserID:   userID,
		TariffID: tariffID,
		Source:   domain.SubscriptionSourcePaid,
		Status:   domain.SubscriptionStatusActive,
	})
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbd"),
		TariffID:          tariffID,
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

	wantErr := errors.New("provider init failed")
	d.provider.initErr = wantErr

	resp, err := d.service.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: string(domain.TariffPro),
		Period:     domain.PeriodMonth,
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected provider init error, got %v", err)
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
	// The confirm URL is not reconstructed after PaymentURLProvider removal.
	if resp.ConfirmURL != "" {
		t.Errorf("expected empty confirm url, got %q", resp.ConfirmURL)
	}
	if d.provider.initCalled {
		t.Error("expected no provider call when reusing existing pending payment")
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
