//go:build integration

package database

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// TestMigrationsUpDownUpCycle mirrors the CI "Backend migrations" job
// (.github/workflows/ci.yml): the whole migration chain must apply up, roll
// back to zero, and apply up again. It exists because the CI job was the only
// place down migrations ever ran: the shared testdb fixture applies MigrateUp
// alone, so a broken down chain (issue #313 / #316 — 000104.down dropped
// user_subscriptions that older down migrations still ALTER) stayed green
// locally and red in CI.
//
// The test always starts its own PostgreSQL container instead of reusing
// TEST_DATABASE_URL: down -all destroys the schema, and the shared database
// may be used concurrently by other test binaries in the same run.
func TestMigrationsUpDownUpCycle(t *testing.T) {
	t.Parallel()

	if !dockerAvailable() {
		t.Skip("docker not available — the cycle test always needs its own container")
	}

	ctx := context.Background()

	ctr, err := postgres.Run(ctx, "postgres:18-alpine",
		postgres.WithDatabase("arenda"),
		postgres.WithUsername("arenda"),
		postgres.WithPassword("arenda"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	url, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("container connection string: %v", err)
	}

	dir := migrationsDir(t)

	if err := MigrateUp(url, dir); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	if err := MigrateDownAll(url, dir); err != nil {
		t.Fatalf("migrate down all: %v", err)
	}

	if err := MigrateUp(url, dir); err != nil {
		t.Fatalf("migrate up again: %v", err)
	}

	assertSchemaAtLatestVersion(t, url, dir)
}

// assertSchemaAtLatestVersion checks that schema_migrations points at the
// highest migration version found on disk and that a core table (users, from
// 000001) is queryable — a smoke assertion that the second up really rebuilt
// the schema rather than leaving a dirty state.
func assertSchemaAtLatestVersion(t *testing.T, url, dir string) {
	t.Helper()

	pool, err := NewPool(context.Background(), url)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	defer pool.Close()

	var version int
	if err := pool.QueryRow(context.Background(), "SELECT version FROM schema_migrations").Scan(&version); err != nil {
		t.Fatalf("read schema_migrations version: %v", err)
	}

	latest := latestMigrationVersion(t, dir)
	if version != latest {
		t.Fatalf("schema_migrations version = %d, want %d (latest migration on disk)", version, latest)
	}

	var users int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM users").Scan(&users); err != nil {
		t.Fatalf("SELECT count(*) FROM users: %v", err)
	}
}

// latestMigrationVersion returns the highest `<version>_` prefix found among
// migration files, so the assertion survives new migrations without edits.
func latestMigrationVersion(t *testing.T, dir string) int {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}

	nameVersion := regexp.MustCompile(`^(\d+)_`)
	latest := 0
	for _, e := range entries {
		if m := nameVersion.FindStringSubmatch(e.Name()); m != nil {
			if v, err := strconv.Atoi(m[1]); err == nil && v > latest {
				latest = v
			}
		}
	}
	if latest == 0 {
		t.Fatalf("no migration files found in %s", dir)
	}
	return latest
}

// dockerAvailable is the same heuristic the testdb fixture uses: produce a
// clear skip message instead of a cryptic testcontainers error when Docker is
// not running.
func dockerAvailable() bool {
	if socket := os.Getenv("DOCKER_HOST"); socket != "" {
		return true
	}
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		return true
	}
	return false
}

// migrationsDir resolves db/migrations relative to this file:
// internal/platform/database → apps/backend/db/migrations.
func migrationsDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed — cannot locate migrations dir")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations")
}
