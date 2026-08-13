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
// fakes, returning the service and fakeStores for assertions.
func newLogoutHarness() (*LogoutService, *fakeStores) {
	stores := newFakeStores()
	svc := NewLogoutService(
		stores.factory(nil),
		LogoutServiceConfig{
			Hasher: fakeHasher{},
		},
	)
	return svc, stores
}

// TestLogoutService_Logout_DeletesSessionByTokenHash proves Logout hashes the
// raw token, deletes the matching session through the transactional store, and
// the UoW commits.
func TestLogoutService_Logout_DeletesSessionByTokenHash(t *testing.T) {
	svc, stores := newLogoutHarness()

	rawToken := "plain-test-value"
	tokenHash := fakeHasher{}.HashToken(rawToken)
	userID := uuid.New()
	stores.sessions.sessions[tokenHash] = domain.Session{UserID: userID, TokenHash: tokenHash}

	if err := svc.Logout(context.Background(), rawToken); err != nil {
		t.Fatalf("Logout error = %v, want nil", err)
	}

	if _, ok := stores.sessions.sessions[tokenHash]; ok {
		t.Fatal("session still present after Logout, want deleted")
	}
	if stores.beginner.committed != 1 {
		t.Errorf("committed = %d, want 1", stores.beginner.committed)
	}
	if stores.beginner.rolledBack != 0 {
		t.Errorf("rolledBack = %d, want 0", stores.beginner.rolledBack)
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
	factory := NewTxStoreFactory(users, codes, attempts, sessions, nil, &fakeUoW{beginner: beginner})
	svc := NewLogoutService(
		factory,
		LogoutServiceConfig{
			Hasher: fakeHasher{},
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
	svc, stores := newLogoutHarness()

	userID := uuid.New()
	otherUserID := uuid.New()
	stores.sessions.sessions["hash-a"] = domain.Session{UserID: userID, TokenHash: "hash-a"}
	stores.sessions.sessions["hash-b"] = domain.Session{UserID: userID, TokenHash: "hash-b"}
	stores.sessions.sessions["hash-c"] = domain.Session{UserID: otherUserID, TokenHash: "hash-c"}

	if err := svc.LogoutAll(context.Background(), userID); err != nil {
		t.Fatalf("LogoutAll error = %v, want nil", err)
	}

	if _, ok := stores.sessions.sessions["hash-a"]; ok {
		t.Error("session hash-a still present, want deleted")
	}
	if _, ok := stores.sessions.sessions["hash-b"]; ok {
		t.Error("session hash-b still present, want deleted")
	}
	if _, ok := stores.sessions.sessions["hash-c"]; !ok {
		t.Error("session hash-c was deleted, want retained (different user)")
	}
	if stores.beginner.committed != 1 {
		t.Errorf("committed = %d, want 1", stores.beginner.committed)
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
	factory := NewTxStoreFactory(users, codes, attempts, sessions, nil, &fakeUoW{beginner: beginner})
	svc := NewLogoutService(
		factory,
		LogoutServiceConfig{
			Hasher: fakeHasher{},
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

func (r *errorSessionRepo) DeleteExpiredBefore(context.Context, time.Time) error {
	return nil
}
func (r *errorSessionRepo) WithTx(transaction.Tx) (SessionRepository, error) { return r, nil }
