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

// The refund-saga integration scenarios of issue #254 against real PostgreSQL
// and the fake provider adapter: the three-phase saga with its reservation
// guards, the reconciliation of stuck refunds, the manual sync and the admin
// payment views with their filters.

// refundIntegrationHarness extends the payment harness with an acting admin
// user (the audit log references the actor).
type refundIntegrationHarness struct {
	*paymentIntegrationHarness
	adminID uuid.UUID
}

func newRefundIntegrationHarness(t *testing.T) *refundIntegrationHarness {
	t.Helper()
	h := &refundIntegrationHarness{paymentIntegrationHarness: newPaymentIntegrationHarness(t)}
	h.adminID = h.seedUser()
	return h
}

// succeededUpgradePayment drives a full upgrade to the business plan through
// the payment flow and returns the succeeded payment — the captured charge
// every refund starts from.
func (h *refundIntegrationHarness) succeededUpgradePayment(t *testing.T) domain.SubscriptionPayment {
	t.Helper()
	sub := h.seedPaidSubscription(t, domain.TariffBasic)
	result, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), sub.UserID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffBusiness,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff(upgrade): %v", err)
	}
	h.confirmFakePayment(t, result.PaymentID)
	payment, err := h.payments.GetByID(h.ctx(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if payment.Status != domain.PaymentStatusSucceeded {
		t.Fatalf("payment status = %q, want succeeded", payment.Status)
	}
	return payment
}

// transitionsOf loads the subscription's transition log, newest first.
func (h *refundIntegrationHarness) transitionsOf(t *testing.T, subscriptionID uuid.UUID) []domain.Transition {
	t.Helper()
	transitions, err := h.transitions.ListBySubscriptionID(h.ctx(), subscriptionID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID: %v", err)
	}
	return transitions
}

// requireRefundedPaymentShape asserts the stored payment is refunded in full.
func (h *refundIntegrationHarness) requireRefundedPaymentShape(t *testing.T, payment domain.SubscriptionPayment) {
	t.Helper()
	stored, err := h.payments.GetByID(h.ctx(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID(refunded): %v", err)
	}
	if stored.Status != domain.PaymentStatusRefunded {
		t.Fatalf("payment status = %q, want refunded", stored.Status)
	}
	if stored.RefundedAmountKopecks == nil || *stored.RefundedAmountKopecks != stored.AmountKopecks {
		t.Fatalf("refunded amount = %v, want the full %d", stored.RefundedAmountKopecks, stored.AmountKopecks)
	}
}

// requireRefundDowngradedSubscription asserts the payer's subscription fell to
// the basic tariff with the validity window and auto-renew cleared, and
// returns it for the transition check.
func (h *refundIntegrationHarness) requireRefundDowngradedSubscription(t *testing.T, userID uuid.UUID) domain.Subscription {
	t.Helper()
	subStored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if subStored.TariffID != h.tariffIDByName(t, domain.TariffBasic) {
		t.Fatalf("tariff = %v, want basic after the refund", subStored.TariffID)
	}
	if subStored.ValidUntil != nil || subStored.AutoRenewEnabled {
		t.Fatalf("subscription = valid_until %v, auto-renew %t; want both cleared", subStored.ValidUntil, subStored.AutoRenewEnabled)
	}
	return subStored
}

// requireRefundTransitionAudit asserts the transition log's newest entry is
// the refund with the admin initiator and the refunded payment, and the audit
// log records the refund attributed to the admin.
func (h *refundIntegrationHarness) requireRefundTransitionAudit(
	t *testing.T, subStored domain.Subscription, payment domain.SubscriptionPayment,
) {
	t.Helper()
	// The transition log records the downgrade with the admin initiator and
	// the refunded payment.
	transitions := h.transitionsOf(t, subStored.ID)
	if len(transitions) == 0 {
		t.Fatal("no transitions recorded")
	}
	refundTransition := transitions[0]
	if refundTransition.Reason != domain.TransitionReasonRefunded {
		t.Fatalf("latest transition reason = %q, want refunded", refundTransition.Reason)
	}
	if refundTransition.Initiator != domain.InitiatorAdmin || refundTransition.InitiatorID == nil ||
		*refundTransition.InitiatorID != h.adminID {
		t.Fatalf("refund transition initiator = %q/%v, want the acting admin", refundTransition.Initiator, refundTransition.InitiatorID)
	}
	if refundTransition.PaymentID == nil || *refundTransition.PaymentID != payment.ID {
		t.Fatalf("refund transition payment = %v, want %v", refundTransition.PaymentID, payment.ID)
	}

	// The audit log records the refund attributed to the admin.
	refundAudit := h.countRows(
		"SELECT COUNT(*) FROM audit_log WHERE action = 'subscription_payment.refunded' AND actor_id = $1 AND entity_id = $2",
		h.adminID, payment.ID)
	if refundAudit != 1 {
		t.Fatalf("refund audit entries = %d, want 1", refundAudit)
	}
}

// TestRefundFlow_SucceedsAndDowngradesToBasic proves the full saga against
// real PostgreSQL (issue #254): the succeeded payment is refunded in full,
// the subscription falls to the basic tariff with its excess objects handled
// by the lifecycle bridges, and both the transition log and the audit log
// attribute the refund to the acting admin.
func TestRefundFlow_SucceedsAndDowngradesToBasic(t *testing.T) {
	t.Parallel()
	h := newRefundIntegrationHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffBasic)
	result, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), sub.UserID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffBusiness,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff(upgrade): %v", err)
	}
	h.confirmFakePayment(t, result.PaymentID)
	payment, err := h.payments.GetByID(h.ctx(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if err := h.paymentsSvc.RefundPayment(h.ctx(), h.adminID, payment.ID); err != nil {
		t.Fatalf("RefundPayment: %v", err)
	}

	h.requireRefundedPaymentShape(t, payment)
	subStored := h.requireRefundDowngradedSubscription(t, sub.UserID)
	h.requireRefundTransitionAudit(t, subStored, payment)
}

// TestRefundFlow_SupersededPaymentKeepsSubscription proves the currency guard
// of issue #430 against real PostgreSQL: refunding a payment a later payment
// superseded — an earlier pro charge after a newer business upgrade was
// applied — returns the money in full but leaves the subscription on the
// newer payment's tariff and period, with no refund entry in the transition
// log; the audit log still records the admin-attributed refund.
func TestRefundFlow_SupersededPaymentKeepsSubscription(t *testing.T) {
	t.Parallel()
	h := newRefundIntegrationHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffBasic)
	proResult, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), sub.UserID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff(pro): %v", err)
	}
	h.confirmFakePayment(t, proResult.PaymentID)
	proPayment, err := h.payments.GetByID(h.ctx(), proResult.PaymentID)
	if err != nil {
		t.Fatalf("GetByID(pro): %v", err)
	}
	// A newer business payment buys the current paid period.
	businessResult, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), sub.UserID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffBusiness,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff(business): %v", err)
	}
	h.confirmFakePayment(t, businessResult.PaymentID)
	applied, err := h.subscriptions.GetByUserID(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID(applied): %v", err)
	}
	wantUntil := applied.ValidUntil
	transitionsBefore := h.transitionsOf(t, sub.ID)

	if err := h.paymentsSvc.RefundPayment(h.ctx(), h.adminID, proPayment.ID); err != nil {
		t.Fatalf("RefundPayment: %v", err)
	}

	h.requireRefundedPaymentShape(t, proPayment)
	stored, err := h.subscriptions.GetByUserID(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID(final): %v", err)
	}
	if stored.TariffID != h.tariffIDByName(t, domain.TariffBusiness) {
		t.Fatalf("tariff = %v, want business (a superseded refund must not downgrade)", stored.TariffID)
	}
	if stored.ValidUntil == nil || !stored.ValidUntil.Equal(*wantUntil) {
		t.Fatalf("valid until = %v, want the newer payment's %v", stored.ValidUntil, wantUntil)
	}
	if got := h.transitionsOf(t, sub.ID); len(got) != len(transitionsBefore) {
		t.Fatalf("transitions = %d, want the pre-refund %d (no refund downgrade entry)", len(got), len(transitionsBefore))
	}
	refundAudit := h.countRows(
		"SELECT COUNT(*) FROM audit_log WHERE action = 'subscription_payment.refunded' AND actor_id = $1 AND entity_id = $2",
		h.adminID, proPayment.ID)
	if refundAudit != 1 {
		t.Fatalf("refund audit entries = %d, want 1", refundAudit)
	}
}

