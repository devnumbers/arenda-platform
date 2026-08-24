// Package domain defines the popup registry: stable keys of onboarding popups and their view markers (ADR 0023).
package domain

// PopupKey identifies an info popup the product can show to a user. Keys are
// stable strings shared with the frontend; the server owns the registry of
// active popups.
type PopupKey string

// ActivePopups lists every active popup in display order: the server offers
// them to the client in this order until the user has seen them. The registry
// is currently empty — the reminders onboarding popup was removed together
// with the reminders feature (issue #438); view markers of seen keys stay
// stored and are simply never offered again.
func ActivePopups() []PopupKey {
	return nil
}

// IsValid reports whether the key is a known active popup.
func (k PopupKey) IsValid() bool {
	return false
}
