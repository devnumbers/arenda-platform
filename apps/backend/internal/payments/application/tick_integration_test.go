//go:build integration

package application_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	paymentspg "github.com/nambers/arenda-planform/apps/backend/internal/payments/adapters/postgres"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	pgdb "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
)

// tickBaseTime anchors the fake clock: 2026-08-25 20:00 UTC, already past
// midnight in Kamchatka (the 26th) and still the 25th in Moscow — the TZ test
// relies on that split.
var tickBaseTime = time.Date(2026, 8, 25, 20, 0, 0, 0, time.UTC)

// Calendar and status fixtures: today (in Moscow) is 2026-08-25.
const (
	day22     = "2026-08-22"
	day23     = "2026-08-23"
	day25     = "2026-08-25"
	day26     = "2026-08-26"
	opPlanned = "planned"
	opPaid    = "paid"
)

// mutableClock is a fake clock.Clock whose Now can be advanced mid-test.
type mutableClock struct{ now time.Time }

func (c *mutableClock) Now() time.Time { return c.now }

// tickHarness wires the tick service to real PostgreSQL through a
// postgres-backed Unit-of-Work, a fake clock and the postgres timezone
// resolver (testcontainers PostgreSQL 18 or TEST_DATABASE_URL).
type tickHarness struct {
	t      *testing.T
	pool   *pgxpool.Pool
	clock  *mutableClock
	tick   *paymentsapp.TickService
	owner  uuid.UUID
	propID uuid.UUID
}

func newTickHarness(t *testing.T) *tickHarness {
	t.Helper()

	pool := testdb.Setup(t)
	clk := &mutableClock{now: tickBaseTime}
	logger := slog.New(slog.DiscardHandler)

	tickStore := paymentspg.NewTickStore(pool)
	uow := pgdb.NewUoW(pool, logger)
	tz := paymentspg.NewOwnerTimezoneResolver(pool)
	factory := paymentsapp.NewTxStoreFactory(tickStore, uow)

	return &tickHarness{
		t:     t,
		pool:  pool,
		clock: clk,
		tick:  paymentsapp.NewTickService(factory, tz, clk),
	}
}

// withOwner seeds a user (the data owner) with the given timezone and one
// active property, and returns the harness scoped to them.
func (h *tickHarness) withOwner(tz string) *tickHarness {
	t := h.t
	t.Helper()

	owner, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	phone := fmt.Sprintf("+7999%010d", time.Now().UnixNano()%10000000000)
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO users (id, phone, role, timezone) VALUES ($1, $2, $3, $4)`,
		owner, phone, actor.RoleOwner, tz,
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	propID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO properties (id, owner_id, name, type, address, status)
		 VALUES ($1, $2, 'Квартира', 'apartment', 'Москва, Тверская 1', 'active')`,
		propID, owner,
	); err != nil {
		t.Fatalf("seed property: %v", err)
	}
	h.owner = owner
	h.propID = propID
	return h
}

// seedPayment inserts a payment rule. Recurrence is the raw jsonb payload.
func (h *tickHarness) seedPayment(since, recurrence string, autoPay bool) uuid.UUID {
	h.t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		h.t.Fatalf("new uuid: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks,
		                       recurrence, since, end_date, auto_pay, payment_form, category_slug)
		 VALUES ($1, $2, $3, 'expense', 'ЖКУ', 500000, $4::jsonb, $5, NULL, $6, 'transfer', 'utilities')`,
		id, h.owner, h.propID, recurrence, since, autoPay,
	); err != nil {
		h.t.Fatalf("seed payment: %v", err)
	}
	return id
}

// seedPause inserts a pause interval [from, to); nil to is the active
// open-ended pause.
func (h *tickHarness) seedPause(paymentID uuid.UUID, from string, to *string) {
	h.t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		h.t.Fatalf("new uuid: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO payment_pauses (id, payment_id, from_date, to_date) VALUES ($1, $2, $3, $4)`,
		id, paymentID, from, to,
	); err != nil {
		h.t.Fatalf("seed pause: %v", err)
	}
}

