package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// memRepo is an in-memory MembershipRepository used by service-level tests. It
// avoids a database dependency while still exercising the allow/deny path
// through the real AccessService + MembershipPolicy.
type memRepo struct {
	rows      []domain.Membership
	byPropUsr map[[2]uuid.UUID]int
}

func newMemRepo() *memRepo {
	return &memRepo{byPropUsr: map[[2]uuid.UUID]int{}}
}

func (r *memRepo) reindex() {
	r.byPropUsr = map[[2]uuid.UUID]int{}
	for i, m := range r.rows {
		r.byPropUsr[[2]uuid.UUID{m.PropertyID, m.UserID}] = i
	}
}

func (r *memRepo) Create(_ context.Context, m domain.Membership) (domain.Membership, error) {
	if _, ok := r.byPropUsr[[2]uuid.UUID{m.PropertyID, m.UserID}]; ok {
		return domain.Membership{}, domain.ErrMemberAlreadyExists
	}
	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()
	r.rows = append(r.rows, m)
	r.reindex()
	return m, nil
}

func (r *memRepo) GetByID(_ context.Context, id, propertyID uuid.UUID) (domain.Membership, error) {
	for _, m := range r.rows {
		if m.ID == id && m.PropertyID == propertyID {
			return m, nil
		}
	}
	return domain.Membership{}, domain.ErrMemberNotFound
}

func (r *memRepo) GetByPropertyAndUser(_ context.Context, propertyID, userID uuid.UUID) (domain.Membership, error) {
	if i, ok := r.byPropUsr[[2]uuid.UUID{propertyID, userID}]; ok {
		return r.rows[i], nil
	}
	return domain.Membership{}, domain.ErrMemberNotFound
}

func (r *memRepo) GetRole(_ context.Context, propertyID, userID uuid.UUID) (domain.Role, error) {
	if i, ok := r.byPropUsr[[2]uuid.UUID{propertyID, userID}]; ok {
		return r.rows[i].Role, nil
	}
	return "", domain.ErrMemberNotFound
}

func (r *memRepo) ListByProperty(_ context.Context, propertyID uuid.UUID) ([]domain.Membership, error) {
	var out []domain.Membership
	for _, m := range r.rows {
		if m.PropertyID == propertyID {
			out = append(out, m)
		}
	}
	return out, nil
}

func (r *memRepo) ListByUser(_ context.Context, userID uuid.UUID) ([]domain.Membership, error) {
	var out []domain.Membership
	for _, m := range r.rows {
		if m.UserID == userID {
			out = append(out, m)
		}
	}
	return out, nil
}

// MaxRoleByOwner is not exercised by the service integration tests; owner-wide
// derived access is covered by policy tests.
func (r *memRepo) MaxRoleByOwner(_ context.Context, _, _ uuid.UUID) (domain.Role, error) {
	return domain.Role(""), nil
}

func (r *memRepo) UpdateRole(_ context.Context, id, propertyID uuid.UUID, role domain.Role) (domain.Membership, error) {
	for i := range r.rows {
		if r.rows[i].ID == id && r.rows[i].PropertyID == propertyID {
			r.rows[i].Role = role
			return r.rows[i], nil
		}
	}
	return domain.Membership{}, domain.ErrMemberNotFound
}

func (r *memRepo) Delete(_ context.Context, id, propertyID uuid.UUID) error {
	for i := range r.rows {
		if r.rows[i].ID == id && r.rows[i].PropertyID == propertyID {
			r.rows = append(r.rows[:i], r.rows[i+1:]...)
			r.reindex()
			return nil
		}
	}
	return nil
}
func (r *memRepo) WithTx(transaction.Tx) MembershipRepository { return r }

// staticResolver resolves a fixed property→owner mapping.
type staticResolver map[uuid.UUID]uuid.UUID

func (s staticResolver) GetOwnerID(_ context.Context, propertyID uuid.UUID) (uuid.UUID, error) {
	if owner, ok := s[propertyID]; ok {
		return owner, nil
	}
	return uuid.Nil, domain.ErrMemberNotFound
}

// stubLookup returns a minimal display user for every id.
type stubLookup struct{}

func (stubLookup) GetByID(_ context.Context, id uuid.UUID) (MemberUser, error) {
	name := "Member"
	return MemberUser{ID: id, Name: &name}, nil
}

func (stubLookup) GetByEmail(_ context.Context, _ string) (MemberUser, error) {
	return MemberUser{}, errors.New("not found")
}

