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

// The stand-only time-travel rig of issue #665 against real PostgreSQL: the
// admin shift moves the subscription's temporal boundaries coherently (the
// aggregate's valid_until, the dunning anchor in the transition log and the
// payments' created_at travel by one delta), the presets resolve against the
// live boundaries, and the untouched worker phases pick the moved boundaries
// up on the next tick — the acceptance walks the full lifecycle without
// waiting out real hours.

// seedGraceEpisode drives a real grace entry at the fake clock's now: an
// expired pro subscription is charged on a declining method, the renewal
// fails into grace, and the episode is pinned to the fake now (the real entry
// carries the database's wall clock, the harness clock stays fixed). The
// reminder flag is set, so a shift must open a fresh reminder window.
// Returns the user id, the stored grace subscription and the dunning anchor.
func seedGraceEpisode(t *testing.T, h *adminSubscriptionHarness) (uuid.UUID, domain.Subscription, time.Time) {
	t.Helper()
	base := h.integrationHarness
	userID, sub := seedExpiredProSubscription(t, base, time.Hour)
	seedActiveMethod(t, base, userID, "fake_fail_card")
	if _, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}
	requireGraceEnteredAfterFailedCharge(t, base, userID, sub.ID)
	pinGraceEpisode(t, base, sub.ID, h.clock.Now())

	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	reminded := h.clock.Now()
	stored.MarkGraceReminded(reminded)
	if err := h.subscriptions.Update(h.ctx(), stored); err != nil {
		t.Fatalf("seed reminded flag: %v", err)
	}
	return userID, stored, h.clock.Now()
}

// latestGraceEntryAt reads the dunning anchor's created_at from the log.
func latestGraceEntryAt(t *testing.T, h *integrationHarness, subID uuid.UUID) time.Time {
	t.Helper()
	transitions, err := h.transitions.ListBySubscriptionID(h.ctx(), subID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID() error = %v", err)
	}
	for _, transition := range transitions {
		if transition.Reason == domain.TransitionReasonGraceEntered {
			return transition.CreatedAt
		}
	}
	t.Fatal("no grace_entered transition in the log")
	return time.Time{}
}

// TestAdmin_Integration_TimeShiftMovesGraceEpisode proves the coherent raw
// shift: valid_until, the dunning anchor and the failed renewal payment all
// move by −25 h, the reminder flag resets to a fresh window, and the move
// lands its time_shifted transition and audit record attributed to the
// acting admin.
func TestAdmin_Integration_TimeShiftMovesGraceEpisode(t *testing.T) {
	t.Parallel()
	h := newAdminSubscriptionHarness(t)
	userID, sub, anchor := seedGraceEpisode(t, h)

	delta := -25 * time.Hour
	if err := h.subscriptionsSvc.ShiftSubscriptionTime(h.ctx(), h.adminID, userID, billingapp.TimeShiftRequest{Shift: delta}); err != nil {
		t.Fatalf("ShiftSubscriptionTime() error = %v", err)
	}

	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID(after shift) error = %v", err)
	}
	wantUntil := sub.ValidUntil.Add(delta)
	if stored.ValidUntil == nil || !stored.ValidUntil.Equal(wantUntil) {
		t.Errorf("ValidUntil = %v, want %v", stored.ValidUntil, wantUntil)
	}
	if stored.Status != domain.SubscriptionStatusGrace {
		t.Errorf("status after shift = %q, want grace", stored.Status)
	}
	if stored.GraceRemindedAt != nil {
		t.Errorf("GraceRemindedAt = %v, want nil for the fresh reminder window", stored.GraceRemindedAt)
	}
	if got := latestGraceEntryAt(t, h.integrationHarness, sub.ID); !got.Equal(anchor.Add(delta)) {
		t.Errorf("dunning anchor = %v, want %v", got, anchor.Add(delta))
	}

	payments, err := h.payments.ListByUserID(h.ctx(), userID)
	if err != nil || len(payments) != 1 {
		t.Fatalf("payments = %d (err %v), want the single failed renewal", len(payments), err)
	}
	// The pinned pre-entry payment sits a minute before the anchor; the
	// shift must carry it along.
	wantPaymentAt := anchor.Add(-time.Minute).Add(delta)
	if !payments[0].CreatedAt.Equal(wantPaymentAt) {
		t.Errorf("payment created_at = %v, want %v", payments[0].CreatedAt, wantPaymentAt)
	}

	transitions, err := h.transitions.ListBySubscriptionID(h.ctx(), sub.ID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID(transitions) error = %v", err)
	}
	requireTimeShiftedTrail(t, h.integrationHarness, h.adminID, sub.ID, transitions[0])
}

