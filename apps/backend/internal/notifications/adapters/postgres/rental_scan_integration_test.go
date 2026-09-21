package postgres

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createUserInZone inserts a user whose timezone is set explicitly — the
// scan's zone directory groups owners by it (ADR 0048).
func createUserInZone(t *testing.T, pool *pgxpool.Pool, timezone string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	// The digits come from the uuid's random half (bytes 8..11): ID() is the
	// millisecond timestamp prefix, and two UUIDv7 minted in the same
	// millisecond — the parallel tests do exactly that — would collide.
	phone := fmt.Sprintf("+7999%07d", binary.BigEndian.Uint32(id[8:12])%10_000_000)
	_, err := pool.Exec(context.Background(),
		`INSERT INTO users (id, phone, role, timezone) VALUES ($1, $2, 'owner', $3)`,
		id, phone, timezone)
	require.NoError(t, err)
	return id
}

// addMember inserts an active or suspended membership row.
func addMember(t *testing.T, pool *pgxpool.Pool, propertyID, userID uuid.UUID, role, status string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO property_members (id, property_id, user_id, role, granted_by, status)
		 VALUES ($1, $2, $3, $4, $3, $5)`,
		uuid.Must(uuid.NewV7()), propertyID, userID, role, status)
	require.NoError(t, err)
}

func TestRentalScanStore_ListScanZones(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	// Two owners with scannable rentals — both zones must be listed.
	msk := createUserInZone(t, pool, "Europe/Moscow")
	mskProp := createLiveProperty(t, pool, msk)
	insertRental(t, pool, msk, mskProp, datePtr(time.September, 1), datePtr(time.September, 10), nil)

	kgd := createUserInZone(t, pool, "Europe/Kaliningrad")
	kgdProp := createLiveProperty(t, pool, kgd)
	insertRental(t, pool, kgd, kgdProp, datePtr(time.September, 1), datePtr(time.September, 10), nil)

	// A completed rental and a planned-end-less rental are not sweep targets
	// — an owner with only those is no zone.
	doneAt := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	quietProp := createLiveProperty(t, pool, msk)
	insertRental(t, pool, msk, quietProp, datePtr(time.September, 1), datePtr(time.September, 10), &doneAt)
	eternalProp := createLiveProperty(t, pool, msk)
	insertRental(t, pool, msk, eternalProp, datePtr(time.September, 1), nil, nil)

	// An archived property is out of the sweep (the ticks' canon): its
	// rentals mutations are rejected (ErrArchivedProperty), the action
	// buttons would be dead ends.
	archiveProperty(t, pool, msk, eternalProp)

	store := NewRentalScanStore(pool)
	zones, err := store.ListScanZones(ctx)
	require.NoError(t, err)

	got := make([]string, 0, len(zones))
	for _, z := range zones {
		got = append(got, z.Timezone)
	}
	// The shared test database's parallel fixtures may add zones — assert
	// this test's zones are swept, not that the world holds exactly them.
	assert.Contains(t, got, "Europe/Moscow")
	assert.Contains(t, got, "Europe/Kaliningrad")
}

// archiveProperty flips one of the owner's properties to the archived
// status.
func archiveProperty(t *testing.T, pool *pgxpool.Pool, ownerID, propertyID uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`UPDATE properties SET status = 'archived' WHERE id = $1 AND owner_id = $2`,
		propertyID, ownerID)
	require.NoError(t, err)
}

func TestRentalScanStore_ListCompletedTargets(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	msk := createUserInZone(t, pool, "Europe/Moscow")
	kgd := createUserInZone(t, pool, "Europe/Kaliningrad")

	awaitingProp := createLiveProperty(t, pool, msk)
	awaiting := insertRental(t, pool, msk, awaitingProp, datePtr(time.September, 1), datePtr(time.September, 18), nil)

	// Still active (planned end in the future) and completed — neither fires.
	activeProp := createLiveProperty(t, pool, msk)
	insertRental(t, pool, msk, activeProp, datePtr(time.September, 1), datePtr(time.October, 10), nil)
	doneAt := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	completedProp := createLiveProperty(t, pool, msk)
	insertRental(t, pool, msk, completedProp, datePtr(time.September, 1), datePtr(time.September, 10), &doneAt)

	// Another zone's awaiting rental stays out of Moscow's pass.
	kgdProp := createLivePropertyFor(t, pool, kgd, "Калининградский объект", "Калининград, Ленина 2")
	insertRental(t, pool, kgd, kgdProp, datePtr(time.September, 1), datePtr(time.September, 17), nil)

	store := NewRentalScanStore(pool)
	targets, err := store.ListCompletedTargets(ctx, "Europe/Moscow", *datePtr(time.September, 19))
	require.NoError(t, err)
	// The tests of this package share one database (setupPushDB), so the
	// parallel fixtures' rows may appear in the same zone's sweep — every
	// assertion filters to the properties this test created.
	got := rentalTargetsOfProps(targets, awaitingProp)
	require.Len(t, got, 1)
	assert.Equal(t, awaiting, got[0].RentalID)
	assert.Equal(t, time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), got[0].PlannedEndDate)
	assert.Equal(t, awaitingProp, got[0].PropertyID)
	assert.Equal(t, "Объект", got[0].PropertyName)
	assert.Equal(t, "Москва, Тверская 1", got[0].PropertyAddress)
	assert.Equal(t, msk, got[0].OwnerID)

	// The boundary: a rental whose planned end IS the zone's today is still
	// active today — strictly before (решение #737: the day after).
	todayProp := createLiveProperty(t, pool, msk)
	insertRental(t, pool, msk, todayProp, datePtr(time.September, 1), datePtr(time.September, 19), nil)
	targets, err = store.ListCompletedTargets(ctx, "Europe/Moscow", *datePtr(time.September, 19))
	require.NoError(t, err)
	assert.Empty(t, rentalTargetsOfProps(targets, todayProp), "today's rental is not needs_attention yet")
	assert.Len(t, rentalTargetsOfProps(targets, awaitingProp), 1)

	// An archived property's rental — even in the needs_attention state —
	// stays out: the ticks sweep active/maintenance only.
	archivedProp := createLiveProperty(t, pool, msk)
	insertRental(t, pool, msk, archivedProp, datePtr(time.September, 1), datePtr(time.September, 15), nil)
	archiveProperty(t, pool, msk, archivedProp)
	targets, err = store.ListCompletedTargets(ctx, "Europe/Moscow", *datePtr(time.September, 19))
	require.NoError(t, err)
	assert.Empty(t, rentalTargetsOfProps(targets, archivedProp), "the archived property stays out of the sweep")
	assert.Len(t, rentalTargetsOfProps(targets, awaitingProp), 1)
}

// The booking window of the completed boundary (issue #777): unfinished
// rentals whose boundary — 00:00 of the day after the planned end in the
// owner's timezone — falls into (from, until]. Moscow's midnight of the 19th
// is 2026-09-18T21:00Z; the boundary carries the zone's shift.
func TestRentalScanStore_ListScheduledCompletedTargets(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	msk := createUserInZone(t, pool, "Europe/Moscow")
	// One unfinished rental per property (idx_rentals_one_unfinished_per_
	// property) — every fixture rental sits on its own object.
	inWindowProp := createLiveProperty(t, pool, msk)
	inWindow := insertRental(t, pool, msk, inWindowProp, datePtr(time.September, 1), datePtr(time.September, 18), nil)

	// A further-out rental books on a later pass; a passed boundary is the
	// sweep's business (the window is open on the left).
	futureProp := createLiveProperty(t, pool, msk)
	future := insertRental(t, pool, msk, futureProp, datePtr(time.September, 1), datePtr(time.September, 25), nil)
	pastProp := createLiveProperty(t, pool, msk)
	past := insertRental(t, pool, msk, pastProp, datePtr(time.September, 1), datePtr(time.September, 17), nil)
	// A completed rental and an eternal one (no planned end) book nothing.
	doneAt := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	completedProp := createLiveProperty(t, pool, msk)
	completed := insertRental(t, pool, msk, completedProp, datePtr(time.September, 1), datePtr(time.September, 18), &doneAt)
	eternalProp := createLiveProperty(t, pool, msk)
	eternal := insertRental(t, pool, msk, eternalProp, datePtr(time.September, 1), nil, nil)

	// An archived property's rental stays out (the ticks' canon).
	archived := createUserInZone(t, pool, "Europe/Moscow")
	archivedProp := createLiveProperty(t, pool, archived)
	archivedRental := insertRental(t, pool, archived, archivedProp, datePtr(time.September, 1), datePtr(time.September, 18), nil)
	archiveProperty(t, pool, archived, archivedProp)

	store := NewRentalScanStore(pool)
	targets, err := store.ListScheduledCompletedTargets(ctx,
		time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	got := make(map[uuid.UUID]application.RentalScheduleTarget, len(targets))
	for _, target := range targets {
		got[target.RentalID] = target
	}
	require.Contains(t, got, inWindow)
	assert.Equal(t, time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), got[inWindow].PlannedEndDate)
	assert.True(t, got[inWindow].FireAt.Equal(time.Date(2026, 9, 18, 21, 0, 0, 0, time.UTC)),
		"the job wakes at 00:00 of the day after the planned end in the owner's zone, got %s",
		got[inWindow].FireAt)
	for _, gone := range []uuid.UUID{future, past, completed, eternal, archivedRental} {
		assert.NotContains(t, got, gone)
	}
}

// The completed boundary job's delivery-time resolution (issue #777): an
// unfinished rental whose planned end is strictly before the zone's today
// answers live; a completed one, an extended one (the planned end moved off
// the booked date) and a job awake before the boundary answer no.
func TestRentalScanStore_GetScheduledCompletedRental(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	msk := createUserInZone(t, pool, "Europe/Moscow")
	prop := createLivePropertyFor(t, pool, msk, "Квартира на Ленина", "г. Москва, ул. Ленина, 1")
	live := insertRental(t, pool, msk, prop, datePtr(time.September, 1), datePtr(time.September, 18), nil)

	doneAt := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	completedProp := createLiveProperty(t, pool, msk)
	completed := insertRental(t, pool, msk, completedProp, datePtr(time.September, 1), datePtr(time.September, 18), &doneAt)

	// The extended rental: the booked job carries the old planned end, the
	// row has moved a month out — the pair misses, the new boundary books
	// its own job.
	extendedProp := createLiveProperty(t, pool, msk)
	extended := insertRental(t, pool, msk, extendedProp, datePtr(time.September, 1), datePtr(time.September, 18), nil)
	_, err := pool.Exec(ctx, `UPDATE rentals SET planned_end_date = '2026-10-18' WHERE id = $1`, extended)
	require.NoError(t, err)

	store := NewRentalScanStore(pool)
	// Moscow's midnight of the 19th has just passed: the 18th is needs_attention.
	now := time.Date(2026, 9, 18, 21, 0, 30, 0, time.UTC)

	target, isLive, err := store.GetScheduledCompletedRental(ctx, live,
		time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), now)
	require.NoError(t, err)
	require.True(t, isLive)
	assert.Equal(t, live, target.RentalID)
	assert.Equal(t, time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), target.PlannedEndDate)
	assert.Equal(t, prop, target.PropertyID)
	assert.Equal(t, "Квартира на Ленина", target.PropertyName)
	assert.Equal(t, "г. Москва, ул. Ленина, 1", target.PropertyAddress)
	assert.Equal(t, msk, target.OwnerID)

	_, isLive, err = store.GetScheduledCompletedRental(ctx, completed,
		time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), now)
	require.NoError(t, err)
	assert.False(t, isLive, "completed")

	_, isLive, err = store.GetScheduledCompletedRental(ctx, extended,
		time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), now)
	require.NoError(t, err)
	assert.False(t, isLive, "the planned end moved off the booked date")

	// Before the boundary — Moscow's midnight of the 19th has not arrived,
	// the 18th is still the zone's today — the leg is silent; the job could
	// not wake this early, the check keeps the leg's semantics honest.
	_, isLive, err = store.GetScheduledCompletedRental(ctx, live,
		time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 18, 20, 59, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.False(t, isLive, "the rental is not needs_attention yet")
}

// rentalTargetsOfProps filters the sweep's targets to the given properties —
// the shared test database's parallel fixtures must not break the count
// assertions.
func rentalTargetsOfProps(targets []application.RentalCompletedTarget, props ...uuid.UUID) []application.RentalCompletedTarget {
	want := make(map[uuid.UUID]bool, len(props))
	for _, p := range props {
		want[p] = true
	}
	out := make([]application.RentalCompletedTarget, 0, len(targets))
	for _, target := range targets {
		if want[target.PropertyID] {
			out = append(out, target)
		}
	}
	return out
}

func TestRentalScanStore_ListActiveRecipients(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	owner := createUserInZone(t, pool, "Europe/Moscow")
	prop := createLiveProperty(t, pool, owner)
	fullAccess := createUserInZone(t, pool, "Europe/Moscow")
	viewer := createUserInZone(t, pool, "Europe/Moscow")
	suspended := createUserInZone(t, pool, "Europe/Moscow")
	addMember(t, pool, prop, fullAccess, "full_access", "active")
	addMember(t, pool, prop, viewer, "viewer", "active")
	addMember(t, pool, prop, suspended, "full_access", "suspended")

	store := NewRentalScanStore(pool)
	recipients, err := store.ListActiveRecipients(ctx, prop)
	require.NoError(t, err)
	assert.ElementsMatch(t, []uuid.UUID{fullAccess, viewer}, recipients,
		"активные участники обоих ролей; suspended — не получатель; владелец добавляет издатель")
}

// createLivePropertyFor is createLiveProperty with an explicit name/address —
// the zone-mixing assertions want to tell the properties apart.
func createLivePropertyFor(t *testing.T, pool *pgxpool.Pool, ownerID uuid.UUID, name, address string) uuid.UUID {
	t.Helper()
	propertyID := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(context.Background(), `
		INSERT INTO properties (id, owner_id, name, type, address, status)
		VALUES ($1, $2, $3, 'apartment', $4, 'active')`,
		propertyID, ownerID, name, address)
	require.NoError(t, err)
	return propertyID
}
