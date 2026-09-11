//go:build integration

package application_test

// The global payment rules reads' integration tests (ticket #575): the
// actor-scoped cross-property read over the rules of the own book plus
// active-membership properties (ADR 0028), the archived ones excluded, the
// per-owner today threading (ADR 0048 — the merged feed mixes owners), the
// overdue aggregates and counters, the search's matched-category chips and
// the «Объекты» stacks, against real PostgreSQL through the shared harness.

import (
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	auditpg "github.com/nambers/arenda-planform/apps/backend/internal/audit/adapters/postgres"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	paymentspg "github.com/nambers/arenda-planform/apps/backend/internal/payments/adapters/postgres"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	pgdb "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
)

// dateOf renders a row's calendar date the way the assertions read it.
func dateOf(t time.Time) string { return t.Format("2006-01-02") }

// Recurring literals of the search-page test (goconst).
const (
	parkingSlug  = "parking"
	typeExpense  = "expense"
	typeIncome   = "income"
	mirrorsTitle = "Зеркала"
)

// objectPropertyName is the harness's own property label in the fixtures;
// goconst wants the recurring literal named.
const objectPropertyName = "Квартира"

// globalSvc builds the global payment rules read service over the harness's
// pool and clock — the wire's shape.
func (h *paymentsHarness) globalSvc() *paymentsapp.GlobalPaymentService {
	h.t.Helper()
	calendar := paymentspg.NewOwnerCalendar(h.pool, h.clock)
	store := paymentspg.NewGlobalPaymentStore(h.pool)
	audit := auditapp.NewService(auditpg.NewWriter(h.pool), h.clock)
	factory := paymentsapp.NewTxStoreFactory(
		paymentspg.NewTickStore(h.pool),
		paymentspg.NewPaymentStore(h.pool),
		paymentspg.NewOperationStore(h.pool),
		paymentspg.NewPropertyStore(h.pool),
		store,
		audit,
		pgdb.NewUoW(h.pool, slog.New(slog.DiscardHandler)),
	)
	return paymentsapp.NewGlobalPaymentService(store, calendar, factory)
}

// seedGlobalRule inserts a payment rule on any property with any owner —
// the row-level twin of seedGlobalProperty for multi-book fixtures.
func (h *paymentsHarness) seedGlobalRule(
	propertyID, ownerID uuid.UUID,
	title, typ string, amountKopecks int64,
	autoPay, isFavorite bool, slug string, userCategoryID *uuid.UUID,
) uuid.UUID {
	h.t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		h.t.Fatalf("new uuid: %v", err)
	}
	var slugArg any
	if slug != "" {
		slugArg = slug
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks,
		                       recurrence, since, auto_pay, payment_form, category_slug, user_category_id, is_favorite)
		 VALUES ($1, $2, $3, $4, $5, $6, '{"kind":"monthly","daysOfMonth":[1]}'::jsonb, '2026-07-01', $7, 'transfer', $8, $9, $10)`,
		id, ownerID, propertyID, typ, title, amountKopecks, autoPay, slugArg, userCategoryID, isFavorite,
	); err != nil {
		h.t.Fatalf("seed rule %s: %v", title, err)
	}
	return id
}

// seedRuleOperation inserts one operation bound to a rule on any property —
// the scheduled history the feed's aggregates read.
func (h *paymentsHarness) seedRuleOperation(
	ruleID, propertyID, ownerID uuid.UUID, date, status, typ, title string, amountKopecks int64,
) uuid.UUID {
	h.t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		h.t.Fatalf("new uuid: %v", err)
	}
	var paid any
	if status == opPaid {
		paid = date
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO operations (id, owner_id, property_id, payment_id, origin, date, paid_date,
		                       status, type, title, amount_kopecks, payment_form, category_label, category_slug)
		 VALUES ($1, $2, $3, $4, 'payment', $5, $6::date, $7, $8, $9, $10, 'transfer', 'Прочее', NULL)`,
		id, ownerID, propertyID, ruleID, date, paid, status, typ, title, amountKopecks,
	); err != nil {
		h.t.Fatalf("seed rule operation %s (%s): %v", title, date, err)
	}
	return id
}

// seedGlobalUser seeds a user with the given timezone and returns the id —
// the multi-owner fixture needs owners whose calendar days differ.
func (h *paymentsHarness) seedGlobalUser(tz string) uuid.UUID {
	h.t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		h.t.Fatalf("new uuid: %v", err)
	}
	phone := fmt.Sprintf("+7999%010d", time.Now().UnixNano()+int64(len(tz)))
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO users (id, phone, role, timezone) VALUES ($1, $2, 'owner', $3)`,
		id, phone, tz,
	); err != nil {
		h.t.Fatalf("seed user: %v", err)
	}
	return id
}

// seedUserCategory inserts a user category of the given owner.
func (h *paymentsHarness) seedUserCategory(ownerID uuid.UUID, name string) uuid.UUID {
	h.t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		h.t.Fatalf("new uuid: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO payment_categories (id, owner_id, name) VALUES ($1, $2, $3)`,
		id, ownerID, name,
	); err != nil {
		h.t.Fatalf("seed user category: %v", err)
	}
	return id
}

