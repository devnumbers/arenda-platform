package application

import (
	"context"
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
	feedPropertyName  = "Моя квартира"
	feedInsuranceSlug = "insurance"
)

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

	gotTodays       map[uuid.UUID]time.Time
	gotQuery        GlobalPaymentRulesQuery
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
	return f.categories, nil
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
				PropertyName: feedPropertyName, Type: domain.TypeExpense, Title: "Страхование",
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

	if _, err := service.SearchGlobalPayments(t.Context(), actor, "СТРАХОВ"); err != nil {
		t.Fatalf("SearchGlobalPayments: %v", err)
	}
	got := reader.gotQuery.CategorySlugs
	if len(got) != 1 || got[0] != feedInsuranceSlug {
		t.Errorf("slugs for «СТРАХОВ» = %v, want [%s]", got, feedInsuranceSlug)
	}

	if _, err := service.SearchGlobalPayments(t.Context(), actor, "арендн"); err != nil {
		t.Fatalf("SearchGlobalPayments: %v", err)
	}
	if got := reader.gotQuery.CategorySlugs; len(got) != 1 || got[0] != "rent" {
		t.Errorf("slugs for «арендн» = %v, want [rent]", got)
	}

	if _, err := service.SearchGlobalPayments(t.Context(), actor, "нет такой категории в каталоге"); err != nil {
		t.Fatalf("SearchGlobalPayments: %v", err)
	}
	if got := reader.gotQuery.CategorySlugs; got != nil {
		t.Errorf("slugs for a label-less query = %v, want none", got)
	}

	if _, err := service.SearchGlobalPayments(t.Context(), actor, ""); err != nil {
		t.Fatalf("SearchGlobalPayments: %v", err)
	}
	if got := reader.gotQuery.CategorySlugs; got != nil {
		t.Errorf("slugs for an empty query = %v, want none", got)
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
		{"full label", "Страхование", []string{feedInsuranceSlug}},
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
