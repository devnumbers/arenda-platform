package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
)

// This file covers the archived-property rules of the membership use cases
// (issue #163): granting new access on an archived object is rejected, while
// managing existing members (role change, revoke) keeps working.

// fakeStatuses is an in-memory PropertyStatusResolver backed by a set of
// archived property ids; an absent property reads as not archived.
type fakeStatuses map[uuid.UUID]bool

func (f fakeStatuses) IsArchived(_ context.Context, propertyID uuid.UUID) (bool, error) {
	return f[propertyID], nil
}

var _ PropertyStatusResolver = fakeStatuses{}

// TestAccessService_ArchivedPropertyRejectsNewMembers verifies that AddMember
// on an archived property fails with ErrPropertyArchived (issue #163).
func TestAccessService_ArchivedPropertyRejectsNewMembers(t *testing.T) {
	owner := uuid.New()
	member := uuid.New()
	property := uuid.New()

	repo := newMemRepo()
	resolver := staticResolver{property: owner}
	policy := NewMembershipPolicy(resolver, repo)
	statuses := fakeStatuses{property: true}
	svc := NewAccessService(repo, resolver, statuses, stubLookup{}, policy, nil, nil, noopBeginner{}, auditapp.Noop{}, nil)

	if _, err := svc.AddMember(t.Context(), owner, property, member, domain.RoleViewer); !errors.Is(err, domain.ErrPropertyArchived) {
		t.Fatalf("AddMember on archived: expected ErrPropertyArchived, got %v", err)
	}
	if len(repo.rows) != 0 {
		t.Errorf("no membership must be created, got %d", len(repo.rows))
	}
}

// TestAccessService_ArchivedPropertyKeepsExistingMembersManageable verifies
// that ChangeMemberRole and RevokeMember are not blocked on an archived
// property (issue #163).
func TestAccessService_ArchivedPropertyKeepsExistingMembersManageable(t *testing.T) {
	owner := uuid.New()
	member := uuid.New()
	property := uuid.New()

	repo := newMemRepo()
	resolver := staticResolver{property: owner}
	policy := NewMembershipPolicy(resolver, repo)
	statuses := fakeStatuses{}
	svc := NewAccessService(repo, resolver, statuses, stubLookup{}, policy, nil, nil, noopBeginner{}, auditapp.Noop{}, nil)

	// The member is added while the property is active; the archive happens
	// afterwards.
	m, err := svc.AddMember(t.Context(), owner, property, member, domain.RoleViewer)
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	statuses[property] = true

	updated, err := svc.ChangeMemberRole(t.Context(), owner, property, m.ID, domain.RoleFullAccess)
	if err != nil {
		t.Fatalf("ChangeMemberRole on archived: %v", err)
	}
	if updated.Role != domain.RoleFullAccess {
		t.Errorf("role = %v, want full_access", updated.Role)
	}

	if err := svc.RevokeMember(t.Context(), owner, property, m.ID); err != nil {
		t.Fatalf("RevokeMember on archived: %v", err)
	}
	if _, err := repo.GetByID(t.Context(), m.ID, property); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("membership must be revoked, got %v", err)
	}
}
