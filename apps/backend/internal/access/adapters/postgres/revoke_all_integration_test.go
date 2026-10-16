package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// Integration coverage of the global «Отозвать доступ всем» (issue #1310)
// against a real Postgres (TEST_DATABASE_URL; skipped when unset, the access
// integration convention): the owner-wide removal listings and the gate
// probes over real rows, and the full RevokeAll use case through the real
// ParticipantMutationService on the shared lifecycle fixture.

// TestParticipantRepository_RemovalScopeByOwner checks the owner-wide
// listings: every leg of every person on the actor's properties, archived
// included — never a foreign row.
func TestParticipantRepository_RemovalScopeByOwner(t *testing.T) {
	t.Parallel()
	pool := setupAccessDB(t)
	ctx, tx, cleanup := beginAccessTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	f := seedRemovalScopeFixture(t, ctx, q, tx)

	// The owner's whole scope: the person's active and archived legs plus the
	// manager's and viewer's legs on the active property — four rows, foreign
	// stays out.
	memberRepo := NewMembershipRepository(tx)
	rows, err := memberRepo.ListForRemovalByOwner(ctx, f.owner)
	if err != nil {
		t.Fatalf("owner ListForRemovalByOwner: %v", err)
	}
	gotProps := map[uuid.UUID]int{}
	for _, m := range rows {
		gotProps[m.PropertyID]++
	}
	if len(rows) != 4 || gotProps[f.pActive] != 3 || gotProps[f.pArchived] != 1 {
		t.Fatalf("rows = %+v, want 3 legs on pActive + 1 on pArchived", rows)
	}

	// Out of the owner-only scope since D8 (issue #1304): the full_access
	// manager and the viewer revoke nothing.
	for _, actor := range []struct {
		name string
		id   uuid.UUID
	}{{"manager", f.manager}, {"viewer", f.viewer}} {
		rows, err = memberRepo.ListForRemovalByOwner(ctx, actor.id)
		if err != nil || len(rows) != 0 {
			t.Fatalf("%s rows = %+v, %v; want empty", actor.name, rows, err)
		}
	}

	assertOwnerInvitationScope(t, ctx, NewInvitationRepository(tx), f)
}

// assertOwnerInvitationScope checks the owner-wide invitation listing: the
// owner sees both of their invitations (archived included), the manager
// sees none.
func assertOwnerInvitationScope(
	t *testing.T, ctx context.Context, repo *InvitationRepository, f removalScopeFixture,
) {
	t.Helper()
	rows, err := repo.ListForRemovalByOwner(ctx, f.owner)
	if err != nil {
		t.Fatalf("owner ListForRemovalByOwner: %v", err)
	}
	gotProps := map[uuid.UUID]bool{}
	for _, inv := range rows {
		gotProps[inv.PropertyID] = true
	}
	if len(rows) != 2 || !gotProps[f.pActive] || !gotProps[f.pArchived] {
		t.Fatalf("invitation rows = %+v, want pActive+pArchived (archived included)", rows)
	}
	rows, err = repo.ListForRemovalByOwner(ctx, f.manager)
	if err != nil || len(rows) != 0 {
		t.Fatalf("manager invitation rows = %+v, %v; want empty", rows, err)
	}
}

// TestRevokeAllScope_Probes checks the gate probes against real rows:
// ownership counts any status; the membership probe is owner-blind.
func TestRevokeAllScope_Probes(t *testing.T) {
	t.Parallel()
	pool := setupAccessDB(t)
	ctx, tx, cleanup := beginAccessTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	owner := createAccessTestUser(t, ctx, q)
	foreign := createAccessTestUser(t, ctx, q)
	participant := createAccessTestUser(t, ctx, q)
	stranger := createAccessTestUser(t, ctx, q)

	pArchived := createAccessTestProperty(t, ctx, q, owner)
	if _, err := q.ArchiveProperty(ctx, genpostgres.ArchivePropertyParams{ID: pgUUID(pArchived), OwnerID: pgUUID(owner)}); err != nil {
		t.Fatalf("archive property: %v", err)
	}
	pForeign := createAccessTestProperty(t, ctx, q, foreign)

	if _, err := NewMembershipRepository(tx).Create(ctx, domain.Membership{
		ID: newTestUUID(t), PropertyID: pForeign, UserID: participant,
		Role: domain.RoleViewer, GrantedBy: foreign,
	}); err != nil {
		t.Fatalf("create membership: %v", err)
	}

	scope := NewRevokeAllScope(tx)

	// Ownership is any-status: an owner whose only object is archived (its
	// legs are still theirs to revoke) stays an owner.
	owns, err := scope.OwnsAnyProperty(ctx, owner)
	if err != nil || !owns {
		t.Errorf("owner OwnsAnyProperty = %v, %v; want true", owns, err)
	}
	owns, err = scope.OwnsAnyProperty(ctx, participant)
	if err != nil || owns {
		t.Errorf("participant OwnsAnyProperty = %v, %v; want false", owns, err)
	}

	// The membership probe is owner-blind: the participant holds a leg on
	// their own foreign object; the owner holds none (ownership is not a
	// membership row); the stranger holds nothing.
	holds, err := scope.HoldsAnyMembership(ctx, participant)
	if err != nil || !holds {
		t.Errorf("participant HoldsAnyMembership = %v, %v; want true", holds, err)
	}
	holds, err = scope.HoldsAnyMembership(ctx, owner)
	if err != nil || holds {
		t.Errorf("owner HoldsAnyMembership = %v, %v; want false", holds, err)
	}
	holds, err = scope.HoldsAnyMembership(ctx, stranger)
	if err != nil || holds {
		t.Errorf("stranger HoldsAnyMembership = %v, %v; want false", holds, err)
	}
}

