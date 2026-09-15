package http

// The global payment rules endpoints (tickets #575, #576): the merged
// «Платежи» feed with the main screen's counters, the search with its
// matched-category chips, the «Объекты» stacks and the favorites manual
// order save. The shared package doc lives in payment_handlers.go.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// GlobalPaymentsManager is the consumer-side port of the global payment
// rules endpoints (ADR 0035). The concrete application service satisfies
// it; the handler tests run against func-backed fakes.
type GlobalPaymentsManager interface {
	ListGlobalPayments(ctx context.Context, actor uuid.UUID) (application.GlobalPaymentFeed, error)
	SearchGlobalPayments(
		ctx context.Context, actor uuid.UUID, search string, page application.GlobalPaymentSearchPage,
	) (application.GlobalPaymentSearch, error)
	ListGlobalPaymentObjects(ctx context.Context, actor uuid.UUID, search string) ([]application.GlobalPaymentObjectCard, error)
	SaveFavoriteOrder(ctx context.Context, actor uuid.UUID, paymentIDs []uuid.UUID) error
}

// GlobalPaymentHandlers implements the generated global payment endpoints.
type GlobalPaymentHandlers struct {
	svc    GlobalPaymentsManager
	logger *slog.Logger
}

// NewGlobalPaymentHandlers creates HTTP handlers for the global payments API.
func NewGlobalPaymentHandlers(svc GlobalPaymentsManager, logger *slog.Logger) *GlobalPaymentHandlers {
	if logger == nil {
		logger = slog.Default()
	}
	return &GlobalPaymentHandlers{svc: svc, logger: logger}
}

// ListGlobalPayments implements GET /payments — the global «Платежи»
// screen's feed with the counters (ticket #575). Reads never tick.
func (h *GlobalPaymentHandlers) ListGlobalPayments(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	feed, err := h.svc.ListGlobalPayments(r.Context(), actor)
	if err != nil {
		h.handleGlobalPaymentError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, paymentsGlobalResponse(feed))
}

// SearchGlobalPayments implements GET /payments/search — the feed narrowed
// by the search query plus the matched categories, in pages of 50 under the
// chip's category/type filter (ticket #575, map #573 rework).
func (h *GlobalPaymentHandlers) SearchGlobalPayments(
	w http.ResponseWriter, r *http.Request, params openapi.SearchGlobalPaymentsParams,
) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	page, err := searchPageFromParams(params)
	if err != nil {
		h.handleGlobalPaymentError(w, r, err)
		return
	}

	search, err := h.svc.SearchGlobalPayments(r.Context(), actor, derefString(params.Search), page)
	if err != nil {
		h.handleGlobalPaymentError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, paymentsSearchGlobalResponse(search))
}

// searchPageFromParams folds the search's query parameters onto the page
// request: the chip's identity (category + direction, the generated enum
// validated) plus the page window — the limit bounded before the int32
// narrowing (the wire carries any int) and the continuation cursor as the
// opaque string the service decodes. Out-of-range numbers are
// ErrInvalidInput (the contract's 400); the zero limit stays zero — the
// service applies its default, one validation home for the page shape.
func searchPageFromParams(params openapi.SearchGlobalPaymentsParams) (application.GlobalPaymentSearchPage, error) {
	page := application.GlobalPaymentSearchPage{Category: derefString(params.Category)}
	if params.Type != nil {
		if !params.Type.Valid() {
			return page, fmt.Errorf("type %q: %w", *params.Type, application.ErrInvalidInput)
		}
		page.Type = domain.PaymentType(*params.Type)
	}
	if params.Limit != nil {
		if *params.Limit < 1 || *params.Limit > application.MaxPaymentRulesPageSize {
			return page, fmt.Errorf("limit %d: %w", *params.Limit, application.ErrInvalidInput)
		}
		page.Limit = int32(*params.Limit)
	}
	page.Cursor = derefString(params.Cursor)
	return page, nil
}

// ListGlobalPaymentObjects implements GET /payments/objects — the visible
// properties with their payment stacks (ticket #575).
func (h *GlobalPaymentHandlers) ListGlobalPaymentObjects(
	w http.ResponseWriter, r *http.Request, params openapi.ListGlobalPaymentObjectsParams,
) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	cards, err := h.svc.ListGlobalPaymentObjects(r.Context(), actor, derefString(params.Search))
	if err != nil {
		h.handleGlobalPaymentError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, paymentObjectsGlobalResponse(cards))
}

// SaveGlobalPaymentFavoritesOrder implements PUT /payments/favorites/order
// (ticket #576): the favorites edit mode's «Сохранить» — the submitted
// list order becomes the rules' manual favorite order, atomically.
func (h *GlobalPaymentHandlers) SaveGlobalPaymentFavoritesOrder(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	body, err := decodeFavoriteOrderBody(w, r)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode favorites order request",
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}
	if err := h.svc.SaveFavoriteOrder(r.Context(), actor, body.PaymentIds); err != nil {
		if !writePaymentsError(r.Context(), w, err) {
			h.logger.ErrorContext(r.Context(), "global payment favorites order save failed",
				slog.String("error", httpsupport.SanitizeError(err)))
			httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError,
				httpsupport.InternalError(r.Context(), err))
		}
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusNoContent, nil)
}

