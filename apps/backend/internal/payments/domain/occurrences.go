package domain

import (
	"slices"
	"time"
)

// maxOccurrences caps enumeration as a safety ceiling (the prototype's
// MAX_OCCURRENCES): a daily rule enumerated over an unbounded window cannot
// loop forever.
const maxOccurrences = 1000

// OccurrencesBetween enumerates the payment's occurrence dates in the
// inclusive window [start, end], as a pure function of the rule:
//
//   - daily — every day;
//   - weekly — the selected weekdays only;
//   - monthly — the anchor day of month, clamped to each month's last day;
//   - yearly — the anchor month and day, February 29 clamped;
//   - generation is lower-bounded by Since (no backdated occurrences, №17)
//     and upper-bounded by EndDate when set;
//   - dates inside pause intervals are cut out: in a pause there are no
//     occurrences at all (prototype ADR 0002).
//
// Dates are never shifted onto business days (№10). The prototype reference
// is occurrences.ts.
func OccurrencesBetween(p Payment, start, end time.Time) []time.Time {
	hardEnd := end
	if p.EndDate != nil && p.EndDate.Before(hardEnd) {
		hardEnd = *p.EndDate
	}
	if start.After(hardEnd) {
		return nil
	}

	first := start
	if p.Since.After(first) {
		first = p.Since
	}
	switch p.Recurrence.Kind() {
	case RecurrenceDaily:
		return dailyBetween(p, first, hardEnd)
	case RecurrenceWeekly:
		return weeklyBetween(p, first, hardEnd)
	case RecurrenceMonthly:
		return monthlyBetween(p, start, hardEnd)
	case RecurrenceYearly:
		return yearlyBetween(p, start, hardEnd)
	default:
		return nil
	}
}

// dailyBetween enumerates every day of the window outside pauses.
func dailyBetween(p Payment, first, hardEnd time.Time) []time.Time {
	out := make([]time.Time, 0)
	for d := first; !d.After(hardEnd) && len(out) < maxOccurrences; d = nextDay(d) {
		if !IsDatePaused(p.Pauses, d) {
			out = append(out, d)
		}
	}
	return out
}

// weeklyBetween enumerates the selected weekdays of the window outside
// pauses.
func weeklyBetween(p Payment, first, hardEnd time.Time) []time.Time {
	weekdays := p.Recurrence.Weekdays()
	out := make([]time.Time, 0)
	for d := first; !d.After(hardEnd) && len(out) < maxOccurrences; d = nextDay(d) {
		if slices.Contains(weekdays, d.Weekday()) && !IsDatePaused(p.Pauses, d) {
			out = append(out, d)
		}
	}
	return out
}

// monthlyBetween enumerates the anchor day of each month from the rule's own
// month on. Every month is rebuilt from the anchor independently (not
// iteratively): otherwise the 31st would "drift" onto the 28th forever.
func monthlyBetween(p Payment, start, hardEnd time.Time) []time.Time {
	out := make([]time.Time, 0)
	for k := 0; len(out) < maxOccurrences; k++ {
		d := dateInMonth(p.Since.Year(), p.Since.Month()+time.Month(k), p.Recurrence.DayOfMonth())
		if d.After(hardEnd) {
			break
		}
		if d.Before(p.Since) || d.Before(start) || IsDatePaused(p.Pauses, d) {
			continue
		}
		out = append(out, d)
	}
	return out
}

// yearlyBetween enumerates the anchor month-and-day of each year from the
// rule's own year on, with February 29 clamped in non-leap years.
func yearlyBetween(p Payment, start, hardEnd time.Time) []time.Time {
	out := make([]time.Time, 0)
	for k := 0; len(out) < maxOccurrences; k++ {
		d := dateInMonth(p.Since.Year()+k, p.Recurrence.Month(), p.Recurrence.Day())
		if d.After(hardEnd) {
			break
		}
		if d.Before(p.Since) || d.Before(start) || IsDatePaused(p.Pauses, d) {
			continue
		}
		out = append(out, d)
	}
	return out
}

// NextOccurrenceAfter returns the first occurrence strictly after date (and
// at or before EndDate when set); the boolean is false when the rule has no
// more occurrences.
func NextOccurrenceAfter(p Payment, date time.Time) (time.Time, bool) {
	horizon := nextDay(addYearsClamped(date, 5))
	for _, d := range OccurrencesBetween(p, date, horizon) {
		if d.After(date) {
			return d, true
		}
	}
	return time.Time{}, false
}