// requireTimeShiftedTrail asserts the shift left its paper trail: the latest
// log entry is the admin's time_shifted and the audit log carries one
// subscription.time_shifted record for the subscription.
func requireTimeShiftedTrail(
	t *testing.T, h *integrationHarness, adminID, subID uuid.UUID, latest domain.Transition,
) {
	t.Helper()
	if latest.Reason != domain.TransitionReasonTimeShifted || latest.Initiator != domain.InitiatorAdmin {
		t.Errorf("latest transition = %q/%q, want time_shifted by the acting admin", latest.Reason, latest.Initiator)
	}
	if latest.InitiatorID == nil || *latest.InitiatorID != adminID {
		t.Errorf("shift initiator id = %v, want the acting admin", latest.InitiatorID)
	}
	if got := h.countRows(
		`SELECT count(*) FROM audit_log WHERE action = 'subscription.time_shifted' AND entity_id = $1`,
		subID); got != 1 {
		t.Errorf("time_shifted audit rows = %d, want 1", got)
	}
}

// TestAdmin_Integration_TimeShiftRetryPresetDrivesRetryPhase proves the +24 h
// preset end to end: the preset lands the dunning anchor exactly past the
// first retry boundary, and the untouched retry phase charges the retry on
// the next pass — the success renews the subscription out of grace.
func TestAdmin_Integration_TimeShiftRetryPresetDrivesRetryPhase(t *testing.T) {
	t.Parallel()
	h := newAdminSubscriptionHarness(t)
	userID, sub, _ := seedGraceEpisode(t, h)

	// The episode is brand new: the schedule retries nothing yet.
	if count, err := h.services.Workers.ProcessGraceRetries(h.ctx(), h.clock.Now()); err != nil || count != 0 {
		t.Fatalf("ProcessGraceRetries(before shift) = %d (err %v), want 0", count, err)
	}

	// The user fixes their payment method; the preset moves the anchor.
	seedActiveMethod(t, h.integrationHarness, userID, "tok_retry_ok")
	if err := h.subscriptionsSvc.ShiftSubscriptionTime(h.ctx(), h.adminID, userID,
		billingapp.TimeShiftRequest{Preset: billingapp.TimeShiftPresetRetryFirstDue}); err != nil {
		t.Fatalf("ShiftSubscriptionTime(retry_24h_due) error = %v", err)
	}
	if got := latestGraceEntryAt(t, h.integrationHarness, sub.ID); !got.Equal(h.clock.Now().Add(-25 * time.Hour)) {
		t.Errorf("dunning anchor = %v, want the +24 h preset anchor", got)
	}

	if count, err := h.services.Workers.ProcessGraceRetries(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessGraceRetries(after shift) error = %v", err)
	} else if count != 1 {
		t.Fatalf("ProcessGraceRetries(after shift) = %d, want 1", count)
	}
	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID(after retry) error = %v", err)
	}
	if stored.Status != domain.SubscriptionStatusActive {
		t.Errorf("status after the retry = %q, want active (the retry renewed)", stored.Status)
	}
}

// TestAdmin_Integration_TimeShiftReminderPresetOpensWindow proves the
// reminder preset: the grace deadline lands a day out — inside the reminder
// window — and the untouched reminder phase dispatches on the next pass.
func TestAdmin_Integration_TimeShiftReminderPresetOpensWindow(t *testing.T) {
	t.Parallel()
	h := newAdminSubscriptionHarness(t)
	userID, _, _ := seedGraceEpisode(t, h)

	if err := h.subscriptionsSvc.ShiftSubscriptionTime(h.ctx(), h.adminID, userID,
		billingapp.TimeShiftRequest{Preset: billingapp.TimeShiftPresetReminderWindow}); err != nil {
		t.Fatalf("ShiftSubscriptionTime(reminder_window) error = %v", err)
	}
	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	wantUntil := h.clock.Now().Add(24 * time.Hour)
	if stored.ValidUntil == nil || !stored.ValidUntil.Equal(wantUntil) {
		t.Fatalf("ValidUntil = %v, want the preset target %v", stored.ValidUntil, wantUntil)
	}

	if count, err := h.services.Workers.ProcessGraceExpiryReminders(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessGraceExpiryReminders() error = %v", err)
	} else if count != 1 {
		t.Fatalf("ProcessGraceExpiryReminders() = %d, want 1", count)
	}
	reminded, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID(after reminder) error = %v", err)
	}
	if reminded.GraceRemindedAt == nil {
		t.Error("GraceRemindedAt = nil, want the dispatched reminder")
	}
}

// TestAdmin_Integration_TimeShiftExpiredPresetEndsGrace proves «истечь из
// grace»: the expired preset lands valid_until an hour into the past and the
// untouched expired-grace phase downgrades to basic on the next pass.
func TestAdmin_Integration_TimeShiftExpiredPresetEndsGrace(t *testing.T) {
	t.Parallel()
	h := newAdminSubscriptionHarness(t)
	userID, _, _ := seedGraceEpisode(t, h)

	if err := h.subscriptionsSvc.ShiftSubscriptionTime(h.ctx(), h.adminID, userID,
		billingapp.TimeShiftRequest{Preset: billingapp.TimeShiftPresetPeriodExpired}); err != nil {
		t.Fatalf("ShiftSubscriptionTime(period_expired) error = %v", err)
	}
	if count, err := h.services.Workers.ProcessExpiredGrace(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessExpiredGrace() error = %v", err)
	} else if count != 1 {
		t.Fatalf("ProcessExpiredGrace() = %d, want 1", count)
	}
	requireBasicDowngrade(t, h.integrationHarness, userID)
}

