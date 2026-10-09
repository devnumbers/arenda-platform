package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// Гард удаления арендованного объекта (тикет #632): DeleteProperty
// отвечает ErrPropertyOccupied при незавершённой аренде, а строки аренд
// уносит до строки объекта — RESTRICT rentals.payment_id не должен
// гоняться с каскадом платежей (прецедент явного порядка — ADR 0025 §2).

// fakeRentalDeletionGuard — двойник порта RentalDeletionGuard: отвечает
// сценарной незавершённой арендой и пишет вызовы в общий журнал для
// проверки порядка «сначала аренды, потом объект».
type fakeRentalDeletionGuard struct {
	unfinished bool
	err        error
	journal    *[]string
}

func (f *fakeRentalDeletionGuard) HasUnfinished(
	_ context.Context, _ transaction.Tx, _, _ uuid.UUID,
) (bool, error) {
	f.record("guard.HasUnfinished")
	return f.unfinished, f.err
}

func (f *fakeRentalDeletionGuard) DeleteByProperty(
	_ context.Context, _ transaction.Tx, _, _ uuid.UUID,
) error {
	f.record("guard.DeleteByProperty")
	return nil
}

func (f *fakeRentalDeletionGuard) record(event string) {
	if f.journal != nil {
		*f.journal = append(*f.journal, event)
	}
}

// journalingDeleteRepo оборачивает fakePropertyRepo журналом удаления —
// так порядок guard → repo видно в одном списке событий; WithTx
// возвращает обёртку, чтобы журнал жил и внутри транзакции.
type journalingDeleteRepo struct {
	*fakePropertyRepo
	journal *[]string
}

func (r journalingDeleteRepo) Delete(ctx context.Context, id, scope uuid.UUID) error {
	*r.journal = append(*r.journal, "repo.Delete")
	return r.fakePropertyRepo.Delete(ctx, id, scope)
}

func (r journalingDeleteRepo) WithTx(_ transaction.Tx) PropertyRepository {
	return r
}

func deleteGuardFixture(
	t *testing.T, unfinished bool, guardErr error,
) (svc *PropertyService, repo *fakePropertyRepo, journal *[]string, ownerID, propertyID uuid.UUID) {
	t.Helper()
	ownerID = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	repo = newFakePropertyRepo(
		domain.Property{
			ID: propertyID, OwnerID: ownerID, Name: testPropertyName, Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
		},
	)
	journal = &[]string{}
	guard := &fakeRentalDeletionGuard{unfinished: unfinished, err: guardErr, journal: journal}
	wrapped := journalingDeleteRepo{fakePropertyRepo: repo, journal: journal}
	svc = NewPropertyService(
		wrapped,
		newFakePropertyPhotoStorage(),
		newPropertyTestFactory(wrapped, nil),
		fakePropertyClock{now: time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)},
		testOwnerPolicy{},
		nil,
	)
	svc.SetRentalDeletionGuard(guard)
	return svc, repo, journal, ownerID, propertyID
}

func TestDeleteProperty_UnfinishedRental_Conflicts(t *testing.T) {
	t.Parallel()

	svc, repo, journal, ownerID, propertyID := deleteGuardFixture(t, true, nil)

	err := svc.DeleteProperty(context.Background(), ownerID, propertyID)
	if !errors.Is(err, ErrPropertyOccupied) {
		t.Fatalf("expected ErrPropertyOccupied, got %v", err)
	}
	if _, ok := repo.data[propertyID]; !ok {
		t.Fatal("property must survive a blocked delete")
	}
	for _, event := range *journal {
		if event == "guard.DeleteByProperty" {
			t.Fatal("rentals teardown must not run behind a blocked delete")
		}
	}
}

func TestDeleteProperty_CompletedRentals_TeardownRunsBeforePropertyRow(t *testing.T) {
	t.Parallel()

	svc, repo, journal, ownerID, propertyID := deleteGuardFixture(t, false, nil)

	if err := svc.DeleteProperty(context.Background(), ownerID, propertyID); err != nil {
		t.Fatalf("DeleteProperty: %v", err)
	}
	if _, ok := repo.data[propertyID]; ok {
		t.Fatal("expected the property to be deleted")
	}
	got := *journal
	want := []string{"guard.HasUnfinished", "guard.DeleteByProperty", "repo.Delete"}
	if len(got) != len(want) {
		t.Fatalf("journal = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("journal = %v, want %v", got, want)
		}
	}
}

func TestDeleteProperty_GuardFailure_BlocksDelete(t *testing.T) {
	t.Parallel()

	svc, repo, _, ownerID, propertyID := deleteGuardFixture(t, false, errors.New("rentals store unavailable"))

	if err := svc.DeleteProperty(context.Background(), ownerID, propertyID); err == nil {
		t.Fatal("expected the guard failure to fail the delete")
	}
	if _, ok := repo.data[propertyID]; !ok {
		t.Fatal("property must survive a failed guard")
	}
}
