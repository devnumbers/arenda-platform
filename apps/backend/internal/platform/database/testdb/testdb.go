//go:build integration

// Package testdb provides a shared PostgreSQL fixture for integration tests.
//
// Setup returns a [*pgxpool.Pool] connected to a real PostgreSQL instance with
// all migrations from db/migrations applied. The server is provisioned once
// per test binary and reused across every package that imports testdb:
//
//   - If the TEST_DATABASE_URL environment variable is set, that server is
//     used as-is (existing convention for CI and local runs without Docker).
//   - Otherwise a PostgreSQL 18 container is started via testcontainers-go.
//
// Isolation is per-database: once per binary a template database is created
// and migrated, and every Setup call copies it (CREATE DATABASE ... TEMPLATE)
// into a private database that is dropped on test cleanup. No cross-test
// state exists, so tests from one binary may run in parallel (t.Parallel,
// remediation wave #371). Truncate-based isolation could not support that:
// one test's cleanup TRUNCATE would wipe a concurrently running test's rows.
//
// The per-test copy requires CREATEDB privilege for the connecting role; the
// testcontainers and docker-compose fixtures connect as the image's superuser
// role, and DROP DATABASE ... WITH (FORCE) requires PostgreSQL 13+ (the repo
// baseline is 18). TestMain drops the template on exit; only a crashed binary
// can leave arenda_tpl_* / arenda_test_* databases behind on a persistent
// TEST_DATABASE_URL server — ephemeral CI runners and containers discard them
// with the host.
//
// Every file in this package carries the "integration" build tag, so a plain
// `go test ./...` does not compile or run it. Use `go test -tags=integration`
// (or `make backend-test-integration`) to include these tests.
package testdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
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

// templateDatabase is computed once per test binary: it creates a
// uniquely-named empty database, applies every migration to it, and returns
// its name. Every [Setup] call then clones this template, so migrations run
// once per binary no matter how many tests (or parallel tests) call Setup.
// A failure to create or migrate the template is fatal for the entire test
// binary, so the function panics; [sync.OnceValue] propagates the panic to
// the first caller of Setup, which surfaces it as a test failure. The name is
// also remembered in [createdTemplate] so TestMain drops it on exit.
var templateDatabase = sync.OnceValue(func() string {
	baseURL := databaseURL()
	name := "arenda_tpl_" + randSuffix()

	if err := execOnMaintenanceDB(baseURL, fmt.Sprintf(`CREATE DATABASE %q`, name)); err != nil {
		panic(fmt.Errorf("testdb: create template database: %w", err))
	}
	if err := database.MigrateUp(withDatabase(baseURL, name), migrationsDir()); err != nil {
		panic(fmt.Errorf("testdb: migrate template: %w", err))
	}
	createdTemplate.Store(&name)
	return name
})

// createdTemplate remembers the per-binary template database name once it
// exists, so [dropTemplate] (wired into TestMain) can remove it after all
// tests finish. An atomic because the write happens on a test goroutine
// inside [templateDatabase] while TestMain reads it after m.Run.
var createdTemplate atomic.Pointer[string]

// cloneMu serializes CREATE DATABASE ... TEMPLATE calls: concurrent clones of
// one template are not guaranteed to queue cleanly inside PostgreSQL, and the
// copy itself is fast, so a coarse lock costs nothing.
var cloneMu sync.Mutex

// execOnMaintenanceDB runs one DDL statement (create/drop database) over a
// short-lived connection to the server's postgres maintenance database. The
// context timeout caps a hang on a wedged server; the close error is folded
// in so cleanup never masks the statement's result.
func execOnMaintenanceDB(baseURL, statement string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, withDatabase(baseURL, "postgres"))
	if err != nil {
		return fmt.Errorf("connect maintenance database: %w", err)
	}
	_, execErr := conn.Exec(ctx, statement)
	closeErr := conn.Close(ctx)
	return errors.Join(execErr, closeErr)
}

// cloneTemplateDatabase copies the migrated template into a fresh private
// database and returns its name. Callers own the clone and must drop it.
func cloneTemplateDatabase(baseURL, template string) (string, error) {
	name := "arenda_test_" + randSuffix()
	cloneMu.Lock()
	defer cloneMu.Unlock()
	if err := execOnMaintenanceDB(baseURL,
		fmt.Sprintf(`CREATE DATABASE %q TEMPLATE %q`, name, template),
	); err != nil {
		return "", fmt.Errorf("clone template database: %w", err)
	}
	return name, nil
}

// dropTemplate removes the per-binary template database when one was created.
// It runs from TestMain after all tests: parallel Setup calls share the
// template, so only binary exit is a safe drop point. A failure is logged,
// not fatal — by then every test has already passed or failed.
func dropTemplate() {
	if stored := createdTemplate.Load(); stored != nil {
		if err := execOnMaintenanceDB(databaseURL(),
			fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, *stored),
		); err != nil {
			logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
			logger.ErrorContext(context.Background(), "testdb: drop template database", "error", err)
		}
	}
}

// Setup returns a [*pgxpool.Pool] connected to a private clone of the
// migrated template database and registers cleanup that closes the pool and
// drops the clone. Each test gets its own database, so parallel tests never
// observe each other's rows.
//
// The per-test pool is bounded (MinConns 0 / MaxConns 4): the production
// DefaultPoolConfig keeps MinConns=16 warm per pool, which exhausts
// max_connections once a dozen parallel tests each hold their own pool.
//
// Setup must be called from within a test (not TestMain) so that cleanup is
// tied to the correct test lifecycle.
func Setup(tb testing.TB) *pgxpool.Pool {
	tb.Helper()

	baseURL := databaseURL()
	template := templateDatabase()
	name, err := cloneTemplateDatabase(baseURL, template)
	if err != nil {
		tb.Fatalf("testdb: %v", err)
	}

	cfg := database.DefaultPoolConfig()
	cfg.MinConns = 0
	cfg.MaxConns = 4
	pool, err := database.NewPoolWithConfig(context.Background(), withDatabase(baseURL, name), cfg)
	if err != nil {
		tb.Fatalf("testdb: new pool: %v", err)
	}

	tb.Cleanup(func() {
		pool.Close()
		dropDatabase(tb, baseURL, name)
	})
	return pool
}

// Reset truncates every user table in the public schema of the pool's
// database. With per-test database isolation it is only needed when a test
// must clear its own state in the middle of its execution.
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

// dropDatabase removes a per-test clone. WITH (FORCE) terminates any
// connection the pool failed to close; a failure is reported through the
// test because a leftover database on a persistent TEST_DATABASE_URL server
// is an isolation leak, not just noise.
func dropDatabase(tb testing.TB, baseURL, name string) {
	tb.Helper()
	if err := execOnMaintenanceDB(baseURL,
		fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, name),
	); err != nil {
		tb.Errorf("testdb: drop database %s: %v", name, err)
	}
}

// withDatabase rewrites the database name in a PostgreSQL URL, preserving
// every other component (credentials, host, port, query parameters).
func withDatabase(baseURL, name string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		panic(fmt.Errorf("testdb: parse database url: %w", err))
	}
	u.Path = "/" + name
	return u.String()
}

// randSuffix returns a random hex string unique enough to name per-binary
// templates and per-test databases without coordination.
func randSuffix() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Errorf("testdb: random suffix: %w", err))
	}
	return hex.EncodeToString(b[:])
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
