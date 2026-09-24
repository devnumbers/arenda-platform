package application

// The use case unit tests over in-memory doubles: the conveyor discipline
// (the lock → the app checks → the change → the audit → the tick verdict),
// the contract validations and the 409 vocabulary. The real-SQL properties of
// the same seams — the store binding, the composite factory, the payments
// gateway mechanics — are covered by the integration families (testcontainers).

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/rentals/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// today is the fixed owner's calendar date of the unit suite.
var today = mustDate("2026-09-04")

func mustDate(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

func mustID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	require.NoError(t, err)
	return id
}

// fakeTx and fakeUoW run work "in a transaction" that does nothing.
type fakeTx struct{}

func (fakeTx) Commit(context.Context) error   { return nil }
func (fakeTx) Rollback(context.Context) error { return nil }

type fakeUoW struct{}

func (fakeUoW) Do(ctx context.Context, work func(transaction.Tx) error) error {
	return work(fakeTx{})
}

// eventAuditRecord and eventHistoryRecord are the journal markers of the
// audit and the action journal writes.
const (
	eventAuditRecord   = "audit.Record"
	eventHistoryRecord = "history.Record"
)

// journal records the cross-fake call order.
type journal struct{ events []string }

func (j *journal) add(event string) { j.events = append(j.events, event) }

type fakeCalendar struct{}

func (fakeCalendar) Today(context.Context, uuid.UUID) (time.Time, error) { return today, nil }

type fakePropertyStore struct {
	owner    uuid.UUID
	archived bool
}

func (s fakePropertyStore) Get(context.Context, uuid.UUID) (paymentsapp.PropertyRef, error) {
	return paymentsapp.PropertyRef{OwnerID: s.owner, Archived: s.archived}, nil
}

func (s fakePropertyStore) GetForUpdate(context.Context, uuid.UUID) (paymentsapp.PropertyRef, error) {
	return paymentsapp.PropertyRef{OwnerID: s.owner, Archived: s.archived}, nil
}

func (s fakePropertyStore) WithTx(transaction.Tx) (paymentsapp.PropertyStore, error) {
	return s, nil
}

type fakePolicy struct{ role sharedpolicy.Role }

func (p fakePolicy) Role(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error) {
	return sharedpolicy.RoleNone, nil
}

func (p fakePolicy) RoleForProperty(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error) {
	return p.role, nil
}

type fakeAudit struct {
	journal *journal
	entries []auditdomain.Entry
}

func (a *fakeAudit) Record(_ context.Context, entry auditdomain.Entry) error {
	a.journal.add(eventAuditRecord)
	a.entries = append(a.entries, entry)
	return nil
}

func (a *fakeAudit) WithTx(transaction.Tx) auditapp.Recorder { return a }

type fakeHistory struct {
	journal *journal
	entries []historydomain.Entry
	err     error
}

func (h *fakeHistory) Record(_ context.Context, entry historydomain.Entry) error {
	h.journal.add(eventHistoryRecord)
	if h.err != nil {
		return h.err
	}
	h.entries = append(h.entries, entry)
	return nil
}

func (h *fakeHistory) WithTx(transaction.Tx) historyapp.Recorder { return h }

type fakeTenantReader struct{ exists bool }

func (r fakeTenantReader) Exists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return r.exists, nil
}

func (r fakeTenantReader) DisplayName(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return "Иван Tenant", nil
}

type fakeRentalStore struct {
	journal       *journal
	rentals       map[uuid.UUID]domain.Rental
	hasUnfinished bool
}

func (s *fakeRentalStore) Get(
	_ context.Context, id, _, _ uuid.UUID,
) (domain.Rental, error) {
	r, ok := s.rentals[id]
	if !ok {
		return domain.Rental{}, ErrNotFound
	}
	return r, nil
}

func (s *fakeRentalStore) ListByProperty(
	_ context.Context, _, _ uuid.UUID,
) ([]domain.Rental, error) {
	out := make([]domain.Rental, 0, len(s.rentals))
	for _, r := range s.rentals {
		out = append(out, r)
	}
	return out, nil
}

func (s *fakeRentalStore) Create(_ context.Context, r domain.Rental) error {
	s.journal.add("rentals.Create")
	s.rentals[r.ID] = r
	return nil
}

func (s *fakeRentalStore) Update(_ context.Context, r domain.Rental) error {
	s.journal.add("rentals.Update")
	s.rentals[r.ID] = r
	return nil
}

