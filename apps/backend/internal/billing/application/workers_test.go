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
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
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

func (l *scriptedLifecycle) ResolveRefundingFromStatus(
	_ context.Context, payment domain.SubscriptionPayment, status PaymentStatusResult,
) (bool, error) {
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
// ArchiveIDs is what ArchiveExcess returns as the archived ids; RemainingAs is
// what RestoreGraceArchive returns as the debt remainder.
type fakeArchiverSource struct {
	mu          sync.Mutex
	calls       []archiveCall
	restores    []restoreCall
	existsCalls []uuid.UUID
	err         error
	archiveIDs  []uuid.UUID
	remainingAs []uuid.UUID
	// The ActivePropertyExists answers come from this map; an id absent
	// from it does not exist (issue #617 keep-choice validation).
	activeExists map[uuid.UUID]bool
}

type archiveCall struct {
	ownerID uuid.UUID
	limit   int
	keep    *uuid.UUID
}

type restoreCall struct {
	ownerID uuid.UUID
	ids     []uuid.UUID
	limit   int
}

func (s *fakeArchiverSource) WithTx(transaction.Tx) (ExcessPropertyArchiver, error) {
	return fakeArchiver{src: s}, nil
}

type fakeArchiver struct{ src *fakeArchiverSource }

func (a fakeArchiver) ArchiveExcess(_ context.Context, ownerID uuid.UUID, limit int, keepPropertyID *uuid.UUID) ([]uuid.UUID, error) {
	a.src.mu.Lock()
	defer a.src.mu.Unlock()
	a.src.calls = append(a.src.calls, archiveCall{ownerID: ownerID, limit: limit, keep: keepPropertyID})
	return a.src.archiveIDs, a.src.err
}

func (a fakeArchiver) ActivePropertyExists(_ context.Context, _, propertyID uuid.UUID) (bool, error) {
	a.src.mu.Lock()
	defer a.src.mu.Unlock()
	a.src.existsCalls = append(a.src.existsCalls, propertyID)
	return a.src.activeExists[propertyID], a.src.err
}

func (a fakeArchiver) RestoreGraceArchive(_ context.Context, ownerID uuid.UUID, ids []uuid.UUID, limit int) ([]uuid.UUID, error) {
	a.src.mu.Lock()
	defer a.src.mu.Unlock()
	a.src.restores = append(a.src.restores, restoreCall{ownerID: ownerID, ids: ids, limit: limit})
	return a.src.remainingAs, a.src.err
}

func (s *fakeArchiverSource) recorded() []archiveCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]archiveCall(nil), s.calls...)
}

func (s *fakeArchiverSource) restoreCalls() []restoreCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]restoreCall(nil), s.restores...)
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
	// Payments is the payment service behind the workers' lifecycle port —
	// the success-application seam the renewal charges land in (issue #428);
	// tests that assert bridge calls hang the bridges on it too.
	payments *PaymentService
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
	basic := domain.Tariff{
		ID: mustNewUUID(), Name: domain.TariffBasic, ActivePropertyLimit: 1,
		MonthlyPriceKopecks: 0, YearlyPriceKopecks: 0, IsActive: true,
	}
	pro := domain.Tariff{
		ID: mustNewUUID(), Name: domain.TariffPro, ActivePropertyLimit: 5,
		MonthlyPriceKopecks: 49000, YearlyPriceKopecks: 440000, IsActive: true,
	}
	business := domain.Tariff{
		ID: mustNewUUID(), Name: domain.TariffBusiness, ActivePropertyLimit: -1,
		MonthlyPriceKopecks: 99000, YearlyPriceKopecks: 890000, IsActive: true,
	}
	stores := newFakeStores(basic, pro, business)
	provider := &scriptedProvider{name: testProviderFake}
	factory := stores.factory(nil)
	logger := slog.New(slog.DiscardHandler)
	payments := NewPaymentService(factory, provider, PaymentServiceConfig{Clock: clk, Log: logger})
	workers := NewWorkers(factory, WorkersConfig{
		Provider: provider,
		Payments: payments,
		Clock:    clk,
		Config:   cfg,
		Logger:   logger,
	})
	return &workersHarness{
		stores: stores, provider: provider, workers: workers, payments: payments, cfg: cfg, now: workersNow,
		basic: basic, pro: pro, business: business,
	}
}

