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
)

// PropertyRef is the payments view of the property a use case targets: the
// data owner (the SQL scope, ADR 0028) and the archived flag (the financial
// read-only state, ticket #446). The properties context owns the entity;
// payments never needs its rest.
type PropertyRef struct {
	OwnerID  uuid.UUID
	Archived bool
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
	// ListGlobal returns one page of the actor's visible paid operations —
	// the global «Операции» screen's merged feed (ticket #540): the paid
	// facts of the actor's own properties plus the properties they can view,
	// the archived ones excluded. The actor-scoped cross-property read: the
	// visibility predicate lives in the store's SQL (the tasks global feed
	// precedent), the propertyIds entries have already been resolved through
	// the view gate by the service. Read-only — never ticks.
	ListGlobal(ctx context.Context, actor uuid.UUID, q GlobalOperationsListQuery) ([]GlobalOperationRow, error)
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
