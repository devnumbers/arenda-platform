package domain

import "time"

// TaskViewStatus is the computed read-side bucket of one task. Not stored:
// "overdue" and "undated" are calculated on every read (tasks/CONTEXT.md
// «Просрочка», «Без срока»), and "active"/"completed" split on the stored
// completion fact.
type TaskViewStatus string

// The four view buckets; the list sections of the tasks screen map onto them
// one to one (Просроченные → overdue, Сегодня/даты → active, Без даты →
// undated, Выполненные → completed).
const (
	ViewActive    TaskViewStatus = "active"
	ViewOverdue   TaskViewStatus = "overdue"
	ViewUndated   TaskViewStatus = "undated"
	ViewCompleted TaskViewStatus = "completed"
)

// IsOverdue reports whether the task's due moment has passed. The now
// parameter is the current moment in the data owner's location with seconds
// truncated to minutes (resolution #496: minute precision, seconds never
// participate):
//
//   - a task with a time is overdue from the due minute inclusive (15:13 →
//     overdue at 15:13);
//   - a date-only task is overdue from the end of the due day — the next
//     midnight (resolution #496);
//   - completed and undated tasks are never overdue.
func IsOverdue(t Task, now time.Time) bool {
	if t.CompletedDate != nil || t.DueDate == nil {
		return false
	}
	due := *t.DueDate
	if t.DueTime != nil {
		moment := time.Date(due.Year(), due.Month(), due.Day(), t.DueTime.Hour(), t.DueTime.Minute(), 0, 0, now.Location())
		return !now.Before(moment)
	}
	// Date-only: the due moment is the end of the due day.
	end := time.Date(due.Year(), due.Month(), due.Day()+1, 0, 0, 0, 0, now.Location())
	return !now.Before(end)
}

// ViewStatus computes the read-side bucket of one task against the same
// owner-local, minute-truncated moment contract as IsOverdue.
func ViewStatus(t Task, now time.Time) TaskViewStatus {
	if t.CompletedDate != nil {
		return ViewCompleted
	}
	if t.DueDate == nil {
		return ViewUndated
	}
	if IsOverdue(t, now) {
		return ViewOverdue
	}
	return ViewActive
}
