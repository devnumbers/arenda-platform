package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	notifdomain "github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	propdomain "github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// This file is the role-matrix cover for the T3 policy enforcement in the
// leases module (issue #166): every public use case is exercised as owner,
// full-access member, viewer, outsider (none) and suspended member. Reads map
// any non-view role to ErrNotFound; writes map none/suspended to ErrNotFound
// and viewer to ErrForbidden. Repository calls always use the data owner
// (scope), never the actor.

var (
	policyOwnerID    = uuid.MustParse("aaaaaaaa-0000-0000-0000-000000000001")
	policyMemberID   = uuid.MustParse("aaaaaaaa-0000-0000-0000-000000000002")
	policyOtherOwner = uuid.MustParse("aaaaaaaa-0000-0000-0000-000000000003")
	policyPropertyID = uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001")
	policyTargetID   = uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000002")
	policyForeignID  = uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000003")
	policyOpID       = uuid.MustParse("cccccccc-0000-0000-0000-000000000001")
	policyLeaseID    = uuid.MustParse("dddddddd-0000-0000-0000-000000000001")
	policyRecID      = uuid.MustParse("eeeeeeee-0000-0000-0000-000000000001")
)

type policyRoleCase struct {
	name string
	role sharedpolicy.Role
}

var policyRoleCases = []policyRoleCase{
	{name: "owner", role: sharedpolicy.RoleOwner},
	{name: "full access", role: sharedpolicy.RoleFullAccess},
	{name: "viewer", role: sharedpolicy.RoleViewer},
	{name: "none", role: sharedpolicy.RoleNone},
	{name: "suspended", role: sharedpolicy.RoleSuspended},
}

// policyActorAndPolicy returns the actor and the policy fake for a role case.
// The owner acts on their own data (the fake policy defaults to RoleOwner);
// other roles act as a member with the role seeded on every test property.
func policyActorAndPolicy(role sharedpolicy.Role) (uuid.UUID, fakePolicy) {
	if role == sharedpolicy.RoleOwner {
		return policyOwnerID, fakePolicy{}
	}
	return policyMemberID, fakePolicy{propertyRoles: map[[2]uuid.UUID]sharedpolicy.Role{
		{policyMemberID, policyPropertyID}: role,
		{policyMemberID, policyTargetID}:   role,
		{policyMemberID, policyForeignID}:  role,
	}}
}

// newPolicyPropertyRepo returns a property fake where policyPropertyID and
// policyTargetID belong to policyOwnerID and policyForeignID belongs to
// policyOtherOwner (for cross-owner move rejection).
func newPolicyPropertyRepo() *fakePropertyRepo {
	return &fakePropertyRepo{
		existsActiveByOwner: map[uuid.UUID]bool{policyPropertyID: true},
		statuses: map[uuid.UUID]string{
			policyPropertyID: "active",
			policyTargetID:   "active",
			policyForeignID:  "active",
		},
		owners: map[uuid.UUID]uuid.UUID{
			policyPropertyID: policyOwnerID,
			policyTargetID:   policyOwnerID,
			policyForeignID:  policyOtherOwner,
		},
	}
}

func policyTestOperation() domain.Operation {
	return domain.Operation{
		ID:            policyOpID,
		OwnerID:       policyOwnerID,
		PropertyID:    policyPropertyID,
		Type:          domain.OperationTypeExpense,
		CategoryID:    testCustomExpenseCategoryID,
		Status:        domain.OperationStatusPending,
		Name:          "test",
		AmountKopecks: 1000,
		OperationDate: date(2026, 6, 20),
	}
}

func newPolicyOperationService(policy fakePolicy, opRepo *fakeOperationRepo, propertyRepo *fakePropertyRepo, scheduler ReminderScheduler, audit auditapp.Recorder) *OperationService {
	return NewOperationService(opRepo, propertyRepo, nil, nil, newFakeCategoryRepoForOwner(policyOwnerID), scheduler, fakeTxBeginner{}, audit, fakeClock{now: date(2026, 6, 15)}, fakeTzResolver{}, policy, nil)
}

