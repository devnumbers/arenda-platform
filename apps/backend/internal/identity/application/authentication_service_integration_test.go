//go:build integration

package application_test

import (
	"errors"
	"testing"
	"time"

	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// TestAuthenticationIntegration_LoginHappyPath drives the full login flow against
// real PostgreSQL: SendCode (code persisted + delivered) → VerifyCode (code
// marked used, session created, attempts reset, audit recorded). The session
// token must resolve back to the same user through SessionService.Load.
func TestAuthenticationIntegration_LoginHappyPath(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	phone := mustPhone(t, "+79160000001")
	email := mustEmail(t, "owner@example.com")
	ctx := h.ctx()

	// Seed an existing user so this is a login, not a registration.
	seeded := h.seedVerifiedUser(t, phone, email)

	if err := h.auth.SendCode(ctx, phone, email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode: %v", err)
	}
	if len(h.sender.codes) != 1 {
		t.Fatalf("delivered codes = %d, want 1", len(h.sender.codes))
	}
	code := h.sender.lastCode(t)

	raw, user, err := h.auth.VerifyCode(ctx, phone, &email, code)
	if err != nil {
		t.Fatalf("VerifyCode: %v", err)
	}
	if raw.Token == "" {
		t.Fatal("VerifyCode returned empty token")
	}
	if user.ID != seeded.ID {
		t.Fatalf("user ID = %s, want seeded %s", user.ID, seeded.ID)
	}

	// The session row exists in PostgreSQL and resolves to the same user.
	if n := h.countSessionsForUser(t, user.ID); n != 1 {
		t.Fatalf("sessions for user = %d, want 1", n)
	}

	loadedSession, loadedUser, err := h.sessionsvc.Load(ctx, raw.Token, h.clock.Now())
	if err != nil {
		t.Fatalf("SessionService.Load: %v", err)
	}
	if loadedSession.UserID != user.ID {
		t.Fatalf("loaded session UserID = %s, want %s", loadedSession.UserID, user.ID)
	}
	if loadedUser.ID != user.ID {
		t.Fatalf("loaded user ID = %s, want %s", loadedUser.ID, user.ID)
	}

	// Registration event must NOT fire for an existing user.
	if len(h.publisher.registered) != 0 {
		t.Fatalf("UserRegistered events = %d, want 0 for existing user", len(h.publisher.registered))
	}

	// The login audit entry was written through the real UoW.
	if !h.auditActionExists(t, string(auditdomain.ActionAuthLogin)) {
		t.Fatal("audit_log missing auth.login entry")
	}

	// The code was marked used: a second verify with the same code fails.
	if _, _, err := h.auth.VerifyCode(ctx, phone, &email, code); !errors.Is(err, domain.ErrLoginCodeInvalid) {
		t.Fatalf("re-verify used code error = %v, want ErrLoginCodeInvalid", err)
	}
}

// TestAuthenticationIntegration_RegistrationCreatesNewUser proves a brand-new
// phone+email pair creates a user row, marks the email verified, publishes a
// UserRegistered event, and records the registration audit action.
func TestAuthenticationIntegration_RegistrationCreatesNewUser(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	phone := mustPhone(t, "+79160000002")
	email := mustEmail(t, "new@example.com")
	ctx := h.ctx()

	if err := h.auth.SendCode(ctx, phone, email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode: %v", err)
	}
	code := h.sender.lastCode(t)

	raw, user, err := h.auth.VerifyCode(ctx, phone, &email, code)
	if err != nil {
		t.Fatalf("VerifyCode: %v", err)
	}
	if raw.Token == "" {
		t.Fatal("VerifyCode returned empty token")
	}
	if user.Email == nil || *user.Email != email {
		t.Fatalf("user email = %v, want %s", user.Email, email)
	}
	if user.EmailVerifiedAt == nil {
		t.Fatal("email not marked verified after registration")
	}

	// The user row exists in PostgreSQL.
	got, err := h.users.GetByPhone(ctx, phone)
	if err != nil {
		t.Fatalf("GetByPhone after registration: %v", err)
	}
	if got.ID != user.ID {
		t.Fatalf("persisted user ID = %s, want %s", got.ID, user.ID)
	}

	if len(h.publisher.registered) != 1 {
		t.Fatalf("UserRegistered events = %d, want 1", len(h.publisher.registered))
	}
	ev := h.publisher.registered[0]
	if ev.UserID != user.ID {
		t.Fatalf("event UserID = %s, want %s", ev.UserID, user.ID)
	}

	if !h.auditActionExists(t, string(auditdomain.ActionAuthRegistered)) {
		t.Fatal("audit_log missing auth.registered entry")
	}
}

// TestAuthenticationIntegration_DuplicateEmailRejectedAtSend proves that sending
// a code for a new phone whose email already belongs to another user is rejected
// with ErrEmailAlreadyTaken before any code is persisted or delivered.
func TestAuthenticationIntegration_DuplicateEmailRejectedAtSend(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	takenEmail := mustEmail(t, "taken@example.com")
	takenPhone := mustPhone(t, "+79160000003")
	h.seedVerifiedUser(t, takenPhone, takenEmail)

	newPhone := mustPhone(t, "+79160000004")

	err := h.auth.SendCode(h.ctx(), newPhone, takenEmail, domain.LoginCodePurposeLogin)
	if !errors.Is(err, application.ErrEmailAlreadyTaken) {
		t.Fatalf("SendCode duplicate email error = %v, want ErrEmailAlreadyTaken", err)
	}
	if len(h.sender.codes) != 0 {
		t.Fatalf("delivered codes = %d, want 0 on duplicate-email rejection", len(h.sender.codes))
	}
}

// TestAuthenticationIntegration_LoginFailRecordsAttemptAndAudit proves an invalid
// code rolls back the success-path transaction, then a separate short transaction
// records the attempt-window failure and the failed-login audit so rate-limiting
// survives the rollback (ADR 0033).
func TestAuthenticationIntegration_LoginFailRecordsAttemptAndAudit(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	phone := mustPhone(t, "+79160000005")
	email := mustEmail(t, "owner@example.com")
	h.seedVerifiedUser(t, phone, email)
	ctx := h.ctx()

	if err := h.auth.SendCode(ctx, phone, email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode: %v", err)
	}

	_, _, err := h.auth.VerifyCode(ctx, phone, &email, "000000")
	if !errors.Is(err, domain.ErrLoginCodeInvalid) {
		t.Fatalf("VerifyCode wrong code error = %v, want ErrLoginCodeInvalid", err)
	}

	// No session was created (success path rolled back).
	if n := h.countSessionsForUser(t, h.userIDForPhone(t, phone)); n != 0 {
		t.Fatalf("sessions = %d, want 0 on failed verify", n)
	}

	// The attempt-window failure was recorded despite the rollback.
	if n := h.countFailedAttempts(t, phone); n != 1 {
		t.Fatalf("failed attempts = %d, want 1", n)
	}

	// The failed-login audit was written through the real UoW.
	if !h.auditActionExists(t, string(auditdomain.ActionAuthLoginFailed)) {
		t.Fatal("audit_log missing auth.login_failed entry")
	}
}

// TestAuthenticationIntegration_SlidingSessionRefresh proves the session sliding
// window works end-to-end through PostgreSQL: Load → Refresh → Update extends the
// expiration within the 30-day absolute cap. It reproduces the middleware's
// refresh path (httpsupport/session.go) against real rows.
func TestAuthenticationIntegration_SlidingSessionRefresh(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	phone := mustPhone(t, "+79160000006")
	email := mustEmail(t, "owner@example.com")
	token, _ := h.registerAndLogin(t, phone, email)
	ctx := h.ctx()

	// Snapshot the original expiration.
	session, _, err := h.sessionsvc.Load(ctx, token, h.clock.Now())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	originalExpires := session.ExpiresAt
	createdAt := session.CreatedAt

	// Advance 3 days and refresh: expiration must move forward toward +7 days
	// from now, but not past the 30-day absolute cap.
	h.clock.advance(3 * 24 * time.Hour)
	if !session.Refresh(h.clock.Now()) {
		t.Fatal("Refresh returned false, want true (within sliding window)")
	}
	if err := h.sessionsvc.Update(ctx, session); err != nil {
		t.Fatalf("Update after refresh: %v", err)
	}

	if !session.ExpiresAt.After(originalExpires) {
		t.Fatalf("expires_at = %v, want after original %v", session.ExpiresAt, originalExpires)
	}

	// The persisted row carries the refreshed expiration.
	reloaded, _, err := h.sessionsvc.Load(ctx, token, h.clock.Now())
	if err != nil {
		t.Fatalf("Load after refresh: %v", err)
	}
	if !reloaded.ExpiresAt.Equal(session.ExpiresAt) {
		t.Fatalf("persisted expires_at = %v, want refreshed %v", reloaded.ExpiresAt, session.ExpiresAt)
	}

	// Advance well beyond the 30-day absolute cap and refresh again. Refresh may
	// extend the window up to the cap but never past CreatedAt + SessionMaxTTL.
	h.clock.advance(40 * 24 * time.Hour)
	session.Refresh(h.clock.Now())
	maxExpires := createdAt.Add(domain.SessionMaxTTL)
	if session.ExpiresAt.After(maxExpires) {
		t.Fatalf("expires_at = %v exceeds 30-day cap %v", session.ExpiresAt, maxExpires)
	}
}
