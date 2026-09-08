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
) {
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
	h.seedRuleOperation(favorite, h.propID, h.owner, "2026-08-10", opCancelled, "expense", "Страхование", 3200000)
	h.seedRuleOperation(favorite, h.propID, h.owner, "2026-08-15", opPlanned, "expense", "Страхование", 3200000)
	h.seedRuleOperation(favorite, h.propID, h.owner, "2026-08-20", opPlanned, "expense", "Страхование", 3200000)
	h.seedRuleOperation(favorite, h.propID, h.owner, "2026-08-30", opPlanned, "expense", "Страхование", 3200000)

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
	assertFavoriteRow(t, byTitle["Страхование"])
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
func assertFavoriteRow(t *testing.T, fav paymentsapp.GlobalPaymentItem) {
	t.Helper()
	if fav.PropertyName != objectPropertyName || !fav.IsFavorite || fav.AutoPay {
		t.Errorf("favorite = (%q, %v, %v), want the quarter, favorite, non-auto", fav.PropertyName, fav.IsFavorite, fav.AutoPay)
	}
	if fav.OverdueCount != 2 || fav.OverdueDays == nil || *fav.OverdueDays != 10 {
		t.Errorf("favorite overdue = (%d, %v), want (2, 10) — the tombstone never counts", fav.OverdueCount, fav.OverdueDays)
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
	h.seedGlobalRule(h.propID, h.owner, "Зеркала", "income", 150000,
		false, false, "", &category)

	cases := []struct {
		name      string
		query     string
		wantItems []string
		wantChips map[string]int64 // «slug or label» → count; the label keys the custom chip.
	}{
		{"the title matches", "страх", []string{"Страхование квартиры"}, map[string]int64{"insurance": 1}},
		{"the default catalog label matches", "коммунал", []string{"Интернет"}, map[string]int64{"utilities": 1}},
		{"the user category name matches", "кофейн", []string{"Зеркала"}, map[string]int64{"Кофейни": 1}},
		{"the chips carry the matched rules' categories", "терне", []string{"Интернет"}, map[string]int64{"utilities": 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			search, err := svc.SearchGlobalPayments(h.ctx(), h.owner, tc.query)
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			assertSearchTitles(t, search, tc.wantItems)
			assertSearchChips(t, search, tc.wantChips)
		})
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

	autoOverdue := h.seedGlobalRule(h.propID, h.owner, "ЖКУ", "expense", 500000, true, false, "utilities", nil)
	h.seedRuleOperation(autoOverdue, h.propID, h.owner, "2026-08-01", opPlanned, "expense", "ЖКУ", 500000)
	h.seedGlobalRule(h.propID, h.owner, "Интернет", "expense", 700000, true, false, "utilities", nil)
	plainOverdue := h.seedGlobalRule(h.propID, h.owner, "Страхование", "expense", 3200000, false, false, "insurance", nil)
	h.seedRuleOperation(plainOverdue, h.propID, h.owner, "2026-08-10", opPlanned, "expense", "Страхование", 3200000)
	h.seedGlobalRule(h.propID, h.owner, "Клининг", "expense", 90000, false, false, "cleaning", nil)

	h.seedGlobalProperty(h.owner, "Дом", "active") // No rules: empty groups.

	partner := h.seedGlobalUser("Europe/Moscow")
	dacha := h.seedGlobalProperty(partner, "Чужая дача", "active")
	h.seedMembership(dacha, h.owner, partner, "viewer", "active")
	dachaRule := h.seedGlobalRule(dacha, partner, "Аренда", "income", 6000000, false, false, "rent", nil)
	h.seedRuleOperation(dachaRule, dacha, partner, "2026-08-01", opPlanned, "income", "Аренда", 6000000)

	archived := h.seedGlobalProperty(h.owner, "Старый объект", "archived")
	h.seedGlobalRule(archived, h.owner, "ЖКУ архива", "expense", 100000, false, false, "utilities", nil)

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