func (s *fakeRentalStore) Complete(
	_ context.Context, id, _ uuid.UUID, completedDate time.Time, depositReturn *DepositReturn,
) error {
	s.journal.add("rentals.Complete")
	r := s.rentals[id]
	r.CompletedDate = &completedDate
	if depositReturn != nil {
		amount := depositReturn.AmountKopecks
		r.DepositReturnKopecks = &amount
		r.DepositReturnComment = depositReturn.Comment
	}
	s.rentals[id] = r
	return nil
}

func (s *fakeRentalStore) Delete(_ context.Context, id, _ uuid.UUID) error {
	s.journal.add("rentals.Delete")
	delete(s.rentals, id)
	return nil
}

func (s *fakeRentalStore) HasUnfinished(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	s.journal.add("rentals.HasUnfinished")
	return s.hasUnfinished, nil
}

func (s *fakeRentalStore) WithTx(transaction.Tx) (RentalStore, error) { return s, nil }

type fakeGatewayTx struct {
	journal   *journal
	gateway   *fakeGateway
	createdID uuid.UUID
}

func (t *fakeGatewayTx) Create(_ context.Context, seed RentPaymentSeed) (uuid.UUID, error) {
	t.journal.add("pay.Create")
	t.gateway.createSeed = seed
	t.createdID = mustIDSeed()
	return t.createdID, nil
}

func (t *fakeGatewayTx) Update(
	_ context.Context, _, _, _ uuid.UUID, change RentPaymentChange, _ time.Time,
) error {
	t.journal.add("pay.Update")
	t.gateway.changes = append(t.gateway.changes, change)
	return nil
}

func (t *fakeGatewayTx) Stop(
	_ context.Context, _, _, _ uuid.UUID, completedDate time.Time,
) error {
	t.journal.add("pay.Stop")
	t.gateway.stoppedAt = &completedDate
	return nil
}

func (t *fakeGatewayTx) Delete(_ context.Context, _, _ uuid.UUID, _ time.Time) error {
	t.journal.add("pay.Delete")
	t.gateway.deleted = true
	return nil
}

func (t *fakeGatewayTx) RunTick(_ context.Context, _ uuid.UUID, _ time.Time) error {
	t.journal.add("pay.RunTick")
	return nil
}

func mustIDSeed() uuid.UUID {
	id, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	return id
}

type fakeGateway struct {
	journal *journal
	state   RentPaymentState
	next    *PlannedOccurrence
	paid    int
	overdue int
	totals  PaymentsTotals
	stateEr error

	createSeed RentPaymentSeed
	changes    []RentPaymentChange
	stoppedAt  *time.Time
	deleted    bool
	tx         *fakeGatewayTx
}

func (g *fakeGateway) WithTx(transaction.Tx) (RentPaymentGatewayTx, error) {
	g.tx = &fakeGatewayTx{journal: g.journal, gateway: g}
	return g.tx, nil
}

