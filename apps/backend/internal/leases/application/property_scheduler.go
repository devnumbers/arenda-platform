package application

import (
	"context"
	"fmt"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/timeutil"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// PropertyRecurringOperationScheduler generates operation instances for property
// archive/unarchive side effects. It implements the properties application
// RecurringOperationScheduler port.
type PropertyRecurringOperationScheduler struct {
	ops   OperationRepository
	clock clock.Clock
}

// NewPropertyRecurringOperationScheduler creates a new scheduler.
func NewPropertyRecurringOperationScheduler(ops OperationRepository, clock clock.Clock) *PropertyRecurringOperationScheduler {
	return &PropertyRecurringOperationScheduler{ops: ops, clock: clock}
}

// WithTx returns an instance bound to the provided transaction.
func (s *PropertyRecurringOperationScheduler) WithTx(tx transaction.Tx) propertiesapp.RecurringOperationScheduler {
	return NewPropertyRecurringOperationScheduler(s.ops.WithTx(tx), s.clock)
}

// GenerateOperations creates missing operation instances for the given recurring
// operation from fromDate up to 12 months ahead (or end_date), skipping dates
// that already have operations.
func (s *PropertyRecurringOperationScheduler) GenerateOperations(ctx context.Context, tx transaction.Tx, rec propertiesapp.RecurringOperation, fromDate time.Time) error {
	txOps := s.ops.WithTx(tx)

	existing, err := txOps.ListOperationDatesByRecurringOperation(ctx, rec.ID)
	if err != nil {
		return fmt.Errorf("list existing operation dates: %w", err)
	}

	existingDates := make(map[time.Time]struct{}, len(existing))
	for _, d := range existing {
		existingDates[timeutil.Date(d)] = struct{}{}
	}

	dates := domain.GenerateDates(rec.StartDate, rec.PaymentDay, rec.EndDate, fromDate)

	from := timeutil.Date(fromDate)
	createdAt := s.clock.Now()
	ops := make([]domain.Operation, 0, len(dates))
	for _, d := range dates {
		d = timeutil.Date(d)
		if d.Before(from) {
			continue
		}
		if _, ok := existingDates[d]; ok {
			continue
		}

		ops = append(ops, domain.Operation{
			OwnerID:              rec.OwnerID,
			PropertyID:           rec.PropertyID,
			LeaseID:              rec.LeaseID,
			RecurringOperationID: rec.ID,
			Type:                 domain.OperationType(rec.Type),
			Category:             domain.OperationCategory(rec.Category),
			AmountKopecks:        rec.AmountKopecks,
			OperationDate:        d,
			Comment:              rec.Comment,
			IsException:          false,
			CreatedAt:            createdAt,
			UpdatedAt:            createdAt,
		})
	}

	if len(ops) == 0 {
		return nil
	}

	if err := txOps.BulkCreate(ctx, ops); err != nil {
		return fmt.Errorf("bulk create operations: %w", err)
	}
	return nil
}
