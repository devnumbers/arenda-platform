package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/timeutil"
	sharedtz "github.com/nambers/arenda-planform/apps/backend/internal/shared/tzresolver"
)

// CreateOperationCommand carries the data needed to create a manual operation.
type CreateOperationCommand struct {
	PropertyID         uuid.UUID
	Type               string
	CategoryID         uuid.UUID
	Name               string
	AmountKopecks      int64
	OperationDate      time.Time
	Comment            *string
	LeaseID            *uuid.UUID
	ReminderOffsetDays *int
}

// UpdateOperationCommand carries the optional updates for an operation.
type UpdateOperationCommand struct {
	Type               *string
	CategoryID         *uuid.UUID
	Name               *string
	AmountKopecks      *int64
	OperationDate      *time.Time
	Comment            *string
	LeaseID            *uuid.UUID
	ReminderOffsetDays *int
}

// CompleteOperationCommand carries the data needed to mark an operation as completed.
type CompleteOperationCommand struct {
	Actor       uuid.UUID
	OperationID uuid.UUID
}

// MarkOperationIncompleteCommand carries the data needed to mark a completed operation as planned again.
type MarkOperationIncompleteCommand struct {
	Actor       uuid.UUID
	OperationID uuid.UUID
}

// MoveOperationCommand carries the target property for moving an operation.
type MoveOperationCommand struct {
	PropertyID uuid.UUID
}

// OperationService orchestrates manual operation use cases within the leases
// bounded context.
type OperationService struct {
	operations   OperationRepository
	properties   PropertyRepository
	leases       LeaseRepository
	recurringOps RecurringOperationRepository
	categories   OperationCategoryRepository
	scheduler    ReminderScheduler
	db           txBeginner
	audit        auditapp.Recorder
	clock        clock.Clock
	tzResolver   sharedtz.OwnerTimezoneResolver
	policy       sharedpolicy.Policy
	// sharedIDs is optionally injected (see SetSharedPropertyIDs); when nil the
	// finance report only covers the actor's own operations (issue #157, T3).
	sharedIDs SharedPropertyIDs
	logger    *slog.Logger
}

