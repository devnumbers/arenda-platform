package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// BillingService orchestrates subscription, tariff and payment operations.
//
// It is a thin facade that delegates every operation to a focused sub-service.
// The public API (the struct and NewBillingService) is preserved so existing
// consumers and tests are unaffected by the decomposition.
type BillingService struct {
	tariffSvc          *TariffService
	subscriptionSvc    *SubscriptionService
	paymentMethodSvc   *PaymentMethodService
	paymentSvc         *PaymentService
	webhookSvc         *WebhookService
	renewalSvc         *RenewalService
	scheduledChangeSvc *ScheduledChangeService
	onboarding         OnboardingService
}

// NewBillingService creates a new billing application service.
func NewBillingService(
	tariffs TariffRepository,
	subscriptions SubscriptionRepository,
	paymentMethods PaymentMethodRepository,
	subscriptionPayments SubscriptionPaymentRepository,
	provider Provider,
	beginner transaction.Beginner,
	clock clock.Clock,
	log *slog.Logger,
	callbackBaseURL string,
	propertyArchiver PropertyArchiver,
	onboarding OnboardingService,
) *BillingService {
	deps := newFlowDeps(
		tariffs,
		subscriptions,
		paymentMethods,
		subscriptionPayments,
		propertyArchiver,
		provider,
		beginner,
		clock,
		log,
		callbackBaseURL,
	)
	return &BillingService{
		tariffSvc:          NewTariffService(deps),
		subscriptionSvc:    NewSubscriptionService(deps),
		paymentMethodSvc:   NewPaymentMethodService(deps),
		paymentSvc:         NewPaymentService(deps),
		webhookSvc:         NewWebhookService(deps),
		renewalSvc:         NewRenewalService(deps),
		scheduledChangeSvc: NewScheduledChangeService(deps),
		onboarding:         onboarding,
	}
}

// OnUserRegistered handles the identity.UserRegistered event by setting up the
// default subscription for a newly-created user.
func (s *BillingService) OnUserRegistered(ctx context.Context, event any) error {
	e, ok := event.(identityapp.UserRegistered)
	if !ok {
		return fmt.Errorf("unexpected event type %T", event)
	}
	return s.onboarding.SetupDefaultSubscription(ctx, e.UserID)
}

// ListTariffs returns all tariffs ordered by price.
func (s *BillingService) ListTariffs(ctx context.Context) ([]domain.Tariff, error) {
	return s.tariffSvc.ListTariffs(ctx)
}

// GetSubscription returns the current subscription with its tariff and active payment method.
func (s *BillingService) GetSubscription(ctx context.Context, userID uuid.UUID) (SubscriptionView, error) {
	return s.subscriptionSvc.GetSubscription(ctx, userID)
}

// ChangeTariff starts an upgrade payment or schedules a downgrade.
func (s *BillingService) ChangeTariff(ctx context.Context, userID uuid.UUID, req ChangeTariffRequest) (ChangeTariffResponse, error) {
	return s.subscriptionSvc.ChangeTariff(ctx, userID, req)
}

// CancelSubscription terminates the paid subscription.
func (s *BillingService) CancelSubscription(ctx context.Context, userID uuid.UUID) error {
	return s.subscriptionSvc.CancelSubscription(ctx, userID)
}

// ToggleAutoRenew enables or disables automatic subscription renewal.
func (s *BillingService) ToggleAutoRenew(ctx context.Context, userID uuid.UUID, enabled bool) error {
	return s.subscriptionSvc.ToggleAutoRenew(ctx, userID, enabled)
}

// AddPaymentMethod stores a new inactive payment method for the user.
func (s *BillingService) AddPaymentMethod(ctx context.Context, userID uuid.UUID, req AddPaymentMethodRequest) (AddPaymentMethodResponse, error) {
	return s.paymentMethodSvc.AddPaymentMethod(ctx, userID, req)
}

// SetActivePaymentMethod activates the given payment method for the user.
func (s *BillingService) SetActivePaymentMethod(ctx context.Context, userID, methodID uuid.UUID) error {
	return s.paymentMethodSvc.SetActivePaymentMethod(ctx, userID, methodID)
}

// DeletePaymentMethod removes a payment method belonging to the user.
func (s *BillingService) DeletePaymentMethod(ctx context.Context, userID, methodID uuid.UUID) error {
	return s.paymentMethodSvc.DeletePaymentMethod(ctx, userID, methodID)
}

