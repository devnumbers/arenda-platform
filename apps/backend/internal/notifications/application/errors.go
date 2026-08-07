package application

import "errors"

var (
	ErrNotFound = errors.New("reminder not found")
	// ErrForbidden is returned when an actor can view a property but lacks the
	// capability for the requested operation (e.g. a viewer editing) — T3,
	// issue #156. RoleNone and RoleSuspended are mapped to ErrNotFound to
	// preserve object privacy (issue #166).
	ErrForbidden                = errors.New("forbidden")
	ErrReminderNotPending       = errors.New("reminder is not pending")
	ErrInvalidReminderDate      = errors.New("invalid reminder date")
	ErrConcurrentUpdate         = errors.New("reminder changed concurrently")
	ErrDuplicateSMSReminder     = errors.New("sms reminder already sent")
	ErrDuplicateEmailReminder   = errors.New("email reminder already sent")
	ErrNoContact                = errors.New("no contact found")
	ErrInvalidPreferences       = errors.New("invalid notification preferences")
	ErrInvalidFreeReminderInput = errors.New("invalid free reminder input")
)