// NewOperationService creates a new operation service.
func NewOperationService(
	operations OperationRepository,
	properties PropertyRepository,
	leases LeaseRepository,
	recurringOps RecurringOperationRepository,
	categories OperationCategoryRepository,
	scheduler ReminderScheduler,
	db txBeginner,
	audit auditapp.Recorder,
	clock clock.Clock,
	tzResolver sharedtz.OwnerTimezoneResolver,
	policy sharedpolicy.Policy,
	logger *slog.Logger,
) *OperationService {
	if db == nil {
		panic("db beginner is required")
	}
	if clock == nil {
		panic("clock is required")
	}
	if categories == nil {
		panic("categories repository is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if audit == nil {
		audit = auditapp.Noop{}
	}
	return &OperationService{
		operations:   operations,
		properties:   properties,
		leases:       leases,
		recurringOps: recurringOps,
		categories:   categories,
		scheduler:    scheduler,
		db:           db,
		audit:        audit,
		clock:        clock,
		tzResolver:   tzResolver,
		policy:       policy,
		logger:       logger,
	}
}

// SetSharedPropertyIDs injects the access-context adapter that resolves the
// property ids shared with an actor via property membership (issue #157, T3).
// Optional: when nil, GetFinanceReport only aggregates the actor's own
// operations; when set, it additionally aggregates operations of the shared
// properties.
func (s *OperationService) SetSharedPropertyIDs(ids SharedPropertyIDs) {
	s.sharedIDs = ids
}

// CreateOperation creates a manual operation for the given owner and property.
func (s *OperationService) CreateOperation(ctx context.Context, actor uuid.UUID, cmd CreateOperationCommand) (domain.Operation, error) {
	role, scope, err := resolveWriteScope(ctx, s.policy, s.properties, actor, cmd.PropertyID)
	if err != nil {
		return domain.Operation{}, err
	}

	if err := validatePropertyNotArchived(ctx, s.properties, scope, cmd.PropertyID); err != nil {
		return domain.Operation{}, err
	}

	opType, categoryID, err := parseTypeAndCategory(cmd.Type, cmd.CategoryID)
	if err != nil {
		return domain.Operation{}, err
	}
	if err := validateCategory(ctx, s.categories, scope, opType, categoryID); err != nil {
		return domain.Operation{}, err
	}

	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return domain.Operation{}, newInvalidInputError("name is required")
	}
	if len([]rune(name)) > 50 {
		return domain.Operation{}, newInvalidInputError("name must be at most 50 characters")
	}

	if err := s.validateAmountAndDate(cmd.AmountKopecks, cmd.OperationDate); err != nil {
		return domain.Operation{}, err
	}

	if err := validateReminderOffsetDays(cmd.ReminderOffsetDays); err != nil {
		return domain.Operation{}, err
	}
	cmd.ReminderOffsetDays = normalizeReminderOffsetDays(cmd.ReminderOffsetDays)

	leaseID, err := s.resolveLeaseID(ctx, scope, cmd.PropertyID, cmd.LeaseID)
	if err != nil {
		return domain.Operation{}, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return domain.Operation{}, fmt.Errorf("generate operation id: %w", err)
	}

	loc, err := s.tzResolver.Resolve(ctx, scope)
	if err != nil {
		return domain.Operation{}, fmt.Errorf("resolve owner timezone: %w", err)
	}
	now := s.clock.Now()
	today := timeutil.DateIn(now, loc)
	var comment string
	if cmd.Comment != nil {
		comment = *cmd.Comment
	}

	op := domain.Operation{
		ID:                 id,
		OwnerID:            scope,
		PropertyID:         cmd.PropertyID,
		LeaseID:            leaseID,
		Type:               opType,
		CategoryID:         categoryID,
		Status:             operationStatusForDate(cmd.OperationDate, today),
		Name:               name,
		AmountKopecks:      cmd.AmountKopecks,
		OperationDate:      cmd.OperationDate,
		Comment:            comment,
		ReminderOffsetDays: cmd.ReminderOffsetDays,
		IsException:        true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := op.ValidateStatusForType(); err != nil {
		return domain.Operation{}, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Operation{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txOps := s.operations.WithTx(tx)
	created, err := txOps.Create(ctx, op)
	if err != nil {
		return domain.Operation{}, fmt.Errorf("create operation: %w", err)
	}

	if s.scheduler != nil && created.ReminderOffsetDays != nil {
		txScheduler := s.scheduler.WithTx(tx)
		txCategories := s.categories.WithTx(tx)
		cat, err := txCategories.GetByIDAndOwner(ctx, created.CategoryID, scope)
		if err != nil {
			return domain.Operation{}, fmt.Errorf("get category for reminder: %w", err)
		}
		reminderDate := created.OperationDate.AddDate(0, 0, -(*created.ReminderOffsetDays))
		if !reminderDate.Before(today) {
			if err := txScheduler.ScheduleForOperation(ctx, ToOperationInfo(created, cat.Name), reminderDate); err != nil {
				return domain.Operation{}, fmt.Errorf("schedule operation reminder: %w", err)
			}
		}
	}

	opCtx := map[string]any{
		"property_id":    domain.PropertyIDPtr(created.PropertyID),
		"type":           string(created.Type),
		"amount_kopecks": created.AmountKopecks,
		"operation_date": created.OperationDate.Format(time.DateOnly),
	}
	if created.LeaseID != uuid.Nil {
		opCtx["lease_id"] = created.LeaseID
	}
	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  actorRoleFromPolicyRole(role),
		Action:     auditdomain.ActionOperationCreated,
		EntityType: auditdomain.EntityOperation,
		EntityID:   &created.ID,
		Context:    opCtx,
	}); err != nil {
		return domain.Operation{}, fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Operation{}, fmt.Errorf("commit tx: %w", err)
	}

	return created, nil
}

func (s *OperationService) GetPropertyOperationsSummary(ctx context.Context, actor, propertyID uuid.UUID) (OperationsSummary, error) {
	scope, err := resolveReadScope(ctx, s.policy, s.properties, actor, propertyID)
	if err != nil {
		return OperationsSummary{}, err
	}
	loc, err := s.tzResolver.Resolve(ctx, scope)
	if err != nil {
		return OperationsSummary{}, fmt.Errorf("resolve owner timezone: %w", err)
	}
	return s.operations.GetPropertyOperationsSummary(ctx, scope, propertyID, timeutil.DateIn(s.clock.Now(), loc))
}

type FinanceReport struct {
	Totals     FinanceReportTotals
	ByProperty []FinanceReportPropertyRow
	ByCategory []FinanceReportCategoryRow
	ByMonth    []FinanceReportMonthRow
}

func (s *OperationService) GetFinanceReport(ctx context.Context, actor uuid.UUID, from, to *time.Time) (FinanceReport, error) {
	// accessiblePropertyIDs restricts the report to the actor's own operations
	// plus operations of properties shared with the actor (issue #157, T3). For
	// the owner this is empty and the report covers all of the owner's
	// operations; for a member it additionally includes the shared properties'
	// operations (matched by property_id, since their owner_id differs).
	var accessible []uuid.UUID
	if s.sharedIDs != nil {
		shared, err := s.sharedIDs.SharedWith(ctx, actor)
		if err != nil {
			return FinanceReport{}, fmt.Errorf("list shared property ids: %w", err)
		}
		accessible = shared
	}

	totals, err := s.operations.GetFinanceReportTotals(ctx, actor, accessible, from, to)
	if err != nil {
		return FinanceReport{}, fmt.Errorf("report totals: %w", err)
	}
	byProperty, err := s.operations.GetFinanceReportByProperty(ctx, actor, accessible, from, to)
	if err != nil {
		return FinanceReport{}, fmt.Errorf("report by property: %w", err)
	}
	byCategory, err := s.operations.GetFinanceReportByCategory(ctx, actor, accessible, from, to)
	if err != nil {
		return FinanceReport{}, fmt.Errorf("report by category: %w", err)
	}
	byMonth, err := s.operations.GetFinanceReportByMonth(ctx, actor, accessible, from, to)
	if err != nil {
		return FinanceReport{}, fmt.Errorf("report by month: %w", err)
	}
	return FinanceReport{
		Totals:     totals,
		ByProperty: byProperty,
		ByCategory: byCategory,
		ByMonth:    byMonth,
	}, nil
}

// ListOperations returns the actor's own operations plus, when the shared-ids
// adapter is injected, operations of properties shared with the actor (issue
// #157, T3). Shared-property operations are restricted to active/maintenance
// properties; the actor's own operations (including archived) are unchanged.
func (s *OperationService) ListOperations(ctx context.Context, actor uuid.UUID, filter OperationFilter) ([]domain.Operation, error) {
	if s.sharedIDs != nil {
		shared, err := s.sharedIDs.SharedWith(ctx, actor)
		if err != nil {
			return nil, fmt.Errorf("list shared property ids: %w", err)
		}
		filter.AccessiblePropertyIDs = shared
	}
	ops, err := s.operations.ListByOwner(ctx, actor, filter)
	if err != nil {
		return nil, fmt.Errorf("list operations: %w", err)
	}
	return ops, nil
}

// ListOperationsByProperty returns operations for the given owner and property,
// filtered by the provided criteria.
func (s *OperationService) ListOperationsByProperty(ctx context.Context, actor, propertyID uuid.UUID, filter OperationFilter) ([]domain.Operation, error) {
	scope, err := resolveReadScope(ctx, s.policy, s.properties, actor, propertyID)
	if err != nil {
		return nil, err
	}

	filter.PropertyID = propertyID
	ops, err := s.operations.ListByOwner(ctx, scope, filter)
	if err != nil {
		return nil, fmt.Errorf("list operations: %w", err)
	}
	return ops, nil
}

// GetOperation returns a single operation after the T3 shared-access read gate.
func (s *OperationService) GetOperation(ctx context.Context, actor, id uuid.UUID) (domain.Operation, error) {
	op, err := s.operations.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Operation{}, ErrNotFound
		}
		return domain.Operation{}, fmt.Errorf("get operation: %w", err)
	}
	role, err := roleForStandalone(ctx, s.policy, actor, op.PropertyID, op.OwnerID)
	if err != nil {
		return domain.Operation{}, err
	}
	if !sharedpolicy.CanView(role) {
		return domain.Operation{}, ErrNotFound
	}
	return op, nil
}

