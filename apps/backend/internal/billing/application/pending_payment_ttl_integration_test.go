//go:build integration

package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// The pending-payment form TTL of issue #616 against real PostgreSQL: the
// persisted deadline, the one-pending-payment locks, the TTL-worker expiry
// with its audit entry, and the late success webhook reconciled against the
// provider through the shared out-of-order seam.

// ttlPendingFlow seeds a paid basic subscription with a live pro upgrade
// payment and returns the subscription with the stored pending payment.
func ttlPendingFlow(t *testing.T, h *paymentIntegrationHarness) (domain.Subscription, domain.SubscriptionPayment) {
	t.Helper()
	sub := h.seedPaidSubscription(t, domain.TariffBasic)
	result, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), sub.UserID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff(upgrade): %v", err)
	}
	stored, err := h.payments.GetByID(h.ctx(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID(payment): %v", err)
	}
	return sub, stored
}

// storedPaymentByID reads one payment through the repository, failing the
// test on a miss.
func storedPaymentByID(t *testing.T, h *paymentIntegrationHarness, id uuid.UUID) domain.SubscriptionPayment {
	t.Helper()
	payment, err := h.payments.GetByID(h.ctx(), id)
	if err != nil {
		t.Fatalf("GetByID(%s): %v", id, err)
	}
	return payment
}

// storedSub re-reads the subscription after a phase ran.
func storedSub(t *testing.T, h *paymentIntegrationHarness, sub domain.Subscription) domain.Subscription {
	t.Helper()
	stored, err := h.subscriptions.GetByUserID(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	return stored
}

// TestPendingPaymentTTL_DeadlineAndView_Integration proves the persisted
// deadline and the contract view of the live pending payment.
func TestPendingPaymentTTL_DeadlineAndView_Integration(t *testing.T) {
	t.Parallel()
	h := newPaymentIntegrationHarness(t)
	sub, stored := ttlPendingFlow(t, h)

	if stored.ExpiresAt == nil || !stored.ExpiresAt.After(h.clock.Now()) {
		t.Fatalf("expiresAt = %v, want a future deadline", stored.ExpiresAt)
	}
	view, err := h.subscriptionsSvc.GetSubscription(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("GetSubscription: %v", err)
	}
	if view.PendingPayment == nil {
		t.Fatal("GetSubscription has no pendingPayment, want the live upgrade payment")
	}
	pp := view.PendingPayment
	if pp.Tariff.Name != domain.TariffPro {
		t.Errorf("pendingPayment tariff = %q, want pro", pp.Tariff.Name)
	}
	if pp.Payment.Period != domain.PeriodMonth || pp.Payment.AmountKopecks != 49000 {
		t.Errorf("pendingPayment = %q/%d, want month/49000", pp.Payment.Period, pp.Payment.AmountKopecks)
	}
	if !pp.Payment.HasPaymentURL() || !pp.Payment.ExpiresAt.Equal(*stored.ExpiresAt) {
		t.Errorf("pendingPayment url/expiresAt = %v/%v, want the confirm url and the deadline",
			pp.Payment.PaymentURL, pp.Payment.ExpiresAt)
	}
}

// TestPendingPaymentTTL_Blocks_Integration proves the one-pending-payment
// rule: every other tariff selection and the cancellation answer the sentinel
// while the payment lives.
func TestPendingPaymentTTL_Blocks_Integration(t *testing.T) {
	t.Parallel()
	h := newPaymentIntegrationHarness(t)
	sub, _ := ttlPendingFlow(t, h)

	_, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), sub.UserID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffBusiness,
		Period:     domain.PeriodYear,
	})
	if !errors.Is(err, billingapp.ErrPendingPaymentExists) {
		t.Fatalf("ChangeTariff(other) err = %v, want ErrPendingPaymentExists", err)
	}
	cancelErr := h.subscriptionsSvc.CancelSubscription(h.ctx(), sub.UserID, nil)
	if !errors.Is(cancelErr, billingapp.ErrPendingPaymentExists) {
		t.Fatalf("CancelSubscription err = %v, want ErrPendingPaymentExists", cancelErr)
	}
}

