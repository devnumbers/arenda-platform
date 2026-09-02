package domain

import "time"

// maxOccurrences caps enumeration as a safety ceiling (the payments/domain
// precedent): a daily rule enumerated over an unbounded window cannot loop
// forever.
const maxOccurrences = 1000

// OccurrencesBetween enumerates the rule's occurrence dates in the inclusive
// window [start, end], as a pure function of the rule:
//
//   - once — the anchor date only;
//   - daily — every day from the anchor on;
//   - weekly — the anchor's weekday, every 7 days;
//   - monthly — the anchor day of month, clamped to each month's last day
//     (the 31st → 28/29), rebuilt from the anchor every month without drift;
//   - yearly — the anchor month and day, February 29 clamped;
//   - generation is lower-bounded by the anchor itself (no backdated
//     occurrences — the application rejects due dates before today).
//
// An undated rule has no occurrences: its single task is materialized
// directly at rule creation (resolution #496). Dates are never shifted onto
// business days.
func OccurrencesBetween(rule TaskRule, start, end time.Time) []time.Time {
	if rule.DueDate == nil || start.After(end) {
		return nil
	}
	anchor := *rule.DueDate
	switch rule.Repeat {
	case RepeatOnce:
		if anchor.Before(start) || anchor.After(end) {
			return nil
		}
		return []time.Time{anchor}
	case RepeatDaily:
		first := maxDate(anchor, start)
		return dailyBetween(first, end)
	case RepeatWeekly:
		first := maxDate(anchor, start)
		return weeklyBetween(first, end)
	case RepeatMonthly:
		return monthlyBetween(anchor, start, end)
	case RepeatYearly:
		return yearlyBetween(anchor, start, end)
	default:
		return nil
	}
}

// dailyBetween enumerates every day of the window.
func dailyBetween(first, end time.Time) []time.Time {
	out := make([]time.Time, 0)
	for d := first; !d.After(end) && len(out) < maxOccurrences; d = nextDay(d) {
		out = append(out, d)
	}
	return out
}

// weeklyBetween enumerates the anchor's weekday of every week of the window.
func weeklyBetween(first, end time.Time) []time.Time {
	out := make([]time.Time, 0)
	for d := first; !d.After(end) && len(out) < maxOccurrences; d = addDays(d, 7) {
		out = append(out, d)
	}
	return out
}

// monthlyBetween enumerates the anchor day of each month from the anchor's
// own month on. Every month is rebuilt from the anchor independently (not
// iteratively): otherwise the 31st would "drift" onto the 28th forever.
func monthlyBetween(anchor, start, end time.Time) []time.Time {
	out := make([]time.Time, 0)
	for k := 0; len(out) < maxOccurrences; k++ {
		// Month arithmetic by number, not AddDate: Jan 31 + 1 month must be
		// February, not March 3rd.
		total := int(anchor.Month()) - 1 + k
		d := dateInMonth(anchor.Year()+total/12, time.Month(total%12+1), anchor.Day())
		if d.After(end) {
			break
		}
		if d.Before(anchor) || d.Before(start) {
			continue
		}
		out = append(out, d)
	}
	return out
}

// yearlyBetween enumerates the anchor month-and-day of each year from the
// anchor's own year on, with February 29 clamped in non-leap years.
func yearlyBetween(anchor, start, end time.Time) []time.Time {
	out := make([]time.Time, 0)
	for k := 0; len(out) < maxOccurrences; k++ {
		d := dateInMonth(anchor.Year()+k, anchor.Month(), anchor.Day())
		if d.After(end) {
			break
		}
		if d.Before(anchor) || d.Before(start) {
			continue
		}
		out = append(out, d)
	}
	return out
}

// maxDate returns the later of two calendar dates.
func maxDate(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
