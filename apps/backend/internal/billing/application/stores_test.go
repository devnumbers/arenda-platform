package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// fakeTx is a no-op transaction that records commit/rollback.
type fakeTx struct {
	mu         sync.Mutex
	committed  int
	rolledBack int
}

func (t *fakeTx) Commit(context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.committed++
	return nil
}

func (t *fakeTx) Rollback(context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.rolledBack++
	return nil
}

// fakeBeginner hands out fakeTx instances and counts begins.
type fakeBeginner struct {
	mu       sync.Mutex
	begun    int
	open     int
	beginErr error
}

func (b *fakeBeginner) Begin(context.Context) (transaction.Tx, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.begun++
	if b.beginErr != nil {
		return nil, b.beginErr
	}
	b.open++
	return &fakeTx{}, nil
}

// fakeUoW adapts fakeBeginner to transaction.UoW with the same
// commit-on-nil / rollback-on-error / panic-reraise semantics as the postgres
// adapter, so the stores tests run without a database.
type fakeUoW struct {
	beginner *fakeBeginner
}

func (u *fakeUoW) Do(ctx context.Context, work func(tx transaction.Tx) error) error {
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

// fakeTariffRepo is an in-memory TariffRepository.
type fakeTariffRepo struct {
	mu      sync.Mutex
	tariffs []domain.Tariff
}

func newFakeTariffRepo(tariffs ...domain.Tariff) *fakeTariffRepo {
	return &fakeTariffRepo{tariffs: tariffs}
}

func (r *fakeTariffRepo) GetByID(_ context.Context, id uuid.UUID) (domain.Tariff, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.tariffs {
		if t.ID == id {
			return t, nil
		}
	}
	return domain.Tariff{}, ErrNotFound
}

func (r *fakeTariffRepo) GetByName(_ context.Context, name domain.TariffName) (domain.Tariff, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.tariffs {
		if t.Name == name {
			return t, nil
		}
	}
	return domain.Tariff{}, ErrNotFound
}

func (r *fakeTariffRepo) List(context.Context) ([]domain.Tariff, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	active := make([]domain.Tariff, 0, len(r.tariffs))
	for _, t := range r.tariffs {
		if t.IsActive {
			active = append(active, t)
		}
	}
	return active, nil
}

func (r *fakeTariffRepo) ListAll(context.Context) ([]domain.Tariff, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	all := make([]domain.Tariff, len(r.tariffs))
	copy(all, r.tariffs)
	return all, nil
}

func (r *fakeTariffRepo) WithTx(transaction.Tx) (TariffRepository, error) { return r, nil }

// fakeSubscriptionRepo is an in-memory SubscriptionRepository.
type fakeSubscriptionRepo struct {
	mu             sync.Mutex
	subs           map[uuid.UUID]domain.Subscription
	forUpdateCalls int
}

func newFakeSubscriptionRepo() *fakeSubscriptionRepo {
	return &fakeSubscriptionRepo{subs: make(map[uuid.UUID]domain.Subscription)}
}

func (r *fakeSubscriptionRepo) GetByUserID(_ context.Context, userID uuid.UUID) (domain.Subscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sub, ok := r.subs[userID]
	if !ok {
		return domain.Subscription{}, ErrNotFound
	}
	return sub, nil
}

func (r *fakeSubscriptionRepo) GetByUserIDForUpdate(ctx context.Context, userID uuid.UUID) (domain.Subscription, error) {
	r.mu.Lock()
	r.forUpdateCalls++
	r.mu.Unlock()
	return r.GetByUserID(ctx, userID)
}

func (r *fakeSubscriptionRepo) Create(_ context.Context, sub domain.Subscription) (domain.Subscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.subs[sub.UserID]; ok {
		return existing, nil
	}
	r.subs[sub.UserID] = sub
	return sub, nil
}

func (r *fakeSubscriptionRepo) Update(_ context.Context, sub domain.Subscription) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.subs[sub.UserID]; !ok {
		return ErrNotFound
	}
	r.subs[sub.UserID] = sub
	return nil
}

func (r *fakeSubscriptionRepo) WithTx(transaction.Tx) (SubscriptionRepository, error) { return r, nil }

// fakeTransitionRepo is an in-memory SubscriptionTransitionRepository.
type fakeTransitionRepo struct {
	mu          sync.Mutex
	transitions []domain.Transition
}