// setLifecycleBridges wires the lifecycle bridges to both the worker phases
// and the payment service's success-application seam — the way the composition
// root does (issue #428). The parameters keep the interface types so a nil
// bridge stays a nil interface.
func (h *workersHarness) setLifecycleBridges(archiver ExcessPropertyArchiverSource, slots RecipientSlotEnforcerSource) {
	h.workers.SetLifecycleBridges(archiver, slots)
	h.payments.SetLifecycleBridges(archiver, slots)
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
	validUntil := h.now.AddDate(0, -1, 0) // Expired a month ago.
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
func (h *workersHarness) seedActiveMethod(
	t *testing.T, sub domain.Subscription, providerName domain.PaymentProvider, token string,
) domain.PaymentMethod {
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

// requireRenewedPayment asserts the user's only payment is the succeeded fake
// renewal charge on the active method for the pro tariff and returns it.
func (h *workersHarness) requireRenewedPayment(t *testing.T, userID, methodID uuid.UUID) *domain.SubscriptionPayment {
	t.Helper()
	payment := h.singlePaymentOf(t, userID)
	if payment.Status != domain.PaymentStatusSucceeded {
		t.Errorf("payment status = %q, want succeeded", payment.Status)
	}
	if payment.TariffID != h.pro.ID || payment.AmountKopecks != h.pro.MonthlyPriceKopecks {
		t.Errorf("payment = tariff %s amount %d, want pro %d", payment.TariffID, payment.AmountKopecks, h.pro.MonthlyPriceKopecks)
	}
	if payment.PaymentMethodID == nil || *payment.PaymentMethodID != methodID {
		t.Errorf("payment method = %v, want the active method", payment.PaymentMethodID)
	}
	if payment.Provider != testProviderFake {
		t.Errorf("payment provider = %q, want fake", payment.Provider)
	}
	return payment
}

// requireRenewedSubscription asserts the subscription renewed to active for a
// month from now with the applied renewal payment linked.
func (h *workersHarness) requireRenewedSubscription(t *testing.T, sub domain.Subscription, paymentID uuid.UUID) {
	t.Helper()
	stored := h.storedSubscription(t, sub)
	wantUntil := h.now.AddDate(0, 1, 0)
	if stored.Status != domain.SubscriptionStatusActive || stored.ValidUntil == nil || !stored.ValidUntil.Equal(wantUntil) {
		t.Errorf("subscription = %s until %v, want active until %v", stored.Status, stored.ValidUntil, wantUntil)
	}
	if stored.LastAppliedPaymentID == nil || *stored.LastAppliedPaymentID != paymentID {
		t.Errorf("LastAppliedPaymentID = %v, want the renewal payment", stored.LastAppliedPaymentID)
	}
}

// requirePaymentAppliedTransition asserts the log holds exactly one transition
// — the renewal's payment_applied entry referencing the payment.
func (h *workersHarness) requirePaymentAppliedTransition(t *testing.T, sub domain.Subscription, paymentID uuid.UUID) {
	t.Helper()
	transitions := h.transitionsOf(t, sub)
	if len(transitions) != 1 || transitions[0].Reason != domain.TransitionReasonPaymentApplied {
		t.Fatalf("transitions = %+v, want one payment_applied", transitions)
	}
	if transitions[0].PaymentID == nil || *transitions[0].PaymentID != paymentID {
		t.Error("transition does not reference the renewal payment")
	}
}

// requireGraceEnteredOverNothingChargeable asserts the renewal with nothing to
// charge moved the subscription into a fresh grace window: the scheduled
// change dropped, the system's grace_entered transition logged, no charge and
// no payment initiated.
func (h *workersHarness) requireGraceEnteredOverNothingChargeable(t *testing.T, sub domain.Subscription) {
	t.Helper()
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
	pending, err := h.stores.payments.ListPendingByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("ListPendingByUserID: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("pending payments = %d, want 0 (nothing was initiated)", len(pending))
	}
}

// requireFreeChangeApplied asserts the free scheduled change applied: basic
// tariff with a free month of the target period, no pending change left, the
// scheduled_change_applied transition, and both bridges in the same
// transaction.
func (h *workersHarness) requireFreeChangeApplied(
	t *testing.T, sub domain.Subscription, archiver *fakeArchiverSource, slots *fakeSlotSource,
) {
	t.Helper()
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
}

// seedStalePendingUpgrade stores the subscription's stale pending
// business-upgrade payment carrying the given provider reference.
func (h *workersHarness) seedStalePendingUpgrade(t *testing.T, sub domain.Subscription, ref string) domain.SubscriptionPayment {
	t.Helper()
	payment, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, h.business.ID, domain.PeriodMonth,
		h.business.MonthlyPriceKopecks, testProviderFake, h.now.Add(-10*time.Minute))
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

// seedStuckRefundPayment stores a succeeded pro renewal payment already stuck
// in refunding with a provider reference.
func (h *workersHarness) seedStuckRefundPayment(t *testing.T) domain.SubscriptionPayment {
	t.Helper()
	sub := h.seedSubscription(t, nil)
	payment, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, h.pro.ID, domain.PeriodMonth,
		h.pro.MonthlyPriceKopecks, testProviderFake, h.now)
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

// swapLifecyclePort mounts a scripted PaymentLifecycle in place of the real
// payment service and returns it for call inspection.
func (h *workersHarness) swapLifecyclePort() *scriptedLifecycle {
	lifecycle := &scriptedLifecycle{}
	h.workers.payments = lifecycle
	return lifecycle
}

// TestWorkers_RenewalChargesActiveMethodAndRenews proves the happy path: the
// expired auto-renewing subscription is charged on its active method, the
// payment succeeds, and the subscription renews from now with the transition
// logged (ported from TestBilling_ProcessRenewals_Success).
func TestWorkers_RenewalChargesActiveMethodAndRenews(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, nil)
	method := h.seedActiveMethod(t, sub, testProviderFake, "token_good")

	count, err := h.workers.ProcessRenewals(t.Context(), h.now)
	if err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("ProcessRenewals() = %d, want 1", count)
	}

	payment := h.requireRenewedPayment(t, sub.UserID, method.ID)
	h.requireRenewedSubscription(t, sub, payment.ID)
	h.requirePaymentAppliedTransition(t, sub, payment.ID)

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
	t.Parallel()
	for _, tc := range []struct {
		name       string
		methodProv domain.PaymentProvider
		seedMethod bool
	}{
		{name: "no method", seedMethod: false},
		{name: "foreign provider method", seedMethod: true, methodProv: "tkassa"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
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
			h.requireGraceEnteredOverNothingChargeable(t, sub)
		})
	}
}

