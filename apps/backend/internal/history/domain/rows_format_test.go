package domain

// The row-text formatters of the new template's rental rows (D3, решение
// #1317 §7): the genitive day-month header («1 ноября»), the term in full
// civil months with a day remainder («11 месяцев», «10 месяцев 20 дней»,
// короче месяца — в днях) and the period line with the years. The month
// arithmetic mirrors rentals/domain.FullMonthsBetween deliberately — the
// duplication is commented at the implementation, not papered over here.

import (
	"testing"
	"time"
)

func at(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestPluralRU(t *testing.T) {
	t.Parallel()
	one, few, many := "месяц", "месяца", "месяцев"
	cases := []struct {
		n    int
		want string
	}{
		{1, one},
		{2, few},
		{5, many},
		{11, many},
		{21, one},
		{22, few},
		{25, many},
		{101, one},
		{111, many},
		{0, many},
	}
	for _, tc := range cases {
		if got := pluralRU(tc.n, "месяц", "месяца", "месяцев"); got != tc.want {
			t.Errorf("pluralRU(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
	dOne, dFew, dMany := "день", "дня", "дней"
	days := []struct {
		n    int
		want string
	}{
		{1, dOne}, {2, dFew}, {5, dMany}, {12, dMany}, {14, dMany}, {23, dFew},
	}
	for _, tc := range days {
		if got := pluralRU(tc.n, "день", "дня", "дней"); got != tc.want {
			t.Errorf("pluralRU(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestFormatDayMonthGenitive(t *testing.T) {
	t.Parallel()
	cases := []struct {
		t    time.Time
		want string
	}{
		{at(2026, time.November, 1), "1 ноября"},
		{at(2027, time.September, 30), "30 сентября"},
		{at(2026, time.March, 21), "21 марта"},
	}
	for _, tc := range cases {
		if got := formatDayMonthGenitive(tc.t); got != tc.want {
			t.Errorf("formatDayMonthGenitive(%s) = %q, want %q", tc.t, got, tc.want)
		}
	}
}

func TestFormatTerm(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		from, to time.Time
		want     string
	}{
		// Конечная дата — последний занятый день: «1 ноября → 30 сентября»
		// = 11 занятых месяцев (канон владельца, D3).
		{"exactly one month", at(2026, time.February, 1), at(2026, time.February, 28), "1 месяц"},
		{"eleven months over the month boundary", at(2026, time.November, 1), at(2027, time.September, 30), "11 месяцев"},
		{"two months", at(2026, time.March, 1), at(2026, time.April, 30), "2 месяца"},
		{"same day next month keeps the day", at(2026, time.February, 1), at(2026, time.March, 1), "1 месяц 1 день"},
		{"shorter than a month reads in days", at(2026, time.November, 1), at(2026, time.November, 23), "23 дня"},
		{"mixed remainder", at(2026, time.November, 1), at(2027, time.September, 21), "10 месяцев 21 день"},
		{"month bound into a shorter month", at(2026, time.January, 31), at(2026, time.February, 28), "1 месяц"},
		{"quarter day clamp", at(2026, time.January, 31), at(2026, time.March, 31), "2 месяца 1 день"},
		{"to not after from", at(2026, time.November, 1), at(2026, time.November, 1), "1 день"},
		{"to before from degenerates to zero", at(2026, time.November, 10), at(2026, time.November, 1), "0 дней"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := formatTerm(tc.from, tc.to); got != tc.want {
				t.Errorf("formatTerm(%s, %s) = %q, want %q", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

func TestFormatPeriodLine(t *testing.T) {
	t.Parallel()
	if got := formatPeriodLine(at(2026, time.November, 1), at(2027, time.September, 30)); got != "с 1 ноября 2026 по 30 сентября 2027" {
		t.Errorf("formatPeriodLine = %q", got)
	}
	// Бессрочная аренда: конечной даты нет — одна граница.
	if got := formatPeriodLine(at(2026, time.November, 1), time.Time{}); got != "с 1 ноября 2026" {
		t.Errorf("formatPeriodLine (open-ended) = %q", got)
	}
}
