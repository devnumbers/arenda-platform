package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"

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
	ErrInvalidPhone = errors.New("invalid Russian phone number")
	emailRegex      = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	phoneRegex      = regexp.MustCompile(`^(?:\+7|7|8)(\d{10})$`)
)

// ValidatePhone validates a Russian phone number. Any 10 digits are accepted
// after the country code: +7XXXXXXXXXX, 7XXXXXXXXXX and 8XXXXXXXXXX.
func ValidatePhone(phone string) error {
	_, err := NormalizePhone(phone)
	return err
}

// NormalizePhone converts a Russian phone number to the canonical
// +7XXXXXXXXXX format. Any 10 digits are accepted after the country code.
func NormalizePhone(phone string) (string, error) {
	digits := phoneRegex.FindStringSubmatch(strings.TrimSpace(phone))
	if digits == nil {
		return "", ErrInvalidPhone
	}
	return "+7" + digits[1], nil
}

// ValidateEmail validates an email address using a simple regex.
func ValidateEmail(email string) error {
	if !emailRegex.MatchString(strings.TrimSpace(email)) {
		return errors.New("invalid email format")
	}
	return nil
}
