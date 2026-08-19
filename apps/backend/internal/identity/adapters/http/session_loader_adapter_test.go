package http

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// fakeSessionRepoForLoader is a minimal SessionRepository for sessionLoader
// tests. It only needs GetByTokenHash (for Load) and Update (for Update).
type fakeSessionRepoForLoader struct {
	getByTokenHash func(tokenHash string, now time.Time) (domain.Session, domain.User, error)
	updateFn       func(session domain.Session) error
}

func (r *fakeSessionRepoForLoader) Create(context.Context, domain.Session) error { return nil }
func (r *fakeSessionRepoForLoader) GetByTokenHash(_ context.Context, hash string, now time.Time) (domain.Session, domain.User, error) {
	if r.getByTokenHash != nil {
		return r.getByTokenHash(hash, now)
	}
	return domain.Session{}, domain.User{}, identityapp.ErrNotFound
}

func (r *fakeSessionRepoForLoader) Update(_ context.Context, session domain.Session) error {
	if r.updateFn != nil {
		return r.updateFn(session)
	}
	return nil
}
func (r *fakeSessionRepoForLoader) DeleteByTokenHash(context.Context, string) error { return nil }
func (r *fakeSessionRepoForLoader) DeleteByUserID(context.Context, uuid.UUID) error { return nil }
func (r *fakeSessionRepoForLoader) DeleteByUserIDExcept(context.Context, uuid.UUID, string) error {
	return nil
}

func (r *fakeSessionRepoForLoader) WithTx(_ transaction.Tx) (identityapp.SessionRepository, error) {
	return r, nil
}

// Build a real sessionService through the exported application constructors so
// we can pass it to NewSessionLoader without implementing the full SessionService
// interface (which has an unexported *txStores parameter on Issue).
func newSessionSvcForLoader(t *testing.T, sessions *fakeSessionRepoForLoader) identityapp.SessionService {
	t.Helper()
	// NewTxStoreFactory returns the unexported txStoreFactory; the concrete
	// sessionService returned by NewSessionService satisfies the exported
	// SessionService interface, so we hold it via := and return it as the
	// interface type.
	factory := identityapp.NewTxStoreFactory(
		nil, // Users — not needed for Load/Update.
		nil, // Codes.
		nil, // Attempts.
		sessions,
		nil, // Audit.
		nil, // UoW — not needed for Load/Update (they don't use runInTx).
	)
	return identityapp.NewSessionService(factory, identityapp.SessionServiceConfig{
		Hasher: passThroughHasher{},
	})
}

// passThroughHasher is a TokenHasher that returns the plaintext unchanged so
// the fake repo can match on the raw token value without a real hash.
type passThroughHasher struct{}

func (passThroughHasher) HashToken(plaintext string) string {
	return plaintext
}

func TestNewSessionLoader_NilServiceReturnsNil(t *testing.T) {
	t.Parallel()
	if got := NewSessionLoader(nil); got != nil {
		t.Fatalf("NewSessionLoader(nil) = %v, want nil", got)
	}
}

func TestSessionLoader_Load_SuccessMapsSessionAndUser(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	userID := uuid.Must(uuid.NewV7())
	wantSession := domain.Session{
		TokenHash:  testRawToken, // Pass-through hasher: TokenHash equals the raw token.
		ExpiresAt:  now.Add(time.Hour),
		CreatedAt:  now,
		LastUsedAt: now,
	}
	repo := &fakeSessionRepoForLoader{
		getByTokenHash: func(hash string, _ time.Time) (domain.Session, domain.User, error) {
			if hash != testRawToken {
				t.Errorf("repo received hash = %q, want %s", hash, testRawToken)
			}
			return wantSession, domain.User{ID: userID, Role: domain.RoleOwner}, nil
		},
	}
	loader := NewSessionLoader(newSessionSvcForLoader(t, repo))

	gotSession, gotUserID, gotRole, err := loader.Load(t.Context(), testRawToken, now)
	if err != nil {
		t.Fatalf("Load error = %v", err)
	}
	if gotSession.TokenHash != testRawToken {
		t.Fatalf("TokenHash = %q, want %s", gotSession.TokenHash, testRawToken)
	}
	if !gotSession.ExpiresAt.Equal(wantSession.ExpiresAt) {
		t.Fatalf("ExpiresAt = %v, want %v", gotSession.ExpiresAt, wantSession.ExpiresAt)
	}
	if gotUserID != userID {
		t.Fatalf("userID = %s, want %s", gotUserID, userID)
	}
	if gotRole != domain.RoleOwner {
		t.Fatalf("role = %s, want owner", gotRole)
	}
}

func TestSessionLoader_Load_NotFoundMapsToSessionNotFound(t *testing.T) {
	t.Parallel()
	repo := &fakeSessionRepoForLoader{} // Defaults to ErrNotFound.
	loader := NewSessionLoader(newSessionSvcForLoader(t, repo))

	session, userID, role, err := loader.Load(t.Context(), "dead-token", time.Now())
	if !httpsupport.IsSessionNotFound(err) {
		t.Fatalf("Load error = %v, want SessionNotFound sentinel", err)
	}
	if session.TokenHash != "" || userID != uuid.Nil || role != "" {
		t.Fatalf("Load returned non-zero results on error: session=%v userID=%s role=%q", session, userID, role)
	}
}

func TestSessionLoader_Load_OtherErrorPropagated(t *testing.T) {
	t.Parallel()
	dbErr := errors.New("db down")
	repo := &fakeSessionRepoForLoader{
		getByTokenHash: func(string, time.Time) (domain.Session, domain.User, error) {
			return domain.Session{}, domain.User{}, dbErr
		},
	}
	loader := NewSessionLoader(newSessionSvcForLoader(t, repo))

	session, userID, role, err := loader.Load(t.Context(), "token", time.Now())
	if !errors.Is(err, dbErr) {
		t.Fatalf("Load error = %v, want wrap of dbErr", err)
	}
	if httpsupport.IsSessionNotFound(err) {
		t.Fatal("non-ErrNotFound error was mapped to SessionNotFound")
	}
	if session.TokenHash != "" || userID != uuid.Nil || role != "" {
		t.Fatalf("Load returned non-zero results on error: session=%v userID=%s role=%q", session, userID, role)
	}
}

func TestSessionLoader_Update_Delegates(t *testing.T) {
	t.Parallel()
	var gotSession domain.Session
	repo := &fakeSessionRepoForLoader{
		updateFn: func(session domain.Session) error {
			gotSession = session
			return nil
		},
	}
	loader := NewSessionLoader(newSessionSvcForLoader(t, repo))

	platformSess := httpsupport.Session{
		TokenHash:  "hash-update",
		ExpiresAt:  time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC),
		LastUsedAt: time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC),
	}
	if err := loader.Update(t.Context(), platformSess); err != nil {
		t.Fatalf("Update error = %v", err)
	}
	if gotSession.TokenHash != "hash-update" {
		t.Fatalf("update received TokenHash = %q, want hash-update", gotSession.TokenHash)
	}
}
