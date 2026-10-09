package http

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/photo"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	storageshared "github.com/nambers/arenda-planform/apps/backend/internal/shared/storage"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// The wire contract of the property photo endpoints (ADR 0065, ticket
// #1227): the upload is multipart (field `file`) validated by the shared
// upload seam before any storage write; the serving endpoint answers 304
// from the key-hash ETag and streams the private bytes with a private
// cache policy.

func photoTestPropertyID() uuid.UUID {
	return uuid.MustParse("22222222-2222-2222-2222-222222222222")
}

// photoWireRepo serves one property with the given photo key.
type photoWireRepo struct {
	property domain.Property
}

func (r *photoWireRepo) Create(_ context.Context, _ uuid.UUID, p domain.Property) (domain.Property, error) {
	return p, nil
}

func (r *photoWireRepo) GetByIDAndOwner(_ context.Context, _, _ uuid.UUID) (domain.Property, error) {
	return r.property, nil
}

func (r *photoWireRepo) GetByIDAndOwnerForUpdate(_ context.Context, _, _ uuid.UUID) (domain.Property, error) {
	return r.property, nil
}

func (r *photoWireRepo) GetByID(_ context.Context, _ uuid.UUID) (domain.Property, error) {
	return r.property, nil
}

func (r *photoWireRepo) GetByIDForUpdate(_ context.Context, _ uuid.UUID) (domain.Property, error) {
	return r.property, nil
}

func (r *photoWireRepo) ListActiveByOwner(context.Context, uuid.UUID) ([]domain.Property, error) {
	return nil, nil
}

func (r *photoWireRepo) ListArchivedByOwner(context.Context, uuid.UUID) ([]domain.Property, error) {
	return nil, nil
}

func (r *photoWireRepo) SearchVisible(context.Context, uuid.UUID, propertiesapp.PropertySearchQuery) ([]domain.Property, error) {
	return nil, nil
}

func (r *photoWireRepo) Update(_ context.Context, _ uuid.UUID, p domain.Property) (domain.Property, error) {
	return p, nil
}

func (r *photoWireRepo) SetPin(_ context.Context, _, _ uuid.UUID, _ *time.Time) (domain.Property, error) {
	return r.property, nil
}

func (r *photoWireRepo) SetPropertyPhoto(_ context.Context, id, _ uuid.UUID, key, contentType *string) (domain.Property, error) {
	r.property.PhotoKey = key
	r.property.PhotoContentType = contentType
	return r.property, nil
}

