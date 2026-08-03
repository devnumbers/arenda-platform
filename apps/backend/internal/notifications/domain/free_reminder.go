package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// FreeReminderPeriodicity is the repetition rule for a free reminder.
type FreeReminderPeriodicity string

const (
	// PeriodicityOnce is a one-shot reminder: a single occurrence at TriggerAt.
	PeriodicityOnce    FreeReminderPeriodicity = "once"
	PeriodicityDaily   FreeReminderPeriodicity = "daily"
	PeriodicityWeekly  FreeReminderPeriodicity = "weekly"
	PeriodicityMonthly FreeReminderPeriodicity = "monthly"
	PeriodicityYearly  FreeReminderPeriodicity = "yearly"
)

// ErrUnknownPeriodicity is returned when a periodicity string is not a known value.
var ErrUnknownPeriodicity = errors.New("unknown free reminder periodicity")

// ParseFreeReminderPeriodicity parses a periodicity string. It returns
// ErrUnknownPeriodicity for unknown values.
func ParseFreeReminderPeriodicity(s string) (FreeReminderPeriodicity, error) {
	switch FreeReminderPeriodicity(s) {
	case PeriodicityOnce, PeriodicityDaily, PeriodicityWeekly, PeriodicityMonthly, PeriodicityYearly:
		return FreeReminderPeriodicity(s), nil
	default:
		return "", fmt.Errorf("%w: %q", ErrUnknownPeriodicity, s)
	}
}

// Valid reports whether the periodicity is a known value.
func (p FreeReminderPeriodicity) Valid() bool {
	switch p {
	case PeriodicityOnce, PeriodicityDaily, PeriodicityWeekly, PeriodicityMonthly, PeriodicityYearly:
		return true
	default:
		return false
	}
}