func (g *fakeGateway) RentPaymentState(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (RentPaymentState, error) {
	return g.state, g.stateEr
}

func (g *fakeGateway) NextPlannedOccurrence(
	context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time,
) (*PlannedOccurrence, error) {
	return g.next, nil
}

func (g *fakeGateway) CountPaidOperations(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (int, error) {
	return g.paid, nil
}

func (g *fakeGateway) CountOverdueOccurrences(
	context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time,
) (int, error) {
	return g.overdue, nil
}

func (g *fakeGateway) SummarizePaidOperations(
	context.Context, uuid.UUID, uuid.UUID, time.Time, time.Time, time.Time,
) (PaymentsTotals, error) {
	return g.totals, nil
}

// harness bundles the doubles into a service like the wire does.
type harness struct {
	t         *testing.T
	journal   *journal
	store     *fakeRentalStore
	gateway   *fakeGateway
	audit     *fakeAudit
	history   *fakeHistory
	svc       *RentalService
	owner     uuid.UUID
	property  uuid.UUID
	propStore fakePropertyStore
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	return newHarnessWithArchivedProperty(t, false)
}

// newHarnessWithArchivedProperty is the harness core with the property
// store's archived flag fixed at construction: the fake is copied into the
// factory by value, so a post-construction mutation would not reach it.
func newHarnessWithArchivedProperty(t *testing.T, archived bool) *harness {
	t.Helper()
	j := &journal{}
	owner, property := mustID(t), mustID(t)
	store := &fakeRentalStore{journal: j, rentals: map[uuid.UUID]domain.Rental{}}
	gateway := &fakeGateway{
		journal: j,
		state:   RentPaymentState{AmountKopecks: 5_000_000, PaymentDay: domain.MustPaymentDay(15)},
		paid:    3,
	}
	audit := &fakeAudit{journal: j}
	propStore := fakePropertyStore{owner: owner, archived: archived}
	history := &fakeHistory{journal: j}
	factory := NewTxStoreFactory(store, propStore, gateway, fakeTenantReader{exists: true}, audit, history, fakeUoW{})
	svc := NewRentalService(factory, fakeCalendar{}, fakePolicy{role: sharedpolicy.RoleOwner})
	return &harness{
		t: t, journal: j, store: store, gateway: gateway, audit: audit, history: history,
		svc: svc, owner: owner, property: property, propStore: propStore,
	}
}

// createCmd is the canonical valid create fixture.
func createCmd() CreateRentalCommand {
	return CreateRentalCommand{
		AmountKopecks:  5_000_000,
		PaymentDay:     domain.MustPaymentDay(15),
		StartDate:      today,
		PlannedEndDate: new(mustDate("2027-09-01")),
		Utilities:      domain.UtilitiesIncluded,
	}
}

func TestCreateRental_HappyPath(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	view, err := h.svc.CreateRental(t.Context(), h.owner, h.property, createCmd())
	require.NoError(t, err)

	// The conveyor discipline, in order: the app check under the lock, the
	// payment first (the rentals row references it), then the rentals row,
	// the audit, and the tick for the new payment.
	assert.Equal(t, []string{
		"rentals.HasUnfinished",
		"pay.Create",
		"rentals.Create",
		eventAuditRecord,
		eventHistoryRecord,
		"pay.RunTick",
	}, h.journal.events)

	seed := h.gateway.createSeed
	assert.Equal(t, h.owner, seed.OwnerID)
	assert.Equal(t, 5_000_000, int(seed.AmountKopecks))
	assert.Equal(t, today, seed.StartDate, "since = start_date, not the wire today")
	assert.Equal(t, mustDate("2027-09-01"), *seed.PlannedEndDate)
	assert.False(t, seed.AutoPay)

	require.Len(t, h.audit.entries, 1)
	entry := h.audit.entries[0]
	assert.Equal(t, auditdomain.ActionRentalCreated, entry.Action)
	assert.Equal(t, auditdomain.EntityRental, entry.EntityType)
	require.NotNil(t, entry.EntityID)
	assert.Equal(t, view.Rental.ID, *entry.EntityID)

	// The view assembles from the payment-backed state (the fake answers
	// 3 paid months of the day 15).
	assert.Equal(t, domain.StatusActive, view.Status, "start == today → active, not upcoming")
	assert.Equal(t, 3, view.Progress.PaidMonths)
	require.NotNil(t, view.Progress.TotalMonths)
	assert.Equal(t, 12, *view.Progress.TotalMonths, "day 15 from Sep 2026 to Sep 2027")
	assert.Equal(t, today, view.Today)
}

// The read path names the managed payment (#531): the client navigates to the
// payment screen by rentPayment.paymentId — the identity comes from the
// rental (1:1), not from the gateway's render state.
func TestGetRental_ViewCarriesPaymentID(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rentalID := h.seedRental(nil)

	view, err := h.svc.GetRental(t.Context(), h.owner, h.property, rentalID)
	require.NoError(t, err)
	assert.Equal(t, h.store.rentals[rentalID].PaymentID, view.Payment.PaymentID)
}

// Прогресс несёт серверный счётчик просрочки Платежа арендной платы (#817):
// число просроченных вхождений приходит из гейтвея, ноль — null (строки на
// экране нет). Форма срока на чтение не влияет: бессрочная считается так же,
// у upcoming просрочек не бывает по построению (вхождения начинаются со
// старта, а старт не в прошлом).
func TestGetRental_ProgressCarriesOverdueMonths(t *testing.T) {
	t.Parallel()
	term := func(r *domain.Rental) { r.PlannedEndDate = new(mustDate("2027-09-01")) }
	tests := []struct {
		name   string
		count  int
		mutate func(*domain.Rental)
		want   *int
	}{
		{"zero overdue is null", 0, term, nil},
		{"one overdue month", 1, term, new(1)},
		{"two overdue months", 2, term, new(2)},
		{"open-ended reads the same", 3, nil, new(3)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			h.gateway.overdue = tt.count
			rentalID := h.seedRental(tt.mutate)

			view, err := h.svc.GetRental(t.Context(), h.owner, h.property, rentalID)
			require.NoError(t, err)
			assert.Equal(t, tt.want, view.Progress.OverdueMonths)
		})
	}
}

func TestCreateRental_TodayStartIsActive(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	cmd := createCmd()
	cmd.PlannedEndDate = nil

	view, err := h.svc.CreateRental(t.Context(), h.owner, h.property, cmd)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusActive, view.Status)
	assert.Nil(t, view.Progress.TotalMonths, "an open-ended rental has no total")
	assert.Nil(t, view.Progress.MonthsRemaining)
}

