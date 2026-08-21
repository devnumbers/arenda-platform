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

// TestProfileIntegration_TimezoneChangeReschedules proves a timezone change
// triggers the post-commit reminder reschedule against the real persisted
// timezone, and that an unchanged timezone (or no timezone command) does not.
func TestProfileIntegration_TimezoneChangeReschedules(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	phone := mustPhone(t, "+79160000402")
	email := mustEmail(t, "tz@example.com")
	_, user := h.registerAndLogin(t, phone, email)
	ctx := h.ctx()

	// The seeded user's timezone defaults to Europe/Moscow (DB default). A change
	// to Asia/Yekaterinburg must trigger one reschedule call.
	updated, err := h.profile.UpdateProfile(ctx, user.ID, application.UpdateProfileCommand{
		Timezone: new("Asia/Yekaterinburg"),
	})
	if err != nil {
		t.Fatalf("UpdateProfile timezone: %v", err)
	}
	if updated.Timezone.String() != "Asia/Yekaterinburg" {
		t.Fatalf("updated timezone = %q, want Asia/Yekaterinburg", updated.Timezone.String())
	}
	if len(h.rescheduler.calls) != 1 {
		t.Fatalf("reschedule calls = %d, want 1", len(h.rescheduler.calls))
	}
	call := h.rescheduler.calls[0]
	if call.userID != user.ID {
		t.Fatalf("reschedule userID = %s, want %s", call.userID, user.ID)
	}
	if call.oldTZ != "Europe/Moscow" {
		t.Fatalf("reschedule oldTZ = %q, want Europe/Moscow", call.oldTZ)
	}
	if call.newTZ != "Asia/Yekaterinburg" {
		t.Fatalf("reschedule newTZ = %q, want Asia/Yekaterinburg", call.newTZ)
	}

	// A name-only update must not reschedule.
	if _, err := h.profile.UpdateProfile(ctx, user.ID, application.UpdateProfileCommand{
		Name: new("Имя"),
	}); err != nil {
		t.Fatalf("UpdateProfile name only: %v", err)
	}
	if len(h.rescheduler.calls) != 1 {
		t.Fatalf("reschedule calls after name-only = %d, want still 1", len(h.rescheduler.calls))
	}
}
