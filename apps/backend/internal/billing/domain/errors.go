package domain

import "errors"

var (
	// ErrInvalidAmount indicates an invalid (zero or negative) amount was supplied for a payment.
	ErrInvalidAmount = errors.New("amount must be positive")
	// ErrInvalidPeriod indicates a subscription period other than month/year was supplied.
	ErrInvalidPeriod = errors.New("period must be month or year")
	// ErrInvalidPaymentStatus indicates a status transition that is not allowed.
	ErrInvalidPaymentStatus = errors.New("payment status transition is invalid")
	// ErrAlreadyOnTariff indicates an attempt to change to the current tariff.
	ErrAlreadyOnTariff = errors.New("already on selected tariff")
	// ErrInvalidTariffChange indicates a tariff change that violates domain rules.
	ErrInvalidTariffChange = errors.New("invalid tariff change")
	// ErrInvalidSubscriptionState indicates a subscription state transition that is not allowed.
	ErrInvalidSubscriptionState = errors.New("invalid subscription state")
	// ErrCannotEnableAutoRenew indicates an attempt to enable auto-renew for a subscription without a validity period.
	ErrCannotEnableAutoRenew = errors.New("cannot enable auto-renew without a validity period")
)
