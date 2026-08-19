package application

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// phoneChangeHarness wires a PhoneChangeService (with its own LoginCodeService)
// to the shared fakes plus a recordingRecorder, so SendChangeCode and ChangePhone
// can be exercised end-to-end through fake repositories.
type phoneChangeHarness struct {
	*fakeStores
	svc    *PhoneChangeService
	sender *fakeCodeSender
	audit  *recordingRecorder
	clock  *fakeClock
	hasher fakeHasher
}

func newPhoneChangeHarness() *phoneChangeHarness {
	stores := newFakeStores()
	audit := &recordingRecorder{}
	sender := &fakeCodeSender{}
	clock := &fakeClock{now: testNow}
	factory := stores.factory(audit)
	loginCodes := NewLoginCodeService(factory, LoginCodeServiceConfig{
		CodeSender: sender,
		Clock:      clock,
		Hasher:     fakeHasher{},
		Logger:     discardLogger(),
	})
	svc := NewPhoneChangeService(factory, PhoneChangeServiceConfig{
		LoginCodes: loginCodes,
		Clock:      clock,
		Hasher:     fakeHasher{},
		Logger:     discardLogger(),
	})
	return &phoneChangeHarness{
		fakeStores: stores,
		svc:        svc,
		sender:     sender,
		audit:      audit,
		clock:      clock,
		hasher:     fakeHasher{},
	}
}

// seedPhoneChangeUser creates a verified owner in the fake repos so
// SendChangeCode and ChangePhone have a user to act on. It also opens a session
// with a known token hash so ChangePhone's "keep current, delete others" logic
// can be asserted.
func (h *phoneChangeHarness) seedPhoneChangeUser(t *testing.T, phone domain.Phone, email domain.Email, sessionToken string) (user domain.User, tokenHash string) {
	t.Helper()
	var err error
	user, err = domain.NewOwner(phone)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	user.Email = &email
	user.EmailVerifiedAt = &testNow
	h.users.byPhone[phone.String()] = user

	tokenHash = h.hasher.HashToken(sessionToken)
	h.sessions.sessions[tokenHash] = domain.Session{
		UserID:     user.ID,
		TokenHash:  tokenHash,
		ExpiresAt:  testNow.Add(domain.SessionBaseTTL),
		CreatedAt:  testNow,
		LastUsedAt: testNow,
	}
	return user, tokenHash
}

func TestPhoneChangeService_SendChangeCode(t *testing.T) {
	t.Parallel()

	t.Run("happy path sends a phone_change code", func(t *testing.T) {
		t.Parallel()
		h := newPhoneChangeHarness()
		oldPhone := mustPhone(t, "+79160000100")
		newPhone := mustPhone(t, "+79160000200")
		email := mustEmail(t, "owner@example.com")
		user, _ := h.seedPhoneChangeUser(t, oldPhone, email, "current-token")

		err := h.svc.SendChangeCode(t.Context(), user.ID, newPhone)
		if err != nil {
			t.Fatalf("SendChangeCode error = %v", err)
		}
		if len(h.sender.sent) != 1 {
			t.Fatalf("sender calls = %d, want 1", len(h.sender.sent))
		}
		sent := h.sender.sent[0]
		if sent.phone != newPhone {
			t.Fatalf("sent phone = %s, want %s (new phone)", sent.phone, newPhone)
		}
		if sent.email != email {
			t.Fatalf("sent email = %s, want %s (current email)", sent.email, email)
		}
		// The code is persisted with purpose = phone_change.
		if len(h.codes.codes) != 1 {
			t.Fatalf("codes persisted = %d, want 1", len(h.codes.codes))
		}
		for _, c := range h.codes.codes {
			if c.Purpose != domain.LoginCodePurposePhoneChange {
				t.Fatalf("code purpose = %s, want phone_change", c.Purpose)
			}
		}
	})

	t.Run("unchanged phone returns ErrPhoneUnchanged", func(t *testing.T) {
		t.Parallel()
		h := newPhoneChangeHarness()
		oldPhone := mustPhone(t, "+79160000101")
		email := mustEmail(t, "owner@example.com")
		user, _ := h.seedPhoneChangeUser(t, oldPhone, email, "token")

		err := h.svc.SendChangeCode(t.Context(), user.ID, oldPhone)
		if !errors.Is(err, ErrPhoneUnchanged) {
			t.Fatalf("SendChangeCode error = %v, want ErrPhoneUnchanged", err)
		}
		if len(h.sender.sent) != 0 {
			t.Fatalf("sender calls = %d, want 0", len(h.sender.sent))
		}
	})

	t.Run("phone taken by another user returns ErrPhoneAlreadyTaken", func(t *testing.T) {
		t.Parallel()
		h := newPhoneChangeHarness()
		oldPhone := mustPhone(t, "+79160000102")
		takenPhone := mustPhone(t, "+79160000103")
		email := mustEmail(t, "owner@example.com")
		user, _ := h.seedPhoneChangeUser(t, oldPhone, email, "token")
		// Seed a second user that already owns takenPhone.
		other, err := domain.NewOwner(takenPhone)
		if err != nil {
			t.Fatalf("create other user: %v", err)
		}
		h.users.byPhone[takenPhone.String()] = other

		err = h.svc.SendChangeCode(t.Context(), user.ID, takenPhone)
		if !errors.Is(err, ErrPhoneAlreadyTaken) {
			t.Fatalf("SendChangeCode error = %v, want ErrPhoneAlreadyTaken", err)
		}
		if len(h.sender.sent) != 0 {
			t.Fatalf("sender calls = %d, want 0", len(h.sender.sent))
		}
	})

	t.Run("user not found returns wrapped error", func(t *testing.T) {
		t.Parallel()
		h := newPhoneChangeHarness()
		newPhone := mustPhone(t, "+79160000104")

		err := h.svc.SendChangeCode(t.Context(), uuid.Must(uuid.NewV7()), newPhone)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("SendChangeCode error = %v, want wrap of ErrNotFound", err)
		}
		if len(h.sender.sent) != 0 {
			t.Fatalf("sender calls = %d, want 0", len(h.sender.sent))
		}
	})

	t.Run("user without email returns ErrEmailDoesNotMatch", func(t *testing.T) {
		t.Parallel()
		h := newPhoneChangeHarness()
		oldPhone := mustPhone(t, "+79160000105")
		newPhone := mustPhone(t, "+79160000106")
		user, err := domain.NewOwner(oldPhone)
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
		// No email set.
		h.users.byPhone[oldPhone.String()] = user

		err = h.svc.SendChangeCode(t.Context(), user.ID, newPhone)
		if !errors.Is(err, ErrEmailDoesNotMatch) {
			t.Fatalf("SendChangeCode error = %v, want ErrEmailDoesNotMatch", err)
		}
	})
}

