package application

import (
	"testing"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// savedMethodMask is the masked card the external CIT deliveries carry in
// these scenarios.
const savedMethodMask = "5536********1448"

// getStateRecoveryToken is the charge token the GetState recovery scenarios
// restore the method from.
const getStateRecoveryToken = "rb_from_getstate"

// The saved-method side of payment notifications (issue #1512): the parent
// CIT payment carries the RebillId in its AUTHORIZED notification (ADR 0017),
// and the notification must produce the user's active payment method —
// otherwise the renewal worker has nothing to charge with after the first
// payment and the subscription drops into grace despite the bank having kept
// the card.

// deliverSavedMethodNotification feeds one payment notification through the
// webhook path.
func (h *paymentHarness) deliverSavedMethodNotification(t *testing.T, n *PaymentNotification) {
	t.Helper()
	h.setNotification(n)
	if err := h.payments.HandleWebhook(t.Context(), testProviderFake, []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook() error = %v", err)
	}
}

// savedMethodNotification builds a CIT notification that carries the saved
// method the provider created for the payment.
func savedMethodNotification(
	payment domain.SubscriptionPayment, status domain.PaymentStatus, token string,
) *PaymentNotification {
	n := &PaymentNotification{
		InternalPaymentID: payment.ID,
		ProviderPaymentID: *payment.ProviderPaymentID,
		Status:            status,
		AmountKopecks:     payment.AmountKopecks,
		CardMask:          savedMethodMask,
	}
	if token != "" {
		n.SavedMethod = &SavedMethod{
			ProviderMethodID: "card_" + token,
			ChargeToken:      token,
			MaskedPan:        savedMethodMask,
			ExpDate:          "0130",
			CustomerRef:      payment.UserID.String(),
		}
	}
	return n
}

// storedMethodByToken finds the user's method with the charge token.
func storedMethodByToken(
	t *testing.T, methods *fakePaymentMethodRepo, userID uuid.UUID, token string,
) domain.PaymentMethod {
	t.Helper()
	stored, err := methods.ListByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("ListByUserID() error = %v", err)
	}
	for _, m := range stored {
		if m.ProviderToken == token {
			return m
		}
	}
	t.Fatalf("no payment method with token %q: %+v", token, stored)
	return domain.PaymentMethod{}
}

// storedMethodsCount returns how many methods the user has.
func storedMethodsCount(t *testing.T, methods *fakePaymentMethodRepo, userID uuid.UUID) int {
	t.Helper()
	stored, err := methods.ListByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("ListByUserID() error = %v", err)
	}
	return len(stored)
}

// requireLinkedActiveMethod proves the subscription's charge target points at
// the method.
func requireLinkedActiveMethod(
	t *testing.T, subs *fakeSubscriptionRepo, userID, methodID uuid.UUID,
) {
	t.Helper()
	sub, err := subs.GetByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if sub.ActivePaymentMethodID == nil || *sub.ActivePaymentMethodID != methodID {
		t.Errorf("active payment method = %v, want %v", sub.ActivePaymentMethodID, methodID)
	}
}

// requireAuditActionOnce proves exactly one audit entry of the action was
// recorded — the convergence of repeated deliveries must stay silent.
func requireAuditActionOnce(t *testing.T, audit *captureRecorder, action auditdomain.Action) {
	t.Helper()
	n := 0
	for _, entry := range audit.recorded() {
		if entry.Action == action {
			n++
		}
	}
	if n != 1 {
		t.Errorf("audit %q entries = %d, want 1", action, n)
	}
}

