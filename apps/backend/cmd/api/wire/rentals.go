package wire

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	contactspg "github.com/nambers/arenda-planform/apps/backend/internal/contacts/adapters/postgres"
	contactsapp "github.com/nambers/arenda-planform/apps/backend/internal/contacts/application"
	paymentspg "github.com/nambers/arenda-planform/apps/backend/internal/payments/adapters/postgres"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	paymentsdomain "github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	rentalspg "github.com/nambers/arenda-planform/apps/backend/internal/rentals/adapters/postgres"
	rentalsapp "github.com/nambers/arenda-planform/apps/backend/internal/rentals/application"
	rentalsdomain "github.com/nambers/arenda-planform/apps/backend/internal/rentals/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// Rentals holds the rentals module's service wired by WireRentals plus the
// read-only occupancy adapter the properties list projections consume
// (ticket #585) and the delete-side guard the property deletion consults
// (issue #632).
type Rentals struct {
	RentalService   *rentalsapp.RentalService
	OccupancyReader *rentalspg.OccupancyReader
	DeletionGuard   *rentalspg.DeletionGuard
}

// WireRentals constructs the rentals context (ADR 0053, ticket #529): the
// rentals store, the payments property store — the same serialization point
// the payments mutations lock — the composite transaction factory and the
// owner calendar (ADR 0048). The seam to the managed rent payment is the
// wiring-bound RentPaymentGateway over the payments stores; the tenant
// reader rides the contacts store. The policy comes from the access module —
// rentals is wired after it, so the membership-aware policy is already
// resolved.
func WireRentals(p platformDeps) (*Rentals, error) {
	rentalStore := rentalspg.NewRentalStore(p.DB)
	propertyStore := paymentspg.NewPropertyStore(p.DB)
	paymentStore := paymentspg.NewPaymentStore(p.DB)
	operationStore := paymentspg.NewOperationStore(p.DB)
	tickStore := paymentspg.NewTickStore(p.DB)
	contactStore := contactspg.NewContactStore(p.DB)
	calendar := paymentspg.NewOwnerCalendar(p.DB, p.Clock)

	changeLogStore := paymentspg.NewPaymentChangeLogStore(p.DB)
	gateway := NewRentPaymentGateway(paymentStore, tickStore, operationStore, changeLogStore)
	factory := rentalsapp.NewTxStoreFactory(
		rentalStore,
		propertyStore,
		gateway,
		NewTenantBookReader(contactStore),
		p.AuditRecorder,
		p.HistoryRecorder,
		p.UoW,
	)

	return &Rentals{
		RentalService:   rentalsapp.NewRentalService(factory, calendar, p.Policy),
		OccupancyReader: rentalspg.NewOccupancyReader(p.DB, calendar),
		DeletionGuard:   rentalspg.NewDeletionGuard(),
	}, nil
}

// rentPaymentGateway fits the rentals consumer port to the payments internals
// (ADR 0053 §3): the recurrence constructor, the tick plan and its
// application, the future-planned teardown — reusing the payments stores the
// composite factory binds to the caller's transaction, with no second
// property lock and no second conveyor. The rent payment is built here as the
// ordinary payments rule it is: income, «Арендная плата», transfer form, the
// rent category. The change log store rides along: the terms edit's diff
// lands in the managed payment's history from this seam too (ADR 0065 §4,
// решение владельца 07.10).
type rentPaymentGateway struct {
	payments   paymentsapp.PaymentStore
	tick       paymentsapp.TickStore
	operations paymentsapp.OperationStore
	changeLog  paymentsapp.PaymentChangeLogStore
}

// NewRentPaymentGateway builds the seam adapter over the payments stores.
func NewRentPaymentGateway(
	payments paymentsapp.PaymentStore, tick paymentsapp.TickStore, operations paymentsapp.OperationStore,
	changeLog paymentsapp.PaymentChangeLogStore,
) rentalsapp.RentPaymentGateway {
	return &rentPaymentGateway{payments: payments, tick: tick, operations: operations, changeLog: changeLog}
}

// WithTx binds the gateway to the caller's transaction: the payments stores
// join the rentals unit of work, so the sync commits — or rolls back — with
// the rental change.
func (g *rentPaymentGateway) WithTx(tx transaction.Tx) (rentalsapp.RentPaymentGatewayTx, error) {
	payments, err := g.payments.WithTx(tx)
	if err != nil {
		return nil, fmt.Errorf("bind payments store to the rentals tx: %w", err)
	}
	tick, err := g.tick.WithTx(tx)
	if err != nil {
		return nil, fmt.Errorf("bind tick store to the rentals tx: %w", err)
	}
	changeLog, err := g.changeLog.WithTx(tx)
	if err != nil {
		return nil, fmt.Errorf("bind change log store to the rentals tx: %w", err)
	}
	return &rentPaymentGatewayTx{payments: payments, tick: tick, changeLog: changeLog}, nil
}

