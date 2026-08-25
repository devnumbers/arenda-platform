// Package application holds the payments use cases and ports: the
// materialization tick run (ADR 0049 §3) with its persistence port and the
// owner calendar (ADR 0048). Payment CRUD, pause/resume and the pay-now use
// cases arrive with their tickets (#457, #461).
package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

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
