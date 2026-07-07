package domain

import (
	"regexp"
	"strings"
)

var emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

type Email struct {
	value string
}

func NewEmail(raw string) (Email, error) {
	normalized, err := NormalizeEmail(raw)
	if err != nil {
		return Email{}, err
	}
	return Email{value: normalized}, nil
}

// EmailFrom creates an Email from an already-normalized address.
// It is intended for trusted sources such as the database. An empty value or
// an invalid address returns an error so callers do not silently propagate
// corrupt or missing data.
func EmailFrom(normalized string) (Email, error) {
	if normalized == "" {
		return Email{}, ErrInvalidEmail
	}
	return NewEmail(normalized)
}

func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || !isValidEmail(email) {
		return "", ErrInvalidEmail
	}
	return email, nil
}

func (e Email) String() string {
	return e.value
}

func isValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}
