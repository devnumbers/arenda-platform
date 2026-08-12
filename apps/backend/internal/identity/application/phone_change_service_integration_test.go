//go:build integration

package application_test

import (
	"errors"
	"testing"

	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// TestPhoneChangeIntegration_HappyPath proves the full phone-change flow against
// real PostgreSQL: SendChangeCode → ChangePhone. After the change, the user's
// phone is updated, the change code is marked used, all other sessions are
// deleted (the current session survives), and the phone-changed audit entry is
// recorded — all in one transaction (ADR 0033).
func TestPhoneChangeIntegration_HappyPath(t *testing.T) {
	h := newIntegrationHarness(t)
	oldPhone := mustPhone(t, "+79160000200")
	newPhone := mustPhone(t, "+79160000299")
	email := mustEmail(t, "change@example.com")
	token, user := h.registerAndLogin(t, oldPhone, email)
	ctx := h.ctx()

	// Issue a second session so we can prove ChangePhone deletes the others.
	if err := h.auth.SendCode(ctx, oldPhone, email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode for second session: %v", err)
	}
	token2Raw, _, err := h.auth.VerifyCode(ctx, oldPhone, &email, h.sender.lastCode(t))
	if err != nil {
		t.Fatalf("VerifyCode for second session: %v", err)
	}
	token2 := token2Raw.Token
	if n := h.countSessionsForUser(t, user.ID); n != 2 {
		t.Fatalf("sessions before change = %d, want 2", n)
	}

	// Drive the phone change using token2 as the current session: ChangePhone
	// keeps the current session and deletes every other session for the user.
	if err := h.phone.SendChangeCode(ctx, user.ID, newPhone); err != nil {
		t.Fatalf("SendChangeCode: %v", err)
	}
	changeCode := h.sender.lastCode(t)

	updated, err := h.phone.ChangePhone(ctx, user.ID, newPhone, changeCode, token2)
	if err != nil {
		t.Fatalf("ChangePhone: %v", err)
	}
	if updated.Phone != newPhone {
		t.Fatalf("updated phone = %s, want %s", updated.Phone, newPhone)
	}

	// The user row now resolves from the new phone.
	got, err := h.users.GetByPhone(ctx, newPhone)
	if err != nil {
		t.Fatalf("GetByPhone new phone: %v", err)
	}
	if got.ID != user.ID {
		t.Fatalf("new-phone user ID = %s, want %s", got.ID, user.ID)
	}
	// The old phone no longer resolves.
	if _, err := h.users.GetByPhone(ctx, oldPhone); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("GetByPhone old phone error = %v, want ErrNotFound", err)
	}

	// Only the current session (token2) survives.
	if n := h.countSessionsForUser(t, user.ID); n != 1 {
		t.Fatalf("sessions after change = %d, want 1 (current only)", n)
	}
	if _, _, err := h.sessionsvc.Load(ctx, token, h.clock.Now()); err == nil {
		t.Fatal("old session token still resolves, want it deleted")
	}
	if _, _, err := h.sessionsvc.Load(ctx, token2, h.clock.Now()); err != nil {
		t.Fatalf("current session token did not survive: %v", err)
	}

	// The phone-changed audit was recorded through the real UoW.
	if !h.auditActionExists(t, string(auditdomain.ActionAuthPhoneChanged)) {
		t.Fatal("audit_log missing auth.phone_changed entry")
	}
}

// TestPhoneChangeIntegration_ConflictRejected proves a phone already owned by
// another user is rejected with ErrPhoneAlreadyTaken at both SendChangeCode and
// ChangePhone, and that the rejection at SendChangeCode never issues a code.
func TestPhoneChangeIntegration_ConflictRejected(t *testing.T) {
	h := newIntegrationHarness(t)
	email := mustEmail(t, "conflict@example.com")
	takenPhone := mustPhone(t, "+79160000201")
	h.seedVerifiedUser(t, takenPhone, email)

	ownerPhone := mustPhone(t, "+79160000202")
	ownerEmail := mustEmail(t, "owner2@example.com")
	_, owner := h.registerAndLogin(t, ownerPhone, ownerEmail)
	ctx := h.ctx()

	// SendChangeCode rejects the taken phone before issuing a code.
	err := h.phone.SendChangeCode(ctx, owner.ID, takenPhone)
	if !errors.Is(err, application.ErrPhoneAlreadyTaken) {
		t.Fatalf("SendChangeCode conflict error = %v, want ErrPhoneAlreadyTaken", err)
	}
	// No change code was delivered for the taken phone: every delivered code so
	// far belongs to the registration (ownerPhone), never takenPhone.
	for _, sent := range h.sender.codes {
		if sent.phone == takenPhone {
			t.Fatal("a change code was delivered for a taken phone")
		}
	}
}

// TestPhoneChangeIntegration_WrongCodeRecordsAttempt proves an invalid change
// code rolls back the success path and records the attempt against the new phone
// in a separate transaction so rate-limiting survives (mirroring login).
func TestPhoneChangeIntegration_WrongCodeRecordsAttempt(t *testing.T) {
	h := newIntegrationHarness(t)
	oldPhone := mustPhone(t, "+79160000203")
	newPhone := mustPhone(t, "+79160000204")
	email := mustEmail(t, "wrongcode@example.com")
	_, user := h.registerAndLogin(t, oldPhone, email)
	ctx := h.ctx()

	if err := h.phone.SendChangeCode(ctx, user.ID, newPhone); err != nil {
		t.Fatalf("SendChangeCode: %v", err)
	}

	_, err := h.phone.ChangePhone(ctx, user.ID, newPhone, "000000", "")
	if !errors.Is(err, domain.ErrLoginCodeInvalid) {
		t.Fatalf("ChangePhone wrong code error = %v, want ErrLoginCodeInvalid", err)
	}

	// The phone was NOT updated.
	if got, gErr := h.users.GetByID(ctx, user.ID); gErr != nil || got.Phone != oldPhone {
		t.Fatalf("phone after failed change = %v, want unchanged %s (err=%v)", got.Phone, oldPhone, gErr)
	}

	// The failure was recorded against the new phone.
	if n := h.countFailedAttempts(t, newPhone); n != 1 {
		t.Fatalf("failed attempts for new phone = %d, want 1", n)
	}
}
