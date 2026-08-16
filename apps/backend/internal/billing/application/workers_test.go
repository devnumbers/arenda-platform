package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// The worker-phase scenarios ported from the pre-rewrite module's renewal and
// scheduled-change services (issue #252): the old application tests are the
// behavioural specification — double-charge protection, crash recovery,
// grace entry, attempt capping, batch guards, the shared expiry path and the
// lifecycle bridges.

// scriptedProvider is a programmable renewalProvider: every capability is a
// hook tests set, and every call is counted so tests can prove a charge never
// happened.
type scriptedProvider struct {
	name        domain.PaymentProvider
	mu          sync.Mutex
	initFn      func(req InitPaymentRequest) (InitPaymentResult, error)
	chargeFn    func(req ChargeRequest) (ChargeResult, error)
	statusFn    func(paymentID uuid.UUID, providerPaymentID string) (PaymentStatusResult, error)
	initCalls   int
	chargeCalls int
	statusCalls int
}

func (p *scriptedProvider) Name() domain.PaymentProvider { return p.name }

func (p *scriptedProvider) InitPayment(ctx context.Context, req InitPaymentRequest) (InitPaymentResult, error) {
	p.mu.Lock()
	p.initCalls++
	fn := p.initFn
	p.mu.Unlock()
	if fn == nil {
		return InitPaymentResult{ProviderPaymentID: "prov_" + req.PaymentID.String(), Status: domain.PaymentStatusPending}, nil
	}
	return fn(req)
}

func (p *scriptedProvider) ChargePayment(ctx context.Context, req ChargeRequest) (ChargeResult, error) {
	p.mu.Lock()
	p.chargeCalls++
	fn := p.chargeFn
	p.mu.Unlock()
	if fn == nil {
		return ChargeResult{ProviderPaymentID: req.ProviderPaymentID, Status: domain.PaymentStatusSucceeded}, nil
	}
	return fn(req)
}

func (p *scriptedProvider) PaymentStatus(ctx context.Context, paymentID uuid.UUID, providerPaymentID string) (PaymentStatusResult, error) {
	p.mu.Lock()
	p.statusCalls++
	fn := p.statusFn
	p.mu.Unlock()
	if fn == nil {
		return PaymentStatusResult{Status: domain.PaymentStatusPending}, nil
	}
	return fn(paymentID, providerPaymentID)
}

// ParseWebhook and WebhookAck satisfy the payment service's finalizer slice;
// the worker phases never deliver webhooks through them.
func (p *scriptedProvider) ParseWebhook(context.Context, []byte) (WebhookEvent, error) {
	return WebhookEvent{}, errors.New("scripted provider parses no webhooks")
}

func (p *scriptedProvider) WebhookAck() []byte { return []byte(`{"status":"ok"}`) }

// RefundPayment satisfies the payment-finalizer slice; the worker phases
// built on scriptedProvider never refund through it (the refund scenarios use
// stubPaymentProvider), so it answers a plain failure.
func (p *scriptedProvider) RefundPayment(_ context.Context, req RefundRequest) (RefundResult, error) {
	_ = req
	return RefundResult{}, errors.New("scripted provider refunds nothing")
}

func (p *scriptedProvider) calls() (inits, charges, statuses int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.initCalls, p.chargeCalls, p.statusCalls
}

// scriptedLifecycle is a programmable PaymentLifecycle: tests script the
// answers and read the calls back, proving the worker phases run through the
// narrow port (issue #288) — nothing else of the payment service is reachable
// through it.
type scriptedLifecycle struct {
	mu         sync.Mutex
	applyErr   error
	resolveOut bool
	resolveErr error
	applied    []PaymentNotification
	resolved   []refundResolution
}

// refundResolution is one stuck-refund resolution the port observed.
type refundResolution struct {
	payment domain.SubscriptionPayment
	status  PaymentStatusResult
}

func (l *scriptedLifecycle) ApplyPaymentNotification(_ context.Context, n *PaymentNotification) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.applied = append(l.applied, *n)
	return l.applyErr
}

func (l *scriptedLifecycle) ResolveRefundingFromStatus(_ context.Context, payment domain.SubscriptionPayment, status PaymentStatusResult) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.resolved = append(l.resolved, refundResolution{payment: payment, status: status})
	return l.resolveOut, l.resolveErr
}

func (l *scriptedLifecycle) applications() []PaymentNotification {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]PaymentNotification(nil), l.applied...)
}

func (l *scriptedLifecycle) resolutions() []refundResolution {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]refundResolution(nil), l.resolved...)
}

// fakeArchiverSource records archive calls for the lifecycle-bridge tests.
type fakeArchiverSource struct {
	mu    sync.Mutex
	calls []archiveCall
	err   error
}

type archiveCall struct {
	ownerID uuid.UUID
	limit   int
}

func (s *fakeArchiverSource) WithTx(transaction.Tx) (ExcessPropertyArchiver, error) {
	return fakeArchiver{src: s}, nil
}

type fakeArchiver struct{ src *fakeArchiverSource }

func (a fakeArchiver) ArchiveExcess(_ context.Context, ownerID uuid.UUID, limit int) error {
	a.src.mu.Lock()
	defer a.src.mu.Unlock()
	a.src.calls = append(a.src.calls, archiveCall{ownerID: ownerID, limit: limit})
	return a.src.err
}

func (s *fakeArchiverSource) recorded() []archiveCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]archiveCall(nil), s.calls...)
}

// fakeSlotSource records enforce calls for the lifecycle-bridge tests.
type fakeSlotSource struct {
	mu    sync.Mutex
	calls []string
	err   error
}

func (s *fakeSlotSource) WithTx(transaction.Tx) (RecipientSlotEnforcer, error) {
	return fakeSlots{src: s}, nil
}

type fakeSlots struct{ src *fakeSlotSource }

func (e fakeSlots) Enforce(_ context.Context, _ uuid.UUID, trigger string) error {
	e.src.mu.Lock()
	defer e.src.mu.Unlock()
	e.src.calls = append(e.src.calls, trigger)
	return e.src.err
}

func (s *fakeSlotSource) recorded() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.calls...)
}

// workersHarness wires Workers over the in-memory fakes and a scripted
// provider. The clock is fixed at workersNow; tariffs mirror the canonical
// seed (basic free, pro, business).
type workersHarness struct {
	stores   *fakeStores
	provider *scriptedProvider
	workers  *Workers
	cfg      Config
	now      time.Time

	basic, pro, business domain.Tariff
}

var workersNow = time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)

func newWorkersHarness(t *testing.T, cfg Config) *workersHarness {
	t.Helper()
	if cfg == (Config{}) {
		cfg = Config{
			GraceDuration:           7 * 24 * time.Hour,
			ChargeAttemptLimit:      3,
			WorkerBatchSize:         100,
			PendingPaymentStaleness: 5 * time.Minute,
		}
	}
	clk := fakeClock{now: workersNow}
	basic := domain.Tariff{ID: mustNewUUID(), Name: domain.TariffBasic, ActivePropertyLimit: 1, MonthlyPriceKopecks: 0, YearlyPriceKopecks: 0, IsActive: true}
	pro := domain.Tariff{ID: mustNewUUID(), Name: domain.TariffPro, ActivePropertyLimit: 5, MonthlyPriceKopecks: 49000, YearlyPriceKopecks: 440000, IsActive: true}
	business := domain.Tariff{ID: mustNewUUID(), Name: domain.TariffBusiness, ActivePropertyLimit: -1, MonthlyPriceKopecks: 99000, YearlyPriceKopecks: 890000, IsActive: true}
	stores := newFakeStores(basic, pro, business)
	provider := &scriptedProvider{name: "fake"}
	factory := stores.factory(nil)
	logger := slog.New(slog.DiscardHandler)
	workers := NewWorkers(factory, WorkersConfig{
		Provider: provider,
		Payments: NewPaymentService(factory, provider, PaymentServiceConfig{Clock: clk, Log: logger}),
		Clock:    clk,
		Config:   cfg,
		Logger:   logger,
	})
	return &workersHarness{stores: stores, provider: provider, workers: workers, cfg: cfg, now: workersNow, basic: basic, pro: pro, business: business}
}

