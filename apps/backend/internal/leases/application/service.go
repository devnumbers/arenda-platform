package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/timeutil"
	sharedtz "github.com/nambers/arenda-planform/apps/backend/internal/shared/tzresolver"
)

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
	ClearTenantContact   *bool
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
	categories     OperationCategoryRepository
	txStoreFactory
	clock      clock.Clock
	tzResolver sharedtz.OwnerTimezoneResolver
	policy     sharedpolicy.Policy
	logger     *slog.Logger
	// sharedIDs is optionally injected (see SetSharedPropertyIDs); when nil the
	// payment-schedule reads (LeasePaymentScheduleIndex,
	// CurrentPeriodOverdueIndex) cover only the actor's own rent operations,
	// when set they additionally include the shared properties' operations
	// (issue #157, T3) so a member's lease card shows the correct schedule.
	sharedIDs SharedPropertyIDs
}

// SetSharedPropertyIDs injects the access-context adapter that resolves the
// property ids shared with an actor via property membership (issue #157, T3).
// Optional: when nil, lease payment-schedule reads cover only the actor's own
// data; when set, they additionally cover the shared properties.
func (s *LeaseService) SetSharedPropertyIDs(ids SharedPropertyIDs) {
	s.sharedIDs = ids
}

// NewLeaseService creates a new lease service. The shared txStoreFactory
// carries the transactional collaborators (repositories, scheduler, audit,
// UoW); the repositories passed separately are the non-transactional read
// handles (ADR 0033).
func NewLeaseService(
	leases LeaseRepository,
	properties PropertyRepository,
	tenantContacts TenantContactRepository,
	categories OperationCategoryRepository,
	factory txStoreFactory,
	clk clock.Clock,
	tzResolver sharedtz.OwnerTimezoneResolver,
	policy sharedpolicy.Policy,
	logger *slog.Logger,
) *LeaseService {
	if clk == nil {
		panic("clk is required")
	}
	if categories == nil {
		panic("categories repository is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &LeaseService{
		leases:         leases,
		properties:     properties,
		tenantContacts: tenantContacts,
		categories:     categories,
		txStoreFactory: factory,
		clock:          clk,
		tzResolver:     tzResolver,
		policy:         policy,
		logger:         logger,
	}
}

func (s *LeaseService) CreateLease(ctx context.Context, actor uuid.UUID, cmd CreateLeaseCommand) (domain.Lease, error) {
	role, scope, err := resolveWriteScope(ctx, s.policy, s.properties, actor, cmd.PropertyID)
	if err != nil {
		return domain.Lease{}, err
	}

	exists, err := s.properties.ExistsActiveByOwner(ctx, cmd.PropertyID, scope)
	if err != nil {
		return domain.Lease{}, fmt.Errorf("check property availability: %w", err)
	}
	if !exists {
		return domain.Lease{}, ErrPropertyNotAvailable
	}

	if cmd.TenantContactID != nil {
		if _, err := s.tenantContacts.GetByIDAndOwner(ctx, *cmd.TenantContactID, scope); err != nil {
			if errors.Is(err, ErrNotFound) {
				return domain.Lease{}, ErrTenantContactNotFound
			}
			return domain.Lease{}, fmt.Errorf("get tenant contact: %w", err)
		}
	}

	lease, err := domain.NewLease(scope, cmd.PropertyID, cmd.StartDate, cmd.RentAmountKopecks, cmd.PaymentDay)
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

	loc, err := s.tzResolver.Resolve(ctx, scope)
	if err != nil {
		return domain.Lease{}, fmt.Errorf("resolve owner timezone: %w", err)
	}
	now := s.clock.Now()
	lease.Status = lease.CalculateStatus(timeutil.DateIn(now, loc))
	lease.CreatedAt = now
	lease.UpdatedAt = now

	rentCategoryID, err := getDefaultCategoryID(ctx, s.categories, scope, domain.OperationCategoryCodeRent)
	if err != nil {
		return domain.Lease{}, fmt.Errorf("rent category: %w", err)
	}

	var created domain.Lease
	err = s.runInTx(ctx, func(stores *txStores) error {
		txRentService := NewRentService(stores.operations, stores.recurringOps, stores.categories, s.clock, s.tzResolver)

		// Lock the property row for the rest of the transaction so a concurrent
		// DeleteProperty cannot remove it between the fast-path check above and
		// the lease insert. A missing or non-active property is rejected the same
		// way as in the fast-path check.
		propertyStatus, err := stores.properties.GetByIDAndOwnerForUpdate(ctx, cmd.PropertyID, scope)
		if err != nil {
			return fmt.Errorf("lock property: %w", err)
		}
		if propertyStatus != "active" {
			return ErrPropertyNotAvailable
		}

		hasOpen, err := stores.properties.HasOpenLease(ctx, cmd.PropertyID)
		if err != nil {
			return fmt.Errorf("check open lease: %w", err)
		}
		if hasOpen {
			return ErrOpenLeaseExists
		}

		created, err = stores.leases.Create(ctx, scope, lease)
		if err != nil {
			return fmt.Errorf("create lease: %w", err)
		}

		recurringOpID, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate recurring operation id: %w", err)
		}

		recurringOp := domain.RecurringOperation{
			ID:            recurringOpID,
			OwnerID:       scope,
			PropertyID:    created.PropertyID,
			LeaseID:       created.ID,
			Type:          domain.OperationTypeIncome,
			CategoryID:    rentCategoryID,
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

		createdRecurring, err := stores.recurringOps.Create(ctx, recurringOp)
		if err != nil {
			return fmt.Errorf("create recurring operation: %w", err)
		}

		ops, err := txRentService.GenerateRentOperations(ctx, created, createdRecurring.ID, scope, rentCategoryID)
		if err != nil {
			return fmt.Errorf("generate rent operations: %w", err)
		}
		if len(ops) > 0 {
			if err := stores.operations.BulkCreate(ctx, ops); err != nil {
				return fmt.Errorf("bulk create operations: %w", err)
			}
		}

		if stores.scheduler != nil {
			if err := stores.scheduler.ScheduleForLease(ctx, notificationsapp.LeaseInfo{
				ID:         created.ID,
				OwnerID:    created.OwnerID,
				PropertyID: created.PropertyID,
				EndDate:    created.EndDate,
			}); err != nil {
				return fmt.Errorf("schedule lease reminders: %w", err)
			}
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  actorRoleFromPolicyRole(role),
			Action:     auditdomain.ActionLeaseCreated,
			EntityType: auditdomain.EntityLease,
			EntityID:   &created.ID,
			Context: map[string]any{
				"property_id":         domain.PropertyIDPtr(created.PropertyID),
				"rent_amount_kopecks": created.RentAmountKopecks,
			},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Lease{}, err
	}

	return created, nil
}

func (s *LeaseService) ListLeases(ctx context.Context, actor uuid.UUID) ([]domain.Lease, error) {
	accessible, err := s.resolveAccessiblePropertyIDs(ctx, actor)
	if err != nil {
		return nil, err
	}
	leases, err := s.leases.ListByOwner(ctx, actor, accessible)
	if err != nil {
		return nil, fmt.Errorf("list leases: %w", err)
	}

	loc, err := s.tzResolver.Resolve(ctx, actor)
	if err != nil {
		return nil, fmt.Errorf("resolve owner timezone: %w", err)
	}

	result := make([]domain.Lease, 0, len(leases))
	for _, lease := range leases {
		result = append(result, s.applyEffectiveStatus(lease, loc))
	}
	return result, nil
}

func (s *LeaseService) GetLease(ctx context.Context, actor, id uuid.UUID) (domain.Lease, error) {
	lease, err := s.leases.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Lease{}, ErrNotFound
		}
		return domain.Lease{}, fmt.Errorf("get lease: %w", err)
	}
	role, err := roleForStandalone(ctx, s.policy, actor, lease.PropertyID, lease.OwnerID)
	if err != nil {
		return domain.Lease{}, err
	}
	if !sharedpolicy.CanView(role) {
		return domain.Lease{}, ErrNotFound
	}
	loc, err := s.tzResolver.Resolve(ctx, lease.OwnerID)
	if err != nil {
		return domain.Lease{}, fmt.Errorf("resolve owner timezone: %w", err)
	}
	return s.applyEffectiveStatus(lease, loc), nil
}

func (s *LeaseService) applyEffectiveStatus(lease domain.Lease, loc *time.Location) domain.Lease {
	lease.Status = lease.EffectiveStatus(timeutil.DateIn(s.clock.Now(), loc))
	return lease
}

// leaseUpdatePatch is the outcome of applying an UpdateLeaseCommand to a
// lease: the patched lease, the update instant, and which schedule sync the
// caller must run afterwards.
type leaseUpdatePatch struct {
	lease           domain.Lease
	now             time.Time
	scheduleRebuilt bool
	scheduleChanged bool
}

// applyLeaseUpdate applies the update command's patch fields to the lease
// (tenant contact, dates, amounts, payment day, comment) with validation and
// recomputes the lease status for the owner's current date.
func (s *LeaseService) applyLeaseUpdate(
	ctx context.Context,
	txTenantContacts TenantContactRepository,
	lease domain.Lease,
	cmd UpdateLeaseCommand,
	scope uuid.UUID,
) (leaseUpdatePatch, error) {
	if cmd.TenantContactID != nil && cmd.ClearTenantContact != nil && *cmd.ClearTenantContact {
		return leaseUpdatePatch{}, newInvalidInputError("tenant_contact_id and clear_tenant_contact cannot both be set")
	}
	if cmd.TenantContactID != nil {
		if _, err := txTenantContacts.GetByIDAndOwner(ctx, *cmd.TenantContactID, scope); err != nil {
			if errors.Is(err, ErrNotFound) {
				return leaseUpdatePatch{}, ErrTenantContactNotFound
			}
			return leaseUpdatePatch{}, fmt.Errorf("get tenant contact: %w", err)
		}
		lease.TenantContactID = cmd.TenantContactID
	}
	if cmd.ClearTenantContact != nil && *cmd.ClearTenantContact {
		lease.TenantContactID = nil
	}

	patch := leaseUpdatePatch{}
	if cmd.StartDate != nil {
		lease.StartDate = *cmd.StartDate
		patch.scheduleRebuilt = true
	}
	if cmd.EndDate != nil {
		lease.EndDate = cmd.EndDate
		patch.scheduleChanged = true
	}
	if cmd.RentAmountKopecks != nil {
		lease.RentAmountKopecks = *cmd.RentAmountKopecks
		patch.scheduleChanged = true
	}
	if cmd.DepositAmountKopecks != nil {
		lease.DepositAmountKopecks = *cmd.DepositAmountKopecks
	}
	if cmd.PaymentDay != nil {
		lease.PaymentDay = *cmd.PaymentDay
		patch.scheduleChanged = true
	}
	if cmd.Comment != nil {
		lease.Comment = *cmd.Comment
	}

	if err := lease.Validate(); err != nil {
		return leaseUpdatePatch{}, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}

	loc, err := s.tzResolver.Resolve(ctx, scope)
	if err != nil {
		return leaseUpdatePatch{}, fmt.Errorf("resolve owner timezone: %w", err)
	}
	now := s.clock.Now()
	lease.Status = lease.CalculateStatus(timeutil.DateIn(now, loc))
	lease.UpdatedAt = now

	patch.lease = lease
	patch.now = now
	return patch, nil
}

// resyncLeaseSchedule brings the rent schedule and reminders back in sync with
// an updated lease: it rebuilds or regenerates the schedule when the command
// demands it, aligns the lease-created recurring series, and re-schedules the
// lease reminders when the end date moved.
func (s *LeaseService) resyncLeaseSchedule(
	ctx context.Context,
	stores *txStores,
	txRentService *RentService,
	updated domain.Lease,
	originalStartDate time.Time,
	patch leaseUpdatePatch,
	endDateChanged bool,
) error {
	scope := updated.OwnerID
	if patch.scheduleRebuilt {
		if err := txRentService.RebuildSchedule(ctx, updated, originalStartDate); err != nil {
			return fmt.Errorf("rebuild schedule: %w", err)
		}
	} else if patch.scheduleChanged {
		if err := txRentService.RegenerateFutureOperations(ctx, updated, patch.now); err != nil {
			return fmt.Errorf("regenerate future operations: %w", err)
		}
	}

	if stores.scheduler != nil {
		if patch.scheduleRebuilt || patch.scheduleChanged {
			rec, err := stores.recurringOps.GetByLeaseID(ctx, scope, updated.ID)
			if err != nil && !errors.Is(err, ErrNotFound) {
				return fmt.Errorf("get recurring operation for lease: %w", err)
			}
			if err == nil {
				if endDateChanged {
					rec.EndDate = updated.EndDate
					if _, err := stores.recurringOps.Update(ctx, rec); err != nil {
						return fmt.Errorf("update recurring operation end date: %w", err)
					}
				}
				if rec.ReminderOffsetDays != nil && rec.Status == domain.RecurringOperationStatusActive {
					if err := stores.scheduler.CancelByRecurringOperation(ctx, scope, rec.ID); err != nil {
						return fmt.Errorf("cancel recurring reminders: %w", err)
					}
					ops, err := stores.operations.ListByRecurringOperation(ctx, rec.ID)
					if err != nil {
						return fmt.Errorf("list operations for scheduling: %w", err)
					}
					categoryNames, err := buildCategoryNamesMap(ctx, stores.categories, scope)
					if err != nil {
						return err
					}
					if err := scheduleRemindersForOperations(ctx, stores.scheduler, rec, ops, categoryNames, patch.now); err != nil {
						return fmt.Errorf("schedule recurring reminders: %w", err)
					}
				}
			}
		}

		if endDateChanged {
			if err := stores.scheduler.CancelByLease(ctx, scope, updated.ID); err != nil {
				return fmt.Errorf("cancel lease reminders: %w", err)
			}
			if updated.EndDate != nil {
				if err := stores.scheduler.ScheduleForLease(ctx, notificationsapp.LeaseInfo{
					ID:         updated.ID,
					OwnerID:    updated.OwnerID,
					PropertyID: updated.PropertyID,
					EndDate:    updated.EndDate,
				}); err != nil {
					return fmt.Errorf("schedule lease reminders: %w", err)
				}
			}
		}
	}
	return nil
}

func (s *LeaseService) UpdateLease(ctx context.Context, actor, id uuid.UUID, cmd UpdateLeaseCommand) (domain.Lease, error) {
	lease, err := s.leases.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Lease{}, ErrNotFound
		}
		return domain.Lease{}, fmt.Errorf("get lease: %w", err)
	}
	role, err := roleForStandalone(ctx, s.policy, actor, lease.PropertyID, lease.OwnerID)
	if err != nil {
		return domain.Lease{}, err
	}
	if err := writeRoleGate(role); err != nil {
		return domain.Lease{}, err
	}
	scope := lease.OwnerID

	var updated domain.Lease
	err = s.runInTx(ctx, func(stores *txStores) error {
		txRentService := NewRentService(stores.operations, stores.recurringOps, stores.categories, s.clock, s.tzResolver)

		lease, err := stores.leases.GetByIDAndOwnerForUpdate(ctx, id, scope)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("get lease: %w", err)
		}

		if lease.Status == domain.LeaseStatusCompleted {
			return ErrAlreadyCompleted
		}
		if lease.Status == domain.LeaseStatusArchived {
			return ErrArchivedLease
		}

		patch, err := s.applyLeaseUpdate(ctx, stores.tenantContacts, lease, cmd, scope)
		if err != nil {
			return err
		}
		originalStartDate := lease.StartDate
		originalEndDate := lease.EndDate

		updated, err = stores.leases.Update(ctx, scope, patch.lease)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("update lease: %w", err)
		}
		endDateChanged := !endDatesEqual(originalEndDate, updated.EndDate)

		if err := s.resyncLeaseSchedule(ctx, stores, txRentService, updated, originalStartDate, patch, endDateChanged); err != nil {
			return err
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  actorRoleFromPolicyRole(role),
			Action:     auditdomain.ActionLeaseUpdated,
			EntityType: auditdomain.EntityLease,
			EntityID:   &id,
			Context:    map[string]any{"fields": updatedLeaseFields(cmd)},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Lease{}, err
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

// updatedLeaseFields lists the names of the fields a command changes. Only
// field names are audited, never their values.
func updatedLeaseFields(cmd UpdateLeaseCommand) []string {
	fields := make([]string, 0, 7)
	if cmd.TenantContactID != nil || (cmd.ClearTenantContact != nil && *cmd.ClearTenantContact) {
		fields = append(fields, "tenant_contact_id")
	}
	if cmd.StartDate != nil {
		fields = append(fields, "start_date")
	}
	if cmd.EndDate != nil {
		fields = append(fields, "end_date")
	}
	if cmd.RentAmountKopecks != nil {
		fields = append(fields, "rent_amount_kopecks")
	}
	if cmd.DepositAmountKopecks != nil {
		fields = append(fields, "deposit_amount_kopecks")
	}
	if cmd.PaymentDay != nil {
		fields = append(fields, "payment_day")
	}
	if cmd.Comment != nil {
		fields = append(fields, "comment")
	}
	return fields
}

func (s *LeaseService) CompleteLease(ctx context.Context, actor, id uuid.UUID) (domain.Lease, error) {
	lease, err := s.leases.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Lease{}, ErrNotFound
		}
		return domain.Lease{}, fmt.Errorf("get lease: %w", err)
	}
	role, err := roleForStandalone(ctx, s.policy, actor, lease.PropertyID, lease.OwnerID)
	if err != nil {
		return domain.Lease{}, err
	}
	if err := writeRoleGate(role); err != nil {
		return domain.Lease{}, err
	}
	scope := lease.OwnerID

	var completed domain.Lease
	err = s.runInTx(ctx, func(stores *txStores) error {
		lease, err := stores.leases.GetByIDAndOwnerForUpdate(ctx, id, scope)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("get lease: %w", err)
		}

		if lease.Status == domain.LeaseStatusCompleted {
			return ErrAlreadyCompleted
		}
		if lease.Status == domain.LeaseStatusArchived {
			return ErrArchivedLease
		}
		if !lease.IsOpen() {
			return &InvalidStatusTransitionError{From: lease.Status, To: domain.LeaseStatusCompleted}
		}

		completed, err = stores.leases.Complete(ctx, id, scope)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("complete lease: %w", err)
		}

		now := s.clock.Now()
		if err := stores.operations.DeleteUneditedFutureOperationsByLease(ctx, id, now); err != nil {
			return fmt.Errorf("delete future operations: %w", err)
		}

		if err := stores.recurringOps.UpdateStatusByLeaseID(ctx, id, scope, string(domain.RecurringOperationStatusPaused)); err != nil {
			return fmt.Errorf("pause recurring operation: %w", err)
		}

		if stores.scheduler != nil {
			if err := stores.scheduler.CancelByLease(ctx, scope, id); err != nil {
				return fmt.Errorf("cancel lease reminders: %w", err)
			}
			rec, err := stores.recurringOps.GetByLeaseID(ctx, scope, id)
			if err != nil && !errors.Is(err, ErrNotFound) {
				return fmt.Errorf("get recurring operation for lease: %w", err)
			}
			if err == nil {
				if err := stores.scheduler.CancelByRecurringOperation(ctx, scope, rec.ID); err != nil {
					return fmt.Errorf("cancel recurring operation reminders: %w", err)
				}
			}
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  actorRoleFromPolicyRole(role),
			Action:     auditdomain.ActionLeaseCompleted,
			EntityType: auditdomain.EntityLease,
			EntityID:   &id,
			Context:    map[string]any{"property_id": domain.PropertyIDPtr(lease.PropertyID)},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Lease{}, err
	}

	return completed, nil
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
	return s.runInTx(ctx, func(stores *txStores) error {
		lease, err := stores.leases.GetByIDForUpdate(ctx, leaseID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil
			}
			return fmt.Errorf("get lease: %w", err)
		}

		if !lease.Status.IsOpen() || lease.EndDate == nil {
			return nil
		}

		loc, err := s.tzResolver.Resolve(ctx, lease.OwnerID)
		if err != nil {
			return fmt.Errorf("resolve owner timezone: %w", err)
		}

		expected := lease.CalculateStatus(timeutil.DateIn(asOf, loc))
		if expected != domain.LeaseStatusRequiresAction {
			return nil
		}

		if lease.Status != domain.LeaseStatusRequiresAction {
			lease.Status = domain.LeaseStatusRequiresAction
			lease.UpdatedAt = s.clock.Now()
			if _, err := stores.leases.Update(ctx, lease.OwnerID, lease); err != nil {
				return fmt.Errorf("update lease status: %w", err)
			}
		}

		if stores.scheduler != nil {
			if err := stores.scheduler.EnsureRequiresActionReminder(ctx, notificationsapp.LeaseInfo{
				ID:         lease.ID,
				OwnerID:    lease.OwnerID,
				PropertyID: lease.PropertyID,
				EndDate:    lease.EndDate,
			}); err != nil {
				return fmt.Errorf("ensure requires_action reminder: %w", err)
			}
		}
		return nil
	})
}
