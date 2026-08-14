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
	// ErrPaymentUnavailable is the explicit temporary error for the flows
	// that require a payment until the payment ticket lands (issue #249:
	// upgrades and same-tariff grace renewals answer with it; issue #250
	// replaces it with the real payment initiation).
	ErrPaymentUnavailable = errors.New("payment unavailable")

	// Provider sentinels classify provider outcomes the application acts
	// on beyond success/failure. They are provider-neutral by contract
	// (issue #248): each adapter maps its own wire vocabulary onto them,
	// wrapping its detailed error so the code stays inspectable.

	// ErrProviderMethodNotFound is returned when the provider reports the
	// saved payment method does not exist (already removed or unknown).
	ErrProviderMethodNotFound = errors.New("provider payment method not found")
	// ErrProviderCustomerNotFound is returned when the provider reports the
	// customer does not exist yet. For read operations such as listing
	// methods it semantically means the user has no saved methods, not a
	// failure.
	ErrProviderCustomerNotFound = errors.New("provider customer not found")
	// ErrProviderAccountNotFound is returned when the provider reports the
	// configured account does not exist (for example a deleted or wrong
	// credentials set). For method-list syncing it is treated as "no methods
	// at the provider", not a hard failure.
	ErrProviderAccountNotFound = errors.New("provider account not found")
	// ErrProviderChargeBlocked is returned when the provider rejects
	// merchant-initiated charges because charging is disabled or
	// saved-credential charging is not enabled for the account.
	ErrProviderChargeBlocked = errors.New("provider charge blocked")
	// ErrProviderAuthRejected is returned when the provider rejects the
	// request's authentication — a signing or credentials misconfiguration
	// that makes every request fail.
	ErrProviderAuthRejected = errors.New("provider auth rejected")
	// ErrProviderPaymentNotFound is returned when the provider does not know
	// the payment being operated on.
	ErrProviderPaymentNotFound = errors.New("provider payment not found")
	// ErrProviderInvalidOperation is returned when the provider rejects the
	// operation parameters as inconsistent or malformed — an integration
	// defect (bad parameter combination, malformed deadline, missing
	// callback URL), not a payment outcome.
	ErrProviderInvalidOperation = errors.New("provider invalid operation")
	// ErrProviderInsufficientFunds is returned when the charge failed
	// because the payer's account lacks funds. It is the main driver of
	// grace-period communication: the remedy is a later retry or another
	// payment method, not a re-bind.
	ErrProviderInsufficientFunds = errors.New("provider insufficient funds")
	// ErrProviderRecurringFailed is returned when a merchant-initiated
	// charge on saved credentials failed at the provider's side without a
	// more specific classification. The remedy is a retry or another
	// payment method.
	ErrProviderRecurringFailed = errors.New("provider recurring charge failed")
	// ErrProviderSavedMethodExpired is returned when the saved payment
	// method can no longer be charged because its underlying mandate has
	// expired at the provider. The remedy is a fresh binding.
	ErrProviderSavedMethodExpired = errors.New("provider saved method expired")
	// ErrProviderDuplicateOperation is returned when the provider reports
	// the requested state transition was already performed (for example a
	// refund repeated after a network duplicate). The transition stands; the
	// caller should reconcile the current state instead of retrying.
	ErrProviderDuplicateOperation = errors.New("provider duplicate operation")
)
