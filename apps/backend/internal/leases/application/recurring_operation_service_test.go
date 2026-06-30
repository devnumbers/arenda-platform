package application

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type fakeTx struct {
	unlock func()
}

func (tx *fakeTx) Commit(_ context.Context) error {
	if tx.unlock != nil {
		tx.unlock()
		tx.unlock = nil
	}
	return nil
}

func (tx *fakeTx) Rollback(_ context.Context) error {
	if tx.unlock != nil {
		tx.unlock()
		tx.unlock = nil
	}
	return nil
}

type fakeTxBeginner struct{}

func (fakeTxBeginner) Begin(_ context.Context) (transaction.Tx, error) {
	return &fakeTx{}, nil
}

type lockingFakeRecurringOperationRepo struct {
	lock *sync.Mutex
	recs map[uuid.UUID]domain.RecurringOperation
	tx   *fakeTx
}

func newLockingFakeRecurringOperationRepo(rec domain.RecurringOperation) *lockingFakeRecurringOperationRepo {
	return &lockingFakeRecurringOperationRepo{
		lock: &sync.Mutex{},
		recs: map[uuid.UUID]domain.RecurringOperation{rec.ID: rec},
	}
}

func (r *lockingFakeRecurringOperationRepo) Create(_ context.Context, rec domain.RecurringOperation) (domain.RecurringOperation, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.recs[rec.ID] = rec
	return rec, nil
}

func (r *lockingFakeRecurringOperationRepo) GetByLeaseID(_ context.Context, _, _ uuid.UUID) (domain.RecurringOperation, error) {
	return domain.RecurringOperation{}, ErrNotFound
}

func (r *lockingFakeRecurringOperationRepo) GetByIDAndOwner(_ context.Context, id, _ uuid.UUID) (domain.RecurringOperation, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	rec, ok := r.recs[id]
	if !ok {
		return domain.RecurringOperation{}, ErrNotFound
	}
	return rec, nil
}

func (r *lockingFakeRecurringOperationRepo) GetByIDAndOwnerForUpdate(_ context.Context, id, _ uuid.UUID) (domain.RecurringOperation, error) {
	r.lock.Lock()
	if r.tx != nil {
		r.tx.unlock = r.lock.Unlock
	} else {
		defer r.lock.Unlock()
	}
	rec, ok := r.recs[id]
	if !ok {
		return domain.RecurringOperation{}, ErrNotFound
	}
	return rec, nil
}

func (r *lockingFakeRecurringOperationRepo) ListByOwner(_ context.Context, _ uuid.UUID) ([]domain.RecurringOperation, error) {
	return nil, nil
}

func (r *lockingFakeRecurringOperationRepo) ListByProperty(_ context.Context, _, _ uuid.UUID) ([]domain.RecurringOperation, error) {
	return nil, nil
}

func (r *lockingFakeRecurringOperationRepo) ListByPropertyID(_ context.Context, _ uuid.UUID) ([]domain.RecurringOperation, error) {
	return nil, nil
}

func (r *lockingFakeRecurringOperationRepo) Update(_ context.Context, rec domain.RecurringOperation) (domain.RecurringOperation, error) {
	// The caller already holds the lock via GetByIDAndOwnerForUpdate.
	if _, ok := r.recs[rec.ID]; !ok {
		return domain.RecurringOperation{}, ErrNotFound
	}
	r.recs[rec.ID] = rec
	return rec, nil
}

func (r *lockingFakeRecurringOperationRepo) UpdateStatus(_ context.Context, _, _ uuid.UUID, _ string) (domain.RecurringOperation, error) {
	return domain.RecurringOperation{}, nil
}

func (r *lockingFakeRecurringOperationRepo) UpdateStatusByLeaseID(_ context.Context, _, _ uuid.UUID, _ string) error {
	return nil
}

func (r *lockingFakeRecurringOperationRepo) SetReminderOffset(_ context.Context, _, _ uuid.UUID, _ *int) error {
	return nil
}

func (r *lockingFakeRecurringOperationRepo) DeleteByLease(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (r *lockingFakeRecurringOperationRepo) SoftDelete(_ context.Context, id, _ uuid.UUID) error {
	// The caller already holds the lock via GetByIDAndOwnerForUpdate.
	if _, ok := r.recs[id]; !ok {
		return ErrNotFound
	}
	rec := r.recs[id]
	now := time.Now()
	rec.DeletedAt = &now
	r.recs[id] = rec
	return nil
}

func (r *lockingFakeRecurringOperationRepo) WithTx(tx transaction.Tx) RecurringOperationRepository {
	return &lockingFakeRecurringOperationRepo{
		lock: r.lock,
		recs: r.recs,
		tx:   tx.(*fakeTx),
	}
}

func TestUpdateRecurringOperation_ConcurrentUpdatesDoNotOverwrite(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	recID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	propertyID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	rec := domain.RecurringOperation{
		ID:            recID,
		OwnerID:       ownerID,
		PropertyID:    propertyID,
		Type:          domain.OperationTypeIncome,
		Category:      domain.OperationCategoryRent,
		AmountKopecks: 1000,
		StartDate:     date(2024, 1, 1),
		PaymentDay:    1,
		Periodicity:   domain.RecurringOperationPeriodicityMonthly,
		Status:        domain.RecurringOperationStatusActive,
		Comment:       "original",
	}

	recRepo := newLockingFakeRecurringOperationRepo(rec)
	opRepo := &fakeOperationRepo{}
	svc := NewRecurringOperationService(
		recRepo,
		opRepo,
		nil, // properties
		nil, // scheduler
		nil, // reminders
		fakeTxBeginner{},
		fakeClock{now: date(2024, 6, 1)},
		nil,
	)

	var wg sync.WaitGroup
	var firstDone atomic.Bool
	wg.Add(2)

	go func() {
		defer wg.Done()
		amount := int64(2000)
		_, err := svc.UpdateRecurringOperation(ctx, ownerID, recID, UpdateRecurringOperationCommand{
			AmountKopecks: &amount,
		})
		if err != nil {
			t.Errorf("amount update failed: %v", err)
		}
		firstDone.Store(true)
	}()

	go func() {
		defer wg.Done()
		// Wait a little so the first goroutine is likely to acquire the lock first.
		for !firstDone.Load() {
			time.Sleep(time.Millisecond)
		}
		comment := "changed"
		_, err := svc.UpdateRecurringOperation(ctx, ownerID, recID, UpdateRecurringOperationCommand{
			Comment: &comment,
		})
		if err != nil {
			t.Errorf("comment update failed: %v", err)
		}
	}()

	wg.Wait()

	final, err := recRepo.GetByIDAndOwner(ctx, recID, ownerID)
	if err != nil {
		t.Fatalf("get final recurring operation: %v", err)
	}
	if final.AmountKopecks != 2000 {
		t.Errorf("amount update lost: got %d, want 2000", final.AmountKopecks)
	}
	if final.Comment != "changed" {
		t.Errorf("comment update lost: got %q, want changed", final.Comment)
	}
}

var _ RecurringOperationRepository = (*lockingFakeRecurringOperationRepo)(nil)
var _ transaction.Tx = (*fakeTx)(nil)