// opRow is the projection tests assert on: the materialized state of one
// operation.
type opRow struct {
	date     string
	status   string
	paidDate *string
}

// operationsOf loads the payment's operations with their full row identity
// (created_at, updated_at included) so a no-op rerun can be proven.
func (h *tickHarness) operationsOf(paymentID uuid.UUID) []opRow {
	h.t.Helper()
	rows, err := h.pool.Query(h.ctx(), `
		SELECT date::text, status, paid_date::text, created_at, updated_at
		FROM operations WHERE payment_id = $1 ORDER BY date`, paymentID)
	if err != nil {
		h.t.Fatalf("query operations: %v", err)
	}
	defer rows.Close()

	out := []opRow{}
	for rows.Next() {
		var (
			r        opRow
			created  time.Time
			updated  time.Time
			paidDate *string
		)
		if err := rows.Scan(&r.date, &r.status, &paidDate, &created, &updated); err != nil {
			h.t.Fatalf("scan operation: %v", err)
		}
		r.paidDate = paidDate
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		h.t.Fatalf("iterate operations: %v", err)
	}
	return out
}

// statusesOf projects the operations into a date→status map.
func statusesOf(ops []opRow) map[string]string {
	out := make(map[string]string, len(ops))
	for _, op := range ops {
		out[op.date] = op.status
	}
	return out
}

func (h *tickHarness) runTick() {
	h.t.Helper()
	if err := h.tick.RunOwnerTick(h.ctx(), h.owner); err != nil {
		h.t.Fatalf("run owner tick: %v", err)
	}
}

func (h *tickHarness) ctx() context.Context { return context.Background() }

func TestTick_MaterializesDueAndSingleFutureThenRerunIsNoop(t *testing.T) {
	t.Parallel()
	h := newTickHarness(t).withOwner("Europe/Moscow")
	paymentID := h.seedPayment(day22, `{"kind":"daily"}`, false)

	h.runTick()

	ops := h.operationsOf(paymentID)
	// Daily since T-3 (today is the 25th in Moscow): 4 due (3 overdue + today)
	// + exactly one future planned (prototype smoke "догон по старому since").
	want := map[string]string{
		day22:        opPlanned,
		day23:        opPlanned,
		"2026-08-24": opPlanned,
		day25:        opPlanned,
		day26:        opPlanned,
	}
	if len(ops) != len(want) {
		t.Fatalf("operations count = %d, want %d: %+v", len(ops), len(want), ops)
	}
	for date, status := range want {
		if got := statusesOf(ops)[date]; got != status {
			t.Fatalf("operation %s status = %q, want %q", date, got, status)
		}
	}

	// The rerun is a full no-op: same rows, same timestamps — the dedup index
	// plus the idempotent plan converge without writing.
	before := h.operationsOf(paymentID)
	h.runTick()
	after := h.operationsOf(paymentID)
	if len(before) != len(after) {
		t.Fatalf("rerun changed row count: %d → %d", len(before), len(after))
	}
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("rerun rewrote row %+v → %+v", before[i], after[i])
		}
	}
}

func TestTick_AutoPayClosesOnlyToday(t *testing.T) {
	t.Parallel()
	h := newTickHarness(t).withOwner("Europe/Moscow")
	paymentID := h.seedPayment(day23, `{"kind":"daily"}`, true)

	h.runTick()

	ops := statusesOf(h.operationsOf(paymentID))
	if got := ops[day23]; got != opPlanned {
		t.Fatalf("the day before yesterday-equivalent (2026-08-23) = %q, want planned — no backdated catch-up (ADR 0049 §2)", got)
	}
	if got := ops["2026-08-24"]; got != opPlanned {
		t.Fatalf("2026-08-24 = %q, want planned — yesterday remains debt", got)
	}
	if got := ops[day25]; got != opPaid {
		t.Fatalf("today = %q, want paid — the auto-pay closes exactly the current day", got)
	}
	if got := ops[day26]; got != opPlanned {
		t.Fatalf("tomorrow = %q, want planned — exactly one future planned", got)
	}
	for _, op := range h.operationsOf(paymentID) {
		if op.status == opPaid {
			if op.paidDate == nil || *op.paidDate != day25 {
				t.Fatalf("paid operation %s has paid_date %v, want 2026-08-25", op.date, op.paidDate)
			}
		}
	}
}

