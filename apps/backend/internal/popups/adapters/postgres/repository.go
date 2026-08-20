// Package postgres persists popup views: recording seen popups and listing the ones still pending per user.
package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/popups/domain"
)

// PopupRepository persists seen popups using generated sqlc queries.
type PopupRepository struct {
	db postgres.DBTX
}

// NewPopupRepository creates a new popup repository.
func NewPopupRepository(db postgres.DBTX) *PopupRepository {
	return &PopupRepository{db: db}
}

func (r *PopupRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// ListSeen returns the keys of the popups the user has already seen.
func (r *PopupRepository) ListSeen(ctx context.Context, userID uuid.UUID) ([]domain.PopupKey, error) {
	keys, err := r.q().ListSeenPopups(ctx, pgconv.UUIDToPgtype(userID))
	if err != nil {
		return nil, err
	}
	out := make([]domain.PopupKey, len(keys))
	for i, k := range keys {
		out[i] = domain.PopupKey(k)
	}
	return out, nil
}

// MarkSeen records that the user has seen the popup; it is idempotent.
func (r *PopupRepository) MarkSeen(ctx context.Context, userID uuid.UUID, key domain.PopupKey) error {
	return r.q().MarkPopupSeen(ctx, postgres.MarkPopupSeenParams{
		UserID:   pgconv.UUIDToPgtype(userID),
		PopupKey: string(key),
	})
}
