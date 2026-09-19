package notificationsjob

import (
	"fmt"

	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// graceNotificationPath is the payment-methods screen — the one place the
// user fixes a failed renewal charge; both grace events route here (#741).
const graceNotificationPath = "/profile/tariff/payment-methods"

// deeplinks maps a feed event type to the in-app path its email button and
// push tap navigate to, resolved from the row's payload. This is the
// publishers' channel vocabulary (#740): the grace path landed with #741,
// the catalog publishers (#748–#752) add theirs per event type. An event
// without an entry — or with a payload missing the ids the path needs —
// delivers without a link: the push shows without a tap target and the
// email renders without a button. The SSE stream's created frame (#742)
// carries the same link.
var deeplinks = map[domain.EventType]func(domain.Payload) string{
	domain.EventSubscriptionGraceEntered:  staticPath(graceNotificationPath),
	domain.EventSubscriptionGraceExpiring: staticPath(graceNotificationPath),
	// The rental's screen path carries both the property and the rental
	// (#748); the snapshot ids are exactly what the publisher put there.
	domain.EventRentalCompleted: func(p domain.Payload) string {
		if p.Property == nil || p.RentalID == nil {
			return ""
		}
		return fmt.Sprintf("/properties/%s/rentals/%s", p.Property.ID, p.RentalID)
	},
	// The payment's page path carries the property and the rule (#749);
	// the snapshot ids are exactly what the publisher put there.
	domain.EventPaymentDue:     paymentPath,
	domain.EventPaymentOverdue: paymentPath,
}

// paymentPath builds the payment page path from the payload: the property
// snapshot's id and the rule's id (Payload.PaymentID).
func paymentPath(p domain.Payload) string {
	if p.Property == nil || p.PaymentID == nil {
		return ""
	}
	return fmt.Sprintf("/properties/%s/payments/%s", p.Property.ID, p.PaymentID)
}

// staticPath adapts a fixed path to the payload resolver signature.
func staticPath(path string) func(domain.Payload) string {
	return func(domain.Payload) string { return path }
}

// DeepLinkFor returns the notification's deep link, empty when none applies.
// Exported for the stream adapter: the email button, the push tap and the
// SSE toast share one deep-link vocabulary.
func DeepLinkFor(n domain.Notification) string {
	if resolve, ok := deeplinks[n.EventType]; ok {
		return resolve(n.Payload)
	}
	return ""
}
