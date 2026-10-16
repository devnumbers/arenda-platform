//go:build integration

package application_test

// The materialization tick family (ADR 0051): due + single future over real
// PostgreSQL, idempotent reruns, the in-mutation materialization, the fresh
// snapshot of the future after a rule edit and the zone-split today.

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
)

func TestTick_MaterializesDueAndSingleFutureThenRerunIsNoop(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwner(taskMoscowTZ)
	// Weekly Thursdays anchored in the past: Aug 27, Sep 3 and Sep 10 are
	// due, Sep 17 is the single future.
	ruleID := h.seedRule(aug27, "", string(domain.RepeatWeekly), "Вынести мусор")

	h.runTick()
	tasks := h.loadTasks(t)
	if got := ruleDatesOf(tasks, ruleID); !slices.Equal(got, []string{aug27, day03, day10, day17}) {
		t.Fatalf("first tick materialized %v, want [%s %s %s %s]", got, aug27, day03, day10, day17)
	}

	h.runTick()
	if got := ruleDatesOf(h.loadTasks(t), ruleID); len(got) != 4 {
		t.Fatalf("rerun produced %v, want the same four rows", got)
	}
}

func TestTick_OnceRuleBeyondTodayNeverGrows(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwner(taskMoscowTZ)
	ruleID := h.seedRule(day10, "15:13", string(domain.RepeatOnce), "Передать показания")

	h.runTick()
	tasks := h.loadTasks(t)
	if got := ruleDatesOf(tasks, ruleID); !slices.Equal(got, []string{day10}) {
		t.Fatalf("once rule materialized %v, want [%s]", got, day10)
	}
	for _, task := range tasksWithDate(tasks, day10) {
		if task.DueTime == nil || *task.DueTime != "15:13:00" {
			t.Fatalf("due time snapshot = %v, want 15:13:00", task.DueTime)
		}
	}

	// A week later the spent once rule has nothing to add.
	h.clock.now = h.clock.now.AddDate(0, 0, 7)
	h.runTick()
	if got := ruleDatesOf(h.loadTasks(t), ruleID); len(got) != 1 {
		t.Fatalf("late tick produced %v, want the same single row", got)
	}
}

func TestCreateRule_MaterializesInSameTransaction(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwner(taskMoscowTZ)

	// Dated weekly anchored today: today's task and the single future.
	rule, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}
	// Undated once: the undated task materializes immediately.
	undatedTitle := "Разобрать кладовку"
	if _, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, application.CreateRuleCommand{
		Title:  undatedTitle,
		Repeat: domain.RepeatOnce,
	}); err != nil {
		t.Fatalf("create undated rule: %v", err)
	}

	tasks := h.loadTasks(t)
	if got := ruleDatesOf(tasks, rule.ID); !slices.Equal(got, []string{day10, day17}) {
		t.Fatalf("create materialized %v, want [%s %s]", got, day10, day17)
	}
	undated := tasksWithDate(tasks, "")
	if len(undated) != 1 || undated[0].Title != undatedTitle {
		t.Fatalf("undated task = %+v, want one with title %q", undated, undatedTitle)
	}

	actions := h.auditActions(t, "task_rule", rule.ID)
	if len(actions) != 1 || actions[0] != "task_rule.created" {
		t.Fatalf("audit = %v, want [task_rule.created]", actions)
	}
}

func TestUpdateRule_ReplacesFutureWithFreshSnapshot(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwner(taskMoscowTZ)
	ruleID := h.seedRule(aug27, "", string(domain.RepeatWeekly), "Старое название")
	h.runTick()

	// The rule title changes; the future occurrence must re-materialize with
	// the fresh snapshot while the already due ones keep the frozen one.
	newTitle := "Новое название"
	updated, err := h.rules.UpdateRule(h.ctx(), h.owner, h.propID, ruleID, application.UpdateRuleCommand{
		Title: &newTitle,
	})
	if err != nil {
		t.Fatalf("update rule: %v", err)
	}
	if updated.Title != newTitle {
		t.Fatalf("updated rule title = %q", updated.Title)
	}
	tasks := h.ruleTasks(h, ruleID)
	if got := datesOf(tasks); !slices.Equal(got, []string{aug27, day03, day10, day17}) {
		t.Fatalf("tasks after edit = %v, want the same four dates", got)
	}
	for _, task := range tasks {
		want := "Старое название"
		if *task.DueDate == day17 {
			want = newTitle
		}
		if task.Title != want {
			t.Fatalf("task at %s carries snapshot %q, want %q", *task.DueDate, task.Title, want)
		}
	}
}

func TestRuleValidation_BackdatedAnchorRejected(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwner(taskMoscowTZ)

	cmd := h.createCmd()
	past := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	cmd.DueDate = &past
	if _, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, cmd); !errors.Is(err, application.ErrInvalidInput) {
		t.Fatalf("backdated create = %v, want ErrInvalidInput", err)
	}
	if tasks := h.loadTasks(t); len(tasks) != 0 {
		t.Fatalf("rejected create left %d tasks", len(tasks))
	}
}

// ruleTasks projects the harness loader onto one rule's tasks.
func (h *tasksHarness) ruleTasks(hh *tasksHarness, ruleID uuid.UUID) []taskRow {
	h.t.Helper()
	out := make([]taskRow, 0)
	for _, task := range hh.loadTasks(h.t) {
		if task.RuleID != nil && *task.RuleID == ruleID {
			out = append(out, task)
		}
	}
	return out
}
