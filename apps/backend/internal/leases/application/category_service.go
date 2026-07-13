package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
)

type CategoryService struct {
	categories OperationCategoryRepository
}

func NewCategoryService(categories OperationCategoryRepository) *CategoryService {
	if categories == nil {
		panic("categories repository is required")
	}
	return &CategoryService{categories: categories}
}

func (s *CategoryService) CreateCategory(ctx context.Context, ownerID uuid.UUID, cmd CreateOperationCategoryCommand) (domain.OperationCategory, error) {
	if err := cmd.validate(); err != nil {
		return domain.OperationCategory{}, err
	}
	opType, err := domain.ParseOperationType(cmd.Type)
	if err != nil {
		return domain.OperationCategory{}, newInvalidInputError(err.Error())
	}
	return s.categories.Create(ctx, ownerID, opType, cmd.Name)
}

func (s *CategoryService) ListCategories(ctx context.Context, ownerID uuid.UUID, q ListOperationCategoriesQuery) ([]domain.OperationCategory, error) {
	var t *domain.OperationType
	if q.Type != "" {
		qt, err := domain.ParseOperationType(q.Type)
		if err != nil {
			return nil, newInvalidInputError(err.Error())
		}
		t = &qt
	}
	return s.categories.ListByOwner(ctx, ownerID, t)
}

func (s *CategoryService) SeedDefaultCategories(ctx context.Context, ownerID uuid.UUID) error {
	return s.categories.CreateDefaultCategories(ctx, ownerID)
}

type CreateOperationCategoryCommand struct {
	Type string
	Name string
}

func (c CreateOperationCategoryCommand) validate() error {
	if c.Name == "" {
		return newInvalidInputError("category name is required")
	}
	return nil
}

type ListOperationCategoriesQuery struct {
	Type string
}
