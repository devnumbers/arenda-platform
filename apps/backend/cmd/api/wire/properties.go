package wire

import (
	"context"

	billingpg "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/postgres"
	paymentspg "github.com/nambers/arenda-planform/apps/backend/internal/payments/adapters/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/adapters/dadata"
	propertiespg "github.com/nambers/arenda-planform/apps/backend/internal/properties/adapters/postgres"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	storageshared "github.com/nambers/arenda-planform/apps/backend/internal/shared/storage"
)

// Properties holds the properties module's service and the dadata address
// suggester wired by WireProperties.
type Properties struct {
	PropertyService *propertiesapp.PropertyService
	DadataClient    *dadata.Client
}

// WireProperties constructs the properties repositories, the subscription
// limiter (billing-backed), the property service and the dadata address
// suggester. It takes the billing subscription limiter deps and the shared
// private-photo storage (ADR 0065, built once in the composition root).
func WireProperties(
	ctx context.Context,
	p platformDeps,
	billing *Billing,
	photoStorage storageshared.PhotoStorage,
) (*Properties, error) {
	propertyRepo := propertiespg.NewPropertyRepository(p.DB)
	propertyLimiter := billing.Services.Limiter
	limiter := billingpg.NewSubscriptionLimiter(propertyLimiter)

	// The single canonical txStoreFactory bundles the properties
	// repositories, the cross-context ports, the audit and history recorders,
	// and the UoW (ADR 0033 γ-factory). Adding an Nth repository is a change
	// here, not in several constructors.
	factory := propertiesapp.NewTxStoreFactory(
		propertyRepo,
		limiter,
		p.AuditRecorder,
		p.HistoryRecorder,
		p.UoW,
	)

	propertyService := propertiesapp.NewPropertyService(
		propertyRepo,
		photoStorage,
		factory,
		p.Clock,
		p.Policy,
		p.Logger,
	)

	// The reading actor's calendar date for the list responses (ticket #586):
	// the same adapter rentals/payments wire their owner-today reads with.
	propertyService.SetOwnerCalendar(paymentspg.NewOwnerCalendar(p.DB, p.Clock))

	dadataClient := dadata.NewClient(dadata.Config{
		BaseURL:   p.Cfg.DaDataBaseURL,
		APIKey:    p.Cfg.DaDataAPIKey,
		SecretKey: p.Cfg.DaDataSecretKey,
		Timeout:   p.Cfg.DaDataTimeout,
		Logger:    p.Logger,
	})

	return &Properties{
		PropertyService: propertyService,
		DadataClient:    dadataClient,
	}, nil
}
