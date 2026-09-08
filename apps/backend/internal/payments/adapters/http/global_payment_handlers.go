package http

// The global payment rules endpoints (ticket #575): the merged «Платежи»
// feed with the main screen's counters, the search with its matched-category
// chips and the «Объекты» stacks. The shared package doc lives in
// payment_handlers.go.

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// GlobalPaymentsManager is the consumer-side port of the global payment
// rules endpoints (ADR 0035). The concrete application service satisfies
// it; the handler tests run against func-backed fakes.
type GlobalPaymentsManager interface {
	ListGlobalPayments(ctx context.Context, actor uuid.UUID) (application.GlobalPaymentFeed, error)
	SearchGlobalPayments(ctx context.Context, actor uuid.UUID, search string) (application.GlobalPaymentSearch, error)
	ListGlobalPaymentObjects(ctx context.Context, actor uuid.UUID, search string) ([]application.GlobalPaymentObjectCard, error)
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
// by the search query plus the matched categories (ticket #575).
func (h *GlobalPaymentHandlers) SearchGlobalPayments(
	w http.ResponseWriter, r *http.Request, params openapi.SearchGlobalPaymentsParams,
) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	search, err := h.svc.SearchGlobalPayments(r.Context(), actor, derefString(params.Search))
	if err != nil {
		h.handleGlobalPaymentError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, paymentsSearchGlobalResponse(search))
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

// handleGlobalPaymentError maps the global reads' failures: there is no
// user-facing vocabulary on these paths (no path params, no command
// validation) — everything is either a session problem the middleware owns
// or an internal failure. Logged once here, one problem out.
func (h *GlobalPaymentHandlers) handleGlobalPaymentError(w http.ResponseWriter, r *http.Request, err error) {
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
	categories := make([]openapi.PaymentSearchCategory, 0, len(search.MatchedCategories))
	for _, category := range search.MatchedCategories {
		categories = append(categories, openapi.PaymentSearchCategory{
			Category: categoryView(category.Category),
			Type:     openapi.PaymentSearchCategoryType(category.Type),
			Count:    int(category.RuleCount),
		})
	}
	return openapi.PaymentsSearchGlobalResponse{Items: items, MatchedCategories: categories}
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
		Id:                    item.ID,
		PropertyId:            item.PropertyID,
		PropertyName:          item.PropertyName,
		Title:                 item.Title,
		AmountKopecks:         item.AmountKopecks,
		Type:                  openapi.PaymentGlobalItemType(item.Type),
		Category:              categoryView(item.Category),
		AutoPay:               item.AutoPay,
		IsFavorite:            item.IsFavorite,
		Today:                 openapi_types.Date{Time: item.Today},
		NearestDate:           dateWirePtr(item.NearestDate),
		OverdueOperationCount: int(item.OverdueCount),
		OverdueDays:           item.OverdueDays,
	}
}

// dateWirePtr lifts an optional calendar date onto the wire's nullable date;
// nil is the contract's null (no next occurrence / no overdue).
func dateWirePtr(d *time.Time) *openapi_types.Date {
	if d == nil {
		return nil
	}
	return &openapi_types.Date{Time: *d}
}
