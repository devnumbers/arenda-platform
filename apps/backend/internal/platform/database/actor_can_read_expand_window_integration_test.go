//go:build integration

package database

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// TestDerivedReadRenameExpandWindow pins the expand window of migration
// 000142: the derived-read canonical function (ADR 0028, #884) moved from
// actor_can_read_history to the context-neutral actor_can_read_property, but
// the old name must keep living until the contract release — the previous
// release's binary calls it from the six history-feed queries, and in the
// window "migrations ran, rollout pending" (docs/deployment.md) only the old
// name answers those calls. Dropping it in the same release breaks the
// two-release destructive rule (ADR 0024 §4, PR-template checklist).
//
// The behavioral half pins the two names to the same verdicts at the window
// edge: the old name must keep the exact semantics the previous binary was
// built against (000137 body), the new one must be its byte-equal successor.
//
// When the contract migration of the next release drops actor_can_read_history
// for good, flip this pin to an assert-drop of the old name.
func TestDerivedReadRenameExpandWindow(t *testing.T) {
	t.Parallel()

	if !dockerAvailable() {
		t.Skip("docker not available — the expand-window test always needs its own container")
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
		t.Fatalf("migrate up to latest: %v", err)
	}

	pool, err := NewPool(ctx, url)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	defer pool.Close()

	f := seedExpandWindowFixture(t, pool)

	// The expand half: the old 000137 name survives alongside the new one —
	// the previous release's binary depends on it in the deploy window.
	assertFunctionExists(t, pool, "actor_can_read_history")
	assertFunctionExists(t, pool, "actor_can_read_property")

	// The behavioral half: both names answer with the same verdicts — owner
	// reads (history outlives the archive), a stranger gets a privacy-404.
	assertDerivedReadVerdict(t, pool, "actor_can_read_history", f.ownerID, f.propertyID, true)
	assertDerivedReadVerdict(t, pool, "actor_can_read_history", f.strangerID, f.propertyID, false)
	assertDerivedReadVerdict(t, pool, "actor_can_read_property", f.ownerID, f.propertyID, true)
	assertDerivedReadVerdict(t, pool, "actor_can_read_property", f.strangerID, f.propertyID, false)
}

// expandWindowFixture holds the ids of the seeded rows: a property with its
// owner plus a stranger with no relation to it.
type expandWindowFixture struct {
	ownerID    uuid.UUID
	strangerID uuid.UUID
	propertyID uuid.UUID
}

// seedExpandWindowFixture plants the minimal read-scope surface: one owner
// with an active property and a stranger who must not see it.
func seedExpandWindowFixture(t *testing.T, pool *pgxpool.Pool) expandWindowFixture {
	t.Helper()

	ctx := context.Background()

	newID := func() uuid.UUID {
		id, err := uuid.NewV7()
		if err != nil {
			t.Fatalf("new uuid: %v", err)
		}
		return id
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed %q: %v", sql, err)
		}
	}

	f := expandWindowFixture{
		ownerID:    newID(),
		strangerID: newID(),
		propertyID: newID(),
	}

	exec(`INSERT INTO users (id, phone, role) VALUES ($1, $2, 'owner')`,
		f.ownerID, "+79990000001")
	exec(`INSERT INTO users (id, phone, role) VALUES ($1, $2, 'owner')`,
		f.strangerID, "+79990000002")
	exec(`INSERT INTO properties (id, owner_id, name, type, address, status)
	      VALUES ($1, $2, 'Expand window', 'apartment', '', 'active')`,
		f.propertyID, f.ownerID)

	return f
}

// assertFunctionExists reports a readable failure when the named function is
// gone instead of leaking a "function does not exist" from the call below.
func assertFunctionExists(t *testing.T, pool *pgxpool.Pool, name string) {
	t.Helper()

	var exists bool
	if err := pool.QueryRow(context.Background(),
		`SELECT to_regproc($1) IS NOT NULL`, name).Scan(&exists); err != nil {
		t.Fatalf("to_regproc(%q): %v", name, err)
	}
	if !exists {
		t.Errorf("function %s missing after MigrateUp — the expand window of the rename is broken (ADR 0024 §4)", name)
	}
}

// assertDerivedReadVerdict calls the named function and compares the verdict.
func assertDerivedReadVerdict(t *testing.T, pool *pgxpool.Pool, fn string, userID, propertyID uuid.UUID, want bool) {
	t.Helper()

	var got bool
	if err := pool.QueryRow(context.Background(),
		`SELECT `+fn+`($1, $2)`, propertyID, userID).Scan(&got); err != nil {
		t.Fatalf("call %s: %v", fn, err)
	}
	if got != want {
		t.Errorf("%s verdict for user %v on property %v = %v, want %v", fn, userID, propertyID, got, want)
	}
}
