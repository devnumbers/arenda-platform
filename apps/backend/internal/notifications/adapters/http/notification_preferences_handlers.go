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
	ownerID, ok := httpsupport.OwnerIDFromContext(r)
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	prefs, err := h.svc.ListPreferences(r.Context(), ownerID)
	if err != nil {
		httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, notificationPreferencesResponse(prefs))
}

// UpdateNotificationPreferences implements PUT /notification-preferences.
func (h *NotificationPreferenceHandlers) UpdateNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := httpsupport.OwnerIDFromContext(r)
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.NotificationPreferencesUpdateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update notification preferences request", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	prefs := make([]notificationsdomain.NotificationPreference, 0, len(body.Preferences))
	for _, p := range body.Preferences {
		prefs = append(prefs, notificationsdomain.NotificationPreference{
			EventType: notificationsdomain.EventType(p.EventType),
			Allowed:   p.Allowed,
		})
	}

	updated, err := h.svc.ReplacePreferences(r.Context(), ownerID, prefs)
	if err != nil {
		if errors.Is(err, notificationsapp.ErrInvalidPreferences) {
			httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректные настройки уведомлений"))
			return
		}
		httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, notificationPreferencesResponse(updated))
}

func notificationPreferencesResponse(prefs []notificationsdomain.NotificationPreference) openapi.NotificationPreferencesResponse {
	items := make([]openapi.NotificationPreference, 0, len(prefs))
	for _, p := range prefs {
		items = append(items, openapi.NotificationPreference{
			EventType: openapi.NotificationPreferenceEventType(p.EventType),
			Allowed:   p.Allowed,
		})
	}
	return openapi.NotificationPreferencesResponse{Preferences: items}
}
