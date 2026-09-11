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
	cancelErr := h.subscriptionsSvc.CancelSubscription(h.ctx(), sub.UserID)
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
