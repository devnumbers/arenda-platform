package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// newLogoutHarness wires a LogoutService to a fakeUoW + the shared identity
// fakes, returning every piece the tests need to assert behavior.
func newLogoutHarness() (*LogoutService, *fakeBeginner, *fakeSessionRepo) {
	users := newFakeUserRepo()
	codes := newFakeCodeRepo()
	attempts := newFakeAttemptRepo()
	sessions := newFakeSessionRepo()
	beginner := &fakeBeginner{}
	svc := NewLogoutService(
		users, codes, attempts, sessions,
		LogoutServiceConfig{
			Hasher: fakeHasher{},
			UoW:    &fakeUoW{beginner: beginner},
		},
	)
	return svc, beginner, sessions
}

// TestLogoutService_Logout_DeletesSessionByTokenHash proves Logout hashes the
// raw token, deletes the matching session through the transactional store, and
// the UoW commits.
func TestLogoutService_Logout_DeletesSessionByTokenHash(t *testing.T) {
	svc, beginner, sessions := newLogoutHarness()

	rawToken := "plain-test-value"
	tokenHash := fakeHasher{}.HashToken(rawToken)
	userID := uuid.New()
	sessions.sessions[tokenHash] = domain.Session{UserID: userID, TokenHash: tokenHash}

	if err := svc.Logout(context.Background(), rawToken); err != nil {
		t.Fatalf("Logout error = %v, want nil", err)
	}

	if _, ok := sessions.sessions[tokenHash]; ok {
		t.Fatal("session still present after Logout, want deleted")
	}
	if beginner.committed != 1 {
		t.Errorf("committed = %d, want 1", beginner.committed)
	}
	if beginner.rolledBack != 0 {
		t.Errorf("rolledBack = %d, want 0", beginner.rolledBack)
	}
}

// TestLogoutService_Logout_WrapsDeleteError proves a repository failure is
// returned and the transaction is rolled back.
func TestLogoutService_Logout_WrapsDeleteError(t *testing.T) {
	users := newFakeUserRepo()
	codes := newFakeCodeRepo()
	attempts := newFakeAttemptRepo()
	sessions := &errorSessionRepo{err: errors.New("db down")}
	beginner := &fakeBeginner{}
	svc := NewLogoutService(
		users, codes, attempts, sessions,
		LogoutServiceConfig{
			Hasher: fakeHasher{},
			UoW:    &fakeUoW{beginner: beginner},
		},
	)

	err := svc.Logout(context.Background(), "any-token")
	if err == nil {
		t.Fatal("Logout error = nil, want delete error")
	}
	if !errors.Is(err, sessions.err) {
		t.Fatalf("Logout error = %v, want wrap of %v", err, sessions.err)
	}
	if beginner.committed != 0 {
		t.Errorf("committed = %d, want 0 on error", beginner.committed)
	}
	if beginner.rolledBack != 1 {
		t.Errorf("rolledBack = %d, want 1 on error", beginner.rolledBack)
	}
}

// TestLogoutService_LogoutAll_DeletesSessionsByUserID proves LogoutAll removes
// every session belonging to the user and commits.
func TestLogoutService_LogoutAll_DeletesSessionsByUserID(t *testing.T) {
	svc, beginner, sessions := newLogoutHarness()

	userID := uuid.New()
	otherUserID := uuid.New()
	sessions.sessions["hash-a"] = domain.Session{UserID: userID, TokenHash: "hash-a"}
	sessions.sessions["hash-b"] = domain.Session{UserID: userID, TokenHash: "hash-b"}
	sessions.sessions["hash-c"] = domain.Session{UserID: otherUserID, TokenHash: "hash-c"}

	if err := svc.LogoutAll(context.Background(), userID); err != nil {
		t.Fatalf("LogoutAll error = %v, want nil", err)
	}

	if _, ok := sessions.sessions["hash-a"]; ok {
		t.Error("session hash-a still present, want deleted")
	}
	if _, ok := sessions.sessions["hash-b"]; ok {
		t.Error("session hash-b still present, want deleted")
	}
	if _, ok := sessions.sessions["hash-c"]; !ok {
		t.Error("session hash-c was deleted, want retained (different user)")
	}
	if beginner.committed != 1 {
		t.Errorf("committed = %d, want 1", beginner.committed)
	}
}

// TestLogoutService_UsesRunInTx proves Logout goes through the UoW seam: the
// session repository is bound to the transaction and the UoW commits. This is
// the core assertion of the ADR 0033 smoke test.
func TestLogoutService_UsesRunInTx(t *testing.T) {
	users := newFakeUserRepo()
	codes := newFakeCodeRepo()
	attempts := newFakeAttemptRepo()
	sessions := &countingSessionRepo{fakeSessionRepo: newFakeSessionRepo()}
	beginner := &fakeBeginner{}
	svc := NewLogoutService(
		users, codes, attempts, sessions,
		LogoutServiceConfig{
			Hasher: fakeHasher{},
			UoW:    &fakeUoW{beginner: beginner},
		},
	)

	if err := svc.Logout(context.Background(), "token"); err != nil {
		t.Fatalf("Logout error = %v, want nil", err)
	}

	if sessions.withTxCalls != 1 {
		t.Errorf("sessions.WithTx calls = %d, want 1 (must bind inside runInTx)", sessions.withTxCalls)
	}
	if beginner.committed != 1 {
		t.Errorf("committed = %d, want 1", beginner.committed)
	}
}

// errorSessionRepo is a SessionRepository whose DeleteByTokenHash always fails,
// so the error/rollback path can be exercised.
type errorSessionRepo struct{ err error }

func (r *errorSessionRepo) Create(context.Context, domain.Session) error { return nil }
func (r *errorSessionRepo) GetByTokenHash(context.Context, string, time.Time) (domain.Session, domain.User, error) {
	return domain.Session{}, domain.User{}, ErrNotFound
}
func (r *errorSessionRepo) Update(context.Context, domain.Session) error    { return nil }
func (r *errorSessionRepo) DeleteByTokenHash(context.Context, string) error { return r.err }
func (r *errorSessionRepo) DeleteByUserID(context.Context, uuid.UUID) error { return r.err }
func (r *errorSessionRepo) DeleteByUserIDExcept(context.Context, uuid.UUID, string) error {
	return r.err
}

func (r *errorSessionRepo) DeleteExpiredBeforeBatch(context.Context, time.Time, int32) (int64, error) {
	return 0, nil
}
func (r *errorSessionRepo) WithTx(transaction.Tx) (SessionRepository, error) { return r, nil }
