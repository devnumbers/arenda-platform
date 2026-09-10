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

// fakeRentalOccupancy returns a fixed per-property occupancy and records the
// owners map it was called with (ticket #585).
type fakeRentalOccupancy struct {
	res   map[uuid.UUID]domain.Occupancy
	err   error
	calls int
	got   PropertyOwners
}

func (f *fakeRentalOccupancy) OccupancyByProperty(
	_ context.Context, owners PropertyOwners,
) (map[uuid.UUID]domain.Occupancy, error) {
	f.calls++
	f.got = owners
	return f.res, f.err
}

// fakeOverdueOps returns a fixed per-property overdue flag set and records
// the owners map it was called with (ticket #585).
type fakeOverdueOps struct {
	res   map[uuid.UUID]bool
	err   error
	calls int
	got   PropertyOwners
}

func (f *fakeOverdueOps) OverdueByProperty(
	_ context.Context, owners PropertyOwners,
) (map[uuid.UUID]bool, error) {
	f.calls++
	f.got = owners
	return f.res, f.err
}

func occupancyTestService(t *testing.T, repo PropertyRepository) *PropertyService {
	t.Helper()
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		newPropertyTestFactory(repo, fakePropertyPhotoRepo{}, nil),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
		nil,
	)
	return svc
}

// listByIDs runs the list use case and indexes the rows by property id.
func listByIDs(t *testing.T, svc *PropertyService, actor uuid.UUID) map[uuid.UUID]domain.Property {
	t.Helper()
	result, err := svc.ListProperties(context.Background(), actor)
	if err != nil {
		t.Fatalf("ListProperties failed: %v", err)
	}
	byID := make(map[uuid.UUID]domain.Property, len(result))
	for _, p := range result {
		byID[p.ID] = p
	}
	return byID
}

// TestListProperties_OccupancyProjection verifies that the list fills the
// per-property occupancy and the overdue flag through the two batched
// projection ports, one call each (ticket #585).
func TestListProperties_OccupancyProjection(t *testing.T) {
	t.Parallel()

	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ownID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	repo := scopedPropertyRepo{newFakePropertyRepo(
		domain.Property{
			ID: ownID, OwnerID: ownerID, Name: testPropertyName, Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
		},
	)}
	svc := occupancyTestService(t, repo)

	plannedEnd := time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC)
	occupancy := &fakeRentalOccupancy{res: map[uuid.UUID]domain.Occupancy{
		ownID: {Status: domain.OccupancyActive, PlannedEndDate: &plannedEnd},
	}}
	overdue := &fakeOverdueOps{res: map[uuid.UUID]bool{ownID: true}}
	svc.SetRentalOccupancyReader(occupancy)
	svc.SetOverdueOperationsReader(overdue)

	byID := listByIDs(t, svc, ownerID)

	got := byID[ownID].Occupancy
	if got == nil || got.Status != domain.OccupancyActive ||
		got.PlannedEndDate == nil || !got.PlannedEndDate.Equal(plannedEnd) {
		t.Errorf("own occupancy = %v, want active with planned end %v", got, plannedEnd)
	}
	if !byID[ownID].HasOverdueOperations {
		t.Errorf("own HasOverdueOperations = false, want true")
	}
	if occupancy.calls != 1 || overdue.calls != 1 {
		t.Errorf("projection reads = %d/%d, want one batched call each", occupancy.calls, overdue.calls)
	}
}

