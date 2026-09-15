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

func (u *fakeUoW) Do(ctx context.Context, work func(tx transaction.Tx) error) (err error) {
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

// Create mirrors the database's name uniqueness (issue #256).
func (r *fakeTariffRepo) Create(_ context.Context, tariff domain.Tariff) (domain.Tariff, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.tariffs {
		if existing.Name == tariff.Name {
			return domain.Tariff{}, ErrAlreadyExists
		}
	}
	r.tariffs = append(r.tariffs, tariff)
	return tariff, nil
}

// Update keeps the name immutable, like the SQL query does (issue #256).
func (r *fakeTariffRepo) Update(_ context.Context, tariff domain.Tariff) (domain.Tariff, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, existing := range r.tariffs {
		if existing.ID == tariff.ID {
			updated := tariff
			updated.Name = existing.Name
			r.tariffs[i] = updated
			return updated, nil
		}
	}
	return domain.Tariff{}, ErrNotFound
}

// Invalidate is a no-op: the fake caches nothing and its WithTx returns
// itself, so writes are visible to reads immediately.
func (r *fakeTariffRepo) Invalidate(context.Context) error { return nil }

func (r *fakeTariffRepo) WithTx(transaction.Tx) (TariffRepository, error) { return r, nil }

// fakeSubscriptionRepo is an in-memory SubscriptionRepository.
type fakeSubscriptionRepo struct {
	mu             sync.Mutex
	subs           map[uuid.UUID]domain.Subscription
	forUpdateCalls int
	// GraceRetryDue mirrors the SQL grace-retry bound (ticket #431): the
	// latest grace-entered transition anchors the schedule, payments since it
	// consume the boundaries. The fakeStores constructor wires it over the
	// transition and payment fakes, the way the SQL query joins the same
	// tables.
	graceRetryDue func(sub domain.Subscription, now time.Time) bool
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

// List interprets the worker selection the way the SQL adapter does (issue
// #286): this mirror is the fake's only statement of batch semantics, and the
// per-Selection integration tests pin it to the real query.
func (r *fakeSubscriptionRepo) List(_ context.Context, sel SubscriptionSelection) ([]domain.Subscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]domain.Subscription, 0)
	for _, s := range r.subs {
		if subscriptionInSelection(s, sel, r.graceRetryDue) {
			result = append(result, s)
		}
	}
	// Mirror the SQL order: the pending-change clock for deferred-change
	// batches, the validity clock otherwise, each with id as the tie-breaker;
	// a nil clock sorts last, as PostgreSQL's NULLS LAST.
	clock := func(s domain.Subscription) time.Time {
		if s.ValidUntil != nil {
			return *s.ValidUntil
		}
		return farFuture
	}
	if sel.PendingChangeDue != nil {
		clock = func(s domain.Subscription) time.Time {
			if s.PendingChangeAt != nil {
				return *s.PendingChangeAt
			}
			return farFuture
		}
	}
	slices.SortFunc(result, func(a, b domain.Subscription) int {
		if c := clock(a).Compare(clock(b)); c != 0 {
			return c
		}
		return bytes.Compare(a.ID[:], b.ID[:])
	})
	if len(result) > sel.Limit {
		result = result[:sel.Limit]
	}
	return result, nil
}

// farFuture mirrors NULLS LAST: a row without the ordering clock sorts after
// every dated row.
var farFuture = time.Unix(1<<62, 0)

// subscriptionInSelection is the fake's reading of the worker selection — the
// Go mirror of ListSubscriptionsBySelection's predicate — composed of the
// sub-predicates below (identity, toggles, validity window, pending change).
func subscriptionInSelection(s domain.Subscription, sel SubscriptionSelection, graceRetry func(domain.Subscription, time.Time) bool) bool {
	return subscriptionIdentityInSelection(s, sel) &&
		subscriptionTogglesInSelection(s, sel) &&
		subscriptionValidityInSelection(s, sel) &&
		subscriptionPendingChangeInSelection(s, sel) &&
		subscriptionGraceRetryInSelection(s, sel, graceRetry)
}

// subscriptionIdentityInSelection matches the user filter and the exact status.
func subscriptionIdentityInSelection(s domain.Subscription, sel SubscriptionSelection) bool {
	if sel.UserID != nil && s.UserID != *sel.UserID {
		return false
	}
	return s.Status == sel.Status
}

// subscriptionTogglesInSelection matches the auto-renew flag and the
// not-yet-reminded-in-grace flag.
func subscriptionTogglesInSelection(s domain.Subscription, sel SubscriptionSelection) bool {
	if sel.AutoRenewEnabled != nil && s.AutoRenewEnabled != *sel.AutoRenewEnabled {
		return false
	}
	return !sel.Unreminded || s.GraceRemindedAt == nil
}

