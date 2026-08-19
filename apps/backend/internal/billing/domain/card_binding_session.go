package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CardBindingSessionStatus is the lifecycle state of a card-binding session
// (billing CONTEXT.md). It matches the card_binding_sessions status CHECK in
// migration 000104.
type CardBindingSessionStatus string

const (
	// CardBindingNew means the binding was initiated and the payer has not
	// completed the provider form yet.
	CardBindingNew CardBindingSessionStatus = "new"
	// CardBindingCompleted means the provider confirmed the binding and the
	// payment method was created.
	CardBindingCompleted CardBindingSessionStatus = "completed"
	// CardBindingRejected means the binding was refused by the payer/provider
	// or expired without completing; it never produces a payment method.
	CardBindingRejected CardBindingSessionStatus = "rejected"
)

// CardBindingSession is one initiated card binding at the provider (billing
// CONTEXT.md): it carries the provider's RequestKey and a TTL. A confirmed
// binding creates the payment method; an expired one creates nothing. It
// replaces the pre-rewrite token-placeholder convention in payment methods
// (issue #251, ADR 0037).
type CardBindingSession struct {
	ID uuid.UUID
	// UserID is the user who initiated the binding; the webhook and polling
	// paths resolve the future payment method's owner from this row, not from
	// provider-carried identifiers.
	UserID uuid.UUID
	// Provider is the processor the binding was initiated at (ADR 0038); the
	// RequestKey is only meaningful together with it.
	Provider PaymentProvider
	// RequestKey is the provider's identifier of the binding session: it links
	// the add-card notification and the status polling to this row.
	RequestKey string
	Status     CardBindingSessionStatus
	// ExpiresAt bounds how long the binding can still complete; after it the
	// session is dead and never produces a payment method.
	ExpiresAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewCardBindingSession creates a fresh open binding session with the given
// lifetime.
func NewCardBindingSession(
	userID uuid.UUID, provider PaymentProvider, requestKey string, expiresAt, now time.Time,
) (CardBindingSession, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return CardBindingSession{}, err
	}
	session := CardBindingSession{
		ID:         id,
		UserID:     userID,
		Provider:   provider,
		RequestKey: requestKey,
		Status:     CardBindingNew,
		ExpiresAt:  expiresAt.UTC(),
		CreatedAt:  now.UTC(),
		UpdatedAt:  now.UTC(),
	}
	if err := session.validate(); err != nil {
		return CardBindingSession{}, err
	}
	return session, nil
}

// ReconstituteCardBindingSession validates a session assembled from persisted
// state and returns it, rejecting unknown statuses and missing identity
// fields.
func ReconstituteCardBindingSession(session CardBindingSession) (CardBindingSession, error) {
	if err := session.validate(); err != nil {
		return CardBindingSession{}, err
	}
	if session.CreatedAt.IsZero() {
		return CardBindingSession{}, errors.New("reconstitute card binding session: missing created at")
	}
	return session, nil
}

func (s *CardBindingSession) validate() error {
	if s.ID == uuid.Nil {
		return errors.New("card binding session: missing id")
	}
	if s.UserID == uuid.Nil {
		return errors.New("card binding session: missing user id")
	}
	if s.Provider == "" {
		return errors.New("card binding session: missing provider")
	}
	if s.RequestKey == "" {
		return errors.New("card binding session: missing request key")
	}
	switch s.Status {
	case CardBindingNew, CardBindingCompleted, CardBindingRejected:
	default:
		return fmt.Errorf("card binding session: unknown status %q", s.Status)
	}
	if s.ExpiresAt.IsZero() {
		return errors.New("card binding session: missing expires at")
	}
	return nil
}

// IsOpen reports whether the binding is still awaiting an outcome.
func (s *CardBindingSession) IsOpen() bool {
	return s.Status == CardBindingNew
}

// IsExpired reports whether the binding's lifetime is over at the given
// moment. An expired session never produces a payment method, whatever the
// provider reports later.
func (s *CardBindingSession) IsExpired(now time.Time) bool {
	return now.UTC().After(s.ExpiresAt)
}

// CanComplete reports whether a binding confirmation may still be applied:
// the session must be open and inside its lifetime.
func (s *CardBindingSession) CanComplete(now time.Time) bool {
	return s.IsOpen() && !s.IsExpired(now)
}

// MarkCompleted records that the binding produced a payment method.
func (s *CardBindingSession) MarkCompleted(now time.Time) error {
	if !s.CanComplete(now) {
		return fmt.Errorf("card binding session %s cannot complete from status %q", s.ID, s.Status)
	}
	s.Status = CardBindingCompleted
	s.UpdatedAt = now.UTC()
	return nil
}

// MarkRejected closes the session without a payment method: the provider
// refused the binding, the payer abandoned it, or its lifetime is over.
// Completing an already-closed session is an idempotent no-op.
func (s *CardBindingSession) MarkRejected(now time.Time) error {
	if !s.IsOpen() {
		return nil
	}
	s.Status = CardBindingRejected
	s.UpdatedAt = now.UTC()
	return nil
}