// TestWebhook_PendingSavedMethodCreatesActiveMethod covers the core scenario
// of issue #1512: the AUTHORIZED notification of the parent CIT payment
// carries the RebillId, and applying it creates the user's active payment
// method linked to the subscription — while the payment itself stays pending
// and, per the issue #426 doctrine, carries no method on its own row.
func TestWebhook_PendingSavedMethodCreatesActiveMethod(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	payment := h.seedPendingUpgradeWithReference(t, sub)

	h.deliverSavedMethodNotification(t, savedMethodNotification(payment, domain.PaymentStatusPending, "rb_parent_1"))

	method := storedMethodByToken(t, h.stores.methods, sub.UserID, "rb_parent_1")
	if !method.IsActive {
		t.Error("saved method = inactive, want the user's single active one")
	}
	if method.Provider != domain.PaymentProvider(testProviderFake) {
		t.Errorf("method provider = %q, want %q", method.Provider, testProviderFake)
	}
	if method.ProviderCardID != "card_rb_parent_1" || method.DisplayMask != savedMethodMask || method.ExpDate != "0130" {
		t.Errorf("method display fields = %q/%q/%q, want the delivered ones", method.ProviderCardID, method.DisplayMask, method.ExpDate)
	}
	requireLinkedActiveMethod(t, h.stores.subscriptions, sub.UserID, method.ID)
	requireAuditActionOnce(t, h.audit, auditdomain.ActionPaymentMethodAdded)

	got, err := h.stores.payments.GetByID(t.Context(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Status != domain.PaymentStatusPending {
		t.Errorf("payment status = %q, want pending (the notification carries no outcome)", got.Status)
	}
	if got.PaymentMethodID != nil {
		t.Errorf("payment method id = %v, want nil (a customer-initiated payment carries no method, issue #426)", got.PaymentMethodID)
	}
}

// TestWebhook_PendingSavedMethodRedeliveryConverges covers repeated
// AUTHORIZED deliveries: the provider retries hourly until acked, and the
// redelivery must converge on the stored row without duplicating it or
// re-recording the audit.
func TestWebhook_PendingSavedMethodRedeliveryConverges(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	payment := h.seedPendingUpgradeWithReference(t, sub)

	h.deliverSavedMethodNotification(t, savedMethodNotification(payment, domain.PaymentStatusPending, "rb_parent_1"))
	h.deliverSavedMethodNotification(t, savedMethodNotification(payment, domain.PaymentStatusPending, "rb_parent_1"))

	if got := storedMethodsCount(t, h.stores.methods, sub.UserID); got != 1 {
		t.Errorf("payment methods = %d, want 1 after the redelivery", got)
	}
	method := storedMethodByToken(t, h.stores.methods, sub.UserID, "rb_parent_1")
	requireLinkedActiveMethod(t, h.stores.subscriptions, sub.UserID, method.ID)
	requireAuditActionOnce(t, h.audit, auditdomain.ActionPaymentMethodAdded)
}

// TestWebhook_SavedMethodOnSucceededNotification covers the CONFIRMED-first
// ordering seen in production: the terminal notification carries the saved
// method (the AUTHORIZED one can be lost), so the finalizing path must store
// it too — and a late AUTHORIZED redelivery of the same token stays a no-op.
func TestWebhook_SavedMethodOnSucceededNotification(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	payment := h.seedPendingUpgradeWithReference(t, sub)

	h.deliverSavedMethodNotification(t, savedMethodNotification(payment, domain.PaymentStatusSucceeded, "rb_confirm_first"))
	h.requireUpgradedSubscription(t, sub.UserID, payment.ID)
	h.requireFinalizedSucceeded(t, payment.ID)

	method := storedMethodByToken(t, h.stores.methods, sub.UserID, "rb_confirm_first")
	if !method.IsActive {
		t.Error("saved method = inactive, want active")
	}
	requireLinkedActiveMethod(t, h.stores.subscriptions, sub.UserID, method.ID)

	// The late AUTHORIZED delivery of the same token: pure convergence.
	h.deliverSavedMethodNotification(t, savedMethodNotification(payment, domain.PaymentStatusPending, "rb_confirm_first"))
	if got := storedMethodsCount(t, h.stores.methods, sub.UserID); got != 1 {
		t.Errorf("payment methods = %d, want 1 after the late AUTHORIZED delivery", got)
	}
	requireAuditActionOnce(t, h.audit, auditdomain.ActionPaymentMethodAdded)
}

// TestWebhook_NewCardReplacesActiveMethod covers the second payment with a
// different card: the new token becomes the single active method and the
// subscription's charge target, the replaced method stays stored but
// inactive — the same semantics as an explicit re-binding.
func TestWebhook_NewCardReplacesActiveMethod(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	old, err := domain.NewPaymentMethod(sub.UserID, domain.PaymentProvider(testProviderFake), "rb_old_card", h.now)
	if err != nil {
		t.Fatalf("NewPaymentMethod() error = %v", err)
	}
	if _, err := h.stores.methods.UpsertByTokenHash(t.Context(), old); err != nil {
		t.Fatalf("UpsertByTokenHash() error = %v", err)
	}
	if err := h.stores.methods.SetActive(t.Context(), sub.UserID, old.ID); err != nil {
		t.Fatalf("SetActive() error = %v", err)
	}
	payment := h.seedPendingUpgradeWithReference(t, sub)

	h.deliverSavedMethodNotification(t, savedMethodNotification(payment, domain.PaymentStatusPending, "rb_new_card"))

	fresh := storedMethodByToken(t, h.stores.methods, sub.UserID, "rb_new_card")
	if !fresh.IsActive {
		t.Error("new card method = inactive, want the active one")
	}
	replaced := storedMethodByToken(t, h.stores.methods, sub.UserID, "rb_old_card")
	if replaced.IsActive {
		t.Error("replaced card method = active, want inactive")
	}
	requireLinkedActiveMethod(t, h.stores.subscriptions, sub.UserID, fresh.ID)
}

// TestWebhook_FailedCITWithSavedMethodLeavesSubscriptionUntouched pins the
// grace guard (issue #426): the method lives on the subscription, not on the
// CIT payment row, so a declined customer-initiated payment fails alone and
// never pulls the subscription into grace — while the saved card stays
// chargeable for the next attempt.
func TestWebhook_FailedCITWithSavedMethodLeavesSubscriptionUntouched(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	payment := h.seedPendingUpgradeWithReference(t, sub)

	h.deliverSavedMethodNotification(t, savedMethodNotification(payment, domain.PaymentStatusPending, "rb_declined_later"))
	h.deliverSavedMethodNotification(t, savedMethodNotification(payment, domain.PaymentStatusFailed, ""))

	got, err := h.stores.payments.GetByID(t.Context(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Status != domain.PaymentStatusFailed {
		t.Errorf("payment status = %q, want failed", got.Status)
	}
	if got.PaymentMethodID != nil {
		t.Errorf("payment method id = %v, want nil on a customer-initiated payment", got.PaymentMethodID)
	}
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.Status != domain.SubscriptionStatusActive {
		t.Errorf("subscription status = %q, want active (a failed CIT must not enter grace)", stored.Status)
	}
	if stored.TariffID != sub.TariffID {
		t.Errorf("tariff changed to %v, want the untouched %v", stored.TariffID, sub.TariffID)
	}
	method := storedMethodByToken(t, h.stores.methods, sub.UserID, "rb_declined_later")
	if !method.IsActive {
		t.Error("saved method = inactive, want kept active (the bank kept the card)")
	}
}

// TestWebhook_SavedMethodWithoutChargeTokenIsIgnored covers the provider
// defect branch: a delivery whose saved method carries no charge token must
// neither store anything nor fail the payment application — no retry can fix
// it, and the payment outcome does not depend on the token.
func TestWebhook_SavedMethodWithoutChargeTokenIsIgnored(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	payment := h.seedPendingUpgradeWithReference(t, sub)

	n := savedMethodNotification(payment, domain.PaymentStatusPending, "")
	h.deliverSavedMethodNotification(t, n)

	if got := storedMethodsCount(t, h.stores.methods, sub.UserID); got != 0 {
		t.Errorf("payment methods = %d, want 0 from a token-less delivery", got)
	}
	got, err := h.stores.payments.GetByID(t.Context(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Status != domain.PaymentStatusPending {
		t.Errorf("payment status = %q, want pending", got.Status)
	}
}

// TestNotificationFromStatus_ChargeTokenBecomesSavedMethod proves the
// GetState translation carries the charge token as a saved method: the
// recovery path of a lost notification must be able to restore the token the
// provider still reports.
func TestNotificationFromStatus_ChargeTokenBecomesSavedMethod(t *testing.T) {
	t.Parallel()
	payment := domain.SubscriptionPayment{
		ID:                uuid.Must(uuid.NewV7()),
		ProviderPaymentID: new("prov_getstate"),
	}
	status := PaymentStatusResult{
		Status:      domain.PaymentStatusSucceeded,
		ChargeToken: getStateRecoveryToken,
		CardMask:    testMaskedPan,
	}

	n := notificationFromStatus(payment, status)

	if n.SavedMethod == nil {
		t.Fatal("notification carries no saved method, want the GetState charge token")
	}
	if n.SavedMethod.ChargeToken != getStateRecoveryToken || n.SavedMethod.MaskedPan != testMaskedPan {
		t.Errorf("saved method = %+v, want the GetState token and mask", n.SavedMethod)
	}
	if n.CardMask != testMaskedPan {
		t.Errorf("card mask = %q, want the provider-reported one", n.CardMask)
	}

	plain := notificationFromStatus(payment, PaymentStatusResult{Status: domain.PaymentStatusSucceeded})
	if plain.SavedMethod != nil {
		t.Errorf("notification without a charge token carries %+v, want nil", plain.SavedMethod)
	}
}

// TestReconciliation_ChargeTokenRecoversSavedMethod covers the lost-webhook
// backstop end to end over the workers: a stale pending CIT payment whose
// AUTHORIZED delivery was lost is resolved through GetState, and the charge
// token the provider reports becomes the user's active method.
func TestReconciliation_ChargeTokenRecoversSavedMethod(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, nil)
	payment := h.seedStalePendingUpgrade(t, sub, "prov_rb_recover")
	h.provider.statusFn = func(uuid.UUID, string) (PaymentStatusResult, error) {
		return PaymentStatusResult{
			Status:      domain.PaymentStatusSucceeded,
			ChargeToken: getStateRecoveryToken,
			CardMask:    testMaskedPan,
		}, nil
	}

	count, err := h.workers.ReconcilePendingPayments(t.Context(), h.now)
	if err != nil {
		t.Fatalf("ReconcilePendingPayments() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}

	method := storedMethodByToken(t, h.stores.methods, sub.UserID, getStateRecoveryToken)
	if !method.IsActive || method.DisplayMask != testMaskedPan {
		t.Errorf("method = %+v, want active with the GetState mask", method)
	}
	requireLinkedActiveMethod(t, h.stores.subscriptions, sub.UserID, method.ID)

	got, err := h.stores.payments.GetByID(t.Context(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Status != domain.PaymentStatusSucceeded {
		t.Errorf("payment status = %q, want succeeded", got.Status)
	}
}
