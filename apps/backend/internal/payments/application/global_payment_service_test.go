package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
)

// The global payment rules reads' unit tests (ticket #575): the
// enrichment the application layer owns — the owner→today threading, the
// nearest-date fallback onto the pure projection, the overdue age and the
// stacks' grouping — against func-backed fakes; the visibility predicate
// and the aggregates are the store's (integration tests).

// The fixtures' recurring literals; goconst wants named constants.
const (
	parkingSlug = "parking"

	feedPropertyName  = "Моя квартира"
	feedInsuranceSlug = "insurance"
)

// feedInsuranceTitle is the recurring seeded rule title (goconst).
const feedInsuranceTitle = "Страхование"

// fakeGlobalCalendar answers the owner→today map with canned dates.
type fakeGlobalCalendar struct {
	todays map[uuid.UUID]time.Time
}

func (f fakeGlobalCalendar) Today(_ context.Context, ownerID uuid.UUID) (time.Time, error) {
	return f.todays[ownerID], nil
}

// fakeGlobalReader plays the GlobalPaymentReader: canned results plus the
// captured arguments the assertions read back.
type fakeGlobalReader struct {
	owners     []uuid.UUID
	rules      []GlobalPaymentRuleRow
	counters   GlobalPaymentCounters
	categories []GlobalPaymentSearchCategory
	objects    []GlobalPaymentObject
	lastDates  map[uuid.UUID]time.Time
	rulesByID  map[uuid.UUID]domain.Payment
	countRules int64

	countCalled     bool
	gotTodays       map[uuid.UUID]time.Time
	gotQuery        GlobalPaymentRulesQuery
	gotSumQuery     GlobalPaymentRulesQuery
	gotCountQuery   GlobalPaymentRulesQuery
	gotObjectSearch string
}

func (f *fakeGlobalReader) ListGlobalPaymentOwnerTodays(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
	return f.owners, nil
}

func (f *fakeGlobalReader) ListGlobalPaymentRules(
	_ context.Context, _ uuid.UUID, todays map[uuid.UUID]time.Time, q GlobalPaymentRulesQuery,
) ([]GlobalPaymentRuleRow, error) {
	f.gotTodays = todays
	f.gotQuery = q
	return f.rules, nil
}

func (f *fakeGlobalReader) SumGlobalPaymentCounters(
	_ context.Context, _ uuid.UUID, _ map[uuid.UUID]time.Time,
) (GlobalPaymentCounters, error) {
	return f.counters, nil
}

func (f *fakeGlobalReader) SumGlobalPaymentSearchCategories(
	_ context.Context, _ uuid.UUID, q GlobalPaymentRulesQuery,
) ([]GlobalPaymentSearchCategory, error) {
	f.gotSumQuery = q
	return f.categories, nil
}

func (f *fakeGlobalReader) CountGlobalPaymentRules(
	_ context.Context, _ uuid.UUID, _ map[uuid.UUID]time.Time, q GlobalPaymentRulesQuery,
) (int64, error) {
	f.countCalled = true
	f.gotCountQuery = q
	return f.countRules, nil
}

func (f *fakeGlobalReader) ListGlobalPaymentObjects(
	_ context.Context, _ uuid.UUID, search string,
) ([]GlobalPaymentObject, error) {
	f.gotObjectSearch = search
	return f.objects, nil
}

func (f *fakeGlobalReader) LastOperationDatesOfPayments(
	_ context.Context, _ []uuid.UUID,
) (map[uuid.UUID]time.Time, error) {
	return f.lastDates, nil
}

func (f *fakeGlobalReader) GetGlobalPaymentRule(
	_ context.Context, _, _, paymentID uuid.UUID,
) (domain.Payment, bool, error) {
	if rule, ok := f.rulesByID[paymentID]; ok {
		return rule, true, nil
	}
	return domain.Payment{}, false, nil
}

