package application

import (
	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// ChangeTariffRequest asks to move a user to another tariff for a period.
type ChangeTariffRequest struct {
	TariffName string
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
