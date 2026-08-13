package domain

import (
	"testing"
	"time"
)

// TestUser_VerifyEmail confirms the aggregate owns its transition into the
// "email verified" state: VerifyEmail sets both the email address and the
// verification timestamp on a fresh owner. This mirrors the unverify-on-change
// logic already living in UpdatePersonalData (commit f77e1f5), closing the
// asymmetry where the aggregate owned one email transition but not the other.
// See issue #241.
func TestUser_VerifyEmail(t *testing.T) {
	email, err := NewEmail("owner@example.com")
	if err != nil {
		t.Fatalf("NewEmail error = %v", err)
	}
	phone := mustPhoneTest(t, "+79150000001")
	user, err := NewOwner(phone)
	if err != nil {
		t.Fatalf("NewOwner error = %v", err)
	}
	at := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

	user.VerifyEmail(email, at)

	if user.Email == nil || *user.Email != email {
		t.Fatalf("user.Email = %v, want %s", user.Email, email)
	}
	if user.EmailVerifiedAt == nil || !user.EmailVerifiedAt.Equal(at) {
		t.Fatalf("user.EmailVerifiedAt = %v, want %v", user.EmailVerifiedAt, at)
	}
}

// mustPhoneTest builds a Phone or fails the test. Kept local to avoid coupling
// the domain test package to application-layer helpers.
func mustPhoneTest(t *testing.T, raw string) Phone {
	t.Helper()
	phone, err := NewPhone(raw)
	if err != nil {
		t.Fatalf("NewPhone(%q) error = %v", raw, err)
	}
	return phone
}
