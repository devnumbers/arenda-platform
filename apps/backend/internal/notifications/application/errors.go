package application

import "errors"

var (
	ErrNotFound            = errors.New("reminder not found")
	ErrReminderNotPending  = errors.New("reminder is not pending")
	ErrInvalidReminderDate = errors.New("invalid reminder date")
)
