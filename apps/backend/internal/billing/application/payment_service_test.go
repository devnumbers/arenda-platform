package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// testProviderFake is the provider identity shared by the application tests.
const testProviderFake = "fake"

// testErrCodeDeclined is the shared declined-charge error-code fixture.
const testErrCodeDeclined = "card_declined"

// stubPaymentProvider is a configurable provider for the payment-flow tests:
// it satisfies both consumer-side provider slices (initiation for
// SubscriptionService, finalization for PaymentService) and the fake
// confirmation hook, with injectable outcomes per capability.
type stubPaymentProvider struct {
	mu sync.Mutex

	initCalls int
	initReqs  []InitPaymentRequest
	initErr   error
	initFn    func(req InitPaymentRequest) (InitPaymentResult, error)

	statusCalls int
	statusRes   PaymentStatusResult
	statusErr   error

	refundCalls int
	refundReqs  []RefundRequest
	// RefundRes is answered for every refund request unless refundFn is set;
	// the zero value means "full refund confirmed" — the default a captured
	// charge produces.
	refundRes RefundResult
	refundErr error
	refundFn  func(req RefundRequest) (RefundResult, error)

	parseEvent WebhookEvent
	parseErr   error
}

func (p *stubPaymentProvider) Name() domain.PaymentProvider { return testProviderFake }

func (p *stubPaymentProvider) InitPayment(_ context.Context, req InitPaymentRequest) (InitPaymentResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.initCalls++
	p.initReqs = append(p.initReqs, req)
	if p.initFn != nil {
		return p.initFn(req)
	}
	if p.initErr != nil {
		return InitPaymentResult{}, p.initErr
	}
	return InitPaymentResult{
		ProviderPaymentID: "stub_" + req.PaymentID.String(),
		PaymentURL:        "https://pay.example/" + req.PaymentID.String(),
		Status:            domain.PaymentStatusPending,
	}, nil
}

func (p *stubPaymentProvider) PaymentStatus(_ context.Context, _ uuid.UUID, _ string) (PaymentStatusResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.statusCalls++
	return p.statusRes, p.statusErr
}

// RefundPayment records the request and answers the programmed outcome: the
// injected function first, then the error, and by default a confirmed full
// refund of the requested amount.
func (p *stubPaymentProvider) RefundPayment(_ context.Context, req RefundRequest) (RefundResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.refundCalls++
	p.refundReqs = append(p.refundReqs, req)
	if p.refundFn != nil {
		return p.refundFn(req)
	}
	if p.refundErr != nil {
		return RefundResult{}, p.refundErr
	}
	if p.refundRes != (RefundResult{}) {
		return p.refundRes, nil
	}
	return RefundResult{
		ProviderPaymentID:     req.ProviderPaymentID,
		Status:                domain.PaymentStatusRefunded,
		RefundedAmountKopecks: req.AmountKopecks,
	}, nil
}

func (p *stubPaymentProvider) ParseWebhook(_ context.Context, _ []byte) (WebhookEvent, error) {
	return p.parseEvent, p.parseErr
}

func (p *stubPaymentProvider) WebhookAck() []byte { return []byte(`{"status":"ok"}`) }

// Compile-time checks against both consumer-side provider slices.
var (
	_ paymentInitiationProvider = (*stubPaymentProvider)(nil)
	_ paymentFinalizerProvider  = (*stubPaymentProvider)(nil)
)

// paymentHarness wires the subscription and payment services over the
// in-memory stores, a capture audit recorder, a fixed clock and the stub
// provider (issue #250 payment flows).
type paymentHarness struct {
	subs     *SubscriptionService
	payments *PaymentService
	provider *stubPaymentProvider
	stores   *fakeStores
	audit    *captureRecorder
	now      time.Time
	tariffs  []domain.Tariff
}

func newPaymentHarness(t *testing.T) *paymentHarness {
	t.Helper()
	tariffs := testTariffs()
	stores := newFakeStores(tariffs...)
	audit := &captureRecorder{}
	now := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	factory := stores.factory(audit)
	provider := &stubPaymentProvider{}
	return &paymentHarness{
		subs: NewSubscriptionService(factory, SubscriptionServiceConfig{
			Clock:    fakeClock{now: now},
			Provider: provider,
			Config:   DefaultConfig(),
		}),
		payments: NewPaymentService(factory, provider, PaymentServiceConfig{Clock: fakeClock{now: now}}),
		provider: provider,
		stores:   stores,
		audit:    audit,
		now:      now,
		tariffs:  tariffs,
	}
}

func (h *paymentHarness) tariffID(t *testing.T, name domain.TariffName) uuid.UUID {
	t.Helper()
	for _, tariff := range h.tariffs {
		if tariff.Name == name {
			return tariff.ID
		}
	}
	t.Fatalf("harness tariffs contain no %q plan", name)
	return uuid.Nil
}

// seedSubscription creates a paid pro subscription with a month of paid
// validity (the payment scenarios upgrade from pro to business) and returns
// it, applying mutate last.
func (h *paymentHarness) seedSubscription(t *testing.T, mutate func(*domain.Subscription)) domain.Subscription {
	t.Helper()
	sub, err := domain.NewBasicSubscription(uuid.Must(uuid.NewV7()), h.tariffID(t, domain.TariffPro))
	if err != nil {
		t.Fatalf("NewBasicSubscription() error = %v", err)
	}
	validUntil := h.now.AddDate(0, 1, 0)
	period := domain.PeriodMonth
	sub.TariffID = h.tariffID(t, domain.TariffPro)
	sub.ValidUntil = &validUntil
	sub.AutoRenewEnabled = true
	sub.CurrentPeriod = &period
	if mutate != nil {
		mutate(&sub)
	}
	if _, err := h.stores.subscriptions.Create(t.Context(), sub); err != nil {
		t.Fatalf("seed Create() error = %v", err)
	}
	return sub
}

// initiateUpgrade runs ChangeTariff to the business plan and returns the
// result, failing the test on anything but success.
func (h *paymentHarness) initiateUpgrade(t *testing.T, sub domain.Subscription) ChangeTariffResult {
	t.Helper()
	result, err := h.subs.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffBusiness,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff(upgrade) error = %v", err)
	}
	return result
}

// notification prepares the provider stub to parse into a payment
// notification and returns it.
func (h *paymentHarness) setNotification(n *PaymentNotification) {
	h.provider.parseEvent = WebhookEvent{Payment: n}
}

