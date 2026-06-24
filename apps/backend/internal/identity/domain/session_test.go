package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewSession(t *testing.T) {
	now := time.Date(2026, 6, 24, 8, 0, 0, 0, time.UTC)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	raw, err := NewSession(userID, now)
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	if raw.Token == "" {
		t.Error("expected non-empty token")
	}
	if raw.Session.UserID != userID {
		t.Errorf("UserID = %v, want %v", raw.Session.UserID, userID)
	}
	if raw.Session.TokenHash == "" {
		t.Error("expected non-empty token hash")
	}
	if !raw.Session.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v", raw.Session.CreatedAt, now)
	}
	if !raw.Session.LastUsedAt.Equal(now) {
		t.Errorf("LastUsedAt = %v, want %v", raw.Session.LastUsedAt, now)
	}
	wantExpires := now.Add(SessionBaseTTL)
	if !raw.Session.ExpiresAt.Equal(wantExpires) {
		t.Errorf("ExpiresAt = %v, want %v", raw.Session.ExpiresAt, wantExpires)
	}
}

func TestSessionIsExpired(t *testing.T) {
	now := time.Date(2026, 6, 24, 8, 0, 0, 0, time.UTC)
	s := Session{ExpiresAt: now.Add(time.Hour)}

	if s.IsExpired(now) {
		t.Error("session should not be expired at now")
	}
	if !s.IsExpired(now.Add(2 * time.Hour)) {
		t.Error("session should be expired after ExpiresAt")
	}
}

func TestSessionRefreshExtendsExpiresAt(t *testing.T) {
	now := time.Date(2026, 6, 24, 8, 0, 0, 0, time.UTC)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	raw, _ := NewSession(userID, now)
	s := raw.Session

	later := now.Add(30 * time.Minute)
	refreshed := s.Refresh(later)
	if !refreshed {
		t.Fatal("expected Refresh to extend session")
	}

	wantExpires := later.Add(SessionBaseTTL)
	if !s.ExpiresAt.Equal(wantExpires) {
		t.Errorf("ExpiresAt = %v, want %v", s.ExpiresAt, wantExpires)
	}
	if !s.LastUsedAt.Equal(later) {
		t.Errorf("LastUsedAt = %v, want %v", s.LastUsedAt, later)
	}
}

func TestSessionRefreshRespectsMaxLifetime(t *testing.T) {
	now := time.Date(2026, 6, 24, 8, 0, 0, 0, time.UTC)

	// Simulate a session that has already been sliding for almost 30 days.
	s := Session{
		UserID:     uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		TokenHash:  "hash",
		CreatedAt:  now,
		LastUsedAt: now,
		ExpiresAt:  now.Add(SessionMaxTTL - 30*time.Minute),
	}

	// Refresh while the session is still valid but close to the absolute cap.
	later := now.Add(SessionMaxTTL - 45*time.Minute)
	refreshed := s.Refresh(later)
	if !refreshed {
		t.Fatal("expected Refresh to extend session")
	}

	wantExpires := now.Add(SessionMaxTTL)
	if !s.ExpiresAt.Equal(wantExpires) {
		t.Errorf("ExpiresAt = %v, want %v", s.ExpiresAt, wantExpires)
	}

	// A further refresh at the cap must not extend beyond the max lifetime.
	atCap := now.Add(SessionMaxTTL)
	refreshed = s.Refresh(atCap)
	if refreshed {
		t.Error("expected Refresh to be a no-op at the max lifetime cap")
	}
	if !s.ExpiresAt.Equal(wantExpires) {
		t.Errorf("ExpiresAt changed at cap: %v, want %v", s.ExpiresAt, wantExpires)
	}
}

func TestSessionRefreshDoesNothingWhenExpired(t *testing.T) {
	now := time.Date(2026, 6, 24, 8, 0, 0, 0, time.UTC)
	s := Session{ExpiresAt: now.Add(time.Hour)}

	if s.Refresh(now.Add(2 * time.Hour)) {
		t.Error("Refresh on expired session should return false")
	}
}
