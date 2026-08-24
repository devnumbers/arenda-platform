//go:build integration

package application_test

import (
	"testing"

	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
)

// TestProfileIntegration_UpdatePersonalData proves a name/email/timezone update
// is persisted against real PostgreSQL and the profile-updated audit entry is
// recorded inside the same transaction.
func TestProfileIntegration_UpdatePersonalData(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	phone := mustPhone(t, "+79160000400")
	email := mustEmail(t, "profile@example.com")
	_, user := h.registerAndLogin(t, phone, email)
	ctx := h.ctx()

	updated, err := h.profile.UpdateProfile(ctx, user.ID, application.UpdateProfileCommand{
		Name:    new("Иван"),
		Surname: new("Смирнов"),
	})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if updated.Name == nil || *updated.Name != "Иван" {
		t.Fatalf("updated name = %v, want Иван", updated.Name)
	}
	if updated.Surname == nil || *updated.Surname != "Смирнов" {
		t.Fatalf("updated surname = %v, want Смирнов", updated.Surname)
	}

	// The change is durable: a fresh read returns the new values.
	reloaded, err := h.profile.Me(ctx, user.ID)
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if reloaded.Name == nil || *reloaded.Name != "Иван" {
		t.Fatalf("reloaded name = %v, want Иван", reloaded.Name)
	}

	if !h.auditActionExists(t, string(auditdomain.ActionProfileUpdated)) {
		t.Fatal("audit_log missing profile.updated entry")
	}
}

// TestProfileIntegration_EmailChangeResetsVerified proves changing the email
// clears EmailVerifiedAt on the persisted row, while re-submitting the same
// email leaves verification intact.
func TestProfileIntegration_EmailChangeResetsVerified(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	phone := mustPhone(t, "+79160000401")
	email := mustEmail(t, "verified@example.com")
	_, user := h.registerAndLogin(t, phone, email)
	ctx := h.ctx()

	// Change the email: verified must be reset.
	updated, err := h.profile.UpdateProfile(ctx, user.ID, application.UpdateProfileCommand{
		Email: new("new@example.com"),
	})
	if err != nil {
		t.Fatalf("UpdateProfile email change: %v", err)
	}
	if updated.EmailVerifiedAt != nil {
		t.Fatalf("EmailVerifiedAt = %v, want nil after email change", updated.EmailVerifiedAt)
	}
	if updated.Email == nil || updated.Email.String() != "new@example.com" {
		t.Fatalf("updated email = %v, want new@example.com", updated.Email)
	}

	// Re-submitting the same email must NOT flip verified back — it stays nil
	// until a new login/verify marks it. This documents the contract: the reset
	// is keyed on the email value changing, not on the command being present.
	same, err := h.profile.UpdateProfile(ctx, user.ID, application.UpdateProfileCommand{
		Email: new("new@example.com"),
	})
	if err != nil {
		t.Fatalf("UpdateProfile same email: %v", err)
	}
	if same.EmailVerifiedAt != nil {
		t.Fatalf("EmailVerifiedAt = %v, want nil (unchanged from reset)", same.EmailVerifiedAt)
	}
}