func (r *photoWireRepo) Archive(context.Context, uuid.UUID, uuid.UUID) error   { return nil }
func (r *photoWireRepo) Unarchive(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (r *photoWireRepo) CountActiveByOwner(context.Context, uuid.UUID) (int, error) {
	return 0, nil
}

func (r *photoWireRepo) CountArchivedByOwner(context.Context, uuid.UUID) (int, error) {
	return 0, nil
}

func (r *photoWireRepo) CountByOwnerAndType(context.Context, uuid.UUID, domain.PropertyType) (int, error) {
	return 0, nil
}

func (r *photoWireRepo) Delete(context.Context, uuid.UUID, uuid.UUID) error { return nil }

func (r *photoWireRepo) WithTx(transaction.Tx) propertiesapp.PropertyRepository { return r }

// photoWireStorage records the storage traffic of the wire tests.
type photoWireStorage struct {
	puts    []string
	deletes []string
}

func (s *photoWireStorage) Put(_ context.Context, key string, data io.Reader, _ string, _ int64) error {
	b, err := io.ReadAll(data)
	if err != nil {
		return err
	}
	s.puts = append(s.puts, key+":"+string(b))
	return nil
}

func (s *photoWireStorage) Open(_ context.Context, key string) (body io.ReadCloser, size int64, contentType string, err error) {
	if !strings.Contains(key, "with-photo") {
		return nil, 0, "", storageshared.ErrNotFound
	}
	return io.NopCloser(strings.NewReader("jpeg-bytes")), 10, "image/jpeg", nil
}

func (s *photoWireStorage) Delete(_ context.Context, key string) error {
	s.deletes = append(s.deletes, key)
	return nil
}

// photoWirePolicy treats the actor as the owner: the wire tests exercise the
// HTTP contract, not the access matrix (the service tests own that).
type photoWirePolicy struct{}

func (photoWirePolicy) Role(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error) {
	return sharedpolicy.RoleOwner, nil
}

func (photoWirePolicy) RoleForProperty(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error) {
	return sharedpolicy.RoleOwner, nil
}

func photoWireHandler(t *testing.T, repo *photoWireRepo, storage *photoWireStorage) *PropertyHandlers {
	t.Helper()
	svc := propertiesapp.NewPropertyService(
		repo, storage,
		propertiesapp.NewTxStoreFactory(repo, nil, nil, nil, nil),
		nil, photoWirePolicy{}, discardLogger(),
	)
	return NewPropertyHandlers(svc, nil, discardLogger(), nil)
}

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func photoGetRequest(t *testing.T, target, ifNoneMatch string) *http.Request {
	t.Helper()
	ctx := httpsupport.WithUserID(context.Background(), uuid.Must(uuid.NewV7()))
	r := httptest.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if ifNoneMatch != "" {
		r.Header.Set("If-None-Match", ifNoneMatch)
	}
	return r
}

func buildUploadBody(t *testing.T, fieldName, value string) (body *bytes.Buffer, contentType string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if fieldName != "" {
		fw, err := w.CreateFormFile("file", "photo.jpg")
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := fw.Write([]byte(value)); err != nil {
			t.Fatalf("write file: %v", err)
		}
	} else if err := w.WriteField("caption", "no file here"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return &buf, w.FormDataContentType()
}

func photoUploadRequest(t *testing.T, body *bytes.Buffer, contentType string) *http.Request {
	t.Helper()
	ctx := httpsupport.WithUserID(context.Background(), uuid.Must(uuid.NewV7()))
	r := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/properties/"+photoTestPropertyID().String()+"/photo", body)
	r.Header.Set("Content-Type", contentType)
	return r
}

func TestPropertyPhotoUpload_WireContract(t *testing.T) {
	t.Parallel()

	handler := photoWireHandler(t, &photoWireRepo{}, &photoWireStorage{})

	t.Run("unsupported format is a 400", func(t *testing.T) {
		t.Parallel()
		body, ct := buildUploadBody(t, "file", "<svg xmlns=\"http://www.w3.org/2000/svg\"/>")
		rr := httptest.NewRecorder()
		handler.UploadPropertyPhoto(rr, photoUploadRequest(t, body, ct), photoTestPropertyID())

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("missing file field is a 400", func(t *testing.T) {
		t.Parallel()
		body, ct := buildUploadBody(t, "", "")
		rr := httptest.NewRecorder()
		handler.UploadPropertyPhoto(rr, photoUploadRequest(t, body, ct), photoTestPropertyID())

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("oversized file is a 400", func(t *testing.T) {
		t.Parallel()
		big := strings.Repeat("x", photo.MaxSize+1)
		body, ct := buildUploadBody(t, "file", big)
		rr := httptest.NewRecorder()
		handler.UploadPropertyPhoto(rr, photoUploadRequest(t, body, ct), photoTestPropertyID())

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("not a multipart form is a 400", func(t *testing.T) {
		t.Parallel()
		ctx := httpsupport.WithUserID(context.Background(), uuid.Must(uuid.NewV7()))
		r := httptest.NewRequestWithContext(ctx, http.MethodPost,
			"/api/v1/properties/"+photoTestPropertyID().String()+"/photo", strings.NewReader("hello"))
		r.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler.UploadPropertyPhoto(rr, r, photoTestPropertyID())

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})
}

func TestPropertyPhotoGet_ServesAndCaches(t *testing.T) {
	t.Parallel()

	key := "photos/01987654-3210-7abc-9def-0123456789ab-with-photo.jpg"
	property := domain.Property{
		ID:      photoTestPropertyID(),
		Name:    "Test",
		Address: "Test Address",
		Type:    domain.PropertyTypeApartment,
		Status:  domain.PropertyStatusActive,
	}
	property.PhotoKey = &key
	repo := &photoWireRepo{property: property}
	storage := &photoWireStorage{}
	handler := photoWireHandler(t, repo, storage)

	target := "/api/v1/properties/" + photoTestPropertyID().String() + "/photo"

	t.Run("404 without a photo", func(t *testing.T) {
		t.Parallel()
		noPhoto := domain.Property{ID: property.ID, Name: property.Name, Address: property.Address, Type: property.Type}
		noPhotoRepo := &photoWireRepo{property: noPhoto}
		noPhotoHandler := photoWireHandler(t, noPhotoRepo, storage)
		rr := httptest.NewRecorder()
		noPhotoHandler.GetPropertyPhoto(rr, photoGetRequest(t, target, ""), photoTestPropertyID())
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})

	t.Run("304 from the key-hash etag without a storage read", func(t *testing.T) {
		t.Parallel()
		etag := storageshared.ETagOf(key)
		rr := httptest.NewRecorder()
		handler.GetPropertyPhoto(rr, photoGetRequest(t, target, etag), photoTestPropertyID())
		if rr.Code != http.StatusNotModified {
			t.Fatalf("status = %d, want 304", rr.Code)
		}
		if got := rr.Header().Get("Cache-Control"); got != "private, max-age=300" {
			t.Errorf("cache-control = %q, want private, max-age=300", got)
		}
	})

	t.Run("200 streams the private bytes", func(t *testing.T) {
		t.Parallel()
		rr := httptest.NewRecorder()
		handler.GetPropertyPhoto(rr, photoGetRequest(t, target, ""), photoTestPropertyID())
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		if got := rr.Header().Get("Content-Type"); got != "image/jpeg" {
			t.Errorf("content-type = %q, want image/jpeg", got)
		}
		if got := rr.Header().Get("Cache-Control"); got != "private, max-age=300" {
			t.Errorf("cache-control = %q, want private, max-age=300", got)
		}
		if got := rr.Header().Get("ETag"); got != storageshared.ETagOf(key) {
			t.Errorf("etag = %q, want the key hash", got)
		}
		if !strings.Contains(rr.Body.String(), "jpeg-bytes") {
			t.Errorf("body = %q, want the stored bytes", rr.Body.String())
		}
	})
}

var (
	_ propertiesapp.PropertyRepository = (*photoWireRepo)(nil)
	_ storageshared.PhotoStorage       = (*photoWireStorage)(nil)
)
