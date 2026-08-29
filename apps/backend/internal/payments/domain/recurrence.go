package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"
)

// RecurrenceKind discriminates the recurrence variants of a payment rule.
type RecurrenceKind string

// The four recurrence kinds (ADR 0047; the one-shot kind was removed from the
// prototype model — a one-off fact is a manually entered operation only).
const (
	RecurrenceDaily   RecurrenceKind = "daily"
	RecurrenceWeekly  RecurrenceKind = "weekly"
	RecurrenceMonthly RecurrenceKind = "monthly"
	RecurrenceYearly  RecurrenceKind = "yearly"
)

// Recurrence is the schedule anchor of a payment rule (prototype decision №17,
// revision of №1): the anchor lives in the recurrence itself — selected
// weekdays, day of month, or month-and-day — while the lower bound of
// generation is the rule's Since date. There is no first-payment date on the
// rule, and no shifting onto business days.
//
// The zero value is not a valid recurrence; the constructors are the only way
// to build one, so an invalid variant is unrepresentable.
type Recurrence struct {
	kind        RecurrenceKind
	weekdays    []time.Weekday // weekly: sorted unique, 0=Sunday..6=Saturday
	daysOfMonth []int          // monthly: sorted unique 1..30
	lastDay     bool           // monthly: additionally the last day of each month
	month       time.Month     // yearly: 1..12
	day         int            // yearly: 1..31, Feb 29 clamps to Feb 28
}

// NewDailyRecurrence returns the every-day recurrence.
func NewDailyRecurrence() Recurrence {
	return Recurrence{kind: RecurrenceDaily}
}

// NewWeeklyRecurrence returns the weekly recurrence for the selected weekdays
// (0=Sunday..6=Saturday, matching time.Weekday). At least one day is required;
// duplicates are removed and the set is sorted.
func NewWeeklyRecurrence(weekdays []time.Weekday) (Recurrence, error) {
	if len(weekdays) == 0 {
		return Recurrence{}, errors.New("weekly recurrence needs at least one weekday, got none")
	}
	if len(weekdays) > 7 {
		return Recurrence{}, fmt.Errorf("weekly recurrence allows at most 7 weekdays, got %d", len(weekdays))
	}
	set := slices.Clone(weekdays)
	slices.Sort(set)
	if unique := slices.Compact(set); len(unique) != len(weekdays) {
		return Recurrence{}, fmt.Errorf("weekly recurrence weekdays must be unique, got %v", weekdays)
	}
	return Recurrence{kind: RecurrenceWeekly, weekdays: set}, nil
}

// NewMonthlyRecurrence returns the monthly recurrence for the selected days
// of month and/or the last-day marker. Days are 1..30: in shorter months a
// day clamps to the month's last day without the schedule drifting (prototype
// decisions №1/№10), and the actual last day is expressed by lastDay rather
// than a 31st entry. At least one day or the marker is required; duplicates
// are removed and the set is sorted.
func NewMonthlyRecurrence(daysOfMonth []int, lastDay bool) (Recurrence, error) {
	if len(daysOfMonth) == 0 && !lastDay {
		return Recurrence{}, errors.New("monthly recurrence needs at least one day of month or the last-day marker")
	}
	if len(daysOfMonth) > 30 {
		return Recurrence{}, fmt.Errorf("monthly recurrence allows at most 30 days of month, got %d", len(daysOfMonth))
	}
	set := slices.Clone(daysOfMonth)
	slices.Sort(set)
	for _, d := range set {
		if d < 1 || d > 30 {
			return Recurrence{}, fmt.Errorf("monthly recurrence day of month must be 1..30, got %d", d)
		}
	}
	if unique := slices.Compact(set); len(unique) != len(daysOfMonth) {
		return Recurrence{}, fmt.Errorf("monthly recurrence days of month must be unique, got %v", daysOfMonth)
	}
	return Recurrence{kind: RecurrenceMonthly, daysOfMonth: set, lastDay: lastDay}, nil
}

// NewYearlyRecurrence returns the yearly recurrence for the given month and
// day. February 29 clamps to the last day of February in non-leap years.
func NewYearlyRecurrence(month time.Month, day int) (Recurrence, error) {
	if month < time.January || month > time.December {
		return Recurrence{}, fmt.Errorf("yearly recurrence month must be 1..12, got %d", month)
	}
	if day < 1 || day > 31 {
		return Recurrence{}, fmt.Errorf("yearly recurrence day must be 1..31, got %d", day)
	}
	return Recurrence{kind: RecurrenceYearly, month: month, day: day}, nil
}

