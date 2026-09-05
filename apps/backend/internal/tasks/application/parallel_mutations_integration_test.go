//go:build integration

package application_test

// The parallel-mutation deadlock contract (#546): two property mutations of
// one owner running concurrently must serialize without a PostgreSQL
// deadlock (SQLSTATE 40P01). The choreography below forces the exact
// interleaving the bug needs — both transactions past their single property
// lock before either reaches the owner-wide tick lock — through an external
// row lock on the first task's row, so the verdict is deterministic.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
)

// seedSecondProperty inserts one more active property of the harness owner
// and a weekly rule on it — the second mutation's target.
func (h *tasksHarness) seedSecondProperty() uuid.UUID {
	h.t.Helper()
	propID, err := uuid.NewV7()
	if err != nil {
		h.t.Fatalf("new uuid: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO properties (id, owner_id, name, type, address, status)
		 VALUES ($1, $2, 'Дом', 'house', 'Москва, Профсоюзная 2', 'active')`,
		propID, h.owner,
	); err != nil {
		h.t.Fatalf("seed second property: %v", err)
	}
	ruleID, err := uuid.NewV7()
	if err != nil {
		h.t.Fatalf("new uuid: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO task_rules (id, owner_id, property_id, title, comment, due_date, due_time, repeat)
		 VALUES ($1, $2, $3, 'Правило второго объекта', NULL, $4, NULL, 'weekly')`,
		ruleID, h.owner, propID, day03,
	); err != nil {
		h.t.Fatalf("seed second property rule: %v", err)
	}
	return propID
}

// activeTaskOf returns one uncompleted task of the property, failing when
// the tick materialized none.
func (h *tasksHarness) activeTaskOf(t *testing.T, propID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := h.pool.QueryRow(h.ctx(),
		`SELECT id FROM tasks WHERE property_id = $1 AND completed_date IS NULL
		 ORDER BY id LIMIT 1`, propID).Scan(&id)
	if err != nil {
		t.Fatalf("no active task to complete on %s: %v", propID, err)
	}
	return id
}

// waitLockWaitingBlocks polls pg_stat_activity until at least n backends of
// the test database sit on a lock wait — the deterministic "the transaction
// reached its contended statement" signal.
func waitLockWaitingBlocks(t *testing.T, pool *pgxpool.Pool, n int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		if err := pool.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_stat_activity
			 WHERE datname = current_database()
			   AND state = 'active' AND wait_event_type = 'Lock'`).Scan(&waiting); err != nil {
			t.Fatalf("poll lock waits: %v", err)
		}
		if waiting >= n {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d lock-blocked backends", n)
}

// completeAsync runs CompleteTask on its own goroutine and reports the error
// back; a hang (a deadlock without a victim) fails via the caller's timeout.
func (h *tasksHarness) completeAsync(propID, taskID uuid.UUID) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		_, err := h.tasks.CompleteTask(h.ctx(), h.owner, propID, taskID)
		errCh <- err
	}()
	return errCh
}

// collectComplete waits for both completes and fails the test naming the
// #546 deadlock when one came back with SQLSTATE 40P01.
func collectComplete(t *testing.T, errChs ...<-chan error) {
	t.Helper()
	for i, errCh := range errChs {
		select {
		case err := <-errCh:
			if err == nil {
				continue
			}
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "40P01" {
				t.Fatalf("parallel completes hit the #546 deadlock (SQLSTATE 40P01): %v", err)
			}
			t.Fatalf("complete %d failed: %v", i+1, err)
		case <-time.After(15 * time.Second):
			t.Fatalf("complete %d did not finish — deadlock without a victim", i+1)
		}
	}
}

// TestParallelPropertyMutations_SerializeWithoutDeadlock pins the #546
// contract: two parallel completes on different properties of one owner both
// succeed — one transaction 40P01-aborted is the bug.
func TestParallelPropertyMutations_SerializeWithoutDeadlock(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwner(taskMoscowTZ)
	prop1 := h.propID
	h.seedRule(day03, "", string(domain.RepeatWeekly), "Правило первого объекта")
	prop2 := h.seedSecondProperty()
	h.runTick()

	task1 := h.activeTaskOf(t, prop1)
	task2 := h.activeTaskOf(t, prop2)

	// Park one transaction mid-mutation: hold task1's row from outside, so
	// the first CompleteTask blocks inside its change step holding prop1.
	locker, err := h.pool.Acquire(h.ctx())
	if err != nil {
		t.Fatalf("acquire locker conn: %v", err)
	}
	defer locker.Release()
	if _, err := locker.Exec(h.ctx(), `BEGIN`); err != nil {
		t.Fatalf("begin external tx: %v", err)
	}
	if _, err := locker.Exec(h.ctx(),
		`SELECT id FROM tasks WHERE id = $1 FOR UPDATE`, task1); err != nil {
		t.Fatalf("lock task row: %v", err)
	}

	err1 := h.completeAsync(prop1, task1)
	waitLockWaitingBlocks(t, h.pool, 1)

	// The second complete passes its own property lock and blocks on the
	// owner-wide tick lock held by the first transaction.
	err2 := h.completeAsync(prop2, task2)
	waitLockWaitingBlocks(t, h.pool, 2)

	// Release the parked mutation: both transactions now race for the
	// owner's property rows — the pre-fix code deadlocks here.
	if _, err := locker.Exec(h.ctx(), `ROLLBACK`); err != nil {
		t.Fatalf("rollback external tx: %v", err)
	}

	collectComplete(t, err1, err2)
}