func mustNewUUID() uuid.UUID {
	id, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	return id
}

// seedSubscription stores a subscription built by the caller's mutation of a
// paid-pro baseline.
func (h *workersHarness) seedSubscription(t *testing.T, mutate func(*domain.Subscription)) domain.Subscription {
	t.Helper()
	userID := mustNewUUID()
	sub, err := domain.NewBasicSubscription(userID, h.basic.ID)
	if err != nil {
		t.Fatalf("new subscription: %v", err)
	}
	month := domain.PeriodMonth
	sub.TariffID = h.pro.ID
	validUntil := h.now.AddDate(0, -1, 0) // expired a month ago
	sub.ValidUntil = &validUntil
	sub.AutoRenewEnabled = true
	sub.CurrentPeriod = &month
	if mutate != nil {
		mutate(&sub)
	}
	stored, err := h.stores.subscriptions.Create(t.Context(), sub)
	if err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
	return stored
}

// seedActiveMethod stores an active payment method and links it as the
// subscription's charge target.
func (h *workersHarness) seedActiveMethod(t *testing.T, sub domain.Subscription, providerName domain.PaymentProvider, token string) domain.PaymentMethod {
	t.Helper()
	method, err := domain.NewPaymentMethod(sub.UserID, providerName, token, h.now)
	if err != nil {
		t.Fatalf("new method: %v", err)
	}
	method.IsActive = true
	stored, err := h.stores.methods.UpsertByTokenHash(t.Context(), method)
	if err != nil {
		t.Fatalf("seed method: %v", err)
	}
	sub.ActivePaymentMethodID = &stored.ID
	if err := h.stores.subscriptions.Update(t.Context(), sub); err != nil {
		t.Fatalf("link method: %v", err)
	}
	return stored
}

// transitionsOf returns the subscription's transitions, newest first.
func (h *workersHarness) transitionsOf(t *testing.T, sub domain.Subscription) []domain.Transition {
	t.Helper()
	list, err := h.stores.transitions.ListBySubscriptionID(t.Context(), sub.ID)
	if err != nil {
		t.Fatalf("list transitions: %v", err)
	}
	return list
}

// singlePaymentOf returns the user's only payment row.
func (h *workersHarness) singlePaymentOf(t *testing.T, userID uuid.UUID) *domain.SubscriptionPayment {
	t.Helper()
	all, err := h.stores.payments.ListByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("list payments: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("payments = %d, want 1", len(all))
	}
	return &all[0]
}

// storedSubscription re-reads the subscription.
func (h *workersHarness) storedSubscription(t *testing.T, sub domain.Subscription) domain.Subscription {
	t.Helper()
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("get subscription: %v", err)
	}
	return stored
}

// TestWorkers_RenewalChargesActiveMethodAndRenews proves the happy path: the
// expired auto-renewing subscription is charged on its active method, the
// payment succeeds, and the subscription renews from now with the transition
// logged (ported from TestBilling_ProcessRenewals_Success).
func TestWorkers_RenewalChargesActiveMethodAndRenews(t *testing.T) {
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, nil)
	method := h.seedActiveMethod(t, sub, "fake", "token_good")

	count, err := h.workers.ProcessRenewals(t.Context(), h.now)
	if err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("ProcessRenewals() = %d, want 1", count)
	}

	payment := h.singlePaymentOf(t, sub.UserID)
	if payment.Status != domain.PaymentStatusSucceeded {
		t.Errorf("payment status = %q, want succeeded", payment.Status)
	}
	if payment.TariffID != h.pro.ID || payment.AmountKopecks != h.pro.MonthlyPriceKopecks {
		t.Errorf("payment = tariff %s amount %d, want pro %d", payment.TariffID, payment.AmountKopecks, h.pro.MonthlyPriceKopecks)
	}
	if payment.PaymentMethodID == nil || *payment.PaymentMethodID != method.ID {
		t.Errorf("payment method = %v, want the active method", payment.PaymentMethodID)
	}
	if payment.Provider != "fake" {
		t.Errorf("payment provider = %q, want fake", payment.Provider)
	}

	stored := h.storedSubscription(t, sub)
	wantUntil := h.now.AddDate(0, 1, 0)
	if stored.Status != domain.SubscriptionStatusActive || stored.ValidUntil == nil || !stored.ValidUntil.Equal(wantUntil) {
		t.Errorf("subscription = %s until %v, want active until %v", stored.Status, stored.ValidUntil, wantUntil)
	}
	if stored.LastAppliedPaymentID == nil || *stored.LastAppliedPaymentID != payment.ID {
		t.Errorf("LastAppliedPaymentID = %v, want the renewal payment", stored.LastAppliedPaymentID)
	}

	transitions := h.transitionsOf(t, sub)
	if len(transitions) != 1 || transitions[0].Reason != domain.TransitionReasonPaymentApplied {
		t.Fatalf("transitions = %+v, want one payment_applied", transitions)
	}
	if transitions[0].PaymentID == nil || *transitions[0].PaymentID != payment.ID {
		t.Error("transition does not reference the renewal payment")
	}

	inits, charges, _ := h.provider.calls()
	if inits != 1 || charges != 1 {
		t.Errorf("provider calls = init %d charge %d, want 1/1 (MIT init then charge)", inits, charges)
	}
}

// TestWorkers_RenewalWithoutChargeableMethodEntersGrace covers the no-method
// and foreign-provider-method cases: both mean "nothing to charge" and move
// the subscription into grace with a scheduled change dropped (ADR 0008; the
// foreign-provider case is the provider-switch semantics of ADR 0038, ported
// from the old foreign-card scenario).
func TestWorkers_RenewalWithoutChargeableMethodEntersGrace(t *testing.T) {
	for _, tc := range []struct {
		name       string
		methodProv domain.PaymentProvider
		seedMethod bool
	}{
		{name: "no method", seedMethod: false},
		{name: "foreign provider method", seedMethod: true, methodProv: "tkassa"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newWorkersHarness(t, Config{})
			sub := h.seedSubscription(t, func(s *domain.Subscription) {
				// A paid deferred downgrade is still pending: grace entry must
				// drop it (ADR 0008 — the failed charge consumed it).
				s.TariffID = h.business.ID
				target := h.pro.ID
				changeAt := h.now.Add(-time.Hour)
				s.PendingTariffID = &target
				s.PendingChangeAt = &changeAt
				period := domain.PeriodMonth
				s.PendingPeriod = &period
			})
			if tc.seedMethod {
				h.seedActiveMethod(t, sub, tc.methodProv, "token_foreign")
			}

			count, err := h.workers.ProcessRenewals(t.Context(), h.now)
			if err != nil {
				t.Fatalf("ProcessRenewals() error = %v", err)
			}
			if count != 1 {
				t.Fatalf("ProcessRenewals() = %d, want 1", count)
			}

			stored := h.storedSubscription(t, sub)
			if stored.Status != domain.SubscriptionStatusGrace {
				t.Fatalf("status = %q, want grace", stored.Status)
			}
			wantUntil := h.now.Add(7 * 24 * time.Hour)
			if stored.ValidUntil == nil || !stored.ValidUntil.Equal(wantUntil) {
				t.Errorf("ValidUntil = %v, want grace window end %v", stored.ValidUntil, wantUntil)
			}
			if stored.HasPendingChange() {
				t.Error("scheduled change survived grace entry, want dropped")
			}

			transitions := h.transitionsOf(t, sub)
			if len(transitions) != 1 || transitions[0].Reason != domain.TransitionReasonGraceEntered {
				t.Fatalf("transitions = %+v, want one grace_entered", transitions)
			}
			if transitions[0].Initiator != domain.InitiatorSystem {
				t.Errorf("initiator = %q, want system", transitions[0].Initiator)
			}

			if _, _, charges := h.provider.calls(); charges != 0 {
				t.Errorf("charge calls = %d, want 0", charges)
			}
			pending, _ := h.stores.payments.ListPendingByUserID(t.Context(), sub.UserID)
			if len(pending) != 0 {
				t.Errorf("pending payments = %d, want 0 (nothing was initiated)", len(pending))
			}
		})
	}
}