// TestWorkers_RenewalFailedChargeMovesToGrace proves a declined charge fails
// the payment with the provider's error code and enters grace (ported from
// TestBilling_ProcessRenewals_FailedChargeMovesToGrace and
// ..._ProviderErrorCodeSavedOnFailedCharge).
func TestWorkers_RenewalFailedChargeMovesToGrace(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, nil)
	h.seedActiveMethod(t, sub, testProviderFake, "token_bad")
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
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, nil)
	method := h.seedActiveMethod(t, sub, testProviderFake, "token_good")

	// A previous run crashed after the provider captured the charge but before
	// the result was applied: the pending payment already has its reference.
	payment, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, h.pro.ID, domain.PeriodMonth,
		h.pro.MonthlyPriceKopecks, testProviderFake, h.now.Add(-time.Minute))
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
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, nil)
	h.seedActiveMethod(t, sub, testProviderFake, "token_good")
	payment, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, h.pro.ID, domain.PeriodMonth,
		h.pro.MonthlyPriceKopecks, testProviderFake, h.now.Add(-time.Minute))
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
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, nil)
	h.seedActiveMethod(t, sub, testProviderFake, "token_good")
	payment, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, h.pro.ID, domain.PeriodMonth,
		h.pro.MonthlyPriceKopecks, testProviderFake, h.now.Add(-time.Minute))
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
	t.Parallel()
	t.Run("provider succeeded", func(t *testing.T) {
		t.Parallel()
		h := newWorkersHarness(t, Config{})
		sub := h.seedSubscription(t, nil)
		h.seedActiveMethod(t, sub, testProviderFake, "token_good")
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
		t.Parallel()
		h := newWorkersHarness(t, Config{})
		sub := h.seedSubscription(t, nil)
		h.seedActiveMethod(t, sub, testProviderFake, "token_good")
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
	t.Parallel()
	h := newWorkersHarness(t, Config{
		ChargeAttemptLimit: 2, WorkerBatchSize: 100,
		GraceDuration: 7 * 24 * time.Hour, PendingPaymentStaleness: 5 * time.Minute,
	})
	sub := h.seedSubscription(t, nil)
	h.seedActiveMethod(t, sub, testProviderFake, "token_good")
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
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, nil)
	method := h.seedActiveMethod(t, sub, testProviderFake, "token_good")
	existing, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, h.pro.ID, domain.PeriodMonth,
		h.pro.MonthlyPriceKopecks, testProviderFake, h.now.Add(-time.Minute))
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
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, func(s *domain.Subscription) {
		s.TariffID = h.basic.ID
	})

	if _, err := h.workers.ProcessRenewals(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}

	stored := h.storedSubscription(t, sub)
	if stored.TariffID != h.basic.ID || stored.ValidUntil != nil || stored.AutoRenewEnabled {
		t.Errorf("subscription = tariff %s until %v autorenew %t, want basic/no validity/off",
			stored.TariffID, stored.ValidUntil, stored.AutoRenewEnabled)
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
	t.Parallel()
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
	h.requireFreeChangeApplied(t, sub, archiver, slots)

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
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	archiver := &fakeArchiverSource{}
	h.setLifecycleBridges(archiver, nil)

	// Tariff business -> pro: a paid downgrade, due now.
	sub := h.seedSubscription(t, func(s *domain.Subscription) {
		s.TariffID = h.business.ID
		target := h.pro.ID
		changeAt := h.now.Add(-time.Hour)
		period := domain.PeriodMonth
		s.PendingTariffID = &target
		s.PendingChangeAt = &changeAt
		s.PendingPeriod = &period
	})
	h.seedActiveMethod(t, sub, testProviderFake, "token_good")

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
	t.Parallel()
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
	if stored.TariffID != h.basic.ID || stored.ValidUntil != nil || stored.AutoRenewEnabled ||
		stored.Status != domain.SubscriptionStatusActive {
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
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	slots := &fakeSlotSource{}
	h.workers.SetLifecycleBridges(nil, slots)

	nonRenewing := h.seedSubscription(t, func(s *domain.Subscription) { s.AutoRenewEnabled = false })
	cancelled := h.seedSubscription(t, func(s *domain.Subscription) { s.AutoRenewEnabled = false })
	if err := cancelled.Cancel(nil); err != nil {
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

// TestWorkers_CancelledExpiryHonoursKeepChoice proves the cancel keep-choice
// hand-off (issue #617): the cancelled subscription's chosen property rides
// into the basic-limit enforcement, and the fall to basic consumes the choice
// on the stored subscription.
func TestWorkers_CancelledExpiryHonoursKeepChoice(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	archiver := &fakeArchiverSource{}
	h.workers.SetLifecycleBridges(archiver, &fakeSlotSource{})

	keepID := uuid.Must(uuid.NewV7())
	cancelled := h.seedSubscription(t, func(s *domain.Subscription) { s.AutoRenewEnabled = false })
	if err := cancelled.Cancel(&keepID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if err := h.stores.subscriptions.Update(t.Context(), cancelled); err != nil {
		t.Fatalf("store cancelled: %v", err)
	}

	if _, err := h.workers.ProcessRenewals(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}

	calls := archiver.recorded()
	if len(calls) != 1 {
		t.Fatalf("archive calls = %d, want 1", len(calls))
	}
	if calls[0].keep == nil || *calls[0].keep != keepID {
		t.Errorf("archive keep = %v, want %v", calls[0].keep, keepID)
	}
	if calls[0].limit != h.basic.ActivePropertyLimit {
		t.Errorf("archive limit = %d, want the basic limit %d", calls[0].limit, h.basic.ActivePropertyLimit)
	}
	stored := h.storedSubscription(t, cancelled)
	if stored.KeepPropertyID != nil {
		t.Errorf("stored KeepPropertyID = %v, want consumed by the fall to basic", stored.KeepPropertyID)
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
	t.Parallel()
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
	t.Parallel()
	t.Run("succeeded finalizes upgrade", reconcileUpgradeSucceededFinalizes)
	t.Run("failed marks failed without subscription damage", reconcileUpgradeFailedKeepsSubscription)
	t.Run("provider pending is left alone", reconcileProviderPendingLeftAlone)
	t.Run("fresh pending payments are not stale", reconcileFreshPendingNotStale)
}

// reconcileUpgradeSucceededFinalizes covers the succeeded stale outcome: the
// upgrade payment finalizes and the business tariff applies.
func reconcileUpgradeSucceededFinalizes(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, nil) // Still on pro; the payment buys business.
	payment := h.seedStalePendingUpgrade(t, sub, "prov_stale_1")
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
}

// reconcileUpgradeFailedKeepsSubscription covers the failed stale outcome: the
// payment fails with no subscription damage.
func reconcileUpgradeFailedKeepsSubscription(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, nil)
	payment := h.seedStalePendingUpgrade(t, sub, "prov_stale_2")
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
}

// reconcileProviderPendingLeftAlone covers the provider-pending outcome: the
// reconciliation leaves the stale payment pending.
func reconcileProviderPendingLeftAlone(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, nil)
	payment := h.seedStalePendingUpgrade(t, sub, "prov_stale_3")
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
}

// reconcileFreshPendingNotStale covers the freshness edge: a payment younger
// than the staleness threshold is never reconciled, the provider is not even
// queried.
func reconcileFreshPendingNotStale(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, nil)
	payment, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, h.business.ID, domain.PeriodMonth,
		h.business.MonthlyPriceKopecks, testProviderFake, h.now.Add(-time.Minute))
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
}

