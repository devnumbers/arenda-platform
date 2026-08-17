package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// The payment-provider port (issue #248, strengthening ADR 0007): the single
// seam between the billing application layer and the external payment
// processor. Everything on this side of the seam is provider-neutral —
// amounts are integer kopecks (ADR 0036), identities are internal UUIDs plus
// an opaque provider-scoped customer reference, and every provider-specific
// concern (wire formats, signatures, callbacks, redirect forms, status and
// error vocabularies) lives inside the adapters that implement it. Exactly
// one provider adapter is active per environment (ADR 0038).

// Initiator classifies who initiated a payment operation. Providers route
// operations differently depending on the initiator, so every payment the
// application starts carries one explicitly; the empty value is invalid.
type Initiator string

const (
	// InitiatorCustomer marks a customer-initiated transaction (CIT): the
	// payer completes the payment themselves on the provider's form. This is
	// the parent payment of a save-method chain — when SaveMethod is set,
	// the provider makes the credentials reusable for later charges.
	InitiatorCustomer Initiator = "cit"
	// InitiatorMerchant marks a merchant-initiated transaction (MIT): a
	// recurring charge against previously saved credentials without payer
	// interaction, used by the renewal worker.
	InitiatorMerchant Initiator = "mit"
)

// Valid reports whether the initiator is one of the defined values.
func (i Initiator) Valid() bool {
	switch i {
	case InitiatorCustomer, InitiatorMerchant:
		return true
	}
	return false
}

// PaymentPurposeKind selects the wording of a payment purpose.
type PaymentPurposeKind string

const (
	// PaymentPurposeSubscription is the first payment for a tariff change.
	PaymentPurposeSubscription PaymentPurposeKind = "subscription_payment"
	// PaymentPurposeRenewal is an automatic or manual renewal of the
	// current tariff.
	PaymentPurposeRenewal PaymentPurposeKind = "subscription_renewal"
)

// PaymentPurpose describes in structured form what a subscription payment
// buys. The provider adapter renders it into the human-readable payment
// description shown to the payer — including the language and the provider's
// length limit — which is why the application never builds the description
// string itself.
type PaymentPurpose struct {
	Kind       PaymentPurposeKind
	TariffName domain.TariffName
	Period     domain.SubscriptionPeriod
}

// SavedMethod is a payment method saved at the provider, described in
// provider-neutral terms: the provider's own method identifier, the token
// later charges are made against, and display fields for the method list.
type SavedMethod struct {
	// ProviderMethodID identifies the saved method at the provider; it is
	// the handle for removing the method later.
	ProviderMethodID string
	// ChargeToken is the token merchant-initiated charges are made against.
	ChargeToken string
	// MaskedPan is the masked card number for display, e.g.
	// "4300********1234".
	MaskedPan string
	// ExpDate is the card expiry in the provider's display format (MMYY).
	ExpDate string
	// CustomerRef is the provider-scoped customer the method belongs to.
	CustomerRef string
}

// InitPaymentRequest starts a payment at the provider.
type InitPaymentRequest struct {
	// PaymentID is the internal subscription-payment identifier; providers
	// that need a merchant-side order identifier use this UUID verbatim.
	PaymentID uuid.UUID
	// AmountKopecks is the payment amount in kopecks.
	AmountKopecks int64
	// Period is the subscription period the payment buys.
	Period domain.SubscriptionPeriod
	// CustomerRef is the provider-scoped customer reference (an opaque
	// string from the application's point of view).
	CustomerRef string
	// Purpose describes what the payment is for, in structured form.
	Purpose PaymentPurpose
	// SaveMethod asks the provider to make the payment credentials
	// reusable for later merchant-initiated charges. It is only meaningful
	// together with InitiatorCustomer: a merchant-initiated charge runs on
	// credentials saved by an earlier customer-initiated payment.
	SaveMethod bool
	// Initiator explicitly declares who initiates the operation. It is
	// mandatory; the adapter rejects requests without it.
	Initiator Initiator
	// FormDeadline bounds how long the provider's payment form stays
	// usable. The zero value lets the provider default apply.
	FormDeadline time.Time
}

