package domain

import (
	"strings"
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

// PendingCardBindingTokenPrefix marks payment-method rows that are internal
// placeholders for an in-flight T-Kassa AddCard bank-form flow. The
// placeholder's ProviderToken is the prefix plus the AddCard RequestKey, so a
// placeholder can never be confused with a real card row: recovery paths
// (RebillId restore from provider status polls, webhook and sync upserts)
// store the raw RebillId, which never carries this prefix.
const PendingCardBindingTokenPrefix = "addcard:"

// PendingCardBindingToken builds the placeholder provider token for a T-Kassa
// AddCard RequestKey.
func PendingCardBindingToken(requestKey string) string {
	return PendingCardBindingTokenPrefix + requestKey
}

// PaymentMethod stores a reusable payment instrument for a user.
type PaymentMethod struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	Provider       PaymentProvider
	ProviderToken  string
	ProviderCardID string
	DisplayMask    string
	ExpDate        string
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// NewPaymentMethod creates a new inactive payment method.
func NewPaymentMethod(userID uuid.UUID, provider PaymentProvider, providerToken, displayMask string, now time.Time) (PaymentMethod, error) {
	id, err := uuid.NewV7()
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

// PendingCardBindingRequestKey returns the T-Kassa AddCard RequestKey when the
// row is a pending card-binding placeholder: an internal row created when the
// AddCard bank-form flow was initiated, before the card is bound. The
// placeholder stores the RequestKey in ProviderToken behind the
// PendingCardBindingTokenPrefix and has neither a card id nor a display mask
// yet; real cards always carry both (from the AddCard webhook or the
// GetCardList sync). Rows storing a raw RebillId — for example recovered from
// a provider status poll — never match, because the RequestKey is returned
// only when the prefix is present (and stripped from the result). It returns
// an empty string for regular payment methods.
func (pm *PaymentMethod) PendingCardBindingRequestKey() string {
	if pm.Provider != ProviderTkassa || pm.ProviderCardID != "" || pm.DisplayMask != "" {
		return ""
	}
	requestKey, ok := strings.CutPrefix(pm.ProviderToken, PendingCardBindingTokenPrefix)
	if !ok {
		return ""
	}
	return requestKey
}

// Deactivate marks the payment method as inactive.
func (pm *PaymentMethod) Deactivate(now time.Time) {
	pm.IsActive = false
	pm.UpdatedAt = now.UTC()
}
