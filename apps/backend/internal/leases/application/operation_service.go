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
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/timeutil"
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
	OwnerID     uuid.UUID
	OperationID uuid.UUID
}

// MarkOperationIncompleteCommand carries the data needed to mark a completed operation as planned again.
type MarkOperationIncompleteCommand struct {
	OwnerID     uuid.UUID
	OperationID uuid.UUID
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
	logger       *slog.Logger
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
		logger:       logger,
	}
}

// CreateOperation creates a manual operation for the given owner and property.
func (s *OperationService) CreateOperation(ctx context.Context, ownerID uuid.UUID, cmd CreateOperationCommand) (domain.Operation, error) {
	if err := validatePropertyNotArchived(ctx, s.properties, ownerID, cmd.PropertyID); err != nil {
		return domain.Operation{}, err
	}

	opType, categoryID, err := parseTypeAndCategory(cmd.Type, cmd.CategoryID)
	if err != nil {
		return domain.Operation{}, err
	}
	if err := validateCategory(ctx, s.categories, ownerID, opType, categoryID); err != nil {
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

	leaseID, err := s.resolveLeaseID(ctx, ownerID, cmd.PropertyID, cmd.LeaseID)
	if err != nil {
		return domain.Operation{}, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return domain.Operation{}, fmt.Errorf("generate operation id: %w", err)
	}

	now := s.clock.Now()
	var comment string
	if cmd.Comment != nil {
		comment = *cmd.Comment
	}

	op := domain.Operation{
		ID:                 id,
		OwnerID:            ownerID,
		PropertyID:         cmd.PropertyID,
		LeaseID:            leaseID,
		Type:               opType,
		CategoryID:         categoryID,
		Status:             operationStatusForDate(opType, cmd.OperationDate, now),
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
		cat, err := txCategories.GetByIDAndOwner(ctx, created.CategoryID, ownerID)
		if err != nil {
			return domain.Operation{}, fmt.Errorf("get category for reminder: %w", err)
		}
		reminderDate := created.OperationDate.AddDate(0, 0, -(*created.ReminderOffsetDays))
		if !reminderDate.Before(timeutil.Date(s.clock.Now())) {
			if err := txScheduler.ScheduleForOperation(ctx, ToOperationInfo(created, cat.Name), reminderDate); err != nil {
				return domain.Operation{}, fmt.Errorf("schedule operation reminder: %w", err)
			}
		}
	}

	opCtx := map[string]any{
		"property_id":    created.PropertyID,
		"type":           string(created.Type),
		"amount_kopecks": created.AmountKopecks,
		"operation_date": created.OperationDate.Format(time.DateOnly),
	}
	if created.LeaseID != uuid.Nil {
		opCtx["lease_id"] = created.LeaseID
	}
	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &ownerID,
		ActorRole:  auditdomain.ActorRoleOwner,
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

func (s *OperationService) GetPropertyOperationsSummary(ctx context.Context, ownerID, propertyID uuid.UUID) (OperationsSummary, error) {
	if err := validateProperty(ctx, s.properties, ownerID, propertyID); err != nil {
		return OperationsSummary{}, err
	}
	return s.operations.GetPropertyOperationsSummary(ctx, ownerID, propertyID, timeutil.Date(s.clock.Now()))
}

type FinanceReport struct {
	Totals     FinanceReportTotals
	ByProperty []FinanceReportPropertyRow
	ByCategory []FinanceReportCategoryRow
	ByMonth    []FinanceReportMonthRow
}

func (s *OperationService) GetFinanceReport(ctx context.Context, ownerID uuid.UUID, from, to *time.Time) (FinanceReport, error) {
	totals, err := s.operations.GetFinanceReportTotals(ctx, ownerID, from, to)
	if err != nil {
		return FinanceReport{}, fmt.Errorf("report totals: %w", err)
	}
	byProperty, err := s.operations.GetFinanceReportByProperty(ctx, ownerID, from, to)
	if err != nil {
		return FinanceReport{}, fmt.Errorf("report by property: %w", err)
	}
	byCategory, err := s.operations.GetFinanceReportByCategory(ctx, ownerID, from, to)
	if err != nil {
		return FinanceReport{}, fmt.Errorf("report by category: %w", err)
	}
	byMonth, err := s.operations.GetFinanceReportByMonth(ctx, ownerID, from, to)
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

// ListOperations returns all operations for the owner filtered by the provided criteria.
func (s *OperationService) ListOperations(ctx context.Context, ownerID uuid.UUID, filter OperationFilter) ([]domain.Operation, error) {
	ops, err := s.operations.ListByOwner(ctx, ownerID, filter)
	if err != nil {
		return nil, fmt.Errorf("list operations: %w", err)
	}
	return ops, nil
}

// ListOperationsByProperty returns operations for the given owner and property,
// filtered by the provided criteria.
func (s *OperationService) ListOperationsByProperty(ctx context.Context, ownerID, propertyID uuid.UUID, filter OperationFilter) ([]domain.Operation, error) {
	if err := validateProperty(ctx, s.properties, ownerID, propertyID); err != nil {
		return nil, err
	}

	filter.PropertyID = propertyID
	ops, err := s.operations.ListByOwner(ctx, ownerID, filter)
	if err != nil {
		return nil, fmt.Errorf("list operations: %w", err)
	}
	return ops, nil
}

// GetOperation returns a single operation owned by the given owner.
func (s *OperationService) GetOperation(ctx context.Context, ownerID, id uuid.UUID) (domain.Operation, error) {
	op, err := s.operations.GetByIDAndOwner(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Operation{}, ErrNotFound
		}
		return domain.Operation{}, fmt.Errorf("get operation: %w", err)
	}
	return op, nil
}

// UpdateOperation updates an operation owned by the given owner.
func (s *OperationService) UpdateOperation(ctx context.Context, ownerID, id uuid.UUID, cmd UpdateOperationCommand) (domain.Operation, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Operation{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txOps := s.operations.WithTx(tx)
	txCategories := s.categories.WithTx(tx)

	op, err := txOps.GetByIDAndOwnerForUpdate(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Operation{}, ErrNotFound
		}
		return domain.Operation{}, fmt.Errorf("get operation: %w", err)
	}

	if err := validatePropertyNotArchived(ctx, s.properties.WithTx(tx), ownerID, op.PropertyID); err != nil {
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
		if err := validateCategory(ctx, txCategories, ownerID, opType, categoryID); err != nil {
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
		leaseID, err := s.resolveLeaseID(ctx, ownerID, op.PropertyID, cmd.LeaseID)
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
	now := s.clock.Now()
	if cmd.OperationDate != nil {
		today := timeutil.Date(now)
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
		cat, err := txCategories.GetByIDAndOwner(ctx, updated.CategoryID, ownerID)
		if err != nil {
			return domain.Operation{}, fmt.Errorf("get category for reminder: %w", err)
		}

		if err := txScheduler.CancelByOperation(ctx, ownerID, updated.ID); err != nil {
			return domain.Operation{}, fmt.Errorf("cancel reminders: %w", err)
		}
		if err := txScheduler.CancelOverdueReminderByOperation(ctx, ownerID, updated.ID); err != nil {
			return domain.Operation{}, fmt.Errorf("cancel overdue reminders: %w", err)
		}

		if updated.ReminderOffsetDays != nil {
			reminderDate := updated.OperationDate.AddDate(0, 0, -(*updated.ReminderOffsetDays))
			if !reminderDate.Before(timeutil.Date(now)) {
				if err := txScheduler.ScheduleForOperation(ctx, ToOperationInfo(updated, cat.Name), reminderDate); err != nil {
					return domain.Operation{}, fmt.Errorf("schedule operation reminder: %w", err)
				}
			}
		}
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &ownerID,
		ActorRole:  auditdomain.ActorRoleOwner,
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

// CompleteOperation marks a pending or overdue operation as completed.
// Expenses become paid, income becomes received, and future reminders are cancelled.
func (s *OperationService) CompleteOperation(ctx context.Context, cmd CompleteOperationCommand) (domain.Operation, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Operation{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txOps := s.operations.WithTx(tx)

	op, err := txOps.GetByIDAndOwnerForUpdate(ctx, cmd.OperationID, cmd.OwnerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Operation{}, ErrNotFound
		}
		return domain.Operation{}, fmt.Errorf("get operation: %w", err)
	}

	if err := validatePropertyNotArchived(ctx, s.properties.WithTx(tx), cmd.OwnerID, op.PropertyID); err != nil {
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
		if err := txScheduler.CancelByOperation(ctx, cmd.OwnerID, updated.ID); err != nil {
			return domain.Operation{}, fmt.Errorf("cancel reminders: %w", err)
		}
		if err := txScheduler.CancelOverdueReminderByOperation(ctx, cmd.OwnerID, updated.ID); err != nil {
			return domain.Operation{}, fmt.Errorf("cancel overdue reminders: %w", err)
		}
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &cmd.OwnerID,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionOperationCompleted,
		EntityType: auditdomain.EntityOperation,
		EntityID:   &cmd.OperationID,
		Context:    map[string]any{"property_id": op.PropertyID},
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
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Operation{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txOps := s.operations.WithTx(tx)

	op, err := txOps.GetByIDAndOwnerForUpdate(ctx, cmd.OperationID, cmd.OwnerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Operation{}, ErrNotFound
		}
		return domain.Operation{}, fmt.Errorf("get operation: %w", err)
	}

	if err := validatePropertyNotArchived(ctx, s.properties.WithTx(tx), cmd.OwnerID, op.PropertyID); err != nil {
		return domain.Operation{}, err
	}

	if !op.Status.IsCompleted() {
		if err := tx.Commit(ctx); err != nil {
			return domain.Operation{}, fmt.Errorf("commit tx: %w", err)
		}
		return op, nil
	}

	now := s.clock.Now()
	status := domain.OperationStatusPending
	if op.OperationDate.Before(timeutil.Date(now)) {
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
		cat, err := txCategories.GetByIDAndOwner(ctx, updated.CategoryID, cmd.OwnerID)
		if err != nil {
			return domain.Operation{}, fmt.Errorf("get category for reminder: %w", err)
		}
		if err := txScheduler.CancelByOperation(ctx, cmd.OwnerID, updated.ID); err != nil {
			return domain.Operation{}, fmt.Errorf("cancel reminders: %w", err)
		}
		if err := txScheduler.CancelOverdueReminderByOperation(ctx, cmd.OwnerID, updated.ID); err != nil {
			return domain.Operation{}, fmt.Errorf("cancel overdue reminders: %w", err)
		}

		if updated.Status == domain.OperationStatusOverdue {
			if err := txScheduler.ScheduleOverdueReminder(ctx, ToOperationInfo(updated, cat.Name), now); err != nil {
				return domain.Operation{}, fmt.Errorf("schedule overdue reminder: %w", err)
			}
		} else if updated.ReminderOffsetDays != nil {
			reminderDate := updated.OperationDate.AddDate(0, 0, -(*updated.ReminderOffsetDays))
			if !reminderDate.Before(timeutil.Date(op.UpdatedAt)) {
				if err := txScheduler.ScheduleForOperation(ctx, ToOperationInfo(updated, cat.Name), reminderDate); err != nil {
					return domain.Operation{}, fmt.Errorf("schedule operation reminder: %w", err)
				}
			}
		}
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &cmd.OwnerID,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionOperationMarkedIncomplete,
		EntityType: auditdomain.EntityOperation,
		EntityID:   &cmd.OperationID,
		Context:    map[string]any{"property_id": op.PropertyID},
	}); err != nil {
		return domain.Operation{}, fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Operation{}, fmt.Errorf("commit tx: %w", err)
	}

	return updated, nil
}

// ListOverdueCandidates returns pending operations with an operation_date before asOf.
func (s *OperationService) ListOverdueCandidates(ctx context.Context, ownerID uuid.UUID, asOf time.Time, limit int) ([]domain.Operation, error) {
	ops, err := s.operations.ListPendingOperationsWithPastDate(ctx, ownerID, asOf, limit)
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
func (s *OperationService) MarkOverdue(ctx context.Context, ownerID uuid.UUID, operationID uuid.UUID) (domain.Operation, bool, error) {
	op, changed, err := s.operations.MarkOverdue(ctx, ownerID, operationID, s.clock.Now())
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
func (s *OperationService) ProcessOverdueOperation(ctx context.Context, ownerID, operationID uuid.UUID, asOf time.Time) (bool, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txOps := s.operations.WithTx(tx)

	op, changed, err := txOps.MarkOverdue(ctx, ownerID, operationID, asOf)
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
		cat, err := txCategories.GetByIDAndOwner(ctx, op.CategoryID, ownerID)
		if err != nil {
			return false, fmt.Errorf("get category for reminder: %w", err)
		}
		if err := txScheduler.ScheduleOverdueReminder(ctx, ToOperationInfo(op, cat.Name), asOf); err != nil {
			return false, fmt.Errorf("schedule overdue reminder: %w", err)
		}
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &ownerID,
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

// DeleteOperation soft-deletes an operation owned by the given owner.
func (s *OperationService) DeleteOperation(ctx context.Context, ownerID, id uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txOps := s.operations.WithTx(tx)

	op, err := txOps.GetByIDAndOwnerForUpdate(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("get operation: %w", err)
	}

	if err := validatePropertyNotArchived(ctx, s.properties.WithTx(tx), ownerID, op.PropertyID); err != nil {
		return err
	}

	if err := txOps.SoftDeleteOperation(ctx, id, ownerID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("delete operation: %w", err)
	}

	if s.scheduler != nil {
		txScheduler := s.scheduler.WithTx(tx)
		if err := txScheduler.CancelByOperation(ctx, ownerID, id); err != nil {
			return fmt.Errorf("cancel reminders: %w", err)
		}
		if err := txScheduler.CancelOverdueReminderByOperation(ctx, ownerID, id); err != nil {
			return fmt.Errorf("cancel overdue reminders: %w", err)
		}
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &ownerID,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionOperationDeleted,
		EntityType: auditdomain.EntityOperation,
		EntityID:   &id,
		Context:    map[string]any{"property_id": op.PropertyID},
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
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
// past-dated operations are immediately completed (paid for expense, received
// for income), operations dated today or later start as pending.
func operationStatusForDate(opType domain.OperationType, operationDate, today time.Time) domain.OperationStatus {
	if timeutil.Date(operationDate).Before(timeutil.Date(today)) {
		if opType == domain.OperationTypeExpense {
			return domain.OperationStatusPaid
		}
		return domain.OperationStatusReceived
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

func (s *OperationService) resolveLeaseID(ctx context.Context, ownerID, propertyID uuid.UUID, leaseID *uuid.UUID) (uuid.UUID, error) {
	if leaseID == nil {
		return uuid.UUID{}, nil
	}

	lease, err := s.leases.GetByIDAndOwner(ctx, *leaseID, ownerID)
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
