// Package http holds the action journal's HTTP adapters: the «История
// действий» feed and the filter-sheet options (карта #704, тикет #708,
// ADR 0061 §7).
package http

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

// HistoryHandlers implements the journal's reading endpoints: the
// bidirectional-keyset feed and the filter options. The same feed endpoint
// serves all three screens: the general feed, an object's history
// (property_ids = one), a participant's actions (actor_ids = one).
type HistoryHandlers struct {
	svc    *historyapp.HistoryReadService
	logger *slog.Logger
}

// NewHistoryHandlers creates the history reading handlers.
func NewHistoryHandlers(svc *historyapp.HistoryReadService, logger *slog.Logger) *HistoryHandlers {
	return &HistoryHandlers{svc: svc, logger: logger}
}

// GetHistory implements GET /history.
func (h *HistoryHandlers) GetHistory(w http.ResponseWriter, r *http.Request, params openapi.GetHistoryParams) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	q := historyapp.FeedQuery{}
	if params.BeforeCursor != nil {
		q.BeforeCursor = *params.BeforeCursor
	}
	if params.AfterCursor != nil {
		q.AfterCursor = *params.AfterCursor
	}
	if params.Limit != nil {
		q.Limit = *params.Limit
	}
	if params.DateFrom != nil {
		from := time.Date(params.DateFrom.Year(), params.DateFrom.Month(), params.DateFrom.Day(), 0, 0, 0, 0, time.UTC)
		q.DateFrom = &from
	}
	if params.DateTo != nil {
		// Верхняя граница периода исключающая — дата + 24ч (канон
		// админ-аудита): экран фильтрует календарными днями.
		to := time.Date(params.DateTo.Year(), params.DateTo.Month(), params.DateTo.Day(), 0, 0, 0, 0, time.UTC).Add(24 * time.Hour)
		q.DateTo = &to
	}
	if params.Actions != nil {
		q.BaseActions = splitCSV(*params.Actions)
	}
	if params.Kinds != nil {
		q.Kinds = splitCSV(*params.Kinds)
	}
	actorIDs, err := parseUUIDList(params.ActorIds, "actor id")
	if err != nil {
		h.writeError(r, w, err)
		return
	}
	q.ActorIDs = actorIDs
	propertyIDs, err := parseUUIDList(params.PropertyIds, "property id")
	if err != nil {
		h.writeError(r, w, err)
		return
	}
	q.PropertyIDs = propertyIDs
	if params.Q != nil {
		q.Query = *params.Q
	}

	page, err := h.svc.Feed(r.Context(), actor, q)
	if err != nil {
		h.writeError(r, w, err)
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, historyPageResponse(page))
}

// GetHistoryFilters implements GET /history/filters.
func (h *HistoryHandlers) GetHistoryFilters(w http.ResponseWriter, r *http.Request, params openapi.GetHistoryFiltersParams) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}
	propertyIDs, err := parseUUIDList(params.PropertyIds, "property id")
	if err != nil {
		h.writeError(r, w, err)
		return
	}

	options, err := h.svc.Filters(r.Context(), actor, propertyIDs)
	if err != nil {
		h.writeError(r, w, err)
		return
	}

	participants := make([]openapi.HistoryParticipant, len(options.Participants))
	for i, p := range options.Participants {
		participants[i] = openapi.HistoryParticipant{Id: p.ID, Name: p.Name, Email: p.Email}
	}
	objects := make([]openapi.HistoryObject, len(options.Objects))
	for i, o := range options.Objects {
		objects[i] = openapi.HistoryObject{Id: o.ID, Name: o.Name, Address: o.Address, PhotoUrl: o.PhotoURL}
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.HistoryFiltersResponse{
		Participants: participants,
		Objects:      objects,
	})
}

// writeError maps the service's sentinels onto the contract's statuses: the
// 404 is the privacy-404 — a foreign or suspended property is answered
// exactly like a missing one, the existence of either is never revealed.
func (h *HistoryHandlers) writeError(r *http.Request, w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, historyapp.ErrNotFound):
		httpsupport.WriteProblem(r.Context(), w, http.StatusNotFound,
			httpsupport.Problem(r.Context(), "Not found", "Объект не найден"))
	case errors.Is(err, historyapp.ErrInvalidInput):
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректный запрос"))
	default:
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
	}
}

func historyPageResponse(page historyapp.FeedPage) openapi.HistoryPageResponse {
	items := make([]openapi.HistoryItem, len(page.Items))
	for i, entry := range page.Items {
		segments := make([]openapi.HistorySegment, len(entry.Segments))
		for j, seg := range entry.Segments {
			item := openapi.HistorySegment{Text: seg.Text}
			if seg.Link != nil {
				item.Link = &openapi.HistorySegmentLink{
					Kind: openapi.HistorySegmentLinkKind(seg.Link.Kind),
					Id:   seg.Link.ID,
				}
			}
			segments[j] = item
		}
		items[i] = openapi.HistoryItem{
			Id:           entry.ID,
			PropertyId:   entry.PropertyID,
			PropertyName: entry.PropertyName,
			ActorId:      entry.ActorID,
			ActorName:    entry.ActorName,
			ActorEmail:   entry.ActorEmail,
			ActorRole:    openapi.HistoryItemActorRole(entry.ActorRole),
			Kind:         openapi.HistoryItemKind(entry.Kind),
			Action:       string(entry.Action),
			BaseAction:   openapi.HistoryItemBaseAction(entry.BaseAction),
			Segments:     segments,
			Context:      entry.Context,
			CreatedAt:    entry.CreatedAt,
		}
	}
	resp := openapi.HistoryPageResponse{Items: items}
	if page.NextCursor != "" {
		resp.NextCursor = &page.NextCursor
	}
	if page.PrevCursor != "" {
		resp.PrevCursor = &page.PrevCursor
	}
	return resp
}

// splitCSV decodes a comma-separated wire list; blanks are dropped.
func splitCSV(raw string) []string {
	out := make([]string, 0, 4)
	for part := range strings.SplitSeq(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// parseUUIDList decodes a comma-separated uuid list; a malformed id is the
// contract's 400.
func parseUUIDList(raw *string, what string) ([]uuid.UUID, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	ids := make([]uuid.UUID, 0, 4)
	for part := range strings.SplitSeq(*raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := uuid.Parse(part)
		if err != nil {
			return nil, fmt.Errorf("%w: %s %q is not a uuid", historyapp.ErrInvalidInput, what, part)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
