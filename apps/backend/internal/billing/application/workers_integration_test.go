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

// capturingArchiver records the archive and restore calls of a worker phase
// or the payment application seam. ArchiveIDs, when set, is what ArchiveExcess
// reports as the archived ids; RemainingAs is the debt remainder the restore
// reports.
type capturingArchiver struct {
	mu          sync.Mutex
	calls       []capturedArchive
	restores    []capturedRestore
	err         error
	archiveIDs  []uuid.UUID
	remainingAs []uuid.UUID
	// The ActivePropertyExists answers — the keep-choice validation of the
	// cancel flow (issue #617); ids absent from the map do not exist.
	activeExists map[uuid.UUID]bool
}

type capturedArchive struct {
	ownerID uuid.UUID
	limit   int
	keep    *uuid.UUID
}

type capturedRestore struct {
	ownerID uuid.UUID
	ids     []uuid.UUID
	limit   int
}

func (s *capturingArchiver) WithTx(transaction.Tx) (billingapp.ExcessPropertyArchiver, error) {
	return boundArchiver{src: s}, nil
}

type boundArchiver struct{ src *capturingArchiver }

func (a boundArchiver) ArchiveExcess(_ context.Context, ownerID uuid.UUID, limit int, keepPropertyID *uuid.UUID) ([]uuid.UUID, error) {
	a.src.mu.Lock()
	defer a.src.mu.Unlock()
	a.src.calls = append(a.src.calls, capturedArchive{ownerID: ownerID, limit: limit, keep: keepPropertyID})
	return a.src.archiveIDs, a.src.err
}

func (a boundArchiver) ActivePropertyExists(_ context.Context, _, propertyID uuid.UUID) (bool, error) {
	a.src.mu.Lock()
	defer a.src.mu.Unlock()
	return a.src.activeExists[propertyID], a.src.err
}

func (a boundArchiver) RestoreGraceArchive(_ context.Context, ownerID uuid.UUID, ids []uuid.UUID, limit int) ([]uuid.UUID, error) {
	a.src.mu.Lock()
	defer a.src.mu.Unlock()
	a.src.restores = append(a.src.restores, capturedRestore{ownerID: ownerID, ids: ids, limit: limit})
	return a.src.remainingAs, a.src.err
}

func (s *capturingArchiver) recorded() []capturedArchive {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]capturedArchive(nil), s.calls...)
}

func (s *capturingArchiver) restoreCalls() []capturedRestore {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]capturedRestore(nil), s.restores...)
}

