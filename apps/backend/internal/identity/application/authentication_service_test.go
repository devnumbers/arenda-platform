package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

var testNow = time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

// Transaction fake.

type fakeTx struct {
	b    *fakeBeginner
	done bool
}

func (tx *fakeTx) Commit(context.Context) error {
	if tx.done {
		return errors.New("transaction already finished")
	}
	tx.done = true
	tx.b.committed++
	tx.b.open--
	return nil
}

func (tx *fakeTx) Rollback(context.Context) error {
	if tx.done {
		return errors.New("transaction already finished")
	}
	tx.done = true
	tx.b.rolledBack++
	tx.b.open--
	return nil
}

type fakeBeginner struct {
	committed  int
	rolledBack int
	open       int
}

func (b *fakeBeginner) Begin(context.Context) (transaction.Tx, error) {
	b.open++
	return &fakeTx{b: b}, nil
}

// Hasher fake: a real SHA-256 so the constant-time compare path is exercised.

type fakeHasher struct{}

func (fakeHasher) HashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// Repository fakes.

type fakeUserRepo struct {
	byEmail map[string]domain.User
}

func newFakeUserRepo() *fakeUserRepo { return &fakeUserRepo{byEmail: map[string]domain.User{}} }

func (r *fakeUserRepo) GetByID(_ context.Context, id uuid.UUID) (domain.User, error) {
	for _, u := range r.byEmail {
		if u.ID == id {
			return u, nil
		}
	}
	return domain.User{}, ErrNotFound
}

func (r *fakeUserRepo) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.User, error) {
	return r.GetByID(ctx, id)
}

func (r *fakeUserRepo) GetByEmail(_ context.Context, email domain.Email) (domain.User, error) {
	u, ok := r.byEmail[email.String()]
	if !ok {
		return domain.User{}, ErrNotFound
	}
	return u, nil
}

func (r *fakeUserRepo) GetByEmailForUpdate(ctx context.Context, email domain.Email) (domain.User, error) {
	return r.GetByEmail(ctx, email)
}

func (r *fakeUserRepo) Create(_ context.Context, user domain.User) (domain.User, error) {
	r.byEmail[user.Email.String()] = user
	return user, nil
}

func (r *fakeUserRepo) Update(_ context.Context, user domain.User) (domain.User, error) {
	if user.Email != nil {
		r.byEmail[user.Email.String()] = user
	}
	return user, nil
}

func (r *fakeUserRepo) UpdateEmailVerified(
	ctx context.Context, id uuid.UUID, email *domain.Email, verifiedAt *time.Time,
) (domain.User, error) {
	u, err := r.GetByID(ctx, id)
	if err != nil {
		return domain.User{}, err
	}
	if email != nil {
		// An address switch must retire the old key, or GetByID would
		// nondeterministically find the stale row (the map iterates the
		// duplicate).
		if u.Email != nil && u.Email.String() != email.String() {
			delete(r.byEmail, u.Email.String())
		}
		u.Email = email
	}
	u.EmailVerifiedAt = verifiedAt
	if u.Email != nil {
		r.byEmail[u.Email.String()] = u
	}
	return u, nil
}

func (r *fakeUserRepo) MarkEmailVerified(
	ctx context.Context, id uuid.UUID, verifiedAt time.Time,
) (domain.User, error) {
	u, err := r.GetByID(ctx, id)
	if err != nil {
		return domain.User{}, err
	}
	u.EmailVerifiedAt = &verifiedAt
	if u.Email != nil {
		r.byEmail[u.Email.String()] = u
	}
	return u, nil
}

func (r *fakeUserRepo) SetPhoto(ctx context.Context, id uuid.UUID, key, contentType *string) (domain.User, error) {
	u, err := r.GetByID(ctx, id)
	if err != nil {
		return domain.User{}, err
	}
	u.PhotoKey = key
	u.PhotoContentType = contentType
	if u.Email != nil {
		r.byEmail[u.Email.String()] = u
	}
	return u, nil
}

