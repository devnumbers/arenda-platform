package domain

import "errors"

var (
	// ErrInvalidPeriod is returned when a subscription period value is neither
	// "month" nor "year".
	ErrInvalidPeriod = errors.New("invalid subscription period")
	// ErrInvalidTariff is returned when a tariff name is not one of the known
	// tariff plans.
	ErrInvalidTariff = errors.New("invalid tariff")
	// ErrAlreadyOnTariff is returned when a tariff change requests the tariff
	// the subscription is already on.
	ErrAlreadyOnTariff = errors.New("subscription is already on the requested tariff")
	// ErrInvalidTariffChange is returned when the requested tariff change
	// direction or timing contradicts the subscription state (ADR 0008).
	ErrInvalidTariffChange = errors.New("invalid tariff change")
	// ErrInvalidSubscriptionState is returned when a transition is attempted
	// from a status that does not allow it.
	ErrInvalidSubscriptionState = errors.New("invalid subscription state")
	// ErrCannotEnableAutoRenew is returned when auto-renew is enabled on a
	// subscription without a validity period (the basic tariff never expires,
	// so there is nothing to renew).
	ErrCannotEnableAutoRenew = errors.New("cannot enable auto renew without a validity period")
	// ErrInvalidTransition is returned when a subscription transition record is
	// built from an incomplete or inconsistent subscription state.
	ErrInvalidTransition = errors.New("invalid subscription transition")
	// ErrInvalidAmount is returned when a payment amount is not a positive
	// integer number of kopecks.
	ErrInvalidAmount = errors.New("invalid payment amount")
	// ErrInvalidPayment is returned when a payment is built from incomplete or
	// inconsistent identity data.
	ErrInvalidPayment = errors.New("invalid subscription payment")
	// ErrInvalidPaymentStatus is returned when a payment transition is
	// attempted from a status that does not allow it.
	ErrInvalidPaymentStatus = errors.New("invalid payment status transition")
)
