package domain

import (
	"errors"
	"testing"
)

func TestNewPhoneValid(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"+79001234567", "+79001234567"},
		{"79001234567", "+79001234567"},
		{"89001234567", "+79001234567"},
		{"  +79001234567  ", "+79001234567"},
	}

	for _, tc := range cases {
		phone, err := NewPhone(tc.input)
		if err != nil {
			t.Fatalf("NewPhone(%q) error: %v", tc.input, err)
		}
		if phone.String() != tc.expected {
			t.Fatalf("NewPhone(%q) = %q, want %q", tc.input, phone.String(), tc.expected)
		}
	}
}

func TestNewPhoneInvalid(t *testing.T) {
	cases := []string{
		"123",
		"+7999123456",
		"not-a-phone",
		"",
	}

	for _, tc := range cases {
		_, err := NewPhone(tc)
		if err == nil {
			t.Fatalf("NewPhone(%q) expected error", tc)
		}
		if !errors.Is(err, ErrInvalidPhone) {
			t.Fatalf("NewPhone(%q) error = %v, want ErrInvalidPhone", tc, err)
		}
	}
}
