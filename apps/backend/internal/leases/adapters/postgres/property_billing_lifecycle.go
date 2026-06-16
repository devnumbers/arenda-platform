package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	leasesdomain "github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/timeutil"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// PropertyBillingLifecycle implements the properties application billing
// lifecycle port using leases-side repositories.
type PropertyBillingLifecycle struct {
	ops          *OperationRepository
	recurringOps *RecurringOperationRepository
	clock        clock.Clock
}

// NewPropertyBillingLifecycle creates a new property billing lifecycle adapter.
func NewPropertyBillingLifecycle(ops *OperationRepository, recurringOps *RecurringOperationRepository, clock clock.Clock) *PropertyBillingLifecycle {
	return &PropertyBillingLifecycle{ops: ops, recurringOps: recurringOps, clock: clock}
}

// WithTx returns an instance bound to the provided transaction.
func (l *PropertyBillingLifecycle) WithTx(tx transaction.Tx) propertiesapp.PropertyBillingLifecycle {
	return NewPropertyBillingLifecycle(
		l.ops.WithTx(tx).(*OperationRepository),
		l.recurringOps.WithTx(tx).(*RecurringOperationRepository),
		l.clock,
	)
}

// Suspend deletes future unedited operations for the property and pauses all
// recurring operations associated with it.
func (l *PropertyBillingLifecycle) Suspend(ctx context.Context, propertyID uuid.UUID, asOf time.Time) error {
	if err := l.ops.DeleteFutureUneditedOperationsByProperty(ctx, propertyID, timeutil.Date(asOf)); err != nil {
		return fmt.Errorf("delete future operations: %w", err)
	}

	if err := l.recurringOps.UpdateStatusByPropertyID(ctx, propertyID, string(leasesdomain.RecurringOperationStatusPaused)); err != nil {
		return fmt.Errorf("pause recurring operations: %w", err)
	}
	return nil
}

// Resume activates all recurring operations for the property and generates
// missing operation instances from asOf up to 12 months ahead (or end_date),
// skipping dates that already have operations.
func (l *PropertyBillingLifecycle) Resume(ctx context.Context, propertyID uuid.UUID, ownerID uuid.UUID, asOf time.Time) error {
	recs, err := l.recurringOps.ListByProperty(ctx, ownerID, propertyID)
	if err != nil {
		return fmt.Errorf("list recurring operations: %w", err)
	}

	from := timeutil.Date(asOf)
	createdAt := l.clock.Now()
	for _, rec := range recs {
		if _, err := l.recurringOps.UpdateStatusByID(ctx, rec.ID, string(leasesdomain.RecurringOperationStatusActive)); err != nil {
			return fmt.Errorf("resume recurring operation: %w", err)
		}

		existing, err := l.ops.ListOperationDatesByRecurringOperation(ctx, rec.ID)
		if err != nil {
			return fmt.Errorf("list existing operation dates: %w", err)
		}
		existingDates := make(map[time.Time]struct{}, len(existing))
		for _, d := range existing {
			existingDates[timeutil.Date(d)] = struct{}{}
		}

		dates := leasesdomain.GenerateDates(rec.StartDate, rec.PaymentDay, rec.EndDate, from)
		ops := make([]leasesdomain.Operation, 0, len(dates))
		for _, d := range dates {
			d = timeutil.Date(d)
			if d.Before(from) {
				continue
			}
			if _, ok := existingDates[d]; ok {
				continue
			}
			ops = append(ops, leasesdomain.Operation{
				OwnerID:              rec.OwnerID,
				PropertyID:           rec.PropertyID,
				LeaseID:              rec.LeaseID,
				RecurringOperationID: rec.ID,
				Type:                 rec.Type,
				Category:             rec.Category,
				AmountKopecks:        rec.AmountKopecks,
				OperationDate:        d,
				Comment:              rec.Comment,
				IsException:          false,
				CreatedAt:            createdAt,
				UpdatedAt:            createdAt,
			})
		}

		if len(ops) == 0 {
			continue
		}
		if err := l.ops.BulkCreate(ctx, ops); err != nil {
			return fmt.Errorf("bulk create operations: %w", err)
		}
	}

	return nil
}

var _ propertiesapp.PropertyBillingLifecycle = (*PropertyBillingLifecycle)(nil)
