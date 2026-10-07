package application

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/photo"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// The property photo use cases under the single-photo contract (ADR 0065,
// ticket #1227): one image per object, validated by the shared upload seam
// before these use cases ever see the bytes, streamed back through the
// service. The TDD seams pre-agreed in the ticket — the storage port and
// the validation — live in shared/storage and shared/photo; these tests pin
// the service orchestration around them.

// smallJPEG mints a minimal processed upload: Process has already run by
// the time a use case is called, so the bytes only need to travel.
func smallJPEG(t *testing.T) photo.Processed {
	t.Helper()
	return photo.Processed{Data: []byte("jpeg-bytes"), ContentType: photo.ContentTypeJPEG, Size: 10}
}

func photoTestProperty(ownerID uuid.UUID) domain.Property {
	return domain.Property{
		ID:      uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		OwnerID: ownerID,
		Name:    testPropertyName,
		Address: testPropertyAddress,
		Type:    domain.PropertyTypeApartment,
		Status:  domain.PropertyStatusActive,
	}
}

func TestPropertyService_SetPropertyPhoto_StoresUnderUUIDKey(t *testing.T) {
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
	if err != nil {
		t.Fatalf("set photo: %v", err)
	}

	if updated.PhotoKey == nil || !strings.HasPrefix(*updated.PhotoKey, "photos/") {
		t.Fatalf("photo key = %v, want a photos/<uuid> key", updated.PhotoKey)
	}
	if updated.PhotoContentType == nil || *updated.PhotoContentType != photo.ContentTypeJPEG {
		t.Fatalf("content type = %v, want image/jpeg", updated.PhotoContentType)
	}
	if len(storage.puts) != 1 || storage.puts[0] != *updated.PhotoKey {
		t.Fatalf("storage puts = %v, want exactly the property's key", storage.puts)
	}
	if string(storage.objects[*updated.PhotoKey]) != "jpeg-bytes" {
		t.Errorf("stored bytes = %q, want jpeg-bytes", storage.objects[*updated.PhotoKey])
	}
}

func TestPropertyService_SetPropertyPhoto_ReplacesOldObject(t *testing.T) {
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

	if _, err := svc.SetPropertyPhoto(context.Background(), ownerID, property.ID, smallJPEG(t)); err != nil {
		t.Fatalf("set first photo: %v", err)
	}
	first := repo.data[property.ID].PhotoKey
	if first == nil {
		t.Fatal("first photo key missing")
	}

	second := photo.Processed{Data: []byte("second"), ContentType: photo.ContentTypePNG, Size: 6}
	updated, err := svc.SetPropertyPhoto(context.Background(), ownerID, property.ID, second)
	if err != nil {
		t.Fatalf("set second photo: %v", err)
	}

	if updated.PhotoKey == nil || *updated.PhotoKey == *first {
		t.Fatalf("replacement must mint a new key, got %v", updated.PhotoKey)
	}
	deleted := false
	for _, key := range storage.deletes {
		if key == *first {
			deleted = true
		}
	}
	if !deleted {
		t.Errorf("replaced object %s was not removed best-effort, deletes = %v", *first, storage.deletes)
	}
}

func TestPropertyService_SetPropertyPhoto_ViewerForbidden(t *testing.T) {
	t.Parallel()

	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	property := photoTestProperty(ownerID)
	repo := newLockingFakePropertyRepo(property)
	svc := NewPropertyService(
		repo,
		newFakePropertyPhotoStorage(),
		newPropertyTestFactory(repo, nil),
		fakePropertyClock{now: time.Now()},
		staticRolePolicy{role: sharedpolicy.RoleViewer},
		nil,
	)

	_, err := svc.SetPropertyPhoto(context.Background(), ownerID, property.ID, smallJPEG(t))
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden (a viewer cannot edit)", err)
	}
}

func TestPropertyService_SetPropertyPhoto_ArchivedRejected(t *testing.T) {
	t.Parallel()

	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	property := photoTestProperty(ownerID)
	property.Status = domain.PropertyStatusArchived
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

	_, err := svc.SetPropertyPhoto(context.Background(), ownerID, property.ID, smallJPEG(t))
	if !errors.Is(err, ErrArchivedProperty) {
		t.Fatalf("err = %v, want ErrArchivedProperty", err)
	}
	if len(storage.puts) != 0 {
		t.Errorf("storage puts = %v, want none (the archived guard runs before any write)", storage.puts)
	}
}

