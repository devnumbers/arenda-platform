package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/timeutil"
	sharedtz "github.com/nambers/arenda-planform/apps/backend/internal/shared/tzresolver"
)

type RentService struct {
	ops          OperationRepository
	recurringOps RecurringOperationRepository
	categories   OperationCategoryRepository
	clock        clock.Clock
	tzResolver   sharedtz.OwnerTimezoneResolver
}

func NewRentService(ops OperationRepository, recurringOps RecurringOperationRepository, categories OperationCategoryRepository, clock clock.Clock, tzResolver sharedtz.OwnerTimezoneResolver) *RentService {
	if categories == nil {
		panic("categories repository is required")
	}
	return &RentService{
		ops:          ops,
		recurringOps: recurringOps,
		categories:   categories,
		clock:        clock,
		tzResolver:   tzResolver,
	}
}

// GenerateRentOperations builds the rent Operation values for a lease using the
// recurring schedule. The returned slice is ready to be persisted in bulk.
func (r *RentService) GenerateRentOperations(
	ctx context.Context,
	lease domain.Lease,
	recurringOpID uuid.UUID,
	scope uuid.UUID,
	rentCategoryID uuid.UUID,
) ([]domain.Operation, error) {
	dates := domain.GenerateDates(lease.StartDate, lease.PaymentDay, lease.EndDate, r.clock.Now(), domain.RecurringOperationPeriodicityMonthly)
	if len(dates) == 0 {
		return nil, nil
	}

	now := r.clock.Now()
	loc, err := r.tzResolver.Resolve(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("resolve owner timezone: %w", err)
	}
	today := timeutil.DateIn(now, loc)
	ops := make([]domain.Operation, 0, len(dates))

	for _, d := range dates {
		opID, err := uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("generate operation id: %w", err)
		}
		sourceDate := d
		ops = append(ops, domain.Operation{
			ID:                   opID,
			OwnerID:              scope,
			PropertyID:           lease.PropertyID,
			LeaseID:              lease.ID,
			RecurringOperationID: recurringOpID,
			Type:                 domain.OperationTypeIncome,
			CategoryID:           rentCategoryID,
			Status:               rentOperationStatus(d, today),
			Name:                 "Арендная плата",
			AmountKopecks:        lease.RentAmountKopecks,
			OperationDate:        d,
			SourceOperationDate:  &sourceDate,
			IsException:          false,
			CreatedAt:            now,
			UpdatedAt:            now,
		})
	}

	return ops, nil
}

// rentOperationStatus returns the initial status for a generated rent operation:
// past-dated operations start as unconfirmed and must be completed explicitly,
// operations dated today or later start as pending.
func rentOperationStatus(operationDate, today time.Time) domain.OperationStatus {
	if timeutil.BeforeDay(operationDate, today) {
		return domain.OperationStatusUnconfirmed
	}
	return domain.OperationStatusPending
}

// RegenerateFutureOperations deletes unedited future rent operations for a lease
// and recreates them according to the current lease terms.
// The service instance must be constructed with transaction-bound repositories
// when this method is called inside a transaction.
func (r *RentService) RegenerateFutureOperations(
	ctx context.Context,
	lease domain.Lease,
	fromDate time.Time,
) error {
	rec, err := r.recurringOps.GetByLeaseID(ctx, lease.OwnerID, lease.ID)
	if err != nil {
		return fmt.Errorf("get recurring operation: %w", err)
	}

	rentCategoryID, err := getDefaultCategoryID(ctx, r.categories, lease.OwnerID, domain.OperationCategoryCodeRent)
	if err != nil {
		return fmt.Errorf("rent category: %w", err)
	}

	// Keep the recurring operation in sync with the lease terms.
	rec.AmountKopecks = lease.RentAmountKopecks
	rec.PaymentDay = lease.PaymentDay
	rec.EndDate = lease.EndDate
	rec.UpdatedAt = r.clock.Now()

	if _, err := r.recurringOps.Update(ctx, rec); err != nil {
		return fmt.Errorf("update recurring operation: %w", err)
	}

	if err := r.ops.DeleteUneditedFutureOperationsByLease(ctx, lease.ID, fromDate); err != nil {
		return fmt.Errorf("delete future operations: %w", err)
	}

	existingDates, err := r.existingOperationDates(ctx, lease.ID)
	if err != nil {
		return fmt.Errorf("list existing operations: %w", err)
	}

	ops, err := r.GenerateRentOperations(ctx, lease, rec.ID, lease.OwnerID, rentCategoryID)
	if err != nil {
		return fmt.Errorf("generate rent operations: %w", err)
	}
	futureOps := filterFutureOperations(ops, fromDate)
	futureOps = excludeExistingDates(futureOps, existingDates)
	if len(futureOps) == 0 {
		return nil
	}

	if err := r.ops.BulkCreate(ctx, futureOps); err != nil {
		return fmt.Errorf("bulk create operations: %w", err)
	}

	return nil
}

