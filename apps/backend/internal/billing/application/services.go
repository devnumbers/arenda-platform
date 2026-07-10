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

// NewServices builds the billing sub-services from one shared flowDeps value
// and returns them typed as their consumer-facing ports.
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
		Tariffs:          NewTariffService(deps),
		Subscriptions:    NewSubscriptionService(deps, provider),
		PaymentMethods:   NewPaymentMethodService(deps, paymentMethodInUseChecker, provider),
		Payments:         NewPaymentService(deps, provider),
		Webhooks:         NewWebhookService(deps, provider),
		Renewals:         NewRenewalService(deps, provider),
		ScheduledChanges: NewScheduledChangeService(deps, provider),
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
