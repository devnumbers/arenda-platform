//go:build integration

package application_test

// The action journal of the tasks mutations (карта #704, тикет #707,
// ADR 0061): the property-bound conveyor journals every manual action, the
// property-less book (ADR 0052) journals nothing — a journal row needs a
// property to hang on — and the bulk journal clear records the removed
// count.

import (
	"strings"
	"testing"

	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
)

func taskJournalActions(t *testing.T, h *tasksHarness) []string {
	t.Helper()
	rows, err := h.pool.Query(h.ctx(),
		`SELECT action FROM action_journal WHERE property_id = $1 ORDER BY created_at, id`,
		h.propID,
	)
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	defer rows.Close()
	var actions []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			t.Fatalf("scan journal: %v", err)
		}
		actions = append(actions, a)
	}
	return actions
}

func TestHistory_TaskMutationsRecordRows(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwner("Europe/Moscow")

	created, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("CreateRule: %v", err)
	}

	got := taskJournalActions(t, h)
	if len(got) != 1 || got[0] != string(historydomain.ActionTaskRuleCreated) {
		t.Fatalf("journal actions = %v, want [task_rule.created]", got)
	}

	var searchable string
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT searchable FROM action_journal WHERE property_id = $1 AND action = 'task_rule.created'`,
		h.propID,
	).Scan(&searchable); err != nil {
		t.Fatalf("read journal row: %v", err)
	}
	if !strings.Contains(searchable, "Проверить счётчики") {
		t.Errorf("searchable = %q, want the rule title snapshot", searchable)
	}

	if err := h.rules.DeleteRule(h.ctx(), h.owner, h.propID, created.ID); err != nil {
		t.Fatalf("DeleteRule: %v", err)
	}
	got = taskJournalActions(t, h)
	if len(got) != 2 || got[1] != string(historydomain.ActionTaskRuleDeleted) {
		t.Fatalf("journal actions = %v, want the delete appended", got)
	}
}

func TestHistory_TaskCompletionAndJournalClear(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwner("Europe/Moscow")

	created, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	tasks := h.loadTasks(t)
	if len(tasks) == 0 {
		t.Fatal("the rule materialized no tasks")
	}
	if _, err := h.tasks.CompleteTask(h.ctx(), h.owner, h.propID, tasks[0].ID); err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}
	if _, err := h.tasks.UncompleteTask(h.ctx(), h.owner, h.propID, tasks[0].ID); err != nil {
		t.Fatalf("UncompleteTask: %v", err)
	}
	if _, err := h.tasks.CompleteTask(h.ctx(), h.owner, h.propID, tasks[0].ID); err != nil {
		t.Fatalf("CompleteTask (again): %v", err)
	}
	// The occurrence rows link the RULE (#713): the only task page is the
	// rule's edit screen, and a hard-deleted rule leaves the link off —
	// the occurrence id itself is navigable nowhere. The complete→
	// uncomplete→complete cycle leaves two task.completed rows; both carry
	// the rule link, so picking either satisfies the pin.
	for _, action := range []string{taskCompletedAction, "task.uncompleted"} {
		var linkID string
		err := h.pool.QueryRow(h.ctx(),
			`SELECT segments->1->'link'->>'id' FROM action_journal
			 WHERE property_id = $1 AND action = $2`,
			h.propID, action,
		).Scan(&linkID)
		if err != nil {
			t.Fatalf("read %s link: %v", action, err)
		}
		if linkID != created.ID.String() {
			t.Errorf("%s link id = %s, want the rule id %s (got the occurrence %s?)",
				action, linkID, created.ID, tasks[0].ID)
		}
	}
	if err := h.rules.DeleteRule(h.ctx(), h.owner, h.propID, created.ID); err != nil {
		t.Fatalf("DeleteRule: %v", err)
	}
	// The completed journal row of the deleted rule survives with its
	// snapshot; the clear removes it and journals the count.
	cleared, err := h.tasks.ClearCompletedJournal(h.ctx(), h.owner, h.propID)
	if err != nil {
		t.Fatalf("ClearCompletedJournal: %v", err)
	}
	if cleared != 1 {
		t.Fatalf("cleared = %d, want 1", cleared)
	}

	var searchable string
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT searchable FROM action_journal
		 WHERE property_id = $1 AND action = 'task.completed_cleared'`,
		h.propID,
	).Scan(&searchable); err != nil {
		t.Fatalf("read journal row: %v", err)
	}
	if !strings.Contains(searchable, "1") {
		t.Errorf("searchable = %q, want the removed count in the text", searchable)
	}
}

func TestHistory_PropertyLessRulesJournalNothing(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwnerWithoutProperty("Europe/Moscow")

	created, err := h.rules.CreateRuleWithoutProperty(h.ctx(), h.owner, h.createCmd())
	if err != nil {
		t.Fatalf("CreateRuleWithoutProperty: %v", err)
	}
	if err := h.rules.DeleteRuleWithoutProperty(h.ctx(), h.owner, created.ID); err != nil {
		t.Fatalf("DeleteRuleWithoutProperty: %v", err)
	}

	var journal int
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT count(*) FROM action_journal WHERE actor_id = $1`, h.owner,
	).Scan(&journal); err != nil {
		t.Fatalf("count journal: %v", err)
	}
	if journal != 0 {
		t.Errorf("journal rows = %d, want 0 — the property-less book has no object to journal on", journal)
	}
}