// transitionCount returns how many transitions were appended for the
// subscription.
func (h *paymentHarness) transitionCount(t *testing.T, subscriptionID uuid.UUID) int {
	t.Helper()
	transitions, err := h.stores.transitions.ListBySubscriptionID(t.Context(), subscriptionID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID() error = %v", err)
	}
	return len(transitions)
}

// paymentPersistedBeforeInit is the injected InitPayment hook proving the
// pending payment row is committed before the provider call: it fails the
// test when the record is missing, not pending, or already referenced.
func (h *paymentHarness) paymentPersistedBeforeInit(
	t *testing.T, req InitPaymentRequest,
) (InitPaymentResult, error) {
	t.Helper()
	payment, err := h.stores.payments.GetByID(t.Context(), req.PaymentID)
	if err != nil {
		t.Errorf("provider.Init called before the payment record was committed: %v", err)
		return InitPaymentResult{}, errors.New("payment not persisted")
	}
	if payment.Status != domain.PaymentStatusPending {
		t.Errorf("payment status before init = %q, want pending", payment.Status)
	}
	if payment.HasProviderReference() {
		t.Error("payment must not carry a provider reference before init")
	}
	return InitPaymentResult{
		ProviderPaymentID: "stub_" + req.PaymentID.String(),
		PaymentURL:        "https://pay.example/" + req.PaymentID.String(),
		Status:            domain.PaymentStatusPending,
	}, nil
}

// requireUpgradedPaymentPersisted loads the started upgrade payment and proves
// the init result was saved atomically with the full business-month price.
func (h *paymentHarness) requireUpgradedPaymentPersisted(
	t *testing.T, result ChangeTariffResult,
) domain.SubscriptionPayment {
	t.Helper()
	payment, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if !payment.HasProviderReference() || *payment.ProviderPaymentID != "stub_"+result.PaymentID.String() {
		t.Errorf("provider payment id = %v, want the init result persisted", payment.ProviderPaymentID)
	}
	if !payment.HasPaymentURL() || *payment.PaymentURL != result.ConfirmURL {
		t.Errorf("payment url = %v, want %q", payment.PaymentURL, result.ConfirmURL)
	}
	if payment.TariffID != h.tariffID(t, domain.TariffBusiness) || payment.AmountKopecks != 99000 {
		t.Errorf("payment = tariff %v amount %d, want business/99000 (full price of the new plan)", payment.TariffID, payment.AmountKopecks)
	}
	if payment.Period != domain.PeriodMonth {
		t.Errorf("period = %q, want month", payment.Period)
	}
	return payment
}

// seedSucceededUpgrade starts the business upgrade for the subscription and
// finalizes it through a succeeded webhook, returning the payment.
func (h *paymentHarness) seedSucceededUpgrade(t *testing.T, sub domain.Subscription) domain.SubscriptionPayment {
	t.Helper()
	result := h.initiateUpgrade(t, sub)
	payment, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if err := h.webhookSucceeded(t, payment); err != nil {
		t.Fatalf("HandleWebhook() error = %v", err)
	}
	return payment
}

// deliverRefundNotification feeds the payment a refunded notification through
// the webhook path.
func (h *paymentHarness) deliverRefundNotification(t *testing.T, paymentID uuid.UUID) {
	t.Helper()
	h.setNotification(&PaymentNotification{
		InternalPaymentID: paymentID,
		ProviderPaymentID: "stub_" + paymentID.String(),
		Status:            domain.PaymentStatusRefunded,
	})
	if err := h.payments.HandleWebhook(t.Context(), testProviderFake, []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook() error = %v", err)
	}
}

// lastRefundTransition finds the refunded transition of the subscription,
// failing when the log contains none.
func (h *paymentHarness) lastRefundTransition(t *testing.T, subscriptionID uuid.UUID) domain.Transition {
	t.Helper()
	transitions, err := h.stores.transitions.ListBySubscriptionID(t.Context(), subscriptionID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID() error = %v", err)
	}
	var refundTransition *domain.Transition
	for i := range transitions {
		if transitions[i].Reason == domain.TransitionReasonRefunded {
			refundTransition = &transitions[i]
		}
	}
	if refundTransition == nil {
		t.Fatalf("transitions contain no refunded entry: %+v", transitions)
	}
	return *refundTransition
}

// requireUpgradedSubscription proves the subscription shape after a succeeded
// upgrade payment: business tariff, active, the period from the payment
// moment, auto-renew on, and the payment recorded as last applied.
func (h *paymentHarness) requireUpgradedSubscription(t *testing.T, userID, paymentID uuid.UUID) {
	t.Helper()
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.TariffID != h.tariffID(t, domain.TariffBusiness) {
		t.Errorf("tariff = %v, want business", stored.TariffID)
	}
	if stored.Status != domain.SubscriptionStatusActive {
		t.Errorf("status = %q, want active", stored.Status)
	}
	wantUntil := h.now.AddDate(0, 1, 0)
	if stored.ValidUntil == nil || !stored.ValidUntil.Equal(wantUntil) {
		t.Errorf("valid until = %v, want %v (the period from the payment moment)", stored.ValidUntil, wantUntil)
	}
	if !stored.AutoRenewEnabled {
		t.Error("auto-renew = false, want true")
	}
	if stored.LastAppliedPaymentID == nil || *stored.LastAppliedPaymentID != paymentID {
		t.Errorf("last applied payment = %v, want %v", stored.LastAppliedPaymentID, paymentID)
	}
	if stored.CurrentPeriod == nil || *stored.CurrentPeriod != domain.PeriodMonth {
		t.Errorf("current period = %v, want month", stored.CurrentPeriod)
	}
}

