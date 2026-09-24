package application

import (
	"bytes"
	"context"
	"errors"
	"slices"
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
	// Owners maps property_id → owner_id, mirroring the properties table join
	// used by the SQL implementation of ListActiveByPropertyOwner. It is
	// populated by SetOwner in tests that exercise owner-wide queries.
	owners map[uuid.UUID]uuid.UUID
}

func newMemRepo() *memRepo {
	return &memRepo{
		byPropUsr: map[[2]uuid.UUID]int{},
		owners:    map[uuid.UUID]uuid.UUID{},
	}
}

// SetOwner records the owner of a property for owner-wide in-memory queries.
func (r *memRepo) SetOwner(propertyID, ownerID uuid.UUID) {
	r.owners[propertyID] = ownerID
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
	if m.Status == "" {
		m.Status = domain.MemberStatusActive
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
			// The production table maintains updated_at by trigger (000094);
			// the in-memory double mirrors it — the role-change event stamps
			// its dedup key with this instant (тикет #830).
			r.rows[i].UpdatedAt = time.Now()
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

func (r *memRepo) Suspend(_ context.Context, id, propertyID uuid.UUID) error {
	for i := range r.rows {
		if r.rows[i].ID == id && r.rows[i].PropertyID == propertyID {
			now := time.Now()
			r.rows[i].Status = domain.MemberStatusSuspended
			r.rows[i].SuspendedAt = &now
			return nil
		}
	}
	return nil
}

func (r *memRepo) Reactivate(_ context.Context, id, propertyID uuid.UUID) (domain.Membership, error) {
	for i := range r.rows {
		if r.rows[i].ID == id && r.rows[i].PropertyID == propertyID {
			r.rows[i].Status = domain.MemberStatusActive
			r.rows[i].SuspendedAt = nil
			return r.rows[i], nil
		}
	}
	return domain.Membership{}, domain.ErrMemberNotFound
}

func (r *memRepo) ListSuspendedByUser(_ context.Context, userID uuid.UUID) ([]domain.Membership, error) {
	var out []domain.Membership
	for _, m := range r.rows {
		if m.UserID == userID && m.Status == domain.MemberStatusSuspended {
			out = append(out, m)
		}
	}
	// FIFO: oldest suspended_at first, then most recently updated.
	slices.SortFunc(out, func(a, b domain.Membership) int {
		if c := cmpNullTimeAsc(a.SuspendedAt, b.SuspendedAt); c != 0 {
			return c
		}
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})
	return out, nil
}

func (r *memRepo) CountActiveByUser(_ context.Context, userID uuid.UUID) (int, error) {
	count := 0
	for _, m := range r.rows {
		if m.UserID == userID && m.Status == domain.MemberStatusActive {
			count++
		}
	}
	return count, nil
}

func (r *memRepo) ListActiveByPropertyOwner(_ context.Context, ownerID uuid.UUID) ([]domain.Membership, error) {
	var out []domain.Membership
	for _, m := range r.rows {
		if m.Status != domain.MemberStatusActive {
			continue
		}
		owner, ok := r.owners[m.PropertyID]
		if !ok || owner != ownerID {
			continue
		}
		out = append(out, m)
	}
	// Match SQL ordering: by user_id, then updated_at DESC.
	slices.SortFunc(out, func(a, b domain.Membership) int {
		if c := bytes.Compare(a.UserID[:], b.UserID[:]); c != 0 {
			return c
		}
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})
	return out, nil
}

func (r *memRepo) ListActiveByUser(_ context.Context, userID uuid.UUID) ([]domain.Membership, error) {
	var out []domain.Membership
	for _, m := range r.rows {
		if m.UserID != userID || m.Status != domain.MemberStatusActive {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

// ListForRemovalByUser mirrors the SQL removal scope (issue #694): the
// person's memberships (any status, archived included) on properties the
// actor owns or manages as an active full_access member.
func (r *memRepo) ListForRemovalByUser(_ context.Context, personID, actorID uuid.UUID) ([]domain.Membership, error) {
	var out []domain.Membership
	for _, m := range r.rows {
		if m.UserID == personID && r.inManageScope(m.PropertyID, actorID) {
			out = append(out, m)
		}
	}
	slices.SortFunc(out, func(a, b domain.Membership) int {
		return a.CreatedAt.Compare(b.CreatedAt)
	})
	return out, nil
}

// inManageScope is the in-memory twin of the SQL manage-scope predicate: the
// actor owns the property or holds an active full_access membership on it.
func (r *memRepo) inManageScope(propertyID, actorID uuid.UUID) bool {
	if r.owners[propertyID] == actorID {
		return true
	}
	for _, m := range r.rows {
		if m.PropertyID == propertyID && m.UserID == actorID &&
			m.Status == domain.MemberStatusActive && m.Role == domain.RoleFullAccess {
			return true
		}
	}
	return false
}

func (r *memRepo) CreateWithStatus(_ context.Context, m domain.Membership) (domain.Membership, error) {
	if m.Status == domain.MemberStatusActive {
		if _, ok := r.byPropUsr[[2]uuid.UUID{m.PropertyID, m.UserID}]; ok {
			return domain.Membership{}, domain.ErrMemberAlreadyExists
		}
	}
	if m.Status == "" {
		m.Status = domain.MemberStatusActive
	}
	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()
	if m.IsSuspended() && m.SuspendedAt == nil {
		now := time.Now()
		m.SuspendedAt = &now
	}
	r.rows = append(r.rows, m)
	r.reindex()
	return m, nil
}

func (r *memRepo) WithTx(transaction.Tx) MembershipRepository { return r }

// cmpNullTimeAsc compares two nullable timestamps the way SQL's
// "ASC NULLS LAST" would: nil sorts after all non-nil values.
func cmpNullTimeAsc(a, b *time.Time) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return 1
	case b == nil:
		return -1
	default:
		return a.Compare(*b)
	}
}

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
	t.Parallel()
	owner := uuid.Must(uuid.NewV7())
	member := uuid.Must(uuid.NewV7())
	property := uuid.Must(uuid.NewV7())

	repo := newMemRepo()
	resolver := staticResolver{property: owner}
	policy := NewMembershipPolicy(resolver, repo)
	svc := NewAccessService(repo, resolver, nil, stubLookup{}, policy, nil, nil,
		newTestFactory(repo, &memInvitationsRepo{}, auditapp.Noop{}), nil)

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
	if _, err := svc.ListMembers(context.Background(), uuid.Must(uuid.NewV7()), property); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("stranger ListMembers: expected ErrMemberNotFound, got %v", err)
	}

	// Duplicate add is rejected.
	if _, err := svc.AddMember(context.Background(), owner, property, member,
		domain.RoleViewer); !errors.Is(err, domain.ErrMemberAlreadyExists) {
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
	roleFull, err := policy.RoleForProperty(context.Background(), member, property)
	if err != nil {
		t.Fatalf("RoleForProperty(member): %v", err)
	}
	if roleFull != sharedpolicy.RoleFullAccess {
		t.Errorf("member role = %v, want full_access", roleFull)
	}
	roleNone, err := policy.RoleForProperty(context.Background(), uuid.Must(uuid.NewV7()), property)
	if err != nil {
		t.Fatalf("RoleForProperty(stranger): %v", err)
	}
	if roleNone != sharedpolicy.RoleNone {
		t.Errorf("stranger role = %v, want none", roleNone)
	}
}

// TestAccessService_LeaveProperty verifies self-exit: a member can leave, the
// owner cannot (issue #156).
func TestAccessService_LeaveProperty(t *testing.T) {
	t.Parallel()
	owner := uuid.Must(uuid.NewV7())
	member := uuid.Must(uuid.NewV7())
	property := uuid.Must(uuid.NewV7())

	repo := newMemRepo()
	resolver := staticResolver{property: owner}
	policy := NewMembershipPolicy(resolver, repo)
	svc := NewAccessService(repo, resolver, nil, stubLookup{}, policy, nil, nil,
		newTestFactory(repo, &memInvitationsRepo{}, auditapp.Noop{}), nil)

	if _, err := svc.AddMember(context.Background(), owner, property, member, domain.RoleViewer); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	if err := svc.LeaveProperty(context.Background(), member, property); err != nil {
		t.Fatalf("LeaveProperty: %v", err)
	}

	// Member no longer listed.
	members, err := svc.ListMembers(context.Background(), owner, property)
	if err != nil {
		t.Fatalf("ListMembers after leave: %v", err)
	}
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

// TestAccessService_LeaveSuspendedProperty verifies the reason sheet's
// «Покинуть объект» (ticket #702): a suspended membership no longer blocks
// self-exit — the recipient sees the blur-card placeholder and may leave
// without freeing a slot first. The delete frees no slot (suspended holds
// none), so no FIFO recovery runs.
func TestAccessService_LeaveSuspendedProperty(t *testing.T) {
	t.Parallel()
	owner := uuid.Must(uuid.NewV7())
	member := uuid.Must(uuid.NewV7())
	property := uuid.Must(uuid.NewV7())

	repo := newMemRepo()
	resolver := staticResolver{property: owner}
	policy := NewMembershipPolicy(resolver, repo)
	svc := NewAccessService(repo, resolver, nil, stubLookup{}, policy, nil, nil,
		newTestFactory(repo, &memInvitationsRepo{}, auditapp.Noop{}), nil)

	if _, err := svc.AddMember(context.Background(), owner, property, member, domain.RoleViewer); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	m, err := repo.GetByPropertyAndUser(context.Background(), property, member)
	if err != nil {
		t.Fatalf("GetByPropertyAndUser: %v", err)
	}
	if err := repo.Suspend(context.Background(), m.ID, property); err != nil {
		t.Fatalf("Suspend: %v", err)
	}

	if err := svc.LeaveProperty(context.Background(), member, property); err != nil {
		t.Fatalf("LeaveProperty on suspended: %v", err)
	}

	members, err := svc.ListMembers(context.Background(), owner, property)
	if err != nil {
		t.Fatalf("ListMembers after leave: %v", err)
	}
	for _, m := range members {
		if m.UserID == member {
			t.Errorf("suspended member still present after leave")
		}
	}
}
