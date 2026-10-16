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
// tests. It only needs GetByTokenHash (Load and the Touch reload) and the
// Touch/Rotate maintenance writes.
type fakeSessionRepoForLoader struct {
	getByTokenHash func(tokenHash string, now time.Time) (domain.Session, domain.User, error)
	touchFn        func(session domain.Session) error
	lastNewHash    string
}

func (r *fakeSessionRepoForLoader) Create(context.Context, domain.Session) error { return nil }
func (r *fakeSessionRepoForLoader) GetByTokenHash(_ context.Context, hash string, now time.Time) (domain.Session, domain.User, error) {
	if r.getByTokenHash != nil {
		return r.getByTokenHash(hash, now)
	}
	return domain.Session{}, domain.User{}, identityapp.ErrNotFound
}

func (r *fakeSessionRepoForLoader) Update(context.Context, domain.Session) error { return nil }

func (r *fakeSessionRepoForLoader) Touch(_ context.Context, session domain.Session) error {
	if r.touchFn != nil {
		return r.touchFn(session)
	}
	return nil
}
func (r *fakeSessionRepoForLoader) DeleteByTokenHash(context.Context, string) error { return nil }
func (r *fakeSessionRepoForLoader) DeleteByUserIDExcept(context.Context, uuid.UUID, string) (int64, error) {
	return 0, nil
}

func (r *fakeSessionRepoForLoader) Rotate(_ context.Context, _ domain.Session, newTokenHash string) (bool, error) {
	r.lastNewHash = newTokenHash
	return true, nil
}

func (r *fakeSessionRepoForLoader) ListByUserID(_ context.Context, _ uuid.UUID, _ time.Time) ([]domain.Session, error) {
	return nil, nil
}

func (r *fakeSessionRepoForLoader) GetByID(context.Context, uuid.UUID) (domain.Session, error) {
	return domain.Session{}, identityapp.ErrNotFound
}

