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
	if !slices.Equal(actions, []string{taskCompletedAction, "task.uncompleted"}) {
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

func TestClearCompletedJournal_ClearsLiveRulesWithHorizon(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwner(taskMoscowTZ)

	// Both rules stay alive: the clear removes their completed rows too,
	// raising each rule's history horizon to max(deleted due)+1 so the tick
	// never resurrects the cleared past (ADR 0051 as amended 2026-10-01).
	ruleA := h.seedRule(aug27, "", string(domain.RepeatWeekly), "Правило A")
	ruleB := h.seedRule(day10, "", string(domain.RepeatOnce), "Правило B")
	h.runTick()

	h.completeAllBut(ruleA, day17) // Aug 27, Sep 3, Sep 10 completed; the future Sep 17 stands.
	h.completeAllBut(ruleB, "")    // Today's once task completed.

	cleared, err := h.tasks.ClearCompletedJournal(h.ctx(), h.owner, h.propID)
	if err != nil {
		t.Fatalf("clear completed journal: %v", err)
	}
	if cleared != 4 {
		t.Fatalf("cleared = %d, want 4 (3 of rule A + 1 of rule B)", cleared)
	}

	// The completed section is empty; the standing future is untouched.
	for _, task := range h.loadTasks(t) {
		if task.CompletedDate != nil {
			t.Fatalf("completed row survived the clear: %+v", task)
		}
	}

	// The horizons: max(deleted due)+1 per rule — today's completion is the
	// latest of both.
	assertHorizon(t, h, ruleA, "2026-09-11")
	assertHorizon(t, h, ruleB, "2026-09-11")

	// The resurrection guard: the tick runs over the cleared state and the
	// past stays deleted — rule A keeps only its future, rule B nothing.
	h.runTick()
	if got := datesOf(h.ruleTasks(h, ruleA)); !slices.Equal(got, []string{day17}) {
		t.Fatalf("rule A tasks after clear+tick = %v, want [%s]", got, day17)
	}
	if got := h.ruleTasks(h, ruleB); len(got) != 0 {
		t.Fatalf("rule B tasks after clear+tick = %v, want none", got)
	}

	// A dated rule's edit keeps the horizon: renaming rule A must not
	// resurrect the cleared history.
	newTitle := "Правило A2"
	if _, err := h.rules.UpdateRule(h.ctx(), h.owner, h.propID, ruleA, application.UpdateRuleCommand{
		Title: &newTitle,
	}); err != nil {
		t.Fatalf("update rule A: %v", err)
	}
	assertHorizon(t, h, ruleA, "2026-09-11")
	h.runTick()
	if got := datesOf(h.ruleTasks(h, ruleA)); !slices.Equal(got, []string{day17}) {
		t.Fatalf("rule A tasks after edit+tick = %v, want [%s]", got, day17)
	}
}

func TestClearCompletedJournal_UndatedRuleDormantUntilEdit(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwner(taskMoscowTZ)
	ruleID := h.seedRule("", "", string(domain.RepeatOnce), "Без срока")
	h.runTick()
	// The undated task has no due date — complete it explicitly (the shared
	// helper skips undated rows).
	for _, task := range h.ruleTasks(h, ruleID) {
		if _, err := h.tasks.CompleteTask(h.ctx(), h.owner, h.propID, task.ID); err != nil {
			t.Fatalf("complete undated task: %v", err)
		}
	}

	cleared, err := h.tasks.ClearCompletedJournal(h.ctx(), h.owner, h.propID)
	if err != nil {
		t.Fatalf("clear completed journal: %v", err)
	}
	if cleared != 1 {
		t.Fatalf("cleared = %d, want the undated completed row", cleared)
	}

	// The dormancy marker rides the same column: the undated rule's date
	// part is meaningless, the owner's today marks the clear.
	assertHorizon(t, h, ruleID, day10)

	// The tick leaves the cleared undated rule dormant — the task does not
	// respawn out of nowhere.
	h.runTick()
	if tasks := h.ruleTasks(h, ruleID); len(tasks) != 0 {
		t.Fatalf("cleared undated task respawned: %+v", tasks)
	}

	// The rule edit resets the marker: the undated task materializes again
	// (the in-transaction tick of the edit stands it immediately).
	newTitle := "Без срока 2"
	if _, err := h.rules.UpdateRule(h.ctx(), h.owner, h.propID, ruleID, application.UpdateRuleCommand{
		Title: &newTitle,
	}); err != nil {
		t.Fatalf("update undated rule: %v", err)
	}
	if tasks := h.ruleTasks(h, ruleID); len(tasks) != 1 {
		t.Fatalf("edited undated rule has %d tasks, want the fresh one", len(tasks))
	}

	// The same holds when the edit turns the rule dated: the stale marker
	// must not survive as a bogus history horizon.
	due := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	if _, err := h.rules.UpdateRule(h.ctx(), h.owner, h.propID, ruleID, application.UpdateRuleCommand{
		DueDate: &application.DueDateUpdate{Value: &due},
	}); err != nil {
		t.Fatalf("date the undated rule: %v", err)
	}
	var horizon *string
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT history_before::text FROM task_rules WHERE id = $1`, ruleID,
	).Scan(&horizon); err != nil {
		t.Fatalf("read history_before after dating the rule: %v", err)
	}
	if horizon != nil {
		t.Fatalf("history_before after dating the rule = %s, want NULL", *horizon)
	}
}

// assertHorizon reads the rule's history horizon straight from the table and
// fails when it differs from want.
func assertHorizon(t *testing.T, h *tasksHarness, ruleID uuid.UUID, want string) {
	t.Helper()
	var got *string
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT history_before::text FROM task_rules WHERE id = $1`, ruleID,
	).Scan(&got); err != nil {
		t.Fatalf("read history_before of %s: %v", ruleID, err)
	}
	if got == nil || *got != want {
		t.Fatalf("history_before of %s = %v, want %s", ruleID, got, want)
	}
}