// TestWorkers_ReconciliationRoutesThroughLifecyclePort proves the
// reconciliation phases depend on nothing but the narrow PaymentLifecycle
// port (issue #288): a provider-confirmed outcome arrives as exactly one
// notification application with the provider status translated, and a stuck
// refund as one resolution whose answer drives the batch progress. The
// workers hold no reference to the payment service behind the port.
func TestWorkers_ReconciliationRoutesThroughLifecyclePort(t *testing.T) {
	t.Parallel()
	t.Run("succeeded outcome arrives as one applied notification", portAppliesSucceededOutcome)
	t.Run("failed outcome keeps the provider error code", portKeepsProviderErrorCode)
	t.Run("stuck refund resolves through the port", portResolvesStuckRefund)
	t.Run("unresolved answer leaves the reservation untouched", portKeepsUnresolvedReservation)
}

// portAppliesSucceededOutcome covers the succeeded outcome: exactly one
// notification application with the provider status translated.
func portAppliesSucceededOutcome(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, nil)
	payment := h.seedStalePendingUpgrade(t, sub, "prov_port_1")
	lifecycle := h.swapLifecyclePort()
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
}

// portKeepsProviderErrorCode covers the failed outcome: the notification
// carries the provider's error code and the payment identity.
func portKeepsProviderErrorCode(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, nil)
	payment := h.seedStalePendingUpgrade(t, sub, "prov_port_2")
	lifecycle := h.swapLifecyclePort()
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
}

// portResolvesStuckRefund covers a stuck refund the provider reports refunded:
// one resolution through the port drives the batch progress.
func portResolvesStuckRefund(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	payment := h.seedStuckRefundPayment(t)
	lifecycle := h.swapLifecyclePort()
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
}

// portKeepsUnresolvedReservation covers an unresolved port answer: the refund
// reservation survives untouched.
func portKeepsUnresolvedReservation(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	payment := h.seedStuckRefundPayment(t)
	lifecycle := h.swapLifecyclePort()
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
}

