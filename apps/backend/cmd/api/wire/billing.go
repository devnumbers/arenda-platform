package wire

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment"
	paymentfake "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment/fake"
	paymenttkassa "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment/tkassa"
	billingpg "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/postgres"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	identityhttp "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/http"
	platformpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
)

// BillingRepos holds the billing module's repositories, onboarding service and
// payment provider wired by WireBillingRepos. These are constructed early
// (before the properties services) because the subscription limiter used by
// PropertyService depends on the tariff and subscription repositories.
type BillingRepos struct {
	TariffRepo                *billingpg.TariffRepository
	SubscriptionRepo          *billingpg.SubscriptionRepository
	PaymentMethodRepo         *billingpg.PaymentMethodRepository
	SubscriptionPaymentRepo   *billingpg.SubscriptionPaymentRepository
	OnboardingService         *billingpg.OnboardingService
	PaymentMethodInUseChecker *billingpg.PaymentMethodInUseChecker
	PaymentProvider           billingapp.Provider
}

// WireBillingRepos constructs the billing repositories, the onboarding service,
// selects the payment provider based on config and logs initialization. The
// billing Services aggregate is built later (BuildBillingServices) once the
// properties service (property archiver) is available.
func WireBillingRepos(ctx context.Context, p platformDeps) (*BillingRepos, error) {
	tariffRepo := billingpg.NewTariffRepository(p.DB, p.Cfg.TariffCacheTTL, p.Clock)
	subscriptionRepo := billingpg.NewSubscriptionRepository(p.DB)
	onboardingService := billingpg.NewOnboardingService(tariffRepo, subscriptionRepo, platformpostgres.NewBeginner(p.Pool, p.Logger))
	paymentMethodRepo := billingpg.NewPaymentMethodRepository(p.DB, p.Encryptor)
	paymentMethodInUseChecker := billingpg.NewPaymentMethodInUseChecker(p.DB)
	subscriptionPaymentRepo := billingpg.NewSubscriptionPaymentRepository(p.DB, p.Encryptor)
	p.Logger.InfoContext(ctx, "billing repositories initialized",
		"payment_methods", paymentMethodRepo != nil,
		"subscription_payments", subscriptionPaymentRepo != nil)

	var paymentProvider billingapp.Provider
	paymentMetrics, err := payment.NewMetrics()
	if err != nil {
		return nil, fmt.Errorf("payment metrics: %w", err)
	}
	switch p.Cfg.PaymentProvider {
	case "fake":
		paymentProvider = paymentfake.NewProvider(p.Cfg.AppBaseURL, p.Logger, p.Clock, paymentMetrics)
	case "tkassa":
		paymentProvider = paymenttkassa.NewProvider(
			p.Cfg.TKassaBaseURL,
			p.Cfg.TKassaTerminalKey,
			p.Cfg.TKassaPassword,
			p.Cfg.TKassaTimeout,
			p.Cfg.TKassaMaxRetries,
			p.Cfg.TKassaRetryBaseDelay,
			p.Cfg.TKassaRetryMaxDelay,
			p.Logger,
			paymentMetrics,
		)
	}
	p.Logger.InfoContext(ctx, "payment provider initialized", "provider", p.Cfg.PaymentProvider, "initialized", paymentProvider != nil)

	return &BillingRepos{
		TariffRepo:                tariffRepo,
		SubscriptionRepo:          subscriptionRepo,
		PaymentMethodRepo:         paymentMethodRepo,
		SubscriptionPaymentRepo:   subscriptionPaymentRepo,
		OnboardingService:         onboardingService,
		PaymentMethodInUseChecker: paymentMethodInUseChecker,
		PaymentProvider:           paymentProvider,
	}, nil
}

// Billing holds the billing Services aggregate wired by BuildBillingServices.
type Billing struct {
	Services billingapp.Services
}

// BuildBillingServices constructs the billing Services aggregate from the
// billing repos, the property service (property archiver) and the recipient
// slot enforcer (access SlotCoordinator). It runs after the properties services
// and the access module are built.
func BuildBillingServices(
	p platformDeps,
	repos *BillingRepos,
	propertyService *propertiesapp.PropertyService,
	recipientSlotEnforcer billingapp.RecipientSlotEnforcer,
) *Billing {
	services := billingapp.NewServices(
		repos.TariffRepo,
		repos.SubscriptionRepo,
		repos.PaymentMethodRepo,
		repos.SubscriptionPaymentRepo,
		repos.PaymentProvider,
		platformpostgres.NewBeginner(p.Pool, p.Logger),
		p.AuditRecorder,
		p.Clock,
		p.Logger,
		p.Cfg.AppBaseURL,
		propertyService,
		recipientSlotEnforcer,
		repos.OnboardingService,
		repos.PaymentMethodInUseChecker,
	)
	return &Billing{Services: services}
}

// BillingMeEnricher returns an identity MeEnricher that adds the current billing
// subscription to a MeResponse. It is the composition-root glue between the
// billing application layer (Subscriber) and the identity HTTP layer
// (MeEnricher); keeping it here means identity does not import billing.
func BillingMeEnricher(billing billingapp.Subscriber) identityhttp.MeEnricher {
	return func(ctx context.Context, userID uuid.UUID, resp *openapi.MeResponse) error {
		if billing == nil {
			return nil
		}
		view, err := billing.GetSubscription(ctx, userID)
		if err != nil {
			if errors.Is(err, billingapp.ErrSubscriptionNotFound) {
				return nil
			}
			return err
		}
		sub := httpsupport.SubscriptionResponse(view)
		resp.Subscription = &sub
		return nil
	}
}
