//go:build integration

package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errSeamDown is the scheduling seam's failure double.
var errSeamDown = errors.New("seam down")

// The scheduling seam's handover (issue #775): a rule create/edit hands the
// standing uncompleted tasks' ids to the notifications seam strictly after
// the commit, so a task born between the hourly passes with its term before
// the next one books its due-minute job at once.

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
	rule, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, application.CreateRuleCommand{
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
	require.Len(t, h.seam.handovers, 1)
	oldFuture := futureTaskID(h, rule.ID, day10)
	require.NotNil(t, oldFuture)

	newDate := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	updated, err := h.rules.UpdateRule(h.ctx(), h.owner, h.propID, rule.ID,
		application.UpdateRuleCommand{DueDate: &application.DueDateUpdate{Value: &newDate}})
	require.NoError(t, err)

	require.Len(t, h.seam.handovers, 2, "one handover per rule create, one per edit")
	handed := h.seam.handovers[1]
	assert.ElementsMatch(t, uncompletedIDs(h, updated.ID), handed)
	assert.NotContains(t, handed, *oldFuture, "the invalidated future's id stays out")
	assert.Contains(t, handed, standingOn(h, updated.ID, day24), "the moved date's task is in")
}

// A create whose handover fails still commits: the seam is best-effort —
// the error is logged away, the rule and its tasks stand, the hourly scan
// remains the backstop.
func TestScheduleSeam_SeamFailureDoesNotFailMutation(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwner(taskMoscowTZ).withSeam(&fakeSeam{err: errSeamDown})

	rule, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, h.createCmd())
	require.NoError(t, err, "a broken seam must never fail the committed rule")

	stored, err := h.rules.GetRule(h.ctx(), h.owner, h.propID, rule.ID)
	require.NoError(t, err)
	assert.Equal(t, rule.ID, stored.ID)
	assert.Empty(t, h.seam.handovers)
}

// No seam wired — the pre-#775 silence: the create succeeds, nothing is
// dispatched, nothing panics.
func TestScheduleSeam_NilSeamKeepsSilence(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwner(taskMoscowTZ)

	_, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, h.createCmd())
	require.NoError(t, err)
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

// futureTaskID finds the rule's standing task on the given anchor date's
// next occurrence — the single future the edit invalidates.
func futureTaskID(h *tasksHarness, ruleID uuid.UUID, afterDate string) *uuid.UUID {
	h.t.Helper()
	for _, id := range uncompletedIDs(h, ruleID) {
		for _, row := range h.loadTasks(h.t) {
			if row.ID == id && row.DueDate != nil && *row.DueDate > afterDate {
				return new(id)
			}
		}
	}
	return nil
}

// standingOn finds the rule's standing task with the given due date.
func standingOn(h *tasksHarness, ruleID uuid.UUID, date string) uuid.UUID {
	h.t.Helper()
	for _, id := range uncompletedIDs(h, ruleID) {
		for _, row := range h.loadTasks(h.t) {
			if row.ID == id && row.DueDate != nil && *row.DueDate == date {
				return id
			}
		}
	}
	h.t.Fatalf("no standing task of rule %s on %s", ruleID, date)
	return uuid.Nil
}
