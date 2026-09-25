package application

// The realtime seam's tests of the properties context (карта #714, #716;
// ADR 0062): the committed object mutations dispatch their property pair —
// the history pair piggybacking when the mutation journaled a row — strictly
// post-commit. (The delete dispatches no pair for its own row — post-commit
// the object row is gone and the derived access resolves to nobody — but the
// slot recovery's reactivated legs live in other owners' objects, and their
// access pairs dispatch the same way.)

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	accessdomain "github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	realtimetest "github.com/nambers/arenda-planform/apps/backend/internal/realtime/realtimetest"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bindPropertyRealtime late-binds the recording carrier — the realtime
// seam's test double (карта #714, #716).
func bindPropertyRealtime(t *testing.T, svc *PropertyService) *realtimetest.RecordingPublisher {
	t.Helper()
	realtime := &realtimetest.RecordingPublisher{}
	svc.SetRealtimePublisher(realtime)
	return realtime
}

func TestCreatePropertyPublishesPropertyAndHistoryFrames(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.Must(uuid.NewV7())
	repo := newFakePropertyRepo()
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		newPropertyTestFactory(repo, fakePropertyPhotoRepo{}, fakeSubscriptionLimiter{limit: 100}),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
		nil,
	)
	realtime := bindPropertyRealtime(t, svc)

	created, err := svc.CreateProperty(ctx, ownerID, CreatePropertyCommand{
		Name:    "Квартира",
		Type:    string(domain.PropertyTypeApartment),
		Address: "Москва, Тверская 1",
	})
	require.NoError(t, err)

	require.Len(t, realtime.Publications, 1)
	assert.Equal(t, ownerID, realtime.Publications[0].Actor)
	assert.Equal(t,
		[]string{"property:" + created.ID.String(), "history:" + created.ID.String()},
		realtime.Pairs())
}

