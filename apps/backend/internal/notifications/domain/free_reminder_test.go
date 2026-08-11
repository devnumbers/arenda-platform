package domain

import (
	"errors"
	"testing"
	"time"
)

func TestParseFreeReminderPeriodicity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		want    FreeReminderPeriodicity
		wantErr bool
	}{
		{name: "once", in: "once", want: PeriodicityOnce},
		{name: "daily", in: "daily", want: PeriodicityDaily},
		{name: "weekly", in: "weekly", want: PeriodicityWeekly},
		{name: "monthly", in: "monthly", want: PeriodicityMonthly},
		{name: "yearly", in: "yearly", want: PeriodicityYearly},
		{name: "unknown", in: "hourly", wantErr: true},
		{name: "empty", in: "", wantErr: true},
		{name: "case sensitive", in: "Monthly", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseFreeReminderPeriodicity(tt.in)
			if tt.wantErr {
				if !errors.Is(err, ErrUnknownPeriodicity) {
					t.Errorf("ParseFreeReminderPeriodicity(%q) error = %v, want ErrUnknownPeriodicity", tt.in, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFreeReminderPeriodicity(%q) unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseFreeReminderPeriodicity(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFreeReminderPeriodicity_Valid(t *testing.T) {
	t.Parallel()

	valid := []FreeReminderPeriodicity{
		PeriodicityOnce, PeriodicityDaily, PeriodicityWeekly, PeriodicityMonthly, PeriodicityYearly,
	}
	invalid := []FreeReminderPeriodicity{"", "hourly", "MONTHLY", " fortnightly"}

	for _, p := range valid {
		if !p.Valid() {
			t.Errorf("Valid() = false for %q, want true", p)
		}
	}
	for _, p := range invalid {
		if p.Valid() {
			t.Errorf("Valid() = true for %q, want false", p)
		}
	}
}

func TestExpandFreeReminderOccurrences(t *testing.T) {
	t.Parallel()

	// Helpers to build pointers for wantFirst/wantLast comparisons.
	ptr := func(t time.Time) *time.Time { return &t }

	tests := []struct {
		name      string
		reminder  FreeReminder
		from      time.Time
		to        time.Time
		wantCount int // -1 means "don't check count"; checked with len(got)
		wantFirst *time.Time
		wantLast  *time.Time
	}{
		// --- 1. All five periodicities ---
		{
			name: "once single occurrence inside window",
			reminder: FreeReminder{
				TriggerAt:   time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC),
				Periodicity: PeriodicityOnce,
			},
			from:      time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			to:        time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			wantCount: 1,
			wantFirst: ptr(time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)),
		},
		{
			name: "daily yields one per day across a week",
			reminder: FreeReminder{
				TriggerAt:   time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC),
				Periodicity: PeriodicityDaily,
			},
			from:      time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			to:        time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC), // 7 days half-open
			wantCount: 7,
			wantFirst: ptr(time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)),
			wantLast:  ptr(time.Date(2026, 6, 7, 9, 0, 0, 0, time.UTC)),
		},
		{
			name: "weekly yields one per week across a month",
			reminder: FreeReminder{
				TriggerAt:   time.Date(2026, 6, 2, 9, 0, 0, 0, time.UTC), // Tuesday
				Periodicity: PeriodicityWeekly,
			},
			from:      time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			to:        time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			wantCount: 5, // Jun 2, 9, 16, 23, 30 (all Tuesdays, all < Jul 1)
			wantFirst: ptr(time.Date(2026, 6, 2, 9, 0, 0, 0, time.UTC)),
			wantLast:  ptr(time.Date(2026, 6, 30, 9, 0, 0, 0, time.UTC)),
		},
		{
			name: "monthly yields one per month across half a year",
			reminder: FreeReminder{
				TriggerAt:   time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC),
				Periodicity: PeriodicityMonthly,
			},
			from:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			to:        time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			wantCount: 6, // Jan 15 .. Jun 15
			wantFirst: ptr(time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)),
			wantLast:  ptr(time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)),
		},
		{
			name: "yearly yields one per year across three years",
			reminder: FreeReminder{
				TriggerAt:   time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC),
				Periodicity: PeriodicityYearly,
			},
			from:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			to:        time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			wantCount: 3, // 2024, 2025, 2026
			wantFirst: ptr(time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC)),
			wantLast:  ptr(time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)),
		},

		// --- 2. Window boundaries ---
		{
			name: "occurrence exactly on from is included",
			reminder: FreeReminder{
				TriggerAt:   time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
				Periodicity: PeriodicityOnce,
			},
			from:      time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
			to:        time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC),
			wantCount: 1,
			wantFirst: ptr(time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)),
		},
		{
			name: "occurrence exactly on to is excluded",
			reminder: FreeReminder{
				TriggerAt:   time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
				Periodicity: PeriodicityOnce,
			},
			from:      time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
			to:        time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC), // == occurrence
			wantCount: 0,
		},
		{
			name: "zero-length window from==to returns nil",
			reminder: FreeReminder{
				TriggerAt:   time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
				Periodicity: PeriodicityDaily,
			},
			from:      time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
			to:        time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
			wantCount: 0,
		},
		{
			name: "inverted window to<from returns nil",
			reminder: FreeReminder{
				TriggerAt:   time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
				Periodicity: PeriodicityDaily,
			},
			from:      time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC),
			to:        time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
			wantCount: 0,
		},

		// --- 3 & 4. once outside / inside the window ---
		{
			name: "once outside window before from returns nothing",
			reminder: FreeReminder{
				TriggerAt:   time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC),
				Periodicity: PeriodicityOnce,
			},
			from:      time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
			to:        time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC),
			wantCount: 0,
		},
		{
			name: "once outside window after to returns nothing",
			reminder: FreeReminder{
				TriggerAt:   time.Date(2026, 6, 25, 9, 0, 0, 0, time.UTC),
				Periodicity: PeriodicityOnce,
			},
			from:      time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
			to:        time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC),
			wantCount: 0,
		},
		{
			name: "once inside window returns exactly one",
			reminder: FreeReminder{
				TriggerAt:   time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC),
				Periodicity: PeriodicityOnce,
			},
			from:      time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
			to:        time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC),
			wantCount: 1,
			wantFirst: ptr(time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)),
		},

		// --- 5. Monthly end-of-month clamping ---
		{
			// Jan 31 + 1 month -> Feb 28 (2026 is not a leap year).
			name: "monthly Jan 31 clamps to Feb 28 non-leap",
			reminder: FreeReminder{
				TriggerAt:   time.Date(2026, 1, 31, 9, 0, 0, 0, time.UTC),
				Periodicity: PeriodicityMonthly,
			},
			from:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			to:        time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			wantCount: 2, // Jan 31, Feb 28
			wantFirst: ptr(time.Date(2026, 1, 31, 9, 0, 0, 0, time.UTC)),
			wantLast:  ptr(time.Date(2026, 2, 28, 9, 0, 0, 0, time.UTC)),
		},
		{
			// Mar 31 + 1 month -> Apr 30.
			name: "monthly Mar 31 clamps to Apr 30",
			reminder: FreeReminder{
				TriggerAt:   time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC),
				Periodicity: PeriodicityMonthly,
			},
			from:      time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			to:        time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
			wantCount: 2, // Mar 31, Apr 30
			wantFirst: ptr(time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC)),
			wantLast:  ptr(time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)),
		},

		// --- 6. Yearly leap-day clamping ---
		{
			// Feb 29 2024 (leap) + 1 year -> Feb 28 2025 (non-leap).
			name: "yearly Feb 29 leap clamps to Feb 28 non-leap",
			reminder: FreeReminder{
				TriggerAt:   time.Date(2024, 2, 29, 9, 0, 0, 0, time.UTC),
				Periodicity: PeriodicityYearly,
			},
			from:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			to:        time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			wantCount: 2, // 2024-02-29, 2025-02-28
			wantFirst: ptr(time.Date(2024, 2, 29, 9, 0, 0, 0, time.UTC)),
			wantLast:  ptr(time.Date(2025, 2, 28, 9, 0, 0, 0, time.UTC)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExpandFreeReminderOccurrences(tt.reminder, tt.from, tt.to)

			if tt.wantCount >= 0 {
				if len(got) != tt.wantCount {
					t.Fatalf("count = %d, want %d (occurrences: %v)", len(got), tt.wantCount, got)
				}
			}
			if tt.wantFirst != nil {
				if len(got) == 0 {
					t.Fatalf("wantFirst %v but got no occurrences", *tt.wantFirst)
				}
				if !got[0].Equal(*tt.wantFirst) {
					t.Errorf("first = %v, want %v", got[0], *tt.wantFirst)
				}
			}
			if tt.wantLast != nil {
				if len(got) == 0 {
					t.Fatalf("wantLast %v but got no occurrences", *tt.wantLast)
				}
				if !got[len(got)-1].Equal(*tt.wantLast) {
					t.Errorf("last = %v, want %v", got[len(got)-1], *tt.wantLast)
				}
			}
		})
	}
}

