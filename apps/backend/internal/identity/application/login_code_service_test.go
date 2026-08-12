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
	svc      *LoginCodeService
	users    *fakeUserRepo
	codes    *fakeCodeRepo
	attempts *fakeAttemptRepo
	sessions *fakeSessionRepo
	sender   *fakeCodeSender
	beginner *fakeBeginner
}

func newLoginCodeHarness() *loginCodeHarness {
	h := &loginCodeHarness{
		users:    newFakeUserRepo(),
		codes:    newFakeCodeRepo(),
		attempts: newFakeAttemptRepo(),
		sessions: newFakeSessionRepo(),
		sender:   &fakeCodeSender{},
		beginner: &fakeBeginner{},
	}
	factory := NewTxStoreFactory(h.users, h.codes, h.attempts, h.sessions, nil, &fakeUoW{beginner: h.beginner})
	h.svc = NewLoginCodeService(factory, LoginCodeServiceConfig{
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

	if err := h.svc.Send(context.Background(), phone, email, domain.LoginCodePurposeLogin, nil); err != nil {
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

	if err := h.svc.Send(context.Background(), phone, email, domain.LoginCodePurposeLogin, nil); err != nil {
		t.Fatalf("first Send error = %v", err)
	}
	// A second send at the same clock time is throttled.
	err := h.svc.Send(context.Background(), phone, email, domain.LoginCodePurposeLogin, nil)
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

	// Seed a blocked attempt window.
	window := domain.NewAttemptWindow(testNow)
	for range domain.MaxLoginFailures {
		_ = window.RecordFailure(testNow)
	}
	h.attempts.windows[phone.String()] = window

	err := h.svc.Send(context.Background(), phone, email, domain.LoginCodePurposeLogin, nil)
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
	ctx := context.Background()

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
	ctx := context.Background()

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
	ctx := context.Background()

	stores, err := h.stores(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	_, err = h.svc.Verify(ctx, stores, phone, email, domain.LoginCodePurposeLogin, "123456")
	if !errors.Is(err, domain.ErrLoginCodeInvalid) {
		t.Fatalf("Verify error = %v, want ErrLoginCodeInvalid", err)
	}
}

func TestLoginCodeService_RecordFailure_IncrementsWindow(t *testing.T) {
	h := newLoginCodeHarness()
	phone := mustPhone(t, "+79150000008")
	ctx := context.Background()

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
	ctx := context.Background()

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
