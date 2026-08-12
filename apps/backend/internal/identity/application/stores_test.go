package application

import (
	"context"
	"errors"
	"testing"

	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// fakeUoW adapts the shared fakeBeginner to the transaction.UoW port so the
// stores smoke test can assert commit/rollback/panic semantics without a
// database, mirroring internal/platform/database/postgres/uow_test.go.
type fakeUoW struct {
	beginner *fakeBeginner
}

func (u *fakeUoW) Do(ctx context.Context, work func(tx transaction.Tx) error) (err error) {
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

// countingUserRepo embeds the shared fakeUserRepo and counts WithTx calls so
// the smoke test can prove runInTx binds every repository to the transaction.
type countingUserRepo struct {
	*fakeUserRepo
	withTxCalls int
	withTxErr   error
}

func (r *countingUserRepo) WithTx(tx transaction.Tx) (UserRepository, error) {
	r.withTxCalls++
	if r.withTxErr != nil {
		return nil, r.withTxErr
	}
	return r, nil
}

type countingCodeRepo struct {
	*fakeCodeRepo
	withTxCalls int
	withTxErr   error
}

func (r *countingCodeRepo) WithTx(tx transaction.Tx) (LoginCodeRepository, error) {
	r.withTxCalls++
	if r.withTxErr != nil {
		return nil, r.withTxErr
	}
	return r, nil
}

type countingAttemptRepo struct {
	*fakeAttemptRepo
	withTxCalls int
	withTxErr   error
}

func (r *countingAttemptRepo) WithTx(tx transaction.Tx) (AttemptRepository, error) {
	r.withTxCalls++
	if r.withTxErr != nil {
		return nil, r.withTxErr
	}
	return r, nil
}

type countingSessionRepo struct {
	*fakeSessionRepo
	withTxCalls int
	withTxErr   error
}

func (r *countingSessionRepo) WithTx(tx transaction.Tx) (SessionRepository, error) {
	r.withTxCalls++
	if r.withTxErr != nil {
		return nil, r.withTxErr
	}
	return r, nil
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

// newCountingFactory builds a txStoreFactory wired to counting fakes and a
// fakeUoW, returning every piece so the test can assert call counts.
func newCountingFactory(t *testing.T) (*txStoreFactory, *fakeBeginner, *countingUserRepo, *countingCodeRepo, *countingAttemptRepo, *countingSessionRepo, *countingRecorder) {
	t.Helper()
	b := &fakeBeginner{}
	users := &countingUserRepo{fakeUserRepo: newFakeUserRepo()}
	codes := &countingCodeRepo{fakeCodeRepo: newFakeCodeRepo()}
	attempts := &countingAttemptRepo{fakeAttemptRepo: newFakeAttemptRepo()}
	sessions := &countingSessionRepo{fakeSessionRepo: newFakeSessionRepo()}
	audit := &countingRecorder{}
	f := &txStoreFactory{
		users: users, codes: codes, attempts: attempts,
		sessions: sessions, audit: audit, uow: &fakeUoW{beginner: b},
	}
	return f, b, users, codes, attempts, sessions, audit
}

// fakeStores bundles the shared identity fake repositories and a fakeBeginner so
// every unit-test harness can construct a txStoreFactory and assert against the
// repos without repeating five field declarations. Harness structs embed it
// anonymously, so h.users, h.beginner, etc. work directly.
type fakeStores struct {
	users    *fakeUserRepo
	codes    *fakeCodeRepo
	attempts *fakeAttemptRepo
	sessions *fakeSessionRepo
	beginner *fakeBeginner
}

// newFakeStores builds a fresh set of shared identity fakes plus a fakeBeginner.
func newFakeStores() *fakeStores {
	return &fakeStores{
		users:    newFakeUserRepo(),
		codes:    newFakeCodeRepo(),
		attempts: newFakeAttemptRepo(),
		sessions: newFakeSessionRepo(),
		beginner: &fakeBeginner{},
	}
}

// factory builds a txStoreFactory from the fakes plus a fakeUoW. audit defaults
// to nil (NewTxStoreFactory substitutes Noop); pass a non-nil recorder (e.g.
// *recordingRecorder) when the test checks audit output.
func (s *fakeStores) factory(audit auditapp.Recorder) txStoreFactory {
	return NewTxStoreFactory(s.users, s.codes, s.attempts, s.sessions, audit, &fakeUoW{beginner: s.beginner})
}

// TestRunInTx_BuildsStoresFromTxAndCommits proves runInTx binds every
// repository and the audit recorder to the same transaction, runs work, and the
// UoW commits on a nil error.
func TestRunInTx_BuildsStoresFromTxAndCommits(t *testing.T) {
	f, b, users, codes, attempts, sessions, audit := newCountingFactory(t)

	workCalled := false
	var got txStores
	err := f.runInTx(t.Context(), func(s *txStores) error {
		workCalled = true
		got = *s
		return nil
	})
	if err != nil {
		t.Fatalf("runInTx returned %v, want nil", err)
	}
	if !workCalled {
		t.Fatal("work was not called")
	}

	// Each repository is bound to the transaction exactly once.
	if users.withTxCalls != 1 {
		t.Errorf("users.WithTx calls = %d, want 1", users.withTxCalls)
	}
	if codes.withTxCalls != 1 {
		t.Errorf("codes.WithTx calls = %d, want 1", codes.withTxCalls)
	}
	if attempts.withTxCalls != 1 {
		t.Errorf("attempts.WithTx calls = %d, want 1", attempts.withTxCalls)
	}
	if sessions.withTxCalls != 1 {
		t.Errorf("sessions.WithTx calls = %d, want 1", sessions.withTxCalls)
	}
	if audit.withTxCalls != 1 {
		t.Errorf("audit.WithTx calls = %d, want 1", audit.withTxCalls)
	}

	// The same transactional instances are handed to work.
	if got.users != users || got.codes != codes || got.attempts != attempts ||
		got.sessions != sessions {
		t.Error("work received stores that do not match the bound repositories")
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
	f, b, _, _, _, _, _ := newCountingFactory(t)

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

	_ = f.runInTx(t.Context(), func(*txStores) error {
		panic(panicVal)
	})
	t.Fatal("expected runInTx to re-panic")
}

// TestRunInTx_RollsBackOnWorkError proves a non-nil work error rolls the
// transaction back and is returned to the caller.
func TestRunInTx_RollsBackOnWorkError(t *testing.T) {
	f, b, _, _, _, _, _ := newCountingFactory(t)

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

// TestRunInTx_WrapsWithTxError proves a repository WithTx failure aborts the
// transaction with a wrapped error rather than proceeding with an unbound store.
func TestRunInTx_WrapsWithTxError(t *testing.T) {
	b := &fakeBeginner{}
	bindErr := errors.New("bind failed")
	users := &countingUserRepo{fakeUserRepo: newFakeUserRepo(), withTxErr: bindErr}
	f := &txStoreFactory{
		users:    users,
		codes:    &countingCodeRepo{fakeCodeRepo: newFakeCodeRepo()},
		attempts: &countingAttemptRepo{fakeAttemptRepo: newFakeAttemptRepo()},
		sessions: &countingSessionRepo{fakeSessionRepo: newFakeSessionRepo()},
		audit:    &countingRecorder{},
		uow:      &fakeUoW{beginner: b},
	}

	workCalled := false
	err := f.runInTx(t.Context(), func(*txStores) error {
		workCalled = true
		return nil
	})

	if !errors.Is(err, bindErr) {
		t.Fatalf("runInTx returned %v, want wrap of %v", err, bindErr)
	}
	if workCalled {
		t.Fatal("work must not run when a repository fails to bind")
	}
	// The transaction is opened but never committed; UoW rolls it back on the
	// returned error.
	if b.committed != 0 {
		t.Errorf("committed = %d, want 0", b.committed)
	}
	if b.open != 0 {
		t.Errorf("open = %d, want 0 (transaction left open)", b.open)
	}
}

// TestRunInTx_ReturnsErrorWhenUoWMissing proves a service that forgot to wire a
// UoW fails loudly at the call site instead of nil-dereferencing.
func TestRunInTx_ReturnsErrorWhenUoWMissing(t *testing.T) {
	f := &txStoreFactory{
		users:    &countingUserRepo{fakeUserRepo: newFakeUserRepo()},
		codes:    &countingCodeRepo{fakeCodeRepo: newFakeCodeRepo()},
		attempts: &countingAttemptRepo{fakeAttemptRepo: newFakeAttemptRepo()},
		sessions: &countingSessionRepo{fakeSessionRepo: newFakeSessionRepo()},
		audit:    &countingRecorder{},
		// uow intentionally nil
	}

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

// storesSentinelError is a typed panic value so the re-panic test can distinguish a
// real re-panic from any stray panic in test machinery.
type storesSentinelError struct{ msg string }

func (s storesSentinelError) Error() string { return "stores sentinel panic: " + s.msg }
