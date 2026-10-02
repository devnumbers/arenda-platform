// Package application holds the payments use cases and ports: the
// materialization tick run (ADR 0049 §3) with its persistence port, the owner
// calendar (ADR 0048), the payment CRUD store with the property serialization
// port (ticket #457), and the operations/favorites use cases of the second
// contracts slice (ticket #461).
package application

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// The application error vocabulary of the payment use cases: the transport
// maps them onto the wire contract (400/403/404/409).
var (
	// ErrNotFound covers a missing property, a missing or foreign payment and
	// an actor without the view capability — the privacy-preserving 404.
	ErrNotFound = errors.New("payments: not found")
	// ErrInvalidInput marks a command that violates the create/update
	// contract (empty title, amount out of bounds, unknown category slug,
	// recurrence zero value, endDate before since).
	ErrInvalidInput = errors.New("payments: invalid input")
	// ErrForbidden marks an actor whose role grants the view capability but
	// not the one the use case needs (viewer on mutations, non-owner on
	// deletion — the ADR 0028 matrix).
	ErrForbidden = errors.New("payments: forbidden")
	// ErrArchivedProperty marks a mutation on an archived property — the
	// financial read-only state (ticket #446).
	ErrArchivedProperty = errors.New("payments: property is archived")
	// ErrAlreadyPaused marks a pause on a rule with an open pause already.
	ErrAlreadyPaused = errors.New("payments: payment already paused")
	// ErrNotPaused marks a resume of a rule without an open pause.
	ErrNotPaused = errors.New("payments: payment is not paused")
	// ErrAlreadyPaid marks «Оплатить сейчас» on an operation that is already
	// paid — the contract's 409 on a repeated pay (ticket #461).
	ErrAlreadyPaid = errors.New("payments: operation already paid")
	// ErrRentManagedPayment marks a rule mutation of the rent payment — the
	// payment a rental row references (ADR 0053, ticket #818): the rental is
	// the source of truth for the amount, the payment day, the auto-pay and
	// the planned end, and the payment is created, edited and deleted only
	// through it (rentals/CONTEXT.md «Платёж арендной платы»). The payment
	// facts (pay) and the favorite star stay open — they are not the
	// rental's terms.
	ErrRentManagedPayment = errors.New("payments: payment is managed by a rental")
)

// PropertyRef is the payments view of the property a use case targets: the
// data owner (the SQL scope, ADR 0028) and the archived flag (the financial
// read-only state, ticket #446). The Maintenance flag rides along for the
// rentals guard (ticket #1050): rentals reject the mutations of an
// unfinished rental on a maintenance property, payments itself stays open
// on it. The properties context owns the entity; the consumers never need
// more of it than these flags.
type PropertyRef struct {
	OwnerID     uuid.UUID
	Archived    bool
	Maintenance bool
}

// PropertyStore resolves the property a payments use case targets. The
// ForUpdate variant is the context's serialization point (ADR 0049 §3): every
// mutation locks the property row before reading or writing a rule, so
// mutation-vs-tick, mutation-vs-mutation and archive-vs-mutation serialize on
// one point.
type PropertyStore interface {
	// Get loads the property reference without locking; ErrNotFound when no
	// such property exists.
	Get(ctx context.Context, propertyID uuid.UUID) (PropertyRef, error)
	// GetForUpdate loads the property reference with the row locked inside
	// the caller's transaction.
	GetForUpdate(ctx context.Context, propertyID uuid.UUID) (PropertyRef, error)
	WithTx(tx transaction.Tx) (PropertyStore, error)
}

// RentalManagedReader is the consumer-declared port answering which payment
// rules a rental manages (ADR 0053, ticket #818 — the reverse half of the
// rentals seam: the rentals context declares its payments-side gateway, this
// port is payments' mirror image of it). The rentals schema stays the
// rentals context's own — the wiring fits the port to the rentals store. The
// gate reads run inside the mutation conveyor's transaction, under the same
// property lock the rentals mutations take, so a rental cannot appear or
// vanish mid-check.
type RentalManagedReader interface {
	// ManagedPaymentIDs returns the subset of the given rules that a rental
	// row references — any rental state: a completed rental is final (the
	// rentals use cases reject its mutations) and its payment is final with
	// it. Ids absent from the result are simply unmanaged; an empty input
	// never reaches the query.
	ManagedPaymentIDs(ctx context.Context, scope uuid.UUID, paymentIDs []uuid.UUID) (map[uuid.UUID]bool, error)
	WithTx(tx transaction.Tx) (RentalManagedReader, error)
}

