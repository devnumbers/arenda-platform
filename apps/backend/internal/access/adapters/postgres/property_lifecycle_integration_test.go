package postgres

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// These integration tests cover the issue #163 acceptance criteria against a
// real Postgres (TEST_DATABASE_URL; skipped when unset, same convention as the
// other access integration tests). Everything runs inside the test transaction
// and is rolled back in cleanup.
//
// A full properties PropertyService is too heavy to construct here (billing
// lifecycle, photo storage, timezone resolver, policy...), so the tests
// emulate only its slot-significant parts by invoking the same sqlc queries
// and SlotCoordinator calls the service uses, in the same order:
//   - ArchiveProperty / ArchiveExcessProperties: q.ArchiveProperty, then
//     SlotCoordinator.RecoverSuspendedForProperty (service.go ArchiveProperty
//     and ArchiveExcessProperties both run exactly this pair).
//   - UnarchiveProperty: q.UnarchiveProperty, then
//     SlotCoordinator.EnforceOnUnarchiveForProperty.
//   - DeleteProperty: SlotCoordinator.RecoverAfterPropertyDelete (drops the
//     membership rows and recovers the freed slots FIFO), then q.DeleteProperty
//     (the property row removal cascade-deletes pending invitations via FK).
//
// The SlotCoordinator itself is real, wired to the real MembershipRepository,
// OwnerResolver and sqlc queries by the shared lifecycleMailFixture; only the
// tariff limiter, occupancy and own-properties ports are in-memory fakes.
//
// The invitation activation test goes through the real InvitationService
// (ActivatePendingInvitations -> activateInvitation) rather than emulating the
// insert, because the fixture already wires it with in-memory fakes for the
// mailer and the no-commit beginner — the archived-property branch (slot check
// skipped) is production code under test here, not a test-side reimplementation.
//
// PinSuspendedAt sets an explicit suspended_at on a membership row. Suspend
// uses now(), which is the transaction start time inside the test tx, so two
// memberships suspended in the same test get identical timestamps and the FIFO
// order becomes nondeterministic; pinning the timestamps simulates the passage
// of time between suspensions.
func pinSuspendedAt(t *testing.T, tx pgx.Tx, memberID, propertyID uuid.UUID, ts time.Time) {
	t.Helper()
	tag, err := tx.Exec(context.Background(),
		"UPDATE property_members SET suspended_at = $3 WHERE id = $1 AND property_id = $2",
		pgUUID(memberID), pgUUID(propertyID), ts)
	if err != nil {
		t.Fatalf("pin suspended_at: %v", err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("pin suspended_at: expected 1 row, got %d", tag.RowsAffected())
	}
}

// addMembership creates an active membership directly through the repository
// (the fixture's AccessService would enforce slots, which the test controls
// explicitly via statuses instead).
func addMembership(t *testing.T, repo *MembershipRepository, propertyID, userID, grantedBy uuid.UUID) domain.Membership {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	m, err := repo.Create(context.Background(), domain.Membership{
		ID: id, PropertyID: propertyID, UserID: userID, Role: domain.RoleViewer, GrantedBy: grantedBy,
	})
	if err != nil {
		t.Fatalf("create membership: %v", err)
	}
	return m
}

// addSuspendedMembership creates a suspended membership with an explicit
// suspended_at timestamp (see pinSuspendedAt).
func addSuspendedMembership(
	t *testing.T, repo *MembershipRepository, tx pgx.Tx,
	propertyID, userID, grantedBy uuid.UUID, suspendedAt time.Time,
) {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	m, err := repo.CreateWithStatus(context.Background(), domain.Membership{
		ID: id, PropertyID: propertyID, UserID: userID, Role: domain.RoleViewer, GrantedBy: grantedBy,
		Status: domain.MemberStatusSuspended,
	})
	if err != nil {
		t.Fatalf("create suspended membership: %v", err)
	}
	pinSuspendedAt(t, tx, m.ID, propertyID, suspendedAt)
}

// activePropertyIDs returns the property ids of the user's active memberships
// (occupied recipient slots, archived properties excluded).
func activePropertyIDs(t *testing.T, repo *MembershipRepository, userID uuid.UUID) map[uuid.UUID]bool {
	t.Helper()
	rows, err := repo.ListActiveByUser(context.Background(), userID)
	if err != nil {
		t.Fatalf("ListActiveByUser: %v", err)
	}
	out := make(map[uuid.UUID]bool, len(rows))
	for _, m := range rows {
		out[m.PropertyID] = true
	}
	return out
}

// TestPropertyLifecycle_ArchiveActiveMemberRecoversSuspendedFIFO covers AC:
// archiving a property with an ACTIVE member keeps the membership row active
// but drops it from the recipient's used slots (ListActiveByUser excludes
// archived), and the freed slot recovers the recipient's oldest suspended
// membership FIFO via RecoverSuspendedForProperty.
func TestPropertyLifecycle_ArchiveActiveMemberRecoversSuspendedFIFO(t *testing.T) {
	t.Parallel()
	f := newAccessLifecycleFixture(t)
	ctx := f.bg()
	repo := NewMembershipRepository(f.tx)

	owner := f.addUserWithEmail(t, f.email("owner"))
	recipient := f.addUserWithEmail(t, f.email("recipient"))
	target := f.addProperty(t, owner, "Квартира на Невском")
	older := f.addProperty(t, owner, "Дача у моря")
	newer := f.addProperty(t, owner, "Студия в центре")

	// One slot total: the active membership on target occupies it, the two
	// suspended memberships wait in the FIFO queue (older suspended first).
	f.limiter.set(recipient, 1)
	addMembership(t, repo, target, recipient, owner)
	base := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	addSuspendedMembership(t, repo, f.tx, older, recipient, owner, base)
	addSuspendedMembership(t, repo, f.tx, newer, recipient, owner, base.Add(time.Hour))

	if got := activePropertyIDs(t, repo, recipient); len(got) != 1 || !got[target] {
		t.Fatalf("active slots before archive = %v, want only target", got)
	}

	// Archive: the sqlc query PropertyService.ArchiveProperty runs.
	if _, err := f.q.ArchiveProperty(ctx, genpostgres.ArchivePropertyParams{ID: pgUUID(target), OwnerID: pgUUID(owner)}); err != nil {
		t.Fatalf("ArchiveProperty: %v", err)
	}

	// The membership row itself is untouched: still active.
	m, err := repo.GetByPropertyAndUser(ctx, target, recipient)
	if err != nil {
		t.Fatalf("GetByPropertyAndUser after archive: %v", err)
	}
	if m.Status != domain.MemberStatusActive {
		t.Errorf("membership on archived property must stay active, got %q", m.Status)
	}

	// ...but it no longer occupies a recipient slot.
	if got := activePropertyIDs(t, repo, recipient); len(got) != 0 {
		t.Fatalf("active slots after archive = %v, want none", got)
	}

	// The service call that recovers the freed slot.
	if _, err := f.slots.RecoverSuspendedForProperty(ctx, accessNoCommitTx{f.tx}, target); err != nil {
		t.Fatalf("RecoverSuspendedForProperty: %v", err)
	}

	// FIFO: the oldest suspended membership is reactivated, the newer one
	// stays suspended.
	oldM, err := repo.GetByPropertyAndUser(ctx, older, recipient)
	if err != nil {
		t.Fatalf("GetByPropertyAndUser older: %v", err)
	}
	if oldM.Status != domain.MemberStatusActive {
		t.Errorf("oldest suspended membership must be reactivated, got %q", oldM.Status)
	}
	newM, err := repo.GetByPropertyAndUser(ctx, newer, recipient)
	if err != nil {
		t.Fatalf("GetByPropertyAndUser newer: %v", err)
	}
	if newM.Status != domain.MemberStatusSuspended {
		t.Errorf("newer suspended membership must stay suspended, got %q", newM.Status)
	}
	if got := activePropertyIDs(t, repo, recipient); len(got) != 1 || !got[older] {
		t.Errorf("active slots after recovery = %v, want only the oldest suspended property", got)
	}
}

// TestPropertyLifecycle_ArchiveKeepsSuspendedMembersAndPendingInvitations
// covers AC: archiving does NOT touch suspended memberships (status and
// suspended_at unchanged, and the archived row is excluded from the FIFO
// recovery queue so it cannot be reactivated while archived) and does NOT
// touch pending invitations (the row is still present afterwards).
func TestPropertyLifecycle_ArchiveKeepsSuspendedMembersAndPendingInvitations(t *testing.T) {
	t.Parallel()
	f := newAccessLifecycleFixture(t)
	ctx := f.bg()
	repo := NewMembershipRepository(f.tx)

	owner := f.addUserWithEmail(t, f.email("owner"))
	recipient := f.addUserWithEmail(t, f.email("recipient"))
	property := f.addProperty(t, owner, "Квартира на Невском")
	f.limiter.set(recipient, 0)

	suspendedAt := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	addSuspendedMembership(t, repo, f.tx, property, recipient, owner, suspendedAt)
	if _, err := f.invites.InviteByEmail(ctx, owner, property, "pending@example.com", domain.RoleViewer); err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}

	if _, err := f.q.ArchiveProperty(ctx, genpostgres.ArchivePropertyParams{ID: pgUUID(property), OwnerID: pgUUID(owner)}); err != nil {
		t.Fatalf("ArchiveProperty: %v", err)
	}
	if _, err := f.slots.RecoverSuspendedForProperty(ctx, accessNoCommitTx{f.tx}, property); err != nil {
		t.Fatalf("RecoverSuspendedForProperty: %v", err)
	}

	// The suspended membership is untouched.
	m, err := repo.GetByPropertyAndUser(ctx, property, recipient)
	if err != nil {
		t.Fatalf("GetByPropertyAndUser after archive: %v", err)
	}
	if m.Status != domain.MemberStatusSuspended {
		t.Errorf("suspended membership must stay suspended, got %q", m.Status)
	}
	if m.SuspendedAt == nil || !m.SuspendedAt.Equal(suspendedAt) {
		t.Errorf("suspended_at = %v, want %v (unchanged)", m.SuspendedAt, suspendedAt)
	}
	// The archived suspended membership must not enter the FIFO recovery
	// queue while the property is archived.
	if rows, err := repo.ListSuspendedByUser(ctx, recipient); err != nil || len(rows) != 0 {
		t.Errorf("ListSuspendedByUser after archive = %d rows, %v; want empty", len(rows), err)
	}

	// The pending invitation survives the archive.
	invitations, err := f.q.ListPropertyMemberInvitations(ctx, pgUUID(property))
	if err != nil {
		t.Fatalf("ListPropertyMemberInvitations: %v", err)
	}
	if len(invitations) != 1 {
		t.Errorf("pending invitations after archive = %d, want 1 (untouched)", len(invitations))
	}
}

