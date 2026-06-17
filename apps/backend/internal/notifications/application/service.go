package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/timeutil"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

const leaseExpiringOffsetDays = 30

// ReminderService implements CRUD and scheduling use cases for reminders.
type ReminderService struct {
	repo     ReminderRepository
	clock    clock.Clock
	beginner transaction.Beginner
}

// NewReminderService creates a new reminder service.
func NewReminderService(repo ReminderRepository, clock clock.Clock, beginner transaction.Beginner) *ReminderService {
	return &ReminderService{repo: repo, clock: clock, beginner: beginner}
}

// WithTx returns a service bound to the provided transaction.
func (s *ReminderService) WithTx(tx transaction.Tx) *ReminderService {
	return &ReminderService{repo: s.repo.WithTx(tx), clock: s.clock, beginner: s.beginner}
}

// CreateForOperation creates a pending reminder for a future operation.
func (s *ReminderService) CreateForOperation(ctx context.Context, op OperationInfo, reminderDate time.Time) (domain.Reminder, error) {
	r, err := buildOperationReminder(op, reminderDate, s.clock.Now())
	if err != nil {
		if errors.Is(err, domain.ErrInvalidReminderDate) {
			return domain.Reminder{}, fmt.Errorf("%w: %w", ErrInvalidReminderDate, err)
		}
		return domain.Reminder{}, err
	}
	if err := s.repo.Save(ctx, r); err != nil {
		return domain.Reminder{}, fmt.Errorf("save reminder: %w", err)
	}
	return r, nil
}

