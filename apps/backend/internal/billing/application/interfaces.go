package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// Tariffer lists available tariffs.
type Tariffer interface {
	ListTariffs(ctx context.Context) ([]domain.Tariff, error)
}

// Subscriber manages the current user's subscription and tariff changes.
type Subscriber interface {
	GetSubscription(ctx context.Context, userID uuid.UUID) (SubscriptionView, error)
	ChangeTariff(ctx context.Context, userID uuid.UUID, req ChangeTariffRequest) (ChangeTariffResponse, error)
	CancelSubscription(ctx context.Context, userID uuid.UUID) error
	ToggleAutoRenew(ctx context.Context, userID uuid.UUID, enabled bool) error
}

// PaymentMethodManager manages the user's saved payment methods.
type PaymentMethodManager interface {
	AddPaymentMethod(ctx context.Context, userID uuid.UUID, req AddPaymentMethodRequest) (AddPaymentMethodResponse, error)
	SetActivePaymentMethod(ctx context.Context, userID, methodID uuid.UUID) error
	DeletePaymentMethod(ctx context.Context, userID, methodID uuid.UUID) error
	ListPaymentMethods(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error)
	SyncPaymentMethods(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error)
}

// PaymentProcessor lists and acts on subscription payments.
type PaymentProcessor interface {
	ListPayments(ctx context.Context, userID uuid.UUID) ([]SubscriptionPaymentView, error)
	GetPayment(ctx context.Context, paymentID uuid.UUID) (AdminSubscriptionPaymentView, error)
	ListAllPayments(ctx context.Context, filters ListAllPaymentsFilters) ([]AdminSubscriptionPaymentView, int64, error)
	ConfirmFakePayment(ctx context.Context, paymentID uuid.UUID) error
	RefundPayment(ctx context.Context, paymentID uuid.UUID) error
	SyncPendingPayment(ctx context.Context, paymentID uuid.UUID) error
	ReconcilePendingPayments(ctx context.Context, now time.Time) (int, error)
	ReconcileStaleRefunds(ctx context.Context, now time.Time) (int, error)
}

// WebhookHandler handles provider webhooks.
type WebhookHandler interface {
	HandleWebhook(ctx context.Context, providerName string, payload []byte) error
	WebhookResponse() []byte
}

// RenewalRunner drives the subscription renewal background jobs.
type RenewalRunner interface {
	ProcessRenewals(ctx context.Context, now time.Time) (int, error)
	ProcessExpiredGrace(ctx context.Context, now time.Time) (int, error)
	ProcessPendingUpgradePayments(ctx context.Context, now time.Time) (int, error)
}

// ScheduledChangeRunner applies scheduled tariff changes.
type ScheduledChangeRunner interface {
	ProcessScheduledChanges(ctx context.Context, now time.Time) (int, error)
}

// Compile-time checks that each sub-service satisfies its consumer-facing port.
var (
	_ Tariffer              = (*TariffService)(nil)
	_ Subscriber            = (*SubscriptionService)(nil)
	_ PaymentMethodManager  = (*PaymentMethodService)(nil)
	_ PaymentProcessor      = (*PaymentService)(nil)
	_ WebhookHandler        = (*WebhookService)(nil)
	_ RenewalRunner         = (*RenewalService)(nil)
	_ ScheduledChangeRunner = (*ScheduledChangeService)(nil)
)