// capturingSlots records the enforce and recover calls of a worker phase.
type capturingSlots struct {
	mu       sync.Mutex
	calls    []string
	recovers []uuid.UUID
	err      error
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

func (e boundSlots) Recover(_ context.Context, userID uuid.UUID) error {
	e.src.mu.Lock()
	defer e.src.mu.Unlock()
	e.src.recovers = append(e.src.recovers, userID)
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

// seedExpiredProSubscription seeds a paid pro subscription whose period ended
// the given duration ago, so the renewal phases select it on the next tick.
func seedExpiredProSubscription(t *testing.T, h *integrationHarness, ago time.Duration) (uuid.UUID, domain.Subscription) {
	t.Helper()
	userID, sub := seedPaidProSubscription(t, h)
	expired := h.clock.Now().Add(-ago)
	sub.ValidUntil = &expired
	if err := h.subscriptions.Update(h.ctx(), sub); err != nil {
		t.Fatalf("expire subscription: %v", err)
	}
	return userID, sub
}

// requireSucceededRenewalPayment asserts the user's only payment is the
// succeeded fake renewal for the given tariff at the canonical pro price and
// returns it.
func requireSucceededRenewalPayment(t *testing.T, h *integrationHarness, userID, tariffID uuid.UUID) domain.SubscriptionPayment {
	t.Helper()
	payments, err := h.payments.ListByUserID(h.ctx(), userID)
	if err != nil || len(payments) != 1 {
		t.Fatalf("payments = %d (err %v), want the single renewal", len(payments), err)
	}
	payment := payments[0]
	if payment.Status != domain.PaymentStatusSucceeded || payment.Provider != testProviderFake {
		t.Errorf("payment = %s/%s, want succeeded/fake", payment.Status, payment.Provider)
	}
	if payment.TariffID != tariffID || payment.AmountKopecks != 49000 {
		t.Errorf("payment = tariff %s amount %d, want pro/49000", payment.TariffID, payment.AmountKopecks)
	}
	if !payment.HasProviderReference() {
		t.Error("renewal payment has no provider reference")
	}
	return payment
}

// requireRenewedActiveSubscription asserts the subscription is active, renewed
// for a month from now, and pointing at the applied renewal payment.
func requireRenewedActiveSubscription(t *testing.T, h *integrationHarness, userID, paymentID uuid.UUID) {
	t.Helper()
	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	wantUntil := h.clock.Now().AddDate(0, 1, 0)
	if stored.Status != domain.SubscriptionStatusActive || stored.ValidUntil == nil || !stored.ValidUntil.Equal(wantUntil) {
		t.Errorf("subscription = %s until %v, want active until %v", stored.Status, stored.ValidUntil, wantUntil)
	}
	if stored.LastAppliedPaymentID == nil || *stored.LastAppliedPaymentID != paymentID {
		t.Errorf("LastAppliedPaymentID = %v, want the renewal payment", stored.LastAppliedPaymentID)
	}
}

// requireRegisteredAndAppliedTransitions asserts the transition log holds the
// onboarding registration plus the renewal's payment_applied entry.
func requireRegisteredAndAppliedTransitions(t *testing.T, h *integrationHarness, subscriptionID uuid.UUID) {
	t.Helper()
	transitions, err := h.transitions.ListBySubscriptionID(h.ctx(), subscriptionID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID() error = %v", err)
	}
	if len(transitions) != 2 || transitions[0].Reason != domain.TransitionReasonPaymentApplied {
		t.Fatalf("transitions = %+v, want registered + payment_applied", transitions)
	}
}

// TestWorkers_Integration_RenewalChargesActiveMethod proves the auto-renewal
// acceptance scenario on the real schema: an expired auto-renewing
// subscription is charged on its active method through the MIT init+charge
// pair, the payment and the renewal land atomically, and the transition log
// records the applied payment.
func TestWorkers_Integration_RenewalChargesActiveMethod(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	userID, sub := seedExpiredProSubscription(t, h, time.Hour)
	seedActiveMethod(t, h, userID, "tok_renew_ok")

	count, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now())
	if err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("ProcessRenewals() = %d, want 1", count)
	}

	payment := requireSucceededRenewalPayment(t, h, userID, sub.TariffID)
	requireRenewedActiveSubscription(t, h, userID, payment.ID)
	requireRegisteredAndAppliedTransitions(t, h, sub.ID)
	if got := h.countRows(
		`SELECT count(*) FROM audit_log WHERE action = 'subscription_payment.succeeded' AND entity_id = $1`,
		payment.ID); got != 1 {
		t.Errorf("succeeded-payment audit rows = %d, want 1", got)
	}
}

// requireGraceEnteredAfterFailedCharge asserts the subscription sits in a
// fresh grace window after the declined charge, with the grace_entered
// transition logged, and returns the grace window end.
func requireGraceEnteredAfterFailedCharge(t *testing.T, h *integrationHarness, userID, subID uuid.UUID) time.Time {
	t.Helper()
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
	if got := h.countRows(
		`SELECT count(*) FROM subscription_transitions WHERE subscription_id = $1 AND reason = 'grace_entered'`,
		subID); got != 1 {
		t.Fatalf("grace_entered transitions = %d, want 1", got)
	}
	return graceEnd
}

// requireBasicDowngrade asserts the subscription fell back to the permanent
// basic state — basic tariff, active, no validity, no auto-renew — and returns
// the basic tariff for follow-up bridge checks.
func requireBasicDowngrade(t *testing.T, h *integrationHarness, userID uuid.UUID) domain.Tariff {
	t.Helper()
	basic, err := h.tariffs.GetByName(h.ctx(), domain.TariffBasic)
	if err != nil {
		t.Fatalf("GetByName(basic) error = %v", err)
	}
	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() after grace expiry error = %v", err)
	}
	if stored.TariffID != basic.ID || stored.Status != domain.SubscriptionStatusActive || stored.ValidUntil != nil || stored.AutoRenewEnabled {
		t.Errorf("subscription = %+v, want basic/no validity/no auto-renew/active", stored)
	}
	return basic
}

