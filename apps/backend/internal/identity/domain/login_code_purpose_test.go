package domain

import (
	"errors"
	"testing"
)

func TestLoginCodePurpose_KnownValues(t *testing.T) {
	t.Parallel()
	known := map[string]LoginCodePurpose{
		"login":        LoginCodePurposeLogin,
		"phone_change": LoginCodePurposePhoneChange,
		"email_change": LoginCodePurposeEmailChange,
	}
	for raw, want := range known {
		got, err := NewLoginCodePurpose(raw)
		if err != nil {
			t.Fatalf("NewLoginCodePurpose(%q) error = %v, want nil", raw, err)
		}
		if got != want {
			t.Fatalf("NewLoginCodePurpose(%q) = %q, want %q", raw, got, want)
		}
		if s := got.String(); s != raw {
			t.Fatalf("%q.String() = %q, want %q", raw, s, raw)
		}
	}
}

func TestLoginCodePurpose_RejectedValues(t *testing.T) {
	t.Parallel()
	rejected := []string{"", "Login", "PHONE_CHANGE", "signup", "verify"}
	for _, raw := range rejected {
		_, err := NewLoginCodePurpose(raw)
		if !errors.Is(err, ErrInvalidLoginCodePurpose) {
			t.Fatalf("NewLoginCodePurpose(%q) error = %v, want ErrInvalidLoginCodePurpose", raw, err)
		}
	}
}
