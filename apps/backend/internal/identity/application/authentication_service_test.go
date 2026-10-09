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
	if !tx.done {
		tx.b.open--
		tx.b.committed++
		tx.done = true
	}
	return tx.b.commitErr
}

func (tx *fakeTx) Rollback(context.Context) error {
	if !tx.done {
		tx.b.rolledBack++
		tx.b.open--
		tx.done = true
	}
	return nil
}

type fakeBeginner struct {
	begun, committed, rolledBack, open int
	beginErr                           error
	commitErr                          error
}

func (b *fakeBeginner) Begin(context.Context) (transaction.Tx, error) {
	if b.beginErr != nil {
		return nil, b.beginErr
	}
	b.begun++
	b.open++
	return &fakeTx{b: b}, nil
}

// Hasher fake.

// fakeHasher produces deterministic hex output, mirroring
// encryption.hashToken, so domain.LoginCode.Verify can hex-decode it.
type fakeHasher struct{}

func (fakeHasher) HashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// Repository fakes.

type fakeUserRepo struct {
	byPhone map[string]domain.User
}

func newFakeUserRepo() *fakeUserRepo { return &fakeUserRepo{byPhone: map[string]domain.User{}} }

func (r *fakeUserRepo) GetByID(_ context.Context, id uuid.UUID) (domain.User, error) {
	for _, u := range r.byPhone {
		if u.ID == id {
			return u, nil
		}
	}
	return domain.User{}, ErrNotFound
}

func (r *fakeUserRepo) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.User, error) {
	return r.GetByID(ctx, id)
}

func (r *fakeUserRepo) GetByPhone(_ context.Context, phone domain.Phone) (domain.User, error) {
	u, ok := r.byPhone[phone.String()]
	if !ok {
		return domain.User{}, ErrNotFound
	}
	return u, nil
}

func (r *fakeUserRepo) GetByPhoneForUpdate(ctx context.Context, phone domain.Phone) (domain.User, error) {
	return r.GetByPhone(ctx, phone)
}

func (r *fakeUserRepo) GetByEmail(_ context.Context, email domain.Email) (domain.User, error) {
	for _, u := range r.byPhone {
		if u.Email != nil && *u.Email == email {
			return u, nil
		}
	}
	return domain.User{}, ErrNotFound
}

func (r *fakeUserRepo) GetByEmailForUpdate(ctx context.Context, email domain.Email) (domain.User, error) {
	return r.GetByEmail(ctx, email)
}

func (r *fakeUserRepo) Create(_ context.Context, user domain.User) (domain.User, error) {
	r.byPhone[user.Phone.String()] = user
	return user, nil
}

func (r *fakeUserRepo) Update(_ context.Context, user domain.User) (domain.User, error) {
	r.byPhone[user.Phone.String()] = user
	return user, nil
}

func (r *fakeUserRepo) UpdatePhone(ctx context.Context, id uuid.UUID, phone domain.Phone) (domain.User, error) {
	u, err := r.GetByID(ctx, id)
	if err != nil {
		return domain.User{}, err
	}
	delete(r.byPhone, u.Phone.String())
	u.Phone = phone
	r.byPhone[phone.String()] = u
	return u, nil
}

func (r *fakeUserRepo) UpdateEmailVerified(
	ctx context.Context,
	id uuid.UUID,
	email *domain.Email,
	verifiedAt *time.Time,
) (domain.User, error) {
	u, err := r.GetByID(ctx, id)
	if err != nil {
		return domain.User{}, err
	}
	u.Email = email
	u.EmailVerifiedAt = verifiedAt
	r.byPhone[u.Phone.String()] = u
	return u, nil
}

func (r *fakeUserRepo) MarkEmailVerified(
	ctx context.Context,
	id uuid.UUID,
	verifiedAt time.Time,
) (domain.User, error) {
	u, err := r.GetByID(ctx, id)
	if err != nil {
		return domain.User{}, err
	}
	u.EmailVerifiedAt = &verifiedAt
	r.byPhone[u.Phone.String()] = u
	return u, nil
}

