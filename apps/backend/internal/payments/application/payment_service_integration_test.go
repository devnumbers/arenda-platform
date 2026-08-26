//go:build integration

package application_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	auditpg "github.com/nambers/arenda-planform/apps/backend/internal/audit/adapters/postgres"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	paymentspg "github.com/nambers/arenda-planform/apps/backend/internal/payments/adapters/postgres"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	pgdb "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

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

// serviceHarness wires the payment use case service to real PostgreSQL with
// the real audit recorder, a mutable clock and an injectable policy
// (testcontainers PostgreSQL 18 or TEST_DATABASE_URL).
type serviceHarness struct {
	t      *testing.T
	pool   *pgxpool.Pool
	clock  *mutableClock
	svc    *paymentsapp.PaymentService
	owner  uuid.UUID
	propID uuid.UUID
}

func newServiceHarness(t *testing.T) *serviceHarness {
	t.Helper()
	return newServiceHarnessWithPolicy(t, nil)
}

func newServiceHarnessWithPolicy(t *testing.T, policy sharedpolicy.Policy) *serviceHarness {
	t.Helper()

	pool := testdb.Setup(t)
	clk := &mutableClock{now: tickBaseTime}

	tickStore := paymentspg.NewTickStore(pool)
	paymentStore := paymentspg.NewPaymentStore(pool)
	propertyStore := paymentspg.NewPropertyStore(pool)
	audit := auditapp.NewService(auditpg.NewWriter(pool), clk)
	uow := pgdb.NewUoW(pool, slog.New(slog.DiscardHandler))
	calendar := paymentspg.NewOwnerCalendar(pool, clk)
	factory := paymentsapp.NewTxStoreFactory(tickStore, paymentStore, propertyStore, audit, uow)

	return &serviceHarness{
		t:     t,
		pool:  pool,
		clock: clk,
		svc:   paymentsapp.NewPaymentService(factory, calendar, policy),
	}
}

