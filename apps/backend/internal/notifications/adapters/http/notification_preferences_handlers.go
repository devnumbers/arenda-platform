// Package http holds the notifications HTTP adapters: notification-preference and push-subscription
// endpoints, including the VAPID public-key endpoint.
package http

import (
	"errors"
	"log/slog"
	"net/http"

	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	notificationsdomain "github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

// NotificationPreferenceHandlers implements the generated notification
// preferences endpoints.
type NotificationPreferenceHandlers struct {
	svc    *notificationsapp.PreferenceService
	logger *slog.Logger
}

// NewNotificationPreferenceHandlers creates HTTP handlers for the notification
// preferences API.
func NewNotificationPreferenceHandlers(svc *notificationsapp.PreferenceService, logger *slog.Logger) *NotificationPreferenceHandlers {
	return &NotificationPreferenceHandlers{svc: svc, logger: logger}
}

// GetNotificationPreferences implements GET /notification-preferences.
func (h *NotificationPreferenceHandlers) GetNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	channelPrefs, err := h.svc.ListChannelPreferences(r.Context(), actor)
	if err != nil {
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, notificationChannelPreferencesResponse(channelPrefs))
}

// UpdateNotificationPreferences implements PUT /notification-preferences.
func (h *NotificationPreferenceHandlers) UpdateNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.NotificationPreferencesUpdateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update notification preferences request",
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	prefs := channelPreferencesFromRequest(body.Preferences)

	updated, err := h.svc.ReplaceChannelPreferences(r.Context(), actor, prefs)
	if err != nil {
		if errors.Is(err, notificationsapp.ErrInvalidPreferences) {
			httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
				httpsupport.Problem(r.Context(), "Bad request", "Некорректные настройки уведомлений"))
			return
		}
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, notificationChannelPreferencesResponse(updated))
}

// channelPreferencesFromRequest expands the per-event-type request items (each
// carrying independent email/push flags) into the per-channel domain slice the
// service expects: one (event_type, channel) entry per flag.
func channelPreferencesFromRequest(items []openapi.NotificationPreference) []notificationsdomain.NotificationChannelPreference {
	prefs := make([]notificationsdomain.NotificationChannelPreference, 0, len(items)*2)
	for _, item := range items {
		eventType := notificationsdomain.EventType(item.EventType)
		prefs = append(prefs,
			notificationsdomain.NotificationChannelPreference{
				EventType: eventType, Channel: notificationsdomain.ChannelEmail, Allowed: item.EmailAllowed,
			},
			notificationsdomain.NotificationChannelPreference{
				EventType: eventType, Channel: notificationsdomain.ChannelPush, Allowed: item.PushAllowed,
			},
		)
	}
	return prefs
}

// notificationChannelPreferencesResponse collapses the per-channel domain
// slice back into one response item per event type, carrying emailAllowed and
// pushAllowed.
func notificationChannelPreferencesResponse(
	prefs []notificationsdomain.NotificationChannelPreference,
) openapi.NotificationPreferencesResponse {
	byType := make(map[notificationsdomain.EventType]struct {
		email bool
		push  bool
	}, len(prefs))
	for _, p := range prefs {
		entry := byType[p.EventType]
		switch p.Channel {
		case notificationsdomain.ChannelEmail:
			entry.email = p.Allowed
		case notificationsdomain.ChannelPush:
			entry.push = p.Allowed
		}
		byType[p.EventType] = entry
	}

	items := make([]openapi.NotificationPreference, 0, len(byType))
	for _, eventType := range notificationsdomain.AllEventTypes() {
		entry, ok := byType[eventType]
		if !ok {
			continue
		}
		items = append(items, openapi.NotificationPreference{
			EventType:    openapi.NotificationPreferenceEventType(eventType),
			EmailAllowed: entry.email,
			PushAllowed:  entry.push,
		})
	}
	return openapi.NotificationPreferencesResponse{Preferences: items}
}