// ListPaymentMethods returns all payment methods for the user.
func (s *BillingService) ListPaymentMethods(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error) {
	return s.paymentMethodSvc.ListPaymentMethods(ctx, userID)
}

// ListPayments returns all subscription payments for the user with their tariffs.
func (s *BillingService) ListPayments(ctx context.Context, userID uuid.UUID) ([]SubscriptionPaymentView, error) {
	return s.paymentSvc.ListPayments(ctx, userID)
}

// GetPayment returns a single subscription payment for admin view.
func (s *BillingService) GetPayment(ctx context.Context, paymentID uuid.UUID) (AdminSubscriptionPaymentView, error) {
	return s.paymentSvc.GetPayment(ctx, paymentID)
}

// ListAllPayments returns all subscription payments for admin view.
func (s *BillingService) ListAllPayments(ctx context.Context, filters ListAllPaymentsFilters) ([]AdminSubscriptionPaymentView, int64, error) {
	return s.paymentSvc.ListAllPayments(ctx, filters)
}

// ConfirmFakePayment confirms a previously initialized fake payment and applies its result.
func (s *BillingService) ConfirmFakePayment(ctx context.Context, paymentID uuid.UUID) error {
	return s.paymentSvc.ConfirmFakePayment(ctx, paymentID)
}

// RefundPayment cancels/refunds a succeeded or pending subscription payment.
func (s *BillingService) RefundPayment(ctx context.Context, paymentID uuid.UUID) error {
	return s.paymentSvc.RefundPayment(ctx, paymentID)
}

// SyncPendingPayment queries the provider for the current status of a single
// pending subscription payment and finalizes it based on the response.
func (s *BillingService) SyncPendingPayment(ctx context.Context, paymentID uuid.UUID) error {
	return s.paymentSvc.SyncPendingPayment(ctx, paymentID)
}

// ReconcilePendingPayments checks all pending subscription payments that have
// been stuck longer than the staleness threshold with the provider in batches
// and finalizes them via SyncPendingPayment.
func (s *BillingService) ReconcilePendingPayments(ctx context.Context, now time.Time) (int, error) {
	return s.paymentSvc.ReconcilePendingPayments(ctx, now)
}

// WebhookResponse returns the provider-specific response body that must be sent
// back after a webhook is handled.
func (s *BillingService) WebhookResponse() []byte {
	return s.webhookSvc.WebhookResponse()
}

// HandleWebhook parses and applies a provider webhook payload.
func (s *BillingService) HandleWebhook(ctx context.Context, providerName string, payload []byte) error {
	return s.webhookSvc.HandleWebhook(ctx, providerName, payload)
}

// ProcessRenewals processes all subscriptions whose validity period has ended.
func (s *BillingService) ProcessRenewals(ctx context.Context, now time.Time) (int, error) {
	return s.renewalSvc.ProcessRenewals(ctx, now)
}

// ProcessExpiredGrace downgrades subscriptions whose grace period has ended to
// the free basic tariff and archives properties that exceed the basic limit.
func (s *BillingService) ProcessExpiredGrace(ctx context.Context, now time.Time) (int, error) {
	return s.renewalSvc.ProcessExpiredGrace(ctx, now)
}

// ProcessPendingUpgradePayments queries the provider for pending upgrade
// payments that have been stale for longer than the configured threshold and
// finalizes them based on the provider status.
func (s *BillingService) ProcessPendingUpgradePayments(ctx context.Context, now time.Time) (int, error) {
	return s.renewalSvc.ProcessPendingUpgradePayments(ctx, now)
}

// ProcessScheduledChanges applies scheduled tariff changes whose
// pending_change_at has been reached.
func (s *BillingService) ProcessScheduledChanges(ctx context.Context, now time.Time) (int, error) {
	return s.scheduledChangeSvc.ProcessScheduledChanges(ctx, now)
}

// renewSubscription is retained on the facade as a delegating wrapper because
// the existing test suite exercises it directly via the BillingService receiver.
func (s *BillingService) renewSubscription(ctx context.Context, sub domain.Subscription, now time.Time) error {
	return s.renewalSvc.renewSubscription(ctx, sub, now)
}
