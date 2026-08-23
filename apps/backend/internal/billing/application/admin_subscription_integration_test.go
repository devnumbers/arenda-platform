//go:build integration

package application_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// The admin subscription operations of issue #255 against real PostgreSQL and
// the fake provider adapter: the service assignment that overwrites a paid
// subscription and expires to basic through the common worker path, the
// upgrade over a service subscription that turns it paid, the force tariff
// change, the manual grace extension and the admin cancellation.

// adminSubscriptionHarness extends the payment harness with an acting admin
// user (the audit log references the actor).
type adminSubscriptionHarness struct {
	*paymentIntegrationHarness
	adminID uuid.UUID
}

func newAdminSubscriptionHarness(t *testing.T) *adminSubscriptionHarness {
	t.Helper()
	h := &adminSubscriptionHarness{paymentIntegrationHarness: newPaymentIntegrationHarness(t)}
	h.adminID = h.seedUser()
	return h
}

// seedGraceSubscription creates a paid pro subscription inside an open grace
// window ending at the given offset from the fixed now. It lives on the
// payment harness so composition-level harnesses can grow a grace fixture
// too (the admin-gate composition test of issue #424 reuses it).
func (h *paymentIntegrationHarness) seedGraceSubscription(t *testing.T, graceIn time.Duration) domain.Subscription {
	t.Helper()
	sub := h.seedPaidSubscription(t, domain.TariffPro)
	graceUntil := h.clock.Now().Add(graceIn)
	sub.Status = domain.SubscriptionStatusGrace
	sub.ValidUntil = &graceUntil
	sub.AutoRenewEnabled = false
	if err := h.subscriptions.Update(h.ctx(), sub); err != nil {
		t.Fatalf("seed grace Update() error = %v", err)
	}
	return sub
}

// requireServiceAssignedShape asserts the assignment overwrote the paid
// subscription — the service source, auto-renew off, a month of the assigned
// plan, and no payment row — and returns the stored subscription for the later
// expiry step.
func (h *adminSubscriptionHarness) requireServiceAssignedShape(t *testing.T, sub domain.Subscription) domain.Subscription {
	t.Helper()
	stored, err := h.subscriptions.GetByUserID(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.Source != domain.SubscriptionSourceService {
		t.Errorf("Source = %q, want service", stored.Source)
	}
	if stored.TariffID != h.tariffIDByName(t, domain.TariffBusiness) {
		t.Errorf("TariffID = %v, want business", stored.TariffID)
	}
	if stored.AutoRenewEnabled {
		t.Error("AutoRenewEnabled = true, want false")
	}
	wantUntil := h.clock.Now().AddDate(0, 1, 0)
	if stored.ValidUntil == nil || !stored.ValidUntil.Equal(wantUntil) {
		t.Errorf("ValidUntil = %v, want %v", stored.ValidUntil, wantUntil)
	}

	// No payment row exists: the assignment is free of money movement.
	if payments := h.countRows("SELECT COUNT(*) FROM subscription_payments WHERE user_id = $1", sub.UserID); payments != 0 {
		t.Fatalf("subscription payments = %d, want 0", payments)
	}
	return stored
}

// requireServiceTermExpiredToBasic advances past the service term end and runs
// the renewal worker: the common non-renewing expiry path downgrades the
// subscription to basic and back onto the paid track.
func (h *adminSubscriptionHarness) requireServiceTermExpiredToBasic(t *testing.T, userID uuid.UUID, termEnd time.Time) {
	t.Helper()
	h.clock.now = termEnd.Add(time.Hour)
	if _, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}
	expired, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID(after expiry) error = %v", err)
	}
	if expired.TariffID != h.tariffIDByName(t, domain.TariffBasic) {
		t.Errorf("tariff after expiry = %v, want basic", expired.TariffID)
	}
	if expired.Source != domain.SubscriptionSourcePaid {
		t.Errorf("source after expiry = %q, want the basic plan back on the paid track", expired.Source)
	}
}

