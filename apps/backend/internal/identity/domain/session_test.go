package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSession_IsExpired(t *testing.T) {
	t.Parallel()
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := Session{
		ID:         uuid.Must(uuid.NewV7()),
		ExpiresAt:  created.Add(SessionBaseTTL),
		CreatedAt:  created,
		LastUsedAt: created,
	}

	tests := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"before expiry", created.Add(time.Hour), false},
		{"exactly at expiry", s.ExpiresAt, false},
		{"after expiry", s.ExpiresAt.Add(time.Nanosecond), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := s.IsExpired(tc.now); got != tc.want {
				t.Fatalf("IsExpired(%v) = %v, want %v", tc.now, got, tc.want)
			}
		})
	}
}

func TestSession_Refresh(t *testing.T) {
	t.Parallel()
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Single-refresh cases: each row seeds a session whose ExpiresAt sits at
	// expiresOff from CreatedAt, refreshes it at nowOff from CreatedAt, and
	// pins the whole post-refresh state. WantOff durations are relative to
	// CreatedAt too; a zero wantLastUsedOff means LastUsedAt stays at CreatedAt
	// (Refresh must not touch it on a no-op).
	tests := []struct {
		name            string
		expiresOff      time.Duration
		nowOff          time.Duration
		want            bool
		wantExpiresOff  time.Duration
		wantLastUsedOff time.Duration
	}{
		{
			name:            "extends expiration by SessionBaseTTL",
			expiresOff:      SessionBaseTTL,
			nowOff:          1 * time.Hour,
			want:            true,
			wantExpiresOff:  SessionBaseTTL + 1*time.Hour,
			wantLastUsedOff: 1 * time.Hour,
		},
		{
			// Session is still active but late in its lifetime: now is 6 days
			// before the hard cap (so now + SessionBaseTTL exceeds it), and
			// ExpiresAt is 3 days before the cap (still active). Refresh must
			// clamp to CreatedAt + SessionMaxTTL.
			name:            "caps expiration at CreatedAt + SessionMaxTTL",
			expiresOff:      SessionMaxTTL - 3*24*time.Hour,
			nowOff:          SessionMaxTTL - 6*24*time.Hour,
			want:            true,
			wantExpiresOff:  SessionMaxTTL,
			wantLastUsedOff: SessionMaxTTL - 6*24*time.Hour,
		},
		{
			name:            "expired session is never refreshed",
			expiresOff:      SessionBaseTTL,
			nowOff:          SessionBaseTTL + time.Second,
			want:            false,
			wantExpiresOff:  SessionBaseTTL,
			wantLastUsedOff: 0,
		},
		{
			// Session already at the hard cap: candidate equals the current
			// ExpiresAt, so candidate.After(ExpiresAt) is false and neither
			// field moves.
			name:            "returns false when expiration does not move forward",
			expiresOff:      SessionMaxTTL,
			nowOff:          SessionMaxTTL - SessionBaseTTL + time.Hour,
			want:            false,
			wantExpiresOff:  SessionMaxTTL,
			wantLastUsedOff: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := Session{
				ID:         uuid.Must(uuid.NewV7()),
				ExpiresAt:  created.Add(tc.expiresOff),
				CreatedAt:  created,
				LastUsedAt: created,
			}
			now := created.Add(tc.nowOff)

			if got := s.Refresh(now); got != tc.want {
				t.Fatalf("Refresh = %v, want %v", got, tc.want)
			}
			if !s.ExpiresAt.Equal(created.Add(tc.wantExpiresOff)) {
				t.Fatalf("ExpiresAt = %v, want %v", s.ExpiresAt, created.Add(tc.wantExpiresOff))
			}
			if !s.LastUsedAt.Equal(created.Add(tc.wantLastUsedOff)) {
				t.Fatalf("LastUsedAt = %v, want %v", s.LastUsedAt, created.Add(tc.wantLastUsedOff))
			}
		})
	}

	t.Run("updates LastUsedAt on every successful refresh", func(t *testing.T) {
		s := Session{
			ID:         uuid.Must(uuid.NewV7()),
			ExpiresAt:  created.Add(SessionBaseTTL),
			CreatedAt:  created,
			LastUsedAt: created,
		}
		first := created.Add(10 * time.Minute)
		second := created.Add(20 * time.Minute)

		s.Refresh(first)
		if !s.LastUsedAt.Equal(first) {
			t.Fatalf("LastUsedAt after first = %v, want %v", s.LastUsedAt, first)
		}
		s.Refresh(second)
		if !s.LastUsedAt.Equal(second) {
			t.Fatalf("LastUsedAt after second = %v, want %v", s.LastUsedAt, second)
		}
	})
}

func TestNewSession(t *testing.T) {
	t.Parallel()

	userID := uuid.Must(uuid.NewV7())
	now := time.Date(2026, 1, 15, 8, 30, 0, 0, time.UTC)

	raw, err := NewSession(userID, now)
	if err != nil {
		t.Fatalf("NewSession error = %v", err)
	}

	// The raw token is a 64-char hex string (32 random bytes → 64 hex chars).
	if len(raw.Token) != 64 {
		t.Fatalf("Token length = %d, want 64", len(raw.Token))
	}
	// Two consecutive calls must produce different tokens.
	other, err := NewSession(userID, now)
	if err != nil {
		t.Fatalf("second NewSession error = %v", err)
	}
	if other.Token == raw.Token {
		t.Fatal("two NewSession calls produced identical tokens")
	}

	sess := raw.Session
	if sess.ID == (uuid.UUID{}) {
		t.Fatal("session ID is zero")
	}
	if sess.UserID != userID {
		t.Fatalf("UserID = %s, want %s", sess.UserID, userID)
	}
	// TokenHash is deliberately left empty — the caller hashes raw.Token before
	// persisting.
	if sess.TokenHash != "" {
		t.Fatalf("TokenHash = %q, want empty (caller must hash before persisting)", sess.TokenHash)
	}
	if !sess.ExpiresAt.Equal(now.Add(SessionBaseTTL)) {
		t.Fatalf("ExpiresAt = %v, want %v", sess.ExpiresAt, now.Add(SessionBaseTTL))
	}
	if !sess.CreatedAt.Equal(now) {
		t.Fatalf("CreatedAt = %v, want %v", sess.CreatedAt, now)
	}
	if !sess.LastUsedAt.Equal(now) {
		t.Fatalf("LastUsedAt = %v, want %v", sess.LastUsedAt, now)
	}
}
