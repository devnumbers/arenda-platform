//go:build integration

package postgres_test

// The integration family of the payments overdue reader (ticket #585): the
// per-owner batched overdue detection over real PostgreSQL — the same
// planned-before-owner-today predicate the payments listings resolve as the
// overdue view status (ADR 0048).

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/adapters/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overdueToday anchors the fake calendar: every owner's fixed calendar date.
var overdueToday = mustOverdueDate("2026-09-10")

func mustOverdueDate(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

type overdueFixedCalendar struct{}

func (overdueFixedCalendar) Today(context.Context, uuid.UUID) (time.Time, error) {
	return overdueToday, nil
}

type overdueFixture struct {
	pool *pgxpool.Pool
}

func setupOverdueReader(t *testing.T) *overdueFixture {
	t.Helper()
	return &overdueFixture{pool: testdb.Setup(t)}
}

// seedPropertyWithOwner inserts one owner (Europe/Moscow) and one property.
func (f *overdueFixture) seedPropertyWithOwner(
	ctx context.Context, t *testing.T,
) (propID, ownerID uuid.UUID) {
	t.Helper()
	owner, err := uuid.NewV7()
	require.NoError(t, err)
	phone := fmt.Sprintf("+7999%010d", time.Now().UnixNano()%10000000000)
	_, err = f.pool.Exec(ctx,
		`INSERT INTO users (id, phone, role, timezone) VALUES ($1, $2, $3, 'Europe/Moscow')`,
		owner, phone, actor.RoleOwner)
	require.NoError(t, err)
	propID, err = uuid.NewV7()
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx,
		`INSERT INTO properties (id, owner_id, name, type, address, status)
		 VALUES ($1, $2, 'Объект', 'apartment', 'Москва, Тверская 1', 'active')`,
		propID, owner)
	require.NoError(t, err)
	return propID, owner
}

// seedOperation inserts one operation in the given stored status at the
// given operation date.
func (f *overdueFixture) seedOperation(
	ctx context.Context, t *testing.T, propID, ownerID uuid.UUID, date, status string,
) {
	t.Helper()
	id, err := uuid.NewV7()
	require.NoError(t, err)
	var paid any
	if status == "paid" {
		paid = date
	}
	_, err = f.pool.Exec(ctx,
		`INSERT INTO operations (id, owner_id, property_id, payment_id, origin, date, paid_date,
		                       status, type, title, amount_kopecks, payment_form, category_label, category_slug)
		 VALUES ($1, $2, $3, NULL, 'manual', $4::date, $5::date,
		         $6, 'expense', 'ЖКУ', 500000, 'transfer', 'Коммунальные услуги', 'utilities')`,
		id, ownerID, propID, date, paid, status)
	require.NoError(t, err)
}

func TestOverdueReader_PlannedBeforeTodayIsOverdue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setupOverdueReader(t)
	reader := postgres.NewOverdueOperationsReader(f.pool, overdueFixedCalendar{})

	overdueID, owner1 := f.seedPropertyWithOwner(ctx, t)
	f.seedOperation(ctx, t, overdueID, owner1, "2026-09-09", "planned")

	futureID, owner2 := f.seedPropertyWithOwner(ctx, t)
	f.seedOperation(ctx, t, futureID, owner2, "2026-09-10", "planned") // On today — not yet overdue.
	f.seedOperation(ctx, t, futureID, owner2, "2026-10-01", "planned")

	paidID, owner3 := f.seedPropertyWithOwner(ctx, t)
	f.seedOperation(ctx, t, paidID, owner3, "2026-09-01", "paid") // The fact beats the stale date.

	cleanID, owner4 := f.seedPropertyWithOwner(ctx, t)

	owners := map[uuid.UUID]uuid.UUID{
		overdueID: owner1,
		futureID:  owner2,
		paidID:    owner3,
		cleanID:   owner4,
	}
	got, err := reader.OverdueByProperty(ctx, owners)
	require.NoError(t, err)

	assert.True(t, got[overdueID], "planned before the owner's today is overdue")
	assert.False(t, got[futureID], "today and future planned rows are not overdue")
	assert.False(t, got[paidID], "a paid operation is never overdue")
	assert.False(t, got[cleanID], "a property without operations is not overdue")
}

func TestOverdueReader_PerOwnerBatching(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setupOverdueReader(t)
	reader := postgres.NewOverdueOperationsReader(f.pool, overdueFixedCalendar{})

	// Two properties of one owner: one overdue, one clean — the per-owner
	// batch must not smear the answer across the owner's set.
	propA, owner := f.seedPropertyWithOwner(ctx, t)
	f.seedOperation(ctx, t, propA, owner, "2026-09-01", "planned")
	propB, _ := f.seedPropertyWithOwner(ctx, t)

	// Another owner's overdue operation on their own property must not leak
	// into the first owner's scope.
	propC, otherOwner := f.seedPropertyWithOwner(ctx, t)
	f.seedOperation(ctx, t, propC, otherOwner, "2026-09-01", "planned")

	got, err := reader.OverdueByProperty(ctx, map[uuid.UUID]uuid.UUID{
		propA: owner,
		propB: owner,
		propC: otherOwner,
	})
	require.NoError(t, err)

	assert.True(t, got[propA])
	assert.False(t, got[propB], "the owner's other property stays clean")
	assert.True(t, got[propC])
}

func TestOverdueReader_EmptyScope(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setupOverdueReader(t)
	reader := postgres.NewOverdueOperationsReader(f.pool, overdueFixedCalendar{})

	got, err := reader.OverdueByProperty(ctx, map[uuid.UUID]uuid.UUID{})
	require.NoError(t, err)
	assert.Empty(t, got)
}
