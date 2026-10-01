//go:build integration

package wire

// The integration family of the rentals↔payments seam (ADR 0053 §3): the
// wiring-bound RentPaymentGateway over the real payments stores, driven by
// the real rental use cases against real PostgreSQL. The managed payment
// behaves exactly like an ordinary payment — the rule shape, the tick
// materialization, the edit invalidation, the completion teardown, the
// delete pair — with no second property lock and no second conveyor.

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
	historypg "github.com/nambers/arenda-planform/apps/backend/internal/history/adapters/postgres"
	paymentspg "github.com/nambers/arenda-planform/apps/backend/internal/payments/adapters/postgres"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	paymentsdomain "github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	pgdb "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
	rentalspg "github.com/nambers/arenda-planform/apps/backend/internal/rentals/adapters/postgres"
	rentalsapp "github.com/nambers/arenda-planform/apps/backend/internal/rentals/application"
	rentalsdomain "github.com/nambers/arenda-planform/apps/backend/internal/rentals/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seamToday is the owner's fixed calendar date of the seam family.
var seamToday = mustSeamDate("2026-09-04")

func mustSeamDate(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

type seamClock struct{}

func (seamClock) Now() time.Time {
	// Noon UTC is the same calendar date in every Russian timezone.
	return time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
}

// seamHarness mirrors WireRentals over the test pool with a fixed clock.
type seamHarness struct {
	t      *testing.T
	pool   *pgxpool.Pool
	svc    *rentalsapp.RentalService
	paySvc *paymentsapp.PaymentService
	ops    *paymentsapp.OperationService
	tick   *paymentsapp.TickService
	owner  uuid.UUID
	propID uuid.UUID
}

func newSeamHarness(t *testing.T) *seamHarness {
	t.Helper()
	pool := testdb.Setup(t)
	logger := slog.New(slog.DiscardHandler)
	audit := auditapp.NewService(auditpg.NewWriter(pool), nil)
	calendar := paymentspg.NewOwnerCalendar(pool, seamClock{})
	gateway := NewRentPaymentGateway(
		paymentspg.NewPaymentStore(pool),
		paymentspg.NewTickStore(pool),
		paymentspg.NewOperationStore(pool),
	)
	factory := rentalsapp.NewTxStoreFactory(
		rentalspg.NewRentalStore(pool),
		paymentspg.NewPropertyStore(pool),
		gateway,
		nil,
		audit,
		historypg.NewRecorder(pool),
		pgdb.NewUoW(pool, logger),
	)
	// The real payments factory backs the tick re-run assertions.
	payFactory := paymentsapp.NewTxStoreFactory(
		paymentspg.NewTickStore(pool),
		paymentspg.NewPaymentStore(pool),
		paymentspg.NewOperationStore(pool),
		paymentspg.NewPropertyStore(pool),
		paymentspg.NewGlobalPaymentStore(pool),
		rentalspg.NewRentalLinkReader(pool),
		audit,
		historypg.NewRecorder(pool),
		pgdb.NewUoW(pool, logger),
	)
	return &seamHarness{
		t:      t,
		pool:   pool,
		svc:    rentalsapp.NewRentalService(factory, calendar, nil),
		paySvc: paymentsapp.NewPaymentService(payFactory, calendar, nil),
		ops:    paymentsapp.NewOperationService(payFactory, calendar, nil),
		tick: paymentsapp.NewTickService(payFactory, paymentspg.NewTickZoneDirectory(pool),
			calendar, nil),
	}
}

func (h *seamHarness) seedOwner() {
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

// createRental starts tomorrow (upcoming, no materialized occurrences yet).
func (h *seamHarness) createRental() rentalsapp.RentalView {
	return h.createRentalWithReminderOffset(nil)
}

// createRentalWithReminderOffset is the same create with an explicit reminder
// lead time (nil = без напоминаний) for the tests that pin the seam's copy
// into the managed payment.
func (h *seamHarness) createRentalWithReminderOffset(offset *int) rentalsapp.RentalView {
	t := h.t
	t.Helper()
	view, err := h.svc.CreateRental(context.Background(), h.owner, h.propID,
		rentalsapp.CreateRentalCommand{
			AmountKopecks:      5_000_000,
			PaymentDay:         rentalsdomain.MustPaymentDay(15),
			StartDate:          seamToday,
			Utilities:          rentalsdomain.UtilitiesIncluded,
			AutoPay:            true,
			ReminderOffsetDays: offset,
		})
	require.NoError(t, err)
	return view
}

func TestSeam_CreateBuildsTheManagedPaymentAndMaterializes(t *testing.T) {
	t.Parallel()
	h := newSeamHarness(t)
	h.seedOwner()

	// The explicit reminder lead time rides the seed through the seam.
	offset := 1
	view := h.createRentalWithReminderOffset(&offset)

	// The managed payment is the ordinary payments rule: income, «Арендная
	// плата», the rent category, monthly on the payment day,
	// since = the rental start.
	payment, err := h.paySvc.GetPayment(context.Background(), h.owner, h.propID, view.Rental.PaymentID)
	require.NoError(t, err)
	require.Equal(t, paymentsdomain.TypeIncome, payment.Type)
	assert.Equal(t, "Арендная плата", payment.Title)
	assert.Equal(t, int64(5_000_000), payment.AmountKopecks)
	require.NotNil(t, payment.Category.Slug)
	assert.Equal(t, "rent", *payment.Category.Slug)
	assert.Equal(t, paymentsdomain.RecurrenceMonthly, payment.Recurrence.Kind())
	assert.Equal(t, []int{15}, payment.Recurrence.DaysOfMonth())
	assert.Equal(t, seamToday, payment.Since)
	assert.True(t, payment.AutoPay)
	assert.Equal(t, 1, *payment.ReminderOffsetDays)

	// The in-transaction tick stood the single future planned at the payment
	// day of the start month (start == today, day 15 ahead).
	next := view.NextPayment
	require.NotNil(t, next)
	assert.Equal(t, mustSeamDate("2026-09-15"), next.Date)
	assert.Equal(t, int64(5_000_000), next.AmountKopecks)
}

func TestSeam_UpdateSyncsThePaymentAndRestandsThePlanned(t *testing.T) {
	t.Parallel()
	h := newSeamHarness(t)
	h.seedOwner()
	view := h.createRental()

	// The terms edit moves the amount, the day and the planned end; the
	// tick stands the single future planned again with fresh snapshots.
	newDay := rentalsdomain.MustPaymentDay(25)
	newEnd := mustSeamDate("2027-06-01")
	updated, err := h.svc.UpdateRental(context.Background(), h.owner, h.propID, view.Rental.ID,
		rentalsapp.UpdateRentalCommand{
			AmountKopecks:  func() *int64 { v := int64(6_000_000); return &v }(),
			PaymentDay:     &newDay,
			PlannedEndDate: &rentalsapp.DateUpdate{Value: &newEnd},
		})
	require.NoError(t, err)

	payment, err := h.paySvc.GetPayment(context.Background(), h.owner, h.propID, view.Rental.PaymentID)
	require.NoError(t, err)
	assert.Equal(t, int64(6_000_000), payment.AmountKopecks)
	assert.Equal(t, []int{25}, payment.Recurrence.DaysOfMonth())
	require.NotNil(t, payment.EndDate)
	assert.Equal(t, newEnd, *payment.EndDate)

	// The response reads the day back from the payment (решение №5).
	assert.Equal(t, 25, updated.Payment.PaymentDay.Day())

	// The single future planned re-stood at the moved day.
	require.NotNil(t, updated.NextPayment)
	assert.Equal(t, mustSeamDate("2026-09-25"), updated.NextPayment.Date)
	assert.Equal(t, int64(6_000_000), updated.NextPayment.AmountKopecks,
		"the fresh snapshot carries the new amount")
}

func TestSeam_ClearingThePlannedEndMakesThePaymentOpenEnded(t *testing.T) {
	t.Parallel()
	h := newSeamHarness(t)
	h.seedOwner()
	view := h.createRental()

	updated, err := h.svc.UpdateRental(context.Background(), h.owner, h.propID, view.Rental.ID,
		rentalsapp.UpdateRentalCommand{PlannedEndDate: &rentalsapp.DateUpdate{}})
	require.NoError(t, err)

	payment, err := h.paySvc.GetPayment(context.Background(), h.owner, h.propID, view.Rental.PaymentID)
	require.NoError(t, err)
	assert.Nil(t, payment.EndDate)
	assert.Nil(t, updated.Progress.TotalMonths, "an open-ended rental has no total")
}

// backdateRentalToMay moves the started pair into the past — the direct SQL
// of a seeding scenario: the rental began in May. The caller runs the owner
// tick to materialize the due months.
func (h *seamHarness) backdateRentalToMay(ctx context.Context, view rentalsapp.RentalView) {
	t := h.t
	t.Helper()
	start := mustSeamDate("2026-05-04")
	_, err := h.pool.Exec(ctx,
		`UPDATE rentals SET start_date = $1 WHERE id = $2`, start, view.Rental.ID)
	require.NoError(t, err)
	_, err = h.pool.Exec(ctx,
		`UPDATE payments SET since = $1 WHERE id = $2`, start, view.Rental.PaymentID)
	require.NoError(t, err)
}

func TestSeam_CompleteStopsThePaymentAtTheCompletionDate(t *testing.T) {
	t.Parallel()
	h := newSeamHarness(t)
	h.seedOwner()
	view := h.createRental()

	// Move the started pair into the past (the rental began in May), then
	// run the owner tick so the due months materialize as the overdue debt.
	h.backdateRentalToMay(context.Background(), view)
	require.NoError(t, h.tick.RunOwnerTick(context.Background(), h.owner))

	completedAt := mustSeamDate("2026-08-20")
	completed, err := h.svc.CompleteRental(context.Background(), h.owner, h.propID,
		view.Rental.ID, rentalsapp.CompleteRentalCommand{CompletedDate: completedAt})
	require.NoError(t, err)

	// The payment ends at the completion date.
	payment, err := h.paySvc.GetPayment(context.Background(), h.owner, h.propID, view.Rental.PaymentID)
	require.NoError(t, err)
	require.NotNil(t, payment.EndDate)
	assert.Equal(t, completedAt, *payment.EndDate)

	// The strictly-after-completion planned is gone; the due occurrences
	// stay with the owner as debt.
	var future int
	require.NoError(t, h.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM operations WHERE payment_id = $1 AND status = 'planned' AND date > $2`,
		view.Rental.PaymentID, completedAt).Scan(&future))
	assert.Zero(t, future)
	var debt int
	require.NoError(t, h.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM operations WHERE payment_id = $1 AND status = 'planned' AND date <= $2`,
		view.Rental.PaymentID, completedAt).Scan(&debt))
	assert.Positive(t, debt, "the overdue months stay as debt")

	assert.Equal(t, rentalsdomain.StatusCompleted, completed.Status)
	assert.Nil(t, completed.NextPayment, "a completed rental has no future payment")
}

func TestSeam_CompletingRentalSettlesThePaymentCompleted(t *testing.T) {
	t.Parallel()
	h := newSeamHarness(t)
	h.seedOwner()
	view := h.createRental()
	paymentID := view.Rental.PaymentID
	ctx := context.Background()

	// Move the started pair into the past (the rental began in May), then
	// run the owner tick so the due months materialize as the overdue debt.
	h.backdateRentalToMay(ctx, view)
	require.NoError(t, h.tick.RunOwnerTick(ctx, h.owner))

	// Unpaid due months keep the settlement view unfinished.
	completed, err := h.paySvc.CompletedStatus(ctx, h.owner, h.propID, paymentID)
	require.NoError(t, err)
	assert.False(t, completed)

	// The owner pays every due month — the pay facts go past the gate
	// («Оплатить» is the canon of marking a rent month paid, #818).
	overdue := paymentsdomain.ViewStatusOverdue
	ops, err := h.ops.ListPaymentOperations(ctx, h.owner, h.propID, paymentID,
		paymentsapp.OperationsListQuery{Status: &overdue, Today: seamToday, Limit: 10})
	require.NoError(t, err)
	require.Len(t, ops, 4, "the May–August due months")
	for _, op := range ops {
		_, err = h.ops.PayOperation(ctx, h.owner, h.propID, op.Operation.ID)
		require.NoError(t, err)
	}

	// Everything up to the future stop is settled, but the rule still stands
	// a future planned — the computed status stays unfinished.
	completed, err = h.paySvc.CompletedStatus(ctx, h.owner, h.propID, paymentID)
	require.NoError(t, err)
	assert.False(t, completed, "the future planned keeps the rule unfinished")

	completedAt := mustSeamDate("2026-08-20")
	_, err = h.svc.CompleteRental(ctx, h.owner, h.propID, view.Rental.ID,
		rentalsapp.CompleteRentalCommand{CompletedDate: completedAt})
	require.NoError(t, err)

	// The stop tore the strictly future planned down and the past months
	// stayed paid — the settlement read flips to «Завершённый платёж» on its
	// own. No separate «complete the payment» action exists: the rental
	// pipeline already carries the semantics (payments/CONTEXT.md; the
	// owner's question the map #984 answers).
	completed, err = h.paySvc.CompletedStatus(ctx, h.owner, h.propID, paymentID)
	require.NoError(t, err)
	assert.True(t, completed)

	var paid int
	require.NoError(t, h.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM operations WHERE payment_id = $1 AND status = 'paid'`,
		paymentID).Scan(&paid))
	assert.Equal(t, 4, paid, "the paid months survive the stop")
}

func TestSeam_DeleteNotStartedRemovesThePair(t *testing.T) {
	t.Parallel()
	h := newSeamHarness(t)
	h.seedOwner()
	view := h.createRental()

	// Move the start into the future: передумал до старта.
	future := mustSeamDate("2026-09-10")
	_, err := h.pool.Exec(context.Background(),
		`UPDATE rentals SET start_date = $1 WHERE id = $2`, future, view.Rental.ID)
	require.NoError(t, err)

	// Not started: the pair goes cleanly — no operations at all.
	require.NoError(t, h.svc.DeleteRental(context.Background(), h.owner, h.propID, view.Rental.ID))

	_, err = h.svc.GetRental(context.Background(), h.owner, h.propID, view.Rental.ID)
	require.ErrorIs(t, err, rentalsapp.ErrNotFound)
	_, err = h.paySvc.GetPayment(context.Background(), h.owner, h.propID, view.Rental.PaymentID)
	require.ErrorIs(t, err, paymentsapp.ErrNotFound, "the managed payment goes with the rental")

	var ops int
	require.NoError(t, h.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM operations WHERE payment_id = $1`,
		view.Rental.PaymentID).Scan(&ops))
	assert.Zero(t, ops)
}

