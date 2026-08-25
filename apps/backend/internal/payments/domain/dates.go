package domain

import "time"

// Calendar-date arithmetic ported from the prototype's dates.ts: every value
// is a UTC-midnight time.Time standing for a pure calendar date, so the math
// has no timezone surprises (ADR 0048 p.2). The time.Date normalization of
// out-of-range components is the mechanism behind both month rollover and
// day clamping.

// nextDay shifts a calendar date one day forward.
func nextDay(d time.Time) time.Time {
	return time.Date(d.Year(), d.Month(), d.Day()+1, 0, 0, 0, 0, time.UTC)
}

// daysInMonth returns the number of days in the given month.
func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// dateInMonth returns the date in the given month (which may overflow past
// December — time.Date normalizes it) with the day clamped to the month's
// last day. This is the recurrence anchor evaluation: day 31 in a 30-day
// month lands on the 30th, February 29 on February 28 (prototype decisions
// №1/№10 — the anchor never drifts, every month is rebuilt from the anchor).
func dateInMonth(year int, month time.Month, day int) time.Time {
	norm := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	day = min(day, daysInMonth(norm.Year(), norm.Month()))
	return time.Date(norm.Year(), norm.Month(), day, 0, 0, 0, 0, time.UTC)
}

// addYearsClamped shifts a calendar date by n years, clamping February 29 to
// the last day of February.
func addYearsClamped(d time.Time, n int) time.Time {
	day := min(d.Day(), daysInMonth(d.Year()+n, d.Month()))
	return time.Date(d.Year()+n, d.Month(), day, 0, 0, 0, 0, time.UTC)
}