// The merged feed (ticket #575): the own and shared-active rules are in,
// the archived and foreign ones are not; every row carries its property's
// name and its owner's calendar date, the overdue aggregates resolve
// against that date, and a rule without stored planned takes the pure
// projection. The harness clock (2026-08-25 20:00 UTC) makes Moscow's today
// the 25th and Kamchatka's the 26th.
func TestListGlobalPayments_MergedFeed(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	svc := h.globalSvc()

	favorite := h.seedGlobalRule(h.propID, h.owner, "Страхование", "expense", 3200000,
		false, true, "insurance", nil)
	tombstone := h.seedRuleOperation(favorite, h.propID, h.owner, "2026-08-10", opCancelled, "expense", "Страхование", 3200000)
	oldestOverdue := h.seedRuleOperation(favorite, h.propID, h.owner, "2026-08-15", opPlanned, "expense", "Страхование", 3200000)
	h.seedRuleOperation(favorite, h.propID, h.owner, "2026-08-20", opPlanned, "expense", "Страхование", 3200000)
	h.seedRuleOperation(favorite, h.propID, h.owner, "2026-08-30", opPlanned, "expense", "Страхование", 3200000)
	_ = tombstone

	h.seedGlobalRule(h.propID, h.owner, "Клининг", "expense", 90000, true, false, "cleaning", nil)

	partner := h.seedGlobalUser("Asia/Kamchatka")
	shared := h.seedGlobalProperty(partner, "Чужая дача", "active")
	h.seedMembership(shared, h.owner, partner, "viewer", "active")
	rent := h.seedGlobalRule(shared, partner, "Аренда", "income", 6000000,
		false, false, "rent", nil)
	h.seedRuleOperation(rent, shared, partner, "2026-08-25", opPlanned, "income", "Аренда", 6000000)
	h.seedRuleOperation(rent, shared, partner, "2026-09-01", opPlanned, "income", "Аренда", 6000000)

	archived := h.seedGlobalProperty(h.owner, "Старый объект", "archived")
	h.seedGlobalRule(archived, h.owner, "ЖКУ архива", "expense", 100000, false, false, "utilities", nil)

	foreignOwner := h.seedGlobalUser("Europe/Moscow")
	foreign := h.seedGlobalProperty(foreignOwner, "Без доступа", "active")
	h.seedGlobalRule(foreign, foreignOwner, "Чужое", "expense", 100000, false, false, "utilities", nil)

	feed, err := svc.ListGlobalPayments(h.ctx(), h.owner)
	if err != nil {
		t.Fatalf("list global payments: %v", err)
	}
	if len(feed.Items) != 3 {
		t.Fatalf("feed = %d rules, want the three visible ones (archived and foreign cut)", len(feed.Items))
	}
	byTitle := map[string]paymentsapp.GlobalPaymentItem{}
	for _, item := range feed.Items {
		byTitle[item.Title] = item
	}
	assertFavoriteRow(t, byTitle["Страхование"], oldestOverdue)
	assertProjectionRow(t, byTitle["Клининг"])
	assertSharedRow(t, byTitle["Аренда"])

	if feed.FavoriteCount != 1 {
		t.Errorf("favoriteCount = %d, want 1", feed.FavoriteCount)
	}
	if feed.OverdueOperationsCount != 3 {
		t.Errorf("overdueOperationsCount = %d, want 3 (2 + 1 across the owners' todays)", feed.OverdueOperationsCount)
	}
}

// assertFavoriteRow checks the favorite fixture row: the label, the star,
// the overdue aggregates (the tombstone never counts) and the stored next
// date against the Moscow owner's today.
func assertFavoriteRow(t *testing.T, fav paymentsapp.GlobalPaymentItem, wantOldest uuid.UUID) {
	t.Helper()
	if fav.PropertyName != objectPropertyName || !fav.IsFavorite || fav.AutoPay {
		t.Errorf("favorite = (%q, %v, %v), want the quarter, favorite, non-auto", fav.PropertyName, fav.IsFavorite, fav.AutoPay)
	}
	if fav.OverdueCount != 2 || fav.OverdueDays == nil || *fav.OverdueDays != 10 {
		t.Errorf("favorite overdue = (%d, %v), want (2, 10) — the tombstone never counts", fav.OverdueCount, fav.OverdueDays)
	}
	if fav.OldestOverdueOperationID == nil || *fav.OldestOverdueOperationID != wantOldest {
		t.Errorf("favorite oldest overdue operation = %v, want the 2026-08-15 planned one %v", fav.OldestOverdueOperationID, wantOldest)
	}
	if fav.NearestDate == nil || dateOf(*fav.NearestDate) != "2026-08-30" {
		t.Errorf("favorite nearest = %v, want 2026-08-30", fav.NearestDate)
	}
	if dateOf(fav.Today) != "2026-08-25" {
		t.Errorf("favorite today = %s, want the Moscow owner's 2026-08-25", dateOf(fav.Today))
	}
}

