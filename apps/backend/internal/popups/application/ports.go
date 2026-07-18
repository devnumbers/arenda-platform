package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/popups/domain"
)

// PopupRepository persists the popups a user has already seen.
type PopupRepository interface {
	// ListSeen returns the keys of the popups the user has already seen.
	ListSeen(ctx context.Context, userID uuid.UUID) ([]domain.PopupKey, error)
	// MarkSeen records that the user has seen the popup. It is idempotent:
	// recording the same popup twice is a no-op.
	MarkSeen(ctx context.Context, userID uuid.UUID, key domain.PopupKey) error
}