func newPolicyLeaseService(policy fakePolicy, leaseRepo *fakeLeaseRepo, propertyRepo *fakePropertyRepo, opRepo *fakeOperationRepo, recRepo *fakeRecurringOperationRepo) *LeaseService {
	return NewLeaseService(leaseRepo, propertyRepo, nil, recRepo, opRepo, newFakeCategoryRepoForOwner(policyOwnerID), nil, fakeTxBeginner{}, nil, fakeClock{now: date(2026, 6, 15)}, fakeTzResolver{}, policy, nil)
}

func newPolicyRecurringService(policy fakePolicy, recRepo *fakeRecurringOperationRepo, opRepo *fakeOperationRepo, propertyRepo *fakePropertyRepo) *RecurringOperationService {
	return NewRecurringOperationService(recRepo, opRepo, propertyRepo, newFakeCategoryRepoForOwner(policyOwnerID), nil, nil, fakeTxBeginner{}, nil, fakeClock{now: date(2026, 6, 15)}, fakeTzResolver{}, policy, nil)
}

// fakeAuditRecorder captures recorded audit entries.
type fakeAuditRecorder struct {
	entries []auditdomain.Entry
}

func (f *fakeAuditRecorder) Record(_ context.Context, entry auditdomain.Entry) error {
	f.entries = append(f.entries, entry)
	return nil
}

func (f *fakeAuditRecorder) WithTx(_ transaction.Tx) auditapp.Recorder { return f }

// fakeReminderScheduler counts reminder cancellations and captures scheduled
// operation reminders.
type fakeReminderScheduler struct {
	cancels        int
	overdueCancels int
	scheduled      []notificationsapp.OperationInfo
}

func (f *fakeReminderScheduler) ScheduleForOperation(_ context.Context, op notificationsapp.OperationInfo, _ time.Time) error {
	f.scheduled = append(f.scheduled, op)
	return nil
}

func (f *fakeReminderScheduler) ScheduleOverdueReminder(context.Context, notificationsapp.OperationInfo, time.Time) error {
	return nil
}

func (f *fakeReminderScheduler) ScheduleForRecurringOperation(context.Context, notificationsapp.RecurringOperationInfo, time.Time, []notificationsapp.OperationInfo) error {
	return nil
}

func (f *fakeReminderScheduler) ScheduleForLease(context.Context, notificationsapp.LeaseInfo) error {
	return nil
}

func (f *fakeReminderScheduler) EnsureRequiresActionReminder(context.Context, notificationsapp.LeaseInfo) error {
	return nil
}

func (f *fakeReminderScheduler) CancelByOperation(context.Context, uuid.UUID, uuid.UUID) error {
	f.cancels++
	return nil
}

func (f *fakeReminderScheduler) CancelOverdueReminderByOperation(context.Context, uuid.UUID, uuid.UUID) error {
	f.overdueCancels++
	return nil
}

