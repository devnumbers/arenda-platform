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

	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return domain.RecurringOperation{}, newInvalidInputError("name is required")
	}
	if len([]rune(name)) > 50 {
		return domain.RecurringOperation{}, newInvalidInputError("name must be at most 50 characters")
	}

	periodicity := domain.RecurringOperationPeriodicityMonthly
	if strings.TrimSpace(cmd.Periodicity) != "" {
		p, err := domain.ParseRecurringOperationPeriodicity(strings.TrimSpace(cmd.Periodicity))
		if err != nil {
			return domain.RecurringOperation{}, newInvalidInputError(err.Error())
		}
		periodicity = p
	}

	if err := validateReminderOffsetDays(cmd.ReminderOffsetDays); err != nil {
		return domain.RecurringOperation{}, err
	}
	cmd.ReminderOffsetDays = normalizeReminderOffsetDays(cmd.ReminderOffsetDays)

	paymentDay := cmd.PaymentDay
	if paymentDay == 0 {
		paymentDay = cmd.StartDate.Day()
	}

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

	loc, err := s.tzResolver.Resolve(ctx, scope)
	if err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("resolve owner timezone: %w", err)
	}
	now := s.clock.Now()
	today := timeutil.DateIn(now, loc)
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

		err = s.generateOperations(ctx, stores.operations, created, today, nil)
		if err != nil {
			return fmt.Errorf("generate operations: %w", err)
		}

		if stores.scheduler != nil && created.ReminderOffsetDays != nil {
			persistedOps, err := stores.operations.ListByRecurringOperation(ctx, created.ID)
			if err != nil {
				return fmt.Errorf("list operations for scheduling: %w", err)
			}
			categoryNames, err := buildCategoryNamesMap(ctx, stores.categories, scope)
			if err != nil {
				return err
			}
			if err := scheduleRemindersForOperations(ctx, stores.scheduler, created, persistedOps, categoryNames, today); err != nil {
				return fmt.Errorf("schedule reminders: %w", err)
			}
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
	rec, err := s.recurringOps.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("get recurring operation: %w", err)
	}
	role, err := roleForStandalone(ctx, s.policy, actor, rec.PropertyID, rec.OwnerID)
	if err != nil {
		return err
	}
	if err := writeRoleGate(role); err != nil {
		return err
	}
	scope := rec.OwnerID

	return s.runInTx(ctx, func(stores *txStores) error {
		rec, err := stores.recurringOps.GetByIDAndOwnerForUpdate(ctx, id, scope)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("get recurring operation: %w", err)
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
			if err := stores.scheduler.CancelByRecurringOperation(ctx, scope, id); err != nil {
				return fmt.Errorf("cancel recurring reminders: %w", err)
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

	if cmd.Name != nil {
		name := strings.TrimSpace(*cmd.Name)
		if name == "" {
			return domain.RecurringOperation{}, newInvalidInputError("name is required")
		}
		if len([]rune(name)) > 50 {
			return domain.RecurringOperation{}, newInvalidInputError("name must be at most 50 characters")
		}
		rec.Name = name
	}

	if cmd.Periodicity != nil {
		periodicity, err := domain.ParseRecurringOperationPeriodicity(strings.TrimSpace(*cmd.Periodicity))
		if err != nil {
			return domain.RecurringOperation{}, newInvalidInputError(err.Error())
		}
		rec.Periodicity = periodicity
	}

	if cmd.AmountKopecks != nil {
		rec.AmountKopecks = *cmd.AmountKopecks
	}
	if cmd.StartDate != nil {
		rec.StartDate = *cmd.StartDate
	}
	if cmd.PaymentDay != nil {
		rec.PaymentDay = *cmd.PaymentDay
	} else if rec.LeaseID == uuid.Nil {
		rec.PaymentDay = rec.StartDate.Day()
	}
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
		if err := stores.scheduler.CancelByRecurringOperation(ctx, scope, updated.ID); err != nil {
			return fmt.Errorf("cancel recurring reminders: %w", err)
		}
	}

	if err := stores.operations.DeleteUneditedFutureOperationsByRecurringOperation(ctx, updated.ID, today); err != nil {
		return fmt.Errorf("delete future operations: %w", err)
	}

	err := s.generateOperations(ctx, stores.operations, updated, today, func(d time.Time) bool {
		return !timeutil.Date(d).Before(today)
	})
	if err != nil {
		return fmt.Errorf("regenerate operations: %w", err)
	}

	if stores.scheduler != nil && updated.ReminderOffsetDays != nil {
		persistedOps, err := stores.operations.ListByRecurringOperation(ctx, updated.ID)
		if err != nil {
			return fmt.Errorf("list operations for scheduling: %w", err)
		}
		categoryNames, err := buildCategoryNamesMap(ctx, stores.categories, scope)
		if err != nil {
			return err
		}
		if err := scheduleRemindersForOperations(ctx, stores.scheduler, updated, persistedOps, categoryNames, today); err != nil {
			return fmt.Errorf("schedule reminders: %w", err)
		}
	}
	return nil
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
	if err := writeRoleGate(role); err != nil {
		return domain.RecurringOperation{}, err
	}
	scope := rec.OwnerID

	var result domain.RecurringOperation
	err = s.runInTx(ctx, func(stores *txStores) error {
		rec, err := stores.recurringOps.GetByIDAndOwnerForUpdate(ctx, id, scope)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("get recurring operation: %w", err)
		}

		if err := validatePropertyNotArchived(ctx, stores.properties, scope, rec.PropertyID); err != nil {
			return err
		}

		loc, err := s.tzResolver.Resolve(ctx, scope)
		if err != nil {
			return fmt.Errorf("resolve owner timezone: %w", err)
		}
		now := s.clock.Now()
		today := timeutil.DateIn(now, loc)
		originalReminderOffset := rec.ReminderOffsetDays

		if cmd.ApplyFromDate != nil {
			if cmd.StartDate != nil {
				return newInvalidInputError("start_date cannot be used with apply_from_date")
			}
			newRec, err := s.splitRecurringOperationSeries(ctx, stores, scope, rec, cmd, today)
			if err != nil {
				return err
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
				return fmt.Errorf("record audit: %w", err)
			}
			result = newRec
			return nil
		}

		rec, err = s.applyRecurringOperationUpdate(ctx, stores.categories, rec, cmd, scope, now)
		if err != nil {
			return err
		}

		updated, err := stores.recurringOps.Update(ctx, rec)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("update recurring operation: %w", err)
		}

		if err := s.resyncRecurringSeries(ctx, stores, updated, cmd, scope, today, originalReminderOffset != nil); err != nil {
			return err
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  actorRoleFromPolicyRole(role),
			Action:     auditdomain.ActionRecurringOperationUpdated,
			EntityType: auditdomain.EntityRecurringOperation,
			EntityID:   &id,
			Context:    map[string]any{auditKeyFields: updatedRecurringOperationFields(cmd)},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}

		result = updated
		return nil
	})
	if err != nil {
		return domain.RecurringOperation{}, err
	}

	return result, nil
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

	name := rec.Name
	if cmd.Name != nil {
		name = strings.TrimSpace(*cmd.Name)
		if name == "" {
			return domain.RecurringOperation{}, newInvalidInputError("name is required")
		}
		if len([]rune(name)) > 50 {
			return domain.RecurringOperation{}, newInvalidInputError("name must be at most 50 characters")
		}
	}

	amountKopecks := rec.AmountKopecks
	if cmd.AmountKopecks != nil {
		amountKopecks = *cmd.AmountKopecks
	}

	paymentDay := rec.PaymentDay
	if cmd.PaymentDay != nil {
		paymentDay = *cmd.PaymentDay
	} else if rec.LeaseID == uuid.Nil {
		paymentDay = applyFromDate.Day()
	}

	periodicity := rec.Periodicity
	if cmd.Periodicity != nil {
		periodicity, err = domain.ParseRecurringOperationPeriodicity(strings.TrimSpace(*cmd.Periodicity))
		if err != nil {
			return domain.RecurringOperation{}, newInvalidInputError(err.Error())
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

	newEndDate := rec.EndDate
	if cmd.EndDate != nil {
		d := timeutil.Date(*cmd.EndDate)
		newEndDate = &d
	}
	if newEndDate != nil {
		if applyFromDate.After(*newEndDate) {
			return domain.RecurringOperation{}, newInvalidInputError("end_date must be on or after apply_from_date")
		}
	}

	if err := s.validateCommand(
		ctx, txCategories, scope, string(opType), categoryID,
		amountKopecks, applyFromDate, paymentDay, newEndDate,
	); err != nil {
		return domain.RecurringOperation{}, err
	}

	return domain.RecurringOperation{
		ID:                 newID,
		OwnerID:            scope,
		PropertyID:         rec.PropertyID,
		LeaseID:            rec.LeaseID,
		Type:               opType,
		CategoryID:         categoryID,
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
	// The caller passes now already normalized to midnight in the owner's timezone.
	today := now

	if applyFromDate.Before(today) {
		return domain.RecurringOperation{}, newInvalidInputError("apply_from_date must be today or in the future")
	}
	if !applyFromDate.After(timeutil.Date(rec.StartDate)) {
		return domain.RecurringOperation{}, newInvalidInputError("apply_from_date must be after the series start date")
	}
	if rec.EndDate != nil && applyFromDate.After(timeutil.Date(*rec.EndDate)) {
		return domain.RecurringOperation{}, newInvalidInputError("apply_from_date must be on or before the series end date")
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

	if stores.scheduler != nil && (rec.ReminderOffsetDays != nil || created.ReminderOffsetDays != nil) {
		categoryNames, err := buildCategoryNamesMap(ctx, stores.categories, scope)
		if err != nil {
			return domain.RecurringOperation{}, err
		}
		if err := stores.scheduler.CancelByRecurringOperation(ctx, scope, rec.ID); err != nil {
			return domain.RecurringOperation{}, fmt.Errorf("cancel recurring reminders: %w", err)
		}

		retainedOps, err := stores.operations.ListByRecurringOperation(ctx, rec.ID)
		if err != nil {
			return domain.RecurringOperation{}, fmt.Errorf("list retained operations for scheduling: %w", err)
		}
		if err := scheduleRemindersForOperations(ctx, stores.scheduler, rec, retainedOps, categoryNames, now); err != nil {
			return domain.RecurringOperation{}, fmt.Errorf("schedule reminders for retained operations: %w", err)
		}

		persistedOps, err := stores.operations.ListByRecurringOperation(ctx, created.ID)
		if err != nil {
			return domain.RecurringOperation{}, fmt.Errorf("list operations for scheduling: %w", err)
		}
		if err := scheduleRemindersForOperations(ctx, stores.scheduler, created, persistedOps, categoryNames, now); err != nil {
			return domain.RecurringOperation{}, fmt.Errorf("schedule reminders: %w", err)
		}
	}

	return created, nil
}

// PauseRecurringOperation marks a recurring operation as paused and cancels its reminders.
func (s *RecurringOperationService) PauseRecurringOperation(
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
	if err := writeRoleGate(role); err != nil {
		return domain.RecurringOperation{}, err
	}
	scope := rec.OwnerID

	var result domain.RecurringOperation
	err = s.runInTx(ctx, func(stores *txStores) error {
		rec, err := stores.recurringOps.GetByIDAndOwnerForUpdate(ctx, id, scope)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("get recurring operation: %w", err)
		}

		if err := validatePropertyNotArchived(ctx, stores.properties, scope, rec.PropertyID); err != nil {
			return err
		}

		if stores.scheduler != nil {
			if err := stores.scheduler.CancelByRecurringOperation(ctx, scope, id); err != nil {
				return fmt.Errorf("cancel reminders: %w", err)
			}
		}

		rec, err = stores.recurringOps.UpdateStatus(ctx, id, scope, string(domain.RecurringOperationStatusPaused))
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("pause recurring operation: %w", err)
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
	if err := writeRoleGate(role); err != nil {
		return domain.RecurringOperation{}, err
	}
	scope := rec.OwnerID

	var result domain.RecurringOperation
	err = s.runInTx(ctx, func(stores *txStores) error {
		rec, err := stores.recurringOps.GetByIDAndOwnerForUpdate(ctx, id, scope)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("get recurring operation: %w", err)
		}

		if err := validatePropertyNotArchived(ctx, stores.properties, scope, rec.PropertyID); err != nil {
			return err
		}

		if rec.Status == domain.RecurringOperationStatusActive {
			result = rec
			return nil
		}

		rec, err = stores.recurringOps.UpdateStatus(ctx, id, scope, string(domain.RecurringOperationStatusActive))
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("resume recurring operation: %w", err)
		}

		loc, err := s.tzResolver.Resolve(ctx, scope)
		if err != nil {
			return fmt.Errorf("resolve owner timezone: %w", err)
		}
		today := timeutil.DateIn(s.clock.Now(), loc)
		err = s.generateOperations(ctx, stores.operations, rec, today, func(d time.Time) bool {
			return !timeutil.Date(d).Before(today)
		})
		if err != nil {
			return fmt.Errorf("generate operations: %w", err)
		}

		if stores.scheduler != nil && rec.ReminderOffsetDays != nil {
			persistedOps, err := stores.operations.ListByRecurringOperation(ctx, rec.ID)
			if err != nil {
				return fmt.Errorf("list operations for scheduling: %w", err)
			}
			categoryNames, err := buildCategoryNamesMap(ctx, stores.categories, scope)
			if err != nil {
				return err
			}
			if err := scheduleRemindersForOperations(ctx, stores.scheduler, rec, persistedOps, categoryNames, today); err != nil {
				return fmt.Errorf("schedule reminders: %w", err)
			}
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

	rec, err := s.recurringOps.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("get recurring operation: %w", err)
	}
	role, err := roleForStandalone(ctx, s.policy, actor, rec.PropertyID, rec.OwnerID)
	if err != nil {
		return err
	}
	if err := writeRoleGate(role); err != nil {
		return err
	}
	scope := rec.OwnerID

	return s.runInTx(ctx, func(stores *txStores) error {
		rec, err := stores.recurringOps.GetByIDAndOwnerForUpdate(ctx, id, scope)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("get recurring operation: %w", err)
		}

		if err := validatePropertyNotArchived(ctx, stores.properties, scope, rec.PropertyID); err != nil {
			return err
		}

		ops, err := stores.operations.ListByRecurringOperation(ctx, id)
		if err != nil {
			return fmt.Errorf("list operations: %w", err)
		}

		loc, err := s.tzResolver.Resolve(ctx, scope)
		if err != nil {
			return fmt.Errorf("resolve owner timezone: %w", err)
		}
		today := timeutil.DateIn(s.clock.Now(), loc)
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
	if err := stores.scheduler.CancelByRecurringOperation(ctx, scope, rec.ID); err != nil {
		return fmt.Errorf("cancel recurring reminders: %w", err)
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

	rec, err := s.recurringOps.GetByID(ctx, recurringOperationID)
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
	if err := writeRoleGate(role); err != nil {
		return nil, err
	}
	scope := rec.OwnerID

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
		rec, err := stores.recurringOps.GetByIDAndOwnerForUpdate(ctx, recurringOperationID, scope)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("get recurring operation: %w", err)
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

		earliest := futureOps[0]
		for _, op := range futureOps {
			if op.OperationDate.Before(earliest.OperationDate) {
				earliest = op
			}
		}

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
