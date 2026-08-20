package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	notificationsdomain "github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/timeutil"
	sharedtz "github.com/nambers/arenda-planform/apps/backend/internal/shared/tzresolver"
)

// CreateRecurringOperationCommand carries the data needed to create a user-managed
// recurring operation for a property.
type CreateRecurringOperationCommand struct {
	PropertyID         uuid.UUID
	Type               string
	CategoryID         uuid.UUID
	Name               string
	AmountKopecks      int64
	StartDate          time.Time
	PaymentDay         int
	EndDate            *time.Time
	Comment            *string
	Periodicity        string
	ReminderOffsetDays *int
}

// UpdateRecurringOperationCommand carries optional updates for a recurring operation.
type UpdateRecurringOperationCommand struct {
	Type               *string
	CategoryID         *uuid.UUID
	Name               *string
	AmountKopecks      *int64
	StartDate          *time.Time
	PaymentDay         *int
	EndDate            *time.Time
	Comment            *string
	Periodicity        *string
	ApplyFromDate      *time.Time
	ReminderOffsetDays *int
}

// RecurringOperationService orchestrates recurring operation use cases within the
// leases bounded context.
type RecurringOperationService struct {
	recurringOps RecurringOperationRepository
	operations   OperationRepository
	properties   PropertyRepository
	categories   OperationCategoryRepository
	reminders    ReminderLister
	txStoreFactory
	clock      clock.Clock
	tzResolver sharedtz.OwnerTimezoneResolver
	policy     sharedpolicy.Policy
	logger     *slog.Logger
	// SharedIDs is optionally injected (see SetSharedPropertyIDs); when nil the
	// aggregate list covers only the actor's own recurring operations, when set
	// it additionally includes recurring operations of properties shared with
	// the actor (issue #157, T3), excluding archived shared properties.
	sharedIDs SharedPropertyIDs
}

// SetSharedPropertyIDs injects the access-context adapter that resolves the
// property ids shared with an actor via property membership (issue #157, T3).
// Optional: when nil, ListRecurringOperations only returns the actor's own
// recurring operations; when set, it additionally returns those of the shared
// properties.
func (s *RecurringOperationService) SetSharedPropertyIDs(ids SharedPropertyIDs) {
	s.sharedIDs = ids
}

// ReminderLister lists reminders for the recurring operation command.
type ReminderLister interface {
	ListByOwner(ctx context.Context, scope uuid.UUID, filter notificationsapp.ListFilter) ([]notificationsdomain.Reminder, error)
	ListByRecurringOperation(
		ctx context.Context, scope, recurringOpID uuid.UUID, filter notificationsapp.ListFilter,
	) ([]notificationsdomain.Reminder, error)
}

