package application

// The archived-property immutability audit (ticket #618, карта #611): the
// owner's invariant — an archived property is read-only history, no
// mutation of its data — as a behavioral net. The photo and pin rejections
// live in service_photos_test.go / service_pin_test.go; the lifecycle doors
// (archive/unarchive/delete) stay open by design and keep their own tests.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
)

func TestPropertyService_UpdateProperty_ArchivedConflict(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	property := domain.Property{
		ID:      uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		OwnerID: ownerID,
		Name:    testPropertyName,
		Address: testPropertyAddress,
		Type:    domain.PropertyTypeApartment,
		Status:  domain.PropertyStatusArchived,
	}

	repo := newLockingFakePropertyRepo(property)
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		newPropertyTestFactory(repo, fakePropertyPhotoRepo{}, nil),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
		nil,
	)

	name := "Новое имя"
	if _, err := svc.UpdateProperty(ctx, ownerID, property.ID, UpdatePropertyCommand{Name: &name}); !errors.Is(err, ErrArchivedProperty) {
		t.Errorf("archived card update err = %v, want ErrArchivedProperty", err)
	}
	stored, err := repo.GetByIDAndOwner(ctx, property.ID, ownerID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.Name != testPropertyName {
		t.Errorf("stored name = %q, want the untouched %q", stored.Name, testPropertyName)
	}
}
