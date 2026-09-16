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

// The mutation side of the owner's participant aggregate (issue #694): the
// removal listings return the person's legs only inside the actor's manage
// scope — owner or active full_access member — with archived properties
// INCLUDED (revoking keeps working there, issue #163).

// TestParticipantRepository_RemovalScopeMemberships checks the membership
// removal listing: the actor's owned and archived properties are in, foreign
// owners are out, and a full_access member is scoped to their one object.
func TestParticipantRepository_RemovalScopeMemberships(t *testing.T) {
	t.Parallel()
	pool := setupAccessDB(t)
	ctx, tx, cleanup := beginAccessTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	f := seedRemovalScopeFixture(t, ctx, q, tx)

	memberRepo := NewMembershipRepository(tx)
	rows, err := memberRepo.ListForRemovalByUser(ctx, f.person, f.owner)
	if err != nil {
		t.Fatalf("owner ListForRemovalByUser: %v", err)
	}
	// The owner's removal scope: both of their properties, archived included,
	// never the foreign one.
	assertRemovalProps(t, rows, f.person, f.pActive, f.pArchived)

	// The full_access manager is scoped to the one object they manage.
	rows, err = memberRepo.ListForRemovalByUser(ctx, f.person, f.manager)
	if err != nil || len(rows) != 1 || rows[0].PropertyID != f.pActive {
		t.Fatalf("manager rows = %+v, %v; want only pActive", rows, err)
	}

	// A viewer manages nothing.
	rows, err = memberRepo.ListForRemovalByUser(ctx, f.person, f.viewer)
	if err != nil || len(rows) != 0 {
		t.Fatalf("viewer rows = %+v, %v; want empty", rows, err)
	}
}

// removalScopeFixture is the shared fixture of the removal-scope tests.
type removalScopeFixture struct {
	owner, manager, viewer, person uuid.UUID
	pActive, pArchived             uuid.UUID
}

// seedRemovalScopeFixture creates the removal-scope data: an owner with an
// active and an archived property, a full_access manager and a viewer on the
// active one, a person with legs (active + archived + one on a foreign
// owner's property), and pending invitations for "gone@x.ru" on all three.
func seedRemovalScopeFixture(t *testing.T, ctx context.Context, q *genpostgres.Queries, tx genpostgres.DBTX) removalScopeFixture {
	t.Helper()
	f := removalScopeFixture{
		owner:   createAccessTestUser(t, ctx, q),
		manager: createAccessTestUser(t, ctx, q),
		viewer:  createAccessTestUser(t, ctx, q),
		person:  createAccessTestUser(t, ctx, q),
	}
	foreign := createAccessTestUser(t, ctx, q)

	f.pActive = createAccessTestProperty(t, ctx, q, f.owner)
	f.pArchived = createAccessTestProperty(t, ctx, q, f.owner)
	if _, err := q.ArchiveProperty(ctx, genpostgres.ArchivePropertyParams{ID: pgUUID(f.pArchived), OwnerID: pgUUID(f.owner)}); err != nil {
		t.Fatalf("archive property: %v", err)
	}
	pForeign := createAccessTestProperty(t, ctx, q, foreign)

	memberRepo := NewMembershipRepository(tx)
	for _, m := range []domain.Membership{
		{ID: newTestUUID(t), PropertyID: f.pActive, UserID: f.person, Role: domain.RoleViewer, GrantedBy: f.owner},
		{ID: newTestUUID(t), PropertyID: f.pArchived, UserID: f.person, Role: domain.RoleViewer, GrantedBy: f.owner},
		{ID: newTestUUID(t), PropertyID: pForeign, UserID: f.person, Role: domain.RoleViewer, GrantedBy: foreign},
		// The manager holds active full_access on the active property; the
		// viewer a viewer role (which does not manage members).
		{ID: newTestUUID(t), PropertyID: f.pActive, UserID: f.manager, Role: domain.RoleFullAccess, GrantedBy: f.owner},
		{ID: newTestUUID(t), PropertyID: f.pActive, UserID: f.viewer, Role: domain.RoleViewer, GrantedBy: f.owner},
	} {
		if _, err := memberRepo.Create(ctx, m); err != nil {
			t.Fatalf("create membership: %v", err)
		}
	}

	invitationRepo := NewInvitationRepository(tx)
	sentAt := time.Now().UTC()
	for _, inv := range []domain.Invitation{
		{ID: newTestUUID(t), PropertyID: f.pActive, Email: "Gone@x.ru", Role: domain.RoleViewer, InvitedBy: f.owner, LastSentAt: sentAt},
		{ID: newTestUUID(t), PropertyID: f.pArchived, Email: "gone@x.ru", Role: domain.RoleViewer, InvitedBy: f.owner, LastSentAt: sentAt},
		{ID: newTestUUID(t), PropertyID: pForeign, Email: "gone@x.ru", Role: domain.RoleViewer, InvitedBy: foreign, LastSentAt: sentAt},
	} {
		if _, err := invitationRepo.Create(ctx, inv); err != nil {
			t.Fatalf("create invitation: %v", err)
		}
	}
	return f
}

// assertRemovalProps checks that the removal rows are exactly the person's
// legs on the wanted properties.
func assertRemovalProps(t *testing.T, rows []domain.Membership, person uuid.UUID, want ...uuid.UUID) {
	t.Helper()
	gotProps := map[uuid.UUID]bool{}
	for _, m := range rows {
		gotProps[m.PropertyID] = true
		if m.UserID != person {
			t.Errorf("row = %+v, want the person's row", m)
		}
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %+v, want %d properties", rows, len(want))
	}
	for _, prop := range want {
		if !gotProps[prop] {
			t.Errorf("missing removal row for %s (rows = %+v)", prop, rows)
		}
	}
}

// TestParticipantRepository_RemovalScopeInvitations checks the invitation
// removal listing: same scope rule, case-insensitive email matching.
func TestParticipantRepository_RemovalScopeInvitations(t *testing.T) {
	t.Parallel()
	pool := setupAccessDB(t)
	ctx, tx, cleanup := beginAccessTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	f := seedRemovalScopeFixture(t, ctx, q, tx)

	invitationRepo := NewInvitationRepository(tx)
	// The owner: both of their invitations, archived included, mixed case
	// matched; the foreign one stays out.
	rows, err := invitationRepo.ListForRemovalByEmail(ctx, "GONE@x.ru", f.owner)
	if err != nil {
		t.Fatalf("owner ListForRemovalByEmail: %v", err)
	}
	gotProps := map[uuid.UUID]bool{}
	for _, inv := range rows {
		gotProps[inv.PropertyID] = true
	}
	if len(rows) != 2 || !gotProps[f.pActive] || !gotProps[f.pArchived] {
		t.Fatalf("rows = %+v, want pActive+pArchived (archived included)", rows)
	}

	// The full_access manager: only the invitation on their object.
	rows, err = invitationRepo.ListForRemovalByEmail(ctx, "gone@x.ru", f.manager)
	if err != nil || len(rows) != 1 || rows[0].PropertyID != f.pActive {
		t.Fatalf("manager rows = %+v, %v; want only pActive", rows, err)
	}
}