func TestSeam_SummaryReadsThePaidOperations(t *testing.T) {
	t.Parallel()
	h := newSeamHarness(t)
	h.seedOwner()
	view := h.createRental()

	// Move the started pair into the past so the summary period covers the
	// seeded months (until must not precede the start).
	start := mustSeamDate("2026-05-04")
	_, err := h.pool.Exec(context.Background(),
		`UPDATE rentals SET start_date = $1 WHERE id = $2`, start, view.Rental.ID)
	require.NoError(t, err)
	_, err = h.pool.Exec(context.Background(),
		`UPDATE payments SET since = $1 WHERE id = $2`, start, view.Rental.PaymentID)
	require.NoError(t, err)

	// Two paid months inside the window and one manual expense.
	for _, day := range []string{"2026-07-15", "2026-08-15"} {
		opID := uuid.Must(uuid.NewV7())
		_, err = h.pool.Exec(context.Background(),
			`INSERT INTO operations (id, owner_id, property_id, payment_id, origin, date,
			                         paid_date, status, type, title, amount_kopecks,
			                         category_label, category_slug)
			 VALUES ($1, $2, $3, $4, 'payment', $5, $5, 'paid', 'income', 'Арендная плата',
			         5000000, 'Арендная плата', 'rent')`,
			opID, h.owner, h.propID, view.Rental.PaymentID, mustSeamDate(day))
		require.NoError(t, err)
	}
	manualID := uuid.Must(uuid.NewV7())
	_, err = h.pool.Exec(context.Background(),
		`INSERT INTO operations (id, owner_id, property_id, origin, date, paid_date, status,
		                         type, title, amount_kopecks, category_label)
		 VALUES ($1, $2, $3, 'manual', $4, $4, 'paid', 'expense', 'Ремонт', 1000000, 'Ремонт')`,
		manualID, h.owner, h.propID, mustSeamDate("2026-08-01"))
	require.NoError(t, err)

	summary, err := h.svc.RentalSummary(context.Background(), h.owner, h.propID, view.Rental.ID,
		func() *time.Time { d := mustSeamDate("2026-09-01"); return &d }())
	require.NoError(t, err)
	assert.Equal(t, int64(10_000_000), summary.IncomeKopecks)
	assert.Equal(t, int64(1_000_000), summary.ExpenseKopecks)
	assert.Equal(t, int64(9_000_000), summary.ProfitKopecks)

	// The progress counts the payment's paid months — re-read after the
	// seeds.
	fresh, err := h.svc.GetRental(context.Background(), h.owner, h.propID, view.Rental.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, fresh.Progress.PaidMonths)
}