// assertProjectionRow checks the no-operations rule: the nearest date is
// the pure projection — the first monthly day-of-1 after yesterday (the
// cursor: nothing materialized).
func assertProjectionRow(t *testing.T, plain paymentsapp.GlobalPaymentItem) {
	t.Helper()
	if plain.NearestDate == nil || dateOf(*plain.NearestDate) != "2026-09-01" {
		t.Errorf("projection nearest = %v, want 2026-09-01", plain.NearestDate)
	}
	if plain.OverdueCount != 0 || plain.OverdueDays != nil {
		t.Errorf("projection overdue = (%d, %v), want (0, nil)", plain.OverdueCount, plain.OverdueDays)
	}
	if plain.OldestOverdueOperationID != nil {
		t.Errorf("projection oldest overdue operation = %v, want nil", plain.OldestOverdueOperationID)
	}
}

// assertSharedRow checks the shared rule: the schedule math runs against
// the property owner's today (Kamchatka, the 26th) — the 25th-planned
// occurrence is overdue there, though it is not for the Moscow actor.
func assertSharedRow(t *testing.T, sharedItem paymentsapp.GlobalPaymentItem) {
	t.Helper()
	if dateOf(sharedItem.Today) != "2026-08-26" {
		t.Errorf("shared today = %s, want the Kamchatka owner's 2026-08-26", dateOf(sharedItem.Today))
	}
	if sharedItem.OverdueCount != 1 || sharedItem.OverdueDays == nil || *sharedItem.OverdueDays != 1 {
		t.Errorf("shared overdue = (%d, %v), want (1, 1) against the owner's today", sharedItem.OverdueCount, sharedItem.OverdueDays)
	}
}

// The search (ticket #575): the title, the default catalog label and the
// user category name all match; the matched categories are the matched
// rules' category identities — the chips — with their rule counts.
func TestSearchGlobalPayments(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	svc := h.globalSvc()

	h.seedGlobalRule(h.propID, h.owner, "Страхование квартиры", "expense", 3200000,
		false, false, "insurance", nil)
	h.seedGlobalRule(h.propID, h.owner, "Интернет", "expense", 700000,
		false, false, "utilities", nil)
	category := h.seedUserCategory(h.owner, "Кофейни")
	h.seedGlobalRule(h.propID, h.owner, mirrorsTitle, "income", 150000,
		false, false, "", &category)

	cases := []struct {
		name      string
		query     string
		wantItems []string
		wantChips map[string]int64 // «slug or label» → count; the label keys the custom chip.
	}{
		{"the title matches", "страх", []string{"Страхование квартиры"}, map[string]int64{"insurance": 1}},
		{"the default catalog label matches", "коммунал", []string{"Интернет"}, map[string]int64{"utilities": 1}},
		{"the user category name matches", "кофейн", []string{mirrorsTitle}, map[string]int64{"Кофейни": 1}},
		{"the chips carry the matched rules' categories", "терне", []string{"Интернет"}, map[string]int64{"utilities": 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			search, err := svc.SearchGlobalPayments(h.ctx(), h.owner, tc.query, paymentsapp.GlobalPaymentSearchPage{})
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			assertSearchTitles(t, search, tc.wantItems)
			assertSearchChips(t, search, tc.wantChips)
		})
	}
}

