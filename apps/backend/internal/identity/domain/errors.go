package domain

import "errors"

var (
	ErrLoginCodeInvalid        = errors.New("login code invalid")
	ErrTooManyAttempts         = errors.New("too many attempts")
	ErrInvalidEmail            = errors.New("invalid email")
	ErrInvalidTimezone         = errors.New("invalid timezone")
	ErrInvalidPhone            = errors.New("invalid Russian phone number")
	ErrInvalidRole             = errors.New("invalid role")
	ErrInvalidLoginCodePurpose = errors.New("invalid login code purpose")
)
