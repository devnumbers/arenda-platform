// Package http holds the action journal's HTTP adapters: the «История
// действий» feed and the filter-sheet options (карта #704, тикет #708,
// ADR 0061 §7).
package http

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// HistoryHandlers implements the journal's reading endpoints: the
// bidirectional-keyset feed and the filter options. The same feed endpoint
// serves all four screens: the general feed, an object's history
// (property_ids = one), a participant's actions (actor_ids = one), a
// participant's actions within one object (actor_ids and property_ids =
// one each, AND-combined — ticket #841).
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

	q := foldFeedPageQuery(
		params.BeforeCursor, params.AfterCursor, params.Limit,
		params.DateFrom, params.DateTo, params.Actions, params.Kinds, params.Q,
	)
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

	page, err := h.svc.Feed(r.Context(), actor, q)
	if err != nil {
		h.writeError(r, w, err)
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, historyPageResponse(page))
}

// GetParticipantHistory implements GET /participants/{participantId}/history
// (карта #1582, тикет #1583): the «Действия участника» page — the server
// derives the scope (the person's user uuid AND their current legs), the
// request carries only the page shape. A pending invitee is an empty page;
// a person outside the reader's scope is the privacy 404, a basic-tariff
// owner is the role-aware 402.
func (h *HistoryHandlers) GetParticipantHistory(
	w http.ResponseWriter, r *http.Request, participantID string, params openapi.GetParticipantHistoryParams,
) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	q := foldFeedPageQuery(
		params.BeforeCursor, params.AfterCursor, params.Limit,
		params.DateFrom, params.DateTo, params.Actions, params.Kinds, params.Q,
	)
	// «Объекты» шита страницы (аменда владельца 22.10.2026): выбор едет в
	// сервис, который пересекает его с ногами агрегата — чужой id молча
	// выпадает; битый uuid — 400, как у /history.
	propertyIDs, err := parseUUIDList(params.PropertyIds, "property id")
	if err != nil {
		h.writeError(r, w, err)
		return
	}
	q.PropertyIDs = propertyIDs

	page, err := h.svc.ParticipantFeed(r.Context(), actor, participantID, q)
	if err != nil {
		h.writeErrorWithNotFound(r, w, err, "Участник не найден")
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, historyPageResponse(page))
}

// foldFeedPageQuery folds the feed's shared page parameters — cursors,
// limit, period, base actions, kinds, search — into the service query. Both
// feed endpoints carry the same aliases (ADR 0061 §7); actor id lists stay
// endpoint-specific (the participant page derives the actor on the server),
// property ids travel on both — /history фильтрует ими область, страница
// участника сужает ею ноги. Malformed values surface as the contract's 400
// from the service's own fold (cursors, page size, vocabulary, search
// length).
func foldFeedPageQuery(
	beforeCursor, afterCursor *string,
	limit *int,
	dateFrom, dateTo *openapi_types.Date,
	actions, kinds, query *string,
) historyapp.FeedQuery {
	q := historyapp.FeedQuery{}
	if beforeCursor != nil {
		q.BeforeCursor = *beforeCursor
	}
	if afterCursor != nil {
		q.AfterCursor = *afterCursor
	}
	if limit != nil {
		q.Limit = *limit
	}
	if dateFrom != nil {
		from := time.Date(dateFrom.Year(), dateFrom.Month(), dateFrom.Day(), 0, 0, 0, 0, time.UTC)
		q.DateFrom = &from
	}
	if dateTo != nil {
		// Верхняя граница периода исключающая — дата + 24ч (канон
		// админ-аудита): экран фильтрует календарными днями.
		to := time.Date(dateTo.Year(), dateTo.Month(), dateTo.Day(), 0, 0, 0, 0, time.UTC).Add(24 * time.Hour)
		q.DateTo = &to
	}
	if actions != nil {
		q.BaseActions = splitCSV(*actions)
	}
	if kinds != nil {
		q.Kinds = splitCSV(*kinds)
	}
	if query != nil {
		q.Query = *query
	}
	return q
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
		participants[i] = openapi.HistoryParticipant{
			Id:        p.ID,
			Name:      p.Name,
			Email:     p.Email,
			FirstName: p.FirstName,
			// '' (фото нет) → null: контракт nullable (ADR 0065), пустая
			// строка на проводе глушит фолбэк заглушки клиента.
			PhotoUrl: httpsupport.StringPtr(p.PhotoURL),
			IsOwner:  p.IsOwner,
			Role:     openapi.HistoryParticipantRole(p.Role),
			Status:   openapi.HistoryParticipantStatus(p.Status),
		}
	}
	objects := make([]openapi.HistoryObject, len(options.Objects))
	for i, o := range options.Objects {
		objects[i] = openapi.HistoryObject{Id: o.ID, Name: o.Name, Address: o.Address, PhotoUrl: o.PhotoURL, Type: openapi.PropertyType(o.Type)}
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.HistoryFiltersResponse{
		Participants: participants,
		Objects:      objects,
	})
}

