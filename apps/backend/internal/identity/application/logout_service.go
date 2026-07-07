package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

// LogoutService terminates sessions.
type LogoutService struct {
	sessions SessionRepository
	hasher   TokenHasher
	logger   *slog.Logger
}

// NewLogoutService creates a LogoutService.
func NewLogoutService(sessions SessionRepository, hasher TokenHasher, logger *slog.Logger) *LogoutService {
	if logger == nil {
		logger = slog.Default()
	}
	return &LogoutService{sessions: sessions, hasher: hasher, logger: logger}
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
