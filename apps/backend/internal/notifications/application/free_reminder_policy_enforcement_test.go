package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// This file is the role-matrix cover for the T3 policy enforcement in the
// free-reminder use cases (issue #166): every public write/read case is
// exercised as owner, full-access member, viewer, outsider (none) and
// suspended member. Reads map any non-view role to ErrNotFound; writes map
// none/suspended to ErrNotFound and viewer to ErrForbidden. Repository calls
// always use the data owner (scope), never the actor.

var (
	policyOwnerID    = uuid.MustParse("aaaaaaaa-0000-0000-0000-000000000001")
	policyMemberID   = uuid.MustParse("aaaaaaaa-0000-0000-0000-000000000002")
	policyPropertyID = uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001")
	policyReminderID = uuid.MustParse("cccccccc-0000-0000-0000-000000000001")
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
// other roles act as a member with the role seeded on the test property.
func policyActorAndPolicy(role sharedpolicy.Role) (uuid.UUID, fakePolicy) {
	if role == sharedpolicy.RoleOwner {
		return policyOwnerID, fakePolicy{}
	}
	return policyMemberID, fakePolicy{propertyRoles: map[[2]uuid.UUID]sharedpolicy.Role{
		{policyMemberID, policyPropertyID}: role,
	}}
}

// fakePolicy maps (actor, scope) -> role and (actor, property) -> role.
// RoleForProperty defaults to RoleOwner so the owner case needs no seeding.
// Implements sharedpolicy.Policy.
type fakePolicy struct {
	roles         map[[2]uuid.UUID]sharedpolicy.Role
	propertyRoles map[[2]uuid.UUID]sharedpolicy.Role
}

func (f fakePolicy) Role(_ context.Context, actor, scope uuid.UUID) (sharedpolicy.Role, error) {
	if actor == scope {
		return sharedpolicy.RoleOwner, nil
	}
	if r, ok := f.roles[[2]uuid.UUID{actor, scope}]; ok {
		return r, nil
	}
	return sharedpolicy.RoleNone, nil
}

func (f fakePolicy) RoleForProperty(_ context.Context, actor, propertyID uuid.UUID) (sharedpolicy.Role, error) {
	if r, ok := f.propertyRoles[[2]uuid.UUID{actor, propertyID}]; ok {
		return r, nil
	}
	return sharedpolicy.RoleOwner, nil
}

var _ sharedpolicy.Policy = fakePolicy{}

// fakeFreeReminderRepo is an in-memory FreeReminderRepository that records the
// scope of every scoped call so tests can prove repository calls use the data
// owner, not the actor.
type fakeFreeReminderRepo struct {
	templates map[uuid.UUID]domain.FreeReminder

	created          []domain.FreeReminder
	updated          []domain.FreeReminder
	deleteCalls      [][2]uuid.UUID // (scope, id)
	cancelCalls      [][2]uuid.UUID // (scope, freeReminderID)
	listPropertyCall [2]uuid.UUID   // (scope, propertyID)
}

func (r *fakeFreeReminderRepo) Create(_ context.Context, fr domain.FreeReminder) (domain.FreeReminder, error) {
	r.created = append(r.created, fr)
	return fr, nil
}

func (r *fakeFreeReminderRepo) GetByID(_ context.Context, id, scope uuid.UUID) (domain.FreeReminder, error) {
	fr, ok := r.templates[id]
	if !ok || fr.OwnerID != scope {
		return domain.FreeReminder{}, ErrNotFound
	}
	return fr, nil
}

func (r *fakeFreeReminderRepo) GetByIDUnscoped(_ context.Context, id uuid.UUID) (domain.FreeReminder, error) {
	fr, ok := r.templates[id]
	if !ok {
		return domain.FreeReminder{}, ErrNotFound
	}
	return fr, nil
}

func (r *fakeFreeReminderRepo) Update(_ context.Context, fr domain.FreeReminder) (domain.FreeReminder, error) {
	r.updated = append(r.updated, fr)
	return fr, nil
}

func (r *fakeFreeReminderRepo) Delete(_ context.Context, scope, id uuid.UUID) error {
	r.deleteCalls = append(r.deleteCalls, [2]uuid.UUID{scope, id})
	return nil
}

func (r *fakeFreeReminderRepo) ListByOwner(context.Context, uuid.UUID, int, int, []uuid.UUID) ([]domain.FreeReminder, error) {
	return nil, nil
}

func (r *fakeFreeReminderRepo) ListByProperty(_ context.Context, scope, propertyID uuid.UUID, _ int) ([]domain.FreeReminder, error) {
	r.listPropertyCall = [2]uuid.UUID{scope, propertyID}
	return nil, nil
}

func (r *fakeFreeReminderRepo) ListTemplatesByOwner(context.Context, uuid.UUID, []uuid.UUID) ([]domain.FreeReminderTemplate, error) {
	return nil, nil
}

func (r *fakeFreeReminderRepo) SaveFreeReminder(context.Context, domain.Reminder) error { return nil }

func (r *fakeFreeReminderRepo) CancelRemindersByFreeReminderID(_ context.Context, scope, freeReminderID uuid.UUID) error {
	r.cancelCalls = append(r.cancelCalls, [2]uuid.UUID{scope, freeReminderID})
	return nil
}

func (r *fakeFreeReminderRepo) WithTx(transaction.Tx) FreeReminderRepository { return r }

var _ FreeReminderRepository = (*fakeFreeReminderRepo)(nil)

// fakeBeginner returns a no-op transaction; the fake repo ignores transaction
// binding anyway.
type fakeBeginner struct{}

type fakeTx struct{}

func (fakeTx) Commit(context.Context) error   { return nil }
func (fakeTx) Rollback(context.Context) error { return nil }

func (fakeBeginner) Begin(context.Context) (transaction.Tx, error) { return fakeTx{}, nil }

type fakeClock struct{ now time.Time }

func (c fakeClock) Now() time.Time { return c.now }

type fakeTzResolver struct{}

func (fakeTzResolver) Resolve(context.Context, uuid.UUID) (*time.Location, error) {
	return time.UTC, nil
}

var policyClock = fakeClock{now: time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)}

