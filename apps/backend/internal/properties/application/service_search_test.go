package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
)

// searchRecordingRepo fronts the map-based fake with a SearchVisible that
// captures the query (the service's normalized window) and answers with a
// canned page.
type searchRecordingRepo struct {
	*fakePropertyRepo
	got      PropertySearchQuery
	gotActor uuid.UUID
	page     []domain.Property
}

func (r *searchRecordingRepo) SearchVisible(
	_ context.Context, actor uuid.UUID, q PropertySearchQuery,
) ([]domain.Property, error) {
	r.got = q
	r.gotActor = actor
	return r.page, nil
}

func newSearchService(t *testing.T, repo PropertyRepository) *PropertyService {
	t.Helper()
	return newPhotoService(t, repo, &fakePhotoRepo{}, &fakePhotoStorage{})
}

func newSearchPhotoService(t *testing.T, repo PropertyRepository, photoRepo *fakePhotoRepo) *PropertyService {
	t.Helper()
	return NewPropertyService(
		repo, photoRepo, &fakePhotoStorage{},
		newPropertyTestFactory(repo, photoRepo, fakeSubscriptionLimiter{limit: 10}),
		fakePropertyClock{now: time.Now()}, testOwnerPolicy{}, nil,
	)
}

const (
	searchQuery = "кварт"
	searchName  = "Квартира"
)

var searchActor = uuid.MustParse("11111111-1111-1111-1111-111111111111")

// The service normalizes the window before SQL: the zero limit becomes the
// contract's default page, the empty cursor reads from the beginning (nil
// keyset key).
func TestSearchProperties_NormalizesPage(t *testing.T) {
	t.Parallel()

	repo := &searchRecordingRepo{page: []domain.Property{}}
	svc := newSearchService(t, repo)

	if _, _, err := svc.SearchProperties(context.Background(), searchActor, searchQuery, SearchPropertiesPage{}); err != nil {
		t.Fatalf("search: %v", err)
	}
	if repo.got.Limit != DefaultPropertySearchPageSize {
		t.Errorf("expected default limit %d, got %d", DefaultPropertySearchPageSize, repo.got.Limit)
	}
	if repo.got.AfterName != nil || repo.got.AfterID != nil {
		t.Errorf("expected nil keyset key without cursor, got %v/%v", repo.got.AfterName, repo.got.AfterID)
	}
	if repo.gotActor != searchActor {
		t.Errorf("expected actor %s, got %s", searchActor, repo.gotActor)
	}
}

// A client-echoed cursor decodes into the keyset key the store resumes
// strictly after.
func TestSearchProperties_DecodesCursor(t *testing.T) {
	t.Parallel()

	repo := &searchRecordingRepo{page: []domain.Property{}}
	svc := newSearchService(t, repo)

	afterID := uuid.MustParse("01933e2f-7b1a-7000-8000-00000000000a")
	cursor := EncodePropertySearchCursor("Квартира на Горького", afterID)

	if _, _, err := svc.SearchProperties(
		context.Background(), searchActor, "горьк",
		SearchPropertiesPage{Limit: 50, Cursor: cursor},
	); err != nil {
		t.Fatalf("search: %v", err)
	}
	if repo.got.AfterName == nil || *repo.got.AfterName != "Квартира на Горького" {
		t.Errorf("expected after name forwarded, got %v", repo.got.AfterName)
	}
	if repo.got.AfterID == nil || *repo.got.AfterID != afterID {
		t.Errorf("expected after id forwarded, got %v", repo.got.AfterID)
	}
}

// A full page answers with the last row's continuation; a short one has
// walked the matches to the end — the empty cursor stops the scroll.
func TestSearchProperties_NextCursorFollowsPageFullness(t *testing.T) {
	t.Parallel()

	full := make([]domain.Property, DefaultPropertySearchPageSize)
	for i := range full {
		full[i] = domain.Property{
			ID:      uuid.Must(uuid.NewV7()),
			OwnerID: searchActor,
			Name:    searchName,
			Status:  domain.PropertyStatusActive,
		}
	}
	last := full[len(full)-1]

	repo := &searchRecordingRepo{page: full}
	svc := newSearchService(t, repo)
	props, nextCursor, err := svc.SearchProperties(
		context.Background(), searchActor, searchQuery, SearchPropertiesPage{Limit: DefaultPropertySearchPageSize},
	)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(props) != DefaultPropertySearchPageSize {
		t.Fatalf("expected %d rows, got %d", DefaultPropertySearchPageSize, len(props))
	}
	gotName, gotID, err := DecodePropertySearchCursor(nextCursor)
	if err != nil {
		t.Fatalf("nextCursor does not decode: %v", err)
	}
	if gotName != last.Name || gotID != last.ID {
		t.Errorf("expected cursor of the last row (%s, %s), got (%s, %s)", last.Name, last.ID, gotName, gotID)
	}

	repo = &searchRecordingRepo{page: full[:2]}
	svc = newSearchService(t, repo)
	_, nextCursor, err = svc.SearchProperties(
		context.Background(), searchActor, searchQuery, SearchPropertiesPage{Limit: DefaultPropertySearchPageSize},
	)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if nextCursor != "" {
		t.Errorf("expected empty cursor on a short page, got %q", nextCursor)
	}
}

// Photos ride along with the search rows (the screen's avatar).
func TestSearchProperties_AttachesPhotos(t *testing.T) {
	t.Parallel()

	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	repo := &searchRecordingRepo{page: []domain.Property{{
		ID: propertyID, OwnerID: searchActor, Name: "Квартира", Status: domain.PropertyStatusActive,
	}}}

	photoRepo := &fakePhotoRepo{photos: map[uuid.UUID][]domain.Photo{
		propertyID: {{ID: uuid.Must(uuid.NewV7()), URL: "https://cdn.example/p.jpg"}},
	}}
	svc := newSearchPhotoService(t, repo, photoRepo)

	props, _, err := svc.SearchProperties(context.Background(), searchActor, searchQuery, SearchPropertiesPage{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(props) != 1 || len(props[0].Photos) != 1 {
		t.Fatalf("expected one property with one photo, got %+v", props)
	}
	if props[0].Photos[0].URL != "https://cdn.example/p.jpg" {
		t.Errorf("unexpected photo url %q", props[0].Photos[0].URL)
	}
}

// The contract's 400 modes: a blank search, an out-of-range page and a
// malformed cursor.
func TestSearchProperties_RejectsInvalidInput(t *testing.T) {
	t.Parallel()

	svc := newSearchService(t, &searchRecordingRepo{})
	oversized := string(make([]byte, 256))

	cases := []struct {
		name   string
		search string
		page   SearchPropertiesPage
	}{
		{"blank search", "   ", SearchPropertiesPage{}},
		{"oversized search", oversized, SearchPropertiesPage{}},
		{"negative limit", searchQuery, SearchPropertiesPage{Limit: -1}},
		{"over-max limit", searchQuery, SearchPropertiesPage{Limit: MaxPropertySearchPageSize + 1}},
		{"malformed cursor", searchQuery, SearchPropertiesPage{Limit: 50, Cursor: "%%%"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, _, err := svc.SearchProperties(context.Background(), searchActor, tc.search, tc.page); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}
		})
	}
}
