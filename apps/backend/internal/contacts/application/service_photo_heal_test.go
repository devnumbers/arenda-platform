package application

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/objectstorage"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/photo"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/stretchr/testify/require"
)

// The dangling-card-photo self-heal (ADR 0065): the storage has lost the
// object the columns advertise — the stream still answers the honest 404,
// and the dangling key is repaired so the book stops promising a photo the
// storage will never deliver.

func TestContactService_OpenContactPhoto_HealsDanglingKey(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t, sharedpolicy.RoleOwner)
	// The service is rebuilt around a harness-owned storage: the test must
	// be able to lose an object behind the backend's back.
	photos := objectstorage.NewFakeStorage()
	h.svc = NewContactService(
		NewTxStoreFactory(
			h.contacts,
			fakePropertyStore{owners: map[uuid.UUID]uuid.UUID{h.property: h.owner}},
			h.audit,
			h.history,
			fakeUoW{},
		),
		stubPolicy{role: sharedpolicy.RoleOwner},
		photos,
	)

	created, err := h.svc.CreateContact(context.Background(), h.owner, createCmd(nil))
	require.NoError(t, err, "create contact")

	key := "photos/" + uuid.Must(uuid.NewV7()).String() + ".jpg"
	contentType := photo.ContentTypeJPEG
	require.NoError(t, photos.Put(context.Background(), key, strings.NewReader("jpeg-bytes"), contentType, 10))
	_, err = h.contacts.SetPhoto(context.Background(), created.ID, h.owner, &key, &contentType)
	require.NoError(t, err, "seed photo")

	// The object vanishes behind the backend's back.
	require.NoError(t, photos.Delete(context.Background(), key))

	body, size, _, key, err := h.svc.OpenContactPhoto(context.Background(), h.owner, created.ID)
	require.ErrorIs(t, err, ErrPhotoNotFound)
	require.Nil(t, body)
	require.Zero(t, size)
	require.Empty(t, key)

	stored, err := h.contacts.GetByID(context.Background(), created.ID)
	require.NoError(t, err)
	require.Nil(t, stored.PhotoKey, "the dangling key must be healed away")
	require.Nil(t, stored.PhotoContentType)
}
