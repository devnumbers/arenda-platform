//go:build integration

package application_test

// The integration family of the rentals use cases over real PostgreSQL
// (testcontainers): the rentals store, the composite factory and the
// conveyor discipline against real SQL — the scope filtering, the list
// ordering, the completion record, the invariant №12 with its partial unique
// backstop. The payments seam runs with the real gateway in the wire-level
// family (cmd/api/wire); here the gateway is a func-backed double so the
// pipeline discipline stays observable.

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	auditpg "github.com/nambers/arenda-planform/apps/backend/internal/audit/adapters/postgres"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	paymentspg "github.com/nambers/arenda-planform/apps/backend/internal/payments/adapters/postgres"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	pgdb "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	rentalspg "github.com/nambers/arenda-planform/apps/backend/internal/rentals/adapters/postgres"
	rentalsapp "github.com/nambers/arenda-planform/apps/backend/internal/rentals/application"
	rentalsdomain "github.com/nambers/arenda-planform/apps/backend/internal/rentals/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// today anchors the fake clock: the owner's fixed calendar date.
var intToday = mustIntDate("2026-09-04")

func mustIntDate(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

type intClock struct{}

func (intClock) Today(context.Context, uuid.UUID) (time.Time, error) { return intToday, nil }

type intPolicy struct{ role sharedpolicy.Role }

func (p intPolicy) Role(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error) {
	return sharedpolicy.RoleNone, nil
}

func (p intPolicy) RoleForProperty(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error) {
	return p.role, nil
}

// fakeSeamGateway is the RentPaymentGateway double for the pipeline tests.
// It inserts a minimal real payments row on Create — the rentals FK is real,
// so even the double must keep the pair consistent — and drops it on Delete.
type fakeSeamGateway struct {
	pool      *pgxpool.Pool
	created   []rentalsapp.RentPaymentSeed
	updates   []uuid.UUID
	stopped   []uuid.UUID
	deleted   []uuid.UUID
	ticked    int
	nextID    uuid.UUID
	day       rentalsdomain.PaymentDay
	paidCount int
}

func (g *fakeSeamGateway) WithTx(tx transaction.Tx) (rentalsapp.RentPaymentGatewayTx, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("fakeSeamGateway: %T is not a postgres.DBTX", tx)
	}
	return &fakeSeamTx{gateway: g, dbtx: dbtx}, nil
}

func (g *fakeSeamGateway) RentPaymentState(
	context.Context, uuid.UUID, uuid.UUID, uuid.UUID,
) (rentalsapp.RentPaymentState, error) {
	return rentalsapp.RentPaymentState{
		AmountKopecks: 5_000_000,
		PaymentDay:    g.day,
		AutoPay:       true,
	}, nil
}

func (g *fakeSeamGateway) NextPlannedOccurrence(
	context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time,
) (*rentalsapp.PlannedOccurrence, error) {
	return nil, nil // Будущего вхождения нет — это nil, не ошибка (ADR 0053 §2; исключение nilnil — в .golangci.yml).
}

func (g *fakeSeamGateway) CountPaidOperations(
	context.Context, uuid.UUID, uuid.UUID, uuid.UUID,
) (int, error) {
	return g.paidCount, nil
}

func (g *fakeSeamGateway) SummarizePaidOperations(
	context.Context, uuid.UUID, uuid.UUID, time.Time, time.Time, time.Time,
) (rentalsapp.PaymentsTotals, error) {
	return rentalsapp.PaymentsTotals{IncomeKopecks: 15_000_000, ExpenseKopecks: 5_000_000}, nil
}

// fakeSeamTx is the transaction-bound half; the seeding runs on the
// caller's connection — a pool side-execution would self-deadlock on the
// property lock's FK check.
type fakeSeamTx struct {
	gateway *fakeSeamGateway
	dbtx    postgres.DBTX
}

func (g *fakeSeamTx) Create(ctx context.Context, seed rentalsapp.RentPaymentSeed) (uuid.UUID, error) {
	g.gateway.created = append(g.gateway.created, seed)
	g.gateway.nextID = uuid.Must(uuid.NewV7())
	var recurrence string
	if seed.PaymentDay.IsLast() {
		recurrence = `{"kind":"monthly","lastDay":true}`
	} else {
		recurrence = fmt.Sprintf(`{"kind":"monthly","daysOfMonth":[%d]}`, seed.PaymentDay.Day())
	}
	if _, err := g.dbtx.Exec(ctx,
		`INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks,
		                      recurrence, since, end_date, auto_pay, payment_form, category_slug)
		 VALUES ($1, $2, $3, 'income', 'Арендная плата', $4, $5::jsonb, $6, $7, $8, 'transfer', 'rent')`,
		g.gateway.nextID, seed.OwnerID, seed.PropertyID, seed.AmountKopecks, recurrence,
		seed.StartDate, seed.PlannedEndDate, seed.AutoPay,
	); err != nil {
		return uuid.Nil, fmt.Errorf("seed rent payment: %w", err)
	}
	return g.gateway.nextID, nil
}

