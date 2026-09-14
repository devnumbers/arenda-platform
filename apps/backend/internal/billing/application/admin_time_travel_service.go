package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// The stand-only time-travel rig of issue #665: the admin operation that
// moves a subscription's temporal boundaries so the acceptance can walk the
// full subscription lifecycle without waiting out real hours. The decisions
// are the grilling of issue #648, confirmed by the owner on 14.09.2026:
// data-side time travel (the boundaries move, the worker phases stay
// untouched — they already catch up with anything overdue by clock.Now), an
// admin-triggered tick beside it, and preset wrappers over the same shift.
// The railguards live in the platform config (BILLING_TIME_TRAVEL fails the
// startup outside the local/dev/stage stands) and at the service level
// (TimeTravelEnabled).

// ErrTimeTravelDisabled is returned when the time-shift operation is called
// on a service wired without the rig — the second railguard layer behind the
// route mounting.
var ErrTimeTravelDisabled = errors.New("billing time travel is disabled")

// TimeShiftPreset names a thin acceptance wrapper over the same signed shift
// (issue #665): the server resolves each preset against the subscription's
// live boundaries, so the operator lands in the target window exactly instead
// of stacking blind offsets. Every preset moves the boundary backward toward
// now — data-side time travel cannot move the clock itself.
type TimeShiftPreset string

const (
	// TimeShiftPresetPeriodExpired lands valid_until an hour into the past.
	// From an active subscription the next tick's renewal charge fails into
	// grace («войти в grace»); from a grace window it expires the window to
	// basic («истечь из grace»). The phases tell the two scenarios apart by
	// the status they find.
	TimeShiftPresetPeriodExpired TimeShiftPreset = "period_expired"
	// TimeShiftPresetRetryFirstDue lands the dunning anchor just past the
	// first retry boundary (+24 h from the grace entry, one hour of slack),
	// keeping the window open — the next tick charges the retry.
	TimeShiftPresetRetryFirstDue TimeShiftPreset = "retry_24h_due"
	// TimeShiftPresetRetrySecondDue lands the dunning anchor past the second
	// retry boundary (+72 h from the grace entry, one hour of slack).
	TimeShiftPresetRetrySecondDue TimeShiftPreset = "retry_72h_due"
	// TimeShiftPresetReminderWindow lands the grace deadline a day out —
	// comfortably inside the reminder window
	// [grace_end − GraceExpiryReminderBefore, grace_end) — so the next tick
	// dispatches the grace-expiry reminder.
	TimeShiftPresetReminderWindow TimeShiftPreset = "reminder_window"
)

// preset validates a wire-supplied preset name.
func (p TimeShiftPreset) valid() bool {
	switch p {
	case TimeShiftPresetPeriodExpired, TimeShiftPresetRetryFirstDue,
		TimeShiftPresetRetrySecondDue, TimeShiftPresetReminderWindow:
		return true
	}
	return false
}

// The preset anchors, relative to the shift moment: the dunning boundaries
// mirror the +24 h/+72 h schedule (graceRetryFirstAfter/SecondAfter) with an
// hour of slack so the boundary is comfortably due when the tick runs; the
// expired boundary sits an hour into the past; the reminder target keeps a
// full day of window ahead.
const (
	timeShiftRetryFirstAnchor  = -25 * time.Hour
	timeShiftRetrySecondAnchor = -73 * time.Hour
	timeShiftExpiredTarget     = -1 * time.Hour
	timeShiftReminderTarget    = 24 * time.Hour
)

// TimeShiftRequest is the application-layer input of the admin time shift
// (issue #665): either the raw signed shift or one named preset — exactly
// one of the two.
type TimeShiftRequest struct {
	// Shift is the raw signed shift, bounded by Config.MaxTimeShift.
	Shift time.Duration
	// Preset names a preset wrapper; it wins over an unset Shift.
	Preset TimeShiftPreset
}

// ShiftSubscriptionTime moves the user's subscription in time by the signed
// delta (issue #665), in one transaction attributed to the acting admin: the
// aggregate's temporal boundaries (domain ShiftTime), the dunning anchor —
// the latest grace-entry transitions the retry schedule reads — and the
// payments' created_at all travel by the same delta, so every worker-phase
// predicate keeps its relative order and the phases themselves stay
// untouched. The shift appends its time_shifted transition and the audit
// record; the next tick does the rest.
func (s *SubscriptionService) ShiftSubscriptionTime(
	ctx context.Context, adminID, userID uuid.UUID, req TimeShiftRequest,
) error {
	if !s.timeTravelEnabled {
		return ErrTimeTravelDisabled
	}
	if err := validateTimeShiftRequest(req); err != nil {
		return err
	}
	return s.runInTx(ctx, func(stores *txStores) error {
		sub, err := stores.subscriptionForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		delta, err := s.resolveTimeShift(ctx, stores, sub, req)
		if err != nil {
			return err
		}
		if delta > s.config.MaxTimeShift || delta < -s.config.MaxTimeShift {
			return fmt.Errorf("%w: the %s shift exceeds the ±%s cap",
				domain.ErrInvalidTimeShift, delta, s.config.MaxTimeShift)
		}
		preset := req.Preset
		if _, err := stores.applyTransition(ctx, &sub,
			func(sc *domain.Subscription) error { return sc.ShiftTime(delta) },
			transitionSpec{
				reason:      domain.TransitionReasonTimeShifted,
				initiator:   domain.InitiatorAdmin,
				initiatorID: &adminID,
				auditAction: auditdomain.ActionSubscriptionTimeShifted,
				auditContext: func(sc domain.Subscription, _ domain.Transition) map[string]any {
					entry := map[string]any{
						auditKeyShiftHours: delta.Hours(),
						auditKeyValidUntil: *sc.ValidUntil,
					}
					if preset != "" {
						entry[auditKeyPreset] = string(preset)
					}
					return entry
				},
			},
		); err != nil {
			return err
		}
		if _, err := stores.transitions.ShiftGraceEntryTimes(ctx, sub.ID, delta); err != nil {
			return fmt.Errorf("shift the grace-entry anchor: %w", err)
		}
		if _, err := stores.payments.ShiftCreatedAt(ctx, sub.ID, delta); err != nil {
			return fmt.Errorf("shift the payment timestamps: %w", err)
		}
		return nil
	})
}