// rentPaymentGatewayTx is the transaction-bound half of the seam.
type rentPaymentGatewayTx struct {
	payments  paymentsapp.PaymentStore
	tick      paymentsapp.TickStore
	changeLog paymentsapp.PaymentChangeLogStore
}

// rentPaymentTitle and rentCategorySlug are the managed payment's fixed
// defaults (ADR 0053 §3: доход, категория rent, форма оплаты transfer).
const (
	rentPaymentTitle = "Арендная плата"
	rentCategorySlug = "rent"
)

// Create inserts the managed payment rule and returns its id: the recurrence
// from the payment day (1..30 → their day, 31 and «последний день» → the
// last-day marker, одно поведение), since = the rental start (старт сегодня →
// ровно today; ретро-материализации нет), end_date = the planned end.
func (t *rentPaymentGatewayTx) Create(
	ctx context.Context, seed rentalsapp.RentPaymentSeed,
) (uuid.UUID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, fmt.Errorf("mint rent payment id: %w", err)
	}
	recurrence, err := recurrenceForDay(seed.PaymentDay)
	if err != nil {
		return uuid.Nil, fmt.Errorf("build rent recurrence: %w", err)
	}
	category := rentCategorySlug
	if err := t.payments.Create(ctx, paymentsdomain.Payment{
		ID:                 id,
		OwnerID:            seed.OwnerID,
		PropertyID:         seed.PropertyID,
		Type:               paymentsdomain.TypeIncome,
		Title:              rentPaymentTitle,
		AmountKopecks:      seed.AmountKopecks,
		Recurrence:         recurrence,
		Since:              seed.StartDate,
		EndDate:            seed.PlannedEndDate,
		AutoPay:            seed.AutoPay,
		NotifyAutoPaid:     seed.NotifyAutoPaid,
		ReminderOffsetDays: seed.ReminderOffsetDays,
		Category:           paymentsdomain.CategoryRef{Slug: &category},
	}); err != nil {
		return uuid.Nil, fmt.Errorf("insert rent payment: %w", err)
	}
	return id, nil
}

// Update syncs the terms edit into the payment and drops its strictly future
// planned — the caller's in-transaction tick stands the single future planned
// again with fresh snapshots (the ordinary payment edit mechanics). The
// edit's diff lands in the managed payment's change log from this seam — the
// second write point of ADR 0065 §4 (решение владельца 07.10): the same
// dictionary, the same domain diff; a terms edit that moves nothing writes no
// row.
func (t *rentPaymentGatewayTx) Update(
	ctx context.Context, scope, propertyID, paymentID, actorID uuid.UUID,
	change rentalsapp.RentPaymentChange, today time.Time,
) error {
	payment, err := t.payments.Get(ctx, paymentID, scope, propertyID)
	if err != nil {
		return fmt.Errorf("load rent payment: %w", err)
	}
	// The diff's before side is the loaded rule: the merge below replaces
	// fields and pointers wholesale, so the shallow copy is a faithful
	// snapshot — the same shape the PaymentService's own edit runs.
	before := payment
	if change.AmountKopecks != nil {
		payment.AmountKopecks = *change.AmountKopecks
	}
	if change.PaymentDay != nil {
		recurrence, err := recurrenceForDay(*change.PaymentDay)
		if err != nil {
			return fmt.Errorf("build rent recurrence: %w", err)
		}
		payment.Recurrence = recurrence
	}
	if change.AutoPay != nil {
		payment.AutoPay = *change.AutoPay
	}
	if change.PlannedEndDate != nil {
		payment.EndDate = change.PlannedEndDate.Value
	}
	changes := paymentsdomain.DiffPaymentChanges(before, payment)
	if err := t.payments.Update(ctx, payment); err != nil {
		return fmt.Errorf("update rent payment: %w", err)
	}
	if err := t.payments.DeleteFuturePlanned(ctx, paymentID, today); err != nil {
		return fmt.Errorf("drop future planned of rent payment: %w", err)
	}
	// The rental pipeline never edits the title, so a diff here never flips
	// the manual-title marker (ADR 0065 §3) — nothing extra to write back.
	if len(changes) > 0 {
		if err := t.changeLog.Record(ctx, scope, propertyID, actorID, paymentsapp.ChangeLogWrite{
			PaymentID: paymentID,
			Action:    paymentsdomain.ChangeUpdated,
			Changes:   changes,
		}); err != nil {
			return fmt.Errorf("record rent payment change log: %w", err)
		}
	}
	return nil
}

