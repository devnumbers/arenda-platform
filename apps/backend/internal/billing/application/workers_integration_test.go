//go:build integration

package application_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// The billing worker phases against real PostgreSQL and the fake provider
// adapter (issue #252): every phase runs on the integration harness with real
// migrations, real row locks and real partial indexes, and proves the
// acceptance scenarios end to end — the renewal charge, grace entry and its
// expiry, deferred changes, the shared expiry path and the lost-webhook
// reconciliation.

// capturingArchiver records the archive calls of a worker phase.
type capturingArchiver struct {
	mu    sync.Mutex
	calls []capturedArchive
	err   error
}

type capturedArchive struct {
	ownerID uuid.UUID
	limit   int
}

func (s *capturingArchiver) WithTx(transaction.Tx) (billingapp.ExcessPropertyArchiver, error) {
	return boundArchiver{src: s}, nil
}

type boundArchiver struct{ src *capturingArchiver }

func (a boundArchiver) ArchiveExcess(_ context.Context, ownerID uuid.UUID, limit int) error {
	a.src.mu.Lock()
	defer a.src.mu.Unlock()
	a.src.calls = append(a.src.calls, capturedArchive{ownerID: ownerID, limit: limit})
	return a.src.err
}

func (s *capturingArchiver) recorded() []capturedArchive {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]capturedArchive(nil), s.calls...)
}

// capturingSlots records the enforce calls of a worker phase.
type capturingSlots struct {
	mu    sync.Mutex
	calls []string
	err   error
}

func (s *capturingSlots) WithTx(transaction.Tx) (billingapp.RecipientSlotEnforcer, error) {
	return boundSlots{src: s}, nil
}

type boundSlots struct{ src *capturingSlots }

func (e boundSlots) Enforce(_ context.Context, _ uuid.UUID, trigger string) error {
	e.src.mu.Lock()
	defer e.src.mu.Unlock()
	e.src.calls = append(e.src.calls, trigger)
	return e.src.err
}

func (s *capturingSlots) recorded() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.calls...)
}

// wireBridges connects capturing lifecycle bridges to the harness workers so
// the phases prove their cross-context calls on the real transaction path.
func wireBridges(h *integrationHarness) (*capturingArchiver, *capturingSlots) {
	archiver := &capturingArchiver{}
	slots := &capturingSlots{}
	h.services.Workers.SetLifecycleBridges(archiver, slots)
	return archiver, slots
}

// seedActiveMethod adds and activates a payment method through the public
// payment-method use case: the fake provider accepts raw tokens, the method
// becomes the subscription's charge target exactly as in a local run.
func seedActiveMethod(t *testing.T, h *integrationHarness, userID uuid.UUID, token string) {
	t.Helper()
	if _, err := h.paymentMethodsSvc.AddPaymentMethod(h.ctx(), userID, billingapp.AddPaymentMethodRequest{ProviderToken: token}); err != nil {
		t.Fatalf("AddPaymentMethod() error = %v", err)
	}
}