// decodeFavoriteOrderBody reads the order save's body strictly — the same
// shadow trick as decodeFavoriteBody: the required paymentIds array cannot
// express its own absence in the generated struct, an empty array is a
// valid save (it resets the order).
func decodeFavoriteOrderBody(w http.ResponseWriter, r *http.Request) (openapi.PaymentsFavoriteOrderUpdate, error) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, httpsupport.MaxRequestBodySize))
	if err != nil {
		return openapi.PaymentsFavoriteOrderUpdate{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var body openapi.PaymentsFavoriteOrderUpdate
	if err := dec.Decode(&body); err != nil {
		return openapi.PaymentsFavoriteOrderUpdate{}, err
	}
	var shadow struct {
		PaymentIds []*openapi_types.UUID `json:"paymentIds"`
	}
	if err := json.Unmarshal(raw, &shadow); err != nil {
		return openapi.PaymentsFavoriteOrderUpdate{}, err
	}
	if shadow.PaymentIds == nil {
		return openapi.PaymentsFavoriteOrderUpdate{}, errors.New("payments: paymentIds is required")
	}
	return body, nil
}

// handleGlobalPaymentError maps the global reads' failures: there is no
// user-facing vocabulary on these paths (no path params, no command
// validation) — everything is either a session problem the middleware owns
// or an internal failure. Logged once here, one problem out.
func (h *GlobalPaymentHandlers) handleGlobalPaymentError(w http.ResponseWriter, r *http.Request, err error) {
	// Invalid input (the search page's out-of-range numbers) is the
	// contract's 400, not an opaque 500 — the shared payments table.
	if writePaymentsError(r.Context(), w, err) {
		return
	}
	h.logger.ErrorContext(r.Context(), "global payments read failed",
		slog.String("error", httpsupport.SanitizeError(err)))
	httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
}

// paymentsGlobalResponse maps the feed onto the wire (ticket #575).
func paymentsGlobalResponse(feed application.GlobalPaymentFeed) openapi.PaymentsGlobalResponse {
	items := make([]openapi.PaymentGlobalItem, 0, len(feed.Items))
	for _, item := range feed.Items {
		items = append(items, paymentGlobalItem(item))
	}
	return openapi.PaymentsGlobalResponse{
		Items:                  items,
		FavoriteCount:          int(feed.FavoriteCount),
		OverdueOperationsCount: int(feed.OverdueOperationsCount),
	}
}

// paymentsSearchGlobalResponse maps the search onto the wire (ticket #575).
func paymentsSearchGlobalResponse(search application.GlobalPaymentSearch) openapi.PaymentsSearchGlobalResponse {
	items := make([]openapi.PaymentGlobalItem, 0, len(search.Items))
	for _, item := range search.Items {
		items = append(items, paymentGlobalItem(item))
	}
	categories := make([]openapi.CategoryView, 0, len(search.MatchedCategories))
	for _, category := range search.MatchedCategories {
		categories = append(categories, categoryView(category))
	}
	return openapi.PaymentsSearchGlobalResponse{
		Items:             items,
		MatchedCategories: categories,
		// The keyset continuation (ticket #597): '' is the wire's null — the
		// matches are exhausted.
		NextCursor: httpsupport.StringPtr(search.NextCursor),
		// The scope's match count (ticket #599): the same on every walked
		// page.
		Total: search.Total,
	}
}

// paymentObjectsGlobalResponse maps the «Объекты» stacks onto the wire
// (ticket #575).
func paymentObjectsGlobalResponse(cards []application.GlobalPaymentObjectCard) openapi.PaymentObjectsGlobalResponse {
	items := make([]openapi.PaymentObjectItem, 0, len(cards))
	for _, card := range cards {
		items = append(items, openapi.PaymentObjectItem{
			PropertyId:   card.PropertyID,
			Name:         card.Name,
			Address:      card.Address,
			PinnedAt:     card.PinnedAt,
			PhotoUrl:     card.PhotoURL,
			AutoPayRules: paymentObjectKeys(card.AutoPayKeys),
			OtherRules:   paymentObjectKeys(card.OtherKeys),
		})
	}
	return openapi.PaymentObjectsGlobalResponse{Items: items}
}

func paymentObjectKeys(keys []application.GlobalPaymentObjectKey) []openapi.PaymentObjectKey {
	out := make([]openapi.PaymentObjectKey, 0, len(keys))
	for _, key := range keys {
		out = append(out, openapi.PaymentObjectKey{PaymentId: key.PaymentID, HasOverdue: key.HasOverdue})
	}
	return out
}

func paymentGlobalItem(item application.GlobalPaymentItem) openapi.PaymentGlobalItem {
	return openapi.PaymentGlobalItem{
		Id:                       item.ID,
		PropertyId:               item.PropertyID,
		PropertyName:             item.PropertyName,
		Title:                    item.Title,
		AmountKopecks:            item.AmountKopecks,
		Type:                     openapi.PaymentGlobalItemType(item.Type),
		Category:                 categoryView(item.Category),
		AutoPay:                  item.AutoPay,
		IsFavorite:               item.IsFavorite,
		FavoriteOrder:            orderWirePtr(item.FavoriteOrder),
		Today:                    openapi_types.Date{Time: item.Today},
		NearestDate:              dateWirePtr(item.NearestDate),
		OverdueOperationCount:    int(item.OverdueCount),
		OverdueDays:              item.OverdueDays,
		OldestOverdueOperationId: item.OldestOverdueOperationID,
	}
}

// orderWirePtr narrows the optional order position onto the wire's nullable
// integer; nil is the contract's null (never in a saved order).
func orderWirePtr(v *int64) *int {
	if v == nil {
		return nil
	}
	out := int(*v)
	return &out
}

// dateWirePtr lifts an optional calendar date onto the wire's nullable date;
// nil is the contract's null (no next occurrence / no overdue).
func dateWirePtr(d *time.Time) *openapi_types.Date {
	if d == nil {
		return nil
	}
	return &openapi_types.Date{Time: *d}
}