// validateTimeShiftRequest rejects the request shapes the rig does not
// accept: a zero delta, a preset stacked on a raw shift, an unknown preset.
func validateTimeShiftRequest(req TimeShiftRequest) error {
	if req.Preset != "" && req.Shift != 0 {
		return fmt.Errorf("%w: a preset and a raw shift are mutually exclusive", domain.ErrInvalidTimeShift)
	}
	if req.Preset == "" && req.Shift == 0 {
		return fmt.Errorf("%w: set a raw shift or a preset", domain.ErrInvalidTimeShift)
	}
	if req.Preset != "" && !req.Preset.valid() {
		return fmt.Errorf("%w: unknown preset %q", domain.ErrInvalidTimeShift, req.Preset)
	}
	return nil
}

// resolveTimeShift turns the request into the signed delta. A raw shift
// passes through; a preset is resolved against the live boundaries — the
// guard errors say which acceptance step is out of order (the boundary is
// already past the target, the episode is not in grace, the shift would
// close the window the preset is meant to observe).
func (s *SubscriptionService) resolveTimeShift(
	ctx context.Context, stores *txStores, sub domain.Subscription, req TimeShiftRequest,
) (time.Duration, error) {
	now := s.clock.Now().UTC()
	if req.Preset == "" {
		return req.Shift, nil
	}
	switch req.Preset {
	case TimeShiftPresetPeriodExpired:
		if sub.ValidUntil == nil || !sub.ValidUntil.After(now.Add(timeShiftExpiredTarget)) {
			return 0, fmt.Errorf("%w: the validity boundary is already at or past the preset target",
				domain.ErrInvalidSubscriptionState)
		}
		return now.Add(timeShiftExpiredTarget).Sub(*sub.ValidUntil), nil
	case TimeShiftPresetRetryFirstDue, TimeShiftPresetRetrySecondDue:
		if sub.Status != domain.SubscriptionStatusGrace {
			return 0, fmt.Errorf("%w: the retry presets re-anchor a grace window, the subscription is %s",
				domain.ErrInvalidSubscriptionState, sub.Status)
		}
		target := timeShiftRetryFirstAnchor
		if req.Preset == TimeShiftPresetRetrySecondDue {
			target = timeShiftRetrySecondAnchor
		}
		anchor, err := latestGraceEntry(ctx, stores, sub.ID)
		if err != nil {
			return 0, err
		}
		delta := now.Add(target).Sub(anchor.CreatedAt)
		if !sub.ValidUntil.Add(delta).After(now) {
			return 0, fmt.Errorf("%w: the shift would close the grace window; run the reminder or expiry preset",
				domain.ErrInvalidSubscriptionState)
		}
		return delta, nil
	case TimeShiftPresetReminderWindow:
		if sub.Status != domain.SubscriptionStatusGrace {
			return 0, fmt.Errorf("%w: the reminder preset re-lands a grace window, the subscription is %s",
				domain.ErrInvalidSubscriptionState, sub.Status)
		}
		if sub.ValidUntil == nil || !sub.ValidUntil.After(now.Add(timeShiftReminderTarget)) {
			return 0, fmt.Errorf("%w: the grace window is already inside or past the reminder window",
				domain.ErrInvalidSubscriptionState)
		}
		return now.Add(timeShiftReminderTarget).Sub(*sub.ValidUntil), nil
	default:
		return 0, fmt.Errorf("%w: unknown preset %q", domain.ErrInvalidTimeShift, req.Preset)
	}
}

// latestGraceEntry reads the dunning anchor — the newest grace-entry
// transition of the subscription, the row the retry schedule anchors at.
func latestGraceEntry(ctx context.Context, stores *txStores, subscriptionID uuid.UUID) (domain.Transition, error) {
	transitions, err := stores.transitions.ListBySubscriptionID(ctx, subscriptionID)
	if err != nil {
		return domain.Transition{}, fmt.Errorf("list transitions for the dunning anchor: %w", err)
	}
	// The listing is newest-first, so the first match is the latest entry.
	for _, transition := range transitions {
		if transition.Reason == domain.TransitionReasonGraceEntered {
			return transition, nil
		}
	}
	return domain.Transition{}, fmt.Errorf("%w: the subscription has no grace entry to re-anchor",
		domain.ErrInvalidSubscriptionState)
}
