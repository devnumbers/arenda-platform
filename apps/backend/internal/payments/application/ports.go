// Package application holds the payments use cases and ports: the
// materialization tick run (ADR 0049 §3) with its persistence port, the owner
// calendar (ADR 0048), and the payment CRUD store with the property
// serialization port (ticket #457). The pay-now use case arrives with its
// ticket (#461).
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
	// with its pause intervals.
	ListByProperty(ctx context.Context, scope, propertyID uuid.UUID) ([]domain.Payment, error)
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
	WithTx(tx transaction.Tx) (PaymentStore, error)
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
