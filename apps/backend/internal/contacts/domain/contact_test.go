package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/contacts/domain"
)

// contactName is the shared name literal of the fixtures (goconst: one home).
const contactName = "Пётр"

// validContact is the canonical valid fixture: only the required name is set.
func validContact() domain.Contact {
	id, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	return domain.Contact{ID: id, OwnerID: id, FirstName: contactName}
}

// wantInvalid asserts the contract's single invalid-input sentinel.
func wantInvalid(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestValidateContact(t *testing.T) {
	t.Parallel()

	t.Run("minimal valid contact", func(t *testing.T) {
		t.Parallel()
		if err := domain.Validate(validContact()); err != nil {
			t.Fatalf("validate minimal contact: %v", err)
		}
	})

	t.Run("full valid contact", func(t *testing.T) {
		t.Parallel()
		c := validContact()
		c.LastName = "Иванов"
		c.Patronymic = "Петрович"
		c.Role = "сантехник"
		c.Phone = "+79161234567"
		c.Email = "plumber@example.ru"
		c.MessengerName = "Telegram"
		c.MessengerUsername = "@plumber"
		c.Note = "Код домофона 1234"
		if err := domain.Validate(c); err != nil {
			t.Fatalf("validate full contact: %v", err)
		}
	})

	t.Run("empty first name is invalid", func(t *testing.T) {
		t.Parallel()
		c := validContact()
		c.FirstName = ""
		wantInvalid(t, domain.Validate(c))
	})

	t.Run("blank first name is invalid", func(t *testing.T) {
		t.Parallel()
		c := validContact()
		c.FirstName = "   "
		wantInvalid(t, domain.Validate(c))
	})

	t.Run("first name over 255 runes is invalid", func(t *testing.T) {
		t.Parallel()
		c := validContact()
		c.FirstName = strings.Repeat("ы", 256)
		wantInvalid(t, domain.Validate(c))
	})

	t.Run("last name over 255 runes is invalid", func(t *testing.T) {
		t.Parallel()
		c := validContact()
		c.LastName = strings.Repeat("ы", 256)
		wantInvalid(t, domain.Validate(c))
	})

	t.Run("invalid phone is invalid", func(t *testing.T) {
		t.Parallel()
		c := validContact()
		c.Phone = "12345"
		wantInvalid(t, domain.Validate(c))
	})

	t.Run("empty optional fields are valid", func(t *testing.T) {
		t.Parallel()
		c := validContact()
		c.Phone = ""
		c.Email = ""
		if err := domain.Validate(c); err != nil {
			t.Fatalf("validate empty optionals: %v", err)
		}
	})
}

func TestNormalizePhone(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"already normalized", "+79161234567", "+79161234567"},
		{"8-prefix", "89161234567", "+79161234567"},
		{"7 without plus", "79161234567", "+79161234567"},
		{"surrounding spaces", "  +79161234567  ", "+79161234567"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.NormalizePhone(tc.in)
			if err != nil {
				t.Fatalf("normalize %q: %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}

	invalid := []string{"", "12345", "+7916123456", "791612345678", "phone", "+89161234567"}
	for _, in := range invalid {
		t.Run("invalid "+in, func(t *testing.T) {
			t.Parallel()
			if _, err := domain.NormalizePhone(in); !errors.Is(err, domain.ErrInvalidPhone) {
				t.Fatalf("normalize %q: want ErrInvalidPhone, got %v", in, err)
			}
		})
	}
}

func TestValidateEmailFormat(t *testing.T) {
	t.Parallel()

	for _, ok := range []string{"a@b.ru", "plumber+tag@example.co.uk"} {
		if err := domain.ValidateEmailFormat(ok); err != nil {
			t.Errorf("validate email %q: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "no-at-sign", "a@b", "a b@c.ru", "@c.ru"} {
		if err := domain.ValidateEmailFormat(bad); err == nil {
			t.Errorf("validate email %q: want error, got nil", bad)
		}
	}
}

func TestFullName(t *testing.T) {
	t.Parallel()

	t.Run("joins non-empty parts", func(t *testing.T) {
		t.Parallel()
		c := domain.Contact{FirstName: contactName, LastName: "Иванов", Patronymic: "Петрович"}
		if got := c.FullName(); got != contactName+" Иванов Петрович" {
			t.Fatalf("want full name, got %q", got)
		}
	})

	t.Run("skips empty parts", func(t *testing.T) {
		t.Parallel()
		c := domain.Contact{FirstName: contactName}
		if got := c.FullName(); got != contactName {
			t.Fatalf("want first name only, got %q", got)
		}
	})
}
