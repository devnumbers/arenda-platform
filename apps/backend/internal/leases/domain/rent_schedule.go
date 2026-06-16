package domain

import "time"

// GenerateDates returns the planned operation dates for a recurring rent
// schedule. The first date is always the lease start date. Subsequent dates
// fall on paymentDay of each month, clamped to the last day of the month when
// necessary. Generation stops at 12 months from now or at endDate, whichever
// comes first.
func GenerateDates(start time.Time, paymentDay int, endDate *time.Time, now time.Time) []time.Time {
	var dates []time.Time

	windowEnd := date(now).AddDate(0, 12, 0)
	current := date(start)
	first := true

	for {
		if !first {
			current = nextPaymentDate(current, paymentDay)
		}
		first = false

		if endDate != nil && current.After(date(*endDate)) {
			break
		}
		if current.After(windowEnd) {
			break
		}

		dates = append(dates, current)

		// Guard against an unbounded loop if the schedule is misconfigured.
		if len(dates) > 36 {
			break
		}
	}

	return dates
}

func nextPaymentDate(current time.Time, paymentDay int) time.Time {
	year, month, _ := current.Date()
	lastDay := lastDayOfMonth(year, month+1)

	day := paymentDay
	if day > lastDay {
		day = lastDay
	}

	return time.Date(year, month+1, day, 0, 0, 0, 0, time.UTC)
}

func lastDayOfMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// date normalizes a timestamp to a UTC date with no time component.
func date(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