// requireFinalizedSucceeded proves the payment is stored succeeded with its
// success timestamp.
func (h *paymentHarness) requireFinalizedSucceeded(t *testing.T, paymentID uuid.UUID) {
	t.Helper()
	finalized, err := h.stores.payments.GetByID(t.Context(), paymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if finalized.Status != domain.PaymentStatusSucceeded || finalized.SucceededAt == nil {
		t.Errorf("payment = %q/%v, want succeeded with a timestamp", finalized.Status, finalized.SucceededAt)
	}
}

// requireAppliedTransition proves the single transition is the payment-applied
// entry of the pro→business upgrade with the system initiator.
func (h *paymentHarness) requireAppliedTransition(
	t *testing.T, sub domain.Subscription, paymentID uuid.UUID,
) domain.Transition {
	t.Helper()
	transitions, err := h.stores.transitions.ListBySubscriptionID(t.Context(), sub.ID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID() error = %v", err)
	}
	if len(transitions) != 1 {
		t.Fatalf("transitions = %d, want 1", len(transitions))
	}
	tr := transitions[0]
	if tr.Reason != domain.TransitionReasonPaymentApplied {
		t.Errorf("transition reason = %q, want payment_applied", tr.Reason)
	}
	if tr.PaymentID == nil || *tr.PaymentID != paymentID {
		t.Errorf("transition payment = %v, want %v", tr.PaymentID, paymentID)
	}
	if tr.FromTariffID == nil || *tr.FromTariffID != sub.TariffID || tr.ToTariffID != h.tariffID(t, domain.TariffBusiness) {
		t.Errorf("transition tariffs = %v→%v, want pro→business", tr.FromTariffID, tr.ToTariffID)
	}
	if tr.Initiator != domain.InitiatorSystem {
		t.Errorf("transition initiator = %q, want system", tr.Initiator)
	}
	return tr
}

// TestChangeTariff_UpgradeCreatesPendingPaymentBeforeProviderCall proves the
// crash-safety ordering of the payment flow (issue #250): the pending payment
// row is committed before the provider is called, so a crash between the two
// leaves a recoverable state instead of a payment the platform does not know
// about. Migrated from the pre-rewrite test
// TestBilling_ChangeTariff_UpgradeCreatesPaymentBeforeProviderCall.
func TestChangeTariff_UpgradeCreatesPendingPaymentBeforeProviderCall(t *testing.T) {
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)

	var persistedBeforeInit bool
	h.provider.initFn = func(req InitPaymentRequest) (InitPaymentResult, error) {
		result, err := h.paymentPersistedBeforeInit(t, req)
		if err == nil {
			persistedBeforeInit = true
		}
		return result, err
	}

	result := h.initiateUpgrade(t, sub)

	if !persistedBeforeInit {
		t.Fatal("provider.Init was never called")
	}
	if result.PaymentID == uuid.Nil || result.ConfirmURL == "" {
		t.Fatalf("result = %+v, want payment id and confirm url", result)
	}
	h.requireUpgradedPaymentPersisted(t, result)
	// The subscription stays untouched until the payment succeeds.
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.TariffID != sub.TariffID || stored.HasPendingChange() {
		t.Errorf("subscription changed before payment success: %+v", stored)
	}

	entries := h.audit.recorded()
	if len(entries) != 1 || entries[0].Action != auditdomain.ActionSubscriptionTariffChanged {
		t.Fatalf("audit entries = %+v, want one tariff_changed for the persisted decision", entries)
	}
}

// TestChangeTariff_UpgradeInitRequestCarriesPortContract proves the initiation
// request is provider-neutral and complete: internal payment id, kopeck
// amount, customer reference, structured purpose, CIT initiator, saved-method
// flag and the form deadline from the module config (issue #250).
func TestChangeTariff_UpgradeInitRequestCarriesPortContract(t *testing.T) {
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)

	result := h.initiateUpgrade(t, sub)

	if h.provider.initCalls != 1 {
		t.Fatalf("init calls = %d, want 1", h.provider.initCalls)
	}
	req := h.provider.initReqs[0]
	if req.PaymentID != result.PaymentID {
		t.Errorf("init payment id = %v, want %v", req.PaymentID, result.PaymentID)
	}
	if req.AmountKopecks != 99000 || req.Period != domain.PeriodMonth {
		t.Errorf("init amount/period = %d/%q, want 99000/month", req.AmountKopecks, req.Period)
	}
	if req.CustomerRef != sub.UserID.String() {
		t.Errorf("init customer ref = %q, want the user id", req.CustomerRef)
	}
	if req.Purpose.Kind != PaymentPurposeSubscription || req.Purpose.TariffName != domain.TariffBusiness {
		t.Errorf("init purpose = %+v, want a subscription payment for business", req.Purpose)
	}
	if req.Initiator != InitiatorCustomer || !req.SaveMethod {
		t.Errorf("init initiator/save method = %q/%v, want cit/true (parent payment of the chain)", req.Initiator, req.SaveMethod)
	}
	if !req.FormDeadline.Equal(h.now.Add(DefaultConfig().PaymentFormTTL)) {
		t.Errorf("init form deadline = %v, want now+%v", req.FormDeadline, DefaultConfig().PaymentFormTTL)
	}
}

// TestChangeTariff_UpgradeReturnsExistingPendingPayment proves a repeated
// upgrade request for the same tariff and period returns the existing pending
// payment instead of creating a duplicate (issue #250). Migrated from the
// pre-rewrite test TestBilling_ChangeTariff_UpgradeReturnsExistingPendingPayment.
func TestChangeTariff_UpgradeReturnsExistingPendingPayment(t *testing.T) {
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)

	first := h.initiateUpgrade(t, sub)
	second := h.initiateUpgrade(t, sub)

	if second.PaymentID != first.PaymentID {
		t.Errorf("second payment id = %v, want the existing %v", second.PaymentID, first.PaymentID)
	}
	if second.ConfirmURL != first.ConfirmURL {
		t.Errorf("second confirm url = %q, want %q", second.ConfirmURL, first.ConfirmURL)
	}
	if h.provider.initCalls != 1 {
		t.Errorf("init calls = %d, want 1 (no re-initiation for an existing pending payment)", h.provider.initCalls)
	}
	payments, err := h.stores.payments.ListByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("ListByUserID() error = %v", err)
	}
	if len(payments) != 1 {
		t.Errorf("payments = %d, want 1", len(payments))
	}
}

