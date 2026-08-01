package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PropertyContact is an arbitrary contact attached to a single property
// (plumber, management company, concierge, dispatcher). It belongs to exactly
// one property and is cascade-deleted with it.
type PropertyContact struct {
	ID         uuid.UUID
	PropertyID uuid.UUID
	OwnerID    uuid.UUID
	Name       string
	Phone      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

var (
	ErrInvalidPhone = errors.New("invalid Russian phone number")
	phoneRegex      = regexp.MustCompile(`^(?:\+7|7|8)(\d{10})$`)
)

// ValidatePhone validates a Russian phone number. Any 10 digits are accepted
// after the country code: +7XXXXXXXXXX, 7XXXXXXXXXX and 8XXXXXXXXXX.
func ValidatePhone(phone string) error {
	_, err := NormalizePhone(phone)
	return err
}

// NormalizePhone converts a Russian phone number to the canonical
// +7XXXXXXXXXX format.
func NormalizePhone(phone string) (string, error) {
	digits := phoneRegex.FindStringSubmatch(strings.TrimSpace(phone))
	if digits == nil {
		return "", ErrInvalidPhone
	}
	return "+7" + digits[1], nil
}
