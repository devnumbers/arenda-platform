package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// fakeOwnerResolver maps property ids to owner ids.
type fakeOwnerResolver map[uuid.UUID]uuid.UUID

func (f fakeOwnerResolver) GetOwnerID(_ context.Context, propertyID uuid.UUID) (uuid.UUID, error) {
	if owner, ok := f[propertyID]; ok {
		return owner, nil
	}
	return uuid.Nil, domain.ErrMemberNotFound
}

// fakeMemberRepo holds lookup tables for policy tests: rolesByProperty maps
// (propertyID, userID) to a role for GetRole, memberships maps (propertyID,
// userID) to a full membership row for GetByPropertyAndUser, and maxByOwner
// maps (userID, ownerID) to the strongest role across the owner's properties
// for MaxRoleByOwner. The remaining MembershipRepository methods panic since
// they are not exercised here.
type fakeMemberRepo struct {
	rolesByProperty map[[2]uuid.UUID]domain.Role
	memberships     map[[2]uuid.UUID]domain.Membership
	maxByOwner      map[[2]uuid.UUID]domain.Role
}

func (f fakeMemberRepo) GetRole(_ context.Context, propertyID, userID uuid.UUID) (domain.Role, error) {
	if role, ok := f.rolesByProperty[[2]uuid.UUID{propertyID, userID}]; ok {
		return role, nil
	}
	return "", domain.ErrMemberNotFound
}

func (f fakeMemberRepo) GetByPropertyAndUser(_ context.Context, propertyID, userID uuid.UUID) (domain.Membership, error) {
	if m, ok := f.memberships[[2]uuid.UUID{propertyID, userID}]; ok {
		return m, nil
	}
	return domain.Membership{}, domain.ErrMemberNotFound
}

func (f fakeMemberRepo) MaxRoleByOwner(_ context.Context, userID, ownerID uuid.UUID) (domain.Role, error) {
	if role, ok := f.maxByOwner[[2]uuid.UUID{userID, ownerID}]; ok {
		return role, nil
	}
	return domain.Role(""), nil
}

func (f fakeMemberRepo) Create(context.Context, domain.Membership) (domain.Membership, error) {
	panic("not implemented")
}

func (f fakeMemberRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (domain.Membership, error) {
	panic("not implemented")
}

func (f fakeMemberRepo) ListByProperty(context.Context, uuid.UUID) ([]domain.Membership, error) {
	panic("not implemented")
}

func (f fakeMemberRepo) ListByUser(context.Context, uuid.UUID) ([]domain.Membership, error) {
	panic("not implemented")
}

func (f fakeMemberRepo) ListForRemovalByUser(context.Context, uuid.UUID, uuid.UUID) ([]domain.Membership, error) {
	panic("not implemented")
}

func (f fakeMemberRepo) UpdateRole(context.Context, uuid.UUID, uuid.UUID, domain.Role) (domain.Membership, error) {
	panic("not implemented")
}
func (f fakeMemberRepo) Delete(context.Context, uuid.UUID, uuid.UUID) error { panic("not implemented") }
func (f fakeMemberRepo) Suspend(context.Context, uuid.UUID, uuid.UUID) error {
	panic("not implemented")
}

func (f fakeMemberRepo) Reactivate(context.Context, uuid.UUID, uuid.UUID) (domain.Membership, error) {
	panic("not implemented")
}

func (f fakeMemberRepo) ListSuspendedByUser(context.Context, uuid.UUID) ([]domain.Membership, error) {
	panic("not implemented")
}

func (f fakeMemberRepo) CountActiveByUser(context.Context, uuid.UUID) (int, error) {
	panic("not implemented")
}

func (f fakeMemberRepo) CountSuspendedByUser(context.Context, uuid.UUID) (int, error) {
	panic("not implemented")
}

func (f fakeMemberRepo) ListActiveByPropertyOwner(context.Context, uuid.UUID) ([]domain.Membership, error) {
	panic("not implemented")
}

func (f fakeMemberRepo) ListActiveByUser(context.Context, uuid.UUID) ([]domain.Membership, error) {
	panic("not implemented")
}

func (f fakeMemberRepo) CreateWithStatus(context.Context, domain.Membership) (domain.Membership, error) {
	panic("not implemented")
}
func (f fakeMemberRepo) WithTx(transaction.Tx) MembershipRepository { return f }

// Compile-time check.
var _ MembershipRepository = fakeMemberRepo{}

// keep the time import for potential future timestamp assertions.
var _ = time.Now

func TestMembershipPolicy_RoleForProperty(t *testing.T) {
	t.Parallel()

	owner := uuid.Must(uuid.NewV7())
	member := uuid.Must(uuid.NewV7())
	suspended := uuid.Must(uuid.NewV7())
	stranger := uuid.Must(uuid.NewV7())
	property := uuid.Must(uuid.NewV7())
	missingProperty := uuid.Must(uuid.NewV7())

	resolver := fakeOwnerResolver{property: owner}
	repo := fakeMemberRepo{
		memberships: map[[2]uuid.UUID]domain.Membership{
			{property, member}: {
				PropertyID: property,
				UserID:     member,
				Role:       domain.RoleFullAccess,
				Status:     domain.MemberStatusActive,
			},
			{property, suspended}: {
				PropertyID: property,
				UserID:     suspended,
				Role:       domain.RoleViewer,
				Status:     domain.MemberStatusSuspended,
			},
		},
	}
	policy := NewMembershipPolicy(resolver, repo)

	tests := []struct {
		name     string
		actor    uuid.UUID
		prop     uuid.UUID
		wantRole sharedpolicy.Role
	}{
		{"owner is owner", owner, property, sharedpolicy.RoleOwner},
		{"full member", member, property, sharedpolicy.RoleFullAccess},
		{"suspended member is distinguishable", suspended, property, sharedpolicy.RoleSuspended},
		{"stranger gets none", stranger, property, sharedpolicy.RoleNone},
		{"missing property looks like none", owner, missingProperty, sharedpolicy.RoleNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := policy.RoleForProperty(context.Background(), tt.actor, tt.prop)
			if err != nil {
				t.Fatalf("RoleForProperty: unexpected error: %v", err)
			}
			if got != tt.wantRole {
				t.Errorf("RoleForProperty(%s) = %v, want %v", tt.name, got, tt.wantRole)
			}
		})
	}
}

