package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// TariffRepository is the persistence port for tariffs. List returns only
// active (user-visible) tariffs; ListAll returns every tariff including hidden
// ones for the admin views. GetByID and GetByName resolve any tariff, including
// hidden ones, because foreign keys keep referencing them (issue #245).
type TariffRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (domain.Tariff, error)
	GetByName(ctx context.Context, name domain.TariffName) (domain.Tariff, error)
	List(ctx context.Context) ([]domain.Tariff, error)
	ListAll(ctx context.Context) ([]domain.Tariff, error)
	WithTx(tx transaction.Tx) (TariffRepository, error)
}

// SubscriptionRepository is the persistence port for subscriptions. The
// ForUpdate variants acquire a row-level pessimistic lock and must only be
// called inside a transaction.
type SubscriptionRepository interface {
	GetByUserID(ctx context.Context, userID uuid.UUID) (domain.Subscription, error)
	GetByUserIDForUpdate(ctx context.Context, userID uuid.UUID) (domain.Subscription, error)
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