// TestWorkers_RenewalFailedChargeMovesToGrace proves a declined charge fails
// the payment with the provider's error code and enters grace (ported from
// TestBilling_ProcessRenewals_FailedChargeMovesToGrace and
// ..._ProviderErrorCodeSavedOnFailedCharge).
func TestWorkers_RenewalFailedChargeMovesToGrace(t *testing.T) {
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, nil)
	h.seedActiveMethod(t, sub, "fake", "token_bad")
	h.provider.chargeFn = func(req ChargeRequest) (ChargeResult, error) {
		return ChargeResult{ProviderPaymentID: req.ProviderPaymentID, Status: domain.PaymentStatusFailed, ErrorCode: "53"}, nil
	}

	if _, err := h.workers.ProcessRenewals(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}

	pending := h.singlePaymentOf(t, sub.UserID)
	if pending.Status != domain.PaymentStatusFailed {
		t.Fatalf("payment status = %q, want failed", pending.Status)
	}
	if pending.ErrorCode == nil || *pending.ErrorCode != "53" {
		t.Errorf("ErrorCode = %v, want 53", pending.ErrorCode)
	}
	stored := h.storedSubscription(t, sub)
	if stored.Status != domain.SubscriptionStatusGrace {
		t.Errorf("status = %q, want grace", stored.Status)
	}
	transitions := h.transitionsOf(t, sub)
	if len(transitions) != 1 || transitions[0].Reason != domain.TransitionReasonGraceEntered {
		t.Errorf("transitions = %+v, want one grace_entered", transitions)
	}
}

// TestWorkers_RenewalSkipsRechargeWhenProviderSucceeded proves the
// double-charge guard: a pending payment that already references a provider
// payment the provider reports as succeeded is applied without a new charge
// (ported from TestBilling_ProcessRenewals_SkipsDuplicateChargeWhenProviderSucceeded).
func TestWorkers_RenewalSkipsRechargeWhenProviderSucceeded(t *testing.T) {
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, nil)
	method := h.seedActiveMethod(t, sub, "fake", "token_good")

	// A previous run crashed after the provider captured the charge but before
	// the result was applied: the pending payment already has its reference.
	payment, err := domain.NewSubscriptionPayment(sub.UserID, sub.ID, h.pro.ID, domain.PeriodMonth, h.pro.MonthlyPriceKopecks, "fake", h.now.Add(-time.Minute))
	if err != nil {
		t.Fatalf("new payment: %v", err)
	}
	methodID := method.ID
	payment.PaymentMethodID = &methodID
	if err := payment.SaveProviderReference("prov_existing", "", h.now.Add(-time.Minute)); err != nil {
		t.Fatalf("save reference: %v", err)
	}
	if _, err := h.stores.payments.Create(t.Context(), payment); err != nil {
		t.Fatalf("seed payment: %v", err)
	}
	h.provider.statusFn = func(uuid.UUID, string) (PaymentStatusResult, error) {
		return PaymentStatusResult{Status: domain.PaymentStatusSucceeded}, nil
	}
	h.provider.chargeFn = func(ChargeRequest) (ChargeResult, error) {
		t.Error("ChargePayment must not be called when the provider already succeeded")
		return ChargeResult{}, nil
	}

	if _, err := h.workers.ProcessRenewals(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}

	stored := h.storedSubscription(t, sub)
	wantUntil := h.now.AddDate(0, 1, 0)
	if stored.ValidUntil == nil || !stored.ValidUntil.Equal(wantUntil) {
		t.Errorf("ValidUntil = %v, want renewed to %v", stored.ValidUntil, wantUntil)
	}
	pending := h.singlePaymentOf(t, sub.UserID)
	if pending.Status != domain.PaymentStatusSucceeded {
		t.Errorf("payment status = %q, want succeeded", pending.Status)
	}
	if inits, charges, _ := h.provider.calls(); inits != 0 || charges != 0 {
		t.Errorf("provider calls = init %d charge %d, want 0/0 (no new initiation, no new charge)", inits, charges)
	}
}

// TestWorkers_RenewalFinalizesProviderFailedWithoutCharge proves a payment
// the provider already reports failed is failed locally with grace entry and
// no charge attempt (ported from
// TestBilling_ProcessRenewals_FinalizesFailedPaymentWithoutCharge).
func TestWorkers_RenewalFinalizesProviderFailedWithoutCharge(t *testing.T) {
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, nil)
	h.seedActiveMethod(t, sub, "fake", "token_good")
	payment, err := domain.NewSubscriptionPayment(sub.UserID, sub.ID, h.pro.ID, domain.PeriodMonth, h.pro.MonthlyPriceKopecks, "fake", h.now.Add(-time.Minute))
	if err != nil {
		t.Fatalf("new payment: %v", err)
	}
	if err := payment.SaveProviderReference("prov_existing", "", h.now.Add(-time.Minute)); err != nil {
		t.Fatalf("save reference: %v", err)
	}
	if _, err := h.stores.payments.Create(t.Context(), payment); err != nil {
		t.Fatalf("seed payment: %v", err)
	}
	h.provider.statusFn = func(uuid.UUID, string) (PaymentStatusResult, error) {
		return PaymentStatusResult{Status: domain.PaymentStatusFailed, ErrorCode: "102"}, nil
	}
	h.provider.chargeFn = func(ChargeRequest) (ChargeResult, error) {
		t.Error("ChargePayment must not be called when the provider already failed")
		return ChargeResult{}, nil
	}

	if _, err := h.workers.ProcessRenewals(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}

	pending := h.singlePaymentOf(t, sub.UserID)
	if pending.Status != domain.PaymentStatusFailed || pending.ErrorCode == nil || *pending.ErrorCode != "102" {
		t.Errorf("payment = %s code %v, want failed/102", pending.Status, pending.ErrorCode)
	}
	if h.storedSubscription(t, sub).Status != domain.SubscriptionStatusGrace {
		t.Error("subscription not in grace after provider-failed renewal")
	}
	if _, charges, _ := h.provider.calls(); charges != 0 {
		t.Errorf("charge calls = %d, want 0", charges)
	}
}

// TestWorkers_RenewalProviderStatusUnknownSkipsCharge proves an unknown
// provider status neither charges nor counts an attempt: the previous outcome
// may be a success (ported from ..._SkipsRechargeWhenProviderStatusUnknown and
// ..._StatusErrorDoesNotCountChargeAttempt).
func TestWorkers_RenewalProviderStatusUnknownSkipsCharge(t *testing.T) {
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, nil)
	h.seedActiveMethod(t, sub, "fake", "token_good")
	payment, err := domain.NewSubscriptionPayment(sub.UserID, sub.ID, h.pro.ID, domain.PeriodMonth, h.pro.MonthlyPriceKopecks, "fake", h.now.Add(-time.Minute))
	if err != nil {
		t.Fatalf("new payment: %v", err)
	}
	if err := payment.SaveProviderReference("prov_existing", "", h.now.Add(-time.Minute)); err != nil {
		t.Fatalf("save reference: %v", err)
	}
	if _, err := h.stores.payments.Create(t.Context(), payment); err != nil {
		t.Fatalf("seed payment: %v", err)
	}
	h.provider.statusFn = func(uuid.UUID, string) (PaymentStatusResult, error) {
		return PaymentStatusResult{}, errors.New("provider unreachable")
	}
	h.provider.chargeFn = func(ChargeRequest) (ChargeResult, error) {
		t.Error("ChargePayment must not be called over an unknown status")
		return ChargeResult{}, nil
	}

	if _, err := h.workers.ProcessRenewals(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}

	pending := h.singlePaymentOf(t, sub.UserID)
	if pending.Status != domain.PaymentStatusPending {
		t.Errorf("payment status = %q, want pending", pending.Status)
	}
	if pending.ChargeAttempts != 0 {
		t.Errorf("ChargeAttempts = %d, want 0 (unknown status is not an attempt)", pending.ChargeAttempts)
	}
	if h.storedSubscription(t, sub).Status != domain.SubscriptionStatusActive {
		t.Error("subscription moved out of active over an unknown status")
	}
}

