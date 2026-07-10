package application

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
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

// Compile-time assertion that Services implements the user-registration event
// port consumed by main.go. This keeps the UserRegisteredHandler contract alive
// even though it is not stored as a struct field.
var _ UserRegisteredHandler = Services{}

// tariffServiceDeps is the narrow dependency bundle for TariffService.
type tariffServiceDeps struct {
	tariffs TariffRepository
}

// subscriptionServiceDeps is the narrow dependency bundle for SubscriptionService.
type subscriptionServiceDeps struct {
	tariffs              TariffRepository
	subscriptions        SubscriptionRepository
	subscriptionPayments SubscriptionPaymentRepository
	paymentMethods       PaymentMethodRepository
	beginner             transaction.Beginner
	clock                clock.Clock
	log                  *slog.Logger
	callbackBaseURL      string
}

// paymentMethodServiceDeps is the narrow dependency bundle for PaymentMethodService.
type paymentMethodServiceDeps struct {
	paymentMethods PaymentMethodRepository
	subscriptions  SubscriptionRepository
	beginner       transaction.Beginner
	clock          clock.Clock
	log            *slog.Logger
}

// paymentServiceDeps is the narrow dependency bundle for PaymentService.
type paymentServiceDeps struct {
	subscriptionPayments SubscriptionPaymentRepository
	subscriptions        SubscriptionRepository
	paymentMethods       PaymentMethodRepository
	tariffs              TariffRepository
	propertyArchiver     PropertyArchiver
	beginner             transaction.Beginner
	clock                clock.Clock
	log                  *slog.Logger
}

// renewalServiceDeps is the narrow dependency bundle for RenewalService.
type renewalServiceDeps struct {
	tariffs              TariffRepository
	subscriptions        SubscriptionRepository
	subscriptionPayments SubscriptionPaymentRepository
	paymentMethods       PaymentMethodRepository
	propertyArchiver     PropertyArchiver
	beginner             transaction.Beginner
	log                  *slog.Logger
	callbackBaseURL      string
}

// webhookServiceDeps is the narrow dependency bundle for WebhookService.
type webhookServiceDeps struct {
	subscriptionPayments SubscriptionPaymentRepository
	subscriptions        SubscriptionRepository
	paymentMethods       PaymentMethodRepository
	tariffs              TariffRepository
	propertyArchiver     PropertyArchiver
	beginner             transaction.Beginner
	clock                clock.Clock
	log                  *slog.Logger
}

// scheduledChangeServiceDeps is the narrow dependency bundle for ScheduledChangeService.
type scheduledChangeServiceDeps struct {
	tariffs              TariffRepository
	subscriptions        SubscriptionRepository
	subscriptionPayments SubscriptionPaymentRepository
	paymentMethods       PaymentMethodRepository
	propertyArchiver     PropertyArchiver
	beginner             transaction.Beginner
	log                  *slog.Logger
	callbackBaseURL      string
}

// NewServices builds the billing sub-services from the shared repository and
// infrastructure dependencies, wires each sub-service with only the fields it
// actually uses, and returns them typed as their consumer-facing ports.
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
	if log == nil {
		log = slog.Default()
	}
	return Services{
		Tariffs: NewTariffService(tariffServiceDeps{tariffs: tariffs}),
		Subscriptions: NewSubscriptionService(subscriptionServiceDeps{
			tariffs:              tariffs,
			subscriptions:        subscriptions,
			subscriptionPayments: subscriptionPayments,
			paymentMethods:       paymentMethods,
			beginner:             beginner,
			clock:                clk,
			log:                  log,
			callbackBaseURL:      callbackBaseURL,
		}, provider),
		PaymentMethods: NewPaymentMethodService(paymentMethodServiceDeps{
			paymentMethods: paymentMethods,
			subscriptions:  subscriptions,
			beginner:       beginner,
			clock:          clk,
			log:            log,
		}, paymentMethodInUseChecker, provider),
		Payments: NewPaymentService(paymentServiceDeps{
			subscriptionPayments: subscriptionPayments,
			subscriptions:        subscriptions,
			paymentMethods:       paymentMethods,
			tariffs:              tariffs,
			propertyArchiver:     propertyArchiver,
			beginner:             beginner,
			clock:                clk,
			log:                  log,
		}, provider),
		Webhooks: NewWebhookService(webhookServiceDeps{
			subscriptionPayments: subscriptionPayments,
			subscriptions:        subscriptions,
			paymentMethods:       paymentMethods,
			tariffs:              tariffs,
			propertyArchiver:     propertyArchiver,
			beginner:             beginner,
			clock:                clk,
			log:                  log,
		}, provider),
		Renewals: NewRenewalService(renewalServiceDeps{
			tariffs:              tariffs,
			subscriptions:        subscriptions,
			subscriptionPayments: subscriptionPayments,
			paymentMethods:       paymentMethods,
			propertyArchiver:     propertyArchiver,
			beginner:             beginner,
			log:                  log,
			callbackBaseURL:      callbackBaseURL,
		}, provider),
		ScheduledChanges: NewScheduledChangeService(scheduledChangeServiceDeps{
			tariffs:              tariffs,
			subscriptions:        subscriptions,
			subscriptionPayments: subscriptionPayments,
			paymentMethods:       paymentMethods,
			propertyArchiver:     propertyArchiver,
			beginner:             beginner,
			log:                  log,
			callbackBaseURL:      callbackBaseURL,
		}, provider),
		Onboarding: onboarding,
	}
}

// OnUserRegistered handles the user registration event by setting up the
// default subscription for a newly-created user. It is kept on the bundle
// (rather than on the Subscriber port) because it is an event adapter, not a
// subscription operation; main.go subscribes it to the event dispatcher.
func (s Services) OnUserRegistered(ctx context.Context, userID uuid.UUID) error {
	return s.Onboarding.SetupDefaultSubscription(ctx, userID)
}
