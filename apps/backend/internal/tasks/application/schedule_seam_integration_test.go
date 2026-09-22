//go:build integration

package application_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	auditpg "github.com/nambers/arenda-planform/apps/backend/internal/audit/adapters/postgres"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	pgdb "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	taskspg "github.com/nambers/arenda-planform/apps/backend/internal/tasks/adapters/postgres"
	tasksapp "github.com/nambers/arenda-planform/apps/backend/internal/tasks/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The scheduling seam's handover (issue #775): a rule create/edit hands the
// standing uncompleted tasks' ids to the notifications seam strictly after
// the commit, so a task born between the hourly passes with its term before
// the next one books its due-minute job at once.

// errSeamDown is the scheduling seam's failure double.
var errSeamDown = errors.New("seam down")

// captureLog collects the slog records' messages — the best-effort log's
// assertion double («отказ — best-effort, лог»).
type captureLog struct{ messages []string }

func (l *captureLog) Enabled(context.Context, slog.Level) bool { return true }

func (l *captureLog) Handle(_ context.Context, r slog.Record) error {
	l.messages = append(l.messages, r.Message)
	return nil
}

func (l *captureLog) WithAttrs([]slog.Attr) slog.Handler { return l }

func (l *captureLog) WithGroup(string) slog.Handler { return l }

// CreateRule hands over the first materialization's standing tasks — the
// today's occurrence and the single future one, both ids exactly as the
// tasks table holds them after the in-transaction tick.
func TestScheduleSeam_CreateRuleHandsOverStandingTasks(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwner(taskMoscowTZ).withSeam(&fakeSeam{})

	rule, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, h.createCmd())
	require.NoError(t, err)

	require.Len(t, h.seam.handovers, 1, "one handover per rule create")
	handed := h.seam.handovers[0]
	standing := uncompletedIDs(h, rule.ID)
	assert.ElementsMatch(t, standing, handed)
	assert.Len(t, handed, 2, "today's task plus the single future")
}

// A rule with a due time hands over the same way — the timed task is the
// case the seam exists for; the term-vs-now decision itself is
// notifications' business (the tasks side hands ids only).
func TestScheduleSeam_CreateTimedRuleHandsOver(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwner(taskMoscowTZ).withSeam(&fakeSeam{})

	due := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	at, err := domain.NewTimeOfDay(15, 13)
	require.NoError(t, err)
	rule, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, tasksapp.CreateRuleCommand{
		Title:   "Показ квартиры",
		DueDate: &due,
		DueTime: &at,
		Repeat:  domain.RepeatOnce,
	})
	require.NoError(t, err)

	require.Len(t, h.seam.handovers, 1)
	assert.ElementsMatch(t, uncompletedIDs(h, rule.ID), h.seam.handovers[0])
}

// The property-less create (ADR 0052) hands over through the owner mutation
// the same way.
func TestScheduleSeam_CreateWithoutPropertyHandsOver(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwnerWithoutProperty(taskMoscowTZ).withSeam(&fakeSeam{})

	rule, err := h.rules.CreateRuleWithoutProperty(h.ctx(), h.owner, withoutPropertyCreateCmd())
	require.NoError(t, err)

	require.Len(t, h.seam.handovers, 1)
	assert.ElementsMatch(t, uncompletedIDs(h, rule.ID), h.seam.handovers[0])
}

