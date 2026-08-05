package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
)

type CategoryService struct {
	categories OperationCategoryRepository
	audit      auditapp.Recorder
}

func NewCategoryService(categories OperationCategoryRepository, audit auditapp.Recorder) *CategoryService {
	if categories == nil {
		panic("categories repository is required")
	}
	if audit == nil {
		audit = auditapp.Noop{}
	}
	return &CategoryService{categories: categories, audit: audit}
}

func (s *CategoryService) CreateCategory(ctx context.Context, actor uuid.UUID, cmd CreateOperationCategoryCommand) (domain.OperationCategory, error) {
	if err := cmd.validate(); err != nil {
		return domain.OperationCategory{}, err
	}
	opType, err := domain.ParseOperationType(cmd.Type)
	if err != nil {
		return domain.OperationCategory{}, newInvalidInputError(err.Error())
	}
	created, err := s.categories.Create(ctx, actor, opType, cmd.Name)
	if err != nil {
		return domain.OperationCategory{}, err
	}
	// Post-commit, fail-loud: the create is already committed, so the audit
	// error is returned deliberately to surface audit gaps. A retry may
	// duplicate the category — acceptable for this entity.
	if err := s.audit.Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionOperationCategoryCreated,
		EntityType: auditdomain.EntityOperationCategory,
		EntityID:   &created.ID,
		Context:    map[string]any{"name": created.Name},
	}); err != nil {
		return domain.OperationCategory{}, fmt.Errorf("record audit: %w", err)
	}
	return created, nil
}

func (s *CategoryService) ListCategories(ctx context.Context, actor uuid.UUID, q ListOperationCategoriesQuery) ([]domain.OperationCategory, error) {
	var t *domain.OperationType
	if q.Type != "" {
		qt, err := domain.ParseOperationType(q.Type)
		if err != nil {
			return nil, newInvalidInputError(err.Error())
		}
		t = &qt
	}
	return s.categories.ListByOwner(ctx, actor, t)
}

func (s *CategoryService) SeedDefaultCategories(ctx context.Context, actor uuid.UUID) error {
	return s.categories.CreateDefaultCategories(ctx, actor)
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
