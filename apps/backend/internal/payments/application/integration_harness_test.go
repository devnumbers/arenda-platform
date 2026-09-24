//go:build integration

package application_test

// The single integration harness of the payments context tests: the tick
// service and the payment use case service over one store factory (mirroring
// the wire), a mutable fake clock, an injectable policy and the shared
// seeding and projection helpers. Both test families — the materialization
// tick and the payment CRUD/pause use cases — run on it; ticket #461's
// operations tests land here too.

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	accesspg "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/postgres"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	auditpg "github.com/nambers/arenda-planform/apps/backend/internal/audit/adapters/postgres"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	paymentspg "github.com/nambers/arenda-planform/apps/backend/internal/payments/adapters/postgres"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	pgdb "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
	rentalspg "github.com/nambers/arenda-planform/apps/backend/internal/rentals/adapters/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
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

// stubPropertyPolicy resolves every actor to one configured role — the
// payments role-matrix double. The real membership policy is the access
// context's own seam; here only the role resolution contract matters.
type stubPropertyPolicy struct{ role sharedpolicy.Role }

func (p stubPropertyPolicy) Role(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error) {
	return sharedpolicy.RoleNone, nil
}

func (p stubPropertyPolicy) RoleForProperty(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error) {
	return p.role, nil
}

// paymentsHarness wires the whole payments context — the tick service, the
// payment use case service and the operations use case service over one store
// factory with the real audit recorder — to real PostgreSQL with a mutable
// clock and an injectable policy (testcontainers PostgreSQL 18 or
// TEST_DATABASE_URL).
type paymentsHarness struct {
	t      *testing.T
	pool   *pgxpool.Pool
	clock  *mutableClock
	tick   *paymentsapp.TickService
	svc    *paymentsapp.PaymentService
	ops    *paymentsapp.OperationService
	global *paymentsapp.GlobalPaymentService
	owner  uuid.UUID
	propID uuid.UUID
}

func newPaymentsHarness(t *testing.T) *paymentsHarness {
	t.Helper()
	return newPaymentsHarnessWithPolicy(t, nil)
}

func newPaymentsHarnessWithPolicy(t *testing.T, policy sharedpolicy.Policy) *paymentsHarness {
	t.Helper()

	pool := testdb.Setup(t)
	clk := &mutableClock{now: tickBaseTime}
	logger := slog.New(slog.DiscardHandler)

	tickStore := paymentspg.NewTickStore(pool)
	paymentStore := paymentspg.NewPaymentStore(pool)
	operationStore := paymentspg.NewOperationStore(pool)
	propertyStore := paymentspg.NewPropertyStore(pool)
	favoriteOrders := paymentspg.NewGlobalPaymentStore(pool)
	audit := auditapp.NewService(auditpg.NewWriter(pool), clk)
	uow := pgdb.NewUoW(pool, logger)
	calendar := paymentspg.NewOwnerCalendar(pool, clk)
	zones := paymentspg.NewTickZoneDirectory(pool)
	factory := paymentsapp.NewTxStoreFactory(
		tickStore, paymentStore, operationStore, propertyStore, favoriteOrders,
		rentalspg.NewRentalLinkReader(pool), audit, uow,
	)

	return &paymentsHarness{
		t:      t,
		pool:   pool,
		clock:  clk,
		tick:   paymentsapp.NewTickService(factory, zones, calendar, nil),
		svc:    paymentsapp.NewPaymentService(factory, calendar, policy),
		ops:    paymentsapp.NewOperationService(factory, calendar, policy),
		global: paymentsapp.NewGlobalPaymentService(favoriteOrders, calendar, factory),
	}
}

// newPaymentsHarnessWithRealPolicy is the harness for the global feed's
// member scenarios (ticket #540): the production membership policy over the
// harness pool, so the propertyIds view gate exercises the same authorization
// the wire runs. The services are rebuilt over the same store wiring as the
// harness's own — only the policy changes.
func newPaymentsHarnessWithRealPolicy(t *testing.T) *paymentsHarness {
	t.Helper()
	h := newPaymentsHarness(t)
	policy := accessapp.NewMembershipPolicy(
		accesspg.NewOwnerResolver(h.pool),
		accesspg.NewMembershipRepository(h.pool),
	)
	clk := h.clock
	logger := slog.New(slog.DiscardHandler)
	factory := paymentsapp.NewTxStoreFactory(
		paymentspg.NewTickStore(h.pool),
		paymentspg.NewPaymentStore(h.pool),
		paymentspg.NewOperationStore(h.pool),
		paymentspg.NewPropertyStore(h.pool),
		paymentspg.NewGlobalPaymentStore(h.pool),
		rentalspg.NewRentalLinkReader(h.pool),
		auditapp.NewService(auditpg.NewWriter(h.pool), clk),
		pgdb.NewUoW(h.pool, logger),
	)
	calendar := paymentspg.NewOwnerCalendar(h.pool, clk)
	h.tick = paymentsapp.NewTickService(factory, paymentspg.NewTickZoneDirectory(h.pool), calendar, nil)
	h.svc = paymentsapp.NewPaymentService(factory, calendar, policy)
	h.ops = paymentsapp.NewOperationService(factory, calendar, policy)
	h.global = paymentsapp.NewGlobalPaymentService(paymentspg.NewGlobalPaymentStore(h.pool), calendar, factory)
	return h
}