// State reads the managed payment's current terms inside the caller's
// transaction — the completion's archive source (the snapshot lands on the
// rental, ревизия ADR 0053 #1161) and the payment-deletion journal row's
// title.
func (t *rentPaymentGatewayTx) State(
	ctx context.Context, scope, propertyID, paymentID uuid.UUID,
) (rentalsapp.RentPaymentTerms, error) {
	payment, err := t.payments.Get(ctx, paymentID, scope, propertyID)
	if err != nil {
		return rentalsapp.RentPaymentTerms{}, fmt.Errorf("load rent payment: %w", err)
	}
	day, err := paymentDayFromRecurrence(payment.Recurrence)
	if err != nil {
		return rentalsapp.RentPaymentTerms{}, fmt.Errorf("read rent payment day: %w", err)
	}
	return rentalsapp.RentPaymentTerms{
		AmountKopecks: payment.AmountKopecks,
		PaymentDay:    day,
		AutoPay:       payment.AutoPay,
		Title:         payment.Title,
	}, nil
}

// Delete removes the payment with the keep_overdue=true semantics: the
// planned from today on always goes, the overdue stays as debt (payment_id
// set to NULL by the FK, origin unchanged).
func (t *rentPaymentGatewayTx) Delete(
	ctx context.Context, scope, paymentID uuid.UUID, today time.Time,
) error {
	if err := t.payments.DeletePlannedFrom(ctx, paymentID, today); err != nil {
		return fmt.Errorf("drop planned from today: %w", err)
	}
	if err := t.payments.Delete(ctx, paymentID, scope); err != nil {
		return fmt.Errorf("delete rent payment: %w", err)
	}
	return nil
}

// RunTick runs the payments materialization tick for the owner inside the
// caller's transaction — the shared tick body, exactly like a payments
// mutation's in-transaction rerun.
func (t *rentPaymentGatewayTx) RunTick(ctx context.Context, ownerID uuid.UUID, today time.Time) error {
	return paymentsapp.RunOwnerTickInTx(ctx, t.tick, ownerID, today)
}

// recurrenceForDay maps the rentals payment day onto the payments monthly
// recurrence: 1..30 keep their day, 31 and «последний день месяца» are the
// one last-day behaviour (решение №5).
func recurrenceForDay(day rentalsdomain.PaymentDay) (paymentsdomain.Recurrence, error) {
	if day.IsLast() {
		return paymentsdomain.NewMonthlyRecurrence(nil, true)
	}
	return paymentsdomain.NewMonthlyRecurrence([]int{day.Day()}, false)
}

// paymentDayFromRecurrence reads the rentals payment day back from the
// payment's monthly recurrence — the render never keeps a rental-side copy
// (решение №5).
func paymentDayFromRecurrence(recurrence paymentsdomain.Recurrence) (rentalsdomain.PaymentDay, error) {
	if recurrence.Kind() != paymentsdomain.RecurrenceMonthly {
		return rentalsdomain.PaymentDay{}, fmt.Errorf(
			"rent payment recurrence must be monthly, got %q", recurrence.Kind(),
		)
	}
	if recurrence.LastDay() {
		return rentalsdomain.NewLastPaymentDay(), nil
	}
	days := recurrence.DaysOfMonth()
	if len(days) != 1 {
		return rentalsdomain.PaymentDay{}, fmt.Errorf(
			"rent payment recurrence must carry one day of month, got %v", days,
		)
	}
	return rentalsdomain.NewPaymentDay(days[0])
}

// RentPaymentState loads the payment's render state (ADR 0053 §2).
func (g *rentPaymentGateway) RentPaymentState(
	ctx context.Context, scope, propertyID, paymentID uuid.UUID,
) (rentalsapp.RentPaymentState, error) {
	payment, err := g.payments.Get(ctx, paymentID, scope, propertyID)
	if err != nil {
		return rentalsapp.RentPaymentState{}, err
	}
	day, err := paymentDayFromRecurrence(payment.Recurrence)
	if err != nil {
		return rentalsapp.RentPaymentState{}, fmt.Errorf("read payment day: %w", err)
	}
	return rentalsapp.RentPaymentState{
		AmountKopecks:      payment.AmountKopecks,
		PaymentDay:         day,
		AutoPay:            payment.AutoPay,
		ReminderOffsetDays: payment.ReminderOffsetDays,
	}, nil
}

