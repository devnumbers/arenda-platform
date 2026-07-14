package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type fakeCategoryRepo struct {
	categories []domain.OperationCategory
}

func (f *fakeCategoryRepo) Create(ctx context.Context, ownerID uuid.UUID, categoryType domain.OperationType, name string) (domain.OperationCategory, error) {
	c := domain.OperationCategory{
		ID:      uuid.New(),
		OwnerID: ownerID,
		Type:    categoryType,
		Name:    name,
	}
	f.categories = append(f.categories, c)
	return c, nil
}

func (f *fakeCategoryRepo) ListByOwner(ctx context.Context, ownerID uuid.UUID, categoryType *domain.OperationType) ([]domain.OperationCategory, error) {
	return f.categories, nil
}

func (f *fakeCategoryRepo) GetByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (domain.OperationCategory, error) {
	for _, c := range f.categories {
		if c.ID == id && c.OwnerID == ownerID {
			return c, nil
		}
	}
	return domain.OperationCategory{}, ErrNotFound
}

func (f *fakeCategoryRepo) GetByOwnerAndCode(ctx context.Context, ownerID uuid.UUID, code domain.OperationCategoryDefaultCode) (domain.OperationCategory, error) {
	for _, c := range f.categories {
		if c.OwnerID == ownerID && c.Code != nil && *c.Code == string(code) {
			return c, nil
		}
	}
	return domain.OperationCategory{}, ErrNotFound
}

func (f *fakeCategoryRepo) CreateDefaultCategories(ctx context.Context, ownerID uuid.UUID) error {
	return nil
}

func (f *fakeCategoryRepo) WithTx(tx transaction.Tx) OperationCategoryRepository {
	return f
}

func TestCreateOperationCategoryCommand_Validate(t *testing.T) {
	svc := NewCategoryService(&fakeCategoryRepo{})

	_, err := svc.CreateCategory(context.Background(), uuid.New(), CreateOperationCategoryCommand{Type: "invalid", Name: "Foo"})
	if err == nil {
		t.Fatal("expected error for invalid type")
	}

	_, err = svc.CreateCategory(context.Background(), uuid.New(), CreateOperationCategoryCommand{Type: "income", Name: ""})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestCategoryService_CreateCategory(t *testing.T) {
	ctx := t.Context()
	owner := uuid.New()
	repo := &fakeCategoryRepo{}
	svc := NewCategoryService(repo)

	cat, err := svc.CreateCategory(ctx, owner, CreateOperationCategoryCommand{Type: "expense", Name: "Custom Expense"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cat.OwnerID != owner {
		t.Fatalf("unexpected owner: got %v, want %v", cat.OwnerID, owner)
	}
	if cat.Type != domain.OperationTypeExpense {
		t.Fatalf("unexpected type: got %v", cat.Type)
	}
	if cat.Name != "Custom Expense" {
		t.Fatalf("unexpected name: got %v", cat.Name)
	}
}
