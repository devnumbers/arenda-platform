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
		tx.b.open--
		tx.b.committed++
		tx.done = true
	}
	return tx.b.commitErr
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
	beginErr                           error
	commitErr                          error
}

func (b *fakeBeginner) Begin(context.Context) (transaction.Tx, error) {
	if b.beginErr != nil {
		return nil, b.beginErr
	}
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

func (r *fakeTariffRepo) WithTx(transaction.Tx) (TariffRepository, error) { return r, nil }

type fakeSubscriptionRepo struct {
	subs                    map[uuid.UUID]domain.Subscription
	listUpForRenewalCalls   int
	listInExpiredGraceCalls int
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
	r.listUpForRenewalCalls++
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
	r.listInExpiredGraceCalls++
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

func (r *fakeSubscriptionRepo) WithTx(transaction.Tx) (SubscriptionRepository, error) { return r, nil }

type fakePaymentMethodRepo struct {
	methods map[uuid.UUID]domain.PaymentMethod
}

func (r *fakePaymentMethodRepo) Create(_ context.Context, pm domain.PaymentMethod) (domain.PaymentMethod, error) {
	r.methods[pm.ID] = pm
	return pm, nil
}

// UpsertByTokenHash mirrors the postgres upsert: the row is keyed by
// (user, token), and an empty incoming card id, display mask or expiry date
// preserves the stored value (the query uses COALESCE).
func (r *fakePaymentMethodRepo) UpsertByTokenHash(_ context.Context, pm domain.PaymentMethod) (domain.PaymentMethod, error) {
	for id, existing := range r.methods {
		if existing.UserID == pm.UserID && existing.ProviderToken == pm.ProviderToken {
			existing.ProviderToken = pm.ProviderToken
			if pm.ProviderCardID != "" {
				existing.ProviderCardID = pm.ProviderCardID
			}
			if pm.DisplayMask != "" {
				existing.DisplayMask = pm.DisplayMask
			}
			if pm.ExpDate != "" {
				existing.ExpDate = pm.ExpDate
			}
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

func (r *fakePaymentMethodRepo) WithTx(transaction.Tx) (PaymentMethodRepository, error) {
	return r, nil
}

type fakeSubscriptionPaymentRepo struct {
	payments                   map[uuid.UUID]domain.SubscriptionPayment
	pendingUpgradePaymentIDs   map[uuid.UUID]bool
	listPendingErr             error
	listPendingLimit           int32
	listPendingCalls           int
	listStaleRefundingErr      error
	listStaleRefundingCalls    int
	forUpdateStatus            map[uuid.UUID]domain.PaymentStatus
	updateProviderPaymentIDErr error
	onMarkFailed               func()
	beforeCreate               func()
}

func (r *fakeSubscriptionPaymentRepo) Create(_ context.Context, p domain.SubscriptionPayment) (domain.SubscriptionPayment, error) {
	if r.beforeCreate != nil {
		r.beforeCreate()
	}
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

func (r *fakeSubscriptionPaymentRepo) ListStaleRefundingPayments(_ context.Context, updatedBefore time.Time, limit int32) ([]domain.SubscriptionPayment, error) {
	r.listStaleRefundingCalls++
	if r.listStaleRefundingErr != nil {
		return nil, r.listStaleRefundingErr
	}
	var out []domain.SubscriptionPayment
	for _, p := range r.payments {
		if p.Status != domain.PaymentStatusRefunding || !p.UpdatedAt.Before(updatedBefore) {
			continue
		}
		if p.ProviderPaymentID == nil || *p.ProviderPaymentID == "" {
			continue
		}
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].UpdatedAt.Before(out[j].UpdatedAt) })
	if len(out) > int(limit) {
		out = out[:limit]
	}
	return out, nil
}

func (r *fakeSubscriptionPaymentRepo) ListAll(_ context.Context, _ ListAllPaymentsFilters) ([]SubscriptionPaymentWithUser, int64, error) {
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
	if r.onMarkFailed != nil {
		r.onMarkFailed()
	}
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

func (r *fakeSubscriptionPaymentRepo) MarkReconciledSucceeded(_ context.Context, id uuid.UUID, now time.Time) error {
	p, ok := r.payments[id]
	if !ok {
		return ErrNotFound
	}
	if err := p.ReconcileToSucceeded(now); err != nil {
		return err
	}
	r.payments[id] = p
	return nil
}

func (r *fakeSubscriptionPaymentRepo) MarkReconciledRefunded(_ context.Context, id uuid.UUID, now time.Time) error {
	p, ok := r.payments[id]
	if !ok {
		return ErrNotFound
	}
	if err := p.ReconcileToRefunded(now); err != nil {
		return err
	}
	r.payments[id] = p
	return nil
}

func (r *fakeSubscriptionPaymentRepo) BeginRefund(_ context.Context, id uuid.UUID, now time.Time) error {
	p, ok := r.payments[id]
	if !ok {
		return ErrNotFound
	}
	if err := p.BeginRefund(now); err != nil {
		return err
	}
	r.payments[id] = p
	return nil
}

func (r *fakeSubscriptionPaymentRepo) RevertRefund(_ context.Context, id uuid.UUID, prev domain.PaymentStatus, now time.Time) error {
	p, ok := r.payments[id]
	if !ok {
		return ErrNotFound
	}
	if err := p.RevertRefund(now, prev); err != nil {
		return err
	}
	r.payments[id] = p
	return nil
}

func (r *fakeSubscriptionPaymentRepo) UpdateProviderPaymentID(_ context.Context, id uuid.UUID, providerPaymentID string) (domain.SubscriptionPayment, error) {
	if r.updateProviderPaymentIDErr != nil {
		return domain.SubscriptionPayment{}, r.updateProviderPaymentIDErr
	}
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

func (r *fakeSubscriptionPaymentRepo) IncrementChargeAttempts(_ context.Context, id uuid.UUID) (int, error) {
	p, ok := r.payments[id]
	if !ok {
		return 0, ErrNotFound
	}
	p.ChargeAttempts++
	r.payments[id] = p
	return p.ChargeAttempts, nil
}

func (r *fakeSubscriptionPaymentRepo) WithTx(transaction.Tx) (SubscriptionPaymentRepository, error) {
	return r, nil
}

// --- payment method in-use checker fake ---

type fakePaymentMethodInUseChecker struct {
	inUse map[uuid.UUID]bool
}

func (c *fakePaymentMethodInUseChecker) IsInUse(_ context.Context, methodID uuid.UUID) (bool, error) {
	if c.inUse == nil {
		return false, nil
	}
	return c.inUse[methodID], nil
}

func (c *fakePaymentMethodInUseChecker) WithTx(transaction.Tx) (PaymentMethodInUseChecker, error) {
	return c, nil
}

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

var (
	_ RenewalProvider             = (*stubProvider)(nil)
	_ WebhookProvider             = (*stubProvider)(nil)
	_ PaymentManager              = (*stubProvider)(nil)
	_ CardProvider                = (*stubProvider)(nil)
	_ SubscriptionPaymentProvider = (*stubProvider)(nil)
	_ ProviderNamer               = (*stubProvider)(nil)
)

type stubProvider struct {
	name       domain.PaymentProvider
	initCalled bool
	initCount  int
	initReq    InitRequest
	initRes    InitResult
	initErr    error
	initFunc   func(InitRequest)

	chargeRes    ChargeResult
	chargeErr    error
	chargeReq    ChargeRequest
	chargeFunc   func(ChargeRequest)
	chargeCalled bool
	chargeCount  int

	statusRes      domain.PaymentStatus
	statusRebillID string
	statusErr      error
	statusFunc     func(uuid.UUID, string)
	statusCalled   bool
	statusCount    int

	parseWebhook func([]byte) (WebhookPayload, error)

	initAddCardRes    InitAddCardResult
	initAddCardErr    error
	initAddCardReq    InitAddCardRequest
	initAddCardCalled bool

	removeCardErr      error
	removeCardCalled   bool
	removeCardCustomer string
	removeCardCardID   string

	getCardListRes         []ProviderCard
	getCardListErr         error
	getCardListCalled      bool
	getCardListCount       int
	getCardListCustomerKey string

	getAddCardStateRes        CardBindingState
	getAddCardStateErr        error
	getAddCardStateCalled     bool
	getAddCardStateRequestKey string

	confirmPaymentRes        WebhookPayload
	confirmPaymentErr        error
	confirmPaymentInternalID string

	cancelRes    CancelResult
	cancelErr    error
	cancelCalled bool
	cancelCount  int
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
	p.initCount++
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
	p.chargeCount++
	p.chargeReq = req
	if p.chargeFunc != nil {
		p.chargeFunc(req)
	}
	return p.chargeRes, p.chargeErr
}

func (p *stubProvider) Status(_ context.Context, paymentID uuid.UUID, providerPaymentID string) (PaymentStatusResult, error) {
	p.statusCalled = true
	p.statusCount++
	if p.statusFunc != nil {
		p.statusFunc(paymentID, providerPaymentID)
	}
	return PaymentStatusResult{Status: p.statusRes, RebillID: p.statusRebillID}, p.statusErr
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

func (p *stubProvider) GetCardList(_ context.Context, customerKey string) ([]ProviderCard, error) {
	p.getCardListCalled = true
	p.getCardListCount++
	p.getCardListCustomerKey = customerKey
	if p.getCardListErr != nil {
		return nil, p.getCardListErr
	}
	return p.getCardListRes, nil
}

func (p *stubProvider) GetAddCardState(_ context.Context, requestKey string) (CardBindingState, error) {
	p.getAddCardStateCalled = true
	p.getAddCardStateRequestKey = requestKey
	return p.getAddCardStateRes, p.getAddCardStateErr
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
	p.cancelCount++
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
	inUseChecker         *fakePaymentMethodInUseChecker
	service              Services
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
		inUseChecker:     &fakePaymentMethodInUseChecker{},
	}
	d.service = NewServices(
		d.tariffs,
		d.subscriptions,
		d.paymentMethods,
		d.subscriptionPayments,
		d.provider,
		d.beginner,
		nil,
		d.clock,
		discardLogger(),
		"http://localhost",
		d.propertyArchiver,
		nil,
		d.inUseChecker,
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

func TestBilling_ListTariffsOrderedByPrice(t *testing.T) {
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

	list, err := d.service.Tariffs.ListTariffs(context.Background())
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

func TestBilling_GetSubscriptionWithActivePaymentMethod(t *testing.T) {
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

	view, err := d.service.Subscriptions.GetSubscription(context.Background(), userID)
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

func TestBilling_ChangeTariff_UpgradeCreatesPendingPayment(t *testing.T) {
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

	resp, err := d.service.Subscriptions.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: domain.TariffPro,
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

func TestBilling_ChangeTariff_UpgradeCreatesPaymentBeforeProviderCall(t *testing.T) {
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

	resp, err := d.service.Subscriptions.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff error: %v", err)
	}
	if seenPaymentID != resp.PaymentID {
		t.Errorf("expected provider.Init to receive payment %s, got %s", resp.PaymentID, seenPaymentID)
	}
}

func TestBilling_ChangeTariff_DowngradeSchedulesPendingChange(t *testing.T) {
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

	resp, err := d.service.Subscriptions.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: domain.TariffBasic,
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
	if !sub.AutoRenewEnabled {
		t.Error("expected auto-renew enabled after scheduling downgrade")
	}
}

func TestBilling_ChangeTariff_AlreadyOnTariff(t *testing.T) {
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

	_, err := d.service.Subscriptions.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: domain.TariffBasic,
		Period:     domain.PeriodMonth,
	})
	if !errors.Is(err, ErrAlreadyOnTariff) {
		t.Fatalf("expected ErrAlreadyOnTariff, got %v", err)
	}
}

func TestBilling_ToggleAutoRenew(t *testing.T) {
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

	if err := d.service.Subscriptions.ToggleAutoRenew(context.Background(), userID, true); err != nil {
		t.Fatalf("ToggleAutoRenew error: %v", err)
	}
	if !d.subscriptions.subs[userID].AutoRenewEnabled {
		t.Error("expected auto-renew enabled")
	}

	if err := d.service.Subscriptions.ToggleAutoRenew(context.Background(), userID, false); err != nil {
		t.Fatalf("ToggleAutoRenew error: %v", err)
	}
	if d.subscriptions.subs[userID].AutoRenewEnabled {
		t.Error("expected auto-renew disabled")
	}
}

func TestBilling_AddPaymentMethod(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccd")

	resp, err := d.service.PaymentMethods.AddPaymentMethod(context.Background(), userID, AddPaymentMethodRequest{
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

func TestBilling_SetActivePaymentMethodUsesTransaction(t *testing.T) {
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

	if err := d.service.PaymentMethods.SetActivePaymentMethod(context.Background(), userID, methodID); err != nil {
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

func TestBilling_DeletePaymentMethodUsesTransaction(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	methodID := uuid.MustParse("11111111-1111-1111-1111-111111111113")

	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:       methodID,
		UserID:   userID,
		Provider: domain.ProviderFake,
	}

	if err := d.service.PaymentMethods.DeletePaymentMethod(context.Background(), userID, methodID); err != nil {
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

func TestBilling_DeletePaymentMethod_RejectsActiveMethodInSingleTransaction(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	methodID := uuid.MustParse("11111111-1111-1111-1111-111111111113")

	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:       methodID,
		UserID:   userID,
		Provider: domain.ProviderFake,
		IsActive: true,
	}

	err := d.service.PaymentMethods.DeletePaymentMethod(context.Background(), userID, methodID)
	if !errors.Is(err, ErrPaymentMethodInUse) {
		t.Fatalf("expected ErrPaymentMethodInUse, got %v", err)
	}
	if _, ok := d.paymentMethods.methods[methodID]; !ok {
		t.Error("expected active method to remain (not deleted)")
	}
	if d.provider.removeCardCalled {
		t.Error("expected provider.RemoveCard not to be called for active method")
	}
	// The active-method guard runs inside the single transaction, which is rolled
	// back on rejection; nothing is committed.
	if d.beginner.begun != 1 || d.beginner.committed != 0 {
		t.Errorf("expected one rolled-back transaction, got begun=%d committed=%d", d.beginner.begun, d.beginner.committed)
	}
	if d.beginner.rolledBack != 1 {
		t.Errorf("expected the transaction to be rolled back, got rolledBack=%d", d.beginner.rolledBack)
	}
}

func TestBilling_DeletePaymentMethod_RejectsInUseMethodInSingleTransaction(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	methodID := uuid.MustParse("11111111-1111-1111-1111-111111111113")

	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:       methodID,
		UserID:   userID,
		Provider: domain.ProviderFake,
		IsActive: false,
	}
	d.inUseChecker.inUse = map[uuid.UUID]bool{methodID: true}

	err := d.service.PaymentMethods.DeletePaymentMethod(context.Background(), userID, methodID)
	if !errors.Is(err, ErrPaymentMethodInUse) {
		t.Fatalf("expected ErrPaymentMethodInUse, got %v", err)
	}
	if _, ok := d.paymentMethods.methods[methodID]; !ok {
		t.Error("expected in-use method to remain (not deleted)")
	}
	if d.provider.removeCardCalled {
		t.Error("expected provider.RemoveCard not to be called for in-use method")
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

func TestBilling_ConfirmFakePayment_AppliesUpgrade(t *testing.T) {
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

	resp, err := d.service.Subscriptions.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: domain.TariffPro,
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

	if err := d.service.Payments.ConfirmFakePayment(context.Background(), resp.PaymentID); err != nil {
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

func TestBilling_HandleWebhook_AppliesFailedUpgradePayment(t *testing.T) {
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

	if err := d.service.Webhooks.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
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

func TestBilling_HandleWebhook_RefundDowngradesToBasic(t *testing.T) {
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

	if err := d.service.Webhooks.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
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

func TestBilling_HandleWebhook_FailedRenewalMovesToGrace(t *testing.T) {
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

	if err := d.service.Webhooks.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
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

func TestBilling_HandleWebhook_FailedRenewalDoesNotShortenFutureValidUntil(t *testing.T) {
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

	if err := d.service.Webhooks.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
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

func TestBilling_HandleWebhook_SucceededAfterFailed_ReconcilesToSucceeded(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-77777777777b")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-88888888888c")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaf")
	proID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	providerPaymentID := "stub_webhook_reconcile_succeeded"
	validUntil := fixedNow.AddDate(0, 0, -1)

	d.addTariff(domain.Tariff{
		ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               subscriptionID,
		UserID:           userID,
		TariffID:         proID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusGrace,
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
		Status:            domain.PaymentStatusFailed,
	}

	d.provider.statusRes = domain.PaymentStatusSucceeded
	d.provider.parseWebhook = func(_ []byte) (WebhookPayload, error) {
		return WebhookPayload{
			ProviderPaymentID: providerPaymentID,
			InternalPaymentID: paymentID,
			Status:            domain.PaymentStatusSucceeded,
		}, nil
	}

	if err := d.service.Webhooks.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook error: %v", err)
	}

	if d.provider.statusCount != 1 {
		t.Errorf("expected provider.Status called once, got %d", d.provider.statusCount)
	}

	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusSucceeded {
		t.Errorf("expected payment succeeded, got %s", payment.Status)
	}
	if payment.ErrorCode != nil {
		t.Errorf("expected error code cleared, got %v", payment.ErrorCode)
	}

	sub := d.subscriptions.subs[userID]
	if sub.Status != domain.SubscriptionStatusActive {
		t.Errorf("expected subscription active, got %s", sub.Status)
	}
	wantValidUntil := validUntil.AddDate(0, 1, 0)
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(wantValidUntil) {
		t.Errorf("expected valid_until extended to %v, got %v", wantValidUntil, sub.ValidUntil)
	}
}

func TestBilling_HandleWebhook_SucceededAfterFailed_AlreadyRefunded(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-77777777777c")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-88888888888d")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1")
	basicID := uuid.MustParse("00000000-0000-0000-0000-000000000004")
	proID := uuid.MustParse("00000000-0000-0000-0000-000000000005")
	providerPaymentID := "stub_webhook_reconcile_refunded"
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
		Status:           domain.SubscriptionStatusGrace,
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
		Status:            domain.PaymentStatusFailed,
	}

	d.provider.statusRes = domain.PaymentStatusRefunded
	d.provider.parseWebhook = func(_ []byte) (WebhookPayload, error) {
		return WebhookPayload{
			ProviderPaymentID: providerPaymentID,
			InternalPaymentID: paymentID,
			Status:            domain.PaymentStatusSucceeded,
		}, nil
	}

	if err := d.service.Webhooks.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook error: %v", err)
	}

	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusRefunded {
		t.Errorf("expected payment refunded, got %s", payment.Status)
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

func TestBilling_HandleWebhook_SucceededAfterFailed_StillFailed(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-77777777777d")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-88888888888e")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa2")
	tariffID := uuid.MustParse("99999999-9999-9999-9999-99999999999c")
	providerPaymentID := "stub_webhook_reconcile_still_failed"
	validUntil := fixedNow.AddDate(0, 0, -1)

	d.addTariff(domain.Tariff{
		ID: tariffID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               subscriptionID,
		UserID:           userID,
		TariffID:         tariffID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusGrace,
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
		Status:            domain.PaymentStatusFailed,
	}

	d.provider.statusRes = domain.PaymentStatusFailed
	d.provider.parseWebhook = func(_ []byte) (WebhookPayload, error) {
		return WebhookPayload{
			ProviderPaymentID: providerPaymentID,
			InternalPaymentID: paymentID,
			Status:            domain.PaymentStatusSucceeded,
		}, nil
	}

	if err := d.service.Webhooks.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook error: %v", err)
	}

	if d.provider.statusCount != 1 {
		t.Errorf("expected provider.Status called once, got %d", d.provider.statusCount)
	}

	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusFailed {
		t.Errorf("expected payment still failed, got %s", payment.Status)
	}

	sub := d.subscriptions.subs[userID]
	if sub.Status != domain.SubscriptionStatusGrace {
		t.Errorf("expected subscription still in grace, got %s", sub.Status)
	}
}

func TestBilling_HandleWebhook_SucceededAfterFailed_StatusError(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-77777777777e")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-88888888888f")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa3")
	tariffID := uuid.MustParse("99999999-9999-9999-9999-99999999999d")
	providerPaymentID := "stub_webhook_reconcile_status_error"
	validUntil := fixedNow.AddDate(0, 0, -1)

	d.addTariff(domain.Tariff{
		ID: tariffID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               subscriptionID,
		UserID:           userID,
		TariffID:         tariffID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusGrace,
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
		Status:            domain.PaymentStatusFailed,
	}

	d.provider.statusErr = errors.New("provider status unavailable")
	d.provider.parseWebhook = func(_ []byte) (WebhookPayload, error) {
		return WebhookPayload{
			ProviderPaymentID: providerPaymentID,
			InternalPaymentID: paymentID,
			Status:            domain.PaymentStatusSucceeded,
		}, nil
	}

	if err := d.service.Webhooks.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook error: %v", err)
	}

	if d.provider.statusCount != 1 {
		t.Errorf("expected provider.Status called once, got %d", d.provider.statusCount)
	}

	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusFailed {
		t.Errorf("expected payment still failed, got %s", payment.Status)
	}

	sub := d.subscriptions.subs[userID]
	if sub.Status != domain.SubscriptionStatusGrace {
		t.Errorf("expected subscription still in grace, got %s", sub.Status)
	}
}

func TestBilling_ChangeTariff_UpgradeDefersPaymentMethodActivation(t *testing.T) {
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

	resp, err := d.service.Subscriptions.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: domain.TariffPro,
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

func TestBilling_ChangeTariff_UpgradeInitFailureMarksPaymentFailed(t *testing.T) {
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

	resp, err := d.service.Subscriptions.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: domain.TariffPro,
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

func TestBilling_SaveProviderInitResult_NoDeadlockOnUpdateFailure(t *testing.T) {
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

	// A non-fake provider keeps stubProvider.Init from auto-filling a saved
	// token, so saveProviderInitResult takes the UpdateProviderPaymentID branch,
	// which we force to fail while the payment row is locked by the outer tx.
	d.provider.name = domain.ProviderTkassa

	d.subscriptionPayments.updateProviderPaymentIDErr = errors.New("boom: update provider payment id failed")

	// Capture how many transactions are open exactly when the best-effort
	// MarkFailed runs. The outer (locked) tx must be rolled back first, so only
	// the best-effort tx is open (open == 1). If the outer tx were still open,
	// open would be 2, which is the self-deadlock on a real database.
	openAtMarkFailed := -1
	d.subscriptionPayments.onMarkFailed = func() {
		openAtMarkFailed = d.beginner.open
	}

	_, err := d.service.Subscriptions.ChangeTariff(t.Context(), userID, ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	})
	if err == nil {
		t.Fatal("expected ChangeTariff error from failing UpdateProviderPaymentID, got nil")
	}
	if !d.provider.initCalled {
		t.Fatal("expected provider.Init to be called")
	}

	paymentID := d.provider.initReq.PaymentID
	payment, ok := d.subscriptionPayments.payments[paymentID]
	if !ok {
		t.Fatalf("payment %s not saved", paymentID)
	}
	if payment.Status != domain.PaymentStatusFailed {
		t.Errorf("expected payment to be marked failed, got status %s", payment.Status)
	}

	if openAtMarkFailed != 1 {
		t.Errorf("best-effort MarkFailed ran with %d open transaction(s); want 1 (outer locked tx must be rolled back first)", openAtMarkFailed)
	}
	if d.beginner.open != 0 {
		t.Errorf("expected no open transactions after return, got %d", d.beginner.open)
	}
}

func TestBilling_ChangeTariff_UpgradeReturnsExistingPendingPayment(t *testing.T) {
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

	resp, err := d.service.Subscriptions.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: domain.TariffPro,
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

func TestBilling_ChangeTariff_DowngradeCancelledInvalidState(t *testing.T) {
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
		Status:           domain.SubscriptionStatusCancelled,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	})

	_, err := d.service.Subscriptions.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: domain.TariffBasic,
		Period:     domain.PeriodMonth,
	})
	if !errors.Is(err, domain.ErrInvalidSubscriptionState) {
		t.Fatalf("expected ErrInvalidSubscriptionState, got %v", err)
	}
}

func TestBilling_ToggleAutoRenewUsesTransaction(t *testing.T) {
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

	if err := d.service.Subscriptions.ToggleAutoRenew(context.Background(), userID, true); err != nil {
		t.Fatalf("ToggleAutoRenew error: %v", err)
	}
	if d.beginner.begun != 1 || d.beginner.committed != 1 {
		t.Errorf("expected one committed transaction, got begun=%d committed=%d", d.beginner.begun, d.beginner.committed)
	}
}

func TestBilling_AddPaymentMethodUsesTransaction(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccd")

	if _, err := d.service.PaymentMethods.AddPaymentMethod(context.Background(), userID, AddPaymentMethodRequest{
		ProviderToken: "raw_token_1234",
	}); err != nil {
		t.Fatalf("AddPaymentMethod error: %v", err)
	}
	if d.beginner.begun != 1 || d.beginner.committed != 1 {
		t.Errorf("expected one committed transaction, got begun=%d committed=%d", d.beginner.begun, d.beginner.committed)
	}
}

func TestBilling_HandleWebhook_SucceededPersistsProviderPaymentID(t *testing.T) {
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

	if err := d.service.Webhooks.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
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

func TestBilling_HandleWebhook_AppliesRenewal(t *testing.T) {
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

	if err := d.service.Webhooks.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
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

func TestBilling_HandleWebhook_DuplicateRenewalWebhookIsIdempotent(t *testing.T) {
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

	if err := d.service.Webhooks.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
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
	if err := d.service.Webhooks.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("second HandleWebhook error: %v", err)
	}

	sub = d.subscriptions.subs[userID]
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(wantValidUntil) {
		t.Errorf("duplicate webhook extended valid until: expected %v, got %v", wantValidUntil, sub.ValidUntil)
	}
}

func TestBilling_HandleWebhook_DuplicateSucceededIsIdempotent(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777778")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-888888888889")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaab")
	tariffID := uuid.MustParse("99999999-9999-9999-9999-99999999999a")
	providerPaymentID := "stub_webhook_duplicate_succeeded"
	existingValidUntil := fixedNow.AddDate(0, 0, 15)

	// Active paid subscription on a paid tariff with a PENDING renewal payment
	// for the SAME tariff, so the success path is ApplyRenewal (extends
	// valid_until by the payment period and records LastAppliedPaymentID) rather
	// than ApplyTariffChange.
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

	// Act 1: first succeeded webhook marks the payment succeeded and applies the
	// renewal, extending valid_until and recording LastAppliedPaymentID.
	if err := d.service.Webhooks.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("first HandleWebhook error: %v", err)
	}

	wantValidUntil := existingValidUntil.AddDate(0, 1, 0)
	sub := d.subscriptions.subs[userID]
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(wantValidUntil) {
		t.Fatalf("after first webhook: expected valid_until extended to %v, got %v", wantValidUntil, sub.ValidUntil)
	}
	if sub.LastAppliedPaymentID == nil || *sub.LastAppliedPaymentID != paymentID {
		t.Fatalf("after first webhook: expected LastAppliedPaymentID = %s, got %v", paymentID, sub.LastAppliedPaymentID)
	}
	if got := d.subscriptionPayments.payments[paymentID].Status; got != domain.PaymentStatusSucceeded {
		t.Fatalf("after first webhook: expected payment succeeded, got %s", got)
	}

	// Snapshot the post-1st state. For a same-tariff renewal the archiver is not
	// invoked (applySubscriptionRenewalAndArchive only archives when the tariff
	// changes), so archiverCalls1 is 0; the meaningful assertion is that the
	// duplicate webhook does not change it.
	validUntil1 := *sub.ValidUntil
	lastApplied1 := *sub.LastAppliedPaymentID
	archiverCalls1 := len(d.propertyArchiver.calls)

	// Act 2: the SAME succeeded webhook arrives again. The payment is already
	// succeeded, so HandleWebhook must return nil and leave state untouched.
	if err := d.service.Webhooks.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("second HandleWebhook error: %v", err)
	}

	sub = d.subscriptions.subs[userID]
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(validUntil1) {
		t.Errorf("duplicate succeeded webhook extended valid_until: expected %v, got %v", validUntil1, sub.ValidUntil)
	}
	if sub.LastAppliedPaymentID == nil || *sub.LastAppliedPaymentID != lastApplied1 {
		t.Errorf("duplicate succeeded webhook changed LastAppliedPaymentID: expected %s, got %v", lastApplied1, sub.LastAppliedPaymentID)
	}
	if got := len(d.propertyArchiver.calls); got != archiverCalls1 {
		t.Errorf("duplicate succeeded webhook invoked archiver again: expected %d calls, got %d", archiverCalls1, got)
	}
	if got := d.subscriptionPayments.payments[paymentID].Status; got != domain.PaymentStatusSucceeded {
		t.Errorf("expected payment to remain succeeded, got %s", got)
	}
}

func TestBilling_HandleWebhook_DuplicateRefundedIsIdempotent(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-77777777777d")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-88888888888d")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaad")
	basicID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	proID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	providerPaymentID := "stub_webhook_duplicate_refunded"
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

	// Act 1: the first refunded webhook marks the payment refunded and downgrades
	// the subscription to basic.
	if err := d.service.Webhooks.HandleWebhook(t.Context(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("first HandleWebhook error: %v", err)
	}

	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusRefunded {
		t.Fatalf("after first webhook: expected payment refunded, got %s", payment.Status)
	}
	if payment.RefundedAmountKopecks == nil || *payment.RefundedAmountKopecks != 5000 {
		t.Fatalf("after first webhook: expected refunded amount 5000, got %v", payment.RefundedAmountKopecks)
	}
	sub := d.subscriptions.subs[userID]
	if sub.TariffID != basicID {
		t.Fatalf("after first webhook: expected subscription downgraded to basic, got tariff %s", sub.TariffID)
	}

	// Snapshot the post-1st state, including the archiver call count: the
	// duplicate webhook must not trigger a second downgrade.
	refundedAmount1 := *payment.RefundedAmountKopecks
	archiverCalls1 := len(d.propertyArchiver.calls)

	// Act 2: the SAME refunded webhook arrives again. The payment is already
	// refunded, so HandleWebhook must return nil and leave state untouched.
	if err := d.service.Webhooks.HandleWebhook(t.Context(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("second HandleWebhook error: %v", err)
	}

	payment = d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusRefunded {
		t.Errorf("expected payment to remain refunded, got %s", payment.Status)
	}
	if payment.RefundedAmountKopecks == nil || *payment.RefundedAmountKopecks != refundedAmount1 {
		t.Errorf("duplicate refunded webhook changed refunded amount: expected %d, got %v", refundedAmount1, payment.RefundedAmountKopecks)
	}
	sub = d.subscriptions.subs[userID]
	if sub.TariffID != basicID {
		t.Errorf("duplicate refunded webhook changed tariff: expected basic, got %s", sub.TariffID)
	}
	if sub.Status != domain.SubscriptionStatusActive {
		t.Errorf("duplicate refunded webhook changed subscription status: expected active, got %s", sub.Status)
	}
	if sub.ValidUntil != nil {
		t.Errorf("duplicate refunded webhook changed valid_until: expected nil, got %v", sub.ValidUntil)
	}
	if sub.AutoRenewEnabled {
		t.Error("duplicate refunded webhook re-enabled auto_renew")
	}
	if got := len(d.propertyArchiver.calls); got != archiverCalls1 {
		t.Errorf("duplicate refunded webhook invoked archiver again: expected %d calls, got %d", archiverCalls1, got)
	}
}

func TestBilling_HandleWebhook_PartialRefundedIsIgnored(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-77777777777e")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-88888888888e")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaae")
	proID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	providerPaymentID := "stub_webhook_partial_refunded"
	validUntil := fixedNow.AddDate(0, 1, 0)

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
			Status:            domain.PaymentStatusPartialRefunded,
			AmountKopecks:     2500,
		}, nil
	}

	// Partial refunds are impossible in this product: the external notification
	// is an anomaly that must be logged and ignored without any state change.
	if err := d.service.Webhooks.HandleWebhook(t.Context(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook error: %v", err)
	}

	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusSucceeded {
		t.Errorf("expected payment to remain succeeded, got %s", payment.Status)
	}
	if payment.RefundedAmountKopecks != nil {
		t.Errorf("expected refunded amount to stay nil, got %v", *payment.RefundedAmountKopecks)
	}
	sub := d.subscriptions.subs[userID]
	if sub.TariffID != proID {
		t.Errorf("expected subscription to stay on pro tariff, got %s", sub.TariffID)
	}
	if sub.Status != domain.SubscriptionStatusActive {
		t.Errorf("expected subscription status active, got %s", sub.Status)
	}
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(validUntil) {
		t.Errorf("expected valid_until unchanged at %v, got %v", validUntil, sub.ValidUntil)
	}
	if !sub.AutoRenewEnabled {
		t.Error("expected auto_renew to stay enabled")
	}
	if got := len(d.propertyArchiver.calls); got != 0 {
		t.Errorf("expected no archiver calls, got %d", got)
	}
}

func TestBilling_ChangeTariff_RejectServiceSubscription(t *testing.T) {
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

	_, err := d.service.Subscriptions.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	})
	if !errors.Is(err, domain.ErrInvalidSubscriptionState) {
		t.Fatalf("expected ErrInvalidSubscriptionState, got %v", err)
	}
}

func TestBilling_ConfirmFakePayment_Idempotent(t *testing.T) {
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

	resp, err := d.service.Subscriptions.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: domain.TariffPro,
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

	if err := d.service.Payments.ConfirmFakePayment(context.Background(), resp.PaymentID); err != nil {
		t.Fatalf("first ConfirmFakePayment error: %v", err)
	}
	if d.subscriptionPayments.payments[resp.PaymentID].Status != domain.PaymentStatusSucceeded {
		t.Fatalf("expected payment succeeded after first confirmation")
	}

	// Second confirmation should be a no-op and not return an error.
	if err := d.service.Payments.ConfirmFakePayment(context.Background(), resp.PaymentID); err != nil {
		t.Fatalf("second ConfirmFakePayment error: %v", err)
	}
}

func TestBilling_ProcessRenewals_Success(t *testing.T) {
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

	count, err := d.service.Renewals.ProcessRenewals(context.Background(), fixedNow)
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

func TestBilling_ProcessRenewals_LastAppliedPaymentID_PreventsFalsePositive(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	methodID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	otherPaymentID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	succeededPaymentID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	validUntil := fixedNow

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addSubscription(domain.Subscription{
		ID:                    uuid.MustParse("77777777-7777-7777-7777-777777777777"),
		UserID:                userID,
		TariffID:              proID,
		Source:                domain.SubscriptionSourcePaid,
		Status:                domain.SubscriptionStatusActive,
		ValidUntil:            &validUntil,
		AutoRenewEnabled:      true,
		ActivePaymentMethodID: &methodID,
		LastAppliedPaymentID:  &otherPaymentID,
	})
	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:            methodID,
		UserID:        userID,
		Provider:      domain.ProviderFake,
		ProviderToken: "fake_token_1234",
		IsActive:      true,
	}
	// Succeeded payment whose expected valid_until is far behind the current
	// subscription valid_until. The old date-based heuristic would mistakenly
	// consider the renewal already applied; LastAppliedPaymentID prevents that.
	succeededAt := fixedNow.AddDate(-2, 0, 0)
	d.subscriptionPayments.payments[succeededPaymentID] = domain.SubscriptionPayment{
		ID:              succeededPaymentID,
		UserID:          userID,
		SubscriptionID:  uuid.MustParse("77777777-7777-7777-7777-777777777777"),
		TariffID:        proID,
		PaymentMethodID: &methodID,
		Period:          domain.PeriodMonth,
		AmountKopecks:   5000,
		Provider:        domain.ProviderFake,
		Status:          domain.PaymentStatusSucceeded,
		CreatedAt:       succeededAt,
		SucceededAt:     &succeededAt,
	}

	count, err := d.service.Renewals.ProcessRenewals(t.Context(), fixedNow)
	if err != nil {
		t.Fatalf("ProcessRenewals error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 renewal processed, got %d", count)
	}

	sub := d.subscriptions.subs[userID]
	if sub.LastAppliedPaymentID == nil || *sub.LastAppliedPaymentID != succeededPaymentID {
		t.Errorf("expected LastAppliedPaymentID = %s, got %v", succeededPaymentID, sub.LastAppliedPaymentID)
	}
	wantValidUntil := validUntil.AddDate(0, 1, 0)
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(wantValidUntil) {
		t.Errorf("expected valid_until = %v, got %v", wantValidUntil, sub.ValidUntil)
	}
}

func TestBilling_ProcessRenewals_LastAppliedPaymentID_SkipsAlreadyApplied(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	methodID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	succeededPaymentID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	subID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	d.addSubscription(domain.Subscription{
		ID:                    subID,
		UserID:                userID,
		TariffID:              proID,
		Source:                domain.SubscriptionSourcePaid,
		Status:                domain.SubscriptionStatusActive,
		ValidUntil:            &validUntil,
		AutoRenewEnabled:      true,
		ActivePaymentMethodID: &methodID,
		LastAppliedPaymentID:  &succeededPaymentID,
	})
	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:            methodID,
		UserID:        userID,
		Provider:      domain.ProviderFake,
		ProviderToken: "fake_token_1234",
		IsActive:      true,
	}
	succeededAt := fixedNow.AddDate(0, 0, -5)
	d.subscriptionPayments.payments[succeededPaymentID] = domain.SubscriptionPayment{
		ID:              succeededPaymentID,
		UserID:          userID,
		SubscriptionID:  subID,
		TariffID:        proID,
		PaymentMethodID: &methodID,
		Period:          domain.PeriodMonth,
		AmountKopecks:   5000,
		Provider:        domain.ProviderFake,
		Status:          domain.PaymentStatusSucceeded,
		CreatedAt:       succeededAt,
		SucceededAt:     &succeededAt,
	}

	// applySubscriptionRenewalBestEffort is the same helper ProcessRenewals uses
	// to self-heal a succeeded payment. When LastAppliedPaymentID already matches
	// the payment, the subscription must not change.
	got, _, _, ok := applySubscriptionRenewalBestEffort(t.Context(), renewalBestEffortDeps{
		beginner:      d.beginner,
		subscriptions: d.subscriptions,
		tariffs:       d.tariffs,
		log:           discardLogger(),
	}, subID, d.subscriptionPayments.payments[succeededPaymentID], fixedNow)
	if !ok {
		t.Fatal("expected best-effort renewal to succeed idempotently")
	}
	if got.LastAppliedPaymentID == nil || *got.LastAppliedPaymentID != succeededPaymentID {
		t.Errorf("expected LastAppliedPaymentID unchanged = %s, got %v", succeededPaymentID, got.LastAppliedPaymentID)
	}
	if got.ValidUntil == nil || !got.ValidUntil.Equal(validUntil) {
		t.Errorf("expected valid_until unchanged = %v, got %v", validUntil, got.ValidUntil)
	}
}

func TestBilling_ProcessRenewals_UpdatesStalePaymentMethodID(t *testing.T) {
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

	count, err := d.service.Renewals.ProcessRenewals(context.Background(), fixedNow)
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

func TestBilling_ProcessRenewals_RecoversExistingPendingPaymentOnCreateRace(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	methodID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	subID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	existingPaymentID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	validUntil := fixedNow

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
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
	// Simulate a concurrent renewal run that commits its pending payment after
	// our pre-lookup misses it but before our Create executes, so Create hits
	// ErrAlreadyExists from the partial unique index backstop.
	d.subscriptionPayments.beforeCreate = func() {
		d.subscriptionPayments.payments[existingPaymentID] = domain.SubscriptionPayment{
			ID:              existingPaymentID,
			UserID:          userID,
			SubscriptionID:  subID,
			TariffID:        proID,
			PaymentMethodID: &methodID,
			Period:          domain.PeriodMonth,
			AmountKopecks:   5000,
			Provider:        domain.ProviderFake,
			Status:          domain.PaymentStatusPending,
		}
	}
	d.provider.chargeRes = ChargeResult{ProviderPaymentID: "charge_1", Status: domain.PaymentStatusSucceeded}

	count, err := d.service.Renewals.ProcessRenewals(t.Context(), fixedNow)
	if err != nil {
		t.Fatalf("ProcessRenewals error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 renewal, got %d", count)
	}

	// The conflicting pending payment must be reused: no duplicate created.
	if len(d.subscriptionPayments.payments) != 1 {
		t.Errorf("expected exactly 1 payment (no duplicate), got %d", len(d.subscriptionPayments.payments))
	}
	if d.provider.initReq.PaymentID != existingPaymentID {
		t.Errorf("expected provider init for existing payment %s, got %s", existingPaymentID, d.provider.initReq.PaymentID)
	}
	if !d.provider.chargeCalled || d.provider.chargeReq.PaymentID != existingPaymentID {
		t.Errorf("expected charge for existing payment %s, got %s (called=%v)", existingPaymentID, d.provider.chargeReq.PaymentID, d.provider.chargeCalled)
	}
	p := d.subscriptionPayments.payments[existingPaymentID]
	if p.Status != domain.PaymentStatusSucceeded {
		t.Errorf("expected existing payment succeeded, got %s", p.Status)
	}
	sub := d.subscriptions.subs[userID]
	if sub.Status != domain.SubscriptionStatusActive {
		t.Errorf("expected status active, got %s", sub.Status)
	}
	if sub.ValidUntil == nil || !sub.ValidUntil.After(fixedNow) {
		t.Errorf("expected valid_until extended, got %v", sub.ValidUntil)
	}
}

func TestBilling_ProcessRenewals_FailedChargeMovesToGrace(t *testing.T) {
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

	count, err := d.service.Renewals.ProcessRenewals(context.Background(), fixedNow)
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

func TestBilling_ProcessRenewals_SkipsDuplicateChargeWhenProviderSucceeded(t *testing.T) {
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

	count, err := d.service.Renewals.ProcessRenewals(context.Background(), fixedNow)
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

func TestBilling_ProcessRenewals_FinalizesFailedPaymentWithoutCharge(t *testing.T) {
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

	count, err := d.service.Renewals.ProcessRenewals(context.Background(), fixedNow)
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

func TestBilling_ProcessRenewals_ProviderErrorCodeSavedOnFailedCharge(t *testing.T) {
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

	count, err := d.service.Renewals.ProcessRenewals(context.Background(), fixedNow)
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

func TestBilling_ProcessExpiredGrace_DowngradesToBasic(t *testing.T) {
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

	count, err := d.service.Renewals.ProcessExpiredGrace(context.Background(), fixedNow)
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

func TestBilling_renewSubscription_ChargeCalledOutsideTransaction(t *testing.T) {
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

	renewal := NewRenewalService(renewalServiceDeps{
		tariffs:              d.tariffs,
		subscriptions:        d.subscriptions,
		subscriptionPayments: d.subscriptionPayments,
		paymentMethods:       d.paymentMethods,
		propertyArchiver:     d.propertyArchiver,
		beginner:             d.beginner,
		log:                  discardLogger(),
		callbackBaseURL:      "http://localhost",
	}, d.provider)
	if err := renewal.renewSubscription(context.Background(), d.subscriptions.subs[userID], fixedNow); err != nil {
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
	d.service = NewServices(
		d.tariffs,
		d.subscriptions,
		d.paymentMethods,
		d.subscriptionPayments,
		d.provider,
		d.beginner,
		nil,
		d.clock,
		log,
		"http://localhost",
		d.propertyArchiver,
		nil,
		d.inUseChecker,
	)
	return d
}

func captureLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewTextHandler(&buf, nil)), &buf
}

func TestBilling_ProcessRenewals_RecoverAfterChargeTimeoutProviderSucceeds(t *testing.T) {
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

	count, err := d.service.Renewals.ProcessRenewals(context.Background(), fixedNow)
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

func TestBilling_ProcessRenewals_RecoverAfterChargeTimeoutProviderUnknownKeepsActive(t *testing.T) {
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
			// The pre-charge status query reports Pending so the Charge attempt
			// still happens; the recovery status query uses the subtest values.
			statusCalls := 0
			d.provider.statusFunc = func(uuid.UUID, string) {
				statusCalls++
				if statusCalls == 1 {
					d.provider.statusRes = domain.PaymentStatusPending
					d.provider.statusErr = nil
					return
				}
				d.provider.statusRes = tt.statusRes
				d.provider.statusErr = tt.statusErr
			}

			count, err := d.service.Renewals.ProcessRenewals(context.Background(), fixedNow)
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

// renewalTestPayment returns the renewal payment created for the subscription.
func renewalTestPayment(t *testing.T, d *testDeps, sub domain.Subscription) domain.SubscriptionPayment {
	t.Helper()
	for _, p := range d.subscriptionPayments.payments {
		if p.SubscriptionID == sub.ID {
			return p
		}
	}
	t.Fatal("expected renewal payment to be created")
	return domain.SubscriptionPayment{}
}

func TestBilling_ProcessRenewals_ChargeAttemptLimitMovesToGrace(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111121")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222121")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333121")
	methodID := uuid.MustParse("44444444-4444-4444-4444-444444444121")
	subID := uuid.MustParse("55555555-5555-5555-5555-555555555121")
	validUntil := fixedNow

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
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

	// Every charge attempt fails and the provider keeps reporting Pending, so
	// recovery counts an attempt on each tick. After maxRenewalChargeAttempts
	// ticks the payment must be marked failed and the subscription moved to
	// grace instead of staying active with an expired valid_until forever.
	d.provider.chargeRes = ChargeResult{ProviderPaymentID: "charge_timeout_limit", Status: domain.PaymentStatusSucceeded}
	d.provider.chargeErr = errors.New("provider timeout")
	d.provider.statusRes = domain.PaymentStatusPending

	for tick := 1; tick <= maxRenewalChargeAttempts; tick++ {
		count, err := d.service.Renewals.ProcessRenewals(context.Background(), fixedNow)
		if err != nil {
			t.Fatalf("tick %d: ProcessRenewals error: %v", tick, err)
		}
		if count != 1 {
			t.Fatalf("tick %d: expected 1 renewal attempt, got %d", tick, count)
		}

		sub := d.subscriptions.subs[userID]
		payment := renewalTestPayment(t, d, sub)
		if tick < maxRenewalChargeAttempts {
			if sub.Status != domain.SubscriptionStatusActive {
				t.Fatalf("tick %d: expected status active before attempt limit, got %s", tick, sub.Status)
			}
			if payment.Status != domain.PaymentStatusPending {
				t.Fatalf("tick %d: expected payment pending before attempt limit, got %s", tick, payment.Status)
			}
			if payment.ChargeAttempts != tick {
				t.Fatalf("tick %d: expected %d charge attempts, got %d", tick, tick, payment.ChargeAttempts)
			}
		}
	}

	sub := d.subscriptions.subs[userID]
	if sub.Status != domain.SubscriptionStatusGrace {
		t.Errorf("expected status grace after %d failed attempts, got %s", maxRenewalChargeAttempts, sub.Status)
	}
	payment := renewalTestPayment(t, d, sub)
	if payment.Status != domain.PaymentStatusFailed {
		t.Errorf("expected payment failed after %d attempts, got %s", maxRenewalChargeAttempts, payment.Status)
	}
	if payment.ChargeAttempts != maxRenewalChargeAttempts {
		t.Errorf("expected %d charge attempts, got %d", maxRenewalChargeAttempts, payment.ChargeAttempts)
	}
	if d.provider.chargeCount != maxRenewalChargeAttempts {
		t.Errorf("expected %d provider charge calls, got %d", maxRenewalChargeAttempts, d.provider.chargeCount)
	}
}

func TestBilling_ProcessRenewals_StatusErrorDoesNotCountChargeAttempt(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111122")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222122")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333122")
	methodID := uuid.MustParse("44444444-4444-4444-4444-444444444122")
	validUntil := fixedNow

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addSubscription(domain.Subscription{
		ID:                    uuid.MustParse("55555555-5555-5555-5555-555555555122"),
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

	d.provider.chargeRes = ChargeResult{ProviderPaymentID: "charge_status_err", Status: domain.PaymentStatusSucceeded}
	d.provider.chargeErr = errors.New("provider timeout")
	// Pre-charge status queries report Pending so each tick attempts a Charge;
	// recovery status queries (issued after a failed Charge) fail, so the
	// attempt counter must never grow and the subscription must stay active.
	chargeSeen := 0
	d.provider.statusFunc = func(uuid.UUID, string) {
		if d.provider.chargeCount > chargeSeen {
			chargeSeen = d.provider.chargeCount
			d.provider.statusRes = domain.PaymentStatusPending
			d.provider.statusErr = errors.New("provider unreachable")
			return
		}
		d.provider.statusRes = domain.PaymentStatusPending
		d.provider.statusErr = nil
	}

	ticks := maxRenewalChargeAttempts + 2
	for tick := 1; tick <= ticks; tick++ {
		count, err := d.service.Renewals.ProcessRenewals(context.Background(), fixedNow)
		if err != nil {
			t.Fatalf("tick %d: ProcessRenewals error: %v", tick, err)
		}
		if count != 1 {
			t.Fatalf("tick %d: expected 1 renewal attempt, got %d", tick, count)
		}
	}

	sub := d.subscriptions.subs[userID]
	if sub.Status != domain.SubscriptionStatusActive {
		t.Errorf("expected status active, got %s", sub.Status)
	}
	payment := renewalTestPayment(t, d, sub)
	if payment.Status != domain.PaymentStatusPending {
		t.Errorf("expected payment pending, got %s", payment.Status)
	}
	if payment.ChargeAttempts != 0 {
		t.Errorf("expected 0 charge attempts counted on status errors, got %d", payment.ChargeAttempts)
	}
	if d.provider.chargeCount != ticks {
		t.Errorf("expected %d provider charge calls, got %d", ticks, d.provider.chargeCount)
	}
}

func TestBilling_ProcessRenewals_SkipsRechargeWhenProviderStatusUnknown(t *testing.T) {
	log, logBuf := captureLogger()
	d := newTestDepsWithLogger(t, log)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111123")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222123")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333123")
	methodID := uuid.MustParse("44444444-4444-4444-4444-444444444123")
	validUntil := fixedNow

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addSubscription(domain.Subscription{
		ID:                    uuid.MustParse("55555555-5555-5555-5555-555555555123"),
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

	// First tick: the pre-charge status query reports Pending so the Charge
	// happens and is "lost" (error response). All later status queries fail,
	// so the next tick must not blindly re-charge a payment that may already
	// be charged on the provider side.
	d.provider.chargeRes = ChargeResult{ProviderPaymentID: "charge_lost", Status: domain.PaymentStatusSucceeded}
	d.provider.chargeErr = errors.New("provider timeout")
	statusCalls := 0
	d.provider.statusFunc = func(uuid.UUID, string) {
		statusCalls++
		if statusCalls == 1 {
			d.provider.statusRes = domain.PaymentStatusPending
			d.provider.statusErr = nil
			return
		}
		d.provider.statusErr = errors.New("provider unreachable")
	}

	for range 2 {
		if _, err := d.service.Renewals.ProcessRenewals(context.Background(), fixedNow); err != nil {
			t.Fatalf("ProcessRenewals error: %v", err)
		}
	}

	if d.provider.chargeCount != 1 {
		t.Errorf("expected exactly 1 charge attempt, got %d", d.provider.chargeCount)
	}
	sub := d.subscriptions.subs[userID]
	if sub.Status != domain.SubscriptionStatusActive {
		t.Errorf("expected status active, got %s", sub.Status)
	}
	payment := renewalTestPayment(t, d, sub)
	if payment.Status != domain.PaymentStatusPending {
		t.Errorf("expected payment pending, got %s", payment.Status)
	}
	if !strings.Contains(logBuf.String(), "provider status unavailable; skipping charge attempt this tick") {
		t.Errorf("expected skip-charge log, got:\n%s", logBuf.String())
	}
}

func TestBilling_ProcessRenewals_BatchWithoutProgressStops(t *testing.T) {
	d := newTestDeps(t)
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222124")
	missingTariffID := uuid.MustParse("33333333-3333-3333-3333-333333333124")
	validUntil := fixedNow

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})

	// A full batch of subscriptions whose renewal always fails (the tariff is
	// missing) stays in the selection; the batch loop must break after one
	// fruitless iteration instead of spinning forever within a single tick.
	for range renewalBatchSize {
		userID := uuid.New()
		d.addSubscription(domain.Subscription{
			ID:               uuid.New(),
			UserID:           userID,
			TariffID:         missingTariffID,
			Source:           domain.SubscriptionSourcePaid,
			Status:           domain.SubscriptionStatusActive,
			ValidUntil:       &validUntil,
			AutoRenewEnabled: true,
		})
	}

	count, err := d.service.Renewals.ProcessRenewals(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("ProcessRenewals error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 processed renewals, got %d", count)
	}
	if d.subscriptions.listUpForRenewalCalls != 1 {
		t.Errorf("expected 1 list call (no-progress break), got %d", d.subscriptions.listUpForRenewalCalls)
	}
}

func TestBilling_ProcessExpiredGrace_BatchWithoutProgressStops(t *testing.T) {
	d := newTestDeps(t)
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222125")
	validUntil := fixedNow

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})

	// A full batch of grace subscriptions whose downgrade always fails (the
	// transaction cannot begin) stays in the selection; the batch loop must
	// break after one fruitless iteration instead of spinning forever.
	d.beginner.beginErr = errors.New("database unavailable")
	for range graceBatchSize {
		userID := uuid.New()
		d.addSubscription(domain.Subscription{
			ID:         uuid.New(),
			UserID:     userID,
			TariffID:   uuid.New(),
			Source:     domain.SubscriptionSourcePaid,
			Status:     domain.SubscriptionStatusGrace,
			ValidUntil: &validUntil,
		})
	}

	count, err := d.service.Renewals.ProcessExpiredGrace(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("ProcessExpiredGrace error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 processed subscriptions, got %d", count)
	}
	if d.subscriptions.listInExpiredGraceCalls != 1 {
		t.Errorf("expected 1 list call (no-progress break), got %d", d.subscriptions.listInExpiredGraceCalls)
	}
}

func TestBilling_ProcessRenewals_ArchivingFailureDoesNotRollbackPayment(t *testing.T) {
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

	count, err := d.service.Renewals.ProcessRenewals(context.Background(), fixedNow)
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

func TestBilling_ChangeTariff_UpgradeRecoversProviderReferenceAfterCrash(t *testing.T) {
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

	resp, err := d.service.Subscriptions.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: domain.TariffPro,
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
	if !d.provider.initReq.RedirectDueDate.Equal(fixedNow.Add(15 * time.Minute)) {
		t.Errorf("expected redirect due date %v, got %v", fixedNow.Add(15*time.Minute), d.provider.initReq.RedirectDueDate)
	}

	payment := d.subscriptionPayments.payments[existingPaymentID]
	if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID == "" {
		t.Error("expected provider payment id recovered and persisted")
	}
	if payment.PaymentMethodID == nil {
		t.Error("expected payment method id recovered and persisted")
	}
}

func TestBilling_ProcessRenewals_ApplyTariffChangeFailureDoesNotRollbackPayment(t *testing.T) {
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

	count, err := d.service.Renewals.ProcessRenewals(context.Background(), fixedNow)
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

	count, err = d.service.Renewals.ProcessRenewals(context.Background(), fixedNow)
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

func TestBilling_HandleWebhook_ApplyTariffChangeFailureDoesNotRollbackPayment(t *testing.T) {
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

	if err := d.service.Webhooks.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
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

	if err := d.service.Webhooks.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
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

func TestBilling_ConfirmFakePayment_ArchivesExcessPropertiesAfterTariffChange(t *testing.T) {
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

	if err := d.service.Payments.ConfirmFakePayment(context.Background(), paymentID); err != nil {
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

func TestBilling_HandleWebhook_ArchivesExcessPropertiesAfterTariffChange(t *testing.T) {
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

	if err := d.service.Webhooks.HandleWebhook(context.Background(), "fake", []byte(`{}`)); err != nil {
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

func TestBilling_ProcessRenewals_ProviderErrorSanitizedInLogs(t *testing.T) {
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

	_, err := d.service.Renewals.ProcessRenewals(context.Background(), fixedNow)
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

func TestBilling_AddPaymentMethod_TkassaReturnsConfirmURL(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccd")

	d.provider.initAddCardRes = InitAddCardResult{PaymentURL: "https://bank.example/add-card"}

	resp, err := d.service.PaymentMethods.AddPaymentMethod(context.Background(), userID, AddPaymentMethodRequest{
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
	if want := "http://localhost/api/subscription/payment-methods/add-card/success"; d.provider.initAddCardReq.SuccessURL != want {
		t.Errorf("expected success url %s, got %s", want, d.provider.initAddCardReq.SuccessURL)
	}
	if want := "http://localhost/api/subscription/payment-methods/add-card/fail"; d.provider.initAddCardReq.FailURL != want {
		t.Errorf("expected fail url %s, got %s", want, d.provider.initAddCardReq.FailURL)
	}
}

func TestBilling_SyncPaymentMethods_ImportsCardActivatesAndLinksSubscription(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777780")
	tariffID := uuid.MustParse("99999999-9999-9999-9999-999999999980")
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{
		ID: tariffID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaa80"),
		UserID:           userID,
		TariffID:         tariffID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: true,
	})

	d.provider.getCardListRes = []ProviderCard{
		{
			CardID:   "card_sync_1",
			Pan:      "430000******0777",
			ExpDate:  "12/30",
			RebillID: "rebill_sync_1",
			Status:   ProviderCardStatusActive,
		},
	}

	methods, err := d.service.PaymentMethods.SyncPaymentMethods(context.Background(), userID)
	if err != nil {
		t.Fatalf("SyncPaymentMethods error: %v", err)
	}
	if !d.provider.getCardListCalled {
		t.Fatal("expected provider.GetCardList to be called")
	}
	if d.provider.getCardListCustomerKey != userID.String() {
		t.Errorf("expected customer key %s, got %s", userID.String(), d.provider.getCardListCustomerKey)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 payment method, got %d", len(methods))
	}
	pm := methods[0]
	if pm.ProviderToken != "rebill_sync_1" {
		t.Errorf("expected provider token rebill_sync_1, got %s", pm.ProviderToken)
	}
	if pm.ProviderCardID != "card_sync_1" {
		t.Errorf("expected card id card_sync_1, got %s", pm.ProviderCardID)
	}
	if pm.DisplayMask != "430000******0777" {
		t.Errorf("expected display mask 430000******0777, got %s", pm.DisplayMask)
	}
	if pm.ExpDate != "12/30" {
		t.Errorf("expected exp date 12/30, got %s", pm.ExpDate)
	}
	if !pm.IsActive {
		t.Error("expected synced payment method to become active")
	}

	sub, ok := d.subscriptions.subs[userID]
	if !ok {
		t.Fatal("expected subscription to exist")
	}
	if sub.ActivePaymentMethodID == nil || *sub.ActivePaymentMethodID != pm.ID {
		t.Errorf("expected subscription active payment method %s, got %v", pm.ID, sub.ActivePaymentMethodID)
	}
}

func TestBilling_SyncPaymentMethods_RepeatedSyncKeepsSingleRowAndActiveMethod(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777782")

	existingID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb2")
	d.paymentMethods.methods[existingID] = domain.PaymentMethod{
		ID:            existingID,
		UserID:        userID,
		Provider:      domain.ProviderTkassa,
		ProviderToken: "existing_token",
		DisplayMask:   "****0777",
		IsActive:      true,
		CreatedAt:     fixedNow.Add(-time.Hour),
		UpdatedAt:     fixedNow.Add(-time.Hour),
	}

	d.provider.getCardListRes = []ProviderCard{
		{
			CardID:   "card_sync_2",
			Pan:      "430000******0888",
			ExpDate:  "12/31",
			RebillID: "rebill_sync_2",
			Status:   ProviderCardStatusActive,
		},
	}

	if _, err := d.service.PaymentMethods.SyncPaymentMethods(context.Background(), userID); err != nil {
		t.Fatalf("first SyncPaymentMethods error: %v", err)
	}
	if len(d.paymentMethods.methods) != 2 {
		t.Fatalf("expected 2 payment methods after first sync, got %d", len(d.paymentMethods.methods))
	}

	methods, err := d.service.PaymentMethods.SyncPaymentMethods(context.Background(), userID)
	if err != nil {
		t.Fatalf("second SyncPaymentMethods error: %v", err)
	}
	if len(methods) != 2 {
		t.Fatalf("expected 2 payment methods after repeated sync, got %d", len(methods))
	}
	if len(d.paymentMethods.methods) != 2 {
		t.Fatalf("expected repeated sync to keep 2 stored methods, got %d", len(d.paymentMethods.methods))
	}

	for _, pm := range d.paymentMethods.methods {
		if pm.ID == existingID && !pm.IsActive {
			t.Error("expected previously active method to stay active")
		}
		if pm.ProviderToken == "rebill_sync_2" && pm.IsActive {
			t.Error("expected sync not to switch the active method")
		}
	}
}

func TestBilling_SyncPaymentMethods_SkipsInactiveAndTokenlessCards(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777783")

	d.provider.getCardListRes = []ProviderCard{
		{CardID: "card_no_rebill", Pan: "430000******0777", ExpDate: "12/30", Status: ProviderCardStatusActive},
		{CardID: "card_inactive", Pan: "430000******0888", ExpDate: "12/31", RebillID: "rebill_inactive", Status: ProviderCardStatusInactive},
		{CardID: "card_deleted", Pan: "430000******0999", ExpDate: "12/32", RebillID: "rebill_deleted", Status: ProviderCardStatusDeleted},
	}

	methods, err := d.service.PaymentMethods.SyncPaymentMethods(context.Background(), userID)
	if err != nil {
		t.Fatalf("SyncPaymentMethods error: %v", err)
	}
	if len(methods) != 0 {
		t.Fatalf("expected no payment methods, got %d", len(methods))
	}
	if len(d.paymentMethods.methods) != 0 {
		t.Fatalf("expected no stored payment methods, got %d", len(d.paymentMethods.methods))
	}
}

func TestBilling_SyncPaymentMethods_ProviderErrorPropagatesWithoutWrites(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777784")
	d.provider.getCardListErr = errors.New("provider unavailable")

	_, err := d.service.PaymentMethods.SyncPaymentMethods(context.Background(), userID)
	if err == nil {
		t.Fatal("expected error when provider GetCardList fails")
	}
	if len(d.paymentMethods.methods) != 0 {
		t.Fatalf("expected no stored payment methods, got %d", len(d.paymentMethods.methods))
	}
	if d.beginner.begun != 0 {
		t.Fatalf("expected no transaction to begin on provider error, got %d", d.beginner.begun)
	}
}

func TestBilling_SyncPaymentMethods_TerminalNotFoundReturnsLocalList(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777785")

	existingID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb5")
	d.paymentMethods.methods[existingID] = domain.PaymentMethod{
		ID:            existingID,
		UserID:        userID,
		Provider:      domain.ProviderTkassa,
		ProviderToken: "existing_token",
		DisplayMask:   "****0777",
		IsActive:      true,
		CreatedAt:     fixedNow.Add(-time.Hour),
		UpdatedAt:     fixedNow.Add(-time.Hour),
	}

	d.provider.getCardListErr = ErrProviderTerminalNotFound

	methods, err := d.service.PaymentMethods.SyncPaymentMethods(context.Background(), userID)
	if err != nil {
		t.Fatalf("SyncPaymentMethods error: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 local payment method, got %d", len(methods))
	}
	if methods[0].ID != existingID {
		t.Errorf("expected local method %s, got %s", existingID, methods[0].ID)
	}
	if d.beginner.begun != 0 {
		t.Fatalf("expected no transaction on terminal-not-found, got %d", d.beginner.begun)
	}
}

func TestBilling_ChangeTariff_TkassaFirstPaymentInitFields(t *testing.T) {
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

	resp, err := d.service.Subscriptions.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: domain.TariffPro,
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
	if !req.RedirectDueDate.Equal(fixedNow.Add(15 * time.Minute)) {
		t.Errorf("expected redirect due date %v, got %v", fixedNow.Add(15*time.Minute), req.RedirectDueDate)
	}

	payment := d.subscriptionPayments.payments[resp.PaymentID]
	if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID != "tkassa_payment_1" {
		t.Errorf("expected provider payment id set, got %v", payment.ProviderPaymentID)
	}
	if payment.PaymentMethodID != nil {
		t.Errorf("expected no local payment method until webhook, got %v", payment.PaymentMethodID)
	}
}

func TestBilling_ChangeTariff_TkassaReturnsExistingPendingPaymentURL(t *testing.T) {
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

	resp, err := d.service.Subscriptions.ChangeTariff(context.Background(), userID, ChangeTariffRequest{
		TariffName: domain.TariffPro,
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

func TestBilling_ProcessRenewals_TkassaInitChargeFlow(t *testing.T) {
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

	count, err := d.service.Renewals.ProcessRenewals(context.Background(), fixedNow)
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
	if initReq.Recurrent {
		t.Errorf("expected Recurrent false for renewal, got true")
	}
	if !initReq.RedirectDueDate.IsZero() {
		t.Errorf("expected no redirect due date for renewal, got %v", initReq.RedirectDueDate)
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

func TestBilling_ProcessRenewals_TkassaInitFailureMovesToGrace(t *testing.T) {
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

	count, err := d.service.Renewals.ProcessRenewals(context.Background(), fixedNow)
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

func TestBilling_HandleWebhook_TkassaAuthorizedThenConfirmed(t *testing.T) {
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

	if err := d.service.Webhooks.HandleWebhook(context.Background(), "tkassa", []byte(`{}`)); err != nil {
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

	if err := d.service.Webhooks.HandleWebhook(context.Background(), "tkassa", []byte(`{}`)); err != nil {
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

func TestBilling_HandleWebhook_TkassaAddCard(t *testing.T) {
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

	if err := d.service.Webhooks.HandleWebhook(context.Background(), "tkassa", []byte(`{}`)); err != nil {
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

func TestBilling_HandleWebhook_AddCardLinksActivePaymentMethodToSubscription(t *testing.T) {
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

	if err := d.service.Webhooks.HandleWebhook(t.Context(), "tkassa", []byte(`{}`)); err != nil {
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

func TestBilling_HandleWebhook_TkassaAddCardLegacy(t *testing.T) {
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

	if err := d.service.Webhooks.HandleWebhook(context.Background(), "tkassa", []byte(`{}`)); err != nil {
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

func TestBilling_HandleWebhook_DuplicateConfirmedIsIdempotent(t *testing.T) {
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

	if err := d.service.Webhooks.HandleWebhook(context.Background(), "tkassa", []byte(`{}`)); err != nil {
		t.Fatalf("first HandleWebhook error: %v", err)
	}
	firstValidUntil := d.subscriptions.subs[userID].ValidUntil

	if err := d.service.Webhooks.HandleWebhook(context.Background(), "tkassa", []byte(`{}`)); err != nil {
		t.Fatalf("second HandleWebhook error: %v", err)
	}
	if !d.subscriptions.subs[userID].ValidUntil.Equal(*firstValidUntil) {
		t.Error("expected duplicate webhook to be idempotent")
	}
}

func TestBilling_ProcessPendingUpgradePayments_SucceededFinalizesUpgrade(t *testing.T) {
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

	count, err := d.service.Renewals.ProcessPendingUpgradePayments(context.Background(), fixedNow)
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

func TestBilling_ProcessPendingUpgradePayments_FailedMarksFailed(t *testing.T) {
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

	count, err := d.service.Renewals.ProcessPendingUpgradePayments(context.Background(), fixedNow)
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

func TestBilling_ProcessPendingUpgradePayments_PendingLeavesAlone(t *testing.T) {
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

	count, err := d.service.Renewals.ProcessPendingUpgradePayments(context.Background(), fixedNow)
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

func TestBilling_DeletePaymentMethod_TkassaRemoveCardFailureIsBestEffort(t *testing.T) {
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

	if err := d.service.PaymentMethods.DeletePaymentMethod(context.Background(), userID, methodID); err != nil {
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

func TestBilling_RefundPayment_SucceedsAndDowngradesToBasic(t *testing.T) {
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

	if err := d.service.Payments.RefundPayment(t.Context(), uuid.Nil, paymentID); err != nil {
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

func TestBilling_RefundPayment_PendingPaymentSucceeds(t *testing.T) {
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

	if err := d.service.Payments.RefundPayment(t.Context(), uuid.Nil, paymentID); err != nil {
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

func TestBilling_RefundPayment_FullRefund(t *testing.T) {
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

	if err := d.service.Payments.RefundPayment(t.Context(), uuid.Nil, paymentID); err != nil {
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

func TestBilling_RefundPayment_RejectedForNonSucceededOrPendingPayment(t *testing.T) {
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

	err := d.service.Payments.RefundPayment(t.Context(), uuid.Nil, paymentID)
	if !errors.Is(err, domain.ErrInvalidPaymentStatus) {
		t.Fatalf("expected ErrInvalidPaymentStatus, got %v", err)
	}
	if d.provider.cancelCalled {
		t.Error("expected provider.Cancel not to be called for non-succeeded/non-pending payment")
	}
}

func TestBilling_RefundPayment_ProviderCancelError(t *testing.T) {
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

	err := d.service.Payments.RefundPayment(t.Context(), uuid.Nil, paymentID)
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

func TestBilling_RefundPayment_RejectedWhenProviderPaymentIDMissing(t *testing.T) {
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

	err := d.service.Payments.RefundPayment(t.Context(), uuid.Nil, paymentID)
	if !errors.Is(err, domain.ErrInvalidPaymentStatus) {
		t.Fatalf("expected ErrInvalidPaymentStatus, got %v", err)
	}
	if d.provider.cancelCalled {
		t.Error("expected provider.Cancel not to be called when provider payment id is missing")
	}
}

func TestBilling_RefundPayment_DoubleRefundRejected(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777771")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-888888888881")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1")
	basicID := uuid.MustParse("00000000-0000-0000-0000-0000000000b1")
	proID := uuid.MustParse("00000000-0000-0000-0000-0000000000c1")
	providerPaymentID := "stub_double_refund"
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

	if err := d.service.Payments.RefundPayment(t.Context(), uuid.Nil, paymentID); err != nil {
		t.Fatalf("first RefundPayment error: %v", err)
	}

	// A second refund on the same (now refunded) payment must be rejected and
	// must NOT reach the provider or downgrade the subscription a second time.
	err := d.service.Payments.RefundPayment(t.Context(), uuid.Nil, paymentID)
	if !errors.Is(err, domain.ErrInvalidPaymentStatus) {
		t.Fatalf("second RefundPayment error = %v, want ErrInvalidPaymentStatus", err)
	}

	if d.provider.cancelCount != 1 {
		t.Errorf("provider.Cancel called %d times, want exactly 1", d.provider.cancelCount)
	}
	if len(d.propertyArchiver.calls) != 1 {
		t.Errorf("downgrade applied %d times, want exactly 1", len(d.propertyArchiver.calls))
	}
	if got := d.subscriptionPayments.payments[paymentID].Status; got != domain.PaymentStatusRefunded {
		t.Errorf("payment status = %s, want refunded", got)
	}
	if got := d.subscriptions.subs[userID].TariffID; got != basicID {
		t.Errorf("subscription tariff = %s, want basic %s", got, basicID)
	}
}

func TestBilling_RefundPayment_CancelFailureRevertsStatus(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777772")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-888888888882")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa2")
	basicID := uuid.MustParse("00000000-0000-0000-0000-0000000000b2")
	proID := uuid.MustParse("00000000-0000-0000-0000-0000000000c2")
	providerPaymentID := "stub_cancel_fail_revert"
	validUntil := fixedNow.AddDate(0, 1, 0)
	cancelErr := errors.New("provider refused refund")

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
	d.provider.cancelErr = cancelErr

	err := d.service.Payments.RefundPayment(t.Context(), uuid.Nil, paymentID)
	if !errors.Is(err, cancelErr) {
		t.Fatalf("expected provider cancel error, got %v", err)
	}

	// The refunding reservation must be reverted to the previous status, and the
	// subscription must NOT be downgraded.
	if got := d.subscriptionPayments.payments[paymentID].Status; got != domain.PaymentStatusSucceeded {
		t.Errorf("status after failed cancel = %s, want succeeded (reverted)", got)
	}
	if got := d.subscriptions.subs[userID].TariffID; got != proID {
		t.Errorf("subscription tariff = %s, want pro %s (must not downgrade)", got, proID)
	}
	if len(d.propertyArchiver.calls) != 0 {
		t.Errorf("downgrade applied %d times, want 0", len(d.propertyArchiver.calls))
	}
	if d.provider.cancelCount != 1 {
		t.Errorf("provider.Cancel called %d times, want 1", d.provider.cancelCount)
	}

	// The reverted payment can be refunded again with a successful cancel.
	d.provider.cancelErr = nil
	d.provider.cancelRes = CancelResult{
		ProviderPaymentID:     providerPaymentID,
		Status:                domain.PaymentStatusRefunded,
		RefundedAmountKopecks: 5000,
	}
	if err := d.service.Payments.RefundPayment(t.Context(), uuid.Nil, paymentID); err != nil {
		t.Fatalf("retry RefundPayment error: %v", err)
	}
	if got := d.subscriptionPayments.payments[paymentID].Status; got != domain.PaymentStatusRefunded {
		t.Errorf("status after retry = %s, want refunded", got)
	}
	if got := d.subscriptions.subs[userID].TariffID; got != basicID {
		t.Errorf("subscription tariff = %s, want basic %s", got, basicID)
	}
	if d.provider.cancelCount != 2 {
		t.Errorf("provider.Cancel called %d times, want 2", d.provider.cancelCount)
	}
}

func TestBilling_RefundPayment_CancelRefundingKeepsReservation(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777775")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-888888888885")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa5")
	basicID := uuid.MustParse("00000000-0000-0000-0000-0000000000b5")
	proID := uuid.MustParse("00000000-0000-0000-0000-0000000000c5")
	providerPaymentID := "stub_cancel_refunding"
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
		ProviderPaymentID: providerPaymentID,
		Status:            domain.PaymentStatusRefunding,
	}

	if err := d.service.Payments.RefundPayment(t.Context(), uuid.Nil, paymentID); err != nil {
		t.Fatalf("RefundPayment error: %v", err)
	}

	// The refund is in flight at the provider: the reservation must stay in
	// refunding (NOT reverted to succeeded) and the subscription must NOT be
	// downgraded. The ReconcileStaleRefunds watchdog resolves the payment.
	if got := d.subscriptionPayments.payments[paymentID].Status; got != domain.PaymentStatusRefunding {
		t.Errorf("status after in-flight refund = %s, want refunding (kept for the watchdog)", got)
	}
	if got := d.subscriptions.subs[userID].TariffID; got != proID {
		t.Errorf("subscription tariff = %s, want pro %s (must not downgrade)", got, proID)
	}
	if len(d.propertyArchiver.calls) != 0 {
		t.Errorf("downgrade applied %d times, want 0", len(d.propertyArchiver.calls))
	}
	if d.provider.cancelCount != 1 {
		t.Errorf("provider.Cancel called %d times, want 1", d.provider.cancelCount)
	}
}

func TestBilling_RefundPayment_CancelPartialRefundedStaysRefunding(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777776")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-888888888886")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa6")
	basicID := uuid.MustParse("00000000-0000-0000-0000-0000000000b6")
	proID := uuid.MustParse("00000000-0000-0000-0000-0000000000c6")
	providerPaymentID := "stub_cancel_partial_refunded"
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
		Status:                domain.PaymentStatusPartialRefunded,
		RefundedAmountKopecks: 2500,
	}

	if err := d.service.Payments.RefundPayment(t.Context(), uuid.Nil, paymentID); err != nil {
		t.Fatalf("RefundPayment error: %v", err)
	}

	// A partial refund answers a full-amount Cancel we never make: it is an
	// anomaly. The payment must stay in refunding for manual review with no
	// refunded amount recorded and no subscription downgrade.
	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusRefunding {
		t.Errorf("status after partial-refund anomaly = %s, want refunding (kept for manual review)", payment.Status)
	}
	if payment.RefundedAmountKopecks != nil {
		t.Errorf("refunded amount = %v, want nil (no refund finalized)", *payment.RefundedAmountKopecks)
	}
	if got := d.subscriptions.subs[userID].TariffID; got != proID {
		t.Errorf("subscription tariff = %s, want pro %s (must not downgrade)", got, proID)
	}
	if len(d.propertyArchiver.calls) != 0 {
		t.Errorf("downgrade applied %d times, want 0", len(d.propertyArchiver.calls))
	}
	if d.provider.cancelCount != 1 {
		t.Errorf("provider.Cancel called %d times, want 1", d.provider.cancelCount)
	}
}

// syncSeed describes the tariff/subscription/payment fixture shared by the
// SyncPendingPayment table setups: basic and pro tariffs, an active paid
// subscription on one of them, and a pending pro payment for it.
type syncSeed struct {
	userID            string
	basicID           string
	proID             string
	paymentID         string
	subscriptionID    string
	providerPaymentID string
	subOnPro          bool // subscription tariff: pro when true, basic when false
	autoRenew         bool
}

// syncScenarioSetup returns a setup that seeds the fixture described by seed
// and yields the ID of the pending payment.
func syncScenarioSetup(seed syncSeed) func(*testDeps) uuid.UUID {
	return func(d *testDeps) uuid.UUID {
		userID := uuid.MustParse(seed.userID)
		basicID := uuid.MustParse(seed.basicID)
		proID := uuid.MustParse(seed.proID)
		paymentID := uuid.MustParse(seed.paymentID)
		subscriptionID := uuid.MustParse(seed.subscriptionID)
		validUntil := fixedNow.AddDate(0, 1, 0)
		subTariffID := basicID
		if seed.subOnPro {
			subTariffID = proID
		}

		d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000})
		d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
		d.addSubscription(domain.Subscription{
			ID:               subscriptionID,
			UserID:           userID,
			TariffID:         subTariffID,
			Source:           domain.SubscriptionSourcePaid,
			Status:           domain.SubscriptionStatusActive,
			ValidUntil:       &validUntil,
			AutoRenewEnabled: seed.autoRenew,
		})
		d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
			ID:                paymentID,
			UserID:            userID,
			SubscriptionID:    subscriptionID,
			TariffID:          proID,
			Period:            domain.PeriodMonth,
			AmountKopecks:     5000,
			Provider:          domain.ProviderFake,
			ProviderPaymentID: &seed.providerPaymentID,
			Status:            domain.PaymentStatusPending,
			CreatedAt:         fixedNow.Add(-10 * time.Minute),
			UpdatedAt:         fixedNow.Add(-10 * time.Minute),
		}
		return paymentID
	}
}

func TestBilling_SyncPendingPayment(t *testing.T) {
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

				err := d.service.Payments.SyncPendingPayment(context.Background(), uuid.Nil, paymentID)

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
					t.Helper()
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
					t.Helper()
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
					t.Helper()
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
				setup: syncScenarioSetup(syncSeed{
					userID:            "11111111-1111-1111-1111-111111111114",
					basicID:           "22222222-2222-2222-2222-222222222225",
					proID:             "33333333-3333-3333-3333-333333333336",
					paymentID:         "44444444-4444-4444-4444-444444444447",
					subscriptionID:    "55555555-5555-5555-5555-555555555558",
					providerPaymentID: "sync_upgrade_failed",
					subOnPro:          false,
					autoRenew:         false,
				}),
				providerStatus:     domain.PaymentStatusFailed,
				wantProviderCalled: true,
				assert: func(t *testing.T, d *testDeps, paymentID uuid.UUID) {
					t.Helper()
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
				setup: syncScenarioSetup(syncSeed{
					userID:            "11111111-1111-1111-1111-11111111111a",
					basicID:           "22222222-2222-2222-2222-22222222222c",
					proID:             "33333333-3333-3333-3333-33333333333d",
					paymentID:         "44444444-4444-4444-4444-44444444444e",
					subscriptionID:    "55555555-5555-5555-5555-55555555555f",
					providerPaymentID: "sync_refunded",
					subOnPro:          true,
					autoRenew:         true,
				}),
				providerStatus:     domain.PaymentStatusRefunded,
				wantProviderCalled: true,
				assert: func(t *testing.T, d *testDeps, paymentID uuid.UUID) {
					t.Helper()
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

	t.Run("partial_refunded ignored as anomaly", func(t *testing.T) {
		runSyncCases(t, []syncCase{
			{
				name: "ignores external partial refund notification without state change",
				setup: syncScenarioSetup(syncSeed{
					userID:            "11111111-1111-1111-1111-11111111111b",
					basicID:           "22222222-2222-2222-2222-22222222222d",
					proID:             "33333333-3333-3333-3333-33333333333e",
					paymentID:         "44444444-4444-4444-4444-44444444444f",
					subscriptionID:    "55555555-5555-5555-5555-555555555550",
					providerPaymentID: "sync_partial_refunded",
					subOnPro:          true,
					autoRenew:         true,
				}),
				providerStatus:     domain.PaymentStatusPartialRefunded,
				wantProviderCalled: true,
				assert: func(t *testing.T, d *testDeps, paymentID uuid.UUID) {
					t.Helper()
					payment := d.subscriptionPayments.payments[paymentID]
					// Partial refunds are impossible in this product: the external
					// notification is an anomaly that is logged and ignored.
					if payment.Status != domain.PaymentStatusPending {
						t.Errorf("expected payment to stay pending, got %s", payment.Status)
					}
					if payment.RefundedAmountKopecks != nil {
						t.Errorf("expected refunded amount to stay nil, got %v", *payment.RefundedAmountKopecks)
					}
					sub := d.subscriptions.subs[payment.UserID]
					proID := uuid.MustParse("33333333-3333-3333-3333-33333333333e")
					if sub.TariffID != proID {
						t.Errorf("expected subscription to stay on pro tariff, got %s", sub.TariffID)
					}
					if sub.Status != domain.SubscriptionStatusActive {
						t.Errorf("expected subscription active, got %s", sub.Status)
					}
					if !sub.AutoRenewEnabled {
						t.Error("expected auto-renew to stay enabled")
					}
					if len(d.propertyArchiver.calls) != 0 {
						t.Errorf("expected no archiver calls, got %d", len(d.propertyArchiver.calls))
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
					t.Helper()
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
					t.Helper()
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

func TestBilling_SyncPendingPayment_UnexpectedProviderStatus(t *testing.T) {
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

	err := d.service.Payments.SyncPendingPayment(context.Background(), uuid.Nil, paymentID)
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

func TestBilling_SyncPendingPayment_DoubleFinalizeGuard(t *testing.T) {
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

	if err := d.service.Payments.SyncPendingPayment(context.Background(), uuid.Nil, paymentID); err != nil {
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

func TestBilling_ReconcilePendingPayments_SyncsInBatches(t *testing.T) {
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
		for i := range totalPayments {
			userID := uuid.MustParse(fmt.Sprintf("11111111-1111-1111-1111-%012d", i))
			subscriptionID := uuid.MustParse(fmt.Sprintf("22222222-2222-2222-2222-%012d", i))
			paymentID := uuid.MustParse(fmt.Sprintf("33333333-3333-3333-3333-%012d", i))
			providerPaymentID := fmt.Sprintf("reconcile_page_%d", i)
			createdAt := fixedNow.Add(-time.Hour - time.Duration(i)*time.Second)
			addBasicPayment(d, userID, subscriptionID, paymentID, providerPaymentID, createdAt)
			paymentIDs = append(paymentIDs, paymentID)
		}

		d.provider.statusRes = domain.PaymentStatusSucceeded

		count, err := d.service.Payments.ReconcilePendingPayments(context.Background(), fixedNow)
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

		count, err := d.service.Payments.ReconcilePendingPayments(context.Background(), fixedNow)
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

		count, err := d.service.Payments.ReconcilePendingPayments(context.Background(), fixedNow)
		if err == nil {
			t.Fatal("expected error from ListPendingPayments")
		}
		if count != 0 {
			t.Errorf("expected 0 payments synced on list error, got %d", count)
		}
	})
}

func TestBilling_ReconcileStaleRefunds(t *testing.T) {
	basicID := uuid.MustParse("44444444-4444-4444-4444-44444444444b")
	proID := uuid.MustParse("55555555-5555-5555-5555-55555555555a")
	staleAt := fixedNow.Add(-time.Hour)

	addTariffs := func(d *testDeps) {
		d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000})
		d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	}
	addRefundingPayment := func(d *testDeps, userID, subscriptionID, paymentID uuid.UUID, providerPaymentID string, updatedAt time.Time) {
		validUntil := fixedNow.AddDate(0, 1, 0)
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
			Status:            domain.PaymentStatusRefunding,
			CreatedAt:         updatedAt.Add(-time.Minute),
			UpdatedAt:         updatedAt,
		}
	}

	t.Run("provider refunded finalizes the refund and downgrades to basic", func(t *testing.T) {
		d := newTestDeps(t)
		addTariffs(d)
		userID := uuid.MustParse("66666666-6666-6666-6666-666666666601")
		subscriptionID := uuid.MustParse("66666666-6666-6666-6666-666666666602")
		paymentID := uuid.MustParse("66666666-6666-6666-6666-666666666603")
		addRefundingPayment(d, userID, subscriptionID, paymentID, "stale_refund_finalize", staleAt)

		d.provider.statusRes = domain.PaymentStatusRefunded

		count, err := d.service.Payments.ReconcileStaleRefunds(t.Context(), fixedNow)
		if err != nil {
			t.Fatalf("ReconcileStaleRefunds error: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected 1 payment resolved, got %d", count)
		}
		if !d.provider.statusCalled {
			t.Error("expected provider.Status to be called")
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
	})

	t.Run("provider succeeded reverts the refund reservation", func(t *testing.T) {
		d := newTestDeps(t)
		addTariffs(d)
		userID := uuid.MustParse("66666666-6666-6666-6666-666666666611")
		subscriptionID := uuid.MustParse("66666666-6666-6666-6666-666666666612")
		paymentID := uuid.MustParse("66666666-6666-6666-6666-666666666613")
		addRefundingPayment(d, userID, subscriptionID, paymentID, "stale_refund_revert", staleAt)

		d.provider.statusRes = domain.PaymentStatusSucceeded

		count, err := d.service.Payments.ReconcileStaleRefunds(t.Context(), fixedNow)
		if err != nil {
			t.Fatalf("ReconcileStaleRefunds error: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected 1 payment resolved, got %d", count)
		}

		payment := d.subscriptionPayments.payments[paymentID]
		if payment.Status != domain.PaymentStatusSucceeded {
			t.Errorf("expected payment back to succeeded, got %s", payment.Status)
		}
		if payment.RefundedAmountKopecks != nil {
			t.Errorf("expected refunded amount nil, got %v", *payment.RefundedAmountKopecks)
		}

		sub := d.subscriptions.subs[userID]
		if sub.TariffID != proID {
			t.Errorf("expected subscription to stay on pro, got tariff %s", sub.TariffID)
		}
		if len(d.propertyArchiver.calls) != 0 {
			t.Errorf("expected property archiver not called, got %v", d.propertyArchiver.calls)
		}
	})

	t.Run("provider pending keeps the payment refunding", func(t *testing.T) {
		d := newTestDeps(t)
		addTariffs(d)
		userID := uuid.MustParse("66666666-6666-6666-6666-666666666621")
		subscriptionID := uuid.MustParse("66666666-6666-6666-6666-666666666622")
		paymentID := uuid.MustParse("66666666-6666-6666-6666-666666666623")
		addRefundingPayment(d, userID, subscriptionID, paymentID, "stale_refund_pending", staleAt)

		d.provider.statusRes = domain.PaymentStatusPending

		count, err := d.service.Payments.ReconcileStaleRefunds(t.Context(), fixedNow)
		if err != nil {
			t.Fatalf("ReconcileStaleRefunds error: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected 1 payment visited, got %d", count)
		}

		payment := d.subscriptionPayments.payments[paymentID]
		if payment.Status != domain.PaymentStatusRefunding {
			t.Errorf("expected payment to stay refunding, got %s", payment.Status)
		}
		if payment.RefundedAmountKopecks != nil {
			t.Errorf("expected refunded amount nil, got %v", *payment.RefundedAmountKopecks)
		}
	})

	t.Run("provider partial refund is an anomaly and stays refunding", func(t *testing.T) {
		d := newTestDeps(t)
		addTariffs(d)
		userID := uuid.MustParse("66666666-6666-6666-6666-666666666631")
		subscriptionID := uuid.MustParse("66666666-6666-6666-6666-666666666632")
		paymentID := uuid.MustParse("66666666-6666-6666-6666-666666666633")
		addRefundingPayment(d, userID, subscriptionID, paymentID, "stale_refund_partial", staleAt)

		d.provider.statusRes = domain.PaymentStatusPartialRefunded

		count, err := d.service.Payments.ReconcileStaleRefunds(t.Context(), fixedNow)
		if err != nil {
			t.Fatalf("ReconcileStaleRefunds error: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected 1 payment visited, got %d", count)
		}

		payment := d.subscriptionPayments.payments[paymentID]
		if payment.Status != domain.PaymentStatusRefunding {
			t.Errorf("expected payment to stay refunding, got %s", payment.Status)
		}
		if len(d.propertyArchiver.calls) != 0 {
			t.Errorf("expected property archiver not called, got %v", d.propertyArchiver.calls)
		}
	})

	t.Run("provider failed keeps the payment refunding", func(t *testing.T) {
		d := newTestDeps(t)
		addTariffs(d)
		userID := uuid.MustParse("66666666-6666-6666-6666-666666666671")
		subscriptionID := uuid.MustParse("66666666-6666-6666-6666-666666666672")
		paymentID := uuid.MustParse("66666666-6666-6666-6666-666666666673")
		addRefundingPayment(d, userID, subscriptionID, paymentID, "stale_refund_failed", staleAt)

		d.provider.statusRes = domain.PaymentStatusFailed

		count, err := d.service.Payments.ReconcileStaleRefunds(t.Context(), fixedNow)
		if err != nil {
			t.Fatalf("ReconcileStaleRefunds error: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected 1 payment visited, got %d", count)
		}

		payment := d.subscriptionPayments.payments[paymentID]
		if payment.Status != domain.PaymentStatusRefunding {
			t.Errorf("expected payment to stay refunding, got %s", payment.Status)
		}
	})

	t.Run("bounded when no payment resolves", func(t *testing.T) {
		d := newTestDeps(t)
		addTariffs(d)

		// A full batch of stuck refunds whose provider never settles: each
		// batch re-reads the same payments, so the loop must stop at the cap
		// instead of spinning for the whole tick.
		for i := range renewalBatchSize {
			userID := uuid.MustParse(fmt.Sprintf("77777777-7777-7777-7777-%012d", i))
			subscriptionID := uuid.MustParse(fmt.Sprintf("88888888-8888-8888-8888-%012d", i))
			paymentID := uuid.MustParse(fmt.Sprintf("99999999-9999-9999-9999-%012d", i))
			addRefundingPayment(d, userID, subscriptionID, paymentID, fmt.Sprintf("stale_refund_stuck_%d", i), staleAt)
		}
		d.provider.statusRes = domain.PaymentStatusPending

		count, err := d.service.Payments.ReconcileStaleRefunds(t.Context(), fixedNow)
		if err != nil {
			t.Fatalf("ReconcileStaleRefunds error: %v", err)
		}
		if d.subscriptionPayments.listStaleRefundingCalls != maxReconcileBatchesPerTick {
			t.Errorf("ListStaleRefundingPayments called %d times, want cap %d",
				d.subscriptionPayments.listStaleRefundingCalls, maxReconcileBatchesPerTick)
		}
		if d.provider.statusCount != maxReconcileBatchesPerTick*renewalBatchSize {
			t.Errorf("provider.Status called %d times, want %d (cap * batch size)",
				d.provider.statusCount, maxReconcileBatchesPerTick*renewalBatchSize)
		}
		if count != maxReconcileBatchesPerTick*renewalBatchSize {
			t.Errorf("checked count = %d, want %d", count, maxReconcileBatchesPerTick*renewalBatchSize)
		}
	})

	t.Run("recent refunding payment is untouched", func(t *testing.T) {
		d := newTestDeps(t)
		addTariffs(d)
		userID := uuid.MustParse("66666666-6666-6666-6666-666666666641")
		subscriptionID := uuid.MustParse("66666666-6666-6666-6666-666666666642")
		paymentID := uuid.MustParse("66666666-6666-6666-6666-666666666643")
		addRefundingPayment(d, userID, subscriptionID, paymentID, "stale_refund_recent", fixedNow)

		count, err := d.service.Payments.ReconcileStaleRefunds(t.Context(), fixedNow)
		if err != nil {
			t.Fatalf("ReconcileStaleRefunds error: %v", err)
		}
		if count != 0 {
			t.Fatalf("expected 0 payments resolved, got %d", count)
		}
		if d.provider.statusCalled {
			t.Error("expected provider.Status not called for a non-stale payment")
		}

		payment := d.subscriptionPayments.payments[paymentID]
		if payment.Status != domain.PaymentStatusRefunding {
			t.Errorf("expected payment to stay refunding, got %s", payment.Status)
		}
	})

	t.Run("returns list error", func(t *testing.T) {
		d := newTestDeps(t)
		addTariffs(d)
		userID := uuid.MustParse("66666666-6666-6666-6666-666666666651")
		subscriptionID := uuid.MustParse("66666666-6666-6666-6666-666666666652")
		paymentID := uuid.MustParse("66666666-6666-6666-6666-666666666653")
		addRefundingPayment(d, userID, subscriptionID, paymentID, "stale_refund_list_err", staleAt)

		d.subscriptionPayments.listStaleRefundingErr = errors.New("list failed")

		count, err := d.service.Payments.ReconcileStaleRefunds(t.Context(), fixedNow)
		if err == nil {
			t.Fatal("expected error from ListStaleRefundingPayments")
		}
		if count != 0 {
			t.Errorf("expected 0 payments resolved on list error, got %d", count)
		}
	})
}

func scheduledChangeSubscription(userID, subID, tariffID, pendingTariffID uuid.UUID, methodID *uuid.UUID) domain.Subscription {
	changeAt := fixedNow.Add(-time.Hour)
	period := domain.PeriodMonth
	validUntil := fixedNow.AddDate(0, 1, 0)
	return domain.Subscription{
		ID:                    subID,
		UserID:                userID,
		TariffID:              tariffID,
		Source:                domain.SubscriptionSourcePaid,
		Status:                domain.SubscriptionStatusActive,
		ValidUntil:            &validUntil,
		AutoRenewEnabled:      true,
		ActivePaymentMethodID: methodID,
		PendingTariffID:       &pendingTariffID,
		PendingChangeAt:       &changeAt,
		PendingPeriod:         &period,
	}
}

func TestBilling_ProcessScheduledChanges_NoChargeApplies(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	subID := uuid.MustParse("55555555-5555-5555-5555-555555555555")

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addSubscription(scheduledChangeSubscription(userID, subID, proID, basicID, nil))

	count, err := d.service.ScheduledChanges.ProcessScheduledChanges(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("ProcessScheduledChanges error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 scheduled change applied, got %d", count)
	}

	sub := d.subscriptions.subs[userID]
	if sub.TariffID != basicID {
		t.Errorf("expected tariff changed to basic, got %s", sub.TariffID)
	}
	wantValidUntil := fixedNow.AddDate(0, 1, 0)
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(wantValidUntil) {
		t.Errorf("expected valid_until extended to %v, got %v", wantValidUntil, sub.ValidUntil)
	}
	if !sub.AutoRenewEnabled {
		t.Errorf("expected auto_renew_enabled after scheduled downgrade")
	}
	if sub.Status != domain.SubscriptionStatusActive {
		t.Errorf("expected status active, got %s", sub.Status)
	}
	if sub.PendingTariffID != nil || sub.PendingChangeAt != nil || sub.PendingPeriod != nil {
		t.Errorf("expected pending_* cleared, got %+v/%+v/%+v", sub.PendingTariffID, sub.PendingChangeAt, sub.PendingPeriod)
	}
	if d.provider.chargeCalled || d.provider.initCalled {
		t.Errorf("expected no provider call for scheduled downgrade")
	}
	if len(d.subscriptionPayments.payments) != 0 {
		t.Errorf("expected no payment for scheduled downgrade, got %d", len(d.subscriptionPayments.payments))
	}
	if len(d.propertyArchiver.calls) != 1 || d.propertyArchiver.calls[0].limit != 5 {
		t.Errorf("expected property archiver called with limit 5, got %v", d.propertyArchiver.calls)
	}
}

func TestBilling_ProcessScheduledChanges_AppliesOnceAndNotAgain(t *testing.T) {
	d := newTestDeps(t)
	userA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userB := uuid.MustParse("11111111-1111-1111-1111-111111111112")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	subA := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	subB := uuid.MustParse("55555555-5555-5555-5555-555555555556")

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})

	// (a) due row: pending_change_at <= now, must be applied.
	d.addSubscription(scheduledChangeSubscription(userA, subA, proID, basicID, nil))

	// (c) future row: pending_change_at > now, must be left untouched.
	futureChangeAt := fixedNow.Add(24 * time.Hour)
	futurePeriod := domain.PeriodMonth
	futureValidUntil := fixedNow.AddDate(0, 1, 0)
	d.addSubscription(domain.Subscription{
		ID:               subB,
		UserID:           userB,
		TariffID:         proID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &futureValidUntil,
		AutoRenewEnabled: true,
		PendingTariffID:  &basicID,
		PendingChangeAt:  &futureChangeAt,
		PendingPeriod:    &futurePeriod,
	})

	count, err := d.service.ScheduledChanges.ProcessScheduledChanges(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("ProcessScheduledChanges error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 scheduled change applied on first tick, got %d", count)
	}

	applied := d.subscriptions.subs[userA]
	if applied.TariffID != basicID {
		t.Errorf("expected tariff changed to basic, got %s", applied.TariffID)
	}
	wantValidUntil := fixedNow.AddDate(0, 1, 0)
	if applied.ValidUntil == nil || !applied.ValidUntil.Equal(wantValidUntil) {
		t.Errorf("expected valid_until extended to %v, got %v", wantValidUntil, applied.ValidUntil)
	}
	if !applied.AutoRenewEnabled {
		t.Errorf("expected auto_renew_enabled after scheduled downgrade")
	}
	if applied.Status != domain.SubscriptionStatusActive {
		t.Errorf("expected status active, got %s", applied.Status)
	}
	if applied.PendingTariffID != nil || applied.PendingChangeAt != nil || applied.PendingPeriod != nil {
		t.Errorf("expected pending_* cleared, got %+v/%+v/%+v", applied.PendingTariffID, applied.PendingChangeAt, applied.PendingPeriod)
	}
	if len(d.propertyArchiver.calls) != 1 || d.propertyArchiver.calls[0].limit != 5 {
		t.Errorf("expected property archiver called once with limit 5, got %v", d.propertyArchiver.calls)
	}

	untouched := d.subscriptions.subs[userB]
	if untouched.TariffID != proID {
		t.Errorf("expected future row tariff unchanged (pro), got %s", untouched.TariffID)
	}
	if untouched.PendingTariffID == nil || *untouched.PendingTariffID != basicID {
		t.Errorf("expected future row pending tariff basic, got %v", untouched.PendingTariffID)
	}
	if untouched.PendingChangeAt == nil || !untouched.PendingChangeAt.Equal(futureChangeAt) {
		t.Errorf("expected future row pending_change_at %v, got %v", futureChangeAt, untouched.PendingChangeAt)
	}

	// (b) a second worker tick must not re-apply the already-processed row.
	count, err = d.service.ScheduledChanges.ProcessScheduledChanges(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("second ProcessScheduledChanges error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 scheduled changes on second tick, got %d", count)
	}
	if len(d.propertyArchiver.calls) != 1 {
		t.Errorf("expected property archiver still called once (no re-apply), got %d", len(d.propertyArchiver.calls))
	}
}

func TestBilling_HandleWebhook_ProviderPaymentIDMismatch(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("77777777-7777-7777-7777-77777777777f")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-888888888890")
	subscriptionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa4")
	tariffID := uuid.MustParse("99999999-9999-9999-9999-99999999999e")
	expectedProviderPaymentID := "expected-provider-id"
	otherProviderPaymentID := "other-provider-id"

	d.addTariff(domain.Tariff{ID: tariffID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addSubscription(domain.Subscription{
		ID:       subscriptionID,
		UserID:   userID,
		TariffID: tariffID,
		Source:   domain.SubscriptionSourcePaid,
		Status:   domain.SubscriptionStatusActive,
	})
	d.subscriptionPayments.payments[paymentID] = domain.SubscriptionPayment{
		ID:                paymentID,
		UserID:            userID,
		SubscriptionID:    subscriptionID,
		TariffID:          tariffID,
		AmountKopecks:     5000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &expectedProviderPaymentID,
		Status:            domain.PaymentStatusPending,
	}

	d.provider.parseWebhook = func(_ []byte) (WebhookPayload, error) {
		return WebhookPayload{
			ProviderPaymentID: otherProviderPaymentID,
			InternalPaymentID: paymentID,
			Status:            domain.PaymentStatusSucceeded,
		}, nil
	}

	err := d.service.Webhooks.HandleWebhook(t.Context(), "fake", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error for provider payment id mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "provider payment id mismatch") {
		t.Errorf("expected mismatch error, got %v", err)
	}

	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusPending {
		t.Errorf("expected payment to stay pending, got %s", payment.Status)
	}

	sub := d.subscriptions.subs[userID]
	if sub.Status != domain.SubscriptionStatusActive {
		t.Errorf("expected subscription status active, got %s", sub.Status)
	}
	if sub.TariffID != tariffID {
		t.Errorf("expected subscription tariff unchanged, got %s", sub.TariffID)
	}
}

func TestBilling_ProcessScheduledChanges_PendingTariffNotFound_ErrTariffNotFound(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111113")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222224")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333335")
	subID := uuid.MustParse("55555555-5555-5555-5555-555555555557")
	missingTariffID := uuid.MustParse("66666666-6666-6666-6666-666666666668")

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addSubscription(scheduledChangeSubscription(userID, subID, basicID, missingTariffID, nil))

	// The top-level ProcessScheduledChange swallows per-item errors and logs
	// them, so we exercise the missing-pending-tariff branch directly.
	svc, ok := d.service.ScheduledChanges.(*ScheduledChangeService)
	if !ok {
		t.Fatalf("ScheduledChanges service is %T, expected *ScheduledChangeService", d.service.ScheduledChanges)
	}
	err := svc.applyScheduledChange(t.Context(), d.subscriptions.subs[userID], fixedNow)
	if !errors.Is(err, ErrTariffNotFound) {
		t.Fatalf("expected ErrTariffNotFound, got %v", err)
	}
}

func TestBilling_ChangeTariff_UpgradeCommitFailure_RollsBackAndMarksFailed(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111114")
	basicID := uuid.MustParse("22222222-2222-2222-2222-222222222225")
	proID := uuid.MustParse("33333333-3333-3333-3333-333333333336")
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000})
	d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
	d.addSubscription(domain.Subscription{
		ID:               uuid.MustParse("44444444-4444-4444-4444-444444444446"),
		UserID:           userID,
		TariffID:         basicID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	})

	d.provider.initFunc = func(_ InitRequest) {
		d.beginner.commitErr = errors.New("commit failed: boom")
	}

	_, err := d.service.Subscriptions.ChangeTariff(t.Context(), userID, ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	})
	if err == nil {
		t.Fatal("expected ChangeTariff error on commit failure, got nil")
	}

	var failedPayment *domain.SubscriptionPayment
	for _, p := range d.subscriptionPayments.payments {
		if p.UserID == userID && p.TariffID == proID && p.Status == domain.PaymentStatusFailed {
			failedPayment = &p
			break
		}
	}
	if failedPayment == nil {
		t.Fatal("expected a failed payment record for the upgrade")
	}

	sub := d.subscriptions.subs[userID]
	if sub.TariffID != basicID {
		t.Errorf("expected subscription tariff to remain basic, got %s", sub.TariffID)
	}
	if sub.PendingTariffID != nil || sub.PendingChangeAt != nil {
		t.Errorf("expected no pending change, got pending=%+v at=%+v", sub.PendingTariffID, sub.PendingChangeAt)
	}
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(validUntil) {
		t.Errorf("expected valid_until unchanged at %v, got %v", validUntil, sub.ValidUntil)
	}
}

// --- T-Kassa spec alignment: pending card binding polling, RebillId recovery,
// typed provider error classification ---

// stubProviderError is a test error carrying a provider error code, mirroring
// the tkassa adapter's ProviderError shape.
type stubProviderError struct{ code string }

func (e stubProviderError) Error() string             { return "stub provider error " + e.code }
func (e stubProviderError) ProviderErrorCode() string { return e.code }

func TestBilling_AddPaymentMethod_TkassaPersistsPendingBinding(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777790")
	d.provider.initAddCardRes = InitAddCardResult{
		PaymentURL:  "http://localhost/add-card/form",
		RequestKey:  "req_pending_1",
		CustomerKey: userID.String(),
	}

	resp, err := d.service.PaymentMethods.AddPaymentMethod(context.Background(), userID, AddPaymentMethodRequest{})
	if err != nil {
		t.Fatalf("AddPaymentMethod error: %v", err)
	}
	if resp.ConfirmURL != "http://localhost/add-card/form" {
		t.Errorf("expected confirm url from add card init, got %q", resp.ConfirmURL)
	}

	if len(d.paymentMethods.methods) != 1 {
		t.Fatalf("expected 1 stored pending binding, got %d", len(d.paymentMethods.methods))
	}
	for _, pm := range d.paymentMethods.methods {
		if pm.ProviderToken != domain.PendingCardBindingToken("req_pending_1") {
			t.Errorf("expected pending binding to store the prefixed request key, got %q", pm.ProviderToken)
		}
		if pm.IsActive {
			t.Error("expected pending binding to be inactive")
		}
		if pm.PendingCardBindingRequestKey() != "req_pending_1" {
			t.Error("expected stored row to be recognized as a pending card binding")
		}
	}

	// Pending bindings are internal sync state and must not be listed.
	methods, err := d.service.PaymentMethods.ListPaymentMethods(context.Background(), userID)
	if err != nil {
		t.Fatalf("ListPaymentMethods error: %v", err)
	}
	if len(methods) != 0 {
		t.Fatalf("expected pending binding to be hidden from the list, got %d methods", len(methods))
	}
}

func TestBilling_SyncPaymentMethods_CompletesPendingBindingViaGetAddCardState(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777791")
	tariffID := uuid.MustParse("99999999-9999-9999-9999-999999999991")
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{
		ID: tariffID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaa91"),
		UserID:           userID,
		TariffID:         tariffID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: true,
	})

	pendingID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbb91")
	d.paymentMethods.methods[pendingID] = domain.PaymentMethod{
		ID:            pendingID,
		UserID:        userID,
		Provider:      domain.ProviderTkassa,
		ProviderToken: domain.PendingCardBindingToken("req_poll_1"),
		CreatedAt:     fixedNow.Add(-time.Hour),
		UpdatedAt:     fixedNow.Add(-time.Hour),
	}

	d.provider.getAddCardStateRes = CardBindingState{
		Status:      CardBindingStatusCompleted,
		CardID:      "card_poll_1",
		RebillID:    "rebill_poll_1",
		CustomerKey: userID.String(),
	}
	d.provider.getCardListRes = []ProviderCard{
		{
			CardID:   "card_poll_1",
			Pan:      "430000******0777",
			ExpDate:  "12/30",
			RebillID: "rebill_poll_1",
			Status:   ProviderCardStatusActive,
		},
	}

	methods, err := d.service.PaymentMethods.SyncPaymentMethods(context.Background(), userID)
	if err != nil {
		t.Fatalf("SyncPaymentMethods error: %v", err)
	}
	if !d.provider.getAddCardStateCalled {
		t.Fatal("expected provider.GetAddCardState to be called")
	}
	if d.provider.getAddCardStateRequestKey != "req_poll_1" {
		t.Errorf("expected request key req_poll_1, got %q", d.provider.getAddCardStateRequestKey)
	}

	if len(methods) != 1 {
		t.Fatalf("expected 1 payment method, got %d", len(methods))
	}
	pm := methods[0]
	if pm.ProviderToken != "rebill_poll_1" {
		t.Errorf("expected provider token rebill_poll_1, got %q", pm.ProviderToken)
	}
	if pm.ProviderCardID != "card_poll_1" {
		t.Errorf("expected card id card_poll_1, got %q", pm.ProviderCardID)
	}
	if pm.DisplayMask != "430000******0777" {
		t.Errorf("expected display mask from GetCardList, got %q", pm.DisplayMask)
	}
	if !pm.IsActive {
		t.Error("expected completed binding to become the active method")
	}
	if _, ok := d.paymentMethods.methods[pendingID]; ok {
		t.Error("expected pending binding placeholder to be dropped")
	}

	sub := d.subscriptions.subs[userID]
	if sub.ActivePaymentMethodID == nil || *sub.ActivePaymentMethodID != pm.ID {
		t.Errorf("expected subscription active payment method %s, got %v", pm.ID, sub.ActivePaymentMethodID)
	}
}

func TestBilling_SyncPaymentMethods_CompletedBindingImportedFromStateWhenCardListLags(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777792")
	tariffID := uuid.MustParse("99999999-9999-9999-9999-999999999992")
	validUntil := fixedNow.AddDate(0, 1, 0)

	d.addTariff(domain.Tariff{
		ID: tariffID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000,
	})
	d.addSubscription(domain.Subscription{
		ID:               uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaa92"),
		UserID:           userID,
		TariffID:         tariffID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: true,
	})

	pendingID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbb92")
	d.paymentMethods.methods[pendingID] = domain.PaymentMethod{
		ID:            pendingID,
		UserID:        userID,
		Provider:      domain.ProviderTkassa,
		ProviderToken: domain.PendingCardBindingToken("req_poll_2"),
		CreatedAt:     fixedNow.Add(-time.Hour),
		UpdatedAt:     fixedNow.Add(-time.Hour),
	}

	d.provider.getAddCardStateRes = CardBindingState{
		Status:      CardBindingStatusCompleted,
		CardID:      "card_poll_2",
		RebillID:    "rebill_poll_2",
		CustomerKey: userID.String(),
	}
	// GetCardList does not report the card yet (eventual consistency).

	methods, err := d.service.PaymentMethods.SyncPaymentMethods(context.Background(), userID)
	if err != nil {
		t.Fatalf("SyncPaymentMethods error: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 payment method imported from the binding state, got %d", len(methods))
	}
	pm := methods[0]
	if pm.ProviderToken != "rebill_poll_2" || pm.ProviderCardID != "card_poll_2" {
		t.Errorf("expected method from binding state, got token=%q card=%q", pm.ProviderToken, pm.ProviderCardID)
	}
	if !pm.IsActive {
		t.Error("expected completed binding to become the active method")
	}
	if _, ok := d.paymentMethods.methods[pendingID]; ok {
		t.Error("expected pending binding placeholder to be dropped")
	}
}

func TestBilling_SyncPaymentMethods_RejectedBindingDropped(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777793")

	pendingID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbb93")
	d.paymentMethods.methods[pendingID] = domain.PaymentMethod{
		ID:            pendingID,
		UserID:        userID,
		Provider:      domain.ProviderTkassa,
		ProviderToken: domain.PendingCardBindingToken("req_rejected_1"),
		CreatedAt:     fixedNow.Add(-time.Hour),
		UpdatedAt:     fixedNow.Add(-time.Hour),
	}

	d.provider.getAddCardStateRes = CardBindingState{Status: CardBindingStatusRejected, ErrorCode: "7"}

	methods, err := d.service.PaymentMethods.SyncPaymentMethods(context.Background(), userID)
	if err != nil {
		t.Fatalf("SyncPaymentMethods error: %v", err)
	}
	if len(methods) != 0 {
		t.Fatalf("expected no payment methods, got %d", len(methods))
	}
	if _, ok := d.paymentMethods.methods[pendingID]; ok {
		t.Error("expected rejected binding placeholder to be dropped")
	}
}

func TestBilling_SyncPaymentMethods_UnknownRequestKeyDroppedAsExpired(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777794")

	pendingID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbb94")
	d.paymentMethods.methods[pendingID] = domain.PaymentMethod{
		ID:            pendingID,
		UserID:        userID,
		Provider:      domain.ProviderTkassa,
		ProviderToken: domain.PendingCardBindingToken("req_unknown_1"),
		CreatedAt:     fixedNow.Add(-time.Hour),
		UpdatedAt:     fixedNow.Add(-time.Hour),
	}

	// T-Kassa error 502: no card found for the RequestKey — the binding
	// session expired without completing.
	d.provider.getAddCardStateErr = stubProviderError{code: "502"}

	methods, err := d.service.PaymentMethods.SyncPaymentMethods(context.Background(), userID)
	if err != nil {
		t.Fatalf("SyncPaymentMethods error: %v", err)
	}
	if len(methods) != 0 {
		t.Fatalf("expected no payment methods, got %d", len(methods))
	}
	if _, ok := d.paymentMethods.methods[pendingID]; ok {
		t.Error("expected expired binding placeholder to be dropped")
	}
}

func TestBilling_SyncPaymentMethods_IntermediateBindingStateKeptPending(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777795")

	pendingID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbb95")
	d.paymentMethods.methods[pendingID] = domain.PaymentMethod{
		ID:            pendingID,
		UserID:        userID,
		Provider:      domain.ProviderTkassa,
		ProviderToken: domain.PendingCardBindingToken("req_inflight_1"),
		CreatedAt:     fixedNow.Add(-time.Hour),
		UpdatedAt:     fixedNow.Add(-time.Hour),
	}

	d.provider.getAddCardStateRes = CardBindingState{Status: CardBindingStatusAuthorizing}

	methods, err := d.service.PaymentMethods.SyncPaymentMethods(context.Background(), userID)
	if err != nil {
		t.Fatalf("SyncPaymentMethods error: %v", err)
	}
	if len(methods) != 0 {
		t.Fatalf("expected pending binding to stay hidden, got %d methods", len(methods))
	}
	if _, ok := d.paymentMethods.methods[pendingID]; !ok {
		t.Error("expected in-flight binding placeholder to be kept")
	}
}

// A payment-method row recovered from a provider status poll stores the raw
// RebillId and has no card data yet. It must NOT be treated as a pending
// card-binding placeholder: the sync must not poll GetAddCardState with the
// RebillId, must not drop the row, and the row stays visible in the list.
func TestBilling_SyncPaymentMethods_RecoveredRebillRowNotPolledAsPlaceholder(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777796")

	recoveredID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbb96")
	d.paymentMethods.methods[recoveredID] = domain.PaymentMethod{
		ID:            recoveredID,
		UserID:        userID,
		Provider:      domain.ProviderTkassa,
		ProviderToken: "rebill_recovered_only",
		IsActive:      true,
		CreatedAt:     fixedNow.Add(-time.Hour),
		UpdatedAt:     fixedNow.Add(-time.Hour),
	}

	// The provider reports no cards for the customer.
	methods, err := d.service.PaymentMethods.SyncPaymentMethods(context.Background(), userID)
	if err != nil {
		t.Fatalf("SyncPaymentMethods error: %v", err)
	}
	if d.provider.getAddCardStateCalled {
		t.Fatalf("expected no GetAddCardState poll for a raw RebillId row, got request key %q", d.provider.getAddCardStateRequestKey)
	}
	if len(methods) != 1 {
		t.Fatalf("expected the recovered row to stay visible, got %d methods", len(methods))
	}
	if methods[0].ID != recoveredID || methods[0].ProviderToken != "rebill_recovered_only" {
		t.Errorf("expected recovered row in the list, got %+v", methods[0])
	}
	if _, ok := d.paymentMethods.methods[recoveredID]; !ok {
		t.Error("expected the recovered row to be kept")
	}
}

// A pending card-binding placeholder is internal sync state: activating it
// must behave as if the row did not exist.
func TestBilling_SetActivePaymentMethod_PendingBindingPlaceholderNotFound(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777797")
	methodID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbb97")

	d.paymentMethods.methods[methodID] = domain.PaymentMethod{
		ID:            methodID,
		UserID:        userID,
		Provider:      domain.ProviderTkassa,
		ProviderToken: domain.PendingCardBindingToken("req_activate_1"),
		CreatedAt:     fixedNow.Add(-time.Hour),
		UpdatedAt:     fixedNow.Add(-time.Hour),
	}

	err := d.service.PaymentMethods.SetActivePaymentMethod(context.Background(), userID, methodID)
	if !errors.Is(err, ErrPaymentMethodNotFound) {
		t.Fatalf("expected ErrPaymentMethodNotFound, got %v", err)
	}
	if d.paymentMethods.methods[methodID].IsActive {
		t.Error("expected placeholder to stay inactive")
	}
}

// The AddCard bank form expires after 2 days: an older placeholder can never
// complete and is dropped without polling the provider, while a fresh one is
// still polled and kept pending.
func TestBilling_SyncPaymentMethods_ExpiredPlaceholderDroppedFreshKept(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777798")

	expiredID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbb98")
	d.paymentMethods.methods[expiredID] = domain.PaymentMethod{
		ID:            expiredID,
		UserID:        userID,
		Provider:      domain.ProviderTkassa,
		ProviderToken: domain.PendingCardBindingToken("req_expired_1"),
		CreatedAt:     fixedNow.Add(-49 * time.Hour),
		UpdatedAt:     fixedNow.Add(-49 * time.Hour),
	}
	freshID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbb99")
	d.paymentMethods.methods[freshID] = domain.PaymentMethod{
		ID:            freshID,
		UserID:        userID,
		Provider:      domain.ProviderTkassa,
		ProviderToken: domain.PendingCardBindingToken("req_fresh_1"),
		CreatedAt:     fixedNow.Add(-time.Hour),
		UpdatedAt:     fixedNow.Add(-time.Hour),
	}

	d.provider.getAddCardStateRes = CardBindingState{Status: CardBindingStatusAuthorizing}

	methods, err := d.service.PaymentMethods.SyncPaymentMethods(context.Background(), userID)
	if err != nil {
		t.Fatalf("SyncPaymentMethods error: %v", err)
	}
	if _, ok := d.paymentMethods.methods[expiredID]; ok {
		t.Error("expected expired placeholder to be dropped")
	}
	if _, ok := d.paymentMethods.methods[freshID]; !ok {
		t.Error("expected fresh placeholder to be kept")
	}
	if !d.provider.getAddCardStateCalled || d.provider.getAddCardStateRequestKey != "req_fresh_1" {
		t.Errorf("expected only the fresh placeholder to be polled with req_fresh_1, got called=%v key=%q",
			d.provider.getAddCardStateCalled, d.provider.getAddCardStateRequestKey)
	}
	if len(methods) != 0 {
		t.Fatalf("expected placeholders to stay hidden, got %d methods", len(methods))
	}
}

func TestBilling_SyncPendingPayment_RecoversRebillIDFromProviderStatus(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("11111111-1111-1111-1111-1111111111a1")
	basicID := uuid.MustParse("22222222-2222-2222-2222-2222222222a2")
	proID := uuid.MustParse("33333333-3333-3333-3333-3333333333a3")
	paymentID := uuid.MustParse("55555555-5555-5555-5555-5555555555a5")
	subscriptionID := uuid.MustParse("66666666-6666-6666-6666-6666666666a6")
	providerPaymentID := "sync_rebill_1"
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
		Provider:          domain.ProviderTkassa,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
		CreatedAt:         fixedNow.Add(-10 * time.Minute),
		UpdatedAt:         fixedNow.Add(-10 * time.Minute),
	}
	d.provider.statusRes = domain.PaymentStatusSucceeded
	d.provider.statusRebillID = "rebill_recovered_1"

	if err := d.service.Payments.SyncPendingPayment(context.Background(), uuid.Nil, paymentID); err != nil {
		t.Fatalf("SyncPendingPayment error: %v", err)
	}

	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusSucceeded {
		t.Fatalf("expected payment succeeded, got %s", payment.Status)
	}
	if payment.PaymentMethodID == nil {
		t.Fatal("expected payment to be linked to the recovered payment method")
	}
	pm, ok := d.paymentMethods.methods[*payment.PaymentMethodID]
	if !ok {
		t.Fatalf("expected recovered payment method %s to exist", payment.PaymentMethodID)
	}
	if pm.ProviderToken != "rebill_recovered_1" {
		t.Errorf("expected recovered token rebill_recovered_1, got %q", pm.ProviderToken)
	}
	if !pm.IsActive {
		t.Error("expected recovered payment method to be active")
	}

	sub := d.subscriptions.subs[userID]
	if sub.ActivePaymentMethodID == nil || *sub.ActivePaymentMethodID != pm.ID {
		t.Errorf("expected subscription active payment method %s, got %v", pm.ID, sub.ActivePaymentMethodID)
	}
}

func TestBilling_ProcessPendingUpgradePayments_RecoversRebillIDFromProviderStatus(t *testing.T) {
	d := newTestDeps(t)
	d.provider.name = domain.ProviderTkassa
	userID := uuid.MustParse("11111111-1111-1111-1111-1111111111b1")
	basicID := uuid.MustParse("22222222-2222-2222-2222-2222222222b2")
	proID := uuid.MustParse("33333333-3333-3333-3333-3333333333b3")
	paymentID := uuid.MustParse("55555555-5555-5555-5555-5555555555b5")
	subscriptionID := uuid.MustParse("66666666-6666-6666-6666-6666666666b6")
	providerPaymentID := "upgrade_rebill_1"
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
		Provider:          domain.ProviderTkassa,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
		CreatedAt:         fixedNow.Add(-10 * time.Minute),
		UpdatedAt:         fixedNow.Add(-10 * time.Minute),
	}
	d.subscriptionPayments.pendingUpgradePaymentIDs[paymentID] = true
	d.provider.statusRes = domain.PaymentStatusSucceeded
	d.provider.statusRebillID = "rebill_recovered_2"

	count, err := d.service.Renewals.ProcessPendingUpgradePayments(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("ProcessPendingUpgradePayments error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 upgrade processed, got %d", count)
	}

	payment := d.subscriptionPayments.payments[paymentID]
	if payment.Status != domain.PaymentStatusSucceeded {
		t.Fatalf("expected payment succeeded, got %s", payment.Status)
	}
	if payment.PaymentMethodID == nil {
		t.Fatal("expected payment to be linked to the recovered payment method")
	}
	pm, ok := d.paymentMethods.methods[*payment.PaymentMethodID]
	if !ok {
		t.Fatalf("expected recovered payment method %s to exist", payment.PaymentMethodID)
	}
	if pm.ProviderToken != "rebill_recovered_2" {
		t.Errorf("expected recovered token rebill_recovered_2, got %q", pm.ProviderToken)
	}
	if !pm.IsActive {
		t.Error("expected recovered payment method to be active")
	}

	sub := d.subscriptions.subs[userID]
	if sub.ActivePaymentMethodID == nil || *sub.ActivePaymentMethodID != pm.ID {
		t.Errorf("expected subscription active payment method %s, got %v", pm.ID, sub.ActivePaymentMethodID)
	}
}

func TestBilling_ProcessRenewals_TypedProviderErrorsKeepGraceAndErrorCode(t *testing.T) {
	cases := []struct {
		name    string
		cause   error
		wantErr string
	}{
		{
			name:    "charge blocked",
			cause:   fmt.Errorf("%w: %w", ErrProviderChargeBlocked, stubProviderError{code: "10"}),
			wantErr: "10",
		},
		{
			name:    "invalid operation",
			cause:   fmt.Errorf("%w: %w", ErrProviderInvalidOperation, stubProviderError{code: "1126"}),
			wantErr: "1126",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newTestDeps(t)
			d.provider.name = domain.ProviderTkassa
			userID := uuid.MustParse("11111111-1111-1111-1111-1111111111c1")
			basicID := uuid.MustParse("22222222-2222-2222-2222-2222222222c2")
			proID := uuid.MustParse("33333333-3333-3333-3333-3333333333c3")
			methodID := uuid.MustParse("44444444-4444-4444-4444-4444444444c4")
			validUntil := fixedNow

			d.addTariff(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 0})
			d.addTariff(domain.Tariff{ID: proID, Name: domain.TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000})
			d.addSubscription(domain.Subscription{
				ID:                    uuid.MustParse("55555555-5555-5555-5555-5555555555c5"),
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
				Provider:      domain.ProviderTkassa,
				ProviderToken: "rebill_valid_1",
				DisplayMask:   "****0777",
				IsActive:      true,
			}
			d.provider.chargeErr = tc.cause
			// The provider keeps reporting the charge as pending, so each tick
			// counts a charge attempt until the attempt cap fails the payment.
			d.provider.statusRes = domain.PaymentStatusPending

			for range maxRenewalChargeAttempts {
				if _, err := d.service.Renewals.ProcessRenewals(context.Background(), fixedNow); err != nil {
					t.Fatalf("ProcessRenewals error: %v", err)
				}
			}

			sub := d.subscriptions.subs[userID]
			if sub.Status != domain.SubscriptionStatusGrace {
				t.Errorf("expected status grace after attempt cap, got %s", sub.Status)
			}

			var failed *domain.SubscriptionPayment
			for _, p := range d.subscriptionPayments.payments {
				if p.UserID == userID && p.Status == domain.PaymentStatusFailed {
					payment := p
					failed = &payment
					break
				}
			}
			if failed == nil {
				t.Fatal("expected the renewal payment to be marked failed after the attempt cap")
			}
			if failed.ErrorCode == nil || *failed.ErrorCode != tc.wantErr {
				t.Errorf("expected persisted provider error code %q, got %v", tc.wantErr, failed.ErrorCode)
			}
		})
	}
}
