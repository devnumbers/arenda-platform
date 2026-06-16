package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	sharedphone "github.com/nambers/arenda-planform/apps/backend/internal/shared/phone"
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
	return sharedphone.Validate(phone)
}

// NormalizePhone converts a Russian mobile phone number to the canonical
// +7XXXXXXXXXX format.
func NormalizePhone(phone string) (string, error) {
	return sharedphone.Normalize(phone)
}

// ValidateEmail validates an email address using a simple regex.
func ValidateEmail(email string) error {
	if !emailRegex.MatchString(strings.TrimSpace(email)) {
		return errors.New("invalid email format")
	}
	return nil
}
