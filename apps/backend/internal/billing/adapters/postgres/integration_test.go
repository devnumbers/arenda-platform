package postgres

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

var (
	migrateOnce sync.Once
	errMigrate  error
)

func setupIntegrationDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	migrateOnce.Do(func() {
		errMigrate = database.MigrateUp(databaseURL, "../../../../db/migrations")
	})
	if errMigrate != nil {
		t.Fatalf("migrate up: %v", errMigrate)
	}

	ctx := context.Background()
	pool, err := database.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func beginTx(t *testing.T, pool *pgxpool.Pool) (context.Context, pgx.Tx, func()) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	return ctx, tx, func() { _ = tx.Rollback(ctx) }
}

func createTestUser(t *testing.T, ctx context.Context, q *genpostgres.Queries) uuid.UUID {
	t.Helper()
	id, err := uuid.NewRandom()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	_, err = q.CreateUser(ctx, genpostgres.CreateUserParams{
		ID:    pgtype.UUID{Bytes: id, Valid: true},
		Phone: fmt.Sprintf("+7999%010d", id.Time()%1e10),
		Role:  "owner",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return id
}

func getTestTariff(t *testing.T, ctx context.Context, q *genpostgres.Queries, name string) genpostgres.Tariff {
	t.Helper()
	tariff, err := q.GetTariffByName(ctx, name)
	if err != nil {
		t.Fatalf("get tariff %q: %v", name, err)
	}
	return tariff
}

func noopEncryptor(t *testing.T) encryption.Encryptor {
	t.Helper()
	enc, err := encryption.NewEncryptor("")
	if err != nil {
		t.Fatalf("create noop encryptor: %v", err)
	}
	return enc
}
