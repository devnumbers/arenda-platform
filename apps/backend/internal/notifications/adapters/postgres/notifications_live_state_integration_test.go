package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	accesspg "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/postgres"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	paymentspg "github.com/nambers/arenda-planform/apps/backend/internal/payments/adapters/postgres"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupLiveState prepares the pool, an owner and the live-state adapter over
// the real owner calendar and the real membership policy — the reader's role
// resolves through the policy port the same way the wiring does.
func setupLiveState(t *testing.T) (*FeedLiveState, *pgxpool.Pool, uuid.UUID) {
	t.Helper()
	pool := setupPushDB(t)
	ctx := context.Background()
	ownerID := createPushTestUser(t, ctx, genpostgres.New(pool))
	pol := accessapp.NewMembershipPolicy(accesspg.NewOwnerResolver(pool), accesspg.NewMembershipRepository(pool))
	live := NewFeedLiveState(pool, paymentspg.NewOwnerCalendar(pool, &fixedClock{
		now: time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC),
	}), pol)
	return live, pool, ownerID
}

// createLiveProperty adds one property of the owner: every unfinished rental
// needs its own property (the one-unfinished-per-property unique index).
func createLiveProperty(t *testing.T, pool *pgxpool.Pool, ownerID uuid.UUID) uuid.UUID {
	t.Helper()
	propertyID := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(context.Background(), `
		INSERT INTO properties (id, owner_id, name, type, address, status)
		VALUES ($1, $2, 'Объект', 'apartment', 'Москва, Тверская 1', 'active')`,
		propertyID, ownerID)
	require.NoError(t, err)
	return propertyID
}

// insertRental adds the referenced payment row plus the rental with the
// given dates.
func insertRental(t *testing.T, pool *pgxpool.Pool, ownerID, propertyID uuid.UUID, start, plannedEnd, completed *time.Time) uuid.UUID {
	t.Helper()
	paymentID := insertPayment(t, pool, ownerID, propertyID)
	id := uuid.Must(uuid.NewV7())
	var err error
	switch {
	case plannedEnd != nil && completed != nil:
		_, err = pool.Exec(context.Background(), `
			INSERT INTO rentals (id, owner_id, property_id, payment_id, start_date, planned_end_date, completed_date, utilities)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'included')`,
			id, ownerID, propertyID, paymentID, start, plannedEnd, completed)
	case plannedEnd != nil:
		_, err = pool.Exec(context.Background(), `
			INSERT INTO rentals (id, owner_id, property_id, payment_id, start_date, planned_end_date, utilities)
			VALUES ($1, $2, $3, $4, $5, $6, 'included')`,
			id, ownerID, propertyID, paymentID, start, plannedEnd)
	default:
		_, err = pool.Exec(context.Background(), `
			INSERT INTO rentals (id, owner_id, property_id, payment_id, start_date, utilities)
			VALUES ($1, $2, $3, $4, $5, 'included')`,
			id, ownerID, propertyID, paymentID, start)
	}
	require.NoError(t, err)
	return id
}

// insertPayment adds the rent payment row (the rental's 1:1 link target).
func insertPayment(t *testing.T, pool *pgxpool.Pool, ownerID, propertyID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(context.Background(), `
		INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks, recurrence, since, auto_pay, payment_form, category_slug)
		VALUES ($1, $2, $3, 'income', 'Аренда', 2000000, '{"kind": "monthly"}', '2026-09-01', false, 'transfer', 'rent')`,
		id, ownerID, propertyID)
	require.NoError(t, err)
	return id
}

// testYear is the fixed year of the live-state fixture dates; the owner
// calendar clock sits in September 2026.
const testYear = 2026