// NewRecurringOperationService creates a new recurring operation service. The
// shared txStoreFactory carries the transactional collaborators (repositories,
// scheduler, audit, UoW); the repositories passed separately are the
// non-transactional read handles (ADR 0033).
func NewRecurringOperationService(
	recurringOps RecurringOperationRepository,
	operations OperationRepository,
	properties PropertyRepository,
	categories OperationCategoryRepository,
	reminders ReminderLister,
	factory txStoreFactory,
	clk clock.Clock,
	tzResolver sharedtz.OwnerTimezoneResolver,
	policy sharedpolicy.Policy,
	logger *slog.Logger,
) *RecurringOperationService {
	if clk == nil {
		panic("clk is required")
	}
	if categories == nil {
		panic("categories repository is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &RecurringOperationService{
		recurringOps:   recurringOps,
		operations:     operations,
		properties:     properties,
		categories:     categories,
		reminders:      reminders,
		txStoreFactory: factory,
		clock:          clk,
		tzResolver:     tzResolver,
		policy:         policy,
		logger:         logger,
	}
}

// CreateRecurringOperation creates a user-managed recurring operation for the
// given owner and property and generates the initial 100-year operation horizon.
func (s *RecurringOperationService) CreateRecurringOperation(
	ctx context.Context,
	actor uuid.UUID,
	cmd CreateRecurringOperationCommand,
) (domain.RecurringOperation, error) {
	role, scope, err := resolveWriteScope(ctx, s.policy, s.properties, actor, cmd.PropertyID)
	if err != nil {
		return domain.RecurringOperation{}, err
	}

	if err := validatePropertyNotArchived(ctx, s.properties, scope, cmd.PropertyID); err != nil {
		return domain.RecurringOperation{}, err
	}

	name, err := validateOperationName(cmd.Name)
	if err != nil {
		return domain.RecurringOperation{}, err
	}

	periodicity, err := parseCommandPeriodicity(cmd.Periodicity)
	if err != nil {
		return domain.RecurringOperation{}, err
	}

	if err := validateReminderOffsetDays(cmd.ReminderOffsetDays); err != nil {
		return domain.RecurringOperation{}, err
	}
	cmd.ReminderOffsetDays = normalizeReminderOffsetDays(cmd.ReminderOffsetDays)

	paymentDay := paymentDayOrDefault(cmd.PaymentDay, cmd.StartDate)

	if err := s.validateCommand(
		ctx, s.categories, scope, cmd.Type, cmd.CategoryID,
		cmd.AmountKopecks, cmd.StartDate, paymentDay, cmd.EndDate,
	); err != nil {
		return domain.RecurringOperation{}, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("generate recurring operation id: %w", err)
	}

	now, today, err := ownerNowAndToday(ctx, s.tzResolver, s.clock, scope)
	if err != nil {
		return domain.RecurringOperation{}, err
	}
	rec := domain.RecurringOperation{
		ID:                 id,
		OwnerID:            scope,
		PropertyID:         cmd.PropertyID,
		Type:               domain.OperationType(cmd.Type),
		CategoryID:         cmd.CategoryID,
		Name:               name,
		AmountKopecks:      cmd.AmountKopecks,
		StartDate:          cmd.StartDate,
		PaymentDay:         paymentDay,
		EndDate:            cmd.EndDate,
		Periodicity:        periodicity,
		Status:             domain.RecurringOperationStatusActive,
		Comment:            stringOrEmpty(cmd.Comment),
		ReminderOffsetDays: cmd.ReminderOffsetDays,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	var created domain.RecurringOperation
	err = s.runInTx(ctx, func(stores *txStores) error {
		var err error
		created, err = stores.recurringOps.Create(ctx, rec)
		if err != nil {
			return fmt.Errorf("create recurring operation: %w", err)
		}

		if err := s.generateOperations(ctx, stores.operations, created, today, nil); err != nil {
			return fmt.Errorf("generate operations: %w", err)
		}

		if err := scheduleNewRecurringReminders(ctx, stores, created, scope, today); err != nil {
			return err
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  actorRoleFromPolicyRole(role),
			Action:     auditdomain.ActionRecurringOperationCreated,
			EntityType: auditdomain.EntityRecurringOperation,
			EntityID:   &created.ID,
			Context: map[string]any{
				auditKeyPropertyID: domain.PropertyIDPtr(created.PropertyID),
				"type":             string(created.Type),
				"amount_kopecks":   created.AmountKopecks,
			},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.RecurringOperation{}, err
	}

	return created, nil
}

// ListRecurringOperations returns the actor's own recurring operations plus,
// when the shared-ids adapter is injected, recurring operations of properties
// shared with the actor (issue #157, T3), excluding archived shared properties.
func (s *RecurringOperationService) ListRecurringOperations(
	ctx context.Context,
	actor uuid.UUID,
) ([]domain.RecurringOperation, error) {
	var accessible []uuid.UUID
	if s.sharedIDs != nil {
		shared, err := s.sharedIDs.SharedWith(ctx, actor)
		if err != nil {
			return nil, fmt.Errorf("list shared property ids: %w", err)
		}
		accessible = shared
	}
	recs, err := s.recurringOps.ListByOwner(ctx, actor, accessible)
	if err != nil {
		return nil, fmt.Errorf("list recurring operations: %w", err)
	}

	return recs, nil
}

// ListRecurringOperationsByProperty returns the recurring operations for a
// property.
func (s *RecurringOperationService) ListRecurringOperationsByProperty(
	ctx context.Context,
	actor, propertyID uuid.UUID,
) ([]domain.RecurringOperation, error) {
	scope, err := resolveReadScope(ctx, s.policy, s.properties, actor, propertyID)
	if err != nil {
		return nil, err
	}

	recs, err := s.recurringOps.ListByProperty(ctx, scope, propertyID)
	if err != nil {
		return nil, fmt.Errorf("list recurring operations: %w", err)
	}

	return recs, nil
}

// GetRecurringOperation returns a single recurring operation after the T3
// shared-access read gate.
func (s *RecurringOperationService) GetRecurringOperation(
	ctx context.Context,
	actor, id uuid.UUID,
) (domain.RecurringOperation, error) {
	rec, err := s.recurringOps.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.RecurringOperation{}, ErrNotFound
		}
		return domain.RecurringOperation{}, fmt.Errorf("get recurring operation: %w", err)
	}

	role, err := roleForStandalone(ctx, s.policy, actor, rec.PropertyID, rec.OwnerID)
	if err != nil {
		return domain.RecurringOperation{}, err
	}
	if !sharedpolicy.CanView(role) {
		return domain.RecurringOperation{}, ErrNotFound
	}

	return rec, nil
}

// DeleteRecurringOperation soft-deletes a user-managed recurring operation series
// and removes its future generated operations. Series created by a lease cannot
// be deleted through this endpoint.
func (s *RecurringOperationService) DeleteRecurringOperation(
	ctx context.Context,
	actor, id uuid.UUID,
) error {
	role, scope, err := s.gateStandaloneRecurringWrite(ctx, actor, id)
	if err != nil {
		return err
	}

	return s.runInTx(ctx, func(stores *txStores) error {
		rec, err := lockRecurringOperationForUpdate(ctx, stores, id, scope)
		if err != nil {
			return err
		}

		if err := validatePropertyNotArchived(ctx, stores.properties, scope, rec.PropertyID); err != nil {
			return err
		}

		if rec.LeaseID != uuid.Nil {
			return ErrRecurringOperationLeaseCreated
		}

		if err := stores.operations.DeleteFutureGeneratedOperations(ctx, id, scope); err != nil {
			return fmt.Errorf("delete future generated operations: %w", err)
		}

		if stores.scheduler != nil {
			if err := cancelRecurringOperationReminders(ctx, stores, scope, id); err != nil {
				return err
			}
		}

		if err := stores.recurringOps.SoftDelete(ctx, id, scope); err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("soft delete recurring operation: %w", err)
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  actorRoleFromPolicyRole(role),
			Action:     auditdomain.ActionRecurringOperationDeleted,
			EntityType: auditdomain.EntityRecurringOperation,
			EntityID:   &id,
			Context:    map[string]any{auditKeyPropertyID: domain.PropertyIDPtr(rec.PropertyID)},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
}

// applyRecurringOperationUpdate applies the update command's patch fields to
// the recurring operation with validation (type, category, name, periodicity,
// amount, dates, payment day, comment, reminder offset) and stamps UpdatedAt.
func (s *RecurringOperationService) applyRecurringOperationUpdate(
	ctx context.Context,
	txCategories OperationCategoryRepository,
	rec domain.RecurringOperation,
	cmd UpdateRecurringOperationCommand,
	scope uuid.UUID,
	now time.Time,
) (domain.RecurringOperation, error) {
	rec, err := applyRecurringTypeAndCategory(ctx, txCategories, rec, cmd, scope)
	if err != nil {
		return domain.RecurringOperation{}, err
	}

	if cmd.Name != nil {
		name, err := validateOperationName(*cmd.Name)
		if err != nil {
			return domain.RecurringOperation{}, err
		}
		rec.Name = name
	}

	if cmd.Periodicity != nil {
		periodicity, err := parseCommandPeriodicity(*cmd.Periodicity)
		if err != nil {
			return domain.RecurringOperation{}, err
		}
		rec.Periodicity = periodicity
	}

	if cmd.AmountKopecks != nil {
		rec.AmountKopecks = *cmd.AmountKopecks
	}
	if cmd.StartDate != nil {
		rec.StartDate = *cmd.StartDate
	}
	rec.PaymentDay = recurringPaymentDay(rec.PaymentDay, cmd.PaymentDay, rec.LeaseID, rec.StartDate)
	if cmd.EndDate != nil {
		d := timeutil.Date(*cmd.EndDate)
		rec.EndDate = &d
	}
	if cmd.Comment != nil {
		rec.Comment = *cmd.Comment
	}
	if cmd.ReminderOffsetDays != nil {
		if err := validateReminderOffsetDays(cmd.ReminderOffsetDays); err != nil {
			return domain.RecurringOperation{}, err
		}
		rec.ReminderOffsetDays = normalizeReminderOffsetDays(cmd.ReminderOffsetDays)
	}

	if err := s.validateCommand(
		ctx, txCategories, scope, string(rec.Type), rec.CategoryID,
		rec.AmountKopecks, rec.StartDate, rec.PaymentDay, rec.EndDate,
	); err != nil {
		return domain.RecurringOperation{}, err
	}

	rec.UpdatedAt = now
	return rec, nil
}

// applyRecurringTypeAndCategory patches the recurring operation type and
// category from the command and revalidates the category when either of them
// changed.
func applyRecurringTypeAndCategory(
	ctx context.Context,
	txCategories OperationCategoryRepository,
	rec domain.RecurringOperation,
	cmd UpdateRecurringOperationCommand,
	scope uuid.UUID,
) (domain.RecurringOperation, error) {
	opType := rec.Type
	categoryID := rec.CategoryID
	if cmd.Type != nil {
		parsedType, err := domain.ParseOperationType(*cmd.Type)
		if err != nil {
			return domain.RecurringOperation{}, err
		}
		opType = parsedType
	}
	if cmd.CategoryID != nil {
		categoryID = *cmd.CategoryID
	}
	if cmd.Type != nil || cmd.CategoryID != nil {
		if err := validateCategory(ctx, txCategories, scope, opType, categoryID); err != nil {
			return domain.RecurringOperation{}, err
		}
	}
	rec.Type = opType
	rec.CategoryID = categoryID
	return rec, nil
}

// recurringPaymentDay resolves the effective payment day of a recurring
// series: an explicit patch wins; a user-managed series (not lease-created)
// without a patch follows its start date; a lease-created series keeps its
// payment day in sync with the lease.
func recurringPaymentDay(current int, cmdPaymentDay *int, leaseID uuid.UUID, startDate time.Time) int {
	if cmdPaymentDay != nil {
		return *cmdPaymentDay
	}
	if leaseID == uuid.Nil {
		return startDate.Day()
	}
	return current
}

// resyncRecurringSeries brings the generated operations and reminders back in
// sync with an updated series: it propagates a changed reminder offset to
// future generated operations, cancels stale recurring reminders, regenerates
// the future operations, and re-schedules reminders when an offset is set.
func (s *RecurringOperationService) resyncRecurringSeries(
	ctx context.Context,
	stores *txStores,
	updated domain.RecurringOperation,
	cmd UpdateRecurringOperationCommand,
	scope uuid.UUID,
	today time.Time,
	hadReminderOffset bool,
) error {
	if cmd.ReminderOffsetDays != nil {
		if err := stores.operations.UpdateFutureGeneratedOperationReminderOffsets(
			ctx, scope, updated.ID, updated.ReminderOffsetDays, today,
		); err != nil {
			return fmt.Errorf("sync generated operation reminder offsets: %w", err)
		}
	}

	if stores.scheduler != nil && (hadReminderOffset || updated.ReminderOffsetDays != nil) {
		if err := cancelRecurringOperationReminders(ctx, stores, scope, updated.ID); err != nil {
			return err
		}
	}

	if err := stores.operations.DeleteUneditedFutureOperationsByRecurringOperation(ctx, updated.ID, today); err != nil {
		return fmt.Errorf("delete future operations: %w", err)
	}

	if err := s.regenerateFutureOperations(ctx, stores, updated, today); err != nil {
		return fmt.Errorf("regenerate operations: %w", err)
	}

	if stores.scheduler != nil && updated.ReminderOffsetDays != nil {
		if err := scheduleRecurringRemindersForSeries(ctx, stores, scope, updated, today); err != nil {
			return err
		}
	}
	return nil
}

// regenerateFutureOperations regenerates the future generated operations of a
// series from today onward.
func (s *RecurringOperationService) regenerateFutureOperations(
	ctx context.Context,
	stores *txStores,
	rec domain.RecurringOperation,
	today time.Time,
) error {
	return s.generateOperations(ctx, stores.operations, rec, today, func(d time.Time) bool {
		return !timeutil.Date(d).Before(today)
	})
}

// UpdateRecurringOperation updates a recurring operation owned by the given owner
// and regenerates future operations to reflect schedule or amount changes.
// When ApplyFromDate is set, the series is split: the existing series ends the
// day before ApplyFromDate and a new series is created with the requested
// changes starting on ApplyFromDate.
func (s *RecurringOperationService) UpdateRecurringOperation(
	ctx context.Context,
	actor, id uuid.UUID,
	cmd UpdateRecurringOperationCommand,
) (domain.RecurringOperation, error) {
	role, scope, err := s.gateStandaloneRecurringWrite(ctx, actor, id)
	if err != nil {
		return domain.RecurringOperation{}, err
	}

	var result domain.RecurringOperation
	err = s.runInTx(ctx, func(stores *txStores) error {
		rec, err := lockRecurringOperationForUpdate(ctx, stores, id, scope)
		if err != nil {
			return err
		}

		if err := validatePropertyNotArchived(ctx, stores.properties, scope, rec.PropertyID); err != nil {
			return err
		}

		now, today, err := ownerNowAndToday(ctx, s.tzResolver, s.clock, scope)
		if err != nil {
			return err
		}

		var updated domain.RecurringOperation
		if cmd.ApplyFromDate != nil {
			updated, err = s.updateRecurringOperationBySplit(ctx, stores, actor, role, id, rec, cmd, scope, today)
		} else {
			updated, err = s.updateRecurringOperationInPlace(ctx, stores, actor, role, id, rec, cmd, scope, now, today)
		}
		if err != nil {
			return err
		}
		result = updated
		return nil
	})
	if err != nil {
		return domain.RecurringOperation{}, err
	}

	return result, nil
}

// updateRecurringOperationBySplit applies an update with ApplyFromDate by
// splitting the series: the existing series is truncated the day before
// applyFromDate and a new series with the requested changes starts on it
// (see splitRecurringOperationSeries), and the split is audited with the new
// series id. The split path consumes today (the owner's midnight), not the
// raw instant — its day-granular contract is validated and stamped in the
// owner's calendar, not at the time of day the request arrives.
func (s *RecurringOperationService) updateRecurringOperationBySplit(
	ctx context.Context,
	stores *txStores,
	actor uuid.UUID,
	role sharedpolicy.Role,
	id uuid.UUID,
	rec domain.RecurringOperation,
	cmd UpdateRecurringOperationCommand,
	scope uuid.UUID,
	today time.Time,
) (domain.RecurringOperation, error) {
	if cmd.StartDate != nil {
		return domain.RecurringOperation{}, newInvalidInputError("start_date cannot be used with apply_from_date")
	}
	newRec, err := s.splitRecurringOperationSeries(ctx, stores, scope, rec, cmd, today)
	if err != nil {
		return domain.RecurringOperation{}, err
	}
	if err := stores.audit.Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  actorRoleFromPolicyRole(role),
		Action:     auditdomain.ActionRecurringOperationUpdated,
		EntityType: auditdomain.EntityRecurringOperation,
		EntityID:   &id,
		Context: map[string]any{
			auditKeyFields:  updatedRecurringOperationFields(cmd),
			"new_series_id": newRec.ID,
		},
	}); err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("record audit: %w", err)
	}
	return newRec, nil
}

// updateRecurringOperationInPlace applies an update without ApplyFromDate:
// the patch is applied, the series is persisted, and its generated operations
// and reminders are resynced.
func (s *RecurringOperationService) updateRecurringOperationInPlace(
	ctx context.Context,
	stores *txStores,
	actor uuid.UUID,
	role sharedpolicy.Role,
	id uuid.UUID,
	rec domain.RecurringOperation,
	cmd UpdateRecurringOperationCommand,
	scope uuid.UUID,
	now, today time.Time,
) (domain.RecurringOperation, error) {
	originalReminderOffset := rec.ReminderOffsetDays

	rec, err := s.applyRecurringOperationUpdate(ctx, stores.categories, rec, cmd, scope, now)
	if err != nil {
		return domain.RecurringOperation{}, err
	}

	updated, err := updateRecurringOperationOrNotFound(ctx, stores, rec)
	if err != nil {
		return domain.RecurringOperation{}, err
	}

	if err := s.resyncRecurringSeries(ctx, stores, updated, cmd, scope, today, originalReminderOffset != nil); err != nil {
		return domain.RecurringOperation{}, err
	}

	if err := stores.audit.Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  actorRoleFromPolicyRole(role),
		Action:     auditdomain.ActionRecurringOperationUpdated,
		EntityType: auditdomain.EntityRecurringOperation,
		EntityID:   &id,
		Context:    map[string]any{auditKeyFields: updatedRecurringOperationFields(cmd)},
	}); err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("record audit: %w", err)
	}

	return updated, nil
}

