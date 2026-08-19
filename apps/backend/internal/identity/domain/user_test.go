package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
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

func TestNewOwner(t *testing.T) {
	t.Parallel()

	phone := mustPhoneTest(t, "+79150000001")
	user, err := NewOwner(phone)
	if err != nil {
		t.Fatalf("NewOwner error = %v", err)
	}
	if user.ID == (uuid.UUID{}) {
		t.Fatal("NewOwner produced a zero ID")
	}
	if user.Phone != phone {
		t.Fatalf("user.Phone = %s, want %s", user.Phone, phone)
	}
	if user.Role != RoleOwner {
		t.Fatalf("user.Role = %q, want %q", user.Role, RoleOwner)
	}
}

func TestUser_UpdatePersonalData(t *testing.T) {
	t.Parallel()

	verifiedAt := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

	newOwnerWithVerifiedEmail := func(t *testing.T) User {
		t.Helper()
		email, err := NewEmail("owner@example.com")
		if err != nil {
			t.Fatalf("NewEmail: %v", err)
		}
		u, err := NewOwner(mustPhoneTest(t, "+79150000001"))
		if err != nil {
			t.Fatalf("NewOwner: %v", err)
		}
		u.VerifyEmail(email, verifiedAt)
		return u
	}

	t.Run("sets each field individually", func(t *testing.T) {
		t.Parallel()
		u := newOwnerWithVerifiedEmail(t)

		err := u.UpdatePersonalData(
			new("Ivan"),
			new("Petrov"),
			new("Sergeevich"),
			nil, // Email untouched.
			new(testTimezone),
		)
		if err != nil {
			t.Fatalf("UpdatePersonalData error = %v", err)
		}
		if u.Name == nil || *u.Name != "Ivan" {
			t.Fatalf("Name = %v, want Ivan", u.Name)
		}
		if u.Surname == nil || *u.Surname != "Petrov" {
			t.Fatalf("Surname = %v, want Petrov", u.Surname)
		}
		if u.Patronymic == nil || *u.Patronymic != "Sergeevich" {
			t.Fatalf("Patronymic = %v, want Sergeevich", u.Patronymic)
		}
		if u.Timezone.String() != testTimezone {
			t.Fatalf("Timezone = %q, want %s", u.Timezone.String(), testTimezone)
		}
	})

	t.Run("email change resets verification", func(t *testing.T) {
		t.Parallel()
		u := newOwnerWithVerifiedEmail(t)

		err := u.UpdatePersonalData(nil, nil, nil, new("new@example.com"), nil)
		if err != nil {
			t.Fatalf("UpdatePersonalData error = %v", err)
		}
		if u.EmailVerifiedAt != nil {
			t.Fatalf("EmailVerifiedAt = %v, want nil after email change", u.EmailVerifiedAt)
		}
		if u.Email == nil || u.Email.String() != "new@example.com" {
			t.Fatalf("Email = %v, want new@example.com", u.Email)
		}
	})

	t.Run("identical email keeps verification", func(t *testing.T) {
		t.Parallel()
		u := newOwnerWithVerifiedEmail(t)

		err := u.UpdatePersonalData(nil, nil, nil, new("owner@example.com"), nil)
		if err != nil {
			t.Fatalf("UpdatePersonalData error = %v", err)
		}
		if u.EmailVerifiedAt == nil || !u.EmailVerifiedAt.Equal(verifiedAt) {
			t.Fatalf("EmailVerifiedAt = %v, want %v (preserved on identical email)", u.EmailVerifiedAt, verifiedAt)
		}
	})

	t.Run("blank strings become nil via nonEmptyPtr", func(t *testing.T) {
		t.Parallel()
		u := newOwnerWithVerifiedEmail(t)

		err := u.UpdatePersonalData(new("  "), new(""), new("\t"), nil, nil)
		if err != nil {
			t.Fatalf("UpdatePersonalData error = %v", err)
		}
		if u.Name != nil {
			t.Fatalf("Name = %v, want nil for blank string", u.Name)
		}
		if u.Surname != nil {
			t.Fatalf("Surname = %v, want nil for empty string", u.Surname)
		}
		if u.Patronymic != nil {
			t.Fatalf("Patronymic = %v, want nil for whitespace", u.Patronymic)
		}
	})

	t.Run("nil fields do not mutate the aggregate", func(t *testing.T) {
		t.Parallel()
		u := newOwnerWithVerifiedEmail(t)
		emailBefore := u.Email
		tzBefore := u.Timezone

		err := u.UpdatePersonalData(nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("UpdatePersonalData error = %v", err)
		}
		if u.Email != emailBefore {
			t.Fatalf("Email changed from nil-field update")
		}
		if u.Timezone != tzBefore {
			t.Fatalf("Timezone changed from nil-field update")
		}
	})

	t.Run("invalid email returns ErrInvalidEmail", func(t *testing.T) {
		t.Parallel()
		u := newOwnerWithVerifiedEmail(t)

		err := u.UpdatePersonalData(nil, nil, nil, new("not-an-email"), nil)
		if !errors.Is(err, ErrInvalidEmail) {
			t.Fatalf("UpdatePersonalData(invalid email) error = %v, want ErrInvalidEmail", err)
		}
	})

	t.Run("invalid timezone returns ErrInvalidTimezone", func(t *testing.T) {
		t.Parallel()
		u := newOwnerWithVerifiedEmail(t)

		err := u.UpdatePersonalData(nil, nil, nil, nil, new("Mars/Olympus"))
		if !errors.Is(err, ErrInvalidTimezone) {
			t.Fatalf("UpdatePersonalData(invalid tz) error = %v, want ErrInvalidTimezone", err)
		}
	})
}