// InitPaymentResult is the provider's response to a started payment.
type InitPaymentResult struct {
	// ProviderPaymentID is the provider's identifier of the created
	// payment; refunds and status queries address it.
	ProviderPaymentID string
	// PaymentURL is the URL the payer follows to complete the payment.
	// Merchant-initiated payments need no payer interaction and return an
	// empty URL.
	PaymentURL string
	// Status is the provider-side status of the freshly created payment
	// (pending unless the provider already knows better).
	Status domain.PaymentStatus
	// SavedMethod is set when the provider synchronously created a saved
	// payment method with this payment.
	SavedMethod *SavedMethod
}

// ChargeRequest completes a merchant-initiated payment on saved credentials.
// The amount is the amount the payment was initialized with; some providers
// do not accept it on the charge call itself but the port keeps the request
// self-contained.
type ChargeRequest struct {
	PaymentID         uuid.UUID
	ProviderPaymentID string
	AmountKopecks     int64
	// ChargeToken is the charge token of the saved payment method
	// (SavedMethod.ChargeToken).
	ChargeToken string
}

// ChargeResult reports the provider's response to a charge.
type ChargeResult struct {
	ProviderPaymentID string
	Status            domain.PaymentStatus
	// ErrorCode is the provider error code reported for a failed charge;
	// empty on success or when the provider does not report one.
	ErrorCode string
}

// PaymentStatusResult is the provider-side state of a payment, used to
// reconcile payments whose webhook was lost.
type PaymentStatusResult struct {
	Status domain.PaymentStatus
	// ChargeToken is the saved-method charge token the provider reports for
	// this payment, when it reports one; it recovers tokens whose delivery
	// notification was lost.
	ChargeToken string
	// ErrorCode is the provider error code reported for a failed payment;
	// empty on success or when the provider does not report one.
	ErrorCode string
}

// RefundRequest asks the provider to return the full payment amount
// (refunds are always full-amount, ADR 0037).
type RefundRequest struct {
	PaymentID         uuid.UUID
	ProviderPaymentID string
	AmountKopecks     int64
}

// RefundResult reports the provider's response to a refund request.
type RefundResult struct {
	ProviderPaymentID     string
	Status                domain.PaymentStatus
	RefundedAmountKopecks int64
}

// WebhookEvent is the neutral form of one incoming provider notification.
// Exactly one of the kind-specific payloads is set.
type WebhookEvent struct {
	// Payment is set for payment-status notifications.
	Payment *PaymentNotification
	// MethodBound is set for saved-method binding notifications.
	MethodBound *MethodBoundNotification
}

// PaymentNotification reports a change in a payment's provider-side status.
type PaymentNotification struct {
	// InternalPaymentID is the internal payment the notification resolves
	// to (the provider echoes the merchant-side identifier).
	InternalPaymentID uuid.UUID
	ProviderPaymentID string
	Status            domain.PaymentStatus
	// ErrorCode is the provider code carried by failed notifications.
	ErrorCode *string
	// AmountKopecks is the payment amount for payment notifications.
	AmountKopecks int64
	// SavedMethod is set when the notification also produced a saved
	// payment method — the token delivery moment of a save-method chain.
	SavedMethod *SavedMethod
}

// MethodBoundNotification reports a completed payment-method binding.
type MethodBoundNotification struct {
	// BindingID links the notification to the binding session started by
	// BindPaymentMethod.
	BindingID string
	Method    SavedMethod
}

// MethodBindingStatus is the neutral state of a payment-method binding
// session. Providers expose richer intermediate states (challenge checks,
// authorization steps); the port collapses them into pending, because the
// application cannot act on anything finer.
type MethodBindingStatus string

