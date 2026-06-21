package application

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

var (
	ErrNotFound                   = errors.New("not found")
	ErrPaymentMethodInUse         = errors.New("payment method is in use")
	ErrPaymentMethodAlreadyExists = errors.New("payment method already exists")
	ErrTariffNotFound             = errors.New("tariff not found")
	ErrSubscriptionNotFound       = errors.New("subscription not found")
	ErrPaymentMethodNotFound      = errors.New("payment method not found")
	ErrPaymentNotFound            = errors.New("payment not found")
	ErrAlreadyOnTariff            = domain.ErrAlreadyOnTariff
	ErrInvalidTariffChange        = domain.ErrInvalidTariffChange
	ErrProviderNotConfirmable     = errors.New("provider does not support confirmation")
)

type TariffRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (domain.Tariff, error)
	GetByName(ctx context.Context, name domain.TariffName) (domain.Tariff, error)
	List(ctx context.Context) ([]domain.Tariff, error)
	WithTx(tx transaction.Tx) TariffRepository
}

type SubscriptionRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (domain.Subscription, error)
	GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Subscription, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) (domain.Subscription, error)
	GetByUserIDForUpdate(ctx context.Context, userID uuid.UUID) (domain.Subscription, error)
	Create(ctx context.Context, sub domain.Subscription) (domain.Subscription, error)
	Update(ctx context.Context, sub domain.Subscription) error
	ListUpForRenewal(ctx context.Context, now time.Time, limit int32) ([]domain.Subscription, error)
	ListInExpiredGrace(ctx context.Context, now time.Time, limit int32) ([]domain.Subscription, error)
	ListExpiredNonRenewing(ctx context.Context, now time.Time, limit int32) ([]domain.Subscription, error)
	ListExpiredCancelled(ctx context.Context, now time.Time, limit int32) ([]domain.Subscription, error)
	ListPendingChanges(ctx context.Context, now time.Time, limit int32) ([]domain.Subscription, error)
	WithTx(tx transaction.Tx) SubscriptionRepository
}

type PaymentMethodRepository interface {
	Create(ctx context.Context, pm domain.PaymentMethod) (domain.PaymentMethod, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.PaymentMethod, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error)
	SetActive(ctx context.Context, userID, methodID uuid.UUID) error
	Delete(ctx context.Context, userID, methodID uuid.UUID) error
	WithTx(tx transaction.Tx) PaymentMethodRepository
}

type SubscriptionPaymentRepository interface {
	Create(ctx context.Context, payment domain.SubscriptionPayment) (domain.SubscriptionPayment, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.SubscriptionPayment, error)
	GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.SubscriptionPayment, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.SubscriptionPayment, error)
	ListPendingSubscriptionPaymentsByUserID(ctx context.Context, userID uuid.UUID) ([]domain.SubscriptionPayment, error)
	GetLastSucceededBySubscriptionID(ctx context.Context, subscriptionID uuid.UUID) (domain.SubscriptionPayment, error)
	MarkSucceeded(ctx context.Context, id uuid.UUID, now time.Time) error
	MarkFailed(ctx context.Context, id uuid.UUID, errorCode *string, now time.Time) error
	UpdateProviderPaymentID(ctx context.Context, id uuid.UUID, providerPaymentID string) (domain.SubscriptionPayment, error)
	UpdatePaymentMethodAndProviderID(ctx context.Context, id, paymentMethodID uuid.UUID, providerPaymentID string) (domain.SubscriptionPayment, error)
	WithTx(tx transaction.Tx) SubscriptionPaymentRepository
}

// PropertyArchiver archives properties when a subscription is downgraded to a
// lower-limit tariff. It is implemented by the properties application service.
type PropertyArchiver interface {
	ArchiveExcessProperties(ctx context.Context, tx transaction.Tx, ownerID uuid.UUID, limit int) error
}

// ConfirmableProvider is implemented by providers that support an explicit
// confirmation step. In the MVP only the fake provider does.
type ConfirmableProvider interface {
	ConfirmPayment(ctx context.Context, internalPaymentID string) (WebhookPayload, error)
}

// PaymentURLProvider is implemented by providers that can reconstruct a payment
// confirmation URL from an internal payment id. This lets the API return a
// confirmation link for an already-pending payment.
type PaymentURLProvider interface {
	PaymentURL(ctx context.Context, paymentID uuid.UUID) (string, error)
}

// Provider abstracts the external payment processor used for subscription payments.
type Provider interface {
	Name() domain.PaymentProvider
	Init(ctx context.Context, req InitRequest) (InitResult, error)
	Charge(ctx context.Context, req ChargeRequest) (ChargeResult, error)
	Status(ctx context.Context, paymentID uuid.UUID, providerPaymentID string) (domain.PaymentStatus, error)
	ParseWebhook(ctx context.Context, payload []byte) (WebhookPayload, error)
}

type InitRequest struct {
	PaymentID     uuid.UUID
	AmountKopecks int64
	Period        domain.SubscriptionPeriod
	UserID        uuid.UUID
	Description   string
	ReturnURL     string
}

type InitResult struct {
	ProviderPaymentID string
	PaymentURL        string
	SavedToken        string // token to save as PaymentMethod
	Status            domain.PaymentStatus
}

type ChargeRequest struct {
	PaymentID     uuid.UUID
	AmountKopecks int64
	Token         string
}

type ChargeResult struct {
	ProviderPaymentID string
	Status            domain.PaymentStatus
}

type WebhookPayload struct {
	ProviderPaymentID string
	InternalPaymentID uuid.UUID
	Status            domain.PaymentStatus
	ErrorCode         *string
}
