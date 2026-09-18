package notificationsjob

import (
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// graceNotificationPath is the payment-methods screen — the one place the
// user fixes a failed renewal charge; both grace events route here (#741).
const graceNotificationPath = "/profile/tariff/payment-methods"

// deeplinks maps a feed event type to the in-app path its email button and
// push tap navigate to. This is the publishers' channel vocabulary (#740):
// the grace path landed with #741, the catalog publishers (#748–#752) add
// theirs per event type. An event without an entry delivers without a link —
// the push shows without a tap target and the email renders without a button.
// The SSE stream's created frame (#742) carries the same link.
var deeplinks = map[domain.EventType]string{
	domain.EventSubscriptionGraceEntered:  graceNotificationPath,
	domain.EventSubscriptionGraceExpiring: graceNotificationPath,
}

// DeepLinkFor returns the event type's deep link, empty when none is mapped.
// Exported for the stream adapter: the email button, the push tap and the
// SSE toast share one deep-link vocabulary.
func DeepLinkFor(eventType domain.EventType) string {
	return deeplinks[eventType]
}
