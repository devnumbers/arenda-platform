package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

type CategoryService struct {
	categories OperationCategoryRepository
	audit      auditapp.Recorder
	// policy is injected after construction (see SetPolicy) because the
	// membership-aware policy is built after the category service in the
	// composition root. When nil, only the actor's own data is listed (pre-T2a).
	policy sharedpolicy.Policy
	// scopes is optionally injected (see SetAccessibleScopes); when nil only the
	// actor's own categories are listed.
	scopes AccessibleScopes
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

// SetPolicy injects the authorization policy. The membership-aware policy is
// built after the category service in the composition root, so it is wired via
// this setter. When not set, only the actor's own data is listed (issue #157).
func (s *CategoryService) SetPolicy(policy sharedpolicy.Policy) {
	s.policy = policy
}

// SetAccessibleScopes injects the access-context adapter that resolves the
// owners whose owner-wide data the actor may read. Optional: when nil, only the
// actor's own categories are listed (issue #157).
func (s *CategoryService) SetAccessibleScopes(scopes AccessibleScopes) {
	s.scopes = scopes
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

	// Owner's own categories.
	own, err := s.categories.ListByOwner(ctx, actor, t)
	if err != nil {
		return nil, err
	}
	result := own

	// Derived access: append categories of owners whose properties the actor is
	// a member of (issue #157). Each accessible owner is gated by the policy
	// port so the actor only sees owners for which CanView holds. When the
	// policy or scopes adapter is not injected, only the actor's own categories
	// are returned (the pre-T2a behaviour).
	if s.policy == nil || s.scopes == nil {
		return result, nil
	}
	owners, err := s.scopes.AccessibleOwners(ctx, actor)
	if err != nil {
		return nil, fmt.Errorf("list accessible owners: %w", err)
	}
	seen := make(map[uuid.UUID]bool, len(result))
	for _, c := range result {
		seen[c.ID] = true
	}
	for _, owner := range owners {
		role, err := s.policy.Role(ctx, actor, owner)
		if err != nil {
			return nil, fmt.Errorf("resolve role for owner: %w", err)
		}
		if !sharedpolicy.CanView(role) {
			continue
		}
		cats, err := s.categories.ListByOwner(ctx, owner, t)
		if err != nil {
			return nil, fmt.Errorf("list categories for accessible owner: %w", err)
		}
		for _, c := range cats {
			if !seen[c.ID] {
				seen[c.ID] = true
				result = append(result, c)
			}
		}
	}

	return result, nil
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
