package application

import "errors"

var (
	ErrNotFound               = errors.New("reminder not found")
	ErrReminderNotPending     = errors.New("reminder is not pending")
	ErrInvalidReminderDate    = errors.New("invalid reminder date")
	ErrConcurrentUpdate       = errors.New("reminder changed concurrently")
	ErrDuplicateSMSReminder   = errors.New("sms reminder already sent")
	ErrDuplicateEmailReminder = errors.New("email reminder already sent")
	ErrNoContact              = errors.New("no contact found")
	ErrInvalidPreferences     = errors.New("invalid notification preferences")
)
