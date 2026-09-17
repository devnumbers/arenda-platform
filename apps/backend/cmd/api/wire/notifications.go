package wire

import (
	notificationspg "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/postgres"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
)

// Notifications holds the notifications module's repositories and services
// wired by WireNotifications. The module owns the delivery channels, the push
// subscriptions and the direct send for subscription_grace events; the stored
// feed repository arrives with the delivery pipeline (#740).
type Notifications struct {
	PushSubscriptionRepo    *notificationspg.PushSubscriptionRepository
	PushSubscriptionService *notificationsapp.PushSubscriptionService
}

// WireNotifications constructs the push-subscription repository and service.
func WireNotifications(p platformDeps) *Notifications {
	pushSubscriptionRepo := notificationspg.NewPushSubscriptionRepository(p.DB)
	pushSubscriptionService := notificationsapp.NewPushSubscriptionService(pushSubscriptionRepo, p.Clock)

	return &Notifications{
		PushSubscriptionRepo:    pushSubscriptionRepo,
		PushSubscriptionService: pushSubscriptionService,
	}
}
