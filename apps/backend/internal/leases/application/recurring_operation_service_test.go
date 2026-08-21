package application

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	notificationsdomain "github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
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

func (r *lockingFakeRecurringOperationRepo) GetByID(_ context.Context, id uuid.UUID) (domain.RecurringOperation, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	rec, ok := r.recs[id]
	if !ok || rec.DeletedAt != nil {
		return domain.RecurringOperation{}, ErrNotFound
	}
	return rec, nil
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

func (r *lockingFakeRecurringOperationRepo) GetByIDAndOwnerForUpdate(
	_ context.Context,
	id, _ uuid.UUID,
) (domain.RecurringOperation, error) {
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

func (r *lockingFakeRecurringOperationRepo) ListByOwner(
	_ context.Context,
	_ uuid.UUID,
	_ []uuid.UUID,
) ([]domain.RecurringOperation, error) {
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
	ftx, ok := tx.(*fakeTx)
	if !ok {
		panic(fmt.Sprintf("lockingFakeRecurringOperationRepo.WithTx: unexpected tx type %T", tx))
	}
	return &lockingFakeRecurringOperationRepo{
		lock: r.lock,
		recs: r.recs,
		tx:   ftx,
	}
}

func TestUpdateRecurringOperation_ConcurrentUpdatesDoNotOverwrite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	recID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	propertyID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	rec := domain.RecurringOperation{
		ID:            recID,
		OwnerID:       ownerID,
		PropertyID:    propertyID,
		Type:          domain.OperationTypeIncome,
		CategoryID:    testRentCategoryID,
		AmountKopecks: 1000,
		StartDate:     date(2024, 1, 1),
		PaymentDay:    1,
		Periodicity:   domain.RecurringOperationPeriodicityMonthly,
		Status:        domain.RecurringOperationStatusActive,
		Comment:       "original",
	}

	recRepo := newLockingFakeRecurringOperationRepo(rec)
	opRepo := &fakeOperationRepo{}
	propertyRepo := &fakePropertyRepo{statuses: map[uuid.UUID]string{propertyID: propertyStatusActive}}
	cats := newFakeCategoryRepoForOwner(ownerID)
	svc := NewRecurringOperationService(
		recRepo, opRepo, propertyRepo, cats, nil,
		NewTxStoreFactory(nil, propertyRepo, nil, recRepo, opRepo, cats, nil, nil, testUoW()),
		fakeClock{now: date(2024, 6, 1)},
		fakeTzResolver{},
		fakePolicy{}, // Policy.
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

var (
	_ RecurringOperationRepository = (*lockingFakeRecurringOperationRepo)(nil)
	_ transaction.Tx               = (*fakeTx)(nil)
)

type fakeReminderLister struct{}

func (fakeReminderLister) ListByOwner(
	_ context.Context,
	_ uuid.UUID,
	_ notificationsapp.ListFilter,
) ([]notificationsdomain.Reminder, error) {
	return nil, nil
}

func (fakeReminderLister) ListByRecurringOperation(
	_ context.Context,
	_, _ uuid.UUID,
	_ notificationsapp.ListFilter,
) ([]notificationsdomain.Reminder, error) {
	return nil, nil
}

func newRecurringGuardService(
	ownerID uuid.UUID,
	recRepo RecurringOperationRepository,
	opRepo *fakeOperationRepo,
	propertyRepo *fakePropertyRepo,
	reminders ReminderLister,
) *RecurringOperationService {
	seedPropertyOwners(propertyRepo, ownerID)
	cats := newFakeCategoryRepoForOwner(ownerID)
	return NewRecurringOperationService(
		recRepo, opRepo, propertyRepo, cats, reminders,
		NewTxStoreFactory(nil, propertyRepo, nil, recRepo, opRepo, cats, nil, nil, testUoW()),
		fakeClock{now: date(2026, 6, 15)}, fakeTzResolver{}, fakePolicy{}, nil,
	)
}

func newGuardTestRecurringOperation(id, ownerID, propertyID uuid.UUID, status domain.RecurringOperationStatus) domain.RecurringOperation {
	return domain.RecurringOperation{
		ID:            id,
		OwnerID:       ownerID,
		PropertyID:    propertyID,
		Type:          domain.OperationTypeExpense,
		CategoryID:    testCustomExpenseCategoryID,
		Name:          testOperationName,
		AmountKopecks: 1000,
		StartDate:     date(2026, 1, 1),
		PaymentDay:    1,
		Periodicity:   domain.RecurringOperationPeriodicityMonthly,
		Status:        status,
	}
}

func TestCreateRecurringOperation_ArchivedPropertyGuard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	cmd := CreateRecurringOperationCommand{
		PropertyID:    propertyID,
		Type:          testTypeExpense,
		CategoryID:    testCustomExpenseCategoryID,
		Name:          testOperationName,
		AmountKopecks: 1000,
		StartDate:     date(2026, 1, 1),
		PaymentDay:    1,
	}

	for _, tc := range []struct {
		name    string
		status  string
		wantErr error
	}{
		{name: testNameArchivedRejected, status: propertyStatusArchived, wantErr: ErrArchivedProperty},
		{name: testNameActiveAllowed, status: propertyStatusActive, wantErr: nil},
		{name: testNameMaintenanceAllowed, status: propertyStatusMaintenance, wantErr: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := newRecurringGuardService(ownerID, &fakeRecurringOperationRepo{}, &fakeOperationRepo{},
				&fakePropertyRepo{statuses: map[uuid.UUID]string{propertyID: tc.status}}, nil)
			_, err := svc.CreateRecurringOperation(ctx, ownerID, cmd)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("CreateRecurringOperation: want %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestUpdateRecurringOperation_ArchivedPropertyGuard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	recID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	comment := testUpdatedValue
	cmd := UpdateRecurringOperationCommand{Comment: &comment}

	for _, tc := range []struct {
		name    string
		status  string
		wantErr error
	}{
		{name: testNameArchivedRejected, status: propertyStatusArchived, wantErr: ErrArchivedProperty},
		{name: testNameActiveAllowed, status: propertyStatusActive, wantErr: nil},
		{name: testNameMaintenanceAllowed, status: propertyStatusMaintenance, wantErr: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			recRepo := &fakeRecurringOperationRepo{recs: map[uuid.UUID]domain.RecurringOperation{
				recID: newGuardTestRecurringOperation(recID, ownerID, propertyID, domain.RecurringOperationStatusActive),
			}}
			svc := newRecurringGuardService(ownerID, recRepo, &fakeOperationRepo{},
				&fakePropertyRepo{statuses: map[uuid.UUID]string{propertyID: tc.status}}, nil)
			_, err := svc.UpdateRecurringOperation(ctx, ownerID, recID, cmd)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("UpdateRecurringOperation: want %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestUpdateRecurringOperation_NoPropertySkipsGuard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	recID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	recRepo := &fakeRecurringOperationRepo{recs: map[uuid.UUID]domain.RecurringOperation{
		recID: newGuardTestRecurringOperation(recID, ownerID, uuid.Nil, domain.RecurringOperationStatusActive),
	}}
	svc := newRecurringGuardService(ownerID, recRepo, &fakeOperationRepo{}, &fakePropertyRepo{}, nil)

	comment := testUpdatedValue
	updated, err := svc.UpdateRecurringOperation(ctx, ownerID, recID, UpdateRecurringOperationCommand{Comment: &comment})
	if err != nil {
		t.Fatalf("UpdateRecurringOperation without property: %v", err)
	}
	if updated.Comment != testUpdatedValue {
		t.Errorf("comment = %q, want updated", updated.Comment)
	}
}

func TestDeleteRecurringOperation_ArchivedPropertyGuard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	recID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	for _, tc := range []struct {
		name    string
		status  string
		wantErr error
	}{
		{name: testNameArchivedRejected, status: propertyStatusArchived, wantErr: ErrArchivedProperty},
		{name: testNameActiveAllowed, status: propertyStatusActive, wantErr: nil},
		{name: testNameMaintenanceAllowed, status: propertyStatusMaintenance, wantErr: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			recRepo := &fakeRecurringOperationRepo{recs: map[uuid.UUID]domain.RecurringOperation{
				recID: newGuardTestRecurringOperation(recID, ownerID, propertyID, domain.RecurringOperationStatusActive),
			}}
			svc := newRecurringGuardService(ownerID, recRepo, &fakeOperationRepo{},
				&fakePropertyRepo{statuses: map[uuid.UUID]string{propertyID: tc.status}}, nil)
			err := svc.DeleteRecurringOperation(ctx, ownerID, recID)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("DeleteRecurringOperation: want %v, got %v", tc.wantErr, err)
			}
			if tc.wantErr != nil {
				if rec := recRepo.recs[recID]; rec.DeletedAt != nil {
					t.Fatal("recurring operation must not be deleted")
				}
			}
		})
	}
}

func TestPauseRecurringOperation_ArchivedPropertyGuard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	recID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	for _, tc := range []struct {
		name    string
		status  string
		wantErr error
	}{
		{name: testNameArchivedRejected, status: propertyStatusArchived, wantErr: ErrArchivedProperty},
		{name: testNameActiveAllowed, status: propertyStatusActive, wantErr: nil},
		{name: testNameMaintenanceAllowed, status: propertyStatusMaintenance, wantErr: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			recRepo := &fakeRecurringOperationRepo{recs: map[uuid.UUID]domain.RecurringOperation{
				recID: newGuardTestRecurringOperation(recID, ownerID, propertyID, domain.RecurringOperationStatusActive),
			}}
			svc := newRecurringGuardService(ownerID, recRepo, &fakeOperationRepo{},
				&fakePropertyRepo{statuses: map[uuid.UUID]string{propertyID: tc.status}}, nil)
			_, err := svc.PauseRecurringOperation(ctx, ownerID, recID)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("PauseRecurringOperation: want %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestResumeRecurringOperation_ArchivedPropertyGuard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	recID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	t.Run("archived rejected", func(t *testing.T) {
		recRepo := &fakeRecurringOperationRepo{recs: map[uuid.UUID]domain.RecurringOperation{
			recID: newGuardTestRecurringOperation(recID, ownerID, propertyID, domain.RecurringOperationStatusPaused),
		}}
		svc := newRecurringGuardService(ownerID, recRepo, &fakeOperationRepo{},
			&fakePropertyRepo{statuses: map[uuid.UUID]string{propertyID: propertyStatusArchived}}, nil)
		if _, err := svc.ResumeRecurringOperation(ctx, ownerID, recID); !errors.Is(err, ErrArchivedProperty) {
			t.Fatalf("ResumeRecurringOperation: want ErrArchivedProperty, got %v", err)
		}
	})

	for _, status := range []string{propertyStatusActive, propertyStatusMaintenance} {
		t.Run(status+" allowed", func(t *testing.T) {
			t.Parallel()
			recRepo := &fakeRecurringOperationRepo{recs: map[uuid.UUID]domain.RecurringOperation{
				recID: newGuardTestRecurringOperation(recID, ownerID, propertyID, domain.RecurringOperationStatusActive),
			}}
			svc := newRecurringGuardService(ownerID, recRepo, &fakeOperationRepo{},
				&fakePropertyRepo{statuses: map[uuid.UUID]string{propertyID: status}}, nil)
			if _, err := svc.ResumeRecurringOperation(ctx, ownerID, recID); err != nil {
				t.Fatalf("ResumeRecurringOperation: %v", err)
			}
		})
	}
}

func TestSetReminderOffset_ArchivedPropertyGuard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	recID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	for _, tc := range []struct {
		name    string
		status  string
		wantErr error
	}{
		{name: testNameArchivedRejected, status: propertyStatusArchived, wantErr: ErrArchivedProperty},
		{name: testNameActiveAllowed, status: propertyStatusActive, wantErr: nil},
		{name: testNameMaintenanceAllowed, status: propertyStatusMaintenance, wantErr: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			recRepo := &fakeRecurringOperationRepo{recs: map[uuid.UUID]domain.RecurringOperation{
				recID: newGuardTestRecurringOperation(recID, ownerID, propertyID, domain.RecurringOperationStatusActive),
			}}
			svc := newRecurringGuardService(ownerID, recRepo, &fakeOperationRepo{},
				&fakePropertyRepo{statuses: map[uuid.UUID]string{propertyID: tc.status}}, nil)
			err := svc.SetReminderOffset(ctx, ownerID, recID, 1)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("SetReminderOffset: want %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestCreateReminder_ArchivedPropertyGuard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	recID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	t.Run("archived rejected", func(t *testing.T) {
		recRepo := &fakeRecurringOperationRepo{recs: map[uuid.UUID]domain.RecurringOperation{
			recID: newGuardTestRecurringOperation(recID, ownerID, propertyID, domain.RecurringOperationStatusActive),
		}}
		svc := newRecurringGuardService(ownerID, recRepo, &fakeOperationRepo{},
			&fakePropertyRepo{statuses: map[uuid.UUID]string{propertyID: propertyStatusArchived}}, fakeReminderLister{})
		if _, err := svc.CreateReminder(ctx, ownerID, recID, date(2026, 6, 15)); !errors.Is(err, ErrArchivedProperty) {
			t.Fatalf("CreateReminder: want ErrArchivedProperty, got %v", err)
		}
	})

	for _, status := range []string{propertyStatusActive, propertyStatusMaintenance} {
		t.Run(status+" allowed", func(t *testing.T) {
			t.Parallel()
			recRepo := &fakeRecurringOperationRepo{recs: map[uuid.UUID]domain.RecurringOperation{
				recID: newGuardTestRecurringOperation(recID, ownerID, propertyID, domain.RecurringOperationStatusActive),
			}}
			opRepo := &fakeOperationRepo{}
			mustCreateOperation(t, opRepo, ctx, domain.Operation{
				ID:                   uuid.MustParse("44444444-4444-4444-4444-444444444444"),
				OwnerID:              ownerID,
				PropertyID:           propertyID,
				RecurringOperationID: recID,
				Type:                 domain.OperationTypeExpense,
				CategoryID:           testCustomExpenseCategoryID,
				Status:               domain.OperationStatusPending,
				AmountKopecks:        1000,
				OperationDate:        date(2026, 6, 20),
			})
			svc := newRecurringGuardService(ownerID, recRepo, opRepo,
				&fakePropertyRepo{statuses: map[uuid.UUID]string{propertyID: status}}, fakeReminderLister{})
			if _, err := svc.CreateReminder(ctx, ownerID, recID, date(2026, 6, 15)); err != nil {
				t.Fatalf("CreateReminder: %v", err)
			}
		})
	}
}

var _ ReminderLister = fakeReminderLister{}