// TestListProperties_OccupancyOwnersMap verifies the owners map travels per
// data owner — the shared row arrives with its own owner, not the actor —
// and a property the reader knows nothing about reports OccupancyNone
// (ticket #585).
func TestListProperties_OccupancyOwnersMap(t *testing.T) {
	t.Parallel()

	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	otherOwnerID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	ownID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	sharedID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	repo := scopedPropertyRepo{newFakePropertyRepo(
		domain.Property{
			ID: ownID, OwnerID: ownerID, Name: testPropertyName, Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
		},
		domain.Property{
			ID: sharedID, OwnerID: otherOwnerID, Name: testPropertyName, Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
		},
	)}
	svc := occupancyTestService(t, repo)
	svc.SetSharedMemberships(fakeSharedMemberships{memberships: []SharedMembership{
		{PropertyID: sharedID, Role: sharedpolicy.RoleViewer},
	}})

	occupancy := &fakeRentalOccupancy{res: map[uuid.UUID]domain.Occupancy{}}
	overdue := &fakeOverdueOps{res: map[uuid.UUID]bool{}}
	svc.SetRentalOccupancyReader(occupancy)
	svc.SetOverdueOperationsReader(overdue)

	byID := listByIDs(t, svc, ownerID)

	for _, tc := range []struct {
		name string
		got  PropertyOwners
	}{{"occupancy", occupancy.got}, {"overdue", overdue.got}} {
		if tc.got[ownID] != ownerID || tc.got[sharedID] != otherOwnerID {
			t.Errorf("%s owners map = %v, want own→owner, shared→otherOwner", tc.name, tc.got)
		}
	}

	// The reader answered nothing: both rows must report the explicit none.
	if got := byID[ownID].Occupancy; got == nil || got.Status != domain.OccupancyNone {
		t.Errorf("own occupancy = %v, want reported none", got)
	}
	if got := byID[sharedID].Occupancy; got == nil || got.Status != domain.OccupancyNone {
		t.Errorf("shared occupancy = %v, want reported none", got)
	}
	if byID[ownID].HasOverdueOperations || byID[sharedID].HasOverdueOperations {
		t.Errorf("overdue flags = %v/%v, want false/false",
			byID[ownID].HasOverdueOperations, byID[sharedID].HasOverdueOperations)
	}
}

// TestListProperties_OccupancyNotWired verifies the optional-ports contract:
// without the readers the list stays loadable and the projections stay
// unreported (nil occupancy, false overdue) — the pre-#585 shape.
func TestListProperties_OccupancyNotWired(t *testing.T) {
	t.Parallel()

	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ownID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	repo := scopedPropertyRepo{newFakePropertyRepo(
		domain.Property{
			ID: ownID, OwnerID: ownerID, Name: testPropertyName, Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
		},
	)}
	svc := occupancyTestService(t, repo)

	result, err := svc.ListProperties(context.Background(), ownerID)
	if err != nil {
		t.Fatalf("ListProperties failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 property, got %d", len(result))
	}
	if result[0].Occupancy != nil {
		t.Errorf("occupancy = %v, want unreported (nil)", result[0].Occupancy)
	}
	if result[0].HasOverdueOperations {
		t.Errorf("HasOverdueOperations = true, want false")
	}
}

// TestListArchivedProperties_OccupancyProjection verifies the archived list
// enriches its rows the same way (ticket #585).
func TestListArchivedProperties_OccupancyProjection(t *testing.T) {
	t.Parallel()

	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	archivedID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	repo := scopedPropertyRepo{newFakePropertyRepo(
		domain.Property{
			ID: archivedID, OwnerID: ownerID, Name: testPropertyName, Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusArchived,
		},
	)}
	svc := occupancyTestService(t, repo)

	svc.SetRentalOccupancyReader(&fakeRentalOccupancy{res: map[uuid.UUID]domain.Occupancy{
		archivedID: {Status: domain.OccupancyNeedsAttention},
	}})
	svc.SetOverdueOperationsReader(&fakeOverdueOps{res: map[uuid.UUID]bool{archivedID: true}})

	result, err := svc.ListArchivedProperties(context.Background(), ownerID)
	if err != nil {
		t.Fatalf("ListArchivedProperties failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 property, got %d", len(result))
	}
	if got := result[0].Occupancy; got == nil || got.Status != domain.OccupancyNeedsAttention {
		t.Errorf("archived occupancy = %v, want needs_attention", got)
	}
	if !result[0].HasOverdueOperations {
		t.Errorf("archived HasOverdueOperations = false, want true")
	}
}

// TestListProperties_OccupancyReadError verifies a projection read failure
// fails the whole list read — the badge data is part of the response contract.
func TestListProperties_OccupancyReadError(t *testing.T) {
	t.Parallel()

	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ownID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	repo := scopedPropertyRepo{newFakePropertyRepo(
		domain.Property{
			ID: ownID, OwnerID: ownerID, Name: testPropertyName, Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
		},
	)}

	svc := occupancyTestService(t, repo)
	svc.SetRentalOccupancyReader(&fakeRentalOccupancy{err: errors.New("boom")})
	if _, err := svc.ListProperties(context.Background(), ownerID); err == nil {
		t.Fatalf("ListProperties must fail when the occupancy read fails")
	}

	svc2 := occupancyTestService(t, repo)
	svc2.SetOverdueOperationsReader(&fakeOverdueOps{err: errors.New("boom")})
	if _, err := svc2.ListProperties(context.Background(), ownerID); err == nil {
		t.Fatalf("ListProperties must fail when the overdue read fails")
	}
}
