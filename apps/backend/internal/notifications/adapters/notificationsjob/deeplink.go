package notificationsjob

import (
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// deeplinks maps a feed event type to the in-app path its email button and
// push tap navigate to. This is the publishers' channel vocabulary (#740):
// #741 ports the grace path (/profile/tariff/payment-methods) here, the
// catalog publishers (#748–#752) add theirs per event type. An event without
// an entry delivers without a link — the push shows without a tap target and
// the email renders without a button.
var deeplinks = map[domain.EventType]string{}

// deeplinkFor returns the event type's deep link, empty when none is mapped.
func deeplinkFor(eventType domain.EventType) string {
	return deeplinks[eventType]
}
