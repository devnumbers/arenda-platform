package notificationsjob

import (
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// The grace events (issue #253, #741) route to the payment-methods screen —
// the one place the user fixes the failed renewal charge. The path is the
// legacy direct channel's path, preserved verbatim.
func TestDeeplinkForGrace(t *testing.T) {
	t.Parallel()

	for _, eventType := range []domain.EventType{
		domain.EventSubscriptionGraceEntered,
		domain.EventSubscriptionGraceExpiring,
	} {
		if got := deeplinkFor(eventType); got != graceNotificationPath {
			t.Errorf("deeplinkFor(%q) = %q, want %s", eventType, got, graceNotificationPath)
		}
	}

	// An event without an entry delivers without a link.
	if got := deeplinkFor(domain.EventSystemMaintenance); got != "" {
		t.Errorf("deeplinkFor(system_maintenance) = %q, want empty", got)
	}
}