// PaymentStore is the persistence port of the payment rules and their pauses
// (ADR 0049 §4). Reads and writes are scoped by the data owner (ADR 0028) and
// the nested path payment→property is enforced in the queries themselves.
type PaymentStore interface {
	// Get loads one rule with its pause intervals; ErrNotFound when the id is
	// unknown, belongs to another owner or hangs on another property.
	Get(ctx context.Context, id, scope, propertyID uuid.UUID) (domain.Payment, error)
	// ListByProperty returns the property's rules in creation order, each
	// with its pause intervals. Search is a case-insensitive substring
	// filter on the title ('' = no filter).
	ListByProperty(ctx context.Context, scope, propertyID uuid.UUID, search string) ([]domain.Payment, error)
	// Create inserts a new rule (id, owner_id, since are app-side).
	Create(ctx context.Context, p domain.Payment) error
	// Update writes the editable fields of the rule (since never among them).
	Update(ctx context.Context, p domain.Payment) error
	// Delete removes the rule; the caller has already resolved the fate of
	// its planned operations (keep_overdue).
	Delete(ctx context.Context, id, scope uuid.UUID) error
	// InsertPause opens the open-ended pause [from, ∞).
	InsertPause(ctx context.Context, paymentID uuid.UUID, from time.Time) error
	// CloseActivePause closes the open pause with to=resumeDay — the resume
	// day is already outside the half-open interval.
	CloseActivePause(ctx context.Context, paymentID uuid.UUID, resumeDay time.Time) error
	// DeletePlannedFrom removes the rule's planned operations with date >=
	// from (deletion always drops the future planned).
	DeletePlannedFrom(ctx context.Context, paymentID uuid.UUID, from time.Time) error
	// DeletePlannedBefore removes the rule's planned operations with date <
	// before (the overdue debt; only when keep_overdue=false).
	DeletePlannedBefore(ctx context.Context, paymentID uuid.UUID, before time.Time) error
	// DeleteFuturePlanned removes the rule's strictly future planned
	// operations (date > today): the edit invalidation whose in-transaction
	// tick then stands the single future planned again with fresh snapshots.
	DeleteFuturePlanned(ctx context.Context, paymentID uuid.UUID, today time.Time) error
	// SetFavorite writes the favorite star in one atomic UPDATE (PUT favorite,
	// ticket #461 — never a read-modify-write through the full rule update).
	SetFavorite(ctx context.Context, id, scope uuid.UUID, favorite bool) error
	WithTx(tx transaction.Tx) (PaymentStore, error)
}

// PaidOverdueCount is one payment's two progress counters (ticket #845):
// the paid facts and the overdue planned rows of one rule — the read the
// Rentals list progress consumes through the RentPaymentGateway. A rule
// with no matching operations is absent from the batch — the consumer
// defaults its zeros.
type PaidOverdueCount struct {
	PaymentID    uuid.UUID
	PaidCount    int64
	OverdueCount int64
}

// NearestDateInputs is the stored half of the next payment date resolution
// (ticket #991): the earliest planned operation on or after today and the
// newest materialized date (the projection cursor). Both travel as nils
// when the rule has no such facts — the resolver falls to the projection
// and, when that has nothing either, to the true null.
type NearestDateInputs struct {
	NextPlannedDate  *time.Time
	LastMaterialized *time.Time
}