// noopBeginner is a txBeginner whose transactions are immediate no-ops; the
// in-memory repo ignores transactions.
type noopBeginner struct{}

func (noopBeginner) Begin(context.Context) (transaction.Tx, error) { return noopTx{}, nil }

type noopTx struct{}

func (noopTx) Commit(context.Context) error   { return nil }
func (noopTx) Rollback(context.Context) error { return nil }

var (
	_ transaction.Tx        = noopTx{}
	_ MembershipRepository  = (*memRepo)(nil)
	_ PropertyOwnerResolver = staticResolver{}
)

// TestAccessService_AllowDenyPath exercises the core T3 access path through the
// real AccessService + MembershipPolicy: a full member is added and can be
// listed; a stranger is denied; the owner is always listed first and cannot be
// re-added; a duplicate add is rejected (issue #156).
func TestAccessService_AllowDenyPath(t *testing.T) {
	owner := uuid.New()
	member := uuid.New()
	property := uuid.New()

	repo := newMemRepo()
	resolver := staticResolver{property: owner}
	policy := NewMembershipPolicy(resolver, repo)
	svc := NewAccessService(repo, resolver, stubLookup{}, policy, noopBeginner{}, auditapp.Noop{}, nil)

	// Owner adds a full member.
	m, err := svc.AddMember(context.Background(), owner, property, member, domain.RoleFullAccess)
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if m.Role != domain.RoleFullAccess {
		t.Errorf("role = %v, want full_access", m.Role)
	}

	// Owner appears first in the list, members after.
	members, err := svc.ListMembers(context.Background(), owner, property)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 2 || !members[0].IsOwner {
		t.Fatalf("expected owner first then 1 member, got %+v", members)
	}

	// Stranger is denied listing (privacy → not found).
	if _, err := svc.ListMembers(context.Background(), uuid.New(), property); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("stranger ListMembers: expected ErrMemberNotFound, got %v", err)
	}

	// Duplicate add is rejected.
	if _, err := svc.AddMember(context.Background(), owner, property, member, domain.RoleViewer); !errors.Is(err, domain.ErrMemberAlreadyExists) {
		t.Errorf("duplicate AddMember: expected ErrMemberAlreadyExists, got %v", err)
	}

	// Adding the owner as a member is rejected.
	if _, err := svc.AddMember(context.Background(), owner, property, owner, domain.RoleViewer); !errors.Is(err, domain.ErrCannotAddOwner) {
		t.Errorf("add owner: expected ErrCannotAddOwner, got %v", err)
	}

	// Self-add is rejected.
	if _, err := svc.AddMember(context.Background(), member, property, member, domain.RoleViewer); !errors.Is(err, domain.ErrCannotAddSelf) {
		t.Errorf("self add: expected ErrCannotAddSelf, got %v", err)
	}

	// Policy resolves roles correctly for allow/deny reasoning.
	roleFull, _ := policy.RoleForProperty(context.Background(), member, property)
	if roleFull != sharedpolicy.RoleFullAccess {
		t.Errorf("member role = %v, want full_access", roleFull)
	}
	roleNone, _ := policy.RoleForProperty(context.Background(), uuid.New(), property)
	if roleNone != sharedpolicy.RoleNone {
		t.Errorf("stranger role = %v, want none", roleNone)
	}
}

// TestAccessService_LeaveProperty verifies self-exit: a member can leave, the
// owner cannot (issue #156).
func TestAccessService_LeaveProperty(t *testing.T) {
	owner := uuid.New()
	member := uuid.New()
	property := uuid.New()

	repo := newMemRepo()
	resolver := staticResolver{property: owner}
	policy := NewMembershipPolicy(resolver, repo)
	svc := NewAccessService(repo, resolver, stubLookup{}, policy, noopBeginner{}, auditapp.Noop{}, nil)

	if _, err := svc.AddMember(context.Background(), owner, property, member, domain.RoleViewer); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	if err := svc.LeaveProperty(context.Background(), member, property); err != nil {
		t.Fatalf("LeaveProperty: %v", err)
	}

	// Member no longer listed.
	members, _ := svc.ListMembers(context.Background(), owner, property)
	for _, m := range members {
		if m.UserID == member {
			t.Errorf("member still present after leave")
		}
	}

	// Owner cannot leave.
	if err := svc.LeaveProperty(context.Background(), owner, property); !errors.Is(err, domain.ErrCannotLeaveOwnProperty) {
		t.Errorf("owner leave: expected ErrCannotLeaveOwnProperty, got %v", err)
	}
}