func TestClearCompletedJournalOwnerBook_WholeBook(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwner(taskMoscowTZ)

	// The bound slice: rule A gets deleted (its journal rows clear with the
	// deleted-rule leg), rule B stays alive (its completed row clears with
	// the live-rule leg, its horizon raised).
	ruleA := h.seedRule(aug27, "", string(domain.RepeatWeekly), "Правило A")
	ruleB := h.seedRule(day10, "", string(domain.RepeatOnce), "Правило B")
	// The property-less slice (ADR 0052) mirrors it: «Личное A» is deleted,
	// «Личное B» stays alive.
	wpDead := h.seedRuleWithoutProperty(aug27, "", string(domain.RepeatOnce), "Личное A")
	wpLive := h.seedRuleWithoutProperty(day10, "", string(domain.RepeatOnce), "Личное B")
	h.runTick()

	h.completeAllBut(ruleA, day17)
	h.completeAllBut(ruleB, "")
	for _, task := range h.loadTasksWithoutProperty(t) {
		if _, err := h.tasks.CompleteTaskWithoutProperty(h.ctx(), h.owner, task.ID); err != nil {
			t.Fatalf("complete property-less task: %v", err)
		}
	}

	// The two journal targets lose their rules; the live ones keep theirs.
	if err := h.rules.DeleteRule(h.ctx(), h.owner, h.propID, ruleA); err != nil {
		t.Fatalf("delete rule A: %v", err)
	}
	if err := h.rules.DeleteRuleWithoutProperty(h.ctx(), h.owner, wpDead); err != nil {
		t.Fatalf("delete property-less rule: %v", err)
	}

	// Rows that must survive the book-wide clear: an archived property's
	// journal (frozen data, ADR 0025) and a foreign owner's journal.
	archivedProp, foreign := h.seedSurvivingJournalRows()

	cleared, err := h.tasks.ClearCompletedJournalOwnerBook(h.ctx(), h.owner)
	if err != nil {
		t.Fatalf("clear completed journal of owner book: %v", err)
	}
	// The immediate repeat finds nothing left to remove — the empty-cleanup
	// leg of the journal contract below (no removed rows, no rows written).
	repeat, err := h.tasks.ClearCompletedJournalOwnerBook(h.ctx(), h.owner)
	if err != nil {
		t.Fatalf("repeat clear of owner book: %v", err)
	}
	assertWholeBookClearState(t, h, archivedProp, foreign, wpLive, cleared, repeat)
}

