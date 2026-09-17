package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// SessionsService serves the devices list: it reads the user's sessions,
// terminates one of them, or terminates every session except the current one.
// Unlike logout (fail-open, ADR 0020), revocations keep the default fail-safe
// audit: the entry and the delete share one transaction, and an audit failure
// rolls the revocation back.
type SessionsService struct {
	txStoreFactory
	hasher TokenHasher
}

// SessionsServiceConfig carries the non-transactional dependencies. The
// transactional repositories, audit recorder, and UoW live in the shared
// txStoreFactory passed to NewSessionsService.
type SessionsServiceConfig struct {
	Hasher TokenHasher
}

// NewSessionsService creates a SessionsService on the shared identity
// txStoreFactory (ADR 0033 γ-factory).
func NewSessionsService(factory txStoreFactory, cfg SessionsServiceConfig) *SessionsService {
	return &SessionsService{
		txStoreFactory: factory,
		hasher:         cfg.Hasher,
	}
}

// List returns the user's sessions (most recent activity first) and the ID of
// the session the presented token belongs to, so the transport can flag the
// current row without touching token hashes itself.
func (s *SessionsService) List(ctx context.Context, userID uuid.UUID, currentToken string) ([]domain.Session, uuid.UUID, error) {
	sessions, err := s.sessions.ListByUserID(ctx, userID)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("list sessions: %w", err)
	}
	currentHash := s.hasher.HashToken(currentToken)
	currentID := uuid.Nil
	for _, sess := range sessions {
		if sess.TokenHash == currentHash {
			currentID = sess.ID
			break
		}
	}
	return sessions, currentID, nil
}

// Revoke terminates one session of the user. Revoking the session carrying
// currentToken is rejected with ErrCurrentSession — the devices list ends the
// current session through logout. An unknown session and a session owned by
// somebody else both answer ErrNotFound, so existence is not revealed.
func (s *SessionsService) Revoke(ctx context.Context, userID, sessionID uuid.UUID, currentToken string, actor auditdomain.Actor) error {
	return s.runInTx(ctx, func(stores *txStores) error {
		sess, err := stores.sessions.GetByID(ctx, sessionID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("get session: %w", err)
		}
		if sess.UserID != userID {
			return ErrNotFound
		}
		if sess.TokenHash == s.hasher.HashToken(currentToken) {
			return ErrCurrentSession
		}

		removed, err := stores.sessions.DeleteByIDForUser(ctx, sessionID, userID)
		if err != nil {
			return fmt.Errorf("delete session: %w", err)
		}
		if !removed {
			return ErrNotFound
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor.ID,
			ActorRole:  actor.Role,
			Action:     auditdomain.ActionAuthSessionRevoked,
			EntityType: auditdomain.EntityUser,
			EntityID:   &userID,
			Context:    map[string]any{"session_id": sessionID.String()},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
}

// RevokeOthers terminates every session of the user except the one carrying
// currentToken and reports how many were removed.
func (s *SessionsService) RevokeOthers(ctx context.Context, userID uuid.UUID, currentToken string, actor auditdomain.Actor) (int64, error) {
	removed := int64(0)
	err := s.runInTx(ctx, func(stores *txStores) error {
		n, err := stores.sessions.DeleteByUserIDExcept(ctx, userID, s.hasher.HashToken(currentToken))
		if err != nil {
			return fmt.Errorf("delete other sessions: %w", err)
		}
		removed = n
		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor.ID,
			ActorRole:  actor.Role,
			Action:     auditdomain.ActionAuthOtherSessionsRevoked,
			EntityType: auditdomain.EntityUser,
			EntityID:   &userID,
			Context:    map[string]any{"count": n},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return removed, nil
}
