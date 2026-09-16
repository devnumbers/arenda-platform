package domain

import (
	"fmt"
)

// LoginCodePurpose represents the intended use of a login code.
type LoginCodePurpose string

const (
	LoginCodePurposeLogin       LoginCodePurpose = "login"
	LoginCodePurposePhoneChange LoginCodePurpose = "phone_change"
	LoginCodePurposeEmailChange LoginCodePurpose = "email_change"
)

// NewLoginCodePurpose validates and creates a LoginCodePurpose from a raw string.
func NewLoginCodePurpose(raw string) (LoginCodePurpose, error) {
	switch LoginCodePurpose(raw) {
	case LoginCodePurposeLogin, LoginCodePurposePhoneChange, LoginCodePurposeEmailChange:
		return LoginCodePurpose(raw), nil
	default:
		return "", fmt.Errorf("invalid login code purpose %q: %w", raw, ErrInvalidLoginCodePurpose)
	}
}

// String returns the string representation of the purpose.
func (p LoginCodePurpose) String() string {
	return string(p)
}
