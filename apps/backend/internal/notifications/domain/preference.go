package domain

// NotificationPreference is the owner's permission to send reminders of one
// event type. Permissions follow an opt-out model: a missing stored row means
// the event type is allowed. They are bound to the event type, not to a
// delivery channel.
type NotificationPreference struct {
	EventType EventType
	Allowed   bool
}

// AllEventTypes lists every reminder event type in stable order.
func AllEventTypes() []EventType {
	return []EventType{
		EventOperationDue,
		EventOperationOverdue,
		EventLeaseExpiring,
		EventLeaseRequiresAction,
		EventFreeReminder,
	}
}

// IsValid reports whether the event type is a known reminder event type.
func (e EventType) IsValid() bool {
	switch e {
	case EventOperationDue, EventOperationOverdue, EventLeaseExpiring, EventLeaseRequiresAction, EventFreeReminder:
		return true
	default:
		return false
	}
}

// DefaultNotificationPreferences returns the effective preferences for an
// owner without stored rows: every event type is allowed.
func DefaultNotificationPreferences() []NotificationPreference {
	types := AllEventTypes()
	prefs := make([]NotificationPreference, 0, len(types))
	for _, t := range types {
		prefs = append(prefs, NotificationPreference{EventType: t, Allowed: true})
	}
	return prefs
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
