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
	// Nil-гейт выключен — конвенция опциональных зависимостей пакета; тарифное
	// измерение пинят отдельные тесты гейта ниже. Резолвер участников не
	// вайрится — его поверхность пинят тесты ParticipantFeed.
	return NewHistoryReadService(reader, policyFake{roles: roles}, nil, nil), reader
}

// scopesFake is the ParticipantScopeResolver test double: a fixed scope (or
// error) plus the call counter — резолв участника происходит ровно один раз
// на запрос, до доказательства скоупа и стора.
type scopesFake struct {
	scope  ParticipantScope
	err    error
	calls  int
	called string
}

func (f *scopesFake) ResolveParticipantScope(_ context.Context, _ uuid.UUID, participantID string) (ParticipantScope, error) {
	f.calls++
	f.called = participantID
	return f.scope, f.err
}

var _ ParticipantScopeResolver = (*scopesFake)(nil)

// newParticipantFeedFake собирает сервис с резолвером участников и пустым
// стором; роли применяются к ногам скоупа. Гейт — интерфейсный параметр:
// нетипизированный nil честно выключает проверку (конвенция пакета).
func newParticipantFeedFake(
	scope ParticipantScope, roles map[uuid.UUID]sharedpolicy.Role, gate PaidSectionsGate,
) (*HistoryReadService, *readerFake, *scopesFake) {
	scopes := &scopesFake{scope: scope}
	reader := &readerFake{}
	return NewHistoryReadService(reader, policyFake{roles: roles}, gate, scopes), reader, scopes
}

// gateStub is the PaidSectionsGate test double: a fixed verdict plus the call
// counter — «гейт спрашивают один раз на запрос, после резолва политики»
// pins itself through calls.
type gateStub struct {
	allows bool
	err    error
	calls  int
}

func (g *gateStub) CanUsePaidFeatures(context.Context, uuid.UUID) (bool, error) {
	g.calls++
	return g.allows, g.err
}

var _ PaidSectionsGate = (*gateStub)(nil)

// newGatedReadServiceFake builds the empty-feed service with the given gate
// verdict; the caller keeps the gate to assert the calls. The gate verdicts
// under test all fire before the store is touched, the fake's rows stay
// empty.
func newGatedReadServiceFake(roles map[uuid.UUID]sharedpolicy.Role, gate *gateStub) (*HistoryReadService, *readerFake) {
	reader := &readerFake{}
	return NewHistoryReadService(reader, policyFake{roles: roles}, gate, nil), reader
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
	svc := NewHistoryReadService(reader, policyFake{roles: roles}, nil, nil)

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

// Ролевой тарифный гейт history-роутов (ADR 0064, ревизия #1465): на
// не-платном тарифе чтения журнала проходят только в суженной membership-
// области — каждый суженный объект обязан быть объектом активного участия
// читателя (не его собственным). Лента без сужения и сужение на собственный
// объект — ErrTariffRequired («свой объект не в истории» буквально, любая
// поверхность). Приватность отвечает раньше тарифа: suspended и чужой id —
// прежний privacy-404, гейт не спрашивается. Шов — после уже существующего
// резолва роли, одним проходом (без второго резолва, ADR 0028).

func TestFeedBasicTariffGlobalFeedRefused(t *testing.T) {
	t.Parallel()

	gate := &gateStub{allows: false}
	svc, reader := newGatedReadServiceFake(nil, gate)

	if _, err := svc.Feed(t.Context(), uuid.Must(uuid.NewV7()), FeedQuery{}); !errors.Is(err, ErrTariffRequired) {
		t.Fatalf("global feed on basic: want ErrTariffRequired, got %v", err)
	}
	if reader.gotQuery != nil {
		t.Fatalf("the store must not be touched when the tariff refuses")
	}
	if gate.calls != 1 {
		t.Fatalf("gate calls = %d, want exactly one consult per request", gate.calls)
	}
}

func TestFeedBasicTariffOwnPropertyNarrowingRefused(t *testing.T) {
	t.Parallel()

	owned := uuid.Must(uuid.NewV7())
	roles := map[uuid.UUID]sharedpolicy.Role{owned: sharedpolicy.RoleOwner}
	gate := &gateStub{allows: false}
	svc, reader := newGatedReadServiceFake(roles, gate)

	if _, err := svc.Feed(t.Context(), uuid.Must(uuid.NewV7()),
		FeedQuery{PropertyIDs: []uuid.UUID{owned}}); !errors.Is(err, ErrTariffRequired) {
		t.Fatalf("own-property narrowing on basic: want ErrTariffRequired, got %v", err)
	}
	if reader.gotQuery != nil {
		t.Fatalf("the store must not be touched when the tariff refuses")
	}
}

func TestFeedBasicTariffMemberNarrowingPasses(t *testing.T) {
	t.Parallel()

	for name, role := range map[string]sharedpolicy.Role{
		"full access": sharedpolicy.RoleFullAccess,
		"viewer":      sharedpolicy.RoleViewer,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			memberProperty := uuid.Must(uuid.NewV7())
			roles := map[uuid.UUID]sharedpolicy.Role{memberProperty: role}
			gate := &gateStub{allows: false}
			svc, reader := newGatedReadServiceFake(roles, gate)

			// «Пригласили — значит пользуется»: участник объекта читает
			// суженную ленту на базовом тарифе, стор получает запрос.
			if _, err := svc.Feed(t.Context(), uuid.Must(uuid.NewV7()), FeedQuery{PropertyIDs: []uuid.UUID{memberProperty}}); err != nil {
				t.Fatalf("member narrowing on basic: want pass, got %v", err)
			}
			if reader.gotQuery == nil {
				t.Fatalf("the store must receive the narrowed query")
			}
		})
	}
}

