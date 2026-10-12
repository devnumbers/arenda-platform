//go:build integration

package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// preContractPinsFavoritesVersion is the last schema version before 000158:
// the cutover tickets have already moved every read and write to the
// personal tables (user_property_pins, user_payment_favorites), the global
// columns (properties.pinned_at, payments.is_favorite, payments.favorite_order)
// sit frozen at their defaults — the exact state production reaches before
// the contract drop runs.
const preContractPinsFavoritesVersion = 157

// TestPersonalPinsFavoritesContract verifies migration 000158 — the contract
// phase of the personal pins and favorites rework (spec #1302, ticket #1311,
// the 000102 precedent). Like the other migration contract tests it always
// starts its own PostgreSQL container: it plants the post-cutover state at
// version 157 and then steps 158 down and up, which the shared testdb
// fixture (already at the latest version) cannot serve.
//
// The contract: the up drops the frozen global columns with their CHECKs —
// no live query answers from them after the cutovers #1307/#1308. The down
// restores the original structure verbatim (000116/000122/000123) and
// backfills the global state from the personal rows of the owners — the
// exact inverse of the expand backfill (000156): the owner's pin time and
// order position come back as-is, participants' rows have no global
// representation, and an archived property stays unpinned (the restored
// CHECK's invariant — the global model had no way to hold a pin on an
// archived row).
func TestPersonalPinsFavoritesContract(t *testing.T) {
	t.Parallel()

	if !dockerAvailable() {
		t.Skip("docker not available — the contract test always needs its own container")
	}

	ctx := t.Context()

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

	// Post-cutover schema state: the frozen columns still exist, every
	// pin/favorite state lives in the personal tables.
	migrateToVersion(t, url, dir, preContractPinsFavoritesVersion)

	pool, err := NewPool(ctx, url)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	defer pool.Close()

	f := seedPinsFavoritesContractFixture(t, pool)

	// Up: the global columns and their CHECKs go; the personal rows — the
	// only state the running code answers from — survive untouched.
	migrateToVersion(t, url, dir, preContractPinsFavoritesVersion+1)
	assertGlobalPinFavoriteColumnsDropped(t, pool)
	assertPinsFavoritesRowsIntact(t, pool, f)

	// Down: the structure comes back verbatim and the owner's global state
	// is backfilled from the personal rows.
	migrateToVersion(t, url, dir, preContractPinsFavoritesVersion)
	assertGlobalPinFavoriteStructureRestored(t, pool)
	assertPinsFavoritesBackfillRestored(t, pool, f)

	// The second up drops the restored columns again on a populated schema,
	// and the tail of the chain must still apply cleanly on top.
	migrateToVersion(t, url, dir, preContractPinsFavoritesVersion+1)
	assertGlobalPinFavoriteColumnsDropped(t, pool)

	if err := MigrateUp(url, dir); err != nil {
		t.Fatalf("migrate up to latest: %v", err)
	}
	assertSchemaAtLatestVersion(t, url, dir)
}

// pinsFavoritesContractFixture holds the ids and timestamps of the
// post-cutover rows planted at version 157: two owners and a participant,
// four properties (one pinned twice — owner plus participant, one unpinned,
// one archived with a surviving owner pin, one owned by the second owner)
// and four payments (one favorite with a saved position, one favorite
// without, one not favorite, one of the second owner).
type pinsFavoritesContractFixture struct {
	owner1ID                   uuid.UUID
	owner2ID                   uuid.UUID
	participantID              uuid.UUID
	pinnedPropertyID           uuid.UUID
	unpinnedPropertyID         uuid.UUID
	archivedPinnedPropertyID   uuid.UUID
	owner2PinnedPropertyID     uuid.UUID
	favoriteOrderedPaymentID   uuid.UUID
	favoriteNullOrderPaymentID uuid.UUID
	unfavoritePaymentID        uuid.UUID
	owner2FavoritePaymentID    uuid.UUID
	owner1PinTime              time.Time
	participantPinTime         time.Time
	archivedPinTime            time.Time
	owner2PinTime              time.Time
}

