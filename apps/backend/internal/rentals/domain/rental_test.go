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
	dayJan15   = "2026-01-15"
	dayMar2    = "2026-03-02"
	dayFeb10   = "2026-02-10"
	dayFeb28   = "2026-02-28"
	dayOct10   = "2026-10-10"
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
			today: dayMar2,
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
			today: dayMar2,
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
			// #802 F2: the term window is half-open — the occurrence on the
			// planned end opens a month beyond the rental.
			name:  "end on the payment day does not count",
			start: dayJan15, end: "2026-04-15", day: 15,
			want: 3,
		},
		{
			name:  "12-month term yields 12 payments",
			start: "2026-10-01", end: "2027-10-01", day: 1,
			want: 12,
		},
		{
			// The clamped occurrence can land on the end itself (a last-day
			// rent due Apr 30 with the term ending Apr 30): the half-open
			// window drops it too — one occurrence short of the calendar
			// months, consistent with FullMonthsBetween.
			name:  "clamped occurrence on the end does not count",
			start: dayJan1, end: "2026-04-30", day: 31,
			want: 3, // Jan 31, Feb 28, Mar 31; the Apr 30 rent is the end.
		},
		{
			// Гараж, the #802 audit case: the end falls between occurrences.
			name:  "end between occurrences keeps the count",
			start: "2026-03-10", end: "2028-03-09", day: 10,
			want: 24,
		},
		{
			name:  "day before the first occurrence",
			start: "2026-01-16", end: "2026-04-14", day: 15,
			want: 2,
		},
		{
			name:  "end before the first occurrence",
			start: "2026-01-16", end: "2026-02-14", day: 15,
			want: 0,
		},
		{
			name:  "end on the start counts nothing",
			start: dayJan15, end: dayJan15, day: 15,
			want: 0,
		},
		{
			name:  "day 31 clamps to the month's last day",
			start: dayJan1, end: "2026-05-01", day: 31,
			want: 4, // Jan 31, Feb 28, Mar 31, Apr 30.
		},
		{
			name:  "day 30 clamps in February",
			start: dayJan1, end: "2026-05-01", day: 30,
			want: 4, // Jan 30, Feb 28, Mar 30, Apr 30.
		},
		{
			name:  "leap February clamps to the 29th",
			start: "2028-01-01", end: "2028-05-01", day: 30,
			want: 4, // Jan 30, Feb 29, Mar 30, Apr 30.
		},
		{
			name:  "one day of month counts each month",
			start: dayJan1, end: dayMar2, day: 1,
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
		{name: "exactly one month", from: dayStart, to: dayFeb10, want: 1},
		{name: "a day short of the month", from: dayStart, to: "2026-02-09", want: 0},
		{name: "a year", from: dayStart, to: "2027-01-10", want: 12},
		{name: "same day", from: dayStart, to: dayStart, want: 0},
		{name: "to before from", from: "2026-03-10", to: dayStart, want: 0},
		{name: "short month boundary", from: "2026-01-31", to: dayFeb28, want: 0},
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

// TestPaymentDayFirstPaymentDate pins the schedule window's lower bound
// (ticket #1154): the first payment-day date on or after the rental start —
// the planned end before it would leave the rent zero payments.
func TestPaymentDayFirstPaymentDate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		day   int // 0 = the last-day marker.
		start string
		want  string
	}{
		{
			// The research case: старт 10-го, день оплаты 25-е → 25-е.
			name:  "keeps its day later in the start month",
			day:   25,
			start: dayOct10,
			want:  "2026-10-25",
		},
		{
			name:  "the passed day jumps to the next month",
			day:   5,
			start: dayOct10,
			want:  "2026-11-05",
		},
		{
			name:  "the last-day marker fires on the month's end",
			day:   0,
			start: dayOct10,
			want:  "2026-10-31",
		},
		{
			// 31 normalizes to the marker (одно поведение, №5): a short
			// month clamps to its actual last day.
			name:  "day 31 from november clamps to the 30th",
			day:   31,
			start: "2026-11-10",
			want:  "2026-11-30",
		},
		{
			name:  "day 31 from february clamps to the 28th",
			day:   31,
			start: dayFeb10,
			want:  dayFeb28,
		},
		{
			name:  "day 30 from february clamps to the 28th",
			day:   30,
			start: dayFeb10,
			want:  dayFeb28,
		},
		{
			// The occurrence on the start day itself counts: the rent is due
			// from the first day on.
			name:  "the start day itself is the first occurrence",
			day:   10,
			start: dayOct10,
			want:  dayOct10,
		},
		{
			// December rolls into the next year.
			name:  "december start rolls to january",
			day:   5,
			start: "2026-12-10",
			want:  "2027-01-05",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			day := domain.NewLastPaymentDay()
			if tt.day != 0 {
				day = domain.MustPaymentDay(tt.day)
			}
			got := day.FirstPaymentDate(date(tt.start))
			if got.Format(time.DateOnly) != tt.want {
				t.Fatalf("FirstPaymentDate = %s, want %s", got.Format(time.DateOnly), tt.want)
			}
		})
	}
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
