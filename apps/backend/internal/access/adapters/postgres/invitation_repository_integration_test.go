package postgres

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// Integration tests for the invitation repository (issue #161, T5). They run
// against a real Postgres via TEST_DATABASE_URL and are skipped when it is
// unset, same convention as member_repository_integration_test.go (whose
// fixtures this file reuses).

func TestInvitationRepository_CreateGetList(t *testing.T) {
	pool := setupAccessDB(t)
	ctx, tx, cleanup := beginAccessTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	owner := createAccessTestUser(t, ctx, q)
	property := createAccessTestProperty(t, ctx, q, owner)
	repo := NewInvitationRepository(tx)

	invID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	created, err := repo.Create(ctx, domain.Invitation{
		ID:         invID,
		PropertyID: property,
		Email:      "invitee@example.com",
		Role:       domain.RoleViewer,
		InvitedBy:  owner,
		LastSentAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Role != domain.RoleViewer || created.Email != "invitee@example.com" {
		t.Errorf("created = %+v", created)
	}
	if created.CreatedAt.IsZero() || created.LastSentAt.IsZero() {
		t.Errorf("timestamps must be set: %+v", created)
	}

	// GetByID round-trip.
	got, err := repo.GetByID(ctx, invID, property)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != invID {
		t.Errorf("GetByID id = %v, want %v", got.ID, invID)
	}

	// Unknown id / other property is not found.
	if _, err := repo.GetByID(ctx, uuid.New(), property); !errors.Is(err, domain.ErrInvitationNotFound) {
		t.Errorf("GetByID unknown: expected ErrInvitationNotFound, got %v", err)
	}
	if _, err := repo.GetByID(ctx, invID, uuid.New()); !errors.Is(err, domain.ErrInvitationNotFound) {
		t.Errorf("GetByID other property: expected ErrInvitationNotFound, got %v", err)
	}

	// Email lookup is case-insensitive on both sides.
	if _, err := repo.GetByPropertyAndEmail(ctx, property, "Invitee@Example.com"); err != nil {
		t.Errorf("GetByPropertyAndEmail case-insensitive: %v", err)
	}
	if _, err := repo.GetByPropertyAndEmail(ctx, property, "nobody@example.com"); !errors.Is(err, domain.ErrInvitationNotFound) {
		t.Errorf("GetByPropertyAndEmail unknown: expected ErrInvitationNotFound, got %v", err)
	}

	list, err := repo.ListByProperty(ctx, property)
	if err != nil {
		t.Fatalf("ListByProperty: %v", err)
	}
	if len(list) != 1 || list[0].ID != invID {
		t.Errorf("ListByProperty = %+v", list)
	}
}

func TestInvitationRepository_UniquePropertyEmail(t *testing.T) {
	pool := setupAccessDB(t)
	ctx, tx, cleanup := beginAccessTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	owner := createAccessTestUser(t, ctx, q)
	property := createAccessTestProperty(t, ctx, q, owner)
	repo := NewInvitationRepository(tx)

	mustCreate := func(email string) {
		t.Helper()
		id, err := uuid.NewV7()
		if err != nil {
			t.Fatalf("new uuid: %v", err)
		}
		if _, err := repo.Create(ctx, domain.Invitation{
			ID:         id,
			PropertyID: property,
			Email:      email,
			Role:       domain.RoleViewer,
			InvitedBy:  owner,
			LastSentAt: time.Now().UTC(),
		}); err != nil {
			t.Fatalf("Create %s: %v", email, err)
		}
	}

	mustCreate("invitee@example.com")

	// A duplicate (property, email) — even in a different casing — violates the
	// unique index on (property_id, lower(email)).
	dupID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := repo.Create(ctx, domain.Invitation{
		ID:         dupID,
		PropertyID: property,
		Email:      "INVITEE@example.com",
		Role:       domain.RoleFullAccess,
		InvitedBy:  owner,
		LastSentAt: time.Now().UTC(),
	}); err == nil {
		t.Error("duplicate (property, lower(email)) insert must fail")
	}
}

func TestInvitationRepository_ListPendingByEmailFIFO(t *testing.T) {
	pool := setupAccessDB(t)
	ctx, tx, cleanup := beginAccessTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	owner := createAccessTestUser(t, ctx, q)
	propA := createAccessTestProperty(t, ctx, q, owner)
	propB := createAccessTestProperty(t, ctx, q, owner)
	repo := NewInvitationRepository(tx)

	base := time.Now().UTC().Add(-time.Hour)
	// Insert propB first, then propA, and pin explicit created_at values
	// afterwards: within a single transaction now() is identical for both
	// rows, so without the explicit timestamps the test would only check
	// insertion-order stability, not ORDER BY created_at ASC. A is the older
	// invitation and must come first even though it is inserted second.
	idB, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := repo.Create(ctx, domain.Invitation{
		ID:         idB,
		PropertyID: propB,
		Email:      "fifo@example.com",
		Role:       domain.RoleFullAccess,
		InvitedBy:  owner,
		LastSentAt: base,
	}); err != nil {
		t.Fatalf("Create B: %v", err)
	}
	idA, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := repo.Create(ctx, domain.Invitation{
		ID:         idA,
		PropertyID: propA,
		Email:      "fifo@example.com",
		Role:       domain.RoleViewer,
		InvitedBy:  owner,
		LastSentAt: base,
	}); err != nil {
		t.Fatalf("Create A: %v", err)
	}
	// A different email must not leak into the result.
	idOther, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := repo.Create(ctx, domain.Invitation{
		ID:         idOther,
		PropertyID: propA,
		Email:      "other@example.com",
		Role:       domain.RoleViewer,
		InvitedBy:  owner,
		LastSentAt: base,
	}); err != nil {
		t.Fatalf("Create other: %v", err)
	}

	// Pin distinct created_at values: A older than B.
	if _, err := tx.Exec(ctx,
		"UPDATE property_member_invitations SET created_at = $1 WHERE id = $2",
		base, idA); err != nil {
		t.Fatalf("pin created_at A: %v", err)
	}
	if _, err := tx.Exec(ctx,
		"UPDATE property_member_invitations SET created_at = $1 WHERE id = $2",
		base.Add(time.Minute), idB); err != nil {
		t.Fatalf("pin created_at B: %v", err)
	}

	pending, err := repo.ListPendingByEmail(ctx, "FIFO@example.com")
	if err != nil {
		t.Fatalf("ListPendingByEmail: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending invitations, got %+v", pending)
	}
	if pending[0].ID != idA || pending[1].ID != idB {
		t.Errorf("FIFO order = [%v %v], want [%v %v]", pending[0].ID, pending[1].ID, idA, idB)
	}
}

func TestInvitationRepository_UpdateRoleLastSentDelete(t *testing.T) {
	pool := setupAccessDB(t)
	ctx, tx, cleanup := beginAccessTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	owner := createAccessTestUser(t, ctx, q)
	property := createAccessTestProperty(t, ctx, q, owner)
	repo := NewInvitationRepository(tx)

	invID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := repo.Create(ctx, domain.Invitation{
		ID:         invID,
		PropertyID: property,
		Email:      "invitee@example.com",
		Role:       domain.RoleViewer,
		InvitedBy:  owner,
		LastSentAt: time.Now().UTC().Add(-48 * time.Hour),
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := repo.UpdateRole(ctx, invID, property, domain.RoleFullAccess)
	if err != nil {
		t.Fatalf("UpdateRole: %v", err)
	}
	if updated.Role != domain.RoleFullAccess {
		t.Errorf("role = %v, want full_access", updated.Role)
	}
	if !updated.UpdatedAt.After(updated.CreatedAt) && !updated.UpdatedAt.Equal(updated.CreatedAt) {
		t.Errorf("updated_at must be maintained by the trigger: %+v", updated)
	}

	newSent := time.Now().UTC()
	if err := repo.UpdateLastSentAt(ctx, invID, property, newSent); err != nil {
		t.Fatalf("UpdateLastSentAt: %v", err)
	}
	got, err := repo.GetByID(ctx, invID, property)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.LastSentAt.Sub(newSent) > time.Second || newSent.Sub(got.LastSentAt) > time.Second {
		t.Errorf("last_sent_at = %v, want ~%v", got.LastSentAt, newSent)
	}

	if err := repo.Delete(ctx, invID, property); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, invID, property); !errors.Is(err, domain.ErrInvitationNotFound) {
		t.Errorf("deleted invitation: expected ErrInvitationNotFound, got %v", err)
	}
}