// The chip filter and the page (map #573 rework): the category+type
// identity narrows the rules list while matchedCategories keep describing
// the whole query scope; the window walks the matched rows by the feed's
// stable order.
func TestSearchGlobalPaymentsChipFilterAndPage(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	svc := h.globalSvc()

	h.seedGlobalRule(h.propID, h.owner, "Парковка", typeExpense, 20000, false, false, "parking", nil)
	h.seedGlobalRule(h.propID, h.owner, "Аренда машиноместа", typeIncome, 200000, false, false, "parking", nil)
	category := h.seedUserCategory(h.owner, "Кофейни")
	h.seedGlobalRule(h.propID, h.owner, mirrorsTitle, typeIncome, 150000, false, false, "", &category)
	h.seedGlobalRule(h.propID, h.owner, "Страхование квартиры", typeExpense, 3200000, false, false, "insurance", nil)

	// The default-catalog chip (parking, expense): only the expense parking
	// rule; the chips still describe the whole query scope. The scope's
	// chips are the (category, direction) pairs — parking carries both
	// directions here, one rule each.
	search, err := svc.SearchGlobalPayments(h.ctx(), h.owner, "", paymentsapp.GlobalPaymentSearchPage{
		Category: parkingSlug, Type: typeExpense, Limit: 50,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	assertSearchTitles(t, search, []string{"Парковка"})
	// The chips are the (category, direction) pairs: parking carries both
	// directions here, one rule each; Кофейни is the user category's chip.
	type chipKey struct {
		identity string
		typ      string
	}
	got := make(map[chipKey]int64, len(search.MatchedCategories))
	for _, chip := range search.MatchedCategories {
		identity := ""
		switch {
		case chip.Category.Slug != nil:
			identity = *chip.Category.Slug
		case chip.Category.UserCategoryName != nil:
			identity = *chip.Category.UserCategoryName
		}
		got[chipKey{identity, string(chip.Type)}] = chip.RuleCount
	}
	want := map[chipKey]int64{
		{parkingSlug, typeExpense}: 1,
		{parkingSlug, typeIncome}:  1,
		{"insurance", typeExpense}: 1,
		{"Кофейни", typeIncome}:    1,
	}
	if len(got) != len(want) {
		t.Fatalf("chips = %+v, want %v", got, want)
	}
	for key, count := range want {
		if got[key] != count {
			t.Errorf("chip %v count = %d, want %d", key, got[key], count)
		}
	}

	// The user category chip matches by the category's id, not its name.
	search, err = svc.SearchGlobalPayments(h.ctx(), h.owner, "", paymentsapp.GlobalPaymentSearchPage{
		Category: category.String(), Type: typeIncome, Limit: 50,
	})
	if err != nil {
		t.Fatalf("search by user category: %v", err)
	}
	assertSearchTitles(t, search, []string{mirrorsTitle})

	// The window walks the matched rows in the feed's own (created_at, id)
	// order by keyset (ticket #597): two per page, the cursor resumes
	// strictly after the previous page's last row.
	search, err = svc.SearchGlobalPayments(h.ctx(), h.owner, "", paymentsapp.GlobalPaymentSearchPage{Limit: 2})
	if err != nil {
		t.Fatalf("search page one: %v", err)
	}
	assertSearchTitles(t, search, []string{"Парковка", "Аренда машиноместа"})
	if search.NextCursor == "" {
		t.Fatal("page one nextCursor = '', want the continuation of the four-row feed")
	}
	search, err = svc.SearchGlobalPayments(h.ctx(), h.owner, "", paymentsapp.GlobalPaymentSearchPage{Limit: 2, Cursor: search.NextCursor})
	if err != nil {
		t.Fatalf("search page two: %v", err)
	}
	assertSearchTitles(t, search, []string{mirrorsTitle, "Страхование квартиры"})
	// A full page still answers with a continuation — the walk stops on the
	// short page it produces: the empty third page ends the matches.
	search, err = svc.SearchGlobalPayments(h.ctx(), h.owner, "", paymentsapp.GlobalPaymentSearchPage{Limit: 2, Cursor: search.NextCursor})
	if err != nil {
		t.Fatalf("search page three: %v", err)
	}
	if len(search.Items) != 0 || search.NextCursor != "" {
		t.Fatalf("page three = %d rows/%q, want the empty exhausted page", len(search.Items), search.NextCursor)
	}
}

// assertSearchTitles checks the matched rows' titles in order.
func assertSearchTitles(t *testing.T, search paymentsapp.GlobalPaymentSearch, wantItems []string) {
	t.Helper()
	titles := make([]string, 0, len(search.Items))
	for _, item := range search.Items {
		titles = append(titles, item.Title)
	}
	if len(titles) != len(wantItems) {
		t.Fatalf("items = %v, want %v", titles, wantItems)
	}
	for i := range wantItems {
		if titles[i] != wantItems[i] {
			t.Fatalf("items = %v, want %v", titles, wantItems)
		}
	}
}

// assertSearchChips checks the matched categories against the expected
// identity→count map; the label keys the custom chip, the slug the default
// one.
func assertSearchChips(t *testing.T, search paymentsapp.GlobalPaymentSearch, wantChips map[string]int64) {
	t.Helper()
	if len(search.MatchedCategories) != len(wantChips) {
		t.Fatalf("chips = %+v, want %v", search.MatchedCategories, wantChips)
	}
	for _, chip := range search.MatchedCategories {
		var key string
		if chip.Category.Slug != nil {
			key = *chip.Category.Slug
		} else if chip.Category.UserCategoryName != nil {
			key = *chip.Category.UserCategoryName
		}
		if wantChips[key] != chip.RuleCount {
			t.Errorf("chip %q count = %d, want %d", key, chip.RuleCount, wantChips[key])
		}
	}
}

// The «Объекты» stacks (ticket #575): the visible non-archived properties
// with their rules grouped into the auto-pay group and the rest, every key
// carrying its overdue flag; the search filters the objects, the stacks
// stay whole; a suspended membership grants no read.
func TestListGlobalPaymentObjects(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	svc := h.globalSvc()

	autoOverdue := h.seedGlobalRule(h.propID, h.owner, "ЖКУ", typeExpense, 500000, true, false, "utilities", nil)
	h.seedRuleOperation(autoOverdue, h.propID, h.owner, "2026-08-01", opPlanned, typeExpense, "ЖКУ", 500000)
	h.seedGlobalRule(h.propID, h.owner, "Интернет", typeExpense, 700000, true, false, "utilities", nil)
	plainOverdue := h.seedGlobalRule(h.propID, h.owner, "Страхование", typeExpense, 3200000, false, false, "insurance", nil)
	h.seedRuleOperation(plainOverdue, h.propID, h.owner, "2026-08-10", opPlanned, typeExpense, "Страхование", 3200000)
	h.seedGlobalRule(h.propID, h.owner, "Клининг", typeExpense, 90000, false, false, "cleaning", nil)

	h.seedGlobalProperty(h.owner, "Дом", "active") // No rules: empty groups.

	partner := h.seedGlobalUser("Europe/Moscow")
	dacha := h.seedGlobalProperty(partner, "Чужая дача", "active")
	h.seedMembership(dacha, h.owner, partner, "viewer", "active")
	dachaRule := h.seedGlobalRule(dacha, partner, "Аренда", typeIncome, 6000000, false, false, "rent", nil)
	h.seedRuleOperation(dachaRule, dacha, partner, "2026-08-01", opPlanned, typeIncome, "Аренда", 6000000)

	archived := h.seedGlobalProperty(h.owner, "Старый объект", "archived")
	h.seedGlobalRule(archived, h.owner, "ЖКУ архива", typeExpense, 100000, false, false, "utilities", nil)

	cards, err := svc.ListGlobalPaymentObjects(h.ctx(), h.owner, "")
	if err != nil {
		t.Fatalf("objects: %v", err)
	}
	assertObjectStacks(t, cards, dachaRule)
	assertObjectSearch(t, h)

	// A suspended membership grants no read — the member's page is empty.
	member := uuid.Must(uuid.NewV7())
	h.seedActor(member)
	h.seedMembership(h.propID, member, h.owner, "viewer", "suspended")
	suspended, err := svc.ListGlobalPaymentObjects(h.ctx(), member, "")
	if err != nil {
		t.Fatalf("suspended objects: %v", err)
	}
	if len(suspended) != 0 {
		t.Errorf("suspended member's cards = %+v, want empty", suspended)
	}
}

// The global pin orders the «Объекты» cards (ticket #577): the pinned first
// — among themselves by the pin time — then the rest by name; the pin time
// travels onto the card.
func TestListGlobalPaymentObjects_PinOrder(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	svc := h.globalSvc()

	zebra := h.seedGlobalProperty(h.owner, "Зебра", "active")
	pinnedLate := h.seedGlobalProperty(h.owner, "Аист", "active")
	pinnedEarly := h.seedGlobalProperty(h.owner, "Бекон", "active")

	// Direct seed: the properties context owns the pin writes; this read
	// only consumes the column.
	pinA := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	pinB := time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC)
	if _, err := h.pool.Exec(h.ctx(),
		`UPDATE properties SET pinned_at = $1 WHERE id = $2`, pinB, pinnedLate,
	); err != nil {
		t.Fatalf("seed pin B: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`UPDATE properties SET pinned_at = $1 WHERE id = $2`, pinA, pinnedEarly,
	); err != nil {
		t.Fatalf("seed pin A: %v", err)
	}

	cards, err := svc.ListGlobalPaymentObjects(h.ctx(), h.owner, "")
	if err != nil {
		t.Fatalf("objects: %v", err)
	}
	want := []struct {
		id  uuid.UUID
		pin *time.Time
	}{
		{pinnedEarly, &pinA},
		{pinnedLate, &pinB},
		{zebra, nil},
		{h.propID, nil}, // Then the rest by name: «Зебра» < «Квартира».
	}
	if len(cards) != len(want) {
		got := make([]uuid.UUID, 0, len(cards))
		for _, c := range cards {
			got = append(got, c.PropertyID)
		}
		t.Fatalf("cards = %v, want %d", got, len(want))
	}
	for i, w := range want {
		if cards[i].PropertyID != w.id {
			t.Errorf("cards[%d].PropertyID = %s, want %s", i, cards[i].PropertyID, w.id)
		}
		switch {
		case w.pin == nil && cards[i].PinnedAt != nil:
			t.Errorf("cards[%d].PinnedAt = %v, want nil", i, cards[i].PinnedAt)
		case w.pin != nil && (cards[i].PinnedAt == nil || !cards[i].PinnedAt.Equal(*w.pin)):
			t.Errorf("cards[%d].PinnedAt = %v, want %v", i, cards[i].PinnedAt, w.pin)
		}
	}
}

// assertObjectStacks checks the three visible cards: the Квартира stacks
// (auto-pay group and the rest, each with its dot), the rule-less Дом's
// empty groups and the shared dacha's key.
func assertObjectStacks(t *testing.T, cards []paymentsapp.GlobalPaymentObjectCard, dachaRule uuid.UUID) {
	t.Helper()
	if len(cards) != 3 {
		t.Fatalf("cards = %d, want the three visible ones (archived cut)", len(cards))
	}
	byName := map[string]paymentsapp.GlobalPaymentObjectCard{}
	for _, card := range cards {
		byName[card.Name] = card
	}
	quarter := byName[objectPropertyName]
	if len(quarter.AutoPayKeys) != 2 || len(quarter.OtherKeys) != 2 {
		t.Fatalf("quarter stacks = (%d auto, %d other), want (2, 2)", len(quarter.AutoPayKeys), len(quarter.OtherKeys))
	}
	if !quarter.AutoPayKeys[0].HasOverdue || quarter.AutoPayKeys[1].HasOverdue {
		t.Errorf("auto-pay dots = (%v, %v), want (overdue, clean)", quarter.AutoPayKeys[0].HasOverdue, quarter.AutoPayKeys[1].HasOverdue)
	}
	if !quarter.OtherKeys[0].HasOverdue || quarter.OtherKeys[1].HasOverdue {
		t.Errorf("other dots = (%v, %v), want (overdue, clean)", quarter.OtherKeys[0].HasOverdue, quarter.OtherKeys[1].HasOverdue)
	}
	empty := byName["Дом"]
	if empty.AutoPayKeys == nil || empty.OtherKeys == nil {
		t.Errorf("Дом groups = (%v, %v), want empty non-nil arrays", empty.AutoPayKeys, empty.OtherKeys)
	}
	if len(byName["Чужая дача"].OtherKeys) != 1 || byName["Чужая дача"].OtherKeys[0].PaymentID != dachaRule {
		t.Errorf("Чужая дача other keys = %+v, want the shared rule", byName["Чужая дача"].OtherKeys)
	}
}

// assertObjectSearch checks the object search: the query filters the
// objects by name or address, their stacks stay whole.
func assertObjectSearch(t *testing.T, h *paymentsHarness) {
	t.Helper()
	narrow, err := h.globalSvc().ListGlobalPaymentObjects(h.ctx(), h.owner, "квар")
	if err != nil {
		t.Fatalf("objects search: %v", err)
	}
	if len(narrow) != 1 || narrow[0].Name != objectPropertyName || len(narrow[0].AutoPayKeys) != 2 {
		t.Fatalf("narrowed = %+v, want only the quarter with its whole stacks", narrow)
	}
	byAddress, err := h.globalSvc().ListGlobalPaymentObjects(h.ctx(), h.owner, "Тверская")
	if err != nil {
		t.Fatalf("objects address search: %v", err)
	}
	if len(byAddress) != 3 {
		t.Errorf("address search = %d cards, want every seeded property (the shared harness address)", len(byAddress))
	}
}

// The object card's avatar photo (ticket #582): the object's first (oldest)
// photo travels on the card, an object without photos carries nil. Direct
// seed: the properties context owns the photo writes; this read only
// consumes the join.
func TestListGlobalPaymentObjects_PhotoURL(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	svc := h.globalSvc()

	withPhoto := h.seedGlobalProperty(h.owner, "С фото", "active")
	h.seedGlobalProperty(h.owner, "Без фото", "active")
	seedObjectPhoto := func(propertyID uuid.UUID, url string, at time.Time) {
		t.Helper()
		if _, err := h.pool.Exec(h.ctx(),
			// The app-side v7 default (000079) owns new ids — the seed carries one.
			`INSERT INTO property_photos (id, property_id, url, created_at) VALUES ($1, $2, $3, $4)`,
			uuid.Must(uuid.NewV7()), propertyID, url, at,
		); err != nil {
			t.Fatalf("seed photo %s: %v", url, err)
		}
	}
	seedObjectPhoto(withPhoto, "/uploads/newer.jpg", time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC))
	seedObjectPhoto(withPhoto, "/uploads/older.jpg", time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC))

	cards, err := svc.ListGlobalPaymentObjects(h.ctx(), h.owner, "")
	if err != nil {
		t.Fatalf("objects: %v", err)
	}
	byName := map[string]paymentsapp.GlobalPaymentObjectCard{}
	for _, card := range cards {
		byName[card.Name] = card
	}
	older := byName["С фото"]
	if older.PhotoURL == nil || *older.PhotoURL != "/uploads/older.jpg" {
		t.Errorf("С фото PhotoURL = %v, want the oldest photo", older.PhotoURL)
	}
	if got := byName["Без фото"].PhotoURL; got != nil {
		t.Errorf("Без фото PhotoURL = %v, want nil", got)
	}

	found, err := svc.ListGlobalPaymentObjects(h.ctx(), h.owner, "С фото")
	if err != nil {
		t.Fatalf("objects search: %v", err)
	}
	if len(found) != 1 || found[0].PhotoURL == nil || *found[0].PhotoURL != "/uploads/older.jpg" {
		t.Errorf("searched photo = %+v, want the oldest photo under the query", found)
	}
}

