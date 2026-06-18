package domain

import (
	"time"

	"github.com/google/uuid"
)

type SubscriptionSource string

const (
	SubscriptionSourcePaid    SubscriptionSource = "paid"
	SubscriptionSourceService SubscriptionSource = "service"
)

type SubscriptionStatus string

const (
	SubscriptionStatusActive    SubscriptionStatus = "active"
	SubscriptionStatusGrace     SubscriptionStatus = "grace"
	SubscriptionStatusBlocked   SubscriptionStatus = "blocked"
	SubscriptionStatusCancelled SubscriptionStatus = "cancelled"
)

type Subscription struct {
	ID                    uuid.UUID
	UserID                uuid.UUID
	TariffID              uuid.UUID
	Source                SubscriptionSource
	Status                SubscriptionStatus
	ValidUntil            *time.Time
	AutoRenewEnabled      bool
	PendingTariffID       *uuid.UUID
	PendingChangeAt       *time.Time
	ActivePaymentMethodID *uuid.UUID
}

// NewOwnerSubscription creates a free basic subscription for a newly-registered owner.
func NewOwnerSubscription(userID, tariffID uuid.UUID) (Subscription, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return Subscription{}, err
	}
	return Subscription{
		ID:               id,
		UserID:           userID,
		TariffID:         tariffID,
		Source:           SubscriptionSourcePaid,
		Status:           SubscriptionStatusActive,
		AutoRenewEnabled: false,
		ValidUntil:       nil,
	}, nil
}

// CanMutateData reports whether the subscription allows the user to mutate property and finance data.
func (s Subscription) CanMutateData() bool {
	return s.Status == SubscriptionStatusActive || s.Status == SubscriptionStatusGrace
}

// IsPaidSource reports whether the subscription is paid for by the owner.
func (s Subscription) IsPaidSource() bool {
	return s.Source == SubscriptionSourcePaid
}

// IsInGrace reports whether the subscription is currently within its grace period.
func (s Subscription) IsInGrace(now time.Time) bool {
	if s.Status != SubscriptionStatusGrace || s.ValidUntil == nil {
		return false
	}
	return !now.After(*s.ValidUntil)
}

// HasPendingChange reports whether a tariff change is scheduled for the subscription.
func (s Subscription) HasPendingChange() bool {
	return s.PendingTariffID != nil && s.PendingChangeAt != nil
}
