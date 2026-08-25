package domain

import "time"

// PauseInterval is one pause of a payment rule (prototype ADR 0002): the
// half-open interval [From, To) — From inclusive, To (the resume day) already
// outside the pause. To == nil is the active open-ended pause. Occurrences
// dated inside a pause interval are never generated at all: no operations, no
// debt, no auto-pay. The holes stay forever; resuming does not shift anchors.
type PauseInterval struct {
	From time.Time
	To   *time.Time
}

// IsDatePaused reports whether the date falls inside one of the pause
// intervals [from, to) — from inclusive, to exclusive.
func IsDatePaused(pauses []PauseInterval, d time.Time) bool {
	for _, p := range pauses {
		if !d.Before(p.From) && (p.To == nil || d.Before(*p.To)) {
			return true
		}
	}
	return false
}

// ActivePause returns the rule's open-ended pause interval, if one exists.
// At most one can exist — a durable partial-unique schema invariant.
func ActivePause(pauses []PauseInterval) (PauseInterval, bool) {
	for _, p := range pauses {
		if p.To == nil {
			return p, true
		}
	}
	return PauseInterval{}, false
}