// TestWorkers_Integration_RenewalChargesActiveMethod proves the auto-renewal
// acceptance scenario on the real schema: an expired auto-renewing
// subscription is charged on its active method through the MIT init+charge
// pair, the payment and the renewal land atomically, and the transition log
// records the applied payment.
func TestWorkers_Integration_RenewalChargesActiveMethod(t *testing.T) {
	h := newIntegrationHarness(t)
	userID, sub := seedPaidProSubscription(t, h)
	// The paid period ended an hour ago.
	expired := h.clock.Now().Add(-time.Hour)
	sub.ValidUntil = &expired
	if err := h.subscriptions.Update(h.ctx(), sub); err != nil {
		t.Fatalf("expire subscription: %v", err)
	}
	seedActiveMethod(t, h, userID, "tok_renew_ok")

	count, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now())
	if err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("ProcessRenewals() = %d, want 1", count)
	}

	payments, err := h.payments.ListByUserID(h.ctx(), userID)
	if err != nil || len(payments) != 1 {
		t.Fatalf("payments = %d (err %v), want the single renewal", len(payments), err)
	}
	payment := payments[0]
	if payment.Status != domain.PaymentStatusSucceeded || payment.Provider != "fake" {
		t.Errorf("payment = %s/%s, want succeeded/fake", payment.Status, payment.Provider)
	}
	if payment.TariffID != sub.TariffID || payment.AmountKopecks != 49000 {
		t.Errorf("payment = tariff %s amount %d, want pro/49000", payment.TariffID, payment.AmountKopecks)
	}
	if !payment.HasProviderReference() {
		t.Error("renewal payment has no provider reference")
	}

	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	wantUntil := h.clock.Now().AddDate(0, 1, 0)
	if stored.Status != domain.SubscriptionStatusActive || stored.ValidUntil == nil || !stored.ValidUntil.Equal(wantUntil) {
		t.Errorf("subscription = %s until %v, want active until %v", stored.Status, stored.ValidUntil, wantUntil)
	}
	if stored.LastAppliedPaymentID == nil || *stored.LastAppliedPaymentID != payment.ID {
		t.Errorf("LastAppliedPaymentID = %v, want the renewal payment", stored.LastAppliedPaymentID)
	}

	transitions, err := h.transitions.ListBySubscriptionID(h.ctx(), sub.ID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID() error = %v", err)
	}
	if len(transitions) != 2 || transitions[0].Reason != domain.TransitionReasonPaymentApplied {
		t.Fatalf("transitions = %+v, want registered + payment_applied", transitions)
	}
	if got := h.countRows(`SELECT count(*) FROM audit_log WHERE action = 'subscription_payment.succeeded' AND entity_id = $1`, payment.ID); got != 1 {
		t.Errorf("succeeded-payment audit rows = %d, want 1", got)
	}
}

// TestWorkers_Integration_FailedChargeGraceThenBasic proves the grace
// acceptance scenario end to end: a declined charge enters grace for the
// configured window, the window's expiry downgrades to basic, and the
// lifecycle bridges run inside the same transaction as the downgrade.
func TestWorkers_Integration_FailedChargeGraceThenBasic(t *testing.T) {
	h := newIntegrationHarness(t)
	archiver, slots := wireBridges(h)
	userID, sub := seedPaidProSubscription(t, h)
	expired := h.clock.Now().Add(-time.Hour)
	sub.ValidUntil = &expired
	if err := h.subscriptions.Update(h.ctx(), sub); err != nil {
		t.Fatalf("expire subscription: %v", err)
	}
	seedActiveMethod(t, h, userID, "fake_fail_card")

	if _, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}

	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.Status != domain.SubscriptionStatusGrace {
		t.Fatalf("status = %q, want grace after the failed charge", stored.Status)
	}
	graceEnd := h.clock.Now().Add(7 * 24 * time.Hour)
	if stored.ValidUntil == nil || !stored.ValidUntil.Equal(graceEnd) {
		t.Errorf("ValidUntil = %v, want the grace window end %v", stored.ValidUntil, graceEnd)
	}
	if got := h.countRows(`SELECT count(*) FROM subscription_transitions WHERE subscription_id = $1 AND reason = 'grace_entered'`, sub.ID); got != 1 {
		t.Fatalf("grace_entered transitions = %d, want 1", got)
	}

	// The grace window passes; the phase downgrades to basic with archiving.
	h.clock.now = graceEnd.Add(time.Hour)
	count, err := h.services.Workers.ProcessExpiredGrace(h.ctx(), h.clock.Now())
	if err != nil {
		t.Fatalf("ProcessExpiredGrace() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("ProcessExpiredGrace() = %d, want 1", count)
	}

	basic, err := h.tariffs.GetByName(h.ctx(), domain.TariffBasic)
	if err != nil {
		t.Fatalf("GetByName(basic) error = %v", err)
	}
	stored, err = h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() after grace expiry error = %v", err)
	}
	if stored.TariffID != basic.ID || stored.Status != domain.SubscriptionStatusActive || stored.ValidUntil != nil || stored.AutoRenewEnabled {
		t.Errorf("subscription = %+v, want basic/no validity/no auto-renew/active", stored)
	}
	if got := archiver.recorded(); len(got) != 1 || got[0].limit != basic.ActivePropertyLimit || got[0].ownerID != userID {
		t.Errorf("archive calls = %+v, want one at the basic limit for the user", got)
	}
	if got := slots.recorded(); len(got) != 1 || got[0] != "grace_expired" {
		t.Errorf("slot calls = %v, want one grace_expired", got)
	}
	if got := h.countRows(`SELECT count(*) FROM subscription_transitions WHERE subscription_id = $1 AND reason = 'expired'`, sub.ID); got != 1 {
		t.Errorf("expired transitions = %d, want 1", got)
	}
}

