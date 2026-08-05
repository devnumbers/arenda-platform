package postgres

import (
	"context"
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
	return ctx, tx, func() { _ = tx.Rollback(ctx) }
}

func createAccessTestUser(t *testing.T, ctx context.Context, q *genpostgres.Queries) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	_, err = q.CreateUser(ctx, genpostgres.CreateUserParams{
		ID:    pgUUID(id),
		Phone: fmt.Sprintf("+7999%010d", id.Time()%1e10),
		Role:  "owner",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return id
}

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
		Type:        "apartment",
		Address:     "",
		Description: pgtype.Text{},
		Attributes:  []byte("{}"),
		Status:      "active",
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
	id1, _ := uuid.NewV7()
	if _, err := repo.Create(ctx, domain.Membership{
		ID: id1, PropertyID: property, UserID: other, Role: domain.RoleViewer, GrantedBy: owner,
	}); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	id2, _ := uuid.NewV7()
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
	id, _ := uuid.NewV7()
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
	missing, _ := uuid.NewV7()
	if _, err := resolver.GetOwnerID(ctx, missing); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("expected ErrMemberNotFound for missing property, got %v", err)
	}
}
