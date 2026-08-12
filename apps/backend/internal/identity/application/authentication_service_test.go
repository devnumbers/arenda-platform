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
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

var testNow = time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

// --- transaction fake ---

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

// --- hasher fake ---

// fakeHasher produces deterministic hex output, mirroring
// encryption.hashToken, so domain.LoginCode.Verify can hex-decode it.
type fakeHasher struct{}

func (fakeHasher) HashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// --- repository fakes ---

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

func (r *fakeUserRepo) UpdateEmailVerified(ctx context.Context, id uuid.UUID, email *domain.Email, verifiedAt *time.Time) (domain.User, error) {
	u, err := r.GetByID(ctx, id)
	if err != nil {
		return domain.User{}, err
	}
	u.Email = email
	u.EmailVerifiedAt = verifiedAt
	r.byPhone[u.Phone.String()] = u
	return u, nil
}

func (r *fakeUserRepo) WithTx(transaction.Tx) (UserRepository, error) { return r, nil }

type fakeCodeRepo struct {
	codes map[uuid.UUID]domain.LoginCode
}

func newFakeCodeRepo() *fakeCodeRepo { return &fakeCodeRepo{codes: map[uuid.UUID]domain.LoginCode{}} }

func (r *fakeCodeRepo) Save(_ context.Context, code domain.LoginCode) error {
	r.codes[code.ID] = code
	return nil
}

func (r *fakeCodeRepo) GetLatestByPhoneAndEmail(_ context.Context, phone domain.Phone, email domain.Email, purpose domain.LoginCodePurpose, now time.Time) (domain.LoginCode, error) {
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

func (r *fakeCodeRepo) DeleteExpiredBeforeBatch(context.Context, time.Time, int32) (int64, error) {
	return 0, nil
}

func (r *fakeCodeRepo) DeleteExpiredByPhoneAndEmail(_ context.Context, phone domain.Phone, email domain.Email, purpose domain.LoginCodePurpose, before time.Time) error {
	for id, c := range r.codes {
		if c.Phone == phone && c.Email == email && c.Purpose == purpose && !c.ExpiresAt.After(before) {
			delete(r.codes, id)
		}
	}
	return nil
}

func (r *fakeCodeRepo) DeleteUnusedByPhoneAndEmail(_ context.Context, phone domain.Phone, email domain.Email, purpose domain.LoginCodePurpose) error {
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

func (r *fakeAttemptRepo) DeleteStaleBeforeBatch(context.Context, time.Time, int32) (int64, error) {
	return 0, nil
}

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

func (r *fakeSessionRepo) DeleteByUserID(_ context.Context, userID uuid.UUID) error {
	for hash, s := range r.sessions {
		if s.UserID == userID {
			delete(r.sessions, hash)
		}
	}
	return nil
}

func (r *fakeSessionRepo) DeleteByUserIDExcept(_ context.Context, userID uuid.UUID, tokenHash string) error {
	for hash, s := range r.sessions {
		if s.UserID == userID && hash != tokenHash {
			delete(r.sessions, hash)
		}
	}
	return nil
}

func (r *fakeSessionRepo) DeleteExpiredBeforeBatch(context.Context, time.Time, int32) (int64, error) {
	return 0, nil
}

func (r *fakeSessionRepo) WithTx(transaction.Tx) (SessionRepository, error) { return r, nil }

// --- sender and publisher fakes ---

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

// --- harness ---

type authServiceHarness struct {
	svc       *AuthenticationService
	users     *fakeUserRepo
	codes     *fakeCodeRepo
	attempts  *fakeAttemptRepo
	sessions  *fakeSessionRepo
	sender    *fakeCodeSender
	publisher *fakePublisher
	beginner  *fakeBeginner
}

func newAuthServiceHarness() *authServiceHarness {
	h := &authServiceHarness{
		users:     newFakeUserRepo(),
		codes:     newFakeCodeRepo(),
		attempts:  newFakeAttemptRepo(),
		sessions:  newFakeSessionRepo(),
		sender:    &fakeCodeSender{},
		publisher: &fakePublisher{},
		beginner:  &fakeBeginner{},
	}
	uow := &fakeUoW{beginner: h.beginner}
	factory := NewTxStoreFactory(h.users, h.codes, h.attempts, h.sessions, nil, uow)
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
			h := newAuthServiceHarness()
			tt.seed(t, h)

			sent, err := h.svc.SendCodeByPhone(context.Background(), phone)
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
	h := newAuthServiceHarness()
	phone := mustPhone(t, "+79150000001")
	email := mustEmail(t, "owner@example.com")
	user := h.seedUser(t, phone, &email)
	ctx := context.Background()

	sent, err := h.svc.SendCodeByPhone(ctx, phone)
	if err != nil || !sent {
		t.Fatalf("SendCodeByPhone = %v, %v", sent, err)
	}
	code := h.sender.sent[0].code

	raw, got, err := h.svc.VerifyCode(ctx, phone, nil, code)
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
	h := newAuthServiceHarness()
	phone := mustPhone(t, "+79150000009")

	_, _, err := h.svc.VerifyCode(context.Background(), phone, nil, "123456")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("VerifyCode error = %v, want ErrNotFound", err)
	}
}

func TestAuthenticationService_VerifyCode_UserWithoutEmailWithoutEmail(t *testing.T) {
	h := newAuthServiceHarness()
	phone := mustPhone(t, "+79150000008")
	h.seedUser(t, phone, nil)

	_, _, err := h.svc.VerifyCode(context.Background(), phone, nil, "123456")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("VerifyCode error = %v, want ErrNotFound", err)
	}
}