// utcDate builds the module's UTC-midnight calendar date.
func utcDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// The feed enrichment (ticket #575): each row keeps its own owner's today,
// a stored next planned travels out untouched, the overdue aggregates turn
// into the «N дней» age, and the counters pass through.
func TestListGlobalPaymentsEnrichesRows(t *testing.T) {
	t.Parallel()

	ownerA := uuid.Must(uuid.NewV7())
	ownerB := uuid.Must(uuid.NewV7())
	propertyA := uuid.Must(uuid.NewV7())
	propertyB := uuid.Must(uuid.NewV7())
	todayA := utcDate(2026, time.September, 8)
	todayB := utcDate(2026, time.September, 7) // Another owner, another day.
	nextA := utcDate(2026, time.September, 15)
	oldestB := utcDate(2026, time.September, 1)
	slug := feedInsuranceSlug

	reader := &fakeGlobalReader{
		owners: []uuid.UUID{ownerA, ownerB},
		rules: []GlobalPaymentRuleRow{
			{
				ID: uuid.Must(uuid.NewV7()), OwnerID: ownerA, PropertyID: propertyA,
				PropertyName: feedPropertyName, Type: domain.TypeExpense, Title: feedInsuranceTitle,
				AmountKopecks: 3200000, Category: domain.CategoryRef{Slug: &slug},
				IsFavorite: true, Today: todayA, NextPlannedDate: &nextA,
			},
			{
				ID: uuid.Must(uuid.NewV7()), OwnerID: ownerB, PropertyID: propertyB,
				PropertyName: "Съёмная", Type: domain.TypeIncome, Title: "Аренда",
				AmountKopecks: 6000000, Today: todayB,
				OverdueCount:      2,
				OldestOverdueDate: &oldestB,
			},
		},
		counters: GlobalPaymentCounters{FavoriteCount: 1, OverdueOperationsCount: 7},
	}
	service := NewGlobalPaymentService(reader, fakeGlobalCalendar{todays: map[uuid.UUID]time.Time{
		ownerA: todayA,
		ownerB: todayB,
	}}, txStoreFactory{})

	feed, err := service.ListGlobalPayments(t.Context(), uuid.Must(uuid.NewV7()))
	if err != nil {
		t.Fatalf("ListGlobalPayments: %v", err)
	}
	if len(feed.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(feed.Items))
	}
	if feed.FavoriteCount != 1 || feed.OverdueOperationsCount != 7 {
		t.Errorf("counters = (%d, %d), want (1, 7)", feed.FavoriteCount, feed.OverdueOperationsCount)
	}
	first := feed.Items[0]
	if !first.Today.Equal(todayA) || !first.IsFavorite {
		t.Errorf("first row: today = %v, favorite = %v, want today %v and favorite", first.Today, first.IsFavorite, todayA)
	}
	if first.NearestDate == nil || !first.NearestDate.Equal(nextA) {
		t.Errorf("first row: nearest = %v, want %v", first.NearestDate, nextA)
	}
	assertOverdueRow(t, feed.Items[1], todayB)
	if len(reader.gotTodays) != 2 || reader.gotTodays[ownerA].IsZero() {
		t.Errorf("reader got todays = %v, want both owners", reader.gotTodays)
	}
}

// assertOverdueRow checks the second fixture row: the second owner's today,
// the overdue aggregates, and no nearest date without stored planned or
// projection input.
func assertOverdueRow(t *testing.T, second GlobalPaymentItem, todayB time.Time) {
	t.Helper()
	if !second.Today.Equal(todayB) {
		t.Errorf("second row: today = %v, want the second owner's %v", second.Today, todayB)
	}
	if second.OverdueCount != 2 || second.OverdueDays == nil || *second.OverdueDays != 6 {
		t.Errorf("second row: overdue = (%d, %v), want (2, 6)", second.OverdueCount, second.OverdueDays)
	}
	if second.NearestDate != nil {
		t.Errorf("second row: nearest = %v, want nil without stored planned or projection input", second.NearestDate)
	}
}

