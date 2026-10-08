package domain

import (
	"time"

	"github.com/google/uuid"
)

// Operation is one occurrence of a payment rule at a concrete date, or a
// one-off fact entered manually. It is the unit that gets paid; closure is by
// payment only, and only an operation can be overdue (computed, never stored).
type Operation struct {
	ID            uuid.UUID
	OwnerID       uuid.UUID
	PropertyID    uuid.UUID
	PaymentID     *uuid.UUID // nil: manual origin, or the payment was deleted
	Origin        OperationOrigin
	Date          time.Time  // Planned occurrence date.
	PaidDate      *time.Time // Actual payment date; nil while planned.
	Status        OperationStatus
	Type          PaymentType
	Title         string
	AmountKopecks int64
	CategoryLabel string  // Snapshot.
	CategorySlug  *string // Snapshot for default-category rendering.
	// UpdatedAt is the row's last touch. A paid operation is never edited
	// afterwards (cancellation leaves every read), so on the wire it is the
	// pay's moment — the payment history interleaves same-day facts with the
	// edit journal by it (ticket #1195). Zero on synthesized in-memory
	// responses; the application layer stamps those with the action's time.
	UpdatedAt time.Time
}

// NewMaterializedOperation builds the planned occurrence of the payment rule
// at date. The rule's title, amount, type and category are snapshotted here
// and never follow later edits of the rule (ADR 0049 §1).
// Auto-pay does not pre-pay at insert: the tick closes today's occurrence
// separately, strictly on its day (ADR 0049 §2).
func NewMaterializedOperation(p Payment, date time.Time) Operation {
	paymentID := p.ID
	return Operation{
		OwnerID:       p.OwnerID,
		PropertyID:    p.PropertyID,
		PaymentID:     &paymentID,
		Origin:        OriginPayment,
		Date:          date,
		Status:        StatusPlanned,
		Type:          p.Type,
		Title:         p.Title,
		AmountKopecks: p.AmountKopecks,
		CategoryLabel: p.Category.SnapshotLabel(),
		CategorySlug:  p.Category.Slug,
	}
}

// PaymentTickPlan is what the tick must execute for one payment rule at a
// given today — a pure computation over rules, pauses, today and the existing
// operation keys (ADR 0049 §2–§3). Applying it is idempotent.
type PaymentTickPlan struct {
	// Materialize lists due occurrence dates (<= today) that have no
	// operation yet and whose recurrence period no operation of the rule
	// claims yet; each becomes a planned operation. Overdue debt
	// accumulates here and is never auto-closed.
	Materialize []time.Time
	// AutoPayToday runs the auto-pay day payment: the planned occurrence with
	// date = today becomes paid with paid_date = today. Strictly today — no
	// backdated catch-up, ever (ADR 0049 §2, revision of prototype №4–№5);
	// an active pause stops it.
	AutoPayToday bool
	// KeepFuture is the only future (date > today) planned occurrence allowed
	// to remain; every other future planned operation of the rule is removed
	// (stale rows left by rule edits, or the whole future when the rule is
	// paused or ended). Nil means no future planned may remain.
	KeepFuture *time.Time
	// InsertFuture is the missing future planned occurrence to insert. When
	// set, it is always equal to KeepFuture.
	InsertFuture *time.Time
}

// PlanPaymentTick computes the tick plan for one payment rule. The existing map holds
// the rule's stored operations keyed by date. Paid operations are never
// touched; overdue planned operations (date < today) are left as debt. A
// period of the recurrence's own calendar (the month for monthly rules, the
// week for weekly, the year for yearly) that already holds one of the rule's
// operations is never re-materialized on a re-dated schedule — the #802 F1
// phantom fix (ticket #815).
func PlanPaymentTick(p Payment, today time.Time, existing map[time.Time]OperationStatus) PaymentTickPlan {
	byDate := existing
	var plan PaymentTickPlan

	// 1. Materialize everything due: occurrences <= today (>= since, <=
	// end_date, outside pauses) that do not exist yet. A period already
	// holding an operation of the rule is claimed: after a payment-day edit
	// the old occurrences keep standing at the old dates, and exact-date
	// dedup alone would resurrect every paid month as phantom overdue debt
	// (ticket #815).
	claimedPeriods := make(map[time.Time]struct{}, len(byDate))
	for date := range byDate {
		claimedPeriods[periodOf(p.Recurrence, date)] = struct{}{}
	}
	todayOccurs := false
	for _, d := range OccurrencesBetween(p, p.Since, today) {
		if d.Equal(today) {
			todayOccurs = true
		}
		if _, ok := byDate[d]; !ok {
			if _, taken := claimedPeriods[periodOf(p.Recurrence, d)]; !taken {
				plan.Materialize = append(plan.Materialize, d)
			}
		}
	}

	// 2. Auto-pay day payment: only on the occurrence's own day, only without
	// an active pause. Occurrences inside a pause are not generated, so the
	// day payment naturally has nothing to close then.
	if _, paused := ActivePause(p.Pauses); !paused && p.AutoPay && todayOccurs {
		plan.AutoPayToday = true
	}

	// 3. Rebuild the single future planned: walk occurrences after today,
	// skipping occurrences already closed ahead ("Оплатить сейчас" does not
	// shift the schedule — the following occurrence becomes the next planned).
	next, found, exists := nextFuturePlanned(p, today, byDate)
	if found {
		plan.KeepFuture = &next
		if !exists {
			plan.InsertFuture = &next
		}
	}
	return plan
}

// nextFuturePlanned finds where the rule's only future planned operation must
// hang: the first occurrence after today without an operation (to insert), or
// the first one whose operation is still planned (already standing). Closed
// occurrences are skipped; found=false means no future planned may remain.
func nextFuturePlanned(
	p Payment, today time.Time, byDate map[time.Time]OperationStatus,
) (next time.Time, found, exists bool) {
	horizon := nextDay(addYearsClamped(today, 5))
	for _, d := range OccurrencesBetween(p, today, horizon) {
		if !d.After(today) {
			continue
		}
		status, ok := byDate[d]
		if !ok {
			return d, true, false
		}
		if status == StatusPlanned {
			return d, true, true
		}
		// Closed (paid) ahead: the occurrence already happened — look at the next.
	}
	return time.Time{}, false, false
}