func TestExpandFreeReminderOccurrences_DailyTriggerFarInPastFastForwards(t *testing.T) {
	t.Parallel()

	// A daily reminder that started years ago. The window is a single day far in
	// the future. fast-forward must skip the intervening thousands of days
	// without generating them, and emit only the one occurrence inside [from, to).
	reminder := FreeReminder{
		TriggerAt:   time.Date(2020, 1, 1, 9, 0, 0, 0, time.UTC),
		Periodicity: PeriodicityDaily,
	}
	from := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC)

	got := ExpandFreeReminderOccurrences(reminder, from, to)
	if len(got) != 1 {
		t.Fatalf("count = %d, want 1 (occurrences: %v)", len(got), got)
	}
	want := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	if !got[0].Equal(want) {
		t.Errorf("occurrence = %v, want %v", got[0], want)
	}
}

func TestExpandFreeReminderOccurrences_Cap(t *testing.T) {
	t.Parallel()

	// A daily reminder with a window spanning many years must not exceed the
	// 2500-occurrence cap and must not hang or panic.
	reminder := FreeReminder{
		TriggerAt:   time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		Periodicity: PeriodicityDaily,
	}
	from := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)

	done := make(chan []time.Time, 1)
	go func() {
		done <- ExpandFreeReminderOccurrences(reminder, from, to)
	}()

	select {
	case got := <-done:
		if len(got) > expandFreeReminderLimit {
			t.Errorf("count = %d, want <= %d", len(got), expandFreeReminderLimit)
		}
		if len(got) == 0 {
			t.Errorf("count = 0, expected some occurrences before hitting the cap")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ExpandFreeReminderOccurrences did not return within 5s (possible infinite loop)")
	}
}

func TestExpandFreeReminderOccurrences_WindowAfterFirstSkip(t *testing.T) {
	t.Parallel()

	// The window starts after the first occurrence; the first occurrence is
	// skipped, but subsequent ones inside the window are emitted.
	reminder := FreeReminder{
		TriggerAt:   time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC),
		Periodicity: PeriodicityDaily,
	}
	from := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC) // skip Jan 1 and Jan 2
	to := time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC)   // emit Jan 3, 4, 5

	got := ExpandFreeReminderOccurrences(reminder, from, to)
	if len(got) != 3 {
		t.Fatalf("count = %d, want 3 (occurrences: %v)", len(got), got)
	}
	wantFirst := time.Date(2026, 1, 3, 9, 0, 0, 0, time.UTC)
	wantLast := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	if !got[0].Equal(wantFirst) {
		t.Errorf("first = %v, want %v", got[0], wantFirst)
	}
	if !got[len(got)-1].Equal(wantLast) {
		t.Errorf("last = %v, want %v", got[len(got)-1], wantLast)
	}
}
