package application

import (
	"strings"
	"testing"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
)

var testToday = time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)

func validRule() domain.TaskRule {
	due := testToday
	title := "Полить цветы"
	return domain.TaskRule{
		Title:   title,
		DueDate: &due,
		Repeat:  domain.RepeatOnce,
	}
}

func TestValidateRule_Accepts(t *testing.T) {
	t.Parallel()

	tods := mustTimeOfDay(t, "15:13")
	comment := "Комментарий"
	tests := []struct {
		name string
		rule domain.TaskRule
	}{
		{"once dated", validRule()},
		{"once dated with time", func() domain.TaskRule {
			rule := validRule()
			rule.DueTime = tods
			return rule
		}()},
		{"undated once", domain.TaskRule{Title: "Разобрать кладовку", Repeat: domain.RepeatOnce}},
		{"daily dated", func() domain.TaskRule {
			rule := validRule()
			rule.Repeat = domain.RepeatDaily
			return rule
		}()},
		{"monthly dated", func() domain.TaskRule {
			rule := validRule()
			rule.Repeat = domain.RepeatMonthly
			return rule
		}()},
		{"with comment", func() domain.TaskRule {
			rule := validRule()
			rule.Comment = &comment
			return rule
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := validateRule(tt.rule); err != nil {
				t.Fatalf("validateRule(%+v) = %v, want nil", tt.rule, err)
			}
		})
	}
}

func TestValidateRule_Rejects(t *testing.T) {
	t.Parallel()

	tods := mustTimeOfDay(t, "09:00")
	tests := []struct {
		name string
		rule domain.TaskRule
	}{
		{"empty title", domain.TaskRule{Repeat: domain.RepeatOnce}},
		{"blank title", func() domain.TaskRule {
			rule := validRule()
			rule.Title = "   "
			return rule
		}()},
		{"title over 255 runes", func() domain.TaskRule {
			rule := validRule()
			rule.Title = strings.Repeat("ж", 256)
			return rule
		}()},
		{"comment over 1000 runes", func() domain.TaskRule {
			rule := validRule()
			comment := strings.Repeat("ж", 1001)
			rule.Comment = &comment
			return rule
		}()},
		{"unknown repeat", func() domain.TaskRule {
			rule := validRule()
			rule.Repeat = domain.RepeatKind("sometimes")
			return rule
		}()},
		{"empty repeat", func() domain.TaskRule {
			rule := validRule()
			rule.Repeat = ""
			return rule
		}()},
		{"time without date", func() domain.TaskRule {
			rule := domain.TaskRule{Title: "Без даты со временем", Repeat: domain.RepeatOnce}
			rule.DueTime = tods
			return rule
		}()},
		{"recurring without anchor", func() domain.TaskRule {
			return domain.TaskRule{Title: "Каждый день без даты", Repeat: domain.RepeatDaily}
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := validateRule(tt.rule); err == nil {
				t.Fatalf("validateRule(%+v) = nil, want ErrInvalidInput", tt.rule)
			}
		})
	}
}

func TestValidateAnchor(t *testing.T) {
	t.Parallel()

	due := testToday
	if err := validateAnchor(&due, testToday); err != nil {
		t.Fatalf("today anchor must pass: %v", err)
	}
	future := testToday.AddDate(0, 0, 1)
	if err := validateAnchor(&future, testToday); err != nil {
		t.Fatalf("future anchor must pass: %v", err)
	}
	past := testToday.AddDate(0, 0, -1)
	if err := validateAnchor(&past, testToday); err == nil {
		t.Fatal("backdated anchor must be rejected")
	}
	if err := validateAnchor(nil, testToday); err != nil {
		t.Fatalf("undated anchor must pass: %v", err)
	}
}

func mustTimeOfDay(t *testing.T, s string) *domain.TimeOfDay {
	t.Helper()
	tod, err := domain.ParseTimeOfDay(s)
	if err != nil {
		t.Fatalf("parse time of day: %v", err)
	}
	return &tod
}
