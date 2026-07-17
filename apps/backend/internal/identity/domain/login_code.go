package domain

import (
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	// LoginCodeTTL is the lifetime of a newly generated login code.
	LoginCodeTTL = 5 * time.Minute
)

type LoginCode struct {
	ID        uuid.UUID
	UserID    *uuid.UUID
	Phone     Phone
	Email     Email
	Purpose   LoginCodePurpose
	CodeHash  string
	ExpiresAt time.Time
	Used      bool
	CreatedAt time.Time
}

func NewLoginCode(phone Phone, email Email, codeHash string, purpose LoginCodePurpose, userID *uuid.UUID, now time.Time) (LoginCode, error) {
	id, err := uuid.NewV7()
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

	return nil
}

// MarkUsed marks the login code as used.
func (c *LoginCode) MarkUsed() {
	c.Used = true
}

// VerifyAndUse checks the code hash and, if valid, marks the code as used.
func (c *LoginCode) VerifyAndUse(codeHash string, now time.Time) error {
	if err := c.Verify(codeHash, now); err != nil {
		return err
	}
	c.MarkUsed()
	return nil
}