// The nearest-date fallback (ticket #575 «planned-операция или проекция»):
// a row without a stored next planned takes the pure projection — the
// first occurrence after the newest materialized date, or after yesterday
// when nothing has been materialized. An open pause projects nothing (the
// true null) and a rule gone mid-read degrades to a null nearest date.
func TestListGlobalPaymentsNearestDateFallback(t *testing.T) {
	t.Parallel()

	owner := uuid.Must(uuid.NewV7())
	property := uuid.Must(uuid.NewV7())
	today := utcDate(2026, time.September, 8)
	lastMaterialized := utcDate(2026, time.September, 1)

	projected := utcDate(2026, time.September, 20)
	recurrence, err := domain.NewMonthlyRecurrence([]int{20}, false)
	if err != nil {
		t.Fatalf("monthly recurrence: %v", err)
	}
	projectable := domain.Payment{
		ID: uuid.Must(uuid.NewV7()), OwnerID: owner, PropertyID: property,
		Title: "Проекция", AmountKopecks: 1, Recurrence: recurrence,
		Since: utcDate(2026, time.August, 1),
	}
	pausedRule := domain.Payment{
		ID: uuid.Must(uuid.NewV7()), OwnerID: owner, PropertyID: property,
		Title: "Пауза", AmountKopecks: 1, Recurrence: recurrence,
		Since:  utcDate(2026, time.August, 1),
		Pauses: []domain.PauseInterval{{From: utcDate(2026, time.September, 1)}},
	}

	rowFor := func(payment domain.Payment) GlobalPaymentRuleRow {
		return GlobalPaymentRuleRow{
			ID: payment.ID, OwnerID: owner, PropertyID: property,
			PropertyName: "Объект", Type: domain.TypeExpense, Title: payment.Title,
			AmountKopecks: 1, Today: today,
		}
	}
	reader := &fakeGlobalReader{
		owners: []uuid.UUID{owner},
		rules: []GlobalPaymentRuleRow{
			rowFor(projectable),
			rowFor(pausedRule),
		},
		lastDates: map[uuid.UUID]time.Time{projectable.ID: lastMaterialized},
		rulesByID: map[uuid.UUID]domain.Payment{
			projectable.ID: projectable,
			pausedRule.ID:  pausedRule,
		},
	}
	service := NewGlobalPaymentService(reader, fakeGlobalCalendar{todays: map[uuid.UUID]time.Time{owner: today}}, txStoreFactory{})

	feed, err := service.ListGlobalPayments(t.Context(), uuid.Must(uuid.NewV7()))
	if err != nil {
		t.Fatalf("ListGlobalPayments: %v", err)
	}
	if got := feed.Items[0].NearestDate; got == nil || !got.Equal(projected) {
		t.Errorf("projected nearest = %v, want %v", got, projected)
	}
	if got := feed.Items[1].NearestDate; got != nil {
		t.Errorf("paused nearest = %v, want nil", got)
	}
}

// The search (ticket #575): the query expands into the default catalog
// slugs whose label contains it (case-insensitively) and travels to the
// reader beside the raw query; the matched categories pass through.
func TestSearchGlobalPaymentsExpandsCategorySlugs(t *testing.T) {
	t.Parallel()

	owner := uuid.Must(uuid.NewV7())
	today := utcDate(2025, time.June, 1) // Arbitrary: the search never reads it.
	reader := &fakeGlobalReader{owners: []uuid.UUID{owner}}
	service := NewGlobalPaymentService(reader, fakeGlobalCalendar{todays: map[uuid.UUID]time.Time{owner: today}}, txStoreFactory{})
	actor := uuid.Must(uuid.NewV7())

	if _, err := service.SearchGlobalPayments(t.Context(), actor, "СТРАХОВ", GlobalPaymentSearchPage{}); err != nil {
		t.Fatalf("SearchGlobalPayments: %v", err)
	}
	got := reader.gotQuery.CategorySlugs
	if len(got) != 1 || got[0] != feedInsuranceSlug {
		t.Errorf("slugs for «СТРАХОВ» = %v, want [%s]", got, feedInsuranceSlug)
	}

	if _, err := service.SearchGlobalPayments(t.Context(), actor, "арендн", GlobalPaymentSearchPage{}); err != nil {
		t.Fatalf("SearchGlobalPayments: %v", err)
	}
	if got := reader.gotQuery.CategorySlugs; len(got) != 1 || got[0] != "rent" {
		t.Errorf("slugs for «арендн» = %v, want [rent]", got)
	}

	if _, err := service.SearchGlobalPayments(t.Context(), actor, "нет такой категории в каталоге", GlobalPaymentSearchPage{}); err != nil {
		t.Fatalf("SearchGlobalPayments: %v", err)
	}
	if got := reader.gotQuery.CategorySlugs; got != nil {
		t.Errorf("slugs for a label-less query = %v, want none", got)
	}

	if _, err := service.SearchGlobalPayments(t.Context(), actor, "", GlobalPaymentSearchPage{}); err != nil {
		t.Fatalf("SearchGlobalPayments: %v", err)
	}
	if got := reader.gotQuery.CategorySlugs; got != nil {
		t.Errorf("slugs for an empty query = %v, want none", got)
	}
}

