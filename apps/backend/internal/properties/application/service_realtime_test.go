package application

// The realtime seam's tests of the properties context (карта #714, #716;
// ADR 0062): the committed object mutations dispatch their property pair —
// the history pair piggybacking when the mutation journaled a row — strictly
// post-commit. (The delete dispatches nothing by construction: post-commit
// the object row is gone and the derived access resolves to nobody.)

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	realtimetest "github.com/nambers/arenda-planform/apps/backend/internal/realtime/realtimetest"
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