// TestPendingPaymentTTL_ExpiryWorker_Integration proves the expiry: the
// worker marks the payment failed with the form-expired code, audits the
// failure, leaves the subscription untouched, and the choice unlocks with a
// fresh payment instead of the dead one.
func TestPendingPaymentTTL_ExpiryWorker_Integration(t *testing.T) {
	t.Parallel()
	h := newPaymentIntegrationHarness(t)
	sub, stored := ttlPendingFlow(t, h)

	h.clock.now = stored.ExpiresAt.Add(time.Minute)
	n, err := h.services.Workers.ProcessExpiredPendingPayments(h.ctx(), h.clock.Now())
	if err != nil {
		t.Fatalf("ProcessExpiredPendingPayments: %v", err)
	}
	if n != 1 {
		t.Fatalf("expired = %d, want 1", n)
	}
	expired := storedPaymentByID(t, h, stored.ID)
	if expired.Status != domain.PaymentStatusFailed || expired.ErrorCode == nil ||
		*expired.ErrorCode != domain.PaymentErrorCodeFormExpired {
		t.Errorf("payment = %q/%v, want failed with %q",
			expired.Status, expired.ErrorCode, domain.PaymentErrorCodeFormExpired)
	}
	if got := h.countRows(
		`SELECT count(*) FROM audit_log WHERE action = 'subscription_payment.failed' AND entity_id = $1`,
		stored.ID); got != 1 {
		t.Errorf("audit rows = %d, want 1", got)
	}
	subAfter := storedSub(t, h, sub)
	if subAfter.Status != domain.SubscriptionStatusActive || subAfter.TariffID != sub.TariffID {
		t.Errorf("subscription = %q, want untouched active", subAfter.Status)
	}
	view, err := h.subscriptionsSvc.GetSubscription(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("GetSubscription(after expiry): %v", err)
	}
	if view.PendingPayment != nil {
		t.Errorf("pendingPayment = %+v, want none after expiry", view.PendingPayment)
	}
	retry, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), sub.UserID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff(after expiry): %v", err)
	}
	if retry.PaymentID == stored.ID {
		t.Error("ChangeTariff(after expiry) returned the expired payment, want a fresh one")
	}
}

// seedFormPayment persists a live pending form payment of the given target.
func (h *paymentIntegrationHarness) seedFormPayment(
	t *testing.T, sub domain.Subscription, name domain.TariffName, period domain.SubscriptionPeriod, amount int64,
) domain.SubscriptionPayment {
	t.Helper()
	return h.seedFormPaymentAt(t, sub, name, period, amount, h.clock.Now(), 15*time.Minute)
}

// seedDeadFormPayment persists a pending form payment of the given target
// whose deadline has already run out — the state the TTL worker's lag leaves
// behind (issue #690). The domain only attaches a future deadline, so both
// instants are backdated.
func (h *paymentIntegrationHarness) seedDeadFormPayment(
	t *testing.T, sub domain.Subscription, name domain.TariffName, period domain.SubscriptionPeriod, amount int64,
) domain.SubscriptionPayment {
	t.Helper()
	return h.seedFormPaymentAt(t, sub, name, period, amount, h.clock.Now().Add(-16*time.Minute), time.Minute)
}

// seedFormPaymentAt persists a pending form payment created at createdAt with
// its deadline lifetime minutes later.
func (h *paymentIntegrationHarness) seedFormPaymentAt(
	t *testing.T, sub domain.Subscription, name domain.TariffName, period domain.SubscriptionPeriod,
	amount int64, createdAt time.Time, lifetime time.Duration,
) domain.SubscriptionPayment {
	t.Helper()
	deadline := createdAt.Add(lifetime)
	payment, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, h.tariffIDByName(t, name), period, amount, testProviderFake, createdAt)
	if err != nil {
		t.Fatalf("new payment: %v", err)
	}
	if err := payment.AttachFormDeadline(deadline, createdAt); err != nil {
		t.Fatalf("attach deadline: %v", err)
	}
	stored, err := h.payments.Create(h.ctx(), payment)
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	return stored
}