// newSearchPageService builds the search service over the reader with the
// fixed owner's today — the page tests' common wiring.
func newSearchPageService(reader *fakeGlobalReader, owner uuid.UUID) *GlobalPaymentService {
	today := utcDate(2025, time.June, 1) // Arbitrary: the page never reads it.
	return NewGlobalPaymentService(reader, fakeGlobalCalendar{todays: map[uuid.UUID]time.Time{owner: today}}, txStoreFactory{})
}

// The search page (map #573, rework): the zero page degenerates to the
// contract's default — the first 50-row page — and the chip filter travels
// to the rules read beside the raw search query; the keyset continuation
// (ticket #597) decodes into the After* keyset key, the empty cursor being
// the list's beginning.
func TestSearchGlobalPaymentsPage(t *testing.T) {
	t.Parallel()

	owner := uuid.Must(uuid.NewV7())
	reader := &fakeGlobalReader{owners: []uuid.UUID{owner}}
	service := newSearchPageService(reader, owner)
	actor := uuid.Must(uuid.NewV7())

	// The zero page (the contract's omitted params) is the default first
	// page: 50 rows, no chip filter, no keyset key.
	if _, err := service.SearchGlobalPayments(t.Context(), actor, "аренд", GlobalPaymentSearchPage{}); err != nil {
		t.Fatalf("SearchGlobalPayments: %v", err)
	}
	if got := reader.gotQuery; got.Limit != DefaultPaymentRulesPageSize {
		t.Errorf("zero page = limit %d, want limit %d", got.Limit, DefaultPaymentRulesPageSize)
	}
	if got := reader.gotQuery; got.Category != "" || got.Type != "" {
		t.Errorf("zero page filter = %q/%q, want none", got.Category, got.Type)
	}
	if got := reader.gotQuery; got.AfterCreatedAt != nil || got.AfterID != nil {
		t.Errorf("zero page keyset key = %v/%v, want none", got.AfterCreatedAt, got.AfterID)
	}

	// The explicit page carries the chip's identity and the window through.
	page := GlobalPaymentSearchPage{Category: parkingSlug, Type: domain.TypeExpense, Limit: 50}
	if _, err := service.SearchGlobalPayments(t.Context(), actor, "", page); err != nil {
		t.Fatalf("SearchGlobalPayments: %v", err)
	}
	got := reader.gotQuery
	if got.Category != parkingSlug || got.Type != domain.TypeExpense {
		t.Errorf("page filter = %q/%q, want parking/expense", got.Category, got.Type)
	}
	if got.Limit != 50 {
		t.Errorf("page window = limit %d, want limit 50", got.Limit)
	}
}

