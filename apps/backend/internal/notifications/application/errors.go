package application

import "errors"

var (
	ErrNotFound = errors.New("reminder not found")
	// ErrForbidden is returned when an actor can view a property but lacks the
	// capability for the requested operation (e.g. a viewer editing) — T3,
	// issue #156. RoleNone and RoleSuspended are mapped to ErrNotFound to
	// preserve object privacy (issue #166).
	ErrForbidden              = errors.New("forbidden")
	ErrReminderNotPending     = errors.New("reminder is not pending")
	ErrInvalidReminderDate    = errors.New("invalid reminder date")
	ErrConcurrentUpdate       = errors.New("reminder changed concurrently")
	ErrDuplicateSMSReminder   = errors.New("sms reminder already sent")
	ErrDuplicateEmailReminder = errors.New("email reminder already sent")
	ErrNoContact              = errors.New("no contact found")
	ErrInvalidPreferences     = errors.New("invalid notification preferences")
	// ErrInvalidPushSubscription is returned when a push subscription field
	// fails validation.
	ErrInvalidPushSubscription = errors.New("invalid push subscription")
	// ErrDuplicatePushReminder is returned when a push audit row for the same
	// reminder and recipient already exists (per-recipient deduplication).
	ErrDuplicatePushReminder = errors.New("push reminder already sent")
	// ErrSubscriptionGone is returned by the PushSender when the push service
	// responds 404 or 410: the subscription is no longer valid and must be
	// deleted (RFC 8030 §7.3).
	ErrSubscriptionGone = errors.New("push subscription gone")
	// ErrRateLimited is returned by the PushSender when the push service
	// responds 429 Too Many Requests (RFC 8030 §8.4). The caller should back
	// off and honour Retry-After if present.
	ErrRateLimited = errors.New("push rate limited")
	// ErrPushPayloadTooLarge is returned when the encrypted payload exceeds the
	// 3993-byte practical ceiling and the push service rejects it with 413.
	ErrPushPayloadTooLarge = errors.New("push payload too large")
)
