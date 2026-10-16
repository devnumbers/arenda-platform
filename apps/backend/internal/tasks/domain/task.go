package domain

import (
	"time"

	"github.com/google/uuid"
)

// Task is one occurrence of a rule at a concrete date — the unit that gets
// completed by hand (tasks/CONTEXT.md «Задача»). It carries the snapshot of
// the rule's title, comment and due fields taken at materialization: later
// rule edits change future materializations, never created tasks. CompletedDate
// replaces a status column: nil = active, a date = the completion fact;
// "overdue" and "undated" are computed, never stored.
type Task struct {
	ID      uuid.UUID
	OwnerID uuid.UUID
	// PropertyID is the task's property binding, snapshotted from the rule at
	// materialization; nil = the task without a property (ADR 0052). A bound
	// task never becomes property-less — deleting the property deletes its
	// tasks outright.
	PropertyID *uuid.UUID
	// RuleID points at the producing rule; nil after the rule's hard delete —
	// the completed journal row stays with its snapshots (resolution #496).
	RuleID *uuid.UUID
	// DueDate is nil for the undated task of an undated rule.
	DueDate *time.Time
	// DueTime is the rule's snapshot; it requires DueDate.
	DueTime *TimeOfDay
	Title   string
	Comment *string
	// CompletedDate is the completion fact; it may differ from the due date
	// and there is no "late" term (tasks/CONTEXT.md «Выполненная задача»).
	CompletedDate *time.Time
	// Repeat is a read projection of the producing rule's repeat (the
	// screen's ↻ mark): the readers LEFT JOIN the live rule, nil once the
	// rule is deleted. Never persisted on the task and not part of the
	// snapshot — writers leave it nil.
	Repeat    *RepeatKind
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewMaterializedTask builds the task of the rule at date (nil = the
// undated task of an undated rule). The rule's title, comment and due time
// are snapshotted here and never follow later edits of the rule
// (resolution #496).
func NewMaterializedTask(rule TaskRule, date *time.Time) Task {
	var ruleID uuid.UUID
	if rule.ID != uuid.Nil {
		ruleID = rule.ID
	}
	task := Task{
		OwnerID:    rule.OwnerID,
		PropertyID: cloneUUIDPtr(rule.PropertyID),
		DueDate:    date,
		DueTime:    rule.DueTime,
		Title:      rule.Title,
	}
	if ruleID != uuid.Nil {
		task.RuleID = &ruleID
	}
	if rule.Comment != nil {
		comment := *rule.Comment
		task.Comment = &comment
	}
	return task
}

// cloneUUIDPtr copies an optional UUID without aliasing the rule's value:
// the task's snapshot must not follow later mutations of the rule struct.
func cloneUUIDPtr(u *uuid.UUID) *uuid.UUID {
	if u == nil {
		return nil
	}
	return new(*u)
}