func newPolicyFreeReminderService(policy fakePolicy, repo *fakeFreeReminderRepo) *FreeReminderService {
	return NewFreeReminderService(repo, fakeBeginner{}, policyClock, fakeTzResolver{}, policy)
}

func policyTestFreeReminder() domain.FreeReminder {
	return domain.FreeReminder{
		ID:          policyReminderID,
		OwnerID:     policyOwnerID,
		PropertyID:  policyPropertyID,
		Title:       "test",
		TriggerAt:   policyClock.now.Add(24 * time.Hour),
		Periodicity: domain.PeriodicityOnce,
		CreatedAt:   policyClock.now,
		UpdatedAt:   policyClock.now,
	}
}

func TestPolicyEnforcement_CreateFreeReminder(t *testing.T) {
	ctx := t.Context()
	input := CreateFreeReminderInput{
		PropertyID:  policyPropertyID,
		Title:       "test",
		TriggerAt:   policyClock.now.Add(24 * time.Hour),
		Periodicity: domain.PeriodicityOnce,
	}
	for _, tc := range policyRoleCases {
		t.Run(tc.name, func(t *testing.T) {
			actor, policy := policyActorAndPolicy(tc.role)
			repo := &fakeFreeReminderRepo{}
			svc := newPolicyFreeReminderService(policy, repo)

			created, err := svc.Create(ctx, actor, policyOwnerID, input)
			switch {
			case sharedpolicy.CanEdit(tc.role):
				if err != nil {
					t.Fatalf("Create: want success, got %v", err)
				}
				if created.OwnerID != policyOwnerID {
					t.Errorf("OwnerID: want the data owner %s, got %s", policyOwnerID, created.OwnerID)
				}
				if len(repo.created) != 1 || repo.created[0].OwnerID != policyOwnerID {
					t.Errorf("repo Create: want one template on the owner scope, got %+v", repo.created)
				}
			case tc.role == sharedpolicy.RoleViewer:
				if !errors.Is(err, ErrForbidden) {
					t.Fatalf("Create: want ErrForbidden, got %v", err)
				}
			default:
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("Create: want ErrNotFound, got %v", err)
				}
			}
		})
	}
}