func (r *fakeUserRepo) SetPhoto(ctx context.Context, id uuid.UUID, key, contentType *string) (domain.User, error) {
	u, err := r.GetByID(ctx, id)
	if err != nil {
		return domain.User{}, err
	}
	u.PhotoKey = key
	u.PhotoContentType = contentType
	r.byPhone[u.Phone.String()] = u
	return u, nil
}

func (r *fakeUserRepo) WithTx(transaction.Tx) (UserRepository, error) { return r, nil }

type fakeGrantRepo struct {
	grants map[uuid.UUID]domain.EmailChangeGrant
}

func newFakeGrantRepo() *fakeGrantRepo {
	return &fakeGrantRepo{grants: map[uuid.UUID]domain.EmailChangeGrant{}}
}

func (r *fakeGrantRepo) Save(_ context.Context, grant domain.EmailChangeGrant) error {
	r.grants[grant.UserID] = grant
	return nil
}

func (r *fakeGrantRepo) GetByUserIDForUpdate(_ context.Context, userID uuid.UUID) (domain.EmailChangeGrant, error) {
	g, ok := r.grants[userID]
	if !ok {
		return domain.EmailChangeGrant{}, ErrNotFound
	}
	return g, nil
}

func (r *fakeGrantRepo) DeleteByID(_ context.Context, id uuid.UUID) error {
	for userID, g := range r.grants {
		if g.ID == id {
			delete(r.grants, userID)
		}
	}
	return nil
}

