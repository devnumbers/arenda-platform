package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// loginCodeHarness wires a LoginCodeService to the shared fakes plus a fakeUoW
// so Send can run through runInTx while Verify/RecordFailure receive stores from
// the caller's runInTx (ADR 0033).
type loginCodeHarness struct {
	*fakeStores
	svc    *LoginCodeService
	sender *fakeCodeSender
}

func newLoginCodeHarness() *loginCodeHarness {
	stores := newFakeStores()
	h := &loginCodeHarness{
		fakeStores: stores,
		sender:     &fakeCodeSender{},
	}
	h.svc = NewLoginCodeService(stores.factory(nil), LoginCodeServiceConfig{
		CodeSender: h.sender,
		Clock:      &fakeClock{now: testNow},
		Hasher:     fakeHasher{},
		Logger:     discardLogger(),
	})
	return h
}

// stores builds a *txStores over a fresh fake transaction so Verify/RecordFailure
// tests exercise the same path the orchestrator uses inside runInTx.
func (h *loginCodeHarness) stores(ctx context.Context) (*txStores, error) {
	if _, err := h.beginner.Begin(ctx); err != nil {
		return nil, err
	}
	return &txStores{
		users:    h.users,
		codes:    h.codes,
		attempts: h.attempts,
		sessions: h.sessions,
		audit:    nil,
	}, nil
}

func TestLoginCodeService_Send_IssuesPersistsAndDelivers(t *testing.T) {
	h := newLoginCodeHarness()
	phone := mustPhone(t, "+79150000001")
	email := mustEmail(t, "owner@example.com")

	if err := h.svc.Send(t.Context(), phone, email, domain.LoginCodePurposeLogin, nil); err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if len(h.codes.codes) != 1 {
		t.Fatalf("codes persisted = %d, want 1", len(h.codes.codes))
	}
	if len(h.sender.sent) != 1 {
		t.Fatalf("sender calls = %d, want 1", len(h.sender.sent))
	}
	if h.sender.sent[0].email != email {
		t.Fatalf("sender email = %s, want %s", h.sender.sent[0].email, email)
	}
}

func TestLoginCodeService_Send_ThrottlesWithinMinInterval(t *testing.T) {
	h := newLoginCodeHarness()
	phone := mustPhone(t, "+79150000002")
	email := mustEmail(t, "owner@example.com")

	if err := h.svc.Send(t.Context(), phone, email, domain.LoginCodePurposeLogin, nil); err != nil {
		t.Fatalf("first Send error = %v", err)
	}
	// A second send at the same clock time is throttled.
	err := h.svc.Send(t.Context(), phone, email, domain.LoginCodePurposeLogin, nil)
	if !errors.Is(err, ErrCodeSentTooRecently) {
		t.Fatalf("second Send error = %v, want ErrCodeSentTooRecently", err)
	}
	if len(h.codes.codes) != 1 {
		t.Fatalf("codes persisted = %d, want 1 after throttle", len(h.codes.codes))
	}
}

func TestLoginCodeService_Send_RejectsWhenBlocked(t *testing.T) {
	h := newLoginCodeHarness()
	phone := mustPhone(t, "+79150000003")
	email := mustEmail(t, "owner@example.com")

	// Seed a blocked attempt window — the persisted state after MaxLoginFailures
	// recorded failures (the transition itself is covered by the domain tests).
	window := domain.AttemptWindow{
		Failures:       domain.MaxLoginFailures,
		FirstFailureAt: testNow,
		LastFailureAt:  testNow,
	}
	h.attempts.windows[phone.String()] = window

	err := h.svc.Send(t.Context(), phone, email, domain.LoginCodePurposeLogin, nil)
	if !errors.Is(err, ErrUserBlocked) {
		t.Fatalf("Send error = %v, want ErrUserBlocked", err)
	}
	if len(h.sender.sent) != 0 {
		t.Fatalf("sender calls = %d, want 0 when blocked", len(h.sender.sent))
	}
}

