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

// fakeUserLookup is a UserLookup keyed by user id (issue T11 DisplayName
// tests).
type fakeUserLookup map[uuid.UUID]MemberUser

func (f fakeUserLookup) GetByID(_ context.Context, id uuid.UUID) (MemberUser, error) {
	u, ok := f[id]
	if !ok {
		return MemberUser{}, errors.New("user not found")
	}
	return u, nil
}

func (f fakeUserLookup) GetByEmail(_ context.Context, _ string) (MemberUser, error) {
	return MemberUser{}, errors.New("user not found")
}

var _ UserLookup = fakeUserLookup{}

// TestAccessService_DisplayName verifies the public display name used by the
// sharing banner (issue T11): "Name Surname" when present, otherwise a masked
// phone — never an email or a raw phone. Lookup failures propagate.
func TestAccessService_DisplayName(t *testing.T) {
	namedID := uuid.New()
	phoneOnlyID := uuid.New()
	missingID := uuid.New()
	name, surname := "Ivan", "Petrov"

	lookup := fakeUserLookup{
		namedID:     {ID: namedID, Name: &name, Surname: &surname, Phone: "+79123456789", HasEmail: true},
		phoneOnlyID: {ID: phoneOnlyID, Phone: "+79123456789"},
	}
	svc := NewAccessService(newMemRepo(), staticResolver{}, nil, lookup, nil, nil, nil, noopBeginner{}, auditapp.Noop{}, nil)

	if got, err := svc.DisplayName(t.Context(), namedID); err != nil || got != "Ivan Petrov" {
		t.Errorf("named user: got %q, %v; want %q, nil", got, err, "Ivan Petrov")
	}
	if got, err := svc.DisplayName(t.Context(), phoneOnlyID); err != nil || got != "+7********89" {
		t.Errorf("phone-only user: got %q, %v; want %q, nil", got, err, "+7********89")
	}
	if _, err := svc.DisplayName(t.Context(), missingID); err == nil {
		t.Errorf("missing user: expected an error, got nil")
	}
}
