package domain

import (
	"time"

	"github.com/google/uuid"
)

// PaymentProvider identifies a payment processing provider.
type PaymentProvider string

const (
	// ProviderFake is a local/dev fake provider used for manual testing.
	ProviderFake PaymentProvider = "fake"
	// ProviderTkassa is the T-Kassa payment provider.
	ProviderTkassa PaymentProvider = "tkassa"
)

// PaymentMethod stores a reusable payment instrument for a user.
type PaymentMethod struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	Provider      PaymentProvider
	ProviderToken string
	DisplayMask   string
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// NewPaymentMethod creates a new inactive payment method.
func NewPaymentMethod(userID uuid.UUID, provider PaymentProvider, providerToken, displayMask string, now time.Time) (PaymentMethod, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return PaymentMethod{}, err
	}
	now = now.UTC()
	return PaymentMethod{
		ID:            id,
		UserID:        userID,
		Provider:      provider,
		ProviderToken: providerToken,
		DisplayMask:   displayMask,
		IsActive:      false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// Activate marks the payment method as active.
func (pm *PaymentMethod) Activate(now time.Time) {
	pm.IsActive = true
	pm.UpdatedAt = now.UTC()
}

// Deactivate marks the payment method as inactive.
func (pm *PaymentMethod) Deactivate(now time.Time) {
	pm.IsActive = false
	pm.UpdatedAt = now.UTC()
}
