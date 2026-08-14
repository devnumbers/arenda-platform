package application

import (
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
