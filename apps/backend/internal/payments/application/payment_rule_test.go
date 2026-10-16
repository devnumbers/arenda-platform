package application

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	"github.com/stretchr/testify/require"
)

// validRule is the base fixture every case mutates: a minimal rule that
// passes validateRule. The pure validator is tested here, at its own seam;
// the integration tests keep only the conveyor sentinels.
func validRule() domain.Payment {
	slug := "rent"
	return domain.Payment{
		Type:          domain.TypeExpense,
		Title:         "Аренда",
		AmountKopecks: 5000000,
		Recurrence:    domain.NewDailyRecurrence(),
		Since:         time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC),
		Category:      domain.CategoryRef{Slug: &slug},
	}
}

// windowRule is the base fixture of the window table: the same valid rule
// whose since/recurrence/endDate each case re-points. 2026-10-07 is a
// Wednesday.
func windowRule() domain.Payment {
	rule := validRule()
	rule.Since = time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	return rule
}

// TestValidateRuleWindow enforces the schedule window (ticket #1154): the
// end date never stands before the schedule's first occurrence (ignoring the
// end date itself). An end on the first occurrence itself stays valid — the
// RFC 5545 UNTIL semantics OccurrencesBetween follows.
func TestValidateRuleWindow(t *testing.T) {
	t.Parallel()

	// The «31-го числа» spelling: the contract normalizes 31 onto the
	// last-day marker (the only way the recurrence API spells it).
	monthly31st, err := domain.NewMonthlyRecurrence(nil, true)
	require.NoError(t, err)
	yearlyFeb29, err := domain.NewYearlyRecurrence(time.February, 29)
	require.NoError(t, err)
	friday, err := domain.NewWeeklyRecurrence([]time.Weekday{time.Friday})
	require.NoError(t, err)

	oct9 := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)   // The Friday.
	oct8 := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)   // The Thursday.
	feb28 := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)  // The February clamp.
	dec31 := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC) // The first year's end.
	leap29 := time.Date(2028, 2, 29, 0, 0, 0, 0, time.UTC) // The rule's own day.

	cases := []struct {
		name    string
		mutate  func(rule *domain.Payment)
		wantErr error
	}{
		{
			// The bugged pair: weekly «пт» ending the Thursday before the
			// first Friday — zero occurrences would materialize.
			name: "weekly endDate before the first friday",
			mutate: func(r *domain.Payment) {
				r.Recurrence = friday
				r.EndDate = &oct8
			},
			wantErr: ErrEndDateBeforeFirstOccurrence,
		},
		{
			name: "weekly endDate on the first friday is valid",
			mutate: func(r *domain.Payment) {
				r.Recurrence = friday
				r.EndDate = &oct9
			},
		},
		{
			// Daily's first occurrence is since itself: the window adds
			// nothing to the existing endDate ≥ since check.
			name: "daily endDate on since is valid",
			mutate: func(r *domain.Payment) {
				r.Recurrence = domain.NewDailyRecurrence()
				r.EndDate = new(r.Since)
			},
		},
		{
			name: "weekly endDate on since is the bug",
			mutate: func(r *domain.Payment) {
				r.Recurrence = friday
				r.EndDate = new(r.Since)
			},
			wantErr: ErrEndDateBeforeFirstOccurrence,
		},
		{
			// The February clamp of the 31st keeps 31.01 the first occurrence.
			name: "monthly 31st with a clamped february end",
			mutate: func(r *domain.Payment) {
				r.Recurrence = monthly31st
				r.Since = time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
				r.EndDate = &feb28
			},
		},
		{
			// The first occurrence of Feb-29 chosen after this February is
			// the next year's clamped anchor (2027-02-28).
			name: "yearly feb29 with an end inside the first year",
			mutate: func(r *domain.Payment) {
				r.Recurrence = yearlyFeb29
				r.Since = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
				r.EndDate = &dec31
			},
			wantErr: ErrEndDateBeforeFirstOccurrence,
		},
		{
			name: "yearly feb29 covered by its second year",
			mutate: func(r *domain.Payment) {
				r.Recurrence = yearlyFeb29
				r.Since = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
				r.EndDate = &leap29
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rule := windowRule()
			tc.mutate(&rule)
			err := validateRuleWindow(rule)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("validateRuleWindow = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateRuleWindow = %v, want nil", err)
			}
		})
	}
}

