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
