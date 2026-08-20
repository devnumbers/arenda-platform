//go:build integration

package application_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// seedPaidProSubscription onboards the user and lifts them onto a paid pro
// subscription with a month of validity, mirroring the state a successful
// payment leaves behind (the payment flow itself lands with issue #250).
func seedPaidProSubscription(t *testing.T, h *integrationHarness) (uuid.UUID, domain.Subscription) {
	t.Helper()
	userID := h.seedUser()
	if err := h.onboarding.OnUserRegistered(h.ctx(), userID); err != nil {
		t.Fatalf("OnUserRegistered() error = %v", err)
	}
	sub, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	pro, err := h.tariffs.GetByName(h.ctx(), domain.TariffPro)
	if err != nil {
		t.Fatalf("GetByName(pro) error = %v", err)
	}
	validUntil := h.clock.Now().AddDate(0, 1, 0)
	period := domain.PeriodMonth
	sub.TariffID = pro.ID
	sub.ValidUntil = &validUntil
	sub.AutoRenewEnabled = true
	sub.CurrentPeriod = &period
	if err := h.subscriptions.Update(h.ctx(), sub); err != nil {
		t.Fatalf("seed Update() error = %v", err)
	}
	return userID, sub
}

// requireCancelledKeepsPaidPeriod asserts the cancelled subscription retains
// its paid period with auto-renew off and drops any scheduled change.
func (h *integrationHarness) requireCancelledKeepsPaidPeriod(t *testing.T, sub domain.Subscription) {
	t.Helper()
	stored, err := h.subscriptions.GetByUserID(h.ctx(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.Status != domain.SubscriptionStatusCancelled {
		t.Errorf("Status = %q, want cancelled", stored.Status)
	}
	if stored.AutoRenewEnabled {
		t.Error("AutoRenewEnabled = true, want false")
	}
	if stored.ValidUntil == nil || !stored.ValidUntil.Equal(*sub.ValidUntil) {
		t.Errorf("ValidUntil = %v, want the retained paid period", stored.ValidUntil)
	}
	if stored.HasPendingChange() {
		t.Error("scheduled downgrade survived cancellation, want it dropped")
	}
}

// requireCancelledTransitionLog asserts the transition log ends with the
// user-initiated cancelled entry behind the registered and downgrade_scheduled
// ones.
func (h *integrationHarness) requireCancelledTransitionLog(t *testing.T, sub domain.Subscription) {
	t.Helper()
	transitions, err := h.transitions.ListBySubscriptionID(h.ctx(), sub.ID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID() error = %v", err)
	}
	if len(transitions) != 3 {
		t.Fatalf("transitions = %d, want 3 (registered + downgrade_scheduled + cancelled)", len(transitions))
	}
	tr := transitions[0] // Newest first.
	if tr.Reason != domain.TransitionReasonCancelled {
		t.Errorf("Reason = %q, want %q", tr.Reason, domain.TransitionReasonCancelled)
	}
	if tr.Initiator != domain.InitiatorUser || tr.InitiatorID == nil || *tr.InitiatorID != sub.UserID {
		t.Errorf("initiator = %q/%v, want user/%v", tr.Initiator, tr.InitiatorID, sub.UserID)
	}
}

// TestSubscriptionLifecycle_Integration_Cancel proves the cancellation use
// case against real PostgreSQL: the subscription row moves to cancelled with
// auto-renew off while the paid period is retained, and the transition log
// and audit trail capture the user-initiated change in the same transaction
// (issue #249).
func TestSubscriptionLifecycle_Integration_Cancel(t *testing.T) {
	h := newIntegrationHarness(t)
	userID, sub := seedPaidProSubscription(t, h)
	// A scheduled downgrade exists before the cancellation: it must be dropped
	// together with the cancellation, because a cancelled subscription runs out
	// its paid period and falls to basic instead of switching tariffs.
	if _, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), userID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffBasic,
		Period:     domain.PeriodMonth,
	}); err != nil {
		t.Fatalf("ChangeTariff() error = %v", err)
	}

	if err := h.subscriptionsSvc.CancelSubscription(h.ctx(), userID); err != nil {
		t.Fatalf("CancelSubscription() error = %v", err)
	}

	h.requireCancelledKeepsPaidPeriod(t, sub)
	h.requireCancelledTransitionLog(t, sub)

	if got := h.countRows(
		`SELECT count(*) FROM audit_log WHERE action = 'subscription.cancelled' AND entity_id = $1 AND actor_id = $2`,
		sub.ID, userID); got != 1 {
		t.Errorf("audit rows = %d, want 1", got)
	}
}

