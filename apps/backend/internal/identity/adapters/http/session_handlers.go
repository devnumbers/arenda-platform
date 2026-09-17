package http

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// currentSessionRequest resolves the actor and the raw session cookie the
// three /me/sessions handlers share. When !ok a problem response has been
// written already.
func (h *AuthHandlers) currentSessionRequest(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, bool) {
	userID, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return uuid.Nil, "", false
	}
	token := httpsupport.SessionTokenFromRequest(r, h.cookieSecure)
	if token == "" {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return uuid.Nil, "", false
	}
	return userID, token, true
}

// ListSessions implements GET /me/sessions — the devices list: every live
// session of the caller, most recent activity first, with the session behind
// the current cookie flagged.
func (h *AuthHandlers) ListSessions(w http.ResponseWriter, r *http.Request) {
	userID, token, ok := h.currentSessionRequest(w, r)
	if !ok {
		return
	}

	sessions, currentID, err := h.sessions.List(r.Context(), userID, token)
	if err != nil {
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}

	devices := make([]openapi.SessionDevice, 0, len(sessions))
	for _, sess := range sessions {
		devices = append(devices, sessionDeviceResponse(sess, sess.ID == currentID))
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.SessionListResponse{Sessions: devices})
}

// RevokeSession implements DELETE /me/sessions/{sessionId} — terminating one
// of the user's other sessions. The current session is refused (409): the
// devices list ends it through logout.
func (h *AuthHandlers) RevokeSession(w http.ResponseWriter, r *http.Request, sessionID openapi_types.UUID) {
	userID, token, ok := h.currentSessionRequest(w, r)
	if !ok {
		return
	}

	err := h.sessions.Revoke(r.Context(), userID, sessionID, token, actorFromContext(r.Context()))
	if err != nil {
		switch {
		case errors.Is(err, identityapp.ErrNotFound):
			httpsupport.WriteProblem(r.Context(), w, http.StatusNotFound,
				httpsupport.Problem(r.Context(), "Not found", "Сессия не найдена"))
		case errors.Is(err, identityapp.ErrCurrentSession):
			httpsupport.WriteProblem(r.Context(), w, http.StatusConflict,
				httpsupport.Problem(r.Context(), "Conflict", "Нельзя завершить текущую сессию — используйте выход"))
		default:
			httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// LogoutOtherSessions implements POST /me/sessions/logout-others — terminating
// every session of the caller except the current one.
func (h *AuthHandlers) LogoutOtherSessions(w http.ResponseWriter, r *http.Request) {
	userID, token, ok := h.currentSessionRequest(w, r)
	if !ok {
		return
	}

	if _, err := h.sessions.RevokeOthers(r.Context(), userID, token, actorFromContext(r.Context())); err != nil {
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// sessionDeviceResponse maps the session aggregate onto the contract DTO. The
// raw token hash never leaves the application: the response carries display
// fields plus the row ID as the revocation handle.
func sessionDeviceResponse(sess domain.Session, current bool) openapi.SessionDevice {
	device := openapi.SessionDevice{
		Id:         sess.ID,
		DeviceType: openapi.SessionDeviceDeviceType(sess.DeviceType),
		Browser:    sess.Browser,
		Os:         sess.OS,
		City:       stringPtr(sess.City),
		LastIp:     stringPtr(sess.LastIP),
		LastSeenAt: sess.LastUsedAt,
		CreatedAt:  sess.CreatedAt,
		Current:    current,
	}
	if sess.BrowserMajor > 0 {
		major := sess.BrowserMajor
		device.BrowserMajor = &major
	}
	return device
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
