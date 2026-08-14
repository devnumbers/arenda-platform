package wire

import (
	"context"
	"errors"

	"github.com/google/uuid"
	billinghttp "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/http"
	billingpg "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/postgres"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	identityhttp "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/http"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

// Billing holds the billing module's repositories and services wired by
// WireBilling. It is constructed before the access and properties modules
// because their subscription limiters consume the tariff and subscription
// repositories. The payment provider returns with the provider-port ticket
// (#248); until then the module has no provider-dependent code.
type Billing struct {
	TariffRepo       *billingpg.TariffRepository
	SubscriptionRepo *billingpg.SubscriptionRepository
	TransitionRepo   *billingpg.SubscriptionTransitionRepository
	Services         billingapp.Services
	// MutationGate adapts the subscription service to the readonly-gate port
	// declared by platform/httpsupport (ADR 0035 consumer-side interface).
	MutationGate httpsupport.SubscriptionMutationChecker
}

// WireBilling constructs the billing repositories, the shared txStoreFactory
// (ADR 0033 γ-factory) and every billing service of the rewritten core module
// (issue #245): tariff and subscription views, registration onboarding, the
// property limiter and the worker shells.
func WireBilling(ctx context.Context, p platformDeps) (*Billing, error) {
	tariffRepo := billingpg.NewTariffRepository(p.DB, p.Cfg.TariffCacheTTL, p.Clock)
	subscriptionRepo := billingpg.NewSubscriptionRepository(p.DB)
	transitionRepo := billingpg.NewSubscriptionTransitionRepository(p.DB)

	factory := billingapp.NewTxStoreFactory(
		tariffRepo,
		subscriptionRepo,
		transitionRepo,
		p.AuditRecorder,
		p.UoW,
	)

	services := billingapp.NewServices(factory, billingapp.ServicesConfig{
		Config: billingapp.DefaultConfig(),
		Clock:  p.Clock,
		Logger: p.Logger,
	})
	p.Logger.InfoContext(ctx, "billing module initialized",
		"tariff_cache_ttl", p.Cfg.TariffCacheTTL.String())

	return &Billing{
		TariffRepo:       tariffRepo,
		SubscriptionRepo: subscriptionRepo,
		TransitionRepo:   transitionRepo,
		Services:         services,
		MutationGate:     billinghttp.NewMutationGate(services.Subscriptions, p.Clock),
	}, nil
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
