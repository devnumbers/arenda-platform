package domain

import (
	"errors"
	"strings"
	"testing"
)

// testUserEmail is the canonical fixture address shared by the email tests.
const testUserEmail = "user@example.com"

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", testUserEmail, testUserEmail},
		{"uppercased is lowercased", "User.Name@Example.COM", "user.name@example.com"},
		{"surrounding spaces trimmed", "  user@example.com  ", testUserEmail},
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

// TestNormalizeEmail_LengthBoundaries covers the RFC 5321 length guards added
// in issue #220. The local-part limit protects the trusted DB path (EmailFrom)
// from oversized imported data; the total limit rejects abusive addresses.
func TestNormalizeEmail_LengthBoundaries(t *testing.T) {
	t.Run("local-part of exactly 64 chars is accepted", func(t *testing.T) {
		addr := strings.Repeat("a", maxEmailLocalLen) + "@x.co"
		got, err := NormalizeEmail(addr)
		if err != nil {
			t.Fatalf("NormalizeEmail local=64 error = %v", err)
		}
		if got != addr {
			t.Fatalf("NormalizeEmail local=64 = %q, want %q", got, addr)
		}
	})

	t.Run("local-part of 65 chars is rejected", func(t *testing.T) {
		addr := strings.Repeat("a", maxEmailLocalLen+1) + "@x.co"
		_, err := NormalizeEmail(addr)
		if !errors.Is(err, ErrInvalidEmail) {
			t.Fatalf("NormalizeEmail local=65 error = %v, want ErrInvalidEmail", err)
		}
	})

	t.Run("total length of exactly 254 chars is accepted", func(t *testing.T) {
		// Local=64, '@'=1, domain=189 → 254 total. Domain has a dot so the
		// shape regex (TLD present) is satisfied.
		domain := strings.Repeat("b", 187) + ".c"
		addr := strings.Repeat("a", maxEmailLocalLen) + "@" + domain
		if len(addr) != maxEmailTotalLen {
			t.Fatalf("fixture length = %d, want %d", len(addr), maxEmailTotalLen)
		}
		if _, err := NormalizeEmail(addr); err != nil {
			t.Fatalf("NormalizeEmail total=254 error = %v", err)
		}
	})

	t.Run("total length of 255 chars is rejected", func(t *testing.T) {
		domain := strings.Repeat("b", 188) + ".c"
		addr := strings.Repeat("a", maxEmailLocalLen) + "@" + domain
		if len(addr) != maxEmailTotalLen+1 {
			t.Fatalf("fixture length = %d, want %d", len(addr), maxEmailTotalLen+1)
		}
		_, err := NormalizeEmail(addr)
		if !errors.Is(err, ErrInvalidEmail) {
			t.Fatalf("NormalizeEmail total=255 error = %v, want ErrInvalidEmail", err)
		}
	})
}

// TestNormalizeEmail_Unicode exercises non-ASCII addresses. NFKC normalization
// is intentionally not applied (local-part is case-sensitive for many
// providers); a well-formed unicode address passes unchanged.
func TestNormalizeEmail_Unicode(t *testing.T) {
	t.Run("unicode local-part passes unchanged", func(t *testing.T) {
		addr := "üser@пример.рф"
		got, err := NormalizeEmail(addr)
		if err != nil {
			t.Fatalf("NormalizeEmail unicode error = %v", err)
		}
		if got != addr {
			t.Fatalf("NormalizeEmail unicode = %q, want %q", got, addr)
		}
	})

	t.Run("unicode local-part of exactly 64 runes is accepted", func(t *testing.T) {
		// 64 multibyte runes before @ — byte length is 128, but the local-part
		// limit counts runes per RFC 5321, so this must pass (regression guard
		// for the byte-vs-rune fix in issue #220).
		addr := strings.Repeat("ä", maxEmailLocalLen) + "@x.co"
		if _, err := NormalizeEmail(addr); err != nil {
			t.Fatalf("NormalizeEmail 64-rune local-part error = %v", err)
		}
	})

	t.Run("unicode local-part over 64 runes is rejected", func(t *testing.T) {
		// 65 'ä' runes before @ — the rune count exceeds the local-part limit.
		addr := strings.Repeat("ä", maxEmailLocalLen+1) + "@x.co"
		_, err := NormalizeEmail(addr)
		if !errors.Is(err, ErrInvalidEmail) {
			t.Fatalf("NormalizeEmail unicode-overlong error = %v, want ErrInvalidEmail", err)
		}
	})
}