func newFakeTransitionRepo() *fakeTransitionRepo { return &fakeTransitionRepo{} }

func (r *fakeTransitionRepo) Append(_ context.Context, transition domain.Transition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transitions = append(r.transitions, transition)
	return nil
}

func (r *fakeTransitionRepo) ListBySubscriptionID(_ context.Context, subscriptionID uuid.UUID) ([]domain.Transition, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]domain.Transition, 0, len(r.transitions))
	for _, t := range r.transitions {
		if t.SubscriptionID == subscriptionID {
			result = append(result, t)
		}
	}
	return result, nil
}

func (r *fakeTransitionRepo) WithTx(transaction.Tx) (SubscriptionTransitionRepository, error) {
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

// fakeClock is a fixed clock.Clock.
type fakeClock struct{ now time.Time }

func (c fakeClock) Now() time.Time { return c.now }

// fakeStores bundles the billing fake repositories and a fakeBeginner so every
// unit-test harness can construct a txStoreFactory without repeating field
// declarations.
type fakeStores struct {
	tariffs       *fakeTariffRepo
	subscriptions *fakeSubscriptionRepo
	transitions   *fakeTransitionRepo
	beginner      *fakeBeginner
}

func newFakeStores(tariffs ...domain.Tariff) *fakeStores {
	return &fakeStores{
		tariffs:       newFakeTariffRepo(tariffs...),
		subscriptions: newFakeSubscriptionRepo(),
		transitions:   newFakeTransitionRepo(),
		beginner:      &fakeBeginner{},
	}
}

// factory builds a txStoreFactory from the fakes plus a fakeUoW. audit defaults
// to nil (NewTxStoreFactory substitutes Noop).
func (s *fakeStores) factory(audit auditapp.Recorder) txStoreFactory {
	return NewTxStoreFactory(s.tariffs, s.subscriptions, s.transitions, audit, &fakeUoW{beginner: s.beginner})
}

// TestRunInTx_BuildsStoresFromTxAndCommits proves runInTx binds every
// repository and the audit recorder to the same transaction, runs work, and the
// UoW commits on a nil error.
func TestRunInTx_BuildsStoresFromTxAndCommits(t *testing.T) {
	audit := &countingRecorder{}
	f := newFakeStores().factory(audit)

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
	if audit.withTxCalls != 1 {
		t.Errorf("audit.WithTx calls = %d, want 1", audit.withTxCalls)
	}
	if got.tariffs == nil || got.subscriptions == nil || got.transitions == nil {
		t.Error("work received stores with an unbound repository")
	}
}

// TestRunInTx_PanicRollsBackAndRepanics proves a panic inside work rolls the
// transaction back and re-panics, so a panicking use case never leaks a tx.
func TestRunInTx_PanicRollsBackAndRepanics(t *testing.T) {
	f := newFakeStores().factory(nil)

	panicVal := errors.New("kaboom")
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected runInTx to re-panic, but it did not panic")
		}
		if err, ok := r.(error); !ok || !errors.Is(err, panicVal) {
			t.Fatalf("recovered %v, want %v", r, panicVal)
		}
	}()

	_ = f.runInTx(t.Context(), func(*txStores) error { panic(panicVal) })
	t.Fatal("expected runInTx to re-panic")
}

// TestRunInTx_RollsBackOnWorkError proves a non-nil work error is returned to
// the caller.
func TestRunInTx_RollsBackOnWorkError(t *testing.T) {
	f := newFakeStores().factory(nil)

	workErr := errors.New("business rule violated")
	err := f.runInTx(t.Context(), func(*txStores) error { return workErr })
	if !errors.Is(err, workErr) {
		t.Fatalf("runInTx returned %v, want %v", err, workErr)
	}
}

// TestRunInTx_ReturnsErrorWhenUoWMissing proves a service that forgot to wire a
// UoW fails loudly at the call site instead of nil-dereferencing.
func TestRunInTx_ReturnsErrorWhenUoWMissing(t *testing.T) {
	f := &txStoreFactory{
		tariffs:       newFakeTariffRepo(),
		subscriptions: newFakeSubscriptionRepo(),
		transitions:   newFakeTransitionRepo(),
		// uow intentionally nil
	}
	err := f.runInTx(t.Context(), func(*txStores) error { return nil })
	if err == nil {
		t.Fatal("runInTx returned nil, want error for missing UoW")
	}
}