// TestChangeTariff_UpgradeUniqueRaceReturnsExistingPayment proves the
// pending-payments unique-index race backstop: when the pending lookup misses
// but Create loses the unique race, the existing payment is returned instead
// of failing (issue #250).
func TestChangeTariff_UpgradeUniqueRaceReturnsExistingPayment(t *testing.T) {
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)

	// Simulate a concurrent winner: the pending lookup is blind once, then a
	// conflicting pending payment appears and Create fails on the index.
	existing, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, h.tariffID(t, domain.TariffBusiness),
		domain.PeriodMonth, 99000, testProviderFake, h.now)
	if err != nil {
		t.Fatalf("NewSubscriptionPayment() error = %v", err)
	}
	providerRef := "stub_concurrent"
	url := "https://pay.example/concurrent"
	if err := existing.SaveProviderReference(providerRef, url, h.now); err != nil {
		t.Fatalf("SaveProviderReference() error = %v", err)
	}
	if _, err := h.stores.payments.Create(t.Context(), existing); err != nil {
		t.Fatalf("seed Create() error = %v", err)
	}
	h.stores.payments.hidePending = 1

	result, err := h.subs.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffBusiness,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff() after unique race error = %v", err)
	}
	if result.PaymentID != existing.ID {
		t.Errorf("payment id = %v, want the concurrently created %v", result.PaymentID, existing.ID)
	}
	if result.ConfirmURL != url {
		t.Errorf("confirm url = %q, want the persisted %q", result.ConfirmURL, url)
	}
}

// TestChangeTariff_UpgradeRecoversProviderReferenceAfterCrash proves the
// recovery path for a crash between the provider initiation and the atomic
// save (issue #250): a pending payment without a provider reference is
// re-initiated idempotently and the reference is recovered, keeping the retry
// path idempotent. Migrated from the pre-rewrite test
// TestBilling_ChangeTariff_UpgradeRecoversProviderReferenceAfterCrash.
func TestChangeTariff_UpgradeRecoversProviderReferenceAfterCrash(t *testing.T) {
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)

	// Simulate the crash state: a pending payment exists but has no provider
	// reference.
	lost, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, h.tariffID(t, domain.TariffBusiness),
		domain.PeriodMonth, 99000, testProviderFake, h.now)
	if err != nil {
		t.Fatalf("NewSubscriptionPayment() error = %v", err)
	}
	if _, err := h.stores.payments.Create(t.Context(), lost); err != nil {
		t.Fatalf("seed Create() error = %v", err)
	}

	result := h.initiateUpgrade(t, sub)

	if result.PaymentID != lost.ID {
		t.Errorf("payment id = %v, want the existing %v (recovery, not a duplicate)", result.PaymentID, lost.ID)
	}
	if result.ConfirmURL == "" {
		t.Error("confirm url is empty after recovery")
	}
	if h.provider.initCalls != 1 {
		t.Errorf("init calls = %d, want 1 (the idempotent recovery initiation)", h.provider.initCalls)
	}
	if h.provider.initReqs[0].PaymentID != lost.ID {
		t.Errorf("recovery initiated payment %v, want the existing %v", h.provider.initReqs[0].PaymentID, lost.ID)
	}
	payment, err := h.stores.payments.GetByID(t.Context(), lost.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if !payment.HasProviderReference() {
		t.Error("provider reference not recovered")
	}
	if !payment.HasPaymentURL() {
		t.Error("payment url not recovered")
	}
}

// TestChangeTariff_UpgradeInitFailureMarksPaymentFailed proves a refused
// provider initiation closes the pending payment as failed so the user can
// start over (issue #250). Migrated from the pre-rewrite test
// TestBilling_ChangeTariff_UpgradeInitFailureMarksPaymentFailed.
func TestChangeTariff_UpgradeInitFailureMarksPaymentFailed(t *testing.T) {
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	h.provider.initErr = errors.New("provider is down")

	result, err := h.subs.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffBusiness,
		Period:     domain.PeriodMonth,
	})
	if err == nil {
		t.Fatal("ChangeTariff() error = nil, want the provider failure")
	}
	if result.PaymentID != uuid.Nil {
		t.Errorf("result payment id = %v, want zero on failure", result.PaymentID)
	}
	payments, err := h.stores.payments.ListByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("ListByUserID() error = %v", err)
	}
	if len(payments) != 1 {
		t.Fatalf("payments = %d, want the failed attempt", len(payments))
	}
	if payments[0].Status != domain.PaymentStatusFailed {
		t.Errorf("payment status = %q, want failed", payments[0].Status)
	}
}

// TestChangeTariff_SameTariffInGraceStartsRenewalPayment proves a same-tariff
// request while in grace is a manual renewal through the payment path, and the
// subscription stays in grace until the payment succeeds (issue #250).
// Migrated from the pre-rewrite test
// TestBilling_ChangeTariff_SameTariffInGraceStartsRenewalPayment.
func TestChangeTariff_SameTariffInGraceStartsRenewalPayment(t *testing.T) {
	h := newPaymentHarness(t)
	graceUntil := h.now.Add(48 * time.Hour)
	sub := h.seedSubscription(t, func(s *domain.Subscription) {
		s.Status = domain.SubscriptionStatusGrace
		s.ValidUntil = &graceUntil
	})

	result, err := h.subs.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff() error = %v", err)
	}
	if result.PaymentID == uuid.Nil || result.ConfirmURL == "" {
		t.Fatalf("result = %+v, want a renewal payment", result)
	}
	if h.provider.initCalls != 1 {
		t.Fatalf("init calls = %d, want 1", h.provider.initCalls)
	}
	if h.provider.initReqs[0].Purpose.Kind != PaymentPurposeRenewal {
		t.Errorf("purpose kind = %q, want renewal", h.provider.initReqs[0].Purpose.Kind)
	}
	payment, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if payment.TariffID != h.tariffID(t, domain.TariffPro) || payment.AmountKopecks != 49000 {
		t.Errorf("payment = tariff %v amount %d, want pro/49000", payment.TariffID, payment.AmountKopecks)
	}
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.Status != domain.SubscriptionStatusGrace {
		t.Errorf("status = %q, want grace until the payment succeeds", stored.Status)
	}
}

// TestChangeTariff_ExpiredGraceRejectsPayment proves no payment can be
// initiated after the grace window has expired (the worker downgrade is due).
// Migrated from the pre-rewrite test
// TestBilling_ChangeTariff_SameTariffExpiredGraceRejected.
func TestChangeTariff_ExpiredGraceRejectsPayment(t *testing.T) {
	h := newPaymentHarness(t)
	expired := h.now.Add(-time.Hour)
	sub := h.seedSubscription(t, func(s *domain.Subscription) {
		s.Status = domain.SubscriptionStatusGrace
		s.ValidUntil = &expired
	})

	_, err := h.subs.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffBusiness,
		Period:     domain.PeriodMonth,
	})
	if !errors.Is(err, domain.ErrInvalidSubscriptionState) {
		t.Fatalf("err = %v, want domain.ErrInvalidSubscriptionState", err)
	}
	if h.provider.initCalls != 0 {
		t.Error("provider must not be called for an expired grace subscription")
	}
}