// TestWorkers_Integration_NoChargeableMethodEntersGrace proves the
// provider-switch semantics on the real schema: an active method of a foreign
// provider is as good as absent — the renewal cannot charge it, the
// subscription enters grace instead of being blocked forever.
func TestWorkers_Integration_NoChargeableMethodEntersGrace(t *testing.T) {
	h := newIntegrationHarness(t)
	userID, sub := seedPaidProSubscription(t, h)
	expired := h.clock.Now().Add(-time.Hour)
	sub.ValidUntil = &expired
	if err := h.subscriptions.Update(h.ctx(), sub); err != nil {
		t.Fatalf("expire subscription: %v", err)
	}
	// A method saved by another provider (the pre-switch card).
	method, err := domain.NewPaymentMethod(userID, "tkassa", "rebill_foreign", h.clock.Now())
	if err != nil {
		t.Fatalf("NewPaymentMethod() error = %v", err)
	}
	saved, err := h.methods.UpsertByTokenHash(h.ctx(), method)
	if err != nil {
		t.Fatalf("UpsertByTokenHash() error = %v", err)
	}
	if err := h.methods.SetActive(h.ctx(), userID, saved.ID); err != nil {
		t.Fatalf("SetActive() error = %v", err)
	}
	sub.ActivePaymentMethodID = &saved.ID
	if err := h.subscriptions.Update(h.ctx(), sub); err != nil {
		t.Fatalf("link method: %v", err)
	}

	if _, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}
	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.Status != domain.SubscriptionStatusGrace {
		t.Fatalf("status = %q, want grace over a foreign-provider method", stored.Status)
	}
	if got := h.countRows(`SELECT count(*) FROM subscription_payments WHERE user_id = $1`, userID); got != 0 {
		t.Errorf("payments = %d, want 0 (nothing was initiated)", got)
	}
}

