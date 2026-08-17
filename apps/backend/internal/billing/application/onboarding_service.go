package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
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
//
// The event may be redelivered and two deliveries may race; both paths are
// idempotent — an existing subscription is left untouched and no second
// transition is recorded. The whole onboarding is one transaction: either the
// subscription and its transition land together or neither does.
func (s *OnboardingService) OnUserRegistered(ctx context.Context, userID uuid.UUID) error {
	return s.runInTx(ctx, func(stores *txStores) error {
		if _, err := stores.subscriptions.GetByUserID(ctx, userID); err == nil {
			return nil
		} else if !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("get subscription: %w", err)
		}

		tariff, err := stores.tariffs.GetByName(ctx, domain.TariffBasic)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return fmt.Errorf("basic tariff seed is missing: %w", ErrTariffNotFound)
			}
			return fmt.Errorf("get basic tariff: %w", err)
		}

		sub, err := domain.NewBasicSubscription(userID, tariff.ID)
		if err != nil {
			return fmt.Errorf("build basic subscription: %w", err)
		}
		created, err := stores.subscriptions.Create(ctx, sub)
		if err != nil {
			return fmt.Errorf("save subscription: %w", err)
		}
		if created.ID != sub.ID {
			// A concurrent delivery won the create race; the repository
			// returned the existing row. Leave it — and its transition —
			// untouched.
			return nil
		}

		transition, err := domain.NewTransition(created, nil, nil, domain.TransitionReasonRegistered, domain.InitiatorSystem, nil)
		if err != nil {
			return fmt.Errorf("build registration transition: %w", err)
		}
		if err := stores.transitions.Append(ctx, transition); err != nil {
			return fmt.Errorf("append registration transition: %w", err)
		}
		s.logger.InfoContext(ctx, "basic subscription created",
			"user_id", userID.String(),
			"subscription_id", created.ID.String(),
			"tariff_id", tariff.ID.String())
		return nil
	})
}
