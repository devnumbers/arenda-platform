//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
)

// seedUser creates and persists one owner so child rows satisfy foreign keys.
func seedUser(t *testing.T, ctx context.Context, poolRepo *UserRepository, emailRaw string) domain.User {
	t.Helper()
	email, err := domain.NewEmail(emailRaw)
	if err != nil {
		t.Fatalf("new email: %v", err)
	}
	user, err := domain.NewOwner(email)
	if err != nil {
		t.Fatalf("new owner: %v", err)
	}
	now := time.Now().UTC()
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
	t.Parallel()
	pool := testdb.Setup(t)
	ctx := context.Background()
	now := time.Now().UTC()

	user := seedUser(t, ctx, NewUserRepository(pool), "cleanup-counts@example.com")

	sessions := NewSessionRepository(pool)
	codes := NewLoginCodeRepository(pool)
	attempts := NewAttemptRepository(pool)

	seedCleanupSessions(t, ctx, pool, user.ID, now)
	seedCleanupLoginCodes(t, ctx, pool, codes, user.ID, now)
	seedCleanupAttemptWindows(t, ctx, attempts, now)

	assertSessionDeleteCounts(t, ctx, sessions, now)
	assertCodeDeleteCount(t, ctx, codes, now)
	assertAttemptDeleteCount(t, ctx, attempts, now)
}

// seedCleanupSessions inserts two expired sessions and one live session.
// Seeded with direct SQL because the repository's Create pins created_at to
// now(), and the schema check chk_sessions_expires_after_created forbids a
// session created already expired — only time passing makes a session expire.
func seedCleanupSessions(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, now time.Time) {
	t.Helper()
	insertSession := func(expiresAt time.Time) {
		t.Helper()
		id, err := uuid.NewV7()
		if err != nil {
			t.Fatalf("new session id: %v", err)
		}
		createdAt := now.Add(-35 * 24 * time.Hour)
		_, err = pool.Exec(ctx,
			`INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at, last_used_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			id, userID, "count-test-"+uuid.Must(uuid.NewV7()).String(), expiresAt, createdAt, now)
		if err != nil {
			t.Fatalf("insert session (expires %v): %v", expiresAt, err)
		}
	}
	insertSession(now.Add(-2 * time.Hour))
	insertSession(now.Add(-time.Hour))
	insertSession(now.Add(24 * time.Hour))
}

// seedCleanupLoginCodes stores one expired login code (address A) and one live
// code (address B). The expired code is seeded with direct SQL for the same
// reason as the sessions above: Save pins created_at to the DB now() while
// expiry comes from the domain object, and chk_login_codes_expires_after_created
// forbids a code created already expired — only time passing makes a code
// expire.
func seedCleanupLoginCodes(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	codes *LoginCodeRepository,
	userID uuid.UUID,
	now time.Time,
) {
	t.Helper()
	expiredEmail, err := domain.NewEmail("expired-code@example.com")
	if err != nil {
		t.Fatalf("new expired email: %v", err)
	}
	liveEmail, err := domain.NewEmail("live-code@example.com")
	if err != nil {
		t.Fatalf("new live email: %v", err)
	}
	expiredCodeID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new expired code id: %v", err)
	}
	_, err = pool.Exec(ctx,
		`INSERT INTO login_codes (id, user_id, email, code_hash, purpose, expires_at, used, created_at)
		 VALUES ($1, $2, $3, $4, 'login', $5, false, $6)`,
		expiredCodeID, userID, expiredEmail.String(), "expiredhash",
		now.Add(-2*time.Hour), now.Add(-35*24*time.Hour))
	if err != nil {
		t.Fatalf("insert expired code: %v", err)
	}
	if err := codes.Save(ctx, mustLoginCode(t, liveEmail, "livehash", &userID, now)); err != nil {
		t.Fatalf("save live code: %v", err)
	}
}

// seedCleanupAttemptWindows stores one stale window (last failure an hour ago)
// and one fresh window.
func seedCleanupAttemptWindows(
	t *testing.T,
	ctx context.Context,
	attempts *AttemptRepository,
	now time.Time,
) {
	t.Helper()
	staleEmail, err := domain.NewEmail("stale-attempts@example.com")
	if err != nil {
		t.Fatalf("new stale email: %v", err)
	}
	freshEmail, err := domain.NewEmail("fresh-attempts@example.com")
	if err != nil {
		t.Fatalf("new fresh email: %v", err)
	}
	staleWindow := domain.AttemptWindow{Failures: 1, FirstFailureAt: now.Add(-2 * time.Hour), LastFailureAt: now.Add(-time.Hour)}
	freshWindow := domain.AttemptWindow{Failures: 1, FirstFailureAt: now.Add(-time.Minute), LastFailureAt: now}
	if err := attempts.Save(ctx, staleEmail, staleWindow, 0); err != nil {
		t.Fatalf("save stale attempt: %v", err)
	}
	if err := attempts.Save(ctx, freshEmail, freshWindow, 0); err != nil {
		t.Fatalf("save fresh attempt: %v", err)
	}
}

// assertSessionDeleteCounts checks that DeleteExpiredBefore removes exactly the
// two expired sessions and reports 0 on the completeness second pass.
func assertSessionDeleteCounts(t *testing.T, ctx context.Context, sessions *SessionRepository, now time.Time) {
	t.Helper()
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
}

// assertCodeDeleteCount checks that only the expired login code is removed.
func assertCodeDeleteCount(t *testing.T, ctx context.Context, codes *LoginCodeRepository, now time.Time) {
	t.Helper()
	deleted, err := codes.DeleteExpiredBefore(ctx, now)
	if err != nil {
		t.Fatalf("delete expired codes: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("codes deleted = %d, want 1", deleted)
	}
}

// assertAttemptDeleteCount checks that only the stale attempt window is
// removed, with cutoff now.
func assertAttemptDeleteCount(t *testing.T, ctx context.Context, attempts *AttemptRepository, now time.Time) {
	t.Helper()
	deleted, err := attempts.DeleteStaleBefore(ctx, now)
	if err != nil {
		t.Fatalf("delete stale attempts: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("attempts deleted = %d, want 1", deleted)
	}
}

func mustLoginCode(
	t *testing.T,
	email domain.Email,
	codeHash string,
	userID *uuid.UUID,
	createdAt time.Time,
) domain.LoginCode {
	t.Helper()
	code, err := domain.NewLoginCode(email, codeHash, domain.LoginCodePurposeLogin, userID, createdAt)
	if err != nil {
		t.Fatalf("new login code: %v", err)
	}
	return code
}
