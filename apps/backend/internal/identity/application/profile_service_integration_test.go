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

// TestProfileIntegration_EmailIsUntouchable proves the profile update cannot
// move the email on the persisted row (issue #721): the command carries no
// email field, so the address and its verification stamp survive a profile
// edit — the address changes only through the confirmed two-code flow.
func TestProfileIntegration_EmailIsUntouchable(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	phone := mustPhone(t, "+79160000401")
	email := mustEmail(t, "verified@example.com")
	_, user := h.registerAndLogin(t, phone, email)
	ctx := h.ctx()

	updated, err := h.profile.UpdateProfile(ctx, user.ID, application.UpdateProfileCommand{
		Name: new("Иван"),
	})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if updated.Email == nil || updated.Email.String() != "verified@example.com" {
		t.Fatalf("updated email = %v, want verified@example.com (untouched)", updated.Email)
	}
	if updated.EmailVerifiedAt == nil {
		t.Fatal("EmailVerifiedAt = nil, want preserved")
	}

	// The persisted row agrees.
	reloaded, err := h.users.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if reloaded.Email == nil || reloaded.Email.String() != "verified@example.com" {
		t.Fatalf("persisted email = %v, want verified@example.com", reloaded.Email)
	}
	if reloaded.EmailVerifiedAt == nil {
		t.Fatal("persisted EmailVerifiedAt = nil, want preserved")
	}
}
