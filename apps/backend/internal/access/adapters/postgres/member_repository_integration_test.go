package postgres

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// pgUUID converts a uuid.UUID to the pgtype.UUID expected by sqlc-generated code.
func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// These integration tests run against a real Postgres via TEST_DATABASE_URL and
// are skipped when it is unset (same convention as the billing integration
// tests). They exercise the membership repository + owner resolver end-to-end.

func setupAccessDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := database.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func beginAccessTx(t *testing.T, pool *pgxpool.Pool) (context.Context, pgx.Tx, func()) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	return ctx, tx, func() {
		if err := tx.Rollback(ctx); err != nil {
			// Rollback failure means test isolation broke: rows written in the
			// aborted test transaction would persist in the shared database.
			t.Errorf("rollback access test tx: %v", err)
		}
	}
}

func createAccessTestUser(t *testing.T, ctx context.Context, q *genpostgres.Queries) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	// The phone must be unique per call: deriving it from the uuid timestamp
	// collides for users created within the same millisecond (V7 timestamps
	// have millisecond precision), and CreateUser's ON CONFLICT DO NOTHING
	// then returns no rows. Deriving from the uuid's random tail avoids that.
	_, err = q.CreateUser(ctx, genpostgres.CreateUserParams{
		ID:    pgUUID(id),
		Phone: fmt.Sprintf("+7999%07d", binary.BigEndian.Uint32(id[12:])%10000000),
		Role:  "owner",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return id
}

// propertyTypeApartment and statusActive are the column fixture values used by
// createAccessTestProperty, its inline copies, and the lease fixtures.
const (
	propertyTypeApartment = "apartment"
	statusActive          = "active"
)

func createAccessTestProperty(t *testing.T, ctx context.Context, q *genpostgres.Queries, owner uuid.UUID) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	_, err = q.CreateProperty(ctx, genpostgres.CreatePropertyParams{
		ID:          pgUUID(id),
		OwnerID:     pgUUID(owner),
		Name:        "Test Property",
		Type:        propertyTypeApartment,
		Address:     "",
		Description: pgtype.Text{},
		Attributes:  []byte("{}"),
		Status:      statusActive,
	})
	if err != nil {
		t.Fatalf("create property: %v", err)
	}
	return id
}

