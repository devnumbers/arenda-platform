package domain

import (
	"errors"
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "user@example.com", "user@example.com"},
		{"uppercased is lowercased", "User.Name@Example.COM", "user.name@example.com"},
		{"surrounding spaces trimmed", "  user@example.com  ", "user@example.com"},
		{"plus addressing", "user.name+tag@example.co.uk", "user.name+tag@example.co.uk"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeEmail(tc.input)
			if err != nil {
				t.Fatalf("NormalizeEmail(%q) error = %v, want nil", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeEmail(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNormalizeEmail_Invalid(t *testing.T) {
	tests := []string{
		"",
		"   ",
		"plainaddress",
		"@example.com",
		"user@",
		"user@example",
		"user@.com",
		"user name@example.com",
	}

	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			_, err := NormalizeEmail(tc)
			if !errors.Is(err, ErrInvalidEmail) {
				t.Fatalf("NormalizeEmail(%q) error = %v, want ErrInvalidEmail", tc, err)
			}
		})
	}
}

func TestNewEmail(t *testing.T) {
	t.Run("parses and lowercases", func(t *testing.T) {
		e, err := NewEmail("Owner@Example.com")
		if err != nil {
			t.Fatalf("NewEmail error = %v", err)
		}
		if got := e.String(); got != "owner@example.com" {
			t.Fatalf("NewEmail().String() = %q, want %q", got, "owner@example.com")
		}
	})

	t.Run("rejects invalid", func(t *testing.T) {
		_, err := NewEmail("not-an-email")
		if !errors.Is(err, ErrInvalidEmail) {
			t.Fatalf("NewEmail error = %v, want ErrInvalidEmail", err)
		}
	})
}

func TestEmailFrom(t *testing.T) {
	t.Run("accepts a valid normalized address", func(t *testing.T) {
		e, err := EmailFrom("owner@example.com")
		if err != nil {
			t.Fatalf("EmailFrom error = %v", err)
		}
		if got := e.String(); got != "owner@example.com" {
			t.Fatalf("EmailFrom().String() = %q, want %q", got, "owner@example.com")
		}
	})

	t.Run("rejects an empty value", func(t *testing.T) {
		_, err := EmailFrom("")
		if !errors.Is(err, ErrInvalidEmail) {
			t.Fatalf("EmailFrom(\"\") error = %v, want ErrInvalidEmail", err)
		}
	})

	t.Run("rejects an invalid address", func(t *testing.T) {
		_, err := EmailFrom("not-an-email")
		if !errors.Is(err, ErrInvalidEmail) {
			t.Fatalf("EmailFrom(invalid) error = %v, want ErrInvalidEmail", err)
		}
	})
}
