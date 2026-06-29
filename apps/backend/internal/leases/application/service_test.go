package application

import (
	"context"
	"errors"
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
}

func (r *fakePropertyRepo) ExistsByOwner(_ context.Context, id, _ uuid.UUID) (bool, error) {
	return r.existsByOwner[id], nil
}

func (r *fakePropertyRepo) ExistsActiveByOwner(_ context.Context, id, _ uuid.UUID) (bool, error) {
	return r.existsActiveByOwner[id], nil
}

func (r *fakePropertyRepo) HasOpenLease(_ context.Context, id uuid.UUID) (bool, error) {
	return r.hasOpenLease[id], nil
}

func (r *fakePropertyRepo) WithTx(_ transaction.Tx) PropertyRepository { return r }

type fakeLeaseRepo struct {
	leases map[uuid.UUID]domain.Lease
}

func (r *fakeLeaseRepo) Create(_ context.Context, _ uuid.UUID, lease domain.Lease) (domain.Lease, error) {
	return lease, nil
}

func (r *fakeLeaseRepo) GetByID(_ context.Context, _ uuid.UUID) (domain.Lease, error) {
	return domain.Lease{}, ErrNotFound
}

func (r *fakeLeaseRepo) GetByIDForUpdate(_ context.Context, _ uuid.UUID) (domain.Lease, error) {
	return domain.Lease{}, ErrNotFound
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

func (r *fakeLeaseRepo) ListByOwner(_ context.Context, _ uuid.UUID) ([]domain.Lease, error) {
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

func (r *fakeLeaseRepo) WithTx(_ transaction.Tx) LeaseRepository { return r }

func TestLeaseService_ReturnDeposit(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	leaseID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	lease := domain.Lease{
		ID:                   leaseID,
		OwnerID:              ownerID,
		PropertyID:           propertyID,
		Status:               domain.LeaseStatusCompleted,
		StartDate:            time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		RentAmountKopecks:    10000,
		DepositAmountKopecks: 20000,
		PaymentDay:           1,
	}

	leaseRepo := &fakeLeaseRepo{leases: map[uuid.UUID]domain.Lease{leaseID: lease}}
	opRepo := &fakeOperationRepo{}
	svc := NewLeaseService(leaseRepo, nil, nil, nil, opRepo, nil, fakeTxBeginner{}, fakeClock{now: now}, nil)

	returnedLease, op, err := svc.ReturnDeposit(ctx, ownerID, leaseID)
	if err != nil {
		t.Fatalf("ReturnDeposit failed: %v", err)
	}

	if returnedLease.Status != domain.LeaseStatusCompleted {
		t.Errorf("expected lease status completed, got %q", returnedLease.Status)
	}
	if op.Type != domain.OperationTypeExpense {
		t.Errorf("expected expense operation, got %q", op.Type)
	}
	if op.Category != domain.OperationCategoryDepositReturn {
		t.Errorf("expected deposit_return category, got %q", op.Category)
	}
	if op.Status != domain.OperationStatusPaid {
		t.Errorf("expected paid status, got %q", op.Status)
	}
	if op.AmountKopecks != lease.DepositAmountKopecks {
		t.Errorf("expected amount %d, got %d", lease.DepositAmountKopecks, op.AmountKopecks)
	}
	if op.Comment != "Возврат залога" {
		t.Errorf("expected comment %q, got %q", "Возврат залога", op.Comment)
	}
}

func TestLeaseService_ReturnDeposit_Duplicate(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	leaseID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	lease := domain.Lease{
		ID:                   leaseID,
		OwnerID:              ownerID,
		PropertyID:           propertyID,
		Status:               domain.LeaseStatusCompleted,
		StartDate:            time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		RentAmountKopecks:    10000,
		DepositAmountKopecks: 20000,
		PaymentDay:           1,
	}

	leaseRepo := &fakeLeaseRepo{leases: map[uuid.UUID]domain.Lease{leaseID: lease}}
	opRepo := &fakeOperationRepo{}
	svc := NewLeaseService(leaseRepo, nil, nil, nil, opRepo, nil, fakeTxBeginner{}, fakeClock{now: now}, nil)

	if _, _, err := svc.ReturnDeposit(ctx, ownerID, leaseID); err != nil {
		t.Fatalf("first ReturnDeposit failed: %v", err)
	}
	_, _, err := svc.ReturnDeposit(ctx, ownerID, leaseID)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput on duplicate, got %v", err)
	}
}

func TestLeaseService_ReturnDeposit_EffectiveStatus(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	leaseID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)

	lease := domain.Lease{
		ID:                   leaseID,
		OwnerID:              ownerID,
		PropertyID:           propertyID,
		Status:               domain.LeaseStatusActive,
		StartDate:            time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:              &endDate,
		RentAmountKopecks:    10000,
		DepositAmountKopecks: 20000,
		PaymentDay:           1,
	}

	leaseRepo := &fakeLeaseRepo{leases: map[uuid.UUID]domain.Lease{leaseID: lease}}
	opRepo := &fakeOperationRepo{}
	svc := NewLeaseService(leaseRepo, nil, nil, nil, opRepo, nil, fakeTxBeginner{}, fakeClock{now: now}, nil)

	_, op, err := svc.ReturnDeposit(ctx, ownerID, leaseID)
	if err != nil {
		t.Fatalf("ReturnDeposit failed for past-end open lease: %v", err)
	}
	if op.Category != domain.OperationCategoryDepositReturn {
		t.Errorf("expected deposit_return category, got %q", op.Category)
	}
}

func TestLeaseService_ReturnDeposit_InvalidStatus(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	leaseID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	lease := domain.Lease{
		ID:                   leaseID,
		OwnerID:              ownerID,
		PropertyID:           propertyID,
		Status:               domain.LeaseStatusActive,
		StartDate:            time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		RentAmountKopecks:    10000,
		DepositAmountKopecks: 20000,
		PaymentDay:           1,
	}

	leaseRepo := &fakeLeaseRepo{leases: map[uuid.UUID]domain.Lease{leaseID: lease}}
	opRepo := &fakeOperationRepo{}
	svc := NewLeaseService(leaseRepo, nil, nil, nil, opRepo, nil, fakeTxBeginner{}, fakeClock{now: now}, nil)

	_, _, err := svc.ReturnDeposit(ctx, ownerID, leaseID)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for active lease, got %v", err)
	}
}

func TestOperationService_GetPropertyOperationsSummary(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	leaseID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

	propertyRepo := &fakePropertyRepo{existsByOwner: map[uuid.UUID]bool{propertyID: true}}
	opRepo := &fakeOperationRepo{}

	incomeOp := domain.Operation{
		ID:            uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		OwnerID:       ownerID,
		PropertyID:    propertyID,
		LeaseID:       leaseID,
		Type:          domain.OperationTypeIncome,
		Category:      domain.OperationCategoryRent,
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
		Category:      domain.OperationCategoryUtilities,
		Status:        domain.OperationStatusPaid,
		AmountKopecks: 10000,
		OperationDate: time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC),
	}
	_, _ = opRepo.Create(ctx, incomeOp)
	_, _ = opRepo.Create(ctx, expenseOp)

	svc := NewOperationService(opRepo, propertyRepo, nil, nil, nil, fakeTxBeginner{}, fakeClock{now: now}, nil)

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

var _ PropertyRepository = (*fakePropertyRepo)(nil)
var _ LeaseRepository = (*fakeLeaseRepo)(nil)
