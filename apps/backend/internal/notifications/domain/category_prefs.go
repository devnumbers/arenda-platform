package domain

// CategoryPrefs is one channel's copy of the settings matrix (карта #734,
// решение #738, ADR 0058): the four user-configurable categories as booleans.
// Email keeps one set per account (notification_email_preferences), push
// keeps one per device — on the push subscription itself. Тариф and Системные
// are service categories: always on, outside the settings screen, nothing
// stores them.
type CategoryPrefs struct {
	Rental             bool
	PaymentsOperations bool
	Tasks              bool
	SharedAccess       bool
}

// DefaultCategoryPrefs returns the all-on set — the default of every account
// and every device («всё включено», решение #738): a missing settings row
// reads as this value.
func DefaultCategoryPrefs() CategoryPrefs {
	return CategoryPrefs{Rental: true, PaymentsOperations: true, Tasks: true, SharedAccess: true}
}

// Allows reports whether the category may deliver through the channel this
// set belongs to. The service categories (Тариф, Системные) are never gated —
// the answer is true without reading anything; an unknown category fails open
// the same way.
func (p CategoryPrefs) Allows(c Category) bool {
	switch c {
	case CategoryRental:
		return p.Rental
	case CategoryPaymentsOperations:
		return p.PaymentsOperations
	case CategoryTasks:
		return p.Tasks
	case CategorySharedAccess:
		return p.SharedAccess
	default:
		return true
	}
}