const (
	// MethodBindingPending means the binding session has not finished.
	MethodBindingPending MethodBindingStatus = "pending"
	// MethodBindingCompleted means the method was bound and is chargeable.
	MethodBindingCompleted MethodBindingStatus = "completed"
	// MethodBindingFailed means the method could not be bound.
	MethodBindingFailed MethodBindingStatus = "failed"
)

// MethodBindingState is the provider-side state of a binding session.
type MethodBindingState struct {
	Status MethodBindingStatus
	// Method is set once the binding completed; it carries the new saved
	// method with its charge token.
	Method *SavedMethod
	// ErrorCode is the provider code for failed bindings; empty otherwise.
	ErrorCode string
}

// BindMethodRequest starts a payment-method binding session: the payer is
// redirected to the provider's form to attach a new method. The verification
// strategy (challenge checks, zero-amount holds) is a provider decision.
type BindMethodRequest struct {
	// CustomerRef is the provider-scoped customer reference the method is
	// bound to.
	CustomerRef string
}

// BindMethodResult is the provider's response to a binding start.
type BindMethodResult struct {
	// FormURL is the provider's binding form the payer follows.
	FormURL string
	// BindingID identifies the binding session for later state polling and
	// for matching binding notifications.
	BindingID string
}

// PaymentInitiator starts payments at the provider, both customer-initiated
// (redirect flow) and merchant-initiated (charge setup).
type PaymentInitiator interface {
	InitPayment(ctx context.Context, req InitPaymentRequest) (InitPaymentResult, error)
}

// PaymentCharger completes merchant-initiated payments on saved credentials.
type PaymentCharger interface {
	ChargePayment(ctx context.Context, req ChargeRequest) (ChargeResult, error)
}

// PaymentStatusReader queries the current provider-side status of a payment.
type PaymentStatusReader interface {
	PaymentStatus(ctx context.Context, paymentID uuid.UUID, providerPaymentID string) (PaymentStatusResult, error)
}

// PaymentRefunder returns the full amount of a finalized payment.
type PaymentRefunder interface {
	RefundPayment(ctx context.Context, req RefundRequest) (RefundResult, error)
}

// WebhookParser parses and verifies an incoming provider webhook payload.
// Verification failures are errors; the caller must answer the webhook
// accordingly.
type WebhookParser interface {
	ParseWebhook(ctx context.Context, payload []byte) (WebhookEvent, error)
}

// WebhookResponder returns the fixed success body the provider expects as a
// webhook acknowledgement.
type WebhookResponder interface {
	WebhookAck() []byte
}

// PaymentMethodBinder starts payment-method binding sessions.
type PaymentMethodBinder interface {
	BindPaymentMethod(ctx context.Context, req BindMethodRequest) (BindMethodResult, error)
}

// PaymentMethodBindingReader queries the state of a binding session started
// by BindPaymentMethod.
type PaymentMethodBindingReader interface {
	PaymentMethodBinding(ctx context.Context, bindingID string) (MethodBindingState, error)
}

// PaymentMethodRemover detaches a saved method from its customer at the
// provider.
type PaymentMethodRemover interface {
	RemovePaymentMethod(ctx context.Context, customerRef, providerMethodID string) error
}

// PaymentMethodLister lists the chargeable methods saved for a customer at
// the provider.
type PaymentMethodLister interface {
	ListPaymentMethods(ctx context.Context, customerRef string) ([]SavedMethod, error)
}

// ProviderNamer reveals the adapter's provider identity — the value payments
// persist as their provider (ADR 0038).
type ProviderNamer interface {
	Name() domain.PaymentProvider
}

// PaymentProvider is the aggregate outbound port to the payment processor:
// every capability the billing flows consume, plus the provider identity.
// Consumers that need only a subset depend on the narrow capability
// interface directly.
type PaymentProvider interface {
	PaymentInitiator
	PaymentCharger
	PaymentStatusReader
	PaymentRefunder
	WebhookParser
	WebhookResponder
	PaymentMethodBinder
	PaymentMethodBindingReader
	PaymentMethodRemover
	PaymentMethodLister
	ProviderNamer
}