// ClearPhotoIfKey mirrors the conditional UPDATE: only the still-advertised
// key is cleared, a concurrently replaced key survives untouched.
func (r *fakeUserRepo) ClearPhotoIfKey(ctx context.Context, id uuid.UUID, key string) (int64, error) {
	u, err := r.GetByID(ctx, id)
	if err != nil {
		return 0, err
	}
	if u.PhotoKey == nil || *u.PhotoKey != key {
		return 0, nil
	}
	if _, err := r.SetPhoto(ctx, id, nil, nil); err != nil {
		return 0, err
	}
	return 1, nil
}

func (r *fakeUserRepo) WithTx(transaction.Tx) (UserRepository, error) { return r, nil }

type fakeGrantRepo struct {
	byUserID map[uuid.UUID]domain.EmailChangeGrant
}

func newFakeGrantRepo() *fakeGrantRepo {
	return &fakeGrantRepo{byUserID: map[uuid.UUID]domain.EmailChangeGrant{}}
}

func (r *fakeGrantRepo) Save(_ context.Context, grant domain.EmailChangeGrant) error {
	r.byUserID[grant.UserID] = grant
	return nil
}

func (r *fakeGrantRepo) UpdateEmail(_ context.Context, id uuid.UUID, email domain.Email) error {
	for uid, g := range r.byUserID {
		if g.ID == id {
			g.Email = &email
			r.byUserID[uid] = g
			return nil
		}
	}
	return nil
}

func (r *fakeGrantRepo) GetByUserIDForUpdate(_ context.Context, userID uuid.UUID) (domain.EmailChangeGrant, error) {
	g, ok := r.byUserID[userID]
	if !ok {
		return domain.EmailChangeGrant{}, ErrNotFound
	}
	return g, nil
}

func (r *fakeGrantRepo) DeleteByID(_ context.Context, id uuid.UUID) error {
	for uid, g := range r.byUserID {
		if g.ID == id {
			delete(r.byUserID, uid)
			return nil
		}
	}
	return nil
}

func (r *fakeGrantRepo) DeleteByUserID(_ context.Context, userID uuid.UUID) error {
	delete(r.byUserID, userID)
	return nil
}

func (r *fakeGrantRepo) WithTx(transaction.Tx) (EmailChangeGrantRepository, error) { return r, nil }

type fakeCodeRepo struct {
	codes map[uuid.UUID]domain.LoginCode
}

func newFakeCodeRepo() *fakeCodeRepo { return &fakeCodeRepo{codes: map[uuid.UUID]domain.LoginCode{}} }

func (r *fakeCodeRepo) Save(_ context.Context, code domain.LoginCode) error {
	r.codes[code.ID] = code
	return nil
}

func (r *fakeCodeRepo) GetLatestByEmailAndPurpose(
	_ context.Context, email domain.Email, purpose domain.LoginCodePurpose, now time.Time,
) (domain.LoginCode, error) {
	var latest domain.LoginCode
	found := false
	for _, c := range r.codes {
		if c.Email == email && c.Purpose == purpose && !c.Used && c.ExpiresAt.After(now) {
			if !found || c.CreatedAt.After(latest.CreatedAt) {
				latest = c
				found = true
			}
		}
	}
	if !found {
		return domain.LoginCode{}, ErrNotFound
	}
	return latest, nil
}

func (r *fakeCodeRepo) MarkUsedByID(_ context.Context, id uuid.UUID) error {
	c, ok := r.codes[id]
	if !ok {
		return ErrNotFound
	}
	c.Used = true
	r.codes[id] = c
	return nil
}

func (r *fakeCodeRepo) DeleteByID(_ context.Context, id uuid.UUID) error {
	delete(r.codes, id)
	return nil
}

func (r *fakeCodeRepo) DeleteExpiredByEmailAndPurpose(
	_ context.Context, email domain.Email, purpose domain.LoginCodePurpose, before time.Time,
) error {
	for id, c := range r.codes {
		if c.Email == email && c.Purpose == purpose && !c.Used && c.ExpiresAt.Before(before) {
			delete(r.codes, id)
		}
	}
	return nil
}