// TestWorkers_Integration_ScheduledChangesFreeAndPaid proves the deferred
// change acceptance scenario on the real schema: a free target applies
// directly in the scheduled phase, a paid target is charged and applied by
// the renewal phase of the same tick.
func TestWorkers_Integration_ScheduledChangesFreeAndPaid(t *testing.T) {
	h := newIntegrationHarness(t)
	archiver, _ := wireBridges(h)

	// Free target: pro -> basic, due now.
	freeUser, freeSub := seedPaidProSubscription(t, h)
	expired := h.clock.Now().Add(-2 * time.Hour)
	freeSub.ValidUntil = &expired
	if err := h.subscriptions.Update(h.ctx(), freeSub); err != nil {
		t.Fatalf("expire subscription: %v", err)
	}
	if _, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), freeUser, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffBasic,
		Period:     domain.PeriodMonth,
	}); err != nil {
		t.Fatalf("ChangeTariff(basic) error = %v", err)
	}

	count, err := h.services.Workers.ProcessScheduledChanges(h.ctx(), h.clock.Now())
	if err != nil {
		t.Fatalf("ProcessScheduledChanges() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("ProcessScheduledChanges() = %d, want 1 (free target)", count)
	}
	basic, err := h.tariffs.GetByName(h.ctx(), domain.TariffBasic)
	if err != nil {
		t.Fatalf("GetByName(basic) error = %v", err)
	}
	stored, err := h.subscriptions.GetByUserID(h.ctx(), freeUser)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.TariffID != basic.ID || stored.HasPendingChange() {
		t.Errorf("subscription = tariff %v pending %v, want basic applied", stored.TariffID, stored.PendingTariffID)
	}
	if got := archiver.recorded(); len(got) != 1 || got[0].limit != basic.ActivePropertyLimit {
		t.Errorf("archive calls = %+v, want one at the basic limit", got)
	}

	// Paid target: business -> pro, due now; charged at apply time.
	paidUser, paidSub := seedPaidProSubscription(t, h)
	business, err := h.tariffs.GetByName(h.ctx(), domain.TariffBusiness)
	if err != nil {
		t.Fatalf("GetByName(business) error = %v", err)
	}
	paidSub.TariffID = business.ID
	paidSub.ValidUntil = &expired
	if err := h.subscriptions.Update(h.ctx(), paidSub); err != nil {
		t.Fatalf("set business: %v", err)
	}
	if _, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), paidUser, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	}); err != nil {
		t.Fatalf("ChangeTariff(pro) error = %v", err)
	}
	seedActiveMethod(t, h, paidUser, "tok_sched_paid")

	skipped, err := h.services.Workers.ProcessScheduledChanges(h.ctx(), h.clock.Now())
	if err != nil {
		t.Fatalf("ProcessScheduledChanges() paid error = %v", err)
	}
	if skipped != 0 {
		t.Fatalf("ProcessScheduledChanges() = %d, want 0 (paid target skipped)", skipped)
	}
	if _, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}
	pro, err := h.tariffs.GetByName(h.ctx(), domain.TariffPro)
	if err != nil {
		t.Fatalf("GetByName(pro) error = %v", err)
	}
	storedPaid, err := h.subscriptions.GetByUserID(h.ctx(), paidUser)
	if err != nil {
		t.Fatalf("GetByUserID() paid error = %v", err)
	}
	if storedPaid.TariffID != pro.ID || storedPaid.HasPendingChange() {
		t.Errorf("subscription = tariff %v pending %v, want pro applied by the charge", storedPaid.TariffID, storedPaid.PendingTariffID)
	}
	payments, err := h.payments.ListByUserID(h.ctx(), paidUser)
	if err != nil || len(payments) != 1 || payments[0].Status != domain.PaymentStatusSucceeded || payments[0].TariffID != pro.ID {
		t.Fatalf("payments = %+v (err %v), want one succeeded pro charge", payments, err)
	}
}

// TestWorkers_Integration_NonRenewingAndCancelledExpireToBasic proves the
// shared expiry acceptance scenario: both a non-renewing subscription and a
// cancelled one whose retained period ended downgrade to basic with the
// expiry transition and the archiving bridges.
func TestWorkers_Integration_NonRenewingAndCancelledExpireToBasic(t *testing.T) {
	h := newIntegrationHarness(t)
	archiver, slots := wireBridges(h)
	basic, err := h.tariffs.GetByName(h.ctx(), domain.TariffBasic)
	if err != nil {
		t.Fatalf("GetByName(basic) error = %v", err)
	}
	expired := h.clock.Now().Add(-time.Hour)

	nonRenewingUser, nonRenewingSub := seedPaidProSubscription(t, h)
	nonRenewingSub.ValidUntil = &expired
	nonRenewingSub.AutoRenewEnabled = false
	if err := h.subscriptions.Update(h.ctx(), nonRenewingSub); err != nil {
		t.Fatalf("expire non-renewing: %v", err)
	}

	cancelledUser, cancelledSub := seedPaidProSubscription(t, h)
	cancelledSub.ValidUntil = &expired
	if err := h.subscriptions.Update(h.ctx(), cancelledSub); err != nil {
		t.Fatalf("expire cancelled: %v", err)
	}
	if err := h.subscriptionsSvc.CancelSubscription(h.ctx(), cancelledUser); err != nil {
		t.Fatalf("CancelSubscription() error = %v", err)
	}

	count, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now())
	if err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("ProcessRenewals() = %d, want 2", count)
	}
	for _, tc := range []struct {
		user uuid.UUID
		sub  domain.Subscription
	}{
		{user: nonRenewingUser, sub: nonRenewingSub},
		{user: cancelledUser, sub: cancelledSub},
	} {
		stored, err := h.subscriptions.GetByUserID(h.ctx(), tc.user)
		if err != nil {
			t.Fatalf("GetByUserID() error = %v", err)
		}
		if stored.TariffID != basic.ID || stored.ValidUntil != nil || stored.Status != domain.SubscriptionStatusActive {
			t.Errorf("subscription %s = %+v, want basic/no validity/active", tc.user, stored)
		}
		if got := h.countRows(`SELECT count(*) FROM subscription_transitions WHERE subscription_id = $1 AND reason = 'expired'`, tc.sub.ID); got != 1 {
			t.Errorf("expired transitions = %d, want 1", got)
		}
	}
	if got := archiver.recorded(); len(got) != 2 {
		t.Errorf("archive calls = %d, want 2", len(got))
	}
	if got := slots.recorded(); len(got) != 2 {
		t.Errorf("slot calls = %v, want 2", got)
	}
}

