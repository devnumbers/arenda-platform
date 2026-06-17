package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

const leaseExpiringOffsetDays = 30

// ReminderService implements CRUD and scheduling use cases for reminders.
type ReminderService struct {
	repo  ReminderRepository
	clock clock.Clock
}

// NewReminderService creates a new reminder service.
func NewReminderService(repo ReminderRepository, clock clock.Clock) *ReminderService {
	return &ReminderService{repo: repo, clock: clock}
}

// CreateForOperation creates a pending reminder for a future operation.
func (s *ReminderService) CreateForOperation(ctx context.Context, op OperationInfo, reminderDate time.Time) (domain.Reminder, error) {
	title := "Напоминание об операции"
	body := fmt.Sprintf("%s %d коп. запланировано на %s", op.Category, op.AmountKopecks, op.OperationDate.Format("02.01.2006"))
	r, err := domain.NewOperationReminder(op.OwnerID, op.ID, op.PropertyID, op.OperationDate, reminderDate, title, body, s.clock.Now(), op.RecurringOperationID)
	if err != nil {
		return domain.Reminder{}, err
	}
	if err := s.repo.Save(ctx, r); err != nil {
		return domain.Reminder{}, fmt.Errorf("save reminder: %w", err)
	}
	return r, nil
}

// CreateForRecurringOperation creates concrete reminders for all generated operations using a computed offset.
func (s *ReminderService) CreateForRecurringOperation(ctx context.Context, rec RecurringOperationInfo, baseReminderDate time.Time, ops []OperationInfo) error {
	if len(ops) == 0 {
		return nil
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
	for _, op := range ops {
		reminderDate := op.OperationDate.AddDate(0, 0, -offset)
		if _, err := s.CreateForOperation(ctx, op, reminderDate); err != nil {
			return fmt.Errorf("create reminder for operation %s: %w", op.ID, err)
		}
	}
	return nil
}

// CreateForLease creates lease expiring and requires_action reminders.
func (s *ReminderService) CreateForLease(ctx context.Context, lease LeaseInfo) error {
	if lease.EndDate == nil || lease.EndDate.IsZero() {
		return nil
	}
	ownerID := lease.OwnerID
	propertyID := lease.PropertyID

	expiringTitle := "Аренда скоро заканчивается"
	expiringBody := fmt.Sprintf("Аренда по объекту заканчивается %s", lease.EndDate.Format("02.01.2006"))
	expiringDate := lease.EndDate.AddDate(0, 0, -leaseExpiringOffsetDays)
	expiring, err := domain.NewLeaseReminder(ownerID, lease.ID, propertyID, *lease.EndDate, expiringDate, expiringTitle, expiringBody, domain.EventLeaseExpiring, s.clock.Now())
	if err != nil {
		return fmt.Errorf("create lease expiring reminder: %w", err)
	}
	if err := s.repo.Save(ctx, expiring); err != nil {
		return fmt.Errorf("save lease expiring reminder: %w", err)
	}

	requiresActionTitle := "Аренда требует действия"
	requiresActionBody := "Срок аренды закончился. Подтвердите продление или завершение аренды."
	requiresActionDate := lease.EndDate.AddDate(0, 0, 1)
	requiresAction, err := domain.NewLeaseReminder(ownerID, lease.ID, propertyID, time.Time{}, requiresActionDate, requiresActionTitle, requiresActionBody, domain.EventLeaseRequiresAction, s.clock.Now())
	if err != nil {
		return fmt.Errorf("create lease requires_action reminder: %w", err)
	}
	if err := s.repo.Save(ctx, requiresAction); err != nil {
		return fmt.Errorf("save lease requires_action reminder: %w", err)
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
	r, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return domain.Reminder{}, err
	}
	if r.OwnerID != ownerID {
		return domain.Reminder{}, ErrNotFound
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
	if err := domain.ValidatePendingDate(r.EventDate, newDate, s.clock.Now()); err != nil {
		return domain.Reminder{}, fmt.Errorf("%w: %w", ErrInvalidReminderDate, err)
	}
	r.ScheduledAt = domain.ScheduledAtForDate(newDate)
	r.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, r); err != nil {
		return domain.Reminder{}, fmt.Errorf("update rescheduled reminder: %w", err)
	}
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
	return s.repo.CancelByTarget(ctx, ownerID, r.TargetType, targetID, r.EventType)
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
	title := "Напоминание об операции"
	body := fmt.Sprintf("%s %d коп. запланировано на %s", op.Category, op.AmountKopecks, op.OperationDate.Format("02.01.2006"))
	r, err := domain.NewOperationReminder(op.OwnerID, op.ID, op.PropertyID, op.OperationDate, reminderDate, title, body, s.clock.Now(), op.RecurringOperationID)
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
	if len(ops) == 0 {
		return nil
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
	if err := s.repo.CancelByRecurringOperationID(ctx, rec.OwnerID, rec.ID); err != nil {
		return err
	}
	for _, op := range ops {
		reminderDate := op.OperationDate.AddDate(0, 0, -offset)
		title := "Напоминание об операции"
		body := fmt.Sprintf("%s %d коп. запланировано на %s", op.Category, op.AmountKopecks, op.OperationDate.Format("02.01.2006"))
		r, err := domain.NewOperationReminder(op.OwnerID, op.ID, op.PropertyID, op.OperationDate, reminderDate, title, body, s.clock.Now(), op.RecurringOperationID)
		if err != nil {
			return fmt.Errorf("create reminder for operation %s: %w", op.ID, err)
		}
		if err := s.repo.Save(ctx, r); err != nil {
			return fmt.Errorf("save reminder for operation %s: %w", op.ID, err)
		}
	}
	return nil
}

// ScheduleForLease recreates lease reminders.
func (s *scheduler) ScheduleForLease(ctx context.Context, lease LeaseInfo) error {
	if lease.EndDate == nil || lease.EndDate.IsZero() {
		return nil
	}
	ownerID := lease.OwnerID
	propertyID := lease.PropertyID
	endDate := *lease.EndDate

	if err := s.repo.CancelByTarget(ctx, ownerID, domain.TargetLease, lease.ID, domain.EventLeaseExpiring); err != nil {
		return err
	}
	if err := s.repo.CancelByTarget(ctx, ownerID, domain.TargetLease, lease.ID, domain.EventLeaseRequiresAction); err != nil {
		return err
	}

	expiringTitle := "Аренда скоро заканчивается"
	expiringBody := fmt.Sprintf("Аренда по объекту заканчивается %s", endDate.Format("02.01.2006"))
	expiringDate := endDate.AddDate(0, 0, -leaseExpiringOffsetDays)
	expiring, err := domain.NewLeaseReminder(ownerID, lease.ID, propertyID, endDate, expiringDate, expiringTitle, expiringBody, domain.EventLeaseExpiring, s.clock.Now())
	if err != nil {
		return fmt.Errorf("create lease expiring reminder: %w", err)
	}
	if err := s.repo.Save(ctx, expiring); err != nil {
		return fmt.Errorf("save lease expiring reminder: %w", err)
	}

	requiresActionTitle := "Аренда требует действия"
	requiresActionBody := "Срок аренды закончился. Подтвердите продление или завершение аренды."
	requiresActionDate := endDate.AddDate(0, 0, 1)
	requiresAction, err := domain.NewLeaseReminder(ownerID, lease.ID, propertyID, time.Time{}, requiresActionDate, requiresActionTitle, requiresActionBody, domain.EventLeaseRequiresAction, s.clock.Now())
	if err != nil {
		return fmt.Errorf("create lease requires_action reminder: %w", err)
	}
	if err := s.repo.Save(ctx, requiresAction); err != nil {
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
