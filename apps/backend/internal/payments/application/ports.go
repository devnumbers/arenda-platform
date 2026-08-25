// Package application holds the payments use cases and ports: the
// materialization tick run (ADR 0049 §3) with its persistence port and the
// owner-timezone resolver (ADR 0048). Payment CRUD, pause/resume and the
// pay-now use cases arrive with their tickets (#457, #461).
package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// TickStore is the persistence port of the materialization tick (ADR 0049 §3):
// the load side (the owner's payment rules with pauses, the existing
// operation dedup keys) and the write side (idempotent inserts, the auto-pay
// day payment, the future-planned rebuild). Every method must run inside the
// tick's unit of work.
type TickStore interface {
	// LockOwnerProperties takes the tick's serialization point: FOR UPDATE on
	// the owner's active/maintenance property rows (ADR 0049 §3). Context
	// mutations lock the same rows per property, so update-vs-tick,
	// archive-vs-tick and delete-vs-tick serialize on one point.
	LockOwnerProperties(ctx context.Context, ownerID uuid.UUID) error
	// LoadOwnerPayments returns the owner's payment rules on non-archived
	// properties, with pause intervals attached.
	LoadOwnerPayments(ctx context.Context, ownerID uuid.UUID) ([]domain.Payment, error)
	// ListOperationStatuses returns the listed payments' operations keyed by
	// payment and date: existence is the dedup key, status drives the
	// future-planned rebuild.
	ListOperationStatuses(
		ctx context.Context, paymentIDs []uuid.UUID,
	) (map[uuid.UUID]map[time.Time]domain.OperationStatus, error)
	// InsertOperation inserts a materialized occurrence; the partial unique
	// (payment_id, date) makes it idempotent (ON CONFLICT DO NOTHING).
	InsertOperation(ctx context.Context, op domain.Operation) error
	// PayDueToday closes the rule's planned occurrence dated today
	// (paid_date = today) — the auto-pay day payment, strictly no backdating
	// (ADR 0049 §2).
	PayDueToday(ctx context.Context, paymentID uuid.UUID, today time.Time) error
	// DeleteFuturePlannedExcept removes the rule's future planned operations
	// except the single allowed date.
	DeleteFuturePlannedExcept(ctx context.Context, paymentID uuid.UUID, today, keep time.Time) error
	// DeleteFuturePlannedAll removes every future planned operation of the
	// rule (paused or ended — no future planned may remain).
	DeleteFuturePlannedAll(ctx context.Context, paymentID uuid.UUID, today time.Time) error
	WithTx(tx transaction.Tx) (TickStore, error)
}

// OwnerTimezoneResolver resolves the data owner's timezone (ADR 0048): the
// tick's "today" is the calendar date in the property owner's timezone —
// users.timezone, NOT NULL with the Europe/Moscow default, IANA-validated on
// write. A timezone change never rewrites history; it only shifts future
// day boundaries.
type OwnerTimezoneResolver interface {
	OwnerTimezone(ctx context.Context, ownerID uuid.UUID) (*time.Location, error)
}
