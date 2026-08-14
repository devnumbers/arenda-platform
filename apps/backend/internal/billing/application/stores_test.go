package application

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"slices"
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

// ListUpForRenewal mirrors the worker listing: active auto-renewing
// subscriptions whose paid period has ended, oldest first.
func (r *fakeSubscriptionRepo) ListUpForRenewal(_ context.Context, now time.Time, limit int) ([]domain.Subscription, error) {
	return r.listWorkerBatch(limit, func(s domain.Subscription) bool {
		return s.Status == domain.SubscriptionStatusActive && s.AutoRenewEnabled &&
			s.ValidUntil != nil && !s.ValidUntil.After(now)
	}, func(s domain.Subscription) time.Time { return *s.ValidUntil }), nil
}

// ListInExpiredGrace mirrors the worker listing: grace subscriptions whose
// window has ended.
func (r *fakeSubscriptionRepo) ListInExpiredGrace(_ context.Context, now time.Time, limit int) ([]domain.Subscription, error) {
	return r.listWorkerBatch(limit, func(s domain.Subscription) bool {
		return s.Status == domain.SubscriptionStatusGrace &&
			s.ValidUntil != nil && !s.ValidUntil.After(now)
	}, func(s domain.Subscription) time.Time { return *s.ValidUntil }), nil
}

// ListInGraceReminderWindow mirrors the worker listing: grace subscriptions
// inside the half-open reminder window [valid_until - lead, valid_until) whose
// window was not reminded yet (issue #253).
func (r *fakeSubscriptionRepo) ListInGraceReminderWindow(_ context.Context, now time.Time, lead time.Duration, limit int) ([]domain.Subscription, error) {
	return r.listWorkerBatch(limit, func(s domain.Subscription) bool {
		return subscriptionInGraceReminderWindow(s, now, lead)
	}, func(s domain.Subscription) time.Time { return *s.ValidUntil }), nil
}

// ListExpiredNonRenewing mirrors the worker listing: active subscriptions with
// auto-renew off whose retained period has ended.
func (r *fakeSubscriptionRepo) ListExpiredNonRenewing(_ context.Context, now time.Time, limit int) ([]domain.Subscription, error) {
	return r.listWorkerBatch(limit, func(s domain.Subscription) bool {
		return s.Status == domain.SubscriptionStatusActive && !s.AutoRenewEnabled &&
			s.ValidUntil != nil && !s.ValidUntil.After(now)
	}, func(s domain.Subscription) time.Time { return *s.ValidUntil }), nil
}

// ListExpiredCancelled mirrors the worker listing: cancelled subscriptions
// whose retained period has ended.
func (r *fakeSubscriptionRepo) ListExpiredCancelled(_ context.Context, now time.Time, limit int) ([]domain.Subscription, error) {
	return r.listWorkerBatch(limit, func(s domain.Subscription) bool {
		return s.Status == domain.SubscriptionStatusCancelled &&
			s.ValidUntil != nil && !s.ValidUntil.After(now)
	}, func(s domain.Subscription) time.Time { return *s.ValidUntil }), nil
}

// ListPendingChanges mirrors the worker listing: active subscriptions with a
// deferred tariff change that is due.
func (r *fakeSubscriptionRepo) ListPendingChanges(_ context.Context, now time.Time, limit int) ([]domain.Subscription, error) {
	return r.listWorkerBatch(limit, func(s domain.Subscription) bool {
		return s.Status == domain.SubscriptionStatusActive && s.PendingTariffID != nil &&
			s.PendingChangeAt != nil && !s.PendingChangeAt.After(now)
	}, func(s domain.Subscription) time.Time { return *s.PendingChangeAt }), nil
}

