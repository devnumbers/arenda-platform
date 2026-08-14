package domain

// AllEventTypes lists every notification event type in stable order.
func AllEventTypes() []EventType {
	return []EventType{
		EventOperationDue,
		EventOperationOverdue,
		EventLeaseExpiring,
		EventLeaseRequiresAction,
		EventFreeReminder,
		EventSubscriptionGrace,
	}
}

// IsValid reports whether the event type is a known notification event type.
func (e EventType) IsValid() bool {
	switch e {
	case EventOperationDue, EventOperationOverdue, EventLeaseExpiring, EventLeaseRequiresAction, EventFreeReminder, EventSubscriptionGrace:
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
