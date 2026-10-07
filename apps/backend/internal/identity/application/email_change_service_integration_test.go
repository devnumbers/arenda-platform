//go:build integration

package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// TestEmailChangeIntegration_HappyPath proves the full flow against real
// PostgreSQL: the code on the current email marks it verified, verify-current
// issues the grant, the new address binds to it and receives the code, and the
// final step applies the change in one commit — new address stored with a
// fresh verified stamp, grant consumed, sessions and the audit entry in place.
func TestEmailChangeIntegration_HappyPath(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	phone := mustPhone(t, "+79160000500")
	currentEmail := mustEmail(t, "current@example.com")
	newEmail := mustEmail(t, "changed@example.com")
	token, user := h.registerAndLogin(t, phone, currentEmail)
	ctx := h.ctx()

	// Step 1: code to the current address.
	if err := h.email.SendCurrentEmailCode(ctx, user.ID); err != nil {
		t.Fatalf("step 1: %v", err)
	}
	currentCode := h.sender.lastCode(t)

	// Step 2: verify the current address, get the addressless grant.
	grantToken, err := h.email.VerifyCurrentEmail(ctx, user.ID, currentCode)
	if err != nil {
		t.Fatalf("step 2: %v", err)
	}
	if grantToken == "" {
		t.Fatal("step 2 returned an empty grant token")
	}

	// Step 3: bind the new address to the grant, receive the code on it.
	if err := h.email.RequestNewEmailCode(ctx, user.ID, grantToken, newEmail); err != nil {
		t.Fatalf("step 3: %v", err)
	}
	newCode := h.sender.lastCode(t)
	if newCode == currentCode {
		t.Fatal("step 3 delivered no new code (same plaintext as step 1)")
	}

	// Final step: change.
	updated, err := h.email.ChangeEmail(ctx, user.ID, newCode, grantToken)
	if err != nil {
		t.Fatalf("final step: %v", err)
	}
	if updated.Email == nil || updated.Email.String() != newEmail.String() {
		t.Fatalf("updated email = %v, want %s", updated.Email, newEmail)
	}
	if updated.EmailVerifiedAt == nil {
		t.Fatal("updated EmailVerifiedAt = nil, want stamped now")
	}

	assertPersistedEmailChange(t, h, user.ID, newEmail)

	// Sessions are untouched: the email is a channel, not the login.
	if _, _, err := h.sessionsvc.Load(ctx, token, h.clock.Now()); err != nil {
		t.Fatalf("session did not survive the email change: %v", err)
	}

	// Audit recorded the success.
	if !h.auditActionExists(t, string(auditdomain.ActionAuthEmailChanged)) {
		t.Fatal("audit_log missing auth.email_changed entry")
	}
}

// assertPersistedEmailChange checks the persisted row carries the new verified
// address and the grant was consumed.
func assertPersistedEmailChange(t *testing.T, h *integrationHarness, userID uuid.UUID, newEmail domain.Email) {
	t.Helper()
	ctx := h.ctx()

	reloaded, err := h.users.GetByID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if reloaded.Email == nil || reloaded.Email.String() != newEmail.String() {
		t.Fatalf("persisted email = %v, want %s", reloaded.Email, newEmail)
	}
	if reloaded.EmailVerifiedAt == nil {
		t.Fatal("persisted EmailVerifiedAt = nil, want stamped")
	}
	if _, err := h.grants.GetByUserIDForUpdate(ctx, userID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("grant lookup error = %v, want ErrNotFound after the change", err)
	}
}

