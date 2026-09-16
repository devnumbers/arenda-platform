package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// newTestUUID mints a V7 id for a fixture row.
func newTestUUID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	return id
}

// The participant read model's scope tests (issue #693) against a real
// Postgres: owner/full_access scoping, the archived-property cut, and the
// membership ∪ invitation union on the scoped properties.

// createSuspendedMembershipFixture inserts a suspended membership row.
func createSuspendedMembershipFixture(
	t *testing.T, ctx context.Context, repo *MembershipRepository,
	id, propertyID, userID, grantedBy uuid.UUID,
) {
	t.Helper()
	_, err := repo.CreateWithStatus(ctx, domain.Membership{
		ID:         id,
		PropertyID: propertyID,
		UserID:     userID,
		Role:       domain.RoleFullAccess,
		GrantedBy:  grantedBy,
		Status:     domain.MemberStatusSuspended,
	})
	if err != nil {
		t.Fatalf("create suspended membership: %v", err)
	}
}

// TestParticipantRepository_OwnerScope checks the owner's path: their whole
// non-archived portfolio is in scope, with the memberships and pending
// invitations of other people visible inside it.
func TestParticipantRepository_OwnerScope(t *testing.T) {
	t.Parallel()
	pool := setupAccessDB(t)
	ctx, tx, cleanup := beginAccessTx(t, pool)
	defer cleanup()

	f := newParticipantScopeFixture(t, ctx, tx)
	repo := NewParticipantRepository(tx)

	// Owner scope: both of their properties.
	scope, err := repo.ListScopeProperties(ctx, f.owner)
	if err != nil {
		t.Fatalf("owner ListScopeProperties: %v", err)
	}
	if len(scope) != 2 {
		t.Fatalf("owner scope = %d properties, want 2", len(scope))
	}

	// The owner sees the member's row and the pending invitation.
	rows, err := repo.ListMembershipsByProperties(ctx, []uuid.UUID{f.p1, f.p2})
	if err != nil {
		t.Fatalf("ListMembershipsByProperties: %v", err)
	}
	if len(rows) != 1 || rows[0].UserID != f.member || rows[0].Role != domain.RoleFullAccess {
		t.Fatalf("memberships = %+v, want the member's full_access row", rows)
	}
	invitations, err := repo.ListInvitationsByProperties(ctx, []uuid.UUID{f.p1, f.p2})
	if err != nil {
		t.Fatalf("ListInvitationsByProperties: %v", err)
	}
	if len(invitations) != 1 || invitations[0].Email != "pending@x.ru" {
		t.Fatalf("invitations = %+v, want the pending email row", invitations)
	}

	// Empty property list short-circuits to no rows.
	rows, err = repo.ListMembershipsByProperties(ctx, nil)
	if err != nil || len(rows) != 0 {
		t.Fatalf("empty list: rows = %+v err = %v, want empty and nil", rows, err)
	}
}

// TestParticipantRepository_ManageMemberScope checks the full_access member's
// path: they are scoped to the one object they manage, while viewers and
// outsiders hold no scope at all (viewers cannot manage members).
func TestParticipantRepository_ManageMemberScope(t *testing.T) {
	t.Parallel()
	pool := setupAccessDB(t)
	ctx, tx, cleanup := beginAccessTx(t, pool)
	defer cleanup()

	f := newParticipantScopeFixture(t, ctx, tx)
	repo := NewParticipantRepository(tx)

	scope, err := repo.ListScopeProperties(ctx, f.member)
	if err != nil {
		t.Fatalf("member ListScopeProperties: %v", err)
	}
	if len(scope) != 1 || scope[0].ID != f.p1 {
		t.Fatalf("member scope = %+v, want only p1", scope)
	}

	scope, err = repo.ListScopeProperties(ctx, f.outsider)
	if err != nil {
		t.Fatalf("outsider ListScopeProperties: %v", err)
	}
	if len(scope) != 0 {
		t.Fatalf("outsider scope = %+v, want empty", scope)
	}
}

// participantScopeFixture is the shared fixture of the scope tests: an owner
// with two properties, a full_access member on p1 and a pending invitation
// on p2.
type participantScopeFixture struct {
	owner, member, outsider uuid.UUID
	p1, p2                  uuid.UUID
}