// TestRefundFlow_DoubleRefundRejected proves the reservation guard on real
// PostgreSQL: the second refund of the same payment is rejected and the
// provider is never called twice.
func TestRefundFlow_DoubleRefundRejected(t *testing.T) {
	t.Parallel()
	h := newRefundIntegrationHarness(t)
	payment := h.succeededUpgradePayment(t)

	if err := h.paymentsSvc.RefundPayment(h.ctx(), h.adminID, payment.ID); err != nil {
		t.Fatalf("first RefundPayment: %v", err)
	}
	err := h.paymentsSvc.RefundPayment(h.ctx(), h.adminID, payment.ID)
	if !errors.Is(err, domain.ErrInvalidPaymentStatus) {
		t.Fatalf("second RefundPayment err = %v, want ErrInvalidPaymentStatus", err)
	}
}

// TestRefundFlow_ProviderErrorRevertsReservation proves the compensation
// transaction: a refused provider refund rolls the reservation back, leaving
// the payment refundable.
func TestRefundFlow_ProviderErrorRevertsReservation(t *testing.T) {
	t.Parallel()
	h := newRefundIntegrationHarness(t)
	payment := h.succeededUpgradePayment(t)
	h.provider.SetRefundOutcome(payment.ID.String(), nil, errors.New("provider is down"))

	if err := h.paymentsSvc.RefundPayment(h.ctx(), h.adminID, payment.ID); err == nil {
		t.Fatal("RefundPayment error = nil, want the provider failure")
	}

	stored, err := h.payments.GetByID(h.ctx(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.Status != domain.PaymentStatusSucceeded {
		t.Fatalf("payment status = %q, want succeeded restored", stored.Status)
	}
}

// TestRefundFlow_StuckReservationReconciledByWorker proves the сторож of the
// saga (issue #254): an in-flight refund keeps the reservation, and once the
// staleness window passes the reconciliation worker resolves it from the
// provider's status — finalizing the refund with its subscription effects.
func TestRefundFlow_StuckReservationReconciledByWorker(t *testing.T) {
	t.Parallel()
	h := newRefundIntegrationHarness(t)
	payment := h.succeededUpgradePayment(t)
	h.provider.SetRefundOutcome(payment.ID.String(), &billingapp.RefundResult{
		ProviderPaymentID:     *payment.ProviderPaymentID,
		Status:                domain.PaymentStatusRefunding,
		RefundedAmountKopecks: 0,
	}, nil)

	if err := h.paymentsSvc.RefundPayment(h.ctx(), h.adminID, payment.ID); err != nil {
		t.Fatalf("RefundPayment: %v", err)
	}
	stored, err := h.payments.GetByID(h.ctx(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.Status != domain.PaymentStatusRefunding {
		t.Fatalf("payment status = %q, want the reservation kept", stored.Status)
	}

	// The provider settles the refund; the worker picks the reservation up
	// after the staleness window. The reservation's updated_at comes from the
	// database clock (the set_updated_at trigger), so the fake clock moves
	// beyond the real now plus the staleness to make the reservation stale.
	h.provider.SetPaymentState(payment.ID.String(), billingapp.PaymentStatusResult{Status: domain.PaymentStatusRefunded})
	afterStaleness := time.Now().UTC().Add(2 * billingapp.DefaultConfig().PendingPaymentStaleness)
	h.clock.now = afterStaleness
	processed, err := h.services.Workers.ReconcileStaleRefunds(h.ctx(), afterStaleness)
	if err != nil {
		t.Fatalf("ReconcileStaleRefunds: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}

	stored, err = h.payments.GetByID(h.ctx(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID(finalized): %v", err)
	}
	if stored.Status != domain.PaymentStatusRefunded {
		t.Fatalf("payment status = %q, want refunded", stored.Status)
	}
	subStored, err := h.subscriptions.GetByUserID(h.ctx(), payment.UserID)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if subStored.TariffID != h.tariffIDByName(t, domain.TariffBasic) {
		t.Fatalf("tariff = %v, want basic after the reconciled refund", subStored.TariffID)
	}
	// The reconciled refund is attributed to the system — no admin request
	// was in flight when the worker resolved the reservation.
	transitions := h.transitionsOf(t, subStored.ID)
	if len(transitions) == 0 || transitions[0].Reason != domain.TransitionReasonRefunded ||
		transitions[0].Initiator != domain.InitiatorSystem {
		t.Fatalf("latest transition = %+v, want a system-initiated refund entry", transitions[0])
	}
}

// TestRefundFlow_PartialRefundAnomalyStaysRefunding proves the anomaly
// handling: the system always refunds the full amount, so a provider
// answering with a partial amount keeps the reservation for review.
func TestRefundFlow_PartialRefundAnomalyStaysRefunding(t *testing.T) {
	t.Parallel()
	h := newRefundIntegrationHarness(t)
	payment := h.succeededUpgradePayment(t)
	h.provider.SetRefundOutcome(payment.ID.String(), &billingapp.RefundResult{
		ProviderPaymentID:     *payment.ProviderPaymentID,
		Status:                domain.PaymentStatusRefunded,
		RefundedAmountKopecks: payment.AmountKopecks - 1,
	}, nil)

	if err := h.paymentsSvc.RefundPayment(h.ctx(), h.adminID, payment.ID); err != nil {
		t.Fatalf("RefundPayment: %v", err)
	}
	stored, err := h.payments.GetByID(h.ctx(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.Status != domain.PaymentStatusRefunding {
		t.Fatalf("payment status = %q, want the reservation kept for review", stored.Status)
	}
	if stored.RefundedAmountKopecks != nil {
		t.Fatalf("refunded amount = %v, want none recorded", stored.RefundedAmountKopecks)
	}
}

// TestSyncPayment_AppliesProviderRefund proves the manual sync resolves a
// provider-side refund the webhook delivery lost (issue #254).
func TestSyncPayment_AppliesProviderRefund(t *testing.T) {
	t.Parallel()
	h := newRefundIntegrationHarness(t)
	payment := h.succeededUpgradePayment(t)
	h.provider.SetPaymentState(payment.ID.String(), billingapp.PaymentStatusResult{Status: domain.PaymentStatusRefunded})

	if err := h.paymentsSvc.SyncPayment(h.ctx(), h.adminID, payment.ID); err != nil {
		t.Fatalf("SyncPayment: %v", err)
	}

	stored, err := h.payments.GetByID(h.ctx(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.Status != domain.PaymentStatusRefunded {
		t.Fatalf("payment status = %q, want refunded", stored.Status)
	}
	subStored, err := h.subscriptions.GetByUserID(h.ctx(), payment.UserID)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if subStored.TariffID != h.tariffIDByName(t, domain.TariffBasic) {
		t.Fatalf("tariff = %v, want basic after the synced refund", subStored.TariffID)
	}
	synced := h.countRows(
		"SELECT COUNT(*) FROM audit_log WHERE action = 'subscription_payment.synced' AND actor_id = $1 AND entity_id = $2",
		h.adminID, payment.ID)
	if synced != 1 {
		t.Fatalf("synced audit entries = %d, want 1", synced)
	}
}

// seedPendingPayment seeds a pending upgrade payment of a payer still on a
// paid tariff and returns it.
func (h *refundIntegrationHarness) seedPendingPayment(t *testing.T) domain.SubscriptionPayment {
	t.Helper()
	pendingSub := h.seedPaidSubscription(t, domain.TariffPro)
	pendingResult, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), pendingSub.UserID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffBusiness,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff(upgrade): %v", err)
	}
	pendingPayment, err := h.payments.GetByID(h.ctx(), pendingResult.PaymentID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	return pendingPayment
}

// refundedPayment seeds an upgraded payment, finalizes it through the webhook,
// refunds it, and returns the stored payment.
func (h *refundIntegrationHarness) refundedPayment(t *testing.T) domain.SubscriptionPayment {
	t.Helper()
	payment := h.succeededUpgradePayment(t)
	if err := h.paymentsSvc.RefundPayment(h.ctx(), h.adminID, payment.ID); err != nil {
		t.Fatalf("RefundPayment: %v", err)
	}
	stored, err := h.payments.GetByID(h.ctx(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID(refunded): %v", err)
	}
	return stored
}

// requireListingCarriesBothPayments asserts the unfiltered listing carries
// both seeded payments with their tariffs resolved.
func (h *refundIntegrationHarness) requireListingCarriesBothPayments(
	t *testing.T, refundedPayment, pendingPayment domain.SubscriptionPayment,
) {
	t.Helper()
	views, total, err := h.paymentsSvc.ListAdminPayments(h.ctx(), billingapp.AdminPaymentFilters{})
	if err != nil {
		t.Fatalf("ListAdminPayments: %v", err)
	}
	if total < 2 || len(views) < 2 {
		t.Fatalf("total/views = %d/%d, want at least 2", total, len(views))
	}
	byID := make(map[uuid.UUID]billingapp.AdminSubscriptionPaymentView, len(views))
	for _, v := range views {
		byID[v.Payment.ID] = v
	}
	if got, ok := byID[refundedPayment.ID]; !ok {
		t.Fatal("the refunded payment is missing from the listing")
	} else if got.Tariff.Name != domain.TariffBusiness {
		t.Fatalf("refunded payment tariff = %q, want business", got.Tariff.Name)
	}
	if _, ok := byID[pendingPayment.ID]; !ok {
		t.Fatal("the pending payment is missing from the listing")
	}
}

// requireGraceFilterFindsNone asserts the subscription-status filter narrows
// to none: both seeded payers are back on active subscriptions.
func (h *refundIntegrationHarness) requireGraceFilterFindsNone(t *testing.T) {
	t.Helper()
	// The subscription-status filter: the refunded payer is back on the basic
	// (active) subscription, the pending payer is still on the paid active
	// one — both active, so grace narrows to none.
	_, graceTotal, err := h.paymentsSvc.ListAdminPayments(h.ctx(), billingapp.AdminPaymentFilters{
		SubscriptionStatus: string(domain.SubscriptionStatusGrace),
	})
	if err != nil {
		t.Fatalf("ListAdminPayments(grace): %v", err)
	}
	if graceTotal != 0 {
		t.Fatalf("grace-filtered total = %d, want 0", graceTotal)
	}
}

// requireFilterReturnsExactly asserts the filter narrows the listing to
// exactly the wanted payment: label names the filter, subject names the
// payment in the failure message.
func (h *refundIntegrationHarness) requireFilterReturnsExactly(
	t *testing.T, label, subject string, filters billingapp.AdminPaymentFilters, wantID uuid.UUID,
) {
	t.Helper()
	views, total, err := h.paymentsSvc.ListAdminPayments(h.ctx(), filters)
	if err != nil {
		t.Fatalf("ListAdminPayments(%s): %v", label, err)
	}
	if total != 1 || len(views) != 1 || views[0].Payment.ID != wantID {
		t.Fatalf("%s filter = %d/%v, want exactly the %s payment", label, total, views, subject)
	}
}

// TestAdminPayments_ListAndFilters proves the admin payment views (issue
// #254): the listing resolves the payer's phone (plaintext and encrypted),
// the subscription-status filter narrows by the payer's current subscription,
// and a filter outside the whitelist answers ErrInvalidFilter.
func TestAdminPayments_ListAndFilters(t *testing.T) {
	t.Parallel()
	h := newRefundIntegrationHarness(t)

	// A refunded payer on the basic tariff (the post-refund shape).
	refundedPayment := h.succeededUpgradePayment(t)
	if err := h.paymentsSvc.RefundPayment(h.ctx(), h.adminID, refundedPayment.ID); err != nil {
		t.Fatalf("RefundPayment: %v", err)
	}
	// A pending payment of another payer still on the paid tariff.
	pendingPayment := h.seedPendingPayment(t)

	h.requireListingCarriesBothPayments(t, refundedPayment, pendingPayment)
	h.requireGraceFilterFindsNone(t)

	// The status filter narrows to the refunded payment only.
	h.requireFilterReturnsExactly(t, "refunded", "refunded", billingapp.AdminPaymentFilters{
		Status: string(domain.PaymentStatusRefunded),
	}, refundedPayment.ID)

	// The user filter narrows to the payer's own payments.
	h.requireFilterReturnsExactly(t, "user", "pending", billingapp.AdminPaymentFilters{
		UserID: &pendingPayment.UserID,
	}, pendingPayment.ID)

	// The single-payment view and the filter whitelist are covered by
	// TestAdminPayments_GetAdminPaymentAndInvalidFilters.
}

// TestAdminPayments_PhoneFilterDecryptsEncryptedPhone proves a payer with an
// encrypted phone is found by the phone filter and the phone is decrypted in
// the views (issue #254).
func TestAdminPayments_PhoneFilterDecryptsEncryptedPhone(t *testing.T) {
	t.Parallel()
	h := newRefundIntegrationHarness(t)

	pendingPayment := h.seedPendingPayment(t)
	encryptedUser := h.seedUser()
	encryptedPhone, err := h.encryptor.DeterministicEncrypt(h.ctx(), "+79990001111")
	if err != nil {
		t.Fatalf("DeterministicEncrypt: %v", err)
	}
	if _, err := h.pool.Exec(
		h.ctx(), "UPDATE users SET phone = $1, phone_encrypted = true WHERE id = $2",
		encryptedPhone, encryptedUser); err != nil {
		t.Fatalf("encrypt user phone: %v", err)
	}
	pendingSub2, err := h.subscriptions.GetByUserID(h.ctx(), pendingPayment.UserID)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	encryptedPayment, err := domain.NewSubscriptionPayment(
		encryptedUser, pendingSub2.ID, pendingSub2.TariffID,
		domain.PeriodMonth, 49000, testProviderFake, time.Now().UTC())
	if err != nil {
		t.Fatalf("NewSubscriptionPayment: %v", err)
	}
	providerPaymentID := "enc_" + encryptedPayment.ID.String()
	encryptedPayment.ProviderPaymentID = &providerPaymentID
	if _, err := h.payments.Create(h.ctx(), encryptedPayment); err != nil {
		t.Fatalf("Create: %v", err)
	}
	phoneViews, _, err := h.paymentsSvc.ListAdminPayments(h.ctx(), billingapp.AdminPaymentFilters{
		UserPhone: "+79990001111",
	})
	if err != nil {
		t.Fatalf("ListAdminPayments(phone): %v", err)
	}
	if len(phoneViews) != 1 || phoneViews[0].Payment.ID != encryptedPayment.ID {
		t.Fatalf("phone filter matched %d payments, want exactly the encrypted payer's", len(phoneViews))
	}
	if phoneViews[0].UserPhone != "+79990001111" {
		t.Fatalf("decrypted phone = %q, want +79990001111", phoneViews[0].UserPhone)
	}
}

// TestAdminPayments_GetAdminPaymentAndInvalidFilters proves the single-payment
// view resolves the same shape as the listing, filters outside the whitelist
// answer ErrInvalidFilter, and an unknown payment answers not found.
func TestAdminPayments_GetAdminPaymentAndInvalidFilters(t *testing.T) {
	t.Parallel()
	h := newRefundIntegrationHarness(t)

	refundedPayment := h.refundedPayment(t)

	got, err := h.paymentsSvc.GetAdminPayment(h.ctx(), refundedPayment.ID)
	if err != nil {
		t.Fatalf("GetAdminPayment: %v", err)
	}
	if got.Payment.ID != refundedPayment.ID || got.Tariff.Name != domain.TariffBusiness {
		t.Fatalf("GetAdminPayment = %v/%v, want the refunded payment with its tariff", got.Payment.ID, got.Tariff.Name)
	}

	// A filter outside the whitelist answers ErrInvalidFilter.
	if _, _, err := h.paymentsSvc.ListAdminPayments(h.ctx(),
		billingapp.AdminPaymentFilters{Sort: "phone"}); !errors.Is(err, billingapp.ErrInvalidFilter) {
		t.Fatalf("bad sort err = %v, want ErrInvalidFilter", err)
	}
	if _, _, err := h.paymentsSvc.ListAdminPayments(h.ctx(),
		billingapp.AdminPaymentFilters{SubscriptionStatus: "blocked"}); !errors.Is(err, billingapp.ErrInvalidFilter) {
		t.Fatalf("bad subscription status err = %v, want ErrInvalidFilter", err)
	}

	// An unknown payment answers not found.
	if _, err := h.paymentsSvc.GetAdminPayment(h.ctx(), uuid.Must(uuid.NewV7())); !errors.Is(err, billingapp.ErrPaymentNotFound) {
		t.Fatalf("unknown payment err = %v, want ErrPaymentNotFound", err)
	}
}

// refundTransitionCount counts the subscription's refunded transitions —
// the idempotency probe of the chargeback scenarios.
func (h *refundIntegrationHarness) refundTransitionCount(t *testing.T, subscriptionID uuid.UUID) int {
	t.Helper()
	count := 0
	for _, tr := range h.transitionsOf(t, subscriptionID) {
		if tr.Reason == domain.TransitionReasonRefunded {
			count++
		}
	}
	return count
}

// TestChargebackReversed_AfterSucceededDowngrades proves the terminal
// processing of a bank-side money reversal (issue #432): a REVERSED delivery
// after a locally successful payment maps to a refund, so the payment becomes
// refunded, the subscription falls to the basic tariff and the transition log
// records the downgrade with the refunded reason attributed to the system. A
// repeated delivery of the same reversal is an idempotent no-op.
func TestChargebackReversed_AfterSucceededDowngrades(t *testing.T) {
	t.Parallel()
	h := newRefundIntegrationHarness(t)
	payment := h.succeededUpgradePayment(t)

	h.deliverWebhook(t, *payment.ProviderPaymentID, domain.PaymentStatusRefunded, payment.ID)

	h.requireRefundedPaymentShape(t, payment)
	subStored := h.requireRefundDowngradedSubscription(t, payment.UserID)

	// The reversal is a provider-reported outcome, not an admin action: the
	// transition entry and the audit record attribute it to the system.
	transitions := h.transitionsOf(t, subStored.ID)
	if len(transitions) == 0 {
		t.Fatal("no transitions recorded")
	}
	refundTransition := transitions[0]
	if refundTransition.Reason != domain.TransitionReasonRefunded {
		t.Fatalf("latest transition reason = %q, want refunded", refundTransition.Reason)
	}
	if refundTransition.Initiator != domain.InitiatorSystem || refundTransition.InitiatorID != nil {
		t.Fatalf("refund transition initiator = %q/%v, want the system", refundTransition.Initiator, refundTransition.InitiatorID)
	}
	if refundTransition.PaymentID == nil || *refundTransition.PaymentID != payment.ID {
		t.Fatalf("refund transition payment = %v, want %v", refundTransition.PaymentID, payment.ID)
	}

	// A repeated delivery of the same reversal changes nothing: the payment
	// stays refunded and no second refund transition appears.
	h.deliverWebhook(t, *payment.ProviderPaymentID, domain.PaymentStatusRefunded, payment.ID)
	h.requireRefundedPaymentShape(t, payment)
	if got := h.refundTransitionCount(t, subStored.ID); got != 1 {
		t.Fatalf("refund transitions = %d after redelivery, want 1", got)
	}
}

// TestChargebackReversed_ConflictWithAdminRefund proves the deterministic
// resolution of a refund that was already administered when the provider-side
// reversal arrives (issue #432): the persisted refunded state wins, the
// redelivery-like reversal is a no-op, and the transition log keeps the
// admin-attributed entry without a duplicate.
func TestChargebackReversed_ConflictWithAdminRefund(t *testing.T) {
	t.Parallel()
	h := newRefundIntegrationHarness(t)
	payment := h.succeededUpgradePayment(t)

	if err := h.paymentsSvc.RefundPayment(h.ctx(), h.adminID, payment.ID); err != nil {
		t.Fatalf("RefundPayment: %v", err)
	}
	h.requireRefundedPaymentShape(t, payment)
	subStored := h.requireRefundDowngradedSubscription(t, payment.UserID)
	if got := h.refundTransitionCount(t, subStored.ID); got != 1 {
		t.Fatalf("refund transitions = %d after the admin refund, want 1", got)
	}

	// The bank reversal of the same money arrives afterwards: the already
	// finalized refund wins — nothing changes, no second transition.
	h.deliverWebhook(t, *payment.ProviderPaymentID, domain.PaymentStatusRefunded, payment.ID)
	h.requireRefundedPaymentShape(t, payment)
	subStored = h.requireRefundDowngradedSubscription(t, payment.UserID)
	if got := h.refundTransitionCount(t, subStored.ID); got != 1 {
		t.Fatalf("refund transitions = %d after the conflicting reversal, want 1", got)
	}

	// The single entry keeps the admin attribution of the refund that
	// finalized first.
	adminTransitions := 0
	for _, tr := range h.transitionsOf(t, subStored.ID) {
		if tr.Reason == domain.TransitionReasonRefunded {
			if tr.Initiator != domain.InitiatorAdmin || tr.InitiatorID == nil || *tr.InitiatorID != h.adminID {
				t.Fatalf("refund transition initiator = %q/%v, want the acting admin", tr.Initiator, tr.InitiatorID)
			}
			adminTransitions++
		}
	}
	if adminTransitions != 1 {
		t.Fatalf("admin refund transitions = %d, want 1", adminTransitions)
	}
}
