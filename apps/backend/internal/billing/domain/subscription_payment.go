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
	PaymentStatusRefunded  PaymentStatus = "refunded"
	// PaymentStatusRefunding is an internal, non-final reservation state. A
	// payment enters it atomically before the external provider cancel call so
	// that concurrent refund attempts are rejected instead of refunding the
	// payment twice. It is never produced by external payloads and is not a
	// terminal state.
	PaymentStatusRefunding PaymentStatus = "refunding"
	// PaymentStatusPartialRefunded is kept for backward compatibility and external
	// webhook payloads. The system no longer initiates partial refunds; it always
	// refunds the full amount.
	PaymentStatusPartialRefunded PaymentStatus = "partial_refunded"
)

// SubscriptionPeriod is the billing period for a subscription payment.
type SubscriptionPeriod string

const (
	PeriodMonth SubscriptionPeriod = "month"
	PeriodYear  SubscriptionPeriod = "year"
)

// SubscriptionPayment records a payment event for a user subscription.
type SubscriptionPayment struct {
	ID                    uuid.UUID
	UserID                uuid.UUID
	SubscriptionID        uuid.UUID
	TariffID              uuid.UUID
	PaymentMethodID       *uuid.UUID
	Period                SubscriptionPeriod
	AmountKopecks         int64
	Provider              PaymentProvider
	ProviderPaymentID     *string
	PaymentURL            *string
	Status                PaymentStatus
	RefundedAmountKopecks *int64
	ChargeAttempts        int
	ErrorCode             *string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	SucceededAt           *time.Time
}

// NewSubscriptionPayment creates a new pending subscription payment.
func NewSubscriptionPayment(userID, subscriptionID, tariffID uuid.UUID, paymentMethodID *uuid.UUID, period SubscriptionPeriod, amountKopecks int64, provider PaymentProvider, now time.Time) (SubscriptionPayment, error) {
	if amountKopecks <= 0 {
		return SubscriptionPayment{}, ErrInvalidAmount
	}
	if period != PeriodMonth && period != PeriodYear {
		return SubscriptionPayment{}, ErrInvalidPeriod
	}
	id, err := uuid.NewV7()
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
	now = now.UTC()
	p.Status = PaymentStatusSucceeded
	p.UpdatedAt = now
	p.SucceededAt = &now
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

// BeginRefund atomically reserves the payment for an in-flight refund by moving
// it to the internal refunding status. Only succeeded or pending payments can
// begin a refund. The reservation is taken before the external provider cancel
// call so that concurrent refund attempts are rejected instead of refunding the
// payment twice. refunding is an internal, non-final state.
func (p *SubscriptionPayment) BeginRefund(now time.Time) error {
	if p.Status != PaymentStatusSucceeded && p.Status != PaymentStatusPending {
		return ErrInvalidPaymentStatus
	}
	p.Status = PaymentStatusRefunding
	p.UpdatedAt = now.UTC()
	return nil
}

// MarkRefunded transitions the payment to refunded (full amount only).
// Only succeeded, pending, or refunding (an in-flight refund reservation)
// payments can be refunded.
func (p *SubscriptionPayment) MarkRefunded(now time.Time) error {
	if p.Status != PaymentStatusSucceeded && p.Status != PaymentStatusPending && p.Status != PaymentStatusRefunding {
		return ErrInvalidPaymentStatus
	}
	now = now.UTC()
	p.UpdatedAt = now
	p.RefundedAmountKopecks = &p.AmountKopecks
	p.Status = PaymentStatusRefunded
	return nil
}

// ReconcileToSucceeded transitions a failed payment to succeeded after an
// explicit provider-side status check. It is used for out-of-order webhooks
// where the provider reports success after the system has already marked the
// payment as failed.
func (p *SubscriptionPayment) ReconcileToSucceeded(now time.Time) error {
	if p.Status != PaymentStatusFailed {
		return ErrInvalidPaymentStatus
	}
	now = now.UTC()
	p.Status = PaymentStatusSucceeded
	p.UpdatedAt = now
	p.SucceededAt = &now
	p.ErrorCode = nil
	return nil
}

// ReconcileToRefunded transitions a failed payment to refunded after an
// explicit provider-side status check. It is used for out-of-order webhooks
// where the provider reports a successful refund after the system has already
// marked the payment as failed.
func (p *SubscriptionPayment) ReconcileToRefunded(now time.Time) error {
	if p.Status != PaymentStatusFailed {
		return ErrInvalidPaymentStatus
	}
	now = now.UTC()
	p.UpdatedAt = now
	p.RefundedAmountKopecks = &p.AmountKopecks
	p.Status = PaymentStatusRefunded
	return nil
}

// RevertRefund rolls back an in-flight refund reservation, restoring the
// payment to its previous status. It is used as compensation when the external
// provider cancel call fails or returns a non-refund status. Only payments in
// the refunding state can be reverted.
func (p *SubscriptionPayment) RevertRefund(now time.Time, prev PaymentStatus) error {
	if p.Status != PaymentStatusRefunding {
		return ErrInvalidPaymentStatus
	}
	p.Status = prev
	p.UpdatedAt = now.UTC()
	return nil
}

// IsFinalized reports whether the payment has reached a terminal state.
func (p *SubscriptionPayment) IsFinalized() bool {
	switch p.Status {
	case PaymentStatusSucceeded, PaymentStatusFailed, PaymentStatusRefunded, PaymentStatusPartialRefunded:
		return true
	case PaymentStatusPending, PaymentStatusRefunding:
		return false
	}
	return false
}