func TestPhoneChangeService_ChangePhone(t *testing.T) {
	t.Parallel()

	t.Run("happy path updates phone and cleans up", func(t *testing.T) {
		t.Parallel()
		h := newPhoneChangeHarness()
		oldPhone := mustPhone(t, "+79160000300")
		newPhone := mustPhone(t, "+79160000399")
		email := mustEmail(t, "change@example.com")
		currentToken := "current-session-token"
		user, currentHash := h.seedPhoneChangeUser(t, oldPhone, email, currentToken)
		ctx := t.Context()

		// Seed a second session so we can prove ChangePhone deletes the others.
		otherHash := h.hasher.HashToken("other-session")
		h.sessions.sessions[otherHash] = domain.Session{
			UserID:    user.ID,
			TokenHash: otherHash,
		}

		// Seed attempt windows for both the old and new phone so we can prove
		// ChangePhone resets them (CONTEXT.md: "при смене телефона окна и старого,
		// и нового номеров сбрасываются").
		h.attempts.windows[oldPhone.String()] = domain.AttemptWindow{Failures: 3, FirstFailureAt: testNow}
		h.attempts.windows[newPhone.String()] = domain.AttemptWindow{Failures: 5, FirstFailureAt: testNow}

		// Issue a phone-change code.
		if err := h.svc.SendChangeCode(ctx, user.ID, newPhone); err != nil {
			t.Fatalf("SendChangeCode: %v", err)
		}
		changeCode := h.sender.sent[0].code

		updated, err := h.svc.ChangePhone(ctx, user.ID, newPhone, changeCode, currentToken)
		if err != nil {
			t.Fatalf("ChangePhone error = %v", err)
		}
		if updated.Phone != newPhone {
			t.Fatalf("updated phone = %s, want %s", updated.Phone, newPhone)
		}

		// The user now resolves from newPhone.
		got, err := h.users.GetByPhone(ctx, newPhone)
		if err != nil {
			t.Fatalf("GetByPhone(newPhone) error = %v", err)
		}
		if got.ID != user.ID {
			t.Fatalf("GetByPhone(newPhone) ID = %s, want %s", got.ID, user.ID)
		}
		// Old phone is released.
		if _, err := h.users.GetByPhone(ctx, oldPhone); !errors.Is(err, ErrNotFound) {
			t.Fatalf("GetByPhone(oldPhone) error = %v, want ErrNotFound", err)
		}

		// Current session survives; the other is deleted.
		if _, ok := h.sessions.sessions[currentHash]; !ok {
			t.Fatal("current session was deleted, want retained")
		}
		if _, ok := h.sessions.sessions[otherHash]; ok {
			t.Fatal("other session still present, want deleted")
		}

		// Login codes are cleared.
		if len(h.codes.codes) != 0 {
			t.Fatalf("codes remaining = %d, want 0", len(h.codes.codes))
		}

		// Attempt windows for both old and new phone are reset (CONTEXT.md).
		if _, ok := h.attempts.windows[oldPhone.String()]; ok {
			t.Fatal("old phone attempt window still present, want cleared")
		}
		if _, ok := h.attempts.windows[newPhone.String()]; ok {
			t.Fatal("new phone attempt window still present, want cleared")
		}

		// Audit recorded the phone-changed action.
		if len(h.audit.entries) != 1 {
			t.Fatalf("audit entries = %d, want 1", len(h.audit.entries))
		}
		if h.audit.entries[0].Action != auditdomain.ActionAuthPhoneChanged {
			t.Fatalf("audit action = %s, want %s", h.audit.entries[0].Action, auditdomain.ActionAuthPhoneChanged)
		}
	})

	t.Run("invalid code records attempt and returns error", func(t *testing.T) {
		t.Parallel()
		h := newPhoneChangeHarness()
		oldPhone := mustPhone(t, "+79160000400")
		newPhone := mustPhone(t, "+79160000499")
		email := mustEmail(t, "change@example.com")
		user, _ := h.seedPhoneChangeUser(t, oldPhone, email, "token")
		ctx := t.Context()

		if err := h.svc.SendChangeCode(ctx, user.ID, newPhone); err != nil {
			t.Fatalf("SendChangeCode: %v", err)
		}

		_, err := h.svc.ChangePhone(ctx, user.ID, newPhone, "000000", "token")
		if !errors.Is(err, domain.ErrLoginCodeInvalid) {
			t.Fatalf("ChangePhone error = %v, want ErrLoginCodeInvalid", err)
		}

		// The attempt-window failure was recorded for the new phone.
		window, ok := h.attempts.windows[newPhone.String()]
		if !ok {
			t.Fatal("attempt window not recorded after invalid phone-change code")
		}
		if window.Failures != 1 {
			t.Fatalf("window failures = %d, want 1", window.Failures)
		}
		// Phone not updated.
		got, err := h.users.GetByPhone(ctx, oldPhone)
		if err != nil {
			t.Fatalf("GetByPhone after failed change: %v", err)
		}
		if got.Phone != oldPhone {
			t.Fatalf("phone = %s, want unchanged %s", got.Phone, oldPhone)
		}
	})

	t.Run("unchanged phone returns ErrPhoneUnchanged", func(t *testing.T) {
		t.Parallel()
		h := newPhoneChangeHarness()
		oldPhone := mustPhone(t, "+79160000500")
		email := mustEmail(t, "change@example.com")
		user, _ := h.seedPhoneChangeUser(t, oldPhone, email, "token")

		_, err := h.svc.ChangePhone(t.Context(), user.ID, oldPhone, "123456", "token")
		if !errors.Is(err, ErrPhoneUnchanged) {
			t.Fatalf("ChangePhone error = %v, want ErrPhoneUnchanged", err)
		}
	})

	t.Run("phone taken by another user returns ErrPhoneAlreadyTaken", func(t *testing.T) {
		t.Parallel()
		h := newPhoneChangeHarness()
		oldPhone := mustPhone(t, "+79160000600")
		takenPhone := mustPhone(t, "+79160000601")
		email := mustEmail(t, "change@example.com")
		user, _ := h.seedPhoneChangeUser(t, oldPhone, email, "token")
		other, err := domain.NewOwner(takenPhone)
		if err != nil {
			t.Fatalf("create other user: %v", err)
		}
		h.users.byPhone[takenPhone.String()] = other

		_, err = h.svc.ChangePhone(t.Context(), user.ID, takenPhone, "123456", "token")
		if !errors.Is(err, ErrPhoneAlreadyTaken) {
			t.Fatalf("ChangePhone error = %v, want ErrPhoneAlreadyTaken", err)
		}
	})

	t.Run("user not found returns wrapped error", func(t *testing.T) {
		t.Parallel()
		h := newPhoneChangeHarness()
		newPhone := mustPhone(t, "+79160000700")

		_, err := h.svc.ChangePhone(t.Context(), uuid.Must(uuid.NewV7()), newPhone, "123456", "token")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("ChangePhone error = %v, want wrap of ErrNotFound", err)
		}
	})
}