// TestWorkers_RenewalUncertainChargeResolvesFromProviderStatus proves the
// crash-recovery path: a charge whose call errored is resolved against the
// provider's status — a reported success is applied without a re-charge, an
// unknown status keeps everything pending (ported from
// TestBilling_ProcessRenewals_RecoverAfterChargeTimeoutProviderSucceeds and
// ..._ProviderUnknownKeepsActive).
func TestWorkers_RenewalUncertainChargeResolvesFromProviderStatus(t *testing.T) {
	t.Run("provider succeeded", func(t *testing.T) {
		h := newWorkersHarness(t, Config{})
		sub := h.seedSubscription(t, nil)
		h.seedActiveMethod(t, sub, "fake", "token_good")
		// The pre-charge status check sees the charge in flight; the charge
		// call itself times out; the recovery query then reports the capture.
		var statusCalls int
		h.provider.statusFn = func(uuid.UUID, string) (PaymentStatusResult, error) {
			statusCalls++
			if statusCalls == 1 {
				return PaymentStatusResult{Status: domain.PaymentStatusPending}, nil
			}
			return PaymentStatusResult{Status: domain.PaymentStatusSucceeded}, nil
		}
		h.provider.chargeFn = func(ChargeRequest) (ChargeResult, error) {
			return ChargeResult{}, errors.New("charge timeout")
		}

		if _, err := h.workers.ProcessRenewals(t.Context(), h.now); err != nil {
			t.Fatalf("ProcessRenewals() error = %v", err)
		}
		pending := h.singlePaymentOf(t, sub.UserID)
		if pending.Status != domain.PaymentStatusSucceeded {
			t.Errorf("payment status = %q, want succeeded", pending.Status)
		}
		wantUntil := h.now.AddDate(0, 1, 0)
		if stored := h.storedSubscription(t, sub); stored.ValidUntil == nil || !stored.ValidUntil.Equal(wantUntil) {
			t.Errorf("ValidUntil = %v, want %v", stored.ValidUntil, wantUntil)
		}
		if _, charges, _ := h.provider.calls(); charges != 1 {
			t.Errorf("charge calls = %d, want exactly 1 (no re-charge)", charges)
		}
	})

	t.Run("provider unknown", func(t *testing.T) {
		h := newWorkersHarness(t, Config{})
		sub := h.seedSubscription(t, nil)
		h.seedActiveMethod(t, sub, "fake", "token_good")
		// Pre-charge check pending, the charge times out, the recovery query
		// cannot resolve the outcome either.
		var statusCalls int
		h.provider.statusFn = func(uuid.UUID, string) (PaymentStatusResult, error) {
			statusCalls++
			if statusCalls == 1 {
				return PaymentStatusResult{Status: domain.PaymentStatusPending}, nil
			}
			return PaymentStatusResult{}, errors.New("status unreachable")
		}
		h.provider.chargeFn = func(ChargeRequest) (ChargeResult, error) {
			return ChargeResult{}, errors.New("charge timeout")
		}

		if _, err := h.workers.ProcessRenewals(t.Context(), h.now); err != nil {
			t.Fatalf("ProcessRenewals() error = %v", err)
		}
		pending := h.singlePaymentOf(t, sub.UserID)
		if pending.Status != domain.PaymentStatusPending || pending.ChargeAttempts != 0 {
			t.Errorf("payment = %s attempts %d, want pending/0", pending.Status, pending.ChargeAttempts)
		}
		if h.storedSubscription(t, sub).Status != domain.SubscriptionStatusActive {
			t.Error("subscription left active over an unknown charge outcome")
		}
	})
}

// TestWorkers_RenewalChargeAttemptLimitMovesToGrace proves the unresolved
// attempt cap: a charge stuck pending at the provider fails after the
// configured number of attempts and the subscription enters grace (ported
// from TestBilling_ProcessRenewals_ChargeAttemptLimitMovesToGrace).
func TestWorkers_RenewalChargeAttemptLimitMovesToGrace(t *testing.T) {
	h := newWorkersHarness(t, Config{ChargeAttemptLimit: 2, WorkerBatchSize: 100, GraceDuration: 7 * 24 * time.Hour, PendingPaymentStaleness: 5 * time.Minute})
	sub := h.seedSubscription(t, nil)
	h.seedActiveMethod(t, sub, "fake", "token_good")
	h.provider.chargeFn = func(ChargeRequest) (ChargeResult, error) {
		return ChargeResult{}, errors.New("charge timeout")
	}
	h.provider.statusFn = func(uuid.UUID, string) (PaymentStatusResult, error) {
		return PaymentStatusResult{Status: domain.PaymentStatusPending}, nil
	}

	// First tick: one unresolved charge counts one attempt, nothing final.
	if _, err := h.workers.ProcessRenewals(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessRenewals() tick 1 error = %v", err)
	}
	pending := h.singlePaymentOf(t, sub.UserID)
	if pending.ChargeAttempts != 1 || pending.Status != domain.PaymentStatusPending {
		t.Fatalf("after tick 1: attempts %d status %s, want 1/pending", pending.ChargeAttempts, pending.Status)
	}

	// Second tick hits the limit on the same pending payment and fails it.
	if _, err := h.workers.ProcessRenewals(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessRenewals() tick 2 error = %v", err)
	}
	all, err := h.stores.payments.ListByUserID(t.Context(), sub.UserID)
	if err != nil || len(all) != 1 {
		t.Fatalf("payments = %d (err %v), want the single renewal payment", len(all), err)
	}
	if all[0].Status != domain.PaymentStatusFailed {
		t.Errorf("payment status = %q, want failed at the attempt limit", all[0].Status)
	}
	if all[0].ChargeAttempts != 2 {
		t.Errorf("ChargeAttempts = %d, want 2", all[0].ChargeAttempts)
	}
	if h.storedSubscription(t, sub).Status != domain.SubscriptionStatusGrace {
		t.Error("subscription not in grace after the attempt limit")
	}
}

// TestWorkers_RenewalRecoversPendingPaymentFromCreateRace proves a pending
// renewal payment from a crashed run is reused — the unique-race backstop
// resolves to the existing row instead of duplicating (ported from
// TestBilling_ProcessRenewals_RecoversExistingPendingPaymentOnCreateRace).
func TestWorkers_RenewalRecoversPendingPaymentFromCreateRace(t *testing.T) {
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, nil)
	method := h.seedActiveMethod(t, sub, "fake", "token_good")
	existing, err := domain.NewSubscriptionPayment(sub.UserID, sub.ID, h.pro.ID, domain.PeriodMonth, h.pro.MonthlyPriceKopecks, "fake", h.now.Add(-time.Minute))
	if err != nil {
		t.Fatalf("new payment: %v", err)
	}
	if _, err := h.stores.payments.Create(t.Context(), existing); err != nil {
		t.Fatalf("seed payment: %v", err)
	}
	// The first pending lookup misses (hidePending), so Create hits the
	// unique-race and the second lookup must recover the existing payment.
	h.stores.payments.hidePending = 1

	if _, err := h.workers.ProcessRenewals(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}

	all, err := h.stores.payments.ListByUserID(t.Context(), sub.UserID)
	if err != nil || len(all) != 1 {
		t.Fatalf("payments = %d (err %v), want the single reused payment", len(all), err)
	}
	if all[0].ID != existing.ID {
		t.Errorf("payment id = %s, want the existing %s", all[0].ID, existing.ID)
	}
	if all[0].PaymentMethodID == nil || *all[0].PaymentMethodID != method.ID {
		t.Errorf("payment method = %v, want the current active method", all[0].PaymentMethodID)
	}
	if all[0].Status != domain.PaymentStatusSucceeded {
		t.Errorf("payment status = %q, want succeeded", all[0].Status)
	}
}