func TestMembershipRepository_CreateAndGet(t *testing.T) {
	pool := setupAccessDB(t)
	ctx, tx, cleanup := beginAccessTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	owner := createAccessTestUser(t, ctx, q)
	other := createAccessTestUser(t, ctx, q)
	property := createAccessTestProperty(t, ctx, q, owner)

	repo := NewMembershipRepository(tx)
	membershipID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	created, err := repo.Create(ctx, domain.Membership{
		ID:         membershipID,
		PropertyID: property,
		UserID:     other,
		Role:       domain.RoleFullAccess,
		GrantedBy:  owner,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Role != domain.RoleFullAccess {
		t.Errorf("created role = %v, want full_access", created.Role)
	}

	got, err := repo.GetByPropertyAndUser(ctx, property, other)
	if err != nil {
		t.Fatalf("GetByPropertyAndUser: %v", err)
	}
	if got.UserID != other {
		t.Errorf("got user = %v, want %v", got.UserID, other)
	}

	role, err := repo.GetRole(ctx, property, other)
	if err != nil {
		t.Fatalf("GetRole: %v", err)
	}
	if role != domain.RoleFullAccess {
		t.Errorf("GetRole = %v, want full_access", role)
	}

	// Missing membership maps to ErrMemberNotFound.
	if _, err := repo.GetByPropertyAndUser(ctx, property, owner); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("expected ErrMemberNotFound, got %v", err)
	}
}

func TestMembershipRepository_DuplicateUnique(t *testing.T) {
	pool := setupAccessDB(t)
	ctx, tx, cleanup := beginAccessTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	owner := createAccessTestUser(t, ctx, q)
	other := createAccessTestUser(t, ctx, q)
	property := createAccessTestProperty(t, ctx, q, owner)

	repo := NewMembershipRepository(tx)
	id1 := uuid.Must(uuid.NewV7())
	if _, err := repo.Create(ctx, domain.Membership{
		ID: id1, PropertyID: property, UserID: other, Role: domain.RoleViewer, GrantedBy: owner,
	}); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	id2 := uuid.Must(uuid.NewV7())
	_, err := repo.Create(ctx, domain.Membership{
		ID: id2, PropertyID: property, UserID: other, Role: domain.RoleFullAccess, GrantedBy: owner,
	})
	if err == nil {
		t.Fatalf("expected duplicate (property, user) violation, got nil")
	}
}

func TestMembershipRepository_UpdateRoleAndDelete(t *testing.T) {
	pool := setupAccessDB(t)
	ctx, tx, cleanup := beginAccessTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	owner := createAccessTestUser(t, ctx, q)
	other := createAccessTestUser(t, ctx, q)
	property := createAccessTestProperty(t, ctx, q, owner)

	repo := NewMembershipRepository(tx)
	id := uuid.Must(uuid.NewV7())
	if _, err := repo.Create(ctx, domain.Membership{
		ID: id, PropertyID: property, UserID: other, Role: domain.RoleViewer, GrantedBy: owner,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := repo.UpdateRole(ctx, id, property, domain.RoleFullAccess)
	if err != nil {
		t.Fatalf("UpdateRole: %v", err)
	}
	if updated.Role != domain.RoleFullAccess {
		t.Errorf("updated role = %v, want full_access", updated.Role)
	}

	if err := repo.Delete(ctx, id, property); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, id, property); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("expected ErrMemberNotFound after delete, got %v", err)
	}
}

// TestMembershipRepository_SuspendedMembershipVisible verifies the T9 lookup
// contract: GetByPropertyAndUser sees a suspended membership (so the policy
// can return RoleSuspended), while the active-only GetRole projection treats
// it as absent.
func TestMembershipRepository_SuspendedMembershipVisible(t *testing.T) {
	pool := setupAccessDB(t)
	ctx, tx, cleanup := beginAccessTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	owner := createAccessTestUser(t, ctx, q)
	other := createAccessTestUser(t, ctx, q)
	property := createAccessTestProperty(t, ctx, q, owner)

	repo := NewMembershipRepository(tx)
	id := uuid.Must(uuid.NewV7())
	if _, err := repo.Create(ctx, domain.Membership{
		ID: id, PropertyID: property, UserID: other, Role: domain.RoleViewer, GrantedBy: owner,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Suspend(ctx, id, property); err != nil {
		t.Fatalf("Suspend: %v", err)
	}

	got, err := repo.GetByPropertyAndUser(ctx, property, other)
	if err != nil {
		t.Fatalf("GetByPropertyAndUser: %v", err)
	}
	if !got.IsSuspended() {
		t.Errorf("expected suspended membership, got status %q", got.Status)
	}

	if _, err := repo.GetRole(ctx, property, other); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("expected GetRole to hide suspended membership as ErrMemberNotFound, got %v", err)
	}
}

func TestOwnerResolver_ReturnsOwner(t *testing.T) {
	pool := setupAccessDB(t)
	ctx, tx, cleanup := beginAccessTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	owner := createAccessTestUser(t, ctx, q)
	property := createAccessTestProperty(t, ctx, q, owner)

	resolver := NewOwnerResolver(tx)
	got, err := resolver.GetOwnerID(ctx, property)
	if err != nil {
		t.Fatalf("GetOwnerID: %v", err)
	}
	if got != owner {
		t.Errorf("owner = %v, want %v", got, owner)
	}

	// Missing property does not leak existence.
	missing := uuid.Must(uuid.NewV7())
	if _, err := resolver.GetOwnerID(ctx, missing); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("expected ErrMemberNotFound for missing property, got %v", err)
	}
}

// setAccessPropertyStatus flips the property's lifecycle through the sqlc
// queries the property service uses (archive or unarchive).
func setAccessPropertyStatus(
	t *testing.T, ctx context.Context, q *genpostgres.Queries, archive bool, owner uuid.UUID, props ...uuid.UUID,
) {
	t.Helper()
	for _, prop := range props {
		var err error
		if archive {
			_, err = q.ArchiveProperty(ctx, genpostgres.ArchivePropertyParams{ID: pgUUID(prop), OwnerID: pgUUID(owner)})
		} else {
			_, err = q.UnarchiveProperty(ctx, genpostgres.UnarchivePropertyParams{ID: pgUUID(prop), OwnerID: pgUUID(owner)})
		}
		if err != nil {
			t.Fatalf("property status flip %s: %v", prop, err)
		}
	}
}

// assertSlotAccounting checks the recipient's slot accounting: the number of
// used slots, which properties occupy them, and how many suspended memberships
// wait in the FIFO recovery queue. The stage argument names the assertion phase.
func assertSlotAccounting(
	t *testing.T, ctx context.Context, repo *MembershipRepository,
	recipient uuid.UUID, stage string, wantActive []uuid.UUID, wantSuspended int,
) {
	t.Helper()
	if count, err := repo.CountActiveByUser(ctx, recipient); err != nil || count != len(wantActive) {
		t.Errorf("CountActiveByUser %s = %d, %v; want %d", stage, count, err, len(wantActive))
	}
	rows, err := repo.ListActiveByUser(ctx, recipient)
	if err != nil {
		t.Fatalf("ListActiveByUser %s: %v", stage, err)
	}
	got := make(map[uuid.UUID]bool, len(rows))
	for _, m := range rows {
		got[m.PropertyID] = true
	}
	for _, prop := range wantActive {
		if !got[prop] {
			t.Errorf("ListActiveByUser %s = %+v, want %s among the active slots", stage, rows, prop)
		}
	}
	if len(rows) != len(wantActive) {
		t.Errorf("ListActiveByUser %s = %d rows, want %d", stage, len(rows), len(wantActive))
	}
	if suspended, err := repo.ListSuspendedByUser(ctx, recipient); err != nil || len(suspended) != wantSuspended {
		t.Errorf("ListSuspendedByUser %s = %d, %v; want %d", stage, len(suspended), err, wantSuspended)
	}
}

// TestMembershipRepository_ArchivedPropertyExcludedFromSlotQueries verifies the
// issue #163 slot-accounting rule: memberships on archived properties occupy no
// recipient tariff slot — they are excluded from ListActiveByUser,
// CountActiveByUser and the FIFO recovery queue ListSuspendedByUser — and
// re-enter the selection when the property is unarchived.
func TestMembershipRepository_ArchivedPropertyExcludedFromSlotQueries(t *testing.T) {
	pool := setupAccessDB(t)
	ctx, tx, cleanup := beginAccessTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	owner := createAccessTestUser(t, ctx, q)
	recipient := createAccessTestUser(t, ctx, q)
	activeProp := createAccessTestProperty(t, ctx, q, owner)
	archivedActiveProp := createAccessTestProperty(t, ctx, q, owner)
	archivedSuspendedProp := createAccessTestProperty(t, ctx, q, owner)

	repo := NewMembershipRepository(tx)
	activeID := uuid.Must(uuid.NewV7())
	if _, err := repo.Create(ctx, domain.Membership{
		ID: activeID, PropertyID: activeProp, UserID: recipient, Role: domain.RoleViewer, GrantedBy: owner,
	}); err != nil {
		t.Fatalf("Create active: %v", err)
	}
	archivedActiveID := uuid.Must(uuid.NewV7())
	if _, err := repo.Create(ctx, domain.Membership{
		ID: archivedActiveID, PropertyID: archivedActiveProp, UserID: recipient, Role: domain.RoleViewer, GrantedBy: owner,
	}); err != nil {
		t.Fatalf("Create archived-active: %v", err)
	}
	archivedSuspendedID := uuid.Must(uuid.NewV7())
	if _, err := repo.CreateWithStatus(ctx, domain.Membership{
		ID: archivedSuspendedID, PropertyID: archivedSuspendedProp, UserID: recipient, Role: domain.RoleViewer, GrantedBy: owner,
		Status: domain.MemberStatusSuspended,
	}); err != nil {
		t.Fatalf("Create archived-suspended: %v", err)
	}

	// Sanity: before archiving everything is visible to the slot queries.
	if count, err := repo.CountActiveByUser(ctx, recipient); err != nil || count != 2 {
		t.Fatalf("CountActiveByUser before archive = %d, %v; want 2", count, err)
	}
	if rows, err := repo.ListSuspendedByUser(ctx, recipient); err != nil || len(rows) != 1 {
		t.Fatalf("ListSuspendedByUser before archive = %d, %v; want 1", len(rows), err)
	}

	// Archive two of the three properties: the memberships on them occupy no
	// slot and leave the FIFO recovery queue.
	setAccessPropertyStatus(t, ctx, q, true, owner, archivedActiveProp, archivedSuspendedProp)
	assertSlotAccounting(t, ctx, repo, recipient, "after archive", []uuid.UUID{activeProp}, 0)

	// Unarchive: both memberships re-enter the selection.
	setAccessPropertyStatus(t, ctx, q, false, owner, archivedActiveProp, archivedSuspendedProp)
	assertSlotAccounting(t, ctx, repo, recipient, "after unarchive",
		[]uuid.UUID{activeProp, archivedActiveProp}, 1)
}