// TestUpdatePropertyPublishesFrames pins the edit; a no-op journal still
// dirties the property view itself.
func TestUpdatePropertyPublishesFrames(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.Must(uuid.NewV7())
	property := domain.Property{
		ID:      uuid.Must(uuid.NewV7()),
		OwnerID: ownerID,
		Name:    testPropertyName,
		Address: testPropertyAddress,
		Type:    domain.PropertyTypeApartment,
		Status:  domain.PropertyStatusActive,
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
	realtime := bindPropertyRealtime(t, svc)

	name := "Новое имя"
	_, err := svc.UpdateProperty(ctx, ownerID, property.ID, UpdatePropertyCommand{Name: &name})
	require.NoError(t, err)

	require.NotEmpty(t, realtime.Publications)
	assert.Contains(t, realtime.Pairs(), "property:"+property.ID.String())
}

// TestArchivedMutationPublishesNothing pins the post-commit canon: the
// archived state rejects the mutation inside its transaction — no frames.
func TestArchivedMutationPublishesNothing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.Must(uuid.NewV7())
	property := domain.Property{
		ID:      uuid.Must(uuid.NewV7()),
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
	realtime := bindPropertyRealtime(t, svc)

	name := "Новое имя"
	_, err := svc.UpdateProperty(ctx, ownerID, property.ID, UpdatePropertyCommand{Name: &name})
	require.ErrorIs(t, err, ErrArchivedProperty)
	assert.Empty(t, realtime.Publications)
}

// recoverySlotPolicy is the RecipientSlotPolicy double whose recoveries hand
// back pre-seeded memberships: the fixture for the properties-side recovery
// dispatch (карта #714, #716; ADR 0062 §3) — the reactivated rows are the
// access legs whose propertyIds the service must collect into access pairs.
type recoverySlotPolicy struct {
	// forProperty legs keyed by the archived/deleted property's id.
	forProperty map[uuid.UUID][]accessdomain.Membership
	// afterDelete legs keyed the same way, for the delete path.
	afterDelete map[uuid.UUID][]accessdomain.Membership
	// onUnarchive legs keyed by the unarchived property's id — the
	// memberships the enforcement suspends on the pool re-entry, the mirror
	// image of the recoveries.
	onUnarchive map[uuid.UUID][]accessdomain.Membership
	// forRecipient legs of the owner's own suspended queue — returned on
	// every RecoverSuspended call, as the real queue would be.
	forRecipient []accessdomain.Membership
}

func (p *recoverySlotPolicy) RecoverSuspendedForProperty(_ context.Context, _ transaction.Tx, propertyID uuid.UUID) ([]accessdomain.Membership, error) {
	return p.forProperty[propertyID], nil
}

func (p *recoverySlotPolicy) EnforceOnUnarchiveForProperty(_ context.Context, _ transaction.Tx, propertyID uuid.UUID) ([]accessdomain.Membership, error) {
	return p.onUnarchive[propertyID], nil
}

func (p *recoverySlotPolicy) RecoverAfterPropertyDelete(_ context.Context, _ transaction.Tx, propertyID uuid.UUID) ([]accessdomain.Membership, error) {
	return p.afterDelete[propertyID], nil
}

func (p *recoverySlotPolicy) RecoverSuspended(context.Context, transaction.Tx, uuid.UUID) ([]accessdomain.Membership, error) {
	return p.forRecipient, nil
}

var _ RecipientSlotPolicy = (*recoverySlotPolicy)(nil)

// suspendedLegOn seeds one reactivated access leg on a foreign object — the
// id is the only field the recovery dispatch reads.
func suspendedLegOn(propertyID uuid.UUID) accessdomain.Membership {
	return accessdomain.Membership{
		ID:         uuid.Must(uuid.NewV7()),
		PropertyID: propertyID,
		UserID:     uuid.Must(uuid.NewV7()),
		Role:       accessdomain.RoleViewer,
		Status:     accessdomain.MemberStatusSuspended,
	}
}

// distinctPairs drops repeated pairs while preserving the order. Collapse is
// the carrier's contract (publisher_test.go): the seam forwards the raw pair
// batch, and the recording double captures it without the carrier's dedup —
// so the assertions compare the distinct pairs the real carrier would emit.
func distinctPairs(pairs []string) []string {
	seen := make(map[string]struct{}, len(pairs))
	out := make([]string, 0, len(pairs))
	for _, p := range pairs {
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

// TestUnarchivePropertyPublishesFrames closes the unarchive into the dispatch
// canon: the journaled unarchive piggybacks its history pair next to the
// property pair, and the over-limit legs the slot enforcement suspends on the
// pool re-entry dispatch their access pairs post-commit — the mirror image of
// the archive's recovery pairs (карта #714, #716; ADR 0062 §3).
func TestUnarchivePropertyPublishesFrames(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.Must(uuid.NewV7())
	foreign := uuid.Must(uuid.NewV7()) // the suspended leg's object

	repo := newFakePropertyRepo()
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		newPropertyTestFactory(repo, fakePropertyPhotoRepo{}, fakeSubscriptionLimiter{limit: 100}),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
		nil,
	)

	created, err := svc.CreateProperty(ctx, ownerID, CreatePropertyCommand{
		Name:    "Квартира",
		Type:    string(domain.PropertyTypeApartment),
		Address: "Москва, Тверская 1",
	})
	require.NoError(t, err)
	_, err = svc.ArchiveProperty(ctx, ownerID, created.ID)
	require.NoError(t, err)

	// The carrier binds after the setup mutations: this test asserts the
	// unarchive's own dispatch, not the create/archive frames.
	realtime := bindPropertyRealtime(t, svc)
	svc.SetRecipientSlotPolicy(&recoverySlotPolicy{
		onUnarchive: map[uuid.UUID][]accessdomain.Membership{
			created.ID: {suspendedLegOn(foreign)},
		},
	})

	_, err = svc.UnarchiveProperty(ctx, ownerID, created.ID)
	require.NoError(t, err)

	require.Len(t, realtime.Publications, 2)
	assert.Equal(t, ownerID, realtime.Publications[0].Actor)
	assert.Equal(t,
		[]string{
			"property:" + created.ID.String(),
			"history:" + created.ID.String(),
			"access:" + foreign.String(),
		},
		realtime.Pairs())
}

// TestArchivePropertyPublishesRecoveryAccessPairs extends the archive
// dispatch with the properties-side recovery pairs (карта #714, #716; ADR
// 0062 §3): freeing the recipients' slots restores their suspended legs on
// other objects, and each restored object's participants view is dirtied —
// several legs on one object ride the batch as repeated pairs, the
// owner-tail recovery (the owner's own suspended queue in yet other owners'
// objects) rides the same dispatch, and the archive's own property and
// history pairs stay singular.
func TestArchivePropertyPublishesRecoveryAccessPairs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.Must(uuid.NewV7())
	propertyID := uuid.Must(uuid.NewV7())
	foreignA := uuid.Must(uuid.NewV7()) // two restored legs on one object
	foreignB := uuid.Must(uuid.NewV7()) // the owner-tail leg's object

	repo := newFakePropertyRepo(domain.Property{
		ID: propertyID, OwnerID: ownerID, Name: testPropertyName, Address: testPropertyAddress,
		Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
	})
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		newPropertyTestFactory(repo, fakePropertyPhotoRepo{}, nil),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
		nil,
	)
	svc.SetRecipientSlotPolicy(&recoverySlotPolicy{
		forProperty:  map[uuid.UUID][]accessdomain.Membership{propertyID: {suspendedLegOn(foreignA), suspendedLegOn(foreignA)}},
		forRecipient: []accessdomain.Membership{suspendedLegOn(foreignB)},
	})
	realtime := bindPropertyRealtime(t, svc)

	_, err := svc.ArchiveProperty(ctx, ownerID, propertyID)
	require.NoError(t, err)

	// The seam forwards the raw batch — the two foreignA legs stay two pairs
	// in the recording double; collapse is the carrier's contract
	// (publisher_test.go), so the assertion compares the distinct pairs.
	assert.Equal(t,
		[]string{
			"property:" + propertyID.String(),
			"history:" + propertyID.String(),
			"access:" + foreignA.String(),
			"access:" + foreignB.String(),
		},
		distinctPairs(realtime.Pairs()))
}

