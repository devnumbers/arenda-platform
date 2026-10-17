//go:build integration

package application_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// The automatic saved-method side of the parent CIT payment (issue #1512):
// the payment the payer completes on the provider form carries the RebillId
// in its notifications (ADR 0017), and the method must exist locally without
// any explicit binding step — otherwise the renewal worker finds nothing to
// charge with after the first payment and the subscription drops into grace
// despite the bank having kept the card.

// citSavedMethod builds the saved method of an external delivery with the
// given charge token.
func citSavedMethod(token string) *billingapp.SavedMethod {
	return &billingapp.SavedMethod{
		ProviderMethodID: "card_external",
		ChargeToken:      token,
		MaskedPan:        integrationFakeCardMask,
		ExpDate:          "1230",
	}
}

// requireSingleActiveMethod proves the user has exactly one active method and
// returns it.
func requireSingleActiveMethod(t *testing.T, h *integrationHarness, userID uuid.UUID) domain.PaymentMethod {
	t.Helper()
	methods, err := h.methods.ListByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("ListByUserID(): %v", err)
	}
	active := 0
	var found domain.PaymentMethod
	for _, m := range methods {
		if m.IsActive {
			active++
			found = m
		}
	}
	if active != 1 {
		t.Fatalf("active payment methods = %d of %d, want exactly one", active, len(methods))
	}
	return found
}

// requireSubscriptionChargeTarget proves the subscription's charge target
// points at the method.
func requireSubscriptionChargeTarget(t *testing.T, h *integrationHarness, userID, methodID uuid.UUID) {
	t.Helper()
	sub, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID(): %v", err)
	}
	if sub.ActivePaymentMethodID == nil || *sub.ActivePaymentMethodID != methodID {
		t.Errorf("active payment method = %v, want %v", sub.ActivePaymentMethodID, methodID)
	}
}

// requireMethodCount proves the raw payment_methods row count for the user.
func requireMethodCount(t *testing.T, h *integrationHarness, userID uuid.UUID, want int) {
	t.Helper()
	if got := h.countRows("SELECT count(*) FROM payment_methods WHERE user_id = $1", userID); got != want {
		t.Fatalf("payment_methods rows = %d, want %d", got, want)
	}
}

// TestPayment_ConfirmCreatesSavedMethodAutomatically is the acceptance
// scenario of issue #1512 end to end: the payer completes the tariff payment
// on the provider form — no explicit binding step anywhere — and the saved
// method with the chargeable token exists, is active and is the
// subscription's renewal target.
func TestPayment_ConfirmCreatesSavedMethodAutomatically(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	userID, _ := seedPaidProSubscription(t, h)

	result, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), userID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffBusiness,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff(business) error = %v", err)
	}
	h.confirmFakePayment(t, result.PaymentID)

	method := requireSingleActiveMethod(t, h, userID)
	requireMethodCount(t, h, userID, 1)
	if !strings.HasPrefix(method.ProviderToken, "fake_token_") {
		t.Errorf("charge token = %q, want the provider-issued fake_token_ token", method.ProviderToken)
	}
	if method.Provider != "fake" {
		t.Errorf("method provider = %q, want fake", method.Provider)
	}
	if method.DisplayMask != integrationFakeCardMask {
		t.Errorf("display mask = %q, want %q", method.DisplayMask, integrationFakeCardMask)
	}
	requireSubscriptionChargeTarget(t, h, userID, method.ID)

	// A customer-initiated payment carries no method on its own row (issue
	// #426): the failure of a declined purchase must never pull the
	// subscription into grace.
	payment, err := h.payments.GetByID(h.ctx(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID(payment): %v", err)
	}
	if payment.Status != domain.PaymentStatusSucceeded {
		t.Errorf("payment status = %q, want succeeded", payment.Status)
	}
	if payment.PaymentMethodID != nil {
		t.Errorf("payment method id = %v, want nil on a customer-initiated payment", payment.PaymentMethodID)
	}
}