// requireAssignmentTransitionPair asserts the transition history records the
// assignment and the expiry — the seed bypasses onboarding, so there is no
// registered entry — both visible in the admin view with the tariff names
// resolved.
func (h *adminSubscriptionHarness) requireAssignmentTransitionPair(t *testing.T, sub domain.Subscription) {
	t.Helper()
	views, err := h.subscriptionsSvc.ListTransitions(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("ListTransitions() error = %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("transitions = %d, want 2 (service_assigned, expired)", len(views))
	}
	if views[0].Transition.Reason != domain.TransitionReasonExpired {
		t.Errorf("latest transition reason = %q, want expired", views[0].Transition.Reason)
	}
	if views[0].Transition.Initiator != domain.InitiatorSystem {
		t.Errorf("expiry initiator = %q, want system", views[0].Transition.Initiator)
	}
	if views[1].Transition.Reason != domain.TransitionReasonServiceAssigned {
		t.Errorf("second transition reason = %q, want service_assigned", views[1].Transition.Reason)
	}
	if views[1].Transition.Initiator != domain.InitiatorAdmin || views[1].Transition.InitiatorID == nil ||
		*views[1].Transition.InitiatorID != h.adminID {
		t.Errorf("assignment initiator = %q/%v, want the acting admin", views[1].Transition.Initiator, views[1].Transition.InitiatorID)
	}
	if views[1].ToTariffName != string(domain.TariffBusiness) {
		t.Errorf("assignment to-tariff name = %q, want business", views[1].ToTariffName)
	}
}

// requireSingleAuditEntry asserts exactly one audit row of the action exists,
// attributed to the acting admin and the entity.
func (h *adminSubscriptionHarness) requireSingleAuditEntry(t *testing.T, action, label string, entityID uuid.UUID) {
	t.Helper()
	if n := h.countRows(
		"SELECT COUNT(*) FROM audit_log WHERE action = $1 AND actor_id = $2 AND entity_id = $3",
		action, h.adminID, entityID); n != 1 {
		t.Fatalf("%s audit entries = %d, want 1", label, n)
	}
}

// TestAdminSubscription_AssignServiceOverwritesAndExpiresToBasic proves the
// service assignment against real PostgreSQL (issue #255): the assignment
// overwrites a paid subscription without payment, auto-renew is off, and once
// the term ends the expiry worker downgrades the subscription to basic through
// the common non-renewing path with its transition recorded.
func TestAdminSubscription_AssignServiceOverwritesAndExpiresToBasic(t *testing.T) {
	t.Parallel()
	h := newAdminSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffPro)

	if err := h.subscriptionsSvc.AssignServiceSubscription(h.ctx(), h.adminID, sub.UserID, billingapp.AssignServiceSubscriptionRequest{
		TariffName: domain.TariffBusiness,
		TermType:   billingapp.ServiceTermMonth,
	}); err != nil {
		t.Fatalf("AssignServiceSubscription() error = %v", err)
	}

	stored := h.requireServiceAssignedShape(t, sub)

	// The term runs out: the common non-renewing expiry path downgrades the
	// subscription to basic.
	h.requireServiceTermExpiredToBasic(t, sub.UserID, *stored.ValidUntil)
	h.requireAssignmentTransitionPair(t, sub)

	// The audit log attributes the assignment to the acting admin.
	h.requireSingleAuditEntry(t, "subscription.service_assigned", "service_assigned", sub.ID)
}

