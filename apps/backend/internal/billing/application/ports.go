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

// SubscriptionRepository is the persistence port for subscriptions. The
// ForUpdate variants acquire a row-level pessimistic lock and must only be
// called inside a transaction. The worker listings are plain selections; each
// processing transaction re-reads and locks the row, so state checked between
// listing and processing is never trusted blindly.
type SubscriptionRepository interface {
	GetByUserID(ctx context.Context, userID uuid.UUID) (domain.Subscription, error)
	GetByUserIDForUpdate(ctx context.Context, userID uuid.UUID) (domain.Subscription, error)
	// ListUpForRenewal returns a batch of active auto-renewing subscriptions
	// whose paid period has ended (the renewal-charge selection, issue #252).
	ListUpForRenewal(ctx context.Context, now time.Time, limit int) ([]domain.Subscription, error)
	// ListInExpiredGrace returns a batch of grace subscriptions whose grace
	// window has ended (the downgrade-to-basic selection, issue #252).
	ListInExpiredGrace(ctx context.Context, now time.Time, limit int) ([]domain.Subscription, error)
	// ListInGraceReminderWindow returns a batch of grace subscriptions inside
	// the grace-expiry reminder window — valid_until is still in the future
	// but arrives within the lead duration — whose window has not been
	// reminded yet (issue #253).
	ListInGraceReminderWindow(ctx context.Context, now time.Time, lead time.Duration, limit int) ([]domain.Subscription, error)
	// ListExpiredNonRenewing returns a batch of active subscriptions with
	// auto-renew off whose retained period has ended (issue #252).
	ListExpiredNonRenewing(ctx context.Context, now time.Time, limit int) ([]domain.Subscription, error)
	// ListExpiredCancelled returns a batch of cancelled subscriptions whose
	// retained period has ended (issue #252).
	ListExpiredCancelled(ctx context.Context, now time.Time, limit int) ([]domain.Subscription, error)
	// ListPendingChanges returns a batch of active subscriptions with a
	// deferred tariff change that is due (issue #252).
	ListPendingChanges(ctx context.Context, now time.Time, limit int) ([]domain.Subscription, error)
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
	// ListStalePending returns a batch of pending payments with a provider
	// reference that have been pending longer than the reconciliation
	// staleness — the lost-webhook set the reconciliation worker re-checks with
	// the provider (issue #252).
	ListStalePending(ctx context.Context, createdBefore time.Time, limit int) ([]domain.SubscriptionPayment, error)
	// ListStalePendingUpgrades narrows the stale-pending set to payments whose
	// target tariff differs from the subscription's current one — the
	// tariff-change payments of the ChangeTariff flow (issue #252).
	ListStalePendingUpgrades(ctx context.Context, createdBefore time.Time, limit int) ([]domain.SubscriptionPayment, error)
	// ListStaleRefunding returns a batch of payments stuck in the refunding
	// reservation longer than the reconciliation staleness — the lost-outcome
	// set the refund reconciliation worker re-checks with the provider
	// (issue #254).
	ListStaleRefunding(ctx context.Context, updatedBefore time.Time, limit int) ([]domain.SubscriptionPayment, error)
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
