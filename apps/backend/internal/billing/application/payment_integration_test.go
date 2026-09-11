//go:build integration

package application_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// testProviderFake is the provider identity of the webhook fixtures; it
// lives in the integration build — the only one that uses it.
const testProviderFake = "fake"

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

// TestPaymentFlow_CancelledSameTariffReactivationEndToEnd proves the
// reactivation of issue #429 against real PostgreSQL through the full webhook
// path: a cancelled subscription paying for the plan it is already on goes
// through the payment flow (no "already on this tariff" rejection), and the
// confirmed payment returns it to active with auto-renew on, the period
// counted from the payment moment, and the transition log recording the move
// out of cancelled.
func TestPaymentFlow_CancelledSameTariffReactivationEndToEnd(t *testing.T) {
	t.Parallel()
	h := newPaymentIntegrationHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffPro)
	if err := h.subscriptionsSvc.CancelSubscription(h.ctx(), sub.UserID, nil); err != nil {
		t.Fatalf("CancelSubscription(): %v", err)
	}

	result, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), sub.UserID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff(reactivation): %v", err)
	}
	if result.PaymentID == uuid.Nil || result.ConfirmURL == "" {
		t.Fatalf("result = %+v, want a payment id and a payer url", result)
	}
	payment, err := h.payments.GetByID(h.ctx(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if payment.AmountKopecks != 49000 {
		t.Fatalf("amount = %d, want the full pro month price 49000", payment.AmountKopecks)
	}

	h.confirmFakePayment(t, payment.ID)

	finalized, err := h.payments.GetByID(h.ctx(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID(finalized): %v", err)
	}
	if finalized.Status != domain.PaymentStatusSucceeded {
		t.Fatalf("payment status = %q, want succeeded", finalized.Status)
	}
	h.requireAppliedProSubscription(t, sub.UserID, payment.ID)
	h.requireCancelledReactivationTransition(t, sub.ID, payment.ID)
}

// requireCancelledReactivationTransition proves the transition log carries
// exactly one payment_applied entry for the reactivation payment, recording
// the move out of cancelled next to the cancellation entry that preceded it
// (issue #429).
func (h *paymentIntegrationHarness) requireCancelledReactivationTransition(
	t *testing.T, subscriptionID, paymentID uuid.UUID,
) {
	t.Helper()
	transitions, err := h.transitions.ListBySubscriptionID(h.ctx(), subscriptionID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID: %v", err)
	}
	applied := 0
	for _, tr := range transitions {
		if tr.Reason != domain.TransitionReasonPaymentApplied {
			continue
		}
		applied++
		if tr.PaymentID == nil || *tr.PaymentID != paymentID {
			t.Errorf("payment_applied entry references %v, want %v", tr.PaymentID, paymentID)
		}
		if tr.FromStatus == nil || *tr.FromStatus != domain.SubscriptionStatusCancelled {
			t.Errorf("payment_applied from_status = %v, want cancelled", tr.FromStatus)
		}
	}
	if applied != 1 {
		t.Errorf("payment_applied transitions = %d, want 1 (in %+v)", applied, transitions)
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

// laterPaymentID returns a UUIDv7 whose timestamp is one millisecond after
// id's — an id guaranteed to sort right after it (the creation order v7 ids
// carry), minted without sleeping off the real clock uuid.NewV7 reads.
func laterPaymentID(id uuid.UUID) uuid.UUID {
	next := id
	// The leading six bytes are the 48-bit big-endian unix-millisecond
	// timestamp; increment it with a carry towards the most significant byte.
	for i := 5; i >= 0; i-- {
		next[i]++
		if next[i] != 0 {
			break
		}
	}
	return next
}

// seedStaleMitCharge seeds the precondition of a stale renewal failure: an
// expired pro subscription with a linked payment method and a pending
// merchant-initiated renewal charge hanging at the provider (issue #426).
func (h *paymentIntegrationHarness) seedStaleMitCharge(t *testing.T) (domain.Subscription, domain.SubscriptionPayment) {
	t.Helper()
	userID := h.seedUser()
	expired := h.clock.Now().AddDate(0, -1, 0)
	period := domain.PeriodMonth
	proID := h.tariffIDByName(t, domain.TariffPro)
	sub, err := domain.NewBasicSubscription(userID, proID)
	if err != nil {
		t.Fatalf("NewBasicSubscription(): %v", err)
	}
	sub.TariffID = proID
	sub.ValidUntil = &expired
	sub.AutoRenewEnabled = true
	sub.CurrentPeriod = &period
	sub, err = h.subscriptions.Create(h.ctx(), sub)
	if err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
	method, err := domain.NewPaymentMethod(userID, testProviderFake, "token_stale", h.clock.Now())
	if err != nil {
		t.Fatalf("new method: %v", err)
	}
	storedMethod, err := h.methods.UpsertByTokenHash(h.ctx(), method)
	if err != nil {
		t.Fatalf("seed method: %v", err)
	}
	stale, err := domain.NewSubscriptionPayment(
		userID, sub.ID, proID, domain.PeriodMonth, 49000, testProviderFake,
		h.clock.Now().Add(-time.Minute))
	if err != nil {
		t.Fatalf("new stale payment: %v", err)
	}
	stale.PaymentMethodID = &storedMethod.ID
	if err := stale.SaveProviderReference("prov_stale", "", h.clock.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("save stale reference: %v", err)
	}
	stale, err = h.payments.Create(h.ctx(), stale)
	if err != nil {
		t.Fatalf("seed stale payment: %v", err)
	}
	return sub, stale
}

// deliverWebhook delivers one raw fake-provider payment notification through
// the synchronous webhook path.
func (h *paymentIntegrationHarness) deliverWebhook(
	t *testing.T, providerPaymentID string, status domain.PaymentStatus, internalID uuid.UUID,
) {
	t.Helper()
	payload := `{"provider_payment_id":"` + providerPaymentID +
		`","internal_payment_id":"` + internalID.String() +
		`","status":"` + string(status) + `"`
	if status == domain.PaymentStatusFailed {
		payload += `,"error_code":"card_declined"`
	}
	payload += `}`
	if err := h.paymentsSvc.HandleWebhook(h.ctx(), testProviderFake, []byte(payload)); err != nil {
		t.Fatalf("HandleWebhook(%s %s): %v", providerPaymentID, status, err)
	}
}

// applyDowngradeThroughWebhook covers the webhook half of the shared-seam
// scenario (issue #428): a pending merchant-initiated pro charge of a business
// subscription (the scheduled-downgrade renewal) succeeds at the provider and
// the webhook delivers it. It returns the payer.
func applyDowngradeThroughWebhook(t *testing.T, h *paymentIntegrationHarness, pro domain.Tariff) uuid.UUID {
	t.Helper()
	sub := h.seedPaidSubscription(t, domain.TariffBusiness)
	payment, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, pro.ID, domain.PeriodMonth,
		49000, testProviderFake, h.clock.Now())
	if err != nil {
		t.Fatalf("new pro payment: %v", err)
	}
	if err := payment.SaveProviderReference("prov_downgrade_webhook", "", h.clock.Now()); err != nil {
		t.Fatalf("save reference: %v", err)
	}
	if _, err := h.payments.Create(h.ctx(), payment); err != nil {
		t.Fatalf("seed pro payment: %v", err)
	}
	h.deliverWebhook(t, "prov_downgrade_webhook", domain.PaymentStatusSucceeded, payment.ID)

	stored, err := h.subscriptions.GetByUserID(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID(webhook): %v", err)
	}
	if stored.TariffID != pro.ID {
		t.Errorf("webhook tariff = %v, want pro applied", stored.TariffID)
	}
	return sub.UserID
}

// applyDowngradeThroughWorker covers the worker half of the shared-seam
// scenario (issue #428): an expired business subscription with a due scheduled
// downgrade to pro is charged by the renewal phase and the success is applied
// through the same seam the webhook ends in. It returns the payer.
func applyDowngradeThroughWorker(t *testing.T, h *paymentIntegrationHarness, pro, business domain.Tariff) uuid.UUID {
	t.Helper()
	userID, sub := seedPaidProSubscription(t, h.integrationHarness)
	expired := h.clock.Now().Add(-2 * time.Hour)
	sub.TariffID = business.ID
	sub.ValidUntil = &expired
	if err := h.subscriptions.Update(h.ctx(), sub); err != nil {
		t.Fatalf("set business: %v", err)
	}
	if _, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), userID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	}); err != nil {
		t.Fatalf("ChangeTariff(pro) error = %v", err)
	}
	seedActiveMethod(t, h.integrationHarness, userID, "tok_seam_worker")
	if _, err := h.services.Workers.ProcessScheduledChanges(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessScheduledChanges(): %v", err)
	}
	if _, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessRenewals(): %v", err)
	}

	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID(worker): %v", err)
	}
	if stored.TariffID != pro.ID {
		t.Errorf("worker tariff = %v, want pro applied", stored.TariffID)
	}
	return userID
}

