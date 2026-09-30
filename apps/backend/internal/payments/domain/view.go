package domain

import "time"

// OperationViewStatus is the computed view status of an operation: the two
// stored statuses plus "overdue". Overdue is not a stored status anywhere —
// it is derived (CONTEXT.md, «Просрочка») and can never be written back.
type OperationViewStatus string

// The three view statuses: planned and paid mirror the stored statuses;
// overdue is planned with the date already past.
const (
	ViewStatusPlanned OperationViewStatus = "planned"
	ViewStatusPaid    OperationViewStatus = "paid"
	ViewStatusOverdue OperationViewStatus = "overdue"
)

// OperationView computes an operation's view status against the owner's
// today (the UTC-midnight calendar date from the owner calendar, ADR 0048):
// a planned operation dated before today is overdue; a paid fact is never
// overdue no matter how its dates relate. The port of the prototype's opView.
func OperationView(op Operation, today time.Time) OperationViewStatus {
	if op.Status == StatusPlanned && op.Date.Before(today) {
		return ViewStatusOverdue
	}
	return OperationViewStatus(op.Status)
}

// ProjectionCursor is the day the schedule's «what's next» math projects
// from (ticket #991): the newest materialized date when the rule has facts,
// yesterday when nothing has been materialized — so a rule whose occurrences
// all lie ahead still projects its first one. IsCompleted and the next
// payment date resolution share the rule.
func ProjectionCursor(today time.Time, lastMaterialized *time.Time) time.Time {
	cursor := today.AddDate(0, 0, -1)
	if lastMaterialized != nil && lastMaterialized.After(cursor) {
		cursor = *lastMaterialized
	}
	return cursor
}

// IsCompleted reports whether the payment rule has no unsettled occurrences
// left: nothing planned and no occurrence beyond the last materialized date
// of any status. Like overdue it is a computed read-side view, never stored —
// the rule itself keeps no lifecycle state (payment.go). The
// lastMaterialized argument is the newest operation date across planned and
// paid (nil — nothing has been materialized yet); the projection cursor then
// falls to yesterday, so a rule whose occurrences all lie ahead is not
// completed. An open-ended rule never completes: occurrences exist beyond
// any cursor.
func IsCompleted(p Payment, today time.Time, lastMaterialized *time.Time, hasPlanned bool) bool {
	if hasPlanned {
		return false
	}
	_, ok := NextOccurrenceAfter(p, ProjectionCursor(today, lastMaterialized))
	return !ok
}