func (f *fakeReminderScheduler) CancelByRecurringOperation(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (f *fakeReminderScheduler) CancelByLease(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (f *fakeReminderScheduler) HasReminderForOperationEvent(context.Context, uuid.UUID, uuid.UUID, notifdomain.EventType) (bool, error) {
	return false, nil
}

func (f *fakeReminderScheduler) ListByOperation(context.Context, uuid.UUID, uuid.UUID, notificationsapp.ListFilter) ([]notifdomain.Reminder, error) {
	return nil, nil
}

func (f *fakeReminderScheduler) ListByLease(context.Context, uuid.UUID, uuid.UUID, notificationsapp.ListFilter) ([]notifdomain.Reminder, error) {
	return nil, nil
}

func (f *fakeReminderScheduler) WithTx(_ transaction.Tx) notificationsapp.ReminderScheduler { return f }

var (
	_ auditapp.Recorder                  = (*fakeAuditRecorder)(nil)
	_ notificationsapp.ReminderScheduler = (*fakeReminderScheduler)(nil)
	_ sharedpolicy.Policy                = fakePolicy{}
)

// --- operations -------------------------------------------------------------

func TestPolicyEnforcement_GetOperation(t *testing.T) {
	ctx := context.Background()
	for _, tc := range policyRoleCases {
		t.Run(tc.name, func(t *testing.T) {
			actor, policy := policyActorAndPolicy(tc.role)
			opRepo := &fakeOperationRepo{ops: []domain.Operation{policyTestOperation()}}
			svc := newPolicyOperationService(policy, opRepo, newPolicyPropertyRepo(), nil, nil)

			_, err := svc.GetOperation(ctx, actor, policyOpID)
			if sharedpolicy.CanView(tc.role) {
				if err != nil {
					t.Fatalf("GetOperation: want success, got %v", err)
				}
				return
			}
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("GetOperation: want ErrNotFound, got %v", err)
			}
		})
	}
}

func TestPolicyEnforcement_ListOperationsByProperty(t *testing.T) {
	ctx := context.Background()
	for _, tc := range policyRoleCases {
		t.Run(tc.name, func(t *testing.T) {
			actor, policy := policyActorAndPolicy(tc.role)
			svc := newPolicyOperationService(policy, &fakeOperationRepo{}, newPolicyPropertyRepo(), nil, nil)

			_, err := svc.ListOperationsByProperty(ctx, actor, policyPropertyID, OperationFilter{})
			if sharedpolicy.CanView(tc.role) {
				if err != nil {
					t.Fatalf("ListOperationsByProperty: want success, got %v", err)
				}
				return
			}
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("ListOperationsByProperty: want ErrNotFound, got %v", err)
			}
		})
	}
}

func TestPolicyEnforcement_UpdateOperation(t *testing.T) {
	ctx := context.Background()
	for _, tc := range policyRoleCases {
		t.Run(tc.name, func(t *testing.T) {
			actor, policy := policyActorAndPolicy(tc.role)
			opRepo := &fakeOperationRepo{ops: []domain.Operation{policyTestOperation()}}
			svc := newPolicyOperationService(policy, opRepo, newPolicyPropertyRepo(), nil, nil)

			name := "renamed"
			updated, err := svc.UpdateOperation(ctx, actor, policyOpID, UpdateOperationCommand{Name: &name})
			switch {
			case sharedpolicy.CanEdit(tc.role):
				if err != nil {
					t.Fatalf("UpdateOperation: want success, got %v", err)
				}
				if updated.OwnerID != policyOwnerID {
					t.Errorf("OwnerID: want the data owner %s, got %s", policyOwnerID, updated.OwnerID)
				}
				if updated.Name != name {
					t.Errorf("Name: want %q, got %q", name, updated.Name)
				}
			case tc.role == sharedpolicy.RoleViewer:
				if !errors.Is(err, ErrForbidden) {
					t.Fatalf("UpdateOperation: want ErrForbidden, got %v", err)
				}
			default:
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("UpdateOperation: want ErrNotFound, got %v", err)
				}
			}
		})
	}
}

func TestPolicyEnforcement_DeleteOperation(t *testing.T) {
	ctx := context.Background()
	for _, tc := range policyRoleCases {
		t.Run(tc.name, func(t *testing.T) {
			actor, policy := policyActorAndPolicy(tc.role)
			opRepo := &fakeOperationRepo{ops: []domain.Operation{policyTestOperation()}}
			svc := newPolicyOperationService(policy, opRepo, newPolicyPropertyRepo(), nil, nil)

			err := svc.DeleteOperation(ctx, actor, policyOpID)
			switch {
			case sharedpolicy.CanEdit(tc.role):
				if err != nil {
					t.Fatalf("DeleteOperation: want success, got %v", err)
				}
				if opRepo.ops[0].DeletedAt == nil {
					t.Error("DeleteOperation: operation was not soft-deleted")
				}
			case tc.role == sharedpolicy.RoleViewer:
				if !errors.Is(err, ErrForbidden) {
					t.Fatalf("DeleteOperation: want ErrForbidden, got %v", err)
				}
			default:
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("DeleteOperation: want ErrNotFound, got %v", err)
				}
			}
		})
	}
}

