//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
)

const (
	fixtureCity          = "Тестоград"
	fixtureOldHash       = "hash-old"
	fixtureNewHash       = "hash-new"
	fixtureIP            = "203.0.113.77"
	fixtureListIP        = "203.0.113.10"
	fixtureRoundTripHash = "hash-device-roundtrip"
	fixtureListKeptHash  = "hash-list-old"
	fixtureExtraHash     = "hash-list-extra"
)

// TestSessionRepository_DeviceFieldsRoundTrip proves the device description
// and the IP/city captured at creation survive the persistence round trip.
func TestSessionRepository_DeviceFieldsRoundTrip(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	ctx := context.Background()
	enc := noopEncryptor(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	user := seedUser(t, ctx, NewUserRepository(pool, enc), "+79990001151", "device-roundtrip@example.com")
	repo := NewSessionRepository(pool, enc)

	raw, err := domain.NewSession(user.ID, now)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	raw.Session.TokenHash = fixtureRoundTripHash
	raw.Session.UserAgent = "Mozilla/5.0 TestUA"
	raw.Session.DeviceType = domain.DevicePhone
	raw.Session.Browser = "Chrome"
	raw.Session.BrowserMajor = 121
	raw.Session.OS = "Android"
	raw.Session.LastIP = fixtureIP
	raw.Session.City = fixtureCity

	if err := repo.Create(ctx, raw.Session); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, _, err := repo.GetByTokenHash(ctx, raw.Session.TokenHash, now)
	if err != nil {
		t.Fatalf("get by token hash: %v", err)
	}
	if got.UserAgent != raw.Session.UserAgent || got.DeviceType != domain.DevicePhone ||
		got.Browser != "Chrome" || got.BrowserMajor != 121 || got.OS != "Android" ||
		got.LastIP != fixtureIP || got.City != fixtureCity {
		t.Fatalf("device fields lost in the round trip: %+v", got)
	}
}

// TestSessionRepository_RotationGrace proves the rotation grace window: the
// previous token hash keeps resolving a rotated session for a short moment,
// the canonical hash always resolves to the same row, and after the grace the
// previous hash stops resolving.
func TestSessionRepository_RotationGrace(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	ctx := context.Background()
	enc := noopEncryptor(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	user := seedUser(t, ctx, NewUserRepository(pool, enc), "+79990001152", "rotation-grace@example.com")
	repo := NewSessionRepository(pool, enc)

	raw, err := domain.NewSession(user.ID, now)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	raw.Session.TokenHash = fixtureOldHash
	if err := repo.Create(ctx, raw.Session); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Rotate in place: the fresh hash becomes canonical, the old one moves to
	// the grace slot.
	rotated := raw.Session
	rotated.RotatedAt = now
	rotated.PreviousTokenHash = fixtureOldHash
	won, err := repo.Rotate(ctx, rotated, fixtureNewHash)
	if err != nil || !won {
		t.Fatalf("rotate: won=%v err=%v", won, err)
	}

	// Within the grace: the old hash resolves, carrying the canonical hash.
	graceSess, _, err := repo.GetByTokenHash(ctx, fixtureOldHash, now.Add(time.Second))
	if err != nil {
		t.Fatalf("old hash within grace: %v", err)
	}
	if graceSess.TokenHash != fixtureNewHash {
		t.Fatalf("canonical TokenHash = %q, want hash-new", graceSess.TokenHash)
	}
	if graceSess.PreviousTokenHash != fixtureOldHash {
		t.Fatalf("PreviousTokenHash = %q, want hash-old", graceSess.PreviousTokenHash)
	}

	// After the grace (rotated_at behind now - SessionRotationGrace): gone.
	if _, _, err := repo.GetByTokenHash(ctx, fixtureOldHash, now.Add(domain.SessionRotationGrace+time.Minute)); err == nil {
		t.Fatal("old hash resolved past the grace window")
	}

	// The canonical hash resolves before and after the grace.
	sess, _, err := repo.GetByTokenHash(ctx, fixtureNewHash, now.Add(domain.SessionRotationGrace+time.Minute))
	if err != nil || sess.TokenHash != fixtureNewHash {
		t.Fatalf("new hash after grace: sess=%+v err=%v", sess, err)
	}
}

// TestSessionRepository_ListByUserID proves the devices-list read: only the
// owner's rows, most recent activity first, device fields intact.
func TestSessionRepository_ListByUserID(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	ctx := context.Background()
	enc := noopEncryptor(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	user := seedUser(t, ctx, NewUserRepository(pool, enc), "+79990001153", "list-revoke@example.com")
	foreign := seedUser(t, ctx, NewUserRepository(pool, enc), "+79990001154", "list-revoke-foreign@example.com")
	repo := NewSessionRepository(pool, enc)

	seed := func(userID uuid.UUID, hash string, lastUsed time.Time) domain.Session {
		raw, err := domain.NewSession(userID, lastUsed)
		if err != nil {
			t.Fatalf("new session: %v", err)
		}
		raw.Session.TokenHash = hash
		raw.Session.LastIP = "203.0.113.10"
		raw.Session.City = fixtureCity
		if err := repo.Create(ctx, raw.Session); err != nil {
			t.Fatalf("create %s: %v", hash, err)
		}
		return raw.Session
	}

	old := seed(user.ID, "hash-list-old", now.Add(-2*time.Hour))
	recent := seed(user.ID, "hash-list-recent", now.Add(-time.Minute))
	seed(foreign.ID, "hash-list-foreign", now)

	sessions, err := repo.ListByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("sessions = %d, want 2 (only the owner's)", len(sessions))
	}
	if sessions[0].ID != recent.ID || sessions[1].ID != old.ID {
		t.Fatalf("order = [%s %s], want most recent activity first", sessions[0].ID, sessions[1].ID)
	}
	for _, s := range sessions {
		if s.LastIP != fixtureListIP || s.City != fixtureCity || s.DeviceType != domain.DeviceUnknown {
			t.Fatalf("list row lost device fields: %+v", s)
		}
	}
}

// TestSessionRepository_RevokePaths proves the revocation paths: a foreign
// session is invisible to the ownership-scoped delete, the owner's delete
// removes exactly the row, and the except-delete keeps the kept session.
func TestSessionRepository_RevokePaths(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	ctx := context.Background()
	enc := noopEncryptor(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	user := seedUser(t, ctx, NewUserRepository(pool, enc), "+79990001155", "revoke-paths@example.com")
	foreign := seedUser(t, ctx, NewUserRepository(pool, enc), "+79990001156", "revoke-paths-foreign@example.com")
	repo := NewSessionRepository(pool, enc)

	seed := func(userID uuid.UUID, hash string, lastUsed time.Time) domain.Session {
		raw, err := domain.NewSession(userID, lastUsed)
		if err != nil {
			t.Fatalf("new session: %v", err)
		}
		raw.Session.TokenHash = hash
		if err := repo.Create(ctx, raw.Session); err != nil {
			t.Fatalf("create %s: %v", hash, err)
		}
		return raw.Session
	}
	recent := seed(user.ID, "hash-revoke-recent", now.Add(-time.Minute))
	seed(foreign.ID, "hash-revoke-foreign", now)

	// A foreign session is invisible to the ownership-scoped delete.
	removed, err := repo.DeleteByIDForUser(ctx, recent.ID, foreign.ID)
	if err != nil || removed {
		t.Fatalf("foreign delete: removed=%v err=%v", removed, err)
	}

	// The owner's delete removes exactly the row.
	removed, err = repo.DeleteByIDForUser(ctx, recent.ID, user.ID)
	if err != nil || !removed {
		t.Fatalf("owner delete: removed=%v err=%v", removed, err)
	}
	if _, err := repo.GetByID(ctx, recent.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("get revoked: err=%v, want ErrNotFound", err)
	}

	// The except-delete keeps the kept session (by its current hash) and
	// removes the rest of the user's rows.
	seed(user.ID, fixtureListKeptHash, now.Add(-2*time.Hour))
	extra := seed(user.ID, fixtureExtraHash, now.Add(-time.Hour))
	n, err := repo.DeleteByUserIDExcept(ctx, user.ID, fixtureListKeptHash)
	if err != nil {
		t.Fatalf("except delete: %v", err)
	}
	if n != 1 {
		t.Fatalf("except delete removed %d rows, want 1", n)
	}
	if _, _, err := repo.GetByTokenHash(ctx, fixtureListKeptHash, now.Add(-3*time.Hour)); err != nil {
		t.Fatalf("kept session lost by the except-delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, extra.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("extra session survived the except-delete: err=%v", err)
	}
}
