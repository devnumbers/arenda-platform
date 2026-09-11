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
		Source:           openapi.SubscriptionSource(sub.Source),
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
	if view.PendingPayment != nil {
		pending := view.PendingPayment
		pp := openapi.SubscriptionPendingPayment{
			TariffName:    openapi.TariffName(pending.Tariff.Name),
			Period:        openapi.AdminSubscriptionPaymentPeriod(pending.Payment.Period),
			AmountKopecks: pending.Payment.AmountKopecks,
		}
		if pending.Payment.ExpiresAt != nil {
			pp.ExpiresAt = *pending.Payment.ExpiresAt
		}
		if pending.Payment.HasPaymentURL() {
			pp.ConfirmUrl = *pending.Payment.PaymentURL
		}
		resp.PendingPayment = &pp
	}
	if view.ActivePaymentMethod != nil {
		method := PaymentMethodResponse(*view.ActivePaymentMethod)
		resp.ActivePaymentMethod = &method
	}
	return resp
}

// PaymentMethodResponse maps a billing PaymentMethod domain value to the
// OpenAPI PaymentMethod DTO (issue #251): display fields only — the charge
// token stays server-side. The card system derives from the display mask's
// BIN prefix (issue #619).
func PaymentMethodResponse(m domain.PaymentMethod) openapi.PaymentMethod {
	resp := openapi.PaymentMethod{
		Id:          m.ID,
		Provider:    string(m.Provider),
		DisplayMask: m.DisplayMask,
		CardSystem:  openapi.CardSystem(domain.CardSystemFromMask(m.DisplayMask)),
		IsActive:    m.IsActive,
		CreatedAt:   m.CreatedAt,
	}
	if m.ExpDate != "" {
		resp.ExpDate = &m.ExpDate
	}
	return resp
}

// SubscriptionPaymentCardResponse maps a payment's resolved card mask to the
// contract DTO (issue #619); nil when no card is known for the payment.
func SubscriptionPaymentCardResponse(cardMask *string) *openapi.SubscriptionPaymentCard {
	if cardMask == nil || *cardMask == "" {
		return nil
	}
	return &openapi.SubscriptionPaymentCard{
		DisplayMask: *cardMask,
		CardSystem:  openapi.CardSystem(domain.CardSystemFromMask(*cardMask)),
	}
}

// TariffResponse maps a billing Tariff domain value to the OpenAPI Tariff DTO.
// Exported so the billing and identity HTTP adapters share a single mapper.
func TariffResponse(t domain.Tariff) openapi.Tariff {
	return openapi.Tariff{
		Name:                openapi.TariffName(t.Name),
		ActivePropertyLimit: t.ActivePropertyLimit,
		MonthlyPriceKopecks: t.MonthlyPriceKopecks,
		YearlyPriceKopecks:  t.YearlyPriceKopecks,
	}
}

// AdminTariffResponse maps a billing Tariff domain value to the OpenAPI
// AdminTariff DTO: the user-facing shape plus the id and the active flag the
// admin tariff screen needs (issue #247).
func AdminTariffResponse(t domain.Tariff) openapi.AdminTariff {
	return openapi.AdminTariff{
		Id:                  t.ID,
		Name:                openapi.TariffName(t.Name),
		IsActive:            t.IsActive,
		ActivePropertyLimit: t.ActivePropertyLimit,
		MonthlyPriceKopecks: t.MonthlyPriceKopecks,
		YearlyPriceKopecks:  t.YearlyPriceKopecks,
	}
}
