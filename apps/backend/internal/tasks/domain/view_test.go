package domain

import (
	"testing"
	"time"
)

// moscowLocal is the current moment in the data owner's location with
// seconds truncated — the contract of the read-side overdue computation
// (#496). The year and month are fixed by the shared September 2026
// fixtures.
func moscowLocal(day, hour, minute int) time.Time {
	return time.Date(2026, time.September, day, hour, minute, 0, 0, moscow)
}

var moscow = mustLocation("Europe/Moscow")

// dueAt1513 is the 15:13 due time of the shared fixtures.
var dueAt1513 = TimeOfDay(15*60 + 13)

func TestIsOverdue_TimeTaskFromTheDueMinuteInclusive(t *testing.T) {
	t.Parallel()

	due := d(2026, time.September, 10)
	task := Task{DueDate: &due, DueTime: &dueAt1513}

	if IsOverdue(task, moscowLocal(10, 15, 12)) {
		t.Fatal("15:12 must not be overdue")
	}
	if !IsOverdue(task, moscowLocal(10, 15, 13)) {
		t.Fatal("15:13 is the due minute — overdue inclusive")
	}
	if !IsOverdue(task, moscowLocal(11, 9, 0)) {
		t.Fatal("next day must be overdue")
	}
}

func TestIsOverdue_DateOnlyFromTheEndOfDay(t *testing.T) {
	t.Parallel()

	due := d(2026, time.September, 10)
	task := Task{DueDate: &due}

	if IsOverdue(task, moscowLocal(10, 23, 59)) {
		t.Fatal("23:59 of the due day is still not overdue")
	}
	if !IsOverdue(task, moscowLocal(11, 0, 0)) {
		t.Fatal("overdue from the end of the due day (next midnight)")
	}
}

func TestIsOverdue_CompletedAndUndatedNeverOverdue(t *testing.T) {
	t.Parallel()

	completed := d(2026, time.September, 1)
	dated := Task{DueDate: new(d(2026, time.September, 1)), CompletedDate: &completed}
	if IsOverdue(dated, moscowLocal(30, 12, 0)) {
		t.Fatal("completed task is never overdue")
	}

	if IsOverdue(Task{}, moscowLocal(30, 12, 0)) {
		t.Fatal("undated task is never overdue")
	}
}

func TestViewStatus_Buckets(t *testing.T) {
	t.Parallel()

	due := d(2026, time.September, 10)
	completedDate := d(2026, time.September, 9)

	tests := []struct {
		name string
		task Task
		now  time.Time
		want TaskViewStatus
	}{
		{
			"completed wins over everything",
			Task{DueDate: &due, DueTime: &dueAt1513, CompletedDate: &completedDate},
			moscowLocal(30, 12, 0),
			ViewCompleted,
		},
		{
			"undated active",
			Task{},
			moscowLocal(10, 12, 0),
			ViewUndated,
		},
		{
			"dated before the due moment is active",
			Task{DueDate: &due, DueTime: &dueAt1513},
			moscowLocal(10, 15, 12),
			ViewActive,
		},
		{
			"dated at the due moment is overdue",
			Task{DueDate: &due, DueTime: &dueAt1513},
			moscowLocal(10, 15, 13),
			ViewOverdue,
		},
		{
			"future date is active",
			Task{DueDate: new(d(2026, time.September, 17))},
			moscowLocal(10, 23, 59),
			ViewActive,
		},
		{
			"date-only today stays active all day",
			Task{DueDate: &due},
			moscowLocal(10, 23, 59),
			ViewActive,
		},
		{
			"date-only turns overdue next midnight",
			Task{DueDate: &due},
			moscowLocal(11, 0, 0),
			ViewOverdue,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ViewStatus(tt.task, tt.now); got != tt.want {
				t.Fatalf("ViewStatus = %q, want %q", got, tt.want)
			}
		})
	}
}