func (g *fakeSeamTx) Update(
	_ context.Context, _, _, paymentID uuid.UUID, _ rentalsapp.RentPaymentChange, _ time.Time,
) error {
	g.gateway.updates = append(g.gateway.updates, paymentID)
	return nil
}

func (g *fakeSeamTx) Stop(
	_ context.Context, _, _, paymentID uuid.UUID, _ time.Time,
) error {
	g.gateway.stopped = append(g.gateway.stopped, paymentID)
	return nil
}

func (g *fakeSeamTx) Delete(ctx context.Context, _, paymentID uuid.UUID, _ time.Time) error {
	g.gateway.deleted = append(g.gateway.deleted, paymentID)
	if _, err := g.dbtx.Exec(ctx, `DELETE FROM payments WHERE id = $1`, paymentID); err != nil {
		return fmt.Errorf("seed-delete rent payment: %w", err)
	}
	return nil
}

func (g *fakeSeamTx) RunTick(context.Context, uuid.UUID, time.Time) error {
	g.gateway.ticked++
	return nil
}

// rentalsHarness wires the whole rentals context to real PostgreSQL with the
// func-backed gateway seam, a fixed owner calendar and an injectable policy.
type rentalsHarness struct {
	t       *testing.T
	pool    *pgxpool.Pool
	gateway *fakeSeamGateway
	svc     *rentalsapp.RentalService
	owner   uuid.UUID
	propID  uuid.UUID
}

func newRentalsHarness(t *testing.T) *rentalsHarness {
	t.Helper()
	pool := testdb.Setup(t)
	gateway := &fakeSeamGateway{pool: pool, day: rentalsdomain.MustPaymentDay(15)}
	audit := auditapp.NewService(auditpg.NewWriter(pool), nil)
	factory := rentalsapp.NewTxStoreFactory(
		rentalspg.NewRentalStore(pool),
		paymentspg.NewPropertyStore(pool),
		gateway,
		nil,
		audit,
		pgdb.NewUoW(pool, slog.New(slog.DiscardHandler)),
	)
	return &rentalsHarness{
		t:       t,
		pool:    pool,
		gateway: gateway,
		svc:     rentalsapp.NewRentalService(factory, intClock{}, intPolicy{role: sharedpolicy.RoleOwner}),
	}
}

func (h *rentalsHarness) seedOwner() {
	t := h.t
	t.Helper()
	owner, err := uuid.NewV7()
	require.NoError(t, err)
	phone := fmt.Sprintf("+7999%010d", time.Now().UnixNano()%10000000000)
	_, err = h.pool.Exec(context.Background(),
		`INSERT INTO users (id, phone, role, timezone) VALUES ($1, $2, $3, 'Europe/Moscow')`,
		owner, phone, actor.RoleOwner)
	require.NoError(t, err)
	propID, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = h.pool.Exec(context.Background(),
		`INSERT INTO properties (id, owner_id, name, type, address, status)
		 VALUES ($1, $2, 'Квартира', 'apartment', 'Москва, Тверская 1', 'active')`,
		propID, owner)
	require.NoError(t, err)
	h.owner = owner
	h.propID = propID
}

func (h *rentalsHarness) createCmd() rentalsapp.CreateRentalCommand {
	return rentalsapp.CreateRentalCommand{
		AmountKopecks:  5_000_000,
		PaymentDay:     rentalsdomain.MustPaymentDay(15),
		StartDate:      intToday,
		PlannedEndDate: func() *time.Time { d := mustIntDate("2027-09-01"); return &d }(),
		Utilities:      rentalsdomain.UtilitiesIncluded,
		AutoPay:        true,
	}
}

func (h *rentalsHarness) createRental() rentalsapp.RentalView {
	t := h.t
	t.Helper()
	view, err := h.svc.CreateRental(context.Background(), h.owner, h.propID, h.createCmd())
	require.NoError(t, err)
	return view
}

