package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	AmountKopecks int64
	OperationDate time.Time
	Comment       *string
	LeaseID       *uuid.UUID
}

// UpdateOperationCommand carries the optional updates for an operation.
type UpdateOperationCommand struct {
	Type          *string
	Category      *string
	AmountKopecks *int64
	OperationDate *time.Time
	Comment       *string
	LeaseID       *uuid.UUID
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
		AmountKopecks: cmd.AmountKopecks,
		OperationDate: cmd.OperationDate,
		Comment:       comment,
		IsException:   true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	created, err := s.operations.Create(ctx, op)
	if err != nil {
		return domain.Operation{}, fmt.Errorf("create operation: %w", err)
	}
	return created, nil
}

// ListOperationsByProperty returns operations for the given owner and property.
func (s *OperationService) ListOperationsByProperty(ctx context.Context, ownerID, propertyID uuid.UUID) ([]domain.Operation, error) {
	if err := validateProperty(ctx, s.properties, ownerID, propertyID); err != nil {
		return nil, err
	}

	ops, err := s.operations.ListByProperty(ctx, ownerID, propertyID)
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
	op.UpdatedAt = s.clock.Now()

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
