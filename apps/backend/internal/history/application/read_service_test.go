package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/cursor"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// The read service's unit seam (тикет #708): the wire params are validated
// and folded into the store query here — the SQL carries the visibility and
// the search predicates, the service owns the contract (400/privacy-404),
// the cursors and the always-OR search predicate (research #839; routing
// retired by #842).

type readerFake struct {
	listRows     []domain.FeedEntry
	participants []domain.FilterParticipant
	objects      []domain.FilterObject
	gotQuery     *JournalQuery
	listErr      error
}

func (f *readerFake) List(_ context.Context, _ uuid.UUID, q JournalQuery) ([]domain.FeedEntry, error) {
	f.gotQuery = &q
	return f.listRows, f.listErr
}

func (f *readerFake) FilterParticipants(_ context.Context, _ uuid.UUID, _ []uuid.UUID) ([]domain.FilterParticipant, error) {
	return f.participants, nil
}

func (f *readerFake) FilterObjects(_ context.Context, _ uuid.UUID, _ []uuid.UUID) ([]domain.FilterObject, error) {
	return f.objects, nil
}

type policyFake struct {
	roles map[uuid.UUID]sharedpolicy.Role
}

func (f policyFake) Role(_ context.Context, _, _ uuid.UUID) (sharedpolicy.Role, error) {
	return sharedpolicy.RoleNone, nil
}

func (f policyFake) RoleForProperty(_ context.Context, _, propertyID uuid.UUID) (sharedpolicy.Role, error) {
	if role, ok := f.roles[propertyID]; ok {
		return role, nil
	}
	return sharedpolicy.RoleNone, nil
}

func newReadServiceFake(rows []domain.FeedEntry, roles map[uuid.UUID]sharedpolicy.Role) (*HistoryReadService, *readerFake) {
	reader := &readerFake{listRows: rows}
	return NewHistoryReadService(reader, policyFake{roles: roles}), reader
}

// encodeString builds a well-formed base64url blob that is not a cursor
// payload — the decode must fold it into ErrInvalidInput all the same.
func encodeString(v string) string {
	return cursor.Encode(v)
}

func TestFeedLimitDefaultsAndBounds(t *testing.T) {
	t.Parallel()

	svc, reader := newReadServiceFake(nil, nil)

	if _, err := svc.Feed(t.Context(), uuid.Must(uuid.NewV7()), FeedQuery{Limit: -1}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("negative limit: want ErrInvalidInput, got %v", err)
	}
	if _, err := svc.Feed(t.Context(), uuid.Must(uuid.NewV7()), FeedQuery{Limit: FeedMaxLimit + 1}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("limit over ceiling: want ErrInvalidInput, got %v", err)
	}
	if _, err := svc.Feed(t.Context(), uuid.Must(uuid.NewV7()), FeedQuery{Limit: 0}); err != nil {
		t.Fatalf("zero limit: want the default applied, got %v", err)
	}
	if got := reader.gotQuery.Limit; got != feedDefaultLimit {
		t.Fatalf("zero limit: want the %d-row default, got %d", feedDefaultLimit, got)
	}
}

func TestFeedCursorsMutuallyExclusive(t *testing.T) {
	t.Parallel()

	svc, _ := newReadServiceFake(nil, nil)

	_, err := svc.Feed(t.Context(), uuid.Must(uuid.NewV7()), FeedQuery{BeforeCursor: "a", AfterCursor: "b"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("both cursors: want ErrInvalidInput, got %v", err)
	}
}

func TestFeedMalformedCursorIs400(t *testing.T) {
	t.Parallel()

	svc, _ := newReadServiceFake(nil, nil)

	for name, blob := range map[string]string{
		"not base64url": "###",
		"not a payload": encodeString("hello"),
	} {
		if _, err := svc.Feed(t.Context(), uuid.Must(uuid.NewV7()), FeedQuery{BeforeCursor: blob}); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("%s: want ErrInvalidInput, got %v", name, err)
		}
	}
}

func TestFeedUnknownVocabularyIs400(t *testing.T) {
	t.Parallel()

	svc, _ := newReadServiceFake(nil, nil)

	_, err := svc.Feed(t.Context(), uuid.Must(uuid.NewV7()), FeedQuery{Kinds: []string{"property", "vehicles"}})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unknown kind: want ErrInvalidInput, got %v", err)
	}
	_, err = svc.Feed(t.Context(), uuid.Must(uuid.NewV7()), FeedQuery{BaseActions: []string{"updated"}})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unknown base action: want ErrInvalidInput, got %v", err)
	}
}

