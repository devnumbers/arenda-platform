package application

import "errors"

var (
	// ErrNotFound is the generic repository miss; services narrow it to the
	// entity-specific sentinels below.
	ErrNotFound = errors.New("not found")
	// ErrTariffNotFound is returned when a tariff lookup misses.
	ErrTariffNotFound = errors.New("tariff not found")
	// ErrSubscriptionNotFound is returned when the user has no subscription
	// (yet). Consumers treat it as "no subscription" rather than an error:
	// the readonly gate allows mutations, /me omits the field.
	ErrSubscriptionNotFound = errors.New("subscription not found")
)