func (r *fakeCodeRepo) DeleteUnusedByEmailAndPurpose(
	_ context.Context, email domain.Email, purpose domain.LoginCodePurpose,
) error {
	for id, c := range r.codes {
		if c.Email == email && c.Purpose == purpose && !c.Used {
			delete(r.codes, id)
		}
	}
	return nil
}

func (r *fakeCodeRepo) WithTx(transaction.Tx) (LoginCodeRepository, error) { return r, nil }

type fakeAttemptRepo struct {
	windows map[string]domain.AttemptWindow
}

func newFakeAttemptRepo() *fakeAttemptRepo {
	return &fakeAttemptRepo{windows: map[string]domain.AttemptWindow{}}
}

func (r *fakeAttemptRepo) GetByEmail(_ context.Context, email domain.Email) (domain.AttemptWindow, error) {
	w, ok := r.windows[email.String()]
	if !ok {
		return domain.AttemptWindow{}, ErrNotFound
	}
	return w, nil
}

func (r *fakeAttemptRepo) GetByEmailForUpdate(ctx context.Context, email domain.Email) (domain.AttemptWindow, error) {
	return r.GetByEmail(ctx, email)
}

func (r *fakeAttemptRepo) Save(_ context.Context, email domain.Email, window domain.AttemptWindow, delta int) error {
	key := email.String()
	if delta > 0 {
		cur, ok := r.windows[key]
		if ok {
			// Increment path: the database adds the delta; the timestamps come
			// in as absolutes.
			cur.Failures += delta
			cur.FirstFailureAt = window.FirstFailureAt
			cur.LastFailureAt = window.LastFailureAt
			r.windows[key] = cur
			return nil
		}
	}
	r.windows[key] = window
	return nil
}

func (r *fakeAttemptRepo) DeleteByEmail(_ context.Context, email domain.Email) error {
	delete(r.windows, email.String())
	return nil
}

func (r *fakeAttemptRepo) WithTx(transaction.Tx) (AttemptRepository, error) { return r, nil }

type fakeSessionRepo struct {
	sessions map[uuid.UUID]domain.Session
}

func newFakeSessionRepo() *fakeSessionRepo {
	return &fakeSessionRepo{sessions: map[uuid.UUID]domain.Session{}}
}

func (r *fakeSessionRepo) Create(_ context.Context, session domain.Session) error {
	r.sessions[session.ID] = session
	return nil
}

func (r *fakeSessionRepo) GetByTokenHash(context.Context, string, time.Time) (domain.Session, domain.User, error) {
	return domain.Session{}, domain.User{}, ErrNotFound
}

func (r *fakeSessionRepo) Touch(_ context.Context, session domain.Session) error {
	r.sessions[session.ID] = session
	return nil
}

func (r *fakeSessionRepo) Rotate(context.Context, domain.Session, string) (bool, error) {
	return true, nil
}

func (r *fakeSessionRepo) DeleteByTokenHash(_ context.Context, tokenHash string) error {
	for id, s := range r.sessions {
		if s.TokenHash == tokenHash {
			delete(r.sessions, id)
		}
	}
	return nil
}

func (r *fakeSessionRepo) DeleteByUserIDExcept(_ context.Context, userID uuid.UUID, tokenHash string) (int64, error) {
	var removed int64
	for id, s := range r.sessions {
		if s.UserID == userID && s.TokenHash != tokenHash {
			delete(r.sessions, id)
			removed++
		}
	}
	return removed, nil
}

