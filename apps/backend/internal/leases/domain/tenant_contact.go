package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// TenantContact mirrors the minimal contact information needed by the leases
// bounded context. The authoritative tenant contacts module owns the full
// lifecycle; this type is used only as a read-model reference here.
type TenantContact struct {
	ID         uuid.UUID
	OwnerID    uuid.UUID
	Name       string
	Surname    *string
	Patronymic *string
	Phone      *string
	Email      *string
	Comment    *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

var (
	emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
)

// ValidatePhone validates a Russian mobile phone number.
// Accepted inputs are +7XXXXXXXXXX, 7XXXXXXXXXX and 8XXXXXXXXXX.
func ValidatePhone(phone string) error {
	_, err := NormalizePhone(phone)
	return err
}

// NormalizePhone converts a Russian mobile phone number to the canonical
// +7XXXXXXXXXX format.
func NormalizePhone(phone string) (string, error) {
	var digits strings.Builder
	for _, r := range phone {
		if unicode.IsDigit(r) {
			digits.WriteRune(r)
		}
	}
	cleaned := digits.String()
	if len(cleaned) != 11 {
		return "", errors.New("phone must contain 11 digits")
	}
	switch cleaned[0] {
	case '8':
		cleaned = "7" + cleaned[1:]
	case '7':
	default:
		return "", errors.New("phone must start with +7, 7 or 8")
	}
	return "+" + cleaned, nil
}

// ValidateEmail validates an email address using a simple regex.
func ValidateEmail(email string) error {
	if !emailRegex.MatchString(email) {
		return errors.New("invalid email format")
	}
	return nil
}