func TestFeedBasicTariffMixedNarrowingRefused(t *testing.T) {
	t.Parallel()

	// Строгий вариант: каждый суженный объект обязан быть членством —
	// собственный объект в списке валит весь запрос.
	owned := uuid.Must(uuid.NewV7())
	memberProperty := uuid.Must(uuid.NewV7())
	roles := map[uuid.UUID]sharedpolicy.Role{
		owned:          sharedpolicy.RoleOwner,
		memberProperty: sharedpolicy.RoleViewer,
	}
	gate := &gateStub{allows: false}
	svc, _ := newGatedReadServiceFake(roles, gate)

	if _, err := svc.Feed(t.Context(), uuid.Must(uuid.NewV7()),
		FeedQuery{PropertyIDs: []uuid.UUID{memberProperty, owned}}); !errors.Is(err, ErrTariffRequired) {
		t.Fatalf("mixed narrowing on basic: want ErrTariffRequired, got %v", err)
	}
}

func TestFeedPrivacyAnswersBeforeTariff(t *testing.T) {
	t.Parallel()

	memberProperty, foreign, suspended := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	roles := map[uuid.UUID]sharedpolicy.Role{
		memberProperty: sharedpolicy.RoleViewer,
		foreign:        sharedpolicy.RoleNone,
		suspended:      sharedpolicy.RoleSuspended,
	}
	gate := &gateStub{allows: false}
	svc, reader := newGatedReadServiceFake(roles, gate)

	for name, ids := range map[string][]uuid.UUID{
		"foreign":            {foreign},
		"suspended":          {suspended},
		"mixed with foreign": {memberProperty, foreign},
	} {
		if _, err := svc.Feed(t.Context(), uuid.Must(uuid.NewV7()), FeedQuery{PropertyIDs: ids}); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: want the privacy ErrNotFound, got %v", name, err)
		}
	}
	if reader.gotQuery != nil {
		t.Errorf("the store must not be touched when the proof fails")
	}
	if gate.calls != 0 {
		t.Errorf("gate calls = %d, want 0 — privacy is the role-policy answer", gate.calls)
	}
}

func TestFeedPaidTariffPassesUnchanged(t *testing.T) {
	t.Parallel()

	owned, memberProperty := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	roles := map[uuid.UUID]sharedpolicy.Role{
		owned:          sharedpolicy.RoleOwner,
		memberProperty: sharedpolicy.RoleViewer,
	}
	gate := &gateStub{allows: true}
	svc, reader := newGatedReadServiceFake(roles, gate)

	// Регрессионное направление: платный тариф читает и глобальную ленту
	// (собственные объекты — дело стора), и любое сужение.
	if _, err := svc.Feed(t.Context(), uuid.Must(uuid.NewV7()), FeedQuery{}); err != nil {
		t.Fatalf("global feed on paid: %v", err)
	}
	if _, err := svc.Feed(t.Context(), uuid.Must(uuid.NewV7()), FeedQuery{PropertyIDs: []uuid.UUID{owned, memberProperty}}); err != nil {
		t.Fatalf("narrowed feed on paid: %v", err)
	}
	if reader.gotQuery == nil {
		t.Fatalf("the store must receive the query on the paid tariff")
	}
}