func TestPropertyService_SetPropertyPhoto_StorageFailureKeepsRowClean(t *testing.T) {
	t.Parallel()

	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	property := photoTestProperty(ownerID)
	repo := newLockingFakePropertyRepo(property)
	storage := newFakePropertyPhotoStorage()
	storage.putErr = errors.New("s3 down")
	svc := NewPropertyService(
		repo,
		storage,
		newPropertyTestFactory(repo, nil),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
		nil,
	)

	_, err := svc.SetPropertyPhoto(context.Background(), ownerID, property.ID, smallJPEG(t))
	if err == nil {
		t.Fatal("expected the storage failure to abort the mutation")
	}
	if repo.data[property.ID].PhotoKey != nil {
		t.Errorf("photo key = %v, want nil (nothing persisted on a failed put)", repo.data[property.ID].PhotoKey)
	}
}

func TestPropertyService_PhotoDescriptor_404WithoutPhoto(t *testing.T) {
	t.Parallel()

	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	property := photoTestProperty(ownerID)
	repo := newLockingFakePropertyRepo(property)
	svc := NewPropertyService(
		repo,
		newFakePropertyPhotoStorage(),
		newPropertyTestFactory(repo, nil),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
		nil,
	)

	if _, _, err := svc.PhotoDescriptor(context.Background(), ownerID, property.ID); !errors.Is(err, ErrPhotoNotFound) {
		t.Fatalf("err = %v, want ErrPhotoNotFound", err)
	}
}

func TestPropertyService_OpenPropertyPhoto_Roundtrip(t *testing.T) {
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

	if _, err := svc.SetPropertyPhoto(context.Background(), ownerID, property.ID, smallJPEG(t)); err != nil {
		t.Fatalf("set photo: %v", err)
	}

	body, size, contentType, key, err := svc.OpenPropertyPhoto(context.Background(), ownerID, property.ID)
	if err != nil {
		t.Fatalf("open photo: %v", err)
	}
	defer func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	}()
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !bytes.Equal(data, []byte("jpeg-bytes")) || size != 10 || contentType != photo.ContentTypeJPEG {
		t.Errorf("body=%q size=%d ct=%q, want jpeg-bytes/10/image/jpeg", data, size, contentType)
	}
	if repo.data[property.ID].PhotoKey == nil || key != *repo.data[property.ID].PhotoKey {
		t.Errorf("key = %q, want the stored property key", key)
	}
}

func TestPropertyService_DeletePropertyPhoto_ClearsAndCleans(t *testing.T) {
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
	if _, err := svc.SetPropertyPhoto(context.Background(), ownerID, property.ID, smallJPEG(t)); err != nil {
		t.Fatalf("set photo: %v", err)
	}
	key := *repo.data[property.ID].PhotoKey

	updated, err := svc.DeletePropertyPhoto(context.Background(), ownerID, property.ID)
	if err != nil {
		t.Fatalf("delete photo: %v", err)
	}
	if updated.PhotoKey != nil {
		t.Errorf("photo key = %v, want nil after the delete", updated.PhotoKey)
	}
	removed := false
	for _, k := range storage.deletes {
		if k == key {
			removed = true
		}
	}
	if !removed {
		t.Errorf("object %s was not removed best-effort, deletes = %v", key, storage.deletes)
	}

	if _, _, err := svc.PhotoDescriptor(context.Background(), ownerID, property.ID); !errors.Is(err, ErrPhotoNotFound) {
		t.Fatalf("descriptor after delete = %v, want ErrPhotoNotFound", err)
	}
}

func TestPropertyService_DeletePropertyPhoto_404WithoutPhoto(t *testing.T) {
	t.Parallel()

	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	property := photoTestProperty(ownerID)
	repo := newLockingFakePropertyRepo(property)
	svc := NewPropertyService(
		repo,
		newFakePropertyPhotoStorage(),
		newPropertyTestFactory(repo, nil),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
		nil,
	)

	if _, err := svc.DeletePropertyPhoto(context.Background(), ownerID, property.ID); !errors.Is(err, ErrPhotoNotFound) {
		t.Fatalf("err = %v, want ErrPhotoNotFound", err)
	}
}

func TestPropertyService_PhotoDescriptor_SuspendedIsDistinguishable(t *testing.T) {
	t.Parallel()

	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	property := photoTestProperty(ownerID)
	repo := newLockingFakePropertyRepo(property)
	svc := NewPropertyService(
		repo,
		newFakePropertyPhotoStorage(),
		newPropertyTestFactory(repo, nil),
		fakePropertyClock{now: time.Now()},
		staticRolePolicy{role: sharedpolicy.RoleSuspended},
		nil,
	)

	if _, _, err := svc.PhotoDescriptor(context.Background(), ownerID, property.ID); !errors.Is(err, ErrAccessSuspended) {
		t.Fatalf("err = %v, want ErrAccessSuspended (the one distinguishable signal)", err)
	}
}
