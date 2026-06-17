package application

import (
	"context"
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
	r, err := domain.NewOperationReminder(op.OwnerID, op.ID, op.PropertyID, op.OperationDate, reminderDate, title, body, s.clock.Now())
	if err != nil {
		return domain.Reminder{}, err
	}
	r.RecurringOperationID = op.RecurringOperationID
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
	nextOp := ops[0]
	for _, op := range ops {
		if op.OperationDate.Before(nextOp.OperationDate) {
			nextOp = op
		}
	}
	offset := domain.ReminderOffset(nextOp.OperationDate, baseReminderDate)
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
	expiring, err := domain.NewLeaseReminder(ownerID, lease.ID, propertyID, expiringDate, expiringTitle, expiringBody, domain.EventLeaseExpiring, s.clock.Now())
	if err != nil {
		return fmt.Errorf("create lease expiring reminder: %w", err)
	}
	if err := s.repo.Save(ctx, expiring); err != nil {
		return fmt.Errorf("save lease expiring reminder: %w", err)
	}

	requiresActionTitle := "Аренда требует действия"
	requiresActionBody := "Срок аренды закончился. Подтвердите продление или завершение аренды."
	requiresActionDate := lease.EndDate.AddDate(0, 0, 1)
	requiresAction, err := domain.NewLeaseReminder(ownerID, lease.ID, propertyID, requiresActionDate, requiresActionTitle, requiresActionBody, domain.EventLeaseRequiresAction, s.clock.Now())
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
	return s.repo.ListByOwner(ctx, ownerID, filter)
}

// GetByID returns a reminder by ID after verifying ownership.
func (s *ReminderService) GetByID(ctx context.Context, ownerID, id uuid.UUID) (domain.Reminder, error) {
	r, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return domain.Reminder{}, err
	}
	if r.OwnerID != ownerID {
		return domain.Reminder{}, fmt.Errorf("reminder not found")
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
		return domain.Reminder{}, fmt.Errorf("cannot reschedule non-pending reminder")
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
		return fmt.Errorf("cannot cancel non-pending reminder")
	}
	return s.repo.CancelByTarget(ctx, ownerID, r.TargetType, targetIDFor(r), r.EventType)
}

func targetIDFor(r domain.Reminder) uuid.UUID {
	if r.OperationID != nil {
		return *r.OperationID
	}
	if r.RecurringOperationID != nil {
		return *r.RecurringOperationID
	}
	return *r.LeaseID
}

// scheduler implements ReminderScheduler and runs inside transactions.
type scheduler struct {
	service *ReminderService
	repo    ReminderRepository
}

// NewReminderScheduler creates a transactional reminder scheduler.
func NewReminderScheduler(service *ReminderService, repo ReminderRepository) ReminderScheduler {
	return &scheduler{service: service, repo: repo}
}

// WithTx returns a scheduler bound to the provided transaction.
func (s *scheduler) WithTx(tx transaction.Tx) ReminderScheduler {
	return &scheduler{service: s.service, repo: s.repo.WithTx(tx)}
}

// ScheduleForOperation ensures a pending reminder exists for an operation.
func (s *scheduler) ScheduleForOperation(ctx context.Context, op OperationInfo, reminderDate time.Time) error {
	if err := s.repo.CancelByTarget(ctx, op.OwnerID, domain.TargetOperation, op.ID, domain.EventOperationDue); err != nil {
		return err
	}
	_, err := s.service.CreateForOperation(ctx, op, reminderDate)
	return err
}

// ScheduleForRecurringOperation cancels existing reminders and creates concrete reminders for generated operations.
func (s *scheduler) ScheduleForRecurringOperation(ctx context.Context, rec RecurringOperationInfo, baseReminderDate time.Time, ops []OperationInfo) error {
	if err := s.repo.CancelByRecurringOperationID(ctx, rec.OwnerID, rec.ID); err != nil {
		return err
	}
	return s.service.CreateForRecurringOperation(ctx, rec, baseReminderDate, ops)
}

// ScheduleForLease recreates lease reminders.
func (s *scheduler) ScheduleForLease(ctx context.Context, lease LeaseInfo) error {
	if lease.EndDate == nil {
		return nil
	}
	_ = s.repo.CancelByTarget(ctx, lease.OwnerID, domain.TargetLease, lease.ID, domain.EventLeaseExpiring)
	_ = s.repo.CancelByTarget(ctx, lease.OwnerID, domain.TargetLease, lease.ID, domain.EventLeaseRequiresAction)
	return s.service.CreateForLease(ctx, lease)
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
	_ = s.repo.CancelByTarget(ctx, ownerID, domain.TargetLease, leaseID, domain.EventLeaseExpiring)
	return s.repo.CancelByTarget(ctx, ownerID, domain.TargetLease, leaseID, domain.EventLeaseRequiresAction)
}