// TestEmailChangeIntegration_StepOneVerifiesCurrentAddress proves the
// verified-semantic seam (#720-6) against real SQL: a user whose stored email
// was never verified can start the flow, and step 1 alone stamps the current
// address verified — even if the flow is abandoned right after.
func TestEmailChangeIntegration_StepOneVerifiesCurrentAddress(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	phone := mustPhone(t, "+79160000501")
	email := mustEmail(t, "unverified@example.com")
	_, user := h.registerAndLogin(t, phone, email)
	ctx := h.ctx()

	// Reset verification to model a user whose address was never confirmed.
	if _, err := h.users.UpdateEmailVerified(ctx, user.ID, &email, nil); err != nil {
		t.Fatalf("clear verified: %v", err)
	}

	if err := h.email.SendCurrentEmailCode(ctx, user.ID); err != nil {
		t.Fatalf("step 1: %v", err)
	}

	reloaded, err := h.users.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if reloaded.EmailVerifiedAt == nil {
		t.Fatal("EmailVerifiedAt = nil after step 1, want stamped")
	}
	if reloaded.Email == nil || reloaded.Email.String() != email.String() {
		t.Fatalf("email = %v, want unchanged %s", reloaded.Email, email)
	}
}

// TestEmailChangeIntegration_GrantLifecycle proves the grant against real
// PostgreSQL: expiry forces a restart, a wrong final code keeps the grant and
// records the phone-window failure, and a consumed grant cannot be replayed.
func TestEmailChangeIntegration_GrantLifecycle(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	phone := mustPhone(t, "+79160000502")
	currentEmail := mustEmail(t, "lifecycle@example.com")
	newEmail := mustEmail(t, "lifecycle-new@example.com")
	_, user := h.registerAndLogin(t, phone, currentEmail)
	ctx := h.ctx()

	if err := h.email.SendCurrentEmailCode(ctx, user.ID); err != nil {
		t.Fatalf("step 1: %v", err)
	}
	grantToken, err := h.email.VerifyCurrentEmail(ctx, user.ID, h.sender.lastCode(t))
	if err != nil {
		t.Fatalf("step 2: %v", err)
	}
	if err := h.email.RequestNewEmailCode(ctx, user.ID, grantToken, newEmail); err != nil {
		t.Fatalf("step 3: %v", err)
	}
	newCode := h.sender.lastCode(t)

	// A wrong final code fails, records the window failure, and keeps the grant.
	if _, err := h.email.ChangeEmail(ctx, user.ID, "000000", grantToken); !errors.Is(err, domain.ErrLoginCodeInvalid) {
		t.Fatalf("wrong code error = %v, want ErrLoginCodeInvalid", err)
	}
	if n := h.countFailedAttempts(t, phone); n != 1 {
		t.Fatalf("failed attempts = %d, want 1", n)
	}
	if _, err := h.email.ChangeEmail(ctx, user.ID, newCode, grantToken); err != nil {
		t.Fatalf("retry with the right code: %v", err)
	}

	// The consumed grant is gone: replaying step 3 fails.
	if _, err := h.email.ChangeEmail(ctx, user.ID, newCode, grantToken); !errors.Is(err, application.ErrEmailChangeGrantInvalid) {
		t.Fatalf("replay error = %v, want ErrEmailChangeGrantInvalid", err)
	}

	// Expiry: a grant that outlived its TTL is refused even with a valid code.
	h.clock.advance(3 * time.Minute)
	if err := h.email.SendCurrentEmailCode(ctx, user.ID); err != nil {
		t.Fatalf("step 1 (second round): %v", err)
	}
	// The user's email is already the new one; restart with a further address.
	thirdEmail := mustEmail(t, "third@example.com")
	secondGrant, err := h.email.VerifyCurrentEmail(ctx, user.ID, h.sender.lastCode(t))
	if err != nil {
		t.Fatalf("step 2 (second round): %v", err)
	}
	if err := h.email.RequestNewEmailCode(ctx, user.ID, secondGrant, thirdEmail); err != nil {
		t.Fatalf("step 3 (second round): %v", err)
	}
	secondCode := h.sender.lastCode(t)
	h.clock.advance(domain.EmailChangeGrantTTL + time.Minute)
	if _, err := h.email.ChangeEmail(ctx, user.ID, secondCode, secondGrant); !errors.Is(err, application.ErrEmailChangeGrantInvalid) {
		t.Fatalf("expired grant error = %v, want ErrEmailChangeGrantInvalid", err)
	}
}