func (r *fakeSessionRepo) ListByUserID(_ context.Context, userID uuid.UUID, _ time.Time) ([]domain.Session, error) {
	out := make([]domain.Session, 0, len(r.sessions))
	for _, s := range r.sessions {
		if s.UserID == userID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (r *fakeSessionRepo) GetByID(_ context.Context, id uuid.UUID) (domain.Session, error) {
	s, ok := r.sessions[id]
	if !ok {
		return domain.Session{}, ErrNotFound
	}
	return s, nil
}

func (r *fakeSessionRepo) DeleteByIDForUser(_ context.Context, id, userID uuid.UUID) (bool, error) {
	s, ok := r.sessions[id]
	if !ok || s.UserID != userID {
		return false, nil
	}
	delete(r.sessions, id)
	return true, nil
}

func (r *fakeSessionRepo) WithTx(transaction.Tx) (SessionRepository, error) { return r, nil }

// Sender and publisher fakes.

type sentCode struct {
	email   domain.Email
	code    string
	purpose domain.LoginCodePurpose
	step    domain.LoginCodeStep
}

type fakeCodeSender struct {
	sent []sentCode
	err  error
}

func (s *fakeCodeSender) Send(
	_ context.Context,
	email domain.Email,
	code string,
	purpose domain.LoginCodePurpose,
	step domain.LoginCodeStep,
) error {
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, sentCode{email: email, code: code, purpose: purpose, step: step})
	return nil
}

type fakePublisher struct {
	registered []UserRegistered
}

func (p *fakePublisher) PublishUserRegistered(_ context.Context, event UserRegistered) error {
	p.registered = append(p.registered, event)
	return nil
}

// Harness.

type authServiceHarness struct {
	*fakeStores
	svc       *AuthenticationService
	loginCode *LoginCodeService
	sender    *fakeCodeSender
	publisher *fakePublisher
}

func newAuthServiceHarness() *authServiceHarness {
	stores := newFakeStores()
	h := &authServiceHarness{
		fakeStores: stores,
		sender:     &fakeCodeSender{},
		publisher:  &fakePublisher{},
	}
	factory := stores.factory(nil)
	loginCodes := NewLoginCodeService(factory, LoginCodeServiceConfig{
		CodeSender: h.sender,
		Clock:      &fakeClock{now: testNow},
		Hasher:     fakeHasher{},
		Logger:     discardLogger(),
	})
	sessionSvc := NewSessionService(factory, SessionServiceConfig{Hasher: fakeHasher{}})
	h.loginCode = loginCodes
	h.svc = NewAuthenticationService(factory, AuthenticationServiceConfig{
		LoginCodes: loginCodes,
		Sessions:   sessionSvc,
		Clock:      &fakeClock{now: testNow},
		Publisher:  h.publisher,
		Logger:     discardLogger(),
	})
	return h
}

func (h *authServiceHarness) seedUser(t *testing.T, email domain.Email) domain.User {
	t.Helper()
	user, err := domain.NewOwner(email)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	user.EmailVerifiedAt = &testNow
	h.users.byEmail[email.String()] = user
	return user
}

func mustEmail(t *testing.T, raw string) domain.Email {
	t.Helper()
	email, err := domain.NewEmail(raw)
	if err != nil {
		t.Fatalf("parse email: %v", err)
	}
	return email
}

// SendCode: entry and registration are one flow (ADR 0067) — every address
// gets a code, and the response never reveals whether an account exists
// (anti-enumeration, research #1526). The user id rides along only when the
// account already exists.

func TestAuthenticationService_SendCode_ExistingEmailAttachesUser(t *testing.T) {
	t.Parallel()
	h := newAuthServiceHarness()
	email := mustEmail(t, "owner@example.com")
	seeded := h.seedUser(t, email)

	if err := h.svc.SendCode(t.Context(), email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode error = %v", err)
	}
	if len(h.sender.sent) != 1 {
		t.Fatalf("sender calls = %d, want 1", len(h.sender.sent))
	}
	if h.sender.sent[0].email != email {
		t.Fatalf("sender email = %s, want %s", h.sender.sent[0].email, email)
	}
	if len(h.codes.codes) != 1 {
		t.Fatalf("codes saved = %d, want 1", len(h.codes.codes))
	}
	for _, c := range h.codes.codes {
		if c.UserID == nil || *c.UserID != seeded.ID {
			t.Fatalf("code user id = %v, want %s", c.UserID, seeded.ID)
		}
	}
}

// TestAuthenticationService_SendCode_UnknownEmailStillSends pins the
// anti-enumeration contract: a code for an address the platform does not know
// is persisted (with its hash) and delivered exactly like for an existing
// account — the only observable difference is the absent user id on the row.
func TestAuthenticationService_SendCode_UnknownEmailStillSends(t *testing.T) {
	t.Parallel()
	h := newAuthServiceHarness()
	email := mustEmail(t, "ghost@example.com")

	if err := h.svc.SendCode(t.Context(), email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode error = %v", err)
	}
	if len(h.sender.sent) != 1 {
		t.Fatalf("sender calls = %d, want 1", len(h.sender.sent))
	}
	if len(h.codes.codes) != 1 {
		t.Fatalf("codes saved = %d, want 1", len(h.codes.codes))
	}
	for _, c := range h.codes.codes {
		if c.UserID != nil {
			t.Fatalf("code user id = %s, want nil for an unknown address", *c.UserID)
		}
	}
	if len(h.publisher.registered) != 0 {
		t.Fatalf("PublishUserRegistered calls = %d, want 0 at send time", len(h.publisher.registered))
	}
}

// TestAuthenticationService_SendCode_GetByEmailError asserts an infrastructure
// error from GetByEmail (not ErrNotFound) is propagated.
func TestAuthenticationService_SendCode_GetByEmailError(t *testing.T) {
	t.Parallel()
	stores := newFakeStores()
	sender := &fakeCodeSender{}
	dbErr := errors.New("db connection lost")

	factory := NewTxStoreFactory(
		&errorUserRepo{err: dbErr}, stores.codes, stores.attempts, stores.sessions,
		newFakeGrantRepo(), auditapp.Noop{}, nil, &fakeUoW{beginner: stores.beginner},
	)
	loginCodes := NewLoginCodeService(factory, LoginCodeServiceConfig{
		CodeSender: sender, Clock: &fakeClock{now: testNow},
		Hasher: fakeHasher{}, Logger: discardLogger(),
	})
	svc := NewAuthenticationService(factory, AuthenticationServiceConfig{
		LoginCodes: loginCodes,
		Sessions:   NewSessionService(factory, SessionServiceConfig{Hasher: fakeHasher{}}),
		Clock:      &fakeClock{now: testNow},
		Publisher:  &fakePublisher{},
		Logger:     discardLogger(),
	})

	err := svc.SendCode(t.Context(),
		mustEmail(t, "owner@example.com"),
		domain.LoginCodePurposeLogin,
	)
	if !errors.Is(err, dbErr) {
		t.Fatalf("SendCode error = %v, want wrap of dbErr", err)
	}
	if len(sender.sent) != 0 {
		t.Fatalf("sender calls = %d, want 0 on get error", len(sender.sent))
	}
}

// VerifyCode.

func TestAuthenticationService_VerifyCode_ExistingEmailLogin(t *testing.T) {
	t.Parallel()
	h := newAuthServiceHarness()
	email := mustEmail(t, "owner@example.com")
	user := h.seedUser(t, email)
	ctx := t.Context()

	if err := h.svc.SendCode(ctx, email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode error = %v", err)
	}
	code := h.sender.sent[0].code

	raw, got, err := h.svc.VerifyCode(ctx, email, code, nil, DeviceContext{})
	if err != nil {
		t.Fatalf("VerifyCode error = %v", err)
	}
	if raw.Token == "" {
		t.Fatal("VerifyCode returned an empty session token")
	}
	if got.ID != user.ID {
		t.Fatalf("VerifyCode user ID = %s, want %s", got.ID, user.ID)
	}
	if len(h.sessions.sessions) != 1 {
		t.Fatalf("sessions created = %d, want 1", len(h.sessions.sessions))
	}
	if len(h.publisher.registered) != 0 {
		t.Fatalf("PublishUserRegistered calls = %d, want 0 for an existing user", len(h.publisher.registered))
	}
}

func TestAuthenticationService_VerifyCode_NewEmailRegisters(t *testing.T) {
	t.Parallel()
	h := newAuthServiceHarness()
	email := mustEmail(t, "new@example.com")
	ctx := t.Context()

	if err := h.svc.SendCode(ctx, email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode error = %v", err)
	}
	code := h.sender.sent[0].code

	raw, user, err := h.svc.VerifyCode(ctx, email, code, nil, DeviceContext{})
	if err != nil {
		t.Fatalf("VerifyCode error = %v", err)
	}
	if raw.Token == "" {
		t.Fatal("VerifyCode returned an empty session token")
	}
	if user.Email == nil || *user.Email != email {
		t.Fatalf("user email = %v, want %s", user.Email, email)
	}
	if user.EmailVerifiedAt == nil {
		t.Fatal("registered user email must be verified")
	}
	if len(h.publisher.registered) != 1 {
		t.Fatalf("PublishUserRegistered calls = %d, want 1 for a new user", len(h.publisher.registered))
	}
	if got := h.publisher.registered[0]; got.Email != email {
		t.Fatalf("registered event email = %s, want %s", got.Email, email)
	}
}

// The #451 rule: the browser-detected timezone travels with the verify request
// and is applied only when the verify creates the account. An absent or invalid
// value keeps the creation default (Europe/Moscow in PostgreSQL), and an
// existing user's saved zone is never overwritten — the manual choice in the
// profile picker is authoritative.

// TestRegistrationTimezone_Table covers the raw-request validation in a table:
// an absent or unparsable value yields nil (the account keeps the
// Europe/Moscow creation default), a valid IANA identifier is kept.
func TestRegistrationTimezone_Table(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  *string
		want *string
	}{
		{"nil field keeps the default", nil, nil},
		{"empty field keeps the default", new(""), nil},
		{"unparsable zone keeps the default", new("Mars/Olympus"), nil},
		{"valid zone is kept", new("Asia/Yekaterinburg"), new("Asia/Yekaterinburg")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := registrationTimezone(tt.raw)
			switch {
			case tt.want == nil && got != nil:
				t.Fatalf("registrationTimezone(%v) = %q, want nil", tt.raw, got.String())
			case tt.want != nil && got == nil:
				t.Fatalf("registrationTimezone(%v) = nil, want %q", tt.raw, *tt.want)
			case tt.want != nil && got.String() != *tt.want:
				t.Fatalf("registrationTimezone(%v) = %q, want %q", tt.raw, got.String(), *tt.want)
			}
		})
	}
}

