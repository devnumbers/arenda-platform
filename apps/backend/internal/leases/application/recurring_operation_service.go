package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
)

// CreateRecurringOperationCommand carries the data needed to create a user-managed
// recurring operation for a property.
type CreateRecurringOperationCommand struct {
	PropertyID    uuid.UUID
	Type          string
	Category      string
	AmountKopecks int64
	StartDate     time.Time
	PaymentDay    int
	EndDate       *time.Time
	Comment       *string
}

// UpdateRecurringOperationCommand carries optional updates for a recurring operation.
type UpdateRecurringOperationCommand struct {
	Type          *string
	Category      *string
	AmountKopecks *int64
	StartDate     *time.Time
	PaymentDay    *int
	EndDate       *time.Time
	Comment       *string
}

// RecurringOperationService orchestrates recurring operation use cases within the
// leases bounded context.
type RecurringOperationService struct {
	recurringOps RecurringOperationRepository
	operations   OperationRepository
	properties   PropertyRepository
	db           txBeginner
	logger       *slog.Logger
}

// NewRecurringOperationService creates a new recurring operation service.
func NewRecurringOperationService(
	recurringOps RecurringOperationRepository,
	operations OperationRepository,
	properties PropertyRepository,
	db txBeginner,
	logger *slog.Logger,
) *RecurringOperationService {
	if logger == nil {
		logger = slog.Default()
	}
	return &RecurringOperationService{
		recurringOps: recurringOps,
		operations:   operations,
		properties:   properties,
		db:           db,
		logger:       logger,
	}
}

// CreateRecurringOperation creates a user-managed recurring operation for the
// given owner and property and generates the initial 12-month operation horizon.
func (s *RecurringOperationService) CreateRecurringOperation(
	ctx context.Context,
	ownerID uuid.UUID,
	cmd CreateRecurringOperationCommand,
) (domain.RecurringOperation, error) {
	if err := s.validateProperty(ctx, ownerID, cmd.PropertyID); err != nil {
		return domain.RecurringOperation{}, err
	}

	if err := s.validateCommand(cmd.Type, cmd.Category, cmd.AmountKopecks, cmd.StartDate, cmd.PaymentDay, cmd.EndDate); err != nil {
		return domain.RecurringOperation{}, err
	}

	id, err := uuid.NewRandom()
	if err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("generate recurring operation id: %w", err)
	}

	now := time.Now()
	rec := domain.RecurringOperation{
		ID:            id,
		OwnerID:       ownerID,
		PropertyID:    cmd.PropertyID,
		Type:          cmd.Type,
		Category:      cmd.Category,
		AmountKopecks: cmd.AmountKopecks,
		StartDate:     cmd.StartDate,
		PaymentDay:    cmd.PaymentDay,
		EndDate:       cmd.EndDate,
		Periodicity:   string(domain.RecurringOperationPeriodicityMonthly),
		Status:        string(domain.RecurringOperationStatusActive),
		Comment:       stringOrEmpty(cmd.Comment),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRecurring := s.recurringOps.WithTx(tx)
	txOps := s.operations.WithTx(tx)

	created, err := txRecurring.Create(ctx, rec)
	if err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("create recurring operation: %w", err)
	}

	if err := s.generateOperations(ctx, txOps, created, now, nil); err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("generate operations: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("commit tx: %w", err)
	}

	return created, nil
}

// ListRecurringOperationsByProperty returns the recurring operations for a
// property and maintains the 12-month operation horizon for active templates.
func (s *RecurringOperationService) ListRecurringOperationsByProperty(
	ctx context.Context,
	ownerID, propertyID uuid.UUID,
) ([]domain.RecurringOperation, error) {
	if err := s.validateProperty(ctx, ownerID, propertyID); err != nil {
		return nil, err
	}

	recs, err := s.recurringOps.ListByProperty(ctx, ownerID, propertyID)
	if err != nil {
		return nil, fmt.Errorf("list recurring operations: %w", err)
	}

	now := time.Now()
	for i := range recs {
		if recs[i].Status != string(domain.RecurringOperationStatusActive) {
			continue
		}
		if err := s.extendHorizon(ctx, recs[i], now); err != nil {
			return nil, fmt.Errorf("extend horizon: %w", err)
		}
	}

	return recs, nil
}

