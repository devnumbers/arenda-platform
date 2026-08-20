// Package domain defines the popup registry: stable keys of onboarding popups and their view markers (ADR 0023).
package domain

// PopupKey identifies an info popup the product can show to a user. Keys are
// stable strings shared with the frontend; the server owns the registry of
// active popups.
type PopupKey string

const (
	// PopupRemindersOnboarding is the onboarding popup about reminders.
	PopupRemindersOnboarding PopupKey = "reminders_onboarding"
)

// ActivePopups lists every active popup in display order: the server offers
// them to the client in this order until the user has seen them.
func ActivePopups() []PopupKey {
	return []PopupKey{
		PopupRemindersOnboarding,
	}
}

// IsValid reports whether the key is a known active popup.
func (k PopupKey) IsValid() bool {
	switch k {
	case PopupRemindersOnboarding:
		return true
	default:
		return false
	}
}
