package domain

import (
	"fmt"
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
	SubscriptionStatusCancelled SubscriptionStatus = "cancelled"
)

const gracePeriod = 7 * 24 * time.Hour

// ParseSubscriptionPeriod validates and converts a string to SubscriptionPeriod.
func ParseSubscriptionPeriod(s string) (SubscriptionPeriod, error) {
	switch SubscriptionPeriod(s) {
	case PeriodMonth, PeriodYear:
		return SubscriptionPeriod(s), nil
	}
	return "", ErrInvalidPeriod
}

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
	PendingPeriod         *SubscriptionPeriod
	ActivePaymentMethodID *uuid.UUID
	LastAppliedPaymentID  *uuid.UUID
}

// NewOwnerSubscription creates a free basic subscription for a newly-registered owner.
// Source is set to "paid" because the owner is on the paid-subscription track,
// even though the initial basic tariff itself is free.
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

// ReconstituteSubscription validates a Subscription assembled from persisted
// state and returns it. Persistence adapters build the aggregate from raw
// storage values (including unchecked enum casts) and pass it here so that
// unknown statuses/sources/pending periods or missing identity fields are
// rejected with a descriptive error instead of silently producing an invalid
// aggregate.
func ReconstituteSubscription(sub Subscription) (Subscription, error) {
	if sub.ID == uuid.Nil {
		return Subscription{}, fmt.Errorf("reconstitute subscription: missing id")
	}
	if sub.UserID == uuid.Nil {
		return Subscription{}, fmt.Errorf("reconstitute subscription: missing user id")
	}
	if sub.TariffID == uuid.Nil {
		return Subscription{}, fmt.Errorf("reconstitute subscription: missing tariff id")
	}
	switch sub.Status {
	case SubscriptionStatusActive, SubscriptionStatusGrace, SubscriptionStatusCancelled:
	default:
		return Subscription{}, fmt.Errorf("reconstitute subscription: unknown status %q", sub.Status)
	}
	switch sub.Source {
	case SubscriptionSourcePaid, SubscriptionSourceService:
	default:
		return Subscription{}, fmt.Errorf("reconstitute subscription: unknown source %q", sub.Source)
	}
	if sub.PendingPeriod != nil {
		switch *sub.PendingPeriod {
		case PeriodMonth, PeriodYear:
		default:
			return Subscription{}, fmt.Errorf("reconstitute subscription: unknown pending period %q", *sub.PendingPeriod)
		}
	}
	return sub, nil
}

// CanMutateData reports whether the subscription allows the user to mutate
// property and finance data at the given moment. Active subscriptions whose
// paid period has already expired are treated as non-mutable until the worker
// transitions them to grace or basic. Cancelled subscriptions remain mutable
// until the end of the already paid period, matching the behaviour of active
// subscriptions.
func (s *Subscription) CanMutateData(now time.Time) bool {
	if s.Status == SubscriptionStatusGrace {
		return s.IsInGrace(now)
	}
	if s.Status != SubscriptionStatusActive && s.Status != SubscriptionStatusCancelled {
		return false
	}
	if s.ValidUntil != nil && now.After(*s.ValidUntil) {
		return false
	}
	return true
}

// IsPaidSource reports whether the subscription is paid for by the owner.
func (s *Subscription) IsPaidSource() bool {
	return s.Source == SubscriptionSourcePaid
}

// CanInitiatePayment reports whether the subscription status allows a new
// payment to be started at the given moment.
func (s *Subscription) CanInitiatePayment(now time.Time) bool {
	if s.Status == SubscriptionStatusActive {
		return true
	}
	return s.IsInGrace(now)
}

// IsInGrace reports whether the subscription is currently within its grace period.
func (s *Subscription) IsInGrace(now time.Time) bool {
	if s.Status != SubscriptionStatusGrace || s.ValidUntil == nil {
		return false
	}
	return !now.After(*s.ValidUntil)
}