// UpdateOperation updates an operation after the T3 shared-access write gate.
func (s *OperationService) UpdateOperation(ctx context.Context, actor, id uuid.UUID, cmd UpdateOperationCommand) (domain.Operation, error) {
	op, err := s.operations.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Operation{}, ErrNotFound
		}
		return domain.Operation{}, fmt.Errorf("get operation: %w", err)
	}
	role, err := roleForStandalone(ctx, s.policy, actor, op.PropertyID, op.OwnerID)
	if err != nil {
		return domain.Operation{}, err
	}
	if err := writeRoleGate(role); err != nil {
		return domain.Operation{}, err
	}
	scope := op.OwnerID

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Operation{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txOps := s.operations.WithTx(tx)
	txCategories := s.categories.WithTx(tx)

	op, err = txOps.GetByIDAndOwnerForUpdate(ctx, id, scope)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Operation{}, ErrNotFound
		}
		return domain.Operation{}, fmt.Errorf("get operation: %w", err)
	}

	if err := validatePropertyNotArchived(ctx, s.properties.WithTx(tx), scope, op.PropertyID); err != nil {
		return domain.Operation{}, err
	}

	originalOffset := op.ReminderOffsetDays
	originalOperationDate := op.OperationDate

	opType := op.Type
	categoryID := op.CategoryID
	if cmd.Type != nil {
		parsedType, err := domain.ParseOperationType(*cmd.Type)
		if err != nil {
			return domain.Operation{}, err
		}
		opType = parsedType
	}
	if cmd.CategoryID != nil {
		categoryID = *cmd.CategoryID
	}
	if cmd.Type != nil || cmd.CategoryID != nil {
		if err := validateCategory(ctx, txCategories, scope, opType, categoryID); err != nil {
			return domain.Operation{}, err
		}
	}
	op.Type = opType
	op.CategoryID = categoryID

	if cmd.Name != nil {
		name := strings.TrimSpace(*cmd.Name)
		if name == "" {
			return domain.Operation{}, newInvalidInputError("name is required")
		}
		if len([]rune(name)) > 50 {
			return domain.Operation{}, newInvalidInputError("name must be at most 50 characters")
		}
		op.Name = name
	}

	if cmd.AmountKopecks != nil {
		op.AmountKopecks = *cmd.AmountKopecks
	}
	if cmd.OperationDate != nil {
		op.OperationDate = *cmd.OperationDate
	}
	if err := s.validateAmountAndDate(op.AmountKopecks, op.OperationDate); err != nil {
		return domain.Operation{}, err
	}

	if cmd.Comment != nil {
		op.Comment = *cmd.Comment
	}

	if cmd.LeaseID != nil {
		leaseID, err := s.resolveLeaseID(ctx, scope, op.PropertyID, cmd.LeaseID)
		if err != nil {
			return domain.Operation{}, err
		}
		op.LeaseID = leaseID
	}

	if cmd.ReminderOffsetDays != nil {
		if err := validateReminderOffsetDays(cmd.ReminderOffsetDays); err != nil {
			return domain.Operation{}, err
		}
		op.ReminderOffsetDays = normalizeReminderOffsetDays(cmd.ReminderOffsetDays)
	}

	op.IsException = true
	loc, err := s.tzResolver.Resolve(ctx, scope)
	if err != nil {
		return domain.Operation{}, fmt.Errorf("resolve owner timezone: %w", err)
	}
	now := s.clock.Now()
	today := timeutil.DateIn(now, loc)
	if cmd.OperationDate != nil {
		switch {
		case op.Status == domain.OperationStatusOverdue && !op.OperationDate.Before(today):
			op.Status = domain.OperationStatusPending
		case op.Status == domain.OperationStatusPending && op.OperationDate.Before(today):
			op.Status = domain.OperationStatusOverdue
		}
	}
	op.UpdatedAt = now

	updated, err := txOps.Update(ctx, op)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Operation{}, ErrNotFound
		}
		return domain.Operation{}, fmt.Errorf("update operation: %w", err)
	}

	offsetChanged := !reminderOffsetDaysEqual(originalOffset, updated.ReminderOffsetDays)
	dateChanged := !originalOperationDate.Equal(updated.OperationDate)
	if s.scheduler != nil && (offsetChanged || dateChanged) {
		txScheduler := s.scheduler.WithTx(tx)
		cat, err := txCategories.GetByIDAndOwner(ctx, updated.CategoryID, scope)
		if err != nil {
			return domain.Operation{}, fmt.Errorf("get category for reminder: %w", err)
		}

		if err := txScheduler.CancelByOperation(ctx, scope, updated.ID); err != nil {
			return domain.Operation{}, fmt.Errorf("cancel reminders: %w", err)
		}
		if err := txScheduler.CancelOverdueReminderByOperation(ctx, scope, updated.ID); err != nil {
			return domain.Operation{}, fmt.Errorf("cancel overdue reminders: %w", err)
		}

		if updated.ReminderOffsetDays != nil {
			reminderDate := updated.OperationDate.AddDate(0, 0, -(*updated.ReminderOffsetDays))
			if !reminderDate.Before(today) {
				if err := txScheduler.ScheduleForOperation(ctx, ToOperationInfo(updated, cat.Name), reminderDate); err != nil {
					return domain.Operation{}, fmt.Errorf("schedule operation reminder: %w", err)
				}
			}
		}
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  actorRoleFromPolicyRole(role),
		Action:     auditdomain.ActionOperationUpdated,
		EntityType: auditdomain.EntityOperation,
		EntityID:   &id,
		Context:    map[string]any{"fields": updatedOperationFields(cmd)},
	}); err != nil {
		return domain.Operation{}, fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Operation{}, fmt.Errorf("commit tx: %w", err)
	}

	return updated, nil
}

