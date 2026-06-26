package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	notificationsdomain "github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/timeutil"
)

// CreateOperationCommand carries the data needed to create a manual operation.
type CreateOperationCommand struct {
	PropertyID    uuid.UUID
	Type          string
	Category      string
	Name          string
	AmountKopecks int64
	OperationDate time.Time
	Comment       *string
	LeaseID       *uuid.UUID
}

// UpdateOperationCommand carries the optional updates for an operation.
type UpdateOperationCommand struct {
	Type          *string
	Category      *string
	Name          *string
	AmountKopecks *int64
	OperationDate *time.Time
	Comment       *string
	LeaseID       *uuid.UUID
}

// CompleteOperationCommand carries the data needed to mark an operation as completed.
type CompleteOperationCommand struct {
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
	scheduler    ReminderScheduler
	db           txBeginner
	clock        clock.Clock
	logger       *slog.Logger
}

// NewOperationService creates a new operation service.
func NewOperationService(
	operations OperationRepository,
	properties PropertyRepository,
	leases LeaseRepository,
	recurringOps RecurringOperationRepository,
	scheduler ReminderScheduler,
	db txBeginner,
	clock clock.Clock,
	logger *slog.Logger,
) *OperationService {
	if db == nil {
		panic("db beginner is required")
	}
	if clock == nil {
		panic("clock is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &OperationService{
		operations:   operations,
		properties:   properties,
		leases:       leases,
		recurringOps: recurringOps,
		scheduler:    scheduler,
		db:           db,
		clock:        clock,
		logger:       logger,
	}
}

// CreateOperation creates a manual operation for the given owner and property.
func (s *OperationService) CreateOperation(ctx context.Context, ownerID uuid.UUID, cmd CreateOperationCommand) (domain.Operation, error) {
	if err := validateProperty(ctx, s.properties, ownerID, cmd.PropertyID); err != nil {
		return domain.Operation{}, err
	}

	opType, category, err := parseTypeAndCategory(cmd.Type, cmd.Category)
	if err != nil {
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

	leaseID, err := s.resolveLeaseID(ctx, ownerID, cmd.PropertyID, cmd.LeaseID)
	if err != nil {
		return domain.Operation{}, err
	}

	id, err := uuid.NewRandom()
	if err != nil {
		return domain.Operation{}, fmt.Errorf("generate operation id: %w", err)
	}

	now := s.clock.Now()
	var comment string
	if cmd.Comment != nil {
		comment = *cmd.Comment
	}

	op := domain.Operation{
		ID:            id,
		OwnerID:       ownerID,
		PropertyID:    cmd.PropertyID,
		LeaseID:       leaseID,
		Type:          opType,
		Category:      category,
		Status:        domain.OperationStatusPending,
		Name:          name,
		AmountKopecks: cmd.AmountKopecks,
		OperationDate: cmd.OperationDate,
		Comment:       comment,
		IsException:   true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := op.ValidateStatusForType(); err != nil {
		return domain.Operation{}, err
	}

	created, err := s.operations.Create(ctx, op)
	if err != nil {
		return domain.Operation{}, fmt.Errorf("create operation: %w", err)
	}
	return created, nil
}

func (s *OperationService) GetPropertyOperationsSummary(ctx context.Context, ownerID, propertyID uuid.UUID) (OperationsSummary, error) {
	if err := validateProperty(ctx, s.properties, ownerID, propertyID); err != nil {
		return OperationsSummary{}, err
	}
	return s.operations.GetPropertyOperationsSummary(ctx, ownerID, propertyID, timeutil.Date(s.clock.Now()))
}

// ListOperationsByProperty returns operations for the given owner and property,
// optionally filtered by status.
func (s *OperationService) ListOperationsByProperty(ctx context.Context, ownerID, propertyID uuid.UUID, statuses []domain.OperationStatus) ([]domain.Operation, error) {
	if err := validateProperty(ctx, s.properties, ownerID, propertyID); err != nil {
		return nil, err
	}

	var ops []domain.Operation
	var err error
	if len(statuses) == 0 {
		ops, err = s.operations.ListByProperty(ctx, ownerID, propertyID)
	} else {
		ops, err = s.operations.ListByPropertyWithStatuses(ctx, ownerID, propertyID, statuses)
	}
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
	txRecurring := s.recurringOps.WithTx(tx)

	op, err := txOps.GetByIDAndOwnerForUpdate(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Operation{}, ErrNotFound
		}
		return domain.Operation{}, fmt.Errorf("get operation: %w", err)
	}

	typeStr := string(op.Type)
	categoryStr := string(op.Category)
	if cmd.Type != nil {
		typeStr = *cmd.Type
	}
	if cmd.Category != nil {
		categoryStr = *cmd.Category
	}
	opType, category, err := parseTypeAndCategory(typeStr, categoryStr)
	if err != nil {
		return domain.Operation{}, err
	}
	op.Type = opType
	op.Category = category

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
	originalOperationDate := op.OperationDate
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

	op.IsException = true
	now := s.clock.Now()
	if cmd.OperationDate != nil && op.Status == domain.OperationStatusOverdue && !op.OperationDate.Before(timeutil.Date(now)) {
		op.Status = domain.OperationStatusPending
	}
	op.UpdatedAt = now

	updated, err := txOps.Update(ctx, op)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Operation{}, ErrNotFound
		}
		return domain.Operation{}, fmt.Errorf("update operation: %w", err)
	}

	if s.scheduler != nil && cmd.OperationDate != nil && !originalOperationDate.Equal(*cmd.OperationDate) {
		txScheduler := s.scheduler.WithTx(tx)

		now := s.clock.Now()
		today := timeutil.Date(now)
		var reminderDate time.Time
		var skipSchedule bool
		if updated.RecurringOperationID != uuid.Nil {
			rec, err := txRecurring.GetByIDAndOwner(ctx, updated.RecurringOperationID, ownerID)
			if err != nil {
				return domain.Operation{}, fmt.Errorf("get recurring operation: %w", err)
			}
			if rec.ReminderOffsetDays == nil {
				skipSchedule = true
			} else {
				reminderDate = updated.OperationDate.AddDate(0, 0, -(*rec.ReminderOffsetDays))
				if reminderDate.Before(today) {
					skipSchedule = true
				}
			}
		} else {
			reminderDate = updated.OperationDate
			if reminderDate.Before(today) {
				skipSchedule = true
			}
			if !skipSchedule {
				hasReminder, err := txScheduler.HasReminderForOperationEvent(ctx, ownerID, updated.ID, notificationsdomain.EventOperationDue)
				if err != nil {
					return domain.Operation{}, fmt.Errorf("check existing reminder: %w", err)
				}
				if !hasReminder {
					skipSchedule = true
				}
			}
		}

		if err := txScheduler.CancelByOperation(ctx, ownerID, updated.ID); err != nil {
			return domain.Operation{}, fmt.Errorf("cancel reminders: %w", err)
		}
		if err := txScheduler.CancelOverdueReminderByOperation(ctx, ownerID, updated.ID); err != nil {
			return domain.Operation{}, fmt.Errorf("cancel overdue reminders: %w", err)
		}

		if !skipSchedule {
			if err := txScheduler.ScheduleForOperation(ctx, ToOperationInfo(updated), reminderDate); err != nil {
				return domain.Operation{}, fmt.Errorf("schedule operation reminder: %w", err)
			}
		}
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
		if err := txScheduler.ScheduleOverdueReminder(ctx, ToOperationInfo(op), asOf); err != nil {
			return false, fmt.Errorf("schedule overdue reminder: %w", err)
		}
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
