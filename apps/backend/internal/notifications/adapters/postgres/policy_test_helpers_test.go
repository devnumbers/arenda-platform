package postgres

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	accesspg "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/postgres"
	accessdomain "github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// Shared fixtures of the notifications postgres policy integration tests:
// a rolled-back transaction over a real Postgres gated by TEST_DATABASE_URL,
// plus seeding helpers for users, properties and memberships.

func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// policyTestClock is a fixed clock for the service under test.
type policyTestClock struct{ now time.Time }

func (c policyTestClock) Now() time.Time { return c.now }

// policyTestTzResolver resolves every owner's timezone as UTC.
type policyTestTzResolver struct{}

func (policyTestTzResolver) Resolve(context.Context, uuid.UUID) (*time.Location, error) {
	return time.UTC, nil
}

func setupPolicyDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	// Tests run in parallel and each holds its own pool for a single rollback
	// transaction. The production DefaultPoolConfig keeps MinConns=16 warm per
	// pool; across the parallel fixtures that is hundreds of connections
	// against one PostgreSQL, so the test pools stay minimal.
	cfg := database.DefaultPoolConfig()
	cfg.MinConns = 0
	cfg.MaxConns = 2
	ctx := context.Background()
	pool, err := database.NewPoolWithConfig(ctx, databaseURL, cfg)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func beginPolicyTx(t *testing.T, pool *pgxpool.Pool) (context.Context, pgx.Tx, func()) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	return ctx, tx, func() {
		// The rollback failure is surfaced as a test error rather than
		// discarded: these tests share the integration database, so a leaked
		// transaction would poison sibling tests.
		if err := tx.Rollback(ctx); err != nil {
			t.Errorf("rollback policy tx: %v", err)
		}
	}
}

func createPolicyTestUser(t *testing.T, ctx context.Context, q *genpostgres.Queries) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	// The phone must be unique per call: derive it from the uuid's random tail.
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

func createPolicyTestProperty(t *testing.T, ctx context.Context, q *genpostgres.Queries, owner uuid.UUID) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	_, err = q.CreateProperty(ctx, genpostgres.CreatePropertyParams{
		ID:          pgUUID(id),
		OwnerID:     pgUUID(owner),
		Name:        "Policy Test Property",
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

func addPolicyMembership(
	t *testing.T, ctx context.Context, repo *accesspg.MembershipRepository,
	property, user, grantedBy uuid.UUID, role accessdomain.Role,
) {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := repo.Create(ctx, accessdomain.Membership{
		ID: id, PropertyID: property, UserID: user, Role: role, GrantedBy: grantedBy,
	}); err != nil {
		t.Fatalf("create membership: %v", err)
	}
}

func addSuspendedPolicyMembership(
	t *testing.T, ctx context.Context, repo *accesspg.MembershipRepository,
	property, user, grantedBy uuid.UUID, role accessdomain.Role,
) {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := repo.CreateWithStatus(ctx, accessdomain.Membership{
		ID: id, PropertyID: property, UserID: user, Role: role, GrantedBy: grantedBy,
		Status: accessdomain.MemberStatusSuspended,
	}); err != nil {
		t.Fatalf("create suspended membership: %v", err)
	}
}
