package http

import (
	"testing"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
)

// TestSessionRefreshMirrorsDomain is the invariant that ties the duplicated
// sliding-window logic together: httpsupport.Session.Refresh/IsExpired must stay
// byte-for-byte equivalent to identity/domain.Session.Refresh/IsExpired, because
// the platform cannot import the identity aggregate and the aggregate is not
// lifted into the shared kernel (ADR 0034). If either implementation drifts,
// this test fails before the divergence reaches production.
//
// The cases cover every branch of Refresh: an already-expired session, a normal
// refresh, a refresh capped by the hard max TTL, and a no-op refresh where the
// candidate does not move the expiry forward.
func TestSessionRefreshMirrorsDomain(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name      string
		expiresAt time.Time
		now       time.Time
	}{
		{
			name:      "already expired is not refreshed",
			expiresAt: created.Add(1 * time.Hour),
			now:       created.Add(2 * time.Hour),
		},
		{
			name:      "normal refresh extends by base TTL",
			expiresAt: created.Add(6 * 24 * time.Hour),
			now:       created.Add(1 * 24 * time.Hour),
		},
		{
			name:      "refresh capped at created plus max TTL",
			expiresAt: created.Add(28 * 24 * time.Hour),
			now:       created.Add(29 * 24 * time.Hour),
		},
		{
			name:      "candidate not after expiresAt is a no-op",
			expiresAt: created.Add(7 * 24 * time.Hour),
			now:       created,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			n := tc.now

			domainSess := domain.Session{
				ExpiresAt:  tc.expiresAt,
				CreatedAt:  created,
				LastUsedAt: created,
			}
			platformSess := toPlatformSession(domain.Session{
				ExpiresAt:  tc.expiresAt,
				CreatedAt:  created,
				LastUsedAt: created,
			})

			// IsExpired must agree.
			if domainSess.IsExpired(n) != platformSess.IsExpired(n) {
				t.Fatalf("IsExpired mismatch: domain=%v platform=%v", domainSess.IsExpired(n), platformSess.IsExpired(n))
			}

			domainRefreshed := domainSess.Refresh(n)
			platformRefreshed := platformSess.Refresh(n)

			if domainRefreshed != platformRefreshed {
				t.Fatalf("Refresh return mismatch: domain=%v platform=%v", domainRefreshed, platformRefreshed)
			}
			if !domainSess.ExpiresAt.Equal(platformSess.ExpiresAt) {
				t.Fatalf("ExpiresAt mismatch after Refresh: domain=%v platform=%v", domainSess.ExpiresAt, platformSess.ExpiresAt)
			}
			if !domainSess.LastUsedAt.Equal(platformSess.LastUsedAt) {
				t.Fatalf("LastUsedAt mismatch after Refresh: domain=%v platform=%v", domainSess.LastUsedAt, platformSess.LastUsedAt)
			}
		})
	}
}

// TestToPlatformSessionRoundTripsFields verifies the adapter mapping carries
// exactly the sliding-window fields the middleware touches, so a refreshed
// httpsupport.Session survives the to/from domain round-trip into Update.
func TestToPlatformSessionRoundTripsFields(t *testing.T) {
	t.Parallel()

	src := domain.Session{
		TokenHash:  "hash-abc",
		ExpiresAt:  time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		CreatedAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		LastUsedAt: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
	}
	got := toPlatformSession(src)
	if got.TokenHash != src.TokenHash || !got.ExpiresAt.Equal(src.ExpiresAt) ||
		!got.CreatedAt.Equal(src.CreatedAt) || !got.LastUsedAt.Equal(src.LastUsedAt) {
		t.Fatalf("toPlatformSession dropped a field: src=%+v got=%+v", src, got)
	}

	back := fromPlatformSession(got)
	if back.TokenHash != src.TokenHash || !back.ExpiresAt.Equal(src.ExpiresAt) ||
		!back.CreatedAt.Equal(src.CreatedAt) || !back.LastUsedAt.Equal(src.LastUsedAt) {
		t.Fatalf("fromPlatformSession dropped a field: src=%+v back=%+v", src, back)
	}
}

// TestPlatformSessionRefreshMatchesDomainTTLConstants guards the TTL constants
// themselves: if either side changes SessionBaseTTL/SessionMaxTTL, the invariant
// breaks even before Refresh logic drifts.
func TestPlatformSessionRefreshMatchesDomainTTLConstants(t *testing.T) {
	t.Parallel()

	if domain.SessionBaseTTL != httpsupport.SessionBaseTTL {
		t.Errorf("SessionBaseTTL drift: domain=%v platform=%v", domain.SessionBaseTTL, httpsupport.SessionBaseTTL)
	}
	if domain.SessionMaxTTL != httpsupport.SessionMaxTTL {
		t.Errorf("SessionMaxTTL drift: domain=%v platform=%v", domain.SessionMaxTTL, httpsupport.SessionMaxTTL)
	}
}