// TestChangeTariff_CancelledUpgradeIsRecovery proves an upgrade on a cancelled
// subscription is the ADR 0008 restoration path: the payment is initiated and
// success reactivates the subscription.
func TestChangeTariff_CancelledUpgradeIsRecovery(t *testing.T) {
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, func(s *domain.Subscription) {
		s.Status = domain.SubscriptionStatusCancelled
		s.AutoRenewEnabled = false
	})

	result := h.initiateUpgrade(t, sub)

	if result.PaymentID == uuid.Nil {
		t.Fatal("recovery upgrade did not start a payment")
	}
	payment, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if payment.Status != domain.PaymentStatusPending {
		t.Errorf("payment status = %q, want pending", payment.Status)
	}
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.Status != domain.SubscriptionStatusCancelled {
		t.Errorf("status = %q, want cancelled until the payment succeeds", stored.Status)
	}
}

// webhookSucceeded prepares the stub provider to deliver a succeeded
// notification for the payment.
func (h *paymentHarness) webhookSucceeded(t *testing.T, payment domain.SubscriptionPayment) error {
	t.Helper()
	h.setNotification(&PaymentNotification{
		InternalPaymentID: payment.ID,
		ProviderPaymentID: *payment.ProviderPaymentID,
		Status:            domain.PaymentStatusSucceeded,
		AmountKopecks:     payment.AmountKopecks,
	})
	return h.payments.HandleWebhook(t.Context(), testProviderFake, []byte(`{}`))
}

// TestWebhook_SucceededAppliesUpgrade proves the acceptance criterion of
// issue #250: a succeeded webhook applies the new tariff at its full price
// with the period counted from the payment moment, enables auto-renew, and
// records the transition-log entry and audit entry for the applied payment.
// Migrated from the pre-rewrite tests
// TestBilling_ConfirmFakePayment_AppliesUpgrade /
// TestBilling_HandleWebhook_AppliesRenewal.
func TestWebhook_SucceededAppliesUpgrade(t *testing.T) {
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	payment := h.seedSucceededUpgrade(t, sub)

	h.requireUpgradedSubscription(t, sub.UserID, payment.ID)
	h.requireFinalizedSucceeded(t, payment.ID)
	h.requireAppliedTransition(t, sub, payment.ID)

	entries := h.audit.recorded()
	// One tariff_changed at initiation plus one payment.succeeded here.
	if len(entries) != 2 {
		t.Fatalf("audit entries = %d, want 2 (initiation decision + applied payment)", len(entries))
	}
	applied := entries[1]
	if applied.Action != auditdomain.ActionSubscriptionPaymentSucceeded {
		t.Errorf("audit action = %q, want subscription_payment.succeeded", applied.Action)
	}
	if applied.EntityID == nil || *applied.EntityID != payment.ID {
		t.Errorf("audit entity = %v, want the payment", applied.EntityID)
	}
}

// TestWebhook_SucceededAppliesGraceRenewal proves a succeeded same-tariff
// payment in grace renews the subscription from the payment moment (grace is
// not paid time and must not be gifted) and reactivates it. Migrated from the
// pre-rewrite test TestBilling_HandleWebhook_AppliesRenewal.
func TestWebhook_SucceededAppliesGraceRenewal(t *testing.T) {
	h := newPaymentHarness(t)
	graceUntil := h.now.Add(24 * time.Hour)
	sub := h.seedSubscription(t, func(s *domain.Subscription) {
		s.Status = domain.SubscriptionStatusGrace
		s.ValidUntil = &graceUntil
	})
	result, err := h.subs.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff(renewal) error = %v", err)
	}
	payment, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if err := h.webhookSucceeded(t, payment); err != nil {
		t.Fatalf("HandleWebhook() error = %v", err)
	}

	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.Status != domain.SubscriptionStatusActive {
		t.Errorf("status = %q, want active after renewal", stored.Status)
	}
	wantUntil := h.now.AddDate(0, 1, 0)
	if stored.ValidUntil == nil || !stored.ValidUntil.Equal(wantUntil) {
		t.Errorf("valid until = %v, want %v (renewal from the payment moment, not from grace end)", stored.ValidUntil, wantUntil)
	}
}

// TestWebhook_DuplicateSucceededIsIdempotent proves the deduplication
// acceptance criterion: a repeated delivery of the same succeeded notification
// is an idempotent no-op — the payment stays succeeded, the period is not
// extended twice, and no extra transition or audit entry appears. Migrated
// from the pre-rewrite test
// TestBilling_HandleWebhook_DuplicateSucceededIsIdempotent.
func TestWebhook_DuplicateSucceededIsIdempotent(t *testing.T) {
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	result := h.initiateUpgrade(t, sub)
	payment, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if err := h.webhookSucceeded(t, payment); err != nil {
		t.Fatalf("first HandleWebhook() error = %v", err)
	}
	first, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	transitionsAfterFirst := h.transitionCount(t, sub.ID)
	auditAfterFirst := len(h.audit.recorded())

	if err := h.webhookSucceeded(t, payment); err != nil {
		t.Fatalf("duplicate HandleWebhook() error = %v (a redelivery is a no-op success)", err)
	}

	second, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if !second.ValidUntil.Equal(*first.ValidUntil) {
		t.Errorf("valid until moved on a duplicate delivery: %v → %v", first.ValidUntil, second.ValidUntil)
	}
	if h.transitionCount(t, sub.ID) != transitionsAfterFirst {
		t.Error("duplicate delivery appended another transition")
	}
	if len(h.audit.recorded()) != auditAfterFirst {
		t.Error("duplicate delivery appended another audit entry")
	}
}