// TestRevokeAll_RemovesEveryLegAndRecoversFIFO is the AC1 end-to-end
// (issue #1310): one call through the real service removes every membership
// and pending invitation on the owner's properties — archived included —
// recovers each affected recipient's suspended accesses FIFO, and never
// touches a foreign leg.

// revokeAllFixture carries the seeded ids of the end-to-end revoke test.
type revokeAllFixture struct {
	owner, foreign   uuid.UUID
	memberA, memberB uuid.UUID
	p1, p2, p3, p4   uuid.UUID // P2 archived; p3/p4 foreign.
	memberRepo       *MembershipRepository
	invitationRepo   *InvitationRepository
}

// seedRevokeAllFixture seeds the end-to-end state: two members with legs on
// the owner's active p1 and archived p2 (A also holds a foreign leg on p3
// and a suspended one on p4), pending invitations on all three owner/foreign
// properties, A's tariff pool of two (one slot occupied, one freed by the
// revoke for the FIFO recovery).
func seedRevokeAllFixture(t *testing.T, f *accessLifecycleFixture) revokeAllFixture {
	t.Helper()
	ctx := f.bg()
	out := revokeAllFixture{
		owner:          createAccessTestUser(t, ctx, f.q),
		foreign:        createAccessTestUser(t, ctx, f.q),
		memberA:        createAccessTestUser(t, ctx, f.q),
		memberB:        createAccessTestUser(t, ctx, f.q),
		memberRepo:     NewMembershipRepository(f.tx),
		invitationRepo: NewInvitationRepository(f.tx),
	}

	out.p1 = f.addProperty(t, out.owner, "Свой 1")
	out.p2 = f.addProperty(t, out.owner, "Свой 2")
	if _, err := f.q.ArchiveProperty(ctx, genpostgres.ArchivePropertyParams{ID: pgUUID(out.p2), OwnerID: pgUUID(out.owner)}); err != nil {
		t.Fatalf("archive p2: %v", err)
	}
	out.p3 = f.addProperty(t, out.foreign, "Чужой")
	out.p4 = f.addProperty(t, out.foreign, "Чужой для suspended")

	// A's two slots: one goes to the active leg on p1, the other stays free
	// for the FIFO recovery; the suspended leg on p4 (another owner's
	// object) waits in A's queue. A's active leg on p3 (foreign) holds the
	// second slot after the revoke, so the limit must exceed it.
	f.limiter.set(out.memberA, 2)
	for _, m := range []domain.Membership{
		{ID: newTestUUID(t), PropertyID: out.p1, UserID: out.memberA, Role: domain.RoleViewer, GrantedBy: out.owner},
		{ID: newTestUUID(t), PropertyID: out.p2, UserID: out.memberA, Role: domain.RoleViewer, GrantedBy: out.owner},
		{ID: newTestUUID(t), PropertyID: out.p1, UserID: out.memberB, Role: domain.RoleViewer, GrantedBy: out.owner},
		{ID: newTestUUID(t), PropertyID: out.p3, UserID: out.memberA, Role: domain.RoleViewer, GrantedBy: out.foreign},
	} {
		if _, err := out.memberRepo.Create(ctx, m); err != nil {
			t.Fatalf("create membership: %v", err)
		}
	}
	if _, err := out.memberRepo.CreateWithStatus(ctx, domain.Membership{
		ID: newTestUUID(t), PropertyID: out.p4, UserID: out.memberA,
		Role: domain.RoleViewer, GrantedBy: out.foreign, Status: domain.MemberStatusSuspended,
	}); err != nil {
		t.Fatalf("create suspended membership: %v", err)
	}

	sentAt := time.Now().UTC()
	for _, inv := range []domain.Invitation{
		{ID: newTestUUID(t), PropertyID: out.p1, Email: f.email("pending"), Role: domain.RoleViewer, InvitedBy: out.owner, LastSentAt: sentAt},
		{ID: newTestUUID(t), PropertyID: out.p2, Email: f.email("pending"), Role: domain.RoleViewer, InvitedBy: out.owner, LastSentAt: sentAt},
		{ID: newTestUUID(t), PropertyID: out.p3, Email: f.email("pending"), Role: domain.RoleViewer, InvitedBy: out.foreign, LastSentAt: sentAt},
	} {
		if _, err := out.invitationRepo.Create(ctx, inv); err != nil {
			t.Fatalf("create invitation: %v", err)
		}
	}
	return out
}