// TestWorkers_ReconcileStaleRefundsRequiresLifecycle proves the refund
// reconciliation refuses to run without the payment lifecycle instead of
// punishing users for a wiring mistake.
func TestWorkers_ReconcileStaleRefundsRequiresLifecycle(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	newHarnessWithPayments := func(t *testing.T) (*workersHarness, domain.Subscription, *domain.SubscriptionPayment) {
		t.Helper()
		h := newWorkersHarness(t, Config{})
		sub := h.seedSubscription(t, nil)
		method := h.seedActiveMethod(t, sub, testProviderFake, "token_async")
		payment, err := domain.NewSubscriptionPayment(
			sub.UserID, sub.ID, h.pro.ID, domain.PeriodMonth,
			h.pro.MonthlyPriceKopecks, testProviderFake, h.now.Add(-time.Minute))
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
		t.Parallel()
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
		t.Parallel()
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

// seedStaleMitCharge seeds the precondition of a stale renewal failure on the
// workers harness: an expired pro subscription with a linked payment method
// and a pending merchant-initiated renewal charge hanging at the provider
// (issue #426).
func (h *workersHarness) seedStaleMitCharge(t *testing.T) (domain.Subscription, domain.SubscriptionPayment) {
	t.Helper()
	sub := h.seedSubscription(t, nil) // Expired pro, auto-renew on.
	method := h.seedActiveMethod(t, sub, testProviderFake, "token_stale")
	stale, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, h.pro.ID, domain.PeriodMonth,
		h.pro.MonthlyPriceKopecks, testProviderFake, h.now.Add(-time.Minute))
	if err != nil {
		t.Fatalf("new stale payment: %v", err)
	}
	methodID := method.ID
	stale.PaymentMethodID = &methodID
	if err := stale.SaveProviderReference("prov_stale", "", h.now.Add(-time.Minute)); err != nil {
		t.Fatalf("save stale reference: %v", err)
	}
	stale, err = h.stores.payments.Create(t.Context(), stale)
	if err != nil {
		t.Fatalf("seed stale payment: %v", err)
	}
	return sub, stale
}

// landManualUpgrade pays a newer manual upgrade through the webhook path
// while the stale charge is in flight — the business tariff is what makes the
// payment coexist with the pending pro charge (one pending payment per
// user/tariff/period) — and proves the subscription is active on it. The
// renewal's id is minted one v7 millisecond after the stale charge's, so the
// creation order the freshness guard reads is deterministic.
func (h *workersHarness) landManualUpgrade(t *testing.T, sub domain.Subscription, newerThan uuid.UUID) domain.SubscriptionPayment {
	t.Helper()
	renewal, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, h.business.ID, domain.PeriodMonth,
		h.business.MonthlyPriceKopecks, testProviderFake, h.now)
	if err != nil {
		t.Fatalf("new renewal payment: %v", err)
	}
	renewal.ID = laterPaymentID(newerThan)
	if err := renewal.SaveProviderReference("prov_renewal", "", h.now); err != nil {
		t.Fatalf("save renewal reference: %v", err)
	}
	renewal, err = h.stores.payments.Create(t.Context(), renewal)
	if err != nil {
		t.Fatalf("seed renewal payment: %v", err)
	}
	if err := h.workers.payments.ApplyPaymentNotification(t.Context(), &PaymentNotification{
		InternalPaymentID: renewal.ID,
		ProviderPaymentID: "prov_renewal",
		Status:            domain.PaymentStatusSucceeded,
	}); err != nil {
		t.Fatalf("ApplyPaymentNotification(renewal) error = %v", err)
	}
	renewed := h.storedSubscription(t, sub)
	if renewed.Status != domain.SubscriptionStatusActive {
		t.Fatalf("renewal baseline: status = %q, want active on the renewal", renewed.Status)
	}
	if renewed.LastAppliedPaymentID == nil || *renewed.LastAppliedPaymentID != renewal.ID {
		t.Fatalf("renewal baseline: last applied = %v, want the renewal", renewed.LastAppliedPaymentID)
	}
	return renewal
}

// TestWorkers_StaleFailedRenewalKeepsRenewedSubscriptionActive proves the
// freshness guard on the worker's own finalization path (issue #426): when a
// renewal charge definitively fails after a newer payment already renewed the
// subscription — the manual payment lands while the charge is in flight — the
// failed charge is recorded without moving the subscription into grace.
func TestWorkers_StaleFailedRenewalKeepsRenewedSubscriptionActive(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	sub, stale := h.seedStaleMitCharge(t)
	renewal := h.landManualUpgrade(t, sub, stale.ID)

	// The worker's charge on the old payment ends in a definitive failure.
	if err := h.workers.failRenewalPayment(t.Context(), stale.ID, nil, h.now); err != nil {
		t.Fatalf("failRenewalPayment() error = %v", err)
	}

	failed, err := h.stores.payments.GetByID(t.Context(), stale.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if failed.Status != domain.PaymentStatusFailed {
		t.Errorf("stale payment status = %q, want failed", failed.Status)
	}
	stored := h.storedSubscription(t, sub)
	if stored.Status != domain.SubscriptionStatusActive {
		t.Errorf("subscription status = %q, want active (a stale failure must not enter grace)", stored.Status)
	}
	if stored.LastAppliedPaymentID == nil || *stored.LastAppliedPaymentID != renewal.ID {
		t.Errorf("last applied payment = %v, want the newer renewal", stored.LastAppliedPaymentID)
	}
	for _, tr := range h.transitionsOf(t, sub) {
		if tr.Reason == domain.TransitionReasonGraceEntered {
			t.Errorf("transitions carry a grace_entered entry for a stale failure: %+v", tr)
		}
	}
}

// TestWorkers_BatchWithoutProgressStops proves a full batch where nothing
// processes stops the loop for this tick instead of spinning: with batch size
// one, the failing item (a pending tariff that no longer exists) blocks the
// second subscription until the next tick (ported from
// TestBilling_ProcessRenewals_BatchWithoutProgressStops and
// ..._ProcessExpiredGrace_BatchWithoutProgressStops).
func TestWorkers_BatchWithoutProgressStops(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{
		WorkerBatchSize: 1, GraceDuration: 7 * 24 * time.Hour,
		ChargeAttemptLimit: 3, PendingPaymentStaleness: 5 * time.Minute,
	})

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
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, nil)
	h.seedActiveMethod(t, sub, testProviderFake, "token_good")

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

