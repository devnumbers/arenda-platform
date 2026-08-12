package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// LogoutService terminates sessions.
type LogoutService struct {
	sessions SessionRepository
	hasher   TokenHasher
	uow      transaction.UoW
}

// NewLogoutService creates a LogoutService. uow is the Unit-of-Work seam used by
// runInTx once the service migrates to the transactional-stores pattern
// (ADR 0033); it is optional during the transition.
func NewLogoutService(sessions SessionRepository, hasher TokenHasher, uow transaction.UoW) *LogoutService {
	return &LogoutService{sessions: sessions, hasher: hasher, uow: uow}
}

// Logout deletes the session associated with the raw token.
func (s *LogoutService) Logout(ctx context.Context, rawToken string) error {
	if err := s.sessions.DeleteByTokenHash(ctx, s.hasher.HashToken(rawToken)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// LogoutAll deletes all sessions for the given user.
func (s *LogoutService) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	if err := s.sessions.DeleteByUserID(ctx, userID); err != nil {
		return fmt.Errorf("delete sessions: %w", err)
	}
	return nil
}