func TestRentalsIntegration_CreatePersistsAndAudits(t *testing.T) {
	t.Parallel()
	h := newRentalsHarness(t)
	h.seedOwner()

	view := h.createRental()

	// The rentals row persists with the seed terms.
	assert.Equal(t, rentalsdomain.StatusActive, view.Status)
	assert.Equal(t, intToday, view.Rental.StartDate)
	require.NotNil(t, view.Rental.PlannedEndDate)
	assert.Equal(t, int64(5_000_000), view.Payment.AmountKopecks)
	assert.Equal(t, 12, *view.Progress.TotalMonths)

	// The audit trail recorded the creation under the rental entity.
	actions := h.rentalActions(view.Rental.ID)
	assert.Equal(t, []string{"rental.created"}, actions)

	// The second unfinished rental on the property is the honest 409.
	_, err := h.svc.CreateRental(context.Background(), h.owner, h.propID, h.createCmd())
	assert.ErrorIs(t, err, rentalsapp.ErrPropertyOccupied)
}

// TestRentalsIntegration_ReminderOffsetFlowsToSeed (карта #822, #824): the
// reminder rides the create command into the managed payment's seed, and a
// bad lead time is the shared invalid-input outcome.
func TestRentalsIntegration_ReminderOffsetFlowsToSeed(t *testing.T) {
	t.Parallel()
	h := newRentalsHarness(t)
	h.seedOwner()

	cmd := h.createCmd()
	cmd.AutoPay = true
	reminder := 1
	cmd.ReminderOffsetDays = &reminder
	view, err := h.svc.CreateRental(context.Background(), h.owner, h.propID, cmd)
	require.NoError(t, err)

	require.Len(t, h.gateway.created, 1)
	seed := h.gateway.created[0]
	require.NotNil(t, seed.ReminderOffsetDays)
	assert.Equal(t, 1, *seed.ReminderOffsetDays)

	// The fake seam answers no reminder — the view renders null.
	assert.Nil(t, view.Payment.ReminderOffsetDays)

	bad := 5
	_, err = h.svc.CreateRental(context.Background(), h.owner, h.propID, func() rentalsapp.CreateRentalCommand {
		c := h.createCmd()
		c.ReminderOffsetDays = &bad
		return c
	}())
	assert.ErrorIs(t, err, rentalsapp.ErrInvalidInput)
}

func TestRentalsIntegration_ListOrdering(t *testing.T) {
	t.Parallel()
	h := newRentalsHarness(t)
	h.seedOwner()

	// The current (unfinished) rental.
	current := h.createRental()

	// A completed one that ended earlier — completed first, then completed later.
	completedAt := mustIntDate("2026-06-01")
	first := h.seedCompletedRental(mustIntDate("2026-01-01"), completedAt)
	second := h.seedCompletedRental(mustIntDate("2026-03-01"), mustIntDate("2026-08-01"))

	views, err := h.svc.ListRentals(context.Background(), h.owner, h.propID)
	require.NoError(t, err)
	require.Len(t, views, 3)

	ids := []uuid.UUID{views[0].Rental.ID, views[1].Rental.ID, views[2].Rental.ID}
	assert.Equal(t, []uuid.UUID{current.Rental.ID, second, first}, ids,
		"unfinished first, then completed by completion date, fresh on top")
}

func TestRentalsIntegration_UpdateSyncsTerms(t *testing.T) {
	t.Parallel()
	h := newRentalsHarness(t)
	h.seedOwner()
	view := h.createRental()

	comment := "новые условия"
	newEnd := mustIntDate("2027-12-01")
	updated, err := h.svc.UpdateRental(context.Background(), h.owner, h.propID, view.Rental.ID,
		rentalsapp.UpdateRentalCommand{
			Comment:        &rentalsapp.StringUpdate{Value: &comment},
			PlannedEndDate: &rentalsapp.DateUpdate{Value: &newEnd},
		})
	require.NoError(t, err)
	assert.Equal(t, comment, updated.Rental.Comment)
	require.NotNil(t, updated.Rental.PlannedEndDate)
	assert.Equal(t, newEnd, *updated.Rental.PlannedEndDate)

	// The cleared comment travels as the stored NULL.
	cleared, err := h.svc.UpdateRental(context.Background(), h.owner, h.propID, view.Rental.ID,
		rentalsapp.UpdateRentalCommand{Comment: &rentalsapp.StringUpdate{}})
	require.NoError(t, err)
	assert.Empty(t, cleared.Rental.Comment)

	assert.Equal(t, []string{"rental.created", "rental.updated", "rental.updated"},
		h.rentalActions(view.Rental.ID))
}

