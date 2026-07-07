package domain

import "errors"

var (
	ErrLoginCodeInvalid        = errors.New("login code invalid")
	ErrTooManyAttempts         = errors.New("too many attempts")
	ErrInvalidEmail            = errors.New("invalid email")
	ErrInvalidPhone            = errors.New("invalid Russian phone number")
	ErrInvalidRole             = errors.New("invalid role")
	ErrInvalidLoginCodePurpose = errors.New("invalid login code purpose")
)