// TestWorkers_Integration_FailedChargeGraceThenBasic proves the grace
// acceptance scenario end to end: a declined charge enters grace for the
// configured window, the window's expiry downgrades to basic, and the
// lifecycle bridges run inside the same transaction as the downgrade.
func TestWorkers_Integration_FailedChargeGraceThenBasic(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	archiver, slots := wireBridges(h)
	userID, sub := seedExpiredProSubscription(t, h, time.Hour)
	seedActiveMethod(t, h, userID, "fake_fail_card")

	if _, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}
	graceEnd := requireGraceEnteredAfterFailedCharge(t, h, userID, sub.ID)

	// The grace window passes; the phase downgrades to basic with archiving.
	h.clock.now = graceEnd.Add(time.Hour)
	count, err := h.services.Workers.ProcessExpiredGrace(h.ctx(), h.clock.Now())
	if err != nil {
		t.Fatalf("ProcessExpiredGrace() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("ProcessExpiredGrace() = %d, want 1", count)
	}

	basic := requireBasicDowngrade(t, h, userID)
	// Grace v2 (ADR 0055): the entry archived at the grace limit, and the
	// expiry downgrade enforces the basic limit — two archive calls, both at
	// one property.
	entry := capturedArchive{ownerID: userID, limit: 1}
	expiry := capturedArchive{ownerID: userID, limit: basic.ActivePropertyLimit}
	if got := archiver.recorded(); len(got) != 2 || got[0] != entry || got[1] != expiry {
		t.Errorf("archive calls = %+v, want %+v then %+v", got, entry, expiry)
	}
	if got := slots.recorded(); len(got) != 2 || got[0] != "grace_entry" || got[1] != "grace_expired" {
		t.Errorf("slot calls = %v, want grace_entry then grace_expired", got)
	}
	if got := h.countRows(
		`SELECT count(*) FROM subscription_transitions WHERE subscription_id = $1 AND reason = 'expired'`,
		sub.ID); got != 1 {
		t.Errorf("expired transitions = %d, want 1", got)
	}
}