func TestRentalsIntegration_CompleteLifecycle(t *testing.T) {
	t.Parallel()
	h := newRentalsHarness(t)
	h.seedOwner()
	view := h.createRental()
	deposit := int64(10_000_000)

	completedAt := intToday
	comment := "вернул частично"
	completed, err := h.svc.CompleteRental(context.Background(), h.owner, h.propID, view.Rental.ID,
		rentalsapp.CompleteRentalCommand{
			CompletedDate: completedAt,
			DepositReturn: &rentalsapp.DepositReturn{AmountKopecks: deposit, Comment: &comment},
		})
	require.NoError(t, err)
	require.NotNil(t, completed.Rental.CompletedDate)
	assert.Equal(t, completedAt, *completed.Rental.CompletedDate)
	require.NotNil(t, completed.Rental.DepositReturnKopecks)
	assert.Equal(t, deposit, *completed.Rental.DepositReturnKopecks)
	assert.Equal(t, rentalsdomain.StatusCompleted, completed.Status)

	// The completion ends the unfinished-only invariant: a new rental fits.
	next := h.createRental()
	assert.NotEqual(t, completed.Rental.ID, next.Rental.ID)

	// Mutating the completed rental stays the honest 409.
	_, err = h.svc.UpdateRental(context.Background(), h.owner, h.propID, completed.Rental.ID,
		rentalsapp.UpdateRentalCommand{})
	require.ErrorIs(t, err, rentalsapp.ErrRentalCompleted)

	// The completed rental deletes together with its history entry.
	require.NoError(t, h.svc.DeleteRental(context.Background(), h.owner, h.propID, completed.Rental.ID))
	_, err = h.svc.GetRental(context.Background(), h.owner, h.propID, completed.Rental.ID)
	assert.ErrorIs(t, err, rentalsapp.ErrNotFound)
}

func TestRentalsIntegration_ScopesHideForeignRentals(t *testing.T) {
	t.Parallel()
	h := newRentalsHarness(t)
	h.seedOwner()
	view := h.createRental()

	// A foreign property and a foreign rental id both read as the privacy
	// 404 — the unknown property surfaces from the read-scope property
	// lookup, the unknown rental from the rentals store.
	_, err := h.svc.GetRental(context.Background(), h.owner, uuid.Must(uuid.NewV7()), view.Rental.ID)
	require.ErrorIs(t, err, paymentsapp.ErrNotFound)

	_, err = h.svc.GetRental(context.Background(), h.owner, h.propID, uuid.Must(uuid.NewV7()))
	require.ErrorIs(t, err, rentalsapp.ErrNotFound)
}

func TestRentalsIntegration_SummaryDefaults(t *testing.T) {
	t.Parallel()
	h := newRentalsHarness(t)
	h.seedOwner()
	view := h.createRental()

	summary, err := h.svc.RentalSummary(context.Background(), h.owner, h.propID, view.Rental.ID, nil)
	require.NoError(t, err)
	assert.Equal(t, intToday, summary.Until, "a running rental sums until today")
	assert.Equal(t, int64(10_000_000), summary.ProfitKopecks)
}

// seedCompletedRental inserts a completed rental pair directly (the payment
// link is a throwaway id — the gateway is a double in this family).
func (h *rentalsHarness) seedCompletedRental(start, completedAt time.Time) uuid.UUID {
	t := h.t
	t.Helper()
	id, err := uuid.NewV7()
	require.NoError(t, err)
	paymentID, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = h.pool.Exec(context.Background(),
		`INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks,
		                      recurrence, since, end_date, auto_pay, payment_form, category_slug)
		 VALUES ($1, $2, $3, 'income', 'Арендная плата', 5000000,
		         '{"kind":"monthly","daysOfMonth":[15]}'::jsonb, $4, $5, false, 'transfer', 'rent')`,
		paymentID, h.owner, h.propID, start, mustIntDate("2027-01-01"))
	require.NoError(t, err)
	_, err = h.pool.Exec(context.Background(),
		`INSERT INTO rentals (id, owner_id, property_id, payment_id, start_date,
		                      planned_end_date, completed_date, utilities)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, 'included')`,
		id, h.owner, h.propID, paymentID, start, mustIntDate("2027-01-01"), completedAt)
	require.NoError(t, err)
	return id
}

// rentalActions loads the rental's audit trail actions in recording order.
func (h *rentalsHarness) rentalActions(entityID uuid.UUID) []string {
	t := h.t
	t.Helper()
	rows, err := h.pool.Query(context.Background(),
		`SELECT action FROM audit_log WHERE entity_type = $1 AND entity_id = $2 ORDER BY created_at, id`,
		auditdomain.EntityRental, entityID)
	require.NoError(t, err)
	defer rows.Close()
	actions := []string{}
	for rows.Next() {
		var action string
		require.NoError(t, rows.Scan(&action))
		actions = append(actions, action)
	}
	require.NoError(t, rows.Err())
	return actions
}