// applyDowngradeThroughReconciliation covers the reconciliation half of the
// shared-seam scenario (issue #428): a stale pending pro charge of a business
// subscription the provider already settled is finalized by
// ReconcilePendingPayments from the provider's status. It returns the payer.
func applyDowngradeThroughReconciliation(t *testing.T, h *paymentIntegrationHarness, pro domain.Tariff) uuid.UUID {
	t.Helper()
	sub := h.seedPaidSubscription(t, domain.TariffBusiness)
	payment, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, pro.ID, domain.PeriodMonth,
		49000, testProviderFake, h.clock.Now())
	if err != nil {
		t.Fatalf("new pro payment: %v", err)
	}
	if err := payment.SaveProviderReference("prov_downgrade_reconcile", "", h.clock.Now()); err != nil {
		t.Fatalf("save reference: %v", err)
	}
	if _, err := h.payments.Create(h.ctx(), payment); err != nil {
		t.Fatalf("seed pro payment: %v", err)
	}
	// The provider settled the charge, but the webhook was lost; the row goes
	// stale and the reconciliation worker resolves it from the status.
	h.provider.SetPaymentState(payment.ID.String(), billingapp.PaymentStatusResult{Status: domain.PaymentStatusSucceeded})
	agePaymentRow(t, h.integrationHarness, payment.ID)
	if count, err := h.services.Workers.ReconcilePendingPayments(h.ctx(), h.clock.Now()); err != nil || count != 1 {
		t.Fatalf("ReconcilePendingPayments() = %d (err %v), want 1 stale payment reconciled", count, err)
	}

	stored, err := h.subscriptions.GetByUserID(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID(reconciliation): %v", err)
	}
	if stored.TariffID != pro.ID {
		t.Errorf("reconciliation tariff = %v, want pro applied", stored.TariffID)
	}
	return sub.UserID
}