// GetRecurringOperation returns a single recurring operation owned by the given
// owner and extends its operation horizon when active.
func (s *RecurringOperationService) GetRecurringOperation(
	ctx context.Context,
	ownerID, id uuid.UUID,
) (domain.RecurringOperation, error) {
	rec, err := s.recurringOps.GetByIDAndOwner(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.RecurringOperation{}, ErrNotFound
		}
		return domain.RecurringOperation{}, fmt.Errorf("get recurring operation: %w", err)
	}

	if rec.Status == string(domain.RecurringOperationStatusActive) {
		if err := s.extendHorizon(ctx, rec, time.Now()); err != nil {
			return domain.RecurringOperation{}, fmt.Errorf("extend horizon: %w", err)
		}
	}

	return rec, nil
}

// UpdateRecurringOperation updates a recurring operation owned by the given owner
// and regenerates future operations to reflect schedule or amount changes.
func (s *RecurringOperationService) UpdateRecurringOperation(
	ctx context.Context,
	ownerID, id uuid.UUID,
	cmd UpdateRecurringOperationCommand,
) (domain.RecurringOperation, error) {
	rec, err := s.recurringOps.GetByIDAndOwner(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.RecurringOperation{}, ErrNotFound
		}
		return domain.RecurringOperation{}, fmt.Errorf("get recurring operation: %w", err)
	}

	if cmd.Type != nil {
		rec.Type = *cmd.Type
	}
	if cmd.Category != nil {
		rec.Category = *cmd.Category
	}
	if _, _, err := s.parseTypeAndCategory(rec.Type, rec.Category); err != nil {
		return domain.RecurringOperation{}, err
	}

	if cmd.AmountKopecks != nil {
		rec.AmountKopecks = *cmd.AmountKopecks
	}
	if cmd.StartDate != nil {
		rec.StartDate = *cmd.StartDate
	}
	if cmd.PaymentDay != nil {
		rec.PaymentDay = *cmd.PaymentDay
	}
	if cmd.EndDate != nil {
		rec.EndDate = cmd.EndDate
	}
	if cmd.Comment != nil {
		rec.Comment = *cmd.Comment
	}

	if err := s.validateCommand(rec.Type, rec.Category, rec.AmountKopecks, rec.StartDate, rec.PaymentDay, rec.EndDate); err != nil {
		return domain.RecurringOperation{}, err
	}

	rec.UpdatedAt = time.Now()

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRecurring := s.recurringOps.WithTx(tx)
	txOps := s.operations.WithTx(tx)

	updated, err := txRecurring.Update(ctx, rec)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.RecurringOperation{}, ErrNotFound
		}
		return domain.RecurringOperation{}, fmt.Errorf("update recurring operation: %w", err)
	}

	now := time.Now()
	if err := txOps.DeleteUneditedFutureOperationsByRecurringOperation(ctx, updated.ID, now); err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("delete future operations: %w", err)
	}

	if err := s.generateOperations(ctx, txOps, updated, now, func(d time.Time) bool {
		return !date(d).Before(date(now))
	}); err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("regenerate operations: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("commit tx: %w", err)
	}

	return updated, nil
}

// PauseRecurringOperation marks a recurring operation as paused.
func (s *RecurringOperationService) PauseRecurringOperation(
	ctx context.Context,
	ownerID, id uuid.UUID,
) (domain.RecurringOperation, error) {
	rec, err := s.recurringOps.UpdateStatus(ctx, id, ownerID, string(domain.RecurringOperationStatusPaused))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.RecurringOperation{}, ErrNotFound
		}
		return domain.RecurringOperation{}, fmt.Errorf("pause recurring operation: %w", err)
	}
	return rec, nil
}

// ResumeRecurringOperation marks a recurring operation as active and generates any
// missing operations from the current date forward.
func (s *RecurringOperationService) ResumeRecurringOperation(
	ctx context.Context,
	ownerID, id uuid.UUID,
) (domain.RecurringOperation, error) {
	rec, err := s.recurringOps.GetByIDAndOwner(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.RecurringOperation{}, ErrNotFound
		}
		return domain.RecurringOperation{}, fmt.Errorf("get recurring operation: %w", err)
	}

	if rec.Status == string(domain.RecurringOperationStatusActive) {
		return rec, nil
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRecurring := s.recurringOps.WithTx(tx)
	txOps := s.operations.WithTx(tx)

	rec, err = txRecurring.UpdateStatus(ctx, id, ownerID, string(domain.RecurringOperationStatusActive))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.RecurringOperation{}, ErrNotFound
		}
		return domain.RecurringOperation{}, fmt.Errorf("resume recurring operation: %w", err)
	}

	now := time.Now()
	if err := s.generateOperations(ctx, txOps, rec, now, func(d time.Time) bool {
		return !date(d).Before(date(now))
	}); err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("generate operations: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("commit tx: %w", err)
	}

	return rec, nil
}

