package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
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
	t.Parallel()
	owner := uuid.Must(uuid.NewV7())
	member := uuid.Must(uuid.NewV7())
	property := uuid.Must(uuid.NewV7())

	repo := newMemRepo()
	resolver := staticResolver{property: owner}
	policy := NewMembershipPolicy(resolver, repo)
	statuses := fakeStatuses{property: true}
	svc := NewAccessService(repo, resolver, statuses, stubLookup{}, policy, nil,
		newTestFactory(repo, &memInvitationsRepo{}, auditapp.Noop{}), nil)

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
	t.Parallel()
	owner := uuid.Must(uuid.NewV7())
	member := uuid.Must(uuid.NewV7())
	property := uuid.Must(uuid.NewV7())

	repo := newMemRepo()
	resolver := staticResolver{property: owner}
	policy := NewMembershipPolicy(resolver, repo)
	statuses := fakeStatuses{}
	svc := NewAccessService(repo, resolver, statuses, stubLookup{}, policy, nil,
		newTestFactory(repo, &memInvitationsRepo{}, auditapp.Noop{}), nil)

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
	t.Parallel()
	namedID := uuid.Must(uuid.NewV7())
	phoneOnlyID := uuid.Must(uuid.NewV7())
	missingID := uuid.Must(uuid.NewV7())
	name, surname := "Ivan", "Petrov"

	lookup := fakeUserLookup{
		namedID:     {ID: namedID, Name: &name, Surname: &surname, Phone: "+79123456789", HasEmail: true},
		phoneOnlyID: {ID: phoneOnlyID, Phone: "+79123456789"},
	}
	svc := NewAccessService(newMemRepo(), staticResolver{}, nil, lookup, nil, nil,
		newTestFactory(newMemRepo(), &memInvitationsRepo{}, auditapp.Noop{}), nil)

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

// fakeAuditRecorder captures recorded audit entries (issue #166 follow-up:
// actor-role attribution tests).
type fakeAuditRecorder struct {
	entries []auditdomain.Entry
}

func (f *fakeAuditRecorder) Record(_ context.Context, entry auditdomain.Entry) error {
	f.entries = append(f.entries, entry)
	return nil
}

func (f *fakeAuditRecorder) WithTx(_ transaction.Tx) auditapp.Recorder { return f }

var _ auditapp.Recorder = (*fakeAuditRecorder)(nil)

// TestAccessService_LeavePropertyAuditActorRole verifies that a member's
// self-exit is attributed with the member's real role instead of being masked
// as the owner's own (issue #166 follow-up). Self-exit is the only write a
// viewer may perform, so it is where ActorRoleViewer enters the journal.
func TestAccessService_LeavePropertyAuditActorRole(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		role domain.Role
		want auditdomain.ActorRole
	}{
		{name: "viewer", role: domain.RoleViewer, want: auditdomain.ActorRoleViewer},
		{name: "full access", role: domain.RoleFullAccess, want: auditdomain.ActorRoleFullAccess},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			owner := uuid.Must(uuid.NewV7())
			member := uuid.Must(uuid.NewV7())
			property := uuid.Must(uuid.NewV7())

			repo := newMemRepo()
			resolver := staticResolver{property: owner}
			policy := NewMembershipPolicy(resolver, repo)
			audit := &fakeAuditRecorder{}
			svc := NewAccessService(repo, resolver, nil, stubLookup{}, policy, nil, newTestFactory(repo, &memInvitationsRepo{}, audit), nil)

			if _, err := svc.AddMember(t.Context(), owner, property, member, tc.role); err != nil {
				t.Fatalf("AddMember: %v", err)
			}
			// Drop the property_member.added entry; only the self-exit matters.
			audit.entries = nil

			if err := svc.LeaveProperty(t.Context(), member, property); err != nil {
				t.Fatalf("LeaveProperty: %v", err)
			}
			if len(audit.entries) != 1 {
				t.Fatalf("audit entries: want 1, got %d", len(audit.entries))
			}
			entry := audit.entries[0]
			if entry.Action != auditdomain.ActionPropertyMemberLeft {
				t.Errorf("audit action: want %q, got %q", auditdomain.ActionPropertyMemberLeft, entry.Action)
			}
			if entry.ActorRole != tc.want {
				t.Errorf("audit actor role: want %q, got %q", tc.want, entry.ActorRole)
			}
		})
	}
}

// TestAccessService_ManageAuditActorRole verifies that member-management
// actions performed by a full-access member are attributed with the full
// role, while the owner's own actions stay attributed as owner (issue #166
// follow-up).
func TestAccessService_ManageAuditActorRole(t *testing.T) {
	t.Parallel()
	owner := uuid.Must(uuid.NewV7())
	full := uuid.Must(uuid.NewV7())
	member := uuid.Must(uuid.NewV7())
	property := uuid.Must(uuid.NewV7())

	repo := newMemRepo()
	resolver := staticResolver{property: owner}
	policy := NewMembershipPolicy(resolver, repo)
	audit := &fakeAuditRecorder{}
	svc := NewAccessService(repo, resolver, nil, stubLookup{}, policy, nil, newTestFactory(repo, &memInvitationsRepo{}, audit), nil)

	// The owner grants full access; this entry must stay owner-attributed.
	if _, err := svc.AddMember(t.Context(), owner, property, full, domain.RoleFullAccess); err != nil {
		t.Fatalf("AddMember full: %v", err)
	}
	audit.entries = nil

	created, err := svc.AddMember(t.Context(), full, property, member, domain.RoleViewer)
	if err != nil {
		t.Fatalf("AddMember by full: %v", err)
	}
	if _, err := svc.ChangeMemberRole(t.Context(), full, property, created.ID, domain.RoleFullAccess); err != nil {
		t.Fatalf("ChangeMemberRole by full: %v", err)
	}
	if err := svc.RevokeMember(t.Context(), full, property, created.ID); err != nil {
		t.Fatalf("RevokeMember by full: %v", err)
	}

	wantActions := []auditdomain.Action{
		auditdomain.ActionPropertyMemberAdded,
		auditdomain.ActionPropertyMemberUpdated,
		auditdomain.ActionPropertyMemberRemoved,
	}
	if len(audit.entries) != len(wantActions) {
		t.Fatalf("audit entries: want %d, got %d", len(wantActions), len(audit.entries))
	}
	for i, want := range wantActions {
		entry := audit.entries[i]
		if entry.Action != want {
			t.Errorf("entry %d action: want %q, got %q", i, want, entry.Action)
		}
		if entry.ActorRole != auditdomain.ActorRoleFullAccess {
			t.Errorf("entry %d actor role: want %q, got %q", i, auditdomain.ActorRoleFullAccess, entry.ActorRole)
		}
	}
}
