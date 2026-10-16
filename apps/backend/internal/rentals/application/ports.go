// Package application holds the rentals use cases and ports (ADR 0053): the
// rental CRUD with the completion and the deletion, the summary over the
// property's paid operations, and the seam to the managed rent payment — the
// composite transaction factory plus the consumer-declared
// RentPaymentGateway the wiring fits to the payments internals.
package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/rentals/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// RentalStore is the persistence port of the rentals (ADR 0053 §1). Reads and
// writes are scoped by the data owner (ADR 0028) and the nested
// rental→property path lives in the queries; the tenant fields travel inside
// domain.Rental, resolved by the adapter's join. The mutating methods must
// run inside the transaction holding the property serialization lock.
type RentalStore interface {
	// Get loads one rental; ErrNotFound when the id is unknown, belongs to
	// another owner or hangs on another property.
	Get(ctx context.Context, id, scope, propertyID uuid.UUID) (domain.Rental, error)
	// ListByProperty returns the property's rentals: unfinished first (newest
	// start on top), then the completed ones by completion date, fresh on top.
	ListByProperty(ctx context.Context, scope, propertyID uuid.UUID) ([]domain.Rental, error)
	// Create inserts the rental (the id, owner and payment link are app-side).
	Create(ctx context.Context, r domain.Rental) error
	// Update writes the editable fields of the rental; the start date is not
	// among them, and completion belongs to Complete.
	Update(ctx context.Context, r domain.Rental) error
	// Complete writes the completion fact and the optional deposit return in
	// one UPDATE, releases the payment link (the payment dies in the same
	// transaction, ревизия #1161) and lands the terms archive: rows affected
	// = 0 is not an error — the use case has already proven existence inside
	// the same transaction and lock.
	Complete(
		ctx context.Context, id, scope uuid.UUID, completedDate time.Time,
		depositReturn *DepositReturn, archived domain.TermsArchive,
	) error
	// Delete removes the rental row; it must run before the managed
	// payment's own delete (the RESTRICT FK releases only afterwards).
	Delete(ctx context.Context, id, scope uuid.UUID) error
	// HasUnfinished reports the property's unfinished rental (invariant №12);
	// the create-time app check under the property lock.
	HasUnfinished(ctx context.Context, scope, propertyID uuid.UUID) (bool, error)
	WithTx(tx transaction.Tx) (RentalStore, error)
}

// RentPaymentSeed is what the rent payment needs at creation (ADR 0053 §3):
// the adapter builds the payments rule from it — income, «Арендная плата»,
// transfer form, the rent category, the recurrence from the payment day,
// since = the rental start, end_date = the planned end.
type RentPaymentSeed struct {
	OwnerID        uuid.UUID
	PropertyID     uuid.UUID
	AmountKopecks  int64
	PaymentDay     domain.PaymentDay
	StartDate      time.Time
	PlannedEndDate *time.Time
	AutoPay        bool
	// NotifyAutoPaid is the managed payment's gate of the «Автоплатёж
	// исполнен» event (payments #1189): the rent pipeline stamps it true
	// unconditionally (#1198, по подписи макета 1428) — an auto-pay turned
	// on later by the terms edit notifies too.
	NotifyAutoPaid bool
	// ReminderOffsetDays is the managed payment's reminder lead time
	// (карта #822): 1/3/7, nil = без напоминаний. Аренда ставит его вместе
	// с автоплатежом (решение #823).
	ReminderOffsetDays *int
}

// ReminderOffsetUpdate is the tri-state resolution of the reminder patch
// (#1208): Value nil turns the managed payment's reminders off («Не
// напоминать»), 1/3/7 sets the lead time; a nil command field keeps the
// current value. The 1/3/7-or-nil contract itself is the payments'
// vocabulary — validated against paymentsapp.IsValidReminderOffset.
type ReminderOffsetUpdate struct {
	Value *int
}

// RentPaymentChange is the partial sync payload of the terms edit: a nil
// field leaves the payment unchanged.
type RentPaymentChange struct {
	AmountKopecks      *int64
	PaymentDay         *domain.PaymentDay
	AutoPay            *bool
	PlannedEndDate     *DateUpdate
	ReminderOffsetDays *ReminderOffsetUpdate
}

// RentPaymentState is the payment's render state (ADR 0053 §2): the day of
// payment lives in the payment's recurrence (решение №5) — the rental
// response reads it from here, never from a rental copy. PaymentID names the
// managed payment for the client's navigation to the payment screen (#531):
// payments are never matched by the rent category slug — properties may hold
// unrelated rent payments.
type RentPaymentState struct {
	PaymentID     uuid.UUID
	AmountKopecks int64
	PaymentDay    domain.PaymentDay
	AutoPay       bool
	// ReminderOffsetDays is the managed payment's reminder lead time read
	// back for the settings screens (карта #822); nil = без напоминаний.
	ReminderOffsetDays *int
}

// RentPaymentTerms is the managed payment's current terms read inside the
// caller's transaction — the completion's archive source (ревизия ADR 0053
// #1161): the snapshot lands on the rental and the title names the
// payment-deletion journal row.
type RentPaymentTerms struct {
	AmountKopecks int64
	PaymentDay    domain.PaymentDay
	AutoPay       bool
	Title         string
}

// PlannedOccurrence is the payment's single future planned operation
// (payments keep exactly one): the «Оплатить платёж» target and the
// nextPayment view.
type PlannedOccurrence struct {
	OperationID   uuid.UUID
	Date          time.Time
	AmountKopecks int64
}

