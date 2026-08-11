package wire

import (
	notificationspg "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/postgres"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
)

// Notifications holds the notifications module's repositories, services and the
// reminder scheduler wired by WireNotifications.
type Notifications struct {
	ReminderRepo            *notificationspg.ReminderRepository
	FreeReminderRepo        *notificationspg.FreeReminderRepository
	PushSubscriptionRepo    *notificationspg.PushSubscriptionRepository
	ReminderService         *notificationsapp.ReminderService
	FreeReminderService     *notificationsapp.FreeReminderService
	CalendarService         *notificationsapp.CalendarService
	PreferenceService       *notificationsapp.PreferenceService
	PushSubscriptionService *notificationsapp.PushSubscriptionService
	ReminderScheduler       notificationsapp.ReminderScheduler
}

// WireNotifications constructs the reminder and free-reminder repositories, the
// reminder, free-reminder, calendar and preference services, and the reminder
// scheduler. The reminder service and scheduler are returned because several
// other modules (identity profile, properties lifecycle, leases) depend on them.
func WireNotifications(p platformDeps) *Notifications {
	reminderRepo := notificationspg.NewReminderRepository(p.DB)
	reminderService := notificationsapp.NewReminderService(reminderRepo, p.Clock, p.TZResolver, p.Policy)

	freeReminderRepo := notificationspg.NewFreeReminderRepository(p.DB)
	freeReminderService := notificationsapp.NewFreeReminderService(freeReminderRepo, p.Beginner, p.Clock, p.TZResolver, p.Policy)
	calendarService := notificationsapp.NewCalendarService(reminderRepo, freeReminderRepo, p.TZResolver)

	preferenceService := notificationsapp.NewPreferenceService(reminderRepo, p.Beginner, p.AuditRecorder)

	pushSubscriptionRepo := notificationspg.NewPushSubscriptionRepository(p.DB)
	pushSubscriptionService := notificationsapp.NewPushSubscriptionService(pushSubscriptionRepo, p.Clock)

	reminderScheduler := notificationsapp.NewReminderScheduler(reminderRepo, p.Clock, p.TZResolver)

	return &Notifications{
		ReminderRepo:            reminderRepo,
		FreeReminderRepo:        freeReminderRepo,
		PushSubscriptionRepo:    pushSubscriptionRepo,
		ReminderService:         reminderService,
		FreeReminderService:     freeReminderService,
		CalendarService:         calendarService,
		PreferenceService:       preferenceService,
		PushSubscriptionService: pushSubscriptionService,
		ReminderScheduler:       reminderScheduler,
	}
}