// FreeReminder is a user-created reminder template: an arbitrary reminder tied
// to a property but not to an operation or lease (e.g. renew insurance, check
// meters). The template stores the first trigger instant and a periodicity;
// concrete reminders are materialized from it into the reminders table. The
// property FK is ON DELETE CASCADE, so deleting a property removes its free
// reminders in both deletion modes of ADR 0025.
type FreeReminder struct {
	ID          uuid.UUID
	OwnerID     uuid.UUID
	PropertyID  uuid.UUID
	Title       string
	TriggerAt   time.Time
	Periodicity FreeReminderPeriodicity
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// expandFreeReminderLimit caps the number of occurrences
// ExpandFreeReminderOccurrences generates, guarding against an unbounded loop
// if the window is huge.
const expandFreeReminderLimit = 2500

// ExpandFreeReminderOccurrences returns the UTC instants at which a free
// reminder fires within the half-open time window [from, to). The first
// occurrence is always the template's TriggerAt; subsequent occurrences advance
// by the periodicity (day, week, month, or year). A 'once' reminder yields at
// most one occurrence. Occurrences before 'from' are skipped; occurrences at or
// after 'to' stop generation. Month and year advances clamp the day to the last
// day of the target month (e.g. Jan 31 + 1 month -> Feb 28).
//
// The function is a pure mapping "template + window -> occurrences": it does
// not consult the database and is independent of the materialization horizon.
func ExpandFreeReminderOccurrences(fr FreeReminder, from, to time.Time) []time.Time {
	if !to.After(from) { // to.Before(from) || to.Equal(from)
		return nil
	}

	var out []time.Time
	current := fr.TriggerAt

	// For daily/weekly periodicity with a trigger far in the past and a window
	// far in the future, advance 'current' to the first occurrence that is on or
	// after 'from' in O(1) instead of stepping day by day.
	if fr.Periodicity == PeriodicityDaily || fr.Periodicity == PeriodicityWeekly {
		current = fastForwardDaily(current, from, fr.Periodicity)
	}

	// Loop over occurrences until we pass the end of the window (to is exclusive).
	for current.Before(to) {
		// Include occurrences that are within [from, to). Occurrences before
		// 'from' are skipped but stepping continues.
		if !current.Before(from) {
			out = append(out, current)
			if len(out) >= expandFreeReminderLimit {
				break
			}
		}

		if fr.Periodicity == PeriodicityOnce {
			break
		}

		next := nextOccurrence(current, fr.Periodicity)
		if !next.After(current) { // safety: never advance backwards / stall
			break
		}
		current = next
	}

	return out
}

// fastForwardDaily advances a daily/weekly occurrence forward so that it lands
// on or as close as possible before 'from' without overshooting, in O(1). The
// returned instant is the last occurrence at or before 'from' (or the original
// trigger if it is already at or after 'from'); the caller's main loop then
// emits only the occurrences that fall inside [from, to).
func fastForwardDaily(trigger, from time.Time, p FreeReminderPeriodicity) time.Time {
	if !trigger.Before(from) {
		return trigger
	}
	step := dailyStep(p)
	if step <= 0 {
		return trigger
	}
	delta := from.Sub(trigger)
	n := max(0, int(delta/step))
	advanced := trigger.Add(time.Duration(n) * step)
	// 'advanced' is the last occurrence at or before 'from'; if it lands exactly
	// on 'from' the main loop includes it (from is inclusive).
	for advanced.Before(from) {
		next := advanced.Add(step)
		if !next.After(from) {
			advanced = next
			continue
		}
		break
	}
	return advanced
}

// dailyStep returns the time.Duration step for daily/weekly periodicity.
func dailyStep(p FreeReminderPeriodicity) time.Duration {
	switch p {
	case PeriodicityDaily:
		return 24 * time.Hour
	case PeriodicityWeekly:
		return 7 * 24 * time.Hour
	default:
		return 0
	}
}

// nextOccurrence advances an occurrence to the next one according to the
// periodicity. Month and year advances clamp the day to the last day of the
// target month (e.g. Jan 31 + 1 month -> Feb 28; Feb 29 + 1 year -> Feb 28 in
// a non-leap year). Note: we do NOT use time.AddDate for month/year because it
// normalizes overflowed days (Feb 29 -> Mar 1) rather than clamping.
func nextOccurrence(current time.Time, p FreeReminderPeriodicity) time.Time {
	switch p {
	case PeriodicityDaily:
		return current.AddDate(0, 0, 1)
	case PeriodicityWeekly:
		return current.AddDate(0, 0, 7)
	case PeriodicityMonthly:
		return advanceMonth(current)
	case PeriodicityYearly:
		return advanceYear(current)
	default:
		return current
	}
}

// advanceYear adds one calendar year, clamping the day to the last day of the
// month when necessary (e.g. Feb 29 in a leap year -> Feb 28 in a non-leap
// year).
func advanceYear(t time.Time) time.Time {
	year, month, day := t.Date()
	hour, minute, sec := t.Clock()
	loc := t.Location()

	targetYear := year + 1
	lastDay := lastDayOfMonth(targetYear, month)
	clampedDay := min(day, lastDay)

	return time.Date(targetYear, month, clampedDay, hour, minute, sec, t.Nanosecond(), loc)
}

// advanceMonth adds one calendar month, clamping the day to the last day of the
// target month when necessary (e.g. Jan 31 -> Feb 28/29).
func advanceMonth(t time.Time) time.Time {
	year, month, day := t.Date()
	hour, minute, sec := t.Clock()
	loc := t.Location()

	targetMonth := month + 1
	targetYear := year
	if targetMonth > 12 {
		targetMonth = 1
		targetYear++
	}

	lastDay := lastDayOfMonth(targetYear, targetMonth)
	clampedDay := min(day, lastDay)

	return time.Date(targetYear, targetMonth, clampedDay, hour, minute, sec, t.Nanosecond(), loc)
}

// lastDayOfMonth returns the last calendar day of the given month/year.
func lastDayOfMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// FreeReminderTemplate is a free reminder template along with its resolved
// property name. It is a read projection used by the calendar endpoint.
type FreeReminderTemplate struct {
	FreeReminder
	PropertyName *string // nil when the property was detached (orphan)
}