func TestCreateRental_Validation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		mut  func(*CreateRentalCommand)
	}{
		{"zero amount", func(c *CreateRentalCommand) { c.AmountKopecks = 0 }},
		{"over the ceiling", func(c *CreateRentalCommand) { c.AmountKopecks = MaxAmountKopecks + 1 }},
		{"start in the past", func(c *CreateRentalCommand) { c.StartDate = today.AddDate(0, 0, -1) }},
		{"planned end at start", func(c *CreateRentalCommand) {
			c.PlannedEndDate = new(c.StartDate)
		}},
		{"unknown utilities", func(c *CreateRentalCommand) { c.Utilities = "квитанция" }},
		{"negative deposit", func(c *CreateRentalCommand) { c.DepositKopecks = new(int64(-1)) }},
		{"overlong comment", func(c *CreateRentalCommand) { c.Comment = strings2001() }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			cmd := createCmd()
			tt.mut(&cmd)
			_, err := h.svc.CreateRental(t.Context(), h.owner, h.property, cmd)
			require.ErrorIs(t, err, ErrInvalidInput)
			assert.Empty(t, h.journal.events, "an invalid command writes nothing")
		})
	}
}

func strings2001() string {
	s := make([]rune, MaxCommentLength+1)
	for i := range s {
		s[i] = 'а'
	}
	return string(s)
}

func TestCreateRental_SecondUnfinishedIs409(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.store.hasUnfinished = true

	_, err := h.svc.CreateRental(t.Context(), h.owner, h.property, createCmd())
	require.ErrorIs(t, err, ErrPropertyOccupied)
	assert.Equal(t, []string{"rentals.HasUnfinished"}, h.journal.events,
		"nothing is created on the occupied property")
}