func TestPolicyEnforcement_CreateOperation(t *testing.T) {
	ctx := context.Background()
	cmd := CreateOperationCommand{
		PropertyID:    policyPropertyID,
		Type:          "expense",
		CategoryID:    testCustomExpenseCategoryID,
		Name:          "test",
		AmountKopecks: 1000,
		OperationDate: date(2026, 6, 10),
	}
	for _, tc := range policyRoleCases {
		t.Run(tc.name, func(t *testing.T) {
			actor, policy := policyActorAndPolicy(tc.role)
			svc := newPolicyOperationService(policy, &fakeOperationRepo{}, newPolicyPropertyRepo(), nil, nil)

			created, err := svc.CreateOperation(ctx, actor, cmd)
			switch {
			case sharedpolicy.CanEdit(tc.role):
				if err != nil {
					t.Fatalf("CreateOperation: want success, got %v", err)
				}
				if created.OwnerID != policyOwnerID {
					t.Errorf("OwnerID: want the data owner %s, got %s", policyOwnerID, created.OwnerID)
				}
			case tc.role == sharedpolicy.RoleViewer:
				if !errors.Is(err, ErrForbidden) {
					t.Fatalf("CreateOperation: want ErrForbidden, got %v", err)
				}
			default:
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("CreateOperation: want ErrNotFound, got %v", err)
				}
			}
		})
	}
}

// --- leases -----------------------------------------------------------------

func policyTestLease() domain.Lease {
	return domain.Lease{
		ID:                policyLeaseID,
		OwnerID:           policyOwnerID,
		PropertyID:        policyPropertyID,
		StartDate:         date(2026, 1, 1),
		RentAmountKopecks: 10000,
		PaymentDay:        1,
	}
}

func TestPolicyEnforcement_GetLease(t *testing.T) {
	ctx := context.Background()
	for _, tc := range policyRoleCases {
		t.Run(tc.name, func(t *testing.T) {
			actor, policy := policyActorAndPolicy(tc.role)
			leaseRepo := &fakeLeaseRepo{leases: map[uuid.UUID]domain.Lease{policyLeaseID: policyTestLease()}}
			svc := newPolicyLeaseService(policy, leaseRepo, newPolicyPropertyRepo(), &fakeOperationRepo{}, &fakeRecurringOperationRepo{})

			_, err := svc.GetLease(ctx, actor, policyLeaseID)
			if sharedpolicy.CanView(tc.role) {
				if err != nil {
					t.Fatalf("GetLease: want success, got %v", err)
				}
				return
			}
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("GetLease: want ErrNotFound, got %v", err)
			}
		})
	}
}

func TestPolicyEnforcement_CreateLease(t *testing.T) {
	ctx := context.Background()
	cmd := CreateLeaseCommand{
		PropertyID:        policyPropertyID,
		StartDate:         date(2026, 6, 1),
		RentAmountKopecks: 10000,
		PaymentDay:        1,
	}
	for _, tc := range policyRoleCases {
		t.Run(tc.name, func(t *testing.T) {
			actor, policy := policyActorAndPolicy(tc.role)
			svc := newPolicyLeaseService(policy, &fakeLeaseRepo{}, newPolicyPropertyRepo(), &fakeOperationRepo{}, &fakeRecurringOperationRepo{})

			created, err := svc.CreateLease(ctx, actor, cmd)
			switch {
			case sharedpolicy.CanEdit(tc.role):
				if err != nil {
					t.Fatalf("CreateLease: want success, got %v", err)
				}
				if created.OwnerID != policyOwnerID {
					t.Errorf("OwnerID: want the data owner %s, got %s", policyOwnerID, created.OwnerID)
				}
			case tc.role == sharedpolicy.RoleViewer:
				if !errors.Is(err, ErrForbidden) {
					t.Fatalf("CreateLease: want ErrForbidden, got %v", err)
				}
			default:
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("CreateLease: want ErrNotFound, got %v", err)
				}
			}
		})
	}
}

