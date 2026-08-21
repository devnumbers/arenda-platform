package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
)

func newOperationGuardService(ownerID uuid.UUID, opRepo *fakeOperationRepo, propertyRepo *fakePropertyRepo) *OperationService {
	seedPropertyOwners(propertyRepo, ownerID)
	cats := newFakeCategoryRepoForOwner(ownerID)
	return NewOperationService(
		opRepo, propertyRepo, nil, cats,
		NewTxStoreFactory(nil, propertyRepo, nil, nil, opRepo, cats, nil, nil, testUoW()),
		fakeClock{now: date(2026, 6, 15)}, fakeTzResolver{}, fakePolicy{}, nil,
	)
}

// seedPropertyOwners maps every property known to the fake repository to the
// given owner so scope resolution returns the real owner instead of the
// repository's fallback (the property id itself).
func seedPropertyOwners(propertyRepo *fakePropertyRepo, ownerID uuid.UUID) {
	if len(propertyRepo.statuses) == 0 {
		return
	}
	if propertyRepo.owners == nil {
		propertyRepo.owners = map[uuid.UUID]uuid.UUID{}
	}
	for id := range propertyRepo.statuses {
		propertyRepo.owners[id] = ownerID
	}
}

// mustCreateOperation seeds an operation through the fake repo, failing the
// test when the seed itself breaks instead of discarding the error.
func mustCreateOperation(t *testing.T, repo *fakeOperationRepo, ctx context.Context, op domain.Operation) {
	t.Helper()
	if _, err := repo.Create(ctx, op); err != nil {
		t.Fatalf("seed operation: %v", err)
	}
}

func newGuardTestOperation(id, ownerID, propertyID uuid.UUID, status domain.OperationStatus) domain.Operation {
	return domain.Operation{
		ID:            id,
		OwnerID:       ownerID,
		PropertyID:    propertyID,
		Type:          domain.OperationTypeExpense,
		CategoryID:    testCustomExpenseCategoryID,
		Status:        status,
		Name:          testOperationName,
		AmountKopecks: 1000,
		OperationDate: date(2026, 6, 10),
	}
}

