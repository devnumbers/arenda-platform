package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// SetPin tests (ticket #577, the PUT favorite's canon #461): the pin is an
// atomic write under the edit capability — Owner and Full Access; a re-pin
// keeps the original pin time (the PUT's idempotency — the order among the
// pinned never shifts); unpinning clears it; an archived property is 409.

func pinTestService(t *testing.T, repo PropertyRepository, policy sharedpolicy.Policy) *PropertyService {
	t.Helper()
	return NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		newPropertyTestFactory(repo, fakePropertyPhotoRepo{}, nil),
		fakePropertyClock{now: time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)},
		policy,
		nil,
	)
}

func pinTestProperty(ownerID uuid.UUID) domain.Property {
	return domain.Property{
		ID:      uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		OwnerID: ownerID,
		Name:    testPropertyName,
		Address: testPropertyAddress,
		Type:    domain.PropertyTypeApartment,
		Status:  domain.PropertyStatusActive,
	}
}

func TestPropertyService_SetPropertyPin_PinsWithClockTime(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	property := pinTestProperty(ownerID)
	repo := newFakePropertyRepo(property)
	svc := pinTestService(t, repo, testOwnerPolicy{})

	updated, err := svc.SetPropertyPin(ctx, ownerID, property.ID, true)
	if err != nil {
		t.Fatalf("set pin: %v", err)
	}
	if updated.PinnedAt == nil {
		t.Fatal("updated.PinnedAt = nil, want the clock time")
	}
	if !updated.PinnedAt.Equal(time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("updated.PinnedAt = %v, want the clock time", updated.PinnedAt)
	}
	if updated.AccessRole != sharedpolicy.RoleOwner {
		t.Errorf("updated.AccessRole = %q, want owner", updated.AccessRole)
	}
}

func TestPropertyService_SetPropertyPin_RepinKeepsOriginalTime(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	firstPin := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	property := pinTestProperty(ownerID)
	property.PinnedAt = &firstPin
	repo := newFakePropertyRepo(property)
	svc := pinTestService(t, repo, testOwnerPolicy{})

	updated, err := svc.SetPropertyPin(ctx, ownerID, property.ID, true)
	if err != nil {
		t.Fatalf("re-pin: %v", err)
	}
	if updated.PinnedAt == nil || !updated.PinnedAt.Equal(firstPin) {
		t.Errorf("updated.PinnedAt = %v, want the original %v", updated.PinnedAt, firstPin)
	}
}

func TestPropertyService_SetPropertyPin_UnpinClears(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	property := pinTestProperty(ownerID)
	repo := newFakePropertyRepo(property)
	svc := pinTestService(t, repo, testOwnerPolicy{})

	if _, err := svc.SetPropertyPin(ctx, ownerID, property.ID, true); err != nil {
		t.Fatalf("pin: %v", err)
	}
	updated, err := svc.SetPropertyPin(ctx, ownerID, property.ID, false)
	if err != nil {
		t.Fatalf("unpin: %v", err)
	}
	if updated.PinnedAt != nil {
		t.Errorf("updated.PinnedAt = %v, want nil", updated.PinnedAt)
	}
	stored, err := repo.GetByIDAndOwner(ctx, property.ID, ownerID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.PinnedAt != nil {
		t.Errorf("stored.PinnedAt = %v, want nil", stored.PinnedAt)
	}
}

func TestPropertyService_SetPropertyPin_ViewerForbidden(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	property := pinTestProperty(ownerID)
	repo := newFakePropertyRepo(property)
	svc := pinTestService(t, repo, staticRolePolicy{role: sharedpolicy.RoleViewer})

	if _, err := svc.SetPropertyPin(ctx, ownerID, property.ID, true); !errors.Is(err, ErrForbidden) {
		t.Errorf("viewer pin err = %v, want ErrForbidden", err)
	}
}

func TestPropertyService_SetPropertyPin_NoRoleNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	property := pinTestProperty(ownerID)
	repo := newFakePropertyRepo(property)
	svc := pinTestService(t, repo, staticRolePolicy{role: sharedpolicy.RoleNone})

	if _, err := svc.SetPropertyPin(ctx, ownerID, property.ID, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("stranger pin err = %v, want ErrNotFound", err)
	}
}

func TestPropertyService_SetPropertyPin_ArchivedConflict(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	property := pinTestProperty(ownerID)
	property.Status = domain.PropertyStatusArchived
	repo := newFakePropertyRepo(property)
	svc := pinTestService(t, repo, testOwnerPolicy{})

	if _, err := svc.SetPropertyPin(ctx, ownerID, property.ID, true); !errors.Is(err, ErrArchivedProperty) {
		t.Errorf("archived pin err = %v, want ErrArchivedProperty", err)
	}
}

// orderedListRepo returns a fixed order from ListActiveByOwner (the map-based
// fake's order is random) so the list-sorting test is deterministic.
type orderedListRepo struct {
	fakePropertyRepo
	list []domain.Property
}

func (r *orderedListRepo) ListActiveByOwner(_ context.Context, _ uuid.UUID) ([]domain.Property, error) {
	return append([]domain.Property(nil), r.list...), nil
}

var _ PropertyRepository = (*orderedListRepo)(nil)

// TestPropertyService_ListProperties_PinnedFirstStable pins rise above the
// unpinned in the merged list — among themselves by the pin time, the
// unpinned keeping the repository's order (ticket #577; a shared property
// pinned by its full-access member rises the same way).
func TestPropertyService_ListProperties_PinnedFirstStable(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	sharedOwnerID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	pinA := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	pinB := time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC)
	unpinned := domain.Property{
		ID:      uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		OwnerID: ownerID,
		Name:    "Unpinned",
		Address: testPropertyAddress,
		Type:    domain.PropertyTypeApartment,
		Status:  domain.PropertyStatusActive,
	}
	pinnedA := domain.Property{
		ID:       uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		OwnerID:  ownerID,
		Name:     "Pinned First",
		Address:  testPropertyAddress,
		Type:     domain.PropertyTypeApartment,
		Status:   domain.PropertyStatusActive,
		PinnedAt: &pinA,
	}
	sharedPinnedB := domain.Property{
		ID:       uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		OwnerID:  sharedOwnerID,
		Name:     "Shared Pinned",
		Address:  testPropertyAddress,
		Type:     domain.PropertyTypeApartment,
		Status:   domain.PropertyStatusActive,
		PinnedAt: &pinB,
	}

	repo := &orderedListRepo{
		fakePropertyRepo: *newFakePropertyRepo(unpinned, pinnedA, sharedPinnedB),
		list:             []domain.Property{unpinned, pinnedA},
	}
	svc := pinTestService(t, repo, testOwnerPolicy{})
	svc.SetSharedMemberships(fakeSharedMemberships{memberships: []SharedMembership{
		{PropertyID: sharedPinnedB.ID, Role: sharedpolicy.RoleFullAccess},
	}})

	list, err := svc.ListProperties(ctx, ownerID)
	if err != nil {
		t.Fatalf("list properties: %v", err)
	}
	want := []uuid.UUID{pinnedA.ID, sharedPinnedB.ID, unpinned.ID}
	if len(list) != len(want) {
		t.Fatalf("list len = %d, want %d", len(list), len(want))
	}
	for i, id := range want {
		if list[i].ID != id {
			t.Errorf("list[%d] = %s, want %s", i, list[i].ID, id)
		}
	}
}
