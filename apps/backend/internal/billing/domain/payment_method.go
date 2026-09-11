package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// CardSystem is the payment system of a card, derived from the BIN prefix of
// its masked number (issue #619): the vocabulary the contract exposes on
// payment methods and payment history. The provider reports no issuer data,
// so this derivation is all the card identification there is.
type CardSystem string

const (
	// CardSystemMir marks a Mir card (BIN prefix 2).
	CardSystemMir CardSystem = "mir"
	// CardSystemVisa marks a Visa card (BIN prefix 4).
	CardSystemVisa CardSystem = "visa"
	// CardSystemMastercard marks a Mastercard card (BIN prefix 5).
	CardSystemMastercard CardSystem = "mastercard"
	// CardSystemUnknown marks a card whose BIN prefix maps to no contract
	// value — old or unrecognized cards (including UnionPay BINs: the
	// contract enum has no value for them).
	CardSystemUnknown CardSystem = "unknown"
)

// CardSystemFromMask derives the card system from the leading digit of a
// masked card number. Everything it cannot classify is unknown, never an
// error: the field is display metadata, not identity.
func CardSystemFromMask(displayMask string) CardSystem {
	for _, c := range []byte(displayMask) {
		if c < '0' || c > '9' {
			break
		}
		switch c {
		case '2':
			return CardSystemMir
		case '4':
			return CardSystemVisa
		case '5':
			return CardSystemMastercard
		default:
			return CardSystemUnknown
		}
	}
	return CardSystemUnknown
}

// PaymentMethod is a saved payment instrument of a user (a bound card,
// billing CONTEXT.md): the provider's charge token with display fields for the
// method list. A user may have several methods but at most one active — the
// one subscription renewals charge. The card-binding flow that produces
// methods is modelled by CardBindingSession; this aggregate deliberately
// carries no binding semantics (issue #251).
type PaymentMethod struct {
	ID uuid.UUID
	// UserID is the owner of the method; binding, activation and deletion are
	// owner-scoped operations.
	UserID uuid.UUID
	// Provider identifies the external processor the token belongs to
	// (ADR 0038); a token is only meaningful together with its provider.
	Provider PaymentProvider
	// ProviderToken is the charge token merchant-initiated charges run on
	// (RebillId in T-Kassa terms). Stored encrypted at rest; uniqueness is
	// enforced per user by its hash.
	ProviderToken string
	// ProviderCardID is the provider's identifier of the saved card, the
	// handle for detaching it later. Stored encrypted at rest.
	ProviderCardID string
	// DisplayMask is the masked card number for display, e.g.
	// "4300********1234". Non-sensitive, stored as-is.
	DisplayMask string
	// ExpDate is the card expiry in the provider's display format (MMYY).
	// Stored encrypted at rest.
	ExpDate string
	// IsActive marks the single method renewals charge. Exactly one active
	// method per user is a database invariant (partial unique index).
	IsActive bool
	// CreatedAt/UpdatedAt carry the updated_at trigger semantics of the table.
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewPaymentMethod creates a fresh inactive payment method: a method becomes
// active only through the explicit activation step, never at creation.
func NewPaymentMethod(userID uuid.UUID, provider PaymentProvider, providerToken string, now time.Time) (PaymentMethod, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return PaymentMethod{}, err
	}
	method := PaymentMethod{
		ID:            id,
		UserID:        userID,
		Provider:      provider,
		ProviderToken: providerToken,
		CreatedAt:     now.UTC(),
		UpdatedAt:     now.UTC(),
	}
	if err := method.validate(); err != nil {
		return PaymentMethod{}, err
	}
	return method, nil
}

// ReconstitutePaymentMethod validates a PaymentMethod assembled from persisted
// state and returns it. Persistence adapters build the aggregate from raw
// storage values (including unchecked enum casts) and pass it here so that
// missing identity fields or an empty charge token are rejected with a
// descriptive error instead of silently producing an invalid aggregate.
func ReconstitutePaymentMethod(method PaymentMethod) (PaymentMethod, error) {
	if err := method.validate(); err != nil {
		return PaymentMethod{}, err
	}
	if method.CreatedAt.IsZero() {
		return PaymentMethod{}, errors.New("reconstitute payment method: missing created at")
	}
	return method, nil
}

func (m PaymentMethod) validate() error {
	if m.ID == uuid.Nil {
		return errors.New("payment method: missing id")
	}
	if m.UserID == uuid.Nil {
		return errors.New("payment method: missing user id")
	}
	if m.Provider == "" {
		return errors.New("payment method: missing provider")
	}
	if m.ProviderToken == "" {
		return errors.New("payment method: missing provider token")
	}
	return nil
}