func TestPolicyEnforcement_GetFreeReminder(t *testing.T) {
	ctx := t.Context()
	for _, tc := range policyRoleCases {
		t.Run(tc.name, func(t *testing.T) {
			actor, policy := policyActorAndPolicy(tc.role)
			repo := &fakeFreeReminderRepo{templates: map[uuid.UUID]domain.FreeReminder{
				policyReminderID: policyTestFreeReminder(),
			}}
			svc := newPolicyFreeReminderService(policy, repo)

			_, err := svc.Get(ctx, actor, policyReminderID)
			if sharedpolicy.CanView(tc.role) {
				if err != nil {
					t.Fatalf("Get: want success, got %v", err)
				}
				return
			}
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("Get: want ErrNotFound, got %v", err)
			}
		})
	}
}

func TestPolicyEnforcement_UpdateFreeReminder(t *testing.T) {
	ctx := t.Context()
	for _, tc := range policyRoleCases {
		t.Run(tc.name, func(t *testing.T) {
			actor, policy := policyActorAndPolicy(tc.role)
			repo := &fakeFreeReminderRepo{templates: map[uuid.UUID]domain.FreeReminder{
				policyReminderID: policyTestFreeReminder(),
			}}
			svc := newPolicyFreeReminderService(policy, repo)

			title := "renamed"
			updated, err := svc.Update(ctx, actor, policyReminderID, UpdateFreeReminderInput{Title: &title})
			switch {
			case sharedpolicy.CanEdit(tc.role):
				if err != nil {
					t.Fatalf("Update: want success, got %v", err)
				}
				if updated.OwnerID != policyOwnerID {
					t.Errorf("OwnerID: want the data owner %s, got %s", policyOwnerID, updated.OwnerID)
				}
				if len(repo.cancelCalls) != 1 || repo.cancelCalls[0] != [2]uuid.UUID{policyOwnerID, policyReminderID} {
					t.Errorf("CancelRemindersByFreeReminderID: want one call on the owner scope, got %+v", repo.cancelCalls)
				}
			case tc.role == sharedpolicy.RoleViewer:
				if !errors.Is(err, ErrForbidden) {
					t.Fatalf("Update: want ErrForbidden, got %v", err)
				}
			default:
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("Update: want ErrNotFound, got %v", err)
				}
			}
		})
	}
}

func TestPolicyEnforcement_DeleteFreeReminder(t *testing.T) {
	ctx := t.Context()
	for _, tc := range policyRoleCases {
		t.Run(tc.name, func(t *testing.T) {
			actor, policy := policyActorAndPolicy(tc.role)
			repo := &fakeFreeReminderRepo{templates: map[uuid.UUID]domain.FreeReminder{
				policyReminderID: policyTestFreeReminder(),
			}}
			svc := newPolicyFreeReminderService(policy, repo)

			err := svc.Delete(ctx, actor, policyReminderID)
			switch {
			case sharedpolicy.CanEdit(tc.role):
				if err != nil {
					t.Fatalf("Delete: want success, got %v", err)
				}
				if len(repo.deleteCalls) != 1 || repo.deleteCalls[0] != [2]uuid.UUID{policyOwnerID, policyReminderID} {
					t.Errorf("repo Delete: want one call on the owner scope, got %+v", repo.deleteCalls)
				}
			case tc.role == sharedpolicy.RoleViewer:
				if !errors.Is(err, ErrForbidden) {
					t.Fatalf("Delete: want ErrForbidden, got %v", err)
				}
			default:
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("Delete: want ErrNotFound, got %v", err)
				}
			}
		})
	}
}

func TestPolicyEnforcement_ListFreeRemindersByProperty(t *testing.T) {
	ctx := t.Context()
	repo := &fakeFreeReminderRepo{}
	svc := newPolicyFreeReminderService(fakePolicy{}, repo)

	// The transport layer resolves the property's data owner and passes it as
	// scope; the service must thread it into the repository untouched.
	if _, err := svc.ListByProperty(ctx, policyOwnerID, policyPropertyID, 3); err != nil {
		t.Fatalf("ListByProperty: %v", err)
	}
	if repo.listPropertyCall != [2]uuid.UUID{policyOwnerID, policyPropertyID} {
		t.Errorf("repo ListByProperty: want the owner scope, got %+v", repo.listPropertyCall)
	}
}