func TestFeedGateFailureFailsClosed(t *testing.T) {
	t.Parallel()

	// Сбой хранилища подписки — не тарифный вердикт: наверх, на 500
	// обработчика (гейт не fail-open, ADR 0064).
	gate := &gateStub{allows: false, err: errors.New("subscriptions store down")}
	svc, reader := newGatedReadServiceFake(nil, gate)

	if _, err := svc.Feed(t.Context(), uuid.Must(uuid.NewV7()), FeedQuery{}); err == nil || errors.Is(err, ErrTariffRequired) {
		t.Fatalf("feed with a broken gate: want a propagated error, got %v", err)
	}
	if reader.gotQuery != nil {
		t.Fatalf("the store must not be touched when the gate fails")
	}
}

func TestFiltersTariffScope(t *testing.T) {
	t.Parallel()

	owned, memberProperty, foreign := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	roles := map[uuid.UUID]sharedpolicy.Role{
		owned:          sharedpolicy.RoleOwner,
		memberProperty: sharedpolicy.RoleViewer,
		foreign:        sharedpolicy.RoleNone,
	}
	reader := &readerFake{
		participants: []domain.FilterParticipant{{ID: uuid.Must(uuid.NewV7()), Name: "Иван Иванов"}},
		objects:      []domain.FilterObject{{ID: uuid.Must(uuid.NewV7()), Name: "Гараж"}},
	}
	gate := &gateStub{allows: false}
	svc := NewHistoryReadService(reader, policyFake{roles: roles}, gate, nil)
	actor := uuid.Must(uuid.NewV7())

	// Глобальная область шита и сужение на собственный объект — 402.
	if _, err := svc.Filters(t.Context(), actor, nil); !errors.Is(err, ErrTariffRequired) {
		t.Errorf("global filter options on basic: want ErrTariffRequired, got %v", err)
	}
	if _, err := svc.Filters(t.Context(), actor, []uuid.UUID{owned}); !errors.Is(err, ErrTariffRequired) {
		t.Errorf("own-property filter options on basic: want ErrTariffRequired, got %v", err)
	}
	// Сужение на объект участия — опции шита, как на платном.
	opts, err := svc.Filters(t.Context(), actor, []uuid.UUID{memberProperty})
	if err != nil {
		t.Fatalf("member filter options on basic: %v", err)
	}
	if len(opts.Participants) != 1 || len(opts.Objects) != 1 {
		t.Fatalf("options did not travel: %+v", opts)
	}
	// Приватность раньше тарифа, гейт не спрашивается.
	if _, err := svc.Filters(t.Context(), actor, []uuid.UUID{foreign}); !errors.Is(err, ErrNotFound) {
		t.Errorf("foreign filter options: want the privacy ErrNotFound, got %v", err)
	}
	if gate.calls != 3 {
		t.Errorf("gate calls = %d, want 3 — the two refusals and the passing scope", gate.calls)
	}
}

// «Действия участника» (карта #1582, решение владельца 22.10.2026 —
// переработка объектного deep-link): лента одного участника по всем его
// текущим ногам. Резолюция #1583: скоуп выводит сервер (uuid юзера AND ноги
// агрегата), pending — пустая страница (ручных действий не бывает, ADR 0061
// §3), не в скоупе читающего — privacy-404, пустые ноги — пустая страница
// без стора, тариф — тот же роль-осознанный гейт (владелец-читатель на
// basic — 402).

