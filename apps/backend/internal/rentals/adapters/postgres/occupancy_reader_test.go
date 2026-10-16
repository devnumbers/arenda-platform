//go:build integration

package postgres_test

// The integration family of the rentals occupancy reader (ticket #585): the
// batched unfinished-rental read over real PostgreSQL with the status math
// resolved against the data owner's today (ADR 0048) — every unfinished
// status, the no-rental default, the completed-rental history and the
// foreign-rental isolation.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
	rentalspg "github.com/nambers/arenda-planform/apps/backend/internal/rentals/adapters/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// occToday anchors the fake calendar: the owner's fixed calendar date.
var occToday = mustOccDate("2026-09-10")

func mustOccDate(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

// occFixedCalendar returns a fixed today for every owner — the reader
// consumes the payments OwnerCalendar port, never a clock.
type occFixedCalendar struct{}

func (occFixedCalendar) Today(context.Context, uuid.UUID) (time.Time, error) {
	return occToday, nil
}

type occupancyFixture struct {
	pool *pgxpool.Pool
}

func setupOccupancyReader(t *testing.T) *occupancyFixture {
	t.Helper()
	return &occupancyFixture{pool: testdb.Setup(t)}
}

// seedOwnerWithProperty inserts one owner (Europe/Moscow) with one active
// property; the property id comes back.
func (f *occupancyFixture) seedOwnerWithProperty(ctx context.Context, t *testing.T) uuid.UUID {
	t.Helper()
	owner, err := uuid.NewV7()
	require.NoError(t, err)
	phone := fmt.Sprintf("+7999%010d", time.Now().UnixNano()%10000000000)
	_, err = f.pool.Exec(ctx,
		`INSERT INTO users (id, phone, role, timezone) VALUES ($1, $2, $3, 'Europe/Moscow')`,
		owner, phone, actor.RoleOwner)
	require.NoError(t, err)
	propID, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx,
		`INSERT INTO properties (id, owner_id, name, type, address, status)
		 VALUES ($1, $2, 'Объект', 'apartment', 'Москва, Тверская 1', 'active')`,
		propID, owner)
	require.NoError(t, err)
	return propID
}

// seedRental inserts the property's rental with its managed payment row —
// the RESTRICT FK of the seam (ADR 0053 §3) demands the pair while the
// rental runs. A completed rental mirrors the post-completion state
// (ревизия #1161): no payment link, the terms archive beside the completion
// fact, the payment row itself gone.
func (f *occupancyFixture) seedRental(
	ctx context.Context, t *testing.T, propID uuid.UUID, start time.Time, plannedEnd, completed *time.Time,
) {
	t.Helper()
	var ownerID uuid.UUID
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT owner_id FROM properties WHERE id = $1`, propID).Scan(&ownerID))
	rentalID, err := uuid.NewV7()
	require.NoError(t, err)
	if completed != nil {
		_, err = f.pool.Exec(ctx,
			`INSERT INTO rentals (id, owner_id, property_id, payment_id, start_date,
			       planned_end_date, completed_date, utilities,
			       rent_amount_kopecks, rent_payment_day, rent_auto_pay)
			 VALUES ($1, $2, $3, NULL, $4, $5, $6, 'included', 5000000, 15, false)`,
			rentalID, ownerID, propID, start, datePtrArg(plannedEnd), *completed)
		require.NoError(t, err)
		return
	}
	paymentID, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx,
		`INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks,
		                       recurrence, since, auto_pay, category_slug)
		 VALUES ($1, $2, $3, 'income', 'Аренда', 5000000,
		         '{"kind":"monthly","daysOfMonth":[15]}'::jsonb, $4, false, 'rent')`,
		paymentID, ownerID, propID, start)
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx,
		`INSERT INTO rentals (id, owner_id, property_id, payment_id, start_date,
		       planned_end_date, completed_date, utilities)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, 'included')`,
		rentalID, ownerID, propID, paymentID, start, datePtrArg(plannedEnd), datePtrArg(completed))
	require.NoError(t, err)
}

func datePtrArg(d *time.Time) any {
	if d == nil {
		return nil
	}
	return *d
}

func (f *occupancyFixture) ownerOf(ctx context.Context, t *testing.T, propID uuid.UUID) uuid.UUID {
	t.Helper()
	var ownerID uuid.UUID
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT owner_id FROM properties WHERE id = $1`, propID).Scan(&ownerID))
	return ownerID
}

