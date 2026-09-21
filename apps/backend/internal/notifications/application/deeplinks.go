package application

import (
	"fmt"

	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// graceNotificationPath is the payment-methods screen — the one place the
// user fixes a failed renewal charge; both grace events route here (#741).
const graceNotificationPath = "/profile/tariff/payment-methods"

// tariffPath is the subscription screen — the plan the user pays for, the
// place a payment or a plan change answers about (#752). The payment
// succeeded event carries no catalog action (решение #737 №13), its push tap
// and email button still land somewhere sensible; the plan changed event's
// action open_tariffs resolves to the same screen (#743).
const tariffPath = "/profile/tariff"

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
	// The subscription screen answers both tariff events (#752): the
	// payment's outcome and the plan's state live there.
	domain.EventSubscriptionPaymentSucceeded: staticPath(tariffPath),
	domain.EventSubscriptionPlanChanged:      staticPath(tariffPath),
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
	// The task's edit screen is its rule's screen (#750): a property-bound
	// task navigates through the property, the task without a property —
	// through the flat tasks route (ADR 0052); the ids are exactly what the
	// publisher put in the payload.
	domain.EventTaskOverdue: taskPath,
}

// taskPath builds the task's rule-edit path from the payload: the property
// snapshot's id and the rule id (Payload.TaskRuleID), or the rule id alone
// for the task without a property.
func taskPath(p domain.Payload) string {
	if p.TaskRuleID == nil {
		return ""
	}
	if p.Property != nil {
		return fmt.Sprintf("/properties/%s/tasks/%s/edit", p.Property.ID, *p.TaskRuleID)
	}
	return fmt.Sprintf("/tasks/%s/edit", *p.TaskRuleID)
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
