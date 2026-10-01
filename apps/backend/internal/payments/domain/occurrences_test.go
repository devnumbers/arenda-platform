package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Calendar fixtures shared by the domain tests: today is 2026-08-25 (t0), the
// t-prefixed names count days back from it.
const (
	dayT3    = "2026-08-22"
	dayT2    = "2026-08-23"
	dayT1    = "2026-08-24"
	dayT0    = "2026-08-25"
	dayNext  = "2026-08-26"
	dayNext2 = "2026-08-27"
	// The 5th of the month after today: the re-dated schedule's next
	// occurrence in the day-edit fixtures (ticket #815).
	daySep5 = "2026-09-05"
)

// d parses a calendar date ('YYYY-MM-DD' at UTC midnight) — the domain's date
// convention.
func d(s string) time.Time {
	parsed, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return parsed
}

func dp(s string) *time.Time {
	date := d(s)
	return &date
}

// rule builds a payment rule for occurrence tests.
func rule(t *testing.T, recurrence Recurrence, since string, endDate *string, pauses ...PauseInterval) Payment {
	t.Helper()
	p := Payment{
		ID:            uuidMustV7(),
		Type:          TypeExpense,
		Title:         "t",
		AmountKopecks: 100,
		Recurrence:    recurrence,
		Since:         d(since),
		Category:      CategoryRef{Slug: new("utilities")},
		Pauses:        pauses,
	}
	if endDate != nil {
		p.EndDate = dp(*endDate)
	}
	return p
}

func dates(ts []time.Time) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Format(time.DateOnly)
	}
	return out
}

func TestOccurrencesBetween_MonthlyClamps31stWithoutDrift(t *testing.T) {
	t.Parallel()
	// Prototype smoke: янв31→фев28→мар31 — the anchor never slides.
	const febLastDay = "2026-02-28"
	rec := mustMonthlyLastDay(t)
	p := rule(t, rec, "2026-01-31", nil)

	got := dates(OccurrencesBetween(p, d("2026-01-01"), d("2026-04-01")))
	assert.Equal(t, []string{"2026-01-31", febLastDay, "2026-03-31"}, got)
}

func TestOccurrencesBetween_MonthlyAnchorInSinceMonthAlreadyPassed(t *testing.T) {
	t.Parallel()
	// Anchor earlier in the month than since: the passed day does not
	// materialize; the first occurrence is next month (prototype smoke c2).
	rec := mustMonthly(t, 5)
	p := rule(t, rec, "2026-08-20", nil)

	got := dates(OccurrencesBetween(p, d("2026-08-01"), d("2026-09-30")))
	assert.Equal(t, []string{daySep5}, got)
}

func TestNextOccurrenceAfter_ClampedFebruary(t *testing.T) {
	t.Parallel()
	rec := mustMonthlyLastDay(t)
	p := rule(t, rec, "2026-01-31", nil)

	next, ok := NextOccurrenceAfter(p, d("2026-02-01"))
	require.True(t, ok)
	assert.Equal(t, "2026-02-28", next.Format(time.DateOnly))
}

func TestOccurrencesBetween_MonthlyMultipleDaysOrdered(t *testing.T) {
	t.Parallel()
	// Several days of month: one occurrence per selected day, in date order.
	rec := mustMonthly(t, 18, 1)
	p := rule(t, rec, "2026-08-01", nil)

	got := dates(OccurrencesBetween(p, d("2026-08-01"), d("2026-09-30")))
	assert.Equal(t, []string{"2026-08-01", "2026-08-18", "2026-09-01", "2026-09-18"}, got)
}

func TestOccurrencesBetween_MonthlyLastDayClampsInShortMonths(t *testing.T) {
	t.Parallel()
	// The last-day marker clamps to the actual last day (February → 28th).
	rec, err := NewMonthlyRecurrence([]int{30}, true)
	require.NoError(t, err)
	p := rule(t, rec, "2026-02-01", nil)

	got := dates(OccurrencesBetween(p, d("2026-02-01"), d("2026-04-30")))
	assert.Equal(t, []string{"2026-02-28", "2026-03-30", "2026-03-31", "2026-04-30"}, got)
}

func TestRecurrenceJSON_MonthlyLegacyDayOfMonth(t *testing.T) {
	t.Parallel()
	// Stored rows of the pre-multi-day shape keep reading: a plain day maps
	// to a one-day set, the legacy 31st maps to the last-day marker.
	var plain Recurrence
	require.NoError(t, plain.UnmarshalJSON([]byte(`{"kind":"monthly","dayOfMonth":10}`)))
	assert.Equal(t, RecurrenceMonthly, plain.Kind())
	assert.Equal(t, []int{10}, plain.DaysOfMonth())
	assert.False(t, plain.LastDay())

	var last Recurrence
	require.NoError(t, last.UnmarshalJSON([]byte(`{"kind":"monthly","dayOfMonth":31}`)))
	assert.Equal(t, RecurrenceMonthly, last.Kind())
	assert.Empty(t, last.DaysOfMonth())
	assert.True(t, last.LastDay())
}

func TestRecurrenceJSON_MonthlyNewShapeRoundTrip(t *testing.T) {
	t.Parallel()
	rec, err := NewMonthlyRecurrence([]int{18, 1}, true)
	require.NoError(t, err)
	data, err := rec.MarshalJSON()
	require.NoError(t, err)
	assert.JSONEq(t, `{"kind":"monthly","daysOfMonth":[1,18],"lastDay":true}`, string(data))

	var back Recurrence
	require.NoError(t, back.UnmarshalJSON(data))
	assert.Equal(t, []int{1, 18}, back.DaysOfMonth())
	assert.True(t, back.LastDay())
}