func TestAuthenticationService_VerifyCode_RegistrationTimezone_NewUser(t *testing.T) {
	t.Parallel()
	h := newAuthServiceHarness()
	email := mustEmail(t, "tz@example.com")
	ctx := t.Context()

	if err := h.svc.SendCode(ctx, email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode error = %v", err)
	}
	code := h.sender.sent[0].code
	timezone := "Asia/Yekaterinburg"

	_, user, err := h.svc.VerifyCode(ctx, email, code, &timezone, DeviceContext{})
	if err != nil {
		t.Fatalf("VerifyCode error = %v", err)
	}
	if user.Timezone.String() != timezone {
		t.Fatalf("returned user timezone = %q, want %q", user.Timezone.String(), timezone)
	}
	stored := h.users.byEmail[email.String()]
	if stored.Timezone.String() != timezone {
		t.Fatalf("stored user timezone = %q, want %q", stored.Timezone.String(), timezone)
	}
	if len(h.publisher.registered) != 1 {
		t.Fatalf("PublishUserRegistered calls = %d, want 1 for a new user", len(h.publisher.registered))
	}
}

func TestAuthenticationService_VerifyCode_RegistrationTimezone_InvalidFallsBack(t *testing.T) {
	t.Parallel()
	h := newAuthServiceHarness()
	email := mustEmail(t, "badtz@example.com")
	ctx := t.Context()

	if err := h.svc.SendCode(ctx, email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode error = %v", err)
	}
	code := h.sender.sent[0].code
	timezone := "Mars/Olympus"

	_, _, err := h.svc.VerifyCode(ctx, email, code, &timezone, DeviceContext{})
	if err != nil {
		t.Fatalf("VerifyCode error = %v", err)
	}
	stored := h.users.byEmail[email.String()]
	if stored.Timezone.String() != "" {
		t.Fatalf("stored user timezone = %q, want the untouched creation default", stored.Timezone.String())
	}
}