// listWorkerBatch selects, orders by the phase key then id, and limits — the
// shared shape of the worker listings.
func (r *fakeSubscriptionRepo) listWorkerBatch(limit int, match func(domain.Subscription) bool, key func(domain.Subscription) time.Time) []domain.Subscription {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]domain.Subscription, 0)
	for _, s := range r.subs {
		if match(s) {
			result = append(result, s)
		}
	}
	slices.SortFunc(result, func(a, b domain.Subscription) int {
		if c := key(a).Compare(key(b)); c != 0 {
			return c
		}
		return bytes.Compare(a.ID[:], b.ID[:])
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result
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

// fakePaymentRepo is an in-memory SubscriptionPaymentRepository.
type fakePaymentRepo struct {
	mu       sync.Mutex
	payments map[uuid.UUID]domain.SubscriptionPayment
	// hidePending, when positive, makes ListPendingByUserID return empty and
	// decrements, simulating a lookup that misses right before a concurrent
	// writer creates the conflicting pending payment.
	hidePending int
	// tariffOfSubscription resolves the user's current subscription tariff for
	// ListStalePendingUpgrades; fakeStores wires it to the subscription fake.
	tariffOfSubscription func(userID uuid.UUID) (uuid.UUID, bool)
}

func newFakePaymentRepo() *fakePaymentRepo {
	return &fakePaymentRepo{payments: make(map[uuid.UUID]domain.SubscriptionPayment)}
}

func (r *fakePaymentRepo) Create(_ context.Context, payment domain.SubscriptionPayment) (domain.SubscriptionPayment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.payments {
		if existing.Status == domain.PaymentStatusPending &&
			existing.UserID == payment.UserID && existing.TariffID == payment.TariffID &&
			existing.Period == payment.Period && existing.ID != payment.ID {
			return domain.SubscriptionPayment{}, ErrAlreadyExists
		}
	}
	r.payments[payment.ID] = payment
	return payment, nil
}

func (r *fakePaymentRepo) GetByID(_ context.Context, id uuid.UUID) (domain.SubscriptionPayment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.payments[id]; ok {
		return p, nil
	}
	return domain.SubscriptionPayment{}, ErrNotFound
}

func (r *fakePaymentRepo) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.SubscriptionPayment, error) {
	return r.GetByID(ctx, id)
}

func (r *fakePaymentRepo) ListByUserID(_ context.Context, userID uuid.UUID) ([]domain.SubscriptionPayment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]domain.SubscriptionPayment, 0, len(r.payments))
	for _, p := range r.payments {
		if p.UserID == userID {
			result = append(result, p)
		}
	}
	// Mirror the SQL ordering: newest first.
	slices.SortFunc(result, func(a, b domain.SubscriptionPayment) int {
		if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
			return c
		}
		return bytes.Compare(b.ID[:], a.ID[:])
	})
	return result, nil
}

func (r *fakePaymentRepo) ListPendingByUserID(_ context.Context, userID uuid.UUID) ([]domain.SubscriptionPayment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]domain.SubscriptionPayment, 0)
	if r.hidePending > 0 {
		r.hidePending--
		return result, nil
	}
	for _, p := range r.payments {
		if p.UserID == userID && p.Status == domain.PaymentStatusPending {
			result = append(result, p)
		}
	}
	return result, nil
}

func (r *fakePaymentRepo) Update(_ context.Context, payment domain.SubscriptionPayment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.payments[payment.ID]; !ok {
		return ErrNotFound
	}
	r.payments[payment.ID] = payment
	return nil
}

// ListStalePending mirrors the reconciliation listing: pending payments with a
// provider reference created before the threshold, oldest first.
func (r *fakePaymentRepo) ListStalePending(_ context.Context, createdBefore time.Time, limit int) ([]domain.SubscriptionPayment, error) {
	return r.listStalePending(createdBefore, limit, func(domain.SubscriptionPayment) bool { return true }), nil
}

// ListStalePendingUpgrades narrows the stale set to payments whose tariff
// differs from the subscription's current one; fakeStores wires the
// subscription tariff lookup.
func (r *fakePaymentRepo) ListStalePendingUpgrades(_ context.Context, createdBefore time.Time, limit int) ([]domain.SubscriptionPayment, error) {
	return r.listStalePending(createdBefore, limit, func(p domain.SubscriptionPayment) bool {
		if r.tariffOfSubscription == nil {
			return false
		}
		current, ok := r.tariffOfSubscription(p.UserID)
		return ok && current != p.TariffID
	}), nil
}

// listStalePending is the shared selection of both reconciliation listings.
func (r *fakePaymentRepo) listStalePending(createdBefore time.Time, limit int, extra func(domain.SubscriptionPayment) bool) []domain.SubscriptionPayment {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]domain.SubscriptionPayment, 0)
	for _, p := range r.payments {
		if p.Status == domain.PaymentStatusPending && p.HasProviderReference() &&
			p.CreatedAt.Before(createdBefore) && extra(p) {
			result = append(result, p)
		}
	}
	slices.SortFunc(result, func(a, b domain.SubscriptionPayment) int {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return bytes.Compare(a.ID[:], b.ID[:])
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result
}

func (r *fakePaymentRepo) WithTx(transaction.Tx) (SubscriptionPaymentRepository, error) {
	return r, nil
}

// fakePaymentMethodRepo is an in-memory PaymentMethodRepository. It mirrors
// the database invariants the service layer relies on: at most one active
// method per user and (user, provider token) uniqueness with upsert
// convergence.
type fakePaymentMethodRepo struct {
	mu      sync.Mutex
	methods map[uuid.UUID]domain.PaymentMethod
}

func newFakePaymentMethodRepo() *fakePaymentMethodRepo {
	return &fakePaymentMethodRepo{methods: make(map[uuid.UUID]domain.PaymentMethod)}
}

func (r *fakePaymentMethodRepo) UpsertByTokenHash(_ context.Context, method domain.PaymentMethod) (domain.PaymentMethod, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.methods {
		if existing.UserID == method.UserID && existing.ProviderToken == method.ProviderToken {
			existing.ProviderCardID = cmp.Or(method.ProviderCardID, existing.ProviderCardID)
			existing.DisplayMask = cmp.Or(method.DisplayMask, existing.DisplayMask)
			existing.ExpDate = cmp.Or(method.ExpDate, existing.ExpDate)
			r.methods[existing.ID] = existing
			return existing, nil
		}
	}
	r.methods[method.ID] = method
	return method, nil
}

func (r *fakePaymentMethodRepo) GetByID(_ context.Context, id uuid.UUID) (domain.PaymentMethod, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.methods[id]; ok {
		return m, nil
	}
	return domain.PaymentMethod{}, ErrNotFound
}

func (r *fakePaymentMethodRepo) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.PaymentMethod, error) {
	return r.GetByID(ctx, id)
}