// withOwner seeds a user (the data owner) in Moscow and one active property.
func (h *serviceHarness) withOwner() *serviceHarness {
	t := h.t
	t.Helper()

	owner, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	phone := fmt.Sprintf("+7999%010d", time.Now().UnixNano()%10000000000)
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO users (id, phone, role, timezone) VALUES ($1, $2, $3, 'Europe/Moscow')`,
		owner, phone, actor.RoleOwner,
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

// createCmd is the canonical valid create fixture: weekly rent.
func (h *serviceHarness) createCmd() paymentsapp.CreatePaymentCommand {
	slug := "rent"
	return paymentsapp.CreatePaymentCommand{
		Type:          domain.TypeExpense,
		Title:         "Аренда",
		AmountKopecks: 5000000,
		Recurrence:    domain.NewDailyRecurrence(),
		PaymentForm:   domain.FormTransfer,
		CategorySlug:  slug,
	}
}

func (h *serviceHarness) ctx() context.Context { return context.Background() }

// standaloneTick builds the worker-path tick service over the harness's pool.
func (h *serviceHarness) standaloneTick() *paymentsapp.TickService {
	tickStore := paymentspg.NewTickStore(h.pool)
	uow := pgdb.NewUoW(h.pool, slog.New(slog.DiscardHandler))
	return paymentsapp.NewTickService(
		paymentsapp.NewTxStoreFactory(tickStore, paymentspg.NewPaymentStore(h.pool), paymentspg.NewPropertyStore(h.pool), nil, uow),
		paymentspg.NewOwnerCalendar(h.pool, h.clock),
	)
}

// seedActor seeds a bare user row so audit_log's actor FK holds.
func (h *serviceHarness) seedActor(id uuid.UUID) {
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

// svcOp is the operation projection of the service tests.
type svcOp struct {
	date   string
	status string
	amount int64
}

// opsOf loads the payment's operations ordered by date.
func (h *serviceHarness) opsOf(t *testing.T, paymentID uuid.UUID) []svcOp {
	t.Helper()
	rows, err := h.pool.Query(h.ctx(), `
		SELECT date::text, status, amount_kopecks
		FROM operations WHERE payment_id = $1 ORDER BY date`, paymentID)
	if err != nil {
		t.Fatalf("query operations: %v", err)
	}
	defer rows.Close()
	out := []svcOp{}
	for rows.Next() {
		var r svcOp
		if err := rows.Scan(&r.date, &r.status, &r.amount); err != nil {
			t.Fatalf("scan operation: %v", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate operations: %v", err)
	}
	return out
}

// opStatuses projects the operations into a date-to-status map.
func opStatuses(ops []svcOp) map[string]string {
	out := make(map[string]string, len(ops))
	for _, op := range ops {
		out[op.date] = op.status
	}
	return out
}

// auditActions loads the payment's audit trail actions in recording order.
func (h *serviceHarness) auditActions(t *testing.T, entityID uuid.UUID) []string {
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

func TestPaymentCRUD_FullLifecycle(t *testing.T) {
	t.Parallel()
	h := newServiceHarness(t).withOwner()

	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	// Since is the owner's today (Moscow), never a client value.
	if created.Since.Format(time.DateOnly) != day25 {
		t.Fatalf("since = %s, want %s — the server sets the owner's today", created.Since.Format(time.DateOnly), day25)
	}
	if created.OwnerID != h.owner {
		t.Fatalf("owner_id = %s, want the property owner (denormalized)", created.OwnerID)
	}
	// The in-transaction tick materialized today's occurrence and stood the
	// single future planned.
	ops := opStatuses(h.opsOf(t, created.ID))
	if ops[day25] != opPlanned || ops[day26] != opPlanned {
		t.Fatalf("after create: today=%q tomorrow=%q, want planned/planned", ops[day25], ops[day26])
	}
	if len(ops) != 2 {
		t.Fatalf("after create: %d operations, want 2 (due today + one future)", len(ops))
	}

	assertPaymentReadBack(t, h, created)

	if err := h.svc.DeletePayment(h.ctx(), h.owner, h.propID, created.ID, true); err != nil {
		t.Fatalf("delete payment: %v", err)
	}
	if _, err := h.svc.GetPayment(h.ctx(), h.owner, h.propID, created.ID); !errors.Is(err, paymentsapp.ErrNotFound) {
		t.Fatalf("get after delete = %v, want ErrNotFound", err)
	}
	actions := h.auditActions(t, created.ID)
	want := []string{"payment.created", "payment.deleted"}
	if len(actions) != len(want) {
		t.Fatalf("audit actions = %v, want %v", actions, want)
	}
	for i, action := range want {
		if actions[i] != action {
			t.Fatalf("audit actions = %v, want %v", actions, want)
		}
	}
}

// assertPaymentReadBack checks the read side: the single rule reads back
// whole (with its category reference) and lists first in creation order.
func assertPaymentReadBack(t *testing.T, h *serviceHarness, created domain.Payment) {
	t.Helper()
	got, err := h.svc.GetPayment(h.ctx(), h.owner, h.propID, created.ID)
	if err != nil {
		t.Fatalf("get payment: %v", err)
	}
	if got.Title != "Аренда" || got.Category.SlugString() != "rent" {
		t.Fatalf("get payment = %+v", got)
	}
	list, err := h.svc.ListPayments(h.ctx(), h.owner, h.propID)
	if err != nil {
		t.Fatalf("list payments: %v", err)
	}
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("list = %d items, want the created one", len(list))
	}
}

func TestPaymentUpdate_RebuildsScheduleAndAudits(t *testing.T) {
	t.Parallel()
	h := newServiceHarness(t).withOwner()
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}

	// Partial update: the amount grows and the schedule moves to monthly on
	// the 10th. The materialized operations keep their snapshots; the future
	// planned is rebuilt by the in-transaction tick.
	newTitle := "Аренда 2027"
	newAmount := int64(6000000)
	recurrence := domain.Recurrence{}
	{
		monthly, err := domain.NewMonthlyRecurrence(10)
		if err != nil {
			t.Fatalf("fixture recurrence: %v", err)
		}
		recurrence = monthly
	}
	updated, err := h.svc.UpdatePayment(h.ctx(), h.owner, h.propID, created.ID, paymentsapp.UpdatePaymentCommand{
		Title:         &newTitle,
		AmountKopecks: &newAmount,
		Recurrence:    &recurrence,
	})
	if err != nil {
		t.Fatalf("update payment: %v", err)
	}
	if updated.Title != newTitle || updated.AmountKopecks != newAmount {
		t.Fatalf("updated = %+v", updated)
	}
	if updated.Since.Format(time.DateOnly) != day25 {
		t.Fatalf("since changed by update: %s", updated.Since.Format(time.DateOnly))
	}
	ops := opStatuses(h.opsOf(t, created.ID))
	if ops["2026-09-10"] != opPlanned {
		t.Fatalf("after update: Sep 10 = %q, want planned (rebuilt future)", ops["2026-09-10"])
	}
	if _, still := ops[day26]; still {
		t.Fatalf("after update: the stale daily future planned %s must be gone", day26)
	}

	// Delete with the debt kept (the default): future planned always goes,
	// paid history stays marked «платёж удалён».
	if err := h.svc.DeletePayment(h.ctx(), h.owner, h.propID, created.ID, true); err != nil {
		t.Fatalf("delete payment: %v", err)
	}
	if _, err := h.svc.GetPayment(h.ctx(), h.owner, h.propID, created.ID); !errors.Is(err, paymentsapp.ErrNotFound) {
		t.Fatalf("get after delete = %v, want ErrNotFound", err)
	}

	actions := h.auditActions(t, created.ID)
	want := []string{"payment.created", "payment.updated", "payment.deleted"}
	if len(actions) != len(want) {
		t.Fatalf("audit actions = %v, want %v", actions, want)
	}
	for i, action := range want {
		if actions[i] != action {
			t.Fatalf("audit actions = %v, want %v", actions, want)
		}
	}
}

func TestCreatePayment_RejectsEndDateBeforeToday(t *testing.T) {
	t.Parallel()
	h := newServiceHarness(t).withOwner()

	past := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	cmd := h.createCmd()
	cmd.EndDate = &past
	if _, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, cmd); !errors.Is(err, paymentsapp.ErrInvalidInput) {
		t.Fatalf("create with past endDate = %v, want ErrInvalidInput", err)
	}
	same := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	cmd.EndDate = &same
	if _, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, cmd); err != nil {
		t.Fatalf("create with today endDate: %v", err)
	}
}

func TestCreatePayment_RejectsInvalidRules(t *testing.T) {
	t.Parallel()
	// The single validator of the create/update contract rules is the
	// payment rule module (validateRule) — this is its seam. The rune-count
	// cases pin the drift fix: the limit counts characters, not bytes.
	badEnumType := domain.PaymentType("profit")
	badForm := domain.PaymentForm("crypto")
	unknownSlug := "not-a-catalog-slug"
	cases := []struct {
		name   string
		mutate func(cmd *paymentsapp.CreatePaymentCommand)
	}{
		{"bad type enum", func(c *paymentsapp.CreatePaymentCommand) { c.Type = badEnumType }},
		{"bad payment form enum", func(c *paymentsapp.CreatePaymentCommand) { c.PaymentForm = badForm }},
		{"empty title", func(c *paymentsapp.CreatePaymentCommand) { c.Title = "   " }},
		{"title over 255 characters", func(c *paymentsapp.CreatePaymentCommand) { c.Title = strings.Repeat("а", 256) }},
		{"zero amount", func(c *paymentsapp.CreatePaymentCommand) { c.AmountKopecks = 0 }},
		{"amount over 10^9", func(c *paymentsapp.CreatePaymentCommand) { c.AmountKopecks = 1_000_000_001 }},
		{"negative amount", func(c *paymentsapp.CreatePaymentCommand) { c.AmountKopecks = -5 }},
		{"unknown category slug", func(c *paymentsapp.CreatePaymentCommand) { c.CategorySlug = unknownSlug }},
		{"zero recurrence", func(c *paymentsapp.CreatePaymentCommand) { c.Recurrence = domain.Recurrence{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hh := newServiceHarness(t).withOwner()
			cmd := hh.createCmd()
			tc.mutate(&cmd)
			if _, err := hh.svc.CreatePayment(hh.ctx(), hh.owner, hh.propID, cmd); !errors.Is(err, paymentsapp.ErrInvalidInput) {
				t.Fatalf("create = %v, want ErrInvalidInput", err)
			}
		})
	}

	// The drift regression: 130 Cyrillic characters are 260 bytes but a
	// valid title — the limit counts runes.
	long := newServiceHarness(t).withOwner()
	cmd := long.createCmd()
	cmd.Title = strings.Repeat("а", 130)
	if _, err := long.svc.CreatePayment(long.ctx(), long.owner, long.propID, cmd); err != nil {
		t.Fatalf("create with 130-character Cyrillic title: %v — the limit counts characters, not bytes", err)
	}

	// The update path validates through the same rule module after the diff
	// is applied.
	upd := newServiceHarness(t).withOwner()
	created, err := upd.svc.CreatePayment(upd.ctx(), upd.owner, upd.propID, upd.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	emptyTitle := " "
	if _, err := upd.svc.UpdatePayment(upd.ctx(), upd.owner, upd.propID, created.ID, paymentsapp.UpdatePaymentCommand{
		Title: &emptyTitle,
	}); !errors.Is(err, paymentsapp.ErrInvalidInput) {
		t.Fatalf("update with blank title = %v, want ErrInvalidInput", err)
	}
}

func TestPauseResume_Lifecycle(t *testing.T) {
	t.Parallel()
	h := newServiceHarness(t).withOwner()
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}

	paused, err := h.svc.PausePayment(h.ctx(), h.owner, h.propID, created.ID)
	if err != nil {
		t.Fatalf("pause payment: %v", err)
	}
	active, ok := domain.ActivePause(paused.Pauses)
	if !ok || active.From.Format(time.DateOnly) != day25 || active.To != nil {
		t.Fatalf("active pause = %+v, want open-ended from %s", active, day25)
	}
	// The pause removes the future planned; today's occurrence stays as debt.
	ops := opStatuses(h.opsOf(t, created.ID))
	if ops[day25] != opPlanned {
		t.Fatalf("today = %q, want planned (the pre-pause occurrence stays)", ops[day25])
	}
	if _, future := ops[day26]; future {
		t.Fatalf("tomorrow = planned, want absent — the pause removes the future planned")
	}

	if _, err := h.svc.PausePayment(h.ctx(), h.owner, h.propID, created.ID); !errors.Is(err, paymentsapp.ErrAlreadyPaused) {
		t.Fatalf("second pause = %v, want ErrAlreadyPaused", err)
	}

	if actions := h.auditActions(t, created.ID); len(actions) != 2 || actions[1] != "payment.paused" {
		t.Fatalf("audit actions = %v, want [payment.created payment.paused]", actions)
	}
}

func TestResume_ClosesPauseAndStandsFuture(t *testing.T) {
	t.Parallel()
	h := newServiceHarness(t).withOwner()
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if _, err := h.svc.PausePayment(h.ctx(), h.owner, h.propID, created.ID); err != nil {
		t.Fatalf("pause payment: %v", err)
	}

	// Days pass inside the pause: nothing accrues.
	h.clock.now = tickBaseTime.AddDate(0, 0, 3)
	resumed, err := h.svc.ResumePayment(h.ctx(), h.owner, h.propID, created.ID)
	if err != nil {
		t.Fatalf("resume payment: %v", err)
	}
	if _, active := domain.ActivePause(resumed.Pauses); active {
		t.Fatalf("resume left an open pause: %+v", resumed.Pauses)
	}
	if len(resumed.Pauses) != 1 || resumed.Pauses[0].To == nil || resumed.Pauses[0].To.Format(time.DateOnly) != "2026-08-28" {
		t.Fatalf("closed pause = %+v, want [2026-08-25, 2026-08-28)", resumed.Pauses)
	}
	// The resume day (the 28th in Moscow) is due and the future planned
	// stands on the 29th.
	ops := opStatuses(h.opsOf(t, created.ID))
	if ops["2026-08-28"] != opPlanned || ops["2026-08-29"] != opPlanned {
		t.Fatalf("after resume: 28th=%q 29th=%q, want planned/planned", ops["2026-08-28"], ops["2026-08-29"])
	}

	if _, err := h.svc.ResumePayment(h.ctx(), h.owner, h.propID, created.ID); !errors.Is(err, paymentsapp.ErrNotPaused) {
		t.Fatalf("resume without pause = %v, want ErrNotPaused", err)
	}
}

func TestDeletePayment_KeepOverdueMatrix(t *testing.T) {
	t.Parallel()
	// One payment accumulates overdue debt (the clock moves two days ahead
	// after creation), then is deleted with each keep_overdue value.
	h := newServiceHarness(t).withOwner()
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}

	// Pay today's occurrence directly: paid facts must survive every delete.
	if _, err := h.pool.Exec(h.ctx(), `
		UPDATE operations SET status = 'paid', paid_date = $2
		WHERE payment_id = $1 AND date = $2`, created.ID, day25,
	); err != nil {
		t.Fatalf("pay today: %v", err)
	}
	h.clock.now = tickBaseTime.AddDate(0, 0, 2)
	// Materialize the debt of the missed days through the standalone tick
	// driver (the worker's path).
	tick := h.standaloneTick()
	if err := tick.RunOwnerTick(h.ctx(), h.owner); err != nil {
		t.Fatalf("run tick: %v", err)
	}

	if err := h.svc.DeletePayment(h.ctx(), h.owner, h.propID, created.ID, true); err != nil {
		t.Fatalf("delete keep_overdue=true: %v", err)
	}
	// The debt survives the rule: planned operations with payment_id set to
	// NULL (the «платёж удалён» mark), the paid fact kept.
	var keptPlanned, keptPaid int
	if err := h.pool.QueryRow(h.ctx(), `
		SELECT count(*) FILTER (WHERE status = 'planned'),
		       count(*) FILTER (WHERE status = 'paid')
		FROM operations
		WHERE origin = 'payment' AND payment_id IS NULL AND title = 'Аренда'`,
	).Scan(&keptPlanned, &keptPaid); err != nil {
		t.Fatalf("count orphan operations: %v", err)
	}
	if keptPlanned == 0 {
		t.Fatalf("keep_overdue=true removed the overdue debt")
	}
	if keptPaid != 1 {
		t.Fatalf("paid facts after keep delete = %d, want 1", keptPaid)
	}

	// The second variant: the debt goes with the rule.
	h2 := newServiceHarness(t).withOwner()
	created2, err := h2.svc.CreatePayment(h2.ctx(), h2.owner, h2.propID, h2.createCmd())
	if err != nil {
		t.Fatalf("create payment 2: %v", err)
	}
	h2.clock.now = tickBaseTime.AddDate(0, 0, 2)
	if err := h2.standaloneTick().RunOwnerTick(h2.ctx(), h2.owner); err != nil {
		t.Fatalf("run tick 2: %v", err)
	}
	if err := h2.svc.DeletePayment(h2.ctx(), h2.owner, h2.propID, created2.ID, false); err != nil {
		t.Fatalf("delete keep_overdue=false: %v", err)
	}
	var leftPlanned int
	if err := h2.pool.QueryRow(h2.ctx(), `
		SELECT count(*) FROM operations
		WHERE origin = 'payment' AND payment_id IS NULL AND title = 'Аренда' AND status = 'planned'`,
	).Scan(&leftPlanned); err != nil {
		t.Fatalf("count orphan planned: %v", err)
	}
	if leftPlanned != 0 {
		t.Fatalf("keep_overdue=false left %d planned debt rows", leftPlanned)
	}
}

func TestPaymentRoleMatrix(t *testing.T) {
	t.Parallel()
	viewer := newServiceHarnessWithPolicy(t, stubPropertyPolicy{role: sharedpolicy.RoleViewer}).withOwner()
	member := uuid.Must(uuid.NewV7())

	if _, err := viewer.svc.ListPayments(viewer.ctx(), member, viewer.propID); err != nil {
		t.Fatalf("viewer list: %v", err)
	}
	if _, err := viewer.svc.CreatePayment(viewer.ctx(), member, viewer.propID, viewer.createCmd()); !errors.Is(err, paymentsapp.ErrForbidden) {
		t.Fatalf("viewer create = %v, want ErrForbidden", err)
	}

	full := newServiceHarnessWithPolicy(t, stubPropertyPolicy{role: sharedpolicy.RoleFullAccess}).withOwner()
	full.seedActor(member)
	created, err := full.svc.CreatePayment(full.ctx(), member, full.propID, full.createCmd())
	if err != nil {
		t.Fatalf("full access create: %v", err)
	}
	// The member writes into the property owner's scope.
	if created.OwnerID != full.owner {
		t.Fatalf("created owner_id = %s, want the property owner's scope", created.OwnerID)
	}
	if err := full.svc.DeletePayment(full.ctx(), member, full.propID, created.ID, true); !errors.Is(err, paymentsapp.ErrForbidden) {
		t.Fatalf("full access delete = %v, want ErrForbidden — deletion is the owner's alone", err)
	}
	// The member's action is attributed to the full_access actor, not masked
	// as the owner's.
	var actorRole string
	if err := full.pool.QueryRow(full.ctx(),
		`SELECT actor_role FROM audit_log WHERE entity_type = 'payment' AND entity_id = $1`, created.ID,
	).Scan(&actorRole); err != nil {
		t.Fatalf("read audit actor role: %v", err)
	}
	if actorRole != "full_access" {
		t.Fatalf("audit actor_role = %q, want full_access", actorRole)
	}

	none := newServiceHarnessWithPolicy(t, stubPropertyPolicy{role: sharedpolicy.RoleNone}).withOwner()
	stranger := uuid.Must(uuid.NewV7())
	if _, err := none.svc.GetPayment(none.ctx(), stranger, none.propID, created.ID); !errors.Is(err, paymentsapp.ErrNotFound) {
		t.Fatalf("stranger get = %v, want the privacy-preserving ErrNotFound", err)
	}

	// The owner (the policy's RoleOwner) may delete.
	ownerH := newServiceHarnessWithPolicy(t, stubPropertyPolicy{role: sharedpolicy.RoleOwner}).withOwner()
	owned, err := ownerH.svc.CreatePayment(ownerH.ctx(), ownerH.owner, ownerH.propID, ownerH.createCmd())
	if err != nil {
		t.Fatalf("owner create: %v", err)
	}
	if err := ownerH.svc.DeletePayment(ownerH.ctx(), ownerH.owner, ownerH.propID, owned.ID, true); err != nil {
		t.Fatalf("owner delete: %v", err)
	}
}

func TestArchivedProperty_IsFinancialReadOnly(t *testing.T) {
	t.Parallel()
	h := newServiceHarness(t).withOwner()
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}

	if _, err := h.pool.Exec(h.ctx(), `UPDATE properties SET status = 'archived' WHERE id = $1`, h.propID); err != nil {
		t.Fatalf("archive property: %v", err)
	}

	if _, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd()); !errors.Is(err, paymentsapp.ErrArchivedProperty) {
		t.Fatalf("create on archived = %v, want ErrArchivedProperty", err)
	}
	emptyCmd := paymentsapp.UpdatePaymentCommand{}
	if _, err := h.svc.UpdatePayment(h.ctx(), h.owner, h.propID, created.ID, emptyCmd); !errors.Is(err, paymentsapp.ErrArchivedProperty) {
		t.Fatalf("update on archived = %v, want ErrArchivedProperty", err)
	}
	if _, err := h.svc.PausePayment(h.ctx(), h.owner, h.propID, created.ID); !errors.Is(err, paymentsapp.ErrArchivedProperty) {
		t.Fatalf("pause on archived = %v, want ErrArchivedProperty", err)
	}
	if err := h.svc.DeletePayment(h.ctx(), h.owner, h.propID, created.ID, true); !errors.Is(err, paymentsapp.ErrArchivedProperty) {
		t.Fatalf("delete on archived = %v, want ErrArchivedProperty", err)
	}

	// Reads stay open on the archived object.
	if _, err := h.svc.GetPayment(h.ctx(), h.owner, h.propID, created.ID); err != nil {
		t.Fatalf("get on archived: %v", err)
	}
	if _, err := h.svc.ListPayments(h.ctx(), h.owner, h.propID); err != nil {
		t.Fatalf("list on archived: %v", err)
	}
}

func TestUpdatePayment_RebuildsFuturePlannedInTx(t *testing.T) {
	t.Parallel()
	// The future planned is rebuilt with the NEW snapshot; the already
	// materialized occurrences keep theirs (ADR 0049 §1: snapshots are frozen
	// at materialization).
	h := newServiceHarness(t).withOwner()
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}

	newAmount := int64(7777777)
	if _, err := h.svc.UpdatePayment(h.ctx(), h.owner, h.propID, created.ID, paymentsapp.UpdatePaymentCommand{
		AmountKopecks: &newAmount,
	}); err != nil {
		t.Fatalf("update payment: %v", err)
	}

	var todayAmount, tomorrowAmount int64
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT amount_kopecks FROM operations WHERE payment_id = $1 AND date = $2`, created.ID, day25,
	).Scan(&todayAmount); err != nil {
		t.Fatalf("read today's amount: %v", err)
	}
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT amount_kopecks FROM operations WHERE payment_id = $1 AND date = $2`, created.ID, day26,
	).Scan(&tomorrowAmount); err != nil {
		t.Fatalf("read tomorrow's amount: %v", err)
	}
	if todayAmount != 5000000 {
		t.Fatalf("today's snapshot = %d, want the original amount", todayAmount)
	}
	if tomorrowAmount != newAmount {
		t.Fatalf("rebuilt future snapshot = %d, want %d", tomorrowAmount, newAmount)
	}
}
