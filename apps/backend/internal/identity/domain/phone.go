package domain

import (
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/phone"
)

// ErrInvalidPhone is re-exported from the shared phone package for convenience.
var ErrInvalidPhone = phone.ErrInvalidPhone

// Phone is a canonical Russian mobile phone number.
type Phone = phone.Phone

// NewPhone parses and normalizes a raw phone string into a Phone value.
func NewPhone(raw string) (Phone, error) {
	return phone.NewPhone(raw)
}
