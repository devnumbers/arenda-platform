package domain

import (
	"errors"
	"regexp"
	"strings"
)

var ErrInvalidPhone = errors.New("invalid Russian phone number")

var phoneRegex = regexp.MustCompile(`^(?:\+7|7|8)(9\d{9})$`)

type Phone string

func NewPhone(raw string) (Phone, error) {
	digits := phoneRegex.FindStringSubmatch(strings.TrimSpace(raw))
	if digits == nil {
		return "", ErrInvalidPhone
	}
	return Phone("+7" + digits[1]), nil
}

func (p Phone) String() string {
	return string(p)
}