// TestPropertyLifecycle_InvitationActivationOnArchivedProperty covers AC:
// activating a pending invitation to an archived property creates the
// membership in the ACTIVE status without slot enforcement (the recipient has
// no free slot here — a non-archived property would have suspended the grant),
// and the recipient's used slots stay unchanged because ListActiveByUser still
// excludes the archived property. Goes through the real InvitationService.
func TestPropertyLifecycle_InvitationActivationOnArchivedProperty(t *testing.T) {
	t.Parallel()
	f := newAccessLifecycleFixture(t)
	ctx := f.bg()
	repo := NewMembershipRepository(f.tx)

	owner := f.addUserWithEmail(t, f.email("owner"))
	property := f.addProperty(t, owner, "Квартира на Невском")
	lateEmail := f.email("late")
	if _, err := f.invites.InviteByEmail(ctx, owner, property, lateEmail, domain.RoleViewer); err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}
	if _, err := f.q.ArchiveProperty(ctx, genpostgres.ArchivePropertyParams{ID: pgUUID(property), OwnerID: pgUUID(owner)}); err != nil {
		t.Fatalf("ArchiveProperty: %v", err)
	}

	// The invitee registers with ZERO free slots: if the slot check were not
	// skipped for archived properties, the membership would be suspended.
	late := f.addUserWithEmail(t, lateEmail)
	f.limiter.set(late, 0)
	if err := f.invites.ActivatePendingInvitations(ctx, late, lateEmail); err != nil {
		t.Fatalf("ActivatePendingInvitations: %v", err)
	}

	m, err := repo.GetByPropertyAndUser(ctx, property, late)
	if err != nil {
		t.Fatalf("GetByPropertyAndUser after activation: %v", err)
	}
	if m.Status != domain.MemberStatusActive {
		t.Errorf("activation to an archived property must create an ACTIVE membership, got %q", m.Status)
	}

	// Slot accounting is unchanged: the archived property occupies nothing.
	if got := activePropertyIDs(t, repo, late); len(got) != 0 {
		t.Errorf("active slots after activation = %v, want none (archived excluded)", got)
	}
	if count, err := repo.CountActiveByUser(ctx, late); err != nil || count != 0 {
		t.Errorf("CountActiveByUser after activation = %d, %v; want 0", count, err)
	}

	// The invitation row is consumed by the activation.
	invitations, err := f.q.ListPropertyMemberInvitations(ctx, pgUUID(property))
	if err != nil {
		t.Fatalf("ListPropertyMemberInvitations: %v", err)
	}
	if len(invitations) != 0 {
		t.Errorf("pending invitations after activation = %d, want 0", len(invitations))
	}
}

