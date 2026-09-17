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
	t.Parallel()
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

// verifiedOwnerEmail is the seeded address of the baseline aggregate and the
// identical-resubmit input of the email-verification cases.
const verifiedOwnerEmail = "owner@example.com"

// newOwnerWithVerifiedEmail builds an owner whose email was verified at the
// given timestamp — the baseline aggregate for UpdatePersonalData cases.
func newOwnerWithVerifiedEmail(t *testing.T, at time.Time) User {
	t.Helper()
	email, err := NewEmail(verifiedOwnerEmail)
	if err != nil {
		t.Fatalf("NewEmail: %v", err)
	}
	u, err := NewOwner(mustPhoneTest(t, "+79150000001"))
	if err != nil {
		t.Fatalf("NewOwner: %v", err)
	}
	u.VerifyEmail(email, at)
	return u
}

// assertStringPtr checks an optional string field: want == "" expects a nil
// pointer (blank input collapses to nil via nonEmptyPtr), otherwise a pointer
// to exactly want.
func assertStringPtr(t *testing.T, label string, got *string, want string) {
	t.Helper()
	if want == "" {
		if got != nil {
			t.Fatalf("%s = %v, want nil", label, got)
		}
		return
	}
	if got == nil || *got != want {
		t.Fatalf("%s = %v, want %s", label, got, want)
	}
}

// assertEmailVerification checks the stored address matches wantEmail and the
// verified-at timestamp is present exactly when wantVerified is set. Since #721
// a profile update never touches either, so the helper also pins that boundary.
func assertEmailVerification(t *testing.T, u User, wantEmail string, wantVerified bool, verifiedAt time.Time) {
	t.Helper()
	if u.Email == nil || u.Email.String() != wantEmail {
		t.Fatalf("Email = %v, want %s", u.Email, wantEmail)
	}
	if wantVerified {
		if u.EmailVerifiedAt == nil || !u.EmailVerifiedAt.Equal(verifiedAt) {
			t.Fatalf("EmailVerifiedAt = %v, want %v (preserved on identical email)", u.EmailVerifiedAt, verifiedAt)
		}
		return
	}
	if u.EmailVerifiedAt != nil {
		t.Fatalf("EmailVerifiedAt = %v, want nil after email change", u.EmailVerifiedAt)
	}
}

func TestUser_UpdatePersonalData(t *testing.T) {
	t.Parallel()

	verifiedAt := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

	fieldCases := []struct {
		name string
		// Update args; nil leaves the field untouched.
		nameArg, surnameArg, patronymicArg, timezoneArg *string
		// Wants; "" on a name field expects a nil pointer, "" on the timezone
		// leaves it unasserted.
		wantName, wantSurname, wantPatronymic, wantTimezone string
	}{
		{
			name:           "sets each field individually",
			nameArg:        new("Ivan"),
			surnameArg:     new("Petrov"),
			patronymicArg:  new("Sergeevich"),
			timezoneArg:    new(testTimezone),
			wantName:       "Ivan",
			wantSurname:    "Petrov",
			wantPatronymic: "Sergeevich",
			wantTimezone:   testTimezone,
		},
		{
			name:          "blank strings become nil via nonEmptyPtr",
			nameArg:       new("  "),
			surnameArg:    new(""),
			patronymicArg: new("\t"),
		},
	}
	for _, tc := range fieldCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			u := newOwnerWithVerifiedEmail(t, verifiedAt)

			err := u.UpdatePersonalData(tc.nameArg, tc.surnameArg, tc.patronymicArg, tc.timezoneArg)
			if err != nil {
				t.Fatalf("UpdatePersonalData error = %v", err)
			}
			assertStringPtr(t, "Name", u.Name, tc.wantName)
			assertStringPtr(t, "Surname", u.Surname, tc.wantSurname)
			assertStringPtr(t, "Patronymic", u.Patronymic, tc.wantPatronymic)
			if tc.wantTimezone != "" && u.Timezone.String() != tc.wantTimezone {
				t.Fatalf("Timezone = %q, want %s", u.Timezone.String(), tc.wantTimezone)
			}
		})
	}

	// The email is deliberately not part of UpdatePersonalData (#721): the
	// address changes only through the confirmed two-code flow, so the old
	// "free edit resets verification" rule is gone — a profile update leaves
	// the address and its verification stamp untouched.
	t.Run("profile update leaves the email and its verification intact", func(t *testing.T) {
		t.Parallel()
		u := newOwnerWithVerifiedEmail(t, verifiedAt)

		err := u.UpdatePersonalData(new("Ivan"), nil, nil, new(testTimezone))
		if err != nil {
			t.Fatalf("UpdatePersonalData error = %v", err)
		}
		assertEmailVerification(t, u, verifiedOwnerEmail, true, verifiedAt)
	})

	t.Run("nil fields do not mutate the aggregate", func(t *testing.T) {
		t.Parallel()
		u := newOwnerWithVerifiedEmail(t, verifiedAt)
		emailBefore := u.Email
		tzBefore := u.Timezone

		err := u.UpdatePersonalData(nil, nil, nil, nil)
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

	errorCases := []struct {
		name        string
		timezoneArg *string
		wantErr     error
	}{
		{"invalid timezone returns ErrInvalidTimezone", new("Mars/Olympus"), ErrInvalidTimezone},
	}
	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			u := newOwnerWithVerifiedEmail(t, verifiedAt)

			err := u.UpdatePersonalData(nil, nil, nil, tc.timezoneArg)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("UpdatePersonalData error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}
