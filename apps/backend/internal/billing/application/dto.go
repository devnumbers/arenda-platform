package application

import (
	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// ChangeTariffRequest asks to move a user to another tariff for a period.
type ChangeTariffRequest struct {
	TariffName domain.TariffName
	Period     domain.SubscriptionPeriod
}

// ChangeTariffResponse returns the pending payment the user must confirm.
type ChangeTariffResponse struct {
	PaymentID  uuid.UUID
	ConfirmURL string
}

// AddPaymentMethodRequest carries a raw payment instrument token from the provider.
type AddPaymentMethodRequest struct {
	ProviderToken string
}

// AddPaymentMethodResponse is the result of adding a payment method.
// For providers that require confirmation (e.g. T-Kassa) ConfirmURL is set.
// For synchronous providers (e.g. fake) PaymentMethod is set.
type AddPaymentMethodResponse struct {
	ConfirmURL    string
	PaymentMethod *domain.PaymentMethod
}

// SubscriptionView is the current subscription together with its tariff and active payment method.
type SubscriptionView struct {
	Subscription        domain.Subscription
	Tariff              domain.Tariff
	PendingTariff       *domain.Tariff
	ActivePaymentMethod *domain.PaymentMethod
}

// SubscriptionPaymentView is a subscription payment together with its tariff.
type SubscriptionPaymentView struct {
	Payment domain.SubscriptionPayment
	Tariff  domain.Tariff
}

// SubscriptionPaymentWithUser is a subscription payment together with its tariff and the user's phone.
type SubscriptionPaymentWithUser struct {
	Payment   domain.SubscriptionPayment
	UserPhone string
}

// AdminSubscriptionPaymentView is a subscription payment with tariff and user phone for admin view.
type AdminSubscriptionPaymentView struct {
	Payment   domain.SubscriptionPayment
	Tariff    domain.Tariff
	UserPhone string
}

// ListAllPaymentsFilters carries optional filters for the admin list endpoint.
type ListAllPaymentsFilters struct {
	Status string
	UserID uuid.UUID
	Limit  int
	Offset int
}