// OperationStore is the persistence port of the operations (ticket #461).
// Reads and writes are scoped by the data owner and the nested property path
// lives in the queries themselves. The mutating methods must run inside the
// transaction holding the property serialization lock.
type OperationStore interface {
	// Get loads one operation; ErrNotFound when the id is unknown, foreign or
	// hangs on another property.
	Get(ctx context.Context, id, scope, propertyID uuid.UUID) (domain.Operation, error)
	// MarkPaid flips the still-planned operation to paid with the given date;
	// rows affected = 0 surfaces as ErrAlreadyPaid.
	MarkPaid(ctx context.Context, id, scope uuid.UUID, paidDate time.Time) error
	// Cancel flips a planned or paid operation to the cancelled tombstone and
	// clears paid_date with the payment fact; rows affected = 0 — unknown id,
	// a foreign row or an already-cancelled one — surfaces as ErrNotFound
	// (cancelled operations are gone for every read).
	Cancel(ctx context.Context, id, scope, propertyID uuid.UUID) error
	// Create inserts the manual one-off operation (ticket #569): the fact is
	// born paid — status='paid' and date=paid_date=today travel app-side —
	// with no rule behind it: the query writes origin='manual' and leaves
	// payment_id NULL.
	Create(ctx context.Context, op domain.Operation) error
	// ListByPayment returns one rule's operations ordered per the query.
	ListByPayment(
		ctx context.Context, scope, propertyID, paymentID uuid.UUID, q OperationsListQuery,
	) ([]domain.Operation, error)
	// ListByProperty returns the property's operations across its rules.
	ListByProperty(ctx context.Context, scope, propertyID uuid.UUID, q OperationsListQuery) ([]domain.Operation, error)
	// SummarizeByProperty returns the property's period aggregate (ticket
	// #473): the totals by direction and the per-category breakdown, with
	// the query's status/period/direction predicate. Read-only.
	SummarizeByProperty(
		ctx context.Context, scope, propertyID uuid.UUID, q OperationsSummaryQuery,
	) (OperationsSummary, error)
	// CountPaidOperationsByPayment counts one rule's paid operations — the
	// read the Rentals progress «N из M месяцев» consumes through the
	// RentPaymentGateway (ADR 0053 §2). Cancelled tombstones never count.
	CountPaidOperationsByPayment(ctx context.Context, scope, propertyID, paymentID uuid.UUID) (int64, error)
	// CountOverdueOperationsByPayment counts one rule's overdue occurrences —
	// planned rows dated before the owner's today — the read the Rentals
	// progress' overdueMonths consumes through the RentPaymentGateway (#817).
	// Paid facts never read as overdue, cancelled tombstones never count.
	CountOverdueOperationsByPayment(
		ctx context.Context, scope, propertyID, paymentID uuid.UUID, today time.Time,
	) (int64, error)
	// CountPaidAndOverdueByPaymentIDs returns the listed rules' progress
	// counters in one batched read (ticket #845) — the same paid and
	// overdue predicates the per-payment counts encode, one GROUP BY over
	// the list where the per-rule pair took two queries. A rule with no
	// matching operations is absent from the result — the consumer defaults
	// its zeros. Read-only — never ticks.
	CountPaidAndOverdueByPaymentIDs(
		ctx context.Context, scope, propertyID uuid.UUID, paymentIDs []uuid.UUID, today time.Time,
	) ([]PaidOverdueCount, error)
	// NearestDateInputsOfPayments returns the listed rules' next-payment-date
	// aggregates in one batched read (ticket #991) — the earliest planned
	// operation on or after today and the newest materialized date, the two
	// stored facts the shared NearestDateOfPayment resolution consumes. A
	// rule with no matching operations is absent from the result — its
	// inputs are the nils and the resolution falls to the pure projection.
	// Read-only — never ticks.
	NearestDateInputsOfPayments(
		ctx context.Context, scope, propertyID uuid.UUID, today time.Time, paymentIDs []uuid.UUID,
	) (map[uuid.UUID]NearestDateInputs, error)
	// ListGlobal returns one page of the actor's visible paid operations —
	// the global «Операции» screen's merged feed (ticket #540): the paid
	// facts of the actor's own properties plus the properties they can view,
	// the archived ones excluded. The actor-scoped cross-property read: the
	// visibility predicate lives in the store's SQL (the tasks global feed
	// precedent), the propertyIds entries have already been resolved through
	// the view gate by the service. Read-only — never ticks.
	ListGlobal(ctx context.Context, actor uuid.UUID, q GlobalOperationsListQuery) ([]GlobalOperationRow, error)
	// CountGlobal counts the actor's visible paid operations over the same
	// visibility and filters as ListGlobal — the whole scope with the
	// keyset key and the window aside (ticket #599): the feed's «найдено
	// N». Read-only — never ticks.
	CountGlobal(ctx context.Context, actor uuid.UUID, q GlobalOperationsListQuery) (int64, error)
	// SummarizeGlobal returns the period aggregate of the actor's visible
	// paid operations (ticket #540) over the same visibility as ListGlobal:
	// the totals by direction and the per-category breakdown. Read-only.
	SummarizeGlobal(ctx context.Context, actor uuid.UUID, q GlobalOperationsSummaryQuery) (OperationsSummary, error)
	WithTx(tx transaction.Tx) (OperationStore, error)
}