// An edit invalidates the not-yet-due tasks and re-materializes the single
// future: the handover carries the NEW standing ids — the moved date's task
// in, the invalidated future's id out.
func TestScheduleSeam_EditRuleHandsOverNewStanding(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwner(taskMoscowTZ).withSeam(&fakeSeam{})

	rule, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, h.createCmd())
	require.NoError(t, err)
	oldFuture := standingTaskDates(h, rule.ID)[day17]
	require.NotEqual(t, uuid.Nil, oldFuture, "the single future stands on the next weekly day")

	newDate := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	updated, err := h.rules.UpdateRule(h.ctx(), h.owner, h.propID, rule.ID,
		tasksapp.UpdateRuleCommand{DueDate: &tasksapp.DueDateUpdate{Value: &newDate}})
	require.NoError(t, err)

	require.Len(t, h.seam.handovers, 2, "one handover per rule create, one per edit")
	handed := h.seam.handovers[1]
	assert.ElementsMatch(t, uncompletedIDs(h, updated.ID), handed)
	assert.NotContains(t, handed, oldFuture, "the invalidated future's id stays out")
	assert.Contains(t, handed, standingTaskDates(h, updated.ID)[day24], "the moved date's task is in")
}

// A create whose handover fails still commits: the seam is best-effort —
// the failure is logged, the rule and its tasks stand, the hourly scan
// remains the backstop.
func TestScheduleSeam_SeamFailureDoesNotFailMutation(t *testing.T) {
	t.Parallel()

	log := &captureLog{}
	h := newTasksHarness(t).withOwner(taskMoscowTZ).withRuleLog(log).withSeam(&fakeSeam{err: errSeamDown})

	rule, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, h.createCmd())
	require.NoError(t, err, "a broken seam must never fail the committed rule")

	stored, err := h.rules.GetRule(h.ctx(), h.owner, h.propID, rule.ID)
	require.NoError(t, err)
	assert.Equal(t, rule.ID, stored.ID)
	assert.Empty(t, h.seam.handovers)
	assert.Contains(t, log.messages, "notify materialized tasks failed", "the failure is logged away")
}

// No seam wired — the pre-#775 silence: the create succeeds, nothing is
// dispatched, nothing panics.
func TestScheduleSeam_NilSeamKeepsSilence(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwner(taskMoscowTZ)

	_, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, h.createCmd())
	require.NoError(t, err)
}

// withRuleLog rebuilds the rule service over a fresh factory with the given
// logger — the best-effort log's assertions door.
func (h *tasksHarness) withRuleLog(log *captureLog) *tasksHarness {
	h.t.Helper()
	factory := tasksapp.NewTxStoreFactory(
		taskspg.NewTickStore(h.pool),
		taskspg.NewRuleStore(h.pool),
		taskspg.NewTaskStore(h.pool),
		taskspg.NewPropertyStore(h.pool),
		auditapp.NewService(auditpg.NewWriter(h.pool), h.clock),
		historyRecorder(h.pool),
		pgdb.NewUoW(h.pool, slog.New(log)),
	)
	h.rules = tasksapp.NewRuleService(factory, taskspg.NewOwnerClock(h.pool, h.clock), nil, slog.New(log))
	return h
}

// uncompletedIDs reads the rule's standing uncompleted tasks' ids straight
// from the table — the handover's expected shape.
func uncompletedIDs(h *tasksHarness, ruleID uuid.UUID) []uuid.UUID {
	h.t.Helper()
	rows, err := h.pool.Query(h.ctx(),
		`SELECT id FROM tasks WHERE rule_id = $1 AND completed_date IS NULL ORDER BY id`, ruleID)
	if err != nil {
		h.t.Fatalf("query standing tasks: %v", err)
	}
	defer rows.Close()
	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			h.t.Fatalf("scan standing task: %v", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		h.t.Fatalf("iterate standing tasks: %v", err)
	}
	return ids
}

// standingTaskDates maps the rule's standing uncompleted tasks by due date
// ("") for the undated one — the handover inspected by date.
func standingTaskDates(h *tasksHarness, ruleID uuid.UUID) map[string]uuid.UUID {
	h.t.Helper()
	byID := make(map[uuid.UUID]taskRow, 2)
	for _, row := range h.loadTasks(h.t) {
		byID[row.ID] = row
	}
	out := map[string]uuid.UUID{}
	for _, id := range uncompletedIDs(h, ruleID) {
		row := byID[id]
		date := ""
		if row.DueDate != nil {
			date = *row.DueDate
		}
		out[date] = id
	}
	return out
}
