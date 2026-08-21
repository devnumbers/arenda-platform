//go:build integration

package application_test

import (
	"testing"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// paymentIntegrationHarness extends the shared integration harness with the
// canonical tariff lookups of the payment scenarios.
type paymentIntegrationHarness struct {
	*integrationHarness
}

func newPaymentIntegrationHarness(t *testing.T) *paymentIntegrationHarness {
	t.Helper()
	return &paymentIntegrationHarness{integrationHarness: newIntegrationHarness(t)}
}

// tariffIDByName resolves a seeded tariff id by name.
func (h *paymentIntegrationHarness) tariffIDByName(t *testing.T, name domain.TariffName) uuid.UUID {
	t.Helper()
	tariff, err := h.tariffs.GetByName(h.ctx(), name)
	if err != nil {
		t.Fatalf("GetByName(%q): %v", name, err)
	}
	return tariff.ID
}

// seedPaidSubscription inserts an owner with a paid subscription on the given
// tariff and returns the subscription.
func (h *paymentIntegrationHarness) seedPaidSubscription(t *testing.T, name domain.TariffName) domain.Subscription {
	t.Helper()
	userID := h.seedUser()
	sub, err := domain.NewBasicSubscription(userID, h.tariffIDByName(t, name))
	if err != nil {
		t.Fatalf("NewBasicSubscription(): %v", err)
	}
	validUntil := h.clock.Now().AddDate(0, 1, 0)
	period := domain.PeriodMonth
	sub.TariffID = h.tariffIDByName(t, name)
	sub.ValidUntil = &validUntil
	sub.AutoRenewEnabled = true
	sub.CurrentPeriod = &period
	created, err := h.subscriptions.Create(h.ctx(), sub)
	if err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
	return created
}

// pendingUpgradePayment loads the started upgrade payment and proves it is a
// pending, fully referenced payment for the full pro month price.
func (h *paymentIntegrationHarness) pendingUpgradePayment(
	t *testing.T, result billingapp.ChangeTariffResult,
) domain.SubscriptionPayment {
	t.Helper()
	payment, err := h.payments.GetByID(h.ctx(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if payment.Status != domain.PaymentStatusPending {
		t.Fatalf("payment status = %q, want pending", payment.Status)
	}
	if !payment.HasProviderReference() || !payment.HasPaymentURL() {
		t.Fatal("the provider reference and payment url must be persisted atomically")
	}
	if payment.AmountKopecks != 49000 {
		t.Fatalf("amount = %d, want the full pro month price 49000", payment.AmountKopecks)
	}
	return payment
}

// requireAppliedProSubscription proves the subscription after a succeeded pro
// payment: pro tariff, active, one month of validity from the payment moment,
// auto-renew on, and the payment recorded as last applied.
func (h *paymentIntegrationHarness) requireAppliedProSubscription(t *testing.T, userID, paymentID uuid.UUID) {
	t.Helper()
	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if stored.TariffID != h.tariffIDByName(t, domain.TariffPro) {
		t.Errorf("tariff = %v, want pro", stored.TariffID)
	}
	if stored.Status != domain.SubscriptionStatusActive {
		t.Errorf("status = %q, want active", stored.Status)
	}
	wantUntil := h.clock.Now().AddDate(0, 1, 0)
	if stored.ValidUntil == nil || !stored.ValidUntil.Equal(wantUntil) {
		t.Errorf("valid until = %v, want %v", stored.ValidUntil, wantUntil)
	}
	if !stored.AutoRenewEnabled {
		t.Error("auto-renew = false, want true")
	}
	if stored.LastAppliedPaymentID == nil || *stored.LastAppliedPaymentID != paymentID {
		t.Errorf("last applied payment = %v, want %v", stored.LastAppliedPaymentID, paymentID)
	}
}

// requirePaymentAppliedTransition proves the transition log carries a
// payment_applied entry referencing the payment.
func (h *paymentIntegrationHarness) requirePaymentAppliedTransition(t *testing.T, subscriptionID, paymentID uuid.UUID) {
	t.Helper()
	transitions, err := h.transitions.ListBySubscriptionID(h.ctx(), subscriptionID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID: %v", err)
	}
	found := false
	for _, tr := range transitions {
		if tr.Reason == domain.TransitionReasonPaymentApplied && tr.PaymentID != nil && *tr.PaymentID == paymentID {
			found = true
		}
	}
	if !found {
		t.Errorf("transitions = %+v, want a payment_applied entry referencing the payment", transitions)
	}
}

// TestPaymentFlow_UpgradeEndToEnd proves the headline acceptance criterion of
// issue #250 against real PostgreSQL and the fake provider adapter: an
// upgrade creates a pending payment with a payer URL, the confirmation
// finalizes it, the tariff is applied at the full price of the new plan with
// the period from the payment moment and auto-renew on, and the transition
// log records the applied payment.
func TestPaymentFlow_UpgradeEndToEnd(t *testing.T) {
	t.Parallel()
	h := newPaymentIntegrationHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffBasic)

	result, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), sub.UserID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff(upgrade): %v", err)
	}
	if result.PaymentID == uuid.Nil || result.ConfirmURL == "" {
		t.Fatalf("result = %+v, want a payment id and a payer url", result)
	}

	payment := h.pendingUpgradePayment(t, result)

	// The user completes the payment at the provider; the webhook carries the
	// provider's notification into the synchronous application path.
	h.confirmFakePayment(t, payment.ID)

	finalized, err := h.payments.GetByID(h.ctx(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID(finalized): %v", err)
	}
	if finalized.Status != domain.PaymentStatusSucceeded {
		t.Fatalf("payment status = %q, want succeeded", finalized.Status)
	}

	h.requireAppliedProSubscription(t, sub.UserID, payment.ID)
	h.requirePaymentAppliedTransition(t, sub.ID, payment.ID)

	// The payments list serves the finalized payment with its tariff.
	views, err := h.paymentsSvc.ListPayments(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("ListPayments: %v", err)
	}
	if len(views) != 1 || views[0].Payment.ID != payment.ID || views[0].Tariff.ID != payment.TariffID {
		t.Errorf("views = %+v, want the single payment with its tariff", views)
	}
}

// TestPaymentFlow_WebhookDuplicateIsIdempotent proves the webhook dedup
// acceptance criterion against real PostgreSQL: delivering the fake
// provider's raw succeeded payload twice applies the tariff once.
func TestPaymentFlow_WebhookDuplicateIsIdempotent(t *testing.T) {
	t.Parallel()
	h := newPaymentIntegrationHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffBasic)

	result, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), sub.UserID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff(upgrade): %v", err)
	}
	payment, err := h.payments.GetByID(h.ctx(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	payload := []byte(`{"provider_payment_id":"` + *payment.ProviderPaymentID +
		`","internal_payment_id":"` + payment.ID.String() + `","status":"succeeded"}`)
	if err := h.paymentsSvc.HandleWebhook(h.ctx(), testProviderFake, payload); err != nil {
		t.Fatalf("first HandleWebhook: %v", err)
	}
	first, err := h.subscriptions.GetByUserID(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if err := h.paymentsSvc.HandleWebhook(h.ctx(), testProviderFake, payload); err != nil {
		t.Fatalf("duplicate HandleWebhook: %v (a redelivery is a no-op success)", err)
	}
	second, err := h.subscriptions.GetByUserID(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID(duplicate): %v", err)
	}
	if !second.ValidUntil.Equal(*first.ValidUntil) {
		t.Errorf("valid until moved on the duplicate delivery: %v → %v", first.ValidUntil, second.ValidUntil)
	}
	transitions, err := h.transitions.ListBySubscriptionID(h.ctx(), sub.ID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID: %v", err)
	}
	applied := 0
	for _, tr := range transitions {
		if tr.Reason == domain.TransitionReasonPaymentApplied {
			applied++
		}
	}
	if applied != 1 {
		t.Errorf("payment_applied transitions = %d, want 1", applied)
	}
}

// TestPaymentFlow_PendingPaymentUniqueIndex proves the durable idempotency
// backstop: the partial unique index rejects a second pending payment for the
// same user/tariff/period at the database level.
func TestPaymentFlow_PendingPaymentUniqueIndex(t *testing.T) {
	t.Parallel()
	h := newPaymentIntegrationHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffBasic)

	first, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), sub.UserID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff(upgrade): %v", err)
	}

	second, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), sub.UserID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("repeat ChangeTariff: %v", err)
	}
	if second.PaymentID != first.PaymentID {
		t.Errorf("second payment = %v, want the existing pending %v", second.PaymentID, first.PaymentID)
	}

	count := h.countRows("SELECT COUNT(*) FROM subscription_payments WHERE user_id = $1", sub.UserID)
	if count != 1 {
		t.Errorf("payments = %d, want a single pending row", count)
	}
}
