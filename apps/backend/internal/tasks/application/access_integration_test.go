//go:build integration

package application_test

// The rights family (ADR 0028 via the policy port) and the read-side moment
// family (ADR 0048 + the minute precision of resolution #496): the role
// matrix over real stores, the timezone split of the zone sweep and the
// overdue boundary on the listings.

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
)

func TestTaskRoleMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		role  sharedpolicy.Role
		read  error
		write error
	}{
		{name: "owner", role: sharedpolicy.RoleOwner},
		{name: "full access", role: sharedpolicy.RoleFullAccess},
		{name: "viewer", role: sharedpolicy.RoleViewer, write: application.ErrForbidden},
		{name: "suspended", role: sharedpolicy.RoleSuspended, read: application.ErrNotFound, write: application.ErrNotFound},
		{name: "none", role: sharedpolicy.RoleNone, read: application.ErrNotFound, write: application.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newTasksHarnessWithPolicy(t, stubPropertyPolicy{role: tt.role}).withOwner(taskMoscowTZ)
			// The actor is a member user of their own; the stub resolves the
			// configured role for anyone, and audit_log needs a users row.
			actorID, err := uuid.NewV7()
			if err != nil {
				t.Fatalf("new actor id: %v", err)
			}
			h.seedActor(actorID)

			// The write gate: a viewer is forbidden, a stranger is the
			// privacy 404 — and never sees any task data on top.
			denial, completed := h.exerciseWriteGate(t, actorID)
			if tt.write == nil {
				if !completed || denial != nil {
					t.Fatalf("write gate = %v (completed: %v), want the full lifecycle", denial, completed)
				}
			} else if !sameErrClass(denial, tt.write) {
				t.Fatalf("write gate = %v, want %v", denial, tt.write)
			}

			// The read gate: view-capable roles read, everyone else gets the
			// privacy 404.
			if _, err := h.tasks.ListTasks(h.ctx(), actorID, h.propID, application.TasksListQuery{}); !sameErrClass(err, tt.read) {
				t.Fatalf("list = %v, want %v", err, tt.read)
			}
		})
	}
}

// exerciseWriteGate drives every mutating use case of the context once: the
// first error is the denial verdict, completed reports whether the whole
// lifecycle (create → complete → uncomplete → delete → clear) passed.
func (h *tasksHarness) exerciseWriteGate(t *testing.T, actorID uuid.UUID) (denial error, completed bool) {
	t.Helper()
	rule, err := h.rules.CreateRule(h.ctx(), actorID, h.propID, h.createCmd())
	if err != nil {
		// The first denial is the verdict of the whole write gate.
		return err, false
	}
	tasks := h.ruleTasks(h, rule.ID)
	if len(tasks) == 0 {
		t.Fatalf("created rule materialized no tasks")
	}
	if _, err := h.tasks.CompleteTask(h.ctx(), actorID, h.propID, tasks[0].ID); err != nil {
		return err, false
	}
	if _, err := h.tasks.UncompleteTask(h.ctx(), actorID, h.propID, tasks[0].ID); err != nil {
		return err, false
	}
	if err := h.rules.DeleteRule(h.ctx(), actorID, h.propID, rule.ID); err != nil {
		return err, false
	}
	if _, err := h.tasks.ClearCompletedJournal(h.ctx(), actorID, h.propID); err != nil {
		return err, false
	}
	return nil, true
}

// sameErrClass compares by errors.Is when both are non-nil, by identity
// otherwise (nil means "no error expected").
func sameErrClass(got, want error) bool {
	if want == nil {
		return got == nil
	}
	return errors.Is(got, want)
}

func TestOwnerClock_ZoneSplit(t *testing.T) {
	t.Parallel()

	// 2026-09-10 16:30 UTC: already the 11th (04:30) in Kamchatka, still
	// 19:30 of the 10th in Moscow.
	h := newTasksHarness(t)
	h.clock.now = time.Date(2026, 9, 10, 16, 30, 0, 0, time.UTC)

	kamchatka := h.withOwner("Asia/Kamchatka")
	ruleKam := kamchatka.seedRule(day10, "23:50", string(domain.RepeatOnce), "Камчатка")
	kamchatka.runTick()

	// For the Kamchatka owner the 10th is over: the once rule anchored on
	// the 10th is due, its single task exists.
	if got := ruleDatesOf(kamchatka.loadTasks(t), ruleKam); !slices.Equal(got, []string{day10}) {
		t.Fatalf("kamchatka once rule = %v, want [%s]", got, day10)
	}

	// A Moscow owner at the same instant still lives on the 10th: a task due
	// 23:50 is not overdue yet.
	h2 := newTasksHarness(t)
	h2.clock.now = time.Date(2026, 9, 10, 16, 30, 0, 0, time.UTC)
	moscow := h2.withOwner(taskMoscowTZ)
	moscow.seedRule(day10, "23:50", string(domain.RepeatOnce), "Москва")
	moscow.runTick()

	page, err := moscow.tasks.ListTasks(moscow.ctx(), moscow.owner, moscow.propID, application.TasksListQuery{})
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Status != domain.ViewActive {
		t.Fatalf("moscow 23:50 task status = %+v, want active (the day is not over)", page.Items)
	}

	// At exactly 23:50 Moscow time the task flips to overdue — the due
	// minute is the inclusive boundary (resolution #496).
	h2.clock.now = time.Date(2026, 9, 10, 20, 50, 0, 0, time.UTC) // 23:50 Moscow
	page, err = moscow.tasks.ListTasks(moscow.ctx(), moscow.owner, moscow.propID, application.TasksListQuery{})
	if err != nil {
		t.Fatalf("list tasks at due minute: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Status != domain.ViewOverdue {
		t.Fatalf("23:50 task at 23:50 = %+v, want overdue (inclusive minute)", page.Items)
	}

	// And the sweep materializes the Kamchatka owner on the 11th while a
	// fresh once rule anchored "tomorrow" for Moscow (the 11th) is future —
	// its single future task stands, nothing overdue.
	moscowFuture := moscow.seedRule(day17, "", string(domain.RepeatOnce), "Будущее")
	moscow.runTick()
	if got := ruleDatesOf(moscow.loadTasks(t), moscowFuture); !slices.Equal(got, []string{day17}) {
		t.Fatalf("future once rule = %v, want [%s]", got, day17)
	}
}