func TestAuthenticationService_ExplicitEmailFlow(t *testing.T) {
	t.Run("new user registers with explicit email", func(t *testing.T) {
		h := newAuthServiceHarness()
		phone := mustPhone(t, "+79150000005")
		email := mustEmail(t, "new@example.com")
		ctx := context.Background()

		if err := h.svc.SendCode(ctx, phone, email, domain.LoginCodePurposeLogin); err != nil {
			t.Fatalf("SendCode error = %v", err)
		}
		code := h.sender.sent[0].code

		raw, user, err := h.svc.VerifyCode(ctx, phone, &email, code)
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
		h := newAuthServiceHarness()
		phone := mustPhone(t, "+79150000006")
		stored := mustEmail(t, "stored@example.com")
		h.seedUser(t, phone, &stored)

		err := h.svc.SendCode(context.Background(), phone, mustEmail(t, "other@example.com"), domain.LoginCodePurposeLogin)
		if !errors.Is(err, ErrEmailDoesNotMatch) {
			t.Fatalf("SendCode error = %v, want ErrEmailDoesNotMatch", err)
		}
	})
}

func TestAuthenticationService_SendCode_NewPhoneEmailPrecheck(t *testing.T) {
	t.Run("email taken by another user is rejected before sending", func(t *testing.T) {
		h := newAuthServiceHarness()
		takenEmail := mustEmail(t, "taken@example.com")
		h.seedUser(t, mustPhone(t, "+79150000010"), &takenEmail)

		err := h.svc.SendCode(context.Background(), mustPhone(t, "+79150000011"), takenEmail, domain.LoginCodePurposeLogin)
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
		h := newAuthServiceHarness()
		phone := mustPhone(t, "+79150000012")
		email := mustEmail(t, "free@example.com")

		if err := h.svc.SendCode(context.Background(), phone, email, domain.LoginCodePurposeLogin); err != nil {
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
	h := newAuthServiceHarness()
	phone := mustPhone(t, "+79150000020")
	email := mustEmail(t, "owner@example.com")
	h.seedUser(t, phone, &email)
	ctx := context.Background()

	// Issue a real code, then verify with a wrong one.
	if _, err := h.svc.SendCodeByPhone(ctx, phone); err != nil {
		t.Fatalf("SendCodeByPhone error = %v", err)
	}

	_, _, err := h.svc.VerifyCode(ctx, phone, &email, "000000")
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
	h := newAuthServiceHarness()
	phone := mustPhone(t, "+79150000021")
	email := mustEmail(t, "owner@example.com")
	h.seedUser(t, phone, &email)
	ctx := context.Background()

	if _, err := h.svc.SendCodeByPhone(ctx, phone); err != nil {
		t.Fatalf("SendCodeByPhone error = %v", err)
	}

	var lastErr error
	for range domain.MaxLoginFailures {
		_, _, lastErr = h.svc.VerifyCode(ctx, phone, &email, "000000")
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
