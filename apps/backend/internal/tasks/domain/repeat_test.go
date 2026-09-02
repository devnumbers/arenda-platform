package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// d builds a UTC-midnight calendar date (the module's date convention).
func d(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func dateOnly(t time.Time) string { return t.Format(time.DateOnly) }

func datedRule(repeat RepeatKind, year int, month time.Month, day int) TaskRule {
	due := d(year, month, day)
	return TaskRule{ID: testRuleID, Repeat: repeat, DueDate: &due}
}

var testRuleID = uuid.MustParse("0198b692-7d00-7000-8000-000000000001")

func TestOccurrencesBetween_Once(t *testing.T) {
	t.Parallel()

	rule := datedRule(RepeatOnce, 2026, time.September, 10)

	got := OccurrencesBetween(rule, d(2026, time.September, 1), d(2026, time.September, 30))
	if len(got) != 1 || dateOnly(got[0]) != sep10 {
		t.Fatalf("once inside window = %v, want [%s]", got, sep10)
	}

	if got := OccurrencesBetween(rule, d(2026, time.September, 11), d(2026, time.September, 30)); len(got) != 0 {
		t.Fatalf("once after window = %v, want empty", got)
	}
	if got := OccurrencesBetween(rule, d(2026, time.September, 1), d(2026, time.September, 9)); len(got) != 0 {
		t.Fatalf("once before window = %v, want empty", got)
	}
}

func TestOccurrencesBetween_UndatedRuleHasNoOccurrences(t *testing.T) {
	t.Parallel()

	rule := TaskRule{Repeat: RepeatOnce} // No due date.

	if got := OccurrencesBetween(rule, d(2026, time.September, 1), d(2026, time.September, 30)); len(got) != 0 {
		t.Fatalf("undated rule = %v, want empty", got)
	}
}

func TestOccurrencesBetween_Daily(t *testing.T) {
	t.Parallel()

	rule := datedRule(RepeatDaily, 2026, time.September, 10)

	got := OccurrencesBetween(rule, d(2026, time.September, 10), d(2026, time.September, 13))
	want := []string{sep10, sep11(), sep12(), sep13()}
	if len(got) != len(want) {
		t.Fatalf("daily = %v, want %v", got, want)
	}
	for i, w := range want {
		if dateOnly(got[i]) != w {
			t.Fatalf("daily[%d] = %s, want %s", i, dateOnly(got[i]), w)
		}
	}
}

// The consecutive September days of the daily fixtures.
func sep11() string { return "2026-09-11" }
func sep12() string { return "2026-09-12" }
func sep13() string { return "2026-09-13" }

func TestOccurrencesBetween_DailyLowerBoundedByAnchor(t *testing.T) {
	t.Parallel()

	rule := datedRule(RepeatDaily, 2026, time.September, 10)

	got := OccurrencesBetween(rule, d(2026, time.September, 1), d(2026, time.September, 12))
	if len(got) != 3 || dateOnly(got[0]) != sep10 {
		t.Fatalf("daily window before anchor = %v, want start at %s", got, sep10)
	}
}

func TestOccurrencesBetween_WeeklyKeepsWeekday(t *testing.T) {
	t.Parallel()

	// 2026-09-10 is a Thursday.
	if wd := d(2026, time.September, 10).Weekday(); wd != time.Thursday {
		t.Fatalf("test anchor weekday = %v, want Thursday", wd)
	}
	rule := datedRule(RepeatWeekly, 2026, time.September, 10)

	got := OccurrencesBetween(rule, d(2026, time.September, 10), d(2026, time.October, 10))
	want := []string{sep10, sep17, sep24, "2026-10-01", "2026-10-08"}
	if len(got) != len(want) {
		t.Fatalf("weekly = %v, want %v", got, want)
	}
	for i, w := range want {
		if dateOnly(got[i]) != w {
			t.Fatalf("weekly[%d] = %s, want %s", i, dateOnly(got[i]), w)
		}
	}
}

func TestOccurrencesBetween_MonthlyClampsWithoutDrift(t *testing.T) {
	t.Parallel()

	// Anchor on the 31st: short months clamp to their last day and never
	// drift — every month is rebuilt from the anchor (resolution #496).
	rule := datedRule(RepeatMonthly, 2026, time.January, 31)

	got := OccurrencesBetween(rule, d(2026, time.January, 31), d(2026, time.June, 30))
	want := []string{
		"2026-01-31", "2026-02-28", "2026-03-31",
		"2026-04-30", "2026-05-31", "2026-06-30",
	}
	if len(got) != len(want) {
		t.Fatalf("monthly = %v, want %v", got, want)
	}
	for i, w := range want {
		if dateOnly(got[i]) != w {
			t.Fatalf("monthly[%d] = %s, want %s", i, dateOnly(got[i]), w)
		}
	}
}

func TestOccurrencesBetween_MonthlyLeapFebruary(t *testing.T) {
	t.Parallel()

	rule := datedRule(RepeatMonthly, 2024, time.January, 29)

	got := OccurrencesBetween(rule, d(2024, time.January, 29), d(2024, time.April, 30))
	want := []string{"2024-01-29", "2024-02-29", "2024-03-29", "2024-04-29"}
	for i, w := range want {
		if dateOnly(got[i]) != w {
			t.Fatalf("monthly leap[%d] = %s, want %s", i, dateOnly(got[i]), w)
		}
	}
}

func TestOccurrencesBetween_YearlyClampsFebruary29(t *testing.T) {
	t.Parallel()

	rule := datedRule(RepeatYearly, 2024, time.February, 29)

	got := OccurrencesBetween(rule, d(2024, time.February, 29), d(2028, time.December, 31))
	want := []string{"2024-02-29", "2025-02-28", "2026-02-28", "2027-02-28", "2028-02-29"}
	if len(got) != len(want) {
		t.Fatalf("yearly = %v, want %v", got, want)
	}
	for i, w := range want {
		if dateOnly(got[i]) != w {
			t.Fatalf("yearly[%d] = %s, want %s", i, dateOnly(got[i]), w)
		}
	}
}

func TestOccurrencesBetween_WindowBoundsInclusive(t *testing.T) {
	t.Parallel()

	rule := datedRule(RepeatDaily, 2026, time.September, 10)

	got := OccurrencesBetween(rule, d(2026, time.September, 11), d(2026, time.September, 13))
	if len(got) != 3 || dateOnly(got[0]) != sep11() || dateOnly(got[len(got)-1]) != sep13() {
		t.Fatalf("window = %v, want inclusive [%s, %s]", got, sep11(), sep13())
	}
}