// The continuation cursor (ticket #597) decodes into the keyset key the
// page resumes after; a malformed one is the contract's 400.
func TestSearchGlobalPaymentsPageCursor(t *testing.T) {
	t.Parallel()

	owner := uuid.Must(uuid.NewV7())
	reader := &fakeGlobalReader{owners: []uuid.UUID{owner}}
	service := newSearchPageService(reader, owner)
	actor := uuid.Must(uuid.NewV7())

	cursorAt := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	cursorID := uuid.Must(uuid.NewV7())
	page := GlobalPaymentSearchPage{Limit: 50, Cursor: encodeRuleCursor(cursorAt, cursorID)}
	if _, err := service.SearchGlobalPayments(t.Context(), actor, "", page); err != nil {
		t.Fatalf("SearchGlobalPayments with cursor: %v", err)
	}
	got := reader.gotQuery
	if got.AfterCreatedAt == nil || !got.AfterCreatedAt.Equal(cursorAt) || got.AfterID == nil || *got.AfterID != cursorID {
		t.Errorf("keyset key = %v/%v, want %v/%s", got.AfterCreatedAt, got.AfterID, cursorAt, cursorID)
	}

	page = GlobalPaymentSearchPage{Limit: 50, Cursor: "!!!"}
	if _, err := service.SearchGlobalPayments(t.Context(), actor, "", page); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("malformed cursor err = %v, want ErrInvalidInput", err)
	}
}

// The matched categories (the chips) always describe the query's whole
// matched scope: the category/type filter narrows the rules list only.
func TestSearchGlobalPaymentsSumIgnoresChipFilter(t *testing.T) {
	t.Parallel()

	owner := uuid.Must(uuid.NewV7())
	reader := &fakeGlobalReader{owners: []uuid.UUID{owner}}
	service := newSearchPageService(reader, owner)
	actor := uuid.Must(uuid.NewV7())

	page := GlobalPaymentSearchPage{Category: parkingSlug, Type: domain.TypeExpense, Limit: 50}
	if _, err := service.SearchGlobalPayments(t.Context(), actor, "аренд", page); err != nil {
		t.Fatalf("SearchGlobalPayments: %v", err)
	}
	sum := reader.gotSumQuery
	if sum.Category != "" || sum.Type != "" {
		t.Errorf("sum query filter = %q/%q, want none — the chips stay whole", sum.Category, sum.Type)
	}
	if sum.Search != "аренд" {
		t.Errorf("sum query search = %q, want «аренд»", sum.Search)
	}
}

// The «Объекты» stacks (ticket #575): the keys group per property into the
// auto-pay group and the rest, every key carrying its overdue flag; the
// stacks always take the object's every rule whatever the object search
// matched.
func TestListGlobalPaymentObjectsGroupsStacks(t *testing.T) {
	t.Parallel()

	owner := uuid.Must(uuid.NewV7())
	property := uuid.Must(uuid.NewV7())
	otherProperty := uuid.Must(uuid.NewV7())
	today := utcDate(2026, time.September, 8)
	plainRule := uuid.Must(uuid.NewV7())
	overdueRule := uuid.Must(uuid.NewV7())

	reader := &fakeGlobalReader{
		owners: []uuid.UUID{owner},
		rules: []GlobalPaymentRuleRow{
			{ID: plainRule, PropertyID: property, Today: today},
			{ID: overdueRule, PropertyID: property, Today: today, AutoPay: true, OverdueCount: 1},
			{ID: uuid.Must(uuid.NewV7()), PropertyID: otherProperty, Today: today},
		},
		objects: []GlobalPaymentObject{
			{PropertyID: property, Name: "Моя квартира", Address: "Тверская 1"},
		},
	}
	service := NewGlobalPaymentService(reader, fakeGlobalCalendar{todays: map[uuid.UUID]time.Time{owner: today}}, txStoreFactory{})

	cards, err := service.ListGlobalPaymentObjects(t.Context(), uuid.Must(uuid.NewV7()), "квартира")
	if err != nil {
		t.Fatalf("ListGlobalPaymentObjects: %v", err)
	}
	if len(cards) != 1 {
		t.Fatalf("cards = %d, want 1", len(cards))
	}
	card := cards[0]
	if card.Name != "Моя квартира" || card.Address != "Тверская 1" {
		t.Errorf("card = (%s, %s), want the searched object", card.Name, card.Address)
	}
	if len(card.AutoPayKeys) != 1 || card.AutoPayKeys[0].PaymentID != overdueRule || !card.AutoPayKeys[0].HasOverdue {
		t.Errorf("auto-pay keys = %+v, want the overdue rule with the dot", card.AutoPayKeys)
	}
	if len(card.OtherKeys) != 1 || card.OtherKeys[0].PaymentID != plainRule || card.OtherKeys[0].HasOverdue {
		t.Errorf("other keys = %+v, want the plain rule without the dot", card.OtherKeys)
	}
	if reader.gotObjectSearch != "квартира" {
		t.Errorf("object search = %q, want the query", reader.gotObjectSearch)
	}
}

