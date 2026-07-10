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
	ErrAlreadyExists              = errors.New("already exists")
	ErrPaymentMethodInUse         = errors.New("payment method is in use")
	ErrPaymentMethodAlreadyExists = errors.New("payment method already exists")
	ErrTariffNotFound             = errors.New("tariff not found")
	ErrSubscriptionNotFound       = errors.New("subscription not found")
	ErrPaymentMethodNotFound      = errors.New("payment method not found")
	ErrPaymentNotFound            = errors.New("payment not found")
	ErrAlreadyOnTariff            = domain.ErrAlreadyOnTariff
	ErrInvalidTariffChange        = domain.ErrInvalidTariffChange
	ErrProviderNotConfirmable     = errors.New("provider does not support confirmation")
	// ErrProviderCardNotFound is returned when a provider reports that the card
	// has already been removed or does not exist.
	ErrProviderCardNotFound = errors.New("provider card not found")
	ErrInvalidFilter        = errors.New("invalid filter")
)

type TariffRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (domain.Tariff, error)
	GetByName(ctx context.Context, name domain.TariffName) (domain.Tariff, error)
	List(ctx context.Context) ([]domain.Tariff, error)
	WithTx(tx transaction.Tx) (TariffRepository, error)
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
	WithTx(tx transaction.Tx) (SubscriptionRepository, error)
}

type PaymentMethodRepository interface {
	Create(ctx context.Context, pm domain.PaymentMethod) (domain.PaymentMethod, error)
	UpsertByTokenHash(ctx context.Context, pm domain.PaymentMethod) (domain.PaymentMethod, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.PaymentMethod, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error)
	SetActive(ctx context.Context, userID, methodID uuid.UUID) error
	Delete(ctx context.Context, userID, methodID uuid.UUID) error
	WithTx(tx transaction.Tx) (PaymentMethodRepository, error)
}

// PaymentMethodInUseChecker checks whether a payment method is referenced by an
// active subscription. The check lives in the application layer so the
// repository does not own business rules.
type PaymentMethodInUseChecker interface {
	IsInUse(ctx context.Context, methodID uuid.UUID) (bool, error)
	WithTx(tx transaction.Tx) (PaymentMethodInUseChecker, error)
}

// PropertyLimiter returns the maximum number of active properties a user is
// allowed to own based on their subscription. The rule lives in the billing
// application layer, not in the postgres adapter.
type PropertyLimiter interface {
	ActivePropertyLimit(ctx context.Context, userID uuid.UUID) (int, error)
	WithTx(tx transaction.Tx) (PropertyLimiter, error)
}

type SubscriptionPaymentRepository interface {
	Create(ctx context.Context, payment domain.SubscriptionPayment) (domain.SubscriptionPayment, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.SubscriptionPayment, error)
	GetByIDAdmin(ctx context.Context, id uuid.UUID) (SubscriptionPaymentWithUser, error)
	GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.SubscriptionPayment, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.SubscriptionPayment, error)
	ListPendingSubscriptionPaymentsByUserID(ctx context.Context, userID uuid.UUID) ([]domain.SubscriptionPayment, error)
	GetLastSucceededBySubscriptionID(ctx context.Context, subscriptionID uuid.UUID) (domain.SubscriptionPayment, error)
	ListPendingUpgradePayments(ctx context.Context, createdBefore time.Time, limit int32) ([]domain.SubscriptionPayment, error)
	ListPendingPayments(ctx context.Context, createdBefore time.Time, limit int32) ([]domain.SubscriptionPayment, error)
	ListAll(ctx context.Context, status string, userID uuid.UUID, limit, offset int) ([]SubscriptionPaymentWithUser, int64, error)
	MarkSucceeded(ctx context.Context, id uuid.UUID, now time.Time) error
	MarkFailed(ctx context.Context, id uuid.UUID, errorCode *string, now time.Time) error
	MarkRefunded(ctx context.Context, id uuid.UUID, now time.Time) error
	MarkReconciledSucceeded(ctx context.Context, id uuid.UUID, now time.Time) error
	MarkReconciledRefunded(ctx context.Context, id uuid.UUID, now time.Time) error
	BeginRefund(ctx context.Context, id uuid.UUID, now time.Time) error
	RevertRefund(ctx context.Context, id uuid.UUID, prev domain.PaymentStatus, now time.Time) error
	UpdateProviderPaymentID(ctx context.Context, id uuid.UUID, providerPaymentID string) (domain.SubscriptionPayment, error)
	UpdatePaymentURL(ctx context.Context, id uuid.UUID, paymentURL string) (domain.SubscriptionPayment, error)
	UpdatePaymentMethodAndProviderID(ctx context.Context, id, paymentMethodID uuid.UUID, providerPaymentID string) (domain.SubscriptionPayment, error)
	UpdatePaymentMethodID(ctx context.Context, id, paymentMethodID uuid.UUID) (domain.SubscriptionPayment, error)
	WithTx(tx transaction.Tx) (SubscriptionPaymentRepository, error)
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

// PaymentInitiator starts a new payment at the provider.
type PaymentInitiator interface {
	Init(ctx context.Context, req InitRequest) (InitResult, error)
}