// HasPendingChange reports whether a tariff change is scheduled for the subscription.
func (s *Subscription) HasPendingChange() bool {
	return s.PendingTariffID != nil && s.PendingChangeAt != nil
}

// ScheduleDowngrade schedules a downgrade to take effect when the current paid
// period ends. It requires a valid_until date and enables auto-renew so the new
// tariff keeps renewing on the normal cycle after it is applied. The new tariff
// must be a downgrade from the current tariff.
func (s *Subscription) ScheduleDowngrade(currentTariff, newTariff Tariff, period SubscriptionPeriod, changeAt time.Time) error {
	if s.Status != SubscriptionStatusActive && s.Status != SubscriptionStatusGrace {
		return ErrInvalidSubscriptionState
	}
	if period != PeriodMonth && period != PeriodYear {
		return ErrInvalidPeriod
	}
	if s.TariffID != currentTariff.ID {
		return ErrInvalidTariffChange
	}
	if s.TariffID == newTariff.ID {
		return ErrAlreadyOnTariff
	}
	if s.ValidUntil == nil {
		return ErrInvalidTariffChange
	}
	if changeAt.Before(*s.ValidUntil) {
		return ErrInvalidTariffChange
	}
	if ClassifyTariffChange(currentTariff, newTariff) != TariffChangeDowngrade {
		return ErrInvalidTariffChange
	}
	s.PendingTariffID = &newTariff.ID
	s.PendingChangeAt = &changeAt
	s.PendingPeriod = &period
	s.AutoRenewEnabled = true
	return nil
}

// ApplyTariffChange applies a successful tariff change immediately. It sets the
// new tariff, extends validity by the chosen period, enables auto-renew,
// records the applied payment and clears any pending change.
func (s *Subscription) ApplyTariffChange(paymentID uuid.UUID, currentTariff, newTariff Tariff, period SubscriptionPeriod, now time.Time) error {
	if period != PeriodMonth && period != PeriodYear {
		return ErrInvalidPeriod
	}
	if s.TariffID != currentTariff.ID {
		return ErrInvalidTariffChange
	}
	if s.TariffID == newTariff.ID {
		return ErrAlreadyOnTariff
	}
	if ClassifyTariffChange(currentTariff, newTariff) == TariffChangeSame {
		return ErrInvalidTariffChange
	}
	validUntil := addSubscriptionPeriod(now, period)
	s.TariffID = newTariff.ID
	s.ValidUntil = &validUntil
	s.AutoRenewEnabled = true
	s.LastAppliedPaymentID = &paymentID
	s.PendingTariffID = nil
	s.PendingChangeAt = nil
	s.PendingPeriod = nil
	s.Status = SubscriptionStatusActive
	return nil
}

// ApplyRenewal extends the subscription validity by one period and records the
// applied payment. The extension is calculated from the current valid_until when
// it exists and is in the future; otherwise it starts from now. A successful
// renewal also clears any scheduled downgrade.
func (s *Subscription) ApplyRenewal(paymentID uuid.UUID, period SubscriptionPeriod, now time.Time) error {
	if period != PeriodMonth && period != PeriodYear {
		return ErrInvalidPeriod
	}
	base := now
	if s.ValidUntil != nil && s.ValidUntil.After(base) {
		base = *s.ValidUntil
	}
	validUntil := addSubscriptionPeriod(base, period)
	s.ValidUntil = &validUntil
	s.LastAppliedPaymentID = &paymentID
	s.Status = SubscriptionStatusActive
	s.PendingTariffID = nil
	s.PendingChangeAt = nil
	s.PendingPeriod = nil
	return nil
}