func TestValidateRule(t *testing.T) {
	t.Parallel()

	endDate := time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC)
	userCategory := uuid.Must(uuid.NewV7())
	cases := []struct {
		name    string
		mutate  func(rule *domain.Payment)
		wantErr bool
	}{
		{"valid rule as-is", nil, false},
		{"bad type enum", func(r *domain.Payment) { r.Type = domain.PaymentType("profit") }, true},
		{"blank title", func(r *domain.Payment) { r.Title = "   " }, true},
		{"title over 255 characters", func(r *domain.Payment) { r.Title = strings.Repeat("а", 256) }, true},
		{
			// The drift regression: the limit counts characters, not bytes —
			// 130 Cyrillic characters are 260 bytes and a valid title.
			"130-character Cyrillic title is valid", func(r *domain.Payment) { r.Title = strings.Repeat("а", 130) },
			false,
		},
		{"zero amount", func(r *domain.Payment) { r.AmountKopecks = 0 }, true},
		{"amount over 10^9", func(r *domain.Payment) { r.AmountKopecks = 1_000_000_001 }, true},
		{"negative amount", func(r *domain.Payment) { r.AmountKopecks = -5 }, true},
		{"unknown category slug", func(r *domain.Payment) { r.Category = domain.CategoryRef{Slug: new("not-a-catalog-slug")} }, true},
		{
			"user category reference is carried unchanged",
			func(r *domain.Payment) { r.Category = domain.CategoryRef{UserCategoryID: &userCategory} },
			false,
		},
		{"category neither slug nor user", func(r *domain.Payment) { r.Category = domain.CategoryRef{} }, true},
		{"zero recurrence", func(r *domain.Payment) { r.Recurrence = domain.Recurrence{} }, true},
		{"endDate before since", func(r *domain.Payment) { r.EndDate = new(r.Since.AddDate(0, 0, -1)) }, true},
		{"endDate equal to since", func(r *domain.Payment) { r.EndDate = new(r.Since) }, false},
		{"endDate after since", func(r *domain.Payment) { r.EndDate = &endDate }, false},
		// Напоминание о платеже (карта #822, #824): nil = напоминаний нет,
		// допустимы 1/3/7 — остальное некорректный ввод.
		{"reminder offset absent", nil, false},
		{"reminder offset 1 day", func(r *domain.Payment) { r.ReminderOffsetDays = new(1) }, false},
		{"reminder offset 3 days", func(r *domain.Payment) { r.ReminderOffsetDays = new(3) }, false},
		{"reminder offset 7 days", func(r *domain.Payment) { r.ReminderOffsetDays = new(7) }, false},
		{"reminder offset zero", func(r *domain.Payment) { r.ReminderOffsetDays = new(0) }, true},
		{"reminder offset 2 days", func(r *domain.Payment) { r.ReminderOffsetDays = new(2) }, true},
		{"reminder offset 8 days", func(r *domain.Payment) { r.ReminderOffsetDays = new(8) }, true},
		{"reminder offset negative", func(r *domain.Payment) { r.ReminderOffsetDays = new(-3) }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rule := validRule()
			if tc.mutate != nil {
				tc.mutate(&rule)
			}
			err := validateRule(rule)
			if gotErr, wantErr := err != nil, tc.wantErr; gotErr != wantErr {
				t.Fatalf("validateRule error = %v, want error=%v", err, wantErr)
			}
			if err != nil && !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("validateRule = %v, want ErrInvalidInput", err)
			}
		})
	}
}