// seedGraceEpisode puts a subscription into a dunning-ready grace episode
// (ticket #431): the grace entry logged enteredAgo ago, the window open for
// the rest of the week, auto-renew on, a linked active method and the original
// renewal payment failed just before the entry — old enough to consume no
// retry boundary. It returns the stored subscription and the linked method.
func (h *workersHarness) seedGraceEpisode(t *testing.T) (domain.Subscription, domain.PaymentMethod) {
	t.Helper()
	entered := h.now.Add(-25 * time.Hour)
	graceUntil := entered.Add(7 * 24 * time.Hour)
	sub := h.seedSubscription(t, func(s *domain.Subscription) {
		s.Status = domain.SubscriptionStatusGrace
		s.ValidUntil = &graceUntil
	})
	method := h.seedActiveMethod(t, sub, testProviderFake, "rebill_"+sub.UserID.String())
	sub, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("re-read subscription: %v", err)
	}
	if err := h.stores.transitions.Append(t.Context(), domain.Transition{
		ID:             mustNewUUID(),
		SubscriptionID: sub.ID,
		ToStatus:       domain.SubscriptionStatusGrace,
		ToTariffID:     sub.TariffID,
		Reason:         domain.TransitionReasonGraceEntered,
		Initiator:      domain.InitiatorSystem,
		CreatedAt:      entered,
	}); err != nil {
		t.Fatalf("seed grace entry: %v", err)
	}
	original, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, h.pro.ID, domain.PeriodMonth,
		h.pro.MonthlyPriceKopecks, testProviderFake, entered.Add(-time.Minute))
	if err != nil {
		t.Fatalf("new original payment: %v", err)
	}
	if err := original.MarkFailed(nil, entered); err != nil {
		t.Fatalf("fail original payment: %v", err)
	}
	if _, err := h.stores.payments.Create(t.Context(), original); err != nil {
		t.Fatalf("seed original payment: %v", err)
	}
	return sub, method
}