// PaymentCharger performs a recurrent charge using a previously saved token.
type PaymentCharger interface {
	Charge(ctx context.Context, req ChargeRequest) (ChargeResult, error)
}

// PaymentCanceler cancels or refunds a finalized payment.
type PaymentCanceler interface {
	Cancel(ctx context.Context, req CancelRequest) (CancelResult, error)
}

// PaymentStatusChecker queries the current provider-side status of a payment.
type PaymentStatusChecker interface {
	Status(ctx context.Context, paymentID uuid.UUID, providerPaymentID string) (domain.PaymentStatus, error)
}

// WebhookParser parses and verifies an incoming provider webhook payload.
type WebhookParser interface {
	ParseWebhook(ctx context.Context, payload []byte) (WebhookPayload, error)
}

// CardManager manages customer cards bound at the provider.
type CardManager interface {
	InitAddCard(ctx context.Context, req InitAddCardRequest) (InitAddCardResult, error)
	RemoveCard(ctx context.Context, customerKey, cardID string) error
}

// WebhookResponder returns the fixed body the provider expects as a webhook ack.
type WebhookResponder interface {
	WebhookResponse() []byte
}

// ProviderNamer identifies the payment provider implementation.
type ProviderNamer interface {
	Name() domain.PaymentProvider
}

// Provider abstracts the external payment processor used for subscription payments.
// It is the aggregate of the narrow payment capability interfaces above plus the
// provider identity. Consumers that need only a subset should depend on the
// narrow interface directly.
type Provider interface {
	PaymentInitiator
	PaymentCharger
	PaymentCanceler
	PaymentStatusChecker
	WebhookParser
	CardManager
	WebhookResponder
	ProviderNamer
}

// RenewalProvider aggregates the capabilities used by RenewalService.
type RenewalProvider interface {
	PaymentInitiator
	PaymentCharger
	PaymentStatusChecker
	ProviderNamer
}

// WebhookProvider aggregates the capabilities used by WebhookService.
type WebhookProvider interface {
	WebhookParser
	WebhookResponder
	PaymentStatusChecker
	ProviderNamer
}

// PaymentManager aggregates the capabilities used by PaymentService.
type PaymentManager interface {
	PaymentCanceler
	PaymentStatusChecker
	ProviderNamer
}

// CardProvider aggregates the capabilities used by PaymentMethodService.
type CardProvider interface {
	CardManager
	ProviderNamer
}

// SubscriptionPaymentProvider aggregates the capabilities used by SubscriptionService.
type SubscriptionPaymentProvider interface {
	PaymentInitiator
	ProviderNamer
}

// ScheduledChangeProvider aggregates the capabilities used by ScheduledChangeService.
type ScheduledChangeProvider interface {
	PaymentInitiator
	PaymentCharger
	PaymentStatusChecker
	ProviderNamer
}

// OnboardingService creates default billing state for newly-registered owners.
type OnboardingService interface {
	SetupDefaultSubscription(ctx context.Context, userID uuid.UUID) error
}

type InitRequest struct {
	PaymentID              uuid.UUID
	AmountKopecks          int64
	Period                 domain.SubscriptionPeriod
	UserID                 uuid.UUID
	CustomerKey            string
	Description            string
	ReturnURL              string
	NotificationURL        string
	SuccessURL             string
	FailURL                string
	Recurrent              bool
	OperationInitiatorType string
}

type InitResult struct {
	ProviderPaymentID string
	PaymentURL        string
	SavedToken        string // token to save as PaymentMethod (RebillId for T-Kassa)
	CustomerKey       string
	Status            domain.PaymentStatus
}

type ChargeRequest struct {
	PaymentID         uuid.UUID
	ProviderPaymentID string // T-Kassa PaymentId from Init
	AmountKopecks     int64
	Token             string // saved token / RebillId
}

type ChargeResult struct {
	ProviderPaymentID string
	Status            domain.PaymentStatus
}

// CancelRequest asks the provider to cancel or refund a finalized payment.
// The system always refunds the full amount; AmountKopecks carries that amount.
type CancelRequest struct {
	PaymentID         uuid.UUID
	ProviderPaymentID string
	AmountKopecks     int64
}

// CancelResult reports the provider's response to a cancel/refund request.
type CancelResult struct {
	ProviderPaymentID     string
	Status                domain.PaymentStatus
	RefundedAmountKopecks int64
}

type WebhookPayload struct {
	ProviderPaymentID string
	InternalPaymentID uuid.UUID
	Status            domain.PaymentStatus
	ErrorCode         *string
	AmountKopecks     int64 // payment amount for regular webhooks, refund amount for refund webhooks
	RebillID          string
	CardID            string
	Pan               string
	ExpDate           string
	CustomerKey       string
	RequestKey        string
	NotificationType  string
}

// InitAddCardRequest starts a T-Kassa "AddCard" initialization.
type InitAddCardRequest struct {
	UserID      uuid.UUID
	CustomerKey string
	CheckType   string
}

// InitAddCardResult carries the T-Kassa response for AddCard initialization.
type InitAddCardResult struct {
	PaymentURL  string
	RequestKey  string
	CustomerKey string
}
