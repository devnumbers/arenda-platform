package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	leasesdomain "github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
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
	scheduler    leasesapp.ReminderScheduler
	clock        clock.Clock
}

// NewPropertyBillingLifecycle creates a new property billing lifecycle adapter.
func NewPropertyBillingLifecycle(ops *OperationRepository, recurringOps *RecurringOperationRepository, scheduler leasesapp.ReminderScheduler, clock clock.Clock) *PropertyBillingLifecycle {
	return &PropertyBillingLifecycle{ops: ops, recurringOps: recurringOps, scheduler: scheduler, clock: clock}
}

// WithTx returns an instance bound to the provided transaction.
func (l *PropertyBillingLifecycle) WithTx(tx transaction.Tx) propertiesapp.PropertyBillingLifecycle {
	var txScheduler leasesapp.ReminderScheduler
	if l.scheduler != nil {
		txScheduler = l.scheduler.WithTx(tx)
	}
	return NewPropertyBillingLifecycle(
		l.ops.WithTx(tx).(*OperationRepository),
		l.recurringOps.WithTx(tx).(*RecurringOperationRepository),
		txScheduler,
		l.clock,
	)
}

// Suspend deletes future unedited operations for the property, pauses all
// recurring operations associated with it, and cancels their reminders.
func (l *PropertyBillingLifecycle) Suspend(ctx context.Context, propertyID uuid.UUID, asOf time.Time) error {
	if err := l.ops.DeleteFutureUneditedOperationsByProperty(ctx, propertyID, timeutil.Date(asOf)); err != nil {
		return fmt.Errorf("delete future operations: %w", err)
	}

	if err := l.recurringOps.UpdateStatusByPropertyID(ctx, propertyID, string(leasesdomain.RecurringOperationStatusPaused)); err != nil {
		return fmt.Errorf("pause recurring operations: %w", err)
	}

	if l.scheduler != nil {
		recs, err := l.recurringOps.ListByPropertyID(ctx, propertyID)
		if err != nil {
			return fmt.Errorf("list recurring operations: %w", err)
		}
		for _, rec := range recs {
			if err := l.scheduler.CancelByRecurringOperation(ctx, rec.OwnerID, rec.ID); err != nil {
				return fmt.Errorf("cancel reminders for recurring operation %s: %w", rec.ID, err)
			}
		}
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

		dates := leasesdomain.GenerateDates(rec.StartDate, rec.PaymentDay, rec.EndDate, from, rec.Periodicity)
		ops := make([]leasesdomain.Operation, 0, len(dates))
		for _, d := range dates {
			d = timeutil.Date(d)
			if d.Before(from) {
				continue
			}
			if _, ok := existingDates[d]; ok {
				continue
			}
			sourceDate := d
			ops = append(ops, leasesdomain.Operation{
				OwnerID:              rec.OwnerID,
				PropertyID:           rec.PropertyID,
				LeaseID:              rec.LeaseID,
				RecurringOperationID: rec.ID,
				Type:                 rec.Type,
				Category:             rec.Category,
				Status:               leasesdomain.OperationStatusPending,
				AmountKopecks:        rec.AmountKopecks,
				OperationDate:        d,
				SourceOperationDate:  &sourceDate,
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

		if l.scheduler != nil && rec.ReminderOffsetDays != nil {
			allOps, err := l.ops.ListByRecurringOperation(ctx, rec.ID)
			if err != nil {
				return fmt.Errorf("list operations for scheduling: %w", err)
			}
			futureOps := make([]leasesdomain.Operation, 0, len(allOps))
			asOfDate := timeutil.Date(asOf)
			for _, op := range allOps {
				if !timeutil.Date(op.OperationDate).Before(asOfDate) {
					futureOps = append(futureOps, op)
				}
			}
			if len(futureOps) > 0 {
				recInfo := notificationsapp.RecurringOperationInfo{
					ID:         rec.ID,
					OwnerID:    rec.OwnerID,
					PropertyID: rec.PropertyID,
					LeaseID:    leasesdomain.LeaseIDPtr(rec.LeaseID),
				}
				baseReminderDate := futureOps[0].OperationDate.AddDate(0, 0, -(*rec.ReminderOffsetDays))
				if err := l.scheduler.ScheduleForRecurringOperation(ctx, recInfo, baseReminderDate, leasesapp.ToOperationInfoSlice(futureOps)); err != nil {
					return fmt.Errorf("schedule reminders: %w", err)
				}
			}
		}
	}

	return nil
}

var _ propertiesapp.PropertyBillingLifecycle = (*PropertyBillingLifecycle)(nil)
