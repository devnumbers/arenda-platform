//go:build integration

// Package testdb provides a shared PostgreSQL fixture for integration tests.
//
// Setup returns a [*pgxpool.Pool] connected to a real PostgreSQL instance with
// all migrations from db/migrations applied. The database is provisioned once
// per test binary and reused across every package that imports testdb:
//
//   - If the TEST_DATABASE_URL environment variable is set, that database is
//     used as-is (existing convention for CI and local runs without Docker).
//   - Otherwise a PostgreSQL 18 container is started via testcontainers-go.
//
// Tests are isolated by truncating every user table (TRUNCATE ... RESTART
// IDENTITY CASCADE) after each test. The cleanup is registered automatically
// via [testing.TB.Cleanup] when [Setup] is called; [Reset] is also exported for
// callers that need to clear state mid-test.
//
// Every file in this package carries the "integration" build tag, so a plain
// `go test ./...` does not compile or run it. Use `go test -tags=integration`
// (or `make backend-test-integration`) to include these tests.
package testdb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
)

// databaseURL is computed once per test binary: it returns TEST_DATABASE_URL
// when set, otherwise starts a PostgreSQL container via testcontainers-go.
var databaseURL = sync.OnceValue(func() string {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}
	return startPostgres()
})

// migrateOnce ensures all migrations are applied exactly once per test binary.
// Migrations are idempotent (golang-migrate tolerates ErrNoChange), but running
// them a second time per binary is wasted work.
var migrateOnce sync.Once

var errMigrate error

// Setup returns a [*pgxpool.Pool] connected to a migrated PostgreSQL database
// and registers a [Reset] call via t.Cleanup so each test starts from a clean
// slate. The underlying database (external or container) is shared across all
// tests in the binary; only the per-test pool is new.
//
// Setup must be called from within a test (not TestMain) so that t.Cleanup is
// tied to the correct test lifecycle.
func Setup(tb testing.TB) *pgxpool.Pool {
	tb.Helper()

	url := databaseURL()

	migrateOnce.Do(func() {
		errMigrate = database.MigrateUp(url, migrationsDir())
	})
	if errMigrate != nil {
		tb.Fatalf("testdb: migrate up: %v", errMigrate)
	}

	pool, err := database.NewPool(context.Background(), url)
	if err != nil {
		tb.Fatalf("testdb: new pool: %v", err)
	}

	tb.Cleanup(func() {
		pool.Close()
	})
	tb.Cleanup(func() {
		Reset(tb, pool)
	})

	return pool
}

// Reset truncates every user table in the public schema. It is called
// automatically by [Setup] via t.Cleanup; calling it manually is only needed
// when a test must clear state in the middle of its own execution.
//
// The table list is queried from information_schema on each call so it stays
// in sync with the migrated schema without manual maintenance. CASCADE follows
// foreign-key edges, and RESTART IDENTITY resets any serial/identity sequences
// so auto-generated IDs are predictable across tests.
func Reset(tb testing.TB, pool *pgxpool.Pool) {
	tb.Helper()

	ctx := context.Background()

	rows, err := pool.Query(ctx, `
		SELECT quote_ident(table_name)
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
	`)
	if err != nil {
		tb.Fatalf("testdb: list tables: %v", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			tb.Fatalf("testdb: scan table name: %v", err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		tb.Fatalf("testdb: iterate tables: %v", err)
	}

	if len(tables) == 0 {
		// No tables means migrations did not run — a bug, not an empty schema.
		tb.Fatalf("testdb: no user tables found — migrations not applied?")
	}

	stmt := fmt.Sprintf(
		"TRUNCATE TABLE %s RESTART IDENTITY CASCADE",
		strings.Join(tables, ", "),
	)
	if _, err := pool.Exec(ctx, stmt); err != nil {
		tb.Fatalf("testdb: truncate tables: %v", err)
	}
}

// migrationsDir resolves the absolute path to db/migrations relative to this
// file's location: internal/platform/database/testdb → apps/backend/db/migrations
// is four directory levels up from testdb (database → platform → internal → backend).
// Using runtime.Caller avoids hardcoding a relative path that breaks when the
// caller package lives at a different directory depth.
func migrationsDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("testdb: runtime.Caller failed — cannot locate migrations dir")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "db", "migrations")
}
