package domain

import (
	"errors"
	"regexp"
	"strings"
)

var emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

var ErrInvalidEmail = errors.New("invalid email")

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
