package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSession_IsExpired(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := Session{
		ID:         uuid.New(),
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
			if got := s.IsExpired(tc.now); got != tc.want {
				t.Fatalf("IsExpired(%v) = %v, want %v", tc.now, got, tc.want)
			}
		})
	}
}

func TestSession_Refresh(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("extends expiration by SessionBaseTTL", func(t *testing.T) {
		s := Session{
			ID:         uuid.New(),
			ExpiresAt:  created.Add(SessionBaseTTL),
			CreatedAt:  created,
			LastUsedAt: created,
		}
		now := created.Add(1 * time.Hour)
		wantExpires := now.Add(SessionBaseTTL)

		if !s.Refresh(now) {
			t.Fatal("Refresh = false, want true")
		}
		if !s.ExpiresAt.Equal(wantExpires) {
			t.Fatalf("ExpiresAt = %v, want %v", s.ExpiresAt, wantExpires)
		}
		if !s.LastUsedAt.Equal(now) {
			t.Fatalf("LastUsedAt = %v, want %v", s.LastUsedAt, now)
		}
	})

	t.Run("caps expiration at CreatedAt + SessionMaxTTL", func(t *testing.T) {
		maxExpires := created.Add(SessionMaxTTL)
		// Session is still active but late in its lifetime: now is 6 days before
		// the hard cap (so now + SessionBaseTTL exceeds it), and ExpiresAt is 3
		// days before the cap (still active). Refresh must clamp to maxExpires.
		now := maxExpires.Add(-6 * 24 * time.Hour)
		s := Session{
			ID:         uuid.New(),
			ExpiresAt:  maxExpires.Add(-3 * 24 * time.Hour),
			CreatedAt:  created,
			LastUsedAt: created,
		}

		if !s.Refresh(now) {
			t.Fatal("Refresh = false, want true")
		}
		if !s.ExpiresAt.Equal(maxExpires) {
			t.Fatalf("ExpiresAt = %v, want cap %v", s.ExpiresAt, maxExpires)
		}
		if !s.LastUsedAt.Equal(now) {
			t.Fatalf("LastUsedAt = %v, want %v", s.LastUsedAt, now)
		}
	})

	t.Run("expired session is never refreshed", func(t *testing.T) {
		expires := created.Add(SessionBaseTTL)
		s := Session{
			ID:         uuid.New(),
			ExpiresAt:  expires,
			CreatedAt:  created,
			LastUsedAt: created,
		}
		now := expires.Add(time.Second)

		if s.Refresh(now) {
			t.Fatal("Refresh = true, want false for expired session")
		}
		if !s.ExpiresAt.Equal(expires) {
			t.Fatalf("ExpiresAt = %v, want unchanged %v", s.ExpiresAt, expires)
		}
		if !s.LastUsedAt.Equal(created) {
			t.Fatalf("LastUsedAt = %v, want unchanged %v", s.LastUsedAt, created)
		}
	})

	t.Run("returns false when expiration does not move forward", func(t *testing.T) {
		// Session already at the hard cap: candidate equals current ExpiresAt,
		// so candidate.After(ExpiresAt) is false.
		maxExpires := created.Add(SessionMaxTTL)
		s := Session{
			ID:         uuid.New(),
			ExpiresAt:  maxExpires,
			CreatedAt:  created,
			LastUsedAt: created,
		}
		now := maxExpires.Add(-SessionBaseTTL + time.Hour)

		if s.Refresh(now) {
			t.Fatal("Refresh = true, want false when expiration does not advance")
		}
		if !s.ExpiresAt.Equal(maxExpires) {
			t.Fatalf("ExpiresAt = %v, want unchanged cap %v", s.ExpiresAt, maxExpires)
		}
		if !s.LastUsedAt.Equal(created) {
			t.Fatalf("LastUsedAt = %v, want unchanged %v (Refresh must not touch it on no-op)", s.LastUsedAt, created)
		}
	})

	t.Run("updates LastUsedAt on every successful refresh", func(t *testing.T) {
		s := Session{
			ID:         uuid.New(),
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
