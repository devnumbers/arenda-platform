package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type txBeginner interface {
	Begin(ctx context.Context) (transaction.Tx, error)
}

type CreatePropertyCommand struct {
	Name        string
	Type        string
	Address     string
	Description string
}

type UpdatePropertyCommand struct {
	Name        *string
	Type        *string
	Address     *string
	Description *string
	Status      *string
}

type PropertyService struct {
	repo              PropertyRepository
	occupancyProvider OccupancyProvider
	limiter           SubscriptionLimiter
	operationArchiver OperationArchiver
	recurringOps      RecurringOperationStatusUpdater
	scheduler         RecurringOperationScheduler
	db                txBeginner
	logger            *slog.Logger
}

func NewPropertyService(
	repo PropertyRepository,
	occupancyProvider OccupancyProvider,
	limiter SubscriptionLimiter,
	operationArchiver OperationArchiver,
	recurringOps RecurringOperationStatusUpdater,
	scheduler RecurringOperationScheduler,
	db txBeginner,
	logger *slog.Logger,
) *PropertyService {
	if logger == nil {
		logger = slog.Default()
	}
	return &PropertyService{
		repo:              repo,
		occupancyProvider: occupancyProvider,
		limiter:           limiter,
		operationArchiver: operationArchiver,
		recurringOps:      recurringOps,
		scheduler:         scheduler,
		db:                db,
		logger:            logger,
	}
}

