// Package application holds the popup use cases and ports: listing popups still pending for a user and
// recording that a popup has been seen.
package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/popups/domain"
)

// PopupService implements the popup use cases: listing the popups still
// pending for a user and recording that a popup has been seen.
type PopupService struct {
	repo PopupRepository
}

// NewPopupService creates a new popup service.
func NewPopupService(repo PopupRepository) *PopupService {
	return &PopupService{repo: repo}
}

// ListPending returns the active popups the user has not seen yet, in display
// order. The result may be empty.
func (s *PopupService) ListPending(ctx context.Context, userID uuid.UUID) ([]domain.PopupKey, error) {
	seen, err := s.repo.ListSeen(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list seen popups: %w", err)
	}
	return pendingPopups(seen), nil
}

// MarkSeen records that the user has seen the popup. Unknown keys are
// rejected; recording an already-seen popup is a no-op.
func (s *PopupService) MarkSeen(ctx context.Context, userID uuid.UUID, key domain.PopupKey) error {
	if !key.IsValid() {
		return fmt.Errorf("%w: %q", ErrUnknownPopupKey, key)
	}
	if err := s.repo.MarkSeen(ctx, userID, key); err != nil {
		return fmt.Errorf("mark popup seen: %w", err)
	}
	return nil
}

// pendingPopups returns the active popups that are not in the seen set,
// keeping the registry display order.
func pendingPopups(seen []domain.PopupKey) []domain.PopupKey {
	seenSet := make(map[domain.PopupKey]struct{}, len(seen))
	for _, k := range seen {
		seenSet[k] = struct{}{}
	}
	active := domain.ActivePopups()
	pending := make([]domain.PopupKey, 0, len(active))
	for _, k := range active {
		if _, ok := seenSet[k]; !ok {
			pending = append(pending, k)
		}
	}
	return pending
}