// writeError maps the service's sentinels onto the contract's statuses: the
// 404 is the privacy-404 — a foreign or suspended property is answered
// exactly like a missing one, the existence of either is never revealed; the
// 402 is the role-aware tariff gate's verdict (ADR 0064, ревизия #1465),
// the middleware's canonical wire shape.
func (h *HistoryHandlers) writeError(r *http.Request, w http.ResponseWriter, err error) {
	h.writeErrorWithNotFound(r, w, err, "Объект не найден")
}

// writeErrorWithNotFound is writeError with the surface's own 404 wording —
// the participant page's privacy-404 names the missing person, not the
// property («Участник не найден»); the statuses and the privacy discipline
// are the same.
func (h *HistoryHandlers) writeErrorWithNotFound(r *http.Request, w http.ResponseWriter, err error, notFoundDetail string) {
	switch {
	case errors.Is(err, historyapp.ErrTariffRequired):
		httpsupport.WriteTariffRequired(r.Context(), w)
	case errors.Is(err, historyapp.ErrNotFound):
		httpsupport.WriteProblem(r.Context(), w, http.StatusNotFound,
			httpsupport.Problem(r.Context(), "Not found", notFoundDetail))
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
			Id:             entry.ID,
			PropertyId:     entry.PropertyID,
			PropertyName:   entry.PropertyName,
			ActorId:        entry.ActorID,
			ActorName:      entry.ActorName,
			ActorEmail:     entry.ActorEmail,
			ActorFirstName: entry.ActorFirstName,
			ActorLastName:  entry.ActorLastName,
			// '' (фото нет) → null: контракт nullable (ADR 0065), пустая
			// строка на проводе глушит фолбэк заглушки клиента.
			ActorPhotoUrl: httpsupport.StringPtr(entry.ActorPhotoURL),
			ActorRole:     openapi.HistoryItemActorRole(entry.ActorRole),
			Kind:          openapi.HistoryItemKind(entry.Kind),
			Action:        string(entry.Action),
			BaseAction:    openapi.HistoryItemBaseAction(entry.BaseAction),
			Segments:      segments,
			Context:       entry.Context,
			CreatedAt:     entry.CreatedAt,
		}
		// Легаси-строки (details NULL) уходят без поля — «не пишется», а не
		// пустой массив (решение #1317 §2).
		if len(entry.Details) > 0 {
			details := make([]string, len(entry.Details))
			copy(details, entry.Details)
			items[i].Details = &details
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
	return httpsupport.SplitCSVParam(raw)
}

// parseUUIDList decodes a comma-separated uuid list; a malformed id is the
// contract's 400.
func parseUUIDList(raw *string, what string) ([]uuid.UUID, error) {
	return httpsupport.ParseUUIDList(raw, func(bad string) error {
		return fmt.Errorf("%w: %s %q is not a uuid", historyapp.ErrInvalidInput, what, bad)
	})
}
