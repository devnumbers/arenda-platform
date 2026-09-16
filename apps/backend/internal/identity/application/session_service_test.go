package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type sessionHarness struct {
	*fakeStores
	svc *sessionService
}

func newSessionHarness() *sessionHarness {
	stores := newFakeStores()
	h := &sessionHarness{
		fakeStores: stores,
	}
	h.svc = NewSessionService(stores.factory(nil), SessionServiceConfig{Hasher: fakeHasher{}})
	return h
}

func (h *sessionHarness) stores(ctx context.Context) (*txStores, error) {
	tx, err := h.beginner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	_ = tx
	return &txStores{
		users:    h.users,
		codes:    h.codes,
		attempts: h.attempts,
		sessions: h.sessions,
	}, nil
}

func TestSessionService_Issue_CreatesNewUserAndSession(t *testing.T) {
	t.Parallel()
	h := newSessionHarness()
	phone := mustPhone(t, "+79150000001")
	email := mustEmail(t, "owner@example.com")
	ctx := t.Context()

	stores, err := h.stores(ctx)
	if err != nil {
		t.Fatalf("stores: %v", err)
	}

	raw, user, isNew, err := h.svc.Issue(ctx, stores, phone, email, testNow)
	if err != nil {
		t.Fatalf("Issue error = %v", err)
	}
	if !isNew {
		t.Fatal("isNew = false, want true for a brand-new user")
	}
	if raw.Token == "" {
		t.Fatal("raw token is empty")
	}
	if raw.Session.TokenHash == "" {
		t.Fatal("session token hash is empty")
	}
	if user.Email == nil || *user.Email != email {
		t.Fatalf("user email = %v, want %s", user.Email, email)
	}
	if user.EmailVerifiedAt == nil {
		t.Fatal("user email not marked verified")
	}
	if len(h.sessions.sessions) != 1 {
		t.Fatalf("sessions persisted = %d, want 1", len(h.sessions.sessions))
	}
}

func TestSessionService_Issue_ExistingUserReturnsNewFalse(t *testing.T) {
	t.Parallel()
	h := newSessionHarness()
	phone := mustPhone(t, "+79150000002")
	email := mustEmail(t, "owner@example.com")
	existing := h.seedSessionUser(t, phone, &email)
	ctx := t.Context()

	stores, err := h.stores(ctx)
	if err != nil {
		t.Fatalf("stores: %v", err)
	}

	raw, user, isNew, err := h.svc.Issue(ctx, stores, phone, email, testNow)
	if err != nil {
		t.Fatalf("Issue error = %v", err)
	}
	if isNew {
		t.Fatal("isNew = true, want false for an existing user")
	}
	if user.ID != existing.ID {
		t.Fatalf("user ID = %s, want %s", user.ID, existing.ID)
	}
	if raw.Token == "" {
		t.Fatal("raw token is empty")
	}
	if len(h.sessions.sessions) != 1 {
		t.Fatalf("sessions persisted = %d, want 1", len(h.sessions.sessions))
	}
}

func TestSessionService_Issue_VerifiesEmailWhenUnverified(t *testing.T) {
	t.Parallel()
	h := newSessionHarness()
	phone := mustPhone(t, "+79150000003")
	email := mustEmail(t, "owner@example.com")
	// Seed a user whose email is present but unverified.
	user, err := domain.NewOwner(phone)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	user.Email = &email
	// EmailVerifiedAt left nil on purpose.
	h.users.byPhone[phone.String()] = user
	ctx := t.Context()

	stores, err := h.stores(ctx)
	if err != nil {
		t.Fatalf("stores: %v", err)
	}

	_, got, isNew, err := h.svc.Issue(ctx, stores, phone, email, testNow)
	if err != nil {
		t.Fatalf("Issue error = %v", err)
	}
	if isNew {
		t.Fatal("isNew = true, want false for an existing user")
	}
	if got.EmailVerifiedAt == nil {
		t.Fatal("email not marked verified after Issue")
	}
}

func (h *sessionHarness) seedSessionUser(t *testing.T, phone domain.Phone, email *domain.Email) domain.User {
	t.Helper()
	user, err := domain.NewOwner(phone)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	user.Email = email
	if email != nil {
		user.EmailVerifiedAt = &testNow
	}
	h.users.byPhone[phone.String()] = user
	return user
}

