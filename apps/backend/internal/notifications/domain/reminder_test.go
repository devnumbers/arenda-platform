package domain

import (
	"errors"
	"testing"
	"time"
)

// mustLoc loads a timezone, failing the test if it cannot be resolved. The
// notifications domain is meaningless without IANA timezone data, so a load
// failure is always fatal.
func mustLoc(tb testing.TB, name string) *time.Location {
	tb.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		tb.Fatalf("time.LoadLocation(%q): %v", name, err)
	}
	return loc
}

func TestScheduledAtForDate(t *testing.T) {
	t.Parallel()

	// All inputs use the default dispatch hour unless the case overrides it.
	const defaultHour = 10

	moscow := mustLoc(t, "Europe/Moscow")
	yekaterinburg := mustLoc(t, "Asia/Yekaterinburg")
	vladivostok := mustLoc(t, "Asia/Vladivostok")
	newYork := mustLoc(t, "America/New_York")
	berlin := mustLoc(t, "Europe/Berlin")

	tests := []struct {
		name string
		date time.Time
		loc  *time.Location
		hour int
		want time.Time // always in UTC
	}{
		// --- Fixed-offset Russian timezones (no DST) ---
		{
			name: "Europe/Moscow UTC+3 hour 10 -> 07:00Z",
			date: time.Date(2026, 3, 15, 12, 0, 0, 0, moscow),
			loc:  moscow,
			hour: defaultHour,
			want: time.Date(2026, 3, 15, 7, 0, 0, 0, time.UTC),
		},
		{
			name: "Asia/Yekaterinburg UTC+5 hour 10 -> 05:00Z",
			date: time.Date(2026, 3, 15, 12, 0, 0, 0, yekaterinburg),
			loc:  yekaterinburg,
			hour: defaultHour,
			want: time.Date(2026, 3, 15, 5, 0, 0, 0, time.UTC),
		},
		{
			name: "Asia/Vladivostok UTC+10 hour 10 -> 00:00Z same day",
			date: time.Date(2026, 3, 15, 12, 0, 0, 0, vladivostok),
			loc:  vladivostok,
			hour: defaultHour,
			want: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		},

		// --- America/New_York: DST boundary (spring forward 2026-03-08 02:00) ---
		{
			name: "America/New_York winter EST UTC-5 hour 10 -> 15:00Z",
			date: time.Date(2026, 1, 15, 12, 0, 0, 0, newYork),
			loc:  newYork,
			hour: defaultHour,
			want: time.Date(2026, 1, 15, 15, 0, 0, 0, time.UTC),
		},
		{
			name: "America/New_York summer EDT UTC-4 hour 10 -> 14:00Z",
			date: time.Date(2026, 7, 15, 12, 0, 0, 0, newYork),
			loc:  newYork,
			hour: defaultHour,
			want: time.Date(2026, 7, 15, 14, 0, 0, 0, time.UTC),
		},
		{
			name: "America/New_York day before spring forward (Mar 7) still EST -> 15:00Z",
			date: time.Date(2026, 3, 7, 12, 0, 0, 0, newYork),
			loc:  newYork,
			hour: defaultHour,
			want: time.Date(2026, 3, 7, 15, 0, 0, 0, time.UTC),
		},
		{
			name: "America/New_York day of spring forward (Mar 8) now EDT -> 14:00Z",
			date: time.Date(2026, 3, 8, 12, 0, 0, 0, newYork),
			loc:  newYork,
			hour: defaultHour,
			want: time.Date(2026, 3, 8, 14, 0, 0, 0, time.UTC),
		},

		// --- Europe/Berlin: CET/CEST DST ---
		{
			name: "Europe/Berlin winter CET UTC+1 hour 10 -> 09:00Z",
			date: time.Date(2026, 1, 15, 12, 0, 0, 0, berlin),
			loc:  berlin,
			hour: defaultHour,
			want: time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC),
		},
		{
			name: "Europe/Berlin summer CEST UTC+2 hour 10 -> 08:00Z",
			date: time.Date(2026, 7, 15, 12, 0, 0, 0, berlin),
			loc:  berlin,
			hour: defaultHour,
			want: time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC),
		},

		// --- Different dispatch hours (Europe/Moscow UTC+3) ---
		{
			name: "Europe/Moscow midnight hour 0 -> previous day 21:00Z",
			date: time.Date(2026, 3, 15, 12, 0, 0, 0, moscow),
			loc:  moscow,
			hour: 0,
			want: time.Date(2026, 3, 14, 21, 0, 0, 0, time.UTC),
		},
		{
			name: "Europe/Moscow late evening hour 23 -> same day 20:00Z",
			date: time.Date(2026, 3, 15, 12, 0, 0, 0, moscow),
			loc:  moscow,
			hour: 23,
			want: time.Date(2026, 3, 15, 20, 0, 0, 0, time.UTC),
		},
		{
			name: "Europe/Moscow default hour 10 -> 07:00Z",
			date: time.Date(2026, 3, 15, 12, 0, 0, 0, moscow),
			loc:  moscow,
			hour: 10,
			want: time.Date(2026, 3, 15, 7, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ScheduledAtForDate(tt.date, tt.loc, tt.hour)
			if !got.Equal(tt.want) {
				t.Errorf("ScheduledAtForDate() = %v (UTC %s), want %v (UTC %s)",
					got, got.Format(time.RFC3339), tt.want, tt.want.Format(time.RFC3339))
			}
			// The result is documented to be a UTC instant.
			if got.Location() != time.UTC {
				t.Errorf("ScheduledAtForDate() location = %v, want UTC", got.Location())
			}
		})
	}
}