// TestDeletePropertyPublishesRecoveryAccessPairs extends the delete with the
// recovery pairs: the object's own row dispatches nothing by construction,
// but the freed slots restore suspended legs on other objects — their access
// pairs dispatch post-commit like in the archive path.
func TestDeletePropertyPublishesRecoveryAccessPairs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.Must(uuid.NewV7())
	propertyID := uuid.Must(uuid.NewV7())
	foreignA := uuid.Must(uuid.NewV7()) // two restored legs on one object
	foreignB := uuid.Must(uuid.NewV7()) // the owner-tail leg's object

	repo := newFakePropertyRepo(domain.Property{
		ID: propertyID, OwnerID: ownerID, Name: testPropertyName, Address: testPropertyAddress,
		Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
	})
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		newPropertyTestFactory(repo, fakePropertyPhotoRepo{}, nil),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
		nil,
	)
	svc.SetRecipientSlotPolicy(&recoverySlotPolicy{
		afterDelete:  map[uuid.UUID][]accessdomain.Membership{propertyID: {suspendedLegOn(foreignA), suspendedLegOn(foreignA)}},
		forRecipient: []accessdomain.Membership{suspendedLegOn(foreignB)},
	})
	realtime := bindPropertyRealtime(t, svc)

	require.NoError(t, svc.DeleteProperty(ctx, ownerID, propertyID))

	// Collapse is the carrier's contract (publisher_test.go): the seam
	// forwards the raw batch, the assertion compares the distinct pairs.
	assert.Equal(t,
		[]string{
			"access:" + foreignA.String(),
			"access:" + foreignB.String(),
		},
		distinctPairs(realtime.Pairs()))
}

// TestArchiveExcessPropertiesPublishesRecoveryAccessPairs extends the billing
// auto-archive with the recovery pairs: every archived property's freed slots
// restore suspended legs on other objects, and the owner-tail queue recovery
// runs once per archived property — the whole batch collapses into one
// dispatch of distinct access pairs (карта #714, #716; ADR 0062 §3).
func TestArchiveExcessPropertiesPublishesRecoveryAccessPairs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.Must(uuid.NewV7())
	excessA := uuid.Must(uuid.NewV7()) // Updated 06-02, archived first.
	excessB := uuid.Must(uuid.NewV7()) // Updated 06-01, archived second.
	foreignA := uuid.Must(uuid.NewV7()) // two restored legs on one object
	foreignB := uuid.Must(uuid.NewV7()) // the second archive's leg
	foreignC := uuid.Must(uuid.NewV7()) // the owner-tail leg, restored twice

	repo := newFakePropertyRepo(
		domain.Property{
			ID: excessA, OwnerID: ownerID, Name: "Excess A", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
			UpdatedAt: time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
		},
		domain.Property{
			ID: excessB, OwnerID: ownerID, Name: "Excess B", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
			UpdatedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		},
	)
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		newPropertyTestFactory(repo, fakePropertyPhotoRepo{}, nil),
		fakePropertyClock{now: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)},
		testOwnerPolicy{},
		nil,
	)
	svc.SetRecipientSlotPolicy(&recoverySlotPolicy{
		forProperty: map[uuid.UUID][]accessdomain.Membership{
			excessA: {suspendedLegOn(foreignA), suspendedLegOn(foreignA)},
			excessB: {suspendedLegOn(foreignB)},
		},
		forRecipient: []accessdomain.Membership{suspendedLegOn(foreignC)},
	})
	realtime := bindPropertyRealtime(t, svc)

	archived, err := svc.ArchiveExcessProperties(ctx, &fakePropertyTx{}, ownerID, 0, nil)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{excessA, excessB}, archived)

	// The capture order per archived property: its per-property legs, then the
	// owner tail — so C (restored first at excessA) precedes B (excessB's leg).
	// Collapse is the carrier's contract (publisher_test.go): the seam
	// forwards the raw batch, the assertion compares the distinct pairs.
	assert.Equal(t,
		[]string{
			"access:" + foreignA.String(),
			"access:" + foreignC.String(),
			"access:" + foreignB.String(),
		},
		distinctPairs(realtime.Pairs()))
}