// --- recurring operations -----------------------------------------------------

func TestPolicyEnforcement_GetRecurringOperation(t *testing.T) {
	ctx := context.Background()
	for _, tc := range policyRoleCases {
		t.Run(tc.name, func(t *testing.T) {
			actor, policy := policyActorAndPolicy(tc.role)
			recRepo := &fakeRecurringOperationRepo{recs: map[uuid.UUID]domain.RecurringOperation{
				policyRecID: newGuardTestRecurringOperation(policyRecID, policyOwnerID, policyPropertyID, domain.RecurringOperationStatusActive),
			}}
			svc := newPolicyRecurringService(policy, recRepo, &fakeOperationRepo{}, newPolicyPropertyRepo())

			_, err := svc.GetRecurringOperation(ctx, actor, policyRecID)
			if sharedpolicy.CanView(tc.role) {
				if err != nil {
					t.Fatalf("GetRecurringOperation: want success, got %v", err)
				}
				return
			}
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("GetRecurringOperation: want ErrNotFound, got %v", err)
			}
		})
	}
}

func TestPolicyEnforcement_CreateRecurringOperation(t *testing.T) {
	ctx := context.Background()
	cmd := CreateRecurringOperationCommand{
		PropertyID:    policyPropertyID,
		Type:          "expense",
		CategoryID:    testCustomExpenseCategoryID,
		Name:          "test",
		AmountKopecks: 1000,
		StartDate:     date(2026, 6, 1),
		PaymentDay:    1,
	}
	for _, tc := range policyRoleCases {
		t.Run(tc.name, func(t *testing.T) {
			actor, policy := policyActorAndPolicy(tc.role)
			svc := newPolicyRecurringService(policy, &fakeRecurringOperationRepo{}, &fakeOperationRepo{}, newPolicyPropertyRepo())

			created, err := svc.CreateRecurringOperation(ctx, actor, cmd)
			switch {
			case sharedpolicy.CanEdit(tc.role):
				if err != nil {
					t.Fatalf("CreateRecurringOperation: want success, got %v", err)
				}
				if created.OwnerID != policyOwnerID {
					t.Errorf("OwnerID: want the data owner %s, got %s", policyOwnerID, created.OwnerID)
				}
			case tc.role == sharedpolicy.RoleViewer:
				if !errors.Is(err, ErrForbidden) {
					t.Fatalf("CreateRecurringOperation: want ErrForbidden, got %v", err)
				}
			default:
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("CreateRecurringOperation: want ErrNotFound, got %v", err)
				}
			}
		})
	}
}

func TestPolicyEnforcement_DeleteRecurringOperation(t *testing.T) {
	ctx := context.Background()
	for _, tc := range policyRoleCases {
		t.Run(tc.name, func(t *testing.T) {
			actor, policy := policyActorAndPolicy(tc.role)
			recRepo := &fakeRecurringOperationRepo{recs: map[uuid.UUID]domain.RecurringOperation{
				policyRecID: newGuardTestRecurringOperation(policyRecID, policyOwnerID, policyPropertyID, domain.RecurringOperationStatusActive),
			}}
			svc := newPolicyRecurringService(policy, recRepo, &fakeOperationRepo{}, newPolicyPropertyRepo())

			err := svc.DeleteRecurringOperation(ctx, actor, policyRecID)
			switch {
			case sharedpolicy.CanEdit(tc.role):
				if err != nil {
					t.Fatalf("DeleteRecurringOperation: want success, got %v", err)
				}
				if recRepo.recs[policyRecID].DeletedAt == nil {
					t.Error("DeleteRecurringOperation: recurring operation was not soft-deleted")
				}
			case tc.role == sharedpolicy.RoleViewer:
				if !errors.Is(err, ErrForbidden) {
					t.Fatalf("DeleteRecurringOperation: want ErrForbidden, got %v", err)
				}
			default:
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("DeleteRecurringOperation: want ErrNotFound, got %v", err)
				}
			}
		})
	}
}

