package domain

import (
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
	SubscriptionStatusBlocked   SubscriptionStatus = "blocked"
	SubscriptionStatusCancelled SubscriptionStatus = "cancelled"
)

type Subscription struct {
	ID       uuid.UUID
	UserID   uuid.UUID
	TariffID uuid.UUID
	Source   SubscriptionSource
	Status   SubscriptionStatus
}

func NewOwnerSubscription(userID, tariffID uuid.UUID) (Subscription, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return Subscription{}, err
	}
	return Subscription{
		ID:       id,
		UserID:   userID,
		TariffID: tariffID,
		Source:   SubscriptionSourceService,
		Status:   SubscriptionStatusActive,
	}, nil
}