// TestAdminSubscription_UpgradeOverServiceTurnsPaid proves the
// upgrade-over-service rule end to end (issue #255): a user on a service
// subscription pays for an upgrade, and the applied payment converts the
// subscription into a paid one from the new period.
func TestAdminSubscription_UpgradeOverServiceTurnsPaid(t *testing.T) {
	t.Parallel()
	h := newAdminSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffPro)
	sub.Source = domain.SubscriptionSourceService
	sub.AutoRenewEnabled = false
	sub.CurrentPeriod = nil
	if err := h.subscriptions.Update(h.ctx(), sub); err != nil {
		t.Fatalf("seed service Update() error = %v", err)
	}

	result, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), sub.UserID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffBusiness,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff(upgrade over service) error = %v", err)
	}
	h.confirmFakePayment(t, result.PaymentID)

	stored, err := h.subscriptions.GetByUserID(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.Source != domain.SubscriptionSourcePaid {
		t.Errorf("Source = %q, want paid after the applied upgrade", stored.Source)
	}
	if !stored.AutoRenewEnabled {
		t.Error("AutoRenewEnabled = false, want true after the applied upgrade")
	}
	if stored.TariffID != h.tariffIDByName(t, domain.TariffBusiness) {
		t.Errorf("TariffID = %v, want business", stored.TariffID)
	}
	wantUntil := h.clock.Now().AddDate(0, 1, 0)
	if stored.ValidUntil == nil || !stored.ValidUntil.Equal(wantUntil) {
		t.Errorf("ValidUntil = %v, want %v (the period from the payment moment)", stored.ValidUntil, wantUntil)
	}

	views, err := h.subscriptionsSvc.ListTransitions(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("ListTransitions() error = %v", err)
	}
	if len(views) == 0 || views[0].Transition.Reason != domain.TransitionReasonPaymentApplied {
		t.Fatalf("latest transition reason = %v, want payment_applied", views[0].Transition.Reason)
	}
	if views[0].Transition.PaymentID == nil || *views[0].Transition.PaymentID != result.PaymentID {
		t.Errorf("payment transition reference = %v, want %v", views[0].Transition.PaymentID, result.PaymentID)
	}
}

// requireForcedChangeShape asserts the force change applied the new tariff
// for the chosen period without any payment row, keeping the paid source and
// the seeded auto-renew setting.
func (h *adminSubscriptionHarness) requireForcedChangeShape(t *testing.T, sub domain.Subscription) {
	t.Helper()
	stored, err := h.subscriptions.GetByUserID(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.TariffID != h.tariffIDByName(t, domain.TariffPro) {
		t.Errorf("TariffID = %v, want pro", stored.TariffID)
	}
	if stored.Source != domain.SubscriptionSourcePaid {
		t.Errorf("Source = %q, want the unchanged paid source", stored.Source)
	}
	if !stored.AutoRenewEnabled {
		t.Error("AutoRenewEnabled = false, want the seeded value kept — force change does not touch auto-renew")
	}
	wantUntil := h.clock.Now().AddDate(1, 0, 0)
	if stored.ValidUntil == nil || !stored.ValidUntil.Equal(wantUntil) {
		t.Errorf("ValidUntil = %v, want %v", stored.ValidUntil, wantUntil)
	}
	if payments := h.countRows("SELECT COUNT(*) FROM subscription_payments WHERE user_id = $1", sub.UserID); payments != 0 {
		t.Fatalf("subscription payments = %d, want 0", payments)
	}
}

// requireForcedChangeTransition asserts the newest transition entry is the
// admin-attributed forced_change.
func (h *adminSubscriptionHarness) requireForcedChangeTransition(t *testing.T, sub domain.Subscription) {
	t.Helper()
	views, err := h.subscriptionsSvc.ListTransitions(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("ListTransitions() error = %v", err)
	}
	if len(views) == 0 || views[0].Transition.Reason != domain.TransitionReasonForcedChange {
		t.Fatalf("latest transition reason = %v, want forced_change", views[0].Transition.Reason)
	}
	if views[0].Transition.Initiator != domain.InitiatorAdmin || views[0].Transition.InitiatorID == nil ||
		*views[0].Transition.InitiatorID != h.adminID {
		t.Errorf("forced-change initiator = %q/%v, want the acting admin", views[0].Transition.Initiator, views[0].Transition.InitiatorID)
	}
}

// TestAdminSubscription_ForceChangeAppliesWithoutPayment proves the force
// tariff change against real PostgreSQL (issue #255): the new tariff applies
// immediately for the chosen period without any payment row, and the
// transition log and audit trail attribute it to the acting admin.
func TestAdminSubscription_ForceChangeAppliesWithoutPayment(t *testing.T) {
	t.Parallel()
	h := newAdminSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffBusiness)

	if err := h.subscriptionsSvc.ForceChangeTariff(h.ctx(), h.adminID, sub.UserID, billingapp.ForceChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodYear,
	}); err != nil {
		t.Fatalf("ForceChangeTariff() error = %v", err)
	}

	h.requireForcedChangeShape(t, sub)
	h.requireForcedChangeTransition(t, sub)
	h.requireSingleAuditEntry(t, "subscription.tariff_forced", "tariff_forced", sub.ID)
}

