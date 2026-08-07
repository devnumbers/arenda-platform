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
	// ErrProviderCustomerNotFound is returned when a provider reports that the
	// customer does not exist yet. For read operations such as card listing it
	// semantically means the user has no cards, not a failure.
	ErrProviderCustomerNotFound = errors.New("provider customer not found")
	// ErrProviderTerminalNotFound is returned when a provider reports that the
	// configured terminal does not exist (for example, a deleted or wrong test
	// terminal). For card-list sync this is treated as "no cards at the provider"
	// rather than a hard failure, because the local payment methods are still
	// valid from the user's point of view.
	ErrProviderTerminalNotFound = errors.New("provider terminal not found")
	// ErrProviderChargeBlocked is returned when the provider rejects a recurrent
	// charge because charging is disabled or COF is not enabled on the terminal
	// (T-Kassa error code 10).
	ErrProviderChargeBlocked = errors.New("provider charge blocked")
	// ErrProviderTokenInvalid is returned when the provider rejects the request
	// token (T-Kassa error codes 204/205) — a signing or terminal-key
	// misconfiguration.
	ErrProviderTokenInvalid = errors.New("provider token invalid")
	// ErrProviderPaymentNotFound is returned when the provider does not know the
	// payment being operated on (T-Kassa error code 255).
	ErrProviderPaymentNotFound = errors.New("provider payment not found")
	// ErrProviderInvalidOperation is returned when the provider rejects the
	// operation parameters as inconsistent (T-Kassa error codes 1125/1126) — an
	// integration misconfiguration, for example a mismatched
	// OperationInitiatorType.
	ErrProviderInvalidOperation = errors.New("provider invalid operation")
	ErrInvalidFilter            = errors.New("invalid filter")
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
	ListStaleRefundingPayments(ctx context.Context, updatedBefore time.Time, limit int32) ([]domain.SubscriptionPayment, error)
	ListAll(ctx context.Context, filters ListAllPaymentsFilters) ([]SubscriptionPaymentWithUser, int64, error)
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
	IncrementChargeAttempts(ctx context.Context, id uuid.UUID) (int, error)
	WithTx(tx transaction.Tx) (SubscriptionPaymentRepository, error)
}

// PropertyArchiver archives properties when a subscription is downgraded to a
// lower-limit tariff. It is implemented by the properties application service.
type PropertyArchiver interface {
	ArchiveExcessProperties(ctx context.Context, tx transaction.Tx, ownerID uuid.UUID, limit int) error
}

// RecipientSlotEnforcer suspends shared-access memberships whose recipient's
// tariff limit is exceeded after a billing limit drop (downgrade / grace expiry
// / non-renewing expiry). It is called with the downgrading user's own id:
// both his shared memberships on other owners' objects and the memberships of
// the recipients on his objects are enforced. Implemented by the access
// bounded context's SlotCoordinator. Called in the same transaction as
// ArchiveExcessProperties. Optional: when nil, no recipient enforcement runs
// (pre-T4 behaviour). See issue #158 (T4).
type RecipientSlotEnforcer interface {
	EnforceRecipientLimit(ctx context.Context, tx transaction.Tx, userID uuid.UUID, trigger string) error
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
	Status(ctx context.Context, paymentID uuid.UUID, providerPaymentID string) (PaymentStatusResult, error)
}

