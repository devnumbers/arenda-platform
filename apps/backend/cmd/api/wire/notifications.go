package wire

import (
	notificationspg "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/postgres"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	paymentspg "github.com/nambers/arenda-planform/apps/backend/internal/payments/adapters/postgres"
)

// Notifications holds the notifications module's repositories and services
// wired by WireNotifications. The module owns the delivery channels, the
// push subscriptions, the stored feed and the delivery queue's inputs
// (#740); the grace events publish through the pipeline since #741; the
// feed's reading API and the per-category settings serve the REST contract
// since #743.
type Notifications struct {
	PushSubscriptionRepo    *notificationspg.PushSubscriptionRepository
	PushSubscriptionService *notificationsapp.PushSubscriptionService
	// NotificationRepo is the stored feed repository (решение #737); the
	// delivery workers and the publisher read and write it.
	NotificationRepo *notificationspg.NotificationRepository
	// FeedService is the reading side of the feed (#743): the keyset page,
	// the unread counter, read/delete mutations, and the single view whose
	// action buttons are computed from the entities' live state.
	FeedService *notificationsapp.FeedService
	// SettingsService answers the settings matrix (решение #738): the email
	// matrix on the account, the push matrix on the device.
	SettingsService *notificationsapp.SettingsService
	// DeliverySettingsGate is the delivery-time read of the email matrix the
	// email worker checks before sending.
	DeliverySettingsGate *notificationspg.DeliverySettingsGate
}

// WireNotifications constructs the push-subscription and feed repositories,
// the feed reading service and the settings service.
func WireNotifications(p platformDeps) *Notifications {
	pushSubscriptionRepo := notificationspg.NewPushSubscriptionRepository(p.DB)
	pushSubscriptionService := notificationsapp.NewPushSubscriptionService(pushSubscriptionRepo, p.Clock)
	notificationRepo := notificationspg.NewNotificationRepository(p.DB)
	emailPreferencesRepo := notificationspg.NewEmailPreferencesRepository(p.DB)
	deliverySettingsGate := notificationspg.NewDeliverySettingsGate(emailPreferencesRepo)

	// The action computation reads the live rental against the data owner's
	// calendar date (ADR 0048) — the same calendar the payments context
	// serves its tick with.
	feedLiveState := notificationspg.NewFeedLiveState(p.DB, paymentspg.NewOwnerCalendar(p.DB, p.Clock), p.Policy)
	feedService := notificationsapp.NewFeedService(notificationRepo, feedLiveState)
	settingsService := notificationsapp.NewSettingsService(emailPreferencesRepo, pushSubscriptionRepo)

	return &Notifications{
		PushSubscriptionRepo:    pushSubscriptionRepo,
		PushSubscriptionService: pushSubscriptionService,
		NotificationRepo:        notificationRepo,
		FeedService:             feedService,
		SettingsService:         settingsService,
		DeliverySettingsGate:    deliverySettingsGate,
	}
}
