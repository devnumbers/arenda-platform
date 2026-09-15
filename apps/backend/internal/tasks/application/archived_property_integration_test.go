//go:build integration

package application_test

// The archived-property immutability audit (ticket #618, карта #611): every
// property-bound tasks mutation on an archived property is the read-only
// state's 409 (tasksapp.ErrArchivedProperty) — the conveyor's property lock
// is the single guard — and the tick skips the archived rules while still
// materializing the owner's active ones. Reads stay open.

import (
	"testing"

	"github.com/google/uuid"
	tasksapp "github.com/nambers/arenda-planform/apps/backend/internal/tasks/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedRuleOn inserts a rule on an arbitrary property of the harness owner —
// the seedRule variant for the second-property setups (the tick test's
// archived twin).
func (h *tasksHarness) seedRuleOn(t *testing.T, propertyID uuid.UUID, anchor, repeat, title string) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO task_rules (id, owner_id, property_id, title, comment, due_date, due_time, repeat)
		 VALUES ($1, $2, $3, $4, NULL, $5, NULL, $6)`,
		id, h.owner, propertyID, title, anchor, repeat,
	); err != nil {
		t.Fatalf("seed task rule: %v", err)
	}
	return id
}

// seedArchivedProperty inserts an archived property of the harness owner and
// returns its id.
func (h *tasksHarness) seedArchivedProperty(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO properties (id, owner_id, name, type, address, status)
		 VALUES ($1, $2, 'Архив', 'apartment', 'Москва, Тверская 2', 'archived')`,
		id, h.owner,
	); err != nil {
		t.Fatalf("seed archived property: %v", err)
	}
	return id
}

// archiveProperty flips the harness property into the archive the way the
// lifecycle use case leaves it.
func (h *tasksHarness) archiveProperty(t *testing.T, propertyID uuid.UUID) {
	t.Helper()
	if _, err := h.pool.Exec(h.ctx(),
		`UPDATE properties SET status = 'archived' WHERE id = $1`, propertyID,
	); err != nil {
		t.Fatalf("archive property: %v", err)
	}
}

// loadTasksOfProperty reads one property's tasks straight from the table —
// loadTasks generalized over the property (the same projection).
func (h *tasksHarness) loadTasksOfProperty(t *testing.T, propertyID uuid.UUID) []taskRow {
	t.Helper()
	rows, err := h.pool.Query(h.ctx(), `
		SELECT id, rule_id, due_date::text, due_time::text, title, completed_date::text
		FROM tasks WHERE property_id = $1
		ORDER BY created_at, id`, propertyID)
	if err != nil {
		t.Fatalf("query tasks: %v", err)
	}
	defer rows.Close()

	out := []taskRow{}
	for rows.Next() {
		var r taskRow
		if err := rows.Scan(&r.ID, &r.RuleID, &r.DueDate, &r.DueTime, &r.Title, &r.CompletedDate); err != nil {
			t.Fatalf("scan task: %v", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate tasks: %v", err)
	}
	return out
}

// The flat run (no subtests): the six use cases assert against one fixture's
// states — the rule created and materialized while active, then the property
// archived once and every mutation tried against the frozen state.
func TestArchivedProperty_RejectsEveryTasksMutation(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t).withOwner(taskMoscowTZ)

	ruleID := h.seedRule(day17, "", string(domain.RepeatWeekly), "Проверить счётчики")
	h.runTick()
	tasks := h.loadTasks(t)
	require.Len(t, tasks, 1, "the rule stands its single future before the archive")
	taskID := tasks[0].ID

	h.archiveProperty(t, h.propID)

	_, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, h.createCmd())
	require.ErrorIs(t, err, tasksapp.ErrArchivedProperty, "rule create")

	_, err = h.rules.UpdateRule(h.ctx(), h.owner, h.propID, ruleID, tasksapp.UpdateRuleCommand{})
	require.ErrorIs(t, err, tasksapp.ErrArchivedProperty, "rule update")

	require.ErrorIs(t, h.rules.DeleteRule(h.ctx(), h.owner, h.propID, ruleID),
		tasksapp.ErrArchivedProperty, "rule delete")

	_, err = h.tasks.CompleteTask(h.ctx(), h.owner, h.propID, taskID)
	require.ErrorIs(t, err, tasksapp.ErrArchivedProperty, "complete")

	// The task is active, so past the lock the verdict would be
	// ErrNotCompleted — the archived lock wins first.
	_, err = h.tasks.UncompleteTask(h.ctx(), h.owner, h.propID, taskID)
	require.ErrorIs(t, err, tasksapp.ErrArchivedProperty, "uncomplete")

	_, err = h.tasks.ClearCompletedJournal(h.ctx(), h.owner, h.propID)
	require.ErrorIs(t, err, tasksapp.ErrArchivedProperty, "clear completed journal")

	// Nothing changed: the rule stands, the task stays active.
	after := h.loadTasks(t)
	require.Len(t, after, 1)
	assert.Nil(t, after[0].CompletedDate, "the task stays active")

	// Reads stay open on the archived object.
	_, err = h.rules.GetRule(h.ctx(), h.owner, h.propID, ruleID)
	require.NoError(t, err)
	_, err = h.tasks.ListTasks(h.ctx(), h.owner, h.propID, tasksapp.TasksListQuery{})
	require.NoError(t, err)
}

func TestTick_SkipsArchivedPropertyRules(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t).withOwner(taskMoscowTZ)

	archivedProp := h.seedArchivedProperty(t)
	archivedRule := h.seedRuleOn(t, archivedProp, day17, string(domain.RepeatWeekly), "Архивная")
	activeRule := h.seedRule(day17, "", string(domain.RepeatWeekly), "Активная")

	h.runTick()

	active := h.loadTasks(t)
	require.Len(t, active, 1, "the owner's active rules still materialize")
	assert.Equal(t, activeRule, *active[0].RuleID)

	archived := h.loadTasksOfProperty(t, archivedProp)
	assert.Empty(t, archived, "the archived property's rules stay frozen")

	// The frozen rule survives for the unarchive day untouched.
	var count int
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT count(*) FROM task_rules WHERE id = $1`, archivedRule,
	).Scan(&count); err != nil {
		t.Fatalf("count archived rules: %v", err)
	}
	assert.Equal(t, 1, count)
}
