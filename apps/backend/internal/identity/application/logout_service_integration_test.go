//go:build integration

package application_test

import (
	"testing"

	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// TestLogoutIntegration_ByToken proves Logout deletes exactly the session whose
// token hash matches the raw token, through the real UoW against PostgreSQL.
// Other sessions for the same user survive.
func TestLogoutIntegration_ByToken(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	phone := mustPhone(t, "+79160000300")
	email := mustEmail(t, "logout1@example.com")
	token1, user := h.registerAndLogin(t, phone, email)
	ctx := h.ctx()

	// Issue a second session so we can prove Logout only removes the target.
	if err := h.auth.SendCode(ctx, phone, email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode second session: %v", err)
	}
	token2Raw, _, err := h.auth.VerifyCode(ctx, phone, &email, h.sender.lastCode(t), nil, application.DeviceContext{})
	if err != nil {
		t.Fatalf("VerifyCode second session: %v", err)
	}
	token2 := token2Raw.Token
	if n := h.countSessionsForUser(t, user.ID); n != 2 {
		t.Fatalf("sessions before logout = %d, want 2", n)
	}

	if err := h.logout.Logout(ctx, token1, auditdomain.Actor{ID: user.ID, Role: auditdomain.ActorRoleOwner}); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	// The target session is gone; the other survives.
	if n := h.countSessionsForUser(t, user.ID); n != 1 {
		t.Fatalf("sessions after logout = %d, want 1", n)
	}
	if _, _, err := h.sessionsvc.Load(ctx, token1, h.clock.Now()); err == nil {
		t.Fatal("logged-out token still resolves, want it deleted")
	}
	if _, _, err := h.sessionsvc.Load(ctx, token2, h.clock.Now()); err != nil {
		t.Fatalf("surviving token did not resolve: %v", err)
	}
}

// TestLogoutIntegration_LogoutAll proves LogoutAll deletes every session for the
// user through the real UoW.
func TestLogoutIntegration_LogoutAll(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	phone := mustPhone(t, "+79160000301")
	email := mustEmail(t, "logoutall@example.com")
	token1, user := h.registerAndLogin(t, phone, email)
	ctx := h.ctx()

	if err := h.auth.SendCode(ctx, phone, email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode second session: %v", err)
	}
	if _, _, err := h.auth.VerifyCode(ctx, phone, &email, h.sender.lastCode(t), nil, application.DeviceContext{}); err != nil {
		t.Fatalf("VerifyCode second session: %v", err)
	}
	if n := h.countSessionsForUser(t, user.ID); n != 2 {
		t.Fatalf("sessions before logout-all = %d, want 2", n)
	}

	if err := h.logout.LogoutAll(ctx, user.ID, auditdomain.Actor{ID: user.ID, Role: auditdomain.ActorRoleOwner}); err != nil {
		t.Fatalf("LogoutAll: %v", err)
	}

	if n := h.countSessionsForUser(t, user.ID); n != 0 {
		t.Fatalf("sessions after logout-all = %d, want 0", n)
	}
	if _, _, err := h.sessionsvc.Load(ctx, token1, h.clock.Now()); err == nil {
		t.Fatal("token still resolves after logout-all, want it deleted")
	}
}
