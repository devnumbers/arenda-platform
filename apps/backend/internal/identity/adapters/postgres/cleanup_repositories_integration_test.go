//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
)

// seedUser creates and persists one owner so child rows satisfy foreign keys.
func seedUser(t *testing.T, ctx context.Context, poolRepo *UserRepository, phoneRaw, emailRaw string) domain.User {
	t.Helper()
	phone, err := domain.NewPhone(phoneRaw)
	if err != nil {
		t.Fatalf("new phone: %v", err)
	}
	email, err := domain.NewEmail(emailRaw)
	if err != nil {
		t.Fatalf("new email: %v", err)
	}
	user, err := domain.NewOwner(phone)
	if err != nil {
		t.Fatalf("new owner: %v", err)
	}
	now := time.Now().UTC()
	user.Email = &email
	user.EmailVerifiedAt = &now
	created, err := poolRepo.Create(ctx, user)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return created
}

// TestCleanupRepositories_DeleteCounts verifies that the expired-data delete
// methods report the number of rows removed, remove only expired rows, and
// report 0 once nothing matches (loop completeness).
func TestCleanupRepositories_DeleteCounts(t *testing.T) {
	pool := testdb.Setup(t)
	ctx := context.Background()
	enc := noopEncryptor(t)
	now := time.Now().UTC()

	user := seedUser(t, ctx, NewUserRepository(pool, enc), "+79990001142", "cleanup-counts@example.com")

	sessions := NewSessionRepository(pool, enc)
	codes := NewLoginCodeRepository(pool, enc)
	attempts := NewAttemptRepository(pool, enc)

	// Sessions: two expired, one live. Seeded with direct SQL because the
	// repository's Create pins created_at to now(), and the schema check
	// chk_sessions_expires_after_created forbids a session created already
	// expired — only time passing makes a session expire.
	insertSession := func(expiresAt time.Time) {
		id, err := uuid.NewV7()
		if err != nil {
			t.Fatalf("new session id: %v", err)
		}
		createdAt := now.Add(-35 * 24 * time.Hour)
		_, err = pool.Exec(ctx,
			`INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at, last_used_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			id, user.ID, "count-test-"+uuid.Must(uuid.NewV7()).String(), expiresAt, createdAt, now)
		if err != nil {
			t.Fatalf("insert session (expires %v): %v", expiresAt, err)
		}
	}
	insertSession(now.Add(-2 * time.Hour))
	insertSession(now.Add(-time.Hour))
	insertSession(now.Add(24 * time.Hour))

	// Login codes: one expired (phone A), one live (phone B).
	expiredPhone, err := domain.NewPhone("+79990001143")
	if err != nil {
		t.Fatalf("new expired phone: %v", err)
	}
	livePhone, err := domain.NewPhone("+79990001144")
	if err != nil {
		t.Fatalf("new live phone: %v", err)
	}
	expiredEmail, err := domain.NewEmail("expired-code@example.com")
	if err != nil {
		t.Fatalf("new expired email: %v", err)
	}
	liveEmail, err := domain.NewEmail("live-code@example.com")
	if err != nil {
		t.Fatalf("new live email: %v", err)
	}
	userID := user.ID
	// The expired code is seeded with direct SQL for the same reason as the
	// sessions above: Save pins created_at to the DB now() while expiry comes
	// from the domain object, and chk_login_codes_expires_after_created
	// forbids a code created already expired — only time passing makes a
	// code expire.
	expiredCodeID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new expired code id: %v", err)
	}
	_, err = pool.Exec(ctx,
		`INSERT INTO login_codes (id, user_id, phone, email, code_hash, purpose, expires_at, used, created_at, phone_encrypted)
		 VALUES ($1, $2, $3, $4, $5, 'login', $6, false, $7, false)`,
		expiredCodeID, userID, expiredPhone.String(), expiredEmail.String(), "expiredhash",
		now.Add(-2*time.Hour), now.Add(-35*24*time.Hour))
	if err != nil {
		t.Fatalf("insert expired code: %v", err)
	}
	if err := codes.Save(ctx, mustLoginCode(t, livePhone, liveEmail, "livehash", &userID, now)); err != nil {
		t.Fatalf("save live code: %v", err)
	}

	// Attempt windows: one stale (last failure an hour ago), one fresh.
	stalePhone, err := domain.NewPhone("+79990001145")
	if err != nil {
		t.Fatalf("new stale phone: %v", err)
	}
	freshPhone, err := domain.NewPhone("+79990001146")
	if err != nil {
		t.Fatalf("new fresh phone: %v", err)
	}
	staleWindow := domain.AttemptWindow{Failures: 1, FirstFailureAt: now.Add(-2 * time.Hour), LastFailureAt: now.Add(-time.Hour)}
	freshWindow := domain.AttemptWindow{Failures: 1, FirstFailureAt: now.Add(-time.Minute), LastFailureAt: now}
	if err := attempts.Save(ctx, stalePhone, user.ID, staleWindow, 0); err != nil {
		t.Fatalf("save stale attempt: %v", err)
	}
	if err := attempts.Save(ctx, freshPhone, user.ID, freshWindow, 0); err != nil {
		t.Fatalf("save fresh attempt: %v", err)
	}

	// Expired sessions: exactly the two expired rows, then completeness.
	deleted, err := sessions.DeleteExpiredBefore(ctx, now)
	if err != nil {
		t.Fatalf("delete expired sessions: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("sessions deleted = %d, want 2", deleted)
	}
	if deleted, err = sessions.DeleteExpiredBefore(ctx, now); err != nil || deleted != 0 {
		t.Fatalf("second pass: deleted = %d, err = %v; want 0, nil", deleted, err)
	}

	// Expired login codes: only the expired one goes.
	deleted, err = codes.DeleteExpiredBefore(ctx, now)
	if err != nil {
		t.Fatalf("delete expired codes: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("codes deleted = %d, want 1", deleted)
	}

	// Stale attempt windows: only the stale one, cutoff now.
	deleted, err = attempts.DeleteStaleBefore(ctx, now)
	if err != nil {
		t.Fatalf("delete stale attempts: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("attempts deleted = %d, want 1", deleted)
	}
}

func mustLoginCode(t *testing.T, phone domain.Phone, email domain.Email, codeHash string, userID *uuid.UUID, createdAt time.Time) domain.LoginCode {
	t.Helper()
	code, err := domain.NewLoginCode(phone, email, codeHash, domain.LoginCodePurposeLogin, userID, createdAt)
	if err != nil {
		t.Fatalf("new login code: %v", err)
	}
	return code
}
