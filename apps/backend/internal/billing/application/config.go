package application

import "time"

// Config bundles the billing module's operational parameters in one place
// (issue #244). Defaults are code constants — product behaviour, not
// deployment knobs — so no new environment variables are introduced. Wire
// builds DefaultConfig and passes it to the module; later tickets (payments
// #250, card binding #251, workers #252) consume the fields instead of
// re-introducing magic numbers.
type Config struct {
	// GraceDuration is how long a subscription stays in grace after a failed
	// renewal charge (ADR 0008: 7 days).
	GraceDuration time.Duration
	// PaymentFormTTL bounds how long a user-facing payment form stays usable
	// (the provider's redirect due date).
	PaymentFormTTL time.Duration
	// ChargeAttemptLimit caps how many times a renewal charge may fail with an
	// unresolved provider outcome before the payment is marked failed and the
	// subscription enters grace.
	ChargeAttemptLimit int
	// WorkerBatchSize is the batch size of every billing-worker phase.
	WorkerBatchSize int
	// CardBindingTTL is how long a card-binding session stays resolvable after
	// initiation (issue #251); expired sessions never produce payment methods.
	CardBindingTTL time.Duration
	// PendingPaymentStaleness is how long a pending payment may linger before
	// the reconciliation worker asks the provider for its status.
	PendingPaymentStaleness time.Duration
}

// DefaultConfig returns the billing operational parameters with their
// production defaults (ADR 0008; values carried over from the pre-rewrite
// module constants).
func DefaultConfig() Config {
	return Config{
		GraceDuration:           7 * 24 * time.Hour,
		PaymentFormTTL:          15 * time.Minute,
		ChargeAttemptLimit:      3,
		WorkerBatchSize:         100,
		CardBindingTTL:          24 * time.Hour,
		PendingPaymentStaleness: 5 * time.Minute,
	}
}