// subscriptionValidityInSelection matches the validity window: a row without
// ValidUntil never falls inside either boundary.
func subscriptionValidityInSelection(s domain.Subscription, sel SubscriptionSelection) bool {
	if sel.ValidUntilBefore != nil && (s.ValidUntil == nil || s.ValidUntil.After(*sel.ValidUntilBefore)) {
		return false
	}
	if sel.ValidUntilAfter != nil && (s.ValidUntil == nil || !s.ValidUntil.After(*sel.ValidUntilAfter)) {
		return false
	}
	return true
}

// subscriptionPendingChangeInSelection matches the due-pending-change filter:
// a row qualifies only with both pending fields set and the change clock due.
func subscriptionPendingChangeInSelection(s domain.Subscription, sel SubscriptionSelection) bool {
	if sel.PendingChangeDue == nil {
		return true
	}
	return s.PendingTariffID != nil && s.PendingChangeAt != nil && !s.PendingChangeAt.After(*sel.PendingChangeDue)
}

// subscriptionGraceRetryInSelection matches the dunning-retry bound of the
// grace-retry batch (ticket #431) — the Go mirror of the grace_retry_due
// predicate: an open window, a linked active method, a grace entry old enough
// for the next scheduled retry (+24 h / +72 h) and no payment created since
// the entry consuming that boundary. The cross-table halves (the latest
// grace-entered transition and the payments since it) resolve through the
// hook fakeStores wires; without it the bound matches nothing, the way a row
// without a grace-entered transition never matches in SQL.
func subscriptionGraceRetryInSelection(
	s domain.Subscription, sel SubscriptionSelection, graceRetry func(domain.Subscription, time.Time) bool,
) bool {
	if sel.GraceRetryDue == nil {
		return true
	}
	if s.ValidUntil == nil || !s.ValidUntil.After(*sel.GraceRetryDue) {
		return false
	}
	if s.ActivePaymentMethodID == nil {
		return false
	}
	if graceRetry == nil {
		return false
	}
	return graceRetry(s, *sel.GraceRetryDue)
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

func (r *fakeTransitionRepo) ShiftGraceEntryTimes(_ context.Context, subscriptionID uuid.UUID, delta time.Duration) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	moved := 0
	for i := range r.transitions {
		t := &r.transitions[i]
		if t.SubscriptionID == subscriptionID && t.Reason == domain.TransitionReasonGraceEntered {
			t.CreatedAt = t.CreatedAt.Add(delta)
			moved++
		}
	}
	return moved, nil
}

func (r *fakeTransitionRepo) WithTx(transaction.Tx) (SubscriptionTransitionRepository, error) {
	return r, nil
}

// fakePaymentRepo is an in-memory SubscriptionPaymentRepository.
type fakePaymentRepo struct {
	mu       sync.Mutex
	payments map[uuid.UUID]domain.SubscriptionPayment
	// HidePending, when positive, makes ListPendingByUserID return empty and
	// decrements, simulating a lookup that misses right before a concurrent
	// writer creates the conflicting pending payment.
	hidePending int
	// TariffOfSubscription resolves the user's current subscription tariff for
	// the tariff-change narrowing of the payment selection; fakeStores wires it
	// to the subscription fake.
	tariffOfSubscription func(userID uuid.UUID) (uuid.UUID, bool)
}

func newFakePaymentRepo() *fakePaymentRepo {
	return &fakePaymentRepo{payments: make(map[uuid.UUID]domain.SubscriptionPayment)}
}

