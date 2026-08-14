package httpsupport

import (
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

// SubscriptionResponse maps a billing SubscriptionView to the OpenAPI
// Subscription DTO. It is a pure transport-edge mapper shared by the identity,
// billing, and admin handler packages, so it lives in the shared support
// package rather than any single domain's HTTP adapter.
func SubscriptionResponse(view billingapp.SubscriptionView) openapi.Subscription {
	sub := view.Subscription
	resp := openapi.Subscription{
		Tariff:           TariffResponse(view.Tariff),
		Status:           openapi.SubscriptionStatus(sub.Status),
		ValidUntil:       sub.ValidUntil,
		AutoRenewEnabled: sub.AutoRenewEnabled,
		PendingChangeAt:  sub.PendingChangeAt,
	}
	if sub.CurrentPeriod != nil {
		period := openapi.AdminSubscriptionPaymentPeriod(*sub.CurrentPeriod)
		resp.CurrentPeriod = &period
	}
	if view.PendingTariff != nil {
		pt := TariffResponse(*view.PendingTariff)
		resp.PendingTariff = &pt
	}
	if sub.PendingPeriod != nil {
		period := openapi.AdminSubscriptionPaymentPeriod(*sub.PendingPeriod)
		resp.PendingPeriod = &period
	}
	return resp
}

// TariffResponse maps a billing Tariff domain value to the OpenAPI Tariff DTO.
// Exported so the billing and identity HTTP adapters share a single mapper.
func TariffResponse(t domain.Tariff) openapi.Tariff {
	return openapi.Tariff{
		Name:                openapi.TariffName(t.Name),
		ActivePropertyLimit: t.ActivePropertyLimit,
		MonthlyPriceKopecks: int(t.MonthlyPriceKopecks),
		YearlyPriceKopecks:  int(t.YearlyPriceKopecks),
	}
}