func TestSessionService_Load_DelegatesToRepositoryWithHashedToken(t *testing.T) {
	t.Parallel()
	beginner := &fakeBeginner{}
	userID := uuid.Must(uuid.NewV7())
	rawToken := "plain-token"
	tokenHash := fakeHasher{}.HashToken(rawToken)

	wantSession := domain.Session{
		ID:         uuid.Must(uuid.NewV7()),
		UserID:     userID,
		TokenHash:  tokenHash,
		ExpiresAt:  testNow.Add(domain.SessionBaseTTL),
		CreatedAt:  testNow,
		LastUsedAt: testNow,
	}
	wantUser := domain.User{ID: userID, Role: domain.RoleOwner}
	repo := &capturingSessionRepo{
		onGetByTokenHash: func(_ string, _ time.Time) (domain.Session, domain.User, error) {
			return wantSession, wantUser, nil
		},
	}
	factory := NewTxStoreFactory(
		newFakeUserRepo(), newFakeCodeRepo(), newFakeAttemptRepo(), repo,
		newFakeGrantRepo(), nil, &fakeUoW{beginner: beginner},
	)
	svc := NewSessionService(factory, SessionServiceConfig{Hasher: fakeHasher{}})

	gotSession, gotUser, err := svc.Load(t.Context(), rawToken, testNow)
	if err != nil {
		t.Fatalf("Load error = %v", err)
	}
	if gotSession.ID != wantSession.ID {
		t.Fatalf("session ID = %s, want %s", gotSession.ID, wantSession.ID)
	}
	if gotUser.ID != userID {
		t.Fatalf("user ID = %s, want %s", gotUser.ID, userID)
	}
	if repo.lastTokenHash != tokenHash {
		t.Fatalf("repo received tokenHash = %s, want %s", repo.lastTokenHash, tokenHash)
	}
}

func TestSessionService_Update_DelegatesToRepository(t *testing.T) {
	t.Parallel()
	h := newSessionHarness()
	sess := domain.Session{
		ID:        uuid.Must(uuid.NewV7()),
		TokenHash: "hash-update",
		ExpiresAt: testNow.Add(time.Hour),
	}

	if err := h.svc.Update(t.Context(), sess); err != nil {
		t.Fatalf("Update error = %v", err)
	}
	stored, ok := h.sessions.sessions["hash-update"]
	if !ok {
		t.Fatal("session not persisted by Update")
	}
	if !stored.ExpiresAt.Equal(sess.ExpiresAt) {
		t.Fatalf("stored ExpiresAt = %v, want %v", stored.ExpiresAt, sess.ExpiresAt)
	}
}

// capturingSessionRepo is a SessionRepository whose GetByTokenHash delegates to
// a function stub and captures the token hash, so the Load test can assert the
// raw token was hashed before the repository call.
type capturingSessionRepo struct {
	sessions         map[string]domain.Session
	onGetByTokenHash func(tokenHash string, now time.Time) (domain.Session, domain.User, error)
	lastTokenHash    string
}

func (r *capturingSessionRepo) Create(_ context.Context, s domain.Session) error {
	r.sessions[s.TokenHash] = s
	return nil
}

func (r *capturingSessionRepo) GetByTokenHash(_ context.Context, tokenHash string, now time.Time) (domain.Session, domain.User, error) {
	r.lastTokenHash = tokenHash
	if r.onGetByTokenHash != nil {
		return r.onGetByTokenHash(tokenHash, now)
	}
	return domain.Session{}, domain.User{}, ErrNotFound
}

func (r *capturingSessionRepo) Update(_ context.Context, s domain.Session) error {
	r.sessions[s.TokenHash] = s
	return nil
}

func (r *capturingSessionRepo) DeleteByTokenHash(_ context.Context, hash string) error {
	delete(r.sessions, hash)
	return nil
}
func (r *capturingSessionRepo) DeleteByUserID(_ context.Context, _ uuid.UUID) error { return nil }
func (r *capturingSessionRepo) DeleteByUserIDExcept(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (r *capturingSessionRepo) WithTx(transaction.Tx) (SessionRepository, error) { return r, nil }