func (r *fakeSessionRepoForLoader) DeleteByIDForUser(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (r *fakeSessionRepoForLoader) WithTx(_ transaction.Tx) (identityapp.SessionRepository, error) {
	return r, nil
}

// The rotation path runs inside the UoW, so the factory needs transactional
// bindings for every store. Only the session repo's binding is ever exercised;
// the rest are nil-interface embedments whose single required method is WithTx.
type (
	stubUsersRepo struct{ identityapp.UserRepository }
	stubCodesRepo struct {
		identityapp.LoginCodeRepository
	}
	stubAttemptsRepo struct{ identityapp.AttemptRepository }
	stubGrantsRepo   struct {
		identityapp.EmailChangeGrantRepository
	}
)

func (stubUsersRepo) WithTx(transaction.Tx) (identityapp.UserRepository, error) {
	return stubUsersRepo{}, nil
}

func (stubCodesRepo) WithTx(transaction.Tx) (identityapp.LoginCodeRepository, error) {
	return stubCodesRepo{}, nil
}

func (stubAttemptsRepo) WithTx(transaction.Tx) (identityapp.AttemptRepository, error) {
	return stubAttemptsRepo{}, nil
}

func (stubGrantsRepo) WithTx(transaction.Tx) (identityapp.EmailChangeGrantRepository, error) {
	return stubGrantsRepo{}, nil
}

type (
	stubUoW struct{}
	stubTx  struct{}
)

func (stubUoW) Do(_ context.Context, work func(transaction.Tx) error) error { return work(stubTx{}) }
func (stubTx) Commit(context.Context) error                                 { return nil }
func (stubTx) Rollback(context.Context) error                               { return nil }

// Build a real sessionService through the exported application constructors so
// we can pass it to NewSessionLoader without implementing the full SessionService
// interface (which has an unexported *txStores parameter on Issue).
func newSessionSvcForLoader(t *testing.T, sessions *fakeSessionRepoForLoader) identityapp.SessionService {
	t.Helper()
	factory := identityapp.NewTxStoreFactory(
		stubUsersRepo{},
		stubCodesRepo{},
		stubAttemptsRepo{},
		sessions,
		stubGrantsRepo{},
		nil, // Audit — Noop by default.
		nil, // Contact-change letters — Noop by default.
		stubUoW{},
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

func TestSessionLoader_Load_SuccessResolvesActor(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	userID := uuid.Must(uuid.NewV7())
	repo := &fakeSessionRepoForLoader{
		getByTokenHash: func(hash string, _ time.Time) (domain.Session, domain.User, error) {
			if hash != testRawToken {
				t.Errorf("repo received hash = %q, want %s", hash, testRawToken)
			}
			return domain.Session{TokenHash: testRawToken}, domain.User{ID: userID, Role: domain.RoleOwner}, nil
		},
	}
	loader := NewSessionLoader(newSessionSvcForLoader(t, repo))

	gotUserID, gotRole, err := loader.Load(t.Context(), testRawToken, now)
	if err != nil {
		t.Fatalf("Load error = %v", err)
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
	repo := &fakeSessionRepoForLoader{} // Defaults to ErrNotFound: unknown and expired tokens alike.
	loader := NewSessionLoader(newSessionSvcForLoader(t, repo))

	userID, role, err := loader.Load(t.Context(), "dead-token", time.Now())
	if !httpsupport.IsSessionNotFound(err) {
		t.Fatalf("Load error = %v, want SessionNotFound sentinel", err)
	}
	if userID != uuid.Nil || role != "" {
		t.Fatalf("Load returned non-zero results on error: userID=%s role=%q", userID, role)
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

	userID, role, err := loader.Load(t.Context(), "token", time.Now())
	if !errors.Is(err, dbErr) {
		t.Fatalf("Load error = %v, want wrap of dbErr", err)
	}
	if httpsupport.IsSessionNotFound(err) {
		t.Fatal("non-ErrNotFound error was mapped to SessionNotFound")
	}
	if userID != uuid.Nil || role != "" {
		t.Fatalf("Load returned non-zero results on error: userID=%s role=%q", userID, role)
	}
}

// liveLoaderSession builds the session the repo answers the Touch reload with:
// a live session keyed by testRawToken whose last write happened two minutes
// ago (the throttle gate is open) from a different IP than the presenting
// client.
func liveLoaderSession(now time.Time) domain.Session {
	return domain.Session{
		ID:         uuid.Must(uuid.NewV7()),
		TokenHash:  testRawToken,
		ExpiresAt:  now.Add(24 * time.Hour),
		CreatedAt:  now.Add(-time.Hour),
		LastUsedAt: now.Add(-2 * time.Minute),
		RotatedAt:  now.Add(-time.Hour),
		LastIP:     "203.0.113.9",
		City:       "Тестгород",
	}
}

func TestSessionLoader_Touch_SlidingMoveReissuesPresentedToken(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 13, 13, 0, 0, 0, time.UTC)
	var touched domain.Session
	repo := &fakeSessionRepoForLoader{
		getByTokenHash: func(string, time.Time) (domain.Session, domain.User, error) {
			return liveLoaderSession(now), domain.User{}, nil
		},
		touchFn: func(session domain.Session) error {
			touched = session
			return nil
		},
	}
	loader := NewSessionLoader(newSessionSvcForLoader(t, repo))

	cookieToken, cookieExpires, err := loader.Touch(t.Context(), testRawToken, "198.51.100.2", now)
	if err != nil {
		t.Fatalf("Touch error = %v", err)
	}
	if cookieToken != "" {
		t.Fatalf("cookieToken = %q, want empty (the presented token stays valid)", cookieToken)
	}
	if cookieExpires == nil {
		t.Fatal("cookieExpires = nil, want the moved sliding expiry")
	}
	if !cookieExpires.Equal(now.Add(domain.SessionBaseTTL)) {
		t.Fatalf("cookieExpires = %v, want %v", cookieExpires, now.Add(domain.SessionBaseTTL))
	}
	// The real service applied the Touch policy before persisting: the changed
	// client IP replaced the stored one and the unknown new city kept the
	// stored value.
	if touched.LastIP != "198.51.100.2" || touched.City != "Тестгород" || touched.ID == uuid.Nil {
		t.Fatalf("touch persisted session = %+v, want the applied maintenance", touched)
	}
}

func TestSessionLoader_Touch_RotationDeliversFreshToken(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 13, 13, 0, 0, 0, time.UTC)
	seed := liveLoaderSession(now)
	seed.RotatedAt = now.Add(-15 * 24 * time.Hour) // The 14-day renewal window has elapsed.
	repo := &fakeSessionRepoForLoader{
		getByTokenHash: func(string, time.Time) (domain.Session, domain.User, error) {
			return seed, domain.User{}, nil
		},
	}
	loader := NewSessionLoader(newSessionSvcForLoader(t, repo))

	cookieToken, cookieExpires, err := loader.Touch(t.Context(), testRawToken, seed.LastIP, now)
	if err != nil {
		t.Fatalf("Touch error = %v", err)
	}
	// Pass-through hasher: the raw rotated token equals the hash the repo
	// persisted, so equality here pins the full round-trip.
	if cookieToken == "" || cookieToken != repo.lastNewHash {
		t.Fatalf("cookieToken = %q, want the fresh rotated token (%q)", cookieToken, repo.lastNewHash)
	}
	if cookieExpires == nil {
		t.Fatal("cookieExpires = nil, want the rotated session expiry")
	}
	if !cookieExpires.Equal(now.Add(domain.SessionBaseTTL)) {
		t.Fatalf("cookieExpires = %v, want %v", cookieExpires, now.Add(domain.SessionBaseTTL))
	}
}

func TestSessionLoader_Touch_NoChangeDeliversNoCookie(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 13, 13, 0, 0, 0, time.UTC)
	seed := liveLoaderSession(now)
	seed.LastUsedAt = now // The throttle gate is closed.
	var touchWrites int
	repo := &fakeSessionRepoForLoader{
		getByTokenHash: func(string, time.Time) (domain.Session, domain.User, error) {
			return seed, domain.User{}, nil
		},
		touchFn: func(domain.Session) error {
			touchWrites++
			return nil
		},
	}
	loader := NewSessionLoader(newSessionSvcForLoader(t, repo))

	cookieToken, cookieExpires, err := loader.Touch(t.Context(), testRawToken, seed.LastIP, now)
	if err != nil {
		t.Fatalf("Touch error = %v", err)
	}
	if cookieToken != "" || cookieExpires != nil {
		t.Fatalf("cookie decision = (%q, %v), want no cookie change", cookieToken, cookieExpires)
	}
	if touchWrites != 0 {
		t.Fatalf("touch writes = %d, want 0", touchWrites)
	}
}

func TestSessionLoader_Touch_LoaderMissMapsToSessionNotFound(t *testing.T) {
	t.Parallel()
	// No getByTokenHash stub: the token died between the middleware Load and
	// the Touch reload.
	repo := &fakeSessionRepoForLoader{}
	loader := NewSessionLoader(newSessionSvcForLoader(t, repo))

	cookieToken, cookieExpires, err := loader.Touch(t.Context(), "dead-token", "", time.Now())
	if !httpsupport.IsSessionNotFound(err) {
		t.Fatalf("Touch error = %v, want SessionNotFound sentinel", err)
	}
	if cookieToken != "" || cookieExpires != nil {
		t.Fatalf("cookie decision = (%q, %v), want no cookie change on a miss", cookieToken, cookieExpires)
	}
}

func TestSessionLoader_Touch_ServiceErrorPropagates(t *testing.T) {
	t.Parallel()
	dbErr := errors.New("touch write failed")
	now := time.Date(2026, 8, 13, 13, 0, 0, 0, time.UTC)
	repo := &fakeSessionRepoForLoader{
		getByTokenHash: func(string, time.Time) (domain.Session, domain.User, error) {
			return liveLoaderSession(now), domain.User{}, nil
		},
		touchFn: func(domain.Session) error { return dbErr },
	}
	loader := NewSessionLoader(newSessionSvcForLoader(t, repo))

	cookieToken, cookieExpires, err := loader.Touch(t.Context(), testRawToken, "198.51.100.2", now)
	if !errors.Is(err, dbErr) {
		t.Fatalf("Touch error = %v, want wrap of dbErr", err)
	}
	if cookieToken != "" || cookieExpires != nil {
		t.Fatalf("cookie decision = (%q, %v), want no cookie change on an error", cookieToken, cookieExpires)
	}
}