func TestScheduledAtForDate_NoOpForDefaultTimezone(t *testing.T) {
	t.Parallel()

	// Issue #112: "пересчёт — no-op для дефолта". The default owner timezone is
	// Europe/Moscow; the canonical dispatch instant (10:00 MSK == 07:00 UTC)
	// must be stable across the refactor.
	moscow := mustLoc(t, "Europe/Moscow")
	date := time.Date(2026, 3, 15, 12, 0, 0, 0, moscow)

	got := ScheduledAtForDate(date, moscow, 10)
	want := time.Date(2026, 3, 15, 7, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("ScheduledAtForDate(default) = %v, want %v (no-op for Europe/Moscow)", got, want)
	}
}

func TestScheduledAtForDate_ReinterpretsInputInLocation(t *testing.T) {
	t.Parallel()

	// The function re-interprets the calendar date of its input in the given
	// location, ignoring the input's own location/timezone offset. Several
	// distinct UTC instants all resolve to the Moscow calendar date
	// 2026-03-15; dispatched at hour 10 in Moscow they must all yield the same
	// 07:00Z instant.
	moscow := mustLoc(t, "Europe/Moscow")

	want := time.Date(2026, 3, 15, 7, 0, 0, 0, time.UTC)

	// Each of these falls on March 15 when viewed from Europe/Moscow (UTC+3):
	//   - 2026-03-14T22:00Z == 2026-03-15T01:00 MSK (just past Moscow midnight)
	//   - 2026-03-15T12:00Z == 2026-03-15T15:00 MSK (afternoon)
	//   - 2026-03-15T20:59Z == 2026-03-15T23:59 MSK (just before next day)
	for _, in := range []time.Time{
		time.Date(2026, 3, 14, 22, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 15, 20, 59, 0, 0, time.UTC),
	} {
		got := ScheduledAtForDate(in, moscow, 10)
		if !got.Equal(want) {
			t.Errorf("ScheduledAtForDate(input=%s) = %v, want %v", in.Format(time.RFC3339), got, want)
		}
	}
}