func TestTick_WorkerMissedTheDayLeavesDebt(t *testing.T) {
	t.Parallel()
	// The tick did not run on the 24th (clock jumped from the 23rd straight
	// to the 25th): the missed occurrence stays planned debt even though the
	// rule is an auto-pay.
	h := newTickHarness(t).withOwner("Europe/Moscow")
	paymentID := h.seedPayment("2026-08-20", `{"kind":"daily"}`, true)

	h.runTick()

	ops := statusesOf(h.operationsOf(paymentID))
	for _, missed := range []string{"2026-08-20", "2026-08-21", day22, day23, "2026-08-24"} {
		if got := ops[missed]; got != opPlanned {
			t.Fatalf("missed day %s = %q, want planned — a worker outage leaves honest debt", missed, got)
		}
	}
	if got := ops[day25]; got != opPaid {
		t.Fatalf("today = %q, want paid", got)
	}
}

func TestTick_ActivePauseExcludesOccurrences(t *testing.T) {
	t.Parallel()
	// Daily since T-3, paused from T-1 on: occurrences before the pause stay,
	// nothing generates during the pause, no future planned remains, and the
	// debt accumulated before the pause is untouched.
	h := newTickHarness(t).withOwner("Europe/Moscow")
	paymentID := h.seedPayment(day22, `{"kind":"daily"}`, true)
	h.seedPause(paymentID, "2026-08-24", nil)

	h.runTick()

	ops := statusesOf(h.operationsOf(paymentID))
	want := map[string]string{
		day22: opPlanned,
		day23: opPlanned,
	}
	if len(ops) != len(want) {
		t.Fatalf("operations = %+v, want exactly %+v (pause cuts the rest)", ops, want)
	}
	for date, status := range want {
		if ops[date] != status {
			t.Fatalf("operation %s = %q, want %q", date, ops[date], status)
		}
	}

	// Days advance inside the pause: still nothing new, no debt accrues.
	h.clock.now = tickBaseTime.AddDate(0, 0, 5)
	h.runTick()
	if got := len(h.operationsOf(paymentID)); got != len(want) {
		t.Fatalf("operations after 5 paused days = %d, want %d", got, len(want))
	}
}