func (s *RecurringOperationService) validateProperty(ctx context.Context, ownerID, propertyID uuid.UUID) error {
	exists, err := s.properties.ExistsByOwner(ctx, propertyID, ownerID)
	if err != nil {
		return fmt.Errorf("check property: %w", err)
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}

func (s *RecurringOperationService) validateCommand(opType, category string, amount int64, startDate time.Time, paymentDay int, endDate *time.Time) error {
	if _, _, err := s.parseTypeAndCategory(opType, category); err != nil {
		return err
	}
	if amount < 0 {
		return fmt.Errorf("%w: amount must be non-negative", ErrInvalidInput)
	}
	if startDate.IsZero() {
		return fmt.Errorf("%w: start_date is required", ErrInvalidInput)
	}
	if paymentDay < 1 || paymentDay > 31 {
		return fmt.Errorf("%w: payment_day must be between 1 and 31", ErrInvalidInput)
	}
	if endDate != nil && date(*endDate).Before(date(startDate)) {
		return fmt.Errorf("%w: end_date must be on or after start_date", ErrInvalidInput)
	}
	return nil
}

func (s *RecurringOperationService) parseTypeAndCategory(typeStr, categoryStr string) (domain.OperationType, domain.OperationCategory, error) {
	opType, err := domain.ParseOperationType(typeStr)
	if err != nil {
		return "", "", fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}

	category, err := domain.ParseOperationCategory(categoryStr)
	if err != nil {
		return "", "", fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}

	if !domain.IsValidCategoryForType(category, opType) {
		return "", "", fmt.Errorf("%w: category %q is not valid for type %q", ErrInvalidInput, category, opType)
	}

	return opType, category, nil
}

func (s *RecurringOperationService) existingOperationDates(ctx context.Context, ops OperationRepository, recurringOperationID uuid.UUID) (map[time.Time]struct{}, error) {
	existing, err := ops.ListOperationDatesByRecurringOperation(ctx, recurringOperationID)
	if err != nil {
		return nil, err
	}
	dates := make(map[time.Time]struct{}, len(existing))
	for _, d := range existing {
		dates[date(d)] = struct{}{}
	}
	return dates, nil
}

// extendHorizon generates additional operations when the furthest generated date
// is less than 12 months from now (and the template has no end_date or the end
// date is further out).
func (s *RecurringOperationService) extendHorizon(ctx context.Context, rec domain.RecurringOperation, now time.Time) error {
	existing, err := s.existingOperationDates(ctx, s.operations, rec.ID)
	if err != nil {
		return fmt.Errorf("list existing dates: %w", err)
	}

	maxDate := date(rec.StartDate)
	for d := range existing {
		if d.After(maxDate) {
			maxDate = d
		}
	}

	horizon := date(now).AddDate(0, 12, 0)
	if !maxDate.Before(horizon) {
		return nil
	}
	if rec.EndDate != nil && date(*rec.EndDate).Before(horizon) {
		return nil
	}

	ops, err := s.buildOperations(rec, now, existing, func(d time.Time) bool {
		return d.After(maxDate)
	})
	if err != nil {
		return err
	}
	if len(ops) == 0 {
		return nil
	}

	if err := s.operations.BulkCreate(ctx, ops); err != nil {
		return fmt.Errorf("bulk create operations: %w", err)
	}
	return nil
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
	dates := domain.GenerateDates(rec.StartDate, rec.PaymentDay, rec.EndDate, now)

	createdAt := time.Now()
	ops := make([]domain.Operation, 0, len(dates))
	for _, d := range dates {
		d = date(d)
		if filter != nil && !filter(d) {
			continue
		}
		if _, ok := existing[d]; ok {
			continue
		}

		ops = append(ops, domain.Operation{
			OwnerID:              rec.OwnerID,
			PropertyID:           rec.PropertyID,
			LeaseID:              rec.LeaseID,
			RecurringOperationID: rec.ID,
			Type:                 rec.Type,
			Category:             rec.Category,
			AmountKopecks:        rec.AmountKopecks,
			OperationDate:        d,
			Comment:              rec.Comment,
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