// GlobalOperationRow is one row of the global feed's read projection (ticket
// #540): the operation plus the bound property's display name — the global
// screen's row label.
type GlobalOperationRow struct {
	Operation    domain.Operation
	PropertyName string
}

// OwnerSnapshot is the tick's read side in one interface fact: the owner's
// payment rules (with pause intervals attached) and the dedup keys of their
// existing operations. How many queries build it is the adapter's business.
type OwnerSnapshot struct {
	Payments []domain.Payment
	// Statuses keys the listed payments' operations by payment and date:
	// existence is the dedup key, status drives the future-planned rebuild.
	Statuses map[uuid.UUID]map[time.Time]domain.OperationStatus
}

// TickStore is the persistence port of the materialization tick (ADR 0049 §3):
// the serialization lock, the owner snapshot, and the application of a tick
// plan. Every method must run inside the tick's unit of work.
type TickStore interface {
	// LockOwnerProperties takes the tick's serialization point: FOR UPDATE on
	// the owner's active/maintenance property rows (ADR 0049 §3). Context
	// mutations lock the same rows per property, so update-vs-tick,
	// archive-vs-tick and delete-vs-tick serialize on one point.
	LockOwnerProperties(ctx context.Context, ownerID uuid.UUID) error
	// LoadOwnerSnapshot returns the owner's payment rules on non-archived
	// properties with their pauses and existing operation statuses.
	LoadOwnerSnapshot(ctx context.Context, ownerID uuid.UUID) (OwnerSnapshot, error)
	// ApplyTickPlan applies one rule's plan inside the caller's transaction:
	// the idempotent occurrence inserts, the auto-pay day payment (strictly
	// today, ADR 0049 §2) and the future-planned rebuild (keep the single
	// allowed date, or remove them all when the plan keeps none). Ordering
	// and the keep-or-none duality live here, not in the caller.
	ApplyTickPlan(ctx context.Context, p domain.Payment, today time.Time, plan domain.PaymentTickPlan) error
	WithTx(tx transaction.Tx) (TickStore, error)
}

// OwnerCalendar gives the data owner's calendar date (ADR 0048): "today" as
// the date in the property owner's timezone — users.timezone, NOT NULL with
// the Europe/Moscow default, IANA-validated on write — normalized to the
// domain's UTC-midnight convention. The normalization order (year/month/day
// read in the owner's location, rebuilt at UTC midnight) is this module's
// implementation; taking the UTC date of the instant instead compares
// wrongly against DATE columns. A timezone change never rewrites history —
// it only shifts future day boundaries.
type OwnerCalendar interface {
	Today(ctx context.Context, ownerID uuid.UUID) (time.Time, error)
}

// DateAtUTCMidnight is the module's date convention (ADR 0048 p.2): read
// the instant's calendar date in loc, then rebuild it at UTC midnight, so
// the result compares correctly against the UTC-midnight dates stored in
// DATE columns. The single home of the normalization both calendar paths —
// the owner lookup and the zone sweep — share.
func DateAtUTCMidnight(t time.Time, loc *time.Location) time.Time {
	zoned := t.In(loc)
	return time.Date(zoned.Year(), zoned.Month(), zoned.Day(), 0, 0, 0, 0, time.UTC)
}

// TickZone is one work item of the tick's hourly zone sweep (ADR 0048 p.3):
// a distinct owner timezone and the data owners in it that have payment rules
// on active/maintenance properties. One "today" is computed per zone and
// every owner of the zone is materialized on it.
type TickZone struct {
	Timezone string
	Owners   []uuid.UUID
}

// TickZoneDirectory lists the hourly sweep targets of the materialization
// tick (ADR 0048 p.3). Stateless by design: every run re-lists the zones —
// idempotent materialization makes the midnights-between runs no-ops, so no
// per-zone or per-owner tick state is kept.
type TickZoneDirectory interface {
	ListTickZones(ctx context.Context) ([]TickZone, error)
}

