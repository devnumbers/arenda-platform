package wire

import (
	notificationspg "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/postgres"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
)

// Notifications holds the notifications module's repositories and services
// wired by WireNotifications. The module is grace-only (issue #438): channels
// delivery, per-channel preferences (ADR 0030), push subscriptions and the
// direct send for subscription_grace events.
type Notifications struct {
	PreferenceRepo          *notificationspg.PreferenceRepository
	PushSubscriptionRepo    *notificationspg.PushSubscriptionRepository
	PreferenceService       *notificationsapp.PreferenceService
	PushSubscriptionService *notificationsapp.PushSubscriptionService
}

// WireNotifications constructs the push-subscription repository, the preference
// and push-subscription services. The repositories are returned because the
// grace notifier (direct send) consumes them.
func WireNotifications(p platformDeps) *Notifications {
	factory := notificationsapp.NewTxStoreFactory(notificationspg.NewPreferenceRepository(p.DB), p.AuditRecorder, p.UoW)
	preferenceService := notificationsapp.NewPreferenceService(factory)

	pushSubscriptionRepo := notificationspg.NewPushSubscriptionRepository(p.DB)
	pushSubscriptionService := notificationsapp.NewPushSubscriptionService(pushSubscriptionRepo, p.Clock)

	return &Notifications{
		PreferenceRepo:          notificationspg.NewPreferenceRepository(p.DB),
		PushSubscriptionRepo:    pushSubscriptionRepo,
		PreferenceService:       preferenceService,
		PushSubscriptionService: pushSubscriptionService,
	}
}