func TestCreateOperation_ArchivedPropertyGuard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	cmd := CreateOperationCommand{
		PropertyID:    propertyID,
		Type:          testTypeExpense,
		CategoryID:    testCustomExpenseCategoryID,
		Name:          testOperationName,
		AmountKopecks: 1000,
		OperationDate: date(2026, 6, 10),
	}

	for _, tc := range []struct {
		name    string
		status  string
		wantErr error
	}{
		{name: testNameArchivedRejected, status: propertyStatusArchived, wantErr: ErrArchivedProperty},
		{name: testNameActiveAllowed, status: propertyStatusActive, wantErr: nil},
		{name: testNameMaintenanceAllowed, status: propertyStatusMaintenance, wantErr: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := newOperationGuardService(ownerID, &fakeOperationRepo{}, &fakePropertyRepo{statuses: map[uuid.UUID]string{propertyID: tc.status}})
			_, err := svc.CreateOperation(ctx, ownerID, cmd)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("CreateOperation: want %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestUpdateOperation_ArchivedPropertyGuard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	operationID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	name := testUpdatedValue
	cmd := UpdateOperationCommand{Name: &name}

	for _, tc := range []struct {
		name    string
		status  string
		wantErr error
	}{
		{name: testNameArchivedRejected, status: propertyStatusArchived, wantErr: ErrArchivedProperty},
		{name: testNameActiveAllowed, status: propertyStatusActive, wantErr: nil},
		{name: testNameMaintenanceAllowed, status: propertyStatusMaintenance, wantErr: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			opRepo := &fakeOperationRepo{}
			mustCreateOperation(t, opRepo, ctx, newGuardTestOperation(operationID, ownerID, propertyID, domain.OperationStatusPending))
			svc := newOperationGuardService(ownerID, opRepo, &fakePropertyRepo{statuses: map[uuid.UUID]string{propertyID: tc.status}})
			_, err := svc.UpdateOperation(ctx, ownerID, operationID, cmd)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("UpdateOperation: want %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestUpdateOperation_NoPropertySkipsGuard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	operationID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	opRepo := &fakeOperationRepo{}
	mustCreateOperation(t, opRepo, ctx, newGuardTestOperation(operationID, ownerID, uuid.Nil, domain.OperationStatusPending))
	svc := newOperationGuardService(ownerID, opRepo, &fakePropertyRepo{})

	name := testUpdatedValue
	updated, err := svc.UpdateOperation(ctx, ownerID, operationID, UpdateOperationCommand{Name: &name})
	if err != nil {
		t.Fatalf("UpdateOperation without property: %v", err)
	}
	if updated.Name != testUpdatedValue {
		t.Errorf("name = %q, want updated", updated.Name)
	}
}

func TestDeleteOperation_ArchivedPropertyGuard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	operationID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	t.Run("archived rejected", func(t *testing.T) {
		opRepo := &fakeOperationRepo{}
		mustCreateOperation(t, opRepo, ctx, newGuardTestOperation(operationID, ownerID, propertyID, domain.OperationStatusPending))
		svc := newOperationGuardService(ownerID, opRepo, &fakePropertyRepo{statuses: map[uuid.UUID]string{propertyID: propertyStatusArchived}})
		if err := svc.DeleteOperation(ctx, ownerID, operationID); !errors.Is(err, ErrArchivedProperty) {
			t.Fatalf("DeleteOperation: want ErrArchivedProperty, got %v", err)
		}
		if _, err := opRepo.GetByIDAndOwner(ctx, operationID, ownerID); err != nil {
			t.Fatalf("operation must not be deleted: %v", err)
		}
	})

	for _, status := range []string{propertyStatusActive, propertyStatusMaintenance} {
		t.Run(status+" allowed", func(t *testing.T) {
			t.Parallel()
			opRepo := &fakeOperationRepo{}
			mustCreateOperation(t, opRepo, ctx, newGuardTestOperation(operationID, ownerID, propertyID, domain.OperationStatusPending))
			svc := newOperationGuardService(ownerID, opRepo, &fakePropertyRepo{statuses: map[uuid.UUID]string{propertyID: status}})
			if err := svc.DeleteOperation(ctx, ownerID, operationID); err != nil {
				t.Fatalf("DeleteOperation: %v", err)
			}
		})
	}
}

// runArchivedPropertyGuard runs the archived-property guard table for one
// mutating use case: the operation is seeded in seedStatus, call invokes the
// use case under test, and every property status case asserts its outcome.
func runArchivedPropertyGuard(
	t *testing.T,
	seedStatus domain.OperationStatus,
	call func(ctx context.Context, svc *OperationService, ownerID, operationID uuid.UUID) error,
) {
	t.Helper()
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	operationID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	for _, tc := range []struct {
		name    string
		status  string
		wantErr error
	}{
		{name: testNameArchivedRejected, status: propertyStatusArchived, wantErr: ErrArchivedProperty},
		{name: testNameActiveAllowed, status: propertyStatusActive, wantErr: nil},
		{name: testNameMaintenanceAllowed, status: propertyStatusMaintenance, wantErr: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			opRepo := &fakeOperationRepo{}
			mustCreateOperation(t, opRepo, ctx, newGuardTestOperation(operationID, ownerID, propertyID, seedStatus))
			svc := newOperationGuardService(ownerID, opRepo, &fakePropertyRepo{statuses: map[uuid.UUID]string{propertyID: tc.status}})
			if err := call(ctx, svc, ownerID, operationID); !errors.Is(err, tc.wantErr) {
				t.Fatalf("use case: want %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestCompleteOperation_ArchivedPropertyGuard(t *testing.T) {
	t.Parallel()
	runArchivedPropertyGuard(t, domain.OperationStatusPending,
		func(ctx context.Context, svc *OperationService, ownerID, operationID uuid.UUID) error {
			_, err := svc.CompleteOperation(ctx, CompleteOperationCommand{Actor: ownerID, OperationID: operationID})
			return err
		})
}

func TestMarkOperationIncomplete_ArchivedPropertyGuard(t *testing.T) {
	t.Parallel()
	runArchivedPropertyGuard(t, domain.OperationStatusPaid,
		func(ctx context.Context, svc *OperationService, ownerID, operationID uuid.UUID) error {
			_, err := svc.MarkOperationIncomplete(ctx, MarkOperationIncompleteCommand{Actor: ownerID, OperationID: operationID})
			return err
		})
}

func TestCreateOperation_BackdatedOperationStartsUnconfirmed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	for _, tc := range []struct {
		name          string
		opType        string
		categoryID    uuid.UUID
		operationDate [3]int
		want          domain.OperationStatus
	}{
		{
			name:          "past expense is unconfirmed",
			opType:        testTypeExpense,
			categoryID:    testCustomExpenseCategoryID,
			operationDate: [3]int{2026, 6, 10},
			want:          domain.OperationStatusUnconfirmed,
		},
		{
			name:          "past income is unconfirmed",
			opType:        testTypeIncome,
			categoryID:    testCustomIncomeCategoryID,
			operationDate: [3]int{2026, 6, 10},
			want:          domain.OperationStatusUnconfirmed,
		},
		{
			name:          "today is pending",
			opType:        testTypeExpense,
			categoryID:    testCustomExpenseCategoryID,
			operationDate: [3]int{2026, 6, 15},
			want:          domain.OperationStatusPending,
		},
		{
			name:          "future is pending",
			opType:        testTypeIncome,
			categoryID:    testCustomIncomeCategoryID,
			operationDate: [3]int{2026, 6, 20},
			want:          domain.OperationStatusPending,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := newOperationGuardService(ownerID, &fakeOperationRepo{},
				&fakePropertyRepo{statuses: map[uuid.UUID]string{propertyID: propertyStatusActive}})
			op, err := svc.CreateOperation(ctx, ownerID, CreateOperationCommand{
				PropertyID:    propertyID,
				Type:          tc.opType,
				CategoryID:    tc.categoryID,
				Name:          testOperationName,
				AmountKopecks: 1000,
				OperationDate: date(tc.operationDate[0], tc.operationDate[1], tc.operationDate[2]),
			})
			if err != nil {
				t.Fatalf("CreateOperation: %v", err)
			}
			if op.Status != tc.want {
				t.Fatalf("status = %s, want %s", op.Status, tc.want)
			}
		})
	}
}

func TestCompleteOperation_Unconfirmed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	for _, tc := range []struct {
		name       string
		opType     domain.OperationType
		categoryID uuid.UUID
		want       domain.OperationStatus
	}{
		{
			name:       "expense becomes paid",
			opType:     domain.OperationTypeExpense,
			categoryID: testCustomExpenseCategoryID,
			want:       domain.OperationStatusPaid,
		},
		{
			name:       "income becomes received",
			opType:     domain.OperationTypeIncome,
			categoryID: testCustomIncomeCategoryID,
			want:       domain.OperationStatusReceived,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			operationID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
			op := newGuardTestOperation(operationID, ownerID, propertyID, domain.OperationStatusUnconfirmed)
			op.Type = tc.opType
			op.CategoryID = tc.categoryID

			opRepo := &fakeOperationRepo{}
			mustCreateOperation(t, opRepo, ctx, op)
			svc := newOperationGuardService(ownerID, opRepo, &fakePropertyRepo{statuses: map[uuid.UUID]string{propertyID: propertyStatusActive}})

			updated, err := svc.CompleteOperation(ctx, CompleteOperationCommand{Actor: ownerID, OperationID: operationID})
			if err != nil {
				t.Fatalf("CompleteOperation: %v", err)
			}
			if updated.Status != tc.want {
				t.Fatalf("status = %s, want %s", updated.Status, tc.want)
			}
		})
	}
}

func TestProcessOverdueOperation_SkipsUnconfirmed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	operationID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	opRepo := &fakeOperationRepo{}
	mustCreateOperation(t, opRepo, ctx, newGuardTestOperation(operationID, ownerID, propertyID, domain.OperationStatusUnconfirmed))
	svc := newOperationGuardService(ownerID, opRepo, &fakePropertyRepo{statuses: map[uuid.UUID]string{propertyID: propertyStatusActive}})

	changed, err := svc.ProcessOverdueOperation(ctx, ownerID, operationID, date(2026, 6, 15))
	if err != nil {
		t.Fatalf("ProcessOverdueOperation: %v", err)
	}
	if changed {
		t.Fatal("unconfirmed operation must not transition to overdue")
	}

	stored, err := opRepo.GetByIDAndOwner(ctx, operationID, ownerID)
	if err != nil {
		t.Fatalf("get operation: %v", err)
	}
	if stored.Status != domain.OperationStatusUnconfirmed {
		t.Fatalf("status = %s, want %s", stored.Status, domain.OperationStatusUnconfirmed)
	}
}

func TestUpdateOperation_UnconfirmedStatusPreservedOnDateChange(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	operationID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	for _, tc := range []struct {
		name    string
		newDate [3]int
	}{
		{name: "another past date", newDate: [3]int{2026, 6, 1}},
		{name: "future date", newDate: [3]int{2026, 6, 20}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			opRepo := &fakeOperationRepo{}
			mustCreateOperation(t, opRepo, ctx, newGuardTestOperation(operationID, ownerID, propertyID, domain.OperationStatusUnconfirmed))
			svc := newOperationGuardService(ownerID, opRepo, &fakePropertyRepo{statuses: map[uuid.UUID]string{propertyID: propertyStatusActive}})

			newDate := date(tc.newDate[0], tc.newDate[1], tc.newDate[2])
			updated, err := svc.UpdateOperation(ctx, ownerID, operationID, UpdateOperationCommand{OperationDate: &newDate})
			if err != nil {
				t.Fatalf("UpdateOperation: %v", err)
			}
			if updated.Status != domain.OperationStatusUnconfirmed {
				t.Fatalf("status = %s, want %s", updated.Status, domain.OperationStatusUnconfirmed)
			}
		})
	}
}
