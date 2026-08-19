package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
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
// access, properties and notifications stores tests.
type fakeUoW struct {
	beginner txBeginner
}

func (u fakeUoW) Do(ctx context.Context, work func(tx transaction.Tx) error) (err error) {
	tx, err := u.beginner.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
		if r := recover(); r != nil {
			panic(r)
		}
	}()
	if err := work(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// countingBeginner counts begun/committed/rolled-back transactions so the
// stores smoke tests can assert the UoW commit/rollback semantics; last is the
// most recently begun tx, so a test can check what runInTx handed to work.
type countingBeginner struct {
	begun, committed, rolledBack, open int
	last                               transaction.Tx
}

func (b *countingBeginner) Begin(context.Context) (transaction.Tx, error) {
	b.begun++
	b.open++
	b.last = &countingTx{b: b}
	return b.last, nil
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

// TestRunInTx_BuildsStoresFromTxAndCommits proves runInTx binds every wired
// collaborator to the transaction, runs work, and the UoW commits on a nil
// error.
func TestRunInTx_BuildsStoresFromTxAndCommits(t *testing.T) {
	b := &countingBeginner{}
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	leaseRepo := &fakeLeaseRepo{}
	propertyRepo := &fakePropertyRepo{}
	tenantContacts := &fakeTenantContactRepo{}
	recurringOps := &fakeRecurringOperationRepo{}
	operations := &fakeOperationRepo{}
	categories := newFakeCategoryRepoForOwner(ownerID)
	scheduler := &fakeReminderScheduler{}
	audit := &countingRecorder{}
	f := NewTxStoreFactory(
		leaseRepo, propertyRepo, tenantContacts,
		recurringOps, operations, categories,
		scheduler, audit, fakeUoW{beginner: b},
	)

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

	// Every store is bound to the transaction, and the same instances are
	// handed to work (the in-memory WithTx implementations return the
	// receiver).
	if audit.withTxCalls != 1 {
		t.Errorf("audit.WithTx calls = %d, want 1", audit.withTxCalls)
	}
	if got.leases != LeaseRepository(leaseRepo) {
		t.Error("work received unbound lease repository")
	}
	if got.properties != PropertyRepository(propertyRepo) {
		t.Error("work received unbound property repository")
	}
	if got.tenantContacts != TenantContactRepository(tenantContacts) {
		t.Error("work received unbound tenant contact repository")
	}
	if got.recurringOps != RecurringOperationRepository(recurringOps) {
		t.Error("work received unbound recurring operation repository")
	}
	if got.operations != OperationRepository(operations) {
		t.Error("work received unbound operation repository")
	}
	if got.categories != OperationCategoryRepository(categories) {
		t.Error("work received unbound category repository")
	}
	if got.scheduler == nil {
		t.Error("work received unbound reminder scheduler")
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

// TestRunInTx_OptionalStoresStayNilWhenUnwired proves runInTx tolerates the
// per-service collaborator subsets (the operation service never wires
// leases/tenantContacts/recurringOps, the recurring operation service never
// wires leases/tenantContacts, and a test may wire only the stores its use
// cases reach): an unwired optional store stays nil instead of panicking on a
// nil WithTx, and audit is still bound.
func TestRunInTx_OptionalStoresStayNilWhenUnwired(t *testing.T) {
	b := &countingBeginner{}
	audit := &countingRecorder{}
	f := NewTxStoreFactory(nil, nil, nil, nil, nil, nil, nil, audit, fakeUoW{beginner: b})

	var got *txStores
	err := f.runInTx(t.Context(), func(stores *txStores) error {
		got = stores
		return nil
	})
	if err != nil {
		t.Fatalf("runInTx returned %v, want nil", err)
	}

	if got.audit == nil {
		t.Error("audit must be bound even with optional stores unwired")
	}
	if got.leases != nil || got.properties != nil || got.tenantContacts != nil ||
		got.recurringOps != nil || got.operations != nil || got.categories != nil ||
		got.scheduler != nil {
		t.Errorf("unwired optional stores must stay nil, got %+v", got)
	}
	if b.committed != 1 {
		t.Errorf("committed = %d, want 1", b.committed)
	}
}

// TestRunInTx_PanicRollsBackAndRepanics proves a panic inside work rolls the
// transaction back and re-panics, so a panicking use case never leaks a tx.
func TestRunInTx_PanicRollsBackAndRepanics(t *testing.T) {
	b := &countingBeginner{}
	f := NewTxStoreFactory(nil, nil, nil, nil, nil, nil, nil, &countingRecorder{}, fakeUoW{beginner: b})

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
			t.Errorf("rolledBack = %d, want 1", b.rolledBack)
		}
		if b.open != 0 {
			t.Errorf("open = %d, want 0 (transaction left open)", b.open)
		}
	}()
	_ = f.runInTx(t.Context(), func(*txStores) error {
		panic(panicVal)
	})
	t.Fatal("expected runInTx to re-panic")
}

// TestRunInTx_RollsBackOnWorkError proves a non-nil work error rolls the
// transaction back and is returned to the caller.
func TestRunInTx_RollsBackOnWorkError(t *testing.T) {
	b := &countingBeginner{}
	f := NewTxStoreFactory(nil, nil, nil, nil, nil, nil, nil, &countingRecorder{}, fakeUoW{beginner: b})

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
		t.Errorf("rolledBack = %d, want 1", b.rolledBack)
	}
	if b.open != 0 {
		t.Errorf("open = %d, want 0 (transaction left open)", b.open)
	}
}

// TestRunInTx_ReturnsErrorWhenUoWMissing proves a service that forgot to wire a
// UoW fails loudly at the call site instead of nil-dereferencing.
func TestRunInTx_ReturnsErrorWhenUoWMissing(t *testing.T) {
	f := NewTxStoreFactory(nil, nil, nil, nil, nil, nil, nil, &countingRecorder{}, nil)

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