func TestAuthenticationService_VerifyCode_RegistrationTimezone_ExistingUntouched(t *testing.T) {
	t.Parallel()
	h := newAuthServiceHarness()
	email := mustEmail(t, "moscow@example.com")
	seeded := h.seedUser(t, email)
	saved, err := domain.NewTimezone("Europe/Moscow")
	if err != nil {
		t.Fatalf("NewTimezone: %v", err)
	}
	seeded.Timezone = saved
	h.users.byEmail[email.String()] = seeded
	ctx := t.Context()

	if err := h.svc.SendCode(ctx, email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode error = %v", err)
	}
	code := h.sender.sent[0].code
	timezone := "Asia/Yekaterinburg"

	_, user, err := h.svc.VerifyCode(ctx, email, code, &timezone, DeviceContext{})
	if err != nil {
		t.Fatalf("VerifyCode error = %v", err)
	}
	if user.Timezone.String() != "Europe/Moscow" {
		t.Fatalf("returned user timezone = %q, want saved Europe/Moscow", user.Timezone.String())
	}
	stored := h.users.byEmail[email.String()]
	if stored.Timezone.String() != "Europe/Moscow" {
		t.Fatalf("stored user timezone = %q, want saved Europe/Moscow", stored.Timezone.String())
	}
	if len(h.publisher.registered) != 0 {
		t.Fatalf("PublishUserRegistered calls = %d, want 0 for an existing user", len(h.publisher.registered))
	}
}

