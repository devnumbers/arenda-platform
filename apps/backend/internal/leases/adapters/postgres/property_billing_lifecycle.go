package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
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
	leases       *LeaseRepository
	categories   leasesapp.OperationCategoryRepository
	scheduler    leasesapp.ReminderScheduler
	audit        auditapp.Recorder
	clock        clock.Clock
}

// NewPropertyBillingLifecycle creates a new property billing lifecycle adapter.
func NewPropertyBillingLifecycle(
	ops *OperationRepository,
	recurringOps *RecurringOperationRepository,
	leases *LeaseRepository,
	categories leasesapp.OperationCategoryRepository,
	scheduler leasesapp.ReminderScheduler,
	audit auditapp.Recorder,
	clk clock.Clock,
) *PropertyBillingLifecycle {
	if audit == nil {
		audit = auditapp.Noop{}
	}
	return &PropertyBillingLifecycle{
		ops: ops, recurringOps: recurringOps, leases: leases,
		categories: categories, scheduler: scheduler, audit: audit, clock: clk,
	}
}

// WithTx returns an instance bound to the provided transaction.
func (l *PropertyBillingLifecycle) WithTx(tx transaction.Tx) propertiesapp.PropertyBillingLifecycle {
	var txScheduler leasesapp.ReminderScheduler
	if l.scheduler != nil {
		txScheduler = l.scheduler.WithTx(tx)
	}
	txOps := l.ops.WithTx(tx)
	ops, ok := txOps.(*OperationRepository)
	if !ok {
		panic(fmt.Sprintf("leases.PropertyBillingLifecycle.WithTx: expected *OperationRepository, got %T", txOps))
	}
	txRecurringOps := l.recurringOps.WithTx(tx)
	recurringOps, ok := txRecurringOps.(*RecurringOperationRepository)
	if !ok {
		panic(fmt.Sprintf("leases.PropertyBillingLifecycle.WithTx: expected *RecurringOperationRepository, got %T", txRecurringOps))
	}
	txLeases := l.leases.WithTx(tx)
	leases, ok := txLeases.(*LeaseRepository)
	if !ok {
		panic(fmt.Sprintf("leases.PropertyBillingLifecycle.WithTx: expected *LeaseRepository, got %T", txLeases))
	}
	return NewPropertyBillingLifecycle(
		ops,
		recurringOps,
		leases,
		l.categories.WithTx(tx),
		txScheduler,
		l.audit.WithTx(tx),
		l.clock,
	)
}

// CompleteOpenLeases force-completes all open leases of the property, applying
// the same side effects as a user-initiated lease completion: future unedited
// operations are deleted, recurring operations are paused, and lease and
// recurring-operation reminders are cancelled. Each completion is audited as
// ActionLeaseCompleted with trigger "billing_limit".
func (l *PropertyBillingLifecycle) CompleteOpenLeases(ctx context.Context, scope, propertyID uuid.UUID, asOf time.Time) error {
	leases, err := l.leases.ListByProperty(ctx, scope, propertyID)
	if err != nil {
		return fmt.Errorf("list leases: %w", err)
	}

	for _, lease := range leases {
		if !lease.IsOpen() {
			continue
		}
		leaseID := lease.ID

		if _, err := l.leases.Complete(ctx, leaseID, scope); err != nil {
			return fmt.Errorf("complete lease %s: %w", leaseID, err)
		}

		if err := l.ops.DeleteUneditedFutureOperationsByLease(ctx, leaseID, asOf); err != nil {
			return fmt.Errorf("delete future operations for lease %s: %w", leaseID, err)
		}

		if err := l.recurringOps.UpdateStatusByLeaseID(ctx, leaseID, scope, string(leasesdomain.RecurringOperationStatusPaused)); err != nil {
			return fmt.Errorf("pause recurring operations for lease %s: %w", leaseID, err)
		}

		if l.scheduler != nil {
			if err := l.scheduler.CancelByLease(ctx, scope, leaseID); err != nil {
				return fmt.Errorf("cancel reminders for lease %s: %w", leaseID, err)
			}
			rec, err := l.recurringOps.GetByLeaseID(ctx, scope, leaseID)
			if err != nil && !errors.Is(err, leasesapp.ErrNotFound) {
				return fmt.Errorf("get recurring operation for lease %s: %w", leaseID, err)
			}
			if err == nil {
				if err := l.scheduler.CancelByRecurringOperation(ctx, scope, rec.ID); err != nil {
					return fmt.Errorf("cancel recurring operation reminders for lease %s: %w", leaseID, err)
				}
			}
		}

		if err := l.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &scope,
			ActorRole:  auditdomain.ActorRoleOwner,
			Action:     auditdomain.ActionLeaseCompleted,
			EntityType: auditdomain.EntityLease,
			EntityID:   &leaseID,
			Context:    map[string]any{"property_id": propertyID, "trigger": "billing_limit"},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
	}

	return nil
}

// Suspend deletes future unedited operations for the property, pauses all
// recurring operations associated with it, and cancels their reminders. The
// caller normalizes asOf to midnight in the owner's timezone before calling.
func (l *PropertyBillingLifecycle) Suspend(ctx context.Context, propertyID, scope uuid.UUID, asOf time.Time) error {
	if err := l.ops.DeleteFutureUneditedOperationsByProperty(ctx, propertyID, asOf); err != nil {
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
// missing operation instances from asOf up to 100 years ahead (or end_date),
// skipping dates that already have operations.
func (l *PropertyBillingLifecycle) Resume(ctx context.Context, propertyID, scope uuid.UUID, asOf time.Time) error {
	recs, err := l.recurringOps.ListByProperty(ctx, scope, propertyID)
	if err != nil {
		return fmt.Errorf("list recurring operations: %w", err)
	}

	cats, err := l.categories.ListByOwner(ctx, scope, nil)
	if err != nil {
		return fmt.Errorf("list categories for reminders: %w", err)
	}
	categoryNames := make(map[uuid.UUID]string, len(cats))
	for _, c := range cats {
		categoryNames[c.ID] = c.Name
	}

	from := asOf
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
			opID, err := uuid.NewV7()
			if err != nil {
				return fmt.Errorf("generate operation id: %w", err)
			}
			sourceDate := d
			ops = append(ops, leasesdomain.Operation{
				ID:                   opID,
				OwnerID:              rec.OwnerID,
				PropertyID:           rec.PropertyID,
				LeaseID:              rec.LeaseID,
				RecurringOperationID: rec.ID,
				Type:                 rec.Type,
				CategoryID:           rec.CategoryID,
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
			for _, op := range allOps {
				if !timeutil.BeforeDay(op.OperationDate, asOf) {
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
				if err := l.scheduler.ScheduleForRecurringOperation(
					ctx, recInfo, baseReminderDate, leasesapp.ToOperationInfoSlice(futureOps, categoryNames),
				); err != nil {
					return fmt.Errorf("schedule reminders: %w", err)
				}
			}
		}
	}

	return nil
}

var _ propertiesapp.PropertyBillingLifecycle = (*PropertyBillingLifecycle)(nil)
