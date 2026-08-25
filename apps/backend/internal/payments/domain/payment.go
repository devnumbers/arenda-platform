// Package domain holds the Payments context core: the payment rule, its
// occurrences, pause intervals, and the pure tick planning function ported
// from the validated prototype (ADR 0047, ADR 0049 §3). Everything here is
// pure computation — no clocks, no I/O; "today" always arrives as a
// parameter. Dates are calendar dates: time.Time values at UTC midnight,
// normalized on construction and compared only with other calendar dates.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// PaymentType is the direction of a payment and its operations: money into
// the property or out of it.
type PaymentType string

// The two payment directions (ADR 0047).
const (
	TypeIncome  PaymentType = "income"
	TypeExpense PaymentType = "expense"
)

// PaymentForm is how the payment actually passes: bank transfer or cash. Not
// "payment method" — that canonical term belongs to the Billing context
// (ADR 0047).
type PaymentForm string

// The two payment forms.
const (
	FormTransfer PaymentForm = "transfer"
	FormCash     PaymentForm = "cash"
)

// OperationStatus is the stored status of an operation. "Overdue" is not a
// status: it is computed (planned with a date before today) and never stored
// (prototype decision №2).
type OperationStatus string

// The two stored statuses; closure is payment only (№16).
const (
	StatusPlanned OperationStatus = "planned"
	StatusPaid    OperationStatus = "paid"
)

// OperationOrigin distinguishes an occurrence materialized from a payment
// rule from a manually entered one-off fact (ticket #446).
type OperationOrigin string

// The two operation origins.
const (
	OriginPayment OperationOrigin = "payment"
	OriginManual  OperationOrigin = "manual"
)

// fallbackCategoryLabel snapshots a category reference that no longer
// resolves: a slug removed from the default catalog, or a dangling user
// category name (ADR 0049 §1).
const fallbackCategoryLabel = "Прочее"

// CategoryRef is the payment's category reference: exactly one of a default
// catalog slug or a user category id (the XOR is a durable schema invariant).
// UserCategoryName carries the user category's current name, resolved by the
// persistence adapter when the rule is loaded — the operation snapshot takes
// it at materialization time.
type CategoryRef struct {
	Slug             *string
	UserCategoryID   *uuid.UUID
	UserCategoryName *string
}

// SnapshotLabel returns the category label a materialized operation freezes:
// the user category's name, the default catalog label for a known slug, or
// the "Прочее" fallback when the reference no longer resolves (ADR 0049 §1).
func (r CategoryRef) SnapshotLabel() string {
	if r.UserCategoryName != nil && *r.UserCategoryName != "" {
		return *r.UserCategoryName
	}
	if r.Slug != nil {
		if entry, ok := CategoryBySlug(*r.Slug); ok {
			return entry.Label
		}
	}
	return fallbackCategoryLabel
}

// SlugString returns the default-catalog slug, or "" when the reference is a
// user category. The CRUD validation reads it; user-category references
// arrive with the categories slice (#447 follow-up).
func (r CategoryRef) SlugString() string {
	if r.Slug != nil {
		return *r.Slug
	}
	return ""
}

// Payment is the rule: what, how much, when and how often must happen on a
// property — income or expense (ADR 0047). It is never itself paid, overdue
// or cancelled; those are states of its operations. Pauses travel with the
// rule as loaded history intervals (ADR 0002 of the prototype).
type Payment struct {
	ID            uuid.UUID
	OwnerID       uuid.UUID
	PropertyID    uuid.UUID
	Type          PaymentType
	Title         string
	AmountKopecks int64
	Recurrence    Recurrence
	// Since is the date the rule was created on: the lower bound of
	// generation. Set by the server, never edited — no backdated occurrences
	// (prototype decision №17).
	Since time.Time
	// EndDate optionally stops generation; nil means open-ended.
	EndDate *time.Time
	AutoPay bool
	// PaymentForm is how the payment passes; operations snapshot it.
	PaymentForm PaymentForm
	Category    CategoryRef
	Pauses      []PauseInterval
	// CreatedAt/UpdatedAt are the rule row's timestamps (updated_at is
	// trigger-maintained on write); they exist for the CRUD read side — the
	// tick never reads them.
	CreatedAt time.Time
	UpdatedAt time.Time
}
