//go:build integration

// Package postgres_test holds the payments adapter integration tests. This
// file pins the in-transaction seam's real-SQL properties (ADR 0048 mutation
// shape, ADR 0033 test fixtures): the tick store bound to a caller's
// transaction works while that transaction already holds the property row
// lock, sees its uncommitted writes, and commits or rolls back with it. The
// orchestration above the store (txStores.tickOwner) is covered by the
// application unit test with a fake store — together they are the no-deadlock
// proof a standalone run could not provide.
package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	paymentspg "github.com/nambers/arenda-planform/apps/backend/internal/payments/adapters/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
	pgxgen "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
)

// tickTxClock is a fixed clock.Clock.
type tickTxClock struct{ now time.Time }

func (c tickTxClock) Now() time.Time { return c.now }

// tickTxHarness seeds one owner with a timezone and one active property.
type tickTxHarness struct {
	t      *testing.T
	pool   *pgxpool.Pool
	store  *paymentspg.TickStore
	owner  uuid.UUID
	propID uuid.UUID
}

func newTickTxHarness(t *testing.T, tz string) *tickTxHarness {
	t.Helper()

	pool := testdb.Setup(t)

	owner, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	phone := fmt.Sprintf("+7999%010d", time.Now().UnixNano()%10000000000)
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO users (id, phone, role, timezone) VALUES ($1, $2, $3, $4)`,
		owner, phone, actor.RoleOwner, tz,
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	propID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO properties (id, owner_id, name, type, address, status)
		 VALUES ($1, $2, 'Квартира', 'apartment', 'Москва, Тверская 1', 'active')`,
		propID, owner,
	); err != nil {
		t.Fatalf("seed property: %v", err)
	}
	return &tickTxHarness{
		t:      t,
		pool:   pool,
		store:  paymentspg.NewTickStore(pool),
		owner:  owner,
		propID: propID,
	}
}

// createPaymentInTx writes a daily rule on the caller's transaction — the
// mutation's own change the tick must react to.
func (h *tickTxHarness) createPaymentInTx(ctx context.Context, dbtx pgxgen.DBTX, since time.Time) (uuid.UUID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.UUID{}, err
	}
	_, err = dbtx.Exec(ctx,
		`INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks,
		                       recurrence, since, auto_pay, category_slug)
		 VALUES ($1, $2, $3, 'expense', 'ЖКУ', 500000, '{"kind":"daily"}'::jsonb, $4, false, 'utilities')`,
		id, h.owner, h.propID, since,
	)
	return id, err
}

// countOperations reports the owner's materialized operations outside any
// transaction.
func (h *tickTxHarness) countOperations() int {
	h.t.Helper()
	var n int
	if err := h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM operations WHERE owner_id = $1`, h.owner,
	).Scan(&n); err != nil {
		h.t.Fatalf("count operations: %v", err)
	}
	return n
}

func TestTickStore_BoundToCallerTransactionUnderPropertyLock(t *testing.T) {
	t.Parallel()
	h := newTickTxHarness(t, "Europe/Moscow")
	ctx := context.Background()

	today := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)

	// The mutation's transaction: take the property lock, write a rule, then
	// use the tx-bound tick store — the exact shape of a #457 mutation that
	// re-ticks after its change. A standalone tick run would deadlock here;
	// the bound store re-enters the same transaction's lock.
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if err := tx.Rollback(ctx); err != nil {
			// Rollback failure means the aborted test transaction's rows
			// would persist in the shared database.
			t.Errorf("rollback tick tx: %v", err)
		}
	}()

	bound, err := h.store.WithTx(tx)
	if err != nil {
		t.Fatalf("bind tick store to tx: %v", err)
	}
	if err := bound.LockOwnerProperties(ctx, h.owner); err != nil {
		t.Fatalf("lock owner properties (held-lock re-entry): %v", err)
	}
	paymentID, err := h.createPaymentInTx(ctx, tx, today)
	if err != nil {
		t.Fatalf("create payment in tx: %v", err)
	}

	// The snapshot must see the caller's uncommitted rule through WithTx.
	snapshot, err := bound.LoadOwnerSnapshot(ctx, h.owner)
	if err != nil {
		t.Fatalf("load owner snapshot in tx: %v", err)
	}
	if len(snapshot.Payments) != 1 || snapshot.Payments[0].ID != paymentID {
		t.Fatalf("snapshot does not see the uncommitted rule: %+v", snapshot.Payments)
	}

	// Applying the rule's plan writes through the same transaction.
	plan := domain.PlanPaymentTick(snapshot.Payments[0], today, snapshot.Statuses[paymentID])
	if err := bound.ApplyTickPlan(ctx, snapshot.Payments[0], today, plan); err != nil {
		t.Fatalf("apply tick plan in tx: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	committed = true
	if got := h.countOperations(); got != 2 {
		t.Fatalf("operations after commit = %d, want 2 (today + single future planned)", got)
	}
}

func TestTickStore_BoundWritesRollBackWithCaller(t *testing.T) {
	t.Parallel()
	h := newTickTxHarness(t, "Europe/Moscow")
	ctx := context.Background()

	today := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	rolledBack := false
	defer func() {
		if rolledBack {
			return
		}
		if err := tx.Rollback(ctx); err != nil {
			// Rollback failure means the aborted test transaction's rows
			// would persist in the shared database.
			t.Errorf("rollback tick tx: %v", err)
		}
	}()

	bound, err := h.store.WithTx(tx)
	if err != nil {
		t.Fatalf("bind tick store to tx: %v", err)
	}
	if _, err := h.createPaymentInTx(ctx, tx, today); err != nil {
		t.Fatalf("create payment in tx: %v", err)
	}
	snapshot, err := bound.LoadOwnerSnapshot(ctx, h.owner)
	if err != nil {
		t.Fatalf("load owner snapshot in tx: %v", err)
	}
	plan := domain.PlanPaymentTick(snapshot.Payments[0], today, snapshot.Statuses[snapshot.Payments[0].ID])
	if err := bound.ApplyTickPlan(ctx, snapshot.Payments[0], today, plan); err != nil {
		t.Fatalf("apply tick plan in tx: %v", err)
	}

	// The caller abandons the transaction: the tick's writes vanish with it.
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	rolledBack = true
	if got := h.countOperations(); got != 0 {
		t.Fatalf("operations after rollback = %d, want 0 — tick writes commit only with the caller", got)
	}
}

// The calendar side of the same seam, through the real adapter.
func TestOwnerCalendar_TodayByOwnerTimezone(t *testing.T) {
	t.Parallel()
	h := newTickTxHarness(t, "Asia/Kamchatka")

	// 2026-08-25 20:00 UTC is already the 26th in Kamchatka.
	calendar := paymentspg.NewOwnerCalendar(h.pool, tickTxClock{now: time.Date(2026, 8, 25, 20, 0, 0, 0, time.UTC)})
	today, err := calendar.Today(context.Background(), h.owner)
	if err != nil {
		t.Fatalf("owner today: %v", err)
	}
	if got := today.Format(time.DateOnly); got != "2026-08-26" {
		t.Fatalf("Kamchatka today = %s, want 2026-08-26 — the calendar owns the normalization", got)
	}
}
