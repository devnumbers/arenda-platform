package application

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
)

// OverdueRentOperation is an overdue rent operation projected for per-lease
// current-period overdue detection.
type OverdueRentOperation struct {
	LeaseID       uuid.UUID
	OperationDate time.Time
}

// NextRentPayment is the nearest unpaid rent operation projected for a lease.
type NextRentPayment struct {
	LeaseID         uuid.UUID
	NextPaymentDate time.Time
}

// LeasePaymentSchedule aggregates per-lease payment pointers used by lease
// read responses. A nil date pointer means the lease has no matching operation.
// NextPaymentDate is the nearest pending rent operation on or after as_of.
// OverdueSince is the earliest overdue rent operation of any period; HasOverdue
// reports whether the lease has at least one overdue rent operation.
type LeasePaymentSchedule struct {
	NextPaymentDate *time.Time // nil, если нет будущих неоплаченных операций (pending, operation_date >= as_of)
	OverdueSince    *time.Time // nil, если у аренды нет просроченных операций
	HasOverdue      bool       // true, если есть хотя бы одна overdue rent-операция любого периода
}

// resolveAccessiblePropertyIDs returns the ids of properties shared with the
// actor (issue #157, T3), or nil when the shared-ids adapter is not injected
// (the pre-T3 behaviour: only the actor's own data). Used by the lease payment-
// schedule reads to fold shared rent operations into the schedule.
func (s *LeaseService) resolveAccessiblePropertyIDs(ctx context.Context, actor uuid.UUID) ([]uuid.UUID, error) {
	if s.sharedIDs == nil {
		return nil, nil
	}
	shared, err := s.sharedIDs.SharedWith(ctx, actor)
	if err != nil {
		return nil, fmt.Errorf("list shared property ids: %w", err)
	}
	return shared, nil
}

// CurrentPeriodOverdueIndex returns a map from lease ID to the operation_date
// of the overdue rent operation that falls on the lease's current period due
// date. The presence of a key means the lease has an overdue rent payment for
// the current period; overdue debt from past periods does not set the flag.
// A single repository call is made regardless of the number of leases.
func (s *LeaseService) CurrentPeriodOverdueIndex(ctx context.Context, actor uuid.UUID, leases []domain.Lease, asOf time.Time) (map[uuid.UUID]time.Time, error) {
	accessible, err := s.resolveAccessiblePropertyIDs(ctx, actor)
	if err != nil {
		return nil, err
	}
	ops, err := s.operations.ListOverdueRentOperations(ctx, actor, accessible)
	if err != nil {
		return nil, fmt.Errorf("list overdue rent operations: %w", err)
	}

	grouped := make(map[uuid.UUID][]time.Time)
	for _, op := range ops {
		grouped[op.LeaseID] = append(grouped[op.LeaseID], op.OperationDate)
	}

	index := make(map[uuid.UUID]time.Time)
	for _, lease := range leases {
		due, ok := domain.CurrentPeriodDueDate(lease.StartDate, lease.PaymentDay, asOf)
		if !ok {
			continue
		}
		if slices.Contains(grouped[lease.ID], due) {
			index[lease.ID] = due
		}
	}
	return index, nil
}

// LeasePaymentScheduleIndex returns a map from lease ID to its payment schedule:
// the nearest pending rent payment on or after as_of, the earliest overdue rent
// operation of any period, and an overdue-presence flag. A lease is present in
// the map only when at least one of the three is known; callers must check key
// presence. Two owner-scoped repository calls are made regardless of the number
// of leases.
func (s *LeaseService) LeasePaymentScheduleIndex(ctx context.Context, actor uuid.UUID, leases []domain.Lease, asOf time.Time) (map[uuid.UUID]LeasePaymentSchedule, error) {
	accessible, err := s.resolveAccessiblePropertyIDs(ctx, actor)
	if err != nil {
		return nil, err
	}
	nextOps, err := s.operations.ListNextRentPayments(ctx, actor, accessible, asOf)
	if err != nil {
		return nil, fmt.Errorf("list next rent payments: %w", err)
	}
	nextByLease := make(map[uuid.UUID]time.Time, len(nextOps))
	for _, op := range nextOps {
		nextByLease[op.LeaseID] = op.NextPaymentDate
	}

	overdueOps, err := s.operations.ListOverdueRentOperations(ctx, actor, accessible)
	if err != nil {
		return nil, fmt.Errorf("list overdue rent operations: %w", err)
	}
	overdueByLease := make(map[uuid.UUID][]time.Time)
	for _, op := range overdueOps {
		overdueByLease[op.LeaseID] = append(overdueByLease[op.LeaseID], op.OperationDate)
	}

	index := make(map[uuid.UUID]LeasePaymentSchedule)
	for _, lease := range leases {
		var schedule LeasePaymentSchedule
		if next, ok := nextByLease[lease.ID]; ok {
			schedule.NextPaymentDate = &next
		}
		if ops := overdueByLease[lease.ID]; len(ops) > 0 {
			schedule.HasOverdue = true
			earliest := ops[0]
			for _, d := range ops[1:] {
				if d.Before(earliest) {
					earliest = d
				}
			}
			schedule.OverdueSince = &earliest
		}
		if schedule.NextPaymentDate != nil || schedule.OverdueSince != nil || schedule.HasOverdue {
			index[lease.ID] = schedule
		}
	}
	return index, nil
}
