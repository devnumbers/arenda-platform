package postgres

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/application"
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
func TestParticipantRepository_ScopeIsOwnerOnly(t *testing.T) {
	t.Parallel()
	pool := setupAccessDB(t)
	ctx, tx, cleanup := beginAccessTx(t, pool)
	defer cleanup()

	f := newParticipantScopeFixture(t, ctx, tx)
	repo := NewParticipantRepository(tx)

	// The full_access member is out of the hub scope since D8 (issue #1304):
	// member management and the hub are the owner's, nobody sees other
	// people's participants.
	scope, err := repo.ListScopeProperties(ctx, f.member)
	if err != nil {
		t.Fatalf("member ListScopeProperties: %v", err)
	}
	if len(scope) != 0 {
		t.Fatalf("member scope = %+v, want empty", scope)
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
// owners are out — and since D8 (issue #1304) a full_access member is out like
// everyone else: the scope is owner-only.
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

	// The full_access manager is out of the removal scope since D8
	// (issue #1304) — the person's leg on the object they hold full access to
	// is the owner's to remove, not theirs.
	rows, err = memberRepo.ListForRemovalByUser(ctx, f.person, f.manager)
	if err != nil || len(rows) != 0 {
		t.Fatalf("manager rows = %+v, %v; want empty", rows, err)
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

	// The full_access manager: out of the removal scope since D8
	// (issue #1304), same as the membership listing.
	rows, err = invitationRepo.ListForRemovalByEmail(ctx, "gone@x.ru", f.manager)
	if err != nil || len(rows) != 0 {
		t.Fatalf("manager rows = %+v, %v; want empty", rows, err)
	}
}

// TestParticipantRepository_ManageScopePredicateMatrix pins the manage-scope
// predicate ACROSS all the queries that carry it (the read scope, the two
// per-person removal listings and the two owner-wide removal listings of the
// global revoke, issue #1310): the same actor must get the same verdict
// everywhere. The predicate is one SQL function now (actor_can_manage,
// migration 000135, issue #794) that every query calls — this matrix turns
// any semantic drift of the function (a dropped status='active', a role
// typo) into a loud failure instead of a silent privacy hole.
func TestParticipantRepository_ManageScopePredicateMatrix(t *testing.T) {
	t.Parallel()
	pool := setupAccessDB(t)
	ctx, tx, cleanup := beginAccessTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	owner := createAccessTestUser(t, ctx, q)
	activeManager := createAccessTestUser(t, ctx, q)
	suspendedManager := createAccessTestUser(t, ctx, q)
	viewer := createAccessTestUser(t, ctx, q)
	person := createAccessTestUser(t, ctx, q)

	prop := createAccessTestProperty(t, ctx, q, owner)
	memberRepo := NewMembershipRepository(tx)
	for _, m := range []domain.Membership{
		{ID: newTestUUID(t), PropertyID: prop, UserID: activeManager, Role: domain.RoleFullAccess, GrantedBy: owner},
		{ID: newTestUUID(t), PropertyID: prop, UserID: viewer, Role: domain.RoleViewer, GrantedBy: owner},
		{ID: newTestUUID(t), PropertyID: prop, UserID: person, Role: domain.RoleViewer, GrantedBy: owner},
	} {
		if _, err := memberRepo.Create(ctx, m); err != nil {
			t.Fatalf("create membership: %v", err)
		}
	}
	// The suspended manager holds the managing role but a paused membership —
	// the predicate must not count them anywhere.
	if _, err := memberRepo.CreateWithStatus(ctx, domain.Membership{
		ID: newTestUUID(t), PropertyID: prop, UserID: suspendedManager,
		Role: domain.RoleFullAccess, GrantedBy: owner, Status: domain.MemberStatusSuspended,
	}); err != nil {
		t.Fatalf("create suspended manager: %v", err)
	}

	invitationRepo := NewInvitationRepository(tx)
	if _, err := invitationRepo.Create(ctx, domain.Invitation{
		ID: newTestUUID(t), PropertyID: prop, Email: "matrix@x.ru",
		Role: domain.RoleViewer, InvitedBy: owner, LastSentAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create invitation: %v", err)
	}

	participants := NewParticipantRepository(tx)
	cases := []struct {
		name  string
		actor uuid.UUID
		want  bool
	}{
		{"owner", owner, true},
		// Owner-only since D8 (issue #1304): an active full_access manager is
		// out of the manage scope like everyone else.
		{"active full_access manager", activeManager, false},
		{"suspended full_access manager", suspendedManager, false},
		{"viewer", viewer, false},
	}
	// Plain sequential loop, no t.Run: the subtests share one tx (a pgx.Tx is
	// not safe for concurrent queries), and the paralleltest linter reads
	// subtest parallelism into the matrix otherwise.
	for _, tc := range cases {
		assertManageScopeVerdict(t, ctx, participants, memberRepo, invitationRepo, tc.actor, prop, person, tc.want)
	}
}

// assertManageScopeVerdict runs all five scope queries for one actor and
// checks they agree on the verdict: the read scope, the two per-person
// removal listings and the two owner-wide removal listings of the global
// revoke (issue #1310) — one predicate everywhere.
func assertManageScopeVerdict(
	t *testing.T,
	ctx context.Context,
	participants *ParticipantRepository,
	memberRepo *MembershipRepository,
	invitationRepo *InvitationRepository,
	actor, prop, person uuid.UUID,
	want bool,
) {
	t.Helper()

	props, err := participants.ListScopeProperties(ctx, actor)
	if err != nil {
		t.Fatalf("ListScopeProperties: %v", err)
	}
	if got := slices.ContainsFunc(props, func(p application.ParticipantScopeProperty) bool { return p.ID == prop }); got != want {
		t.Errorf("read scope = %v, want %v", got, want)
	}

	memberships, err := memberRepo.ListForRemovalByUser(ctx, person, actor)
	if err != nil {
		t.Fatalf("ListForRemovalByUser: %v", err)
	}
	if got := len(memberships) > 0; got != want {
		t.Errorf("membership removal scope = %v, want %v", got, want)
	}

	invitations, err := invitationRepo.ListForRemovalByEmail(ctx, "matrix@x.ru", actor)
	if err != nil {
		t.Fatalf("ListForRemovalByEmail: %v", err)
	}
	if got := len(invitations) > 0; got != want {
		t.Errorf("invitation removal scope = %v, want %v", got, want)
	}

	// The owner-wide listings of the global revoke (issue #1310) carry
	// the same predicate — the same verdict everywhere.
	ownerMemberships, err := memberRepo.ListForRemovalByOwner(ctx, actor)
	if err != nil {
		t.Fatalf("ListForRemovalByOwner: %v", err)
	}
	if got := len(ownerMemberships) > 0; got != want {
		t.Errorf("owner-wide membership removal scope = %v, want %v", got, want)
	}

	ownerInvitations, err := invitationRepo.ListForRemovalByOwner(ctx, actor)
	if err != nil {
		t.Fatalf("ListForRemovalByOwner: %v", err)
	}
	if got := len(ownerInvitations) > 0; got != want {
		t.Errorf("owner-wide invitation removal scope = %v, want %v", got, want)
	}
}

// anyIDLookup resolves every id to the anonymous «Member» row and no email —
// the display data is not the seam under test here, the scope privacy is.
type anyIDLookup struct{}

func (anyIDLookup) GetByID(_ context.Context, id uuid.UUID) (application.MemberUser, error) {
	name := "Member"
	return application.MemberUser{ID: id, Name: &name}, nil
}

func (anyIDLookup) GetByEmail(_ context.Context, _ string) (application.MemberUser, error) {
	return application.MemberUser{}, domain.ErrUserNotFound
}

// TestParticipantRepository_FullAccessSeesNoForeignParticipants is the D5/D14
// isolation regression (issue #1304): the hub scope is owner-only, so a
// full_access member of a foreign object builds no participant aggregates from
// it — the owner's people stay invisible, the deep link on a foreign
// participant answers the privacy-preserving not-found, and the summary counts
// nothing. The test went red on the pre-D8 scope (owner OR active full_access),
// where the member saw the owner's person.
func TestParticipantRepository_FullAccessSeesNoForeignParticipants(t *testing.T) {
	t.Parallel()
	pool := setupAccessDB(t)
	ctx, tx, cleanup := beginAccessTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	owner := createAccessTestUser(t, ctx, q)
	member := createAccessTestUser(t, ctx, q)
	person := createAccessTestUser(t, ctx, q)

	prop := createAccessTestProperty(t, ctx, q, owner)
	memberRepo := NewMembershipRepository(tx)
	for _, m := range []domain.Membership{
		{ID: newTestUUID(t), PropertyID: prop, UserID: member, Role: domain.RoleFullAccess, GrantedBy: owner},
		{ID: newTestUUID(t), PropertyID: prop, UserID: person, Role: domain.RoleViewer, GrantedBy: owner},
	} {
		if _, err := memberRepo.Create(ctx, m); err != nil {
			t.Fatalf("create membership: %v", err)
		}
	}

	participants := NewParticipantRepository(tx)
	svc := application.NewParticipantService(participants, anyIDLookup{}, nil, nil)

	// The owner's own view keeps both of their people (the full_access member
	// is one of them).
	own, err := svc.ListParticipants(ctx, owner)
	if err != nil {
		t.Fatalf("owner ListParticipants: %v", err)
	}
	if len(own) != 2 {
		t.Fatalf("owner participants = %+v, want both people", own)
	}

	// The full_access member sees none of the owner's people.
	foreign, err := svc.ListParticipants(ctx, member)
	if err != nil {
		t.Fatalf("member ListParticipants: %v", err)
	}
	if len(foreign) != 0 {
		t.Fatalf("member participants = %+v, want none of the owner's people", foreign)
	}
	if _, err := svc.GetParticipant(ctx, member, person.String()); !errors.Is(err, domain.ErrParticipantNotFound) {
		t.Errorf("member GetParticipant(foreign person): expected ErrParticipantNotFound, got %v", err)
	}
}
