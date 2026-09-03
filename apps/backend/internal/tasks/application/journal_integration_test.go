//go:build integration

package application_test

// The completion and journal family (ADR 0051): the completion toggle
// semantics, the hard rule deletion (the tombstone seam — uncompleted
// tasks die, the completed journal survives with snapshots), and the
// «Удалить все выполненные» scoping.

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
)

func TestCompleteAndUncomplete_Semantics(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwner(taskMoscowTZ)
	rule, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}
	tasks := h.ruleTasks(h, rule.ID)
	if len(tasks) == 0 || tasks[0].DueDate == nil || *tasks[0].DueDate != day10 {
		t.Fatalf("today's task missing: %+v", tasks)
	}
	taskID := tasks[0].ID

	// Complete: the fact is stamped with the owner's today.
	completed, err := h.tasks.CompleteTask(h.ctx(), h.owner, h.propID, taskID)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if completed.CompletedDate == nil || completed.CompletedDate.Format("2006-01-02") != day10 {
		t.Fatalf("completed_date = %v, want %s", completed.CompletedDate, day10)
	}
	// A repeated completion is a 409.
	if _, err := h.tasks.CompleteTask(h.ctx(), h.owner, h.propID, taskID); !errors.Is(err, application.ErrAlreadyCompleted) {
		t.Fatalf("repeated complete = %v, want ErrAlreadyCompleted", err)
	}

	// Uncomplete returns the task to the active ones.
	reopened, err := h.tasks.UncompleteTask(h.ctx(), h.owner, h.propID, taskID)
	if err != nil {
		t.Fatalf("uncomplete: %v", err)
	}
	if reopened.CompletedDate != nil {
		t.Fatalf("completed_date after uncomplete = %v, want nil", reopened.CompletedDate)
	}
	// Uncompleting an active task is a 409.
	if _, err := h.tasks.UncompleteTask(h.ctx(), h.owner, h.propID, taskID); !errors.Is(err, application.ErrNotCompleted) {
		t.Fatalf("uncomplete of active = %v, want ErrNotCompleted", err)
	}

	actions := h.auditActions(t, "task", taskID)
	if !slices.Equal(actions, []string{"task.completed", "task.uncompleted"}) {
		t.Fatalf("audit = %v, want [task.completed task.uncompleted]", actions)
	}
}

func TestDeleteRule_KillsUncompletedKeepsCompletedJournal(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwner(taskMoscowTZ)
	ruleID := h.seedRule(aug27, "", string(domain.RepeatWeekly), "Вынести мусор")
	h.runTick()

	// Complete every due occurrence (Aug 27, Sep 3, Sep 10): they form the
	// journal; the future Sep 17 stays active.
	h.completeAllBut(ruleID, day17)

	if err := h.rules.DeleteRule(h.ctx(), h.owner, h.propID, ruleID); err != nil {
		t.Fatalf("delete rule: %v", err)
	}

	// The journal rows lost their rule_id (SET NULL) — read them by the fact.
	var journal []taskRow
	for _, task := range h.loadTasks(t) {
		if task.CompletedDate != nil && task.RuleID == nil {
			journal = append(journal, task)
		}
	}
	if len(journal) != 3 {
		t.Fatalf("journal after rule deletion = %d rows, want the 3 completed", len(journal))
	}
	for _, task := range journal {
		if task.Title != "Вынести мусор" {
			t.Fatalf("snapshot lost: %q", task.Title)
		}
	}
	// The read projection follows the rule's fate: journal rows of the
	// deleted rule read back with no repeat (the wire's null ↻).
	journalPage, err := h.tasks.ListTasks(h.ctx(), h.owner, h.propID, application.TasksListQuery{Completed: true})
	if err != nil {
		t.Fatalf("list journal: %v", err)
	}
	for _, item := range journalPage.Items {
		if item.Task.Repeat != nil {
			t.Fatalf("journal row of the deleted rule carries repeat %q, want nil", *item.Task.Repeat)
		}
	}

	// The journal row is read-only: uncompleting it is ErrRuleDeleted.
	if _, err := h.tasks.UncompleteTask(h.ctx(), h.owner, h.propID, journal[0].ID); !errors.Is(err, application.ErrRuleDeleted) {
		t.Fatalf("uncomplete of journal row = %v, want ErrRuleDeleted", err)
	}

	actions := h.auditActions(t, "task_rule", ruleID)
	if len(actions) != 1 || actions[0] != "task_rule.deleted" {
		t.Fatalf("rule audit = %v, want [task_rule.deleted]", actions)
	}
}

