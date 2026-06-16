package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// OnboardingService creates default billing state for newly-registered owners.
type OnboardingService struct {
	tariffs       application.TariffRepository
	subscriptions application.SubscriptionRepository
}

// NewOnboardingService creates an onboarding service backed by postgres repositories.
func NewOnboardingService(tariffs application.TariffRepository, subscriptions application.SubscriptionRepository) *OnboardingService {
	return &OnboardingService{
		tariffs:       tariffs,
		subscriptions: subscriptions,
	}
}

// SetupDefaultSubscription creates the default owner subscription for a new user inside the given transaction.
func (s *OnboardingService) SetupDefaultSubscription(ctx context.Context, tx transaction.Tx, userID uuid.UUID) error {
	tariff, err := s.tariffs.WithTx(tx).GetByName(ctx, domain.TariffBasic)
	if err != nil {
		return fmt.Errorf("get basic tariff: %w", err)
	}
	sub, err := domain.NewOwnerSubscription(userID, tariff.ID)
	if err != nil {
		return fmt.Errorf("create subscription: %w", err)
	}
	if err := s.subscriptions.WithTx(tx).Create(ctx, sub); err != nil {
		return fmt.Errorf("save subscription: %w", err)
	}
	return nil
}
