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
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/timeutil"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type txBeginner interface {
	Begin(ctx context.Context) (transaction.Tx, error)
}

type CreateLeaseCommand struct {
	PropertyID           uuid.UUID
	TenantContactID      *uuid.UUID
	StartDate            time.Time
	EndDate              *time.Time
	RentAmountKopecks    int64
	DepositAmountKopecks int64
	PaymentDay           int
	Comment              string
}

type UpdateLeaseCommand struct {
	TenantContactID      *uuid.UUID
	StartDate            *time.Time
	EndDate              *time.Time
	RentAmountKopecks    *int64
	DepositAmountKopecks *int64
	PaymentDay           *int
	Comment              *string
}

type LeaseService struct {
	leases         LeaseRepository
	properties     PropertyRepository
	tenantContacts TenantContactRepository
	recurringOps   RecurringOperationRepository
	operations     OperationRepository
	scheduler      notificationsapp.ReminderScheduler
	db             txBeginner
	clock          clock.Clock
	logger         *slog.Logger
}

func NewLeaseService(
	leases LeaseRepository,
	properties PropertyRepository,
	tenantContacts TenantContactRepository,
	recurringOps RecurringOperationRepository,
	operations OperationRepository,
	scheduler notificationsapp.ReminderScheduler,
	db txBeginner,
	clock clock.Clock,
	logger *slog.Logger,
) *LeaseService {
	if db == nil {
		panic("db beginner is required")
	}
	if clock == nil {
		panic("clock is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &LeaseService{
		leases:         leases,
		properties:     properties,
		tenantContacts: tenantContacts,
		recurringOps:   recurringOps,
		operations:     operations,
		scheduler:      scheduler,
		db:             db,
		clock:          clock,
		logger:         logger,
	}
}

func (s *LeaseService) CreateLease(ctx context.Context, ownerID uuid.UUID, cmd CreateLeaseCommand) (domain.Lease, error) {
	exists, err := s.properties.ExistsActiveByOwner(ctx, cmd.PropertyID, ownerID)
	if err != nil {
		return domain.Lease{}, fmt.Errorf("check property availability: %w", err)
	}
	if !exists {
		return domain.Lease{}, ErrPropertyNotAvailable
	}

	if cmd.TenantContactID != nil {
		if _, err := s.tenantContacts.GetByIDAndOwner(ctx, *cmd.TenantContactID, ownerID); err != nil {
			if errors.Is(err, ErrNotFound) {
				return domain.Lease{}, ErrTenantContactNotFound
			}
			return domain.Lease{}, fmt.Errorf("get tenant contact: %w", err)
		}
	}

	lease, err := domain.NewLease(ownerID, cmd.PropertyID, cmd.StartDate, cmd.RentAmountKopecks, cmd.PaymentDay)
	if err != nil {
		return domain.Lease{}, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}

	lease.TenantContactID = cmd.TenantContactID
	lease.EndDate = cmd.EndDate
	lease.DepositAmountKopecks = cmd.DepositAmountKopecks
	lease.Comment = cmd.Comment

	if err := lease.Validate(); err != nil {
		return domain.Lease{}, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}

	now := s.clock.Now()
	lease.Status = lease.CalculateStatus(now)
	lease.CreatedAt = now
	lease.UpdatedAt = now

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Lease{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txLeases := s.leases.WithTx(tx)
	txRecurring := s.recurringOps.WithTx(tx)
	txOps := s.operations.WithTx(tx)
	txProperties := s.properties.WithTx(tx)
	txRentService := NewRentService(txOps, txRecurring, s.clock)

	hasOpen, err := txProperties.HasOpenLease(ctx, cmd.PropertyID)
	if err != nil {
		return domain.Lease{}, fmt.Errorf("check open lease: %w", err)
	}
	if hasOpen {
		return domain.Lease{}, ErrOpenLeaseExists
	}

	created, err := txLeases.Create(ctx, ownerID, lease)
	if err != nil {
		return domain.Lease{}, fmt.Errorf("create lease: %w", err)
	}

	recurringOpID, err := uuid.NewRandom()
	if err != nil {
		return domain.Lease{}, fmt.Errorf("generate recurring operation id: %w", err)
	}

	recurringOp := domain.RecurringOperation{
		ID:            recurringOpID,
		OwnerID:       ownerID,
		PropertyID:    created.PropertyID,
		LeaseID:       created.ID,
		Type:          domain.OperationTypeIncome,
		Category:      domain.OperationCategoryRent,
		Name:          "Арендная плата",
		AmountKopecks: created.RentAmountKopecks,
		StartDate:     created.StartDate,
		PaymentDay:    created.PaymentDay,
		EndDate:       created.EndDate,
		Periodicity:   domain.RecurringOperationPeriodicityMonthly,
		Status:        domain.RecurringOperationStatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	createdRecurring, err := txRecurring.Create(ctx, recurringOp)
	if err != nil {
		return domain.Lease{}, fmt.Errorf("create recurring operation: %w", err)
	}

	ops := txRentService.GenerateRentOperations(ctx, created, createdRecurring.ID, ownerID)
	if len(ops) > 0 {
		if err := txOps.BulkCreate(ctx, ops); err != nil {
			return domain.Lease{}, fmt.Errorf("bulk create operations: %w", err)
		}
	}

	if s.scheduler != nil {
		txScheduler := s.scheduler.WithTx(tx)
		if err := txScheduler.ScheduleForLease(ctx, notificationsapp.LeaseInfo{
			ID:         created.ID,
			OwnerID:    created.OwnerID,
			PropertyID: created.PropertyID,
			EndDate:    created.EndDate,
		}); err != nil {
			return domain.Lease{}, fmt.Errorf("schedule lease reminders: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Lease{}, fmt.Errorf("commit tx: %w", err)
	}

	return created, nil
}

func (s *LeaseService) ListLeases(ctx context.Context, ownerID uuid.UUID) ([]domain.Lease, error) {
	leases, err := s.leases.ListByOwner(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list leases: %w", err)
	}

	result := make([]domain.Lease, 0, len(leases))
	for _, lease := range leases {
		result = append(result, s.applyEffectiveStatus(lease))
	}
	return result, nil
}

func (s *LeaseService) GetLease(ctx context.Context, ownerID, id uuid.UUID) (domain.Lease, error) {
	lease, err := s.leases.GetByIDAndOwner(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Lease{}, ErrNotFound
		}
		return domain.Lease{}, fmt.Errorf("get lease: %w", err)
	}
	return s.applyEffectiveStatus(lease), nil
}

func (s *LeaseService) applyEffectiveStatus(lease domain.Lease) domain.Lease {
	lease.Status = lease.EffectiveStatus(s.clock.Now())
	return lease
}

func (s *LeaseService) UpdateLease(ctx context.Context, ownerID, id uuid.UUID, cmd UpdateLeaseCommand) (domain.Lease, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Lease{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txLeases := s.leases.WithTx(tx)
	txTenantContacts := s.tenantContacts.WithTx(tx)
	txRecurring := s.recurringOps.WithTx(tx)
	txOps := s.operations.WithTx(tx)
	txRentService := NewRentService(txOps, txRecurring, s.clock)

	lease, err := txLeases.GetByIDAndOwnerForUpdate(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Lease{}, ErrNotFound
		}
		return domain.Lease{}, fmt.Errorf("get lease: %w", err)
	}

	if lease.Status == domain.LeaseStatusCompleted {
		return domain.Lease{}, ErrAlreadyCompleted
	}
	if lease.Status == domain.LeaseStatusArchived {
		return domain.Lease{}, ErrArchivedLease
	}

	if cmd.TenantContactID != nil {
		if _, err := txTenantContacts.GetByIDAndOwner(ctx, *cmd.TenantContactID, ownerID); err != nil {
			if errors.Is(err, ErrNotFound) {
				return domain.Lease{}, ErrTenantContactNotFound
			}
			return domain.Lease{}, fmt.Errorf("get tenant contact: %w", err)
		}
		lease.TenantContactID = cmd.TenantContactID
	}

	scheduleChanged := false
	scheduleRebuilt := false
	originalStartDate := lease.StartDate
	originalEndDate := lease.EndDate

	if cmd.StartDate != nil {
		lease.StartDate = *cmd.StartDate
		scheduleRebuilt = true
	}
	if cmd.EndDate != nil {
		lease.EndDate = cmd.EndDate
		scheduleChanged = true
	}
	if cmd.RentAmountKopecks != nil {
		lease.RentAmountKopecks = *cmd.RentAmountKopecks
		scheduleChanged = true
	}
	if cmd.DepositAmountKopecks != nil {
		lease.DepositAmountKopecks = *cmd.DepositAmountKopecks
	}
	if cmd.PaymentDay != nil {
		lease.PaymentDay = *cmd.PaymentDay
		scheduleChanged = true
	}
	if cmd.Comment != nil {
		lease.Comment = *cmd.Comment
	}

	if err := lease.Validate(); err != nil {
		return domain.Lease{}, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}

	now := s.clock.Now()
	lease.Status = lease.CalculateStatus(now)
	lease.UpdatedAt = now

	updated, err := txLeases.Update(ctx, ownerID, lease)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Lease{}, ErrNotFound
		}
		return domain.Lease{}, fmt.Errorf("update lease: %w", err)
	}
	endDateChanged := !endDatesEqual(originalEndDate, updated.EndDate)

	if scheduleRebuilt {
		if err := txRentService.RebuildSchedule(ctx, updated, originalStartDate); err != nil {
			return domain.Lease{}, fmt.Errorf("rebuild schedule: %w", err)
		}
	} else if scheduleChanged {
		if err := txRentService.RegenerateFutureOperations(ctx, updated, now); err != nil {
			return domain.Lease{}, fmt.Errorf("regenerate future operations: %w", err)
		}
	}

	if s.scheduler != nil {
		txScheduler := s.scheduler.WithTx(tx)

		if scheduleRebuilt || scheduleChanged {
			rec, err := txRecurring.GetByLeaseID(ctx, ownerID, updated.ID)
			if err != nil && !errors.Is(err, ErrNotFound) {
				return domain.Lease{}, fmt.Errorf("get recurring operation for lease: %w", err)
			}
			if err == nil {
				if endDateChanged {
					rec.EndDate = updated.EndDate
					if _, err := txRecurring.Update(ctx, rec); err != nil {
						return domain.Lease{}, fmt.Errorf("update recurring operation end date: %w", err)
					}
				}
				if rec.ReminderOffsetDays != nil && rec.Status == domain.RecurringOperationStatusActive {
					if err := txScheduler.CancelByRecurringOperation(ctx, ownerID, rec.ID); err != nil {
						return domain.Lease{}, fmt.Errorf("cancel recurring reminders: %w", err)
					}
					ops, err := txOps.ListByRecurringOperation(ctx, rec.ID)
					if err != nil {
						return domain.Lease{}, fmt.Errorf("list operations for scheduling: %w", err)
					}
					if err := scheduleRemindersForOperations(ctx, txScheduler, rec, ops, now); err != nil {
						return domain.Lease{}, fmt.Errorf("schedule recurring reminders: %w", err)
					}
				}
			}
		}

		if endDateChanged {
			if err := txScheduler.CancelByLease(ctx, ownerID, updated.ID); err != nil {
				return domain.Lease{}, fmt.Errorf("cancel lease reminders: %w", err)
			}
			if updated.EndDate != nil {
				if err := txScheduler.ScheduleForLease(ctx, notificationsapp.LeaseInfo{
					ID:         updated.ID,
					OwnerID:    updated.OwnerID,
					PropertyID: updated.PropertyID,
					EndDate:    updated.EndDate,
				}); err != nil {
					return domain.Lease{}, fmt.Errorf("schedule lease reminders: %w", err)
				}
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Lease{}, fmt.Errorf("commit tx: %w", err)
	}

	return updated, nil
}

func endDatesEqual(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}

func (s *LeaseService) CompleteLease(ctx context.Context, ownerID, id uuid.UUID) (domain.Lease, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Lease{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txLeases := s.leases.WithTx(tx)
	txRecurring := s.recurringOps.WithTx(tx)
	txOps := s.operations.WithTx(tx)

	lease, err := txLeases.GetByIDAndOwnerForUpdate(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Lease{}, ErrNotFound
		}
		return domain.Lease{}, fmt.Errorf("get lease: %w", err)
	}

	if lease.Status == domain.LeaseStatusCompleted {
		return domain.Lease{}, ErrAlreadyCompleted
	}
	if lease.Status == domain.LeaseStatusArchived {
		return domain.Lease{}, ErrArchivedLease
	}
	if !lease.IsOpen() {
		return domain.Lease{}, &InvalidStatusTransitionError{From: lease.Status, To: domain.LeaseStatusCompleted}
	}

	completed, err := txLeases.Complete(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Lease{}, ErrNotFound
		}
		return domain.Lease{}, fmt.Errorf("complete lease: %w", err)
	}

	now := s.clock.Now()
	if err := txOps.DeleteUneditedFutureOperationsByLease(ctx, id, now); err != nil {
		return domain.Lease{}, fmt.Errorf("delete future operations: %w", err)
	}

	if err := txRecurring.UpdateStatusByLeaseID(ctx, id, ownerID, string(domain.RecurringOperationStatusPaused)); err != nil {
		return domain.Lease{}, fmt.Errorf("pause recurring operation: %w", err)
	}

	if s.scheduler != nil {
		txScheduler := s.scheduler.WithTx(tx)
		if err := txScheduler.CancelByLease(ctx, ownerID, id); err != nil {
			return domain.Lease{}, fmt.Errorf("cancel lease reminders: %w", err)
		}
		rec, err := txRecurring.GetByLeaseID(ctx, ownerID, id)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return domain.Lease{}, fmt.Errorf("get recurring operation for lease: %w", err)
		}
		if err == nil {
			if err := txScheduler.CancelByRecurringOperation(ctx, ownerID, rec.ID); err != nil {
				return domain.Lease{}, fmt.Errorf("cancel recurring operation reminders: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Lease{}, fmt.Errorf("commit tx: %w", err)
	}

	return completed, nil
}

func (s *LeaseService) ReturnDeposit(ctx context.Context, ownerID, leaseID uuid.UUID) (domain.Lease, domain.Operation, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Lease{}, domain.Operation{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txLeases := s.leases.WithTx(tx)
	txOps := s.operations.WithTx(tx)

	lease, err := txLeases.GetByIDAndOwnerForUpdate(ctx, leaseID, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Lease{}, domain.Operation{}, ErrNotFound
		}
		return domain.Lease{}, domain.Operation{}, fmt.Errorf("get lease: %w", err)
	}
	if lease.DepositAmountKopecks <= 0 {
		return domain.Lease{}, domain.Operation{}, fmt.Errorf("%w: no deposit to return", ErrInvalidInput)
	}

	now := s.clock.Now()
	if lease.EffectiveStatus(now) == domain.LeaseStatusAwaitingStart || lease.EffectiveStatus(now) == domain.LeaseStatusActive {
		return domain.Lease{}, domain.Operation{}, fmt.Errorf("%w: cannot return deposit for an active or not-yet-started lease", ErrInvalidInput)
	}

	exists, err := txOps.HasDepositReturnForLease(ctx, lease.ID)
	if err != nil {
		return domain.Lease{}, domain.Operation{}, fmt.Errorf("check deposit return: %w", err)
	}
	if exists {
		return domain.Lease{}, domain.Operation{}, fmt.Errorf("%w: deposit has already been returned for this lease", ErrInvalidInput)
	}

	opID, err := uuid.NewRandom()
	if err != nil {
		return domain.Lease{}, domain.Operation{}, fmt.Errorf("generate operation id: %w", err)
	}
	const depositReturnComment = "Возврат залога"
	op := domain.Operation{
		ID:            opID,
		OwnerID:       ownerID,
		PropertyID:    lease.PropertyID,
		LeaseID:       lease.ID,
		Type:          domain.OperationTypeExpense,
		Category:      domain.OperationCategoryDepositReturn,
		Status:        domain.OperationStatusPaid,
		Name:          depositReturnComment,
		AmountKopecks: lease.DepositAmountKopecks,
		OperationDate: timeutil.Date(now),
		Comment:       depositReturnComment,
		IsException:   true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := op.ValidateStatusForType(); err != nil {
		return domain.Lease{}, domain.Operation{}, err
	}
	createdOp, err := txOps.Create(ctx, op)
	if err != nil {
		return domain.Lease{}, domain.Operation{}, fmt.Errorf("create deposit return operation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Lease{}, domain.Operation{}, fmt.Errorf("commit tx: %w", err)
	}

	lease.Status = lease.EffectiveStatus(now)
	return lease, createdOp, nil
}

// ListOpenLeasesWithPastEndDate returns open leases whose end date is before asOf.
func (s *LeaseService) ListOpenLeasesWithPastEndDate(ctx context.Context, asOf time.Time, limit int) ([]domain.Lease, error) {
	leases, err := s.leases.ListOpenLeasesWithPastEndDate(ctx, asOf, limit)
	if err != nil {
		return nil, fmt.Errorf("list open leases with past end date: %w", err)
	}
	return leases, nil
}

// ReconcileRequiresAction moves an open lease whose end date has passed to
// requires_action and ensures a requires_action reminder exists. It is
// idempotent: repeated calls with the same lease are no-ops once the status
// and reminder are in place.
func (s *LeaseService) ReconcileRequiresAction(ctx context.Context, leaseID uuid.UUID, asOf time.Time) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txLeases := s.leases.WithTx(tx)

	lease, err := txLeases.GetByIDForUpdate(ctx, leaseID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return fmt.Errorf("get lease: %w", err)
	}

	if !lease.Status.IsOpen() || lease.EndDate == nil {
		return nil
	}

	expected := lease.CalculateStatus(asOf)
	if expected != domain.LeaseStatusRequiresAction {
		return nil
	}

	if lease.Status != domain.LeaseStatusRequiresAction {
		lease.Status = domain.LeaseStatusRequiresAction
		lease.UpdatedAt = s.clock.Now()
		if _, err := txLeases.Update(ctx, lease.OwnerID, lease); err != nil {
			return fmt.Errorf("update lease status: %w", err)
		}
	}

	if s.scheduler != nil {
		txScheduler := s.scheduler.WithTx(tx)
		if err := txScheduler.EnsureRequiresActionReminder(ctx, notificationsapp.LeaseInfo{
			ID:         lease.ID,
			OwnerID:    lease.OwnerID,
			PropertyID: lease.PropertyID,
			EndDate:    lease.EndDate,
		}); err != nil {
			return fmt.Errorf("ensure requires_action reminder: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