func TestRevokeAll_RemovesEveryLegAndRecoversFIFO(t *testing.T) {
	t.Parallel()
	f := newAccessLifecycleFixture(t)
	ctx := f.bg()
	s := seedRevokeAllFixture(t, f)

	if err := f.mutations.RevokeAll(ctx, s.owner); err != nil {
		t.Fatalf("revoke all: %v", err)
	}

	// Every leg on the owner's properties is gone — the archived p2's too.
	for _, leg := range []struct {
		property, user uuid.UUID
	}{{s.p1, s.memberA}, {s.p2, s.memberA}, {s.p1, s.memberB}} {
		if _, err := s.memberRepo.GetByPropertyAndUser(ctx, leg.property, leg.user); !errors.Is(err, domain.ErrMemberNotFound) {
			t.Errorf("leg (%s, %s) must be removed, got %v", leg.property, leg.user, err)
		}
	}
	// The freed slots recover A's suspended access FIFO.
	recovered, err := s.memberRepo.GetByPropertyAndUser(ctx, s.p4, s.memberA)
	if err != nil || recovered.Status != domain.MemberStatusActive {
		t.Errorf("A's suspended leg must be recovered FIFO, got %+v (%v)", recovered, err)
	}
	// The foreign object's leg stays.
	if _, err := s.memberRepo.GetByPropertyAndUser(ctx, s.p3, s.memberA); err != nil {
		t.Errorf("foreign leg must stay, got %v", err)
	}

	// The owner's invitations are gone (both, archived included); the
	// foreign one stays.
	invitations, err := s.invitationRepo.ListForRemovalByOwner(ctx, s.owner)
	if err != nil || len(invitations) != 0 {
		t.Errorf("owner invitations = %+v, %v; want empty", invitations, err)
	}
	foreignInvitations, err := s.invitationRepo.ListForRemovalByOwner(ctx, s.foreign)
	if err != nil || len(foreignInvitations) != 1 {
		t.Errorf("foreign invitations = %+v, %v; want 1", foreignInvitations, err)
	}
}

// TestRevokeAll_IdempotentNoOp is the AC2 (issue #1310): the repeat on an
// empty state succeeds and changes nothing.
func TestRevokeAll_IdempotentNoOp(t *testing.T) {
	t.Parallel()
	f := newAccessLifecycleFixture(t)
	ctx := f.bg()

	owner := createAccessTestUser(t, ctx, f.q)
	memberA := f.addUserWithEmail(t, f.email("member.a"))
	p1 := f.addProperty(t, owner, "Свой 1")
	if _, err := NewMembershipRepository(f.tx).Create(ctx, domain.Membership{
		ID: newTestUUID(t), PropertyID: p1, UserID: memberA,
		Role: domain.RoleViewer, GrantedBy: owner,
	}); err != nil {
		t.Fatalf("create membership: %v", err)
	}

	if err := f.mutations.RevokeAll(ctx, owner); err != nil {
		t.Fatalf("first revoke all: %v", err)
	}
	if err := f.mutations.RevokeAll(ctx, owner); err != nil {
		t.Fatalf("repeat revoke all: %v", err)
	}

	// The state stays empty — the repeat wrote no new rows.
	memberRepo := NewMembershipRepository(f.tx)
	rows, err := memberRepo.ListForRemovalByOwner(ctx, owner)
	if err != nil || len(rows) != 0 {
		t.Errorf("membership rows after repeat = %+v, %v; want empty", rows, err)
	}
}

// TestRevokeAll_GateRefusesNonOwnerAndStranger is the HTTP-contract base
// (issue #1310) at the service seam: the non-owner participant gets the loud
// refusal, the stranger the privacy not-found — and neither call touches
// any leg.
func TestRevokeAll_GateRefusesNonOwnerAndStranger(t *testing.T) {
	t.Parallel()
	f := newAccessLifecycleFixture(t)
	ctx := f.bg()

	foreign := createAccessTestUser(t, ctx, f.q)
	participant := f.addUserWithEmail(t, f.email("participant"))
	stranger := createAccessTestUser(t, ctx, f.q)

	p1 := f.addProperty(t, foreign, "Чужой")
	if _, err := NewMembershipRepository(f.tx).Create(ctx, domain.Membership{
		ID: newTestUUID(t), PropertyID: p1, UserID: participant,
		Role: domain.RoleViewer, GrantedBy: foreign,
	}); err != nil {
		t.Fatalf("create membership: %v", err)
	}

	if err := f.mutations.RevokeAll(ctx, participant); !errors.Is(err, domain.ErrNotPropertyOwner) {
		t.Errorf("non-owner participant: got %v, want ErrNotPropertyOwner", err)
	}
	if err := f.mutations.RevokeAll(ctx, stranger); !errors.Is(err, domain.ErrParticipantNotFound) {
		t.Errorf("stranger: got %v, want ErrParticipantNotFound", err)
	}

	// Both refusals left the leg alone.
	if _, err := NewMembershipRepository(f.tx).GetByPropertyAndUser(ctx, p1, participant); err != nil {
		t.Errorf("refused calls must not remove legs, got %v", err)
	}
}