func TestFeedInvisiblePropertyIsPrivacy404(t *testing.T) {
	t.Parallel()

	visible, foreign, suspended := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	roles := map[uuid.UUID]sharedpolicy.Role{
		foreign:   sharedpolicy.RoleNone,
		suspended: sharedpolicy.RoleSuspended,
	}
	svc, reader := newReadServiceFake(nil, roles)

	for name, ids := range map[string][]uuid.UUID{
		"no access":  {visible, foreign},
		"suspended":  {suspended},
		"unknown id": {uuid.Must(uuid.NewV7())},
	} {
		if _, err := svc.Feed(t.Context(), visible, FeedQuery{PropertyIDs: ids}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s: want the privacy ErrNotFound, got %v", name, err)
		}
	}
	if reader.gotQuery != nil {
		t.Fatalf("the store must not be touched when the proof fails")
	}
}

func TestFeedViewerPropertyPasses(t *testing.T) {
	t.Parallel()

	viewerOwned := uuid.Must(uuid.NewV7())
	roles := map[uuid.UUID]sharedpolicy.Role{viewerOwned: sharedpolicy.RoleViewer}
	svc, _ := newReadServiceFake(nil, roles)

	if _, err := svc.Feed(t.Context(), uuid.Must(uuid.NewV7()), FeedQuery{PropertyIDs: []uuid.UUID{viewerOwned}}); err != nil {
		t.Fatalf("viewer-readable property: want pass, got %v", err)
	}
}

func TestFeedPageEncodesBothCursorDirections(t *testing.T) {
	t.Parallel()

	rows := make([]domain.FeedEntry, 3)
	for i := range rows {
		rows[i] = domain.FeedEntry{ID: uuid.Must(uuid.NewV7()), CreatedAt: time.Unix(int64(100-i), 0)}
	}
	svc, reader := newReadServiceFake(rows, nil)

	// Полная страница: продолжение в старую сторону (последняя строка) и в
	// новую (первая строка) кодируются обе.
	page, err := svc.Feed(t.Context(), uuid.Must(uuid.NewV7()), FeedQuery{Limit: 3})
	if err != nil {
		t.Fatalf("full page: %v", err)
	}
	if want := encodeFeedCursor(CursorKey{CreatedAt: rows[2].CreatedAt, ID: rows[2].ID}); page.NextCursor != want {
		t.Fatalf("next cursor: want the last row's key, got %q", page.NextCursor)
	}
	if want := encodeFeedCursor(CursorKey{CreatedAt: rows[0].CreatedAt, ID: rows[0].ID}); page.PrevCursor != want {
		t.Fatalf("prev cursor: want the first row's key, got %q", page.PrevCursor)
	}
	if reader.gotQuery.Limit != 3 {
		t.Fatalf("store limit: want 3, got %d", reader.gotQuery.Limit)
	}

	// Короткая страница: в старую сторону идти некуда, в новую — есть.
	reader.listRows = rows[:1]
	page, err = svc.Feed(t.Context(), uuid.Must(uuid.NewV7()), FeedQuery{Limit: 3})
	if err != nil {
		t.Fatalf("short page: %v", err)
	}
	if page.NextCursor != "" {
		t.Fatalf("short page: next cursor must be absent, got %q", page.NextCursor)
	}
	if page.PrevCursor == "" {
		t.Fatalf("short page: prev cursor must be present")
	}

	// Пустая страница: обоих курсоров нет.
	reader.listRows = nil
	page, err = svc.Feed(t.Context(), uuid.Must(uuid.NewV7()), FeedQuery{Limit: 3})
	if err != nil {
		t.Fatalf("empty page: %v", err)
	}
	if page.NextCursor != "" || page.PrevCursor != "" {
		t.Fatalf("empty page: both cursors must be absent, got %q/%q", page.NextCursor, page.PrevCursor)
	}
}

func TestFeedFoldsValidatedParamsIntoStoreQuery(t *testing.T) {
	t.Parallel()

	actor := uuid.Must(uuid.NewV7())
	actorID := uuid.Must(uuid.NewV7())
	propertyID := uuid.Must(uuid.NewV7())
	from := time.Unix(1000, 0)
	to := time.Unix(2000, 0)
	roles := map[uuid.UUID]sharedpolicy.Role{propertyID: sharedpolicy.RoleOwner}
	svc, reader := newReadServiceFake(nil, roles)

	_, err := svc.Feed(t.Context(), actor, FeedQuery{
		DateFrom:    &from,
		DateTo:      &to,
		BaseActions: []string{"added", "changed"},
		Kinds:       []string{"payment"},
		ActorIDs:    []uuid.UUID{actorID},
		PropertyIDs: []uuid.UUID{propertyID},
	})
	if err != nil {
		t.Fatalf("feed: %v", err)
	}
	got := reader.gotQuery
	assertPeriod(t, got, from, to)
	if len(got.BaseActions) != 2 || got.BaseActions[0] != domain.BaseAdded || got.BaseActions[1] != domain.BaseChanged {
		t.Fatalf("base actions: %+v", got.BaseActions)
	}
	assertIDFilters(t, got, actorID, propertyID)
	if got.Search != "" {
		t.Fatalf("empty query must carry no search, got %+v", got.Search)
	}

	// Поиск едет в стор обрезанным; решение о предикате принимает SQL —
	// всегда-OR обеих ног (ресерч #839), в Go только trim и гард длины.
	if _, err := svc.Feed(t.Context(), actor, FeedQuery{Query: "  петр  ", PropertyIDs: []uuid.UUID{propertyID}}); err != nil {
		t.Fatalf("feed search: %v", err)
	}
	if got := reader.gotQuery.Search; got != "петр" {
		t.Fatalf("search must travel trimmed, got %q", got)
	}
}

