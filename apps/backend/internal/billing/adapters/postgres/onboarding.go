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
	beginner      transaction.Beginner
}

// NewOnboardingService creates an onboarding service backed by postgres repositories.
func NewOnboardingService(tariffs application.TariffRepository, subscriptions application.SubscriptionRepository, beginner transaction.Beginner) *OnboardingService {
	return &OnboardingService{
		tariffs:       tariffs,
		subscriptions: subscriptions,
		beginner:      beginner,
	}
}

// SetupDefaultSubscription creates the default owner subscription for a new user.
// The owner starts on the free basic plan: paid source, active status, no renewal, and no expiration.
func (s *OnboardingService) SetupDefaultSubscription(ctx context.Context, userID uuid.UUID) error {
	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin onboarding transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txTariffs, err := s.tariffs.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind tariffs transaction: %w", err)
	}
	tariff, err := txTariffs.GetByName(ctx, domain.TariffBasic)
	if err != nil {
		return fmt.Errorf("get basic tariff: %w", err)
	}
	sub, err := domain.NewOwnerSubscription(userID, tariff.ID)
	if err != nil {
		return fmt.Errorf("create subscription: %w", err)
	}

	txSubscriptions, err := s.subscriptions.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind subscriptions transaction: %w", err)
	}
	if _, err := txSubscriptions.Create(ctx, sub); err != nil {
		return fmt.Errorf("save subscription: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit onboarding transaction: %w", err)
	}
	return nil
}
