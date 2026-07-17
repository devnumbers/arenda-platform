package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	// SessionBaseTTL is the per-request sliding window for an active session.
	SessionBaseTTL = 7 * 24 * time.Hour
	// SessionMaxTTL is the hard upper bound for a session since creation.
	SessionMaxTTL = 30 * 24 * time.Hour
)

type Session struct {
	ID     uuid.UUID
	UserID uuid.UUID
	// TokenHash is intentionally left empty by NewSession. The caller must hash
	// the raw token (RawSession.Token) and set this field before persisting.
	TokenHash  string
	ExpiresAt  time.Time
	CreatedAt  time.Time
	LastUsedAt time.Time
}

type RawSession struct {
	Token   string
	Session Session
}

func (s *Session) IsExpired(now time.Time) bool {
	return now.After(s.ExpiresAt)
}

// Refresh extends the session expiration by SessionBaseTTL, capped at
// CreatedAt + SessionMaxTTL. It returns true when the expiration was actually
// moved forward. Expired sessions are never refreshed.
func (s *Session) Refresh(now time.Time) bool {
	if s.IsExpired(now) {
		return false
	}

	maxExpires := s.CreatedAt.Add(SessionMaxTTL)
	candidate := now.Add(SessionBaseTTL)
	if candidate.After(maxExpires) {
		candidate = maxExpires
	}
	if !candidate.After(s.ExpiresAt) {
		return false
	}

	s.ExpiresAt = candidate
	s.LastUsedAt = now
	return true
}

// NewSession creates a new session and a raw token. The returned Session has
// TokenHash left empty; the caller must hash RawSession.Token and assign the
// hash to Session.TokenHash before persisting it.
func NewSession(userID uuid.UUID, now time.Time) (RawSession, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return RawSession{}, fmt.Errorf("generate token: %w", err)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return RawSession{}, fmt.Errorf("generate session id: %w", err)
	}
	token := hex.EncodeToString(b)
	return RawSession{
		Token: token,
		Session: Session{
			ID:         id,
			UserID:     userID,
			TokenHash:  "",
			ExpiresAt:  now.Add(SessionBaseTTL),
			CreatedAt:  now,
			LastUsedAt: now,
		},
	}, nil
}
