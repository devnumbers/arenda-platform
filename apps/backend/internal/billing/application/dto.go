package application

import (
	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// SubscriptionView is the read model of the user's current subscription with
// its tariffs and the active payment method resolved (issue #251).
type SubscriptionView struct {
	Subscription        domain.Subscription
	Tariff              domain.Tariff
	PendingTariff       *domain.Tariff
	ActivePaymentMethod *domain.PaymentMethod
}

// ChangeTariffRequest is the application-layer input of the tariff-change use
// case: the target plan by canonical name and the billing period to apply.
type ChangeTariffRequest struct {
	TariffName domain.TariffName
	Period     domain.SubscriptionPeriod
}

// ChangeTariffResult is the outcome of the tariff-change use case. The
// downgrade path schedules the change and returns zero values: no payment is
// involved. PaymentID and ConfirmURL are populated by the payment flow (issue
// #250) for upgrades and same-tariff grace renewals.
type ChangeTariffResult struct {
	PaymentID  uuid.UUID
	ConfirmURL string
}

// SubscriptionPaymentView is the read model of one subscription payment with
// its tariff resolved — the shape of GET /subscription/payments (issue #250).
type SubscriptionPaymentView struct {
	Payment domain.SubscriptionPayment
	Tariff  domain.Tariff
}

// AddPaymentMethodRequest is the application-layer input of the
// add-payment-method use case (issue #251). ProviderToken carries the raw
// charge token of synchronous providers (the fake); bank-form providers such
// as T-Kassa ignore it and run the binding-session flow instead.
type AddPaymentMethodRequest struct {
	ProviderToken string
}

// AddPaymentMethodResult carries either the created payment method (the
// synchronous token path) or the confirmation URL of the binding form the
// payer follows (the binding-session path). Exactly one of the two is set.
type AddPaymentMethodResult struct {
	ConfirmURL    string
	PaymentMethod *domain.PaymentMethod
}