// CompleteOperation marks a pending, overdue, or unconfirmed operation as completed.
// Expenses become paid, income becomes received, and future reminders are cancelled.
func (s *OperationService) CompleteOperation(ctx context.Context, cmd CompleteOperationCommand) (domain.Operation, error) {
	op, err := s.operations.GetByID(ctx, cmd.OperationID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Operation{}, ErrNotFound
		}
		return domain.Operation{}, fmt.Errorf("get operation: %w", err)
	}
	role, err := roleForStandalone(ctx, s.policy, cmd.Actor, op.PropertyID, op.OwnerID)
	if err != nil {
		return domain.Operation{}, err
	}
	if err := writeRoleGate(role); err != nil {
		return domain.Operation{}, err
	}
	scope := op.OwnerID

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Operation{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txOps := s.operations.WithTx(tx)

	op, err = txOps.GetByIDAndOwnerForUpdate(ctx, cmd.OperationID, scope)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Operation{}, ErrNotFound
		}
		return domain.Operation{}, fmt.Errorf("get operation: %w", err)
	}

	if err := validatePropertyNotArchived(ctx, s.properties.WithTx(tx), scope, op.PropertyID); err != nil {
		return domain.Operation{}, err
	}

	if !op.Status.CanComplete() {
		return domain.Operation{}, fmt.Errorf("%w: operation is already completed", ErrOperationAlreadyCompleted)
	}

	switch op.Type {
	case domain.OperationTypeExpense:
		op.Status = domain.OperationStatusPaid
	case domain.OperationTypeIncome:
		op.Status = domain.OperationStatusReceived
	}
	op.UpdatedAt = s.clock.Now()

	if err := op.ValidateStatusForType(); err != nil {
		return domain.Operation{}, err
	}

	updated, err := txOps.Update(ctx, op)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Operation{}, ErrNotFound
		}
		return domain.Operation{}, fmt.Errorf("update operation: %w", err)
	}

	if s.scheduler != nil {
		txScheduler := s.scheduler.WithTx(tx)
		if err := txScheduler.CancelByOperation(ctx, scope, updated.ID); err != nil {
			return domain.Operation{}, fmt.Errorf("cancel reminders: %w", err)
		}
		if err := txScheduler.CancelOverdueReminderByOperation(ctx, scope, updated.ID); err != nil {
			return domain.Operation{}, fmt.Errorf("cancel overdue reminders: %w", err)
		}
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &cmd.Actor,
		ActorRole:  actorRoleFromPolicyRole(role),
		Action:     auditdomain.ActionOperationCompleted,
		EntityType: auditdomain.EntityOperation,
		EntityID:   &cmd.OperationID,
		Context:    map[string]any{"property_id": domain.PropertyIDPtr(op.PropertyID)},
	}); err != nil {
		return domain.Operation{}, fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Operation{}, fmt.Errorf("commit tx: %w", err)
	}

	return updated, nil
}