func TestRecurrenceConstructors_MonthlyRejectsEmptyAndOutOfRange(t *testing.T) {
	t.Parallel()
	_, err := NewMonthlyRecurrence(nil, false)
	require.Error(t, err)
	_, err = NewMonthlyRecurrence([]int{1, 1}, false)
	require.Error(t, err)
	_, err = NewMonthlyRecurrence([]int{0, 15}, false)
	require.Error(t, err)
	_, err = NewMonthlyRecurrence([]int{31}, false)
	require.Error(t, err)
}

func TestOccurrencesBetween_WeeklyOnlySelectedDaysFromSince(t *testing.T) {
	t.Parallel()
	// Prototype smoke: created on Wednesday, Saturdays only — the creation
	// day itself is not an occurrence.
	rec := mustWeekly(t, time.Saturday)
	p := rule(t, rec, "2026-08-26", nil)

	got := dates(OccurrencesBetween(p, d("2026-08-20"), d(daySep5)))
	assert.Equal(t, []string{"2026-08-29", daySep5}, got)
}

func TestOccurrencesBetween_YearlyClampsLeapDay(t *testing.T) {
	t.Parallel()
	rec := mustYearly(t, time.February, 29)
	p := rule(t, rec, "2024-02-29", nil)

	got := dates(OccurrencesBetween(p, d("2024-01-01"), d("2027-01-01")))
	assert.Equal(t, []string{"2024-02-29", "2025-02-28", "2026-02-28"}, got)
}

func TestOccurrencesBetween_RespectsSinceAndEndDate(t *testing.T) {
	t.Parallel()
	// No backdated occurrences (№17): everything before since is cut.
	rec := NewDailyRecurrence()

	p := rule(t, rec, dayT3, nil)
	got := dates(OccurrencesBetween(p, d("2026-08-01"), d(dayT0)))
	assert.Equal(t, []string{dayT3, dayT2, dayT1, dayT0}, got)

	// An endDate stops generation: nothing after it (prototype smoke).
	end := dayT1
	p = rule(t, rec, dayT3, &end)
	got = dates(OccurrencesBetween(p, d("2026-08-01"), d("2026-08-30")))
	assert.Equal(t, []string{dayT3, dayT2, dayT1}, got)

	// An endDate before the window start: nothing at all.
	end = "2026-08-01"
	p = rule(t, rec, dayT3, &end)
	assert.Empty(t, OccurrencesBetween(p, d("2026-08-02"), d("2026-08-30")))
}

func TestOccurrencesBetween_PauseCutsIntervalsHalfOpen(t *testing.T) {
	t.Parallel()
	// [from, to): from inclusive, to (the resume day) already outside.
	rec := NewDailyRecurrence()
	pauses := []PauseInterval{{From: d(dayT1), To: dp(dayNext)}}
	p := rule(t, rec, dayT3, nil, pauses...)

	got := dates(OccurrencesBetween(p, d(dayT3), d("2026-08-28")))
	assert.Equal(t, []string{dayT3, dayT2, dayNext, dayNext2, "2026-08-28"}, got)

	// An active open-ended pause cuts everything from its start on.
	active := []PauseInterval{{From: d(dayT1), To: nil}}
	p = rule(t, rec, dayT3, nil, active...)
	got = dates(OccurrencesBetween(p, d(dayT3), d("2026-08-28")))
	assert.Equal(t, []string{dayT3, dayT2}, got)
}

func TestIsDatePaused(t *testing.T) {
	t.Parallel()
	pauses := []PauseInterval{{From: d(dayT1), To: dp(dayNext)}}

	assert.False(t, IsDatePaused(pauses, d(dayT2)), "before the pause")
	assert.True(t, IsDatePaused(pauses, d(dayT1)), "from is inclusive")
	assert.True(t, IsDatePaused(pauses, d(dayT0)), "inside the pause")
	assert.False(t, IsDatePaused(pauses, d(dayNext)), "to is exclusive")
	assert.False(t, IsDatePaused(nil, d(dayT0)), "no pauses")

	active := []PauseInterval{{From: d(dayT1), To: nil}}
	assert.True(t, IsDatePaused(active, d("2027-01-01")), "open-ended covers the future")

	_, is := ActivePause(pauses)
	assert.False(t, is, "closed interval is not active")
	got, is := ActivePause(active)
	assert.True(t, is)
	assert.Equal(t, d(dayT1), got.From)
	assert.Nil(t, got.To)
}

func mustWeekly(t *testing.T, days ...time.Weekday) Recurrence {
	t.Helper()
	rec, err := NewWeeklyRecurrence(days)
	require.NoError(t, err)
	return rec
}

func mustMonthly(t *testing.T, days ...int) Recurrence {
	t.Helper()
	rec, err := NewMonthlyRecurrence(days, false)
	require.NoError(t, err)
	return rec
}

func mustMonthlyLastDay(t *testing.T) Recurrence {
	t.Helper()
	rec, err := NewMonthlyRecurrence(nil, true)
	require.NoError(t, err)
	return rec
}

func mustYearly(t *testing.T, month time.Month, day int) Recurrence {
	t.Helper()
	rec, err := NewYearlyRecurrence(month, day)
	require.NoError(t, err)
	return rec
}
