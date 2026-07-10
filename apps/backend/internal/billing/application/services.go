package application

import (
	"context"
	"fmt"
	"log/slog"

	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// Services bundles the consumer-facing billing ports. It is the single
// composition point that main.go uses to wire the billing sub-services without
// going through a facade: every field is typed as the narrow port it exposes,
// so transport and scheduler consumers depend only on what they use.
type Services struct {
	Tariffs          Tariffer
	Subscriptions    Subscriber
	PaymentMethods   PaymentMethodManager
	Payments         PaymentProcessor
	Webhooks         WebhookHandler
	Renewals         RenewalRunner
	ScheduledChanges ScheduledChangeRunner
	Onboarding       OnboardingService
}

// tariffServiceDeps is the narrow dependency bundle for TariffService. It embeds
// the shared flowDeps so field access (s.deps.tariffs) and the cross-cutting flow
// helpers in flows.go keep working unchanged while the service is wired through
// its own typed deps at the composition point.
type tariffServiceDeps struct{ flowDeps }

// subscriptionServiceDeps is the narrow dependency bundle for SubscriptionService.
type subscriptionServiceDeps struct{ flowDeps }

// paymentMethodServiceDeps is the narrow dependency bundle for PaymentMethodService.
type paymentMethodServiceDeps struct{ flowDeps }

// paymentServiceDeps is the narrow dependency bundle for PaymentService.
type paymentServiceDeps struct{ flowDeps }

// renewalServiceDeps is the narrow dependency bundle for RenewalService.
type renewalServiceDeps struct{ flowDeps }

// webhookServiceDeps is the narrow dependency bundle for WebhookService.
type webhookServiceDeps struct{ flowDeps }

// scheduledChangeServiceDeps is the narrow dependency bundle for ScheduledChangeService.
type scheduledChangeServiceDeps struct{ flowDeps }

// NewServices builds the billing sub-services from one shared flowDeps value,
// wraps it into each sub-service's narrow deps type, and returns them typed as
// their consumer-facing ports.
func NewServices(
	tariffs TariffRepository,
	subscriptions SubscriptionRepository,
	paymentMethods PaymentMethodRepository,
	subscriptionPayments SubscriptionPaymentRepository,
	provider Provider,
	beginner transaction.Beginner,
	clk clock.Clock,
	log *slog.Logger,
	callbackBaseURL string,
	propertyArchiver PropertyArchiver,
	onboarding OnboardingService,
	paymentMethodInUseChecker PaymentMethodInUseChecker,
) Services {
	deps := newFlowDeps(
		tariffs,
		subscriptions,
		paymentMethods,
		subscriptionPayments,
		propertyArchiver,
		provider,
		beginner,
		clk,
		log,
		callbackBaseURL,
	)
	return Services{
		Tariffs:          NewTariffService(tariffServiceDeps{flowDeps: deps}),
		Subscriptions:    NewSubscriptionService(subscriptionServiceDeps{flowDeps: deps}, provider),
		PaymentMethods:   NewPaymentMethodService(paymentMethodServiceDeps{flowDeps: deps}, paymentMethodInUseChecker, provider),
		Payments:         NewPaymentService(paymentServiceDeps{flowDeps: deps}, provider),
		Webhooks:         NewWebhookService(webhookServiceDeps{flowDeps: deps}, provider),
		Renewals:         NewRenewalService(renewalServiceDeps{flowDeps: deps}, provider),
		ScheduledChanges: NewScheduledChangeService(scheduledChangeServiceDeps{flowDeps: deps}, provider),
		Onboarding:       onboarding,
	}
}

// OnUserRegistered handles the identity.UserRegistered event by setting up the
// default subscription for a newly-created user. It is kept on the bundle
// (rather than on the Subscriber port) because it is an event adapter, not a
// subscription operation; main.go subscribes it to the event dispatcher.
func (s Services) OnUserRegistered(ctx context.Context, event any) error {
	e, ok := event.(identityapp.UserRegistered)
	if !ok {
		return fmt.Errorf("unexpected event type %T", event)
	}
	return s.Onboarding.SetupDefaultSubscription(ctx, e.UserID)
}
