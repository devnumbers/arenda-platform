package application

import (
	"context"
	"errors"
	"testing"

	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// txBeginner begins a database transaction. Production code reaches the
// database through the UoW (ADR 0033); only the test fakeUoW below adapts a
// txBeginner to the UoW port.
type txBeginner interface {
	Begin(ctx context.Context) (transaction.Tx, error)
}

// fakeUoW adapts a txBeginner to the transaction.UoW port with the production
// commit/rollback semantics: a deferred rollback after a successful commit is a
// no-op, and a panic in work rolls back and re-panics. Mirrors the identity,
// access and properties stores tests and
// internal/platform/database/postgres/uow.go.
type fakeUoW struct {
	beginner txBeginner
}

func (u fakeUoW) Do(ctx context.Context, work func(tx transaction.Tx) error) (err error) {
	tx, err := u.beginner.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		// Same defer shape as the production UoW (rollback, then recover and
		// re-panic). Unlike prod's `_ =` discard, the rollback error is folded
		// into the named return — and only when work succeeded, so it never
		// masks the work error or the re-panicked value. The fake
		// transactions never fail to roll back, so the fold never fires.
		rollbackErr := tx.Rollback(ctx)
		if r := recover(); r != nil {
			panic(r)
		}
		if rollbackErr != nil && err == nil {
			err = rollbackErr
		}
	}()
	if err := work(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// countingBeginner counts begun/committed/rolled-back transactions so the
// stores smoke tests can assert the UoW commit/rollback semantics.
type countingBeginner struct {
	begun, committed, rolledBack, open int
}

func (b *countingBeginner) Begin(context.Context) (transaction.Tx, error) {
	b.begun++
	b.open++
	return &countingTx{b: b}, nil
}

// countingTx tracks the done flag so a deferred rollback after commit counts
// as a no-op, matching pgx v5 (relied upon by ADR 0033).
type countingTx struct {
	b    *countingBeginner
	done bool
}

func (tx *countingTx) Commit(context.Context) error {
	if !tx.done {
		tx.b.committed++
		tx.b.open--
		tx.done = true
	}
	return nil
}

func (tx *countingTx) Rollback(context.Context) error {
	if !tx.done {
		tx.b.rolledBack++
		tx.b.open--
		tx.done = true
	}
	return nil
}

// countingRecorder wraps auditapp.Noop to count WithTx bindings.
type countingRecorder struct {
	auditapp.Noop
	withTxCalls int
}

func (r *countingRecorder) WithTx(transaction.Tx) auditapp.Recorder {
	r.withTxCalls++
	return r
}

// TestRunInTx_BuildsStoresFromTxAndCommits proves runInTx binds the repository
// and the audit recorder to the transaction, runs work, and the UoW commits on
// a nil error. PreferenceRepository.WithTx is infallible (it returns the bound
// port directly), so — unlike identity — there are no bind-error paths to cover.
func TestRunInTx_BuildsStoresFromTxAndCommits(t *testing.T) {
	t.Parallel()

	b := &countingBeginner{}
	repo := &fakePreferenceRepo{}
	audit := &countingRecorder{}
	f := NewTxStoreFactory(repo, audit, fakeUoW{beginner: b})

	workCalled := false
	var got *txStores
	err := f.runInTx(t.Context(), func(stores *txStores) error {
		workCalled = true
		got = stores
		return nil
	})
	if err != nil {
		t.Fatalf("runInTx returned %v, want nil", err)
	}
	if !workCalled {
		t.Fatal("work was not called")
	}

	// Both stores are bound to the transaction, and the same instances are
	// handed to work (the in-memory WithTx implementations return the
	// receiver).
	if audit.withTxCalls != 1 {
		t.Errorf("audit.WithTx calls = %d, want 1", audit.withTxCalls)
	}
	if got.repo != PreferenceRepository(repo) {
		t.Error("work received unbound preference repository")
	}

	// UoW commits on nil error and leaves no transaction open.
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

// TestRunInTx_PanicRollsBackAndRepanics proves a panic inside work rolls the
// transaction back and re-panics, so a panicking use case never leaks a tx.
func TestRunInTx_PanicRollsBackAndRepanics(t *testing.T) {
	t.Parallel()

	b := &countingBeginner{}
	f := NewTxStoreFactory(&fakePreferenceRepo{}, &countingRecorder{}, fakeUoW{beginner: b})

	panicVal := storesSentinelError{"kaboom"}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected runInTx to re-panic, but it did not panic")
		}
		got, ok := r.(storesSentinelError)
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
	// Unreachable while runInTx honors the re-panic contract; if it ever
	// returns, fail with the value instead of discarding it.
	err := f.runInTx(t.Context(), func(*txStores) error {
		panic(panicVal)
	})
	t.Fatalf("runInTx returned %v, want re-panic", err)
}

// TestRunInTx_RollsBackOnWorkError proves a non-nil work error rolls the
// transaction back and is returned to the caller.
func TestRunInTx_RollsBackOnWorkError(t *testing.T) {
	t.Parallel()

	b := &countingBeginner{}
	f := NewTxStoreFactory(&fakePreferenceRepo{}, &countingRecorder{}, fakeUoW{beginner: b})

	workErr := errors.New("business rule violated")
	err := f.runInTx(t.Context(), func(*txStores) error {
		return workErr
	})

	if !errors.Is(err, workErr) {
		t.Fatalf("runInTx returned %v, want %v", err, workErr)
	}
	if b.committed != 0 {
		t.Errorf("committed = %d, want 0 on work error", b.committed)
	}
	if b.rolledBack != 1 {
		t.Errorf("rolledBack = %d, want 1 on work error", b.rolledBack)
	}
	if b.open != 0 {
		t.Errorf("open = %d, want 0 (transaction left open)", b.open)
	}
}

// TestRunInTx_ReturnsErrorWhenUoWMissing proves a service that forgot to wire a
// UoW fails loudly at the call site instead of nil-dereferencing.
func TestRunInTx_ReturnsErrorWhenUoWMissing(t *testing.T) {
	t.Parallel()

	f := NewTxStoreFactory(&fakePreferenceRepo{}, &countingRecorder{}, nil)

	workCalled := false
	err := f.runInTx(t.Context(), func(*txStores) error {
		workCalled = true
		return nil
	})

	if err == nil {
		t.Fatal("runInTx returned nil, want error for missing UoW")
	}
	if workCalled {
		t.Fatal("work must not run when UoW is missing")
	}
}

// storesSentinelError is a typed panic value so the re-panic test can
// distinguish a real re-panic from any stray panic in test machinery.
type storesSentinelError struct{ msg string }

func (s storesSentinelError) Error() string { return "stores sentinel panic: " + s.msg }