// TestPaymentFlow_SucceededDowngradeArchivesExcessOnEveryPath proves the
// shared success-application seam of issue #428 against real PostgreSQL: a
// succeeded payment that lowers the tariff limit enforces the limit through
// the lifecycle bridges identically on every delivery path — the webhook, the
// renewal worker's charge and the lost-webhook reconciliation — one archive
// call at the new plan's limit per payer, with the same renewal trigger.
func TestPaymentFlow_SucceededDowngradeArchivesExcessOnEveryPath(t *testing.T) {
	t.Parallel()
	h := newPaymentIntegrationHarness(t)
	archiver := &capturingArchiver{}
	slots := &capturingSlots{}
	// The bridges go to the payment service — the seam every success path,
	// the webhook and the worker's lifecycle port alike, ends in.
	h.paymentsSvc.SetLifecycleBridges(archiver, slots)
	pro, err := h.tariffs.GetByName(h.ctx(), domain.TariffPro)
	if err != nil {
		t.Fatalf("GetByName(pro): %v", err)
	}
	business, err := h.tariffs.GetByName(h.ctx(), domain.TariffBusiness)
	if err != nil {
		t.Fatalf("GetByName(business): %v", err)
	}

	payers := []uuid.UUID{
		applyDowngradeThroughWebhook(t, h, pro),
		applyDowngradeThroughWorker(t, h, pro, business),
		applyDowngradeThroughReconciliation(t, h, pro),
	}

	// Every path enforced the pro limit through the bridges, once per payer
	// and nothing else.
	got := archiver.recorded()
	if len(got) != len(payers) {
		t.Fatalf("archive calls = %+v, want %d (one per payer)", got, len(payers))
	}
	seen := make(map[uuid.UUID]int, len(payers))
	for _, call := range got {
		if call.limit != pro.ActivePropertyLimit {
			t.Errorf("archive call %+v: limit = %d, want the pro limit %d", call, call.limit, pro.ActivePropertyLimit)
		}
		seen[call.ownerID]++
	}
	for _, payer := range payers {
		if seen[payer] != 1 {
			t.Errorf("payer %s archived %d times, want exactly 1", payer, seen[payer])
		}
	}
	for _, trigger := range slots.recorded() {
		if trigger != "renewal_downgrade" {
			t.Errorf("slot trigger = %q, want renewal_downgrade", trigger)
		}
	}
}

