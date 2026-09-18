// Package http holds the notifications HTTP adapters: the stored feed
// reading API, the per-category settings and the push-subscription
// endpoints.
package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	notificationsdomain "github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

// FeedHandlers implements the stored feed's reading endpoints: the keyset
// page, the unread counter, the read/delete mutations and the single
// notification view with live actions (#743).
type FeedHandlers struct {
	svc    *notificationsapp.FeedService
	logger *slog.Logger
}

// NewFeedHandlers creates the feed reading handlers.
func NewFeedHandlers(svc *notificationsapp.FeedService, logger *slog.Logger) *FeedHandlers {
	return &FeedHandlers{svc: svc, logger: logger}
}

// ListNotifications implements GET /notifications.
func (h *FeedHandlers) ListNotifications(w http.ResponseWriter, r *http.Request, params openapi.ListNotificationsParams) {
	user, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	pageParams := notificationsapp.FeedPageParams{UnreadOnly: params.Unread != nil && *params.Unread}
	if params.Limit != nil {
		pageParams.Limit = *params.Limit
	}
	if params.Cursor != nil {
		pageParams.Cursor = *params.Cursor
	}

	page, err := h.svc.Page(r.Context(), user, pageParams)
	if err != nil {
		h.writeError(r, w, err)
		return
	}

	items := make([]openapi.NotificationItem, len(page.Items))
	for i, n := range page.Items {
		items[i] = notificationItem(n)
	}
	resp := openapi.NotificationsPageResponse{Items: items}
	if page.NextCursor != "" {
		resp.NextCursor = &page.NextCursor
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, resp)
}

// GetUnreadNotificationCount implements GET /notifications/unread-count.
func (h *FeedHandlers) GetUnreadNotificationCount(w http.ResponseWriter, r *http.Request) {
	user, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	count, err := h.svc.UnreadCount(r.Context(), user)
	if err != nil {
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.UnreadCountResponse{Count: count})
}

// GetNotification implements GET /notifications/{notificationId}.
func (h *FeedHandlers) GetNotification(w http.ResponseWriter, r *http.Request, notificationID uuid.UUID) {
	user, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	detail, err := h.svc.Get(r.Context(), user, notificationID)
	if err != nil {
		h.writeError(r, w, err)
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, notificationDetail(detail))
}

// MarkNotificationRead implements POST /notifications/{notificationId}/read.
func (h *FeedHandlers) MarkNotificationRead(w http.ResponseWriter, r *http.Request, notificationID uuid.UUID) {
	user, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}
	if err := h.svc.MarkRead(r.Context(), user, notificationID); err != nil {
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// MarkAllNotificationsRead implements POST /notifications/read-all.
func (h *FeedHandlers) MarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	user, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}
	if _, err := h.svc.MarkAllRead(r.Context(), user); err != nil {
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteNotification implements DELETE /notifications/{notificationId}.
func (h *FeedHandlers) DeleteNotification(w http.ResponseWriter, r *http.Request, notificationID uuid.UUID) {
	user, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}
	if err := h.svc.Delete(r.Context(), user, notificationID); err != nil {
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteAllNotifications implements DELETE /notifications.
func (h *FeedHandlers) DeleteAllNotifications(w http.ResponseWriter, r *http.Request) {
	user, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}
	if _, err := h.svc.DeleteAll(r.Context(), user); err != nil {
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeError maps the service's sentinel errors onto the contract's statuses.
func (h *FeedHandlers) writeError(r *http.Request, w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, notificationsapp.ErrNotFound):
		httpsupport.WriteProblem(r.Context(), w, http.StatusNotFound,
			httpsupport.Problem(r.Context(), "Not found", "Уведомление не найдено"))
	case errors.Is(err, notificationsapp.ErrInvalidInput):
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректный запрос"))
	default:
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
	}
}

func notificationItem(n notificationsdomain.Notification) openapi.NotificationItem {
	item := openapi.NotificationItem{
		Id:        n.ID,
		EventType: string(n.EventType),
		Category:  string(n.Category),
		Title:     n.Title,
		Body:      n.Body,
		CreatedAt: n.CreatedAt,
		ReadAt:    n.ReadAt,
	}
	if n.ContextLabel != "" {
		item.ContextLabel = &n.ContextLabel
	}
	return item
}

// notificationDetail converts the stored row plus its live actions into the
// contract's detail response; the payload travels as the free-form object
// the client renders cards from.
func notificationDetail(d notificationsapp.NotificationDetail) openapi.NotificationDetailResponse {
	n := d.Notification
	resp := openapi.NotificationDetailResponse{
		Id:        n.ID,
		EventType: string(n.EventType),
		Category:  string(n.Category),
		Title:     n.Title,
		Body:      n.Body,
		CreatedAt: n.CreatedAt,
		ReadAt:    n.ReadAt,
	}
	if n.ContextLabel != "" {
		resp.ContextLabel = &n.ContextLabel
	}
	if payload, err := json.Marshal(n.Payload); err == nil {
		var raw map[string]any
		if json.Unmarshal(payload, &raw) == nil {
			resp.Payload = raw
		}
	}
	actions := make([]openapi.NotificationAction, len(d.Actions))
	for i, a := range d.Actions {
		actions[i] = openapi.NotificationAction(a)
	}
	resp.Actions = actions
	return resp
}