// RebuildSchedule re-creates the rent schedule for a lease.
// It updates the recurring operation template, removes operations that fall
// outside the lease date range (including manual exceptions), deletes
// unedited generated operations on or after the earlier of the original and
// current lease start dates, and regenerates the schedule. Manual exception
// operations are preserved.
// The service instance must be constructed with transaction-bound repositories
// when this method is called inside a transaction.
func (r *RentService) RebuildSchedule(
	ctx context.Context,
	lease domain.Lease,
	originalStartDate time.Time,
) error {
	rec, err := r.recurringOps.GetByLeaseID(ctx, lease.OwnerID, lease.ID)
	if err != nil {
		return fmt.Errorf("get recurring operation: %w", err)
	}

	rentCategoryID, err := getDefaultCategoryID(ctx, r.categories, lease.OwnerID, domain.OperationCategoryCodeRent)
	if err != nil {
		return fmt.Errorf("rent category: %w", err)
	}

	rec.StartDate = lease.StartDate
	rec.AmountKopecks = lease.RentAmountKopecks
	rec.PaymentDay = lease.PaymentDay
	rec.EndDate = lease.EndDate
	rec.UpdatedAt = r.clock.Now()

	if _, err := r.recurringOps.Update(ctx, rec); err != nil {
		return fmt.Errorf("update recurring operation: %w", err)
	}

	if err := r.ops.DeleteOperationsOutsideLeaseRange(ctx, lease.ID, lease.StartDate, lease.EndDate); err != nil {
		return fmt.Errorf("delete out-of-range operations: %w", err)
	}

	cutoff := timeutil.Date(originalStartDate)
	if timeutil.Date(lease.StartDate).Before(cutoff) {
		cutoff = timeutil.Date(lease.StartDate)
	}

	if err := r.ops.DeleteUneditedOperationsByLease(ctx, lease.ID, cutoff); err != nil {
		return fmt.Errorf("delete unedited operations: %w", err)
	}

	existingDates, err := r.existingOperationDates(ctx, lease.ID)
	if err != nil {
		return fmt.Errorf("list existing operations: %w", err)
	}

	ops, err := r.GenerateRentOperations(ctx, lease, rec.ID, lease.OwnerID, rentCategoryID)
	if err != nil {
		return fmt.Errorf("generate rent operations: %w", err)
	}
	ops = excludeExistingDates(ops, existingDates)
	if len(ops) == 0 {
		return nil
	}

	if err := r.ops.BulkCreate(ctx, ops); err != nil {
		return fmt.Errorf("bulk create operations: %w", err)
	}

	return nil
}

func (r *RentService) existingOperationDates(ctx context.Context, leaseID uuid.UUID) (map[time.Time]struct{}, error) {
	existing, err := r.ops.ListOperationDatesByLease(ctx, leaseID)
	if err != nil {
		return nil, err
	}
	dates := make(map[time.Time]struct{}, len(existing))
	for _, d := range existing {
		dates[timeutil.Date(d)] = struct{}{}
	}
	return dates, nil
}

func excludeExistingDates(ops []domain.Operation, existing map[time.Time]struct{}) []domain.Operation {
	if len(existing) == 0 {
		return ops
	}
	filtered := make([]domain.Operation, 0, len(ops))
	for _, op := range ops {
		if _, ok := existing[timeutil.Date(op.OperationDate)]; ok {
			continue
		}
		filtered = append(filtered, op)
	}
	return filtered
}

func filterFutureOperations(ops []domain.Operation, fromDate time.Time) []domain.Operation {
	from := timeutil.Date(fromDate)
	filtered := make([]domain.Operation, 0, len(ops))
	for _, op := range ops {
		if !timeutil.Date(op.OperationDate).Before(from) {
			filtered = append(filtered, op)
		}
	}
	return filtered
}