// --- export -----------------------------------------------------------------

func TestPolicyEnforcement_ExportProperty(t *testing.T) {
	ctx := context.Background()
	for _, tc := range policyRoleCases {
		t.Run(tc.name, func(t *testing.T) {
			actor, policy := policyActorAndPolicy(tc.role)
			prop := &exportPropertyRepo{row: ExportPropertyRow{
				Name:    "Тест",
				Type:    propdomain.PropertyTypeApartment,
				Address: "Москва, Тверская 1",
			}}
			svc := NewExportService(&exportOperationRepo{}, &exportLeaseRepo{}, prop, &exportContactRepo{}, policy, fakeClock{now: date(2026, 6, 15)}, slog.Default())

			_, err := svc.ExportProperty(ctx, actor, policyPropertyID)
			if sharedpolicy.CanView(tc.role) {
				if err != nil {
					t.Fatalf("ExportProperty: want success, got %v", err)
				}
				return
			}
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("ExportProperty: want ErrNotFound, got %v", err)
			}
		})
	}
}

// --- move operation -----------------------------------------------------------

// newMoveOperationService wires an OperationService for move tests with a
// seeded movable operation.
func newMoveOperationService(policy fakePolicy, propertyRepo *fakePropertyRepo, op domain.Operation, scheduler ReminderScheduler, audit auditapp.Recorder) (*OperationService, *fakeOperationRepo) {
	opRepo := &fakeOperationRepo{ops: []domain.Operation{op}}
	return newPolicyOperationService(policy, opRepo, propertyRepo, scheduler, audit), opRepo
}

func TestPolicyEnforcement_MoveOperation_RoleMatrix(t *testing.T) {
	ctx := context.Background()
	for _, tc := range policyRoleCases {
		t.Run(tc.name, func(t *testing.T) {
			actor, policy := policyActorAndPolicy(tc.role)
			svc, opRepo := newMoveOperationService(policy, newPolicyPropertyRepo(), policyTestOperation(), nil, nil)

			moved, err := svc.MoveOperation(ctx, actor, policyOpID, MoveOperationCommand{PropertyID: policyTargetID})
			switch {
			case sharedpolicy.CanEdit(tc.role):
				if err != nil {
					t.Fatalf("MoveOperation: want success, got %v", err)
				}
				if moved.PropertyID != policyTargetID {
					t.Errorf("PropertyID: want %s, got %s", policyTargetID, moved.PropertyID)
				}
				// The fake MoveToProperty matches on OwnerID == scope: a
				// successful move proves the repository was called with the
				// data owner, not the actor.
				if opRepo.ops[0].PropertyID != policyTargetID {
					t.Error("MoveOperation: stored operation was not moved")
				}
			case tc.role == sharedpolicy.RoleViewer:
				if !errors.Is(err, ErrForbidden) {
					t.Fatalf("MoveOperation: want ErrForbidden, got %v", err)
				}
			default:
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("MoveOperation: want ErrNotFound, got %v", err)
				}
			}
		})
	}
}