// TestWorkers_Integration_BridgeFailureRollsBack proves the atomicity of the
// rewritten expiry path on a real database: an archiving failure rolls the
// subscription change back with its transition, and the next tick — with the
// failure cleared — applies it.
func TestWorkers_Integration_BridgeFailureRollsBack(t *testing.T) {
	h := newIntegrationHarness(t)
	archiver, slots := wireBridges(h)
	archiver.err = errArchiveBoom
	userID, sub := seedPaidProSubscription(t, h)
	graceEnd := h.clock.Now().Add(-time.Hour)
	sub.Status = domain.SubscriptionStatusGrace
	sub.ValidUntil = &graceEnd
	if err := h.subscriptions.Update(h.ctx(), sub); err != nil {
		t.Fatalf("seed expired grace: %v", err)
	}

	count, err := h.services.Workers.ProcessExpiredGrace(h.ctx(), h.clock.Now())
	if err != nil {
		t.Fatalf("ProcessExpiredGrace() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("ProcessExpiredGrace() = %d, want 0 while archiving fails", count)
	}
	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.Status != domain.SubscriptionStatusGrace || stored.TariffID != sub.TariffID {
		t.Fatalf("subscription = %s/%s, want the grace state rolled back", stored.Status, stored.TariffID)
	}
	if got := h.countRows(`SELECT count(*) FROM subscription_transitions WHERE subscription_id = $1`, sub.ID); got != 1 {
		t.Errorf("transitions = %d, want 1 (only the onboarding one; the expiry rolled back)", got)
	}

	// The failure clears; the next tick applies the downgrade.
	archiver.err = nil
	if _, err := h.services.Workers.ProcessExpiredGrace(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessExpiredGrace() retry error = %v", err)
	}
	if got := len(slots.recorded()); got != 1 {
		t.Errorf("slot calls after retry = %d, want 1", got)
	}
	stored, err = h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() retry error = %v", err)
	}
	if stored.ValidUntil != nil {
		t.Errorf("ValidUntil = %v after retry, want nil (basic)", stored.ValidUntil)
	}
}

