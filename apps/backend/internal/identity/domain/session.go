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
	// SessionRenewalInterval is how often the session token rotates (the OWASP
	// renewal timeout that compensates for the absent absolute cap, ADR 0056):
	// every 14 days of session life the token and its hash are replaced.
	SessionRenewalInterval = 14 * 24 * time.Hour
	// SessionRotationGrace keeps the previous token accepted for a short moment
	// after a rotation so in-flight requests carrying the old cookie survive
	// the swap.
	SessionRotationGrace = 2 * time.Minute
)

// Session is a server-side opaque browser session (ADR 0004). Its lifetime
// slides: every authenticated use pushes ExpiresAt forward by SessionBaseTTL,
// with no absolute cap — an active session never expires (ADR 0056).
type Session struct {
	ID     uuid.UUID
	UserID uuid.UUID
	// TokenHash is intentionally left empty by NewSession. The caller must hash
	// the raw token (RawSession.Token) and set this field before persisting.
	TokenHash string
	// PreviousTokenHash is the hash the token had before the most recent
	// rotation. It stays accepted for SessionRotationGrace after RotatedAt so
	// in-flight requests survive the cookie swap; empty until the first
	// rotation.
	PreviousTokenHash string
	ExpiresAt         time.Time
	CreatedAt         time.Time
	LastUsedAt        time.Time
	// RotatedAt is when the session token was last replaced; creation counts as
	// the first rotation and seeds the renewal window.
	RotatedAt time.Time
	// LastIP is the client IP of the most recent request. Empty when unknown.
	LastIP string
	// City is the GeoIP-resolved city for LastIP in the ru locale (fallback
	// en). Empty when the IP is unknown, private, or missing from the base.
	City string

	// Device description captured once, at session creation.
	UserAgent    string
	DeviceType   DeviceType
	Browser      string
	BrowserMajor int
	OS           string
}

type RawSession struct {
	Token   string
	Session Session
}

func (s *Session) IsExpired(now time.Time) bool {
	return now.After(s.ExpiresAt)
}

// Refresh extends the session expiration to now + SessionBaseTTL whenever that
// moves the expiration forward. There is no cap: an actively used session
// slides forever (ADR 0056). It returns true when the expiration was actually
// moved forward. Expired sessions are never refreshed.
func (s *Session) Refresh(now time.Time) bool {
	if s.IsExpired(now) {
		return false
	}

	candidate := now.Add(SessionBaseTTL)
	if !candidate.After(s.ExpiresAt) {
		return false
	}

	s.ExpiresAt = candidate
	s.LastUsedAt = now
	return true
}

// RenewalDue reports whether the session token must rotate: at least
// SessionRenewalInterval has passed since the last rotation (creation counts
// as the first rotation).
func (s *Session) RenewalDue(now time.Time) bool {
	return now.Sub(s.RotatedAt) >= SessionRenewalInterval
}

// NewToken generates a fresh opaque session token: 32 random bytes as
// lowercase hex (256 bits of entropy).
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// NewSession creates a new session and a raw token. The returned Session has
// TokenHash left empty; the caller must hash RawSession.Token and assign the
// hash to Session.TokenHash before persisting it.
func NewSession(userID uuid.UUID, now time.Time) (RawSession, error) {
	token, err := NewToken()
	if err != nil {
		return RawSession{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return RawSession{}, fmt.Errorf("generate session id: %w", err)
	}
	return RawSession{
		Token: token,
		Session: Session{
			ID:         id,
			UserID:     userID,
			TokenHash:  "",
			ExpiresAt:  now.Add(SessionBaseTTL),
			CreatedAt:  now,
			LastUsedAt: now,
			RotatedAt:  now,
		},
	}, nil
}
