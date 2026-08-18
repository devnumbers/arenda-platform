package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

// categoryNamesByID loads all operation categories of the owner (any type) and
// returns a map from category ID to category name for response enrichment.
func categoryNamesByID(ctx context.Context, svc *leasesapp.CategoryService, actor uuid.UUID) (map[uuid.UUID]string, error) {
	categories, err := svc.ListCategories(ctx, actor, leasesapp.ListOperationCategoriesQuery{})
	if err != nil {
		return nil, err
	}
	names := make(map[uuid.UUID]string, len(categories))
	for _, c := range categories {
		names[c.ID] = c.Name
	}
	return names, nil
}

// CategoryHandlers implements the generated operation-category endpoints.
type CategoryHandlers struct {
	svc    *leasesapp.CategoryService
	logger *slog.Logger
}

// NewCategoryHandlers creates HTTP handlers for the operation categories API.
func NewCategoryHandlers(svc *leasesapp.CategoryService, logger *slog.Logger) *CategoryHandlers {
	return &CategoryHandlers{svc: svc, logger: logger}
}

func (h *CategoryHandlers) handleCategoryError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, leasesapp.ErrInvalidInput), errors.Is(err, domain.ErrInvalidOperationType):
		detail, ok := httpsupport.UserFacingDetail(err)
		if !ok {
			httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", detail))
	case errors.Is(err, leasesapp.ErrNotFound):
		httpsupport.WriteProblem(r.Context(), w, http.StatusNotFound, httpsupport.Problem(r.Context(), "Not found", "Категория не найдена"))
	case errors.Is(err, leasesapp.ErrForbidden):
		httpsupport.WriteProblem(r.Context(), w, http.StatusForbidden, httpsupport.Problem(r.Context(), "Forbidden", "Недостаточно прав для создания категории"))
	case errors.Is(err, leasesapp.ErrDuplicateCategoryName):
		httpsupport.WriteProblem(r.Context(), w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", "Категория с таким названием уже существует"))
	default:
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
	}
}

// ListOperationCategories implements GET /operation-categories.
func (h *CategoryHandlers) ListOperationCategories(w http.ResponseWriter, r *http.Request, params openapi.ListOperationCategoriesParams) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	q := leasesapp.ListOperationCategoriesQuery{}
	if params.Type != nil {
		q.Type = string(*params.Type)
	}

	categories, err := h.svc.ListCategories(r.Context(), actor, q)
	if err != nil {
		h.handleCategoryError(w, r, err)
		return
	}

	items := make([]openapi.OperationCategory, 0, len(categories))
	for _, c := range categories {
		items = append(items, openapi.OperationCategory{
			Id:        c.ID,
			Name:      c.Name,
			Code:      c.Code,
			Type:      openapi.OperationType(c.Type),
			CreatedAt: c.CreatedAt,
		})
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, items)
}

// CreateOperationCategory implements POST /operation-categories.
func (h *CategoryHandlers) CreateOperationCategory(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.OperationCategoryCreateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create operation category request", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	category, err := h.svc.CreateCategory(r.Context(), actor, leasesapp.CreateOperationCategoryCommand{
		Type:       string(body.Type),
		Name:       body.Name,
		PropertyID: body.PropertyId,
	})
	if err != nil {
		h.handleCategoryError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusCreated, openapi.OperationCategory{
		Id:        category.ID,
		Name:      category.Name,
		Code:      category.Code,
		Type:      openapi.OperationType(category.Type),
		CreatedAt: category.CreatedAt,
	})
}
