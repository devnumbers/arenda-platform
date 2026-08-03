package application

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type fakePhotoStorage struct {
	uploadURL    string
	uploadErr    error
	uploadedSize int64
}

func (s *fakePhotoStorage) Upload(_ context.Context, _, _ string, size int64, _ io.Reader) (string, error) {
	s.uploadedSize = size
	if s.uploadErr != nil {
		return "", s.uploadErr
	}
	return s.uploadURL, nil
}

func (s *fakePhotoStorage) Delete(_ context.Context, _ string) error {
	return nil
}

func (s *fakePhotoStorage) HeadBucket(_ context.Context) error {
	return nil
}

type fakePhotoRepo struct {
	photos      map[uuid.UUID][]domain.Photo
	createErr   error
	count       int
	countErr    error
	getErr      error
	getByIDsErr error
}

func (r *fakePhotoRepo) Create(_ context.Context, _ uuid.UUID, propertyID uuid.UUID, url string) (domain.Photo, error) {
	if r.createErr != nil {
		return domain.Photo{}, r.createErr
	}
	photo := domain.Photo{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), URL: url}
	r.photos[propertyID] = append(r.photos[propertyID], photo)
	return photo, nil
}

func (r *fakePhotoRepo) GetByPropertyID(_ context.Context, propertyID uuid.UUID) ([]domain.Photo, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	return r.photos[propertyID], nil
}

func (r *fakePhotoRepo) GetByPropertyIDs(_ context.Context, propertyIDs []uuid.UUID) (map[uuid.UUID][]domain.Photo, error) {
	if r.getByIDsErr != nil {
		return nil, r.getByIDsErr
	}
	result := make(map[uuid.UUID][]domain.Photo, len(propertyIDs))
	for _, id := range propertyIDs {
		result[id] = r.photos[id]
	}
	return result, nil
}

func (r *fakePhotoRepo) CountByPropertyID(_ context.Context, _ uuid.UUID) (int, error) {
	if r.countErr != nil {
		return 0, r.countErr
	}
	return r.count, nil
}

func (r *fakePhotoRepo) GetByID(_ context.Context, _ uuid.UUID) (domain.Photo, error) {
	return domain.Photo{}, nil
}

func (r *fakePhotoRepo) GetByIDAndPropertyID(_ context.Context, _, _ uuid.UUID) (domain.Photo, error) {
	return domain.Photo{}, nil
}