// TestPendingPaymentTTL_OneFormPerUser_Index_Integration proves the schema
// backstop of issue #690 on the real database: a second live form of another
// tariff and period violates the user-level partial unique index and the
// adapter narrows exactly that index to ErrPendingPaymentExists — while a
// merchant-initiated charge (no deadline) still coexists with the live form,
// the deliberate #616 invariant the index predicate keeps out of its way.
func TestPendingPaymentTTL_OneFormPerUser_Index_Integration(t *testing.T) {
	t.Parallel()
	h := newPaymentIntegrationHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffBasic)
	live := h.seedFormPayment(t, sub, domain.TariffPro, domain.PeriodMonth, 49000)

	form, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, h.tariffIDByName(t, domain.TariffBusiness),
		domain.PeriodYear, 890000, testProviderFake, h.clock.Now())
	if err != nil {
		t.Fatalf("new payment: %v", err)
	}
	if err := form.AttachFormDeadline(h.clock.Now().Add(15*time.Minute), h.clock.Now()); err != nil {
		t.Fatalf("attach deadline: %v", err)
	}
	if _, err := h.payments.Create(h.ctx(), form); !errors.Is(err, billingapp.ErrPendingPaymentExists) {
		t.Fatalf("second live form err = %v, want ErrPendingPaymentExists", err)
	}

	mit, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, h.tariffIDByName(t, domain.TariffBusiness),
		domain.PeriodMonth, 99000, testProviderFake, h.clock.Now())
	if err != nil {
		t.Fatalf("new merchant-initiated payment: %v", err)
	}
	if _, err := h.payments.Create(h.ctx(), mit); err != nil {
		t.Fatalf("merchant-initiated charge next to a live form: %v", err)
	}
	if _, err := h.payments.GetByID(h.ctx(), live.ID); err != nil {
		t.Fatalf("GetByID(live): %v", err)
	}
}

// TestPendingPaymentTTL_SweepsDeadFormsOfAnyTarget_Integration proves the
// planning sweep of issue #690 against the real schema: a dead pending form
// of another target — the state the TTL worker's lag leaves behind — is
// expired by the planning transaction itself, so a fresh selection starts
// instead of dying on the user-level unique index.
func TestPendingPaymentTTL_SweepsDeadFormsOfAnyTarget_Integration(t *testing.T) {
	t.Parallel()
	h := newPaymentIntegrationHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffBasic)
	dead := h.seedDeadFormPayment(t, sub, domain.TariffBusiness, domain.PeriodYear, 890000)

	result, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), sub.UserID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodYear,
	})
	if err != nil {
		t.Fatalf("ChangeTariff() with a dead form of another target: %v", err)
	}
	if result.PaymentID == dead.ID {
		t.Fatal("ChangeTariff() returned the dead payment, want a fresh one")
	}
	swept := storedPaymentByID(t, h, dead.ID)
	if swept.Status != domain.PaymentStatusFailed || swept.ErrorCode == nil ||
		*swept.ErrorCode != domain.PaymentErrorCodeFormExpired {
		t.Errorf("dead form = %q/%v, want failed with %q",
			swept.Status, swept.ErrorCode, domain.PaymentErrorCodeFormExpired)
	}
}

// TestPendingPaymentTTL_LateSuccessWebhook_Integration proves the race of
// issue #616 point 6: the user pays at the last second, the webhook arrives
// after the TTL worker expired the payment — the provider (the source of
// truth, ADR 0010) confirms the capture and the success applies.
func TestPendingPaymentTTL_LateSuccessWebhook_Integration(t *testing.T) {
	t.Parallel()
	h := newPaymentIntegrationHarness(t)
	sub, stored := ttlPendingFlow(t, h)

	// The user completed the payment at the provider in the last seconds of
	// the form's life; the success webhook has not arrived yet.
	h.provider.SetPaymentState(stored.ID.String(), billingapp.PaymentStatusResult{
		Status: domain.PaymentStatusSucceeded,
	})

	// The form deadline runs out and the worker expires the payment.
	h.clock.now = stored.ExpiresAt.Add(time.Minute)
	if _, err := h.services.Workers.ProcessExpiredPendingPayments(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessExpiredPendingPayments: %v", err)
	}

	// The late success webhook arrives: reconciled against the provider and
	// applied — the upgrade takes effect.
	h.deliverWebhook(t, *stored.ProviderPaymentID, domain.PaymentStatusSucceeded, stored.ID)

	final := storedPaymentByID(t, h, stored.ID)
	if final.Status != domain.PaymentStatusSucceeded || final.ErrorCode != nil || final.SucceededAt == nil {
		t.Errorf("payment = %q/%v, want reconciled to succeeded", final.Status, final.ErrorCode)
	}
	subAfter := storedSub(t, h, sub)
	if subAfter.TariffID != h.tariffIDByName(t, domain.TariffPro) {
		t.Errorf("subscription tariff = %v, want the late payment applied", subAfter.TariffID)
	}
}