// MarkOperationIncomplete marks a completed operation as planned again.
func (s *OperationService) MarkOperationIncomplete(ctx context.Context, cmd MarkOperationIncompleteCommand) (domain.Operation, error) {
	op, err := s.operations.GetByID(ctx, cmd.OperationID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Operation{}, ErrNotFound
		}
		return domain.Operation{}, fmt.Errorf("get operation: %w", err)
	}
	role, err := roleForStandalone(ctx, s.policy, cmd.Actor, op.PropertyID, op.OwnerID)
	if err != nil {
		return domain.Operation{}, err
	}
	if err := writeRoleGate(role); err != nil {
		return domain.Operation{}, err
	}
	scope := op.OwnerID

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Operation{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txOps := s.operations.WithTx(tx)

	op, err = txOps.GetByIDAndOwnerForUpdate(ctx, cmd.OperationID, scope)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Operation{}, ErrNotFound
		}
		return domain.Operation{}, fmt.Errorf("get operation: %w", err)
	}

	if err := validatePropertyNotArchived(ctx, s.properties.WithTx(tx), scope, op.PropertyID); err != nil {
		return domain.Operation{}, err
	}

	if !op.Status.IsCompleted() {
		if err := tx.Commit(ctx); err != nil {
			return domain.Operation{}, fmt.Errorf("commit tx: %w", err)
		}
		return op, nil
	}

	loc, err := s.tzResolver.Resolve(ctx, scope)
	if err != nil {
		return domain.Operation{}, fmt.Errorf("resolve owner timezone: %w", err)
	}
	now := s.clock.Now()
	today := timeutil.DateIn(now, loc)
	status := domain.OperationStatusPending
	if op.OperationDate.Before(today) {
		status = domain.OperationStatusOverdue
	}
	op.Status = status
	op.UpdatedAt = now

	if err := op.ValidateStatusForType(); err != nil {
		return domain.Operation{}, err
	}

	updated, err := txOps.Update(ctx, op)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Operation{}, ErrNotFound
		}
		return domain.Operation{}, fmt.Errorf("update operation: %w", err)
	}

	if s.scheduler != nil {
		txScheduler := s.scheduler.WithTx(tx)
		txCategories := s.categories.WithTx(tx)
		cat, err := txCategories.GetByIDAndOwner(ctx, updated.CategoryID, scope)
		if err != nil {
			return domain.Operation{}, fmt.Errorf("get category for reminder: %w", err)
		}
		if err := txScheduler.CancelByOperation(ctx, scope, updated.ID); err != nil {
			return domain.Operation{}, fmt.Errorf("cancel reminders: %w", err)
		}
		if err := txScheduler.CancelOverdueReminderByOperation(ctx, scope, updated.ID); err != nil {
			return domain.Operation{}, fmt.Errorf("cancel overdue reminders: %w", err)
		}

		if updated.Status == domain.OperationStatusOverdue {
			if err := txScheduler.ScheduleOverdueReminder(ctx, ToOperationInfo(updated, cat.Name), now); err != nil {
				return domain.Operation{}, fmt.Errorf("schedule overdue reminder: %w", err)
			}
		} else if updated.ReminderOffsetDays != nil {
			reminderDate := updated.OperationDate.AddDate(0, 0, -(*updated.ReminderOffsetDays))
			if !reminderDate.Before(today) {
				if err := txScheduler.ScheduleForOperation(ctx, ToOperationInfo(updated, cat.Name), reminderDate); err != nil {
					return domain.Operation{}, fmt.Errorf("schedule operation reminder: %w", err)
				}
			}
		}
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &cmd.Actor,
		ActorRole:  actorRoleFromPolicyRole(role),
		Action:     auditdomain.ActionOperationMarkedIncomplete,
		EntityType: auditdomain.EntityOperation,
		EntityID:   &cmd.OperationID,
		Context:    map[string]any{"property_id": domain.PropertyIDPtr(op.PropertyID)},
	}); err != nil {
		return domain.Operation{}, fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Operation{}, fmt.Errorf("commit tx: %w", err)
	}

	return updated, nil
}