func (s *PropertyService) CreateProperty(ctx context.Context, ownerID uuid.UUID, cmd CreatePropertyCommand) (domain.Property, error) {
	propertyType, err := domain.ParsePropertyType(cmd.Type)
	if err != nil {
		return domain.Property{}, fmt.Errorf("%w: invalid property type: %w", ErrInvalidInput, err)
	}

	property, err := domain.NewProperty(ownerID, cmd.Name, cmd.Address, cmd.Description, propertyType)
	if err != nil {
		return domain.Property{}, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}

	now := time.Now()
	property.CreatedAt = now
	property.UpdatedAt = now

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Property{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRepo := s.repo.WithTx(tx)

	limit, err := s.limiter.ActivePropertyLimit(ctx, ownerID)
	if err != nil {
		_ = tx.Rollback(ctx)
		return domain.Property{}, fmt.Errorf("get active property limit: %w", err)
	}

	count, err := txRepo.CountActiveByOwner(ctx, ownerID)
	if err != nil {
		return domain.Property{}, fmt.Errorf("count active properties: %w", err)
	}
	if count >= limit {
		_ = tx.Rollback(ctx)
		return domain.Property{}, ErrLimitExceeded
	}

	created, err := txRepo.Create(ctx, ownerID, property)
	if err != nil {
		return domain.Property{}, fmt.Errorf("create property: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Property{}, fmt.Errorf("commit tx: %w", err)
	}

	created.Occupancy = domain.OccupancyFree
	return created, nil
}

func (s *PropertyService) ListProperties(ctx context.Context, ownerID uuid.UUID) ([]domain.Property, error) {
	properties, err := s.repo.ListActiveByOwner(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list properties: %w", err)
	}

	ids := make([]uuid.UUID, len(properties))
	for i, p := range properties {
		ids[i] = p.ID
	}

	occupied, err := s.occupancyProvider.OccupiedPropertyIDs(ctx, ownerID, ids)
	if err != nil {
		return nil, fmt.Errorf("check occupancy: %w", err)
	}

	for i := range properties {
		if occupied[properties[i].ID] {
			properties[i].Occupancy = domain.OccupancyOccupied
		} else {
			properties[i].Occupancy = domain.OccupancyFree
		}
	}

	return properties, nil
}

func (s *PropertyService) GetProperty(ctx context.Context, ownerID, id uuid.UUID) (domain.Property, error) {
	property, err := s.repo.GetByIDAndOwner(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Property{}, ErrNotFound
		}
		return domain.Property{}, fmt.Errorf("get property: %w", err)
	}

	occupied, err := s.occupancyProvider.OccupiedPropertyIDs(ctx, ownerID, []uuid.UUID{property.ID})
	if err != nil {
		return domain.Property{}, fmt.Errorf("check occupancy: %w", err)
	}
	if occupied[property.ID] {
		property.Occupancy = domain.OccupancyOccupied
	} else {
		property.Occupancy = domain.OccupancyFree
	}

	return property, nil
}

func (s *PropertyService) UpdateProperty(ctx context.Context, ownerID, id uuid.UUID, cmd UpdatePropertyCommand) (domain.Property, error) {
	property, err := s.repo.GetByIDAndOwner(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Property{}, ErrNotFound
		}
		return domain.Property{}, fmt.Errorf("get property: %w", err)
	}

	if property.Status == domain.PropertyStatusArchived {
		return domain.Property{}, ErrArchivedProperty
	}

	if cmd.Name != nil {
		property.Name = *cmd.Name
	}
	if cmd.Address != nil {
		property.Address = *cmd.Address
	}
	if cmd.Description != nil {
		property.Description = *cmd.Description
	}
	if cmd.Type != nil {
		propertyType, err := domain.ParsePropertyType(*cmd.Type)
		if err != nil {
			return domain.Property{}, fmt.Errorf("%w: invalid property type: %w", ErrInvalidInput, err)
		}
		property.Type = propertyType
	}
	if cmd.Status != nil {
		status, err := domain.ParsePropertyStatus(*cmd.Status)
		if err != nil {
			return domain.Property{}, fmt.Errorf("%w: invalid property status: %w", ErrInvalidInput, err)
		}
		if !isUpdatableStatusTransition(property.Status, status) {
			return domain.Property{}, &InvalidStatusTransitionError{From: property.Status, To: status}
		}
		if status == domain.PropertyStatusMaintenance && property.Status == domain.PropertyStatusActive {
			occupied, err := s.occupancyProvider.OccupiedPropertyIDs(ctx, ownerID, []uuid.UUID{property.ID})
			if err != nil {
				return domain.Property{}, fmt.Errorf("check occupancy: %w", err)
			}
			if occupied[property.ID] {
				return domain.Property{}, ErrPropertyHasOpenLease
			}
		}
		property.Status = status
	}

	if err := property.Validate(); err != nil {
		return domain.Property{}, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}

	property.UpdatedAt = time.Now()

	updated, err := s.repo.Update(ctx, ownerID, property)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Property{}, ErrNotFound
		}
		return domain.Property{}, fmt.Errorf("update property: %w", err)
	}

	return updated, nil
}