// newSeriesFromUpdate computes the new series record for a split: it applies
// the update command's changes to a copy of the original series starting at
// applyFromDate, validating the category, the fields, and the combined
// command (including end_date >= apply_from_date).
func (s *RecurringOperationService) newSeriesFromUpdate(
	ctx context.Context,
	txCategories OperationCategoryRepository,
	scope uuid.UUID,
	rec domain.RecurringOperation,
	cmd UpdateRecurringOperationCommand,
	applyFromDate, now time.Time,
) (domain.RecurringOperation, error) {
	newID, err := uuid.NewV7()
	if err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("generate recurring operation id: %w", err)
	}

	rec, err = applyRecurringTypeAndCategory(ctx, txCategories, rec, cmd, scope)
	if err != nil {
		return domain.RecurringOperation{}, err
	}

	name := rec.Name
	if cmd.Name != nil {
		name, err = validateOperationName(*cmd.Name)
		if err != nil {
			return domain.RecurringOperation{}, err
		}
	}

	amountKopecks := rec.AmountKopecks
	if cmd.AmountKopecks != nil {
		amountKopecks = *cmd.AmountKopecks
	}

	paymentDay := recurringPaymentDay(rec.PaymentDay, cmd.PaymentDay, rec.LeaseID, applyFromDate)

	periodicity := rec.Periodicity
	if cmd.Periodicity != nil {
		periodicity, err = parseCommandPeriodicity(*cmd.Periodicity)
		if err != nil {
			return domain.RecurringOperation{}, err
		}
	}

	comment := rec.Comment
	if cmd.Comment != nil {
		comment = *cmd.Comment
	}

	reminderOffsetDays := rec.ReminderOffsetDays
	if cmd.ReminderOffsetDays != nil {
		if err := validateReminderOffsetDays(cmd.ReminderOffsetDays); err != nil {
			return domain.RecurringOperation{}, err
		}
		reminderOffsetDays = normalizeReminderOffsetDays(cmd.ReminderOffsetDays)
	}

	newEndDate, err := seriesEndDate(rec, cmd, applyFromDate)
	if err != nil {
		return domain.RecurringOperation{}, err
	}

	if err := s.validateCommand(
		ctx, txCategories, scope, string(rec.Type), rec.CategoryID,
		amountKopecks, applyFromDate, paymentDay, newEndDate,
	); err != nil {
		return domain.RecurringOperation{}, err
	}

	return domain.RecurringOperation{
		ID:                 newID,
		OwnerID:            scope,
		PropertyID:         rec.PropertyID,
		LeaseID:            rec.LeaseID,
		Type:               rec.Type,
		CategoryID:         rec.CategoryID,
		Name:               name,
		AmountKopecks:      amountKopecks,
		StartDate:          applyFromDate,
		PaymentDay:         paymentDay,
		EndDate:            newEndDate,
		Periodicity:        periodicity,
		Status:             rec.Status,
		Comment:            comment,
		ReminderOffsetDays: reminderOffsetDays,
		CreatedAt:          now,
		UpdatedAt:          now,
	}, nil
}

