package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/timeutil"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
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
	scheduler    ReminderScheduler
	db           txBeginner
	clock        clock.Clock
	logger       *slog.Logger
}

// NewRecurringOperationService creates a new recurring operation service.
func NewRecurringOperationService(
	recurringOps RecurringOperationRepository,
	operations OperationRepository,
	properties PropertyRepository,
	scheduler ReminderScheduler,
	db txBeginner,
	clock clock.Clock,
	logger *slog.Logger,
) *RecurringOperationService {
	if logger == nil {
		logger = slog.Default()
	}
	return &RecurringOperationService{
		recurringOps: recurringOps,
		operations:   operations,
		properties:   properties,
		scheduler:    scheduler,
		db:           db,
		clock:        clock,
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
	if err := validateProperty(ctx, s.properties, ownerID, cmd.PropertyID); err != nil {
		return domain.RecurringOperation{}, err
	}

	if err := s.validateCommand(cmd.Type, cmd.Category, cmd.AmountKopecks, cmd.StartDate, cmd.PaymentDay, cmd.EndDate); err != nil {
		return domain.RecurringOperation{}, err
	}

	id, err := uuid.NewRandom()
	if err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("generate recurring operation id: %w", err)
	}

	now := s.clock.Now()
	rec := domain.RecurringOperation{
		ID:            id,
		OwnerID:       ownerID,
		PropertyID:    cmd.PropertyID,
		Type:          domain.OperationType(cmd.Type),
		Category:      domain.OperationCategory(cmd.Category),
		AmountKopecks: cmd.AmountKopecks,
		StartDate:     cmd.StartDate,
		PaymentDay:    cmd.PaymentDay,
		EndDate:       cmd.EndDate,
		Periodicity:   domain.RecurringOperationPeriodicityMonthly,
		Status:        domain.RecurringOperationStatusActive,
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

	generatedOps, err := s.generateOperations(ctx, txOps, created, now, nil)
	if err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("generate operations: %w", err)
	}

	if s.scheduler != nil && created.ReminderOffsetDays != nil && len(generatedOps) > 0 {
		txScheduler := s.scheduler.WithTx(tx)
		if err := s.scheduleRemindersForOperations(ctx, txScheduler, created, generatedOps); err != nil {
			return domain.RecurringOperation{}, fmt.Errorf("schedule reminders: %w", err)
		}
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
	if err := validateProperty(ctx, s.properties, ownerID, propertyID); err != nil {
		return nil, err
	}

	recs, err := s.recurringOps.ListByProperty(ctx, ownerID, propertyID)
	if err != nil {
		return nil, fmt.Errorf("list recurring operations: %w", err)
	}

	now := s.clock.Now()
	for i := range recs {
		if recs[i].Status != domain.RecurringOperationStatusActive {
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

	if rec.Status == domain.RecurringOperationStatusActive {
		if err := s.extendHorizon(ctx, rec, s.clock.Now()); err != nil {
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

	typeStr := string(rec.Type)
	categoryStr := string(rec.Category)
	if cmd.Type != nil {
		typeStr = *cmd.Type
	}
	if cmd.Category != nil {
		categoryStr = *cmd.Category
	}
	opType, category, err := parseTypeAndCategory(typeStr, categoryStr)
	if err != nil {
		return domain.RecurringOperation{}, err
	}
	rec.Type = opType
	rec.Category = category

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

	if err := s.validateCommand(string(rec.Type), string(rec.Category), rec.AmountKopecks, rec.StartDate, rec.PaymentDay, rec.EndDate); err != nil {
		return domain.RecurringOperation{}, err
	}

	rec.UpdatedAt = s.clock.Now()

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRecurring := s.recurringOps.WithTx(tx)
	txOps := s.operations.WithTx(tx)
	txScheduler := s.scheduler.WithTx(tx)

	updated, err := txRecurring.Update(ctx, rec)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.RecurringOperation{}, ErrNotFound
		}
		return domain.RecurringOperation{}, fmt.Errorf("update recurring operation: %w", err)
	}

	if s.scheduler != nil && updated.ReminderOffsetDays != nil {
		if err := txScheduler.CancelByRecurringOperation(ctx, ownerID, updated.ID); err != nil {
			return domain.RecurringOperation{}, fmt.Errorf("cancel recurring reminders: %w", err)
		}
	}

	now := s.clock.Now()
	if err := txOps.DeleteUneditedFutureOperationsByRecurringOperation(ctx, updated.ID, now); err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("delete future operations: %w", err)
	}

	generatedOps, err := s.generateOperations(ctx, txOps, updated, now, func(d time.Time) bool {
		return !timeutil.Date(d).Before(timeutil.Date(now))
	})
	if err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("regenerate operations: %w", err)
	}

	if s.scheduler != nil && updated.ReminderOffsetDays != nil && len(generatedOps) > 0 {
		if err := s.scheduleRemindersForOperations(ctx, txScheduler, updated, generatedOps); err != nil {
			return domain.RecurringOperation{}, fmt.Errorf("schedule reminders: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("commit tx: %w", err)
	}

	return updated, nil
}

// PauseRecurringOperation marks a recurring operation as paused and cancels its reminders.
func (s *RecurringOperationService) PauseRecurringOperation(
	ctx context.Context,
	ownerID, id uuid.UUID,
) (domain.RecurringOperation, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if s.scheduler != nil {
		txScheduler := s.scheduler.WithTx(tx)
		if err := txScheduler.CancelByRecurringOperation(ctx, ownerID, id); err != nil {
			return domain.RecurringOperation{}, fmt.Errorf("cancel reminders: %w", err)
		}
	}

	rec, err := s.recurringOps.WithTx(tx).UpdateStatus(ctx, id, ownerID, string(domain.RecurringOperationStatusPaused))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.RecurringOperation{}, ErrNotFound
		}
		return domain.RecurringOperation{}, fmt.Errorf("pause recurring operation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("commit tx: %w", err)
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

	if rec.Status == domain.RecurringOperationStatusActive {
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

	now := s.clock.Now()
	generatedOps, err := s.generateOperations(ctx, txOps, rec, now, func(d time.Time) bool {
		return !timeutil.Date(d).Before(timeutil.Date(now))
	})
	if err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("generate operations: %w", err)
	}

	if s.scheduler != nil && rec.ReminderOffsetDays != nil && len(generatedOps) > 0 {
		txScheduler := s.scheduler.WithTx(tx)
		if err := s.scheduleRemindersForOperations(ctx, txScheduler, rec, generatedOps); err != nil {
			return domain.RecurringOperation{}, fmt.Errorf("schedule reminders: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("commit tx: %w", err)
	}

	return rec, nil
}

func (s *RecurringOperationService) validateCommand(opType, category string, amount int64, startDate time.Time, paymentDay int, endDate *time.Time) error {
	if _, _, err := parseTypeAndCategory(opType, category); err != nil {
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
	if endDate != nil && timeutil.Date(*endDate).Before(timeutil.Date(startDate)) {
		return fmt.Errorf("%w: end_date must be on or after start_date", ErrInvalidInput)
	}
	return nil
}

func (s *RecurringOperationService) existingOperationDates(ctx context.Context, ops OperationRepository, recurringOperationID uuid.UUID) (map[time.Time]struct{}, error) {
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

// extendHorizon generates additional operations when the furthest generated date
// is less than 12 months from now (and the template has no end_date or the end
// date is further out).
func (s *RecurringOperationService) extendHorizon(ctx context.Context, rec domain.RecurringOperation, now time.Time) error {
	existing, err := s.existingOperationDates(ctx, s.operations, rec.ID)
	if err != nil {
		return fmt.Errorf("list existing dates: %w", err)
	}

	maxDate := timeutil.Date(rec.StartDate)
	for d := range existing {
		if d.After(maxDate) {
			maxDate = d
		}
	}

	horizon := timeutil.Date(now).AddDate(0, 12, 0)
	if !maxDate.Before(horizon) {
		return nil
	}
	if rec.EndDate != nil && timeutil.Date(*rec.EndDate).Before(horizon) {
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

	if s.scheduler != nil && rec.ReminderOffsetDays != nil {
		allOps, err := s.operations.ListByRecurringOperation(ctx, rec.ID)
		if err != nil {
			return fmt.Errorf("list operations for scheduling: %w", err)
		}
		if len(allOps) > 0 {
			if err := s.scheduleRemindersForOperations(ctx, s.scheduler, rec, allOps); err != nil {
				return fmt.Errorf("schedule reminders: %w", err)
			}
		}
	}

	return nil
}

func (s *RecurringOperationService) generateOperations(
	ctx context.Context,
	ops OperationRepository,
	rec domain.RecurringOperation,
	now time.Time,
	filter func(time.Time) bool,
) ([]domain.Operation, error) {
	existing, err := s.existingOperationDates(ctx, ops, rec.ID)
	if err != nil {
		return nil, fmt.Errorf("list existing dates: %w", err)
	}

	toCreate, err := s.buildOperations(rec, now, existing, filter)
	if err != nil {
		return nil, err
	}
	if len(toCreate) == 0 {
		return nil, nil
	}

	if err := ops.BulkCreate(ctx, toCreate); err != nil {
		return nil, fmt.Errorf("bulk create operations: %w", err)
	}
	return toCreate, nil
}

func (s *RecurringOperationService) buildOperations(
	rec domain.RecurringOperation,
	now time.Time,
	existing map[time.Time]struct{},
	filter func(time.Time) bool,
) ([]domain.Operation, error) {
	dates := domain.GenerateDates(rec.StartDate, rec.PaymentDay, rec.EndDate, now)

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

func (s *RecurringOperationService) scheduleRemindersForOperations(
	ctx context.Context,
	scheduler ReminderScheduler,
	rec domain.RecurringOperation,
	ops []domain.Operation,
) error {
	if len(ops) == 0 || rec.ReminderOffsetDays == nil {
		return nil
	}

	offsetDays := *rec.ReminderOffsetDays
	baseReminderDate := ops[0].OperationDate.AddDate(0, 0, -offsetDays)

	recInfo := notificationsapp.RecurringOperationInfo{
		ID:         rec.ID,
		OwnerID:    rec.OwnerID,
		PropertyID: rec.PropertyID,
		LeaseID:    leaseIDPtr(rec.LeaseID),
	}

	return scheduler.ScheduleForRecurringOperation(ctx, recInfo, baseReminderDate, ToOperationInfoSlice(ops))
}

func leaseIDPtr(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}