// errArchiveBoom simulates an archiving outage inside the expiry transaction.
var errArchiveBoom = &testError{"archive bridge failed"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

// TestWorkers_Integration_ReconcileLostWebhook proves the reconciliation
// acceptance scenario: a tariff-change payment the provider settled but whose
// webhook was never delivered is finalized from the provider's status once it
// goes stale, and the upgrade applies through the synchronous path.
func TestWorkers_Integration_ReconcileLostWebhook(t *testing.T) {
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

	// Still fresh: the reconciliation must not touch it.
	if count, err := h.services.Workers.ReconcilePendingPayments(h.ctx(), h.clock.Now()); err != nil || count != 0 {
		t.Fatalf("ReconcilePendingPayments() = %d (err %v), want 0 while fresh", count, err)
	}

	// The row's created_at comes from the database clock; pin it to the fake
	// clock's past so the staleness threshold of the reconciliation selects it.
	staleCreated := h.clock.Now().Add(-10 * time.Minute)
	if _, err := h.pool.Exec(h.ctx(), `UPDATE subscription_payments SET created_at = $1 WHERE id = $2`, staleCreated, result.PaymentID); err != nil {
		t.Fatalf("age payment row: %v", err)
	}
	if count, err := h.services.Workers.ReconcilePendingPayments(h.ctx(), h.clock.Now()); err != nil || count != 1 {
		t.Fatalf("ReconcilePendingPayments() = %d (err %v), want 1 stale payment reconciled", count, err)
	}

	payment, err := h.payments.GetByID(h.ctx(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if payment.Status != domain.PaymentStatusSucceeded {
		t.Fatalf("payment status = %q, want succeeded from the provider status", payment.Status)
	}
	business, err := h.tariffs.GetByName(h.ctx(), domain.TariffBusiness)
	if err != nil {
		t.Fatalf("GetByName(business) error = %v", err)
	}
	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.TariffID != business.ID || stored.Status != domain.SubscriptionStatusActive {
		t.Errorf("subscription = %s/%s, want business applied and active", stored.TariffID, stored.Status)
	}
	wantUntil := h.clock.Now().AddDate(0, 1, 0)
	if stored.ValidUntil == nil || !stored.ValidUntil.Equal(wantUntil) {
		t.Errorf("ValidUntil = %v, want %v from the reconciliation moment", stored.ValidUntil, wantUntil)
	}
}

// TestWorkers_Integration_RenewalDoubleChargeGuardOnCrash proves the
// double-charge acceptance scenario on the real schema: a renewal payment the
// provider captured while the local application crashed is applied from the
// provider status on the next tick — without a second charge (the fake
// provider records confirmed amounts, so a second charge would show).
func TestWorkers_Integration_RenewalDoubleChargeGuardOnCrash(t *testing.T) {
	h := newIntegrationHarness(t)
	userID, sub := seedPaidProSubscription(t, h)
	expired := h.clock.Now().Add(-time.Hour)
	sub.ValidUntil = &expired
	if err := h.subscriptions.Update(h.ctx(), sub); err != nil {
		t.Fatalf("expire subscription: %v", err)
	}
	seedActiveMethod(t, h, userID, "tok_guard")

	// First tick: the charge captures at the provider, but the local
	// application "crashes" — simulate by finalizing only the provider side
	// and reverting the local rows to the pre-charge state.
	if _, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}
	payments, err := h.payments.ListByUserID(h.ctx(), userID)
	if err != nil || len(payments) != 1 {
		t.Fatalf("payments = %d (err %v), want the renewal", len(payments), err)
	}
	payment := payments[0]
	if payment.Status != domain.PaymentStatusSucceeded {
		t.Fatalf("payment status = %q, want succeeded after the first tick", payment.Status)
	}

	// Revert the local state to the crash moment: payment pending again,
	// subscription still expired. The provider keeps the captured charge.
	reverted := payment
	reverted.Status = domain.PaymentStatusPending
	reverted.SucceededAt = nil
	if err := h.payments.Update(h.ctx(), reverted); err != nil {
		t.Fatalf("revert payment: %v", err)
	}
	subAgain, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	subAgain.ValidUntil = &expired
	subAgain.LastAppliedPaymentID = nil
	if err := h.subscriptions.Update(h.ctx(), subAgain); err != nil {
		t.Fatalf("revert subscription: %v", err)
	}

	// The recovery tick must resolve from the provider status without
	// charging again. The fake provider's confirmed amount for the internal
	// payment id stays exactly one charge's worth.
	if _, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessRenewals() recovery error = %v", err)
	}
	recovered, err := h.payments.GetByID(h.ctx(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if recovered.Status != domain.PaymentStatusSucceeded {
		t.Errorf("payment status = %q, want succeeded recovered from the provider", recovered.Status)
	}
	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.Status != domain.SubscriptionStatusActive {
		t.Errorf("subscription status = %q, want active after the recovery", stored.Status)
	}
	if got := h.countRows(`SELECT count(*) FROM subscription_payments WHERE user_id = $1`, userID); got != 1 {
		t.Errorf("payments = %d, want 1 (no duplicate renewal)", got)
	}
	if charges := h.provider.ChargeCount(payment.ID); charges != 1 {
		t.Errorf("provider charge count = %d, want exactly 1 (no double charge)", charges)
	}
}
