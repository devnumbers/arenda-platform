package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	sharedtz "github.com/nambers/arenda-planform/apps/backend/internal/shared/tzresolver"
)

// Fixed test category IDs used across application tests.
var (
	testRentCategoryID          = uuid.MustParse("11111111-1111-1111-1111-111111111001")
	testCustomExpenseCategoryID = uuid.MustParse("11111111-1111-1111-1111-111111111002")
	testCustomIncomeCategoryID  = uuid.MustParse("11111111-1111-1111-1111-111111111004")
)

// Fixture constants shared across the application package tests.
const (
	// Operation type wire values, as the transport commands carry them.
	testTypeExpense = string(domain.OperationTypeExpense)
	testTypeIncome  = string(domain.OperationTypeIncome)

	// Generic operation/recurring name fixture and its updated value.
	testOperationName = "test"
	testUpdatedValue  = "updated"

	// Table-test case names for the property-status guard.
	testNameActiveAllowed      = "active allowed"
	testNameMaintenanceAllowed = "maintenance allowed"
	testNameArchivedRejected   = "archived rejected"

	// Tenant contact fixture family (setup + unchanged-field assertions).
	testTenantName       = "Ivan"
	testTenantRenameTo   = "Petr"
	testTenantSurname    = "Ivanov"
	testTenantPatronymic = "Ivanovich"
	testTenantEmail      = "ivan@example.com"
	testTenantComment    = "initial comment"

	// Ownership marker names for derived-access tests: the "Own" row belongs
	// to the data owner, the "Other" row to a different owner.
	testOwnedRowName = "Own"
	testOtherRowName = "Other"

	// Property fixtures.
	testPropertyName         = "Тест"
	testPropertyAddress      = "Москва, Тверская 1"
	testPropertyShortAddress = "Адрес"
)

// fakeTzResolver is a stub OwnerTimezoneResolver that always resolves UTC,
// keeping existing tests (which were written against UTC date semantics)
// unchanged.
type fakeTzResolver struct{}

func (fakeTzResolver) Resolve(_ context.Context, _ uuid.UUID) (*time.Location, error) {
	return time.UTC, nil
}

var _ sharedtz.OwnerTimezoneResolver = fakeTzResolver{}

// newFakeCategoryRepoForOwner returns a fake category repository seeded with
// default categories for the given owner, plus user-style categories without
// a code for tests that need a generic income/expense category.
func newFakeCategoryRepoForOwner(ownerID uuid.UUID) *fakeCategoryRepo {
	return &fakeCategoryRepo{categories: []domain.OperationCategory{
		{
			ID: testRentCategoryID, OwnerID: ownerID, Type: domain.OperationTypeIncome,
			Name: "rent", Code: new(string(domain.OperationCategoryCodeRent)),
		},
		{ID: testCustomExpenseCategoryID, OwnerID: ownerID, Type: domain.OperationTypeExpense, Name: "Custom expense"},
		{ID: testCustomIncomeCategoryID, OwnerID: ownerID, Type: domain.OperationTypeIncome, Name: "Custom income"},
	}}
}

// testUoW adapts the package's fakeTxBeginner to the transaction.UoW port so
// unit-test factories exercise the production commit/rollback semantics (the
// production UoW lives in platform/database/postgres).
func testUoW() fakeUoW {
	return fakeUoW{beginner: fakeTxBeginner{}}
}