// TestWebhook_AppliesFailedUpgradePayment proves a failed notification marks
// the payment failed with the provider error code and leaves the subscription
// untouched (issue #250: no grace transition — the upgrade never applied).
// Migrated from the pre-rewrite test
// TestBilling_HandleWebhook_AppliesFailedUpgradePayment.
func TestWebhook_AppliesFailedUpgradePayment(t *testing.T) {
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	result := h.initiateUpgrade(t, sub)
	errCode := testErrCodeDeclined
	h.setNotification(&PaymentNotification{
		InternalPaymentID: result.PaymentID,
		ProviderPaymentID: "stub_" + result.PaymentID.String(),
		Status:            domain.PaymentStatusFailed,
		ErrorCode:         &errCode,
	})

	if err := h.payments.HandleWebhook(t.Context(), testProviderFake, []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook() error = %v", err)
	}

	payment, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if payment.Status != domain.PaymentStatusFailed {
		t.Errorf("payment status = %q, want failed", payment.Status)
	}
	if payment.ErrorCode == nil || *payment.ErrorCode != errCode {
		t.Errorf("error code = %v, want %q", payment.ErrorCode, errCode)
	}
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.TariffID != sub.TariffID || stored.Status != sub.Status {
		t.Errorf("failed upgrade changed the subscription: %+v", stored)
	}
	if h.transitionCount(t, sub.ID) != 0 {
		t.Error("failed upgrade appended a subscription transition")
	}
	entries := h.audit.recorded()
	if len(entries) != 2 || entries[1].Action != auditdomain.ActionSubscriptionPaymentFailed {
		t.Fatalf("audit entries = %+v, want a payment.failed after the initiation entry", entries)
	}
}