// Create mirrors the database's pending-payment backstops (issue #690): the
// same-target duplicate answers ErrAlreadyExists — the caller resolves it to
// the existing pending payment; a form insert (deadline set) meets any other
// pending form of the user — even a dead one the TTL worker has not reached
// yet — and answers ErrPendingPaymentExists, the conflict the sweep must
// clear first. A merchant-initiated charge (no deadline) is outside the form
// index entirely, in either role.
func (r *fakePaymentRepo) Create(_ context.Context, payment domain.SubscriptionPayment) (domain.SubscriptionPayment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.payments {
		if existing.Status != domain.PaymentStatusPending || existing.ID == payment.ID {
			continue
		}
		if existing.UserID == payment.UserID && existing.TariffID == payment.TariffID &&
			existing.Period == payment.Period {
			return domain.SubscriptionPayment{}, ErrAlreadyExists
		}
		if existing.UserID == payment.UserID && payment.ExpiresAt != nil && existing.ExpiresAt != nil {
			return domain.SubscriptionPayment{}, ErrPendingPaymentExists
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
	return r.listByUserIDLocked(userID), nil
}

// listByUserIDLocked collects the user's payments newest-first; r.mu must be
// held.
func (r *fakePaymentRepo) listByUserIDLocked(userID uuid.UUID) []domain.SubscriptionPayment {
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
	return result
}

func (r *fakePaymentRepo) ListByUserIDWithCard(_ context.Context, userID uuid.UUID) ([]SubscriptionPaymentWithCard, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	payments := r.listByUserIDLocked(userID)
	result := make([]SubscriptionPaymentWithCard, 0, len(payments))
	for _, p := range payments {
		result = append(result, SubscriptionPaymentWithCard{Payment: p, ResolvedCardMask: p.CardMask})
	}
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

// ListExpiredPending mirrors the SQL predicate of the TTL-expiry batch
// (issue #616): still-pending payments with a deadline in the past, oldest
// deadline first.
func (r *fakePaymentRepo) ListExpiredPending(_ context.Context, before time.Time, limit int) ([]domain.SubscriptionPayment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]domain.SubscriptionPayment, 0)
	for _, p := range r.payments {
		if p.UserID == uuid.Nil || p.Status != domain.PaymentStatusPending {
			continue
		}
		if p.ExpiresAt == nil || !p.ExpiresAt.Before(before) {
			continue
		}
		result = append(result, p)
	}
	slices.SortFunc(result, func(a, b domain.SubscriptionPayment) int {
		if c := a.ExpiresAt.Compare(*b.ExpiresAt); c != 0 {
			return c
		}
		return bytes.Compare(a.ID[:], b.ID[:])
	})
	if len(result) > limit {
		result = result[:limit]
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

// List interprets the worker selection the way the SQL adapter does (issue
// #286): the provider reference is mandatory, staleness is strict, and the
// order follows the staleness clock — updated for updated-stale batches,
// created otherwise, each with id as the tie-breaker.
func (r *fakePaymentRepo) List(_ context.Context, sel PaymentSelection) ([]domain.SubscriptionPayment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]domain.SubscriptionPayment, 0)
	for _, p := range r.payments {
		if !paymentInSelection(p, sel, r.tariffOfSubscription) {
			continue
		}
		result = append(result, p)
	}
	clock := func(p domain.SubscriptionPayment) time.Time { return p.CreatedAt }
	if sel.UpdatedBefore != nil {
		clock = func(p domain.SubscriptionPayment) time.Time { return p.UpdatedAt }
	}
	slices.SortFunc(result, func(a, b domain.SubscriptionPayment) int {
		if c := clock(a).Compare(clock(b)); c != 0 {
			return c
		}
		return bytes.Compare(a.ID[:], b.ID[:])
	})
	if len(result) > sel.Limit {
		result = result[:sel.Limit]
	}
	return result, nil
}

// Count counts the whole selection, ignoring its limit — the gauge shape of
// the hygiene phase (ticket #433).
func (r *fakePaymentRepo) Count(ctx context.Context, sel PaymentSelection) (int64, error) {
	// A zero limit would truncate the fake's List, so the count re-lists
	// through an unbounded copy of the selection.
	sel.Limit = int(^uint(0) >> 1)
	payments, err := r.List(ctx, sel)
	if err != nil {
		return 0, err
	}
	return int64(len(payments)), nil
}

// paymentInSelection is the fake's reading of the worker selection — the Go
// mirror of ListSubscriptionPaymentsBySelection's predicate, including the
// tariff-change join the fake resolves through tariffOfSubscription.
func paymentInSelection(p domain.SubscriptionPayment, sel PaymentSelection, tariffOfSubscription func(uuid.UUID) (uuid.UUID, bool)) bool {
	if p.Status != sel.Status || !p.HasProviderReference() {
		return false
	}
	if sel.CreatedBefore != nil && !p.CreatedAt.Before(*sel.CreatedBefore) {
		return false
	}
	if sel.UpdatedBefore != nil && !p.UpdatedAt.Before(*sel.UpdatedBefore) {
		return false
	}
	if sel.TariffChangeOnly {
		if tariffOfSubscription == nil {
			return false
		}
		current, ok := tariffOfSubscription(p.UserID)
		if !ok || current == p.TariffID {
			return false
		}
	}
	return true
}

func (r *fakePaymentRepo) ShiftCreatedAt(_ context.Context, subscriptionID uuid.UUID, delta time.Duration) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	moved := int64(0)
	for id, p := range r.payments {
		if p.SubscriptionID == subscriptionID {
			p.CreatedAt = p.CreatedAt.Add(delta)
			r.payments[id] = p
			moved++
		}
	}
	return moved, nil
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

func (r *fakeBindingRepo) GetByRequestKeyForUpdate(
	_ context.Context, provider domain.PaymentProvider, requestKey string,
) (domain.CardBindingSession, error) {
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

func (r *fakeBindingRepo) CountStartedSince(_ context.Context, userID uuid.UUID, since time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, session := range r.sessions {
		if session.UserID == userID && !session.CreatedAt.Before(since.UTC()) {
			count++
		}
	}
	return count, nil
}

// DeleteExpired mirrors the SQL hygiene batch (ticket #433): sessions past
// their lifetime go, oldest first, capped by the selection's limit.
func (r *fakeBindingRepo) DeleteExpired(_ context.Context, sel CardBindingSelection) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	expired := make([]domain.CardBindingSession, 0)
	for _, session := range r.sessions {
		if session.IsExpired(sel.ExpiredBefore) {
			expired = append(expired, session)
		}
	}
	slices.SortFunc(expired, func(a, b domain.CardBindingSession) int {
		return a.ExpiresAt.Compare(b.ExpiresAt)
	})
	if len(expired) > sel.Limit {
		expired = expired[:sel.Limit]
	}
	for _, session := range expired {
		delete(r.sessions, session.ID)
	}
	return len(expired), nil
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
	transitions := newFakeTransitionRepo()
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
	// The grace-retry listing joins the subscription's latest grace-entered
	// transition and its payments since that entry in SQL (ticket #431); the
	// fake expresses the same joins through these lookups.
	subs.graceRetryDue = func(sub domain.Subscription, now time.Time) bool {
		list, err := transitions.ListBySubscriptionID(context.Background(), sub.ID)
		if err != nil {
			return false
		}
		var entered *time.Time
		for _, t := range list {
			if t.ToStatus != domain.SubscriptionStatusGrace || t.Reason != domain.TransitionReasonGraceEntered {
				continue
			}
			if entered == nil || t.CreatedAt.After(*entered) {
				created := t.CreatedAt
				entered = &created
			}
		}
		if entered == nil {
			return false
		}
		sinceEntry := 0
		for _, p := range payments.payments {
			if p.SubscriptionID == sub.ID && !p.CreatedAt.Before(*entered) {
				sinceEntry++
			}
		}
		return (!now.Before(entered.Add(graceRetryFirstAfter)) && sinceEntry == 0) ||
			(!now.Before(entered.Add(graceRetrySecondAfter)) && sinceEntry <= 1)
	}
	return &fakeStores{
		tariffs:       newFakeTariffRepo(tariffs...),
		subscriptions: subs,
		transitions:   transitions,
		payments:      payments,
		methods:       newFakePaymentMethodRepo(),
		bindings:      newFakeBindingRepo(),
		beginner:      &fakeBeginner{},
	}
}

// Factory builds a txStoreFactory from the fakes plus a fakeUoW. Audit defaults
// to nil (NewTxStoreFactory substitutes Noop).
func (s *fakeStores) factory(audit auditapp.Recorder) txStoreFactory {
	return NewTxStoreFactory(s.tariffs, s.subscriptions, s.transitions, s.payments,
		s.methods, s.bindings, audit, &fakeUoW{beginner: s.beginner})
}

// TestRunInTx_BuildsStoresFromTxAndCommits proves runInTx binds every
// repository and the audit recorder to the same transaction, runs work, and the
// UoW commits on a nil error.
func TestRunInTx_BuildsStoresFromTxAndCommits(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

	// Unreachable while runInTx honors the re-panic contract; if it ever
	// returns, fail with the value instead of discarding it.
	err := f.runInTx(t.Context(), func(*txStores) error { panic(panicVal) })
	t.Fatalf("runInTx returned %v, want re-panic", err)
}

// TestRunInTx_RollsBackOnWorkError proves a non-nil work error is returned to
// the caller.
func TestRunInTx_RollsBackOnWorkError(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	f := &txStoreFactory{
		tariffs:       newFakeTariffRepo(),
		subscriptions: newFakeSubscriptionRepo(),
		transitions:   newFakeTransitionRepo(),
		// The uow is intentionally nil.
	}
	err := f.runInTx(t.Context(), func(*txStores) error { return nil })
	if err == nil {
		t.Fatal("runInTx returned nil, want error for missing UoW")
	}
}