func datePtr(m time.Month, d int) *time.Time {
	t := time.Date(testYear, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

// The rental's buttons live exactly in the needs_attention state (ADR 0053):
// not completed, planned end already passed against the owner's today.
func TestFeedLiveState_RentalActionState(t *testing.T) {
	t.Parallel()

	live, pool, ownerID := setupLiveState(t)
	ctx := context.Background()

	awaiting := insertRental(t, pool, ownerID, createLiveProperty(t, pool, ownerID),
		datePtr(time.September, 1), datePtr(time.September, 10), nil)
	state, err := live.RentalActionState(ctx, awaiting)
	require.NoError(t, err)
	assert.True(t, state.Exists)
	assert.True(t, state.AwaitingAction, "the planned end passed, the rental is not completed")

	active := insertRental(t, pool, ownerID, createLiveProperty(t, pool, ownerID),
		datePtr(time.September, 1), datePtr(time.October, 10), nil)
	state, err = live.RentalActionState(ctx, active)
	require.NoError(t, err)
	assert.True(t, state.Exists)
	assert.False(t, state.AwaitingAction, "the planned end has not come — the rental is active (extended)")

	completedAt := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	completed := insertRental(t, pool, ownerID, createLiveProperty(t, pool, ownerID),
		datePtr(time.September, 1), datePtr(time.September, 10), &completedAt)
	state, err = live.RentalActionState(ctx, completed)
	require.NoError(t, err)
	assert.True(t, state.Exists)
	assert.False(t, state.AwaitingAction, "the completion fact is stored")

	state, err = live.RentalActionState(ctx, uuid.Must(uuid.NewV7()))
	require.NoError(t, err)
	assert.False(t, state.Exists, "a deleted rental is gone")
}

func TestFeedLiveState_PaymentOpen(t *testing.T) {
	t.Parallel()

	live, pool, ownerID := setupLiveState(t)
	ctx := context.Background()
	propertyID := createLiveProperty(t, pool, ownerID)

	insertOperation := func(status string) uuid.UUID {
		t.Helper()
		id := uuid.Must(uuid.NewV7())
		var err error
		if status == "paid" {
			_, err = pool.Exec(ctx, `
				INSERT INTO operations (id, owner_id, property_id, origin, date, paid_date, status, type, title, amount_kopecks, category_label)
				VALUES ($1, $2, $3, 'manual', '2026-10-01', '2026-10-01', $4, 'income', 'Аренда', 2000000, 'Аренда')`,
				id, ownerID, propertyID, status)
		} else {
			_, err = pool.Exec(ctx, `
				INSERT INTO operations (id, owner_id, property_id, origin, date, status, type, title, amount_kopecks, category_label)
				VALUES ($1, $2, $3, 'manual', '2026-10-01', $4, 'income', 'Аренда', 2000000, 'Аренда')`,
				id, ownerID, propertyID, status)
		}
		require.NoError(t, err)
		return id
	}

	planned := insertOperation("planned")
	open, err := live.PaymentOpen(ctx, planned)
	require.NoError(t, err)
	assert.True(t, open)

	paid := insertOperation("paid")
	open, err = live.PaymentOpen(ctx, paid)
	require.NoError(t, err)
	assert.False(t, open, "the payment is settled — the button goes")

	cancelled := insertOperation("cancelled")
	open, err = live.PaymentOpen(ctx, cancelled)
	require.NoError(t, err)
	assert.False(t, open)

	open, err = live.PaymentOpen(ctx, uuid.Must(uuid.NewV7()))
	require.NoError(t, err)
	assert.False(t, open, "a deleted operation leaves no button")
}

func TestFeedLiveState_TaskOpen(t *testing.T) {
	t.Parallel()

	live, pool, ownerID := setupLiveState(t)
	ctx := context.Background()
	propertyID := createLiveProperty(t, pool, ownerID)

	insertTask := func(completed *string) uuid.UUID {
		t.Helper()
		id := uuid.Must(uuid.NewV7())
		var err error
		if completed != nil {
			_, err = pool.Exec(ctx, `
				INSERT INTO tasks (id, owner_id, property_id, title, completed_date)
				VALUES ($1, $2, $3, 'Задача', $4)`, id, ownerID, propertyID, *completed)
		} else {
			_, err = pool.Exec(ctx, `
				INSERT INTO tasks (id, owner_id, property_id, title)
				VALUES ($1, $2, $3, 'Задача')`, id, ownerID, propertyID)
		}
		require.NoError(t, err)
		return id
	}

	openTask := insertTask(nil)
	open, err := live.TaskOpen(ctx, openTask)
	require.NoError(t, err)
	assert.True(t, open)

	done := "2026-09-17"
	completedTask := insertTask(&done)
	open, err = live.TaskOpen(ctx, completedTask)
	require.NoError(t, err)
	assert.False(t, open, "a completed task has no button")
}

// The reader's live role decides: the owner and full_access manage, the
// viewer only reads, a stranger and a gone property see nothing.
func TestFeedLiveState_PropertyAccess(t *testing.T) {
	t.Parallel()

	live, pool, ownerID := setupLiveState(t)
	ctx := context.Background()
	propertyID := createLiveProperty(t, pool, ownerID)

	access, err := live.PropertyAccess(ctx, propertyID, ownerID)
	require.NoError(t, err)
	assert.True(t, access.Exists)
	assert.True(t, access.Manageable, "the owner manages")

	addMember := func(role string) uuid.UUID {
		t.Helper()
		userID := createPushTestUser(t, ctx, genpostgres.New(pool))
		_, err := pool.Exec(ctx, `
			INSERT INTO property_members (id, property_id, user_id, role, granted_by)
			VALUES ($1, $2, $3, $4, $5)`,
			uuid.Must(uuid.NewV7()), propertyID, userID, role, ownerID)
		require.NoError(t, err)
		return userID
	}

	fullAccess := addMember("full_access")
	access, err = live.PropertyAccess(ctx, propertyID, fullAccess)
	require.NoError(t, err)
	assert.True(t, access.Exists)
	assert.True(t, access.Manageable)

	viewer := addMember("viewer")
	access, err = live.PropertyAccess(ctx, propertyID, viewer)
	require.NoError(t, err)
	assert.True(t, access.Exists, "the viewer still opens the property")
	assert.False(t, access.Manageable, "the viewer does not manage")

	stranger := createPushTestUser(t, ctx, genpostgres.New(pool))
	access, err = live.PropertyAccess(ctx, propertyID, stranger)
	require.NoError(t, err)
	assert.False(t, access.Exists)
	assert.False(t, access.Manageable)

	access, err = live.PropertyAccess(ctx, uuid.Must(uuid.NewV7()), ownerID)
	require.NoError(t, err)
	assert.False(t, access.Exists, "a deleted property is gone")
}

// The adapter satisfies the consumer-declared port.
var _ notificationsapp.FeedLiveState = (*FeedLiveState)(nil)