// TestPayment_AuthorizedNotificationSavesMethodAndConverges covers the wire
// shape of ADR 0017 against real PostgreSQL: the AUTHORIZED notification is
// where the RebillId arrives, it creates the active method while the payment
// is still pending, its redelivery converges on the row, and the terminal
// CONFIRMED — which production shows may arrive without the token — changes
// nothing about the method.
func TestPayment_AuthorizedNotificationSavesMethodAndConverges(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	userID, _ := seedPaidProSubscription(t, h)

	result, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), userID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffBusiness,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff(business) error = %v", err)
	}
	payment, err := h.payments.GetByID(h.ctx(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID(payment): %v", err)
	}

	authorized := &billingapp.PaymentNotification{
		InternalPaymentID: payment.ID,
		ProviderPaymentID: *payment.ProviderPaymentID,
		Status:            domain.PaymentStatusPending,
		AmountKopecks:     payment.AmountKopecks,
		CardMask:          integrationFakeCardMask,
		SavedMethod:       citSavedMethod("rb_authorized"),
	}
	if err := h.paymentsSvc.ApplyPaymentNotification(h.ctx(), authorized); err != nil {
		t.Fatalf("ApplyPaymentNotification(authorized) error = %v", err)
	}
	method := requireSingleActiveMethod(t, h, userID)
	if method.ProviderToken != "rb_authorized" {
		t.Errorf("charge token = %q, want rb_authorized", method.ProviderToken)
	}
	requireSubscriptionChargeTarget(t, h, userID, method.ID)
	stillPending, err := h.payments.GetByID(h.ctx(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID(payment): %v", err)
	}
	if stillPending.Status != domain.PaymentStatusPending {
		t.Errorf("payment status = %q, want pending (AUTHORIZED carries no outcome)", stillPending.Status)
	}

	// The provider retries an unacked notification hourly; the redelivery is
	// a convergence no-op.
	if err := h.paymentsSvc.ApplyPaymentNotification(h.ctx(), authorized); err != nil {
		t.Fatalf("ApplyPaymentNotification(redelivery) error = %v", err)
	}
	requireMethodCount(t, h, userID, 1)

	// The terminal notification arrives without the token, as in production:
	// the method survives unchanged and the upgrade applies.
	if err := h.paymentsSvc.ApplyPaymentNotification(h.ctx(), &billingapp.PaymentNotification{
		InternalPaymentID: payment.ID,
		ProviderPaymentID: *payment.ProviderPaymentID,
		Status:            domain.PaymentStatusSucceeded,
		AmountKopecks:     payment.AmountKopecks,
		CardMask:          integrationFakeCardMask,
	}); err != nil {
		t.Fatalf("ApplyPaymentNotification(confirmed) error = %v", err)
	}
	requireMethodCount(t, h, userID, 1)
	requireSubscriptionChargeTarget(t, h, userID, method.ID)
	requireSingleActiveMethod(t, h, userID)
}

// TestPayment_FailedCITKeepsMethodWithoutGrace pins the grace guard of issue
// #426 over the saved-method flow: the declined customer-initiated payment
// fails alone, the subscription stays untouched, and the card the bank saved
// at authorization stays the chargeable method.
func TestPayment_FailedCITKeepsMethodWithoutGrace(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	userID, sub := seedPaidProSubscription(t, h)

	result, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), userID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffBusiness,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff(business) error = %v", err)
	}
	payment, err := h.payments.GetByID(h.ctx(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID(payment): %v", err)
	}
	if err := h.paymentsSvc.ApplyPaymentNotification(h.ctx(), &billingapp.PaymentNotification{
		InternalPaymentID: payment.ID,
		ProviderPaymentID: *payment.ProviderPaymentID,
		Status:            domain.PaymentStatusPending,
		AmountKopecks:     payment.AmountKopecks,
		SavedMethod:       citSavedMethod("rb_declined"),
	}); err != nil {
		t.Fatalf("ApplyPaymentNotification(authorized) error = %v", err)
	}
	code := "51"
	if err := h.paymentsSvc.ApplyPaymentNotification(h.ctx(), &billingapp.PaymentNotification{
		InternalPaymentID: payment.ID,
		ProviderPaymentID: *payment.ProviderPaymentID,
		Status:            domain.PaymentStatusFailed,
		AmountKopecks:     payment.AmountKopecks,
		ErrorCode:         &code,
	}); err != nil {
		t.Fatalf("ApplyPaymentNotification(rejected) error = %v", err)
	}

	failed, err := h.payments.GetByID(h.ctx(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID(payment): %v", err)
	}
	if failed.Status != domain.PaymentStatusFailed {
		t.Errorf("payment status = %q, want failed", failed.Status)
	}
	if failed.PaymentMethodID != nil {
		t.Errorf("payment method id = %v, want nil on a customer-initiated payment", failed.PaymentMethodID)
	}
	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID(): %v", err)
	}
	if stored.Status != domain.SubscriptionStatusActive {
		t.Errorf("subscription status = %q, want active (a failed CIT must not enter grace)", stored.Status)
	}
	if stored.TariffID != sub.TariffID {
		t.Errorf("tariff = %v, want the untouched %v", stored.TariffID, sub.TariffID)
	}
	method := requireSingleActiveMethod(t, h, userID)
	if method.ProviderToken != "rb_declined" {
		t.Errorf("charge token = %q, want rb_declined (the bank kept the card)", method.ProviderToken)
	}
	requireSubscriptionChargeTarget(t, h, userID, method.ID)
}

