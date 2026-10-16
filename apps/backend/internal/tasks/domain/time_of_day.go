package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// TimeOfDay is a wall-clock time at minute precision — minutes since
// midnight. The contract carries HH:MM only (#496: время задаётся с
// точностью до минут; seconds never enter the system). Zero value is
// midnight, so the optional rule field travels as *TimeOfDay.
type TimeOfDay int

// NewTimeOfDay builds a TimeOfDay from a wall-clock hour and minute,
// rejecting out-of-day values (the day's bounds 0..23:59 are expressed
// directly through the hour/minute ranges).
func NewTimeOfDay(hour, minute int) (TimeOfDay, error) {
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, fmt.Errorf("time of day %02d:%02d out of day bounds", hour, minute)
	}
	return TimeOfDay(hour*60 + minute), nil
}

// ParseTimeOfDay parses the strict HH:MM contract form (two digits, colon,
// two digits — no seconds, no whitespace, no single-digit hours).
func ParseTimeOfDay(s string) (TimeOfDay, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 2 {
		return 0, fmt.Errorf("time of day %q is not HH:MM", s)
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("time of day %q hour: %w", s, err)
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("time of day %q minute: %w", s, err)
	}
	return NewTimeOfDay(hour, minute)
}

// String returns the HH:MM contract form.
func (t TimeOfDay) String() string {
	return fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute())
}

// Hour returns the wall-clock hour 0..23.
func (t TimeOfDay) Hour() int { return int(t) / 60 }

// Minute returns the minute within the hour 0..59.
func (t TimeOfDay) Minute() int { return int(t) % 60 }
