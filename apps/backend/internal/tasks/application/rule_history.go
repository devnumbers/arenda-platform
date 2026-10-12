package application

// Диффы правила задачи для единой строки истории (решение владельца 12.10,
// карта #1324): «Задача изменена: [название]» + по пояснению на каждое
// изменившееся поле — название, дата, время, повтор, комментарий; значения
// готовыми строками (дата точечная «19.02.2026», время «09:00», повтор
// словесностью домена), комментарий полным текстом без резки.

import (
	"time"

	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
)

// taskRuleHistoryChanges diffs the loaded rule against the patched one for
// the action journal's unified change row (ADR 0061 §5, аменда 12.10:
// пояснения несут все изменения).
func taskRuleHistoryChanges(before, after domain.TaskRule) []historydomain.AttributeChange {
	var changes []historydomain.AttributeChange
	if before.Title != after.Title {
		changes = append(changes, historydomain.AttributeChange{
			Key: "title", Label: "Название",
			Old: before.Title, New: after.Title,
		})
	}
	if !taskDateEqual(before.DueDate, after.DueDate) {
		changes = append(changes, historydomain.AttributeChange{
			Key: "due_date", Label: "Дата",
			Old: taskDateLabel(before.DueDate), New: taskDateLabel(after.DueDate),
		})
	}
	if !taskTimeEqual(before.DueTime, after.DueTime) {
		changes = append(changes, historydomain.AttributeChange{
			Key: "due_time", Label: "Время",
			Old: taskTimeLabel(before.DueTime), New: taskTimeLabel(after.DueTime),
		})
	}
	if before.Repeat != after.Repeat {
		changes = append(changes, historydomain.AttributeChange{
			Key: "repeat", Label: "Повтор",
			Old: domain.RepeatLabel(before.Repeat), New: domain.RepeatLabel(after.Repeat),
		})
	}
	if !taskCommentEqual(before.Comment, after.Comment) {
		changes = append(changes, historydomain.AttributeChange{
			Key: "comment", Label: "Комментарий",
			Old: taskCommentLabel(before.Comment), New: taskCommentLabel(after.Comment),
		})
	}
	return changes
}

// taskDateLabel renders an optional anchor date for the diff: nil (undated)
// stays nil so the line reads as an addition or a removal; a date reads
// dotted («19.02.2026»).
func taskDateLabel(t *time.Time) any {
	if t == nil {
		return nil
	}
	return historydomain.FormatDateDotted(*t)
}

// taskTimeLabel renders the optional wall-clock time («09:00»); nil stays
// nil.
func taskTimeLabel(t *domain.TimeOfDay) any {
	if t == nil {
		return nil
	}
	return t.String()
}

// taskCommentLabel renders the optional comment with its full text — the
// details are ready lines, no truncation (решение владельца 12.10).
func taskCommentLabel(c *string) any {
	if c == nil {
		return nil
	}
	return *c
}

func taskDateEqual(a, b *time.Time) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return a.Equal(*b)
	}
}

func taskTimeEqual(a, b *domain.TimeOfDay) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return *a == *b
	}
}

func taskCommentEqual(a, b *string) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return *a == *b
	}
}
