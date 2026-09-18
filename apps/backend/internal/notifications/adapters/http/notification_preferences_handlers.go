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

// NotificationPreferencesHandlers implements the per-category settings
// endpoints (решение #738, ADR 0056): the account-level email matrix and the
// per-device push preferences.
type NotificationPreferencesHandlers struct {
	settings *notificationsapp.SettingsService
	logger   *slog.Logger
}

// NewNotificationPreferencesHandlers creates the settings handlers.
func NewNotificationPreferencesHandlers(settings *notificationsapp.SettingsService, logger *slog.Logger) *NotificationPreferencesHandlers {
	return &NotificationPreferencesHandlers{settings: settings, logger: logger}
}

// GetNotificationPreferences implements GET /notification-preferences.
func (h *NotificationPreferencesHandlers) GetNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	user, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	prefs, err := h.settings.EmailPreferences(r.Context(), user)
	if err != nil {
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.NotificationPreferencesResponse{
		Email: categoriesToOpenAPI(prefs),
	})
}

// PutNotificationPreferences implements PUT /notification-preferences.
func (h *NotificationPreferencesHandlers) PutNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	user, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.NotificationPreferencesRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode notification preferences request",
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	prefs := categoriesFromOpenAPI(body.Email)
	if err := h.settings.SetEmailPreferences(r.Context(), user, prefs); err != nil {
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.NotificationPreferencesResponse{
		Email: categoriesToOpenAPI(prefs),
	})
}

// GetPushSubscriptionPreferences implements GET /push/subscriptions/preferences.
func (h *NotificationPreferencesHandlers) GetPushSubscriptionPreferences(
	w http.ResponseWriter, r *http.Request, params openapi.GetPushSubscriptionPreferencesParams,
) {
	user, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	sub, err := h.settings.PushPreferences(r.Context(), user, params.Endpoint)
	if err != nil {
		h.writeError(r, w, err)
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.PushPreferencesResponse{
		Endpoint:   sub.Endpoint,
		Enabled:    sub.Enabled,
		Categories: categoriesToOpenAPI(sub.Categories),
	})
}

// PutPushSubscriptionPreferences implements PUT /push/subscriptions/preferences.
func (h *NotificationPreferencesHandlers) PutPushSubscriptionPreferences(w http.ResponseWriter, r *http.Request) {
	user, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.PushPreferencesRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode push preferences request",
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	prefs := categoriesFromOpenAPI(body.Categories)
	if err := h.settings.SetPushPreferences(r.Context(), user, body.Endpoint, body.Enabled, prefs); err != nil {
		h.writeError(r, w, err)
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.PushPreferencesResponse{
		Endpoint:   body.Endpoint,
		Enabled:    body.Enabled,
		Categories: categoriesToOpenAPI(prefs),
	})
}

// writeError maps the settings sentinel errors onto the contract's statuses.
func (h *NotificationPreferencesHandlers) writeError(r *http.Request, w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, notificationsapp.ErrNotFound):
		httpsupport.WriteProblem(r.Context(), w, http.StatusNotFound,
			httpsupport.Problem(r.Context(), "Not found", "Push-подписка не найдена"))
	case errors.Is(err, notificationsapp.ErrInvalidPushSubscription):
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректный endpoint"))
	default:
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
	}
}

func categoriesToOpenAPI(p notificationsdomain.CategoryPrefs) openapi.NotificationCategoryPreferences {
	return openapi.NotificationCategoryPreferences{
		Rental:             p.Rental,
		PaymentsOperations: p.PaymentsOperations,
		Tasks:              p.Tasks,
		SharedAccess:       p.SharedAccess,
	}
}

func categoriesFromOpenAPI(p openapi.NotificationCategoryPreferences) notificationsdomain.CategoryPrefs {
	return notificationsdomain.CategoryPrefs{
		Rental:             p.Rental,
		PaymentsOperations: p.PaymentsOperations,
		Tasks:              p.Tasks,
		SharedAccess:       p.SharedAccess,
	}
}