// requireGraceExtendedWindow asserts the grace window lengthened to the wanted
// deadline.
func (h *adminSubscriptionHarness) requireGraceExtendedWindow(t *testing.T, sub domain.Subscription, want time.Time) {
	t.Helper()
	stored, err := h.subscriptions.GetByUserID(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.ValidUntil == nil || !stored.ValidUntil.Equal(want) {
		t.Fatalf("ValidUntil = %v, want %v", stored.ValidUntil, want)
	}
}

// requireGraceSurvivesOriginalDeadline advances past the original deadline and
// runs the grace-expiry worker: the extended subscription survives it.
func (h *adminSubscriptionHarness) requireGraceSurvivesOriginalDeadline(t *testing.T, sub domain.Subscription, originalUntil time.Time) {
	t.Helper()
	// The original deadline passes; the extended subscription survives.
	h.clock.now = originalUntil.Add(time.Hour)
	if _, err := h.services.Workers.ProcessExpiredGrace(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessExpiredGrace() error = %v", err)
	}
	survived, err := h.subscriptions.GetByUserID(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID(after the original deadline) error = %v", err)
	}
	if survived.Status != domain.SubscriptionStatusGrace {
		t.Errorf("status = %q, want grace to survive the original deadline", survived.Status)
	}
	if !survived.CanMutateData(h.clock.Now()) {
		t.Error("extended grace must stay mutable")
	}
}

// requireGraceExpiryDowngradesToBasic advances past the extended deadline and
// runs the grace-expiry worker: the worker downgrades the subscription to
// basic.
func (h *adminSubscriptionHarness) requireGraceExpiryDowngradesToBasic(t *testing.T, sub domain.Subscription, extendedUntil time.Time) {
	t.Helper()
	// The extended deadline passes; the worker downgrades to basic.
	h.clock.now = extendedUntil.Add(time.Hour)
	if _, err := h.services.Workers.ProcessExpiredGrace(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessExpiredGrace(extended) error = %v", err)
	}
	expired, err := h.subscriptions.GetByUserID(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID(after the extended deadline) error = %v", err)
	}
	if expired.TariffID != h.tariffIDByName(t, domain.TariffBasic) {
		t.Errorf("tariff after the extended grace = %v, want basic", expired.TariffID)
	}
}

// requireGraceExtendedTransition asserts the transition log carries the
// admin-attributed grace_extended entry.
func (h *adminSubscriptionHarness) requireGraceExtendedTransition(t *testing.T, sub domain.Subscription) {
	t.Helper()
	views, err := h.subscriptionsSvc.ListTransitions(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("ListTransitions() error = %v", err)
	}
	found := false
	for _, view := range views {
		if view.Transition.Reason == domain.TransitionReasonGraceExtended {
			found = true
			if view.Transition.Initiator != domain.InitiatorAdmin || view.Transition.InitiatorID == nil ||
				*view.Transition.InitiatorID != h.adminID {
				t.Errorf("grace-extension initiator = %q/%v, want the acting admin", view.Transition.Initiator, view.Transition.InitiatorID)
			}
		}
	}
	if !found {
		t.Error("no grace_extended transition recorded")
	}
}

// TestAdminSubscription_ExtendGraceKeepsWindowAlive proves the manual grace
// extension against real PostgreSQL (issue #255): the window lengthens from
// the current deadline, the grace-expiry worker no longer downgrades the
// subscription at the original deadline, and the transition log records the
// extension.
func TestAdminSubscription_ExtendGraceKeepsWindowAlive(t *testing.T) {
	t.Parallel()
	h := newAdminSubscriptionHarness(t)
	sub := h.seedGraceSubscription(t, 24*time.Hour)
	originalUntil := *sub.ValidUntil

	if err := h.subscriptionsSvc.ExtendGrace(h.ctx(), h.adminID, sub.UserID, 3); err != nil {
		t.Fatalf("ExtendGrace() error = %v", err)
	}

	want := originalUntil.Add(3 * 24 * time.Hour)
	h.requireGraceExtendedWindow(t, sub, want)
	h.requireGraceSurvivesOriginalDeadline(t, sub, originalUntil)
	h.requireGraceExpiryDowngradesToBasic(t, sub, want)
	h.requireGraceExtendedTransition(t, sub)
	h.requireSingleAuditEntry(t, "subscription.grace_extended", "grace_extended", sub.ID)
}

// requireCancelledRunsOutPaidPeriod advances past the retained paid period and
// runs the renewal worker: the cancelled subscription falls to basic through
// the cancelled-expiry path.
func (h *adminSubscriptionHarness) requireCancelledRunsOutPaidPeriod(
	t *testing.T, userID uuid.UUID, retainedUntil *time.Time,
) {
	t.Helper()
	h.clock.now = retainedUntil.Add(time.Hour)
	if _, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}
	expired, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID(after the retained period) error = %v", err)
	}
	if expired.TariffID != h.tariffIDByName(t, domain.TariffBasic) {
		t.Errorf("tariff after the retained period = %v, want basic", expired.TariffID)
	}
}