// TestSubscriptionLifecycle_Integration_ToggleAutoRenew proves the auto-renew
// use case persists the flag and its audit entry, and that enabling without a
// validity period is rejected by the domain rule.
func TestSubscriptionLifecycle_Integration_ToggleAutoRenew(t *testing.T) {
	h := newIntegrationHarness(t)
	userID, sub := seedPaidProSubscription(t, h)

	if err := h.subscriptionsSvc.ToggleAutoRenew(h.ctx(), userID, false); err != nil {
		t.Fatalf("ToggleAutoRenew(false) error = %v", err)
	}
	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.AutoRenewEnabled {
		t.Error("AutoRenewEnabled = true after disable, want false")
	}
	if got := h.countRows(
		`SELECT count(*) FROM audit_log WHERE action = 'subscription.auto_renew_toggled' AND entity_id = $1`,
		sub.ID); got != 1 {
		t.Errorf("audit rows = %d, want 1", got)
	}

	if err := h.subscriptionsSvc.ToggleAutoRenew(h.ctx(), userID, true); err != nil {
		t.Fatalf("ToggleAutoRenew(true) error = %v", err)
	}

	// A fresh basic subscription has no validity period: enabling auto-renew
	// is a domain violation.
	basicUser := h.seedUser()
	if err := h.onboarding.OnUserRegistered(h.ctx(), basicUser); err != nil {
		t.Fatalf("OnUserRegistered() error = %v", err)
	}
	if err := h.subscriptionsSvc.ToggleAutoRenew(h.ctx(), basicUser, true); !errors.Is(err, domain.ErrCannotEnableAutoRenew) {
		t.Fatalf("err = %v, want domain.ErrCannotEnableAutoRenew", err)
	}
}

// requireDowngradeScheduledState asserts the deferred change (tariff, period,
// date) persists with the enabled auto-renew while the current tariff stays.
func (h *integrationHarness) requireDowngradeScheduledState(
	t *testing.T, sub, stored domain.Subscription, basic domain.Tariff,
) {
	t.Helper()
	if stored.TariffID != sub.TariffID {
		t.Errorf("TariffID = %v, want unchanged %v (downgrade is deferred)", stored.TariffID, sub.TariffID)
	}
	if stored.PendingTariffID == nil || *stored.PendingTariffID != basic.ID {
		t.Errorf("PendingTariffID = %v, want basic", stored.PendingTariffID)
	}
	if stored.PendingPeriod == nil || *stored.PendingPeriod != domain.PeriodMonth {
		t.Errorf("PendingPeriod = %v, want month", stored.PendingPeriod)
	}
	if stored.PendingChangeAt == nil || !stored.PendingChangeAt.Equal(*sub.ValidUntil) {
		t.Errorf("PendingChangeAt = %v, want %v", stored.PendingChangeAt, sub.ValidUntil)
	}
	if !stored.AutoRenewEnabled {
		t.Error("AutoRenewEnabled = false, want true")
	}
}