// NextPlannedOccurrence returns the payment's single future planned operation
// — the earliest view-planned (date >= today) — or nil when there is none:
// nil — будущего вхождения нет, не ошибка (ADR 0053 §2).
func (g *rentPaymentGateway) NextPlannedOccurrence(
	ctx context.Context, scope, propertyID, paymentID uuid.UUID, today time.Time,
) (*rentalsapp.PlannedOccurrence, error) {
	planned := paymentsdomain.ViewStatusPlanned
	ops, err := g.operations.ListByPayment(ctx, scope, propertyID, paymentID,
		paymentsapp.OperationsListQuery{
			Status: &planned,
			Today:  today,
			Asc:    true,
			Limit:  1,
		})
	if err != nil {
		return nil, err
	}
	if len(ops) == 0 {
		return nil, nil // Будущего вхождения нет — это nil, не ошибка (ADR 0053 §2; исключение nilnil — в .golangci.yml).
	}
	op := ops[0]
	return &rentalsapp.PlannedOccurrence{
		OperationID:   op.ID,
		Date:          op.Date,
		AmountKopecks: op.AmountKopecks,
	}, nil
}

// CountPaidOperations counts the payment's paid operations — the progress'
// paidMonths (ADR 0053 §2).
func (g *rentPaymentGateway) CountPaidOperations(
	ctx context.Context, scope, propertyID, paymentID uuid.UUID,
) (int, error) {
	count, err := g.operations.CountPaidOperationsByPayment(ctx, scope, propertyID, paymentID)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// CountOverdueOccurrences counts the payment's overdue occurrences — the
// planned dated before the owner's today — the progress' overdueMonths
// (#817).
func (g *rentPaymentGateway) CountOverdueOccurrences(
	ctx context.Context, scope, propertyID, paymentID uuid.UUID, today time.Time,
) (int, error) {
	count, err := g.operations.CountOverdueOperationsByPayment(ctx, scope, propertyID, paymentID, today)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// CountProgressByPayments reads the listed payments' progress counters in
// one batched query (#845): the store's GROUP BY answers one row per rule
// — a payment without operations is absent from the rows, the returned map
// defaults its zeros on the lookup.
func (g *rentPaymentGateway) CountProgressByPayments(
	ctx context.Context, scope, propertyID uuid.UUID, paymentIDs []uuid.UUID, today time.Time,
) (map[uuid.UUID]rentalsapp.ProgressCounts, error) {
	counts, err := g.operations.CountPaidAndOverdueByPaymentIDs(ctx, scope, propertyID, paymentIDs, today)
	if err != nil {
		return nil, err
	}
	progress := make(map[uuid.UUID]rentalsapp.ProgressCounts, len(counts))
	for _, c := range counts {
		progress[c.PaymentID] = rentalsapp.ProgressCounts{
			PaidCount:    int(c.PaidCount),
			OverdueCount: int(c.OverdueCount),
		}
	}
	return progress, nil
}

// SummarizePaidOperations totals the property's paid operations with the
// operation date inside [from, until] — of any payment and manual ones.
func (g *rentPaymentGateway) SummarizePaidOperations(
	ctx context.Context, scope, propertyID uuid.UUID, from, until, today time.Time,
) (rentalsapp.PaymentsTotals, error) {
	paid := paymentsdomain.ViewStatusPaid
	summary, err := g.operations.SummarizeByProperty(ctx, scope, propertyID,
		paymentsapp.OperationsSummaryQuery{
			Status:   &paid,
			DateFrom: &from,
			DateTo:   &until,
			Today:    today,
		})
	if err != nil {
		return rentalsapp.PaymentsTotals{}, err
	}
	return rentalsapp.PaymentsTotals{
		IncomeKopecks:  summary.IncomeTotalKopecks,
		ExpenseKopecks: summary.ExpenseTotalKopecks,
	}, nil
}

// tenantBookReader fits the rentals tenant port to the contacts store: the
// contact's owner check is the application's duty (ADR 0053 — the FK does
// not verify the owner), and the reader answers for the scope's book only.
type tenantBookReader struct {
	contacts contactsapp.ContactStore
}

// NewTenantBookReader builds the tenant reader over the contacts store.
func NewTenantBookReader(contacts contactsapp.ContactStore) rentalsapp.TenantReader {
	return &tenantBookReader{contacts: contacts}
}

// ValidatedTenantLabel reads the tenant card once and resolves its ФИО for
// the action journal rows (ADR 0061 §3); the boolean reports whether the
// card belongs to the scope owner's book (ADR 0053 — the FK does not verify
// the owner). An unknown or foreign card is an empty label and false, not
// an error; a failing read is loud.
func (r *tenantBookReader) ValidatedTenantLabel(
	ctx context.Context, scope, contactID uuid.UUID,
) (label string, ok bool, err error) {
	contact, err := r.contacts.GetByID(ctx, contactID)
	if errors.Is(err, contactsapp.ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if contact.OwnerID != scope {
		return "", false, nil
	}
	return contact.FullName(), true, nil
}
