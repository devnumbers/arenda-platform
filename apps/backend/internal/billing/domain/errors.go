package domain

import "errors"

var (
	// ErrInvalidAmount indicates a negative amount was supplied for a payment.
	ErrInvalidAmount = errors.New("amount must be non-negative")
	// ErrInvalidPeriod indicates a subscription period other than month/year was supplied.
	ErrInvalidPeriod = errors.New("period must be month or year")
	// ErrInvalidPaymentStatus indicates a status transition that is not allowed.
	ErrInvalidPaymentStatus = errors.New("payment status transition is invalid")
)