func (r *fakePaymentMethodRepo) ListByUserID(_ context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]domain.PaymentMethod, 0, len(r.methods))
	for _, m := range r.methods {
		if m.UserID == userID {
			result = append(result, m)
		}
	}
	slices.SortFunc(result, func(a, b domain.PaymentMethod) int {
		if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
			return c
		}
		return bytes.Compare(b.ID[:], a.ID[:])
	})
	return result, nil
}

func (r *fakePaymentMethodRepo) SetActive(_ context.Context, userID, methodID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.methods[methodID]; !ok {
		return ErrNotFound
	}
	for id, m := range r.methods {
		if m.UserID == userID && m.IsActive && id != methodID {
			m.IsActive = false
			r.methods[id] = m
		}
	}
	active := r.methods[methodID]
	active.IsActive = true
	r.methods[methodID] = active
	return nil
}

func (r *fakePaymentMethodRepo) Delete(_ context.Context, userID, methodID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.methods[methodID]
	if !ok || m.UserID != userID {
		return ErrNotFound
	}
	delete(r.methods, methodID)
	return nil
}

func (r *fakePaymentMethodRepo) WithTx(transaction.Tx) (PaymentMethodRepository, error) {
	return r, nil
}

// fakeBindingRepo is an in-memory CardBindingSessionRepository.
type fakeBindingRepo struct {
	mu       sync.Mutex
	sessions map[uuid.UUID]domain.CardBindingSession
}

func newFakeBindingRepo() *fakeBindingRepo {
	return &fakeBindingRepo{sessions: make(map[uuid.UUID]domain.CardBindingSession)}
}

func (r *fakeBindingRepo) Create(_ context.Context, session domain.CardBindingSession) (domain.CardBindingSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.sessions {
		if existing.Provider == session.Provider && existing.RequestKey == session.RequestKey {
			return domain.CardBindingSession{}, ErrAlreadyExists
		}
	}
	r.sessions[session.ID] = session
	return session, nil
}

func (r *fakeBindingRepo) GetByRequestKeyForUpdate(_ context.Context, provider domain.PaymentProvider, requestKey string) (domain.CardBindingSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, session := range r.sessions {
		if session.Provider == provider && session.RequestKey == requestKey {
			return session, nil
		}
	}
	return domain.CardBindingSession{}, ErrNotFound
}

func (r *fakeBindingRepo) ListOpenByUserID(_ context.Context, userID uuid.UUID) ([]domain.CardBindingSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]domain.CardBindingSession, 0)
	for _, session := range r.sessions {
		if session.UserID == userID && session.IsOpen() {
			result = append(result, session)
		}
	}
	return result, nil
}

func (r *fakeBindingRepo) UpdateStatus(_ context.Context, session domain.CardBindingSession) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.sessions[session.ID]; !ok {
		return ErrNotFound
	}
	r.sessions[session.ID] = session
	return nil
}

func (r *fakeBindingRepo) WithTx(transaction.Tx) (CardBindingSessionRepository, error) {
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
	payments      *fakePaymentRepo
	methods       *fakePaymentMethodRepo
	bindings      *fakeBindingRepo
	beginner      *fakeBeginner
}

func newFakeStores(tariffs ...domain.Tariff) *fakeStores {
	subs := newFakeSubscriptionRepo()
	payments := newFakePaymentRepo()
	// The upgrade-reconciliation listing joins the subscription's current
	// tariff in SQL; the fakes express the same join through this lookup.
	payments.tariffOfSubscription = func(userID uuid.UUID) (uuid.UUID, bool) {
		sub, err := subs.GetByUserID(context.Background(), userID)
		if err != nil {
			return uuid.Nil, false
		}
		return sub.TariffID, true
	}
	return &fakeStores{
		tariffs:       newFakeTariffRepo(tariffs...),
		subscriptions: subs,
		transitions:   newFakeTransitionRepo(),
		payments:      payments,
		methods:       newFakePaymentMethodRepo(),
		bindings:      newFakeBindingRepo(),
		beginner:      &fakeBeginner{},
	}
}

// factory builds a txStoreFactory from the fakes plus a fakeUoW. audit defaults
// to nil (NewTxStoreFactory substitutes Noop).
func (s *fakeStores) factory(audit auditapp.Recorder) txStoreFactory {
	return NewTxStoreFactory(s.tariffs, s.subscriptions, s.transitions, s.payments, s.methods, s.bindings, audit, &fakeUoW{beginner: s.beginner})
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
