package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	SMSCodeTTL          = 5 * time.Minute
	SMSAttemptWindowTTL = 30 * time.Minute
	MaxSMSFailures      = 5
)

const (
	SMSCodePurposeLogin       = "login"
	SMSCodePurposePhoneChange = "phone_change"
)

var (
	ErrSMSCodeInvalid  = errors.New("sms code invalid")
	ErrTooManyAttempts = errors.New("too many attempts")
)

type SMSCode struct {
	ID        uuid.UUID
	UserID    *uuid.UUID
	Phone     Phone
	Purpose   string
	CodeHash  string
	ExpiresAt time.Time
	Used      bool
	CreatedAt time.Time
}

func NewSMSCode(phone Phone, code, purpose string, userID *uuid.UUID, now time.Time) (SMSCode, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return SMSCode{}, fmt.Errorf("generate sms code id: %w", err)
	}
	return SMSCode{
		ID:        id,
		UserID:    userID,
		Phone:     phone,
		Purpose:   purpose,
		CodeHash:  hashCode(phone.String(), code),
		ExpiresAt: now.Add(SMSCodeTTL),
		Used:      false,
		CreatedAt: now,
	}, nil
}

func (c *SMSCode) Verify(code string, now time.Time) error {
	if c.Used || now.After(c.ExpiresAt) || hashCode(c.Phone.String(), code) != c.CodeHash {
		return ErrSMSCodeInvalid
	}
	c.Used = true
	return nil
}

func hashCode(phone, code string) string {
	sum := sha256.Sum256([]byte(phone + ":" + code))
	return hex.EncodeToString(sum[:])
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
	if w.Failures == 0 || now.Sub(w.FirstFailureAt) > SMSAttemptWindowTTL {
		w.FirstFailureAt = now
		w.Failures = 0
	}
	w.Failures++
	w.LastFailureAt = now
	if w.Failures >= MaxSMSFailures {
		return ErrTooManyAttempts
	}
	return nil
}

func (w *AttemptWindow) Blocked(now time.Time) bool {
	if w.Failures < MaxSMSFailures {
		return false
	}
	return now.Before(w.FirstFailureAt.Add(SMSAttemptWindowTTL))
}