func TestLoginCodeService_Verify_AcceptsValidCode(t *testing.T) {
	h := newLoginCodeHarness()
	phone := mustPhone(t, "+79150000004")
	email := mustEmail(t, "owner@example.com")
	ctx := t.Context()

	if err := h.svc.Send(ctx, phone, email, domain.LoginCodePurposeLogin, nil); err != nil {
		t.Fatalf("Send error = %v", err)
	}
	plaintext := h.sender.sent[0].code

	stores, err := h.stores(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	loginCode, err := h.svc.Verify(ctx, stores, phone, email, domain.LoginCodePurposeLogin, plaintext)
	if err != nil {
		t.Fatalf("Verify error = %v", err)
	}
	if loginCode.Phone != phone {
		t.Fatalf("returned code phone = %s, want %s", loginCode.Phone, phone)
	}
}

func TestLoginCodeService_Verify_RejectsInvalidCode(t *testing.T) {
	h := newLoginCodeHarness()
	phone := mustPhone(t, "+79150000005")
	email := mustEmail(t, "owner@example.com")
	ctx := t.Context()

	if err := h.svc.Send(ctx, phone, email, domain.LoginCodePurposeLogin, nil); err != nil {
		t.Fatalf("Send error = %v", err)
	}

	stores, err := h.stores(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	_, err = h.svc.Verify(ctx, stores, phone, email, domain.LoginCodePurposeLogin, "000000")
	if !errors.Is(err, domain.ErrLoginCodeInvalid) {
		t.Fatalf("Verify error = %v, want ErrLoginCodeInvalid", err)
	}
	// Verify does NOT record the failure (the success-path tx would roll it
	// back); the orchestrator records it via RecordFailure in a separate tx.
	if _, ok := h.attempts.windows[phone.String()]; ok {
		t.Fatal("attempt window must not be created by Verify alone")
	}
}

func TestLoginCodeService_Verify_MissingCode(t *testing.T) {
	h := newLoginCodeHarness()
	phone := mustPhone(t, "+79150000006")
	email := mustEmail(t, "owner@example.com")
	ctx := t.Context()

	stores, err := h.stores(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	_, err = h.svc.Verify(ctx, stores, phone, email, domain.LoginCodePurposeLogin, "123456")
	if !errors.Is(err, domain.ErrLoginCodeInvalid) {
		t.Fatalf("Verify error = %v, want ErrLoginCodeInvalid", err)
	}
}

func TestLoginCodeService_Verify_RejectsWhenBlocked(t *testing.T) {
	h := newLoginCodeHarness()
	phone := mustPhone(t, "+79150000007")
	email := mustEmail(t, "owner@example.com")
	ctx := t.Context()

	if err := h.svc.Send(ctx, phone, email, domain.LoginCodePurposeLogin, nil); err != nil {
		t.Fatalf("Send error = %v", err)
	}
	plaintext := h.sender.sent[0].code

	// Seed a blocked attempt window — the persisted state after MaxLoginFailures
	// recorded failures (the transition itself is covered by the domain tests).
	window := domain.AttemptWindow{
		Failures:       domain.MaxLoginFailures,
		FirstFailureAt: testNow,
		LastFailureAt:  testNow,
	}
	h.attempts.windows[phone.String()] = window

	stores, err := h.stores(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	_, err = h.svc.Verify(ctx, stores, phone, email, domain.LoginCodePurposeLogin, plaintext)
	if !errors.Is(err, ErrUserBlocked) {
		t.Fatalf("Verify error = %v, want ErrUserBlocked", err)
	}
}

func TestLoginCodeService_RecordFailure_IncrementsWindow(t *testing.T) {
	h := newLoginCodeHarness()
	phone := mustPhone(t, "+79150000008")
	ctx := t.Context()

	stores, err := h.stores(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := h.svc.RecordFailure(ctx, stores, phone, uuid.Nil); err != nil {
		t.Fatalf("RecordFailure error = %v", err)
	}
	window, ok := h.attempts.windows[phone.String()]
	if !ok {
		t.Fatal("attempt window not created")
	}
	if window.Failures != 1 {
		t.Fatalf("window failures = %d, want 1", window.Failures)
	}
}

func TestLoginCodeService_RecordFailure_ReachesTooManyAttempts(t *testing.T) {
	h := newLoginCodeHarness()
	phone := mustPhone(t, "+79150000009")
	ctx := t.Context()

	var lastErr error
	for range domain.MaxLoginFailures {
		stores, err := h.stores(ctx)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		lastErr = h.svc.RecordFailure(ctx, stores, phone, uuid.Nil)
	}
	if !errors.Is(lastErr, domain.ErrTooManyAttempts) {
		t.Fatalf("last RecordFailure error = %v, want ErrTooManyAttempts", lastErr)
	}
	window, ok := h.attempts.windows[phone.String()]
	if !ok {
		t.Fatal("attempt window not created")
	}
	if window.Failures != domain.MaxLoginFailures {
		t.Fatalf("window failures = %d, want %d", window.Failures, domain.MaxLoginFailures)
	}
}

func TestUserEmail(t *testing.T) {
	t.Parallel()

	email := mustEmail(t, "owner@example.com")

	t.Run("user with email returns it", func(t *testing.T) {
		t.Parallel()
		u := domain.User{Email: &email}
		got, err := userEmail(u)
		if err != nil {
			t.Fatalf("userEmail error = %v", err)
		}
		if got != email {
			t.Fatalf("userEmail = %s, want %s", got, email)
		}
	})

	t.Run("user without email returns ErrEmailDoesNotMatch", func(t *testing.T) {
		t.Parallel()
		u := domain.User{}
		_, err := userEmail(u)
		if !errors.Is(err, ErrEmailDoesNotMatch) {
			t.Fatalf("userEmail error = %v, want ErrEmailDoesNotMatch", err)
		}
	})
}

func TestGenerateCode(t *testing.T) {
	t.Parallel()

	seen := make(map[string]bool, 1000)
	for range 1000 {
		code, err := generateCode()
		if err != nil {
			t.Fatalf("generateCode error = %v", err)
		}
		if len(code) != 6 {
			t.Fatalf("code length = %d, want 6", len(code))
		}
		// The code must be zero-padded numeric in 000000..999999.
		n, ok := parseCodeDigits(code)
		if !ok {
			t.Fatalf("code %q is not 6 numeric digits", code)
		}
		if n < 0 || n > 999999 {
			t.Fatalf("code value = %d, out of range", n)
		}
		seen[code] = true
	}
	// A crypto-random generator across 1000 samples in a 10^6 space must
	// produce a substantial number of distinct values (collisions exist but
	// should be rare). This guards against a broken constant-output generator.
	if len(seen) < 500 {
		t.Fatalf("distinct codes = %d out of 1000, expected high diversity", len(seen))
	}
}

// parseCodeDigits returns the integer value of a 6-digit string and true, or
// (0, false) when the string is not exactly 6 ASCII digits.
func parseCodeDigits(s string) (int, bool) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

func TestAttemptDelta(t *testing.T) {
	t.Parallel()
	start := testNow

	t.Run("reset path returns zero", func(t *testing.T) {
		t.Parallel()
		// The prev window has a different FirstFailureAt → WasReset is true.
		prev := domain.AttemptWindow{Failures: 3, FirstFailureAt: start}
		next := domain.AttemptWindow{Failures: 1, FirstFailureAt: start.Add(domain.LoginAttemptWindowTTL)}
		if got := attemptDelta(prev, next); got != 0 {
			t.Fatalf("attemptDelta(reset) = %d, want 0", got)
		}
	})

	t.Run("increment path returns positive delta", func(t *testing.T) {
		t.Parallel()
		// Same FirstFailureAt → plain increment, delta = next - prev.
		prev := domain.AttemptWindow{Failures: 3, FirstFailureAt: start}
		next := domain.AttemptWindow{Failures: 4, FirstFailureAt: start}
		if got := attemptDelta(prev, next); got != 1 {
			t.Fatalf("attemptDelta(increment) = %d, want 1", got)
		}
	})
}
