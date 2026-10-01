package application

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
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