// Kind reports the recurrence variant.
func (r *Recurrence) Kind() RecurrenceKind { return r.kind }

// Weekdays returns the weekday set of a weekly recurrence (sorted unique,
// 0=Sunday..6=Saturday). The slice is shared — callers must not mutate it.
func (r *Recurrence) Weekdays() []time.Weekday { return r.weekdays }

// DaysOfMonth returns the day-of-month set of a monthly recurrence (sorted
// unique 1..30). The slice is shared — callers must not mutate it.
func (r *Recurrence) DaysOfMonth() []int { return r.daysOfMonth }

// LastDay reports whether a monthly recurrence also fires on each month's
// actual last day.
func (r *Recurrence) LastDay() bool { return r.lastDay }

// Month returns the anchor month of a yearly recurrence.
func (r *Recurrence) Month() time.Month { return r.month }

// Day returns the anchor day of a yearly recurrence (1..31).
func (r *Recurrence) Day() int { return r.day }

// recurrenceJSON is the wire shape of Recurrence for the payments.recurrence
// jsonb column (and the OpenAPI oneOf by kind). Weekdays encode as 0..6.
// DayOfMonth is the pre-multi-day monthly shape: kept for reading stored rows
// (31 meant the clamped last day), never written.
type recurrenceJSON struct {
	Kind        RecurrenceKind `json:"kind"`
	Weekdays    []int          `json:"weekdays,omitempty"`
	DayOfMonth  int            `json:"dayOfMonth,omitempty"`
	DaysOfMonth []int          `json:"daysOfMonth,omitempty"`
	LastDay     bool           `json:"lastDay,omitempty"`
	Month       int            `json:"month,omitempty"`
	Day         int            `json:"day,omitempty"`
}

// MarshalJSON encodes the recurrence in its canonical per-kind shape.
func (r *Recurrence) MarshalJSON() ([]byte, error) {
	out := recurrenceJSON{Kind: r.kind}
	switch r.kind {
	case RecurrenceWeekly:
		out.Weekdays = make([]int, len(r.weekdays))
		for i, wd := range r.weekdays {
			out.Weekdays[i] = int(wd)
		}
	case RecurrenceMonthly:
		out.DaysOfMonth = r.daysOfMonth
		out.LastDay = r.lastDay
	case RecurrenceYearly:
		out.Month = int(r.month)
		out.Day = r.day
	case RecurrenceDaily:
	default:
		return nil, fmt.Errorf("marshal recurrence: unknown kind %q", r.kind)
	}
	return json.Marshal(out)
}

// UnmarshalJSON decodes a recurrence and validates it through the domain
// constructors, so a stored row can never resurrect an invalid variant.
func (r *Recurrence) UnmarshalJSON(data []byte) error {
	var in recurrenceJSON
	if err := json.Unmarshal(data, &in); err != nil {
		return fmt.Errorf("unmarshal recurrence: %w", err)
	}
	var rec Recurrence
	var err error
	switch in.Kind {
	case RecurrenceDaily:
		rec = NewDailyRecurrence()
	case RecurrenceWeekly:
		weekdays := make([]time.Weekday, len(in.Weekdays))
		for i, wd := range in.Weekdays {
			if wd < 0 || wd > 6 {
				return fmt.Errorf("unmarshal recurrence: weekday must be 0..6, got %d", wd)
			}
			weekdays[i] = time.Weekday(wd)
		}
		rec, err = NewWeeklyRecurrence(weekdays)
	case RecurrenceMonthly:
		// Legacy stored rows carry a single dayOfMonth (1..31, where 31
		// meant the clamped last day) — convert instead of dropping them.
		if len(in.DaysOfMonth) == 0 && !in.LastDay && in.DayOfMonth != 0 {
			if in.DayOfMonth == 31 {
				rec, err = NewMonthlyRecurrence(nil, true)
			} else {
				rec, err = NewMonthlyRecurrence([]int{in.DayOfMonth}, false)
			}
		} else {
			rec, err = NewMonthlyRecurrence(in.DaysOfMonth, in.LastDay)
		}
	case RecurrenceYearly:
		rec, err = NewYearlyRecurrence(time.Month(in.Month), in.Day)
	default:
		return fmt.Errorf("unmarshal recurrence: unknown kind %q", in.Kind)
	}
	if err != nil {
		return fmt.Errorf("unmarshal recurrence: %w", err)
	}
	*r = rec
	return nil
}