// TestWorkers_RenewalFreeBasicTermsFallToBasic proves the free-terms renewal:
// a basic subscription left with a validity window and auto-renew on (the
// state a scheduled downgrade to basic produces) falls to the permanent basic
// state — validity cleared, auto-renew off — through the shared expiry
// outcome (ported from the old free-renewal path).
func TestWorkers_RenewalFreeBasicTermsFallToBasic(t *testing.T) {
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, func(s *domain.Subscription) {
		s.TariffID = h.basic.ID
	})

	if _, err := h.workers.ProcessRenewals(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}

	stored := h.storedSubscription(t, sub)
	if stored.TariffID != h.basic.ID || stored.ValidUntil != nil || stored.AutoRenewEnabled {
		t.Errorf("subscription = tariff %s until %v autorenew %t, want basic/no validity/off", stored.TariffID, stored.ValidUntil, stored.AutoRenewEnabled)
	}
	if stored.Status != domain.SubscriptionStatusActive {
		t.Errorf("status = %q, want active", stored.Status)
	}
	transitions := h.transitionsOf(t, sub)
	if len(transitions) != 1 || transitions[0].Reason != domain.TransitionReasonExpired {
		t.Fatalf("transitions = %+v, want one expired", transitions)
	}
	if _, _, charges := h.provider.calls(); charges != 0 {
		t.Errorf("charge calls = %d, want 0 (free terms)", charges)
	}
}

