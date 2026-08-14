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

// AdminPaymentFilters carries the optional filters of the admin payment
// listing (issue #254). Status and SubscriptionStatus are validated against
// the domain vocabularies; Sort against the endpoint whitelist; the repository
// layer encrypts the phone filter.
type AdminPaymentFilters struct {
	UserID             *uuid.UUID
	Status             string
	UserPhone          string
	SubscriptionStatus string
	Sort               string
	Order              string
	Limit              int
	Offset             int
}

// AdminPaymentRow is one row of the admin payment listing as the persistence
// adapter produces it: the payment aggregate plus the payer's phone resolved
// from identity (decrypted when stored as ciphertext).
type AdminPaymentRow struct {
	Payment   domain.SubscriptionPayment
	UserPhone string
}

// AdminSubscriptionPaymentView is the read model of one subscription payment
// for the admin views (issue #254): the payment with its tariff resolved and
// the payer's phone.
type AdminSubscriptionPaymentView struct {
	Payment   domain.SubscriptionPayment
	Tariff    domain.Tariff
	UserPhone string
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
