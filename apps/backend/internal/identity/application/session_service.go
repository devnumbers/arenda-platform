package application

import (
	"context"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// SessionService is the application-layer facade for session lookups and updates.
// It wraps the SessionRepository so that transport code does not depend directly
// on persistence details.
type SessionService interface {
	Load(ctx context.Context, tokenHash string, now time.Time) (domain.Session, domain.User, error)
	Update(ctx context.Context, session domain.Session) error
}

type sessionService struct {
	sessions SessionRepository
}

// NewSessionService creates a SessionService backed by the provided repository.
func NewSessionService(sessions SessionRepository) SessionService {
	return &sessionService{sessions: sessions}
}

func (s *sessionService) Load(ctx context.Context, tokenHash string, now time.Time) (domain.Session, domain.User, error) {
	return s.sessions.GetByTokenHash(ctx, tokenHash, now)
}

func (s *sessionService) Update(ctx context.Context, session domain.Session) error {
	return s.sessions.Update(ctx, session)
}