// unarchiveSlotScenario seeds one recipient with an active membership on the
// target property (plus one on a second, occupied property when withOccupied
// is set), archives and unarchives the target, then runs the recipient slot
// enforcement — the slot-significant steps of PropertyService.UnarchiveProperty
// in their production order.
func unarchiveSlotScenario(t *testing.T, limit int, withOccupied bool) (repo *MembershipRepository, recipient, target uuid.UUID) {
	t.Helper()
	f := newAccessLifecycleFixture(t)
	repo = NewMembershipRepository(f.tx)
	owner := f.addUserWithEmail(t, f.email("owner"))
	recipient = f.addUserWithEmail(t, f.email("recipient"))
	target = f.addProperty(t, owner, "Квартира на Невском")
	f.limiter.set(recipient, limit)

	if withOccupied {
		occupied := f.addProperty(t, owner, "Дача у моря")
		addMembership(t, repo, occupied, recipient, owner)
	}
	addMembership(t, repo, target, recipient, owner)
	ctx := f.bg()
	if _, err := f.q.ArchiveProperty(ctx, genpostgres.ArchivePropertyParams{ID: pgUUID(target), OwnerID: pgUUID(owner)}); err != nil {
		t.Fatalf("ArchiveProperty: %v", err)
	}
	if _, err := f.q.UnarchiveProperty(ctx, genpostgres.UnarchivePropertyParams{ID: pgUUID(target), OwnerID: pgUUID(owner)}); err != nil {
		t.Fatalf("UnarchiveProperty: %v", err)
	}
	if err := f.slots.EnforceOnUnarchiveForProperty(ctx, accessNoCommitTx{f.tx}, target); err != nil {
		t.Fatalf("EnforceOnUnarchiveForProperty: %v", err)
	}
	return repo, recipient, target
}

