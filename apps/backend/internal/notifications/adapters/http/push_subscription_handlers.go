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

// PushSubscriptionHandlers implements the generated Web Push endpoints:
// delivering the VAPID public key to the frontend and accepting or removing
// per-device subscriptions.
type PushSubscriptionHandlers struct {
	svc         *notificationsapp.PushSubscriptionService
	vapidPubKey string
	logger      *slog.Logger
}

// NewPushSubscriptionHandlers creates HTTP handlers for the Web Push API.
// vapidPublicKey is the application server's public VAPID key served to the
// frontend; when empty the VAPID endpoint returns 503.
func NewPushSubscriptionHandlers(svc *notificationsapp.PushSubscriptionService, vapidPublicKey string, logger *slog.Logger) *PushSubscriptionHandlers {
	return &PushSubscriptionHandlers{svc: svc, vapidPubKey: vapidPublicKey, logger: logger}
}

// GetVapidPublicKey implements GET /push/vapid-public-key.
func (h *PushSubscriptionHandlers) GetVapidPublicKey(w http.ResponseWriter, r *http.Request) {
	if _, ok := httpsupport.UserIDFromContext(r.Context()); !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if h.vapidPubKey == "" {
		// VAPID is not configured on the server. The frontend treats this as
		// "push unavailable" rather than retrying.
		httpsupport.WriteProblem(w, http.StatusServiceUnavailable, httpsupport.Problem(r.Context(), "Service Unavailable", "Web Push не настроен"))
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.VapidPublicKeyResponse{PublicKey: h.vapidPubKey})
}

// CreatePushSubscription implements POST /push/subscriptions.
func (h *PushSubscriptionHandlers) CreatePushSubscription(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.PushSubscriptionCreateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create push subscription request", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	sub, err := h.svc.Upsert(r.Context(), actor, notificationsapp.UpsertPushSubscriptionInput{
		Endpoint:       body.Endpoint,
		P256dh:         body.P256dh,
		Auth:           body.Auth,
		ExpirationTime: body.ExpirationTime,
	})
	if err != nil {
		if errors.Is(err, notificationsapp.ErrInvalidPushSubscription) {
			httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректная push-подписка"))
			return
		}
		httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusCreated, pushSubscriptionResponse(sub))
}

// DeletePushSubscription implements DELETE /push/subscriptions.
func (h *PushSubscriptionHandlers) DeletePushSubscription(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.PushSubscriptionDeleteRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode delete push subscription request", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	if err := h.svc.Delete(r.Context(), actor, body.Endpoint); err != nil {
		if errors.Is(err, notificationsapp.ErrNotFound) {
			httpsupport.WriteProblem(w, http.StatusNotFound, httpsupport.Problem(r.Context(), "Not found", "Push-подписка не найдена"))
			return
		}
		if errors.Is(err, notificationsapp.ErrInvalidPushSubscription) {
			httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректная push-подписка"))
			return
		}
		httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func pushSubscriptionResponse(sub notificationsdomain.PushSubscription) openapi.PushSubscriptionResponse {
	return openapi.PushSubscriptionResponse{
		Id:        sub.ID,
		Endpoint:  sub.Endpoint,
		CreatedAt: sub.CreatedAt,
		UpdatedAt: sub.UpdatedAt,
	}
}