// seriesEndDate resolves the end date of a split's new series and rejects an
// end date before applyFromDate.
func seriesEndDate(rec domain.RecurringOperation, cmd UpdateRecurringOperationCommand, applyFromDate time.Time) (*time.Time, error) {
	newEndDate := rec.EndDate
	if cmd.EndDate != nil {
		d := timeutil.Date(*cmd.EndDate)
		newEndDate = &d
	}
	if newEndDate != nil && applyFromDate.After(*newEndDate) {
		return nil, newInvalidInputError("end_date must be on or after apply_from_date")
	}
	return newEndDate, nil
}

// splitRecurringOperationSeries truncates the existing recurring operation at
// the day before applyFromDate, deletes its unedited operations from that date
// onward, and creates a new recurring operation with the requested changes.
func (s *RecurringOperationService) splitRecurringOperationSeries(
	ctx context.Context,
	stores *txStores,
	scope uuid.UUID,
	rec domain.RecurringOperation,
	cmd UpdateRecurringOperationCommand,
	now time.Time,
) (domain.RecurringOperation, error) {
	applyFromDate := timeutil.Date(*cmd.ApplyFromDate)
	// The caller passes now already normalized to midnight in the owner's
	// timezone.
	today := now

	if err := validateApplyFromDate(rec, applyFromDate, today); err != nil {
		return domain.RecurringOperation{}, err
	}

	newRec, err := s.newSeriesFromUpdate(ctx, stores.categories, scope, rec, cmd, applyFromDate, now)
	if err != nil {
		return domain.RecurringOperation{}, err
	}

	oldEndDate := applyFromDate.AddDate(0, 0, -1)
	rec.EndDate = &oldEndDate
	rec.UpdatedAt = now

	if _, err := stores.recurringOps.Update(ctx, rec); err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.RecurringOperation{}, ErrNotFound
		}
		return domain.RecurringOperation{}, fmt.Errorf("truncate recurring operation: %w", err)
	}

	if err := stores.operations.DeleteUneditedOperationsByRecurringOperation(ctx, rec.ID, applyFromDate); err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("delete future operations: %w", err)
	}

	created, err := stores.recurringOps.Create(ctx, newRec)
	if err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("create new recurring operation: %w", err)
	}

	err = s.generateOperations(ctx, stores.operations, created, now, func(d time.Time) bool {
		return !timeutil.Date(d).Before(applyFromDate)
	})
	if err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("generate operations: %w", err)
	}

	if err := rescheduleSplitSeriesReminders(ctx, stores, scope, rec, created, now); err != nil {
		return domain.RecurringOperation{}, err
	}

	return created, nil
}