// Прогресс несёт серверный счётчик просрочки (#817): planned-вхождения с
// датой раньше «сегодня» собственника. Upcoming не имеет просрочек по
// построению — null; просроченные месяцы материализуются тиком как долг и
// считаются гейтвеем на реальном SQL.
func TestSeam_ProgressCountsTheOverdueOccurrences(t *testing.T) {
	t.Parallel()
	h := newSeamHarness(t)
	h.seedOwner()
	view := h.createRental()

	// Upcoming (старт в будущем): вхождения начинаются со старта — просрочек
	// не бывает, счётчик null.
	future := mustSeamDate("2026-09-10")
	_, err := h.pool.Exec(context.Background(),
		`UPDATE rentals SET start_date = $1 WHERE id = $2`, future, view.Rental.ID)
	require.NoError(t, err)
	upcoming, err := h.svc.GetRental(context.Background(), h.owner, h.propID, view.Rental.ID)
	require.NoError(t, err)
	assert.Equal(t, rentalsdomain.StatusUpcoming, upcoming.Status)
	assert.Nil(t, upcoming.Progress.OverdueMonths)

	// Backdate the pair into July (the seeding scenario, as completion) and
	// run the owner tick: Jul 15 and Aug 15 materialize as the overdue debt,
	// Sep 15 stands the single future planned — the count is 2.
	start := mustSeamDate("2026-07-15")
	_, err = h.pool.Exec(context.Background(),
		`UPDATE rentals SET start_date = $1 WHERE id = $2`, start, view.Rental.ID)
	require.NoError(t, err)
	_, err = h.pool.Exec(context.Background(),
		`UPDATE payments SET since = $1 WHERE id = $2`, start, view.Rental.PaymentID)
	require.NoError(t, err)
	require.NoError(t, h.tick.RunOwnerTick(context.Background(), h.owner))

	overdue, err := h.svc.GetRental(context.Background(), h.owner, h.propID, view.Rental.ID)
	require.NoError(t, err)
	require.NotNil(t, overdue.Progress.OverdueMonths)
	assert.Equal(t, 2, *overdue.Progress.OverdueMonths)
	require.NotNil(t, overdue.NextPayment,
		"the future planned stays — the blue line keeps its data")
	assert.Equal(t, mustSeamDate("2026-09-15"), overdue.NextPayment.Date)

	// A paid fact closes its month (решение #815): marking Aug 15 paid drops
	// the count to 1 — paid rows never read as overdue.
	_, err = h.pool.Exec(context.Background(),
		`UPDATE operations SET status = 'paid', paid_date = $2
		 WHERE payment_id = $1 AND status = 'planned' AND date = $2`,
		view.Rental.PaymentID, mustSeamDate("2026-08-15"))
	require.NoError(t, err)
	paid, err := h.svc.GetRental(context.Background(), h.owner, h.propID, view.Rental.ID)
	require.NoError(t, err)
	require.NotNil(t, paid.Progress.OverdueMonths)
	assert.Equal(t, 1, *paid.Progress.OverdueMonths)
	assert.Equal(t, 1, paid.Progress.PaidMonths)
}

