package postgres

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// Integration coverage of the single member read (issue #1357) against a real
// Postgres (TEST_DATABASE_URL; skipped when unset, the access integration
// convention): the row the list shows, read alone through the real policy and
// SQL path — the owner row by the owner's user id, a membership row by its id
// and its user id, a pending invitation row by its invitation id, and the
// privacy-preserving not-found for a stranger, a suspended member and an
// unknown id.

func TestGetMemberLifecycle_OwnerAndMemberRowsOverSQL(t *testing.T) {
	t.Parallel()
	f := newAccessLifecycleFixture(t)
	ctx := f.bg()

	owner := f.addUserWithEmail(t, f.email("owner"))
	property := f.addProperty(t, owner, "Квартира на тесте")
	viewer := f.addUserWithEmail(t, f.email("viewer"))
	f.limiter.set(viewer, 10)
	membership, err := f.access.AddMember(ctx, owner, property, viewer, domain.RoleViewer)
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	// The owner row is addressed by the owner's user id and carries the
	// owner's email resolved over the real user row.
	ownerRow, err := f.invites.GetMember(ctx, owner, property, owner)
	if err != nil {
		t.Fatalf("GetMember owner: %v", err)
	}
	if !ownerRow.IsOwner || ownerRow.UserID != owner {
		t.Errorf("owner row = %+v", ownerRow)
	}
	if ownerRow.Email == nil || *ownerRow.Email != f.email("owner") {
		t.Errorf("owner row email = %v, want %s", ownerRow.Email, f.email("owner"))
	}

	// A member row answers to its membership id and to its user id, both
	// equal to the list's row.
	byID, err := f.invites.GetMember(ctx, owner, property, membership.ID)
	if err != nil {
		t.Fatalf("GetMember by membership id: %v", err)
	}
	if byID.UserID != viewer || byID.Role != sharedpolicy.RoleViewer {
		t.Errorf("member row by id = %+v", byID)
	}
	byUserID, err := f.invites.GetMember(ctx, owner, property, viewer)
	if err != nil {
		t.Fatalf("GetMember by user id: %v", err)
	}
	if byUserID.ID != membership.ID {
		t.Errorf("member row by user id = %+v, want membership %s", byUserID, membership.ID)
	}
}

func TestGetMemberLifecycle_PendingRowByInvitationIDOverSQL(t *testing.T) {
	t.Parallel()
	f := newAccessLifecycleFixture(t)
	ctx := f.bg()

	owner := f.addUserWithEmail(t, f.email("owner"))
	property := f.addProperty(t, owner, "Квартира на тесте")
	pendingEmail := f.email("pending")
	outcome, err := f.invites.InviteByEmail(ctx, owner, property, pendingEmail, domain.RoleViewer)
	if err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}
	if outcome.Invitation == nil {
		t.Fatalf("expected a pending invitation outcome, got %+v", outcome)
	}

	// A pending invitation row answers to its invitation id (the «Аккаунт не
	// создан» card).
	pendingRow, err := f.invites.GetMember(ctx, owner, property, outcome.Invitation.ID)
	if err != nil {
		t.Fatalf("GetMember pending: %v", err)
	}
	if !pendingRow.Pending || pendingRow.Email == nil || *pendingRow.Email != pendingEmail {
		t.Errorf("pending row = %+v", pendingRow)
	}
}

func TestGetMemberLifecycle_PrivacyNotFoundOverSQL(t *testing.T) {
	t.Parallel()
	f := newAccessLifecycleFixture(t)
	ctx := f.bg()

	owner := f.addUserWithEmail(t, f.email("owner"))
	property := f.addProperty(t, owner, "Квартира на тесте")
	viewer := f.addUserWithEmail(t, f.email("viewer"))
	f.limiter.set(viewer, 10)
	membership, err := f.access.AddMember(ctx, owner, property, viewer, domain.RoleViewer)
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	// A stranger with no relation to the object gets the privacy-preserving
	// not-found.
	stranger := f.addUserWithEmail(t, f.email("stranger"))
	if _, err := f.invites.GetMember(ctx, stranger, property, owner); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("stranger owner row: expected ErrMemberNotFound, got %v", err)
	}
	if _, err := f.invites.GetMember(ctx, stranger, property, membership.ID); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("stranger member row: expected ErrMemberNotFound, got %v", err)
	}

	// A suspended member sees nothing, their own row included: the zero free
	// slot budget suspends the grant at creation, and CanView(RoleSuspended)
	// is false (канон #162).
	suspended := f.addUserWithEmail(t, f.email("suspended"))
	f.limiter.set(suspended, 0)
	if _, err := f.access.AddMember(ctx, owner, property, suspended, domain.RoleViewer); err != nil {
		t.Fatalf("suspended AddMember: %v", err)
	}
	sm, err := NewMembershipRepository(f.tx).GetByPropertyAndUser(ctx, property, suspended)
	if err != nil {
		t.Fatalf("suspended membership: %v", err)
	}
	if _, err := f.invites.GetMember(ctx, suspended, property, sm.ID); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("suspended reader: expected ErrMemberNotFound, got %v", err)
	}

	// An authorized reader gets not-found for an id no row carries.
	if _, err := f.invites.GetMember(ctx, owner, property, uuid.Must(uuid.NewV7())); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("unknown id: expected ErrMemberNotFound, got %v", err)
	}
}