// seedGlobalRuleCreatedAt inserts a payment rule with an explicit created_at
// — the keyset fixtures pin the (created_at, id) reading order.
func (h *paymentsHarness) seedGlobalRuleCreatedAt(
	propertyID, ownerID uuid.UUID, title string, amountKopecks int64, createdAt time.Time,
) uuid.UUID {
	h.t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		h.t.Fatalf("new uuid: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks,
		                       recurrence, since, auto_pay, payment_form, category_slug, created_at)
		 VALUES ($1, $2, $3, 'expense', $4, $5, '{"kind":"monthly","daysOfMonth":[1]}'::jsonb,
		         '2026-06-01', false, 'transfer', 'utilities', $6)`,
		id, ownerID, propertyID, title, amountKopecks, createdAt,
	); err != nil {
		h.t.Fatalf("seed rule %s: %v", title, err)
	}
	return id
}

// The ticket's acceptance (map #596, ticket #597): a 150-row search walks
// three full 50-row pages, and the mutations between the loads — the
// property rename that shifted the old property-name-ordered offset windows,
// a rule created mid-walk — never duplicate or drop a row: the keyset
// resumes on the feed's own (created_at, id) order.
func TestSearchGlobalPaymentsKeyset_MutationsBetweenPages(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	svc := h.globalSvc()

	const total = 150
	const pageSize = 50
	wantIDs := make([]uuid.UUID, 0, total)
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	for i := range total {
		wantIDs = append(wantIDs, h.seedGlobalRuleCreatedAt(h.propID, h.owner,
			fmt.Sprintf("Правило %03d", i), 100000+int64(i), base.Add(time.Duration(i)*time.Minute)))
	}

	page := func(cursor string) paymentsapp.GlobalPaymentSearch {
		h.t.Helper()
		search, err := svc.SearchGlobalPayments(h.ctx(), h.owner, "",
			paymentsapp.GlobalPaymentSearchPage{Limit: pageSize, Cursor: cursor})
		if err != nil {
			h.t.Fatalf("search page (cursor %q): %v", cursor, err)
		}
		return search
	}

	first := page("")
	// The rename between the loads — the offset window's row-shifter — moves
	// not a single row of the (created_at, id) order.
	if _, err := h.pool.Exec(h.ctx(),
		`UPDATE properties SET name = 'Переименовано' WHERE id = $1`, h.propID); err != nil {
		h.t.Fatalf("rename property: %v", err)
	}
	second := page(first.NextCursor)
	// A rule created between the loads carries the newest created_at — it
	// lands past the walk, never inside the unwalked window.
	h.seedGlobalRule(h.propID, h.owner, "Между порциями", "income", 500000, false, false, "rent", nil)
	third := page(second.NextCursor)

	got := make([]uuid.UUID, 0, total)
	seen := make(map[uuid.UUID]bool, total)
	for _, search := range []paymentsapp.GlobalPaymentSearch{first, second, third} {
		if len(search.Items) != pageSize {
			t.Errorf("page rows = %d, want a full %d-row page", len(search.Items), pageSize)
		}
		for _, item := range search.Items {
			if seen[item.ID] {
				t.Errorf("rule %s (row %q) arrived twice", item.ID, item.Title)
			}
			seen[item.ID] = true
			got = append(got, item.ID)
		}
	}
	if len(got) != total {
		t.Fatalf("three pages carry %d rows, want %d — rows dropped or duplicated", len(got), total)
	}
	for i := range wantIDs {
		if got[i] != wantIDs[i] {
			t.Fatalf("row %d = %s, want %s — the walk broke the created_at order", i, got[i], wantIDs[i])
		}
	}
	// The third page came back full, so it still answers with a
	// continuation. The page it produces holds exactly the rule created
	// between the loads — its newest created_at sorts right past the walk —
	// and then the matches are exhausted.
	fourth := page(third.NextCursor)
	if len(fourth.Items) != 1 || fourth.Items[0].Title != "Между порциями" {
		t.Errorf("fourth page = %v, want the single mid-walk rule", fourth.Items)
	}
	if fourth.NextCursor != "" {
		t.Errorf("fourth page nextCursor = %q, want '' — the matches are exhausted", fourth.NextCursor)
	}
}

// Equal created_at values do not break the walk either: the id tiebreak
// keeps the keyset partitioning the ties across pages without repetition
// (the ties' relative order is the id's, not the test's).
func TestSearchGlobalPaymentsKeyset_CreatedAtTies(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	svc := h.globalSvc()

	const total = 20
	const pageSize = 6
	sameMoment := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	want := make(map[uuid.UUID]bool, total)
	for i := range total {
		id := h.seedGlobalRuleCreatedAt(h.propID, h.owner,
			fmt.Sprintf("Двойник %02d", i), 200000+int64(i), sameMoment)
		want[id] = true
	}

	cursor := ""
	seen := make(map[uuid.UUID]bool, total)
	pages := 0
	for {
		search, err := svc.SearchGlobalPayments(h.ctx(), h.owner, "",
			paymentsapp.GlobalPaymentSearchPage{Limit: pageSize, Cursor: cursor})
		if err != nil {
			t.Fatalf("search page %d: %v", pages+1, err)
		}
		pages++
		if pages > total/pageSize+2 {
			t.Fatal("the walk never ended — the cursor stopped advancing")
		}
		for _, item := range search.Items {
			if seen[item.ID] {
				t.Errorf("rule %s arrived twice", item.ID)
			}
			seen[item.ID] = true
			if !want[item.ID] {
				t.Errorf("unexpected rule %s in the walk", item.ID)
			}
		}
		if search.NextCursor == "" {
			break
		}
		cursor = search.NextCursor
	}
	if len(seen) != total {
		t.Errorf("the walk covered %d rules, want all %d", len(seen), total)
	}
}

// Deletion between the loads (ticket #597's mutation list) — including the
// harshest case: the cursor's own anchor row deleted before the next page
// is fetched. The cursor is a coordinate, not a live reference: the walk
// resumes at the same (created_at, id) boundary and covers every surviving
// row exactly once.
func TestSearchGlobalPaymentsKeyset_DeletedRowsBetweenPages(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	svc := h.globalSvc()

	const total = 60
	const pageSize = 20
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	titles := make([]string, 0, total)
	for i := range total {
		title := fmt.Sprintf("Правило %03d", i)
		titles = append(titles, title)
		h.seedGlobalRuleCreatedAt(h.propID, h.owner, title, 300000+int64(i), base.Add(time.Duration(i)*time.Minute))
	}

	first, err := svc.SearchGlobalPayments(h.ctx(), h.owner, "",
		paymentsapp.GlobalPaymentSearchPage{Limit: pageSize})
	if err != nil {
		t.Fatalf("page one: %v", err)
	}
	if len(first.Items) != pageSize {
		t.Fatalf("page one rows = %d, want %d", len(first.Items), pageSize)
	}
	// Delete an unwalked row and the cursor's anchor row itself.
	for _, victim := range []string{"Правило 030", titles[pageSize-1]} {
		if _, err := h.pool.Exec(h.ctx(),
			`DELETE FROM payments WHERE title = $1 AND owner_id = $2`, victim, h.owner); err != nil {
			t.Fatalf("delete %s: %v", victim, err)
		}
	}

	seen := make(map[string]bool, total-1)
	seenTitles := func(search paymentsapp.GlobalPaymentSearch) {
		t.Helper()
		for _, item := range search.Items {
			if seen[item.Title] {
				t.Errorf("row %q arrived twice", item.Title)
			}
			seen[item.Title] = true
		}
	}
	seenTitles(first)
	cursor := first.NextCursor
	pages := 1
	for cursor != "" {
		next, err := svc.SearchGlobalPayments(h.ctx(), h.owner, "",
			paymentsapp.GlobalPaymentSearchPage{Limit: pageSize, Cursor: cursor})
		if err != nil {
			t.Fatalf("walk page %d: %v", pages+1, err)
		}
		pages++
		if pages > total/pageSize+2 {
			t.Fatal("the walk never ended — the cursor stopped advancing")
		}
		seenTitles(next)
		cursor = next.NextCursor
	}
	// The anchor row was delivered on page one before the deletion — that
	// delivery stands; the unwalked deleted row must never appear.
	if len(seen) != total-1 {
		t.Errorf("the walk covered %d rules, want %d — the unwalked deleted row gone, none dropped or duplicated", len(seen), total-1)
	}
	if seen["Правило 030"] {
		t.Error("unwalked deleted row 'Правило 030' came back")
	}
}
