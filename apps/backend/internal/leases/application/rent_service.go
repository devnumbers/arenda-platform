package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type RentService struct {
	ops          OperationRepository
	recurringOps RecurringOperationRepository
}

func NewRentService(ops OperationRepository, recurringOps RecurringOperationRepository) *RentService {
	return &RentService{
		ops:          ops,
		recurringOps: recurringOps,
	}
}

// GenerateRentOperations builds the rent Operation values for a lease using the
// recurring schedule. The returned slice is ready to be persisted in bulk.
func (r *RentService) GenerateRentOperations(
	ctx context.Context,
	lease domain.Lease,
	recurringOpID uuid.UUID,
	ownerID uuid.UUID,
) []domain.Operation {
	dates := domain.GenerateDates(lease.StartDate, lease.PaymentDay, lease.EndDate, time.Now())
	if len(dates) == 0 {
		return nil
	}

	now := time.Now()
	ops := make([]domain.Operation, 0, len(dates))

	for _, d := range dates {
		ops = append(ops, domain.Operation{
			OwnerID:              ownerID,
			PropertyID:           lease.PropertyID,
			LeaseID:              lease.ID,
			RecurringOperationID: recurringOpID,
			Type:                 string(domain.OperationTypeIncome),
			Category:             string(domain.OperationCategoryRent),
			AmountKopecks:        lease.RentAmountKopecks,
			OperationDate:        d,
			IsException:          false,
			CreatedAt:            now,
			UpdatedAt:            now,
		})
	}

	return ops
}

// RegenerateFutureOperations deletes unedited future rent operations for a lease
// and recreates them according to the current lease terms.
func (r *RentService) RegenerateFutureOperations(
	ctx context.Context,
	tx transaction.Tx,
	lease domain.Lease,
	fromDate time.Time,
) error {
	txOps := r.ops.WithTx(tx)
	txRecurring := r.recurringOps.WithTx(tx)

	rec, err := txRecurring.GetByLease(ctx, lease.ID)
	if err != nil {
		return fmt.Errorf("get recurring operation: %w", err)
	}

	// Keep the recurring operation in sync with the lease terms.
	rec.AmountKopecks = lease.RentAmountKopecks
	rec.PaymentDay = lease.PaymentDay
	rec.EndDate = lease.EndDate
	rec.UpdatedAt = time.Now()

	if _, err := txRecurring.Update(ctx, rec); err != nil {
		return fmt.Errorf("update recurring operation: %w", err)
	}

	if err := txOps.DeleteUneditedFutureOperationsByLease(ctx, lease.ID, fromDate); err != nil {
		return fmt.Errorf("delete future operations: %w", err)
	}

	existingDates, err := r.existingOperationDates(ctx, txOps, lease.ID)
	if err != nil {
		return fmt.Errorf("list existing operations: %w", err)
	}

	ops := r.GenerateRentOperations(ctx, lease, rec.ID, lease.OwnerID)
	futureOps := filterFutureOperations(ops, fromDate)
	futureOps = excludeExistingDates(futureOps, existingDates)
	if len(futureOps) == 0 {
		return nil
	}

	if err := txOps.BulkCreate(ctx, futureOps); err != nil {
		return fmt.Errorf("bulk create operations: %w", err)
	}

	return nil
}

// RebuildSchedule re-creates the entire rent schedule for a lease.
// It updates the recurring operation template, removes operations that fall
// outside the lease date range (including manual exceptions), and regenerates
// all operations according to the current lease terms. Manual exception
// operations inside the lease range are preserved.
func (r *RentService) RebuildSchedule(
	ctx context.Context,
	tx transaction.Tx,
	lease domain.Lease,
) error {
	txOps := r.ops.WithTx(tx)
	txRecurring := r.recurringOps.WithTx(tx)

	rec, err := txRecurring.GetByLease(ctx, lease.ID)
	if err != nil {
		return fmt.Errorf("get recurring operation: %w", err)
	}

	rec.StartDate = lease.StartDate
	rec.AmountKopecks = lease.RentAmountKopecks
	rec.PaymentDay = lease.PaymentDay
	rec.EndDate = lease.EndDate
	rec.UpdatedAt = time.Now()

	if _, err := txRecurring.Update(ctx, rec); err != nil {
		return fmt.Errorf("update recurring operation: %w", err)
	}

	if err := txOps.DeleteOperationsOutsideLeaseRange(ctx, lease.ID, lease.StartDate, lease.EndDate); err != nil {
		return fmt.Errorf("delete out-of-range operations: %w", err)
	}

	if err := txOps.DeleteUneditedOperationsByLease(ctx, lease.ID); err != nil {
		return fmt.Errorf("delete unedited operations: %w", err)
	}

	existingDates, err := r.existingOperationDates(ctx, txOps, lease.ID)
	if err != nil {
		return fmt.Errorf("list existing operations: %w", err)
	}

	ops := r.GenerateRentOperations(ctx, lease, rec.ID, lease.OwnerID)
	ops = excludeExistingDates(ops, existingDates)
	if len(ops) == 0 {
		return nil
	}

	if err := txOps.BulkCreate(ctx, ops); err != nil {
		return fmt.Errorf("bulk create operations: %w", err)
	}

	return nil
}

func (r *RentService) existingOperationDates(ctx context.Context, ops OperationRepository, leaseID uuid.UUID) (map[time.Time]struct{}, error) {
	existing, err := ops.ListByLease(ctx, leaseID)
	if err != nil {
		return nil, err
	}
	dates := make(map[time.Time]struct{}, len(existing))
	for _, op := range existing {
		dates[date(op.OperationDate)] = struct{}{}
	}
	return dates, nil
}

func excludeExistingDates(ops []domain.Operation, existing map[time.Time]struct{}) []domain.Operation {
	if len(existing) == 0 {
		return ops
	}
	filtered := make([]domain.Operation, 0, len(ops))
	for _, op := range ops {
		if _, ok := existing[date(op.OperationDate)]; ok {
			continue
		}
		filtered = append(filtered, op)
	}
	return filtered
}

func filterFutureOperations(ops []domain.Operation, fromDate time.Time) []domain.Operation {
	from := date(fromDate)
	filtered := make([]domain.Operation, 0, len(ops))
	for _, op := range ops {
		if !date(op.OperationDate).Before(from) {
			filtered = append(filtered, op)
		}
	}
	return filtered
}

func date(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
