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
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/tzresolver"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

const (
	leaseExpiringOffsetDays = 30
	dispatchHour            = 10
)

// ReminderService implements CRUD and scheduling use cases for reminders.
type ReminderService struct {
	repo       ReminderRepository
	clock      clock.Clock
	tzResolver tzresolver.OwnerTimezoneResolver
}

// NewReminderService creates a new reminder service.
func NewReminderService(repo ReminderRepository, clock clock.Clock, tzResolver tzresolver.OwnerTimezoneResolver) *ReminderService {
	return &ReminderService{repo: repo, clock: clock, tzResolver: tzResolver}
}

// WithTx returns a service bound to the provided transaction.
func (s *ReminderService) WithTx(tx transaction.Tx) *ReminderService {
	return &ReminderService{repo: s.repo.WithTx(tx), clock: s.clock, tzResolver: s.tzResolver}
}

// CreateForOperation creates a pending reminder for a future operation. It is
// idempotent: any existing pending/sending reminder for the same operation and
// event is atomically replaced by the new one.
func (s *ReminderService) CreateForOperation(ctx context.Context, op OperationInfo, reminderDate time.Time) (domain.Reminder, error) {
	loc, err := s.tzResolver.Resolve(ctx, op.OwnerID)
	if err != nil {
		return domain.Reminder{}, fmt.Errorf("resolve owner timezone: %w", err)
	}
	r, err := buildOperationReminder(op, reminderDate, s.clock.Now(), loc)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidReminderDate) {
			return domain.Reminder{}, fmt.Errorf("%w: %w", ErrInvalidReminderDate, err)
		}
		return domain.Reminder{}, err
	}
	if err := s.repo.SaveOrReplaceOperationReminder(ctx, r); err != nil {
		return domain.Reminder{}, fmt.Errorf("save operation reminder: %w", err)
	}
	return r, nil
}

// CreateForRecurringOperation creates concrete reminders for all generated operations using a computed offset.
// The caller is responsible for providing a transaction-bound service when the
// reminders must be committed atomically with another operation.
func (s *ReminderService) CreateForRecurringOperation(ctx context.Context, rec RecurringOperationInfo, baseReminderDate time.Time, ops []OperationInfo) error {
	loc, err := s.tzResolver.Resolve(ctx, rec.OwnerID)
	if err != nil {
		return fmt.Errorf("resolve owner timezone: %w", err)
	}
	reminders, err := buildRecurringReminders(rec, baseReminderDate, ops, s.clock.Now(), loc)
	if err != nil {
		return err
	}

	for _, r := range reminders {
		if err := s.repo.Save(ctx, r); err != nil {
			return fmt.Errorf("save reminder: %w", err)
		}
	}
	return nil
}

