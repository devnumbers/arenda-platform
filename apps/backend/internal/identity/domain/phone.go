package domain

import (
	"errors"
	"regexp"
	"strings"
)

var ErrInvalidPhone = errors.New("invalid Russian phone number")

var phoneRegex = regexp.MustCompile(`^(?:\+7|7|8)(9\d{9})$`)

// Phone is a canonical Russian mobile phone number in +7XXXXXXXXXX format.
type Phone string

// NewPhone parses and normalizes a raw phone string into a Phone value.
func NewPhone(raw string) (Phone, error) {
	normalized, err := NormalizePhone(raw)
	if err != nil {
		return "", err
	}
	return Phone(normalized), nil
}

// String returns the canonical phone string.
func (p Phone) String() string {
	return string(p)
}

// NormalizePhone converts a Russian mobile phone number to the canonical
// +7XXXXXXXXXX format.
func NormalizePhone(raw string) (string, error) {
	digits := phoneRegex.FindStringSubmatch(strings.TrimSpace(raw))
	if digits == nil {
		return "", ErrInvalidPhone
	}
	return "+7" + digits[1], nil
}

// ValidatePhone reports whether the raw phone string is a valid Russian mobile number.
func ValidatePhone(raw string) error {
	_, err := NormalizePhone(raw)
	return err
}