// validateApplyFromDate rejects a split date in the past, at or before the
// series start, or after the series end.
func validateApplyFromDate(rec domain.RecurringOperation, applyFromDate, today time.Time) error {
	if applyFromDate.Before(today) {
		return newInvalidInputError("apply_from_date must be today or in the future")
	}
	if !applyFromDate.After(timeutil.Date(rec.StartDate)) {
		return newInvalidInputError("apply_from_date must be after the series start date")
	}
	if rec.EndDate != nil && applyFromDate.After(timeutil.Date(*rec.EndDate)) {
		return newInvalidInputError("apply_from_date must be on or before the series end date")
	}
	return nil
}

// rescheduleSplitSeriesReminders reschedules the reminders of both halves of a
// split: the truncated original series keeps reminders for its retained
// operations, the new series gets reminders for its generated operations.
func rescheduleSplitSeriesReminders(
	ctx context.Context,
	stores *txStores,
	scope uuid.UUID,
	rec, created domain.RecurringOperation,
	now time.Time,
) error {
	if stores.scheduler == nil || (rec.ReminderOffsetDays == nil && created.ReminderOffsetDays == nil) {
		return nil
	}
	categoryNames, err := buildCategoryNamesMap(ctx, stores.categories, scope)
	if err != nil {
		return err
	}
	if err := stores.scheduler.CancelByRecurringOperation(ctx, scope, rec.ID); err != nil {
		return fmt.Errorf("cancel recurring reminders: %w", err)
	}

	retainedOps, err := stores.operations.ListByRecurringOperation(ctx, rec.ID)
	if err != nil {
		return fmt.Errorf("list retained operations for scheduling: %w", err)
	}
	if err := scheduleRemindersForOperations(ctx, stores.scheduler, rec, retainedOps, categoryNames, now); err != nil {
		return fmt.Errorf("schedule reminders for retained operations: %w", err)
	}

	persistedOps, err := stores.operations.ListByRecurringOperation(ctx, created.ID)
	if err != nil {
		return fmt.Errorf("list operations for scheduling: %w", err)
	}
	if err := scheduleRemindersForOperations(ctx, stores.scheduler, created, persistedOps, categoryNames, now); err != nil {
		return fmt.Errorf("schedule reminders: %w", err)
	}
	return nil
}

// PauseRecurringOperation marks a recurring operation as paused and cancels its reminders.
func (s *RecurringOperationService) PauseRecurringOperation(
	ctx context.Context,
	actor, id uuid.UUID,
) (domain.RecurringOperation, error) {
	role, scope, err := s.gateStandaloneRecurringWrite(ctx, actor, id)
	if err != nil {
		return domain.RecurringOperation{}, err
	}

	var result domain.RecurringOperation
	err = s.runInTx(ctx, func(stores *txStores) error {
		rec, err := lockRecurringOperationForUpdate(ctx, stores, id, scope)
		if err != nil {
			return err
		}

		if err := validatePropertyNotArchived(ctx, stores.properties, scope, rec.PropertyID); err != nil {
			return err
		}

		if stores.scheduler != nil {
			if err := cancelRecurringOperationReminders(ctx, stores, scope, id); err != nil {
				return err
			}
		}

		rec, err = updateRecurringStatusOrNotFound(ctx, stores, id, scope, domain.RecurringOperationStatusPaused, "pause recurring operation")
		if err != nil {
			return err
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  actorRoleFromPolicyRole(role),
			Action:     auditdomain.ActionRecurringOperationPaused,
			EntityType: auditdomain.EntityRecurringOperation,
			EntityID:   &id,
			Context:    map[string]any{auditKeyPropertyID: domain.PropertyIDPtr(rec.PropertyID)},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}

		result = rec
		return nil
	})
	if err != nil {
		return domain.RecurringOperation{}, err
	}

	return result, nil
}

// ResumeRecurringOperation marks a recurring operation as active and generates any
// missing operations from the current date forward.
func (s *RecurringOperationService) ResumeRecurringOperation(
	ctx context.Context,
	actor, id uuid.UUID,
) (domain.RecurringOperation, error) {
	role, scope, err := s.gateStandaloneRecurringWrite(ctx, actor, id)
	if err != nil {
		return domain.RecurringOperation{}, err
	}

	var result domain.RecurringOperation
	err = s.runInTx(ctx, func(stores *txStores) error {
		rec, err := lockRecurringOperationForUpdate(ctx, stores, id, scope)
		if err != nil {
			return err
		}

		if err := validatePropertyNotArchived(ctx, stores.properties, scope, rec.PropertyID); err != nil {
			return err
		}

		if rec.Status == domain.RecurringOperationStatusActive {
			result = rec
			return nil
		}

		rec, err = updateRecurringStatusOrNotFound(ctx, stores, id, scope, domain.RecurringOperationStatusActive, "resume recurring operation")
		if err != nil {
			return err
		}

		today, err := ownerTodayFor(ctx, s.tzResolver, scope, s.clock.Now())
		if err != nil {
			return err
		}
		if err := s.regenerateFutureOperations(ctx, stores, rec, today); err != nil {
			return fmt.Errorf("generate operations: %w", err)
		}

		if err := scheduleNewRecurringReminders(ctx, stores, rec, scope, today); err != nil {
			return err
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  actorRoleFromPolicyRole(role),
			Action:     auditdomain.ActionRecurringOperationResumed,
			EntityType: auditdomain.EntityRecurringOperation,
			EntityID:   &id,
			Context:    map[string]any{auditKeyPropertyID: domain.PropertyIDPtr(rec.PropertyID)},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}

		result = rec
		return nil
	})
	if err != nil {
		return domain.RecurringOperation{}, err
	}

	return result, nil
}