// PaymentStatusResult is the provider-side state of a payment.
type PaymentStatusResult struct {
	Status domain.PaymentStatus
	// RebillID is the saved-card token (T-Kassa RebillId) reported by the
	// provider for this payment; empty when the provider does not report it.
	RebillID string
	// ErrorCode is the provider error code reported for a failed payment;
	// empty on success or when the provider does not report one.
	ErrorCode string
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

// CardLister lists cards bound to a customer at the provider.
type CardLister interface {
	GetCardList(ctx context.Context, customerKey string) ([]ProviderCard, error)
}

// CardBindingStateChecker queries the state of a card-binding session started
// by InitAddCard.
type CardBindingStateChecker interface {
	GetAddCardState(ctx context.Context, requestKey string) (CardBindingState, error)
}

// CardBindingState is the provider-side state of a card-binding session.
type CardBindingState struct {
	Status      CardBindingStatus
	CardID      string
	RebillID    string
	CustomerKey string
	// ErrorCode is the provider error code for failed bindings; empty on
	// success.
	ErrorCode string
}

// CardBindingStatus mirrors the T-Kassa GetAddCardState response statuses.
type CardBindingStatus string

const (
	// CardBindingStatusNew is a freshly created card-binding session.
	CardBindingStatusNew CardBindingStatus = "NEW"
	// CardBindingStatusFormShowed means the binding form was shown to the user.
	CardBindingStatusFormShowed CardBindingStatus = "FORM_SHOWED"
	// CardBindingStatus3DSChecking means the user was sent to the 3DS check.
	CardBindingStatus3DSChecking CardBindingStatus = "3DS_CHECKING"
	// CardBindingStatus3DSChecked means the user passed the 3DS check.
	CardBindingStatus3DSChecked CardBindingStatus = "3DS_CHECKED"
	// CardBindingStatusAuthorizing means the 0 RUB authorization payment is in
	// flight.
	CardBindingStatusAuthorizing CardBindingStatus = "AUTHORIZING"
	// CardBindingStatusAuthorized means the 0 RUB authorization succeeded.
	CardBindingStatusAuthorized CardBindingStatus = "AUTHORIZED"
	// CardBindingStatusCompleted means the card was bound successfully.
	CardBindingStatusCompleted CardBindingStatus = "COMPLETED"
	// CardBindingStatusRejected means the card could not be bound.
	CardBindingStatusRejected CardBindingStatus = "REJECTED"
)

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
	CardLister
	CardBindingStateChecker
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
	CardLister
	CardBindingStateChecker
	ProviderNamer
}

// SubscriptionPaymentProvider aggregates the capabilities used by SubscriptionService.
type SubscriptionPaymentProvider interface {
	PaymentInitiator
	ProviderNamer
}

// OnboardingService creates default billing state for newly-registered owners.
type OnboardingService interface {
	SetupDefaultSubscription(ctx context.Context, userID uuid.UUID) error
}

// UserRegisteredHandler handles the user registration event. Billing owns this
// narrow port so it does not depend on the identity context.
type UserRegisteredHandler interface {
	OnUserRegistered(ctx context.Context, userID uuid.UUID) error
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
	OperationInitiatorType OperationInitiatorType
	// RedirectDueDate is the absolute deadline of the payment form link
	// (T-Kassa Init RedirectDueDate, RFC3339 on the wire). Zero value omits
	// the field and the provider default (24h) applies.
	RedirectDueDate time.Time
}

// OperationInitiatorType marks who initiated a payment operation. The values
// mirror the T-Kassa OperationInitiatorType enum (DATA field of Init); the
// application layer defines only the values it actually uses.
type OperationInitiatorType string

const (
	// InitiatorTypeCITCC is a customer-initiated payment with card credentials
	// (CIT CC): the parent payment of a CC/COF chain that obtains the RebillId.
	InitiatorTypeCITCC OperationInitiatorType = "1"
	// InitiatorTypeMITRecurring is a merchant-initiated recurring charge
	// (MIT COF Recurring) on previously saved credentials.
	InitiatorTypeMITRecurring OperationInitiatorType = "R"
)

// CardCheckType selects the verification performed when binding a card. The
// values mirror the T-Kassa AddCard CheckType enum.
type CardCheckType string

const (
	// CardCheckType3DSHold checks 3DS support while binding the card and holds
	// 0 RUB when the card does not support 3DS.
	CardCheckType3DSHold CardCheckType = "3DSHOLD"
)

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
	// ErrorCode is the provider error code reported for a failed charge;
	// empty on success or when the provider does not report one.
	ErrorCode string
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
	UserID          uuid.UUID
	CustomerKey     string
	CheckType       CardCheckType
	SuccessURL      string
	FailURL         string
	NotificationURL string
}

// InitAddCardResult carries the T-Kassa response for AddCard initialization.
type InitAddCardResult struct {
	PaymentURL  string
	RequestKey  string
	CustomerKey string
}

// ProviderCardStatus is the provider-side status of a bound card. The values
// mirror the T-Kassa card list wire format.
type ProviderCardStatus string

const (
	// ProviderCardStatusActive marks a card that is bound and chargeable.
	ProviderCardStatusActive ProviderCardStatus = "A"
	// ProviderCardStatusInactive marks a temporarily inactive card.
	ProviderCardStatusInactive ProviderCardStatus = "I"
	// ProviderCardStatusDeleted marks a card detached from the customer.
	ProviderCardStatusDeleted ProviderCardStatus = "D"
)

// ProviderCard describes a card bound to a customer at the payment provider.
type ProviderCard struct {
	CardID   string
	Pan      string
	ExpDate  string
	RebillID string
	Status   ProviderCardStatus
}
