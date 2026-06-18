package domain

import (
	"time"

	"github.com/google/uuid"
)

// PaymentStatus is the lifecycle state of a subscription payment.
type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusSucceeded PaymentStatus = "succeeded"
	PaymentStatusFailed    PaymentStatus = "failed"
)

// SubscriptionPeriod is the billing period for a subscription payment.
type SubscriptionPeriod string

const (
	PeriodMonth SubscriptionPeriod = "month"
	PeriodYear  SubscriptionPeriod = "year"
)

// SubscriptionPayment records a payment event for a user subscription.
type SubscriptionPayment struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	SubscriptionID    uuid.UUID
	TariffID          uuid.UUID
	PaymentMethodID   *uuid.UUID
	Period            SubscriptionPeriod
	AmountKopecks     int64
	Provider          PaymentProvider
	ProviderPaymentID *string
	Status            PaymentStatus
	ErrorCode         *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// NewSubscriptionPayment creates a new pending subscription payment.
func NewSubscriptionPayment(userID, subscriptionID, tariffID uuid.UUID, paymentMethodID *uuid.UUID, period SubscriptionPeriod, amountKopecks int64, provider PaymentProvider, now time.Time) (SubscriptionPayment, error) {
	if amountKopecks < 0 {
		return SubscriptionPayment{}, ErrInvalidAmount
	}
	if period != PeriodMonth && period != PeriodYear {
		return SubscriptionPayment{}, ErrInvalidPeriod
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return SubscriptionPayment{}, err
	}
	now = now.UTC()
	return SubscriptionPayment{
		ID:              id,
		UserID:          userID,
		SubscriptionID:  subscriptionID,
		TariffID:        tariffID,
		PaymentMethodID: paymentMethodID,
		Period:          period,
		AmountKopecks:   amountKopecks,
		Provider:        provider,
		Status:          PaymentStatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

// MarkSucceeded transitions the payment to succeeded.
// Only pending payments can be transitioned.
func (p *SubscriptionPayment) MarkSucceeded(now time.Time) error {
	if p.Status != PaymentStatusPending {
		return ErrInvalidPaymentStatus
	}
	p.Status = PaymentStatusSucceeded
	p.UpdatedAt = now.UTC()
	return nil
}

// MarkFailed transitions the payment to failed and records an optional error code.
// Only pending payments can be transitioned.
func (p *SubscriptionPayment) MarkFailed(errorCode *string, now time.Time) error {
	if p.Status != PaymentStatusPending {
		return ErrInvalidPaymentStatus
	}
	p.Status = PaymentStatusFailed
	p.ErrorCode = errorCode
	p.UpdatedAt = now.UTC()
	return nil
}
