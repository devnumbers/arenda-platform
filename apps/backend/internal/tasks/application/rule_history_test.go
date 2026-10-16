package application

// Диффы правила задачи для пояснений единой строки (решение владельца
// 12.10, карта #1324): название, дата точечной датой, время HH:MM, повтор
// словесностью домена, комментарий полным текстом без резки.

import (
	"testing"
	"time"

	"github.com/google/uuid"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
)

func ruleHistoryTestRule(mutate func(r *domain.TaskRule)) domain.TaskRule {
	r := domain.TaskRule{
		ID:     uuid.Must(uuid.NewV7()),
		Title:  "Поливка",
		Repeat: domain.RepeatMonthly,
	}
	if mutate != nil {
		mutate(&r)
	}
	return r
}

func ruleDate(s string) *time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return &t
}

func ruleTime(h, m int) *domain.TimeOfDay {
	t, err := domain.NewTimeOfDay(h, m)
	if err != nil {
		panic(err)
	}
	return &t
}

func TestTaskRuleHistoryChanges_CarriesEveryMovedField(t *testing.T) {
	t.Parallel()
	before := ruleHistoryTestRule(func(r *domain.TaskRule) {
		r.DueDate = ruleDate("2026-02-19")
		r.DueTime = ruleTime(9, 0)
		r.Comment = new("старый комментарий")
	})
	after := ruleHistoryTestRule(func(r *domain.TaskRule) {
		r.Title = "Поливка цветов"
		r.DueDate = ruleDate("2027-03-20")
		r.DueTime = ruleTime(10, 30)
		r.Repeat = domain.RepeatWeekly
		r.Comment = new("новый, куда более длинный комментарий без всякой резки")
	})

	changes := taskRuleHistoryChanges(before, after)
	if len(changes) != 5 {
		t.Fatalf("changes = %d, want 5 (title, date, time, repeat, comment)", len(changes))
	}
	entry := historydomain.TaskRuleUpdated(after.ID, after.Title, changes)
	want := historydomain.DetailLines{
		`Название: «Поливка» → «Поливка цветов»`,
		`Дата: «19.02.2026» → «20.03.2027»`,
		`Время: «09:00» → «10:30»`,
		`Повтор: «Ежемесячно» → «Еженедельно»`,
		`Комментарий: «старый комментарий» → «новый, куда более длинный комментарий без всякой резки»`,
	}
	if len(entry.Details) != len(want) {
		t.Fatalf("details = %q, want %q", entry.Details, want)
	}
	for i := range want {
		if entry.Details[i] != want[i] {
			t.Errorf("details[%d] = %q, want %q", i, entry.Details[i], want[i])
		}
	}
}

func TestTaskRuleHistoryChanges_OptionalFieldEdges(t *testing.T) {
	t.Parallel()
	// Дата заводится («Дата: «20.03.2027»»), время убирается («Время:
	// удалено»), комментарий стирается («Комментарий: удалено»).
	before := ruleHistoryTestRule(func(r *domain.TaskRule) {
		r.DueTime = ruleTime(9, 0)
		r.Comment = new("стираем")
	})
	after := ruleHistoryTestRule(func(r *domain.TaskRule) {
		r.DueDate = ruleDate("2027-03-20")
	})

	changes := taskRuleHistoryChanges(before, after)
	if len(changes) != 3 {
		t.Fatalf("changes = %d, want 3 (date, time, comment)", len(changes))
	}
	entry := historydomain.TaskRuleUpdated(after.ID, after.Title, changes)
	want := historydomain.DetailLines{
		`Дата: «20.03.2027»`,
		"Время: удалено",
		"Комментарий: удалено",
	}
	if len(entry.Details) != len(want) {
		t.Fatalf("details = %q, want %q", entry.Details, want)
	}
	for i := range want {
		if entry.Details[i] != want[i] {
			t.Errorf("details[%d] = %q, want %q", i, entry.Details[i], want[i])
		}
	}
}

func TestTaskRuleHistoryChanges_EqualRulesDiffToNil(t *testing.T) {
	t.Parallel()
	if changes := taskRuleHistoryChanges(ruleHistoryTestRule(nil), ruleHistoryTestRule(nil)); changes != nil {
		t.Errorf("changes = %v, want nil", changes)
	}
}