// TestEmailChangeIntegration_ResendNewEmailCode proves the resend (#732)
// against real PostgreSQL: past the send throttle a fresh code is issued for
// the grant's address, the stale code stops verifying, and the fresh
// code with the same grant completes the change.
func TestEmailChangeIntegration_ResendNewEmailCode(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	phone := mustPhone(t, "+79160000505")
	currentEmail := mustEmail(t, "resend@example.com")
	newEmail := mustEmail(t, "resend-new@example.com")
	_, user := h.registerAndLogin(t, phone, currentEmail)
	ctx := h.ctx()

	if err := h.email.SendCurrentEmailCode(ctx, user.ID); err != nil {
		t.Fatalf("step 1: %v", err)
	}
	grantToken, err := h.email.VerifyCurrentEmail(ctx, user.ID, h.sender.lastCode(t))
	if err != nil {
		t.Fatalf("step 2: %v", err)
	}
	if err := h.email.RequestNewEmailCode(ctx, user.ID, grantToken, newEmail); err != nil {
		t.Fatalf("step 3: %v", err)
	}
	staleCode := h.sender.lastCode(t)

	// Inside the throttle window the resend is refused.
	if err := h.email.ResendNewEmailCode(ctx, user.ID, grantToken); !errors.Is(err, application.ErrCodeSentTooRecently) {
		t.Fatalf("throttled resend error = %v, want ErrCodeSentTooRecently", err)
	}

	// The 3-minute advance clears the 1-minute throttle with margin: the
	// code row's created_at is stamped by the database wall clock, which by
	// insert time runs ahead of the fake clock by the real elapsed work
	// (same margin TestEmailChangeIntegration_GrantLifecycle uses).
	h.clock.advance(3 * time.Minute)
	if err := h.email.ResendNewEmailCode(ctx, user.ID, grantToken); err != nil {
		t.Fatalf("resend: %v", err)
	}
	freshCode := h.sender.lastCode(t)
	if freshCode == staleCode {
		t.Fatal("resend delivered no fresh code (same plaintext as step 3)")
	}

	// The stale code no longer verifies; the fresh one completes the change.
	if _, err := h.email.ChangeEmail(ctx, user.ID, staleCode, grantToken); !errors.Is(err, domain.ErrLoginCodeInvalid) {
		t.Fatalf("stale code error = %v, want ErrLoginCodeInvalid", err)
	}
	updated, err := h.email.ChangeEmail(ctx, user.ID, freshCode, grantToken)
	if err != nil {
		t.Fatalf("final step with resent code: %v", err)
	}
	if updated.Email == nil || updated.Email.String() != newEmail.String() {
		t.Fatalf("updated email = %v, want %s", updated.Email, newEmail)
	}
	assertPersistedEmailChange(t, h, user.ID, newEmail)
}

// TestEmailChangeIntegration_TakenBetweenSteps proves the taken-email guard
// re-runs inside the step-3 transaction: an address that became owned between
// steps 2 and 3 is refused and nothing changes.
func TestEmailChangeIntegration_TakenBetweenSteps(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	phone := mustPhone(t, "+79160000503")
	currentEmail := mustEmail(t, "race@example.com")
	newEmail := mustEmail(t, "race-new@example.com")
	_, user := h.registerAndLogin(t, phone, currentEmail)
	ctx := h.ctx()

	if err := h.email.SendCurrentEmailCode(ctx, user.ID); err != nil {
		t.Fatalf("step 1: %v", err)
	}
	grantToken, err := h.email.VerifyCurrentEmail(ctx, user.ID, h.sender.lastCode(t))
	if err != nil {
		t.Fatalf("step 2: %v", err)
	}
	if err := h.email.RequestNewEmailCode(ctx, user.ID, grantToken, newEmail); err != nil {
		t.Fatalf("step 3: %v", err)
	}
	newCode := h.sender.lastCode(t)

	// Another user takes the address between the steps.
	h.seedVerifiedUser(t, mustPhone(t, "+79160000504"), newEmail)

	if _, err := h.email.ChangeEmail(ctx, user.ID, newCode, grantToken); !errors.Is(err, application.ErrEmailAlreadyTaken) {
		t.Fatalf("error = %v, want ErrEmailAlreadyTaken", err)
	}
	reloaded, err := h.users.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if reloaded.Email == nil || reloaded.Email.String() != currentEmail.String() {
		t.Fatalf("email = %v, want unchanged %s", reloaded.Email, currentEmail)
	}
}