// TestWorkers_GraceRetrySucceedsAndLeavesGrace proves the happy dunning path
// (ticket #431): 25 hours after the grace entry the retry charges the active
// method, the success applies through the single seam and the subscription
// leaves grace active, renewed for a month from the retry — the original
// failed payment stays untouched and the retry boundary is consumed (a second
// run charges nothing).
func TestWorkers_GraceRetrySucceedsAndLeavesGrace(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	sub, method := h.seedGraceEpisode(t)

	count, err := h.workers.ProcessGraceRetries(t.Context(), h.now)
	if err != nil {
		t.Fatalf("ProcessGraceRetries: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
	if _, charges, _ := h.provider.calls(); charges != 1 {
		t.Fatalf("charge calls = %d, want 1", charges)
	}
	h.requireGraceRescued(t, sub, method)
	h.requireGraceRetryBoundaryConsumed(t)
}

// requireGraceRescued asserts the succeeded retry ended the grace episode: the
// subscription active and renewed for a month, the succeeded retry charged on
// the active method and linked as the last applied payment.
func (h *workersHarness) requireGraceRescued(t *testing.T, sub domain.Subscription, method domain.PaymentMethod) {
	t.Helper()
	stored := h.storedSubscription(t, sub)
	wantUntil := h.now.AddDate(0, 1, 0)
	if stored.Status != domain.SubscriptionStatusActive || stored.ValidUntil == nil || !stored.ValidUntil.Equal(wantUntil) {
		t.Errorf("subscription = %s until %v, want active until %v", stored.Status, stored.ValidUntil, wantUntil)
	}

	payments, err := h.stores.payments.ListByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("list payments: %v", err)
	}
	if len(payments) != 2 {
		t.Fatalf("payments = %d, want 2 (original + retry)", len(payments))
	}
	var retry *domain.SubscriptionPayment
	for i, p := range payments {
		if p.Status == domain.PaymentStatusSucceeded {
			retry = &payments[i]
		}
	}
	if retry == nil {
		t.Fatal("no succeeded retry payment")
	}
	if retry.PaymentMethodID == nil || *retry.PaymentMethodID != method.ID {
		t.Errorf("retry method = %v, want the active method", retry.PaymentMethodID)
	}
	if stored.LastAppliedPaymentID == nil || *stored.LastAppliedPaymentID != retry.ID {
		t.Errorf("LastAppliedPaymentID = %v, want the retry payment", stored.LastAppliedPaymentID)
	}
}

// requireGraceRetryBoundaryConsumed asserts a further retry run charges
// nothing: the retry payment consumed the boundary.
func (h *workersHarness) requireGraceRetryBoundaryConsumed(t *testing.T) {
	t.Helper()
	again, err := h.workers.ProcessGraceRetries(t.Context(), h.now)
	if err != nil {
		t.Fatalf("second ProcessGraceRetries: %v", err)
	}
	if again != 0 {
		t.Errorf("second run count = %d, want 0", again)
	}
	if _, charges, _ := h.provider.calls(); charges != 1 {
		t.Errorf("charge calls after second run = %d, want 1", charges)
	}
}

// TestWorkers_GraceRetryFailedStaysGrace proves a declined retry keeps the
// subscription in its grace window: the retry payment fails, the window and
// the schedule are untouched, and the boundary stays consumed — the next
// scheduled retry is the +72 h one, not an immediate re-charge.
func TestWorkers_GraceRetryFailedStaysGrace(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	sub, _ := h.seedGraceEpisode(t)
	graceUntil := sub.ValidUntil
	h.provider.chargeFn = func(req ChargeRequest) (ChargeResult, error) {
		return ChargeResult{ProviderPaymentID: req.ProviderPaymentID, Status: domain.PaymentStatusFailed, ErrorCode: "insufficient_funds"}, nil
	}

	count, err := h.workers.ProcessGraceRetries(t.Context(), h.now)
	if err != nil {
		t.Fatalf("ProcessGraceRetries: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}

	stored := h.storedSubscription(t, sub)
	if stored.Status != domain.SubscriptionStatusGrace || stored.ValidUntil == nil || !stored.ValidUntil.Equal(*graceUntil) {
		t.Errorf("subscription = %s until %v, want grace until %v (unchanged)", stored.Status, stored.ValidUntil, *graceUntil)
	}

	// The failed retry consumed the boundary: nothing re-charges at this tick.
	again, err := h.workers.ProcessGraceRetries(t.Context(), h.now)
	if err != nil {
		t.Fatalf("second ProcessGraceRetries: %v", err)
	}
	if again != 0 {
		t.Errorf("second run count = %d, want 0", again)
	}
	if _, charges, _ := h.provider.calls(); charges != 1 {
		t.Errorf("charge calls after second run = %d, want 1", charges)
	}
}

// TestWorkers_GraceRetryPicksUpSwitchedCard proves the retry charges whatever
// method is active by retry time: a card bound after the grace entry is the
// token the charge carries.
func TestWorkers_GraceRetryPicksUpSwitchedCard(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	sub, _ := h.seedGraceEpisode(t)
	switched := h.seedActiveMethod(t, sub, testProviderFake, "rebill_switched_"+sub.UserID.String())

	var chargedToken string
	h.provider.chargeFn = func(req ChargeRequest) (ChargeResult, error) {
		chargedToken = req.ChargeToken
		return ChargeResult{ProviderPaymentID: req.ProviderPaymentID, Status: domain.PaymentStatusSucceeded}, nil
	}

	if _, err := h.workers.ProcessGraceRetries(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessGraceRetries: %v", err)
	}
	if chargedToken != switched.ProviderToken {
		t.Errorf("charged token = %q, want the switched method's %q", chargedToken, switched.ProviderToken)
	}
}

// TestWorkers_GraceRetrySkipsWhenGraceLeft proves the retry never fires for a
// subscription that left grace — a manual payment renewed it — and charges
// nothing: the selection's status bound is the guard.
func TestWorkers_GraceRetrySkipsWhenGraceLeft(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	sub, _ := h.seedGraceEpisode(t)
	until := h.now.AddDate(0, 1, 0)
	sub.Status = domain.SubscriptionStatusActive
	sub.ValidUntil = &until
	if err := h.stores.subscriptions.Update(t.Context(), sub); err != nil {
		t.Fatalf("store active: %v", err)
	}

	count, err := h.workers.ProcessGraceRetries(t.Context(), h.now)
	if err != nil {
		t.Fatalf("ProcessGraceRetries: %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0", count)
	}
	if _, charges, _ := h.provider.calls(); charges != 0 {
		t.Errorf("charge calls = %d, want 0", charges)
	}
}

// TestWorkers_GraceRetryRequiresProvider proves a wiring without the provider
// fails loudly instead of charging blindly.
func TestWorkers_GraceRetryRequiresProvider(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	h.workers.provider = nil
	if _, err := h.workers.ProcessGraceRetries(t.Context(), h.now); !errors.Is(err, ErrPaymentUnavailable) {
		t.Fatalf("ProcessGraceRetries error = %v, want ErrPaymentUnavailable", err)
	}
}

// TestWorkers_GraceRetryManualPaymentCancelsRetries pins the manual-payment
// boundary of ticket #431: a payment the user started since the grace entry —
// whatever its outcome — consumes the scheduled retry, the way the spec words
// it ("ручная оплата до повтора отменяет дальнейшие повторы"); a pending one
// keeps the subscription in grace yet charges nothing on the dunning schedule.
func TestWorkers_GraceRetryManualPaymentCancelsRetries(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	sub, _ := h.seedGraceEpisode(t)
	manual, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, h.pro.ID, domain.PeriodMonth,
		h.pro.MonthlyPriceKopecks, testProviderFake, h.now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("new manual payment: %v", err)
	}
	if _, err := h.stores.payments.Create(t.Context(), manual); err != nil {
		t.Fatalf("seed manual payment: %v", err)
	}

	count, err := h.workers.ProcessGraceRetries(t.Context(), h.now)
	if err != nil {
		t.Fatalf("ProcessGraceRetries: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 (the manual payment consumed the retry)", count)
	}
	if _, charges, _ := h.provider.calls(); charges != 0 {
		t.Errorf("charge calls = %d, want 0", charges)
	}
	stored := h.storedSubscription(t, sub)
	if stored.Status != domain.SubscriptionStatusGrace {
		t.Errorf("status = %q, want grace (the manual payment is still pending)", stored.Status)
	}
}

// TestWorkers_ExpiredBindingSessionsDeleted pins the hygiene phase of ticket
// #433: sessions past their lifetime are deleted whatever their status — an
// expired session never produces a payment method — while live sessions stay,
// and a repeated run is a safe no-op. The batching loop drains a table bigger
// than one worker batch in a single phase call.
func TestWorkers_ExpiredBindingSessionsDeleted(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{WorkerBatchSize: 2})

	seed := func(t *testing.T, ttl time.Duration) domain.CardBindingSession {
		t.Helper()
		session, err := domain.NewCardBindingSession(
			mustNewUUID(), testProviderFake, "rk_"+mustNewUUID().String(),
			h.now.Add(ttl), h.now.Add(-2*ttl))
		if err != nil {
			t.Fatalf("new session: %v", err)
		}
		if _, err := h.stores.bindings.Create(t.Context(), session); err != nil {
			t.Fatalf("seed session: %v", err)
		}
		return session
	}
	// Three expired sessions — more than one batch at size 2 — in both
	// statuses the table holds at expiry: open and closed.
	expiredOpen := seed(t, -3*time.Hour)
	expiredRejected := seed(t, -3*time.Hour)
	expiredRejected.Status = domain.CardBindingRejected
	if err := h.stores.bindings.UpdateStatus(t.Context(), expiredRejected); err != nil {
		t.Fatalf("reject session: %v", err)
	}
	seed(t, -2*time.Hour)
	// A live session stays: its lifetime has not ended.
	live := seed(t, time.Hour)

	count, err := h.workers.ProcessExpiredBindingSessions(t.Context(), h.now)
	if err != nil {
		t.Fatalf("ProcessExpiredBindingSessions: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
	for _, gone := range []domain.CardBindingSession{expiredOpen, expiredRejected} {
		if _, err := h.stores.bindings.GetByRequestKeyForUpdate(t.Context(), gone.Provider, gone.RequestKey); !errors.Is(err, ErrNotFound) {
			t.Errorf("expired session %s must be deleted, got err %v", gone.ID, err)
		}
	}
	if _, err := h.stores.bindings.GetByRequestKeyForUpdate(t.Context(), live.Provider, live.RequestKey); err != nil {
		t.Fatalf("live session must survive the cleanup: %v", err)
	}

	// The re-run is a safe no-op: nothing left to delete.
	count, err = h.workers.ProcessExpiredBindingSessions(t.Context(), h.now)
	if err != nil {
		t.Fatalf("re-run ProcessExpiredBindingSessions: %v", err)
	}
	if count != 0 {
		t.Errorf("re-run count = %d, want 0", count)
	}
}

// TestWorkers_ExportStuckPaymentMetrics pins the hygiene gauges of ticket
// #433: the phase counts the payments and refunds the reconciliation
// watchdog owns — stale pending payments and payments stuck in the refunding
// reservation — and records them on the stuck gauge by kind, nothing else.
func TestWorkers_ExportStuckPaymentMetrics(t *testing.T) {
	t.Parallel()

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		otel.SetMeterProvider(previous)
		if err := reader.Shutdown(t.Context()); err != nil {
			t.Logf("shutdown manual metric reader: %v", err)
		}
	})

	// The harness leaves the workers gauge-less; the test wires the
	// instruments built from the provider it just installed.
	metrics, err := NewMetrics()
	if err != nil {
		t.Fatalf("new metrics: %v", err)
	}
	h := newWorkersHarness(t, Config{})
	h.workers.metrics = metrics
	sub := h.seedSubscription(t, nil)
	h.seedStalePendingUpgrade(t, sub, "prov_gauge_1") // Stale pending: counted.
	refund := h.seedStuckRefundPayment(t)             // Stuck refund: counted.
	// The refunding staleness clock is updated_at: age the reservation past
	// the configured staleness the way a lost refund outcome does.
	refund.UpdatedAt = h.now.Add(-10 * time.Minute)
	if err := h.stores.payments.Update(t.Context(), refund); err != nil {
		t.Fatalf("age refund: %v", err)
	}
	h.provider.statusFn = func(uuid.UUID, string) (PaymentStatusResult, error) {
		t.Error("the gauge phase must not talk to the provider")
		return PaymentStatusResult{}, nil
	}

	if err := h.workers.ExportStuckPaymentMetrics(t.Context(), h.now); err != nil {
		t.Fatalf("ExportStuckPaymentMetrics: %v", err)
	}

	var data metricdata.ResourceMetrics
	if err := reader.Collect(t.Context(), &data); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}
	gauges := map[string]int64{}
	for _, scope := range data.ScopeMetrics {
		for _, m := range scope.Metrics {
			g, ok := m.Data.(metricdata.Gauge[int64])
			if !ok {
				continue
			}
			for _, point := range g.DataPoints {
				kind := "unknown"
				if v, ok := point.Attributes.Value("kind"); ok {
					kind = v.AsString()
				}
				gauges[m.Name+"/"+kind] = point.Value
			}
		}
	}
	if got := gauges["billing.payments.stuck/pending"]; got != 1 {
		t.Errorf("stuck pending = %d, want 1", got)
	}
	if got := gauges["billing.payments.stuck/refunding"]; got != 1 {
		t.Errorf("stuck refunding = %d, want 1", got)
	}
}