// TestPropertyLifecycle_UnarchiveEnforcesRecipientSlots covers AC: after the
// property is unarchived (back to active), EnforceOnUnarchiveForProperty
// suspends the membership when the recipient has no free slot and keeps it
// active when a slot exists.
func TestPropertyLifecycle_UnarchiveEnforcesRecipientSlots(t *testing.T) {
	t.Parallel()
	t.Run("no free slot suspends the membership", func(t *testing.T) {
		t.Parallel()
		// One slot, already occupied by the other property: the unarchived
		// object does not fit and its membership is suspended.
		repo, recipient, target := unarchiveSlotScenario(t, 1, true)

		m, err := repo.GetByPropertyAndUser(context.Background(), target, recipient)
		if err != nil {
			t.Fatalf("GetByPropertyAndUser: %v", err)
		}
		if m.Status != domain.MemberStatusSuspended {
			t.Errorf("membership on unarchived property must be suspended without a free slot, got %q", m.Status)
		}
		if got := activePropertyIDs(t, repo, recipient); len(got) != 1 || got[target] {
			t.Errorf("active slots after unarchive = %v, want only the previously occupied property", got)
		}
	})

	t.Run("free slot keeps the membership active", func(t *testing.T) {
		t.Parallel()
		// Two slots, none otherwise occupied: the unarchived object fits and
		// stays active.
		repo, recipient, target := unarchiveSlotScenario(t, 2, false)

		m, err := repo.GetByPropertyAndUser(context.Background(), target, recipient)
		if err != nil {
			t.Fatalf("GetByPropertyAndUser: %v", err)
		}
		if m.Status != domain.MemberStatusActive {
			t.Errorf("membership on unarchived property must stay active with a free slot, got %q", m.Status)
		}
		if got := activePropertyIDs(t, repo, recipient); len(got) != 1 || !got[target] {
			t.Errorf("active slots after unarchive = %v, want the unarchived property", got)
		}
	})

	t.Run("pool exactly at limit keeps the membership active", func(t *testing.T) {
		t.Parallel()
		// Two slots, one occupied by the other property: after the unarchive
		// the pool is exactly at the limit — the object still fits and its
		// membership must stay active (only strictly over the limit suspends).
		repo, recipient, target := unarchiveSlotScenario(t, 2, true)

		m, err := repo.GetByPropertyAndUser(context.Background(), target, recipient)
		if err != nil {
			t.Fatalf("GetByPropertyAndUser: %v", err)
		}
		if m.Status != domain.MemberStatusActive {
			t.Errorf("membership on unarchived property must stay active at exactly the limit, got %q", m.Status)
		}
		if got := activePropertyIDs(t, repo, recipient); len(got) != 2 || !got[target] {
			t.Errorf("active slots after unarchive = %v, want both properties", got)
		}
	})
}

