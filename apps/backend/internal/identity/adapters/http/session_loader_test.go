package http

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
)

// TestToPlatformSessionRoundTripsFields verifies the adapter mapping carries
// exactly the fields the middleware and its Touch seam read or write, so a
// maintained httpsupport.Session survives the to/from domain round-trip.
// UserID is deliberately outside the platform view (ADR 0034).
func TestToPlatformSessionRoundTripsFields(t *testing.T) {
	t.Parallel()

	src := domain.Session{
		ID:         uuid.Must(uuid.NewV7()),
		TokenHash:  "hash-abc",
		ExpiresAt:  time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		CreatedAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		LastUsedAt: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		RotatedAt:  time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC),
		LastIP:     "203.0.113.5",
		City:       "Тестоград",
	}

	t.Run("to platform carries every mapped field", func(t *testing.T) {
		t.Parallel()
		got := toPlatformSession(src)
		want := httpsupport.Session{
			ID: src.ID, TokenHash: src.TokenHash, ExpiresAt: src.ExpiresAt,
			CreatedAt: src.CreatedAt, LastUsedAt: src.LastUsedAt, RotatedAt: src.RotatedAt,
			LastIP: src.LastIP, City: src.City,
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("toPlatformSession = %+v, want %+v", got, want)
		}
	})

	t.Run("from platform carries every mapped field and drops UserID", func(t *testing.T) {
		t.Parallel()
		back := fromPlatformSession(toPlatformSession(src))
		// UserID is outside the platform view: the round trip zeroes it.
		want := src
		want.UserID = uuid.UUID{}
		if !reflect.DeepEqual(back, want) {
			t.Fatalf("fromPlatformSession = %+v, want %+v", back, want)
		}
	})
}

// TestPlatformSessionIsExpiredMirrorsDomain guards the one behavior the
// middleware checks itself: expiry must read identically on both sides of the
// seam (ADR 0034).
func TestPlatformSessionIsExpiredMirrorsDomain(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)

	domainSess := domain.Session{ExpiresAt: now.Add(-time.Second)}
	platformSess := toPlatformSession(domainSess)

	if domainSess.IsExpired(now) != platformSess.IsExpired(now) {
		t.Fatal("IsExpired disagrees between domain and platform views")
	}
}