func newParticipantScopeFixture(t *testing.T, ctx context.Context, tx genpostgres.DBTX) participantScopeFixture {
	t.Helper()
	q := genpostgres.New(tx)
	f := participantScopeFixture{
		owner:    createAccessTestUser(t, ctx, q),
		member:   createAccessTestUser(t, ctx, q),
		outsider: createAccessTestUser(t, ctx, q),
	}
	f.p1 = createAccessTestProperty(t, ctx, q, f.owner)
	f.p2 = createAccessTestProperty(t, ctx, q, f.owner)

	memberRepo := NewMembershipRepository(tx)
	if _, err := memberRepo.Create(ctx, domain.Membership{
		ID:         newTestUUID(t),
		PropertyID: f.p1,
		UserID:     f.member,
		Role:       domain.RoleFullAccess,
		GrantedBy:  f.owner,
	}); err != nil {
		t.Fatalf("create membership: %v", err)
	}
	invitationRepo := NewInvitationRepository(tx)
	if _, err := invitationRepo.Create(ctx, domain.Invitation{
		ID:         newTestUUID(t),
		PropertyID: f.p2,
		Email:      "pending@x.ru",
		Role:       domain.RoleViewer,
		InvitedBy:  f.owner,
		LastSentAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	return f
}

// TestParticipantRepository_ArchivedExcluded checks the archived cut: the
// owner's archived object leaves the scope together with its membership rows.
func TestParticipantRepository_ArchivedExcluded(t *testing.T) {
	t.Parallel()
	pool := setupAccessDB(t)
	ctx, tx, cleanup := beginAccessTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	owner := createAccessTestUser(t, ctx, q)
	member := createAccessTestUser(t, ctx, q)

	active := createAccessTestProperty(t, ctx, q, owner)
	archived := createAccessTestProperty(t, ctx, q, owner)
	if _, err := q.ArchiveProperty(ctx, genpostgres.ArchivePropertyParams{ID: pgUUID(archived), OwnerID: pgUUID(owner)}); err != nil {
		t.Fatalf("archive property: %v", err)
	}

	memberRepo := NewMembershipRepository(tx)
	for _, p := range []struct {
		prop uuid.UUID
		id   uuid.UUID
	}{{active, newTestUUID(t)}, {archived, newTestUUID(t)}} {
		if _, err := memberRepo.Create(ctx, domain.Membership{
			ID:         p.id,
			PropertyID: p.prop,
			UserID:     member,
			Role:       domain.RoleViewer,
			GrantedBy:  owner,
		}); err != nil {
			t.Fatalf("create membership: %v", err)
		}
	}

	repo := NewParticipantRepository(tx)
	scope, err := repo.ListScopeProperties(ctx, owner)
	if err != nil {
		t.Fatalf("ListScopeProperties: %v", err)
	}
	if len(scope) != 1 || scope[0].ID != active {
		t.Fatalf("scope = %+v, want only the active property", scope)
	}

	rows, err := repo.ListMembershipsByProperties(ctx, []uuid.UUID{active})
	if err != nil {
		t.Fatalf("ListMembershipsByProperties: %v", err)
	}
	if len(rows) != 1 || rows[0].UserID != member {
		t.Fatalf("rows = %+v, want the member's active-property row", rows)
	}
}

// TestParticipantRepository_ActiveAndSuspendedRows checks that both rows of a
// (active, suspended) pair on one (property, user) are returned — the
// application layer collapses them (active wins).
func TestParticipantRepository_ActiveAndSuspendedRows(t *testing.T) {
	t.Parallel()
	pool := setupAccessDB(t)
	ctx, tx, cleanup := beginAccessTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	owner := createAccessTestUser(t, ctx, q)
	member := createAccessTestUser(t, ctx, q)
	property := createAccessTestProperty(t, ctx, q, owner)

	memberRepo := NewMembershipRepository(tx)
	if _, err := memberRepo.Create(ctx, domain.Membership{
		ID:         newTestUUID(t),
		PropertyID: property,
		UserID:     member,
		Role:       domain.RoleViewer,
		GrantedBy:  owner,
	}); err != nil {
		t.Fatalf("create active membership: %v", err)
	}
	suspendedID := newTestUUID(t)
	createSuspendedMembershipFixture(t, ctx, memberRepo, suspendedID, property, member, owner)

	repo := NewParticipantRepository(tx)
	rows, err := repo.ListMembershipsByProperties(ctx, []uuid.UUID{property})
	if err != nil {
		t.Fatalf("ListMembershipsByProperties: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want both the active and the suspended row", rows)
	}
	statuses := map[domain.MemberStatus]bool{}
	for _, row := range rows {
		statuses[row.Status] = true
	}
	if !statuses[domain.MemberStatusActive] || !statuses[domain.MemberStatusSuspended] {
		t.Fatalf("statuses = %v, want active+suspended", statuses)
	}
}
