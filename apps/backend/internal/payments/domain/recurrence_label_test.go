package domain

// Словесность расписания — порт recurrenceLabel прототипа (резолюция #452):
// формулировки совпадают с фронтом дословно, включая порядок дней «пн, чт,
// вс» и родительный падеж годового «Каждое 13 мая».

import (
	"testing"
	"time"
)

func TestRecurrenceLabel(t *testing.T) {
	t.Parallel()
	weekly := func(days ...time.Weekday) Recurrence {
		r, err := NewWeeklyRecurrence(days)
		if err != nil {
			t.Fatalf("NewWeeklyRecurrence: %v", err)
		}
		return r
	}
	monthly := func(days []int, lastDay bool) Recurrence {
		r, err := NewMonthlyRecurrence(days, lastDay)
		if err != nil {
			t.Fatalf("NewMonthlyRecurrence: %v", err)
		}
		return r
	}
	yearly := func(month time.Month, day int) Recurrence {
		r, err := NewYearlyRecurrence(month, day)
		if err != nil {
			t.Fatalf("NewYearlyRecurrence: %v", err)
		}
		return r
	}

	cases := []struct {
		name string
		r    Recurrence
		want string
	}{
		{"daily", NewDailyRecurrence(), "Ежедневно"},
		{"single monday", weekly(time.Monday), "Каждый понедельник"},
		// Род и падеж: среда и воскресенье склоняются иначе понедельника.
		{"single wednesday", weekly(time.Wednesday), "Каждую среду"},
		{"single sunday", weekly(time.Sunday), "Каждое воскресенье"},
		{"multiple weekdays monday-first", weekly(time.Sunday, time.Thursday, time.Monday), "Каждую неделю в пн, чт, вс"},
		{"single day of month", monthly([]int{1}, false), "Каждый месяц 1 числа"},
		{"two days", monthly([]int{18, 1}, false), "Каждый месяц 1 и 18 числа"},
		{"last day only", monthly(nil, true), "Последний день каждого месяца"},
		{"days plus last day", monthly([]int{1}, true), "Каждый месяц 1 числа и в последний день месяца"},
		{"yearly", yearly(time.May, 13), "Каждое 13 мая"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := RecurrenceLabel(tc.r); got != tc.want {
				t.Errorf("RecurrenceLabel = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPaymentTypeLabel(t *testing.T) {
	t.Parallel()
	if got := PaymentTypeLabel(TypeIncome); got != "Доход" {
		t.Errorf("income = %q", got)
	}
	if got := PaymentTypeLabel(TypeExpense); got != "Расход" {
		t.Errorf("expense = %q", got)
	}
}
