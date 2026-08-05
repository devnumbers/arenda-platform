package wire

import (
	"context"
	"fmt"

	billingpg "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/adapters/dadata"
	propertiespg "github.com/nambers/arenda-planform/apps/backend/internal/properties/adapters/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/adapters/storage"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
)

// Properties holds the properties module's services, the dadata address
// suggester and the occupancy provider wired by WireProperties.
type Properties struct {
	PropertyService        *propertiesapp.PropertyService
	PropertyContactService *propertiesapp.PropertyContactService
	DadataClient           *dadata.Client
	OccupancyProvider      *propertiespg.OccupancyProvider
}

// WireProperties constructs the properties repositories, the subscription
// limiter (billing-backed), the photo storage (S3 or fake based on config), the
// property and property-contact services and the dadata address suggester. It
// takes the billing subscription limiter deps and the leases repos (for the
// property billing lifecycle and shared lease repo).
func WireProperties(
	ctx context.Context,
	p platformDeps,
	billingRepos *BillingRepos,
	leasesRepos *LeasesRepos,
) (*Properties, error) {
	propertyRepo := propertiespg.NewPropertyRepository(p.DB)
	propertyPhotoRepo := propertiespg.NewPropertyPhotoRepository(p.DB)
	occupancyProvider := propertiespg.NewOccupancyProvider(p.DB)
	propertyLimiter := application.NewSubscriptionPropertyLimiter(billingRepos.SubscriptionRepo, billingRepos.TariffRepo)
	limiter := billingpg.NewSubscriptionLimiter(propertyLimiter)

	var photoStorage propertiesapp.PhotoStorage
	if p.Cfg.PhotoStorageS3Enabled {
		var err error
		photoStorage, err = storage.NewS3Storage( //nolint:contextcheck // constructor uses context.Background internally; signature unchanged from original
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
		p.Logger.InfoContext(ctx, "photo storage initialized", "provider", "s3", "bucket", p.Cfg.PhotoStorageBucket, "endpoint", p.Cfg.PhotoStorageEndpoint)
	} else {
		photoStorage = storage.NewFakeStorage(p.Cfg.PhotoStoragePublicBaseURL)
		p.Logger.InfoContext(ctx, "photo storage initialized", "provider", "fake")
	}

	propertyService := propertiesapp.NewPropertyService(
		propertyRepo,
		propertyPhotoRepo,
		photoStorage,
		occupancyProvider,
		limiter,
		leasesRepos.PropertyBillingLifecycle,
		leasesRepos.LeaseRepo,
		p.Beginner,
		p.AuditRecorder,
		p.Clock,
		p.TZResolver,
		p.Logger,
	)

	propertyContactRepo := propertiespg.NewPropertyContactRepository(p.DB)
	propertyContactService := propertiesapp.NewPropertyContactService(propertyContactRepo, propertyRepo, p.Beginner, p.AuditRecorder, p.Logger)

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
		OccupancyProvider:      occupancyProvider,
	}, nil
}