// assertPeriod checks both period bounds travelled into the store query.
func assertPeriod(t *testing.T, got *JournalQuery, from, to time.Time) {
	t.Helper()
	if got.DateFrom == nil || !got.DateFrom.Equal(from) {
		t.Fatalf("date_from did not travel: %+v", got.DateFrom)
	}
	if got.DateTo == nil || !got.DateTo.Equal(to) {
		t.Fatalf("date_to did not travel: %+v", got.DateTo)
	}
}

// assertIDFilters checks the actor and property id filters travelled.
func assertIDFilters(t *testing.T, got *JournalQuery, actorID, propertyID uuid.UUID) {
	t.Helper()
	if len(got.ActorIDs) != 1 || got.ActorIDs[0] != actorID {
		t.Fatalf("actor ids: %+v", got.ActorIDs)
	}
	if len(got.PropertyIDs) != 1 || got.PropertyIDs[0] != propertyID {
		t.Fatalf("property ids: %+v", got.PropertyIDs)
	}
	if len(got.Kinds) != 1 || got.Kinds[0] != domain.KindPayment {
		t.Fatalf("kinds: %+v", got.Kinds)
	}
}

func TestFiltersProvesScopeAndReturnsOptions(t *testing.T) {
	t.Parallel()

	foreign := uuid.Must(uuid.NewV7())
	roles := map[uuid.UUID]sharedpolicy.Role{foreign: sharedpolicy.RoleNone}
	reader := &readerFake{
		participants: []domain.FilterParticipant{{ID: uuid.Must(uuid.NewV7()), Name: "Иван Иванов"}},
		objects:      []domain.FilterObject{{ID: uuid.Must(uuid.NewV7()), Name: "Гараж"}},
	}
	svc := NewHistoryReadService(reader, policyFake{roles: roles})

	if _, err := svc.Filters(t.Context(), uuid.Must(uuid.NewV7()), []uuid.UUID{foreign}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign scope: want the privacy ErrNotFound, got %v", err)
	}
	opts, err := svc.Filters(t.Context(), uuid.Must(uuid.NewV7()), nil)
	if err != nil {
		t.Fatalf("filters: %v", err)
	}
	if len(opts.Participants) != 1 || len(opts.Objects) != 1 {
		t.Fatalf("options did not travel: %+v", opts)
	}
}

func TestCursorRoundtrip(t *testing.T) {
	t.Parallel()

	key := CursorKey{CreatedAt: time.Unix(1758542400, 123456000).UTC(), ID: uuid.Must(uuid.NewV7())}

	decoded, err := decodeFeedCursor(encodeFeedCursor(key))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !decoded.CreatedAt.Equal(key.CreatedAt) || decoded.ID != key.ID {
		t.Fatalf("roundtrip lost the key: %+v vs %+v", decoded, key)
	}
}

func TestSanitizeSearch(t *testing.T) {
	t.Parallel()

	// Пустое и пробельное — поиска нет (канон #601: пустой ввод запроса
	// не порождает).
	for name, q := range map[string]string{"empty": "", "spaces": "   "} {
		got, err := sanitizeSearch(q)
		if err != nil {
			t.Fatalf("%s: want no search, got error %v", name, err)
		}
		if got != "" {
			t.Fatalf("%s: want no search, got %q", name, got)
		}
	}

	// Края не участвуют в матче (канон поиска).
	got, err := sanitizeSearch("  аренда  ")
	if err != nil || got != "аренда" {
		t.Fatalf("trim: want «аренда», got %q, %v", got, err)
	}

	// Гард длины (ресерч #839): 400 байт и больше — контрактный 400; 399 —
	// проходит целиком.
	_, err = sanitizeSearch(strings.Repeat("a", searchMaxBytes))
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("%d bytes: want ErrInvalidInput, got %v", searchMaxBytes, err)
	}
	got, err = sanitizeSearch(strings.Repeat("a", searchMaxBytes-1))
	if err != nil || len(got) != searchMaxBytes-1 {
		t.Fatalf("%d bytes: want pass-through, got %d bytes, %v", searchMaxBytes-1, len(got), err)
	}

	// Гард считается по байтам: 255 кириллических символов контракта —
	// 510 байт, серверный бэкстоп срабатывает и на них.
	_, err = sanitizeSearch(strings.Repeat("а", 255))
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("255 cyrillic chars (510 bytes): want ErrInvalidInput, got %v", err)
	}
}