// TestWorkers_Integration_NoChargeableMethodEntersGrace proves the
// provider-switch semantics on the real schema: an active method of a foreign
// provider is as good as absent — the renewal cannot charge it, the
// subscription enters grace instead of being blocked forever.
func TestWorkers_Integration_NoChargeableMethodEntersGrace(t *testing.T) {
	t.Parallel()
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

// applyFreeScheduledChange covers the free half of the deferred-change
// scenario: a pro subscription expired two hours ago schedules a downgrade to
// basic, and the scheduled phase applies it directly, with the archive bridge.
func applyFreeScheduledChange(t *testing.T, h *integrationHarness, archiver *capturingArchiver) {
	t.Helper()
	// Free target: pro -> basic, due now.
	freeUser, _ := seedExpiredProSubscription(t, h, 2*time.Hour)
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
}

// requirePaidChangeCharged asserts the paid deferred change landed through the
// renewal charge: the target tariff applied, the pending change cleared, and
// exactly one succeeded payment for the target.
func requirePaidChangeCharged(t *testing.T, h *integrationHarness, paidUser uuid.UUID) {
	t.Helper()
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

// chargePaidScheduledChange covers the paid half of the deferred-change
// scenario: a business subscription expired two hours ago schedules an upgrade
// to pro, the scheduled phase skips it, and the renewal phase of the same tick
// charges and applies it.
func chargePaidScheduledChange(t *testing.T, h *integrationHarness) {
	t.Helper()
	// Paid target: business -> pro, due now; charged at apply time.
	paidUser, paidSub := seedPaidProSubscription(t, h)
	business, err := h.tariffs.GetByName(h.ctx(), domain.TariffBusiness)
	if err != nil {
		t.Fatalf("GetByName(business) error = %v", err)
	}
	expired := h.clock.Now().Add(-2 * time.Hour)
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
	requirePaidChangeCharged(t, h, paidUser)
}

// TestWorkers_Integration_ScheduledChangesFreeAndPaid proves the deferred
// change acceptance scenario on the real schema: a free target applies
// directly in the scheduled phase, a paid target is charged and applied by
// the renewal phase of the same tick.
func TestWorkers_Integration_ScheduledChangesFreeAndPaid(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	archiver, _ := wireBridges(h)
	applyFreeScheduledChange(t, h, archiver)
	chargePaidScheduledChange(t, h)
}

// TestWorkers_Integration_NonRenewingAndCancelledExpireToBasic proves the
// shared expiry acceptance scenario: both a non-renewing subscription and a
// cancelled one whose retained period ended downgrade to basic with the
// expiry transition and the archiving bridges.
func TestWorkers_Integration_NonRenewingAndCancelledExpireToBasic(t *testing.T) {
	t.Parallel()
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
	if err := h.subscriptionsSvc.CancelSubscription(h.ctx(), cancelledUser, nil); err != nil {
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
		if got := h.countRows(
			`SELECT count(*) FROM subscription_transitions WHERE subscription_id = $1 AND reason = 'expired'`,
			tc.sub.ID); got != 1 {
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
	t.Parallel()
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

// requireReconciledPaymentSucceeded asserts the stale payment was finalized as
// succeeded from the provider's status.
func requireReconciledPaymentSucceeded(t *testing.T, h *integrationHarness, paymentID uuid.UUID) {
	t.Helper()
	payment, err := h.payments.GetByID(h.ctx(), paymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if payment.Status != domain.PaymentStatusSucceeded {
		t.Fatalf("payment status = %q, want succeeded from the provider status", payment.Status)
	}
}

// requireReconciledUpgradeApplied asserts the reconciled upgrade landed: the
// business tariff active with a month of validity from the reconciliation.
func requireReconciledUpgradeApplied(t *testing.T, h *integrationHarness, userID uuid.UUID) {
	t.Helper()
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

// agePaymentRow pins a payment row's created_at to the fake clock's past so
// the staleness threshold of a reconciliation phase selects it (created_at
// otherwise comes from the database clock).
func agePaymentRow(t *testing.T, h *integrationHarness, paymentID uuid.UUID) {
	t.Helper()
	staleCreated := h.clock.Now().Add(-10 * time.Minute)
	if _, err := h.pool.Exec(
		h.ctx(), `UPDATE subscription_payments SET created_at = $1 WHERE id = $2`, staleCreated,
		paymentID); err != nil {
		t.Fatalf("age payment row: %v", err)
	}
}

// TestWorkers_Integration_ReconcileLostWebhook proves the reconciliation
// acceptance scenario: a tariff-change payment the provider settled but whose
// webhook was never delivered is finalized from the provider's status once it
// goes stale, and the upgrade applies through the synchronous path.
func TestWorkers_Integration_ReconcileLostWebhook(t *testing.T) {
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

	// Still fresh: the reconciliation must not touch it.
	if count, err := h.services.Workers.ReconcilePendingPayments(h.ctx(), h.clock.Now()); err != nil || count != 0 {
		t.Fatalf("ReconcilePendingPayments() = %d (err %v), want 0 while fresh", count, err)
	}

	// The row's created_at comes from the database clock; pin it to the fake
	// clock's past so the staleness threshold of the reconciliation selects it.
	agePaymentRow(t, h, result.PaymentID)
	if count, err := h.services.Workers.ReconcilePendingPayments(h.ctx(), h.clock.Now()); err != nil || count != 1 {
		t.Fatalf("ReconcilePendingPayments() = %d (err %v), want 1 stale payment reconciled", count, err)
	}

	requireReconciledPaymentSucceeded(t, h, result.PaymentID)
	requireReconciledUpgradeApplied(t, h, userID)
}

// TestWorkers_Integration_ReconcileUnknownPaymentStaysPending is the honest
// counterpart of the lost-webhook scenario (issue #420): a stale pending
// payment the provider never knew is reported not-found by the contract-
// faithful fake, so the reconciliation cannot fabricate a success — the
// payment stays pending and the subscription keeps its seeded state.
func TestWorkers_Integration_ReconcileUnknownPaymentStaysPending(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	userID, sub := seedPaidProSubscription(t, h)
	business, err := h.tariffs.GetByName(h.ctx(), domain.TariffBusiness)
	if err != nil {
		t.Fatalf("GetByName(business) error = %v", err)
	}

	// A pending business payment the fake provider never saw: seeded straight
	// into the database, no Init call, like a row left by a crashed external
	// import or a provider switch.
	payment, err := domain.NewSubscriptionPayment(
		userID, sub.ID, business.ID, domain.PeriodMonth, 99000, testProviderFake, h.clock.Now())
	if err != nil {
		t.Fatalf("NewSubscriptionPayment() error = %v", err)
	}
	provRef := "prov_never_seen"
	payment.ProviderPaymentID = &provRef
	if _, err := h.payments.Create(h.ctx(), payment); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	agePaymentRow(t, h, payment.ID)

	// The worker checks the payment with the provider — the not-found answer
	// leaves it untouched instead of finalizing it as succeeded.
	count, err := h.services.Workers.ReconcilePendingPayments(h.ctx(), h.clock.Now())
	if err != nil {
		t.Fatalf("ReconcilePendingPayments() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("ReconcilePendingPayments() = %d, want 1 checked payment", count)
	}

	stored, err := h.payments.GetByID(h.ctx(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored.Status != domain.PaymentStatusPending {
		t.Fatalf("payment status = %q, want pending (no fabricated success)", stored.Status)
	}
	storedSub, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if storedSub.TariffID != sub.TariffID || storedSub.Status != domain.SubscriptionStatusActive {
		t.Fatalf("subscription = %s/%s, want the seeded pro subscription untouched",
			storedSub.TariffID, storedSub.Status)
	}
}

// revertToCrashMoment rewinds the local rows to the crash moment: the payment
// is pending again and the subscription still expired, while the provider
// keeps the captured charge.
func revertToCrashMoment(t *testing.T, h *integrationHarness, userID uuid.UUID, payment domain.SubscriptionPayment, expired time.Time) {
	t.Helper()
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
}

// requireRecoveredWithoutSecondCharge asserts the recovery tick finalized the
// payment from the provider status, reactivated the subscription, and never
// charged again nor duplicated the payment.
func requireRecoveredWithoutSecondCharge(t *testing.T, h *integrationHarness, userID, paymentID uuid.UUID) {
	t.Helper()
	recovered, err := h.payments.GetByID(h.ctx(), paymentID)
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
	if charges := h.provider.ChargeCount(paymentID); charges != 1 {
		t.Errorf("provider charge count = %d, want exactly 1 (no double charge)", charges)
	}
}

// TestWorkers_Integration_RenewalDoubleChargeGuardOnCrash proves the
// double-charge acceptance scenario on the real schema: a renewal payment the
// provider captured while the local application crashed is applied from the
// provider status on the next tick — without a second charge (the fake
// provider records confirmed amounts, so a second charge would show).
func TestWorkers_Integration_RenewalDoubleChargeGuardOnCrash(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	userID, _ := seedExpiredProSubscription(t, h, time.Hour)
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
	revertToCrashMoment(t, h, userID, payment, h.clock.Now().Add(-time.Hour))

	// The recovery tick must resolve from the provider status without
	// charging again. The fake provider's confirmed amount for the internal
	// payment id stays exactly one charge's worth.
	if _, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessRenewals() recovery error = %v", err)
	}
	requireRecoveredWithoutSecondCharge(t, h, userID, payment.ID)
}

// pinGraceEpisode anchors a grace episode to the fake clock (ticket #431):
// the subscription_transitions and subscription_payments rows carry the
// database clock's now() by default, while the dunning schedule runs on the
// harness clock — the same pinning agePaymentRow applies to stale payments.
// The log is append-only (updates are rejected), so the grace-entered entry is
// re-logged with the pinned instant: the same rows, the fake clock's times.
// Every payment created before the entry stays before it (the failed renewal
// charge that caused the grace).
func pinGraceEpisode(t *testing.T, h *integrationHarness, subID uuid.UUID, enteredAt time.Time) {
	t.Helper()
	if _, err := h.pool.Exec(h.ctx(),
		`DELETE FROM subscription_transitions WHERE subscription_id = $1 AND reason = 'grace_entered'`,
		subID); err != nil {
		t.Fatalf("drop grace entry for re-pin: %v", err)
	}
	var tariffID uuid.UUID
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT tariff_id FROM user_subscriptions WHERE id = $1`, subID).Scan(&tariffID); err != nil {
		t.Fatalf("read subscription tariff: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(), `
		INSERT INTO subscription_transitions (id, subscription_id, to_status, to_tariff_id, reason, initiator_type, created_at)
		VALUES ($1, $2, 'grace', $3, 'grace_entered', 'system', $4)`,
		uuid.Must(uuid.NewV7()), subID, tariffID, enteredAt); err != nil {
		t.Fatalf("re-log grace entry: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`UPDATE subscription_payments SET created_at = $1, updated_at = $1
		 WHERE subscription_id = $2 AND created_at > $3`,
		enteredAt.Add(-time.Minute), subID, enteredAt); err != nil {
		t.Fatalf("pin pre-entry payments: %v", err)
	}
}

// TestWorkers_Integration_GraceRetryRescuesSubscription proves the dunning
// acceptance scenario (ticket #431) end to end on the real schema: a declined
// renewal enters grace, the +24 h retry charges whatever method is active by
// then (a card switched after the failure), the success applies through the
// single seam and the subscription leaves grace renewed for a month from the
// retry. The +24 h boundary itself is pinned: an episode younger than a day
// retries nothing.
func TestWorkers_Integration_GraceRetryRescuesSubscription(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	userID, sub := seedExpiredProSubscription(t, h, time.Hour)
	seedActiveMethod(t, h, userID, "fake_fail_card")

	if _, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}
	requireGraceEnteredAfterFailedCharge(t, h, userID, sub.ID)

	// Younger than the +24 h boundary: the schedule retries nothing yet.
	pinGraceEpisode(t, h, sub.ID, h.clock.Now().Add(-23*time.Hour))
	if count, err := h.services.Workers.ProcessGraceRetries(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessGraceRetries() (too young) error = %v", err)
	} else if count != 0 {
		t.Fatalf("ProcessGraceRetries() (too young) = %d, want 0", count)
	}

	// The boundary passes and the user switched cards in between.
	pinGraceEpisode(t, h, sub.ID, h.clock.Now().Add(-25*time.Hour))
	seedActiveMethod(t, h, userID, "tok_retry_ok")

	count, err := h.services.Workers.ProcessGraceRetries(h.ctx(), h.clock.Now())
	if err != nil {
		t.Fatalf("ProcessGraceRetries() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("ProcessGraceRetries() = %d, want 1", count)
	}

	payments, err := h.payments.ListByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("ListByUserID() error = %v", err)
	}
	if len(payments) != 2 {
		t.Fatalf("payments = %d, want 2 (failed renewal + retry)", len(payments))
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
	requireRenewedActiveSubscription(t, h, userID, retry.ID)

	// The retry consumed the boundary: nothing is due anymore.
	if count, err := h.services.Workers.ProcessGraceRetries(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("second ProcessGraceRetries() error = %v", err)
	} else if count != 0 {
		t.Fatalf("second ProcessGraceRetries() = %d, want 0", count)
	}
}

// TestWorkers_Integration_GraceRetryFailureThenSecondRetry proves the rest of
// the dunning schedule: a declined +24 h retry leaves the subscription in
// grace with the window untouched, the +72 h retry fires with the card the
// user switched to meanwhile, and its success ends the episode. After the
// schedule is spent the phase charges nothing — the expired-grace phase keeps
// its old behaviour from there.
func TestWorkers_Integration_GraceRetryFailureThenSecondRetry(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	userID, sub := seedExpiredProSubscription(t, h, time.Hour)
	seedActiveMethod(t, h, userID, "fake_fail_card")

	if _, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}
	graceEnd := requireGraceEnteredAfterFailedCharge(t, h, userID, sub.ID)

	// The +24 h retry is declined: the subscription stays in its grace window.
	pinGraceEpisode(t, h, sub.ID, h.clock.Now().Add(-25*time.Hour))
	requireDeclinedGraceRetryKeepsWindow(t, h, userID, graceEnd)

	// The +72 h boundary passes and the user switched cards: the second retry
	// succeeds and ends the episode.
	h.clock.now = h.clock.Now().Add(48 * time.Hour) // Entry + 73 h.
	seedActiveMethod(t, h, userID, "tok_retry_late")
	if count, err := h.services.Workers.ProcessGraceRetries(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("second ProcessGraceRetries() error = %v", err)
	} else if count != 1 {
		t.Fatalf("second ProcessGraceRetries() = %d, want 1", count)
	}

	requireDunningEpisodePayments(t, h, userID)

	// The schedule is spent: a third tick charges nothing.
	if count, err := h.services.Workers.ProcessGraceRetries(h.ctx(), h.clock.Now().Add(time.Hour)); err != nil {
		t.Fatalf("third ProcessGraceRetries() error = %v", err)
	} else if count != 0 {
		t.Fatalf("third ProcessGraceRetries() = %d, want 0", count)
	}
}

// requireDeclinedGraceRetryKeepsWindow runs the +24 h retry against the
// failing card and asserts the declined charge left the subscription in its
// grace window untouched.
func requireDeclinedGraceRetryKeepsWindow(t *testing.T, h *integrationHarness, userID uuid.UUID, graceEnd time.Time) {
	t.Helper()
	if count, err := h.services.Workers.ProcessGraceRetries(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessGraceRetries() error = %v", err)
	} else if count != 1 {
		t.Fatalf("ProcessGraceRetries() = %d, want 1", count)
	}
	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.Status != domain.SubscriptionStatusGrace || stored.ValidUntil == nil || !stored.ValidUntil.Equal(graceEnd) {
		t.Errorf("subscription = %s until %v, want grace until %v (unchanged)", stored.Status, stored.ValidUntil, graceEnd)
	}
}

// requireDunningEpisodePayments asserts the ended episode's payment history —
// the failed renewal, the declined +24 h retry and the succeeded +72 h one —
// and the active renewal the success bought.
func requireDunningEpisodePayments(t *testing.T, h *integrationHarness, userID uuid.UUID) {
	t.Helper()
	payments, err := h.payments.ListByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("ListByUserID() error = %v", err)
	}
	succeeded := 0
	for _, p := range payments {
		switch p.Status {
		case domain.PaymentStatusSucceeded:
			succeeded++
			requireRenewedActiveSubscription(t, h, userID, p.ID)
		case domain.PaymentStatusFailed:
		default:
			t.Errorf("payment %s status = %q, want terminal", p.ID, p.Status)
		}
	}
	if succeeded != 1 || len(payments) != 3 {
		t.Fatalf("payments = %d (%d succeeded), want 3 (1 succeeded)", len(payments), succeeded)
	}
}

// TestWorkers_Integration_CancelledExpiryCarriesKeepChoice proves the cancel
// keep-choice hand-off end to end (issue #617): the choice survives on the
// subscription row, rides into the basic-limit enforcement at expiry, and is
// consumed by the fall to basic.
func TestWorkers_Integration_CancelledExpiryCarriesKeepChoice(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	archiver, _ := wireBridges(h)
	basic, err := h.tariffs.GetByName(h.ctx(), domain.TariffBasic)
	if err != nil {
		t.Fatalf("GetByName(basic) error = %v", err)
	}
	expired := h.clock.Now().Add(-time.Hour)
	keepID := uuid.Must(uuid.NewV7())
	archiver.activeExists = map[uuid.UUID]bool{keepID: true}
	// The cancel use case validates the keep choice through the same
	// properties bridge the composition root wires onto both services.
	h.services.Subscriptions.SetLifecycleBridges(archiver, nil)

	userID, sub := seedPaidProSubscription(t, h)
	sub.ValidUntil = &expired
	if err := h.subscriptions.Update(h.ctx(), sub); err != nil {
		t.Fatalf("expire cancelled: %v", err)
	}
	if err := h.subscriptionsSvc.CancelSubscription(h.ctx(), userID, &keepID); err != nil {
		t.Fatalf("CancelSubscription(keep) error = %v", err)
	}
	if stored, err := h.subscriptions.GetByUserID(h.ctx(), userID); err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	} else if stored.KeepPropertyID == nil || *stored.KeepPropertyID != keepID {
		t.Fatalf("stored KeepPropertyID = %v, want %v (the column did not round-trip)", stored.KeepPropertyID, keepID)
	}

	if _, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}

	calls := archiver.recorded()
	if len(calls) != 1 {
		t.Fatalf("archive calls = %d, want 1", len(calls))
	}
	if calls[0].keep == nil || *calls[0].keep != keepID {
		t.Errorf("archive keep = %v, want %v", calls[0].keep, keepID)
	}
	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.TariffID != basic.ID || stored.KeepPropertyID != nil {
		t.Errorf("subscription = %+v, want basic with the keep choice consumed", stored)
	}
}
