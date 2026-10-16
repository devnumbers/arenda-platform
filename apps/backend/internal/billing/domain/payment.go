package domain

// PaymentStatus is the lifecycle state of a subscription payment (ADR 0008,
// issue #248). The statuses match the subscription_payments status CHECK in
// migration 000104. Refunds are full-amount only (ADR 0037): the legacy
// partial_refunded status is gone, and refunding is the internal reserving
// state a payment passes through while a refund is in flight.
type PaymentStatus string

const (
	// PaymentStatusPending means the payment outcome is not final yet: the
	// user has not completed the provider form, or the provider reports a
	// transitional state.
	PaymentStatusPending PaymentStatus = "pending"
	// PaymentStatusSucceeded means the provider confirmed the charge.
	PaymentStatusSucceeded PaymentStatus = "succeeded"
	// PaymentStatusFailed means the provider rejected or cancelled the charge,
	// or the payment form deadline expired.
	PaymentStatusFailed PaymentStatus = "failed"
	// PaymentStatusRefunding is the internal reservation while a refund
	// request is in flight; it resolves to refunded or back to the previous
	// status.
	PaymentStatusRefunding PaymentStatus = "refunding"
	// PaymentStatusRefunded means the full amount was returned to the payer.
	PaymentStatusRefunded PaymentStatus = "refunded"
)

// PaymentProvider identifies which external payment processor handled a
// payment. Every payment carries the provider that created it, so historical
// payments stay interpretable across provider switches (ADR 0038). The values
// are the adapter identities declared by the provider adapters themselves;
// the domain deliberately declares no provider constants.
type PaymentProvider string
