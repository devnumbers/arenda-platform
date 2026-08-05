package wire

import (
	"context"
	"fmt"

	"github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment"
	paymentfake "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment/fake"
	paymenttkassa "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment/tkassa"
	billingpg "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/postgres"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	platformpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
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
// billing repos and the property service (property archiver). It runs after the
// properties services are built.
func BuildBillingServices(p platformDeps, repos *BillingRepos, propertyService *propertiesapp.PropertyService) *Billing {
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
		repos.OnboardingService,
		repos.PaymentMethodInUseChecker,
	)
	return &Billing{Services: services}
}