// The default catalog slug expansion mirrors the SQL ILIKE contains.
func TestDefaultCategorySlugsMatching(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{"empty query matches nothing", "", nil},
		{"case-insensitive label substring", "страхов", []string{feedInsuranceSlug}},
		{"full label", feedInsuranceTitle, []string{feedInsuranceSlug}},
		{"no such label", "бассейн", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := defaultCategorySlugsMatching(tc.query)
			if len(got) != len(tc.want) {
				t.Fatalf("slugs = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("slugs = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// The search's total (ticket #599): the whole scope's match count under the
// current query travels with every page — the count's predicate mirrors the
// list's search and chip filters but carries no keyset key (the cursor only
// positions the window), and an empty visible scope counts nothing.
func TestSearchGlobalPaymentsCountsMatches(t *testing.T) {
	t.Parallel()

	owner := uuid.Must(uuid.NewV7())
	today := utcDate(2026, time.September, 11)
	reader := &fakeGlobalReader{
		owners: []uuid.UUID{owner},
		rules: []GlobalPaymentRuleRow{
			{
				ID: uuid.Must(uuid.NewV7()), OwnerID: owner, PropertyID: uuid.Must(uuid.NewV7()),
				PropertyName: feedPropertyName, Type: domain.TypeExpense, Title: feedInsuranceTitle,
				AmountKopecks: 3200000, Today: today,
			},
		},
		countRules: 7,
	}
	service := NewGlobalPaymentService(reader, fakeGlobalCalendar{todays: map[uuid.UUID]time.Time{
		owner: today,
	}}, txStoreFactory{})

	// A full page walks on with the cursor; the total still counts the whole
	// matched scope (7), not the window or its remainder.
	search, err := service.SearchGlobalPayments(t.Context(), owner, "страх", GlobalPaymentSearchPage{
		Limit: 1, Category: feedInsuranceSlug, Type: domain.TypeExpense,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if search.Total != 7 {
		t.Fatalf("total = %d, want 7", search.Total)
	}
	if search.NextCursor == "" {
		t.Fatal("nextCursor = '', want the continuation of the full page")
	}
	if !reader.countCalled {
		t.Fatal("CountGlobalPaymentRules was not called")
	}
	if reader.gotCountQuery.Search != "страх" ||
		reader.gotCountQuery.Category != feedInsuranceSlug ||
		reader.gotCountQuery.Type != domain.TypeExpense {
		t.Fatalf("count query = %+v, want the list's search and chip filter", reader.gotCountQuery)
	}
	if len(reader.gotCountQuery.CategorySlugs) != len(defaultCategorySlugsMatching("страх")) {
		t.Fatalf("count query slugs = %v, want the expanded label slugs", reader.gotCountQuery.CategorySlugs)
	}
	if reader.gotCountQuery.AfterCreatedAt != nil || reader.gotCountQuery.AfterID != nil {
		t.Fatalf("count query keyset key = %v/%v, want nil/nil",
			reader.gotCountQuery.AfterCreatedAt, reader.gotCountQuery.AfterID)
	}

	// An empty visible scope counts nothing and does not reach the store.
	empty := &fakeGlobalReader{}
	service = NewGlobalPaymentService(empty, fakeGlobalCalendar{}, txStoreFactory{})
	search, err = service.SearchGlobalPayments(t.Context(), owner, "", GlobalPaymentSearchPage{})
	if err != nil {
		t.Fatalf("empty-scope search: %v", err)
	}
	if search.Total != 0 {
		t.Fatalf("empty scope total = %d, want 0", search.Total)
	}
	if empty.countCalled {
		t.Fatal("count reached the store on an empty scope")
	}
}