func TestParticipantFeedScopeTravelsToStore(t *testing.T) {
	t.Parallel()

	actor := uuid.Must(uuid.NewV7())
	userID := uuid.Must(uuid.NewV7())
	leg1, leg2 := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	roles := map[uuid.UUID]sharedpolicy.Role{
		leg1: sharedpolicy.RoleOwner,
		leg2: sharedpolicy.RoleOwner,
	}
	svc, reader, scopes := newParticipantFeedFake(
		ParticipantScope{UserID: userID, PropertyIDs: []uuid.UUID{leg1, leg2}},
		roles, nil,
	)

	_, err := svc.ParticipantFeed(t.Context(), actor, "12111111-1111-4111-8111-111111111121", FeedQuery{
		BeforeCursor: encodeFeedCursor(CursorKey{CreatedAt: time.Unix(50, 0), ID: uuid.Must(uuid.NewV7())}),
		Limit:        10,
		Query:        "гараж",
	})
	if err != nil {
		t.Fatalf("participant feed: %v", err)
	}
	if scopes.calls != 1 || scopes.called != "12111111-1111-4111-8111-111111111121" {
		t.Fatalf("resolver calls = %d (%q), want one call with the page's id", scopes.calls, scopes.called)
	}
	got := reader.gotQuery
	if got == nil {
		t.Fatalf("the store must receive the scoped query")
	}
	if len(got.ActorIDs) != 1 || got.ActorIDs[0] != userID {
		t.Fatalf("actor ids: %+v, want the participant's user uuid", got.ActorIDs)
	}
	if len(got.PropertyIDs) != 2 || got.PropertyIDs[0] != leg1 || got.PropertyIDs[1] != leg2 {
		t.Fatalf("property ids: %+v, want the aggregate's legs in order", got.PropertyIDs)
	}
	if got.Limit != 10 || got.Search != "гараж" {
		t.Fatalf("page params did not travel: limit %d, search %q", got.Limit, got.Search)
	}
}

func TestParticipantFeedPendingIsEmpty(t *testing.T) {
	t.Parallel()

	// Pending-приглашение: uuid юзера не существует, actor_id журнала —
	// всегда uuid (ADR 0061 §3) — пустая страница без стора и без
	// доказательств скоупа (доказывать нечего).
	leg := uuid.Must(uuid.NewV7())
	svc, reader, scopes := newParticipantFeedFake(
		ParticipantScope{UserID: uuid.UUID{}, PropertyIDs: []uuid.UUID{leg}},
		nil, nil,
	)

	page, err := svc.ParticipantFeed(t.Context(), uuid.Must(uuid.NewV7()), "pending@example.com", FeedQuery{})
	if err != nil {
		t.Fatalf("pending feed: %v", err)
	}
	if len(page.Items) != 0 || page.NextCursor != "" || page.PrevCursor != "" {
		t.Fatalf("pending feed must be empty: %+v", page)
	}
	if scopes.calls != 1 {
		t.Fatalf("resolver calls = %d, want one", scopes.calls)
	}
	if reader.gotQuery != nil {
		t.Fatalf("the store must not be touched for a pending invitee")
	}
}

func TestParticipantFeedNoLegsIsEmpty(t *testing.T) {
	t.Parallel()

	// Оборонительный гард: агрегат с uuid, но без ног (контракт требует
	// хотя бы одну) не превращается в глобальную ленту.
	userID := uuid.Must(uuid.NewV7())
	svc, reader, _ := newParticipantFeedFake(
		ParticipantScope{UserID: userID, PropertyIDs: nil},
		nil, nil,
	)

	page, err := svc.ParticipantFeed(t.Context(), uuid.Must(uuid.NewV7()), userID.String(), FeedQuery{})
	if err != nil {
		t.Fatalf("no-legs feed: %v", err)
	}
	if len(page.Items) != 0 || page.NextCursor != "" || page.PrevCursor != "" {
		t.Fatalf("no-legs feed must be empty: %+v", page)
	}
	if reader.gotQuery != nil {
		t.Fatalf("empty legs must not reach the store as a global feed")
	}
}

