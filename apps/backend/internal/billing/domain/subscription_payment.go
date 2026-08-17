package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SubscriptionPayment is a processing-world money event (ADR 0036): one charge
// of a subscription tariff through the payment provider. The lifecycle states
// and transitions mirror the subscription_payments table of migration 000104:
// a payment is created pending, finalizes to succeeded or failed, and may later
// move to refunded through the admin refund flow (issue #254). The internal
// refunding reservation state is entered by the refund flow only.
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

// NewSubscriptionPayment builds a pending payment for a tariff purchase. The
// provider reference fields are empty: they are filled atomically after the
// provider accepts the initiation (the pending row survives a crash between
// the provider call and the save).
func NewSubscriptionPayment(userID, subscriptionID, tariffID uuid.UUID, period SubscriptionPeriod, amountKopecks int64, provider PaymentProvider, now time.Time) (SubscriptionPayment, error) {
	if amountKopecks <= 0 {
		return SubscriptionPayment{}, ErrInvalidAmount
	}
	if period != PeriodMonth && period != PeriodYear {
		return SubscriptionPayment{}, ErrInvalidPeriod
	}
	if userID == uuid.Nil || subscriptionID == uuid.Nil || tariffID == uuid.Nil {
		return SubscriptionPayment{}, ErrInvalidPayment
	}
	id, err := uuid.NewV7()
	if err != nil {
		return SubscriptionPayment{}, err
	}
	now = now.UTC()
	return SubscriptionPayment{
		ID:             id,
		UserID:         userID,
		SubscriptionID: subscriptionID,
		TariffID:       tariffID,
		Period:         period,
		AmountKopecks:  amountKopecks,
		Provider:       provider,
		Status:         PaymentStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// ReconstituteSubscriptionPayment validates a payment assembled from persisted
// state and returns it. Persistence adapters pass raw storage values through it
// so unknown statuses or missing identity fields are rejected with a
// descriptive error instead of silently producing an invalid aggregate.
func ReconstituteSubscriptionPayment(p SubscriptionPayment) (SubscriptionPayment, error) {
	if p.ID == uuid.Nil || p.UserID == uuid.Nil || p.SubscriptionID == uuid.Nil || p.TariffID == uuid.Nil {
		return SubscriptionPayment{}, errors.New("reconstitute payment: missing identity field")
	}
	switch p.Status {
	case PaymentStatusPending, PaymentStatusSucceeded, PaymentStatusFailed, PaymentStatusRefunding, PaymentStatusRefunded:
	default:
		return SubscriptionPayment{}, fmt.Errorf("reconstitute payment: unknown status %q", p.Status)
	}
	switch p.Period {
	case PeriodMonth, PeriodYear:
	default:
		return SubscriptionPayment{}, fmt.Errorf("reconstitute payment: unknown period %q", p.Period)
	}
	if p.Provider == "" {
		return SubscriptionPayment{}, errors.New("reconstitute payment: missing provider")
	}
	return p, nil
}

// HasProviderReference reports whether the provider's payment identifier is
// persisted for this payment.
func (p *SubscriptionPayment) HasProviderReference() bool {
	return p.ProviderPaymentID != nil && *p.ProviderPaymentID != ""
}

// HasPaymentURL reports whether the payer-facing payment URL is persisted.
func (p *SubscriptionPayment) HasPaymentURL() bool {
	return p.PaymentURL != nil && *p.PaymentURL != ""
}

// IsFinalized reports whether the payment has reached a terminal outcome.
func (p *SubscriptionPayment) IsFinalized() bool {
	switch p.Status {
	case PaymentStatusSucceeded, PaymentStatusFailed, PaymentStatusRefunded:
		return true
	case PaymentStatusPending, PaymentStatusRefunding:
		return false
	}
	return false
}

// SaveProviderReference records the provider's payment identifier and the
// payer-facing URL returned by a successful initiation. Only a pending payment
// without a reference accepts them: once the provider knows the payment, the
// reference never changes.
func (p *SubscriptionPayment) SaveProviderReference(providerPaymentID, paymentURL string, now time.Time) error {
	if p.Status != PaymentStatusPending {
		return ErrInvalidPaymentStatus
	}
	if p.HasProviderReference() {
		return ErrInvalidPaymentStatus
	}
	if providerPaymentID == "" {
		return ErrInvalidPayment
	}
	p.ProviderPaymentID = &providerPaymentID
	if paymentURL != "" {
		p.PaymentURL = &paymentURL
	}
	p.UpdatedAt = now.UTC()
	return nil
}

// RecordChargeAttempt counts one charge attempt that ended with an unresolved
// provider outcome (still pending or unexpected). The renewal worker caps
// these via the configured ChargeAttemptLimit and then fails the payment with
// grace entry, so a permanently unresolved charge cannot keep an expired
// subscription active forever (issue #252). Outcomes the worker can resolve —
// a definitive success or failure — never pass through here.
func (p *SubscriptionPayment) RecordChargeAttempt() {
	p.ChargeAttempts++
}

// MarkSucceeded finalizes a pending payment as succeeded.
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

// MarkFailed finalizes a pending payment as failed and records the provider
// error code when one was reported.
func (p *SubscriptionPayment) MarkFailed(errorCode *string, now time.Time) error {
	if p.Status != PaymentStatusPending {
		return ErrInvalidPaymentStatus
	}
	p.Status = PaymentStatusFailed
	p.ErrorCode = errorCode
	p.UpdatedAt = now.UTC()
	return nil
}

// ReconcileToSucceeded moves a failed payment back to succeeded after the
// provider — the source of truth (ADR 0010) — confirmed the charge was
// captured. Used for out-of-order webhook deliveries where a succeeded
// notification arrives after the payment was already marked failed.
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

// BeginRefund reserves the payment for a full refund (issue #254): the
// internal refunding state is the double-refund guard — the reservation
// transaction moves the payment into it atomically, so a concurrent refund
// loses the race and never reaches the provider. A succeeded payment (the
// captured charge) and a still-pending one (the provider may have captured it
// while the webhook was lost) can be reserved; every terminal or already
// reserved state is rejected.
func (p *SubscriptionPayment) BeginRefund(now time.Time) error {
	if p.Status != PaymentStatusSucceeded && p.Status != PaymentStatusPending {
		return ErrInvalidPaymentStatus
	}
	p.Status = PaymentStatusRefunding
	p.UpdatedAt = now.UTC()
	return nil
}

// RevertRefundReservation rolls the refunding reservation back to the status
// the payment had before it was reserved — the compensation step of the refund
// saga when the provider reports the refund did not happen (issue #254). Only
// the statuses BeginRefund accepts can be restored.
func (p *SubscriptionPayment) RevertRefundReservation(prev PaymentStatus, now time.Time) error {
	if p.Status != PaymentStatusRefunding {
		return ErrInvalidPaymentStatus
	}
	if prev != PaymentStatusSucceeded && prev != PaymentStatusPending {
		return ErrInvalidPaymentStatus
	}
	p.Status = prev
	p.UpdatedAt = now.UTC()
	return nil
}

// StatusBeforeRefundReservation reconstructs the status the payment had before
// the refunding reservation from its persisted shape: the reservation is taken
// from succeeded or pending only, and a payment whose charge was captured
// carries succeeded_at. Callers that hold the payment from before the
// reservation capture the status directly; this serves the paths that read the
// payment after the fact (the reconciliation worker).
func (p *SubscriptionPayment) StatusBeforeRefundReservation() PaymentStatus {
	if p.SucceededAt != nil {
		return PaymentStatusSucceeded
	}
	return PaymentStatusPending
}

// MarkRefunded records a full refund of the payment. Refunds are always
// full-amount (ADR 0037); the subscription-side effects of a refund land with
// the admin refund flow (issue #254).
func (p *SubscriptionPayment) MarkRefunded(now time.Time) error {
	if p.Status != PaymentStatusSucceeded && p.Status != PaymentStatusPending && p.Status != PaymentStatusRefunding {
		return ErrInvalidPaymentStatus
	}
	now = now.UTC()
	amount := p.AmountKopecks
	p.RefundedAmountKopecks = &amount
	p.Status = PaymentStatusRefunded
	p.UpdatedAt = now
	return nil
}
