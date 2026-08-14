package tkassa

import (
	"strings"
	"unicode/utf8"

	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// maxDescriptionLength is the T-Kassa Description limit for card and
// credentials-on-file payments: 140 characters.
const maxDescriptionLength = 140

// paymentDescription renders the structured payment purpose into the
// human-readable Russian description shown to the payer (issue #244). The
// wording lives here — in the adapter — so the application layer stays free
// of provider formatting rules, and each provider can truncate natively.
// Unknown purpose kinds fall back to the subscription wording and unknown
// periods to "месяц": the application layer validates purpose fields before
// they reach the port, and rendering a description must never fail a payment.
func paymentDescription(purpose application.PaymentPurpose) string {
	var kind string
	switch purpose.Kind {
	case application.PaymentPurposeRenewal:
		kind = "Продление подписки"
	default:
		kind = "Оплата подписки"
	}
	return kind + " " + tariffDisplayName(purpose.TariffName) + " (" + periodDisplayName(purpose.Period) + ")"
}

// tariffDisplayName capitalizes the machine tariff name for display
// ("pro" → "Pro").
func tariffDisplayName(name domain.TariffName) string {
	runes := []rune(string(name))
	if len(runes) == 0 {
		return ""
	}
	return strings.ToUpper(string(runes[0])) + string(runes[1:])
}

// periodDisplayName renders a subscription period in Russian.
func periodDisplayName(period domain.SubscriptionPeriod) string {
	switch period {
	case domain.PeriodYear:
		return "год"
	default:
		return "месяц"
	}
}

// truncateDescription shortens s to at most maxDescriptionLength characters,
// counting runes and not bytes: Russian descriptions are UTF-8 and a
// byte-based cut would split a rune in half and send an invalid string
// (issue #246 §2.1).
func truncateDescription(s string) string {
	if utf8.RuneCountInString(s) <= maxDescriptionLength {
		return s
	}
	return string([]rune(s)[:maxDescriptionLength])
}
