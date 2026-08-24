package wire

import (
	"context"
	"fmt"

	billingpg "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/adapters/dadata"
	propertiespg "github.com/nambers/arenda-planform/apps/backend/internal/properties/adapters/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/adapters/storage"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
)

// Properties holds the properties module's services and the dadata address
// suggester wired by WireProperties.
type Properties struct {
	PropertyService        *propertiesapp.PropertyService
	PropertyContactService *propertiesapp.PropertyContactService
	DadataClient           *dadata.Client
}

// WireProperties constructs the properties repositories, the subscription
// limiter (billing-backed), the photo storage (S3 or fake based on config), the
// property and property-contact services and the dadata address suggester. It
// takes the billing subscription limiter deps.
func WireProperties(
	ctx context.Context,
	p platformDeps,
	billing *Billing,
) (*Properties, error) {
	propertyRepo := propertiespg.NewPropertyRepository(p.DB)
	propertyPhotoRepo := propertiespg.NewPropertyPhotoRepository(p.DB)
	propertyContactRepo := propertiespg.NewPropertyContactRepository(p.DB)
	propertyLimiter := billing.Services.Limiter
	limiter := billingpg.NewSubscriptionLimiter(propertyLimiter)

	var photoStorage propertiesapp.PhotoStorage
	if p.Cfg.PhotoStorageS3Enabled {
		var err error
		photoStorage, err = storage.NewS3Storage(
			ctx,
			p.Cfg.PhotoStorageEndpoint,
			p.Cfg.PhotoStorageRegion,
			p.Cfg.PhotoStorageBucket,
			p.Cfg.PhotoStorageAccessKey,
			p.Cfg.PhotoStorageSecretKey,
			p.Cfg.PhotoStoragePublicBaseURL,
			p.Cfg.PhotoStoragePathStyle,
		)
		if err != nil {
			return nil, fmt.Errorf("photo storage: %w", err)
		}
		if err := photoStorage.HeadBucket(ctx); err != nil {
			return nil, fmt.Errorf("photo storage: head bucket %q: %w", p.Cfg.PhotoStorageBucket, err)
		}
		p.Logger.InfoContext(ctx, "photo storage initialized",
			"provider", "s3", "bucket", p.Cfg.PhotoStorageBucket, "endpoint", p.Cfg.PhotoStorageEndpoint)
	} else {
		photoStorage = storage.NewFakeStorage(p.Cfg.PhotoStoragePublicBaseURL)
		p.Logger.InfoContext(ctx, "photo storage initialized", "provider", "fake")
	}

	// The single canonical txStoreFactory bundles the properties
	// repositories, the cross-context ports, the audit recorder, and the UoW
	// (ADR 0033 γ-factory). It is passed to both properties services so adding
	// an Nth repository is a change here, not in several constructors.
	factory := propertiesapp.NewTxStoreFactory(
		propertyRepo,
		propertyPhotoRepo,
		propertyContactRepo,
		limiter,
		p.AuditRecorder,
		p.UoW,
	)

	propertyService := propertiesapp.NewPropertyService(
		propertyRepo,
		propertyPhotoRepo,
		photoStorage,
		factory,
		p.Clock,
		p.Policy,
		p.Logger,
	)

	propertyContactService := propertiesapp.NewPropertyContactService(propertyContactRepo, propertyRepo, factory, p.Logger)

	dadataClient := dadata.NewClient(dadata.Config{
		BaseURL:   p.Cfg.DaDataBaseURL,
		APIKey:    p.Cfg.DaDataAPIKey,
		SecretKey: p.Cfg.DaDataSecretKey,
		Timeout:   p.Cfg.DaDataTimeout,
		Logger:    p.Logger,
	})

	return &Properties{
		PropertyService:        propertyService,
		PropertyContactService: propertyContactService,
		DadataClient:           dadataClient,
	}, nil
}
