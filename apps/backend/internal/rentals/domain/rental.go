// Package domain holds the Rentals context core (ADR 0053): the rental — a
// period of a property's occupancy with its terms — and the pure read-side
// functions the server computes for the TZ-blind client: the statuses, the
// «N из M месяцев» progress math and the payment day. Everything here is pure
// computation — no clocks, no I/O; "today" always arrives as a parameter.
// Dates are calendar dates: time.Time values at UTC midnight, normalized on
// construction and compared only with other calendar dates.
package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Utilities is the required utilities mode of a rental (CONTEXT.md
// «Коммунальные платежи»): in v1 a record on the rental, nothing more.
type Utilities string

// The three utilities modes.
const (
	UtilitiesIncluded    Utilities = "included"
	UtilitiesMetersOnly  Utilities = "meters_only"
	UtilitiesFullReceipt Utilities = "full_receipt"
)

// Valid reports whether the mode is one of the three contract values.
func (u Utilities) Valid() bool {
	switch u {
	case UtilitiesIncluded, UtilitiesMetersOnly, UtilitiesFullReceipt:
		return true
	}
	return false
}

// Rental is the period of a property's occupancy with its terms (CONTEXT.md
// «Аренда»). The stored status is the completion fact only (решение №1);
// the other three are computed by StatusOf. The payment day lives in the
// managed rent payment's recurrence (решение №5) — the rental carries no
// copy; the rent payment's id is the 1:1 link (NOT NULL, UNIQUE).
type Rental struct {
	ID         uuid.UUID
	OwnerID    uuid.UUID
	PropertyID uuid.UUID
	PaymentID  uuid.UUID
	// ContactID is the optional tenant reference; after the contact's
	// deletion the link is NULL and the tenant is simply absent (решение
	// #528).
	ContactID *uuid.UUID
	// StartDate is today or later at creation — no backdated rentals.
	StartDate time.Time
	// PlannedEndDate is the optional term: nil is an open-ended rental.
	PlannedEndDate *time.Time
	// CompletedDate is the stored completion fact; nil while the rental runs.
	CompletedDate *time.Time
	Utilities     Utilities
	// DepositKopecks and CommissionKopecks are optional records; neither
	// spawns payments or operations (решение №6).
	DepositKopecks    *int64
	CommissionKopecks *int64
	// DepositReturnKopecks/DepositReturnComment are written by the completion
	// only (the schema CHECKs keep that); a zero sum is valid («не вернул»).
	DepositReturnKopecks *int64
	DepositReturnComment *string
	// Comment is the free-text terms note, at most 2000 characters.
	Comment string
	// Tenant is the embedded tenant view, resolved by the persistence
	// adapter when the row is loaded (the same pattern as the payments
	// category name); nil — «Контакта нет», including after the contact's
	// deletion (решение #528). Never written back.
	Tenant *TenantContact
	// CreatedAt/UpdatedAt are the row's timestamps (updated_at is
	// trigger-maintained on write).
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TenantContact is the embedded tenant view (ADR 0053 §4): the referenced
// contact card's display fields. The contact belongs to the scope owner's
// book; after the contact's deletion the rental simply has no tenant.
type TenantContact struct {
	ContactID uuid.UUID
	FirstName string
	LastName  string
	Phone     string
}

// Status is the computed read-side status of a rental (CONTEXT.md «Статусы
// Аренды»): only the completion fact is stored, the rest derives from the
// dates against the owner's today.
type Status string

// The four statuses.
const (
	// StatusUpcoming — the start has not come yet.
	StatusUpcoming Status = "upcoming"
	// StatusActive — the rental runs.
	StatusActive Status = "active"
	// StatusNeedsAttention — the planned end has already passed and the
	// rental is not completed. An open-ended rental never needs attention.
	StatusNeedsAttention Status = "needs_attention"
	// StatusCompleted — the completion fact is stored.
	StatusCompleted Status = "completed"
)

// StatusOf computes the rental's status against the owner's today (ADR 0048):
// the completion date wins; otherwise upcoming while the start has not come,
// then needs_attention once the planned end has passed (in the planned-end day
// itself the rental is still active), else active.
func StatusOf(r Rental, today time.Time) Status {
	if r.CompletedDate != nil {
		return StatusCompleted
	}
	if r.StartDate.After(today) {
		return StatusUpcoming
	}
	if r.PlannedEndDate != nil && r.PlannedEndDate.Before(today) {
		return StatusNeedsAttention
	}
	return StatusActive
}

// PaymentDay is the day of month the rent falls due (CONTEXT.md «День
// оплаты»): 1..30 as their own day, 31 and «последний день месяца» as the one
// last-day behaviour (решение №5) — in a shorter month it clamps to the
// month's actual last day. The constructors are the only way to build one, so
// an out-of-range day is unrepresentable.
type PaymentDay struct {
	day  int // 1..30, 0 when last.
	last bool
}

// NewPaymentDay builds the day from its 1..31 contract value; 31 normalizes
// to the last-day marker — the two spellings are one behaviour.
func NewPaymentDay(day int) (PaymentDay, error) {
	if day < 1 || day > 31 {
		return PaymentDay{}, fmt.Errorf("payment day must be 1..31, got %d", day)
	}
	if day == 31 {
		return PaymentDay{last: true}, nil
	}
	return PaymentDay{day: day}, nil
}

// NewLastPaymentDay builds the «последний день месяца» marker directly.
func NewLastPaymentDay() PaymentDay {
	return PaymentDay{last: true}
}

// Day returns the contract day value, 0 for the last-day marker.
func (d PaymentDay) Day() int { return d.day }

// IsLast reports the last-day marker.
func (d PaymentDay) IsLast() bool { return d.last }

// MustPaymentDay is NewPaymentDay for literals the tests and fixtures
// control; a bad value panics.
func MustPaymentDay(day int) PaymentDay {
	d, err := NewPaymentDay(day)
	if err != nil {
		panic(err)
	}
	return d
}

// CountPaymentDays counts the payment-day occurrences in the half-open
// window [start, end): each month's day clamped to the month's length (the
// same clamp semantics the payments monthly recurrence materializes with).
// The occurrence exactly on `end` — the planned end — does not count: the
// month it opens lies beyond the rental, so a 12-month term is 12 payments
// (the #802 F2 half-open window fix, ticket #816). It is the progress'
// totalMonths for a term rental — «N из M месяцев» (ADR 0053 §2). An end on
// or before start counts nothing.
func CountPaymentDays(start, end time.Time, day PaymentDay) int {
	if !end.After(start) {
		return 0
	}
	count := 0
	for year, month := start.Year(), start.Month(); ; {
		d := dayInMonth(year, month, day)
		if !d.Before(end) {
			break
		}
		if !d.Before(start) {
			count++
		}
		if month == time.December {
			year, month = year+1, time.January
		} else {
			month++
		}
	}
	return count
}

// dayInMonth rebuilds the month's occurrence date for the payment day: the
// selected day clamped to the month's length, or the month's actual last day
// for the marker. Month arithmetic by number, not AddDate — the same
// no-drift rule the payments recurrence follows.
func dayInMonth(year int, month time.Month, day PaymentDay) time.Time {
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	d := day.day
	if day.last || d > last {
		d = last
	}
	return time.Date(year, month, d, 0, 0, 0, 0, time.UTC)
}

// FullMonthsBetween counts the full calendar months from `from` to `to`: the
// month indices differ by one less when `to`'s day has not reached `from`'s
// day yet. It is the progress' monthsRemaining (ADR 0053 §2); a `to` before
// or on `from` counts nothing.
func FullMonthsBetween(from, to time.Time) int {
	if !to.After(from) {
		return 0
	}
	months := (to.Year()-from.Year())*12 + int(to.Month()) - int(from.Month())
	if to.Day() < from.Day() {
		months--
	}
	if months < 0 {
		return 0
	}
	return months
}
