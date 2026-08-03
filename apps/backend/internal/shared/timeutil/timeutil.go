package timeutil

import "time"

// DateIn returns the midnight timestamp at the start of the day containing t,
// interpreted in the given location.
func DateIn(t time.Time, loc *time.Location) time.Time {
	return time.Date(t.In(loc).Year(), t.In(loc).Month(), t.In(loc).Day(), 0, 0, 0, 0, loc)
}

// Date returns the UTC midnight date for the given time.
func Date(t time.Time) time.Time {
	return DateIn(t, time.UTC)
}

// BeforeDay reports whether the calendar day of a is strictly before the
// calendar day of b. Each time is interpreted in its own location, so a
// timezone-normalized value (e.g. midnight in the owner's timezone, whose UTC
// representation differs) is compared by its real calendar date rather than by
// its UTC representation. This avoids the off-by-one day shift that
// Date(a).Before(Date(b)) introduces for non-UTC timezones.
//
// Both operands must represent calendar dates that are semantically comparable:
// either both in the same owner timezone, or one a pure DATE (UTC midnight from
// a database DATE column) and the other normalized via DateIn in the owner's
// timezone. Passing two instants in different timezones produces a meaningless
// result.
func BeforeDay(a, b time.Time) bool {
	if a.Year() != b.Year() {
		return a.Year() < b.Year()
	}
	if a.Month() != b.Month() {
		return a.Month() < b.Month()
	}
	return a.Day() < b.Day()
}
