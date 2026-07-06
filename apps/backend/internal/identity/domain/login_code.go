package domain

import (
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	LoginCodeTTL          = 5 * time.Minute
	LoginAttemptWindowTTL = 30 * time.Minute
	MaxLoginFailures      = 5
)

const (
	LoginCodePurposeLogin       = "login"
	LoginCodePurposePhoneChange = "phone_change"
)

var (
	ErrLoginCodeInvalid = errors.New("login code invalid")
	ErrTooManyAttempts  = errors.New("too many attempts")
)

type LoginCode struct {
	ID        uuid.UUID
	UserID    *uuid.UUID
	Phone     Phone
	Email     Email
	Purpose   string
	CodeHash  string
	ExpiresAt time.Time
	Used      bool
	CreatedAt time.Time
}

func NewLoginCode(phone Phone, email Email, codeHash, purpose string, userID *uuid.UUID, now time.Time) (LoginCode, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return LoginCode{}, fmt.Errorf("generate login code id: %w", err)
	}
	return LoginCode{
		ID:        id,
		UserID:    userID,
		Phone:     phone,
		Email:     email,
		Purpose:   purpose,
		CodeHash:  codeHash,
		ExpiresAt: now.Add(LoginCodeTTL),
		Used:      false,
		CreatedAt: now,
	}, nil
}

func (c *LoginCode) Verify(codeHash string, now time.Time) error {
	if c.Used || now.After(c.ExpiresAt) {
		return ErrLoginCodeInvalid
	}

	expected, err := hex.DecodeString(c.CodeHash)
	if err != nil {
		return ErrLoginCodeInvalid
	}
	actual, err := hex.DecodeString(codeHash)
	if err != nil {
		return ErrLoginCodeInvalid
	}
	if len(expected) != len(actual) || subtle.ConstantTimeCompare(expected, actual) != 1 {
		return ErrLoginCodeInvalid
	}

	c.Used = true
	return nil
}

type AttemptWindow struct {
	Failures       int
	FirstFailureAt time.Time
	LastFailureAt  time.Time
}

func NewAttemptWindow(now time.Time) AttemptWindow {
	return AttemptWindow{FirstFailureAt: now, LastFailureAt: now}
}

func (w *AttemptWindow) RecordFailure(now time.Time) error {
	if w.Failures == 0 || now.Sub(w.FirstFailureAt) > LoginAttemptWindowTTL {
		w.FirstFailureAt = now
		w.Failures = 0
	}
	w.Failures++
	w.LastFailureAt = now
	if w.Failures >= MaxLoginFailures {
		return ErrTooManyAttempts
	}
	return nil
}

func (w *AttemptWindow) Blocked(now time.Time) bool {
	if w.Failures < MaxLoginFailures {
		return false
	}
	return now.Before(w.FirstFailureAt.Add(LoginAttemptWindowTTL))
}