func TestReminderOffset(t *testing.T) {
	t.Parallel()

	utc := time.UTC
	moscow := mustLoc(t, "Europe/Moscow")

	tests := []struct {
		name          string
		operationDate time.Time
		reminderDate  time.Time
		loc           *time.Location
		want          int
	}{
		{
			name:          "operation 10 days after reminder",
			operationDate: time.Date(2026, 3, 20, 12, 0, 0, 0, moscow),
			reminderDate:  time.Date(2026, 3, 10, 9, 0, 0, 0, moscow),
			loc:           moscow,
			want:          10,
		},
		{
			name:          "operation 3 days after reminder",
			operationDate: time.Date(2026, 3, 13, 12, 0, 0, 0, moscow),
			reminderDate:  time.Date(2026, 3, 10, 9, 0, 0, 0, moscow),
			loc:           moscow,
			want:          3,
		},
		{
			name:          "same calendar day offset zero",
			operationDate: time.Date(2026, 3, 10, 23, 59, 0, 0, moscow),
			reminderDate:  time.Date(2026, 3, 10, 0, 1, 0, 0, moscow),
			loc:           moscow,
			want:          0,
		},
		{
			name:          "reminder after operation negative offset",
			operationDate: time.Date(2026, 3, 7, 12, 0, 0, 0, moscow),
			reminderDate:  time.Date(2026, 3, 10, 9, 0, 0, 0, moscow),
			loc:           moscow,
			want:          -3,
		},
		{
			name:          "UTC interpretation",
			operationDate: time.Date(2026, 3, 20, 12, 0, 0, 0, utc),
			reminderDate:  time.Date(2026, 3, 10, 0, 0, 0, 0, utc),
			loc:           utc,
			want:          10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ReminderOffset(tt.operationDate, tt.reminderDate, tt.loc)
			if got != tt.want {
				t.Errorf("ReminderOffset() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestReminderOffset_TimezoneShiftsDayBoundary(t *testing.T) {
	t.Parallel()

	// A single UTC instant can fall on different calendar days depending on the
	// timezone. 2026-03-15T22:00:00Z is March 15 in UTC, but already March 16
	// 08:00 in Asia/Vladivostok (UTC+10). Compared against a reminder on March
	// 5, the offset differs by one day between the two interpretations.
	vladivostok := mustLoc(t, "Asia/Vladivostok")
	operation := time.Date(2026, 3, 15, 22, 0, 0, 0, time.UTC)
	reminder := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)

	if got := ReminderOffset(operation, reminder, time.UTC); got != 10 {
		t.Errorf("ReminderOffset UTC = %d, want 10 (March 15 - March 5)", got)
	}
	if got := ReminderOffset(operation, reminder, vladivostok); got != 11 {
		t.Errorf("ReminderOffset Vladivostok = %d, want 11 (March 16 - March 5)", got)
	}
}

func TestValidateReminderDate(t *testing.T) {
	t.Parallel()

	utc := time.UTC
	moscow := mustLoc(t, "Europe/Moscow")
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, utc)

	tests := []struct {
		name         string
		reminderDate time.Time
		now          time.Time
		loc          *time.Location
		wantErr      bool
	}{
		{
			name:         "today is valid",
			reminderDate: time.Date(2026, 3, 15, 9, 0, 0, 0, moscow),
			now:          now,
			loc:          moscow,
			wantErr:      false,
		},
		{
			name:         "tomorrow is valid",
			reminderDate: time.Date(2026, 3, 16, 9, 0, 0, 0, moscow),
			now:          now,
			loc:          moscow,
			wantErr:      false,
		},
		{
			name:         "yesterday is invalid",
			reminderDate: time.Date(2026, 3, 14, 9, 0, 0, 0, moscow),
			now:          now,
			loc:          moscow,
			wantErr:      true,
		},
		{
			name:         "far future is valid",
			reminderDate: time.Date(2027, 1, 1, 0, 0, 0, 0, moscow),
			now:          now,
			loc:          moscow,
			wantErr:      false,
		},
		{
			name:         "same day in UTC",
			reminderDate: time.Date(2026, 3, 15, 23, 0, 0, 0, utc),
			now:          now,
			loc:          utc,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateReminderDate(tt.reminderDate, tt.now, tt.loc)
			if tt.wantErr && !errors.Is(err, ErrInvalidReminderDate) {
				t.Errorf("ValidateReminderDate() error = %v, want ErrInvalidReminderDate", err)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateReminderDate() error = %v, want nil", err)
			}
		})
	}
}

func TestValidateReminderDate_TimezoneShiftsTodayBoundary(t *testing.T) {
	t.Parallel()

	// now = 2026-03-15T23:30:00Z. In UTC it is still March 15; in
	// Asia/Vladivostok (UTC+10) it is already March 16 09:30. The acceptance of
	// a reminder date therefore depends on the owner's timezone: "сегодня" is
	// interpreted in the owner's timezone.
	vladivostok := mustLoc(t, "Asia/Vladivostok")
	now := time.Date(2026, 3, 15, 23, 30, 0, 0, time.UTC)

	// March 16 with loc=UTC: today is March 15 UTC, so March 16 is the future.
	if err := ValidateReminderDate(time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC), now, time.UTC); err != nil {
		t.Errorf("March 16 in UTC: error = %v, want nil (future day)", err)
	}

	// March 16 with loc=Vladivostok: today is already March 16 there, and
	// today is allowed.
	if err := ValidateReminderDate(time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC), now, vladivostok); err != nil {
		t.Errorf("March 16 in Vladivostok: error = %v, want nil (today allowed)", err)
	}

	// March 15 with loc=Vladivostok: today is March 16 there, so March 15 is
	// yesterday and must be rejected.
	if err := ValidateReminderDate(time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), now, vladivostok); !errors.Is(err, ErrInvalidReminderDate) {
		t.Errorf("March 15 in Vladivostok: error = %v, want ErrInvalidReminderDate (yesterday)", err)
	}

	// March 15 with loc=UTC: today is March 15 UTC, so March 15 is today and
	// allowed.
	if err := ValidateReminderDate(time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), now, time.UTC); err != nil {
		t.Errorf("March 15 in UTC: error = %v, want nil (today allowed)", err)
	}
}
