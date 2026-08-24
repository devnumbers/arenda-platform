// Package domain defines the notifications vocabulary: the event types,
// delivery channels, per-channel preferences and push subscriptions.
package domain

// EventType identifies the kind of a notification. The domain knows only
// subscription_grace (issue #438): the rental event types were removed with
// the leases context, but their values remain as dead rows in the
// notification_event_type enum in PostgreSQL (the #277 precedent) — stored
// preference rows referencing them are simply ignored.
type EventType string

// EventSubscriptionGrace covers the billing grace lifecycle notifications:
// a failed renewal charge entering grace and the grace window closing.
const EventSubscriptionGrace EventType = "subscription_grace"

// AllEventTypes lists every notification event type in stable order.
func AllEventTypes() []EventType {
	return []EventType{
		EventSubscriptionGrace,
	}
}

// IsValid reports whether the event type is a known notification event type.
func (e EventType) IsValid() bool {
	switch e {
	case EventSubscriptionGrace:
		return true
	default:
		return false
	}
}

// NotificationChannel is a delivery channel for reminders. Per-channel
// preferences (ADR 0030) let the owner enable or disable email and push
// independently for each event type.
type NotificationChannel string

const (
	// ChannelEmail is the email delivery channel.
	ChannelEmail NotificationChannel = "email"
	// ChannelPush is the Web Push delivery channel.
	ChannelPush NotificationChannel = "push"
)

// IsValid reports whether the channel is a known delivery channel.
func (c NotificationChannel) IsValid() bool {
	switch c {
	case ChannelEmail, ChannelPush:
		return true
	default:
		return false
	}
}

// AllNotificationChannels lists every delivery channel in stable order.
func AllNotificationChannels() []NotificationChannel {
	return []NotificationChannel{ChannelEmail, ChannelPush}
}

// NotificationChannelPreference is the owner's permission to send reminders of
// one event type over one delivery channel. Permissions follow an opt-out
// model: a missing stored row means the (event type, channel) pair is allowed.
type NotificationChannelPreference struct {
	EventType EventType
	Channel   NotificationChannel
	Allowed   bool
}

// DefaultNotificationChannelPreferences returns the effective per-channel
// preferences for an owner without stored rows: every event type is allowed on
// every channel.
func DefaultNotificationChannelPreferences() []NotificationChannelPreference {
	types := AllEventTypes()
	channels := AllNotificationChannels()
	prefs := make([]NotificationChannelPreference, 0, len(types)*len(channels))
	for _, t := range types {
		for _, c := range channels {
			prefs = append(prefs, NotificationChannelPreference{EventType: t, Channel: c, Allowed: true})
		}
	}
	return prefs
}