// ApplyScheduledDowngrade applies a deferred downgrade at the end of the paid
// period. It requires the subscription to be active with a matching pending
// change that is already due: the pending tariff and period must equal the
// requested ones and pending_change_at must be set at or before now.
// Per ADR 0008 §3 there is no charge at apply time: it switches the tariff,
// extends valid_until by the chosen period from now, enables auto-renew, sets
// status to active, and clears the pending change. LastAppliedPaymentID is
// intentionally left untouched because no payment is involved.
func (s *Subscription) ApplyScheduledDowngrade(newTariff Tariff, period SubscriptionPeriod, now time.Time) error {
	if period != PeriodMonth && period != PeriodYear {
		return ErrInvalidPeriod
	}
	if s.TariffID == newTariff.ID {
		return ErrAlreadyOnTariff
	}
	if s.Status != SubscriptionStatusActive {
		return ErrInvalidSubscriptionState
	}
	if s.PendingTariffID == nil || *s.PendingTariffID != newTariff.ID {
		return ErrInvalidSubscriptionState
	}
	if s.PendingPeriod == nil || *s.PendingPeriod != period {
		return ErrInvalidSubscriptionState
	}
	if s.PendingChangeAt == nil || s.PendingChangeAt.After(now) {
		return ErrInvalidSubscriptionState
	}
	validUntil := addSubscriptionPeriod(now, period)
	s.TariffID = newTariff.ID
	s.ValidUntil = &validUntil
	s.AutoRenewEnabled = true
	s.Status = SubscriptionStatusActive
	s.PendingTariffID = nil
	s.PendingChangeAt = nil
	s.PendingPeriod = nil
	return nil
}

// ClearPendingChange clears any scheduled tariff change pending on the
// subscription without modifying the current tariff, validity or status.
func (s *Subscription) ClearPendingChange() {
	s.PendingTariffID = nil
	s.PendingChangeAt = nil
	s.PendingPeriod = nil
}

// SetActivePaymentMethod records the payment method that future renewals
// should charge.
func (s *Subscription) SetActivePaymentMethod(id uuid.UUID) {
	s.ActivePaymentMethodID = &id
}

// MarkPaymentApplied records the payment whose effects are already reflected
// in the subscription state, so the same payment is never applied twice.
func (s *Subscription) MarkPaymentApplied(paymentID uuid.UUID) {
	s.LastAppliedPaymentID = &paymentID
}

// SetAutoRenew toggles automatic subscription renewal. Enabling auto-renew is
// only allowed when the subscription has a validity period.
func (s *Subscription) SetAutoRenew(enabled bool) error {
	if enabled && s.ValidUntil == nil {
		return ErrCannotEnableAutoRenew
	}
	s.AutoRenewEnabled = enabled
	return nil
}

// Cancel terminates the paid subscription at the end of the already paid period.
// The validity date is retained so the worker can downgrade the subscription to
// basic once the period expires. Auto-renew is disabled immediately.
func (s *Subscription) Cancel() error {
	if s.Status != SubscriptionStatusActive && s.Status != SubscriptionStatusGrace {
		return ErrInvalidSubscriptionState
	}
	s.Status = SubscriptionStatusCancelled
	s.AutoRenewEnabled = false
	return nil
}

// DowngradeToBasic resets the subscription to the free basic tariff. It clears
// any validity period, pending change and auto-renewal state.
func (s *Subscription) DowngradeToBasic(basicTariffID uuid.UUID) {
	s.TariffID = basicTariffID
	s.Status = SubscriptionStatusActive
	s.ValidUntil = nil
	s.AutoRenewEnabled = false
	s.PendingTariffID = nil
	s.PendingChangeAt = nil
	s.PendingPeriod = nil
}

// EnterGrace moves the subscription into the grace period. The validity date is
// extended to now+gracePeriod unless the subscription is already paid up beyond
// that point, so an already-paid period is never shortened.
func (s *Subscription) EnterGrace(now time.Time) {
	s.Status = SubscriptionStatusGrace
	graceUntil := now.Add(gracePeriod)
	if s.ValidUntil == nil || graceUntil.After(*s.ValidUntil) {
		s.ValidUntil = &graceUntil
	}
}

// addSubscriptionPeriod returns the time one subscription period after start.
func addSubscriptionPeriod(start time.Time, period SubscriptionPeriod) time.Time {
	if period == PeriodYear {
		return start.AddDate(1, 0, 0)
	}
	return start.AddDate(0, 1, 0)
}