// ListOverdueCandidates returns pending operations with an operation_date before asOf.
func (s *OperationService) ListOverdueCandidates(ctx context.Context, actor uuid.UUID, asOf time.Time, limit int) ([]domain.Operation, error) {
	ops, err := s.operations.ListPendingOperationsWithPastDate(ctx, actor, asOf, limit)
	if err != nil {
		return nil, fmt.Errorf("list overdue candidates: %w", err)
	}
	return ops, nil
}

// ListAllOverdueCandidates returns all pending operations with an operation_date before asOf,
// regardless of owner. It is intended for system-scoped workers.
func (s *OperationService) ListAllOverdueCandidates(ctx context.Context, asOf time.Time, limit int) ([]domain.Operation, error) {
	ops, err := s.operations.ListAllPendingOperationsWithPastDate(ctx, asOf, limit)
	if err != nil {
		return nil, fmt.Errorf("list all overdue candidates: %w", err)
	}
	return ops, nil
}

// MarkOverdue transitions a pending operation to overdue idempotently. The returned
// bool is true when the operation was actually changed from pending to overdue.
func (s *OperationService) MarkOverdue(ctx context.Context, actor uuid.UUID, operationID uuid.UUID) (domain.Operation, bool, error) {
	op, changed, err := s.operations.MarkOverdue(ctx, actor, operationID, s.clock.Now())
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Operation{}, false, ErrNotFound
		}
		return domain.Operation{}, false, fmt.Errorf("mark overdue: %w", err)
	}
	return op, changed, nil
}

// ProcessOverdueOperation marks a pending operation as overdue and schedules an
// overdue reminder atomically. The returned bool is true when the operation was
// actually changed.
func (s *OperationService) ProcessOverdueOperation(ctx context.Context, actor, operationID uuid.UUID, asOf time.Time) (bool, error) {
	loc, err := s.tzResolver.Resolve(ctx, actor)
	if err != nil {
		return false, fmt.Errorf("resolve owner timezone: %w", err)
	}
	normalizedAsOf := timeutil.DateIn(asOf, loc)

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txOps := s.operations.WithTx(tx)

	op, changed, err := txOps.MarkOverdue(ctx, actor, operationID, normalizedAsOf)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, ErrNotFound
		}
		return false, fmt.Errorf("mark overdue: %w", err)
	}
	if !changed {
		if err := tx.Commit(ctx); err != nil {
			return false, fmt.Errorf("commit tx: %w", err)
		}
		return false, nil
	}

	if s.scheduler != nil {
		txScheduler := s.scheduler.WithTx(tx)
		txCategories := s.categories.WithTx(tx)
		cat, err := txCategories.GetByIDAndOwner(ctx, op.CategoryID, actor)
		if err != nil {
			return false, fmt.Errorf("get category for reminder: %w", err)
		}
		if err := txScheduler.ScheduleOverdueReminder(ctx, ToOperationInfo(op, cat.Name), normalizedAsOf); err != nil {
			return false, fmt.Errorf("schedule overdue reminder: %w", err)
		}
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionOperationUpdated,
		EntityType: auditdomain.EntityOperation,
		EntityID:   &operationID,
		Context:    map[string]any{"trigger": "scheduler", "fields": []string{"status"}},
	}); err != nil {
		return false, fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit tx: %w", err)
	}

	return true, nil
}

