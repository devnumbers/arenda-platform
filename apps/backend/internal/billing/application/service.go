package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// ErrSubscriptionAlreadyExists is returned by SubscriptionRepository.Create
// when a subscription for the user already exists. It is kept in the billing
// context and must not be referenced by other contexts.
var ErrSubscriptionAlreadyExists = errors.New("subscription already exists")

type Service struct {
	tariffs       TariffRepository
	subscriptions SubscriptionRepository
}

func NewService(tariffs TariffRepository, subscriptions SubscriptionRepository) *Service {
	return &Service{tariffs: tariffs, subscriptions: subscriptions}
}

func (s *Service) CreateDefaultSubscriptionForOwner(ctx context.Context, tx transaction.Tx, userID uuid.UUID) error {
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
