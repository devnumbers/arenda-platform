package domain

import "time"

// PauseOrigin is where one pause interval came from (тикет #1342): the
// owner's manual pause or the system pause the property archive places on
// every live rule. The archive bridge closes only its own auto_archive
// pauses; the manual ones outlive the archive round-trip. The same
// dictionary marks the archive bridge's rows in the change log.
type PauseOrigin string

const (
	// PauseOriginManual is the owner's pause through the rule mutations.
	PauseOriginManual PauseOrigin = "manual"
	// PauseOriginAutoArchive is the system pause placed when the property
	// was archived and removed when it returns.
	PauseOriginAutoArchive PauseOrigin = "auto_archive"
)

// PauseInterval is one pause of a payment rule (prototype ADR 0002): the
// half-open interval [From, To) — From inclusive, To (the resume day) already
// outside the pause. To == nil is the active open-ended pause. Occurrences
// dated inside a pause interval are never generated at all: no operations, no
// debt, no auto-pay. The holes stay forever; resuming does not shift anchors.
// Origin carries the interval's provenance (PauseOrigin); rows stored before
// тикет #1342 read as PauseOriginManual.
type PauseInterval struct {
	From   time.Time
	To     *time.Time
	Origin PauseOrigin
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