// TestWorkers_ScheduledChangesApplyFreeTarget proves the free deferred change
// applies at the end of the paid period without a payment: the tariff
// switches, validity extends from now, the change clears and the lifecycle
// bridges run in the same transaction (ported from
// TestBilling_ProcessScheduledChanges_NoChargeApplies and
// ..._AppliesOnceAndNotAgain).
func TestWorkers_ScheduledChangesApplyFreeTarget(t *testing.T) {
	h := newWorkersHarness(t, Config{})
	archiver := &fakeArchiverSource{}
	slots := &fakeSlotSource{}
	h.workers.SetLifecycleBridges(archiver, slots)

	sub := h.seedSubscription(t, func(s *domain.Subscription) {
		target := h.basic.ID
		changeAt := h.now.Add(-time.Hour)
		period := domain.PeriodMonth
		s.PendingTariffID = &target
		s.PendingChangeAt = &changeAt
		s.PendingPeriod = &period
	})

	count, err := h.workers.ProcessScheduledChanges(t.Context(), h.now)
	if err != nil {
		t.Fatalf("ProcessScheduledChanges() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("ProcessScheduledChanges() = %d, want 1", count)
	}

	stored := h.storedSubscription(t, sub)
	if stored.TariffID != h.basic.ID {
		t.Errorf("TariffID = %s, want basic", stored.TariffID)
	}
	wantUntil := h.now.AddDate(0, 1, 0)
	if stored.ValidUntil == nil || !stored.ValidUntil.Equal(wantUntil) {
		t.Errorf("ValidUntil = %v, want %v (a free month of the target period)", stored.ValidUntil, wantUntil)
	}
	if stored.HasPendingChange() {
		t.Error("pending change survived the apply")
	}
	transitions := h.transitionsOf(t, sub)
	if len(transitions) != 1 || transitions[0].Reason != domain.TransitionReasonScheduledChangeApplied {
		t.Fatalf("transitions = %+v, want one scheduled_change_applied", transitions)
	}
	if got := archiver.recorded(); len(got) != 1 || got[0].limit != h.basic.ActivePropertyLimit || got[0].ownerID != sub.UserID {
		t.Errorf("archive calls = %+v, want one at the basic limit", got)
	}
	if got := slots.recorded(); len(got) != 1 || got[0] != "scheduled_downgrade" {
		t.Errorf("slot calls = %v, want one scheduled_downgrade", got)
	}

	// A second run has nothing left to apply.
	again, err := h.workers.ProcessScheduledChanges(t.Context(), h.now)
	if err != nil {
		t.Fatalf("second ProcessScheduledChanges() error = %v", err)
	}
	if again != 0 {
		t.Errorf("second ProcessScheduledChanges() = %d, want 0", again)
	}
}

// TestWorkers_ScheduledChangesSkipPaidTargetForRenewalCharge proves a paid
// deferred change is left for the renewal phase, which charges it at apply
// time and switches the tariff on success (ported from
// TestBilling_ProcessScheduledChanges_PaidDowngradeSkippedForRenewalCharge
// and TestBilling_ProcessRenewals_PaidScheduledDowngradeChargedAtApply).
func TestWorkers_ScheduledChangesSkipPaidTargetForRenewalCharge(t *testing.T) {
	h := newWorkersHarness(t, Config{})
	archiver := &fakeArchiverSource{}
	h.workers.SetLifecycleBridges(archiver, nil)

	// business -> pro: a paid downgrade, due now.
	sub := h.seedSubscription(t, func(s *domain.Subscription) {
		s.TariffID = h.business.ID
		target := h.pro.ID
		changeAt := h.now.Add(-time.Hour)
		period := domain.PeriodMonth
		s.PendingTariffID = &target
		s.PendingChangeAt = &changeAt
		s.PendingPeriod = &period
	})
	h.seedActiveMethod(t, sub, "fake", "token_good")

	count, err := h.workers.ProcessScheduledChanges(t.Context(), h.now)
	if err != nil {
		t.Fatalf("ProcessScheduledChanges() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("ProcessScheduledChanges() = %d, want 0 (paid target skipped)", count)
	}
	if stored := h.storedSubscription(t, sub); !stored.HasPendingChange() {
		t.Fatal("paid pending change was consumed by the scheduled phase, want left for the renewal charge")
	}

	// The renewal phase charges the pending target and applies the switch.
	renewed, err := h.workers.ProcessRenewals(t.Context(), h.now)
	if err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}
	if renewed != 1 {
		t.Fatalf("ProcessRenewals() = %d, want 1", renewed)
	}
	stored := h.storedSubscription(t, sub)
	if stored.TariffID != h.pro.ID {
		t.Errorf("TariffID = %s, want pro applied at charge time", stored.TariffID)
	}
	if stored.HasPendingChange() {
		t.Error("pending change survived the charged apply")
	}
	pending := h.singlePaymentOf(t, sub.UserID)
	if pending.TariffID != h.pro.ID || pending.Status != domain.PaymentStatusSucceeded {
		t.Errorf("payment = tariff %s status %s, want pro/succeeded", pending.TariffID, pending.Status)
	}
	// The limit dropped from business (unlimited) to pro (5): the archive
	// bridge ran in the applying transaction.
	if got := archiver.recorded(); len(got) != 1 || got[0].limit != h.pro.ActivePropertyLimit {
		t.Errorf("archive calls = %+v, want one at the pro limit", got)
	}
}

// TestWorkers_ExpiredGraceDowngradesToBasicWithBridges proves the expired
// grace path: basic tariff, expiry transition, and the archive/slot bridges in
// the same transaction — and that a bridge failure rolls the whole phase back
// for the next tick (the shared expiry path, ported from
// TestBilling_ProcessExpiredGrace_DowngradesToBasic).
func TestWorkers_ExpiredGraceDowngradesToBasicWithBridges(t *testing.T) {
	h := newWorkersHarness(t, Config{})
	archiver := &fakeArchiverSource{}
	slots := &fakeSlotSource{}
	h.workers.SetLifecycleBridges(archiver, slots)

	sub := h.seedSubscription(t, func(s *domain.Subscription) {
		s.Status = domain.SubscriptionStatusGrace
		until := h.now.Add(-time.Hour)
		s.ValidUntil = &until
	})

	count, err := h.workers.ProcessExpiredGrace(t.Context(), h.now)
	if err != nil {
		t.Fatalf("ProcessExpiredGrace() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("ProcessExpiredGrace() = %d, want 1", count)
	}
	stored := h.storedSubscription(t, sub)
	if stored.TariffID != h.basic.ID || stored.ValidUntil != nil || stored.AutoRenewEnabled || stored.Status != domain.SubscriptionStatusActive {
		t.Errorf("subscription = %+v, want basic/no validity/no autorenew/active", stored)
	}
	transitions := h.transitionsOf(t, sub)
	if len(transitions) != 1 || transitions[0].Reason != domain.TransitionReasonExpired {
		t.Fatalf("transitions = %+v, want one expired", transitions)
	}
	if transitions[0].FromStatus == nil || *transitions[0].FromStatus != domain.SubscriptionStatusGrace {
		t.Error("expiry transition does not record the grace from-side")
	}
	if got := archiver.recorded(); len(got) != 1 || got[0].limit != h.basic.ActivePropertyLimit {
		t.Errorf("archive calls = %+v, want one at the basic limit", got)
	}
	if got := slots.recorded(); len(got) != 1 || got[0] != "grace_expired" {
		t.Errorf("slot calls = %v, want one grace_expired", got)
	}
}

// TestWorkers_RenewalsExpireNonRenewingAndCancelled proves the non-renewing
// and cancelled expiries share the downgrade-to-basic path with their archive
// triggers (ported from the old expireNonRenewingSubscription scenarios).
func TestWorkers_RenewalsExpireNonRenewingAndCancelled(t *testing.T) {
	h := newWorkersHarness(t, Config{})
	slots := &fakeSlotSource{}
	h.workers.SetLifecycleBridges(nil, slots)

	nonRenewing := h.seedSubscription(t, func(s *domain.Subscription) { s.AutoRenewEnabled = false })
	cancelled := h.seedSubscription(t, func(s *domain.Subscription) { s.AutoRenewEnabled = false })
	if err := cancelled.Cancel(); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if err := h.stores.subscriptions.Update(t.Context(), cancelled); err != nil {
		t.Fatalf("store cancelled: %v", err)
	}

	count, err := h.workers.ProcessRenewals(t.Context(), h.now)
	if err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("ProcessRenewals() = %d, want 2 (both expired)", count)
	}
	for _, sub := range []domain.Subscription{nonRenewing, cancelled} {
		stored := h.storedSubscription(t, sub)
		if stored.TariffID != h.basic.ID || stored.ValidUntil != nil {
			t.Errorf("subscription %s: tariff %s until %v, want basic/no validity", sub.UserID, stored.TariffID, stored.ValidUntil)
		}
		transitions := h.transitionsOf(t, sub)
		if len(transitions) != 1 || transitions[0].Reason != domain.TransitionReasonExpired {
			t.Errorf("subscription %s transitions = %+v, want one expired", sub.UserID, transitions)
		}
	}
	if got := slots.recorded(); len(got) != 2 || !containsAll(got, "non_renewing_expired", "cancelled_expired") {
		t.Errorf("slot calls = %v, want one non_renewing_expired and one cancelled_expired", got)
	}
}

// containsAll reports whether every label occurs in the slice.
func containsAll(got []string, want ...string) bool {
	for _, w := range want {
		if !slices.Contains(got, w) {
			return false
		}
	}
	return true
}

// TestWorkers_ProcessRenewalsRequiresProvider proves the charge-driven phase
// refuses to run without a provider instead of punishing users for a wiring
// mistake.
func TestWorkers_ProcessRenewalsRequiresProvider(t *testing.T) {
	h := newWorkersHarness(t, Config{})
	h.workers.provider = nil
	if _, err := h.workers.ProcessRenewals(t.Context(), h.now); !errors.Is(err, ErrPaymentUnavailable) {
		t.Fatalf("ProcessRenewals() error = %v, want ErrPaymentUnavailable", err)
	}
}

// TestWorkers_ReconcileStalePendingPayments proves the lost-webhook backstop:
// a stale pending payment is finalized from the provider's status through the
// synchronous notification path; a provider-pending payment is left alone
// (ported from TestBilling_ProcessPendingUpgradePayments_*).
func TestWorkers_ReconcileStalePendingPayments(t *testing.T) {
	newStaleUpgrade := func(t *testing.T, h *workersHarness, sub domain.Subscription, ref string) domain.SubscriptionPayment {
		t.Helper()
		payment, err := domain.NewSubscriptionPayment(sub.UserID, sub.ID, h.business.ID, domain.PeriodMonth, h.business.MonthlyPriceKopecks, "fake", h.now.Add(-10*time.Minute))
		if err != nil {
			t.Fatalf("new payment: %v", err)
		}
		if err := payment.SaveProviderReference(ref, "http://pay", h.now.Add(-10*time.Minute)); err != nil {
			t.Fatalf("save reference: %v", err)
		}
		stored, err := h.stores.payments.Create(t.Context(), payment)
		if err != nil {
			t.Fatalf("seed payment: %v", err)
		}
		return stored
	}

	t.Run("succeeded finalizes upgrade", func(t *testing.T) {
		h := newWorkersHarness(t, Config{})
		sub := h.seedSubscription(t, nil) // still on pro; the payment buys business
		payment := newStaleUpgrade(t, h, sub, "prov_stale_1")
		h.provider.statusFn = func(uuid.UUID, string) (PaymentStatusResult, error) {
			return PaymentStatusResult{Status: domain.PaymentStatusSucceeded}, nil
		}

		count, err := h.workers.ProcessPendingUpgradePayments(t.Context(), h.now)
		if err != nil {
			t.Fatalf("ProcessPendingUpgradePayments() error = %v", err)
		}
		if count != 1 {
			t.Fatalf("count = %d, want 1", count)
		}
		stored, err := h.stores.payments.GetByID(t.Context(), payment.ID)
		if err != nil || stored.Status != domain.PaymentStatusSucceeded {
			t.Fatalf("payment = %s (err %v), want succeeded", stored.Status, err)
		}
		if got := h.storedSubscription(t, sub); got.TariffID != h.business.ID {
			t.Errorf("TariffID = %s, want business applied", got.TariffID)
		}
	})

	t.Run("failed marks failed without subscription damage", func(t *testing.T) {
		h := newWorkersHarness(t, Config{})
		sub := h.seedSubscription(t, nil)
		payment := newStaleUpgrade(t, h, sub, "prov_stale_2")
		h.provider.statusFn = func(uuid.UUID, string) (PaymentStatusResult, error) {
			return PaymentStatusResult{Status: domain.PaymentStatusFailed, ErrorCode: "53"}, nil
		}

		if _, err := h.workers.ProcessPendingUpgradePayments(t.Context(), h.now); err != nil {
			t.Fatalf("ProcessPendingUpgradePayments() error = %v", err)
		}
		stored, err := h.stores.payments.GetByID(t.Context(), payment.ID)
		if err != nil || stored.Status != domain.PaymentStatusFailed {
			t.Fatalf("payment = %s (err %v), want failed", stored.Status, err)
		}
		if got := h.storedSubscription(t, sub); got.Status != domain.SubscriptionStatusActive || got.TariffID != h.pro.ID {
			t.Errorf("subscription = %s/%s, want untouched active/pro", got.Status, got.TariffID)
		}
	})

	t.Run("provider pending is left alone", func(t *testing.T) {
		h := newWorkersHarness(t, Config{})
		sub := h.seedSubscription(t, nil)
		payment := newStaleUpgrade(t, h, sub, "prov_stale_3")
		h.provider.statusFn = func(uuid.UUID, string) (PaymentStatusResult, error) {
			return PaymentStatusResult{Status: domain.PaymentStatusPending}, nil
		}

		if _, err := h.workers.ReconcilePendingPayments(t.Context(), h.now); err != nil {
			t.Fatalf("ReconcilePendingPayments() error = %v", err)
		}
		stored, err := h.stores.payments.GetByID(t.Context(), payment.ID)
		if err != nil || stored.Status != domain.PaymentStatusPending {
			t.Fatalf("payment = %s (err %v), want still pending", stored.Status, err)
		}
	})

	t.Run("fresh pending payments are not stale", func(t *testing.T) {
		h := newWorkersHarness(t, Config{})
		sub := h.seedSubscription(t, nil)
		payment, err := domain.NewSubscriptionPayment(sub.UserID, sub.ID, h.business.ID, domain.PeriodMonth, h.business.MonthlyPriceKopecks, "fake", h.now.Add(-time.Minute))
		if err != nil {
			t.Fatalf("new payment: %v", err)
		}
		if err := payment.SaveProviderReference("prov_fresh", "http://pay", h.now.Add(-time.Minute)); err != nil {
			t.Fatalf("save reference: %v", err)
		}
		if _, err := h.stores.payments.Create(t.Context(), payment); err != nil {
			t.Fatalf("seed payment: %v", err)
		}
		h.provider.statusFn = func(uuid.UUID, string) (PaymentStatusResult, error) {
			t.Error("the provider must not be queried for a fresh payment")
			return PaymentStatusResult{}, nil
		}
		if _, err := h.workers.ReconcilePendingPayments(t.Context(), h.now); err != nil {
			t.Fatalf("ReconcilePendingPayments() error = %v", err)
		}
	})
}

// TestWorkers_ReconciliationRoutesThroughLifecyclePort proves the
// reconciliation phases depend on nothing but the narrow PaymentLifecycle
// port (issue #288): a provider-confirmed outcome arrives as exactly one
// notification application with the provider status translated, and a stuck
// refund as one resolution whose answer drives the batch progress. The
// workers hold no reference to the payment service behind the port.
func TestWorkers_ReconciliationRoutesThroughLifecyclePort(t *testing.T) {
	seedStalePending := func(t *testing.T, h *workersHarness, ref string) domain.SubscriptionPayment {
		t.Helper()
		sub := h.seedSubscription(t, nil)
		payment, err := domain.NewSubscriptionPayment(sub.UserID, sub.ID, h.business.ID, domain.PeriodMonth, h.business.MonthlyPriceKopecks, "fake", h.now.Add(-10*time.Minute))
		if err != nil {
			t.Fatalf("new payment: %v", err)
		}
		if err := payment.SaveProviderReference(ref, "http://pay", h.now.Add(-10*time.Minute)); err != nil {
			t.Fatalf("save reference: %v", err)
		}
		stored, err := h.stores.payments.Create(t.Context(), payment)
		if err != nil {
			t.Fatalf("seed payment: %v", err)
		}
		return stored
	}
	seedStuckRefund := func(t *testing.T, h *workersHarness) domain.SubscriptionPayment {
		t.Helper()
		sub := h.seedSubscription(t, nil)
		payment, err := domain.NewSubscriptionPayment(sub.UserID, sub.ID, h.pro.ID, domain.PeriodMonth, h.pro.MonthlyPriceKopecks, "fake", h.now)
		if err != nil {
			t.Fatalf("new payment: %v", err)
		}
		providerPaymentID := "prov_stuck_port"
		payment.ProviderPaymentID = &providerPaymentID
		if err := payment.MarkSucceeded(h.now); err != nil {
			t.Fatalf("mark succeeded: %v", err)
		}
		if err := payment.BeginRefund(h.now); err != nil {
			t.Fatalf("begin refund: %v", err)
		}
		stored, err := h.stores.payments.Create(t.Context(), payment)
		if err != nil {
			t.Fatalf("seed payment: %v", err)
		}
		return stored
	}
	swapPort := func(t *testing.T, h *workersHarness) *scriptedLifecycle {
		t.Helper()
		lifecycle := &scriptedLifecycle{}
		h.workers.payments = lifecycle
		return lifecycle
	}

	t.Run("succeeded outcome arrives as one applied notification", func(t *testing.T) {
		h := newWorkersHarness(t, Config{})
		payment := seedStalePending(t, h, "prov_port_1")
		lifecycle := swapPort(t, h)
		h.provider.statusFn = func(uuid.UUID, string) (PaymentStatusResult, error) {
			return PaymentStatusResult{Status: domain.PaymentStatusSucceeded}, nil
		}

		count, err := h.workers.ReconcilePendingPayments(t.Context(), h.now)
		if err != nil {
			t.Fatalf("ReconcilePendingPayments() error = %v", err)
		}
		if count != 1 {
			t.Fatalf("count = %d, want 1", count)
		}
		applied := lifecycle.applications()
		if len(applied) != 1 {
			t.Fatalf("port applications = %d, want 1", len(applied))
		}
		want := PaymentNotification{
			InternalPaymentID: payment.ID,
			ProviderPaymentID: "prov_port_1",
			Status:            domain.PaymentStatusSucceeded,
			AmountKopecks:     payment.AmountKopecks,
		}
		if applied[0] != want {
			t.Errorf("notification = %+v, want %+v", applied[0], want)
		}
		if got := lifecycle.resolutions(); len(got) != 0 {
			t.Errorf("port resolutions = %d, want 0", len(got))
		}
	})

	t.Run("failed outcome keeps the provider error code", func(t *testing.T) {
		h := newWorkersHarness(t, Config{})
		payment := seedStalePending(t, h, "prov_port_2")
		lifecycle := swapPort(t, h)
		h.provider.statusFn = func(uuid.UUID, string) (PaymentStatusResult, error) {
			return PaymentStatusResult{Status: domain.PaymentStatusFailed, ErrorCode: "53"}, nil
		}

		if _, err := h.workers.ReconcilePendingPayments(t.Context(), h.now); err != nil {
			t.Fatalf("ReconcilePendingPayments() error = %v", err)
		}
		applied := lifecycle.applications()
		if len(applied) != 1 {
			t.Fatalf("port applications = %d, want 1", len(applied))
		}
		if applied[0].Status != domain.PaymentStatusFailed || applied[0].ErrorCode == nil || *applied[0].ErrorCode != "53" {
			t.Errorf("notification = %s/%v, want failed with code 53", applied[0].Status, applied[0].ErrorCode)
		}
		if applied[0].InternalPaymentID != payment.ID {
			t.Errorf("notification payment = %s, want %s", applied[0].InternalPaymentID, payment.ID)
		}
	})

	t.Run("stuck refund resolves through the port", func(t *testing.T) {
		h := newWorkersHarness(t, Config{})
		payment := seedStuckRefund(t, h)
		lifecycle := swapPort(t, h)
		lifecycle.resolveOut = true
		h.provider.statusFn = func(uuid.UUID, string) (PaymentStatusResult, error) {
			return PaymentStatusResult{Status: domain.PaymentStatusRefunded}, nil
		}

		processed, err := h.workers.ReconcileStaleRefunds(t.Context(), h.now.Add(2*h.cfg.PendingPaymentStaleness))
		if err != nil {
			t.Fatalf("ReconcileStaleRefunds() error = %v", err)
		}
		if processed != 1 {
			t.Fatalf("processed = %d, want 1", processed)
		}
		resolutions := lifecycle.resolutions()
		if len(resolutions) != 1 {
			t.Fatalf("port resolutions = %d, want 1", len(resolutions))
		}
		if resolutions[0].payment.ID != payment.ID {
			t.Errorf("resolution payment = %s, want %s", resolutions[0].payment.ID, payment.ID)
		}
		if resolutions[0].status.Status != domain.PaymentStatusRefunded {
			t.Errorf("resolution status = %q, want refunded", resolutions[0].status.Status)
		}
		if got := lifecycle.applications(); len(got) != 0 {
			t.Errorf("port applications = %d, want 0", len(got))
		}
	})

	t.Run("unresolved answer leaves the reservation untouched", func(t *testing.T) {
		h := newWorkersHarness(t, Config{})
		payment := seedStuckRefund(t, h)
		lifecycle := swapPort(t, h)
		h.provider.statusFn = func(uuid.UUID, string) (PaymentStatusResult, error) {
			return PaymentStatusResult{Status: domain.PaymentStatusRefunded}, nil
		}

		if _, err := h.workers.ReconcileStaleRefunds(t.Context(), h.now.Add(2*h.cfg.PendingPaymentStaleness)); err != nil {
			t.Fatalf("ReconcileStaleRefunds() error = %v", err)
		}
		if got := lifecycle.resolutions(); len(got) != 1 {
			t.Fatalf("port resolutions = %d, want 1", len(got))
		}
		stored, err := h.stores.payments.GetByID(t.Context(), payment.ID)
		if err != nil {
			t.Fatalf("get payment: %v", err)
		}
		if stored.Status != domain.PaymentStatusRefunding {
			t.Errorf("payment status = %q, want the reservation kept when the port reports no resolution", stored.Status)
		}
	})
}

// TestWorkers_ReconcileStaleRefundsRequiresLifecycle proves the refund
// reconciliation refuses to run without the payment lifecycle instead of
// punishing users for a wiring mistake.
func TestWorkers_ReconcileStaleRefundsRequiresLifecycle(t *testing.T) {
	h := newWorkersHarness(t, Config{})
	h.workers.payments = nil
	if _, err := h.workers.ReconcileStaleRefunds(t.Context(), h.now); !errors.Is(err, ErrPaymentUnavailable) {
		t.Fatalf("ReconcileStaleRefunds() error = %v, want ErrPaymentUnavailable", err)
	}
}

// TestWorkers_AsyncFailedRenewalChargeEntersGrace proves the asynchronous
// failure path of a merchant-initiated charge: the provider reports the
// outcome later (webhook or reconciliation) and the payment carries the
// charged method — the failed renewal enters grace exactly like the
// synchronous decline, so the next tick does not charge a fresh payment
// forever. A customer-initiated payment (no method) leaves the subscription
// untouched (ported from TestBilling_HandleWebhook_FailedRenewalMovesToGrace).
func TestWorkers_AsyncFailedRenewalChargeEntersGrace(t *testing.T) {
	newHarnessWithPayments := func(t *testing.T) (*workersHarness, domain.Subscription, *domain.SubscriptionPayment) {
		t.Helper()
		h := newWorkersHarness(t, Config{})
		sub := h.seedSubscription(t, nil)
		method := h.seedActiveMethod(t, sub, "fake", "token_async")
		payment, err := domain.NewSubscriptionPayment(sub.UserID, sub.ID, h.pro.ID, domain.PeriodMonth, h.pro.MonthlyPriceKopecks, "fake", h.now.Add(-time.Minute))
		if err != nil {
			t.Fatalf("new payment: %v", err)
		}
		methodID := method.ID
		payment.PaymentMethodID = &methodID
		if err := payment.SaveProviderReference("prov_async", "", h.now.Add(-time.Minute)); err != nil {
			t.Fatalf("save reference: %v", err)
		}
		stored, err := h.stores.payments.Create(t.Context(), payment)
		if err != nil {
			t.Fatalf("seed payment: %v", err)
		}
		return h, sub, &stored
	}

	t.Run("merchant-initiated renewal enters grace", func(t *testing.T) {
		h, sub, payment := newHarnessWithPayments(t)
		err := h.workers.payments.ApplyPaymentNotification(t.Context(), &PaymentNotification{
			InternalPaymentID: payment.ID,
			ProviderPaymentID: "prov_async",
			Status:            domain.PaymentStatusFailed,
		})
		if err != nil {
			t.Fatalf("ApplyPaymentNotification() error = %v", err)
		}
		stored := h.storedSubscription(t, sub)
		if stored.Status != domain.SubscriptionStatusGrace {
			t.Fatalf("status = %q, want grace after the async failed renewal", stored.Status)
		}
		transitions := h.transitionsOf(t, sub)
		if len(transitions) != 1 || transitions[0].Reason != domain.TransitionReasonGraceEntered {
			t.Fatalf("transitions = %+v, want one grace_entered", transitions)
		}
		failed, err := h.stores.payments.GetByID(t.Context(), payment.ID)
		if err != nil || failed.Status != domain.PaymentStatusFailed {
			t.Fatalf("payment = %s (err %v), want failed", failed.Status, err)
		}
	})

	t.Run("customer-initiated payment leaves subscription untouched", func(t *testing.T) {
		h, sub, payment := newHarnessWithPayments(t)
		// Strip the method: the payment becomes a customer-initiated one.
		payment.PaymentMethodID = nil
		if err := h.stores.payments.Update(t.Context(), *payment); err != nil {
			t.Fatalf("strip method: %v", err)
		}
		err := h.workers.payments.ApplyPaymentNotification(t.Context(), &PaymentNotification{
			InternalPaymentID: payment.ID,
			ProviderPaymentID: "prov_async",
			Status:            domain.PaymentStatusFailed,
		})
		if err != nil {
			t.Fatalf("ApplyPaymentNotification() error = %v", err)
		}
		if stored := h.storedSubscription(t, sub); stored.Status != domain.SubscriptionStatusActive {
			t.Errorf("status = %q, want active (a failed CIT payment changes nothing)", stored.Status)
		}
		if got := h.transitionsOf(t, sub); len(got) != 0 {
			t.Errorf("transitions = %d, want 0", len(got))
		}
	})
}

// TestWorkers_BatchWithoutProgressStops proves a full batch where nothing
// processes stops the loop for this tick instead of spinning: with batch size
// one, the failing item (a pending tariff that no longer exists) blocks the
// second subscription until the next tick (ported from
// TestBilling_ProcessRenewals_BatchWithoutProgressStops and
// ..._ProcessExpiredGrace_BatchWithoutProgressStops).
func TestWorkers_BatchWithoutProgressStops(t *testing.T) {
	h := newWorkersHarness(t, Config{WorkerBatchSize: 1, GraceDuration: 7 * 24 * time.Hour, ChargeAttemptLimit: 3, PendingPaymentStaleness: 5 * time.Minute})

	// First by pending_change_at: its pending tariff is missing, so it fails
	// every attempt. The second is a healthy free downgrade that must wait.
	failing := h.seedSubscription(t, func(s *domain.Subscription) {
		target := mustNewUUID()
		changeAt := h.now.Add(-2 * time.Hour)
		period := domain.PeriodMonth
		s.PendingTariffID = &target
		s.PendingChangeAt = &changeAt
		s.PendingPeriod = &period
	})
	healthy := h.seedSubscription(t, func(s *domain.Subscription) {
		target := h.basic.ID
		changeAt := h.now.Add(-time.Hour)
		period := domain.PeriodMonth
		s.PendingTariffID = &target
		s.PendingChangeAt = &changeAt
		s.PendingPeriod = &period
	})

	count, err := h.workers.ProcessScheduledChanges(t.Context(), h.now)
	if err != nil {
		t.Fatalf("ProcessScheduledChanges() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("ProcessScheduledChanges() = %d, want 0", count)
	}
	if got := h.storedSubscription(t, healthy); !got.HasPendingChange() {
		t.Error("the healthy change was processed in the same tick, want deferred by the no-progress guard")
	}
	if got := h.storedSubscription(t, failing); !got.HasPendingChange() {
		t.Error("the failing change vanished, want kept for retry")
	}
}

// TestWorkers_UncertainChargeKeepsProviderErrorCode proves the provider's own
// error code survives into the failed payment when the outcome was resolved
// after an errored charge (ported from
// TestBilling_ProcessRenewals_TypedProviderErrorsKeepGraceAndErrorCode).
func TestWorkers_UncertainChargeKeepsProviderErrorCode(t *testing.T) {
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, nil)
	h.seedActiveMethod(t, sub, "fake", "token_good")

	codeErr := fmt.Errorf("charge rejected: %w", &testProviderError{code: "156"})
	var statusCalls int
	h.provider.statusFn = func(uuid.UUID, string) (PaymentStatusResult, error) {
		statusCalls++
		if statusCalls == 1 {
			return PaymentStatusResult{Status: domain.PaymentStatusPending}, nil
		}
		return PaymentStatusResult{Status: domain.PaymentStatusFailed, ErrorCode: "156"}, nil
	}
	h.provider.chargeFn = func(ChargeRequest) (ChargeResult, error) {
		return ChargeResult{}, codeErr
	}

	if _, err := h.workers.ProcessRenewals(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}
	pending := h.singlePaymentOf(t, sub.UserID)
	if pending.Status != domain.PaymentStatusFailed || pending.ErrorCode == nil || *pending.ErrorCode != "156" {
		t.Errorf("payment = %s code %v, want failed/156", pending.Status, pending.ErrorCode)
	}
	if h.storedSubscription(t, sub).Status != domain.SubscriptionStatusGrace {
		t.Error("subscription not in grace after the typed provider failure")
	}
}

// testProviderError carries a provider error code the way adapter errors do.
type testProviderError struct{ code string }

func (e *testProviderError) Error() string             { return "provider error " + e.code }
func (e *testProviderError) ProviderErrorCode() string { return e.code }