// ListOperationsByRecurringOperation returns the generated operations for a
// recurring operation after the T3 shared-access read gate on the parent.
func (s *RecurringOperationService) ListOperationsByRecurringOperation(
	ctx context.Context,
	actor, id uuid.UUID,
) ([]domain.Operation, error) {
	rec, err := s.recurringOps.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get recurring operation: %w", err)
	}
	role, err := roleForStandalone(ctx, s.policy, actor, rec.PropertyID, rec.OwnerID)
	if err != nil {
		return nil, err
	}
	if !sharedpolicy.CanView(role) {
		return nil, ErrNotFound
	}

	ops, err := s.operations.ListByRecurringOperation(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list operations: %w", err)
	}
	return ops, nil
}

// SetReminderOffset updates the reminder offset for a recurring operation and
// rebuilds its concrete reminders for all future generated operations.
func (s *RecurringOperationService) SetReminderOffset(
	ctx context.Context,
	actor, id uuid.UUID,
	offsetDays int,
) error {
	if err := validateReminderOffsetDays(&offsetDays); err != nil {
		return err
	}

	_, scope, err := s.gateStandaloneRecurringWrite(ctx, actor, id)
	if err != nil {
		return err
	}

	return s.runInTx(ctx, func(stores *txStores) error {
		rec, err := lockRecurringOperationForUpdate(ctx, stores, id, scope)
		if err != nil {
			return err
		}

		if err := validatePropertyNotArchived(ctx, stores.properties, scope, rec.PropertyID); err != nil {
			return err
		}

		ops, err := stores.operations.ListByRecurringOperation(ctx, id)
		if err != nil {
			return fmt.Errorf("list operations: %w", err)
		}

		today, err := ownerTodayFor(ctx, s.tzResolver, scope, s.clock.Now())
		if err != nil {
			return err
		}
		return s.applyReminderOffset(ctx, stores, scope, rec, ops, offsetDays, today)
	})
}

// applyReminderOffset stores the offset and rebuilds concrete reminders inside
// the caller's transaction; the caller's runInTx commits the change.
func (s *RecurringOperationService) applyReminderOffset(
	ctx context.Context,
	stores *txStores,
	scope uuid.UUID,
	rec domain.RecurringOperation,
	ops []domain.Operation,
	offsetDays int,
	now time.Time,
) error {
	normalizedOffsetDays := normalizeReminderOffsetDays(&offsetDays)

	if err := stores.recurringOps.SetReminderOffset(ctx, scope, rec.ID, normalizedOffsetDays); err != nil {
		return fmt.Errorf("set reminder offset: %w", err)
	}
	if err := stores.operations.UpdateFutureGeneratedOperationReminderOffsets(ctx, scope, rec.ID, normalizedOffsetDays, now); err != nil {
		return fmt.Errorf("sync generated operation reminder offsets: %w", err)
	}

	if stores.scheduler == nil {
		return nil
	}

	categoryNames, err := buildCategoryNamesMap(ctx, stores.categories, scope)
	if err != nil {
		return err
	}
	if err := cancelRecurringOperationReminders(ctx, stores, scope, rec.ID); err != nil {
		return err
	}
	if normalizedOffsetDays == nil {
		return nil
	}

	rec.ReminderOffsetDays = normalizedOffsetDays
	if err := scheduleRemindersForOperations(ctx, stores.scheduler, rec, ops, categoryNames, now); err != nil {
		return fmt.Errorf("schedule reminders: %w", err)
	}
	return nil
}

// CreateReminder creates concrete reminders for all future generated operations
// of a recurring operation using the reminder date selected by the user. It
// persists the computed offset and returns the created reminders.
func (s *RecurringOperationService) CreateReminder(
	ctx context.Context,
	actor, recurringOperationID uuid.UUID,
	reminderDate time.Time,
) ([]notificationsdomain.Reminder, error) {
	if s.reminders == nil {
		return nil, errors.New("reminder lister is required")
	}

	_, scope, err := s.gateStandaloneRecurringWrite(ctx, actor, recurringOperationID)
	if err != nil {
		return nil, err
	}

	loc, err := s.tzResolver.Resolve(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("resolve owner timezone: %w", err)
	}
	now := s.clock.Now()
	today := timeutil.DateIn(now, loc)
	if err := notificationsdomain.ValidateReminderDate(reminderDate, now, loc); err != nil {
		return nil, newInvalidInputError("reminder date must be today or in the future")
	}

	err = s.runInTx(ctx, func(stores *txStores) error {
		rec, err := lockRecurringOperationForUpdate(ctx, stores, recurringOperationID, scope)
		if err != nil {
			return err
		}

		if err := validatePropertyNotArchived(ctx, stores.properties, scope, rec.PropertyID); err != nil {
			return err
		}

		ops, err := stores.operations.ListByRecurringOperation(ctx, recurringOperationID)
		if err != nil {
			return fmt.Errorf("list operations: %w", err)
		}

		futureOps := futureOperations(ops, today)
		if len(futureOps) == 0 {
			return newInvalidInputError("no future operations for reminder")
		}

		earliest := earliestFutureOperation(futureOps)
		offsetDays := notificationsdomain.ReminderOffset(earliest.OperationDate, reminderDate, loc)
		if offsetDays < 0 {
			return newInvalidInputError("reminder date must be on or before the earliest future operation date")
		}

		return s.applyReminderOffset(ctx, stores, scope, rec, futureOps, offsetDays, today)
	})
	if err != nil {
		return nil, err
	}

	reminders, err := s.reminders.ListByRecurringOperation(ctx, scope, recurringOperationID, notificationsapp.ListFilter{Limit: 3000})
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	return reminders, nil
}