func TestCreateRental_UnknownTenantIsInvalid(t *testing.T) {
	t.Parallel()
	j := &journal{}
	owner, property := mustID(t), mustID(t)
	factory := NewTxStoreFactory(
		&fakeRentalStore{journal: j, rentals: map[uuid.UUID]domain.Rental{}},
		fakePropertyStore{owner: owner},
		&fakeGateway{journal: j},
		fakeTenantReader{exists: false},
		&fakeAudit{journal: j},
		&fakeHistory{journal: j},
		fakeUoW{},
	)
	svc := NewRentalService(factory, fakeCalendar{}, fakePolicy{role: sharedpolicy.RoleOwner})
	contact := mustID(t)
	cmd := createCmd()
	cmd.ContactID = &contact

	_, err := svc.CreateRental(t.Context(), owner, property, cmd)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestUpdateRental_PaymentTermsSyncAndTick(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rentalID := h.seedRental(func(r *domain.Rental) {
		r.PlannedEndDate = new(mustDate("2027-09-01"))
	})

	cmd := UpdateRentalCommand{
		AmountKopecks:  new(int64(6_000_000)),
		AutoPay:        new(true),
		PlannedEndDate: &DateUpdate{}, // Явный null = очистить (бессрочная).
		Comment:        &StringUpdate{Value: new(" нюансы ")},
	}
	view, err := h.svc.UpdateRental(t.Context(), h.owner, h.property, rentalID, cmd)
	require.NoError(t, err)

	assert.Equal(t, []string{"pay.Update", "rentals.Update", eventAuditRecord, eventHistoryRecord, "pay.RunTick"}, h.journal.events)
	require.Len(t, h.gateway.changes, 1)
	change := h.gateway.changes[0]
	require.NotNil(t, change.AmountKopecks)
	assert.Equal(t, int64(6_000_000), *change.AmountKopecks)
	require.NotNil(t, change.AutoPay)
	assert.True(t, *change.AutoPay)
	require.NotNil(t, change.PlannedEndDate)
	assert.Nil(t, change.PlannedEndDate.Value, "the null clears the payment end date")

	// The rentals-side terms write without a tick-worthy change.
	assert.Equal(t, "нюансы", view.Rental.Comment)
	assert.Equal(t, auditdomain.ActionRentalUpdated, h.audit.entries[0].Action)
	require.Contains(t, h.audit.entries[0].Context, "fields")
}

func TestUpdateRental_NonPaymentFieldsSkipTick(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rentalID := h.seedRental(nil)

	cmd := UpdateRentalCommand{
		Utilities:      new(domain.UtilitiesMetersOnly),
		DepositKopecks: &AmountUpdate{Value: new(int64(30_000_000))},
	}
	_, err := h.svc.UpdateRental(t.Context(), h.owner, h.property, rentalID, cmd)
	require.NoError(t, err)

	assert.Equal(t, []string{"rentals.Update", eventAuditRecord, eventHistoryRecord}, h.journal.events,
		"the tick runs only when the payment changed")
}

func TestUpdateRental_PlannedEndRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		end  time.Time
		want error
	}{
		{"into the past is invalid", today.AddDate(0, 0, -1), ErrInvalidInput},
		{"at or before start is invalid", seedStart(), ErrInvalidInput},
		{"strictly forward is fine", mustDate("2027-01-01"), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			rentalID := h.seedRental(nil)
			cmd := UpdateRentalCommand{PlannedEndDate: &DateUpdate{Value: new(tt.end)}}
			_, err := h.svc.UpdateRental(t.Context(), h.owner, h.property, rentalID, cmd)
			if tt.want != nil {
				assert.ErrorIs(t, err, tt.want)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestUpdateRental_CompletedIs409(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rentalID := h.seedRental(func(r *domain.Rental) {
		r.CompletedDate = new(today.AddDate(0, 0, -10))
	})

	_, err := h.svc.UpdateRental(t.Context(), h.owner, h.property, rentalID, UpdateRentalCommand{})
	require.ErrorIs(t, err, ErrRentalCompleted)
	assert.Empty(t, h.journal.events)
}

func TestCompleteRental(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rentalID := h.seedRental(nil)

	deposit := int64(10_000_000)
	comment := "вернул частично"
	completed := today.AddDate(0, 0, -5)
	view, err := h.svc.CompleteRental(t.Context(), h.owner, h.property, rentalID, CompleteRentalCommand{
		CompletedDate: completed,
		DepositReturn: &DepositReturn{AmountKopecks: deposit, Comment: &comment},
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"pay.Stop", "rentals.Complete", eventAuditRecord, eventHistoryRecord}, h.journal.events,
		"the completion resolves the plan itself — no tick")
	require.NotNil(t, h.gateway.stoppedAt)
	assert.Equal(t, completed, *h.gateway.stoppedAt)
	require.NotNil(t, view.Rental.CompletedDate)
	assert.Equal(t, completed, *view.Rental.CompletedDate)
	assert.Equal(t, domain.StatusCompleted, view.Status)
	assert.Equal(t, auditdomain.ActionRentalCompleted, h.audit.entries[0].Action)

	t.Run("second completion is 409", func(t *testing.T) {
		t.Parallel()
		_, err := h.svc.CompleteRental(t.Context(), h.owner, h.property, rentalID,
			CompleteRentalCommand{CompletedDate: today})
		assert.ErrorIs(t, err, ErrRentalCompleted)
	})
}

func TestCompleteRental_Validation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cmd  CompleteRentalCommand
		want error
	}{
		{"before start", CompleteRentalCommand{CompletedDate: today.AddDate(0, 0, -40)}, ErrInvalidInput},
		{"in the future", CompleteRentalCommand{CompletedDate: today.AddDate(0, 0, 1)}, ErrInvalidInput},
		{
			"zero amount is valid (не вернул)",
			CompleteRentalCommand{CompletedDate: today, DepositReturn: &DepositReturn{AmountKopecks: 0}},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			rentalID := h.seedRental(nil)
			_, err := h.svc.CompleteRental(t.Context(), h.owner, h.property, rentalID, tt.cmd)
			if tt.want != nil {
				assert.ErrorIs(t, err, tt.want)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestDeleteRental(t *testing.T) {
	t.Parallel()
	t.Run("not started goes with the payment", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		rentalID := h.seedRental(func(r *domain.Rental) { r.StartDate = today.AddDate(0, 0, 3) })

		require.NoError(t, h.svc.DeleteRental(t.Context(), h.owner, h.property, rentalID))
		assert.Equal(t, []string{"rentals.Delete", "pay.Delete", eventAuditRecord, eventHistoryRecord}, h.journal.events,
			"the rentals row releases the RESTRICT FK before the payment goes")
		assert.True(t, h.gateway.deleted)
	})
	t.Run("completed goes too", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		rentalID := h.seedRental(func(r *domain.Rental) {
			r.CompletedDate = new(today.AddDate(0, 0, -1))
		})
		require.NoError(t, h.svc.DeleteRental(t.Context(), h.owner, h.property, rentalID))
	})
	t.Run("started unfinished is 409", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		rentalID := h.seedRental(nil)
		err := h.svc.DeleteRental(t.Context(), h.owner, h.property, rentalID)
		require.ErrorIs(t, err, ErrRentalStarted)
		assert.Empty(t, h.journal.events)
	})
}

func TestRoleMatrix(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.svc.policy = fakePolicy{role: sharedpolicy.RoleViewer}
	h.svc.writeGate = newCapabilityGate(h.svc.policy, sharedpolicy.CanEdit)
	h.svc.delGate = newCapabilityGate(h.svc.policy, sharedpolicy.CanLifecycle)

	t.Run("viewer cannot create", func(t *testing.T) {
		t.Parallel()
		_, err := h.svc.CreateRental(t.Context(), h.owner, h.property, createCmd())
		assert.ErrorIs(t, err, ErrForbidden)
	})
	t.Run("viewer cannot delete", func(t *testing.T) {
		t.Parallel()
		err := h.svc.DeleteRental(t.Context(), h.owner, h.property, mustID(t))
		assert.ErrorIs(t, err, ErrForbidden)
	})
	t.Run("stranger is the privacy 404", func(t *testing.T) {
		t.Parallel()
		stranger := fakePolicy{role: sharedpolicy.RoleNone}
		svc := NewRentalService(
			NewTxStoreFactory(h.store, h.propStore, h.gateway, fakeTenantReader{exists: true}, h.audit, &fakeHistory{journal: h.journal}, fakeUoW{}),
			fakeCalendar{}, stranger,
		)
		_, err := svc.GetRental(t.Context(), h.owner, h.property, mustID(t))
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestRentalSummary(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.gateway.totals = PaymentsTotals{IncomeKopecks: 10_000_000, ExpenseKopecks: 3_000_000}

	t.Run("defaults to today for a running rental", func(t *testing.T) {
		t.Parallel()
		sub := newHarness(t)
		sub.gateway.totals = h.gateway.totals
		rentalID := sub.seedRental(nil)
		sum, err := sub.svc.RentalSummary(t.Context(), sub.owner, sub.property, rentalID, nil)
		require.NoError(t, err)
		assert.Equal(t, today, sum.Until)
		assert.Equal(t, int64(7_000_000), sum.ProfitKopecks)
	})
	t.Run("explicit until", func(t *testing.T) {
		t.Parallel()
		sub := newHarness(t)
		rentalID := sub.seedRental(nil)
		until := today.AddDate(0, 0, -1)
		sum, err := sub.svc.RentalSummary(t.Context(), sub.owner, sub.property, rentalID, &until)
		require.NoError(t, err)
		assert.Equal(t, until, sum.Until)
	})
	t.Run("completed defaults to the completion date", func(t *testing.T) {
		t.Parallel()
		sub := newHarness(t)
		completed := today.AddDate(0, 0, -5)
		id := sub.seedRental(func(r *domain.Rental) { r.CompletedDate = &completed })
		sum, err := sub.svc.RentalSummary(t.Context(), sub.owner, sub.property, id, nil)
		require.NoError(t, err)
		assert.Equal(t, completed, sum.Until)
	})
	t.Run("until before start is invalid", func(t *testing.T) {
		t.Parallel()
		sub := newHarness(t)
		rentalID := sub.seedRental(nil)
		early := today.AddDate(0, 0, -100)
		_, err := sub.svc.RentalSummary(t.Context(), sub.owner, sub.property, rentalID, &early)
		assert.ErrorIs(t, err, ErrInvalidInput)
	})
}

func seedStart() time.Time { return today.AddDate(0, 0, -20) }

// seedRental stores a rental directly (start 20 days back) and returns its id.
func (h *harness) seedRental(mutate func(*domain.Rental)) uuid.UUID {
	h.t.Helper()
	id := mustID(h.t)
	r := domain.Rental{
		ID:         id,
		OwnerID:    h.owner,
		PropertyID: h.property,
		PaymentID:  mustID(h.t),
		StartDate:  seedStart(),
		Utilities:  domain.UtilitiesIncluded,
	}
	if mutate != nil {
		mutate(&r)
	}
	h.store.rentals[id] = r
	return id
}
