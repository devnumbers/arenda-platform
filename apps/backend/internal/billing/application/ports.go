package application

import (
	"context"
	"errors"

	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

var ErrNotFound = errors.New("not found")

type TariffRepository interface {
	GetByName(ctx context.Context, name domain.TariffName) (domain.Tariff, error)
	WithTx(tx transaction.Tx) TariffRepository
}

type SubscriptionRepository interface {
	Create(ctx context.Context, sub domain.Subscription) error
	WithTx(tx transaction.Tx) SubscriptionRepository
}
