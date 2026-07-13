package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

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
		detail, ok := UserFacingDetail(err)
		if !ok {
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
			return
		}
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", detail))
	case errors.Is(err, leasesapp.ErrNotFound):
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", "Категория не найдена"))
	case errors.Is(err, leasesapp.ErrDuplicateCategoryName):
		writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", "Категория с таким названием уже существует"))
	default:
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
	}
}

// ListOperationCategories implements GET /operation-categories.
func (h *CategoryHandlers) ListOperationCategories(w http.ResponseWriter, r *http.Request, params openapi.ListOperationCategoriesParams) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	q := leasesapp.ListOperationCategoriesQuery{}
	if params.Type != nil {
		q.Type = string(*params.Type)
	}

	categories, err := h.svc.ListCategories(r.Context(), ownerID, q)
	if err != nil {
		h.handleCategoryError(w, r, err)
		return
	}

	items := make([]openapi.OperationCategory, 0, len(categories))
	for _, c := range categories {
		items = append(items, openapi.OperationCategory{
			Id:        c.ID,
			Name:      c.Name,
			Type:      openapi.OperationType(c.Type),
			CreatedAt: c.CreatedAt,
		})
	}

	writeJSON(r.Context(), w, http.StatusOK, items)
}

// CreateOperationCategory implements POST /operation-categories.
func (h *CategoryHandlers) CreateOperationCategory(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.OperationCategoryCreateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create operation category request", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	category, err := h.svc.CreateCategory(r.Context(), ownerID, leasesapp.CreateOperationCategoryCommand{
		Type: string(body.Type),
		Name: body.Name,
	})
	if err != nil {
		h.handleCategoryError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusCreated, openapi.OperationCategory{
		Id:        category.ID,
		Name:      category.Name,
		Type:      openapi.OperationType(category.Type),
		CreatedAt: category.CreatedAt,
	})
}