// TestPaymentFlow_StaleFailedRenewalAfterManualRenewal proves the freshness
// guard of issue #426 end to end against real PostgreSQL: a merchant-initiated
// renewal charge hangs at the provider, a newer manual renewal lands after it,
// and the old charge's late failed webhook arrives last. The payment fails,
// the subscription stays active on the renewed period, and the transition log
// carries no grace entry.
func TestPaymentFlow_StaleFailedRenewalAfterManualRenewal(t *testing.T) {
	t.Parallel()
	h := newPaymentIntegrationHarness(t)
	sub, stale := h.seedStaleMitCharge(t)

	// While the old charge hangs, a newer manual payment renews the
	// subscription through the webhook path — the year period of the manual
	// payment is what makes it coexist with the pending monthly charge (one
	// pending payment per user/tariff/period). The renewal's id is minted one
	// v7 millisecond after the stale charge's, so the creation order the
	// freshness guard reads is deterministic.
	renewal, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, h.tariffIDByName(t, domain.TariffPro),
		domain.PeriodYear, 440000, testProviderFake, h.clock.Now())
	if err != nil {
		t.Fatalf("new renewal payment: %v", err)
	}
	renewal.ID = laterPaymentID(stale.ID)
	if err := renewal.SaveProviderReference("prov_renewal", "", h.clock.Now()); err != nil {
		t.Fatalf("save renewal reference: %v", err)
	}
	renewal, err = h.payments.Create(h.ctx(), renewal)
	if err != nil {
		t.Fatalf("seed renewal payment: %v", err)
	}
	h.deliverWebhook(t, "prov_renewal", domain.PaymentStatusSucceeded, renewal.ID)

	// The old charge finally fails at the provider.
	h.deliverWebhook(t, "prov_stale", domain.PaymentStatusFailed, stale.ID)

	failed, err := h.payments.GetByID(h.ctx(), stale.ID)
	if err != nil {
		t.Fatalf("GetByID(stale): %v", err)
	}
	if failed.Status != domain.PaymentStatusFailed {
		t.Errorf("stale payment status = %q, want failed", failed.Status)
	}
	stored, err := h.subscriptions.GetByUserID(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if stored.Status != domain.SubscriptionStatusActive {
		t.Errorf("subscription status = %q, want active (a stale failure must not enter grace)", stored.Status)
	}
	if stored.LastAppliedPaymentID == nil || *stored.LastAppliedPaymentID != renewal.ID {
		t.Errorf("last applied payment = %v, want the newer renewal", stored.LastAppliedPaymentID)
	}
	transitions, err := h.transitions.ListBySubscriptionID(h.ctx(), sub.ID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID: %v", err)
	}
	for _, tr := range transitions {
		if tr.Reason == domain.TransitionReasonGraceEntered {
			t.Errorf("transitions carry a grace_entered entry for a stale failure: %+v", tr)
		}
	}
}
