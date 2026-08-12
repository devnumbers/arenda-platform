package application

import (
	"context"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

type sessionHarness struct {
	svc      *sessionService
	users    *fakeUserRepo
	codes    *fakeCodeRepo
	attempts *fakeAttemptRepo
	sessions *fakeSessionRepo
	beginner *fakeBeginner
}

func newSessionHarness() *sessionHarness {
	h := &sessionHarness{
		users:    newFakeUserRepo(),
		codes:    newFakeCodeRepo(),
		attempts: newFakeAttemptRepo(),
		sessions: newFakeSessionRepo(),
		beginner: &fakeBeginner{},
	}
	h.svc = NewSessionService(h.users, h.codes, h.attempts, h.sessions, fakeHasher{}, SessionServiceConfig{
		UoW: &fakeUoW{beginner: h.beginner},
	})
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
	h := newSessionHarness()
	phone := mustPhone(t, "+79150000001")
	email := mustEmail(t, "owner@example.com")
	ctx := context.Background()

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
	h := newSessionHarness()
	phone := mustPhone(t, "+79150000002")
	email := mustEmail(t, "owner@example.com")
	existing := h.seedSessionUser(t, phone, &email)
	ctx := context.Background()

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
	ctx := context.Background()

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
