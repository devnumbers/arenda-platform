package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/rentals/domain"
)

// Shared fixture dates of the status table (goconst)..
const (
	dayStart   = "2026-01-10"
	dayMidTerm = "2026-03-01"
	dayJan1    = "2026-01-01"
)

// date is the UTC-midnight calendar date convention of the module (ADR 0048).
func date(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

func rental() domain.Rental {
	return domain.Rental{
		ID:        mustNewID(),
		StartDate: date(dayStart),
		Utilities: domain.UtilitiesIncluded,
	}
}

func mustNewID() uuid.UUID {
	id, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	return id
}

func TestStatusOf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*domain.Rental)
		today  string
		want   domain.Status
	}{
		{
			name:  "start today is active, not upcoming",
			today: dayStart,
			want:  domain.StatusActive,
		},
		{
			name:  "start tomorrow is upcoming",
			today: "2026-01-09",
			want:  domain.StatusUpcoming,
		},
		{
			name:  "inside the term is active",
			today: dayMidTerm,
			want:  domain.StatusActive,
		},
		{
			name: "planned end day itself is still active",
			mutate: func(r *domain.Rental) {
				r.PlannedEndDate = new(date(dayMidTerm))
			},
			today: dayMidTerm,
			want:  domain.StatusActive,
		},
		{
			name: "planned end passed is needs_attention",
			mutate: func(r *domain.Rental) {
				r.PlannedEndDate = new(date(dayMidTerm))
			},
			today: "2026-03-02",
			want:  domain.StatusNeedsAttention,
		},
		{
			name: "open-ended never needs attention",
			mutate: func(r *domain.Rental) {
				r.PlannedEndDate = nil
			},
			today: "2027-06-01",
			want:  domain.StatusActive,
		},
		{
			name: "completion date wins over everything",
			mutate: func(r *domain.Rental) {
				r.PlannedEndDate = new(date(dayJan1)) // Even a past planned end.
				r.CompletedDate = new(date("2026-02-01"))
			},
			today: "2026-03-02",
			want:  domain.StatusCompleted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := rental()
			if tt.mutate != nil {
				tt.mutate(&r)
			}
			if got := domain.StatusOf(r, date(tt.today)); got != tt.want {
				t.Fatalf("StatusOf = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCountPaymentDays(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		start string
		end   string
		day   int
		want  int
	}{
		{
			name:  "full months inclusive both ends",
			start: "2026-01-15", end: "2026-04-15", day: 15,
			want: 4,
		},
		{
			name:  "day before the first occurrence",
			start: "2026-01-16", end: "2026-04-15", day: 15,
			want: 3,
		},
		{
			name:  "end before the first occurrence",
			start: "2026-01-16", end: "2026-02-14", day: 15,
			want: 0,
		},
		{
			name:  "day 31 clamps to the month's last day",
			start: dayJan1, end: "2026-04-30", day: 31,
			want: 4, // Jan 31, Feb 28, Mar 31, Apr 30.
		},
		{
			name:  "day 30 clamps in February",
			start: dayJan1, end: "2026-04-30", day: 30,
			want: 4, // Jan 30, Feb 28, Mar 30, Apr 30.
		},
		{
			name:  "leap February clamps to the 29th",
			start: "2028-01-01", end: "2028-04-30", day: 30,
			want: 4, // Jan 30, Feb 29, Mar 30, Apr 30.
		},
		{
			name:  "one day of month counts each month",
			start: dayJan1, end: dayMidTerm, day: 1,
			want: 3,
		},
		{
			name:  "end before start counts nothing",
			start: "2026-04-01", end: dayJan1, day: 15,
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := domain.CountPaymentDays(date(tt.start), date(tt.end), domain.MustPaymentDay(tt.day))
			if got != tt.want {
				t.Fatalf("CountPaymentDays = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFullMonthsBetween(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		from string
		to   string
		want int
	}{
		{name: "exactly one month", from: dayStart, to: "2026-02-10", want: 1},
		{name: "a day short of the month", from: dayStart, to: "2026-02-09", want: 0},
		{name: "a year", from: dayStart, to: "2027-01-10", want: 12},
		{name: "same day", from: dayStart, to: dayStart, want: 0},
		{name: "to before from", from: "2026-03-10", to: dayStart, want: 0},
		{name: "short month boundary", from: "2026-01-31", to: "2026-02-28", want: 0},
		{name: "short month boundary reached", from: "2026-01-31", to: dayMidTerm, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := domain.FullMonthsBetween(date(tt.from), date(tt.to))
			if got != tt.want {
				t.Fatalf("FullMonthsBetween = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPaymentDay(t *testing.T) {
	t.Parallel()
	t.Run("1..30 keep their day", func(t *testing.T) {
		t.Parallel()
		d, err := domain.NewPaymentDay(15)
		if err != nil {
			t.Fatalf("NewPaymentDay(15): %v", err)
		}
		if d.IsLast() || d.Day() != 15 {
			t.Fatalf("day 15 became %+v", d)
		}
	})
	t.Run("31 is the last-day behaviour (одно поведение, №5)", func(t *testing.T) {
		t.Parallel()
		d, err := domain.NewPaymentDay(31)
		if err != nil {
			t.Fatalf("NewPaymentDay(31): %v", err)
		}
		if !d.IsLast() {
			t.Fatalf("day 31 must normalize to the last-day marker, got %+v", d)
		}
	})
	t.Run("out of range is rejected", func(t *testing.T) {
		t.Parallel()
		for _, n := range []int{0, 32, -1} {
			if _, err := domain.NewPaymentDay(n); err == nil {
				t.Fatalf("NewPaymentDay(%d) accepted", n)
			}
		}
	})
	t.Run("last day reports day 0", func(t *testing.T) {
		t.Parallel()
		d := domain.NewLastPaymentDay()
		if !d.IsLast() || d.Day() != 0 {
			t.Fatalf("last day became %+v", d)
		}
	})
}

func TestUtilitiesValid(t *testing.T) {
	t.Parallel()
	for _, u := range []domain.Utilities{
		domain.UtilitiesIncluded, domain.UtilitiesMetersOnly, domain.UtilitiesFullReceipt,
	} {
		if !u.Valid() {
			t.Fatalf("%q must be valid", u)
		}
	}
	if domain.Utilities("квитанция").Valid() {
		t.Fatal("an unknown utilities mode must not be valid")
	}
}
