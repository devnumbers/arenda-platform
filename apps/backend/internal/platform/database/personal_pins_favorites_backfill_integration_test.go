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

// preExpandPinsFavoritesVersion is the last schema version before 000156:
// the global pin/favorite state (properties.pinned_at, payments.is_favorite,
// payments.favorite_order) is still the only state there is, so the rows the
// backfill must move can be planted.
const preExpandPinsFavoritesVersion = 155

// TestPersonalPinsFavoritesBackfill verifies migration 000156 — the expand
// phase of the personal pins and favorites rework (spec #1302, model #1299,
// ticket #1303). Like the other migration contract tests it always starts
// its own PostgreSQL container: it plants pre-expand data at version 155
// and then steps 156 up and down, which the shared testdb fixture (already
// at the latest version) cannot serve.
//
// The contract: existing global pins and favorites move to the property
// owner (D11/D12) with the pin timestamp and the order position carried
// over as-is (a NULL position stays NULL — «новое избранное — в конец»),
// unpinned/unfavorited rows produce nothing, and the down migration loses
// nothing: in the expand window the frozen old columns remain the source
// of truth, so the rollback drops the new tables while the original data
// stays put.
func TestPersonalPinsFavoritesBackfill(t *testing.T) {
	t.Parallel()

	if !dockerAvailable() {
		t.Skip("docker not available — the backfill test always needs its own container")
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

	// Pre-expand schema state: the global columns the backfill reads exist.
	migrateToVersion(t, url, dir, preExpandPinsFavoritesVersion)

	pool, err := NewPool(ctx, url)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	defer pool.Close()

	f := seedPinsFavoritesFixture(t, pool)

	// Up: the backfill moves every global pin/favorite to the property owner.
	migrateToVersion(t, url, dir, preExpandPinsFavoritesVersion+1)
	assertPinsFavoritesBackfilled(t, pool, f)

	// Down: the paired migration drops the new tables — and nothing else.
	// The frozen old columns must still carry the original state.
	migrateToVersion(t, url, dir, preExpandPinsFavoritesVersion)
	assertPinsFavoritesExpandReverted(t, pool, f)

	// The second up re-runs the backfill on the untouched source columns,
	// and the tail of the chain must still apply cleanly on top.
	migrateToVersion(t, url, dir, preExpandPinsFavoritesVersion+1)
	assertPinsFavoritesBackfilled(t, pool, f)

	if err := MigrateUp(url, dir); err != nil {
		t.Fatalf("migrate up to latest: %v", err)
	}
	assertSchemaAtLatestVersion(t, url, dir)
}

// pinsFavoritesFixture holds the ids and timestamps of the pre-expand rows
// planted at version 155: two owners, three properties (two pinned, one
// not) and four payments (three favorites — one with a saved position, two
// without — one not favorite).
type pinsFavoritesFixture struct {
	owner1ID                   uuid.UUID
	owner2ID                   uuid.UUID
	pinnedPropertyID           uuid.UUID
	unpinnedPropertyID         uuid.UUID
	owner2PinnedPropertyID     uuid.UUID
	favoriteOrderedPaymentID   uuid.UUID
	favoriteNullOrderPaymentID uuid.UUID
	unfavoritePaymentID        uuid.UUID
	owner2FavoritePaymentID    uuid.UUID
	pinTime1                   time.Time
	pinTime2                   time.Time
}

func seedPinsFavoritesFixture(t *testing.T, pool *pgxpool.Pool) pinsFavoritesFixture {
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

	f := pinsFavoritesFixture{
		owner1ID:                   newID(),
		owner2ID:                   newID(),
		pinnedPropertyID:           newID(),
		unpinnedPropertyID:         newID(),
		owner2PinnedPropertyID:     newID(),
		favoriteOrderedPaymentID:   newID(),
		favoriteNullOrderPaymentID: newID(),
		unfavoritePaymentID:        newID(),
		owner2FavoritePaymentID:    newID(),
		// Whole seconds: timestamptz keeps microsecond precision, so the
		// round trip through the backfill must return these exact values.
		pinTime1: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
		pinTime2: time.Date(2026, 8, 15, 12, 30, 0, 0, time.UTC),
	}

	exec(`INSERT INTO users (id, phone, role) VALUES ($1, $2, 'owner'), ($3, $4, 'owner')`,
		f.owner1ID, "+79990000001", f.owner2ID, "+79990000002")
	exec(`INSERT INTO properties (id, owner_id, name, type, address, status, pinned_at)
	      VALUES ($1, $2, 'Pinned', 'apartment', '', 'active', $3),
	             ($4, $2, 'Unpinned', 'apartment', '', 'active', NULL),
	             ($5, $6, 'Second owner pinned', 'apartment', '', 'active', $7)`,
		f.pinnedPropertyID, f.owner1ID, f.pinTime1,
		f.unpinnedPropertyID,
		f.owner2PinnedPropertyID, f.owner2ID, f.pinTime2)
	exec(`INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks,
	                           recurrence, since, category_slug, is_favorite, favorite_order)
	      VALUES ($1, $3, $4, 'expense', 'Rent ordered', 10000, '{"kind":"monthly"}', CURRENT_DATE, 'rent', true, 2),
	             ($2, $3, $4, 'expense', 'Rent unordered', 20000, '{"kind":"monthly"}', CURRENT_DATE, 'rent', true, NULL),
	             ($5, $3, $4, 'expense', 'Not favorite', 30000, '{"kind":"monthly"}', CURRENT_DATE, 'rent', false, NULL)`,
		f.favoriteOrderedPaymentID, f.favoriteNullOrderPaymentID, f.owner1ID, f.pinnedPropertyID,
		f.unfavoritePaymentID)
	exec(`INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks,
	                           recurrence, since, category_slug, is_favorite, favorite_order)
	      VALUES ($1, $2, $3, 'expense', 'Second owner favorite', 40000, '{"kind":"monthly"}', CURRENT_DATE, 'rent', true, NULL)`,
		f.owner2FavoritePaymentID, f.owner2ID, f.owner2PinnedPropertyID)

	return f
}

// assertPinsFavoritesBackfilled checks the up migration effects: exactly the
// pinned properties and favorited payments have rows, each attributed to its
// property's owner, with the pin timestamp and the order position carried
// over verbatim (NULL order stays NULL).
func assertPinsFavoritesBackfilled(t *testing.T, pool *pgxpool.Pool, f pinsFavoritesFixture) {
	t.Helper()

	if got := countRows(t, pool, `SELECT count(*) FROM user_property_pins`); got != 2 {
		t.Errorf("user_property_pins rows after up = %d, want 2", got)
	}
	if got := countRows(t, pool, `SELECT count(*) FROM user_payment_favorites`); got != 3 {
		t.Errorf("user_payment_favorites rows after up = %d, want 3", got)
	}

	assertPinRow(t, pool, f.owner1ID, f.pinnedPropertyID, f.pinTime1)
	assertNoPinRow(t, pool, f.owner1ID, f.unpinnedPropertyID)
	assertPinRow(t, pool, f.owner2ID, f.owner2PinnedPropertyID, f.pinTime2)

	assertFavoriteRow(t, pool, f.owner1ID, f.favoriteOrderedPaymentID, sql.NullInt64{Int64: 2, Valid: true})
	assertFavoriteRow(t, pool, f.owner1ID, f.favoriteNullOrderPaymentID, sql.NullInt64{})
	assertNoFavoriteRow(t, pool, f.owner1ID, f.unfavoritePaymentID)
	assertFavoriteRow(t, pool, f.owner2ID, f.owner2FavoritePaymentID, sql.NullInt64{})

	// The backfill stamps created_at with the default (the global flag has
	// no favorite timestamp to carry over).
	var createdIsNull bool
	if err := pool.QueryRow(context.Background(),
		`SELECT created_at IS NULL FROM user_payment_favorites
		 WHERE user_id = $1 AND payment_id = $2`, f.owner1ID, f.favoriteOrderedPaymentID,
	).Scan(&createdIsNull); err != nil {
		t.Fatalf("read created_at: %v", err)
	}
	if createdIsNull {
		t.Errorf("user_payment_favorites.created_at after up is NULL, want the default now()")
	}
}

// assertPinsFavoritesExpandReverted checks the down migration effects: the
// new tables are gone while the frozen global columns still hold the exact
// original state — the expand rollback loses nothing.
func assertPinsFavoritesExpandReverted(t *testing.T, pool *pgxpool.Pool, f pinsFavoritesFixture) {
	t.Helper()

	if tableExists(t, pool, "user_property_pins") {
		t.Errorf("user_property_pins after down still exists")
	}
	if tableExists(t, pool, "user_payment_favorites") {
		t.Errorf("user_payment_favorites after down still exists")
	}

	var pinnedAt time.Time
	if err := pool.QueryRow(context.Background(),
		`SELECT pinned_at FROM properties WHERE id = $1`, f.pinnedPropertyID,
	).Scan(&pinnedAt); err != nil {
		t.Fatalf("read pinned_at after down: %v", err)
	}
	if !pinnedAt.Equal(f.pinTime1) {
		t.Errorf("properties.pinned_at after down = %v, want %v (source data must survive the rollback)", pinnedAt, f.pinTime1)
	}

	row := pool.QueryRow(context.Background(),
		`SELECT is_favorite, favorite_order FROM payments WHERE id = $1`, f.favoriteOrderedPaymentID)
	var isFavorite bool
	var order sql.NullInt64
	if err := row.Scan(&isFavorite, &order); err != nil {
		t.Fatalf("read favorite state after down: %v", err)
	}
	if !isFavorite || !order.Valid || order.Int64 != 2 {
		t.Errorf("payments favorite state after down = (%v, %v), want (true, 2)", isFavorite, order)
	}
}

// assertPinRow reads the pinned_at of one (user, property) pin row and
// compares it with the planted global pin timestamp.
func assertPinRow(t *testing.T, pool *pgxpool.Pool, userID, propertyID uuid.UUID, want time.Time) {
	t.Helper()

	var got time.Time
	err := pool.QueryRow(context.Background(),
		`SELECT pinned_at FROM user_property_pins WHERE user_id = $1 AND property_id = $2`,
		userID, propertyID).Scan(&got)
	if err != nil {
		t.Fatalf("read pin row (%v, %v): %v", userID, propertyID, err)
	}
	if !got.Equal(want) {
		t.Errorf("pin row (%v, %v) pinned_at = %v, want %v (must be carried over, not reset)", userID, propertyID, got, want)
	}
}

func assertNoPinRow(t *testing.T, pool *pgxpool.Pool, userID, propertyID uuid.UUID) {
	t.Helper()

	if got := countRows(t, pool,
		`SELECT count(*) FROM user_property_pins WHERE user_id = $1 AND property_id = $2`,
		userID, propertyID); got != 0 {
		t.Errorf("unpinned property produced a pin row (%v, %v) = %d, want 0", userID, propertyID, got)
	}
}

// assertFavoriteRow reads the favorite_order of one (user, payment) favorite
// row and compares it with the carried-over position (zero value = NULL).
func assertFavoriteRow(t *testing.T, pool *pgxpool.Pool, userID, paymentID uuid.UUID, wantOrder sql.NullInt64) {
	t.Helper()

	var gotOrder sql.NullInt64
	err := pool.QueryRow(context.Background(),
		`SELECT favorite_order FROM user_payment_favorites WHERE user_id = $1 AND payment_id = $2`,
		userID, paymentID).Scan(&gotOrder)
	if err != nil {
		t.Fatalf("read favorite row (%v, %v): %v", userID, paymentID, err)
	}
	if gotOrder != wantOrder {
		t.Errorf("favorite row (%v, %v) favorite_order = %v, want %v (position must be carried over verbatim)",
			userID, paymentID, gotOrder, wantOrder)
	}
}

func assertNoFavoriteRow(t *testing.T, pool *pgxpool.Pool, userID, paymentID uuid.UUID) {
	t.Helper()

	if got := countRows(t, pool,
		`SELECT count(*) FROM user_payment_favorites WHERE user_id = $1 AND payment_id = $2`,
		userID, paymentID); got != 0 {
		t.Errorf("unfavorited payment produced a favorite row (%v, %v) = %d, want 0", userID, paymentID, got)
	}
}