func TestClearCompletedJournal_ScopedToDeletedRules(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwner(taskMoscowTZ)

	// Rule A will be deleted (its journal is the clear's target); rule B
	// stays alive (its completed task must survive the clear).
	ruleA := h.seedRule(aug27, "", string(domain.RepeatWeekly), "Правило A")
	ruleB := h.seedRule(day10, "", string(domain.RepeatOnce), "Правило B")
	h.runTick()

	h.completeAllBut(ruleA, day17)
	h.completeAllBut(ruleB, "")

	if err := h.rules.DeleteRule(h.ctx(), h.owner, h.propID, ruleA); err != nil {
		t.Fatalf("delete rule A: %v", err)
	}

	cleared, err := h.tasks.ClearCompletedJournal(h.ctx(), h.owner, h.propID)
	if err != nil {
		t.Fatalf("clear completed journal: %v", err)
	}
	// Rule A (weekly) completed three past occurrences before deletion.
	if cleared != 3 {
		t.Fatalf("cleared = %d, want the 3 journal rows of rule A", cleared)
	}

	// No ownerless completed row survived the clear...
	for _, task := range h.loadTasks(t) {
		if task.CompletedDate != nil && task.RuleID == nil {
			t.Fatalf("journal row of the deleted rule survived the clear: %+v", task)
		}
	}
	// ...while the live rule B keeps its completed row (the dedup key).
	var liveB int
	for _, task := range h.ruleTasks(h, ruleB) {
		if task.CompletedDate != nil {
			liveB++
		}
	}
	if liveB != 1 {
		t.Fatalf("live rule B completed rows = %d, want 1 (the dedup key)", liveB)
	}
}

func TestListTasks_BucketsAndTotal(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwner(taskMoscowTZ)
	ruleID := h.seedRule(aug27, "", string(domain.RepeatWeekly), "Вынести мусор")
	h.runTick()

	// Complete the two past occurrences.
	h.completeAllBut(ruleID, day10, day17)

	active, err := h.tasks.ListTasks(h.ctx(), h.owner, h.propID, application.TasksListQuery{})
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if active.Total != 2 || len(active.Items) != 2 {
		t.Fatalf("active page = %d items / %d total, want 2/2", len(active.Items), active.Total)
	}
	if !active.Today.Equal(time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("today = %v, want 2026-09-10", active.Today)
	}
	assertActiveBuckets(t, active.Items)
	// The live rule's repeat rides along as the read projection (the wire's
	// ↻ mark).
	for _, item := range active.Items {
		if item.Task.Repeat == nil || *item.Task.Repeat != domain.RepeatWeekly {
			t.Fatalf("repeat projection = %v, want weekly", item.Task.Repeat)
		}
	}

	completed, err := h.tasks.ListTasks(h.ctx(), h.owner, h.propID, application.TasksListQuery{Completed: true})
	if err != nil {
		t.Fatalf("list completed: %v", err)
	}
	if completed.Total != 2 || len(completed.Items) != 2 {
		t.Fatalf("completed page = %d items / %d total, want 2/2", len(completed.Items), completed.Total)
	}
	for _, item := range completed.Items {
		if item.Status != domain.ViewCompleted {
			t.Fatalf("completed bucket = %q", item.Status)
		}
		if item.Task.Repeat == nil || *item.Task.Repeat != domain.RepeatWeekly {
			t.Fatalf("completed repeat projection = %v, want weekly", item.Task.Repeat)
		}
	}
}

// completeAllBut completes every due task of the rule except the given due
// dates (the shared completion helper of the journal family).
func (h *tasksHarness) completeAllBut(ruleID uuid.UUID, keep ...string) {
	h.t.Helper()
	for _, task := range h.ruleTasks(h, ruleID) {
		if task.DueDate == nil || slices.Contains(keep, *task.DueDate) {
			continue
		}
		if _, err := h.tasks.CompleteTask(h.ctx(), h.owner, h.propID, task.ID); err != nil {
			h.t.Fatalf("complete %s: %v", *task.DueDate, err)
		}
	}
}

// assertActiveBuckets pins the section buckets of the active page: the
// undated one, today's date-only task (active until the day ends) and the
// future one.
func assertActiveBuckets(t *testing.T, items []application.TaskListItem) {
	t.Helper()
	for _, item := range items {
		if item.Task.DueDate == nil {
			if item.Status != domain.ViewUndated {
				t.Fatalf("undated bucket = %q", item.Status)
			}
			continue
		}
		date := item.Task.DueDate.Format(time.DateOnly)
		if date != day10 && date != day17 {
			t.Fatalf("unexpected active task at %s", date)
		}
		if item.Status != domain.ViewActive {
			t.Fatalf("task at %s = %q, want active", date, item.Status)
		}
	}
}