// gateStandaloneRecurringWrite loads the recurring operation by id, resolves
// the actor's role through the T3 standalone write gate, and returns the
// audit role and the data-owner scope for the transactional phase.
func (s *RecurringOperationService) gateStandaloneRecurringWrite(
	ctx context.Context, actor, id uuid.UUID,
) (sharedpolicy.Role, uuid.UUID, error) {
	rec, err := s.recurringOps.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", uuid.Nil, ErrNotFound
		}
		return "", uuid.Nil, fmt.Errorf("get recurring operation: %w", err)
	}
	role, err := roleForStandalone(ctx, s.policy, actor, rec.PropertyID, rec.OwnerID)
	if err != nil {
		return "", uuid.Nil, err
	}
	if err := writeRoleGate(role); err != nil {
		return "", uuid.Nil, err
	}
	return role, rec.OwnerID, nil
}

// lockRecurringOperationForUpdate re-reads the recurring operation row under
// the transaction lock.
func lockRecurringOperationForUpdate(ctx context.Context, stores *txStores, id, scope uuid.UUID) (domain.RecurringOperation, error) {
	rec, err := stores.recurringOps.GetByIDAndOwnerForUpdate(ctx, id, scope)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.RecurringOperation{}, ErrNotFound
		}
		return domain.RecurringOperation{}, fmt.Errorf("get recurring operation: %w", err)
	}
	return rec, nil
}

// updateRecurringOperationOrNotFound persists the recurring operation, mapping
// a missing row back to ErrNotFound.
func updateRecurringOperationOrNotFound(
	ctx context.Context,
	stores *txStores,
	rec domain.RecurringOperation,
) (domain.RecurringOperation, error) {
	updated, err := stores.recurringOps.Update(ctx, rec)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.RecurringOperation{}, ErrNotFound
		}
		return domain.RecurringOperation{}, fmt.Errorf("update recurring operation: %w", err)
	}
	return updated, nil
}

// updateRecurringStatusOrNotFound transitions the recurring operation to the
// target status, mapping a missing row back to ErrNotFound; failAction names
// the transition in the error message.
func updateRecurringStatusOrNotFound(
	ctx context.Context,
	stores *txStores,
	id, scope uuid.UUID,
	status domain.RecurringOperationStatus,
	failAction string,
) (domain.RecurringOperation, error) {
	rec, err := stores.recurringOps.UpdateStatus(ctx, id, scope, string(status))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.RecurringOperation{}, ErrNotFound
		}
		return domain.RecurringOperation{}, fmt.Errorf("%s: %w", failAction, err)
	}
	return rec, nil
}

// cancelRecurringOperationReminders cancels the concrete reminders of a
// recurring operation.
func cancelRecurringOperationReminders(ctx context.Context, stores *txStores, scope, recurringOpID uuid.UUID) error {
	if err := stores.scheduler.CancelByRecurringOperation(ctx, scope, recurringOpID); err != nil {
		return fmt.Errorf("cancel recurring reminders: %w", err)
	}
	return nil
}

// scheduleRecurringRemindersForSeries rebuilds the concrete reminders of a
// recurring series from its persisted operations and the owner's category
// names.
func scheduleRecurringRemindersForSeries(
	ctx context.Context,
	stores *txStores,
	scope uuid.UUID,
	rec domain.RecurringOperation,
	now time.Time,
) error {
	persistedOps, err := stores.operations.ListByRecurringOperation(ctx, rec.ID)
	if err != nil {
		return fmt.Errorf("list operations for scheduling: %w", err)
	}
	categoryNames, err := buildCategoryNamesMap(ctx, stores.categories, scope)
	if err != nil {
		return err
	}
	if err := scheduleRemindersForOperations(ctx, stores.scheduler, rec, persistedOps, categoryNames, now); err != nil {
		return fmt.Errorf("schedule reminders: %w", err)
	}
	return nil
}

// scheduleNewRecurringReminders schedules the reminders of a recurring series
// when a reminder scheduler is wired and the series carries a reminder offset.
func scheduleNewRecurringReminders(
	ctx context.Context,
	stores *txStores,
	rec domain.RecurringOperation,
	scope uuid.UUID,
	today time.Time,
) error {
	if stores.scheduler == nil || rec.ReminderOffsetDays == nil {
		return nil
	}
	return scheduleRecurringRemindersForSeries(ctx, stores, scope, rec, today)
}

// parseCommandPeriodicity parses the periodicity of a command, defaulting to
// monthly when the value is empty.
func parseCommandPeriodicity(v string) (domain.RecurringOperationPeriodicity, error) {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return domain.RecurringOperationPeriodicityMonthly, nil
	}
	periodicity, err := domain.ParseRecurringOperationPeriodicity(trimmed)
	if err != nil {
		return "", newInvalidInputError(err.Error())
	}
	return periodicity, nil
}

// paymentDayOrDefault falls back to the start-date day number when the create
// command omits the payment day.
func paymentDayOrDefault(paymentDay int, startDate time.Time) int {
	if paymentDay == 0 {
		return startDate.Day()
	}
	return paymentDay
}

// earliestFutureOperation returns the future operation with the earliest
// operation date.
func earliestFutureOperation(futureOps []domain.Operation) domain.Operation {
	earliest := futureOps[0]
	for _, op := range futureOps {
		if op.OperationDate.Before(earliest.OperationDate) {
			earliest = op
		}
	}
	return earliest
}

func (s *RecurringOperationService) validateCommand(
	ctx context.Context,
	categories OperationCategoryRepository,
	scope uuid.UUID,
	opType string,
	categoryID uuid.UUID,
	amount int64,
	startDate time.Time,
	paymentDay int,
	endDate *time.Time,
) error {
	parsedType, _, err := parseTypeAndCategory(opType, categoryID)
	if err != nil {
		return err
	}
	if err := validateCategory(ctx, categories, scope, parsedType, categoryID); err != nil {
		return err
	}
	if amount < 0 {
		return newInvalidInputError("amount must be non-negative")
	}
	if startDate.IsZero() {
		return newInvalidInputError("start_date is required")
	}
	if paymentDay < 1 || paymentDay > 31 {
		return newInvalidInputError("payment_day must be between 1 and 31")
	}
	if endDate != nil && timeutil.Date(*endDate).Before(timeutil.Date(startDate)) {
		return newInvalidInputError("end_date must be on or after start_date")
	}
	return nil
}