func seedPinsFavoritesContractFixture(t *testing.T, pool *pgxpool.Pool) pinsFavoritesContractFixture {
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

	f := pinsFavoritesContractFixture{
		owner1ID:                   newID(),
		owner2ID:                   newID(),
		participantID:              newID(),
		pinnedPropertyID:           newID(),
		unpinnedPropertyID:         newID(),
		archivedPinnedPropertyID:   newID(),
		owner2PinnedPropertyID:     newID(),
		favoriteOrderedPaymentID:   newID(),
		favoriteNullOrderPaymentID: newID(),
		unfavoritePaymentID:        newID(),
		owner2FavoritePaymentID:    newID(),
		// Whole seconds: timestamptz keeps microsecond precision, so the
		// round trip through the down backfill must return these values.
		owner1PinTime:      time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC),
		participantPinTime: time.Date(2026, 9, 20, 15, 10, 0, 0, time.UTC),
		archivedPinTime:    time.Date(2026, 9, 21, 10, 30, 0, 0, time.UTC),
		owner2PinTime:      time.Date(2026, 9, 22, 18, 45, 0, 0, time.UTC),
	}

	exec(`INSERT INTO users (id, phone, role) VALUES ($1, $2, 'owner'), ($3, $4, 'owner'), ($5, $6, 'owner')`,
		f.owner1ID, "+79991000001", f.owner2ID, "+79991000002", f.participantID, "+79991000003")
	// The frozen global columns sit at their defaults — the cutover moved
	// every state into the personal tables (D16: the archive does not touch
	// personal pins, hence the archived property keeps its owner's row).
	exec(`INSERT INTO properties (id, owner_id, name, type, address, status)
	      VALUES ($1, $2, 'Pinned', 'apartment', '', 'active'),
	             ($3, $2, 'Unpinned', 'apartment', '', 'active'),
	             ($4, $2, 'Archived pinned', 'apartment', '', 'archived'),
	             ($5, $6, 'Second owner', 'apartment', '', 'active')`,
		f.pinnedPropertyID, f.owner1ID,
		f.unpinnedPropertyID,
		f.archivedPinnedPropertyID,
		f.owner2PinnedPropertyID, f.owner2ID)
	exec(`INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks,
	                           recurrence, since, category_slug)
	      VALUES ($1, $3, $6, 'expense', 'Rent ordered', 10000, '{"kind":"monthly"}', CURRENT_DATE, 'rent'),
	             ($2, $3, $6, 'expense', 'Rent unordered', 20000, '{"kind":"monthly"}', CURRENT_DATE, 'rent'),
	             ($4, $3, $6, 'expense', 'Not favorite', 30000, '{"kind":"monthly"}', CURRENT_DATE, 'rent'),
	             ($5, $7, $8, 'expense', 'Second owner favorite', 40000, '{"kind":"monthly"}', CURRENT_DATE, 'rent')`,
		f.favoriteOrderedPaymentID, f.favoriteNullOrderPaymentID, f.owner1ID, f.unfavoritePaymentID,
		f.owner2FavoritePaymentID, f.pinnedPropertyID, f.owner2ID, f.owner2PinnedPropertyID)

	exec(`INSERT INTO user_property_pins (user_id, property_id, pinned_at)
	      VALUES ($1, $2, $3), ($4, $2, $5), ($1, $6, $7), ($8, $9, $10)`,
		f.owner1ID, f.pinnedPropertyID, f.owner1PinTime,
		f.participantID, f.participantPinTime,
		f.archivedPinnedPropertyID, f.archivedPinTime,
		f.owner2ID, f.owner2PinnedPropertyID, f.owner2PinTime)
	exec(`INSERT INTO user_payment_favorites (user_id, payment_id, favorite_order)
	      VALUES ($1, $2, 2), ($3, $2, NULL), ($1, $4, NULL), ($5, $6, NULL)`,
		f.owner1ID, f.favoriteOrderedPaymentID,
		f.participantID,
		f.favoriteNullOrderPaymentID,
		f.owner2ID, f.owner2FavoritePaymentID)

	return f
}

// Schema names the assertions probe in both directions (up drops, down
// restores); constants keep the goconst gate quiet.
const (
	propertiesTable = "properties"
	paymentsTable   = "payments"
)