// Список аренд читает оба счётчика прогресса одним батчем (#845): GROUP BY
// по платежам на реальном SQL; у платежа без операций строки в батче нет —
// нули дефолтятся в приложении. Ответ списка эквивалентен одиночным
// чтениям тех же аренд.
func TestSeam_ListRentalsBatchesTheProgressCounters(t *testing.T) {
	t.Parallel()
	h := newSeamHarness(t)
	h.seedOwner()

	// The first rental: backdated into July, the owner tick materializes
	// Jul+Aug as the overdue debt and Sep as the future planned; Aug then
	// gets paid — 1 paid, 1 overdue. The completion (today) drops the
	// future planned and frees the unfinished slot.
	first := h.createRental()
	start := mustSeamDate("2026-07-15")
	_, err := h.pool.Exec(context.Background(),
		`UPDATE rentals SET start_date = $1 WHERE id = $2`, start, first.Rental.ID)
	require.NoError(t, err)
	_, err = h.pool.Exec(context.Background(),
		`UPDATE payments SET since = $1 WHERE id = $2`, start, first.Rental.PaymentID)
	require.NoError(t, err)
	require.NoError(t, h.tick.RunOwnerTick(context.Background(), h.owner))
	_, err = h.pool.Exec(context.Background(),
		`UPDATE operations SET status = 'paid', paid_date = $2
		 WHERE payment_id = $1 AND status = 'planned' AND date = $2`,
		first.Rental.PaymentID, mustSeamDate("2026-08-15"))
	require.NoError(t, err)
	_, err = h.svc.CompleteRental(context.Background(), h.owner, h.propID,
		first.Rental.ID, rentalsapp.CompleteRentalCommand{CompletedDate: seamToday})
	require.NoError(t, err)

	// The second rental lives one create-completion pair: the create's
	// in-transaction tick stands Sep 15 and the completion drops it — its
	// payment ends with no operations rows at all, the batch has no row for
	// it and the service defaults its zeros.
	second := h.createRental()
	_, err = h.svc.CompleteRental(context.Background(), h.owner, h.propID,
		second.Rental.ID, rentalsapp.CompleteRentalCommand{CompletedDate: seamToday})
	require.NoError(t, err)

	views, err := h.svc.ListRentals(context.Background(), h.owner, h.propID)
	require.NoError(t, err)
	require.Len(t, views, 2)

	byID := make(map[uuid.UUID]rentalsapp.RentalView, len(views))
	for _, v := range views {
		byID[v.Rental.ID] = v
	}
	firstView, secondView := byID[first.Rental.ID], byID[second.Rental.ID]
	assert.Equal(t, 1, firstView.Progress.PaidMonths)
	require.NotNil(t, firstView.Progress.OverdueMonths)
	assert.Equal(t, 1, *firstView.Progress.OverdueMonths)
	assert.Zero(t, secondView.Progress.PaidMonths,
		"a payment without operations rows defaults its zeros")
	assert.Nil(t, secondView.Progress.OverdueMonths,
		"zero overdue reads as null — no red line (#817)")

	// The response equivalence (#845): the listed progress matches the
	// single reads of the same rentals.
	firstSingle, err := h.svc.GetRental(context.Background(), h.owner, h.propID, first.Rental.ID)
	require.NoError(t, err)
	assert.Equal(t, firstSingle.Progress, firstView.Progress)
	secondSingle, err := h.svc.GetRental(context.Background(), h.owner, h.propID, second.Rental.ID)
	require.NoError(t, err)
	assert.Equal(t, secondSingle.Progress, secondView.Progress)
}
