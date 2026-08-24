// Package application holds the billing use cases and ports: the subscription lifecycle with its worker phases,
// payment, tariff and payment-method services, onboarding and grace-event publishing.
package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// TariffRepository is the persistence port for tariffs. List returns only
// active (user-visible) tariffs; ListAll returns every tariff including hidden
// ones for the admin views. GetByID and GetByName resolve any tariff, including
// hidden ones, because foreign keys keep referencing them (issue #245).
//
// The write methods back the admin tariff management (issue #256): Create
// narrows a name collision to ErrAlreadyExists, Update misses to ErrNotFound.
// The postgres adapter serves reads from an in-memory cache with a TTL, and
// writes go through a transaction-bound instance with a cache of its own, so
// after a committed write the caller must Invalidate the shared instance to
// make the change visible to its readers immediately.
type TariffRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (domain.Tariff, error)
	GetByName(ctx context.Context, name domain.TariffName) (domain.Tariff, error)
	List(ctx context.Context) ([]domain.Tariff, error)
	ListAll(ctx context.Context) ([]domain.Tariff, error)
	Create(ctx context.Context, tariff domain.Tariff) (domain.Tariff, error)
	Update(ctx context.Context, tariff domain.Tariff) (domain.Tariff, error)
	// Invalidate drops this instance's cached reads so the next one observes
	// committed writes from transaction-bound instances.
	Invalidate(ctx context.Context) error
	WithTx(tx transaction.Tx) (TariffRepository, error)
}

// SubscriptionSelection is the parameterized worker batch of the subscription
// phases (issue #286): the values a phase selects its batch by. The matching
// predicate — how these values pick rows — lives once in the SQL adapter; the
// application never re-states it. A phase describes its selection in one
// constructor next to the phase and uses it for both the batch listing and the
// under-lock re-check, so a new phase is new values, not a new port method.
type SubscriptionSelection struct {
	// UserID narrows the selection to one user's subscription — the shape of
	// the under-lock re-check after the phase locked the listed row. Nil lists
	// the whole batch.
	UserID *uuid.UUID
	// Status is required: every phase selects exactly one lifecycle status.
	Status domain.SubscriptionStatus
	// AutoRenewEnabled narrows to subscriptions with the flag set either way;
	// nil leaves both.
	AutoRenewEnabled *bool
	// ValidUntilBefore selects subscriptions whose paid or grace window has
	// ended by the instant (valid_until <= t); ValidUntilAfter keeps those
	// whose window is still open past it (valid_until > t). Nil drops the
	// bound; a subscription without validity never matches a bounded
	// selection.
	ValidUntilBefore *time.Time
	ValidUntilAfter  *time.Time
	// Unreminded keeps only grace windows whose expiry reminder was not
	// dispatched yet (grace_reminded_at IS NULL, issue #253).
	Unreminded bool
	// PendingChangeDue selects subscriptions whose deferred tariff change is
	// due (pending_change_at <= t); nil drops the condition.
	PendingChangeDue *time.Time
	// GraceRetryDue selects grace subscriptions with a due dunning retry
	// (ticket #431): the latest grace entry is old enough for the next
	// scheduled retry (+24 h or +72 h, consumed by any payment since the
	// entry), the grace window is still open and an active payment method is
	// linked. The value is the tick's now; nil drops the condition. The
	// predicate behind it — the latest reason='grace_entered' transition as
	// the anchor — lives once in the SQL adapter.
	GraceRetryDue *time.Time
	// Limit caps the batch; the worker loop re-lists until the selection is
	// exhausted.
	Limit int
}

// SubscriptionRepository is the persistence port for subscriptions. The
// ForUpdate variant acquires a row-level pessimistic lock and must only be
// called inside a transaction. List is the plain selection the worker phases
// list and re-check their batches through; each processing transaction
// re-reads and locks the row, so state checked between listing and processing
// is never trusted blindly.
type SubscriptionRepository interface {
	GetByUserID(ctx context.Context, userID uuid.UUID) (domain.Subscription, error)
	GetByUserIDForUpdate(ctx context.Context, userID uuid.UUID) (domain.Subscription, error)
	// List returns a batch of subscriptions matching the worker selection
	// (issue #286) — the single parameterized query behind every phase batch
	// of the ADR 0008 lifecycle.
	List(ctx context.Context, sel SubscriptionSelection) ([]domain.Subscription, error)
	Create(ctx context.Context, sub domain.Subscription) (domain.Subscription, error)
	Update(ctx context.Context, sub domain.Subscription) error
	WithTx(tx transaction.Tx) (SubscriptionRepository, error)
}

// SubscriptionTransitionRepository appends to and reads the immutable
// subscription transition log (issue #245).
type SubscriptionTransitionRepository interface {
	Append(ctx context.Context, transition domain.Transition) error
	ListBySubscriptionID(ctx context.Context, subscriptionID uuid.UUID) ([]domain.Transition, error)
	WithTx(tx transaction.Tx) (SubscriptionTransitionRepository, error)
}

// PaymentSelection is the parameterized worker batch of the payment
// reconciliation phases (issue #286). Every selection requires a provider
// reference — without one there is nothing to reconcile with the provider —
// and staleness is measured on exactly one clock: created (how long the
// payment sits unresolved) or updated (how long it sits stuck), whichever the
// phase sets. The matching predicate lives once in the SQL adapter.
type PaymentSelection struct {
	// Status is required: every phase selects exactly one payment status.
	Status domain.PaymentStatus
	// CreatedBefore selects payments sitting unresolved since before the
	// instant (created_at < t); UpdatedBefore selects payments not updated
	// since before it (updated_at < t). A phase sets exactly one; both nil
	// drop the staleness bound.
	CreatedBefore *time.Time
	UpdatedBefore *time.Time
	// TariffChangeOnly narrows to payments whose target tariff differs from
	// the subscription's current one — the tariff-change payments of the
	// ChangeTariff flow whose lost webhook leaves the paid change unapplied.
	TariffChangeOnly bool
	// Limit caps the batch; the worker loop re-lists until the selection is
	// exhausted.
	Limit int
}