func TestOccupancyReader_StatusesOverOwnerToday(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setupOccupancyReader(t)
	reader := rentalspg.NewOccupancyReader(f.pool, occFixedCalendar{})

	activeID := f.seedOwnerWithProperty(ctx, t)
	activeEnd := mustOccDate("2027-03-01")
	f.seedRental(ctx, t, activeID, mustOccDate("2026-01-01"), &activeEnd, nil)

	upcomingID := f.seedOwnerWithProperty(ctx, t)
	upcomingStart := mustOccDate("2026-10-01")
	f.seedRental(ctx, t, upcomingID, upcomingStart, nil, nil)

	openEndedID := f.seedOwnerWithProperty(ctx, t)
	f.seedRental(ctx, t, openEndedID, mustOccDate("2026-01-01"), nil, nil)

	attentionID := f.seedOwnerWithProperty(ctx, t)
	pastEnd := mustOccDate("2026-09-01")
	f.seedRental(ctx, t, attentionID, mustOccDate("2026-01-01"), &pastEnd, nil)

	completedID := f.seedOwnerWithProperty(ctx, t)
	completed := mustOccDate("2026-08-01")
	f.seedRental(ctx, t, completedID, mustOccDate("2026-01-01"), &pastEnd, &completed)

	noneID := f.seedOwnerWithProperty(ctx, t)

	owners := map[uuid.UUID]uuid.UUID{
		activeID:    f.ownerOf(ctx, t, activeID),
		upcomingID:  f.ownerOf(ctx, t, upcomingID),
		openEndedID: f.ownerOf(ctx, t, openEndedID),
		attentionID: f.ownerOf(ctx, t, attentionID),
		completedID: f.ownerOf(ctx, t, completedID),
		noneID:      f.ownerOf(ctx, t, noneID),
	}

	got, err := reader.OccupancyByProperty(ctx, owners)
	require.NoError(t, err)
	require.Len(t, got, len(owners))

	assert.Equal(t, "active", string(got[activeID].Status))
	require.NotNil(t, got[activeID].PlannedEndDate)
	assert.Equal(t, activeEnd, *got[activeID].PlannedEndDate)
	require.NotNil(t, got[activeID].StartDate)

	assert.Equal(t, "upcoming", string(got[upcomingID].Status))
	require.NotNil(t, got[upcomingID].StartDate)
	assert.Equal(t, upcomingStart, *got[upcomingID].StartDate)

	assert.Equal(t, "active", string(got[openEndedID].Status))
	assert.Nil(t, got[openEndedID].PlannedEndDate, "open-ended rental carries no planned end")

	assert.Equal(t, "needs_attention", string(got[attentionID].Status))

	// The explicitly completed rental is history: the occupancy is none —
	// «после явного завершения исчезает» (резолюция #584).
	assert.Equal(t, "none", string(got[completedID].Status))

	assert.Equal(t, "none", string(got[noneID].Status))
	assert.Nil(t, got[noneID].StartDate)
	assert.Nil(t, got[noneID].PlannedEndDate)
}

func TestOccupancyReader_NeedsAttentionBoundary(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setupOccupancyReader(t)
	reader := rentalspg.NewOccupancyReader(f.pool, occFixedCalendar{})

	// The planned-end day itself is still active (StatusOf canon); the day
	// after it needs attention.
	onEndID := f.seedOwnerWithProperty(ctx, t)
	endDay := occToday
	f.seedRental(ctx, t, onEndID, mustOccDate("2026-01-01"), &endDay, nil)

	afterEndID := f.seedOwnerWithProperty(ctx, t)
	yesterday := occToday.AddDate(0, 0, -1)
	f.seedRental(ctx, t, afterEndID, mustOccDate("2026-01-01"), &yesterday, nil)

	owners := map[uuid.UUID]uuid.UUID{
		onEndID:    f.ownerOf(ctx, t, onEndID),
		afterEndID: f.ownerOf(ctx, t, afterEndID),
	}
	got, err := reader.OccupancyByProperty(ctx, owners)
	require.NoError(t, err)

	assert.Equal(t, "active", string(got[onEndID].Status))
	assert.Equal(t, "needs_attention", string(got[afterEndID].Status))
}

func TestOccupancyReader_EmptyAndForeignRentals(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setupOccupancyReader(t)
	reader := rentalspg.NewOccupancyReader(f.pool, occFixedCalendar{})

	got, err := reader.OccupancyByProperty(ctx, map[uuid.UUID]uuid.UUID{})
	require.NoError(t, err)
	assert.Empty(t, got)

	propID := f.seedOwnerWithProperty(ctx, t)
	end := mustOccDate("2027-01-01")
	f.seedRental(ctx, t, propID, mustOccDate("2026-01-01"), &end, nil)

	// A property id outside the owners map must not leak its rental.
	other, err := uuid.NewV7()
	require.NoError(t, err)
	got, err = reader.OccupancyByProperty(ctx, map[uuid.UUID]uuid.UUID{other: f.ownerOf(ctx, t, propID)})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "none", string(got[other].Status))
}
