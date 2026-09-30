package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
)

const (
	// Явное имя в тестах «явное имя остаётся»/«отсутствие не трогает».
	autonameLuxury = "Люкс"
)

// newAutonameTestService builds the property service over a locking fake
// repo seeded with the given properties — the auto-name fixture
// (ticket #1001). The limiter is wired: CreateProperty consults it inside
// the transaction.
func newAutonameTestService(repo *lockingFakePropertyRepo) *PropertyService {
	return NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		newPropertyTestFactory(repo, fakePropertyPhotoRepo{}, fakeSubscriptionLimiter{limit: 10}),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
		nil,
	)
}

func autonameSeed(owner uuid.UUID, name string, typ domain.PropertyType, status domain.PropertyStatus) domain.Property {
	return domain.Property{
		ID:      uuid.Must(uuid.NewV7()),
		OwnerID: owner,
		Name:    name,
		Type:    typ,
		Address: testPropertyAddress,
		Status:  status,
	}
}

func TestCreateProperty_AutoNameFromEmptyInput(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	owner := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	repo := newLockingFakePropertyRepo()
	svc := newAutonameTestService(repo)

	created, err := svc.CreateProperty(ctx, owner, CreatePropertyCommand{
		Name: "", Type: string(domain.PropertyTypeApartment), Address: testPropertyAddress,
	})
	if err != nil {
		t.Fatalf("CreateProperty: %v", err)
	}
	if created.Name != "Моя квартира 1" {
		t.Fatalf("auto name = %q, want %q", created.Name, "Моя квартира 1")
	}
}

func TestCreateProperty_AutoNameCountsTypeAcrossStatuses(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	owner := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	// Правило Б: серийник — число объектов типа у владельца в любом статусе
	// (архив считается, удалённые выпали), плюс один.
	repo := newLockingFakePropertyRepo(
		autonameSeed(owner, "Квартира", domain.PropertyTypeApartment, domain.PropertyStatusActive),
		autonameSeed(owner, "Старая", domain.PropertyTypeApartment, domain.PropertyStatusArchived),
		autonameSeed(owner, "Гараж", domain.PropertyTypeGarage, domain.PropertyStatusActive),
	)
	svc := newAutonameTestService(repo)

	apartment, err := svc.CreateProperty(ctx, owner, CreatePropertyCommand{
		Type: string(domain.PropertyTypeApartment), Address: testPropertyAddress,
	})
	if err != nil {
		t.Fatalf("CreateProperty apartment: %v", err)
	}
	if apartment.Name != "Моя квартира 3" {
		t.Fatalf("apartment auto name = %q, want %q", apartment.Name, "Моя квартира 3")
	}

	garage, err := svc.CreateProperty(ctx, owner, CreatePropertyCommand{
		Type: string(domain.PropertyTypeGarage), Address: testPropertyAddress,
	})
	if err != nil {
		t.Fatalf("CreateProperty garage: %v", err)
	}
	if garage.Name != "Мой гараж 2" {
		t.Fatalf("garage auto name = %q, want %q", garage.Name, "Мой гараж 2")
	}
}

func TestCreateProperty_ExplicitNameKept(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	owner := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	repo := newLockingFakePropertyRepo()
	svc := newAutonameTestService(repo)

	created, err := svc.CreateProperty(ctx, owner, CreatePropertyCommand{
		Name: "Люкс", Type: string(domain.PropertyTypeApartment), Address: testPropertyAddress,
	})
	if err != nil {
		t.Fatalf("CreateProperty: %v", err)
	}
	if created.Name != "Люкс" {
		t.Fatalf("explicit name = %q, want %q", created.Name, autonameLuxury)
	}
}

func TestUpdateProperty_EmptyNameRegenerates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	owner := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	property := autonameSeed(owner, autonameLuxury, domain.PropertyTypeApartment, domain.PropertyStatusActive)
	property.ID = propertyID
	repo := newLockingFakePropertyRepo(property)
	svc := newAutonameTestService(repo)

	// Сам объект считается в серийнике: единственная квартира, очищенная
	// при правке, становится «Моя квартира 2» (она уже вторая созданная).
	empty := ""
	updated, err := svc.UpdateProperty(ctx, owner, propertyID, UpdatePropertyCommand{
		Name: &empty,
	})
	if err != nil {
		t.Fatalf("UpdateProperty: %v", err)
	}
	if updated.Name != "Моя квартира 2" {
		t.Fatalf("regenerated name = %q, want %q", updated.Name, "Моя квартира 2")
	}
}

func TestUpdateProperty_AbsentNameKept(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	owner := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	property := autonameSeed(owner, autonameLuxury, domain.PropertyTypeApartment, domain.PropertyStatusActive)
	property.ID = propertyID
	repo := newLockingFakePropertyRepo(property)
	svc := newAutonameTestService(repo)

	address := "Новый адрес"
	updated, err := svc.UpdateProperty(ctx, owner, propertyID, UpdatePropertyCommand{
		Address: &address,
	})
	if err != nil {
		t.Fatalf("UpdateProperty: %v", err)
	}
	if updated.Name != autonameLuxury {
		t.Fatalf("absent name changed to %q, want %q", updated.Name, "Люкс")
	}
}

func TestUpdateProperty_EmptyNameAfterTypeChangeCountsNewType(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	owner := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	property := autonameSeed(owner, autonameLuxury, domain.PropertyTypeApartment, domain.PropertyStatusActive)
	property.ID = propertyID
	repo := newLockingFakePropertyRepo(
		property,
		autonameSeed(owner, "Гараж 1", domain.PropertyTypeGarage, domain.PropertyStatusActive),
		autonameSeed(owner, "Гараж 2", domain.PropertyTypeGarage, domain.PropertyStatusArchived),
	)
	svc := newAutonameTestService(repo)

	// Тип меняется тем же PATCH, что и очистка имени: серийник считается по
	// новому типу.
	garage := "garage"
	empty := ""
	updated, err := svc.UpdateProperty(ctx, owner, propertyID, UpdatePropertyCommand{
		Type: &garage,
		Name: &empty,
	})
	if err != nil {
		t.Fatalf("UpdateProperty: %v", err)
	}
	if updated.Name != "Мой гараж 3" {
		t.Fatalf("regenerated name = %q, want %q", updated.Name, "Мой гараж 3")
	}
}
