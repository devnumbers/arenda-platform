package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createUserInZone inserts a user whose timezone is set explicitly — the
// scan's zone directory groups owners by it (ADR 0048).
func createUserInZone(t *testing.T, pool *pgxpool.Pool, timezone string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	phone := fmt.Sprintf("+7999%07d", id.ID())
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
	assert.ElementsMatch(t, []string{"Europe/Kaliningrad", "Europe/Moscow"}, got)
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
	require.Len(t, targets, 1)

	got := targets[0]
	assert.Equal(t, awaiting, got.RentalID)
	assert.Equal(t, time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), got.PlannedEndDate)
	assert.Equal(t, awaitingProp, got.PropertyID)
	assert.Equal(t, "Объект", got.PropertyName)
	assert.Equal(t, "Москва, Тверская 1", got.PropertyAddress)
	assert.Equal(t, msk, got.OwnerID)

	// The boundary: a rental whose planned end IS the zone's today is still
	// active today — strictly before (решение #737: the day after).
	todayProp := createLiveProperty(t, pool, msk)
	insertRental(t, pool, msk, todayProp, datePtr(time.September, 1), datePtr(time.September, 19), nil)
	targets, err = store.ListCompletedTargets(ctx, "Europe/Moscow", *datePtr(time.September, 19))
	require.NoError(t, err)
	require.Len(t, targets, 1)
	assert.Equal(t, awaiting, targets[0].RentalID)

	// An archived property's rental — even in the needs_attention state —
	// stays out: the ticks sweep active/maintenance only.
	archivedProp := createLiveProperty(t, pool, msk)
	insertRental(t, pool, msk, archivedProp, datePtr(time.September, 1), datePtr(time.September, 15), nil)
	archiveProperty(t, pool, msk, archivedProp)
	targets, err = store.ListCompletedTargets(ctx, "Europe/Moscow", *datePtr(time.September, 19))
	require.NoError(t, err)
	require.Len(t, targets, 1)
	assert.Equal(t, awaiting, targets[0].RentalID)
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