// DeleteOperation soft-deletes an operation after the T3 shared-access write
// gate.
func (s *OperationService) DeleteOperation(ctx context.Context, actor, id uuid.UUID) error {
	op, err := s.operations.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("get operation: %w", err)
	}
	role, err := roleForStandalone(ctx, s.policy, actor, op.PropertyID, op.OwnerID)
	if err != nil {
		return err
	}
	if err := writeRoleGate(role); err != nil {
		return err
	}
	scope := op.OwnerID

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txOps := s.operations.WithTx(tx)

	op, err = txOps.GetByIDAndOwnerForUpdate(ctx, id, scope)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("get operation: %w", err)
	}

	if err := validatePropertyNotArchived(ctx, s.properties.WithTx(tx), scope, op.PropertyID); err != nil {
		return err
	}

	if err := txOps.SoftDeleteOperation(ctx, id, scope); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("delete operation: %w", err)
	}

	if s.scheduler != nil {
		txScheduler := s.scheduler.WithTx(tx)
		if err := txScheduler.CancelByOperation(ctx, scope, id); err != nil {
			return fmt.Errorf("cancel reminders: %w", err)
		}
		if err := txScheduler.CancelOverdueReminderByOperation(ctx, scope, id); err != nil {
			return fmt.Errorf("cancel overdue reminders: %w", err)
		}
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  actorRoleFromPolicyRole(role),
		Action:     auditdomain.ActionOperationDeleted,
		EntityType: auditdomain.EntityOperation,
		EntityID:   &id,
		Context:    map[string]any{"property_id": domain.PropertyIDPtr(op.PropertyID)},
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

// MoveOperation moves a manual operation to another property of the same data
// owner (T3, issue #166). Only same-owner moves are allowed: a target property
// of another owner is rejected with a user-facing 400. Operations generated by
// a recurring operation, linked to a lease, already on the target property, or
// attached to an archived source/target property are rejected the same way.
// Completed operations may be moved.
func (s *OperationService) MoveOperation(ctx context.Context, actor, id uuid.UUID, cmd MoveOperationCommand) (domain.Operation, error) {
	op, err := s.operations.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Operation{}, ErrNotFound
		}
		return domain.Operation{}, fmt.Errorf("get operation: %w", err)
	}
	role, err := roleForStandalone(ctx, s.policy, actor, op.PropertyID, op.OwnerID)
	if err != nil {
		return domain.Operation{}, err
	}
	if err := writeRoleGate(role); err != nil {
		return domain.Operation{}, err
	}
	scope := op.OwnerID

	if err := validateMovableOperation(op, cmd.PropertyID); err != nil {
		return domain.Operation{}, err
	}

	// The actor needs write access to the target property too, and the target
	// must belong to the same data owner: cross-owner moves are rejected.
	targetRole, err := s.policy.RoleForProperty(ctx, actor, cmd.PropertyID)
	if err != nil {
		return domain.Operation{}, fmt.Errorf("resolve role: %w", err)
	}
	if err := writeRoleGate(targetRole); err != nil {
		return domain.Operation{}, err
	}
	targetOwner, err := propertyScope(ctx, s.properties, cmd.PropertyID)
	if err != nil {
		return domain.Operation{}, err
	}
	if targetOwner != scope {
		return domain.Operation{}, newInvalidInputError("cannot move an operation to a property of another owner")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Operation{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txOps := s.operations.WithTx(tx)

	op, err = txOps.GetByIDAndOwnerForUpdate(ctx, id, scope)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Operation{}, ErrNotFound
		}
		return domain.Operation{}, fmt.Errorf("get operation: %w", err)
	}

	// Repeat the guards on the locked row.
	if err := validateMovableOperation(op, cmd.PropertyID); err != nil {
		return domain.Operation{}, err
	}
	if err := validateMovePropertiesNotArchived(ctx, s.properties.WithTx(tx), scope, op.PropertyID, cmd.PropertyID); err != nil {
		return domain.Operation{}, err
	}

	fromPropertyID := op.PropertyID
	now := s.clock.Now()

	updated, err := txOps.MoveToProperty(ctx, id, scope, cmd.PropertyID, now)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Operation{}, ErrNotFound
		}
		return domain.Operation{}, fmt.Errorf("move operation: %w", err)
	}

	// Recreate the reminders so their payloads point at the new property,
	// mirroring the UpdateOperation reschedule flow.
	if s.scheduler != nil {
		loc, err := s.tzResolver.Resolve(ctx, scope)
		if err != nil {
			return domain.Operation{}, fmt.Errorf("resolve owner timezone: %w", err)
		}
		today := timeutil.DateIn(now, loc)

		txScheduler := s.scheduler.WithTx(tx)
		txCategories := s.categories.WithTx(tx)
		cat, err := txCategories.GetByIDAndOwner(ctx, updated.CategoryID, scope)
		if err != nil {
			return domain.Operation{}, fmt.Errorf("get category for reminder: %w", err)
		}
		if err := txScheduler.CancelByOperation(ctx, scope, updated.ID); err != nil {
			return domain.Operation{}, fmt.Errorf("cancel reminders: %w", err)
		}
		if err := txScheduler.CancelOverdueReminderByOperation(ctx, scope, updated.ID); err != nil {
			return domain.Operation{}, fmt.Errorf("cancel overdue reminders: %w", err)
		}
		if updated.ReminderOffsetDays != nil {
			reminderDate := updated.OperationDate.AddDate(0, 0, -(*updated.ReminderOffsetDays))
			if !reminderDate.Before(today) {
				if err := txScheduler.ScheduleForOperation(ctx, ToOperationInfo(updated, cat.Name), reminderDate); err != nil {
					return domain.Operation{}, fmt.Errorf("schedule operation reminder: %w", err)
				}
			}
		}
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  actorRoleFromPolicyRole(role),
		Action:     auditdomain.ActionOperationMoved,
		EntityType: auditdomain.EntityOperation,
		EntityID:   &id,
		Context: map[string]any{
			"from_property_id": domain.PropertyIDPtr(fromPropertyID),
			"to_property_id":   cmd.PropertyID,
		},
	}); err != nil {
		return domain.Operation{}, fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Operation{}, fmt.Errorf("commit tx: %w", err)
	}

	return updated, nil
}

