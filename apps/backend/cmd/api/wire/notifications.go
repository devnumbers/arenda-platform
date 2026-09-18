package wire

import (
	notificationspg "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/postgres"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
)

// Notifications holds the notifications module's repositories and services
// wired by WireNotifications. The module owns the delivery channels, the
// push subscriptions, the stored feed and the delivery queue's inputs
// (#740); the grace events publish through the pipeline since #741.
type Notifications struct {
	PushSubscriptionRepo    *notificationspg.PushSubscriptionRepository
	PushSubscriptionService *notificationsapp.PushSubscriptionService
	// NotificationRepo is the stored feed repository (решение #737); the
	// delivery workers and the publisher read and write it.
	NotificationRepo *notificationspg.NotificationRepository
}

// WireNotifications constructs the push-subscription and feed repositories
// plus the push-subscription service.
func WireNotifications(p platformDeps) *Notifications {
	pushSubscriptionRepo := notificationspg.NewPushSubscriptionRepository(p.DB)
	pushSubscriptionService := notificationsapp.NewPushSubscriptionService(pushSubscriptionRepo, p.Clock)
	notificationRepo := notificationspg.NewNotificationRepository(p.DB)

	return &Notifications{
		PushSubscriptionRepo:    pushSubscriptionRepo,
		PushSubscriptionService: pushSubscriptionService,
		NotificationRepo:        notificationRepo,
	}
}
