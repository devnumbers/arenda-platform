package domain

import (
	"errors"
	"testing"
)

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Accepted prefixes for Russian mobile numbers (+7 / 7 / 8).
		{"+7 mobile", "+79123456789", "+79123456789"},
		{"7 mobile", "79123456789", "+79123456789"},
		{"8 mobile", "89123456789", "+79123456789"},
		{"+7 with surrounding spaces", "  +79123456789  ", "+79123456789"},
		// Leading-zero (9\d{9}) group: only the mobile prefix 9xxxxxxxxx matches.
		{"minimal mobile", "+79000000000", "+79000000000"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizePhone(tc.input)
			if err != nil {
				t.Fatalf("NormalizePhone(%q) error = %v, want nil", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("NormalizePhone(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNormalizePhone_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"too short", "+7912345678"},
		{"landline not mobile (+7 495)", "+74951234567"},
		{"8 landline", "84951234567"},
		{"second digit not 9", "+78001234567"},
		{"letters", "not-a-phone"},
		{"extra digits", "+791234567890"},
		{"missing prefix", "9123456789"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NormalizePhone(tc.input)
			if !errors.Is(err, ErrInvalidPhone) {
				t.Fatalf("NormalizePhone(%q) error = %v, want ErrInvalidPhone", tc.input, err)
			}
		})
	}
}

func TestNewPhone(t *testing.T) {
	t.Run("parses and canonicalizes a mobile number", func(t *testing.T) {
		p, err := NewPhone("89123456789")
		if err != nil {
			t.Fatalf("NewPhone error = %v", err)
		}
		if got := p.String(); got != "+79123456789" {
			t.Fatalf("NewPhone().String() = %q, want %q", got, "+79123456789")
		}
	})

	t.Run("rejects an invalid number", func(t *testing.T) {
		_, err := NewPhone("12345")
		if !errors.Is(err, ErrInvalidPhone) {
			t.Fatalf("NewPhone error = %v, want ErrInvalidPhone", err)
		}
	})
}

func TestValidatePhone(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid mobile +7", "+79123456789", false},
		{"valid mobile 8", "89123456789", false},
		{"landline rejected", "+74951234567", true},
		{"garbage rejected", "nope", true},
		{"empty rejected", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePhone(tc.input)
			if tc.wantErr && !errors.Is(err, ErrInvalidPhone) {
				t.Fatalf("ValidatePhone(%q) error = %v, want ErrInvalidPhone", tc.input, err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("ValidatePhone(%q) error = %v, want nil", tc.input, err)
			}
		})
	}
}