// GlobalPaymentRulesQuery is the global payment rules read's request
// (ticket #575): the search query plus the default-catalog slugs the
// application layer expanded it into (the catalog's labels live in code,
// not in the database — the store matches them as a set).
type GlobalPaymentRulesQuery struct {
	Search        string
	CategorySlugs []string
	// The search screen's server-side chip filter (map #573 rework): the
	// chip's identity — the default catalog's slug or the user category's
	// id as text — plus the direction. '' = no filter. It narrows the rules
	// list only; the matched-categories sum ignores it (the chips always
	// show the query's every matched category — the operations' canon).
	Category string
	Type     domain.PaymentType
	// The search screen's page (50 per page, infinite scroll). The zero
	// limit means no window — the whole matched scope, the feed's and the
	// stacks' reads.
	Limit int32
	// The keyset continuation (ticket #597): resume strictly after the
	// (CreatedAt, ID) row — the previous page's last one. Both nil = the
	// window starts at the beginning; the pair travels together.
	AfterCreatedAt *time.Time
	AfterID        *uuid.UUID
}

// GlobalPaymentRuleRow is one raw row of the global payment rules read
// (ticket #575): the rule's card fields plus the bound property's display
// name, the owner's today the row's schedule math ran against (ADR 0048)
// and the stored-operation aggregates. The application layer turns it into
// a GlobalPaymentItem — the nearest-date fallback and the overdue age are
// its business.
type GlobalPaymentRuleRow struct {
	ID            uuid.UUID
	OwnerID       uuid.UUID
	PropertyID    uuid.UUID
	PropertyName  string
	Type          domain.PaymentType
	Title         string
	AmountKopecks int64
	AutoPay       bool
	IsFavorite    bool
	Category      domain.CategoryRef
	// CreatedAt is the rule's creation moment — the feed's first sort key
	// and the page cursor's anchor (ticket #597).
	CreatedAt time.Time
	Today     time.Time
	// NextPlannedDate is the earliest stored planned operation on or after
	// today; nil hands the nearest date to the projection fallback.
	NextPlannedDate *time.Time
	OverdueCount    int64
	// OldestOverdueDate is the oldest overdue operation's date; nil unless
	// the rule has overdue operations.
	OldestOverdueDate *time.Time
	// OldestOverdueOperationID is that oldest operation's id — the overdue
	// card's link target on the global main screen (ticket #578); nil
	// unless the rule has overdue operations.
	OldestOverdueOperationID *uuid.UUID
	// FavoriteOrder is the rule's manual favorite order — the 1-based
	// position the favorites edit mode's save assigned (ticket #576); nil
	// when the rule has never been in a saved order (new and legacy
	// favorites sort last, the new-favorite-at-the-end rule).
	FavoriteOrder *int64
}

// GlobalPaymentCounters are the main screen's two scope counters (ticket
// #575): the «Все избранные (N)» and «Все просроченные (N)» cards. They
// describe the whole visible scope — a search never narrows them.
type GlobalPaymentCounters struct {
	FavoriteCount          int64
	OverdueOperationsCount int64
}

// GlobalPaymentObject is one visible non-archived property of the «Объекты»
// read (ticket #575); the service groups the feed's rows onto it. PinnedAt is
// the property's global pin (ticket #577): nil — not pinned, a moment —
// pinned since then; the SQL order carries the pinned first. PhotoURL is the
// card avatar's photo — the object's first (oldest) one (ticket #582); nil
// when the object has no photos.
type GlobalPaymentObject struct {
	PropertyID uuid.UUID
	Name       string
	Address    string
	PinnedAt   *time.Time
	PhotoURL   *string
}

