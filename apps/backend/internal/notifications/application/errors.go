// Package application holds the notifications use cases and ports: per-channel
// preferences, push subscriptions and direct grace notifications.
package application

import "errors"

var (
	ErrNotFound           = errors.New("not found")
	ErrNoContact          = errors.New("no contact found")
	ErrInvalidPreferences = errors.New("invalid notification preferences")
	// ErrInvalidPushSubscription is returned when a push subscription field
	// fails validation.
	ErrInvalidPushSubscription = errors.New("invalid push subscription")
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
