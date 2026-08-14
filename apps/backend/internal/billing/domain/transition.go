package domain

import (
	"time"

	"github.com/google/uuid"
)

// TransitionInitiator names who caused a subscription transition.
type TransitionInitiator string

const (
	// InitiatorUser is the subscription owner acting through the user API.
	InitiatorUser TransitionInitiator = "user"
	// InitiatorAdmin is a platform administrator acting through the admin API.
	InitiatorAdmin TransitionInitiator = "admin"
	// InitiatorSystem is the platform itself (onboarding, workers).
	InitiatorSystem TransitionInitiator = "system"
)

// ParseTransitionInitiator validates and converts a string to
// TransitionInitiator.
func ParseTransitionInitiator(s string) (TransitionInitiator, error) {
	switch TransitionInitiator(s) {
	case InitiatorUser, InitiatorAdmin, InitiatorSystem:
		return TransitionInitiator(s), nil
	}
	return "", ErrInvalidTransition
}

// TransitionReason names why a subscription transition happened. Reasons grow
// with the flows that land (cancellation and downgrade scheduling in #249,
// payments in #250, workers in #252, admin operations in #255); the log table
// stores them as free text so old rows never need rewriting.
type TransitionReason string

const (
	// TransitionReasonRegistered marks the creation of the basic subscription
	// at user registration (issue #245 onboarding).
	TransitionReasonRegistered TransitionReason = "registered"
)

// Transition is one immutable entry of the subscription transition log: the
// status and tariff the subscription moved from and to, why, who caused it,
// and the payment that triggered it when there was one. The first transition
// of a subscription (creation) has no prior status or tariff.
type Transition struct {
	ID             uuid.UUID
	SubscriptionID uuid.UUID
	FromStatus     *SubscriptionStatus
	ToStatus       SubscriptionStatus
	FromTariffID   *uuid.UUID
	ToTariffID     uuid.UUID
	Reason         TransitionReason
	Initiator      TransitionInitiator
	// InitiatorID is the user or admin actor id; nil for the system initiator.
	InitiatorID *uuid.UUID
	// PaymentID references the subscription payment that caused the
	// transition, when there was one.
	PaymentID *uuid.UUID
	CreatedAt time.Time
}

// NewTransition builds the log entry for a subscription state that was just
// applied: it records the move to the subscription's current status and
// tariff, so services call it after mutating the aggregate, inside the same
// transaction. There is deliberately no "from" argument — the from-side is
// captured by the caller before mutation or reconstituted from the previous
// persistent state.
func NewTransition(
	sub Subscription,
	fromStatus *SubscriptionStatus,
	fromTariffID *uuid.UUID,
	reason TransitionReason,
	initiator TransitionInitiator,
	initiatorID *uuid.UUID,
) (Transition, error) {
	if reason == "" {
		return Transition{}, ErrInvalidTransition
	}
	if _, err := ParseTransitionInitiator(string(initiator)); err != nil {
		return Transition{}, err
	}
	if sub.ID == uuid.Nil || sub.TariffID == uuid.Nil {
		return Transition{}, ErrInvalidTransition
	}
	if !validSubscriptionStatus(sub.Status) {
		return Transition{}, ErrInvalidTransition
	}
	if initiator == InitiatorSystem && initiatorID != nil {
		return Transition{}, ErrInvalidTransition
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Transition{}, err
	}
	return Transition{
		ID:             id,
		SubscriptionID: sub.ID,
		FromStatus:     fromStatus,
		ToStatus:       sub.Status,
		FromTariffID:   fromTariffID,
		ToTariffID:     sub.TariffID,
		Reason:         reason,
		Initiator:      initiator,
		InitiatorID:    initiatorID,
	}, nil
}

// ReconstituteTransition validates a Transition assembled from persisted state.
// Persistence adapters pass raw storage values through it so unknown enum
// values are rejected with a descriptive error.
func ReconstituteTransition(t Transition) (Transition, error) {
	if _, err := ParseTransitionInitiator(string(t.Initiator)); err != nil {
		return Transition{}, err
	}
	if t.Reason == "" {
		return Transition{}, ErrInvalidTransition
	}
	if t.SubscriptionID == uuid.Nil || t.ToTariffID == uuid.Nil {
		return Transition{}, ErrInvalidTransition
	}
	if !validSubscriptionStatus(t.ToStatus) {
		return Transition{}, ErrInvalidTransition
	}
	if t.FromStatus != nil && !validSubscriptionStatus(*t.FromStatus) {
		return Transition{}, ErrInvalidTransition
	}
	return t, nil
}

// validSubscriptionStatus reports whether the value is one of the ADR 0008
// lifecycle statuses.
func validSubscriptionStatus(status SubscriptionStatus) bool {
	switch status {
	case SubscriptionStatusActive, SubscriptionStatusGrace, SubscriptionStatusCancelled:
		return true
	}
	return false
}
