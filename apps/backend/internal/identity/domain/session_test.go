package domain

import (
	"strings"
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
			// Pure sliding (ADR 0056): there is no absolute cap, so an active
			// session keeps sliding past the old 30-day bound — a session last
			// refreshed on day 90 is pushed to day 96.
			name:            "slides past the old 30-day cap without limit",
			expiresOff:      90 * 24 * time.Hour,
			nowOff:          90*24*time.Hour - 24*time.Hour,
			want:            true,
			wantExpiresOff:  96 * 24 * time.Hour,
			wantLastUsedOff: 90*24*time.Hour - 24*time.Hour,
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
			// Candidate does not move the expiration forward: neither field
			// changes and Refresh reports a no-op.
			name:            "returns false when expiration does not move forward",
			expiresOff:      8 * 24 * time.Hour,
			nowOff:          24 * time.Hour,
			want:            false,
			wantExpiresOff:  8 * 24 * time.Hour,
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

func TestSession_RenewalDue(t *testing.T) {
	t.Parallel()
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		rotatedAt time.Time
		now       time.Time
		want      bool
	}{
		{
			name:      "young session is not due",
			rotatedAt: created,
			now:       created.Add(SessionRenewalInterval - time.Second),
			want:      false,
		},
		{
			name:      "exactly at the renewal interval the rotation is due",
			rotatedAt: created,
			now:       created.Add(SessionRenewalInterval),
			want:      true,
		},
		{
			name:      "long-lived session is due",
			rotatedAt: created,
			now:       created.Add(90 * 24 * time.Hour),
			want:      true,
		},
		{
			name:      "recently rotated session is not due again",
			rotatedAt: created.Add(89 * 24 * time.Hour),
			now:       created.Add(90 * 24 * time.Hour),
			want:      false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := Session{RotatedAt: tc.rotatedAt}
			if got := s.RenewalDue(tc.now); got != tc.want {
				t.Fatalf("RenewalDue(%v) = %v, want %v", tc.now, got, tc.want)
			}
		})
	}
}

func TestNewToken(t *testing.T) {
	t.Parallel()

	token, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken error = %v", err)
	}
	// 32 random bytes → 64 lowercase hex characters, same shape as the
	// issuance token in NewSession.
	if len(token) != 64 {
		t.Fatalf("token length = %d, want 64", len(token))
	}
	const hexDigits = "0123456789abcdef"
	if strings.Trim(token, hexDigits) != "" {
		t.Fatalf("token %q is not lowercase hex", token)
	}

	other, err := NewToken()
	if err != nil {
		t.Fatalf("second NewToken error = %v", err)
	}
	if other == token {
		t.Fatal("two NewToken calls produced identical tokens")
	}
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
	// Creation is rotation zero: a fresh session is never immediately due for
	// a token rotation and has no previous token.
	if !sess.RotatedAt.Equal(now) {
		t.Fatalf("RotatedAt = %v, want %v", sess.RotatedAt, now)
	}
	if sess.PreviousTokenHash != "" {
		t.Fatalf("PreviousTokenHash = %q, want empty on a fresh session", sess.PreviousTokenHash)
	}
	if sess.RenewalDue(now.Add(SessionRenewalInterval - time.Second)) {
		t.Fatal("fresh session must not be due for rotation within the interval")
	}
}
