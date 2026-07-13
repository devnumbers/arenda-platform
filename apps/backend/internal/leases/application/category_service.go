package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
)

type CategoryService struct {
	categories OperationCategoryRepository
}

func NewCategoryService(categories OperationCategoryRepository) *CategoryService {
	return &CategoryService{categories: categories}
}

func (s *CategoryService) CreateCategory(ctx context.Context, ownerID uuid.UUID, cmd CreateOperationCategoryCommand) (domain.OperationCategory, error) {
	if err := cmd.validate(); err != nil {
		return domain.OperationCategory{}, err
	}
	opType, err := domain.ParseOperationType(cmd.Type)
	if err != nil {
		return domain.OperationCategory{}, err
	}
	return s.categories.Create(ctx, ownerID, opType, cmd.Name)
}

func (s *CategoryService) ListCategories(ctx context.Context, ownerID uuid.UUID, q ListOperationCategoriesQuery) ([]domain.OperationCategory, error) {
	var t *domain.OperationType
	if q.Type != "" {
		qt, err := domain.ParseOperationType(q.Type)
		if err != nil {
			return nil, err
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
		return fmt.Errorf("category name is required")
	}
	return nil
}

type ListOperationCategoriesQuery struct {
	Type string
}