// withOwner seeds a user (the data owner) with the given timezone and one
// active property, and returns the harness scoped to them.
func (h *paymentsHarness) withOwner(tz string) *paymentsHarness {
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

// seedActor seeds a bare user row so audit_log's actor FK holds for members
// and strangers of the role-matrix tests.
func (h *paymentsHarness) seedActor(id uuid.UUID) {
	t := h.t
	t.Helper()
	phone := fmt.Sprintf("+7999%010d", time.Now().UnixNano()%10000000000)
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO users (id, phone, role, timezone) VALUES ($1, $2, $3, 'Europe/Moscow')`,
		id, phone, actor.RoleOwner,
	); err != nil {
		t.Fatalf("seed actor: %v", err)
	}
}

// seedPayment inserts a payment rule directly. Recurrence is the raw jsonb
// payload — the tick tests drive materialization over pre-seeded rules.
func (h *paymentsHarness) seedPayment(since, recurrence string, autoPay bool) uuid.UUID {
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
func (h *paymentsHarness) seedPause(paymentID uuid.UUID, from string, to *string) {
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

// createCmd is the canonical valid create fixture: daily rent.
func (h *paymentsHarness) createCmd() paymentsapp.CreatePaymentCommand {
	slug := testIntegrationSlugRent
	return paymentsapp.CreatePaymentCommand{
		Type:          domain.TypeExpense,
		Title:         "Аренда",
		AmountKopecks: 5000000,
		Recurrence:    domain.NewDailyRecurrence(),
		PaymentForm:   domain.FormTransfer,
		CategorySlug:  slug,
	}
}

func (h *paymentsHarness) ctx() context.Context { return context.Background() }

// runTick runs the worker-path tick over the harness owner.
func (h *paymentsHarness) runTick() {
	h.t.Helper()
	if err := h.tick.RunOwnerTick(h.ctx(), h.owner); err != nil {
		h.t.Fatalf("run owner tick: %v", err)
	}
}

// operationFullRow is the projection-free row of the shared operations
// loader: everything any test family needs, read once.
type operationFullRow struct {
	Date                 string
	Status               string
	PaidDate             *string
	AmountKopecks        int64
	CreatedAt, UpdatedAt time.Time
}

// loadOperations reads a rule's ordered operations straight from the table —
// the single SQL home of both families' fixtures inspection.
func (h *paymentsHarness) loadOperations(t *testing.T, paymentID uuid.UUID) []operationFullRow {
	t.Helper()
	rows, err := h.pool.Query(h.ctx(), `
		SELECT date::text, status, paid_date::text, amount_kopecks, created_at, updated_at
		FROM operations WHERE payment_id = $1 ORDER BY date`, paymentID)
	if err != nil {
		t.Fatalf("query operations: %v", err)
	}
	defer rows.Close()

	out := []operationFullRow{}
	for rows.Next() {
		var r operationFullRow
		var paid *string
		if err := rows.Scan(&r.Date, &r.Status, &paid, &r.AmountKopecks, &r.CreatedAt, &r.UpdatedAt); err != nil {
			t.Fatalf("scan operation: %v", err)
		}
		r.PaidDate = paid
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate operations: %v", err)
	}
	return out
}

// opRow is the tick family's projection of one materialized operation.
type opRow struct {
	date     string
	status   string
	paidDate *string
}

// operationsOf projects the shared loader onto the tick family's shape.
func (h *paymentsHarness) operationsOf(paymentID uuid.UUID) []opRow {
	rows := h.loadOperations(h.t, paymentID)
	out := make([]opRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, opRow{date: row.Date, status: row.Status, paidDate: row.PaidDate})
	}
	return out
}

// statusesOf projects the tick family's operations into a date→status map.
func statusesOf(ops []opRow) map[string]string {
	out := make(map[string]string, len(ops))
	for _, op := range ops {
		out[op.date] = op.status
	}
	return out
}

// svcOp is the use-case family's projection of one operation (the amount
// snapshot matters there; timestamps do not).
type svcOp struct {
	date   string
	status string
	amount int64
}

// opsOf projects the shared loader onto the use-case family's shape.
func (h *paymentsHarness) opsOf(t *testing.T, paymentID uuid.UUID) []svcOp {
	t.Helper()
	rows := h.loadOperations(t, paymentID)
	out := make([]svcOp, 0, len(rows))
	for _, row := range rows {
		out = append(out, svcOp{date: row.Date, status: row.Status, amount: row.AmountKopecks})
	}
	return out
}

// opStatuses projects the use-case family's operations into a date→status map.
func opStatuses(ops []svcOp) map[string]string {
	out := make(map[string]string, len(ops))
	for _, op := range ops {
		out[op.date] = op.status
	}
	return out
}

// auditActions loads the payment's audit trail actions in recording order.
func (h *paymentsHarness) auditActions(t *testing.T, entityID uuid.UUID) []string {
	t.Helper()
	rows, err := h.pool.Query(h.ctx(),
		`SELECT action FROM audit_log WHERE entity_type = 'payment' AND entity_id = $1 ORDER BY created_at, id`,
		entityID)
	if err != nil {
		t.Fatalf("query audit: %v", err)
	}
	defer rows.Close()
	actions := []string{}
	for rows.Next() {
		var action string
		if err := rows.Scan(&action); err != nil {
			t.Fatalf("scan audit: %v", err)
		}
		actions = append(actions, action)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate audit: %v", err)
	}
	return actions
}
