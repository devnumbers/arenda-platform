package application

import "errors"

var (
	ErrNotFound          = errors.New("not found")
	ErrLimitExceeded     = errors.New("active property limit exceeded")
	ErrInvalidTransition = errors.New("invalid property status transition")
	ErrInvalidInput      = errors.New("invalid property input")
)
