package application

import (
	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// SubscriptionView is the read model of the user's current subscription with
// its tariffs resolved. The active payment method joins the view again with
// the payment-methods ticket (#251); until then it is always nil.
type SubscriptionView struct {
	Subscription  domain.Subscription
	Tariff        domain.Tariff
	PendingTariff *domain.Tariff
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
