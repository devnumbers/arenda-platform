package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const SessionTTL = 30 * 24 * time.Hour

type Session struct {
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
}

type RawSession struct {
	Token   string
	Session Session
}

func (s Session) IsExpired(now time.Time) bool {
	return now.After(s.ExpiresAt)
}

func NewSession(userID uuid.UUID, now time.Time) (RawSession, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return RawSession{}, fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(token))
	return RawSession{
		Token: token,
		Session: Session{
			UserID:    userID,
			TokenHash: hex.EncodeToString(sum[:]),
			ExpiresAt: now.Add(SessionTTL),
		},
	}, nil
}