// TestPayment_NewCardReplacesActiveMethod covers the second payment with a
// different card: the new token becomes the single active method and the
// subscription's charge target, the replaced card stays stored but inactive —
// the same semantics as an explicit re-binding.
func TestPayment_NewCardReplacesActiveMethod(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	userID, _ := seedPaidProSubscription(t, h)
	seedActiveMethod(t, h, userID, "rb_previous_card")

	result, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), userID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffBusiness,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff(business) error = %v", err)
	}
	payment, err := h.payments.GetByID(h.ctx(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID(payment): %v", err)
	}
	if err := h.paymentsSvc.ApplyPaymentNotification(h.ctx(), &billingapp.PaymentNotification{
		InternalPaymentID: payment.ID,
		ProviderPaymentID: *payment.ProviderPaymentID,
		Status:            domain.PaymentStatusPending,
		AmountKopecks:     payment.AmountKopecks,
		SavedMethod:       citSavedMethod("rb_new_card"),
	}); err != nil {
		t.Fatalf("ApplyPaymentNotification(authorized) error = %v", err)
	}

	requireMethodCount(t, h, userID, 2)
	fresh := requireSingleActiveMethod(t, h, userID)
	if fresh.ProviderToken != "rb_new_card" {
		t.Errorf("active charge token = %q, want rb_new_card", fresh.ProviderToken)
	}
	requireSubscriptionChargeTarget(t, h, userID, fresh.ID)
}

// TestWorkers_ReconcileLostWebhookRecoversSavedMethod proves the GetState
// recovery of issue #1512: the provider settles the payment and keeps the
// token, the webhook is lost, and the stale-payment reconciliation restores
// both the outcome and the chargeable method from the GetState answer.
func TestWorkers_ReconcileLostWebhookRecoversSavedMethod(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	userID, _ := seedPaidProSubscription(t, h)

	result, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), userID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffBusiness,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff(business) error = %v", err)
	}
	// The provider settles the payment, but the webhook is lost: the local
	// row never hears about it.
	if _, err := h.provider.ConfirmPayment(h.ctx(), result.PaymentID.String()); err != nil {
		t.Fatalf("ConfirmPayment() error = %v", err)
	}
	agePaymentRow(t, h, result.PaymentID)

	count, err := h.services.Workers.ReconcilePendingPayments(h.ctx(), h.clock.Now())
	if err != nil {
		t.Fatalf("ReconcilePendingPayments() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("ReconcilePendingPayments() = %d, want 1", count)
	}

	method := requireSingleActiveMethod(t, h, userID)
	if !strings.HasPrefix(method.ProviderToken, "fake_token_") {
		t.Errorf("recovered charge token = %q, want the provider's fake_token_ token", method.ProviderToken)
	}
	if method.DisplayMask != integrationFakeCardMask {
		t.Errorf("recovered display mask = %q, want %q", method.DisplayMask, integrationFakeCardMask)
	}
	requireSubscriptionChargeTarget(t, h, userID, method.ID)
	requireReconciledPaymentSucceeded(t, h, result.PaymentID)
	requireReconciledUpgradeApplied(t, h, userID)
}
