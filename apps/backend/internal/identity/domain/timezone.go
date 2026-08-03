package domain

import (
	"strings"
	"time"
)

type Timezone struct {
	value string
}

// NewTimezone validates and creates a Timezone from untrusted input.
// It uses time.LoadLocation to verify the value is a valid IANA timezone
// identifier (e.g. "Europe/Moscow", "Asia/Yekaterinburg").
func NewTimezone(raw string) (Timezone, error) {
	tz := strings.TrimSpace(raw)
	if tz == "" {
		return Timezone{}, ErrInvalidTimezone
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return Timezone{}, ErrInvalidTimezone
	}
	return Timezone{value: tz}, nil
}

// TimezoneFrom creates a Timezone from an already-validated identifier.
// It is intended for trusted sources such as the database. An empty value
// returns an error so callers do not silently propagate missing data.
func TimezoneFrom(trusted string) (Timezone, error) {
	if trusted == "" {
		return Timezone{}, ErrInvalidTimezone
	}
	return NewTimezone(trusted)
}

func (t Timezone) String() string {
	return t.value
}
