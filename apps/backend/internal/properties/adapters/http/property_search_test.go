package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/objectstorage"
	openapi "github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// The GET /properties/search wire tests (ticket #601): the query travels
// into the service, the keyset continuation rides back as nextCursor (null
// once the matches are exhausted), and a malformed cursor is the contract's
// 400.

// searchWireRepo answers SearchVisible with a canned page and records the
// query; every other repository method is dead weight for this test.
type searchWireRepo struct {
	page     []domain.Property
	recorder func(propertiesapp.PropertySearchQuery)
	// The archivedCount feeds CountArchivedByOwner for the list wire tests'
	// «Архив» gate fixture (issue #1233); zero for the search tests.
	archivedCount int
}

func (r *searchWireRepo) Create(_ context.Context, _ uuid.UUID, _ domain.Property) (domain.Property, error) {
	return domain.Property{}, nil
}

func (r *searchWireRepo) GetByIDAndOwner(_ context.Context, _, _ uuid.UUID) (domain.Property, error) {
	return domain.Property{}, propertiesapp.ErrNotFound
}

func (r *searchWireRepo) GetByIDAndOwnerForUpdate(_ context.Context, _, _ uuid.UUID) (domain.Property, error) {
	return domain.Property{}, propertiesapp.ErrNotFound
}

func (r *searchWireRepo) GetByID(_ context.Context, _ uuid.UUID) (domain.Property, error) {
	return domain.Property{}, propertiesapp.ErrNotFound
}

func (r *searchWireRepo) GetByIDForUpdate(_ context.Context, _ uuid.UUID) (domain.Property, error) {
	return domain.Property{}, propertiesapp.ErrNotFound
}

func (r *searchWireRepo) ListActiveByOwner(_ context.Context, _ uuid.UUID) ([]domain.Property, error) {
	return nil, nil
}

func (r *searchWireRepo) ListArchivedByOwner(_ context.Context, _ uuid.UUID) ([]domain.Property, error) {
	return nil, nil
}

func (r *searchWireRepo) SearchVisible(_ context.Context, _ uuid.UUID, q propertiesapp.PropertySearchQuery) ([]domain.Property, error) {
	if r.recorder != nil {
		r.recorder(q)
	}
	return r.page, nil
}

func (r *searchWireRepo) Update(_ context.Context, _ uuid.UUID, _ domain.Property) (domain.Property, error) {
	return domain.Property{}, nil
}

func (r *searchWireRepo) SetPin(_ context.Context, _, _ uuid.UUID, _ *time.Time) (domain.Property, error) {
	return domain.Property{}, nil
}

func (r *searchWireRepo) Archive(_ context.Context, _, _ uuid.UUID) error { return nil }

func (r *searchWireRepo) Unarchive(_ context.Context, _, _ uuid.UUID) error { return nil }

func (r *searchWireRepo) CountActiveByOwner(_ context.Context, _ uuid.UUID) (int, error) {
	return 0, nil
}

func (r *searchWireRepo) CountArchivedByOwner(_ context.Context, _ uuid.UUID) (int, error) {
	return r.archivedCount, nil
}

func (r *searchWireRepo) CountByOwnerAndType(_ context.Context, _ uuid.UUID, _ domain.PropertyType) (int, error) {
	return 0, nil
}

func (r *searchWireRepo) Delete(_ context.Context, _, _ uuid.UUID) error { return nil }

func (r *searchWireRepo) SetPropertyPhoto(_ context.Context, id, _ uuid.UUID, key, contentType *string) (domain.Property, error) {
	return domain.Property{}, nil
}

func (r *searchWireRepo) WithTx(_ transaction.Tx) propertiesapp.PropertyRepository { return r }

const wireSearch = "кварт"

