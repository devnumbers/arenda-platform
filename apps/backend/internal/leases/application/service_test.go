package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type fakePropertyRepo struct {
	existsByOwner       map[uuid.UUID]bool
	existsActiveByOwner map[uuid.UUID]bool
	hasOpenLease        map[uuid.UUID]bool
	statuses            map[uuid.UUID]string
	owners              map[uuid.UUID]uuid.UUID
}

func (r *fakePropertyRepo) ExistsByOwner(_ context.Context, id, _ uuid.UUID) (bool, error) {
	return r.existsByOwner[id], nil
}

func (r *fakePropertyRepo) ExistsActiveByOwner(_ context.Context, id, _ uuid.UUID) (bool, error) {
	return r.existsActiveByOwner[id], nil
}

func (r *fakePropertyRepo) GetStatusByOwner(_ context.Context, id, _ uuid.UUID) (string, error) {
	if status, ok := r.statuses[id]; ok {
		return status, nil
	}
	if r.existsByOwner[id] {
		return propertyStatusActive, nil
	}
	return "", nil
}

func (r *fakePropertyRepo) HasOpenLease(_ context.Context, id uuid.UUID) (bool, error) {
	return r.hasOpenLease[id], nil
}

func (r *fakePropertyRepo) GetNameByOwner(_ context.Context, _, _ uuid.UUID) (string, error) {
	return "", nil
}

// GetForExport is required to satisfy PropertyRepository but is unused by the
// lease service tests; it reports not found.
func (r *fakePropertyRepo) GetForExport(_ context.Context, _, _ uuid.UUID) (ExportPropertyRow, error) {
	return ExportPropertyRow{}, ErrNotFound
}

func (r *fakePropertyRepo) GetByIDAndOwnerForUpdate(ctx context.Context, id, ownerID uuid.UUID) (string, error) {
	return r.GetStatusByOwner(ctx, id, ownerID)
}

func (r *fakePropertyRepo) GetOwnerByID(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	if owner, ok := r.owners[id]; ok {
		return owner, nil
	}
	if _, ok := r.statuses[id]; ok {
		return id, nil
	}
	return uuid.Nil, ErrNotFound
}

func (r *fakePropertyRepo) WithTx(_ transaction.Tx) PropertyRepository { return r }

type fakeLeaseRepo struct {
	leases map[uuid.UUID]domain.Lease
}

func (r *fakeLeaseRepo) Create(_ context.Context, _ uuid.UUID, lease domain.Lease) (domain.Lease, error) {
	return lease, nil
}

func (r *fakeLeaseRepo) GetByID(_ context.Context, id uuid.UUID) (domain.Lease, error) {
	l, ok := r.leases[id]
	if !ok {
		return domain.Lease{}, ErrNotFound
	}
	return l, nil
}

func (r *fakeLeaseRepo) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Lease, error) {
	return r.GetByID(ctx, id)
}

func (r *fakeLeaseRepo) GetByIDAndOwner(_ context.Context, id, _ uuid.UUID) (domain.Lease, error) {
	l, ok := r.leases[id]
	if !ok {
		return domain.Lease{}, ErrNotFound
	}
	return l, nil
}

func (r *fakeLeaseRepo) GetByIDAndOwnerForUpdate(ctx context.Context, id, ownerID uuid.UUID) (domain.Lease, error) {
	return r.GetByIDAndOwner(ctx, id, ownerID)
}

func (r *fakeLeaseRepo) ListByOwner(_ context.Context, _ uuid.UUID, _ []uuid.UUID) ([]domain.Lease, error) {
	return nil, nil
}

func (r *fakeLeaseRepo) Update(_ context.Context, _ uuid.UUID, lease domain.Lease) (domain.Lease, error) {
	return lease, nil
}

func (r *fakeLeaseRepo) Complete(_ context.Context, _, _ uuid.UUID) (domain.Lease, error) {
	return domain.Lease{}, nil
}

func (r *fakeLeaseRepo) CountOpenLeasesByProperty(_ context.Context, _ uuid.UUID) (int, error) {
	return 0, nil
}

func (r *fakeLeaseRepo) GetOpenLeaseByProperty(_ context.Context, _, _ uuid.UUID) (domain.Lease, error) {
	return domain.Lease{}, ErrNotFound
}

func (r *fakeLeaseRepo) ListOpenLeasesWithPastEndDate(_ context.Context, _ time.Time, _ int) ([]domain.Lease, error) {
	return nil, nil
}

func (r *fakeLeaseRepo) ListByProperty(_ context.Context, _, _ uuid.UUID) ([]domain.Lease, error) {
	return nil, nil
}

func (r *fakeLeaseRepo) ListWithTenantForExport(_ context.Context, _, _ uuid.UUID) ([]ExportLeaseRow, error) {
	return nil, nil
}

func (r *fakeLeaseRepo) WithTx(_ transaction.Tx) LeaseRepository { return r }

func TestOperationService_GetPropertyOperationsSummary(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	leaseID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

	propertyRepo := &fakePropertyRepo{
		existsByOwner: map[uuid.UUID]bool{propertyID: true},
		owners:        map[uuid.UUID]uuid.UUID{propertyID: ownerID},
	}
	opRepo := &fakeOperationRepo{}

	incomeOp := domain.Operation{
		ID:            uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		OwnerID:       ownerID,
		PropertyID:    propertyID,
		LeaseID:       leaseID,
		Type:          domain.OperationTypeIncome,
		CategoryID:    testRentCategoryID,
		Status:        domain.OperationStatusReceived,
		AmountKopecks: 50000,
		OperationDate: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
	}
	expenseOp := domain.Operation{
		ID:            uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		OwnerID:       ownerID,
		PropertyID:    propertyID,
		LeaseID:       leaseID,
		Type:          domain.OperationTypeExpense,
		CategoryID:    testCustomExpenseCategoryID,
		Status:        domain.OperationStatusPaid,
		AmountKopecks: 10000,
		OperationDate: time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC),
	}
	mustCreateOperation(t, opRepo, ctx, incomeOp)
	mustCreateOperation(t, opRepo, ctx, expenseOp)

	cats := newFakeCategoryRepoForOwner(ownerID)
	svc := NewOperationService(
		opRepo, propertyRepo, nil, cats,
		NewTxStoreFactory(nil, propertyRepo, nil, nil, opRepo, cats, nil, nil, testUoW()),
		fakeClock{now: now}, fakeTzResolver{}, fakePolicy{}, nil,
	)

	summary, err := svc.GetPropertyOperationsSummary(ctx, ownerID, propertyID)
	if err != nil {
		t.Fatalf("GetPropertyOperationsSummary failed: %v", err)
	}

	wantAllTime := int64(40000)
	wantMonthly := int64(40000)
	if summary.AllTimeProfitKopecks != wantAllTime {
		t.Errorf("all-time profit: want %d, got %d", wantAllTime, summary.AllTimeProfitKopecks)
	}
	if summary.MonthlyProfitKopecks != wantMonthly {
		t.Errorf("monthly profit: want %d, got %d", wantMonthly, summary.MonthlyProfitKopecks)
	}
	if summary.OverdueRentCount != 0 {
		t.Errorf("overdue rent count: want 0, got %d", summary.OverdueRentCount)
	}
	if summary.OverdueTotalCount != 0 {
		t.Errorf("overdue total count: want 0, got %d", summary.OverdueTotalCount)
	}
}

var (
	_ PropertyRepository = (*fakePropertyRepo)(nil)
	_ LeaseRepository    = (*fakeLeaseRepo)(nil)
)
