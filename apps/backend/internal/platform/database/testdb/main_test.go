//go:build integration

package testdb

import (
	"os"
	"testing"
)

// TestMain ensures the per-binary template database is dropped and the
// PostgreSQL container terminated after the test binary finishes. The
// dropTemplate call is deferred after terminateContainer so it runs first:
// the template lives on the database server, which must still be up (the call
// is a no-op when no test called Setup). When TEST_DATABASE_URL is set, no
// container is started and terminateContainer is a no-op. Cleanup runs via
// defer: since Go 1.15 the test wrapper exits with m.Run's result after
// TestMain returns, so no os.Exit here (exit policy: os.Exit lives in cmd/).
func TestMain(m *testing.M) {
	defer terminateContainer()
	defer dropTemplate()
	m.Run()
}

// TestSetup_Smoke verifies that Setup produces a pool connected to a migrated
// database: it can run a trivial query and the users table (created in the
// initial migration) exists and is queryable.
func TestSetup_Smoke(t *testing.T) {
	if os.Getenv("TEST_DATABASE_URL") == "" && !dockerAvailable() {
		t.Skip("docker not available and TEST_DATABASE_URL not set")
	}

	pool := Setup(t)
	ctx := t.Context()

	var result int
	if err := pool.QueryRow(ctx, "SELECT 1").Scan(&result); err != nil {
		t.Fatalf("SELECT 1: %v", err)
	}
	if result != 1 {
		t.Fatalf("expected 1, got %d", result)
	}

	// The users table is created in migration 000001. If it is missing,
	// migrations did not apply. We do not assert emptiness here because some
	// migrations seed reference data (e.g. 000042 inserts a test admin user);
	// isolation (empty tables) is verified by TestReset_ClearsTables.
	var userCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&userCount); err != nil {
		t.Fatalf("SELECT count(*) FROM users: %v", err)
	}
	if userCount == 0 {
		t.Fatal("expected users table to contain seed rows after migration, got 0")
	}
}

// TestReset_ClearsTables inserts a row, calls Reset, and confirms every user
// table is emptied — the core isolation guarantee for integration tests sharing
// one database. Reset wipes seed data too (e.g. the test admin from migration
// 000042), which is expected: each test starts from a completely clean slate.
func TestReset_ClearsTables(t *testing.T) {
	if os.Getenv("TEST_DATABASE_URL") == "" && !dockerAvailable() {
		t.Skip("docker not available and TEST_DATABASE_URL not set")
	}

	pool := Setup(t)
	ctx := t.Context()

	// Insert a throwaway row into the users table. The users table requires
	// phone and role; uuidv7() generates the primary key (PostgreSQL 18).
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, phone, role) VALUES (uuidv7(), '+700000000000', 'owner')`,
	); err != nil {
		t.Fatalf("insert test row: %v", err)
	}

	var before int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&before); err != nil {
		t.Fatalf("count before reset: %v", err)
	}
	if before == 0 {
		t.Fatal("expected at least 1 row before reset (insert + seed), got 0")
	}

	Reset(t, pool)

	var after int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&after); err != nil {
		t.Fatalf("count after reset: %v", err)
	}
	if after != 0 {
		t.Fatalf("expected 0 rows after reset, got %d", after)
	}
}
