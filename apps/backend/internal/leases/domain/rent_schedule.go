package domain

import "time"

// GenerateDates returns the planned operation dates for a recurring rent
// schedule. The first date is always the lease start date. Subsequent dates
// fall on paymentDay of each month or year (depending on periodicity), clamped
// to the last day of the month when necessary. Generation stops at 12 months
// from now or at endDate, whichever comes first.
func GenerateDates(start time.Time, paymentDay int, endDate *time.Time, now time.Time, periodicity RecurringOperationPeriodicity) []time.Time {
	var dates []time.Time

	windowEnd := date(now).AddDate(0, 12, 0)
	current := date(start)
	first := true

	for {
		if !first {
			switch periodicity {
			case RecurringOperationPeriodicityYearly:
				current = nextPaymentDateYearly(current, paymentDay)
			default:
				current = nextPaymentDate(current, paymentDay)
			}
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

	day := min(paymentDay, lastDay)

	return time.Date(year, month+1, day, 0, 0, 0, 0, time.UTC)
}

func nextPaymentDateYearly(current time.Time, paymentDay int) time.Time {
	year, month, _ := current.Date()
	year++
	lastDay := lastDayOfMonth(year, month)

	day := min(paymentDay, lastDay)

	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func lastDayOfMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// CurrentPeriodDueDate returns the planned payment date of the current period:
// the last paymentDay (clamped to the month's length) on or before asOf. It
// reports ok=false when the lease has not started yet (startDate is after asOf).
func CurrentPeriodDueDate(startDate time.Time, paymentDay int, asOf time.Time) (due time.Time, ok bool) {
	start := date(startDate)
	asOfDate := date(asOf)
	if start.After(asOfDate) {
		return time.Time{}, false
	}

	year, month, _ := asOfDate.Date()
	candidate := clampedPaymentDate(year, month, paymentDay)
	if !candidate.After(asOfDate) {
		return candidate, true
	}

	prev := asOfDate.AddDate(0, -1, 0)
	return clampedPaymentDate(prev.Year(), prev.Month(), paymentDay), true
}

func clampedPaymentDate(year int, month time.Month, paymentDay int) time.Time {
	day := min(paymentDay, lastDayOfMonth(year, month))
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