// TestAuthenticationService_VerifyCode_InvalidCodeRecordsAttemptAndAudit
// verifies the post-refactor failed-login behavior (ADR 0033): an invalid code
// rolls back the success path, then a separate short transaction records the
// attempt-window failure so rate-limiting survives the rollback. The pre-refactor
// early-Commit-on-error is gone.
func TestAuthenticationService_VerifyCode_InvalidCodeRecordsAttemptAndAudit(t *testing.T) {
	t.Parallel()
	h := newAuthServiceHarness()
	email := mustEmail(t, "owner@example.com")
	h.seedUser(t, email)
	ctx := t.Context()

	// Issue a real code, then verify with a wrong one.
	if err := h.svc.SendCode(ctx, email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode error = %v", err)
	}

	_, _, err := h.svc.VerifyCode(ctx, email, "000000", nil, DeviceContext{})
	if !errors.Is(err, domain.ErrLoginCodeInvalid) {
		t.Fatalf("VerifyCode error = %v, want ErrLoginCodeInvalid", err)
	}

	// The attempt-window failure was recorded despite the success-path rollback.
	window, ok := h.attempts.windows[email.String()]
	if !ok {
		t.Fatal("attempt window not recorded after invalid verify")
	}
	if window.Failures != 1 {
		t.Fatalf("window failures = %d, want 1", window.Failures)
	}

	// No session was created for the failed verification.
	if len(h.sessions.sessions) != 0 {
		t.Fatalf("sessions created = %d, want 0 on invalid verify", len(h.sessions.sessions))
	}
	// No registration event was published.
	if len(h.publisher.registered) != 0 {
		t.Fatalf("PublishUserRegistered calls = %d, want 0", len(h.publisher.registered))
	}
}

// TestAuthenticationService_VerifyCode_TooManyAttemptsBlocks ensures repeated
// invalid verifications reach the attempt threshold and then block further
// attempts.
func TestAuthenticationService_VerifyCode_TooManyAttemptsBlocks(t *testing.T) {
	t.Parallel()
	h := newAuthServiceHarness()
	email := mustEmail(t, "owner@example.com")
	h.seedUser(t, email)
	ctx := t.Context()

	if err := h.svc.SendCode(ctx, email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode error = %v", err)
	}

	var lastErr error
	for range domain.MaxLoginFailures {
		_, _, lastErr = h.svc.VerifyCode(ctx, email, "000000", nil, DeviceContext{})
	}
	if !errors.Is(lastErr, domain.ErrTooManyAttempts) {
		t.Fatalf("last VerifyCode error = %v, want ErrTooManyAttempts", lastErr)
	}

	// After hitting the threshold, a subsequent send is blocked.
	err := h.svc.SendCode(ctx, email, domain.LoginCodePurposeLogin)
	if !errors.Is(err, ErrUserBlocked) {
		t.Fatalf("SendCode after block = %v, want ErrUserBlocked", err)
	}
}