// seedSurvivingJournalRows inserts the journal rows the book-wide clear must
// not touch: one on the owner's own archived property, one in a foreign
// owner's book — plus a live rule with a completed task on the archived
// property, whose completed row and history horizon must both stay frozen
// (ADR 0025). Returns the archived property id and the foreign owner id.
func (h *tasksHarness) seedSurvivingJournalRows() (archivedProp, foreign uuid.UUID) {
	h.t.Helper()

	archivedProp = uuid.Must(uuid.NewV7())
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO properties (id, owner_id, name, type, address, status)
		 VALUES ($1, $2, 'Архив', 'apartment', 'Москва, Тверская 2', 'archived')`,
		archivedProp, h.owner); err != nil {
		h.t.Fatalf("seed archived property: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO tasks (id, owner_id, property_id, rule_id, title, completed_date)
		 VALUES ($1, $2, $3, NULL, 'Архивный журнал', $4)`,
		uuid.Must(uuid.NewV7()), h.owner, archivedProp, day10); err != nil {
		h.t.Fatalf("seed archived journal row: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO task_rules (id, owner_id, property_id, title, comment, due_date, repeat)
		 VALUES ($1, $2, $3, 'Архивное правило', NULL, $4, 'once')`,
		uuid.Must(uuid.NewV7()), h.owner, archivedProp, day10); err != nil {
		h.t.Fatalf("seed archived property rule: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO tasks (id, owner_id, property_id, rule_id, title, completed_date)
		 SELECT $1, r.owner_id, r.property_id, r.id, r.title, $2
		 FROM task_rules r WHERE r.title = 'Архивное правило'`,
		uuid.Must(uuid.NewV7()), day10); err != nil {
		h.t.Fatalf("seed archived rule's completed task: %v", err)
	}
	foreign = uuid.Must(uuid.NewV7())
	h.seedActor(foreign)
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO tasks (id, owner_id, rule_id, title, completed_date)
		 VALUES ($1, $2, NULL, 'Чужой журнал', $3)`,
		uuid.Must(uuid.NewV7()), foreign, day10); err != nil {
		h.t.Fatalf("seed foreign journal row: %v", err)
	}
	return archivedProp, foreign
}

// assertWholeBookClearState pins the post-clear state: every completed row
// of the owner's book is gone — the deleted-rule journals and the live
// rules' rows alike (the live rules' horizons raised, ADR 0051 amended) —
// while the archived and the foreign journals survive; the audit entry
// carries the removed count; the action journal carries one
// task.completed_cleared row per touched object and nothing for the
// property-less leg or the 0-removed repeat (ADR 0061 §3).
func assertWholeBookClearState(
	t *testing.T, h *tasksHarness, archivedProp, foreign, wpLive uuid.UUID, cleared, repeat int64,
) {
	t.Helper()

	if cleared != 6 {
		t.Fatalf("cleared = %d, want 6 (3 bound journal + 1 live rule B + 1 property-less journal + 1 live «Личное B»)", cleared)
	}
	if repeat != 0 {
		t.Fatalf("repeat clear = %d, want 0 (the book was just cleared)", repeat)
	}
	// No clearable journal row survived on either slice...
	countJournal(t, h, `owner_id = $1 AND completed_date IS NOT NULL AND rule_id IS NULL
		AND (property_id IS NULL OR EXISTS (
		      SELECT 1 FROM properties p
		      WHERE p.id = property_id AND p.status != 'archived'))`, h.owner,
		"clearable journal rows survived the clear", 0)

	// The live rules' completed rows are gone too — the horizons took over
	// the dedup duty.
	countJournal(t, h, `owner_id = $1 AND completed_date IS NOT NULL AND rule_id IS NOT NULL
		AND (property_id IS NULL OR EXISTS (
		      SELECT 1 FROM properties p
		      WHERE p.id = property_id AND p.status != 'archived'))`, h.owner,
		"live rules' completed rows", 0)
	// The horizons of the surviving live rules: max(deleted due)+1 — today's
	// completions of both.
	assertHorizon(t, h, ruleBOf(t, h), "2026-09-11")
	assertHorizon(t, h, wpLive, "2026-09-11")
	// The archived property's journal stays frozen — both the ownerless row
	// and the archived rule's completed one...
	countJournal(t, h, `property_id = $1`, archivedProp, "archived property journal rows", 2)
	// ...and the archived rule's horizon stays untouched (its rows were
	// never the clear's business, ADR 0025).
	var archivedHorizon *string
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT history_before::text FROM task_rules WHERE title = 'Архивное правило'`,
	).Scan(&archivedHorizon); err != nil {
		t.Fatalf("read archived rule's history_before: %v", err)
	}
	if archivedHorizon != nil {
		t.Fatalf("archived rule's history_before = %s, want NULL", *archivedHorizon)
	}
	// ...and the foreign owner's book is untouched.
	countJournal(t, h, `owner_id = $1 AND completed_date IS NOT NULL AND rule_id IS NULL`, foreign,
		"foreign owner journal rows", 1)

	// The audit entry carries the removed count (the shared clear action).
	var auditCount string
	if err := h.pool.QueryRow(h.ctx(), `
		SELECT context->>'count' FROM audit_log
		WHERE action = 'task.completed_cleared' AND actor_id = $1
		ORDER BY created_at DESC LIMIT 1`,
		h.owner).Scan(&auditCount); err != nil {
		t.Fatalf("read audit entry: %v", err)
	}
	if auditCount != "6" {
		t.Fatalf("audit count = %q, want \"6\"", auditCount)
	}

	// The action journal follows the bulk canon (ADR 0061 §3): exactly one
	// task.completed_cleared row per touched object — the one active
	// property, carrying its 4 removed rows in the count (3 journal + 1 live
	// rule B). The property-less removal anchors no row (every row anchors
	// to a property), and the 0-removed repeat wrote none.
	type clearedRow struct {
		property uuid.UUID
		count    *string
	}
	var rows []clearedRow
	clearedRows, err := h.pool.Query(h.ctx(), `
		SELECT property_id, context->>'count' FROM action_journal
		WHERE action = 'task.completed_cleared' AND actor_id = $1`,
		h.owner)
	if err != nil {
		t.Fatalf("query cleared journal rows: %v", err)
	}
	defer clearedRows.Close()
	for clearedRows.Next() {
		var row clearedRow
		if err := clearedRows.Scan(&row.property, &row.count); err != nil {
			t.Fatalf("scan cleared journal row: %v", err)
		}
		rows = append(rows, row)
	}
	if err := clearedRows.Err(); err != nil {
		t.Fatalf("iterate cleared journal rows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("task.completed_cleared journal rows = %d, want 1 (one per touched object; "+
			"property-less legs and empty clears write none)", len(rows))
	}
	if rows[0].property != h.propID {
		t.Fatalf("cleared journal row anchored to %s, want the touched property %s", rows[0].property, h.propID)
	}
	if rows[0].count == nil || *rows[0].count != "4" {
		t.Fatalf("cleared journal row count = %v, want \"4\"", rows[0].count)
	}
}

// ruleBOf finds the live bound rule by its title — the book-clear test only
// carries the variable locally, so the state assertion re-reads it by name.
func ruleBOf(t *testing.T, h *tasksHarness) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT id FROM task_rules WHERE owner_id = $1 AND property_id = $2 AND title = 'Правило B'`,
		h.owner, h.propID).Scan(&id); err != nil {
		t.Fatalf("read rule B id: %v", err)
	}
	return id
}

// countJournal counts the tasks table rows matching one raw predicate and
// fails when the count differs from want.
func countJournal(t *testing.T, h *tasksHarness, where string, arg any, label string, want int) {
	t.Helper()
	var got int
	if err := h.pool.QueryRow(h.ctx(), `SELECT count(*) FROM tasks WHERE `+where, arg).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", label, err)
	}
	if got != want {
		t.Fatalf("%s = %d, want %d", label, got, want)
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
