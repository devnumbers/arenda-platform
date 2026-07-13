package application

import (
	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
)

// Fixed test category IDs used across application tests.
var (
	testRentCategoryID          = uuid.MustParse("11111111-1111-1111-1111-111111111001")
	testUtilitiesCategoryID     = uuid.MustParse("11111111-1111-1111-1111-111111111002")
	testDepositReturnCategoryID = uuid.MustParse("11111111-1111-1111-1111-111111111003")
	testOtherIncomeCategoryID   = uuid.MustParse("11111111-1111-1111-1111-111111111004")
)

// newFakeCategoryRepoForOwner returns a fake category repository seeded with
// default categories for the given owner.
func newFakeCategoryRepoForOwner(ownerID uuid.UUID) *fakeCategoryRepo {
	return &fakeCategoryRepo{categories: []domain.OperationCategory{
		{ID: testRentCategoryID, OwnerID: ownerID, Type: domain.OperationTypeIncome, Name: "rent", Code: new(string(domain.OperationCategoryCodeRent))},
		{ID: testOtherIncomeCategoryID, OwnerID: ownerID, Type: domain.OperationTypeIncome, Name: "other_income", Code: new(string(domain.OperationCategoryCodeOtherIncome))},
		{ID: testUtilitiesCategoryID, OwnerID: ownerID, Type: domain.OperationTypeExpense, Name: "utilities", Code: new(string(domain.OperationCategoryCodeUtilities))},
		{ID: testDepositReturnCategoryID, OwnerID: ownerID, Type: domain.OperationTypeExpense, Name: "deposit_return", Code: new(string(domain.OperationCategoryCodeDepositReturn))},
	}}
}