// TestEmailChangeIntegration_AddresslessGrant proves the new grant lifecycle
// against real PostgreSQL (#1202): a wrong verify-current code records the
// phone-window failure, the grant is born with a NULL email, refuses the
// resend and the change until an address binds to it, and the binding itself
// is visible in the persisted row.
func TestEmailChangeIntegration_AddresslessGrant(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	phone := mustPhone(t, "+79160000506")
	currentEmail := mustEmail(t, "addressless@example.com")
	newEmail := mustEmail(t, "addressless-new@example.com")
	_, user := h.registerAndLogin(t, phone, currentEmail)
	ctx := h.ctx()

	// A wrong step-2 code records the window failure — the verification
	// behavior moved to verify-current intact.
	if err := h.email.SendCurrentEmailCode(ctx, user.ID); err != nil {
		t.Fatalf("step 1: %v", err)
	}
	if _, err := h.email.VerifyCurrentEmail(ctx, user.ID, "000000"); !errors.Is(err, domain.ErrLoginCodeInvalid) {
		t.Fatalf("wrong code error = %v, want ErrLoginCodeInvalid", err)
	}
	if n := h.countFailedAttempts(t, phone); n != 1 {
		t.Fatalf("failed attempts = %d, want 1", n)
	}

	// The fresh code verifies; the grant is born without an address.
	grantToken, err := h.email.VerifyCurrentEmail(ctx, user.ID, h.sender.lastCode(t))
	if err != nil {
		t.Fatalf("step 2: %v", err)
	}
	stored, err := h.grants.GetByUserIDForUpdate(ctx, user.ID)
	if err != nil {
		t.Fatalf("grant lookup: %v", err)
	}
	if stored.Email != nil {
		t.Fatalf("grant email = %s, want NULL before step 3", stored.Email)
	}

	// The addressless grant cannot anchor the resend or the change; the
	// change refuses before any code check, so the window stays at 1.
	if err := h.email.ResendNewEmailCode(ctx, user.ID, grantToken); !errors.Is(err, application.ErrEmailChangeGrantInvalid) {
		t.Fatalf("addressless resend error = %v, want ErrEmailChangeGrantInvalid", err)
	}
	if _, err := h.email.ChangeEmail(ctx, user.ID, "123456", grantToken); !errors.Is(err, application.ErrEmailChangeGrantInvalid) {
		t.Fatalf("addressless change error = %v, want ErrEmailChangeGrantInvalid", err)
	}
	if n := h.countFailedAttempts(t, phone); n != 1 {
		t.Fatalf("failed attempts = %d, want 1 (no code check ran)", n)
	}

	// Step 3 binds the address; the same token then completes the flow.
	if err := h.email.RequestNewEmailCode(ctx, user.ID, grantToken, newEmail); err != nil {
		t.Fatalf("step 3: %v", err)
	}
	stored, err = h.grants.GetByUserIDForUpdate(ctx, user.ID)
	if err != nil {
		t.Fatalf("grant lookup: %v", err)
	}
	if stored.Email == nil || stored.Email.String() != newEmail.String() {
		t.Fatalf("grant email = %v, want bound %s", stored.Email, newEmail)
	}
	if _, err := h.email.ChangeEmail(ctx, user.ID, h.sender.lastCode(t), grantToken); err != nil {
		t.Fatalf("final step: %v", err)
	}
}