// assertGlobalPinFavoriteColumnsDropped checks the up migration effects: the
// three global columns and both CHECKs are gone from the schema.
func assertGlobalPinFavoriteColumnsDropped(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	for _, tc := range []struct {
		table, column string
	}{
		{propertiesTable, "pinned_at"},
		{paymentsTable, "is_favorite"},
		{paymentsTable, "favorite_order"},
	} {
		if got := countRows(t, pool,
			`SELECT count(*) FROM information_schema.columns WHERE table_name = $1 AND column_name = $2`,
			tc.table, tc.column); got != 0 {
			t.Errorf("%s.%s still exists after up", tc.table, tc.column)
		}
	}
	for _, constraint := range []string{"properties_pinned_at_check", "payments_favorite_order_check"} {
		if constraintExists(t, pool, constraint) {
			t.Errorf("constraint %s still exists after up", constraint)
		}
	}
}

// assertGlobalPinFavoriteStructureRestored checks the down migration's
// structural half: the columns and CHECKs are back in their original shape
// (000116/000122/000123).
func assertGlobalPinFavoriteStructureRestored(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	for _, tc := range []struct {
		table, column, dataType string
		nullable                bool
	}{
		{propertiesTable, "pinned_at", "timestamp with time zone", true},
		{paymentsTable, "is_favorite", "boolean", false},
		{paymentsTable, "favorite_order", "bigint", true},
	} {
		var dataType string
		var nullable bool
		err := pool.QueryRow(context.Background(),
			`SELECT data_type, is_nullable = 'YES' FROM information_schema.columns
			 WHERE table_name = $1 AND column_name = $2`, tc.table, tc.column,
		).Scan(&dataType, &nullable)
		if err != nil {
			t.Fatalf("%s.%s after down: %v", tc.table, tc.column, err)
		}
		if dataType != tc.dataType || nullable != tc.nullable {
			t.Errorf("%s.%s after down = (%s, nullable=%v), want (%s, nullable=%v)",
				tc.table, tc.column, dataType, nullable, tc.dataType, tc.nullable)
		}
	}
	for _, constraint := range []string{"properties_pinned_at_check", "payments_favorite_order_check"} {
		if !constraintExists(t, pool, constraint) {
			t.Errorf("constraint %s missing after down", constraint)
		}
	}
}

// assertPinsFavoritesBackfillRestored checks the down migration's data half:
// the owner's global state comes back from the personal rows — the pin
// timestamps and the order position verbatim, the participant's rows have no
// global representation, and the archived property stays unpinned (the
// restored CHECK's invariant).
func assertPinsFavoritesBackfillRestored(t *testing.T, pool *pgxpool.Pool, f pinsFavoritesContractFixture) {
	t.Helper()

	assertPropertyPinnedAt(t, pool, f.pinnedPropertyID, sql.NullTime{Time: f.owner1PinTime, Valid: true})
	assertPropertyPinnedAt(t, pool, f.unpinnedPropertyID, sql.NullTime{})
	// D16 lets the owner's personal pin outlive the archive, but the global
	// model cannot hold it: the restored CHECK forbids a pinned archived row.
	assertPropertyPinnedAt(t, pool, f.archivedPinnedPropertyID, sql.NullTime{})
	assertPropertyPinnedAt(t, pool, f.owner2PinnedPropertyID, sql.NullTime{Time: f.owner2PinTime, Valid: true})

	if got := countRows(t, pool, `SELECT count(*) FROM properties WHERE pinned_at IS NOT NULL`); got != 2 {
		t.Errorf("properties with restored pinned_at = %d, want 2 (the participant's pin must not surface)", got)
	}

	assertPaymentFavoriteState(t, pool, f.favoriteOrderedPaymentID, true, sql.NullInt64{Int64: 2, Valid: true})
	assertPaymentFavoriteState(t, pool, f.favoriteNullOrderPaymentID, true, sql.NullInt64{})
	assertPaymentFavoriteState(t, pool, f.unfavoritePaymentID, false, sql.NullInt64{})
	assertPaymentFavoriteState(t, pool, f.owner2FavoritePaymentID, true, sql.NullInt64{})

	if got := countRows(t, pool, `SELECT count(*) FROM payments WHERE is_favorite`); got != 3 {
		t.Errorf("payments with restored is_favorite = %d, want 3 (the participant's star must not surface)", got)
	}
}