// SubscriptionPaymentRepository is the persistence port for subscription
// payments (issue #250). The ForUpdate variant acquires a row-level
// pessimistic lock and must only be called inside a transaction. Create
// returns ErrAlreadyExists when the partial unique index over pending payments
// rejects a duplicate initiation for the same user/tariff/period — the durable
// backstop the payment flow turns into "return the existing pending payment".
type SubscriptionPaymentRepository interface {
	Create(ctx context.Context, payment domain.SubscriptionPayment) (domain.SubscriptionPayment, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.SubscriptionPayment, error)
	GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.SubscriptionPayment, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.SubscriptionPayment, error)
	ListPendingByUserID(ctx context.Context, userID uuid.UUID) ([]domain.SubscriptionPayment, error)
	// List returns a batch of payments matching the worker selection (issue
	// #286) — the single parameterized query behind every reconciliation
	// batch.
	List(ctx context.Context, sel PaymentSelection) ([]domain.SubscriptionPayment, error)
	// Count returns how many payments match the worker selection ignoring its
	// limit — the stuck-payment gauges of the hygiene phase (ticket #433)
	// count the whole batch, not one page of it.
	Count(ctx context.Context, sel PaymentSelection) (int64, error)
	Update(ctx context.Context, payment domain.SubscriptionPayment) error
	WithTx(tx transaction.Tx) (SubscriptionPaymentRepository, error)
}

// PaymentMethodRepository is the persistence port for payment methods
// (issue #251). The ForUpdate variant acquires a row-level pessimistic lock
// and must only be called inside a transaction. Sensitive columns (charge
// token, provider card id, expiry) are encrypted at rest by the adapter;
// duplicate cards converge on the (user, token hash) row instead of
// duplicating.
type PaymentMethodRepository interface {
	// UpsertByTokenHash inserts the method or converges on the existing row
	// with the same user and token hash: repeated deliveries of one binding
	// resolve to a single row and refresh its display fields.
	UpsertByTokenHash(ctx context.Context, method domain.PaymentMethod) (domain.PaymentMethod, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.PaymentMethod, error)
	GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.PaymentMethod, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error)
	// SetActive makes the method the user's single active one, deactivating
	// every other method of the user in the same call. Must run inside a
	// transaction.
	SetActive(ctx context.Context, userID, methodID uuid.UUID) error
	// Delete removes the method; an active-payment-method FK still referencing
	// it surfaces as ErrPaymentMethodInUse (the durable backstop of the
	// "active method cannot be deleted" rule).
	Delete(ctx context.Context, userID, methodID uuid.UUID) error
	WithTx(tx transaction.Tx) (PaymentMethodRepository, error)
}

// CardBindingSelection is the parameterized batch of the binding-session
// hygiene phase (ticket #433): the values the cleanup selects its batch by,
// shaped after the subscription and payment selections (issue #286) so the
// predicate lives once in SQL.
type CardBindingSelection struct {
	// ExpiredBefore selects sessions whose lifetime has ended by the instant
	// (expires_at < t), whatever their status: an expired session never
	// produces a payment method.
	ExpiredBefore time.Time
	// Limit caps the batch; the hygiene loop re-runs the delete until the
	// selection is exhausted.
	Limit int
}

// CardBindingSessionRepository is the persistence port for card-binding
// sessions (issue #251). The ForUpdate variant acquires a row-level
// pessimistic lock and must only be called inside a transaction.
type CardBindingSessionRepository interface {
	Create(ctx context.Context, session domain.CardBindingSession) (domain.CardBindingSession, error)
	// GetByRequestKeyForUpdate resolves the add-card notification to its
	// session under the row lock; misses narrow to ErrNotFound.
	GetByRequestKeyForUpdate(ctx context.Context, provider domain.PaymentProvider, requestKey string) (domain.CardBindingSession, error)
	// ListOpenByUserID returns the user's sessions still awaiting an outcome,
	// newest first — the polling set of the sync flow.
	ListOpenByUserID(ctx context.Context, userID uuid.UUID) ([]domain.CardBindingSession, error)
	// CountStartedSince returns how many binding sessions the user started at
	// or after the instant — the sliding window of the per-user binding limit
	// (ticket #427). Every started session counts, whatever its later
	// outcome: a closed session has still consumed the window.
	CountStartedSince(ctx context.Context, userID uuid.UUID, since time.Time) (int, error)
	// DeleteExpired removes a batch of sessions past their lifetime (ticket
	// #433) and returns how many rows it deleted. Repeated runs are safe: the
	// batch shrinks to zero once the table is clean.
	DeleteExpired(ctx context.Context, sel CardBindingSelection) (int, error)
	UpdateStatus(ctx context.Context, session domain.CardBindingSession) error
	WithTx(tx transaction.Tx) (CardBindingSessionRepository, error)
}

// AdminPaymentListing is the cross-context read port behind the admin payment
// views (issue #254): payments of any user joined with the payer's phone. The
// postgres payment repository implements it; it lives here, next to its
// consumer (ADR 0035), because it reads identity data (users.phone) the
// billing payment port deliberately does not model.
type AdminPaymentListing interface {
	ListAdminPayments(ctx context.Context, filters AdminPaymentFilters) ([]AdminPaymentRow, int64, error)
	GetAdminPayment(ctx context.Context, paymentID uuid.UUID) (AdminPaymentRow, error)
}
