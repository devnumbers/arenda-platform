package domain

import (
	"errors"
	"testing"
)

func TestNormalizePhoneValid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input    string
		expected string
	}{
		{"+79001234567", "+79001234567"},
		{"79001234567", "+79001234567"},
		{"89001234567", "+79001234567"},
		{"  +79001234567  ", "+79001234567"},
		{"+71110001122", "+71110001122"},
		{"84951234567", "+74951234567"},
		{"+70000000000", "+70000000000"},
	}

	for _, tc := range cases {
		got, err := NormalizePhone(tc.input)
		if err != nil {
			t.Fatalf("NormalizePhone(%q) error: %v", tc.input, err)
		}
		if got != tc.expected {
			t.Fatalf("NormalizePhone(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestNormalizePhoneInvalid(t *testing.T) {
	t.Parallel()
	cases := []string{
		"123",
		"+7999123456",
		"not-a-phone",
		"",
	}

	for _, tc := range cases {
		_, err := NormalizePhone(tc)
		if err == nil {
			t.Fatalf("NormalizePhone(%q) expected error", tc)
		}
		if !errors.Is(err, ErrInvalidPhone) {
			t.Fatalf("NormalizePhone(%q) error = %v, want ErrInvalidPhone", tc, err)
		}
	}
}

func TestValidatePhone(t *testing.T) {
	t.Parallel()
	if err := ValidatePhone("+79001234567"); err != nil {
		t.Fatalf("ValidatePhone valid phone error: %v", err)
	}
	if err := ValidatePhone("+74951234567"); err != nil {
		t.Fatalf("ValidatePhone valid phone error: %v", err)
	}
	if err := ValidatePhone("invalid"); err == nil {
		t.Fatal("ValidatePhone invalid phone expected error")
	}
}

func TestValidateEmail(t *testing.T) {
	t.Parallel()
	cases := []struct {
		email string
		valid bool
	}{
		{"test@example.com", true},
		{"user.name+tag@example.co.uk", true},
		{"invalid", false},
		{"@example.com", false},
		{"", false},
	}

	for _, tc := range cases {
		err := ValidateEmail(tc.email)
		if tc.valid && err != nil {
			t.Fatalf("ValidateEmail(%q) expected valid, got error: %v", tc.email, err)
		}
		if !tc.valid && err == nil {
			t.Fatalf("ValidateEmail(%q) expected invalid", tc.email)
		}
	}
}
