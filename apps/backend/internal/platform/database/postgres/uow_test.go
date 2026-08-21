package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// --- transaction fakes ---
//
// The Do logic is independent of pgx; it talks only to the Beginner/Tx ports.
// These fakes mirror identity/application's fakeBeginner/fakeTx so the tests
// exercise commit/rollback/panic semantics without a database.

type fakeTx struct {
	b    *fakeBeginner
	done bool
}

func (tx *fakeTx) Commit(context.Context) error {
	if !tx.done {
		tx.b.committed++
		tx.b.open--
		tx.done = true
	}
	return tx.b.commitErr
}

func (tx *fakeTx) Rollback(context.Context) error {
	if !tx.done {
		tx.b.rolledBack++
		tx.b.open--
		tx.done = true
	}
	return nil
}

type fakeBeginner struct {
	begun, committed, rolledBack, open int
	beginErr                           error
	commitErr                          error
	lastTx                             *fakeTx
}

func (b *fakeBeginner) Begin(context.Context) (transaction.Tx, error) {
	if b.beginErr != nil {
		return nil, b.beginErr
	}
	b.begun++
	b.open++
	tx := &fakeTx{b: b}
	b.lastTx = tx
	return tx, nil
}

func newUoWWithBeginner(b *fakeBeginner) *uow {
	return &uow{beginner: b}
}

func TestUoW_Do_CommitsOnNilError(t *testing.T) {
	t.Parallel()

	b := &fakeBeginner{}
	u := newUoWWithBeginner(b)

	workCalled := false
	err := u.Do(t.Context(), func(tx transaction.Tx) error {
		workCalled = true
		return nil
	})
	if err != nil {
		t.Fatalf("Do returned %v, want nil", err)
	}
	if !workCalled {
		t.Fatal("work was not called")
	}
	if b.begun != 1 {
		t.Errorf("begun = %d, want 1", b.begun)
	}
	if b.committed != 1 {
		t.Errorf("committed = %d, want 1", b.committed)
	}
	if b.rolledBack != 0 {
		t.Errorf("rolledBack = %d, want 0", b.rolledBack)
	}
	if b.open != 0 {
		t.Errorf("open = %d, want 0 (transaction left open)", b.open)
	}
}

func TestUoW_Do_RollsBackOnError(t *testing.T) {
	t.Parallel()

	b := &fakeBeginner{}
	u := newUoWWithBeginner(b)

	workErr := errors.New("boom")
	err := u.Do(t.Context(), func(tx transaction.Tx) error {
		return workErr
	})

	if !errors.Is(err, workErr) {
		t.Fatalf("Do returned %v, want %v", err, workErr)
	}
	if b.committed != 0 {
		t.Errorf("committed = %d, want 0", b.committed)
	}
	if b.rolledBack != 1 {
		t.Errorf("rolledBack = %d, want 1", b.rolledBack)
	}
	if b.open != 0 {
		t.Errorf("open = %d, want 0 (transaction left open)", b.open)
	}
}

func TestUoW_Do_PanicsRollsBackAndRepanics(t *testing.T) {
	t.Parallel()

	b := &fakeBeginner{}
	u := newUoWWithBeginner(b)

	panicVal := sentinelError{"kaboom"}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected Do to re-panic, but it did not panic")
		}
		got, ok := r.(sentinelError)
		if !ok || got.msg != panicVal.msg {
			t.Fatalf("recovered %v, want %v", r, panicVal)
		}
		if b.committed != 0 {
			t.Errorf("committed = %d, want 0 on panic", b.committed)
		}
		if b.rolledBack != 1 {
			t.Errorf("rolledBack = %d, want 1 on panic", b.rolledBack)
		}
		if b.open != 0 {
			t.Errorf("open = %d, want 0 (transaction left open)", b.open)
		}
	}()

	if err := u.Do(t.Context(), func(tx transaction.Tx) error {
		panic(panicVal)
	}); err != nil {
		t.Errorf("Do returned %v before re-panic, want panic", err)
	}
	t.Fatal("expected Do to re-panic")
}

func TestUoW_Do_PropagatesCommitError(t *testing.T) {
	t.Parallel()

	commitErr := errors.New("commit failed")
	b := &fakeBeginner{commitErr: commitErr}
	u := newUoWWithBeginner(b)

	err := u.Do(t.Context(), func(tx transaction.Tx) error {
		return nil
	})

	if !errors.Is(err, commitErr) {
		t.Fatalf("Do returned %v, want %v", err, commitErr)
	}
	if b.committed != 1 {
		t.Errorf("commit attempts = %d, want 1", b.committed)
	}
	// The deferred rollback is a no-op after a commit attempt (ADR 0033 trusts
	// pgx v5 here); it must not double-count as a rollback.
	if b.rolledBack != 0 {
		t.Errorf("rolledBack = %d, want 0 (deferred rollback is a no-op after commit)", b.rolledBack)
	}
	if b.open != 0 {
		t.Errorf("open = %d, want 0", b.open)
	}
}

func TestUoW_Do_PropagatesBeginError(t *testing.T) {
	t.Parallel()

	beginErr := errors.New("begin failed")
	b := &fakeBeginner{beginErr: beginErr}
	u := newUoWWithBeginner(b)

	err := u.Do(t.Context(), func(tx transaction.Tx) error {
		t.Fatal("work must not run when Begin fails")
		return nil
	})

	if !errors.Is(err, beginErr) {
		t.Fatalf("Do returned %v, want %v", err, beginErr)
	}
	if b.begun != 0 {
		t.Errorf("begun = %d, want 0 (Begin aborted)", b.begun)
	}
	// Begin fails before opening a transaction, so nothing to commit or roll
	// back and no transaction left open.
	if b.committed != 0 || b.rolledBack != 0 {
		t.Errorf("committed=%d rolledBack=%d, want 0/0 (no tx to finish)", b.committed, b.rolledBack)
	}
	if b.open != 0 {
		t.Errorf("open = %d, want 0", b.open)
	}
}

// sentinelError is a typed panic value so the re-panic test can distinguish a
// real re-panic from any stray panic in test machinery.
type sentinelError struct{ msg string }

func (s sentinelError) Error() string { return "sentinel panic: " + s.msg }