// CreateForRecurringOperation creates concrete reminders for all generated operations using a computed offset.
func (s *ReminderService) CreateForRecurringOperation(ctx context.Context, rec RecurringOperationInfo, baseReminderDate time.Time, ops []OperationInfo) error {
	reminders, err := buildRecurringReminders(rec, baseReminderDate, ops, s.clock.Now())
	if err != nil {
		return err
	}

	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create recurring reminders transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txService := s.WithTx(tx)
	for _, r := range reminders {
		if err := txService.repo.Save(ctx, r); err != nil {
			return fmt.Errorf("save reminder: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit create recurring reminders transaction: %w", err)
	}
	return nil
}

// CreateForLease creates lease expiring and requires_action reminders.
func (s *ReminderService) CreateForLease(ctx context.Context, lease LeaseInfo) error {
	reminders, err := buildLeaseReminders(lease, s.clock.Now())
	if err != nil {
		return err
	}

	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create lease reminders transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txService := s.WithTx(tx)
	for _, r := range reminders {
		if err := txService.repo.Save(ctx, r); err != nil {
			return fmt.Errorf("save lease %s reminder: %w", r.EventType, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit create lease reminders transaction: %w", err)
	}
	return nil
}

// ListByOwner returns reminders for an owner with optional status filter.
func (s *ReminderService) ListByOwner(ctx context.Context, ownerID uuid.UUID, filter ListFilter) ([]domain.Reminder, error) {
	reminders, err := s.repo.ListByOwner(ctx, ownerID, filter)
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	return reminders, nil
}

// GetByID returns a reminder by ID after verifying ownership.
func (s *ReminderService) GetByID(ctx context.Context, ownerID, id uuid.UUID) (domain.Reminder, error) {
	r, err := s.repo.GetByID(ctx, id, ownerID)
	if err != nil {
		return domain.Reminder{}, err
	}
	return r, nil
}

// Reschedule updates the scheduled date of a pending reminder.
func (s *ReminderService) Reschedule(ctx context.Context, ownerID, id uuid.UUID, newDate time.Time) (domain.Reminder, error) {
	r, err := s.GetByID(ctx, ownerID, id)
	if err != nil {
		return domain.Reminder{}, err
	}
	if r.Status != domain.ReminderPending {
		return domain.Reminder{}, ErrReminderNotPending
	}
	if err := domain.ValidateReminderDate(newDate, s.clock.Now()); err != nil {
		return domain.Reminder{}, fmt.Errorf("%w: %w", ErrInvalidReminderDate, err)
	}
	scheduledAt := domain.ScheduledAtForDate(newDate)
	if err := s.repo.UpdateScheduledAt(ctx, ownerID, id, scheduledAt); err != nil {
		return domain.Reminder{}, fmt.Errorf("update rescheduled reminder: %w", err)
	}
	r.ScheduledAt = scheduledAt
	return r, nil
}

// Cancel cancels a pending reminder.
func (s *ReminderService) Cancel(ctx context.Context, ownerID, id uuid.UUID) error {
	r, err := s.GetByID(ctx, ownerID, id)
	if err != nil {
		return err
	}
	if r.Status != domain.ReminderPending {
		return ErrReminderNotPending
	}
	targetID, err := targetIDFor(r)
	if err != nil {
		return err
	}

	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin cancel reminder transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txService := s.WithTx(tx)
	if err := txService.repo.CancelByTarget(ctx, ownerID, r.TargetType, targetID, r.EventType); err != nil {
		return fmt.Errorf("cancel reminder: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit cancel reminder transaction: %w", err)
	}
	return nil
}

func targetIDFor(r domain.Reminder) (uuid.UUID, error) {
	if r.OperationID != nil {
		return *r.OperationID, nil
	}
	if r.RecurringOperationID != nil {
		return *r.RecurringOperationID, nil
	}
	if r.LeaseID != nil {
		return *r.LeaseID, nil
	}
	return uuid.UUID{}, errors.New("reminder has no target id")
}

func buildOperationReminder(op OperationInfo, reminderDate time.Time, now time.Time) (domain.Reminder, error) {
	title := "Напоминание об операции"
	amountRubles := float64(op.AmountKopecks) / 100
	body := fmt.Sprintf("%s %.2f ₽ запланировано на %s", op.Category, amountRubles, op.OperationDate.Format("02.01.2006"))
	return domain.NewOperationReminder(op.OwnerID, op.ID, op.PropertyID, reminderDate, title, body, now)
}

func buildRecurringReminders(rec RecurringOperationInfo, baseReminderDate time.Time, ops []OperationInfo, now time.Time) ([]domain.Reminder, error) {
	if len(ops) == 0 {
		return nil, nil
	}
	earliestOp := ops[0]
	for _, op := range ops {
		if op.OperationDate.Before(earliestOp.OperationDate) {
			earliestOp = op
		}
	}
	offset := domain.ReminderOffset(earliestOp.OperationDate, baseReminderDate)
	if offset < 0 {
		offset = 0
	}
	reminders := make([]domain.Reminder, 0, len(ops))
	for _, op := range ops {
		reminderDate := op.OperationDate.AddDate(0, 0, -offset)
		r, err := buildOperationReminder(op, reminderDate, now)
		if err != nil {
			return nil, fmt.Errorf("create reminder for operation %s: %w", op.ID, err)
		}
		reminders = append(reminders, r)
	}
	return reminders, nil
}

func buildLeaseReminders(lease LeaseInfo, now time.Time) ([]domain.Reminder, error) {
	if lease.EndDate == nil || lease.EndDate.IsZero() {
		return nil, nil
	}
	ownerID := lease.OwnerID
	propertyID := lease.PropertyID
	endDate := *lease.EndDate

	var out []domain.Reminder

	expiringDate := endDate.AddDate(0, 0, -leaseExpiringOffsetDays)
	if !timeutil.Date(expiringDate).Before(timeutil.Date(now)) {
		expiringTitle := "Аренда скоро заканчивается"
		expiringBody := fmt.Sprintf("Аренда по объекту заканчивается %s", endDate.Format("02.01.2006"))
		expiring, err := domain.NewLeaseReminder(ownerID, lease.ID, propertyID, expiringDate, expiringTitle, expiringBody, domain.EventLeaseExpiring, now)
		if err != nil {
			return nil, fmt.Errorf("create lease expiring reminder: %w", err)
		}
		out = append(out, expiring)
	}

	requiresAction, err := buildLeaseRequiresActionReminder(lease, now, endDate.AddDate(0, 0, 1))
	if err != nil {
		return nil, fmt.Errorf("create lease requires_action reminder: %w", err)
	}
	out = append(out, requiresAction)
	return out, nil
}

func buildLeaseRequiresActionReminder(lease LeaseInfo, now, reminderDate time.Time) (domain.Reminder, error) {
	requiresActionTitle := "Аренда требует действия"
	requiresActionBody := "Срок аренды закончился. Подтвердите продление или завершение аренды."
	return domain.NewLeaseReminder(lease.OwnerID, lease.ID, lease.PropertyID, reminderDate, requiresActionTitle, requiresActionBody, domain.EventLeaseRequiresAction, now)
}

// scheduler implements ReminderScheduler and runs inside transactions.
type scheduler struct {
	repo  ReminderRepository
	clock clock.Clock
}

// NewReminderScheduler creates a transactional reminder scheduler.
func NewReminderScheduler(repo ReminderRepository, clock clock.Clock) ReminderScheduler {
	return &scheduler{repo: repo, clock: clock}
}

// WithTx returns a scheduler bound to the provided transaction.
func (s *scheduler) WithTx(tx transaction.Tx) ReminderScheduler {
	return &scheduler{repo: s.repo.WithTx(tx), clock: s.clock}
}

// ScheduleForOperation ensures a pending reminder exists for an operation.
func (s *scheduler) ScheduleForOperation(ctx context.Context, op OperationInfo, reminderDate time.Time) error {
	if err := s.repo.CancelByTarget(ctx, op.OwnerID, domain.TargetOperation, op.ID, domain.EventOperationDue); err != nil {
		return err
	}
	r, err := buildOperationReminder(op, reminderDate, s.clock.Now())
	if err != nil {
		return err
	}
	if err := s.repo.Save(ctx, r); err != nil {
		return fmt.Errorf("save reminder: %w", err)
	}
	return nil
}

// ScheduleForRecurringOperation cancels existing reminders and creates concrete reminders for generated operations.
func (s *scheduler) ScheduleForRecurringOperation(ctx context.Context, rec RecurringOperationInfo, baseReminderDate time.Time, ops []OperationInfo) error {
	if err := s.repo.CancelByRecurringOperationID(ctx, rec.OwnerID, rec.ID); err != nil {
		return err
	}
	reminders, err := buildRecurringReminders(rec, baseReminderDate, ops, s.clock.Now())
	if err != nil {
		return err
	}
	for _, r := range reminders {
		if err := s.repo.Save(ctx, r); err != nil {
			return fmt.Errorf("save reminder for operation %s: %w", *r.OperationID, err)
		}
	}
	return nil
}

// ScheduleForLease recreates lease reminders.
func (s *scheduler) ScheduleForLease(ctx context.Context, lease LeaseInfo) error {
	reminders, err := buildLeaseReminders(lease, s.clock.Now())
	if err != nil {
		return err
	}
	if len(reminders) == 0 {
		return nil
	}
	ownerID := lease.OwnerID
	if err := s.repo.CancelByTarget(ctx, ownerID, domain.TargetLease, lease.ID, domain.EventLeaseExpiring); err != nil {
		return err
	}
	if err := s.repo.CancelByTarget(ctx, ownerID, domain.TargetLease, lease.ID, domain.EventLeaseRequiresAction); err != nil {
		return err
	}
	for _, r := range reminders {
		if err := s.repo.Save(ctx, r); err != nil {
			return fmt.Errorf("save lease %s reminder: %w", r.EventType, err)
		}
	}
	return nil
}

// EnsureRequiresActionReminder cancels any pending lease_requires_action reminder
// and creates a fresh one. It is used by the lease reconciliation worker for leases
// whose end date has already passed, where a lease_expiring reminder is no longer
// applicable.
func (s *scheduler) EnsureRequiresActionReminder(ctx context.Context, lease LeaseInfo) error {
	if lease.EndDate == nil || lease.EndDate.IsZero() {
		return nil
	}
	exists, err := s.repo.HasReminderForLeaseEvent(ctx, lease.OwnerID, lease.ID, domain.EventLeaseRequiresAction)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	if err := s.repo.CancelByTarget(ctx, lease.OwnerID, domain.TargetLease, lease.ID, domain.EventLeaseRequiresAction); err != nil {
		return err
	}

	now := s.clock.Now()
	reminderDate := lease.EndDate.AddDate(0, 0, 1)
	if reminderDate.Before(now) {
		reminderDate = now
	}

	r, err := buildLeaseRequiresActionReminder(lease, now, reminderDate)
	if err != nil {
		return err
	}
	if err := s.repo.Save(ctx, r); err != nil {
		return fmt.Errorf("save lease requires_action reminder: %w", err)
	}
	return nil
}

// CancelByOperation cancels reminders for a concrete operation.
func (s *scheduler) CancelByOperation(ctx context.Context, ownerID, opID uuid.UUID) error {
	return s.repo.CancelByTarget(ctx, ownerID, domain.TargetOperation, opID, domain.EventOperationDue)
}

// CancelByRecurringOperation cancels reminders for a recurring operation template.
func (s *scheduler) CancelByRecurringOperation(ctx context.Context, ownerID, recID uuid.UUID) error {
	return s.repo.CancelByRecurringOperationID(ctx, ownerID, recID)
}

// CancelByLease cancels reminders for a lease.
func (s *scheduler) CancelByLease(ctx context.Context, ownerID, leaseID uuid.UUID) error {
	var errs []error
	if err := s.repo.CancelByTarget(ctx, ownerID, domain.TargetLease, leaseID, domain.EventLeaseExpiring); err != nil {
		errs = append(errs, err)
	}
	if err := s.repo.CancelByTarget(ctx, ownerID, domain.TargetLease, leaseID, domain.EventLeaseRequiresAction); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