// ProgressCounts is one payment's progress counters (ticket #845): the paid
// facts and the overdue planned rows behind the «N из M» progress.
type ProgressCounts struct {
	PaidCount    int
	OverdueCount int
}

// PaymentsTotals is the paid-operations aggregate of the property over a
// period: both directions, always (the profit computes as income − expense
// and may be negative).
type PaymentsTotals struct {
	IncomeKopecks  int64
	ExpenseKopecks int64
}

// RentPaymentGatewayTx is the transaction-bound half of the payments seam
// (ADR 0053 §3): the payment sync runs inside the caller's open transaction,
// after the property lock — one serialization point, one in-transaction tick,
// no second conveyor.
type RentPaymentGatewayTx interface {
	// Create inserts the rent payment rule and returns its id; the rentals
	// row referencing it follows within the same transaction.
	Create(ctx context.Context, seed RentPaymentSeed) (uuid.UUID, error)
	// Update syncs the terms edit into the payment: the editable fields,
	// then the strictly future planned is dropped — the caller's tick
	// verdict stands the single future planned again with fresh snapshots.
	// The actor travels for the payment change log (ADR 0065): the terms
	// edit's diff lands in the managed payment's history from this seam too.
	Update(
		ctx context.Context, scope, propertyID, paymentID, actorID uuid.UUID,
		change RentPaymentChange, today time.Time,
	) error
	// State reads the managed payment's current terms inside the caller's
	// transaction — the completion's archive source and the journal row's
	// title (ревизия #1161).
	State(ctx context.Context, scope, propertyID, paymentID uuid.UUID) (RentPaymentTerms, error)
	// Delete removes the payment with the keep_overdue=true semantics: the
	// planned from today on goes, the overdue stays as debt (payment_id set
	// to NULL by the FK, origin unchanged). The completion releases the
	// rental's link (and lands the terms archive) before this call; the
	// unfinished rental's deletion removes the rentals row first.
	Delete(ctx context.Context, scope, paymentID uuid.UUID, today time.Time) error
	// RunTick runs the payments materialization tick for the owner inside
	// the same transaction (the in-mutation rerun, ADR 0049 §3).
	RunTick(ctx context.Context, ownerID uuid.UUID, today time.Time) error
}

// RentPaymentGateway is the consumer-declared seam port to the managed rent
// payment (ADR 0053 §3: «порты объявляет consumer»). The reads serve the
// rental views; the mutations are transaction-bound via WithTx so the sync
// lands in the same commit as the rental change. The wiring fits this port
// to the payments internals — the recurrence constructor, the tick plan and
// its application, the future-planned teardown.
type RentPaymentGateway interface {
	WithTx(tx transaction.Tx) (RentPaymentGatewayTx, error)
	// RentPaymentState loads the payment's render state; ErrNotFound when
	// the payment is unknown or foreign.
	RentPaymentState(
		ctx context.Context, scope, propertyID, paymentID uuid.UUID,
	) (RentPaymentState, error)
	// NextPlannedOccurrence returns the payment's single future planned
	// operation (date >= today); nil when there is none — after the planned
	// end or at a completed rental.
	NextPlannedOccurrence(
		ctx context.Context, scope, propertyID, paymentID uuid.UUID, today time.Time,
	) (*PlannedOccurrence, error)
	// CountPaidOperations counts the payment's paid operations — the
	// progress' paidMonths, «N из M» (ADR 0053 §2).
	CountPaidOperations(ctx context.Context, scope, propertyID, paymentID uuid.UUID) (int, error)
	// CountOverdueOccurrences counts the payment's overdue occurrences — the
	// planned rows dated before the owner's today — the progress'
	// overdueMonths, «Просрочено N месяцев» (#817). An upcoming rental has
	// none by construction: the occurrences begin at the start, and the
	// start is never in the past.
	CountOverdueOccurrences(
		ctx context.Context, scope, propertyID, paymentID uuid.UUID, today time.Time,
	) (int, error)
	// CountProgressByPayments reads the listed payments' progress counters
	// in one batched read (ticket #845): the list's «N из M» / «Просрочено
	// N месяцев» — one query where the per-rental pair takes two. A payment
	// with no operations is absent from the result — the map lookup
	// defaults its zeros. The single-rental views keep their per-rental
	// reads.
	CountProgressByPayments(
		ctx context.Context, scope, propertyID uuid.UUID, paymentIDs []uuid.UUID, today time.Time,
	) (map[uuid.UUID]ProgressCounts, error)
	// SummarizePaidOperations totals the property's paid operations — of any
	// payment and manual ones — with the operation date inside [from, until]
	// (решение №13: the period is by the operation date, not the payment
	// date). Today carries the owner's today the underlying view-status
	// machinery expects; the paid filter itself never uses it.
	SummarizePaidOperations(
		ctx context.Context, scope, propertyID uuid.UUID, from, until, today time.Time,
	) (PaymentsTotals, error)
}

// TenantReader reads the tenant's contact card from the contacts book. The
// contacts owner check is the application's duty (ADR 0053: the FK does not
// verify the owner), so the reader answers for the scope's book only.
type TenantReader interface {
	// ValidatedTenantLabel reads the tenant card once and resolves its
	// display name (the ФИО) — the label snapshot the action journal rows
	// carry (ADR 0061 §3). The boolean reports whether the card belongs to
	// the scope owner's book: an unknown or foreign card is an empty label
	// and false, not an error.
	ValidatedTenantLabel(ctx context.Context, scope, contactID uuid.UUID) (string, bool, error)
}
