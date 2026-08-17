package application

import (
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// TestCategoryService_ListCategories_DerivedAccess verifies that the owner-wide
// derived access (issue #157) is honoured: a member with full access to one of
// the owner's properties sees the owner's categories (RoleFullAccess ->
// CanView), a viewer sees them too (RoleViewer -> CanView), and a stranger with
// RoleNone sees nothing.
func TestCategoryService_ListCategories_DerivedAccess(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	owner := uuid.Must(uuid.NewV7())
	memberFull := uuid.Must(uuid.NewV7())
	memberViewer := uuid.Must(uuid.NewV7())
	stranger := uuid.Must(uuid.NewV7())

	ownerCat := domain.OperationCategory{ID: uuid.Must(uuid.NewV7()), OwnerID: owner, Type: domain.OperationTypeIncome, Name: "Owner Cat"}
	repo := &fakeCategoryRepo{categories: []domain.OperationCategory{ownerCat}}

	policy := fakePolicy{
		roles: map[[2]uuid.UUID]sharedpolicy.Role{
			{memberFull, owner}:   sharedpolicy.RoleFullAccess,
			{memberViewer, owner}: sharedpolicy.RoleViewer,
			// stranger has no entry -> RoleNone.
		},
	}
	scopes := fakeAccessibleScopes{
		memberFull:   []uuid.UUID{owner},
		memberViewer: []uuid.UUID{owner},
		// stranger -> nil (no accessible owners).
	}

	tests := []struct {
		name      string
		actor     uuid.UUID
		wantCount int
		wantCat   bool // expects ownerCat in result
	}{
		{"owner sees own", owner, 1, true},
		{"member with full access sees owner categories", memberFull, 1, true},
		{"member with viewer sees owner categories", memberViewer, 1, true},
		{"stranger sees nothing (none)", stranger, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := NewCategoryService(repo, nil)
			svc.SetPolicy(policy)
			svc.SetAccessibleScopes(scopes)

			got, err := svc.ListCategories(ctx, tt.actor, ListOperationCategoriesQuery{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tt.wantCount {
				t.Fatalf("got %d categories, want %d: %+v", len(got), tt.wantCount, got)
			}
			if tt.wantCat {
				if idx := slices.IndexFunc(got, func(c domain.OperationCategory) bool { return c.ID == ownerCat.ID }); idx < 0 {
					t.Fatalf("expected ownerCat in result, got %+v", got)
				}
			}
		})
	}
}

// TestCategoryService_ListCategories_DerivedAccess_Dedup verifies that when the
// actor is also the owner (own category plus listed as accessible owner), the
// category is not duplicated in the result.
func TestCategoryService_ListCategories_DerivedAccess_Dedup(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	owner := uuid.Must(uuid.NewV7())
	ownerCat := domain.OperationCategory{ID: uuid.Must(uuid.NewV7()), OwnerID: owner, Type: domain.OperationTypeExpense, Name: "Rent"}
	repo := &fakeCategoryRepo{categories: []domain.OperationCategory{ownerCat}}

	policy := fakePolicy{}
	scopes := fakeAccessibleScopes{
		// owner is (incorrectly) listed as accessible to themselves; the
		// service must dedup so the own category is not returned twice.
		owner: []uuid.UUID{owner},
	}

	svc := NewCategoryService(repo, nil)
	svc.SetPolicy(policy)
	svc.SetAccessibleScopes(scopes)

	got, err := svc.ListCategories(ctx, owner, ListOperationCategoriesQuery{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d categories, want 1 (deduped): %+v", len(got), got)
	}
	if got[0].ID != ownerCat.ID {
		t.Fatalf("got category %v, want %v", got[0].ID, ownerCat.ID)
	}
}

// TestCategoryService_ListCategories_DerivedAccess_TypeFilter ensures the type
// filter is applied to both own and derived categories.
func TestCategoryService_ListCategories_DerivedAccess_TypeFilter(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	owner := uuid.Must(uuid.NewV7())
	member := uuid.Must(uuid.NewV7())
	repo := &fakeCategoryRepo{categories: []domain.OperationCategory{
		{ID: uuid.Must(uuid.NewV7()), OwnerID: owner, Type: domain.OperationTypeIncome, Name: "Inc"},
		{ID: uuid.Must(uuid.NewV7()), OwnerID: owner, Type: domain.OperationTypeExpense, Name: "Exp"},
	}}
	policy := fakePolicy{roles: map[[2]uuid.UUID]sharedpolicy.Role{
		{member, owner}: sharedpolicy.RoleFullAccess,
	}}
	scopes := fakeAccessibleScopes{member: []uuid.UUID{owner}}

	svc := NewCategoryService(repo, nil)
	svc.SetPolicy(policy)
	svc.SetAccessibleScopes(scopes)

	got, err := svc.ListCategories(ctx, member, ListOperationCategoriesQuery{Type: "expense"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d categories, want 1 (expense only): %+v", len(got), got)
	}
	if got[0].Type != domain.OperationTypeExpense {
		t.Fatalf("got type %v, want expense", got[0].Type)
	}
}

// TestCategoryService_ListCategories_NilSafe_OwnOnly verifies that without
// policy/scopes injected, only the actor's own categories are returned.
func TestCategoryService_ListCategories_NilSafe_OwnOnly(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	owner := uuid.Must(uuid.NewV7())
	other := uuid.Must(uuid.NewV7())
	repo := &fakeCategoryRepo{categories: []domain.OperationCategory{
		{ID: uuid.Must(uuid.NewV7()), OwnerID: owner, Type: domain.OperationTypeIncome, Name: "Own"},
		{ID: uuid.Must(uuid.NewV7()), OwnerID: other, Type: domain.OperationTypeIncome, Name: "Other"},
	}}

	svc := NewCategoryService(repo, nil) // no SetPolicy/SetAccessibleScopes

	got, err := svc.ListCategories(ctx, owner, ListOperationCategoriesQuery{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d categories, want 1 (owner's own only): %+v", len(got), got)
	}
	if got[0].OwnerID != owner {
		t.Fatalf("got owner %v, want %v", got[0].OwnerID, owner)
	}
}

// TestCategoryService_ListCategories_NilSafe_OnlyPolicy verifies that when only
// the policy is injected (scopes is nil), the service still falls back to
// own-only behaviour.
func TestCategoryService_ListCategories_NilSafe_OnlyPolicy(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	owner := uuid.Must(uuid.NewV7())
	repo := &fakeCategoryRepo{categories: []domain.OperationCategory{
		{ID: uuid.Must(uuid.NewV7()), OwnerID: owner, Type: domain.OperationTypeIncome, Name: "Own"},
	}}

	svc := NewCategoryService(repo, nil)
	svc.SetPolicy(fakePolicy{}) // scopes intentionally left nil

	got, err := svc.ListCategories(ctx, owner, ListOperationCategoriesQuery{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d categories, want 1 (nil scopes -> own only): %+v", len(got), got)
	}
}
