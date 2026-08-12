package domain

import (
	"time"
)

const (
	// LoginAttemptWindowTTL is the duration during which login failures are counted.
	LoginAttemptWindowTTL = 30 * time.Minute
	// MaxLoginFailures is the number of failures that trigger a temporary block.
	MaxLoginFailures = 15
)

// AttemptWindow tracks failed login attempts within a sliding window.
type AttemptWindow struct {
	Failures       int
	FirstFailureAt time.Time
	LastFailureAt  time.Time
}

// NewAttemptWindow creates a fresh attempt window starting at now.
func NewAttemptWindow(now time.Time) AttemptWindow {
	return AttemptWindow{FirstFailureAt: now, LastFailureAt: now}
}

// RecordFailure increments the failure count and returns ErrTooManyAttempts
// when the threshold is reached. It resets the window when it has expired.
//
// Invariant: FirstFailureAt is modified only when the window is created or
// restarted after its TTL — never on a plain increment within the live window.
// Callers (notably application.attemptDelta) rely on this to distinguish a
// reset (write the absolute counter) from an increment (atomic delta). Do not
// touch FirstFailureAt on the increment path without updating that logic.
func (w *AttemptWindow) RecordFailure(now time.Time) error {
	if w.Failures == 0 || now.Sub(w.FirstFailureAt) >= LoginAttemptWindowTTL {
		w.FirstFailureAt = now
		w.Failures = 0
	}
	w.Failures++
	w.LastFailureAt = now
	if w.Failures >= MaxLoginFailures {
		return ErrTooManyAttempts
	}
	return nil
}

// Blocked reports whether attempts are currently blocked.
func (w *AttemptWindow) Blocked(now time.Time) bool {
	if w.Failures < MaxLoginFailures {
		return false
	}
	return now.Before(w.FirstFailureAt.Add(LoginAttemptWindowTTL))
}