// TestPropertyLifecycle_DeleteDropsMembershipsAndRecoversFIFO covers AC:
// deleting a property removes its memberships (RecoverAfterPropertyDelete
// drops them before the property row is removed; the FK cascade would take
// them otherwise — identical for cascade and detach delete modes), cascade-
// deletes the pending invitations with the property row, and the freed slot
// recovers the recipient's oldest suspended membership FIFO.
func TestPropertyLifecycle_DeleteDropsMembershipsAndRecoversFIFO(t *testing.T) {
	t.Parallel()
	f := newAccessLifecycleFixture(t)
	ctx := f.bg()
	repo := NewMembershipRepository(f.tx)

	owner := f.addUserWithEmail(t, f.email("owner"))
	recipient := f.addUserWithEmail(t, f.email("recipient"))
	target := f.addProperty(t, owner, "Квартира на Невском")
	other := f.addProperty(t, owner, "Дача у моря")
	f.limiter.set(recipient, 1)

	// The active membership on target occupies the only slot; the suspended
	// membership on other waits for a free one.
	addMembership(t, repo, target, recipient, owner)
	addSuspendedMembership(t, repo, f.tx, other, recipient, owner, time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC))
	if _, err := f.invites.InviteByEmail(ctx, owner, target, "pending@example.com", domain.RoleViewer); err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}

	// The slot-significant delete steps in the PropertyService.DeleteProperty
	// order: drop the memberships and recover the freed slots first, then
	// remove the property row.
	if _, err := f.slots.RecoverAfterPropertyDelete(ctx, accessNoCommitTx{f.tx}, target); err != nil {
		t.Fatalf("RecoverAfterPropertyDelete: %v", err)
	}

	// The membership rows on the deleted object are already gone.
	if _, err := repo.GetByPropertyAndUser(ctx, target, recipient); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("expected ErrMemberNotFound for the dropped membership, got %v", err)
	}

	// The freed slot reactivates the suspended membership FIFO.
	m, err := repo.GetByPropertyAndUser(ctx, other, recipient)
	if err != nil {
		t.Fatalf("GetByPropertyAndUser other: %v", err)
	}
	if m.Status != domain.MemberStatusActive {
		t.Errorf("suspended membership must be reactivated after the delete freed a slot, got %q", m.Status)
	}
	if got := activePropertyIDs(t, repo, recipient); len(got) != 1 || !got[other] {
		t.Errorf("active slots after delete = %v, want only the other property", got)
	}

	// The property row removal cascade-deletes the pending invitations (FK).
	if err := f.q.DeleteProperty(ctx, genpostgres.DeletePropertyParams{ID: pgUUID(target), OwnerID: pgUUID(owner)}); err != nil {
		t.Fatalf("DeleteProperty: %v", err)
	}
	if _, err := f.q.GetPropertyByID(ctx, pgUUID(target)); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected the property row to be gone, got %v", err)
	}
	invitations, err := f.q.ListPropertyMemberInvitations(ctx, pgUUID(target))
	if err != nil {
		t.Fatalf("ListPropertyMemberInvitations: %v", err)
	}
	if len(invitations) != 0 {
		t.Errorf("pending invitations after delete = %d, want 0 (FK cascade)", len(invitations))
	}
}
