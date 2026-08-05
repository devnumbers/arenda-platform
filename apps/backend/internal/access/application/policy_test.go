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

// fakeMemberRepo maps (propertyID, userID) to a role. Only GetRole is exercised
// by policy tests; the remaining MembershipRepository methods panic since they
// are not needed here.
type fakeMemberRepo map[[2]uuid.UUID]domain.Role

func (f fakeMemberRepo) GetRole(_ context.Context, propertyID, userID uuid.UUID) (domain.Role, error) {
	if role, ok := f[[2]uuid.UUID{propertyID, userID}]; ok {
		return role, nil
	}
	return "", domain.ErrMemberNotFound
}

func (f fakeMemberRepo) Create(context.Context, domain.Membership) (domain.Membership, error) {
	panic("not implemented")
}

func (f fakeMemberRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (domain.Membership, error) {
	panic("not implemented")
}

func (f fakeMemberRepo) GetByPropertyAndUser(context.Context, uuid.UUID, uuid.UUID) (domain.Membership, error) {
	panic("not implemented")
}

func (f fakeMemberRepo) ListByProperty(context.Context, uuid.UUID) ([]domain.Membership, error) {
	panic("not implemented")
}

func (f fakeMemberRepo) ListByUser(context.Context, uuid.UUID) ([]domain.Membership, error) {
	panic("not implemented")
}

func (f fakeMemberRepo) UpdateRole(context.Context, uuid.UUID, uuid.UUID, domain.Role) (domain.Membership, error) {
	panic("not implemented")
}
func (f fakeMemberRepo) Delete(context.Context, uuid.UUID, uuid.UUID) error { panic("not implemented") }
func (f fakeMemberRepo) WithTx(transaction.Tx) MembershipRepository         { return f }

// Compile-time check.
var _ MembershipRepository = fakeMemberRepo{}

// keep the time import for potential future timestamp assertions.
var _ = time.Now

func TestMembershipPolicy_RoleForProperty(t *testing.T) {
	t.Parallel()

	owner := uuid.New()
	member := uuid.New()
	stranger := uuid.New()
	property := uuid.New()
	missingProperty := uuid.New()

	resolver := fakeOwnerResolver{property: owner}
	repo := fakeMemberRepo{
		{property, member}: domain.RoleFullAccess,
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

	owner := uuid.New()
	viewer := uuid.New()
	property := uuid.New()

	resolver := fakeOwnerResolver{property: owner}
	repo := fakeMemberRepo{{property, viewer}: domain.RoleViewer}
	policy := NewMembershipPolicy(resolver, repo)

	got, err := policy.RoleForProperty(context.Background(), viewer, property)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != sharedpolicy.RoleViewer {
		t.Errorf("RoleForProperty(viewer) = %v, want viewer", got)
	}
}

func TestMembershipPolicy_Role_OwnerWideIsOwnerOnly(t *testing.T) {
	t.Parallel()

	owner := uuid.New()
	other := uuid.New()
	policy := NewMembershipPolicy(fakeOwnerResolver{}, fakeMemberRepo{})

	if got, err := policy.Role(context.Background(), owner, owner); err != nil || got != sharedpolicy.RoleOwner {
		t.Errorf("Role(owner, owner) = %v, %v, want owner, nil", got, err)
	}
	if got, err := policy.Role(context.Background(), other, owner); err != nil || got != sharedpolicy.RoleNone {
		t.Errorf("Role(other, owner) = %v, %v, want none, nil", got, err)
	}
}
