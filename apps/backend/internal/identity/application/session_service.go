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
	Load(ctx context.Context, rawToken string, now time.Time) (domain.Session, domain.User, error)
	Update(ctx context.Context, session domain.Session) error
}

type sessionService struct {
	sessions SessionRepository
	hasher   TokenHasher
}

// NewSessionService creates a SessionService backed by the provided repository.
// The hasher is used to look up sessions by the hashed value of the raw token.
func NewSessionService(sessions SessionRepository, hasher TokenHasher) *sessionService {
	return &sessionService{sessions: sessions, hasher: hasher}
}

func (s *sessionService) Load(ctx context.Context, rawToken string, now time.Time) (domain.Session, domain.User, error) {
	return s.sessions.GetByTokenHash(ctx, s.hasher.HashToken(rawToken), now)
}

func (s *sessionService) Update(ctx context.Context, session domain.Session) error {
	return s.sessions.Update(ctx, session)
}