// TestWebhook_SucceededPersistsProviderPaymentID proves a webhook that
// arrives before the initiation result was saved still finalizes the payment:
// the carried provider reference is persisted, then the outcome applied.
// Migrated from the pre-rewrite test
// TestBilling_HandleWebhook_SucceededPersistsProviderPaymentID.
func TestWebhook_SucceededPersistsProviderPaymentID(t *testing.T) {
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	// Crash state: the provider was called but the reference was lost.
	payment, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, h.tariffID(t, domain.TariffBusiness),
		domain.PeriodMonth, 99000, testProviderFake, h.now)
	if err != nil {
		t.Fatalf("NewSubscriptionPayment() error = %v", err)
	}
	if _, err := h.stores.payments.Create(t.Context(), payment); err != nil {
		t.Fatalf("seed Create() error = %v", err)
	}
	lateRef := "stub_late_" + payment.ID.String()
	h.setNotification(&PaymentNotification{
		InternalPaymentID: payment.ID,
		ProviderPaymentID: lateRef,
		Status:            domain.PaymentStatusSucceeded,
		AmountKopecks:     99000,
	})

	if err := h.payments.HandleWebhook(t.Context(), testProviderFake, []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook() error = %v", err)
	}

	stored, err := h.stores.payments.GetByID(t.Context(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if !stored.HasProviderReference() || *stored.ProviderPaymentID != lateRef {
		t.Errorf("provider payment id = %v, want %q persisted from the notification", stored.ProviderPaymentID, lateRef)
	}
	if stored.Status != domain.PaymentStatusSucceeded {
		t.Errorf("payment status = %q, want succeeded", stored.Status)
	}
	subsStored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if subsStored.TariffID != h.tariffID(t, domain.TariffBusiness) {
		t.Errorf("tariff = %v, want business applied", subsStored.TariffID)
	}
}

// TestWebhook_PendingNotificationBackfillsReference proves a pending-status
// notification carries no outcome: it only backfills a lost provider
// reference and leaves the payment pending.
func TestWebhook_PendingNotificationBackfillsReference(t *testing.T) {
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	payment, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, h.tariffID(t, domain.TariffBusiness),
		domain.PeriodMonth, 99000, testProviderFake, h.now)
	if err != nil {
		t.Fatalf("NewSubscriptionPayment() error = %v", err)
	}
	if _, err := h.stores.payments.Create(t.Context(), payment); err != nil {
		t.Fatalf("seed Create() error = %v", err)
	}
	ref := "stub_authorized"
	h.setNotification(&PaymentNotification{
		InternalPaymentID: payment.ID,
		ProviderPaymentID: ref,
		Status:            domain.PaymentStatusPending,
	})

	if err := h.payments.HandleWebhook(t.Context(), testProviderFake, []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook() error = %v", err)
	}

	stored, err := h.stores.payments.GetByID(t.Context(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if !stored.HasProviderReference() || *stored.ProviderPaymentID != ref {
		t.Errorf("provider payment id = %v, want %q backfilled", stored.ProviderPaymentID, ref)
	}
	if stored.Status != domain.PaymentStatusPending {
		t.Errorf("status = %q, want still pending", stored.Status)
	}
}

// TestWebhook_SucceededAfterFailed_ReconcilesToSucceeded proves the
// out-of-order acceptance criterion: a succeeded notification for a payment
// already marked failed is resolved against the provider status as the source
// of truth (ADR 0010) — a provider-confirmed success is reconciled and
// applied. Migrated from the pre-rewrite test
// TestBilling_HandleWebhook_SucceededAfterFailed_ReconcilesToSucceeded.
func TestWebhook_SucceededAfterFailed_ReconcilesToSucceeded(t *testing.T) {
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	result := h.initiateUpgrade(t, sub)
	errCode := "timeout"
	h.setNotification(&PaymentNotification{
		InternalPaymentID: result.PaymentID,
		ProviderPaymentID: "stub_" + result.PaymentID.String(),
		Status:            domain.PaymentStatusFailed,
		ErrorCode:         &errCode,
	})
	if err := h.payments.HandleWebhook(t.Context(), testProviderFake, []byte(`{}`)); err != nil {
		t.Fatalf("failed HandleWebhook() error = %v", err)
	}

	// The provider actually captured the money: the out-of-order success is
	// confirmed against the provider before anything is applied.
	h.provider.statusRes = PaymentStatusResult{Status: domain.PaymentStatusSucceeded}
	h.setNotification(&PaymentNotification{
		InternalPaymentID: result.PaymentID,
		ProviderPaymentID: "stub_" + result.PaymentID.String(),
		Status:            domain.PaymentStatusSucceeded,
	})
	if err := h.payments.HandleWebhook(t.Context(), testProviderFake, []byte(`{}`)); err != nil {
		t.Fatalf("out-of-order HandleWebhook() error = %v", err)
	}
	if h.provider.statusCalls == 0 {
		t.Fatal("provider status was never consulted for the out-of-order success")
	}

	payment, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if payment.Status != domain.PaymentStatusSucceeded {
		t.Errorf("payment status = %q, want reconciled to succeeded", payment.Status)
	}
	if payment.ErrorCode != nil {
		t.Errorf("error code = %v, want cleared on reconciliation", payment.ErrorCode)
	}
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.TariffID != h.tariffID(t, domain.TariffBusiness) || stored.Status != domain.SubscriptionStatusActive {
		t.Errorf("subscription = %q/%v, want the business tariff applied and active", stored.Status, stored.TariffID)
	}
}

// TestWebhook_SucceededAfterFailed_ProviderStillFailsIsNoOp proves the other
// side of the out-of-order rule: when the provider still reports the payment
// failed, the late success notification changes nothing. Migrated from the
// pre-rewrite test
// TestBilling_HandleWebhook_SucceededAfterFailed_StillFailed.
func TestWebhook_SucceededAfterFailed_ProviderStillFailsIsNoOp(t *testing.T) {
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	result := h.initiateUpgrade(t, sub)
	errCode := testErrCodeDeclined
	h.setNotification(&PaymentNotification{
		InternalPaymentID: result.PaymentID,
		ProviderPaymentID: "stub_" + result.PaymentID.String(),
		Status:            domain.PaymentStatusFailed,
		ErrorCode:         &errCode,
	})
	if err := h.payments.HandleWebhook(t.Context(), testProviderFake, []byte(`{}`)); err != nil {
		t.Fatalf("failed HandleWebhook() error = %v", err)
	}

	h.provider.statusRes = PaymentStatusResult{Status: domain.PaymentStatusFailed, ErrorCode: testErrCodeDeclined}
	h.setNotification(&PaymentNotification{
		InternalPaymentID: result.PaymentID,
		ProviderPaymentID: "stub_" + result.PaymentID.String(),
		Status:            domain.PaymentStatusSucceeded,
	})
	if err := h.payments.HandleWebhook(t.Context(), testProviderFake, []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook() error = %v (contradicted success is a no-op success)", err)
	}

	payment, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if payment.Status != domain.PaymentStatusFailed {
		t.Errorf("payment status = %q, want still failed (the provider is the source of truth)", payment.Status)
	}
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.TariffID != sub.TariffID {
		t.Errorf("tariff = %v, want unchanged pro", stored.TariffID)
	}
}

// TestWebhook_SucceededAfterFailed_StatusErrorPropagates proves an
// unresolvable provider status check fails the delivery (non-200, provider
// retries) instead of silently dropping the notification. Migrated from the
// pre-rewrite test
// TestBilling_HandleWebhook_SucceededAfterFailed_StatusError.
func TestWebhook_SucceededAfterFailed_StatusErrorPropagates(t *testing.T) {
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	result := h.initiateUpgrade(t, sub)
	errCode := "timeout"
	h.setNotification(&PaymentNotification{
		InternalPaymentID: result.PaymentID,
		ProviderPaymentID: "stub_" + result.PaymentID.String(),
		Status:            domain.PaymentStatusFailed,
		ErrorCode:         &errCode,
	})
	if err := h.payments.HandleWebhook(t.Context(), testProviderFake, []byte(`{}`)); err != nil {
		t.Fatalf("failed HandleWebhook() error = %v", err)
	}

	h.provider.statusErr = errors.New("provider unreachable")
	h.setNotification(&PaymentNotification{
		InternalPaymentID: result.PaymentID,
		ProviderPaymentID: "stub_" + result.PaymentID.String(),
		Status:            domain.PaymentStatusSucceeded,
	})
	if err := h.payments.HandleWebhook(t.Context(), testProviderFake, []byte(`{}`)); err == nil {
		t.Fatal("HandleWebhook() error = nil, want the provider status failure to fail the delivery")
	}
}

// TestWebhook_FailedAfterSucceededIsNoOp proves a terminal state is never
// overridden by a late opposite outcome: money captured stays captured.
func TestWebhook_FailedAfterSucceededIsNoOp(t *testing.T) {
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	result := h.initiateUpgrade(t, sub)
	payment, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if err := h.webhookSucceeded(t, payment); err != nil {
		t.Fatalf("HandleWebhook() error = %v", err)
	}

	errCode := "late_failure"
	h.setNotification(&PaymentNotification{
		InternalPaymentID: result.PaymentID,
		ProviderPaymentID: "stub_" + result.PaymentID.String(),
		Status:            domain.PaymentStatusFailed,
		ErrorCode:         &errCode,
	})
	if err := h.payments.HandleWebhook(t.Context(), testProviderFake, []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook() error = %v (a late failure after success is a no-op)", err)
	}
	stored, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored.Status != domain.PaymentStatusSucceeded {
		t.Errorf("payment status = %q, want still succeeded", stored.Status)
	}
}

// TestWebhook_RefundedMarksPaymentRefunded proves a refund notification
// records the full refund on the payment; the subscription-side effects of a
// refund land with the admin refund flow (issue #254).
func TestWebhook_RefundedMarksPaymentRefunded(t *testing.T) {
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	payment := h.seedSucceededUpgrade(t, sub)

	h.deliverRefundNotification(t, payment.ID)
	stored, err := h.stores.payments.GetByID(t.Context(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored.Status != domain.PaymentStatusRefunded {
		t.Errorf("payment status = %q, want refunded", stored.Status)
	}
	if stored.RefundedAmountKopecks == nil || *stored.RefundedAmountKopecks != 99000 {
		t.Errorf("refunded amount = %v, want the full 99000", stored.RefundedAmountKopecks)
	}
	// A refund notification applies the refund's subscription effects with the
	// system initiator (issue #254): the paid time is returned, so the
	// subscription falls to the basic tariff.
	subsStored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if subsStored.TariffID != h.tariffID(t, domain.TariffBasic) {
		t.Errorf("tariff = %v, want basic after the refund", subsStored.TariffID)
	}
	if subsStored.ValidUntil != nil || subsStored.AutoRenewEnabled {
		t.Errorf("subscription = valid_until %v, auto-renew %t; want both cleared by the downgrade to basic",
			subsStored.ValidUntil, subsStored.AutoRenewEnabled)
	}
	// The transition log records the refund downgrade with the system
	// initiator and the refunded payment.
	refundTransition := h.lastRefundTransition(t, subsStored.ID)
	if refundTransition.Initiator != domain.InitiatorSystem || refundTransition.InitiatorID != nil {
		t.Errorf("refund transition initiator = %q/%v, want system without an actor", refundTransition.Initiator, refundTransition.InitiatorID)
	}
	if refundTransition.PaymentID == nil || *refundTransition.PaymentID != payment.ID {
		t.Errorf("refund transition payment = %v, want %v", refundTransition.PaymentID, payment.ID)
	}
}

// TestWebhook_Rejections proves the guard rails: a provider other than the
// active one, a notification for an unknown payment and a notification whose
// provider payment id contradicts the persisted reference all fail the
// delivery without state changes.
func TestWebhook_Rejections(t *testing.T) {
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	result := h.initiateUpgrade(t, sub)
	payment, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if err := h.payments.HandleWebhook(t.Context(), "tkassa", []byte(`{}`)); !errors.Is(err, ErrWebhookProviderMismatch) {
		t.Errorf("foreign provider err = %v, want ErrWebhookProviderMismatch", err)
	}

	h.provider.parseErr = errors.New("bad signature")
	if err := h.payments.HandleWebhook(t.Context(), testProviderFake, []byte(`{}`)); err == nil {
		t.Error("parse error = nil, want the delivery to fail")
	}
	h.provider.parseErr = nil

	h.setNotification(&PaymentNotification{
		InternalPaymentID: uuid.Must(uuid.NewV7()),
		ProviderPaymentID: "unknown",
		Status:            domain.PaymentStatusSucceeded,
	})
	if err := h.payments.HandleWebhook(t.Context(), testProviderFake, []byte(`{}`)); !errors.Is(err, ErrPaymentNotFound) {
		t.Errorf("unknown payment err = %v, want ErrPaymentNotFound", err)
	}

	h.setNotification(&PaymentNotification{
		InternalPaymentID: payment.ID,
		ProviderPaymentID: "other_payment",
		Status:            domain.PaymentStatusSucceeded,
	})
	if err := h.payments.HandleWebhook(t.Context(), testProviderFake, []byte(`{}`)); !errors.Is(err, ErrWebhookPaymentMismatch) {
		t.Errorf("mismatched provider payment id err = %v, want ErrWebhookPaymentMismatch", err)
	}

	stored, err := h.stores.payments.GetByID(t.Context(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored.Status != domain.PaymentStatusPending {
		t.Errorf("payment status = %q, want untouched pending", stored.Status)
	}
}

// The method-bound branch went live with issue #251; its delivery semantics
// (completion, redelivery idempotency, expired and unknown sessions) are
// covered by the payment-method service tests.

// TestApplyProviderEvent_AppliesPaymentBranch proves the parsed-event seam
// (issue #287): a payment event handed over by any delivery channel — the
// webhook flow after parsing, the local fake confirmation after the adapter
// reports the completed entry — runs through the same synchronous finalization.
func TestApplyProviderEvent_AppliesPaymentBranch(t *testing.T) {
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	result := h.initiateUpgrade(t, sub)
	payment, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	err = h.payments.ApplyProviderEvent(t.Context(), WebhookEvent{Payment: &PaymentNotification{
		InternalPaymentID: payment.ID,
		ProviderPaymentID: *payment.ProviderPaymentID,
		Status:            domain.PaymentStatusSucceeded,
		AmountKopecks:     payment.AmountKopecks,
	}})
	if err != nil {
		t.Fatalf("ApplyProviderEvent() error = %v", err)
	}

	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.TariffID != h.tariffID(t, domain.TariffBusiness) || stored.Status != domain.SubscriptionStatusActive {
		t.Errorf("subscription = %q/%v, want business applied and active", stored.Status, stored.TariffID)
	}
}

// TestApplyProviderEvent_EmptyEventRejected proves an event with no
// payload branch is rejected with the unsupported sentinel instead of
// silently applying nothing.
func TestApplyProviderEvent_EmptyEventRejected(t *testing.T) {
	h := newPaymentHarness(t)

	if err := h.payments.ApplyProviderEvent(t.Context(), WebhookEvent{}); !errors.Is(err, ErrWebhookUnsupported) {
		t.Errorf("err = %v, want ErrWebhookUnsupported", err)
	}
}

// The local fake confirmation flow moved to the HTTP adapter level (issue
// #287): its unit seam is the fake-confirm handlers over the ApplyProviderEvent
// port, and its end-to-end behaviour is pinned by the integration tests.

// TestListPayments_ReturnsPaymentsWithTariffs proves GET /subscription/payments
// data: the user's payments with the tariff resolved, newest first (issue
// #250).
func TestListPayments_ReturnsPaymentsWithTariffs(t *testing.T) {
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	first := h.initiateUpgrade(t, sub)
	// Finalize the first payment, then a later pending one exists directly.
	payment, err := h.stores.payments.GetByID(t.Context(), first.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if err := h.webhookSucceeded(t, payment); err != nil {
		t.Fatalf("HandleWebhook() error = %v", err)
	}
	later, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, h.tariffID(t, domain.TariffBusiness),
		domain.PeriodYear, 890000, testProviderFake, h.now.Add(time.Minute))
	if err != nil {
		t.Fatalf("NewSubscriptionPayment() error = %v", err)
	}
	if _, err := h.stores.payments.Create(t.Context(), later); err != nil {
		t.Fatalf("seed Create() error = %v", err)
	}

	views, err := h.payments.ListPayments(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("ListPayments() error = %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("views = %d, want 2", len(views))
	}
	for _, v := range views {
		if v.Tariff.ID != v.Payment.TariffID {
			t.Errorf("view tariff = %v, want the payment tariff %v", v.Tariff.ID, v.Payment.TariffID)
		}
	}
	if views[0].Payment.ID != later.ID || views[0].Payment.Status != domain.PaymentStatusPending {
		t.Errorf("newest view = %v/%q, want the later pending payment first", views[0].Payment.ID, views[0].Payment.Status)
	}
	if views[1].Payment.Status != domain.PaymentStatusSucceeded {
		t.Errorf("oldest view status = %q, want succeeded", views[1].Payment.Status)
	}

	other, err := h.payments.ListPayments(t.Context(), uuid.Must(uuid.NewV7()))
	if err != nil {
		t.Fatalf("ListPayments(other) error = %v", err)
	}
	if len(other) != 0 {
		t.Errorf("other user payments = %d, want 0", len(other))
	}
}