// validateMovableOperation rejects operations that cannot be moved between
// properties: generated by a recurring operation, linked to a lease, or
// already attached to the target property.
func validateMovableOperation(op domain.Operation, targetPropertyID uuid.UUID) error {
	if op.RecurringOperationID != uuid.Nil {
		return newInvalidInputError("cannot move an operation generated by a recurring operation")
	}
	if op.LeaseID != uuid.Nil {
		return newInvalidInputError("cannot move an operation linked to a lease")
	}
	if targetPropertyID == op.PropertyID {
		return newInvalidInputError("operation already belongs to this property")
	}
	return nil
}

// validateMovePropertiesNotArchived rejects a move when the source or the
// target property is archived. A property-less operation (detached source)
// skips the source check.
func validateMovePropertiesNotArchived(ctx context.Context, properties PropertyRepository, scope, sourcePropertyID, targetPropertyID uuid.UUID) error {
	if sourcePropertyID != uuid.Nil {
		status, err := properties.GetStatusByOwner(ctx, sourcePropertyID, scope)
		if err != nil {
			return fmt.Errorf("check source property: %w", err)
		}
		if status == propertyStatusArchived {
			return newInvalidInputError("cannot move an operation of an archived property")
		}
	}
	status, err := properties.GetStatusByOwner(ctx, targetPropertyID, scope)
	if err != nil {
		return fmt.Errorf("check target property: %w", err)
	}
	if status == propertyStatusArchived {
		return newInvalidInputError("cannot move an operation to an archived property")
	}
	return nil
}

func (s *OperationService) validateAmountAndDate(amount int64, operationDate time.Time) error {
	if amount < 0 {
		return fmt.Errorf("%w: amount must be non-negative", ErrInvalidInput)
	}
	if operationDate.IsZero() {
		return fmt.Errorf("%w: operation_date is required", ErrInvalidInput)
	}
	return nil
}

// operationStatusForDate returns the initial status for a newly created operation:
// past-dated operations start as unconfirmed and must be completed explicitly,
// operations dated today or later start as pending.
func operationStatusForDate(operationDate, today time.Time) domain.OperationStatus {
	if timeutil.BeforeDay(operationDate, today) {
		return domain.OperationStatusUnconfirmed
	}
	return domain.OperationStatusPending
}

func validateReminderOffsetDays(v *int) error {
	if v == nil {
		return nil
	}
	switch *v {
	case 0, 1, 3, 7:
		return nil
	default:
		return fmt.Errorf("%w: reminder_offset_days must be 0, 1, 3, or 7", ErrInvalidInput)
	}
}

func normalizeReminderOffsetDays(v *int) *int {
	if v != nil && *v == 0 {
		return nil
	}
	return v
}

func reminderOffsetDaysEqual(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func (s *OperationService) resolveLeaseID(ctx context.Context, scope, propertyID uuid.UUID, leaseID *uuid.UUID) (uuid.UUID, error) {
	if leaseID == nil {
		return uuid.UUID{}, nil
	}

	lease, err := s.leases.GetByIDAndOwner(ctx, *leaseID, scope)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return uuid.UUID{}, fmt.Errorf("%w: lease not found", ErrInvalidInput)
		}
		return uuid.UUID{}, fmt.Errorf("get lease: %w", err)
	}

	if lease.PropertyID != propertyID {
		return uuid.UUID{}, fmt.Errorf("%w: lease does not belong to the property", ErrInvalidInput)
	}

	return lease.ID, nil
}

// updatedOperationFields lists the names of the fields a command changes. Only
// field names are audited, never their values.
func updatedOperationFields(cmd UpdateOperationCommand) []string {
	fields := make([]string, 0, 8)
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
	if cmd.OperationDate != nil {
		fields = append(fields, "operation_date")
	}
	if cmd.Comment != nil {
		fields = append(fields, "comment")
	}
	if cmd.LeaseID != nil {
		fields = append(fields, "lease_id")
	}
	if cmd.ReminderOffsetDays != nil {
		fields = append(fields, "reminder_offset_days")
	}
	return fields
}
