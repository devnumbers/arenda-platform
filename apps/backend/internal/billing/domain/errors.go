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
	// ErrResumeNotAvailable is returned when the free resume of a cancelled
	// subscription is impossible (issue #617): the subscription is not in the
	// cancelled state, or its paid period has already expired — restoration
	// of an expired period goes through paying for a tariff (ADR 0008).
	ErrResumeNotAvailable = errors.New("resume not available")
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
	// ErrInvalidTerm is returned when an admin service-subscription term is
	// inconsistent: an explicit until date in the past (issue #255).
	ErrInvalidTerm = errors.New("invalid service subscription term")
	// ErrInvalidGraceExtension is returned when an admin grace extension adds
	// no time or more than the operational cap allows (issue #255).
	ErrInvalidGraceExtension = errors.New("invalid grace extension")
	// ErrInvalidTimeShift is returned when the stand-only time-travel shift
	// (issue #665) is asked for a zero delta or a subscription without a
	// validity boundary — there is no time to travel on the free basic state.
	ErrInvalidTimeShift = errors.New("invalid subscription time shift")
	// ErrInvalidTariffPricing is returned when a tariff's admin-editable
	// fields break their invariants: a negative price or a property limit
	// below -1 (issue #256).
	ErrInvalidTariffPricing = errors.New("invalid tariff pricing")
)
