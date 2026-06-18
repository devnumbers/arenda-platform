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
	PendingPeriod         *SubscriptionPeriod
	ActivePaymentMethodID *uuid.UUID
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

// CanMutateData reports whether the subscription allows the user to mutate property and finance data.
func (s *Subscription) CanMutateData() bool {
	return s.Status == SubscriptionStatusActive || s.Status == SubscriptionStatusGrace
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
// period ends. It requires a valid_until date and enables auto-renew so the
// downgrade can be applied automatically. The new tariff must be a downgrade
// from the current tariff.
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
// new tariff, extends validity by the chosen period, enables auto-renew and
// clears any pending change.
func (s *Subscription) ApplyTariffChange(currentTariff, newTariff Tariff, period SubscriptionPeriod, now time.Time) error {
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
	s.PendingTariffID = nil
	s.PendingChangeAt = nil
	s.Status = SubscriptionStatusActive
	return nil
}

// ApplyRenewal extends the subscription validity by one period. The extension
// is calculated from the current valid_until when it exists and is in the
// future; otherwise it starts from now. A successful renewal also clears any
// scheduled downgrade.
func (s *Subscription) ApplyRenewal(period SubscriptionPeriod, now time.Time) error {
	if period != PeriodMonth && period != PeriodYear {
		return ErrInvalidPeriod
	}
	base := now
	if s.ValidUntil != nil && s.ValidUntil.After(base) {
		base = *s.ValidUntil
	}
	validUntil := addSubscriptionPeriod(base, period)
	s.ValidUntil = &validUntil
	s.Status = SubscriptionStatusActive
	s.PendingTariffID = nil
	s.PendingChangeAt = nil
	s.PendingPeriod = nil
	return nil
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

// addSubscriptionPeriod returns the time one subscription period after start.
func addSubscriptionPeriod(start time.Time, period SubscriptionPeriod) time.Time {
	if period == PeriodYear {
		return start.AddDate(1, 0, 0)
	}
	return start.AddDate(0, 1, 0)
}