func searchWireHandler(t *testing.T, repo *searchWireRepo) *PropertyHandlers {
	t.Helper()
	// The transactional collaborators stay nil: the search read walks the
	// repository directly, the factory is only the service's constructor
	// bundle. The real clock serves the list reads' «today» fallback (the
	// list wire tests ride this harness too, issue #1233).
	svc := propertiesapp.NewPropertyService(
		repo, objectstorage.NewFakeStorage(),
		propertiesapp.NewTxStoreFactory(repo, nil, nil, nil, nil),
		clock.Real{}, nil, slog.New(slog.DiscardHandler),
	)
	return NewPropertyHandlers(svc, nil, slog.New(slog.DiscardHandler), nil)
}

func searchRequest(target string) *http.Request {
	return httptest.NewRequestWithContext(
		httpsupport.WithUserID(context.Background(), uuid.Must(uuid.NewV7())),
		http.MethodGet, target, nil,
	)
}

func TestSearchProperties_WireShape(t *testing.T) {
	t.Parallel()

	property := domain.Property{
		ID:      uuid.Must(uuid.NewV7()),
		OwnerID: uuid.Must(uuid.NewV7()),
		Name:    "Квартира на Горького",
		Address: "Горького, 37",
		Status:  domain.PropertyStatusActive,
	}
	repo := &searchWireRepo{page: []domain.Property{property}}
	h := searchWireHandler(t, repo)

	rr := httptest.NewRecorder()
	h.SearchProperties(rr, searchRequest("/properties/search?search=%D0%B3%D0%BE%D1%80%D1%8C%D0%BA"),
		openapi.SearchPropertiesParams{Search: "горьк"})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body openapi.PropertiesSearchResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body.Items) != 1 || body.Items[0].Name != property.Name {
		t.Errorf("expected the matched card, got %+v", body.Items)
	}
	// A single row is a short page — the matches are exhausted, the cursor
	// travels as null.
	if body.NextCursor != nil {
		t.Errorf("nextCursor = %q, want null", *body.NextCursor)
	}
}

// The limit and cursor parameters travel into the service's window: the
// limit survives as-is, the cursor decodes into the keyset key.
func TestSearchProperties_ParamsTravelIntoWindow(t *testing.T) {
	t.Parallel()

	limit := openapi.PropertiesSearchLimit(25)
	afterID := uuid.Must(uuid.NewV7())
	cursor := propertiesapp.EncodePropertySearchCursor("Квартира на Горького", afterID)
	repo := &searchWireRepo{}
	repo.recorder = func(q propertiesapp.PropertySearchQuery) {
		if q.Limit != 25 {
			t.Errorf("limit = %d, want 25", q.Limit)
		}
		if q.AfterName == nil || *q.AfterName != "Квартира на Горького" {
			t.Errorf("expected decoded after name, got %v", q.AfterName)
		}
		if q.AfterID == nil || *q.AfterID != afterID {
			t.Errorf("expected decoded after id %s, got %v", afterID, q.AfterID)
		}
		if q.Search != wireSearch {
			t.Errorf("search = %q, want wireSearch", q.Search)
		}
	}
	h := searchWireHandler(t, repo)

	rr := httptest.NewRecorder()
	h.SearchProperties(rr, searchRequest("/properties/search"), openapi.SearchPropertiesParams{
		Search: wireSearch,
		Limit:  &limit,
		Cursor: &cursor,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
}

// A malformed cursor is the contract's 400 — the search response's own
// BadRequest, not a 500.
func TestSearchProperties_MalformedCursorIs400(t *testing.T) {
	t.Parallel()

	bad := "не курсор"
	h := searchWireHandler(t, &searchWireRepo{})

	rr := httptest.NewRecorder()
	h.SearchProperties(rr, searchRequest("/properties/search"), openapi.SearchPropertiesParams{
		Search: "кварт",
		Cursor: &bad,
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

// An unauthorized request stays the 401 problem, before any service work.
func TestSearchProperties_Unauthorized(t *testing.T) {
	t.Parallel()

	h := searchWireHandler(t, &searchWireRepo{})

	rr := httptest.NewRecorder()
	h.SearchProperties(rr, httptest.NewRequestWithContext(
		context.Background(), http.MethodGet, "/properties/search", nil,
	), openapi.SearchPropertiesParams{Search: "кварт"})
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}
