package application

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
)

// OnboardingService creates the default billing state for newly-registered
// owners: the free basic subscription (issue #245). It is subscribed to the
// user_registered in-process event at the composition root.
type OnboardingService struct {
	txStoreFactory
	logger *slog.Logger
}

// OnboardingServiceConfig carries the non-transactional dependencies of the
// onboarding service.
type OnboardingServiceConfig struct {
	Logger *slog.Logger
}

// NewOnboardingService creates an onboarding service over the shared factory.
func NewOnboardingService(factory txStoreFactory, cfg OnboardingServiceConfig) *OnboardingService {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &OnboardingService{txStoreFactory: factory, logger: cfg.Logger}
}

// OnUserRegistered creates the owner's basic subscription: active, paid
// source, no expiry, auto-renew off (ADR 0008), and appends the first entry to
// the subscription transition log (reason "registered", system initiator).
// The creation itself is the ensureBasicSubscription seam (issue #1516) shared
// with the admin tools that onboard a missing row the same way.
//
// The event may be redelivered and two deliveries may race; both paths are
// idempotent — an existing subscription is left untouched and no second
// transition is recorded. The whole onboarding is one transaction: either the
// subscription and its transition land together or neither does.
func (s *OnboardingService) OnUserRegistered(ctx context.Context, userID uuid.UUID) error {
	return s.runInTx(ctx, func(stores *txStores) error {
		sub, created, err := stores.ensureBasicSubscription(ctx, userID)
		if err != nil {
			return err
		}
		if created {
			s.logger.InfoContext(ctx, "basic subscription created",
				"user_id", userID.String(),
				"subscription_id", sub.ID.String(),
				"tariff_id", sub.TariffID.String())
		}
		return nil
	})
}