func TestTick_PausedAutoPayDoesNotCloseToday(t *testing.T) {
	t.Parallel()
	// Yesterday's tick stood the future planned on today; today the rule is
	// paused before the tick runs: the standing occurrence stays planned
	// (future debt), the pause stops the auto-pay day payment and removes the
	// future planned.
	h := newTickHarness(t).withOwner("Europe/Moscow")
	paymentID := h.seedPayment(day23, `{"kind":"daily"}`, true)

	h.clock.now = tickBaseTime.AddDate(0, 0, -1)
	h.runTick()

	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO payment_pauses (id, payment_id, from_date) VALUES ($1, $2, '2026-08-25')`,
		uuid.Must(uuid.NewV7()), paymentID,
	); err != nil {
		t.Fatalf("seed pause: %v", err)
	}

	h.clock.now = tickBaseTime
	h.runTick()

	ops := statusesOf(h.operationsOf(paymentID))
	if got := ops[day25]; got != opPlanned {
		t.Fatalf("today = %q, want planned — an active pause stops the auto-pay day payment", got)
	}
	if got := ops[day26]; got != "" {
		t.Fatalf("2026-08-26 = %q, want absent — the pause removes the future planned", got)
	}
}

func TestTick_TodayFollowsOwnerTimezone(t *testing.T) {
	t.Parallel()
	// Same instant (2026-08-25 20:00 UTC), two owners in different zones:
	// Kamchatka is already on the 26th, Moscow still on the 25th. Each
	// owner's materialization follows their own calendar date (ADR 0048).
	kamchatka := newTickHarness(t).withOwner("Asia/Kamchatka")
	kPayment := kamchatka.seedPayment("2026-08-20", `{"kind":"daily"}`, false)

	moscow := newTickHarness(t).withOwner("Europe/Moscow")
	mPayment := moscow.seedPayment("2026-08-20", `{"kind":"daily"}`, false)

	kamchatka.runTick()
	moscow.runTick()

	kOps := statusesOf(kamchatka.operationsOf(kPayment))
	if got := kOps[day26]; got != opPlanned {
		t.Fatalf("Kamchatka today (2026-08-26) = %q, want planned — the 26th is already due in UTC+12", got)
	}
	if got := kOps["2026-08-27"]; got != opPlanned {
		t.Fatalf("Kamchatka future planned (2026-08-27) = %q, want planned", got)
	}

	mOps := statusesOf(moscow.operationsOf(mPayment))
	if got := mOps[day25]; got != opPlanned {
		t.Fatalf("Moscow today (2026-08-25) = %q, want planned", got)
	}
	if got := mOps[day26]; got != opPlanned {
		t.Fatalf("Moscow future planned (2026-08-26) = %q, want planned", got)
	}
	if got := mOps["2026-08-27"]; got != "" {
		t.Fatalf("Moscow 2026-08-27 = %q, want absent — Moscow is still on the 25th", got)
	}
}

func TestTick_RebuildsFuturePlannedAfterEarlyPayment(t *testing.T) {
	t.Parallel()
	// Monthly on the 10th; today is the 25th, so the future planned stands on
	// Sep 10. "Оплатить сейчас" (simulated directly) closes it ahead of time:
	// the next tick must stand Oct 10 as the new single future planned without
	// touching the paid fact.
	h := newTickHarness(t).withOwner("Europe/Moscow")
	paymentID := h.seedPayment("2026-01-10", `{"kind":"monthly","dayOfMonth":10}`, false)

	h.runTick()

	if _, err := h.pool.Exec(h.ctx(), `
		UPDATE operations SET status = 'paid', paid_date = '2026-08-25'
		WHERE payment_id = $1 AND date = '2026-09-10'`, paymentID,
	); err != nil {
		t.Fatalf("pay ahead: %v", err)
	}

	h.runTick()

	ops := statusesOf(h.operationsOf(paymentID))
	if got := ops["2026-09-10"]; got != opPaid {
		t.Fatalf("the paid-ahead occurrence = %q, want paid — facts are untouchable", got)
	}
	if got := ops["2026-10-10"]; got != opPlanned {
		t.Fatalf("the occurrence after the paid one = %q, want planned — the schedule does not shift", got)
	}
	if got := ops["2026-11-10"]; got != "" {
		t.Fatalf("2026-11-10 = %q, want absent — exactly one future planned", got)
	}
}

func TestTick_SkipsArchivedProperty(t *testing.T) {
	t.Parallel()
	h := newTickHarness(t).withOwner("Europe/Moscow")
	paymentID := h.seedPayment(day22, `{"kind":"daily"}`, true)

	if _, err := h.pool.Exec(h.ctx(),
		`UPDATE properties SET status = 'archived' WHERE id = $1`, h.propID,
	); err != nil {
		t.Fatalf("archive property: %v", err)
	}

	h.runTick()

	if ops := h.operationsOf(paymentID); len(ops) != 0 {
		t.Fatalf("archived property got %d operations, want 0 — the tick skips archived properties", len(ops))
	}
}

func TestTick_CategorySnapshotFromCatalog(t *testing.T) {
	t.Parallel()
	h := newTickHarness(t).withOwner("Europe/Moscow")
	paymentID := h.seedPayment(day25, `{"kind":"monthly","dayOfMonth":25}`, false)

	h.runTick()

	var label string
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT category_label FROM operations WHERE payment_id = $1 AND date = '2026-08-25'`,
		paymentID,
	).Scan(&label); err != nil {
		t.Fatalf("read snapshot label: %v", err)
	}
	if label != "Коммунальные услуги" {
		t.Fatalf("category label = %q, want the catalog label for slug utilities", label)
	}
}