// TestAuthenticationService_VerifyCode_BlockedEmailSkipsRecovery proves the
// recovery-branch narrowing (#239): verifying a code on an already-blocked
// address returns ErrUserBlocked — surfaced authoritatively by
// LoginCodeService.Verify inside the runInTx — without invoking recovery
// (RecordFailureAndAudit). The attempt that caused the block was recorded when
// the block took effect, so the recovery-free `case err != nil` branch is
// correct: the failure counter must not grow beyond MaxLoginFailures.
func TestAuthenticationService_VerifyCode_BlockedEmailSkipsRecovery(t *testing.T) {
	t.Parallel()
	h := newAuthServiceHarness()
	email := mustEmail(t, "owner@example.com")
	h.seedUser(t, email)
	ctx := t.Context()

	if err := h.svc.SendCode(ctx, email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode error = %v", err)
	}

	// Drive exactly MaxLoginFailures invalid verifications to reach the block.
	var lastErr error
	for range domain.MaxLoginFailures {
		_, _, lastErr = h.svc.VerifyCode(ctx, email, "000000", nil, DeviceContext{})
	}
	if !errors.Is(lastErr, domain.ErrTooManyAttempts) {
		t.Fatalf("last VerifyCode error = %v, want ErrTooManyAttempts", lastErr)
	}

	// The address is now blocked. A further verify must return ErrUserBlocked
	// straight from LoginCodeService.Verify, without recovery.
	_, _, err := h.svc.VerifyCode(ctx, email, "000000", nil, DeviceContext{})
	if !errors.Is(err, ErrUserBlocked) {
		t.Fatalf("VerifyCode on blocked address error = %v, want ErrUserBlocked", err)
	}

	// Recovery was not invoked: the failure counter stays at MaxLoginFailures.
	// A spurious increment would prove RecordFailureAndAudit ran despite the
	// block, which the contract forbids.
	window, ok := h.attempts.windows[email.String()]
	if !ok {
		t.Fatal("attempt window missing for blocked address")
	}
	if window.Failures != domain.MaxLoginFailures {
		t.Fatalf("window failures after blocked verify = %d, want %d (recovery must not run)",
			window.Failures, domain.MaxLoginFailures)
	}
}

// errorUserRepo is a UserRepository whose GetByEmail always fails with err.
type errorUserRepo struct{ err error }

func (r *errorUserRepo) GetByID(context.Context, uuid.UUID) (domain.User, error) {
	return domain.User{}, r.err
}

func (r *errorUserRepo) GetByIDForUpdate(context.Context, uuid.UUID) (domain.User, error) {
	return domain.User{}, r.err
}

func (r *errorUserRepo) GetByEmail(context.Context, domain.Email) (domain.User, error) {
	return domain.User{}, r.err
}

func (r *errorUserRepo) GetByEmailForUpdate(context.Context, domain.Email) (domain.User, error) {
	return domain.User{}, r.err
}

func (r *errorUserRepo) Create(_ context.Context, user domain.User) (domain.User, error) {
	return user, nil
}

func (r *errorUserRepo) Update(_ context.Context, user domain.User) (domain.User, error) {
	return user, nil
}

func (r *errorUserRepo) UpdateEmailVerified(context.Context, uuid.UUID, *domain.Email, *time.Time) (domain.User, error) {
	return domain.User{}, nil
}

func (r *errorUserRepo) MarkEmailVerified(context.Context, uuid.UUID, time.Time) (domain.User, error) {
	return domain.User{}, nil
}

func (r *errorUserRepo) SetPhoto(context.Context, uuid.UUID, *string, *string) (domain.User, error) {
	return domain.User{}, r.err
}

func (r *errorUserRepo) ClearPhotoIfKey(context.Context, uuid.UUID, string) (int64, error) {
	return 0, r.err
}

func (r *errorUserRepo) WithTx(transaction.Tx) (UserRepository, error) { return r, nil }
