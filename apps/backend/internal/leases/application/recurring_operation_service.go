package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	notificationsdomain "github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/timeutil"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
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
	reminders    ReminderLister
	db           txBeginner
	clock        clock.Clock
	logger       *slog.Logger
}

// ReminderLister lists reminders for the recurring operation command.
type ReminderLister interface {
	ListByOwner(ctx context.Context, ownerID uuid.UUID, filter notificationsapp.ListFilter) ([]notificationsdomain.Reminder, error)
	ListByRecurringOperation(ctx context.Context, ownerID, recurringOpID uuid.UUID, filter notificationsapp.ListFilter) ([]notificationsdomain.Reminder, error)
}

// NewRecurringOperationService creates a new recurring operation service.
func NewRecurringOperationService(
	recurringOps RecurringOperationRepository,
	operations OperationRepository,
	properties PropertyRepository,
	scheduler ReminderScheduler,
	reminders ReminderLister,
	db txBeginner,
	clock clock.Clock,
	logger *slog.Logger,
) *RecurringOperationService {
	if db == nil {
		panic("db beginner is required")
	}
	if clock == nil {
		panic("clock is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &RecurringOperationService{
		recurringOps: recurringOps,
		operations:   operations,
		properties:   properties,
		scheduler:    scheduler,
		reminders:    reminders,
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

	_, err = s.generateOperations(ctx, txOps, created, now, nil)
	if err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("generate operations: %w", err)
	}

	if s.scheduler != nil && created.ReminderOffsetDays != nil {
		persistedOps, err := txOps.ListByRecurringOperation(ctx, created.ID)
		if err != nil {
			return domain.RecurringOperation{}, fmt.Errorf("list operations for scheduling: %w", err)
		}
		txScheduler := s.scheduler.WithTx(tx)
		if err := scheduleRemindersForOperations(ctx, txScheduler, created, persistedOps, now); err != nil {
			return domain.RecurringOperation{}, fmt.Errorf("schedule reminders: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("commit tx: %w", err)
	}

	return created, nil
}

// ListRecurringOperationsByProperty returns the recurring operations for a
// property.
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

	return recs, nil
}

// GetRecurringOperation returns a single recurring operation owned by the given
// owner.
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

	return rec, nil
}

// UpdateRecurringOperation updates a recurring operation owned by the given owner
// and regenerates future operations to reflect schedule or amount changes.
func (s *RecurringOperationService) UpdateRecurringOperation(
	ctx context.Context,
	ownerID, id uuid.UUID,
	cmd UpdateRecurringOperationCommand,
) (domain.RecurringOperation, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRecurring := s.recurringOps.WithTx(tx)
	txOps := s.operations.WithTx(tx)

	rec, err := txRecurring.GetByIDAndOwnerForUpdate(ctx, id, ownerID)
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

	updated, err := txRecurring.Update(ctx, rec)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.RecurringOperation{}, ErrNotFound
		}
		return domain.RecurringOperation{}, fmt.Errorf("update recurring operation: %w", err)
	}

	now := s.clock.Now()
	if s.scheduler != nil && updated.ReminderOffsetDays != nil {
		txScheduler := s.scheduler.WithTx(tx)
		if err := txScheduler.CancelByRecurringOperation(ctx, ownerID, updated.ID); err != nil {
			return domain.RecurringOperation{}, fmt.Errorf("cancel recurring reminders: %w", err)
		}
	}

	if err := txOps.DeleteUneditedFutureOperationsByRecurringOperation(ctx, updated.ID, now); err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("delete future operations: %w", err)
	}

	_, err = s.generateOperations(ctx, txOps, updated, now, func(d time.Time) bool {
		return !timeutil.Date(d).Before(timeutil.Date(now))
	})
	if err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("regenerate operations: %w", err)
	}

	if s.scheduler != nil && updated.ReminderOffsetDays != nil {
		persistedOps, err := txOps.ListByRecurringOperation(ctx, updated.ID)
		if err != nil {
			return domain.RecurringOperation{}, fmt.Errorf("list operations for scheduling: %w", err)
		}
		txScheduler := s.scheduler.WithTx(tx)
		if err := scheduleRemindersForOperations(ctx, txScheduler, updated, persistedOps, now); err != nil {
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

	txRecurring := s.recurringOps.WithTx(tx)
	var rec domain.RecurringOperation

	_, err = txRecurring.GetByIDAndOwnerForUpdate(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.RecurringOperation{}, ErrNotFound
		}
		return domain.RecurringOperation{}, fmt.Errorf("get recurring operation: %w", err)
	}

	if s.scheduler != nil {
		txScheduler := s.scheduler.WithTx(tx)
		if err := txScheduler.CancelByRecurringOperation(ctx, ownerID, id); err != nil {
			return domain.RecurringOperation{}, fmt.Errorf("cancel reminders: %w", err)
		}
	}

	rec, err = txRecurring.UpdateStatus(ctx, id, ownerID, string(domain.RecurringOperationStatusPaused))
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
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRecurring := s.recurringOps.WithTx(tx)
	txOps := s.operations.WithTx(tx)

	rec, err := txRecurring.GetByIDAndOwnerForUpdate(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.RecurringOperation{}, ErrNotFound
		}
		return domain.RecurringOperation{}, fmt.Errorf("get recurring operation: %w", err)
	}

	if rec.Status == domain.RecurringOperationStatusActive {
		if err := tx.Commit(ctx); err != nil {
			return domain.RecurringOperation{}, fmt.Errorf("commit tx: %w", err)
		}
		return rec, nil
	}

	rec, err = txRecurring.UpdateStatus(ctx, id, ownerID, string(domain.RecurringOperationStatusActive))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.RecurringOperation{}, ErrNotFound
		}
		return domain.RecurringOperation{}, fmt.Errorf("resume recurring operation: %w", err)
	}

	now := s.clock.Now()
	_, err = s.generateOperations(ctx, txOps, rec, now, func(d time.Time) bool {
		return !timeutil.Date(d).Before(timeutil.Date(now))
	})
	if err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("generate operations: %w", err)
	}

	if s.scheduler != nil && rec.ReminderOffsetDays != nil {
		persistedOps, err := txOps.ListByRecurringOperation(ctx, rec.ID)
		if err != nil {
			return domain.RecurringOperation{}, fmt.Errorf("list operations for scheduling: %w", err)
		}
		txScheduler := s.scheduler.WithTx(tx)
		if err := scheduleRemindersForOperations(ctx, txScheduler, rec, persistedOps, now); err != nil {
			return domain.RecurringOperation{}, fmt.Errorf("schedule reminders: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.RecurringOperation{}, fmt.Errorf("commit tx: %w", err)
	}

	return rec, nil
}

// ListOperationsByRecurringOperation returns the generated operations for a
// recurring operation after verifying ownership.
func (s *RecurringOperationService) ListOperationsByRecurringOperation(
	ctx context.Context,
	ownerID, id uuid.UUID,
) ([]domain.Operation, error) {
	if _, err := s.recurringOps.GetByIDAndOwner(ctx, id, ownerID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get recurring operation: %w", err)
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
	ownerID, id uuid.UUID,
	offsetDays int,
) error {
	if offsetDays < 0 {
		return newInvalidInputError("offset_days must be non-negative")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRecurring := s.recurringOps.WithTx(tx)
	txOps := s.operations.WithTx(tx)

	rec, err := txRecurring.GetByIDAndOwnerForUpdate(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("get recurring operation: %w", err)
	}

	ops, err := txOps.ListByRecurringOperation(ctx, id)
	if err != nil {
		return fmt.Errorf("list operations: %w", err)
	}

	now := s.clock.Now()
	if err := s.applyReminderOffsetInTx(ctx, tx, ownerID, rec, ops, offsetDays, now); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// applyReminderOffsetInTx stores the offset and rebuilds concrete reminders
// inside an existing transaction. The caller is responsible for committing.
func (s *RecurringOperationService) applyReminderOffsetInTx(
	ctx context.Context,
	tx transaction.Tx,
	ownerID uuid.UUID,
	rec domain.RecurringOperation,
	ops []domain.Operation,
	offsetDays int,
	now time.Time,
) error {
	txRecurring := s.recurringOps.WithTx(tx)
	if err := txRecurring.SetReminderOffset(ctx, ownerID, rec.ID, offsetDays); err != nil {
		return fmt.Errorf("set reminder offset: %w", err)
	}

	if s.scheduler == nil {
		return nil
	}

	txScheduler := s.scheduler.WithTx(tx)
	if err := txScheduler.CancelByRecurringOperation(ctx, ownerID, rec.ID); err != nil {
		return fmt.Errorf("cancel recurring reminders: %w", err)
	}
	if offsetDays >= 0 {
		rec.ReminderOffsetDays = &offsetDays
		if err := scheduleRemindersForOperations(ctx, txScheduler, rec, ops, now); err != nil {
			return fmt.Errorf("schedule reminders: %w", err)
		}
	}
	return nil
}

// CreateReminder creates concrete reminders for all future generated operations
// of a recurring operation using the reminder date selected by the user. It
// persists the computed offset and returns the created reminders.
func (s *RecurringOperationService) CreateReminder(
	ctx context.Context,
	ownerID, recurringOperationID uuid.UUID,
	reminderDate time.Time,
) ([]notificationsdomain.Reminder, error) {
	if s.reminders == nil {
		return nil, fmt.Errorf("reminder lister is required")
	}

	now := s.clock.Now()
	if err := notificationsdomain.ValidateReminderDate(reminderDate, now); err != nil {
		return nil, newInvalidInputError("reminder date must be today or in the future")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRecurring := s.recurringOps.WithTx(tx)
	txOps := s.operations.WithTx(tx)

	rec, err := txRecurring.GetByIDAndOwnerForUpdate(ctx, recurringOperationID, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get recurring operation: %w", err)
	}

	ops, err := txOps.ListByRecurringOperation(ctx, recurringOperationID)
	if err != nil {
		return nil, fmt.Errorf("list operations: %w", err)
	}

	futureOps := futureOperations(ops, now)
	if len(futureOps) == 0 {
		return nil, newInvalidInputError("no future operations for reminder")
	}

	earliest := futureOps[0]
	for _, op := range futureOps {
		if op.OperationDate.Before(earliest.OperationDate) {
			earliest = op
		}
	}

	offsetDays := notificationsdomain.ReminderOffset(earliest.OperationDate, reminderDate)
	if offsetDays < 0 {
		return nil, newInvalidInputError("reminder date must be on or before the earliest future operation date")
	}

	if err := s.applyReminderOffsetInTx(ctx, tx, ownerID, rec, futureOps, offsetDays, now); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	reminders, err := s.reminders.ListByRecurringOperation(ctx, ownerID, recurringOperationID, notificationsapp.ListFilter{Limit: 1000})
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	return reminders, nil
}

func (s *RecurringOperationService) validateCommand(opType, category string, amount int64, startDate time.Time, paymentDay int, endDate *time.Time) error {
	if _, _, err := parseTypeAndCategory(opType, category); err != nil {
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

func futureOperations(ops []domain.Operation, now time.Time) []domain.Operation {
	today := timeutil.Date(now)
	out := make([]domain.Operation, 0, len(ops))
	for _, op := range ops {
		if !timeutil.Date(op.OperationDate).Before(today) {
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
	now time.Time,
) error {
	if scheduler == nil {
		return nil
	}
	if len(ops) == 0 || rec.ReminderOffsetDays == nil {
		return nil
	}

	offsetDays := *rec.ReminderOffsetDays
	today := timeutil.Date(now)

	filtered := make([]domain.Operation, 0, len(ops))
	var earliestOp domain.Operation
	for i := range ops {
		op := ops[i]
		reminderDate := op.OperationDate.AddDate(0, 0, -offsetDays)
		if timeutil.Date(reminderDate).Before(today) {
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

	if err := scheduler.ScheduleForRecurringOperation(ctx, recInfo, baseReminderDate, ToOperationInfoSlice(filtered)); err != nil {
		if errors.Is(err, notificationsdomain.ErrInvalidReminderDate) {
			return newInvalidInputError("reminder date must be today or in the future")
		}
		return err
	}
	return nil
}

