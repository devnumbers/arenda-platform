package wire

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	billingevents "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/events"
	billinghttp "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/http"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment"
	paymentfake "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment/fake"
	paymenttkassa "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment/tkassa"
	billingpg "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/postgres"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	identityhttp "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/http"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/config"
	platformevents "github.com/nambers/arenda-planform/apps/backend/internal/platform/events"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// Billing holds the billing module's repositories and services wired by
// WireBilling. It is constructed before the access and properties modules
// because their subscription limiters consume the tariff and subscription
// repositories.
type Billing struct {
	TariffRepo        *billingpg.TariffRepository
	SubscriptionRepo  *billingpg.SubscriptionRepository
	TransitionRepo    *billingpg.SubscriptionTransitionRepository
	PaymentRepo       *billingpg.SubscriptionPaymentRepository
	PaymentMethodRepo *billingpg.PaymentMethodRepository
	CardBindingRepo   *billingpg.CardBindingSessionRepository
	Services          billingapp.Services
	// PaymentProvider is the single active payment provider adapter behind
	// the neutral provider port (issue #248, ADR 0038). Nil never occurs
	// because config validation pins PAYMENT_PROVIDER to fake or tkassa.
	PaymentProvider billingapp.PaymentProvider
	// FakeConfirms serves the local-only fake confirmation endpoints
	// (issues #250/#251); it is non-nil only when the fake provider is
	// active, and the HTTP wiring mounts its routes only then — a tkassa
	// (production) build has no such endpoints at all (issue #287).
	FakeConfirms *billinghttp.FakeConfirmHandlers
	// MutationGate adapts the subscription service to the readonly-gate port
	// declared by platform/httpsupport (ADR 0035 consumer-side interface).
	MutationGate httpsupport.SubscriptionMutationChecker
}

// WireBilling constructs the billing repositories, the shared txStoreFactory
// (ADR 0033 γ-factory), every billing service of the rewritten core module
// (issue #245), the payment flows of issue #250, the payment-method flows of
// issue #251 and the active payment provider adapter (issue #248). The grace
// lifecycle events of issue #253 are published through eventDispatcher.
func WireBilling(ctx context.Context, p platformDeps, eventDispatcher platformevents.Dispatcher) (*Billing, error) {
	tariffRepo := billingpg.NewTariffRepository(p.DB, p.Cfg.TariffCacheTTL, p.Clock)
	subscriptionRepo := billingpg.NewSubscriptionRepository(p.DB)
	transitionRepo := billingpg.NewSubscriptionTransitionRepository(p.DB)
	paymentRepo := billingpg.NewSubscriptionPaymentRepository(p.DB, p.Encryptor)
	paymentMethodRepo := billingpg.NewPaymentMethodRepository(p.DB, p.Encryptor)
	cardBindingRepo := billingpg.NewCardBindingSessionRepository(p.DB)

	paymentMetrics, err := payment.NewMetrics()
	if err != nil {
		return nil, fmt.Errorf("payment metrics: %w", err)
	}
	provider, err := wirePaymentProvider(p.Cfg, p.Logger, p.Clock, paymentMetrics)
	if err != nil {
		return nil, err
	}

	factory := billingapp.NewTxStoreFactory(
		tariffRepo,
		subscriptionRepo,
		transitionRepo,
		paymentRepo,
		paymentMethodRepo,
		cardBindingRepo,
		p.AuditRecorder,
		p.UoW,
	)

	services := billingapp.NewServices(factory, billingapp.ServicesConfig{
		Config:        billingapp.DefaultConfig(),
		Clock:         p.Clock,
		Logger:        p.Logger,
		Provider:      provider,
		Publisher:     billingevents.NewPublisher(eventDispatcher),
		AdminPayments: paymentRepo,
	})

	p.Logger.InfoContext(ctx, "billing module initialized",
		"tariff_cache_ttl", p.Cfg.TariffCacheTTL.String(),
		"payment_provider", string(provider.Name()),
	)

	// The local confirmation endpoints exist only under the fake provider:
	// they drive the fake adapter directly at the adapter level while the
	// application layer stays provider-neutral (issue #287). Under any other
	// provider the handlers stay nil and the routes are never mounted.
	var fakeConfirms *billinghttp.FakeConfirmHandlers
	if fakeProvider, ok := provider.(*paymentfake.Provider); ok {
		fakeConfirms = billinghttp.NewFakeConfirmHandlers(fakeProvider, services.Payments, p.Logger)
	}

	return &Billing{
		TariffRepo:        tariffRepo,
		SubscriptionRepo:  subscriptionRepo,
		TransitionRepo:    transitionRepo,
		PaymentRepo:       paymentRepo,
		PaymentMethodRepo: paymentMethodRepo,
		CardBindingRepo:   cardBindingRepo,
		Services:          services,
		PaymentProvider:   provider,
		FakeConfirms:      fakeConfirms,
		MutationGate:      billinghttp.NewMutationGate(services.Subscriptions, p.Clock),
	}, nil
}

// wirePaymentProvider selects the single active provider adapter from the
// configuration (ADR 0038). The adapter itself fails fast on missing
// provider endpoint configuration — the test base URL is deliberately not a
// default (issue #248) — and the wrap below names the env vars so the error
// is actionable in local, where the config layer allows an empty
// T_KASSA_BASE_URL.
func wirePaymentProvider(
	cfg *config.Config, log *slog.Logger, clk clock.Clock, metrics *payment.Metrics,
) (billingapp.PaymentProvider, error) {
	switch cfg.PaymentProvider {
	case providerFake:
		return paymentfake.NewProvider(cfg.AppBaseURL, log, clk, metrics), nil
	case providerTkassa:
		provider, err := paymenttkassa.NewProvider(paymenttkassa.Config{
			BaseURL:        cfg.TKassaBaseURL,
			TerminalKey:    cfg.TKassaTerminalKey,
			Password:       cfg.TKassaPassword,
			AppBaseURL:     cfg.AppBaseURL,
			Timeout:        cfg.TKassaTimeout,
			MaxRetries:     cfg.TKassaMaxRetries,
			RetryBaseDelay: cfg.TKassaRetryBaseDelay,
			RetryMaxDelay:  cfg.TKassaRetryMaxDelay,
		}, log, metrics)
		if err != nil {
			return nil, fmt.Errorf("init tkassa payment provider (check T_KASSA_BASE_URL, T_KASSA_TERMINAL_KEY, "+
				"T_KASSA_PASSWORD, APP_BASE_URL): %w", err)
		}
		return provider, nil
	default:
		return nil, fmt.Errorf("unsupported payment provider %q", cfg.PaymentProvider)
	}
}

// subscriptionViewer is the billing port consumed by the /me enricher glue
// below — the narrow slice of the subscription service it needs, declared at
// the consumer per ADR 0035.
type subscriptionViewer interface {
	GetSubscription(ctx context.Context, userID uuid.UUID) (billingapp.SubscriptionView, error)
}

// BillingMeEnricher returns an identity MeEnricher that adds the current billing
// subscription to a MeResponse. It is the composition-root glue between the
// billing application layer and the identity HTTP layer (MeEnricher); keeping
// it here means identity does not import billing.
func BillingMeEnricher(subscriptions subscriptionViewer) identityhttp.MeEnricher {
	return func(ctx context.Context, userID uuid.UUID, resp *openapi.MeResponse) error {
		if subscriptions == nil {
			return nil
		}
		view, err := subscriptions.GetSubscription(ctx, userID)
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