func TestMembershipPolicy_ViewerMember(t *testing.T) {
	t.Parallel()

	owner := uuid.Must(uuid.NewV7())
	viewer := uuid.Must(uuid.NewV7())
	property := uuid.Must(uuid.NewV7())

	resolver := fakeOwnerResolver{property: owner}
	repo := fakeMemberRepo{
		memberships: map[[2]uuid.UUID]domain.Membership{
			{property, viewer}: {
				PropertyID: property,
				UserID:     viewer,
				Role:       domain.RoleViewer,
				Status:     domain.MemberStatusActive,
			},
		},
	}
	policy := NewMembershipPolicy(resolver, repo)

	got, err := policy.RoleForProperty(context.Background(), viewer, property)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != sharedpolicy.RoleViewer {
		t.Errorf("RoleForProperty(viewer) = %v, want viewer", got)
	}
}

func TestMembershipPolicy_Role_DerivedOwnerWide(t *testing.T) {
	t.Parallel()

	owner := uuid.Must(uuid.NewV7())
	fullMember := uuid.Must(uuid.NewV7())
	viewer := uuid.Must(uuid.NewV7())
	stranger := uuid.Must(uuid.NewV7())

	repo := fakeMemberRepo{
		maxByOwner: map[[2]uuid.UUID]domain.Role{
			{fullMember, owner}: domain.RoleFullAccess,
			{viewer, owner}:     domain.RoleViewer,
		},
	}
	policy := NewMembershipPolicy(fakeOwnerResolver{}, repo)

	tests := []struct {
		name     string
		actor    uuid.UUID
		wantRole sharedpolicy.Role
	}{
		{"owner is owner", owner, sharedpolicy.RoleOwner},
		{"full access member derives full access", fullMember, sharedpolicy.RoleFullAccess},
		{"viewer member derives viewer", viewer, sharedpolicy.RoleViewer},
		{"stranger gets none", stranger, sharedpolicy.RoleNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := policy.Role(context.Background(), tt.actor, owner)
			if err != nil {
				t.Fatalf("Role: unexpected error: %v", err)
			}
			if got != tt.wantRole {
				t.Errorf("Role(%s) = %v, want %v", tt.name, got, tt.wantRole)
			}
		})
	}
}