// TestAdmin_Integration_TimeShiftEnterGraceFromActive proves «войти в grace»:
// the same expired preset applied to an active subscription backdates its
// paid period, and the untouched renewal phase fails the charge into grace.
func TestAdmin_Integration_TimeShiftEnterGraceFromActive(t *testing.T) {
	t.Parallel()
	h := newAdminSubscriptionHarness(t)
	userID, sub := seedPaidProSubscription(t, h.integrationHarness)
	seedActiveMethod(t, h.integrationHarness, userID, "fake_fail_card")

	if err := h.subscriptionsSvc.ShiftSubscriptionTime(h.ctx(), h.adminID, userID,
		billingapp.TimeShiftRequest{Preset: billingapp.TimeShiftPresetPeriodExpired}); err != nil {
		t.Fatalf("ShiftSubscriptionTime(period_expired) error = %v", err)
	}
	if count, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	} else if count != 1 {
		t.Fatalf("ProcessRenewals() = %d, want 1", count)
	}
	requireGraceEnteredAfterFailedCharge(t, h.integrationHarness, userID, sub.ID)
}

// TestAdmin_Integration_TimeShiftValidation pins the input and state guards
// of the rig: the request shape, the ±90 d cap, the basic subscription with
// nothing to travel, and the service-level railguard.
func TestAdmin_Integration_TimeShiftValidation(t *testing.T) {
	t.Parallel()
	h := newAdminSubscriptionHarness(t)
	userID, sub, _ := seedGraceEpisode(t, h)

	cases := []struct {
		name string
		req  billingapp.TimeShiftRequest
		want error
	}{
		{"zero raw shift", billingapp.TimeShiftRequest{Shift: 0}, domain.ErrInvalidTimeShift},
		{"preset and shift together", billingapp.TimeShiftRequest{
			Shift:  time.Hour,
			Preset: billingapp.TimeShiftPresetReminderWindow,
		}, domain.ErrInvalidTimeShift},
		{"unknown preset", billingapp.TimeShiftRequest{
			Preset: billingapp.TimeShiftPreset("go_back_in_time"),
		}, domain.ErrInvalidTimeShift},
		{"beyond the cap", billingapp.TimeShiftRequest{Shift: 91 * 24 * time.Hour}, domain.ErrInvalidTimeShift},
	}
	for _, tc := range cases {
		err := h.subscriptionsSvc.ShiftSubscriptionTime(h.ctx(), h.adminID, userID, tc.req)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: ShiftSubscriptionTime() error = %v, want %v", tc.name, err, tc.want)
		}
	}

	// The retry presets re-anchor a grace window: outside grace the state
	// guard fires — a basic subscription has neither the status nor the
	// boundary the rig travels.
	basic, err := h.tariffs.GetByName(h.ctx(), domain.TariffBasic)
	if err != nil {
		t.Fatalf("GetByName(basic) error = %v", err)
	}
	sub.DowngradeToBasic(basic.ID)
	if err := h.subscriptions.Update(h.ctx(), sub); err != nil {
		t.Fatalf("downgrade seed: %v", err)
	}
	if err := h.subscriptionsSvc.ShiftSubscriptionTime(h.ctx(), h.adminID, userID,
		billingapp.TimeShiftRequest{Preset: billingapp.TimeShiftPresetRetryFirstDue}); !errors.Is(err, domain.ErrInvalidSubscriptionState) {
		t.Errorf("retry preset outside grace: error = %v, want ErrInvalidSubscriptionState", err)
	}
	if err := h.subscriptionsSvc.ShiftSubscriptionTime(h.ctx(), h.adminID, userID,
		billingapp.TimeShiftRequest{Shift: -time.Hour}); !errors.Is(err, domain.ErrInvalidTimeShift) {
		t.Errorf("basic subscription: error = %v, want ErrInvalidTimeShift", err)
	}

	// A user without a subscription has nothing to travel.
	if err := h.subscriptionsSvc.ShiftSubscriptionTime(h.ctx(), h.adminID, uuid.Must(uuid.NewV7()),
		billingapp.TimeShiftRequest{Shift: time.Hour}); !errors.Is(err, billingapp.ErrSubscriptionNotFound) {
		t.Errorf("unknown user: error = %v, want ErrSubscriptionNotFound", err)
	}

	// The service-level railguard: a rig-less service refuses the operation
	// before anything else.
	rigLess := h.newSubscriptionService(billingapp.SubscriptionServiceConfig{
		Clock: h.clock, Config: billingapp.DefaultConfig(),
	})
	if err := rigLess.ShiftSubscriptionTime(h.ctx(), h.adminID, userID,
		billingapp.TimeShiftRequest{Shift: -time.Hour}); !errors.Is(err, billingapp.ErrTimeTravelDisabled) {
		t.Errorf("rig-less service: error = %v, want ErrTimeTravelDisabled", err)
	}
}