// TestAdminSubscription_AdminCancelRunsOutPaidPeriod proves the admin
// cancellation against real PostgreSQL (issue #255): the subscription cancels
// with the admin attribution, stays working until the retained period ends,
// then falls to basic through the cancelled-expiry worker path.
func TestAdminSubscription_AdminCancelRunsOutPaidPeriod(t *testing.T) {
	t.Parallel()
	h := newAdminSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffPro)

	if err := h.subscriptionsSvc.CancelSubscriptionAsAdmin(h.ctx(), h.adminID, sub.UserID); err != nil {
		t.Fatalf("CancelSubscriptionAsAdmin() error = %v", err)
	}

	cancelled, err := h.subscriptions.GetByUserID(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if cancelled.Status != domain.SubscriptionStatusCancelled {
		t.Errorf("Status = %q, want cancelled", cancelled.Status)
	}
	if !cancelled.CanMutateData(h.clock.Now()) {
		t.Error("cancelled subscription must stay mutable until the paid period ends")
	}

	views, err := h.subscriptionsSvc.ListTransitions(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("ListTransitions() error = %v", err)
	}
	if len(views) == 0 || views[0].Transition.Reason != domain.TransitionReasonCancelled {
		t.Fatalf("latest transition reason = %v, want cancelled", views[0].Transition.Reason)
	}
	if views[0].Transition.Initiator != domain.InitiatorAdmin || views[0].Transition.InitiatorID == nil ||
		*views[0].Transition.InitiatorID != h.adminID {
		t.Errorf("cancellation initiator = %q/%v, want the acting admin", views[0].Transition.Initiator, views[0].Transition.InitiatorID)
	}
	if n := h.countRows(
		"SELECT COUNT(*) FROM audit_log WHERE action = 'subscription.cancelled' AND actor_id = $1 AND entity_id = $2",
		h.adminID, sub.ID); n != 1 {
		t.Fatalf("cancelled audit entries = %d, want 1", n)
	}

	h.requireCancelledRunsOutPaidPeriod(t, sub.UserID, cancelled.ValidUntil)
}
