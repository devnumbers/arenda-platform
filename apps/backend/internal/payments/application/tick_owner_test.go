package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTickStore is the in-memory TickStore double for the tickOwner
// orchestration test: it records the call order and serves a canned
// snapshot. The real-SQL properties of the same seam (WithTx binding,
// re-entrant property lock, atomicity with the caller's transaction) are
// covered by the adapter integration test over a rolled-back transaction
// (ADR 0033 test fixtures).
type fakeTickStore struct {
	locked   []uuid.UUID
	snapshot OwnerSnapshot
	applied  []appliedPlan
	withTxTx transaction.Tx

	lockErr    error
	snapshotEr error
	applyErr   error
}

type appliedPlan struct {
	payment uuid.UUID
	today   time.Time
	plan    domain.PaymentTickPlan
}

func (f *fakeTickStore) LockOwnerProperties(_ context.Context, ownerID uuid.UUID) error {
	if f.lockErr != nil {
		return f.lockErr
	}
	f.locked = append(f.locked, ownerID)
	return nil
}

func (f *fakeTickStore) LoadOwnerSnapshot(_ context.Context, _ uuid.UUID) (OwnerSnapshot, error) {
	return f.snapshot, f.snapshotEr
}

func (f *fakeTickStore) ApplyTickPlan(
	_ context.Context, p domain.Payment, today time.Time, plan domain.PaymentTickPlan,
) error {
	if f.applyErr != nil {
		return f.applyErr
	}
	f.applied = append(f.applied, appliedPlan{payment: p.ID, today: today, plan: plan})
	return nil
}

func (f *fakeTickStore) WithTx(tx transaction.Tx) (TickStore, error) {
	f.withTxTx = tx
	return f, nil
}

func tickOwnerPayment(t *testing.T, id uuid.UUID, since string) domain.Payment {
	t.Helper()
	parsed, err := time.Parse(time.DateOnly, since)
	require.NoError(t, err)
	return domain.Payment{
		ID:            id,
		OwnerID:       uuid.Must(uuid.NewV7()),
		PropertyID:    uuid.Must(uuid.NewV7()),
		Type:          domain.TypeExpense,
		Title:         "t",
		AmountKopecks: 100,
		Recurrence:    domain.NewDailyRecurrence(),
		Since:         parsed,
	}
}

func TestTickOwner_Orchestration(t *testing.T) {
	t.Parallel()
	owner := uuid.Must(uuid.NewV7())
	payment := tickOwnerPayment(t, uuid.Must(uuid.NewV7()), "2026-08-23")
	today, err := time.Parse(time.DateOnly, "2026-08-25")
	require.NoError(t, err)

	store := &fakeTickStore{
		snapshot: OwnerSnapshot{
			Payments: []domain.Payment{payment},
			Statuses: map[uuid.UUID]map[time.Time]domain.OperationStatus{
				payment.ID: {today.AddDate(0, 0, -1): domain.StatusPaid},
			},
		},
	}
	stores := &txStores{tick: store}

	require.NoError(t, stores.tickOwner(t.Context(), owner, today))

	// The serialization lock comes first, then one snapshot, then one apply
	// per rule with the owner's today.
	assert.Equal(t, []uuid.UUID{owner}, store.locked)
	require.Len(t, store.applied, 1)
	applied := store.applied[0]
	assert.Equal(t, payment.ID, applied.payment)
	assert.Equal(t, today, applied.today)
	// The plan is the pure domain computation for this rule, snapshot and
	// today included: two due occurrences materialize, tomorrow is the single
	// future planned, and the paid fact is untouched.
	assert.Equal(t, []time.Time{today.AddDate(0, 0, -2), today}, applied.plan.Materialize)
	require.NotNil(t, applied.plan.KeepFuture)
	assert.Equal(t, today.AddDate(0, 0, 1), *applied.plan.KeepFuture)
}

func TestTickOwner_EmptySnapshotAppliesNothing(t *testing.T) {
	t.Parallel()
	owner := uuid.Must(uuid.NewV7())
	today, err := time.Parse(time.DateOnly, "2026-08-25")
	require.NoError(t, err)

	store := &fakeTickStore{snapshot: OwnerSnapshot{
		Statuses: map[uuid.UUID]map[time.Time]domain.OperationStatus{},
	}}
	stores := &txStores{tick: store}

	require.NoError(t, stores.tickOwner(t.Context(), owner, today))
	assert.Equal(t, []uuid.UUID{owner}, store.locked, "the lock is still taken first")
	assert.Empty(t, store.applied)
}

func TestTickOwner_ErrorsPropagate(t *testing.T) {
	t.Parallel()
	owner := uuid.Must(uuid.NewV7())
	today, err := time.Parse(time.DateOnly, "2026-08-25")
	require.NoError(t, err)

	lockFail := errors.New("lock failed")
	require.ErrorIs(t,
		(&txStores{tick: &fakeTickStore{lockErr: lockFail}}).tickOwner(t.Context(), owner, today),
		lockFail)

	snapshotFail := errors.New("snapshot failed")
	require.ErrorIs(t,
		(&txStores{tick: &fakeTickStore{snapshotEr: snapshotFail}}).tickOwner(t.Context(), owner, today),
		snapshotFail)

	payment := tickOwnerPayment(t, uuid.Must(uuid.NewV7()), "2026-08-23")
	applyFail := errors.New("apply failed")
	store := &fakeTickStore{
		snapshot: OwnerSnapshot{Payments: []domain.Payment{payment}},
		applyErr: applyFail,
	}
	require.ErrorIs(t,
		(&txStores{tick: store}).tickOwner(t.Context(), owner, today),
		applyFail)
}
