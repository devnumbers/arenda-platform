// Package domain holds the Tasks context core: the task rule, its
// occurrences, the pure materialization plan, and the computed read-side
// views (ADR 0051). Everything here is pure computation — no clocks, no
// I/O; "today" and the read-side moment always arrive as parameters. Dates
// are calendar dates: time.Time values at UTC midnight, normalized on
// construction and compared only with other calendar dates; the time of day
// is minutes since midnight at HH:MM precision (#496).
package domain

import (
	"time"

	"github.com/google/uuid"
)

// RepeatKind is how often a rule produces tasks: once, or on a calendar
// cadence. The parameterless enum mirrors the TEXT-enum of migration 000118:
// recurrence parameters (weekday sets, day-of-month lists) were rejected for
// v1 (resolution #496).
type RepeatKind string

// The five repeat kinds; the human names live in the API contract.
const (
	RepeatOnce    RepeatKind = "once"
	RepeatDaily   RepeatKind = "daily"
	RepeatWeekly  RepeatKind = "weekly"
	RepeatMonthly RepeatKind = "monthly"
	RepeatYearly  RepeatKind = "yearly"
)

// IsValidRepeat reports whether the value is one of the five repeat kinds.
func IsValidRepeat(r RepeatKind) bool {
	switch r {
	case RepeatOnce, RepeatDaily, RepeatWeekly, RepeatMonthly, RepeatYearly:
		return true
	default:
		return false
	}
}

// TaskRule is the rule: a named setting that produces tasks — title
// (required), optional comment, an optional due date and due time (the time
// requires the date), and a repeat. A once rule produces a single task; an
// undated rule (no due date) may only be a once rule — the schema
// constraint task_rules_repeat_needs_anchor pins that, the application
// validates it before insert. The rule itself is never completed, overdue or
// undated — those are states of its tasks (tasks/CONTEXT.md «Правило»).
type TaskRule struct {
	ID      uuid.UUID
	OwnerID uuid.UUID
	// PropertyID is the rule's property binding; nil = the rule without a
	// property («Задача без объекта», ADR 0052): it lives in the owner's own
	// book, owner-only, and its «today» is the owner's timezone. A bound rule
	// never becomes property-less: deleting the property deletes its rules
	// and tasks outright (the cascade stays).
	PropertyID *uuid.UUID
	Title      string
	Comment    *string
	// DueDate is the rule's anchor: the first occurrence date and the
	// generation lower bound. Undated (nil) only with RepeatOnce. The
	// application enforces due_date ≥ today at create and edit (resolution
	// #496 — backdated rules are rejected, «прошлое не его задача»).
	DueDate *time.Time
	// DueTime is minutes since midnight at minute precision; nil = the date
	// only. Requires DueDate.
	DueTime *TimeOfDay
	Repeat  RepeatKind
	// CreatedAt/UpdatedAt are the rule row's timestamps (updated_at is
	// trigger-maintained on write); the tick never reads them.
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Undated reports whether the rule produces a dateless task («Без срока»).
func (r TaskRule) Undated() bool { return r.DueDate == nil }