// requireDowngradeScheduledTransitionLog asserts the transition log ends with
// the user-initiated downgrade_scheduled entry naming the basic target.
func (h *integrationHarness) requireDowngradeScheduledTransitionLog(
	t *testing.T, sub domain.Subscription, basic domain.Tariff,
) {
	t.Helper()
	transitions, err := h.transitions.ListBySubscriptionID(h.ctx(), sub.ID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID() error = %v", err)
	}
	if len(transitions) != 2 {
		t.Fatalf("transitions = %d, want 2 (registered + downgrade_scheduled)", len(transitions))
	}
	tr := transitions[0] // Newest first.
	if tr.Reason != domain.TransitionReasonDowngradeScheduled {
		t.Errorf("Reason = %q, want %q", tr.Reason, domain.TransitionReasonDowngradeScheduled)
	}
	if tr.ToTariffID != basic.ID {
		t.Errorf("ToTariffID = %v, want the scheduled basic tariff", tr.ToTariffID)
	}
	if tr.Initiator != domain.InitiatorUser {
		t.Errorf("Initiator = %q, want user", tr.Initiator)
	}
}

// TestSubscriptionLifecycle_Integration_DowngradeScheduling proves the
// downgrade use case against real PostgreSQL: the deferred change (tariff,
// period, date) and the enabled auto-renew persist, and the transition log
// and audit capture the scheduling (issue #249, ADR 0008 §3).
func TestSubscriptionLifecycle_Integration_DowngradeScheduling(t *testing.T) {
	h := newIntegrationHarness(t)
	userID, sub := seedPaidProSubscription(t, h)
	basic, err := h.tariffs.GetByName(h.ctx(), domain.TariffBasic)
	if err != nil {
		t.Fatalf("GetByName(basic) error = %v", err)
	}

	result, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), userID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffBasic,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff() error = %v", err)
	}
	if result.PaymentID != uuid.Nil || result.ConfirmURL != "" {
		t.Errorf("result = %+v, want zero values (no payment on the free path)", result)
	}

	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	h.requireDowngradeScheduledState(t, sub, stored, basic)
	h.requireDowngradeScheduledTransitionLog(t, sub, basic)

	if got := h.countRows(
		`SELECT count(*) FROM audit_log WHERE action = 'subscription.tariff_changed' AND entity_id = $1 AND actor_id = $2`,
		sub.ID, userID); got != 1 {
		t.Errorf("audit rows = %d, want 1", got)
	}
}

// TestSubscriptionLifecycle_Integration_ChangeTariffRejections proves the
// rejection paths of the tariff-change use case on the real schema: a
// same-tariff request on an active subscription leaves no trace, and an
// upgrade no longer fails with the temporary payment-unavailable error of
// issue #249 — it starts a pending payment (issue #250).
func TestSubscriptionLifecycle_Integration_ChangeTariffRejections(t *testing.T) {
	h := newIntegrationHarness(t)
	userID, sub := seedPaidProSubscription(t, h)

	if _, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), userID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	}); !errors.Is(err, domain.ErrAlreadyOnTariff) {
		t.Fatalf("same-tariff err = %v, want domain.ErrAlreadyOnTariff", err)
	}

	result, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), userID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffBusiness,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("upgrade err = %v, want a started payment (issue #250)", err)
	}
	if result.PaymentID == uuid.Nil || result.ConfirmURL == "" {
		t.Fatalf("result = %+v, want a payment id and a payer url", result)
	}
	if got := h.countRows(`SELECT count(*) FROM subscription_payments WHERE id = $1 AND status = 'pending'`, result.PaymentID); got != 1 {
		t.Errorf("pending payment rows = %d, want 1", got)
	}

	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.HasPendingChange() {
		t.Error("a paid upgrade must not leave a scheduled pending change")
	}
	if got := h.countRows(`SELECT count(*) FROM subscription_transitions WHERE subscription_id = $1`, sub.ID); got != 1 {
		t.Errorf("transitions = %d, want 1 (only the onboarding registration; the tariff lands with the payment)", got)
	}
}