func (r *fakePhotoRepo) Delete(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (r *fakePhotoRepo) WithTx(_ transaction.Tx) PropertyPhotoRepository {
	return r
}

func newPhotoService(t *testing.T, repo PropertyRepository, photoRepo PropertyPhotoRepository, storage PhotoStorage) *PropertyService {
	t.Helper()
	return NewPropertyService(
		repo,
		photoRepo,
		storage,
		fakeOccupancyProvider{},
		fakeSubscriptionLimiter{limit: 10},
		fakePropertyBillingLifecycle{},
		stubLeaseRepo{},
		fakePropertyTxBeginner{},
		nil,
		fakePropertyClock{now: time.Now()},
		fakeTzResolver{},
		nil,
	)
}

func TestAddPropertyPhoto_InvalidContentType(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	repo := newLockingFakePropertyRepo(domain.Property{ID: propertyID, OwnerID: ownerID})
	svc := newPhotoService(t, repo, &fakePhotoRepo{}, &fakePhotoStorage{})

	_, err := svc.AddPropertyPhoto(ctx, ownerID, propertyID, bytes.NewReader([]byte("x")), "file.gif", "image/gif", 100)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestAddPropertyPhoto_FileTooLarge(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	repo := newLockingFakePropertyRepo(domain.Property{ID: propertyID, OwnerID: ownerID})
	svc := newPhotoService(t, repo, &fakePhotoRepo{}, &fakePhotoStorage{})

	_, err := svc.AddPropertyPhoto(ctx, ownerID, propertyID, bytes.NewReader([]byte("x")), "file.jpg", "image/jpeg", maxPhotoSize+1)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestAddPropertyPhoto_PropertyNotFound(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	repo := newLockingFakePropertyRepo(domain.Property{})
	svc := newPhotoService(t, repo, &fakePhotoRepo{}, &fakePhotoStorage{})

	_, err := svc.AddPropertyPhoto(ctx, ownerID, propertyID, bytes.NewReader([]byte("x")), "file.jpg", "image/jpeg", 100)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAddPropertyPhoto_PhotoLimitReached(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	repo := newLockingFakePropertyRepo(domain.Property{ID: propertyID, OwnerID: ownerID})
	photoRepo := &fakePhotoRepo{count: maxPhotoCount, photos: map[uuid.UUID][]domain.Photo{}}
	svc := newPhotoService(t, repo, photoRepo, &fakePhotoStorage{})

	_, err := svc.AddPropertyPhoto(ctx, ownerID, propertyID, bytes.NewReader([]byte("x")), "file.jpg", "image/jpeg", 100)
	if !errors.Is(err, ErrPhotoLimitReached) {
		t.Fatalf("expected ErrPhotoLimitReached, got %v", err)
	}
}

func TestAddPropertyPhoto_Success(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	repo := newLockingFakePropertyRepo(domain.Property{ID: propertyID, OwnerID: ownerID})
	photoRepo := &fakePhotoRepo{count: 0, photos: map[uuid.UUID][]domain.Photo{}}
	storage := &fakePhotoStorage{uploadURL: "https://cdn.example.com/properties/22222222-2222-2222-2222-222222222222/photo.jpg"}
	svc := newPhotoService(t, repo, photoRepo, storage)

	property, err := svc.AddPropertyPhoto(ctx, ownerID, propertyID, bytes.NewReader([]byte("image-data")), "file.jpg", "image/jpeg", 100)
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	if len(property.Photos) != 1 {
		t.Fatalf("expected 1 photo, got %d", len(property.Photos))
	}
	if property.Photos[0].URL != storage.uploadURL {
		t.Errorf("photo url = %q, want %q", property.Photos[0].URL, storage.uploadURL)
	}
	if property.Photos[0].ID == uuid.Nil {
		t.Error("expected non-nil photo id")
	}
}

func TestAddPropertyPhoto_PassesSizeToStorage(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	repo := newLockingFakePropertyRepo(domain.Property{ID: propertyID, OwnerID: ownerID})
	photoRepo := &fakePhotoRepo{count: 0, photos: map[uuid.UUID][]domain.Photo{}}
	storage := &fakePhotoStorage{uploadURL: "https://cdn.example.com/properties/22222222-2222-2222-2222-222222222222/photo.jpg"}
	svc := newPhotoService(t, repo, photoRepo, storage)

	wantSize := int64(12345)
	_, err := svc.AddPropertyPhoto(ctx, ownerID, propertyID, bytes.NewReader([]byte("image-data")), "file.jpg", "image/jpeg", wantSize)
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	if storage.uploadedSize != wantSize {
		t.Errorf("uploaded size = %d, want %d", storage.uploadedSize, wantSize)
	}
}

func TestAddPropertyPhoto_UploadError(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	repo := newLockingFakePropertyRepo(domain.Property{ID: propertyID, OwnerID: ownerID})
	storage := &fakePhotoStorage{uploadErr: errors.New("upload failed")}
	svc := newPhotoService(t, repo, &fakePhotoRepo{photos: map[uuid.UUID][]domain.Photo{}}, storage)

	_, err := svc.AddPropertyPhoto(ctx, ownerID, propertyID, bytes.NewReader([]byte("x")), "file.jpg", "image/jpeg", 100)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWithPhotos_AttachesPhotos(t *testing.T) {
	ctx := context.Background()
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	photo := domain.Photo{ID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), URL: "https://cdn.example.com/photo.jpg"}

	repo := newLockingFakePropertyRepo(domain.Property{ID: propertyID})
	photoRepo := &fakePhotoRepo{photos: map[uuid.UUID][]domain.Photo{propertyID: {photo}}}
	svc := newPhotoService(t, repo, photoRepo, &fakePhotoStorage{})

	properties, err := svc.withPhotos(ctx, domain.Property{ID: propertyID})
	if err != nil {
		t.Fatalf("withPhotos failed: %v", err)
	}

	if len(properties) != 1 {
		t.Fatalf("expected 1 property, got %d", len(properties))
	}
	if len(properties[0].Photos) != 1 {
		t.Fatalf("expected 1 photo, got %d", len(properties[0].Photos))
	}
	if properties[0].Photos[0].URL != photo.URL {
		t.Errorf("photo url = %q, want %q", properties[0].Photos[0].URL, photo.URL)
	}
}

func TestWithPhotos_Empty(t *testing.T) {
	ctx := context.Background()
	svc := newPhotoService(t, newLockingFakePropertyRepo(domain.Property{}), &fakePhotoRepo{}, &fakePhotoStorage{})

	properties, err := svc.withPhotos(ctx)
	if err != nil {
		t.Fatalf("withPhotos failed: %v", err)
	}
	if len(properties) != 0 {
		t.Errorf("expected 0 properties, got %d", len(properties))
	}
}

func TestAddPropertyPhoto_ArchivedProperty(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	repo := newLockingFakePropertyRepo(domain.Property{ID: propertyID, OwnerID: ownerID, Status: domain.PropertyStatusArchived})
	svc := newPhotoService(t, repo, &fakePhotoRepo{photos: map[uuid.UUID][]domain.Photo{}}, &fakePhotoStorage{})

	_, err := svc.AddPropertyPhoto(ctx, ownerID, propertyID, bytes.NewReader([]byte("x")), "file.jpg", "image/jpeg", 100)
	if !errors.Is(err, ErrArchivedProperty) {
		t.Fatalf("expected ErrArchivedProperty, got %v", err)
	}
}

func TestDeletePropertyPhoto_ArchivedProperty(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	photoID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	repo := newLockingFakePropertyRepo(domain.Property{ID: propertyID, OwnerID: ownerID, Status: domain.PropertyStatusArchived})
	svc := newPhotoService(t, repo, &fakePhotoRepo{photos: map[uuid.UUID][]domain.Photo{}}, &fakePhotoStorage{})

	err := svc.DeletePropertyPhoto(ctx, ownerID, propertyID, photoID)
	if !errors.Is(err, ErrArchivedProperty) {
		t.Fatalf("expected ErrArchivedProperty, got %v", err)
	}
}