func (s *PropertyService) ArchiveProperty(ctx context.Context, ownerID, id uuid.UUID) (domain.Property, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Property{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRepo := s.repo.WithTx(tx)

	property, err := txRepo.GetByIDAndOwner(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Property{}, ErrNotFound
		}
		return domain.Property{}, fmt.Errorf("get property: %w", err)
	}

	if property.Status == domain.PropertyStatusArchived {
		_ = tx.Rollback(ctx)
		return domain.Property{}, ErrAlreadyArchived
	}

	occupied, err := s.occupancyProvider.OccupiedPropertyIDs(ctx, ownerID, []uuid.UUID{property.ID})
	if err != nil {
		_ = tx.Rollback(ctx)
		return domain.Property{}, fmt.Errorf("check occupancy: %w", err)
	}
	if occupied[property.ID] {
		_ = tx.Rollback(ctx)
		return domain.Property{}, ErrPropertyHasOpenLease
	}

	txOperationArchiver := s.operationArchiver.WithTx(tx)
	txRecurringOps := s.recurringOps.WithTx(tx)

	if err := txOperationArchiver.DeleteFutureUneditedOperationsByProperty(ctx, id, date(time.Now())); err != nil {
		_ = tx.Rollback(ctx)
		return domain.Property{}, fmt.Errorf("delete future operations: %w", err)
	}

	recs, err := txRecurringOps.ListByProperty(ctx, ownerID, id)
	if err != nil {
		_ = tx.Rollback(ctx)
		return domain.Property{}, fmt.Errorf("list recurring operations: %w", err)
	}

	for _, rec := range recs {
		if err := txRecurringOps.UpdateStatus(ctx, rec.ID, "paused"); err != nil {
			_ = tx.Rollback(ctx)
			return domain.Property{}, fmt.Errorf("pause recurring operation: %w", err)
		}
	}

	if err := txRepo.Archive(ctx, id, ownerID); err != nil {
		if errors.Is(err, ErrNotFound) {
			_ = tx.Rollback(ctx)
			return domain.Property{}, ErrNotFound
		}
		_ = tx.Rollback(ctx)
		return domain.Property{}, fmt.Errorf("archive property: %w", err)
	}

	archived, err := txRepo.GetByIDAndOwner(ctx, id, ownerID)
	if err != nil {
		_ = tx.Rollback(ctx)
		return domain.Property{}, fmt.Errorf("reload archived property: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Property{}, fmt.Errorf("commit tx: %w", err)
	}

	return archived, nil
}

func (s *PropertyService) UnarchiveProperty(ctx context.Context, ownerID, id uuid.UUID) (domain.Property, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Property{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRepo := s.repo.WithTx(tx)

	property, err := txRepo.GetByIDAndOwner(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Property{}, ErrNotFound
		}
		return domain.Property{}, fmt.Errorf("get property: %w", err)
	}

	if property.Status != domain.PropertyStatusArchived {
		_ = tx.Rollback(ctx)
		return domain.Property{}, ErrNotArchived
	}

	limit, err := s.limiter.ActivePropertyLimit(ctx, ownerID)
	if err != nil {
		_ = tx.Rollback(ctx)
		return domain.Property{}, fmt.Errorf("get active property limit: %w", err)
	}

	count, err := txRepo.CountActiveByOwner(ctx, ownerID)
	if err != nil {
		return domain.Property{}, fmt.Errorf("count active properties: %w", err)
	}
	if count >= limit {
		_ = tx.Rollback(ctx)
		return domain.Property{}, ErrLimitExceeded
	}

	if err := txRepo.Unarchive(ctx, id, ownerID); err != nil {
		return domain.Property{}, fmt.Errorf("unarchive property: %w", err)
	}

	txRecurringOps := s.recurringOps.WithTx(tx)
	txScheduler := s.scheduler.WithTx(tx)

	recs, err := txRecurringOps.ListByProperty(ctx, ownerID, id)
	if err != nil {
		return domain.Property{}, fmt.Errorf("list recurring operations: %w", err)
	}

	now := time.Now()
	for _, rec := range recs {
		if err := txRecurringOps.UpdateStatus(ctx, rec.ID, "active"); err != nil {
			return domain.Property{}, fmt.Errorf("resume recurring operation: %w", err)
		}
		if err := txScheduler.GenerateOperations(ctx, tx, rec, date(now)); err != nil {
			return domain.Property{}, fmt.Errorf("generate operations: %w", err)
		}
	}

	unarchived, err := txRepo.GetByIDAndOwner(ctx, id, ownerID)
	if err != nil {
		return domain.Property{}, fmt.Errorf("reload unarchived property: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Property{}, fmt.Errorf("commit tx: %w", err)
	}

	return unarchived, nil
}

func isUpdatableStatusTransition(from, to domain.PropertyStatus) bool {
	if from == to {
		return true
	}
	switch from {
	case domain.PropertyStatusActive:
		return to == domain.PropertyStatusMaintenance
	case domain.PropertyStatusMaintenance:
		return to == domain.PropertyStatusActive
	}
	return false
}

func date(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