func TestPolicyEnforcement_MoveOperation_TargetRoleGate(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name       string
		targetRole sharedpolicy.Role
		wantErr    error
	}{
		{name: "viewer target", targetRole: sharedpolicy.RoleViewer, wantErr: ErrForbidden},
		{name: "none target", targetRole: sharedpolicy.RoleNone, wantErr: ErrNotFound},
		{name: "suspended target", targetRole: sharedpolicy.RoleSuspended, wantErr: ErrNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := fakePolicy{propertyRoles: map[[2]uuid.UUID]sharedpolicy.Role{
				{policyMemberID, policyPropertyID}: sharedpolicy.RoleFullAccess,
				{policyMemberID, policyTargetID}:   tc.targetRole,
			}}
			svc, _ := newMoveOperationService(policy, newPolicyPropertyRepo(), policyTestOperation(), nil, nil)

			_, err := svc.MoveOperation(ctx, policyMemberID, policyOpID, MoveOperationCommand{PropertyID: policyTargetID})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("MoveOperation: want %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestPolicyEnforcement_MoveOperation_Guards(t *testing.T) {
	ctx := context.Background()
	recurringChild := policyTestOperation()
	recurringChild.RecurringOperationID = policyRecID
	leaseLinked := policyTestOperation()
	leaseLinked.LeaseID = policyLeaseID

	archivedSourceRepo := newPolicyPropertyRepo()
	archivedSourceRepo.statuses[policyPropertyID] = "archived"
	archivedTargetRepo := newPolicyPropertyRepo()
	archivedTargetRepo.statuses[policyTargetID] = "archived"

	for _, tc := range []struct {
		name         string
		op           domain.Operation
		propertyRepo *fakePropertyRepo
		target       uuid.UUID
	}{
		{name: "recurring child rejected", op: recurringChild, propertyRepo: newPolicyPropertyRepo(), target: policyTargetID},
		{name: "lease linked rejected", op: leaseLinked, propertyRepo: newPolicyPropertyRepo(), target: policyTargetID},
		{name: "same target rejected", op: policyTestOperation(), propertyRepo: newPolicyPropertyRepo(), target: policyPropertyID},
		{name: "cross-owner target rejected", op: policyTestOperation(), propertyRepo: newPolicyPropertyRepo(), target: policyForeignID},
		{name: "archived source rejected", op: policyTestOperation(), propertyRepo: archivedSourceRepo, target: policyTargetID},
		{name: "archived target rejected", op: policyTestOperation(), propertyRepo: archivedTargetRepo, target: policyTargetID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newMoveOperationService(fakePolicy{}, tc.propertyRepo, tc.op, nil, nil)

			_, err := svc.MoveOperation(ctx, policyOwnerID, policyOpID, MoveOperationCommand{PropertyID: tc.target})
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("MoveOperation: want ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestPolicyEnforcement_MoveOperation_SideEffects(t *testing.T) {
	ctx := context.Background()
	offset := 2
	op := policyTestOperation()
	op.ReminderOffsetDays = &offset

	scheduler := &fakeReminderScheduler{}
	audit := &fakeAuditRecorder{}
	svc, _ := newMoveOperationService(fakePolicy{}, newPolicyPropertyRepo(), op, scheduler, audit)

	moved, err := svc.MoveOperation(ctx, policyOwnerID, policyOpID, MoveOperationCommand{PropertyID: policyTargetID})
	if err != nil {
		t.Fatalf("MoveOperation: %v", err)
	}

	if scheduler.cancels != 1 || scheduler.overdueCancels != 1 {
		t.Errorf("reminder cancellations: want 1+1, got %d+%d", scheduler.cancels, scheduler.overdueCancels)
	}
	if len(scheduler.scheduled) != 1 {
		t.Fatalf("scheduled reminders: want 1, got %d", len(scheduler.scheduled))
	}
	if got := scheduler.scheduled[0].PropertyID; got != moved.PropertyID {
		t.Errorf("scheduled reminder property: want %s, got %s", moved.PropertyID, got)
	}

	if len(audit.entries) != 1 {
		t.Fatalf("audit entries: want 1, got %d", len(audit.entries))
	}
	entry := audit.entries[0]
	if entry.Action != auditdomain.ActionOperationMoved {
		t.Errorf("audit action: want %q, got %q", auditdomain.ActionOperationMoved, entry.Action)
	}
	from, ok := entry.Context["from_property_id"].(*uuid.UUID)
	if !ok || from == nil || *from != policyPropertyID {
		t.Errorf("audit from_property_id: want %s, got %v", policyPropertyID, entry.Context["from_property_id"])
	}
	if entry.Context["to_property_id"] != policyTargetID {
		t.Errorf("audit to_property_id: want %s, got %v", policyTargetID, entry.Context["to_property_id"])
	}
}