// CreateForLease creates lease expiring and requires_action reminders.
// The caller is responsible for providing a transaction-bound service when the
// reminders must be committed atomically with another operation.
func (s *ReminderService) CreateForLease(ctx context.Context, lease LeaseInfo) error {
	loc, err := s.tzResolver.Resolve(ctx, lease.OwnerID)
	if err != nil {
		return fmt.Errorf("resolve owner timezone: %w", err)
	}
	reminders, err := buildLeaseReminders(lease, s.clock.Now(), loc)
	if err != nil {
		return err
	}

	for _, r := range reminders {
		if err := s.repo.Save(ctx, r); err != nil {
			return fmt.Errorf("save lease %s reminder: %w", r.EventType, err)
		}
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

// ListByOperation returns non-cancelled reminders for a concrete operation and owner.
func (s *ReminderService) ListByOperation(ctx context.Context, ownerID, operationID uuid.UUID, filter ListFilter) ([]domain.Reminder, error) {
	reminders, err := s.repo.ListByOperation(ctx, ownerID, operationID, filter)
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	return reminders, nil
}

// ListByLease returns non-cancelled reminders for a lease and owner.
func (s *ReminderService) ListByLease(ctx context.Context, ownerID, leaseID uuid.UUID, filter ListFilter) ([]domain.Reminder, error) {
	reminders, err := s.repo.ListByLease(ctx, ownerID, leaseID, filter)
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	return reminders, nil
}

// ListByRecurringOperation returns non-cancelled reminders for a recurring
// operation template and owner.
func (s *ReminderService) ListByRecurringOperation(ctx context.Context, ownerID, recurringOpID uuid.UUID, filter ListFilter) ([]domain.Reminder, error) {
	reminders, err := s.repo.ListByRecurringOperation(ctx, ownerID, recurringOpID, filter)
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
	loc, err := s.tzResolver.Resolve(ctx, ownerID)
	if err != nil {
		return domain.Reminder{}, fmt.Errorf("resolve owner timezone: %w", err)
	}
	if err := domain.ValidateReminderDate(newDate, s.clock.Now(), loc); err != nil {
		return domain.Reminder{}, fmt.Errorf("%w: %w", ErrInvalidReminderDate, err)
	}
	scheduledAt := domain.ScheduledAtForDate(newDate, loc, dispatchHour)
	if err := s.repo.UpdateScheduledAt(ctx, ownerID, id, scheduledAt); err != nil {
		return domain.Reminder{}, fmt.Errorf("update rescheduled reminder: %w", err)
	}
	r.ScheduledAt = scheduledAt
	return r, nil
}

// RescheduleForTimezoneChange recalculates all pending reminders for the owner
// using wall-clock timezone conversion. It is a no-op when oldTZ == newTZ.
func (s *ReminderService) RescheduleForTimezoneChange(ctx context.Context, ownerID uuid.UUID, oldTZ, newTZ string) error {
	if oldTZ == newTZ {
		return nil
	}
	if err := s.repo.ReschedulePendingRemindersByOwner(ctx, ownerID, oldTZ, newTZ); err != nil {
		return fmt.Errorf("reschedule reminders for timezone change: %w", err)
	}
	return nil
}

var _ tzresolver.ReminderRescheduler = (*ReminderService)(nil)

// Cancel cancels a pending or sending reminder.
func (s *ReminderService) Cancel(ctx context.Context, ownerID, id uuid.UUID) error {
	cancelled, err := s.repo.CancelByIDAndOwner(ctx, ownerID, id)
	if err != nil {
		return fmt.Errorf("cancel reminder: %w", err)
	}
	if !cancelled {
		return ErrNotFound
	}
	return nil
}

func buildOperationReminder(op OperationInfo, reminderDate time.Time, now time.Time, loc *time.Location) (domain.Reminder, error) {
	title := "Напоминание об операции"
	body := formatOperationAmountAndDate(op)
	return domain.NewOperationReminder(op.OwnerID, op.ID, op.PropertyID, reminderDate, title, body, now, loc, dispatchHour)
}

func buildOverdueReminder(op OperationInfo, reminderDate time.Time, now time.Time, loc *time.Location) (domain.Reminder, error) {
	title := "Операция просрочена"
	body := fmt.Sprintf("Операция %s просрочена. %s", formatOperationAmountAndDate(op), overdueCTA(op.Type))
	return domain.NewOperationOverdueReminder(op.OwnerID, op.ID, op.PropertyID, reminderDate, title, body, now, loc, dispatchHour)
}

func formatOperationAmountAndDate(op OperationInfo) string {
	// OperationInfo.AmountKopecks is guaranteed to be non-negative by the
	// operations bounded context; integer division and modulo behave as expected.
	rubles := op.AmountKopecks / 100
	kopecks := op.AmountKopecks % 100
	amountStr := fmt.Sprintf("%d.%02d", rubles, kopecks)
	return fmt.Sprintf("%s на %s ₽ от %s", operationCategoryDisplayName(op.CategoryName), amountStr, op.OperationDate.Format("02.01.2006"))
}

func operationCategoryDisplayName(category string) string {
	switch category {
	case "rent":
		return "Аренда"
	case "other_income":
		return "Прочий доход"
	case "utilities":
		return "Коммунальные услуги"
	case "repair":
		return "Ремонт"
	case "tax":
		return "Налог"
	case "other_expense":
		return "Прочий расход"
	default:
		return category
	}
}

func overdueCTA(opType string) string {
	switch opType {
	case "income":
		return "Отметьте получение в приложении."
	default:
		return "Отметьте оплату в приложении."
	}
}

func buildRecurringReminders(rec RecurringOperationInfo, baseReminderDate time.Time, ops []OperationInfo, now time.Time, loc *time.Location) ([]domain.Reminder, error) {
	if len(ops) == 0 {
		return nil, nil
	}
	earliestOp := ops[0]
	for _, op := range ops {
		if op.OperationDate.Before(earliestOp.OperationDate) {
			earliestOp = op
		}
	}
	offset := max(0, domain.ReminderOffset(earliestOp.OperationDate, baseReminderDate, loc))
	reminders := make([]domain.Reminder, 0, len(ops))
	for _, op := range ops {
		reminderDate := op.OperationDate.AddDate(0, 0, -offset)
		r, err := buildOperationReminder(op, reminderDate, now, loc)
		if err != nil {
			return nil, fmt.Errorf("create reminder for operation %s: %w", op.ID, err)
		}
		r.RecurringOperationID = &rec.ID
		reminders = append(reminders, r)
	}
	return reminders, nil
}

func buildLeaseReminders(lease LeaseInfo, now time.Time, loc *time.Location) ([]domain.Reminder, error) {
	if lease.EndDate == nil || lease.EndDate.IsZero() {
		return nil, nil
	}
	ownerID := lease.OwnerID
	propertyID := lease.PropertyID
	endDate := *lease.EndDate

	var out []domain.Reminder

	expiringDate := endDate.AddDate(0, 0, -leaseExpiringOffsetDays)
	if !timeutil.DateIn(expiringDate, loc).Before(timeutil.DateIn(now, loc)) {
		expiringTitle := "Аренда скоро заканчивается"
		expiringBody := "Аренда по объекту заканчивается " + endDate.Format("02.01.2006")
		expiring, err := domain.NewLeaseReminder(ownerID, lease.ID, propertyID, expiringDate, expiringTitle, expiringBody, domain.EventLeaseExpiring, now, loc, dispatchHour)
		if err != nil {
			return nil, fmt.Errorf("create lease expiring reminder: %w", err)
		}
		out = append(out, expiring)
	}

	requiresActionDate := endDate.AddDate(0, 0, 1)
	if requiresActionDate.Before(now) {
		requiresActionDate = now
	}
	requiresAction, err := buildLeaseRequiresActionReminder(lease, now, requiresActionDate, loc)
	if err != nil {
		return nil, fmt.Errorf("create lease requires_action reminder: %w", err)
	}
	out = append(out, requiresAction)
	return out, nil
}

func buildLeaseRequiresActionReminder(lease LeaseInfo, now, reminderDate time.Time, loc *time.Location) (domain.Reminder, error) {
	requiresActionTitle := "Аренда требует действия"
	requiresActionBody := "Срок аренды закончился. Подтвердите продление или завершение аренды."
	return domain.NewLeaseReminder(lease.OwnerID, lease.ID, lease.PropertyID, reminderDate, requiresActionTitle, requiresActionBody, domain.EventLeaseRequiresAction, now, loc, dispatchHour)
}

// scheduler implements ReminderScheduler and runs inside transactions.
type scheduler struct {
	repo       ReminderRepository
	clock      clock.Clock
	tzResolver tzresolver.OwnerTimezoneResolver
}

// NewReminderScheduler creates a transactional reminder scheduler.
func NewReminderScheduler(repo ReminderRepository, clock clock.Clock, tzResolver tzresolver.OwnerTimezoneResolver) ReminderScheduler {
	return &scheduler{repo: repo, clock: clock, tzResolver: tzResolver}
}

// WithTx returns a scheduler bound to the provided transaction.
func (s *scheduler) WithTx(tx transaction.Tx) ReminderScheduler {
	return &scheduler{repo: s.repo.WithTx(tx), clock: s.clock, tzResolver: s.tzResolver}
}

// ScheduleForOperation ensures a pending reminder exists for an operation.
func (s *scheduler) ScheduleForOperation(ctx context.Context, op OperationInfo, reminderDate time.Time) error {
	if err := s.repo.CancelByTarget(ctx, op.OwnerID, domain.TargetOperation, op.ID, domain.EventOperationDue); err != nil {
		return err
	}
	loc, err := s.tzResolver.Resolve(ctx, op.OwnerID)
	if err != nil {
		return fmt.Errorf("resolve owner timezone: %w", err)
	}
	r, err := buildOperationReminder(op, reminderDate, s.clock.Now(), loc)
	if err != nil {
		return err
	}
	if err := s.repo.Save(ctx, r); err != nil {
		return fmt.Errorf("save reminder: %w", err)
	}
	return nil
}

// ScheduleOverdueReminder ensures a pending overdue reminder exists for an operation.
// It is idempotent: if an active overdue reminder already exists, it returns nil.
func (s *scheduler) ScheduleOverdueReminder(ctx context.Context, op OperationInfo, reminderDate time.Time) error {
	exists, err := s.repo.HasReminderForOperationEvent(ctx, op.OwnerID, op.ID, domain.EventOperationOverdue)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	if err := s.repo.CancelByTarget(ctx, op.OwnerID, domain.TargetOperation, op.ID, domain.EventOperationDue); err != nil {
		return err
	}
	if err := s.repo.CancelByTarget(ctx, op.OwnerID, domain.TargetOperation, op.ID, domain.EventOperationOverdue); err != nil {
		return err
	}
	loc, err := s.tzResolver.Resolve(ctx, op.OwnerID)
	if err != nil {
		return fmt.Errorf("resolve owner timezone: %w", err)
	}
	r, err := buildOverdueReminder(op, reminderDate, s.clock.Now(), loc)
	if err != nil {
		return err
	}
	if err := s.repo.Save(ctx, r); err != nil {
		return fmt.Errorf("save overdue reminder: %w", err)
	}
	return nil
}

// ScheduleForRecurringOperation cancels existing reminders and creates concrete reminders for generated operations.
func (s *scheduler) ScheduleForRecurringOperation(ctx context.Context, rec RecurringOperationInfo, baseReminderDate time.Time, ops []OperationInfo) error {
	if err := s.repo.CancelByRecurringOperationID(ctx, rec.OwnerID, rec.ID); err != nil {
		return err
	}
	loc, err := s.tzResolver.Resolve(ctx, rec.OwnerID)
	if err != nil {
		return fmt.Errorf("resolve owner timezone: %w", err)
	}
	reminders, err := buildRecurringReminders(rec, baseReminderDate, ops, s.clock.Now(), loc)
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
	loc, err := s.tzResolver.Resolve(ctx, lease.OwnerID)
	if err != nil {
		return fmt.Errorf("resolve owner timezone: %w", err)
	}
	reminders, err := buildLeaseReminders(lease, s.clock.Now(), loc)
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

	loc, err := s.tzResolver.Resolve(ctx, lease.OwnerID)
	if err != nil {
		return fmt.Errorf("resolve owner timezone: %w", err)
	}
	r, err := buildLeaseRequiresActionReminder(lease, now, reminderDate, loc)
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

// CancelOverdueReminderByOperation cancels overdue reminders for a concrete operation.
func (s *scheduler) CancelOverdueReminderByOperation(ctx context.Context, ownerID, opID uuid.UUID) error {
	return s.repo.CancelByTarget(ctx, ownerID, domain.TargetOperation, opID, domain.EventOperationOverdue)
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

// HasReminderForOperationEvent reports whether an active reminder exists for the
// given operation and event type.
func (s *scheduler) HasReminderForOperationEvent(ctx context.Context, ownerID, operationID uuid.UUID, eventType domain.EventType) (bool, error) {
	return s.repo.HasReminderForOperationEvent(ctx, ownerID, operationID, eventType)
}

// ListByOperation returns non-cancelled reminders for a concrete operation and owner.
func (s *scheduler) ListByOperation(ctx context.Context, ownerID, operationID uuid.UUID, filter ListFilter) ([]domain.Reminder, error) {
	return s.repo.ListByOperation(ctx, ownerID, operationID, filter)
}

// ListByLease returns non-cancelled reminders for a lease and owner.
func (s *scheduler) ListByLease(ctx context.Context, ownerID, leaseID uuid.UUID, filter ListFilter) ([]domain.Reminder, error) {
	return s.repo.ListByLease(ctx, ownerID, leaseID, filter)
}