// GlobalPaymentReader is the persistence port of the global payment rules
// read side (ticket #575). The visibility predicate lives in the store's
// SQL — the actor-scoped cross-property read (the tasks global feed's
// rule, ticket #521); the owner→today map is resolved by the application
// layer (the owner calendar, ADR 0048) and threaded in for every per-row
// schedule computation. Read-only: none of it ticks.
type GlobalPaymentReader interface {
	// ListGlobalPaymentOwnerTodays returns the distinct data owners of the
	// actor's visible non-archived properties — the keys of the owner→today
	// map.
	ListGlobalPaymentOwnerTodays(ctx context.Context, actor uuid.UUID) ([]uuid.UUID, error)
	// ListGlobalPaymentRules returns the actor's visible merged feed rows
	// under the query's search; every row carries its owner's today.
	ListGlobalPaymentRules(
		ctx context.Context, actor uuid.UUID, todays map[uuid.UUID]time.Time, q GlobalPaymentRulesQuery,
	) ([]GlobalPaymentRuleRow, error)
	// SumGlobalPaymentCounters returns the scope counters over the whole
	// visible feed, the search never narrowing them.
	SumGlobalPaymentCounters(
		ctx context.Context, actor uuid.UUID, todays map[uuid.UUID]time.Time,
	) (GlobalPaymentCounters, error)
	// SumGlobalPaymentSearchCategories returns the matched categories of the
	// search — the chips' category identities, one per category (ticket
	// #602), the largest match count first.
	SumGlobalPaymentSearchCategories(
		ctx context.Context, actor uuid.UUID, q GlobalPaymentRulesQuery,
	) ([]domain.CategoryRef, error)
	// CountGlobalPaymentRules counts the search's whole-scope matches under
	// the query's search and chip filters — the list's predicate with the
	// window and the keyset key aside (ticket #599): the search screen's
	// «найдено N», the same on every walked page.
	CountGlobalPaymentRules(
		ctx context.Context, actor uuid.UUID, todays map[uuid.UUID]time.Time, q GlobalPaymentRulesQuery,
	) (int64, error)
	// ListGlobalPaymentObjects returns the actor's visible non-archived
	// properties under the object search — the «Объекты» screen's cards
	// without their stacks.
	ListGlobalPaymentObjects(ctx context.Context, actor uuid.UUID, search string) ([]GlobalPaymentObject, error)
	// LastOperationDatesOfPayments returns the newest materialized date
	// across planned and paid per listed rule — the projection cursor of the
	// nearest-date fallback. Ids not present have no materialized facts.
	LastOperationDatesOfPayments(ctx context.Context, paymentIDs []uuid.UUID) (map[uuid.UUID]time.Time, error)
	// GetGlobalPaymentRule loads one rule with its pause intervals — the
	// projection fallback's input. The boolean is false when the rule is
	// gone (a deletion mid-read leaves the row's nearest date null).
	GetGlobalPaymentRule(ctx context.Context, ownerID, propertyID, paymentID uuid.UUID) (domain.Payment, bool, error)
}

// GlobalPaymentFavoriteLock is one locked row of the favorites order save
// (ticket #576): the rule's id, its favorite flag at lock time and the
// actor's role on the rule's property — the write gate's raw material (the
// favorite star's matrix, #461: Full Access+).
type GlobalPaymentFavoriteLock struct {
	ID uuid.UUID
	// PropertyID is the rule's object — the realtime carrier's frame anchor
	// (карта #714, #716): the order save dirties the `payments` view of
	// every touched rule's property.
	PropertyID uuid.UUID
	IsFavorite bool
	Role       sharedpolicy.Role
}

// GlobalPaymentOrderStore is the write side of the favorites manual order
// (ticket #576) — the global payments' first cross-property mutation. The
// visibility predicate and the per-row role live in the lock's SQL (the
// global reads' rule, ticket #521): a submitted id the actor cannot see
// never returns from the lock, which keeps the foreign case the
// privacy-preserving 404; the write capability itself is the favorite
// star's (#461, Full Access+) and is resolved per row beside the data.
// Every method runs inside the caller's transaction.
type GlobalPaymentOrderStore interface {
	// LockVisibleFavorites takes FOR UPDATE row locks on the submitted
	// rules — in the given order, the caller's sorted ids being the
	// deadlock-safety — and returns the rows visible to the actor on
	// non-archived properties with the actor's per-row role. A matched set
	// smaller than the submission means an unknown, foreign or invisible
	// id.
	LockVisibleFavorites(ctx context.Context, actor uuid.UUID, ids []uuid.UUID) ([]GlobalPaymentFavoriteLock, error)
	// SaveFavoritePosition writes one rule's 1-based order position and
	// must affect exactly one row: the lock has already proven existence
	// and visibility inside the same transaction.
	SaveFavoritePosition(ctx context.Context, id uuid.UUID, position int64) error
	// ClearFavoriteOrdersOutside drops the positions of the actor's visible
	// favorites outside the submitted list (the save's full-replacement
	// pass: what the save does not place falls to the end of the reading
	// order). KeepIDs travels as the list of placed ids, '' meaning none.
	ClearFavoriteOrdersOutside(ctx context.Context, actor uuid.UUID, keepIDs []uuid.UUID) error
	WithTx(tx transaction.Tx) (GlobalPaymentOrderStore, error)
}
