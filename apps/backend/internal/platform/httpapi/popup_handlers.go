package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	popupsapp "github.com/nambers/arenda-planform/apps/backend/internal/popups/application"
	popupsdomain "github.com/nambers/arenda-planform/apps/backend/internal/popups/domain"
)

// PopupHandlers implements the generated popup endpoints.
type PopupHandlers struct {
	svc    *popupsapp.PopupService
	logger *slog.Logger
}

// NewPopupHandlers creates HTTP handlers for the popups API.
func NewPopupHandlers(svc *popupsapp.PopupService, logger *slog.Logger) *PopupHandlers {
	return &PopupHandlers{svc: svc, logger: logger}
}

// GetPendingPopups implements GET /popups/pending.
func (h *PopupHandlers) GetPendingPopups(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	pending, err := h.svc.ListPending(r.Context(), ownerID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, pendingPopupsResponse(pending))
}

// MarkPopupSeen implements POST /popups/{popupKey}/seen.
func (h *PopupHandlers) MarkPopupSeen(w http.ResponseWriter, r *http.Request, popupKey string) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.svc.MarkSeen(r.Context(), ownerID, popupsdomain.PopupKey(popupKey)); err != nil {
		if errors.Is(err, popupsapp.ErrUnknownPopupKey) {
			writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "Неизвестный попап"))
			return
		}
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func pendingPopupsResponse(pending []popupsdomain.PopupKey) openapi.PendingPopupsResponse {
	popups := make([]string, 0, len(pending))
	for _, k := range pending {
		popups = append(popups, string(k))
	}
	return openapi.PendingPopupsResponse{Popups: popups}
}