func (r *fakeGrantRepo) DeleteByUserID(_ context.Context, userID uuid.UUID) error {
	delete(r.grants, userID)
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

func (r *fakeCodeRepo) GetLatestByPhoneAndEmail(
	_ context.Context,
	phone domain.Phone,
	email domain.Email,
	purpose domain.LoginCodePurpose,
	now time.Time,
) (domain.LoginCode, error) {
	var latest domain.LoginCode
	found := false
	for _, c := range r.codes {
		if c.Phone != phone || c.Email != email || c.Purpose != purpose || !c.ExpiresAt.After(now) {
			continue
		}
		if !found || c.CreatedAt.After(latest.CreatedAt) {
			latest = c
			found = true
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

func (r *fakeCodeRepo) DeleteByUserID(_ context.Context, userID uuid.UUID) error {
	for id, c := range r.codes {
		if c.UserID != nil && *c.UserID == userID {
			delete(r.codes, id)
		}
	}
	return nil
}

func (r *fakeCodeRepo) DeleteExpiredByPhoneAndEmail(
	_ context.Context,
	phone domain.Phone,
	email domain.Email,
	purpose domain.LoginCodePurpose,
	before time.Time,
) error {
	for id, c := range r.codes {
		if c.Phone == phone && c.Email == email && c.Purpose == purpose && !c.ExpiresAt.After(before) {
			delete(r.codes, id)
		}
	}
	return nil
}

func (r *fakeCodeRepo) DeleteUnusedByPhoneAndEmail(
	_ context.Context,
	phone domain.Phone,
	email domain.Email,
	purpose domain.LoginCodePurpose,
) error {
	for id, c := range r.codes {
		if c.Phone == phone && c.Email == email && c.Purpose == purpose && !c.Used {
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

func (r *fakeAttemptRepo) GetByPhone(_ context.Context, phone domain.Phone) (domain.AttemptWindow, error) {
	w, ok := r.windows[phone.String()]
	if !ok {
		return domain.AttemptWindow{}, ErrNotFound
	}
	return w, nil
}

func (r *fakeAttemptRepo) GetByPhoneForUpdate(ctx context.Context, phone domain.Phone) (domain.AttemptWindow, error) {
	return r.GetByPhone(ctx, phone)
}

func (r *fakeAttemptRepo) Save(_ context.Context, phone domain.Phone, _ uuid.UUID, window domain.AttemptWindow, delta int) error {
	if delta <= 0 {
		// Reset path: write the absolute counter.
		r.windows[phone.String()] = window
		return nil
	}
	// Increment path: add the delta to the previously stored absolute.
	prev := r.windows[phone.String()]
	window.Failures = prev.Failures + delta
	r.windows[phone.String()] = window
	return nil
}

func (r *fakeAttemptRepo) DeleteByPhone(_ context.Context, phone domain.Phone) error {
	delete(r.windows, phone.String())
	return nil
}

func (r *fakeAttemptRepo) DeleteByUserID(context.Context, uuid.UUID) error { return nil }

func (r *fakeAttemptRepo) WithTx(transaction.Tx) (AttemptRepository, error) { return r, nil }

type fakeSessionRepo struct {
	sessions map[string]domain.Session
}

func newFakeSessionRepo() *fakeSessionRepo {
	return &fakeSessionRepo{sessions: map[string]domain.Session{}}
}

func (r *fakeSessionRepo) Create(_ context.Context, session domain.Session) error {
	r.sessions[session.TokenHash] = session
	return nil
}

func (r *fakeSessionRepo) GetByTokenHash(context.Context, string, time.Time) (domain.Session, domain.User, error) {
	return domain.Session{}, domain.User{}, ErrNotFound
}

func (r *fakeSessionRepo) Update(_ context.Context, session domain.Session) error {
	r.sessions[session.TokenHash] = session
	return nil
}

func (r *fakeSessionRepo) DeleteByTokenHash(_ context.Context, tokenHash string) error {
	delete(r.sessions, tokenHash)
	return nil
}

func (r *fakeSessionRepo) DeleteByUserIDExcept(_ context.Context, userID uuid.UUID, tokenHash string) (int64, error) {
	var removed int64
	for hash, s := range r.sessions {
		if s.UserID == userID && hash != tokenHash {
			delete(r.sessions, hash)
			removed++
		}
	}
	return removed, nil
}

func (r *fakeSessionRepo) Touch(_ context.Context, session domain.Session) error {
	r.sessions[session.TokenHash] = session
	return nil
}

func (r *fakeSessionRepo) Rotate(context.Context, domain.Session, string) (bool, error) {
	return true, nil
}

func (r *fakeSessionRepo) ListByUserID(_ context.Context, userID uuid.UUID, _ time.Time) ([]domain.Session, error) {
	var out []domain.Session
	for _, s := range r.sessions {
		if s.UserID == userID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (r *fakeSessionRepo) GetByID(_ context.Context, id uuid.UUID) (domain.Session, error) {
	for _, s := range r.sessions {
		if s.ID == id {
			return s, nil
		}
	}
	return domain.Session{}, ErrNotFound
}

func (r *fakeSessionRepo) DeleteByIDForUser(_ context.Context, id, userID uuid.UUID) (bool, error) {
	for hash, s := range r.sessions {
		if s.ID == id && s.UserID == userID {
			delete(r.sessions, hash)
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeSessionRepo) WithTx(transaction.Tx) (SessionRepository, error) { return r, nil }

// Sender and publisher fakes.

type sentCode struct {
	phone domain.Phone
	email domain.Email
	code  string
}

type fakeCodeSender struct {
	sent []sentCode
	err  error
}

func (s *fakeCodeSender) Send(_ context.Context, phone domain.Phone, email domain.Email, code string) error {
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, sentCode{phone: phone, email: email, code: code})
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
	h.svc = NewAuthenticationService(factory, AuthenticationServiceConfig{
		LoginCodes: loginCodes,
		Sessions:   sessionSvc,
		Clock:      &fakeClock{now: testNow},
		Publisher:  h.publisher,
		Logger:     discardLogger(),
	})
	return h
}

func (h *authServiceHarness) seedUser(t *testing.T, phone domain.Phone, email *domain.Email) domain.User {
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

func mustPhone(t *testing.T, raw string) domain.Phone {
	t.Helper()
	phone, err := domain.NewPhone(raw)
	if err != nil {
		t.Fatalf("parse phone: %v", err)
	}
	return phone
}

func mustEmail(t *testing.T, raw string) domain.Email {
	t.Helper()
	email, err := domain.NewEmail(raw)
	if err != nil {
		t.Fatalf("parse email: %v", err)
	}
	return email
}

func TestAuthenticationService_SendCodeByPhone(t *testing.T) {
	t.Parallel()
	phone := mustPhone(t, "+79150000001")
	email := mustEmail(t, "owner@example.com")

	tests := []struct {
		name      string
		seed      func(t *testing.T, h *authServiceHarness)
		wantSent  bool
		wantCalls int
	}{
		{
			name: "user with stored email gets a code",
			seed: func(t *testing.T, h *authServiceHarness) {
				t.Helper()
				h.seedUser(t, phone, &email)
			},
			wantSent:  true,
			wantCalls: 1,
		},
		{
			name:      "unknown phone returns sent false without sending",
			seed:      func(*testing.T, *authServiceHarness) {},
			wantSent:  false,
			wantCalls: 0,
		},
		{
			name: "user without email returns sent false without sending",
			seed: func(t *testing.T, h *authServiceHarness) {
				t.Helper()
				h.seedUser(t, phone, nil)
			},
			wantSent:  false,
			wantCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newAuthServiceHarness()
			tt.seed(t, h)

			sent, err := h.svc.SendCodeByPhone(t.Context(), phone)
			if err != nil {
				t.Fatalf("SendCodeByPhone error = %v", err)
			}
			if sent != tt.wantSent {
				t.Fatalf("SendCodeByPhone sent = %v, want %v", sent, tt.wantSent)
			}
			if len(h.sender.sent) != tt.wantCalls {
				t.Fatalf("sender calls = %d, want %d", len(h.sender.sent), tt.wantCalls)
			}
			if tt.wantSent && h.sender.sent[0].email != email {
				t.Fatalf("sender email = %s, want stored %s", h.sender.sent[0].email, email)
			}
		})
	}
}

func TestAuthenticationService_VerifyCode_ResolvesEmailFromUser(t *testing.T) {
	t.Parallel()
	h := newAuthServiceHarness()
	phone := mustPhone(t, "+79150000001")
	email := mustEmail(t, "owner@example.com")
	user := h.seedUser(t, phone, &email)
	ctx := t.Context()

	sent, err := h.svc.SendCodeByPhone(ctx, phone)
	if err != nil || !sent {
		t.Fatalf("SendCodeByPhone = %v, %v", sent, err)
	}
	code := h.sender.sent[0].code

	raw, got, err := h.svc.VerifyCode(ctx, phone, nil, code, nil, DeviceContext{})
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

func TestAuthenticationService_VerifyCode_UnknownPhoneWithoutEmail(t *testing.T) {
	t.Parallel()
	h := newAuthServiceHarness()
	phone := mustPhone(t, "+79150000009")

	_, _, err := h.svc.VerifyCode(t.Context(), phone, nil, "123456", nil, DeviceContext{})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("VerifyCode error = %v, want ErrNotFound", err)
	}
}

func TestAuthenticationService_VerifyCode_UserWithoutEmailWithoutEmail(t *testing.T) {
	t.Parallel()
	h := newAuthServiceHarness()
	phone := mustPhone(t, "+79150000008")
	h.seedUser(t, phone, nil)

	_, _, err := h.svc.VerifyCode(t.Context(), phone, nil, "123456", nil, DeviceContext{})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("VerifyCode error = %v, want ErrNotFound", err)
	}
}

func TestAuthenticationService_ExplicitEmailFlow(t *testing.T) {
	t.Parallel()
	t.Run("new user registers with explicit email", func(t *testing.T) {
		t.Parallel()
		h := newAuthServiceHarness()
		phone := mustPhone(t, "+79150000005")
		email := mustEmail(t, "new@example.com")
		ctx := t.Context()

		if err := h.svc.SendCode(ctx, phone, email, domain.LoginCodePurposeLogin); err != nil {
			t.Fatalf("SendCode error = %v", err)
		}
		code := h.sender.sent[0].code

		raw, user, err := h.svc.VerifyCode(ctx, phone, &email, code, nil, DeviceContext{})
		if err != nil {
			t.Fatalf("VerifyCode error = %v", err)
		}
		if raw.Token == "" {
			t.Fatal("VerifyCode returned an empty session token")
		}
		if user.Email == nil || *user.Email != email {
			t.Fatalf("user email = %v, want %s", user.Email, email)
		}
		if len(h.publisher.registered) != 1 {
			t.Fatalf("PublishUserRegistered calls = %d, want 1 for a new user", len(h.publisher.registered))
		}
	})

	t.Run("send code with mismatched email is rejected", func(t *testing.T) {
		t.Parallel()
		h := newAuthServiceHarness()
		phone := mustPhone(t, "+79150000006")
		stored := mustEmail(t, "stored@example.com")
		h.seedUser(t, phone, &stored)

		err := h.svc.SendCode(t.Context(), phone, mustEmail(t, "other@example.com"), domain.LoginCodePurposeLogin)
		if !errors.Is(err, ErrEmailDoesNotMatch) {
			t.Fatalf("SendCode error = %v, want ErrEmailDoesNotMatch", err)
		}
	})
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
	phone := mustPhone(t, "+79150000011")
	email := mustEmail(t, "tz@example.com")
	ctx := t.Context()

	if err := h.svc.SendCode(ctx, phone, email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode error = %v", err)
	}
	code := h.sender.sent[0].code
	timezone := "Asia/Yekaterinburg"

	_, user, err := h.svc.VerifyCode(ctx, phone, &email, code, &timezone, DeviceContext{})
	if err != nil {
		t.Fatalf("VerifyCode error = %v", err)
	}
	if user.Timezone.String() != timezone {
		t.Fatalf("returned user timezone = %q, want %q", user.Timezone.String(), timezone)
	}
	stored := h.users.byPhone[phone.String()]
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
	phone := mustPhone(t, "+79150000012")
	email := mustEmail(t, "badtz@example.com")
	ctx := t.Context()

	if err := h.svc.SendCode(ctx, phone, email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode error = %v", err)
	}
	code := h.sender.sent[0].code
	timezone := "Mars/Olympus"

	_, _, err := h.svc.VerifyCode(ctx, phone, &email, code, &timezone, DeviceContext{})
	if err != nil {
		t.Fatalf("VerifyCode error = %v", err)
	}
	stored := h.users.byPhone[phone.String()]
	if stored.Timezone.String() != "" {
		t.Fatalf("stored user timezone = %q, want the untouched creation default", stored.Timezone.String())
	}
}

func TestAuthenticationService_VerifyCode_RegistrationTimezone_ExistingUntouched(t *testing.T) {
	t.Parallel()
	h := newAuthServiceHarness()
	phone := mustPhone(t, "+79150000013")
	email := mustEmail(t, "moscow@example.com")
	seeded := h.seedUser(t, phone, &email)
	saved, err := domain.NewTimezone("Europe/Moscow")
	if err != nil {
		t.Fatalf("NewTimezone: %v", err)
	}
	seeded.Timezone = saved
	h.users.byPhone[phone.String()] = seeded
	ctx := t.Context()

	if err := h.svc.SendCode(ctx, phone, email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode error = %v", err)
	}
	code := h.sender.sent[0].code
	timezone := "Asia/Yekaterinburg"

	_, user, err := h.svc.VerifyCode(ctx, phone, &email, code, &timezone, DeviceContext{})
	if err != nil {
		t.Fatalf("VerifyCode error = %v", err)
	}
	if user.Timezone.String() != "Europe/Moscow" {
		t.Fatalf("returned user timezone = %q, want saved Europe/Moscow", user.Timezone.String())
	}
	stored := h.users.byPhone[phone.String()]
	if stored.Timezone.String() != "Europe/Moscow" {
		t.Fatalf("stored user timezone = %q, want saved Europe/Moscow", stored.Timezone.String())
	}
	if len(h.publisher.registered) != 0 {
		t.Fatalf("PublishUserRegistered calls = %d, want 0 for an existing user", len(h.publisher.registered))
	}
}

func TestAuthenticationService_SendCode_NewPhoneEmailPrecheck(t *testing.T) {
	t.Parallel()
	t.Run("email taken by another user is rejected before sending", func(t *testing.T) {
		t.Parallel()
		h := newAuthServiceHarness()
		takenEmail := mustEmail(t, "taken@example.com")
		h.seedUser(t, mustPhone(t, "+79150000010"), &takenEmail)

		err := h.svc.SendCode(t.Context(), mustPhone(t, "+79150000011"), takenEmail, domain.LoginCodePurposeLogin)
		if !errors.Is(err, ErrEmailAlreadyTaken) {
			t.Fatalf("SendCode error = %v, want ErrEmailAlreadyTaken", err)
		}
		if len(h.sender.sent) != 0 {
			t.Fatalf("sender calls = %d, want 0", len(h.sender.sent))
		}
		if len(h.codes.codes) != 0 {
			t.Fatalf("codes saved = %d, want 0", len(h.codes.codes))
		}
	})

	t.Run("free email sends the code", func(t *testing.T) {
		t.Parallel()
		h := newAuthServiceHarness()
		phone := mustPhone(t, "+79150000012")
		email := mustEmail(t, "free@example.com")

		if err := h.svc.SendCode(t.Context(), phone, email, domain.LoginCodePurposeLogin); err != nil {
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
	})
}

// TestAuthenticationService_VerifyCode_InvalidCodeRecordsAttemptAndAudit
// verifies the post-refactor failed-login behavior (ADR 0033): an invalid code
// rolls back the success path, then a separate short transaction records the
// attempt-window failure so rate-limiting survives the rollback. The pre-refactor
// early-Commit-on-error is gone.
func TestAuthenticationService_VerifyCode_InvalidCodeRecordsAttemptAndAudit(t *testing.T) {
	t.Parallel()
	h := newAuthServiceHarness()
	phone := mustPhone(t, "+79150000020")
	email := mustEmail(t, "owner@example.com")
	h.seedUser(t, phone, &email)
	ctx := t.Context()

	// Issue a real code, then verify with a wrong one.
	if _, err := h.svc.SendCodeByPhone(ctx, phone); err != nil {
		t.Fatalf("SendCodeByPhone error = %v", err)
	}

	_, _, err := h.svc.VerifyCode(ctx, phone, &email, "000000", nil, DeviceContext{})
	if !errors.Is(err, domain.ErrLoginCodeInvalid) {
		t.Fatalf("VerifyCode error = %v, want ErrLoginCodeInvalid", err)
	}

	// The attempt-window failure was recorded despite the success-path rollback.
	window, ok := h.attempts.windows[phone.String()]
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
	phone := mustPhone(t, "+79150000021")
	email := mustEmail(t, "owner@example.com")
	h.seedUser(t, phone, &email)
	ctx := t.Context()

	if _, err := h.svc.SendCodeByPhone(ctx, phone); err != nil {
		t.Fatalf("SendCodeByPhone error = %v", err)
	}

	var lastErr error
	for range domain.MaxLoginFailures {
		_, _, lastErr = h.svc.VerifyCode(ctx, phone, &email, "000000", nil, DeviceContext{})
	}
	if !errors.Is(lastErr, domain.ErrTooManyAttempts) {
		t.Fatalf("last VerifyCode error = %v, want ErrTooManyAttempts", lastErr)
	}

	// After hitting the threshold, a subsequent send is blocked.
	err := h.svc.SendCode(ctx, phone, email, domain.LoginCodePurposeLogin)
	if !errors.Is(err, ErrUserBlocked) {
		t.Fatalf("SendCode after block = %v, want ErrUserBlocked", err)
	}
}

// TestAuthenticationService_VerifyCode_BlockedPhoneSkipsRecovery proves the
// recovery-branch narrowing (#239): verifying a code on an already-blocked
// phone returns ErrUserBlocked — surfaced authoritatively by
// LoginCodeService.Verify inside the runInTx — without invoking recovery
// (RecordFailureAndAudit). The attempt that caused the block was recorded when
// the block took effect, so the recovery-free `case err != nil` branch is
// correct: the failure counter must not grow beyond MaxLoginFailures.
func TestAuthenticationService_VerifyCode_BlockedPhoneSkipsRecovery(t *testing.T) {
	t.Parallel()
	h := newAuthServiceHarness()
	phone := mustPhone(t, "+79150000022")
	email := mustEmail(t, "owner@example.com")
	h.seedUser(t, phone, &email)
	ctx := t.Context()

	if _, err := h.svc.SendCodeByPhone(ctx, phone); err != nil {
		t.Fatalf("SendCodeByPhone error = %v", err)
	}

	// Drive exactly MaxLoginFailures invalid verifications to reach the block.
	var lastErr error
	for range domain.MaxLoginFailures {
		_, _, lastErr = h.svc.VerifyCode(ctx, phone, &email, "000000", nil, DeviceContext{})
	}
	if !errors.Is(lastErr, domain.ErrTooManyAttempts) {
		t.Fatalf("last VerifyCode error = %v, want ErrTooManyAttempts", lastErr)
	}

	// The phone is now blocked. A further verify must return ErrUserBlocked
	// straight from LoginCodeService.Verify, without recovery.
	_, _, err := h.svc.VerifyCode(ctx, phone, &email, "000000", nil, DeviceContext{})
	if !errors.Is(err, ErrUserBlocked) {
		t.Fatalf("VerifyCode on blocked phone error = %v, want ErrUserBlocked", err)
	}

	// Recovery was not invoked: the failure counter stays at MaxLoginFailures.
	// A spurious increment would prove RecordFailureAndAudit ran despite the
	// block, which the contract forbids.
	window, ok := h.attempts.windows[phone.String()]
	if !ok {
		t.Fatal("attempt window missing for blocked phone")
	}
	if window.Failures != domain.MaxLoginFailures {
		t.Fatalf("window failures after blocked verify = %d, want %d (recovery must not run)",
			window.Failures, domain.MaxLoginFailures)
	}
}

// TestAuthenticationService_SendCode_GetByPhoneError asserts an infrastructure
// error from GetByPhone (not ErrNotFound) is propagated.
func TestAuthenticationService_SendCode_GetByPhoneError(t *testing.T) {
	t.Parallel()
	stores := newFakeStores()
	sender := &fakeCodeSender{}
	dbErr := errors.New("db connection lost")

	factory := NewTxStoreFactory(
		&errorUserRepo{err: dbErr}, stores.codes, stores.attempts, stores.sessions,
		newFakeGrantRepo(), auditapp.Noop{}, &fakeUoW{beginner: stores.beginner},
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
		mustPhone(t, "+79150000030"),
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

// TestAuthenticationService_SendCode_GetByEmailError asserts an infrastructure
// error from GetByEmail for a new phone (not ErrNotFound/nil) is propagated.
func TestAuthenticationService_SendCode_GetByEmailError(t *testing.T) {
	t.Parallel()
	stores := newFakeStores()
	sender := &fakeCodeSender{}
	dbErr := errors.New("db connection lost")

	// The user repo returns ErrNotFound for GetByPhone (new phone) but an
	// infrastructure error for GetByEmail.
	users := &errorOnGetByEmailRepo{fakeUserRepo: newFakeUserRepo(), err: dbErr}
	factory := NewTxStoreFactory(
		users, stores.codes, stores.attempts, stores.sessions,
		newFakeGrantRepo(), auditapp.Noop{}, &fakeUoW{beginner: stores.beginner},
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
		mustPhone(t, "+79150000031"), // New phone, not seeded.
		mustEmail(t, "owner@example.com"),
		domain.LoginCodePurposeLogin,
	)
	if !errors.Is(err, dbErr) {
		t.Fatalf("SendCode error = %v, want wrap of dbErr", err)
	}
	if len(sender.sent) != 0 {
		t.Fatalf("sender calls = %d, want 0 on get-by-email error", len(sender.sent))
	}
}

// TestAuthenticationService_SendCodeByPhone_GetByPhoneError asserts an
// infrastructure error from GetByPhone in SendCodeByPhone is propagated.
func TestAuthenticationService_SendCodeByPhone_GetByPhoneError(t *testing.T) {
	t.Parallel()
	stores := newFakeStores()
	dbErr := errors.New("db connection lost")

	factory := NewTxStoreFactory(
		&errorUserRepo{err: dbErr}, stores.codes, stores.attempts, stores.sessions,
		newFakeGrantRepo(), auditapp.Noop{}, &fakeUoW{beginner: stores.beginner},
	)
	svc := NewAuthenticationService(factory, AuthenticationServiceConfig{
		LoginCodes: NewLoginCodeService(factory, LoginCodeServiceConfig{
			CodeSender: &fakeCodeSender{}, Clock: &fakeClock{now: testNow},
			Hasher: fakeHasher{}, Logger: discardLogger(),
		}),
		Sessions: NewSessionService(factory, SessionServiceConfig{Hasher: fakeHasher{}}),
		Clock:    &fakeClock{now: testNow},
		Logger:   discardLogger(),
	})

	sent, err := svc.SendCodeByPhone(t.Context(), mustPhone(t, "+79150000032"))
	if !errors.Is(err, dbErr) {
		t.Fatalf("SendCodeByPhone error = %v, want wrap of dbErr", err)
	}
	if sent {
		t.Fatal("sent = true, want false on error")
	}
}

// errorUserRepo is a UserRepository whose GetByPhone always fails with err.
type errorUserRepo struct{ err error }

func (r *errorUserRepo) GetByID(context.Context, uuid.UUID) (domain.User, error) {
	return domain.User{}, r.err
}

func (r *errorUserRepo) GetByIDForUpdate(context.Context, uuid.UUID) (domain.User, error) {
	return domain.User{}, r.err
}

func (r *errorUserRepo) GetByPhone(context.Context, domain.Phone) (domain.User, error) {
	return domain.User{}, r.err
}

func (r *errorUserRepo) GetByPhoneForUpdate(context.Context, domain.Phone) (domain.User, error) {
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

func (r *errorUserRepo) UpdatePhone(context.Context, uuid.UUID, domain.Phone) (domain.User, error) {
	return domain.User{}, nil
}

func (r *errorUserRepo) UpdateEmailVerified(context.Context, uuid.UUID, *domain.Email, *time.Time) (domain.User, error) {
	return domain.User{}, nil
}

func (r *errorUserRepo) MarkEmailVerified(context.Context, uuid.UUID, time.Time) (domain.User, error) {
	return domain.User{}, nil
}

func (r *errorUserRepo) SetPhoto(context.Context, uuid.UUID, *string, *string) (domain.User, error) {
	return domain.User{}, nil
}

func (r *errorUserRepo) WithTx(transaction.Tx) (UserRepository, error) { return r, nil }

// errorOnGetByEmailRepo returns ErrNotFound for GetByPhone but a custom error
// for GetByEmail, so the new-phone email precheck path can be exercised.
type errorOnGetByEmailRepo struct {
	*fakeUserRepo
	err error
}

func (r *errorOnGetByEmailRepo) GetByEmail(context.Context, domain.Email) (domain.User, error) {
	return domain.User{}, r.err
}
func (r *errorOnGetByEmailRepo) WithTx(transaction.Tx) (UserRepository, error) { return r, nil }
