package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type SubscriptionLimiter interface {
	ActivePropertyLimit(ctx context.Context, userID uuid.UUID) (int, error)
}

type PropertyRepository interface {
	Create(ctx context.Context, ownerID uuid.UUID, property domain.Property) (domain.Property, error)
	GetByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (domain.Property, error)
	ListActiveByOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.Property, error)
	Update(ctx context.Context, ownerID uuid.UUID, property domain.Property) (domain.Property, error)
	Archive(ctx context.Context, id, ownerID uuid.UUID) error
	Unarchive(ctx context.Context, id, ownerID uuid.UUID) error
	CountActiveByOwner(ctx context.Context, ownerID uuid.UUID) (int, error)
	WithTx(tx transaction.Tx) PropertyRepository
}
