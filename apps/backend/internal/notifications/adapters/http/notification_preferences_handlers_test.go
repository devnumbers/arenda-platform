package http

import (
	"testing"

	notificationsdomain "github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

// TestChannelPreferencesFromRequest verifies that each per-event-type request
// item expands into exactly two domain entries (email + push) carrying the
// request's flags.
func TestChannelPreferencesFromRequest(t *testing.T) {
	items := []openapi.NotificationPreference{
		{
			EventType:    openapi.NotificationPreferenceEventType(notificationsdomain.EventOperationDue),
			Allowed:      false, // legacy field ignored on writes
			EmailAllowed: true,
			PushAllowed:  false,
		},
		{
			EventType:    openapi.NotificationPreferenceEventType(notificationsdomain.EventLeaseExpiring),
			Allowed:      true,
			EmailAllowed: false,
			PushAllowed:  true,
		},
	}

	prefs := channelPreferencesFromRequest(items)

	if len(prefs) != 4 {
		t.Fatalf("expected 4 domain entries (2 event types × 2 channels), got %d", len(prefs))
	}

	byKey := make(map[notificationsdomain.NotificationChannelPreference]bool, len(prefs))
	for _, p := range prefs {
		byKey[p] = true
	}

	assertTrue := func(eventType notificationsdomain.EventType, channel notificationsdomain.NotificationChannel, allowed bool) {
		t.Helper()
		entry := notificationsdomain.NotificationChannelPreference{EventType: eventType, Channel: channel, Allowed: allowed}
		if !byKey[entry] {
			t.Errorf("missing expected entry %+v", entry)
		}
	}

	assertTrue(notificationsdomain.EventOperationDue, notificationsdomain.ChannelEmail, true)
	assertTrue(notificationsdomain.EventOperationDue, notificationsdomain.ChannelPush, false)
	assertTrue(notificationsdomain.EventLeaseExpiring, notificationsdomain.ChannelEmail, false)
	assertTrue(notificationsdomain.EventLeaseExpiring, notificationsdomain.ChannelPush, true)
}

// TestNotificationChannelPreferencesResponse verifies that the per-channel
// domain slice collapses back into one response item per event type, with
// allowed == emailAllowed and the push flag carried through.
func TestNotificationChannelPreferencesResponse(t *testing.T) {
	// Build a full default set, then flip operation_due/email off and
	// lease_expiring/push off.
	prefs := notificationsdomain.DefaultNotificationChannelPreferences()
	for i := range prefs {
		if prefs[i].EventType == notificationsdomain.EventOperationDue && prefs[i].Channel == notificationsdomain.ChannelEmail {
			prefs[i].Allowed = false
		}
		if prefs[i].EventType == notificationsdomain.EventLeaseExpiring && prefs[i].Channel == notificationsdomain.ChannelPush {
			prefs[i].Allowed = false
		}
	}

	resp := notificationChannelPreferencesResponse(prefs)

	if len(resp.Preferences) != len(notificationsdomain.AllEventTypes()) {
		t.Fatalf("expected %d response items, got %d", len(notificationsdomain.AllEventTypes()), len(resp.Preferences))
	}

	byType := make(map[notificationsdomain.EventType]openapi.NotificationPreference, len(resp.Preferences))
	for _, p := range resp.Preferences {
		byType[notificationsdomain.EventType(p.EventType)] = p
	}

	due := byType[notificationsdomain.EventOperationDue]
	if due.EmailAllowed {
		t.Errorf("operation_due emailAllowed: expected false, got true")
	}
	if !due.PushAllowed {
		t.Errorf("operation_due pushAllowed: expected true, got false")
	}
	if due.Allowed != due.EmailAllowed {
		t.Errorf("operation_due allowed (%v) must equal emailAllowed (%v)", due.Allowed, due.EmailAllowed)
	}

	lease := byType[notificationsdomain.EventLeaseExpiring]
	if !lease.EmailAllowed {
		t.Errorf("lease_expiring emailAllowed: expected true, got false")
	}
	if lease.PushAllowed {
		t.Errorf("lease_expiring pushAllowed: expected false, got true")
	}
	if lease.Allowed != lease.EmailAllowed {
		t.Errorf("lease_expiring allowed (%v) must equal emailAllowed (%v)", lease.Allowed, lease.EmailAllowed)
	}
}

// TestNotificationChannelPreferencesResponse_RoundTrip verifies that expanding
// a response into domain prefs and collapsing back is stable for an untouched
// default set.
func TestNotificationChannelPreferencesResponse_RoundTrip(t *testing.T) {
	defaults := notificationsdomain.DefaultNotificationChannelPreferences()
	resp := notificationChannelPreferencesResponse(defaults)

	if len(resp.Preferences) != len(notificationsdomain.AllEventTypes()) {
		t.Fatalf("expected %d items, got %d", len(notificationsdomain.AllEventTypes()), len(resp.Preferences))
	}

	for _, p := range resp.Preferences {
		if !p.EmailAllowed || !p.PushAllowed || !p.Allowed {
			t.Errorf("expected all default flags true for %s (email=%v, push=%v, allowed=%v)",
				p.EventType, p.EmailAllowed, p.PushAllowed, p.Allowed)
		}
	}
}
