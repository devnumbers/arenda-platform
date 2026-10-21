package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// The dangling-object-photo self-heal (ADR 0065): the storage has lost the
// object the columns advertise — the stream still answers the honest 404,
// and the dangling key is repaired so the object DTOs stop promising a
// photo the storage will never deliver.

func TestPropertyService_OpenPropertyPhoto_HealsDanglingKey(t *testing.T) {
	t.Parallel()

	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	property := photoTestProperty(ownerID)
	repo := newLockingFakePropertyRepo(property)
	storage := newFakePropertyPhotoStorage()
	svc := NewPropertyService(
		repo,
		storage,
		newPropertyTestFactory(repo, nil),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
		nil,
	)

	updated, err := svc.SetPropertyPhoto(context.Background(), ownerID, property.ID, smallJPEG(t))
	require.NoError(t, err, "set photo")
	// The object vanishes behind the backend's back.
	require.NoError(t, storage.Delete(context.Background(), *updated.PhotoKey))

	body, size, _, key, err := svc.OpenPropertyPhoto(context.Background(), ownerID, property.ID, false)
	require.ErrorIs(t, err, ErrPhotoNotFound)
	require.Nil(t, body)
	require.Zero(t, size)
	require.Empty(t, key)

	stored, err := repo.GetByID(context.Background(), property.ID)
	require.NoError(t, err)
	require.Nil(t, stored.PhotoKey, "the dangling key must be healed away")
	require.Nil(t, stored.PhotoContentType)
}