func (s *RecurringOperationService) existingOperationDates(
	ctx context.Context,
	ops OperationRepository,
	recurringOperationID uuid.UUID,
) (map[time.Time]struct{}, error) {
	existing, err := ops.ListOperationDatesByRecurringOperation(ctx, recurringOperationID)
	if err != nil {
		return nil, err
	}
	dates := make(map[time.Time]struct{}, len(existing))
	for _, d := range existing {
		dates[timeutil.Date(d)] = struct{}{}
	}
	return dates, nil
}

func (s *RecurringOperationService) generateOperations(
	ctx context.Context,
	ops OperationRepository,
	rec domain.RecurringOperation,
	now time.Time,
	filter func(time.Time) bool,
) error {
	existing, err := s.existingOperationDates(ctx, ops, rec.ID)
	if err != nil {
		return fmt.Errorf("list existing dates: %w", err)
	}

	toCreate, err := s.buildOperations(rec, now, existing, filter)
	if err != nil {
		return err
	}
	if len(toCreate) == 0 {
		return nil
	}

	if err := ops.BulkCreate(ctx, toCreate); err != nil {
		return fmt.Errorf("bulk create operations: %w", err)
	}
	return nil
}

func (s *RecurringOperationService) buildOperations(
	rec domain.RecurringOperation,
	now time.Time,
	existing map[time.Time]struct{},
	filter func(time.Time) bool,
) ([]domain.Operation, error) {
	dates := domain.GenerateDates(rec.StartDate, rec.PaymentDay, rec.EndDate, now, rec.Periodicity)

	createdAt := s.clock.Now()
	ops := make([]domain.Operation, 0, len(dates))
	for _, d := range dates {
		d = timeutil.Date(d)
		if filter != nil && !filter(d) {
			continue
		}
		if _, ok := existing[d]; ok {
			continue
		}

		opID, err := uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("generate operation id: %w", err)
		}

		sourceDate := d
		ops = append(ops, domain.Operation{
			ID:                   opID,
			OwnerID:              rec.OwnerID,
			PropertyID:           rec.PropertyID,
			LeaseID:              rec.LeaseID,
			RecurringOperationID: rec.ID,
			Type:                 rec.Type,
			CategoryID:           rec.CategoryID,
			Status:               operationStatusForDate(d, now),
			Name:                 rec.Name,
			AmountKopecks:        rec.AmountKopecks,
			OperationDate:        d,
			SourceOperationDate:  &sourceDate,
			Comment:              rec.Comment,
			ReminderOffsetDays:   rec.ReminderOffsetDays,
			IsException:          false,
			CreatedAt:            createdAt,
			UpdatedAt:            createdAt,
		})
	}

	return ops, nil
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// updatedRecurringOperationFields lists the names of the fields a command
// changes. Only field names are audited, never their values.
func updatedRecurringOperationFields(cmd UpdateRecurringOperationCommand) []string {
	fields := make([]string, 0, 11)
	if cmd.Type != nil {
		fields = append(fields, "type")
	}
	if cmd.CategoryID != nil {
		fields = append(fields, "category_id")
	}
	if cmd.Name != nil {
		fields = append(fields, "name")
	}
	if cmd.AmountKopecks != nil {
		fields = append(fields, "amount_kopecks")
	}
	if cmd.StartDate != nil {
		fields = append(fields, "start_date")
	}
	if cmd.PaymentDay != nil {
		fields = append(fields, "payment_day")
	}
	if cmd.EndDate != nil {
		fields = append(fields, "end_date")
	}
	if cmd.Comment != nil {
		fields = append(fields, "comment")
	}
	if cmd.Periodicity != nil {
		fields = append(fields, "periodicity")
	}
	if cmd.ApplyFromDate != nil {
		fields = append(fields, "apply_from_date")
	}
	if cmd.ReminderOffsetDays != nil {
		fields = append(fields, "reminder_offset_days")
	}
	return fields
}

func buildCategoryNamesMap(ctx context.Context, categories OperationCategoryRepository, scope uuid.UUID) (map[uuid.UUID]string, error) {
	cats, err := categories.ListByOwner(ctx, scope, nil)
	if err != nil {
		return nil, fmt.Errorf("list categories for reminders: %w", err)
	}
	categoryNames := make(map[uuid.UUID]string, len(cats))
	for _, c := range cats {
		categoryNames[c.ID] = c.Name
	}
	return categoryNames, nil
}

func futureOperations(ops []domain.Operation, now time.Time) []domain.Operation {
	out := make([]domain.Operation, 0, len(ops))
	for _, op := range ops {
		if !timeutil.BeforeDay(op.OperationDate, now) {
			out = append(out, op)
		}
	}
	return out
}

func scheduleRemindersForOperations(
	ctx context.Context,
	scheduler ReminderScheduler,
	rec domain.RecurringOperation,
	ops []domain.Operation,
	categoryNames map[uuid.UUID]string,
	now time.Time,
) error {
	if scheduler == nil {
		return nil
	}
	if len(ops) == 0 || rec.ReminderOffsetDays == nil {
		return nil
	}

	offsetDays := *rec.ReminderOffsetDays

	filtered := make([]domain.Operation, 0, len(ops))
	var earliestOp domain.Operation
	for i := range ops {
		op := ops[i]
		reminderDate := op.OperationDate.AddDate(0, 0, -offsetDays)
		if timeutil.BeforeDay(reminderDate, now) {
			continue
		}
		filtered = append(filtered, op)
		if earliestOp.OperationDate.IsZero() || op.OperationDate.Before(earliestOp.OperationDate) {
			earliestOp = op
		}
	}
	if len(filtered) == 0 {
		return nil
	}

	baseReminderDate := earliestOp.OperationDate.AddDate(0, 0, -offsetDays)

	recInfo := notificationsapp.RecurringOperationInfo{
		ID:         rec.ID,
		OwnerID:    rec.OwnerID,
		PropertyID: rec.PropertyID,
		LeaseID:    domain.LeaseIDPtr(rec.LeaseID),
	}

	if err := scheduler.ScheduleForRecurringOperation(
		ctx, recInfo, baseReminderDate, ToOperationInfoSlice(filtered, categoryNames),
	); err != nil {
		if errors.Is(err, notificationsdomain.ErrInvalidReminderDate) {
			return newInvalidInputError("reminder date must be today or in the future")
		}
		return err
	}
	return nil
}
