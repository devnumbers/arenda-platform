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