// assertPinsFavoritesRowsIntact checks that the up migration touches only the
// schema: every personal row planted before it — including the ones the down
// backfill cannot represent globally (the participant's rows, the archived
// property's pin) — keeps its exact values.
func assertPinsFavoritesRowsIntact(t *testing.T, pool *pgxpool.Pool, f pinsFavoritesContractFixture) {
	t.Helper()

	if got := countRows(t, pool, `SELECT count(*) FROM user_property_pins`); got != 4 {
		t.Errorf("user_property_pins rows after up = %d, want 4", got)
	}
	if got := countRows(t, pool, `SELECT count(*) FROM user_payment_favorites`); got != 4 {
		t.Errorf("user_payment_favorites rows after up = %d, want 4", got)
	}
	assertPinRow(t, pool, f.owner1ID, f.pinnedPropertyID, f.owner1PinTime)
	assertPinRow(t, pool, f.participantID, f.pinnedPropertyID, f.participantPinTime)
	assertPinRow(t, pool, f.owner1ID, f.archivedPinnedPropertyID, f.archivedPinTime)
	assertPinRow(t, pool, f.owner2ID, f.owner2PinnedPropertyID, f.owner2PinTime)
	assertFavoriteRow(t, pool, f.owner1ID, f.favoriteOrderedPaymentID, sql.NullInt64{Int64: 2, Valid: true})
	assertFavoriteRow(t, pool, f.participantID, f.favoriteOrderedPaymentID, sql.NullInt64{})
	assertFavoriteRow(t, pool, f.owner1ID, f.favoriteNullOrderPaymentID, sql.NullInt64{})
	assertFavoriteRow(t, pool, f.owner2ID, f.owner2FavoritePaymentID, sql.NullInt64{})
}

// assertPropertyPinnedAt reads properties.pinned_at of one property and
// compares it with the expected value (zero value = NULL).
func assertPropertyPinnedAt(t *testing.T, pool *pgxpool.Pool, propertyID uuid.UUID, want sql.NullTime) {
	t.Helper()

	var got sql.NullTime
	err := pool.QueryRow(context.Background(),
		`SELECT pinned_at FROM properties WHERE id = $1`, propertyID).Scan(&got)
	if err != nil {
		t.Fatalf("read pinned_at of property %v: %v", propertyID, err)
	}
	if got.Valid != want.Valid || (got.Valid && !got.Time.Equal(want.Time)) {
		t.Errorf("property %v pinned_at = %v, want %v (the owner's pin must be restored verbatim, an archived row stays unpinned)",
			propertyID, got, want)
	}
}

// assertPaymentFavoriteState reads the restored global favorite pair of one
// payment and compares it with the expected values.
func assertPaymentFavoriteState(t *testing.T, pool *pgxpool.Pool, paymentID uuid.UUID, wantFavorite bool, wantOrder sql.NullInt64) {
	t.Helper()

	var gotFavorite bool
	var gotOrder sql.NullInt64
	err := pool.QueryRow(context.Background(),
		`SELECT is_favorite, favorite_order FROM payments WHERE id = $1`, paymentID,
	).Scan(&gotFavorite, &gotOrder)
	if err != nil {
		t.Fatalf("read favorite state of payment %v: %v", paymentID, err)
	}
	if gotFavorite != wantFavorite || gotOrder != wantOrder {
		t.Errorf("payment %v favorite state = (%v, %v), want (%v, %v) (the owner's state must be restored verbatim)",
			paymentID, gotFavorite, gotOrder, wantFavorite, wantOrder)
	}
}

// constraintExists reports whether the named table constraint is present.
func constraintExists(t *testing.T, pool *pgxpool.Pool, name string) bool {
	t.Helper()

	var exists bool
	if err := pool.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = $1)`, name,
	).Scan(&exists); err != nil {
		t.Fatalf("read constraint %q: %v", name, err)
	}
	return exists
}