func TestParticipantFeedForeignParticipantIsPrivacy404(t *testing.T) {
	t.Parallel()

	// Резолвер отвечает сентиеллом доступа (мост в вайре переводит
	// ErrParticipantNotFound доступа на сентиелл истории): наверх —
	// privacy-404, стор не трогается.
	scopes := &scopesFake{err: ErrNotFound}
	reader := &readerFake{}
	svc := NewHistoryReadService(reader, policyFake{}, nil, scopes)

	if _, err := svc.ParticipantFeed(t.Context(), uuid.Must(uuid.NewV7()), "foreign@example.com", FeedQuery{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign participant: want the privacy ErrNotFound, got %v", err)
	}
	if reader.gotQuery != nil {
		t.Fatalf("the store must not be touched when the resolution fails")
	}
}

func TestParticipantFeedTariffGateApplies(t *testing.T) {
	t.Parallel()

	// Скоуп страницы — объекты читающего (он владелец ног): на basic тарифе
	// гейт отвечает ErrTariffRequired, как у всякого сужения на свой объект
	// (ADR 0064, ревизия #1465).
	userID := uuid.Must(uuid.NewV7())
	leg := uuid.Must(uuid.NewV7())
	roles := map[uuid.UUID]sharedpolicy.Role{leg: sharedpolicy.RoleOwner}
	gate := &gateStub{allows: false}
	svc, reader, scopes := newParticipantFeedFake(
		ParticipantScope{UserID: userID, PropertyIDs: []uuid.UUID{leg}},
		roles, gate,
	)

	if _, err := svc.ParticipantFeed(t.Context(), uuid.Must(uuid.NewV7()), userID.String(), FeedQuery{}); !errors.Is(err, ErrTariffRequired) {
		t.Fatalf("participant feed on basic: want ErrTariffRequired, got %v", err)
	}
	if reader.gotQuery != nil {
		t.Fatalf("the store must not be touched when the tariff refuses")
	}
	if scopes.calls != 1 || gate.calls != 1 {
		t.Fatalf("resolver %d / gate %d calls, want one each", scopes.calls, gate.calls)
	}
}

func TestParticipantFeedObjectsFilterIntersectsWithLegs(t *testing.T) {
	t.Parallel()

	// Группа «Объекты» шита страницы (аменда владельца 22.10.2026:
	// «объекты опять же только те, в которые приглашён участник»):
	// клиентский выбор сужает скоуп пересечением с ногами — чужой объект
	// выбора молчит, порядок ног хранится, пустой выбор — все ноги.
	actor := uuid.Must(uuid.NewV7())
	userID := uuid.Must(uuid.NewV7())
	leg1, leg2, leg3 := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	foreign := uuid.Must(uuid.NewV7())
	roles := map[uuid.UUID]sharedpolicy.Role{
		leg1: sharedpolicy.RoleOwner,
		leg2: sharedpolicy.RoleOwner,
		leg3: sharedpolicy.RoleOwner,
	}
	svc, reader, _ := newParticipantFeedFake(
		ParticipantScope{UserID: userID, PropertyIDs: []uuid.UUID{leg1, leg2, leg3}},
		roles, nil,
	)

	if _, err := svc.ParticipantFeed(t.Context(), actor, userID.String(), FeedQuery{
		PropertyIDs: []uuid.UUID{foreign, leg3, leg1},
	}); err != nil {
		t.Fatalf("participant feed with objects filter: %v", err)
	}
	got := reader.gotQuery
	if got == nil {
		t.Fatalf("the store must receive the scoped query")
	}
	if len(got.PropertyIDs) != 2 || got.PropertyIDs[0] != leg1 || got.PropertyIDs[1] != leg3 {
		t.Fatalf("property ids: %+v, want the choice ∩ legs in legs order (leg1, leg3)", got.PropertyIDs)
	}
	if len(got.ActorIDs) != 1 || got.ActorIDs[0] != userID {
		t.Fatalf("actor ids: %+v, want the participant's user uuid", got.ActorIDs)
	}
}

func TestParticipantFeedForeignObjectsOnlyIsEmpty(t *testing.T) {
	t.Parallel()

	// Выбор шита, не задевший ни одной ноги (hand-crafted адрес тоже):
	// пересечение пусто — пустая страница без стора, гард от деградации
	// пустого списка в глобальную ленту (тот же, что у ног).
	userID := uuid.Must(uuid.NewV7())
	leg := uuid.Must(uuid.NewV7())
	svc, reader, _ := newParticipantFeedFake(
		ParticipantScope{UserID: userID, PropertyIDs: []uuid.UUID{leg}},
		map[uuid.UUID]sharedpolicy.Role{leg: sharedpolicy.RoleOwner},
		nil,
	)

	page, err := svc.ParticipantFeed(t.Context(), uuid.Must(uuid.NewV7()), userID.String(), FeedQuery{
		PropertyIDs: []uuid.UUID{uuid.Must(uuid.NewV7())},
	})
	if err != nil {
		t.Fatalf("foreign-objects feed: %v", err)
	}
	if len(page.Items) != 0 || page.NextCursor != "" || page.PrevCursor != "" {
		t.Fatalf("foreign-objects feed must be empty: %+v", page)
	}
	if reader.gotQuery != nil {
		t.Fatalf("empty intersection must not reach the store as a global feed")
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
