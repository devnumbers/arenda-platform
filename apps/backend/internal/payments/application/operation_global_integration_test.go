//go:build integration

package application_test

// The global feed integration tests (ticket #540): the actor-scoped
// cross-property read over paid facts — own book plus active-membership
// properties (ADR 0028), the archived ones excluded, the propertyIds
// multi-select behind the view gate — and the global summary twin, against
// real PostgreSQL through the shared harness.

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
)

// globalAmountQuery is the «2 500» search probe: the display amount whose
// digits match the fixtures' single 250 000-kopecks row.
const globalAmountQuery = "2 500"

// seedGlobalProperty inserts a property for any owner with the given status
// — the merged-feed fixtures need several books side by side.
func (h *paymentsHarness) seedGlobalProperty(ownerID uuid.UUID, name, status string) uuid.UUID {
	h.t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		h.t.Fatalf("new uuid: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO properties (id, owner_id, name, type, address, status)
		 VALUES ($1, $2, $3, 'apartment', 'Москва, Тверская 1', $4)`,
		id, ownerID, name, status,
	); err != nil {
		h.t.Fatalf("seed property %s: %v", name, err)
	}
	return id
}

// seedGlobalOperation inserts one operation row on any property with any
// owner — the row-level twin of seedOperationRow for multi-book fixtures.
func (h *paymentsHarness) seedGlobalOperation(
	propertyID, ownerID uuid.UUID, date, status, typ, title string, amountKopecks int64, slug, label string,
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
		`INSERT INTO operations (id, owner_id, property_id, origin, date, paid_date,
		                       status, type, title, amount_kopecks, payment_form, category_label, category_slug)
		 VALUES ($1, $2, $3, 'payment', $4, $5::date, $6, $7, $8, $9, 'transfer', $10, $11)`,
		id, ownerID, propertyID, date, paid, status, typ, title, amountKopecks, label, slug,
	); err != nil {
		h.t.Fatalf("seed operation %s (%s): %v", title, date, err)
	}
	return id
}

// seedMembership inserts a property_members row; grantedBy is the property's
// owner (the audit FK needs a user row).
func (h *paymentsHarness) seedMembership(propertyID, userID, grantedBy uuid.UUID, role, status string) {
	h.t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		h.t.Fatalf("new uuid: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO property_members (id, property_id, user_id, role, granted_by, status)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		id, propertyID, userID, role, grantedBy, status,
	); err != nil {
		h.t.Fatalf("seed membership: %v", err)
	}
}

// globalFeedFixture seeds one August bookkeeping spread over four visible
// cuts — two own properties, one shared-active (another owner's), one
// archived own — plus a foreign property with no membership, and returns the
// property ids: ownA (the harness's), ownB, shared, archived, foreign.
func (h *paymentsHarness) globalFeedFixture() (ownB, shared, archived, foreign uuid.UUID) {
	h.t.Helper()

	ownB = h.seedGlobalProperty(h.owner, "Дом", "active")
	archived = h.seedGlobalProperty(h.owner, "Старый объект", "archived")

	stranger := uuid.Must(uuid.NewV7())
	h.seedActor(stranger)
	foreign = h.seedGlobalProperty(stranger, "Без доступа", "active")
	h.seedGlobalOperation(foreign, stranger, "2026-08-09", opPaid, "expense", "Чужой расход", 555000, searchSlugSecurity, "Охрана")

	sharedOwner := uuid.Must(uuid.NewV7())
	h.seedActor(sharedOwner)
	shared = h.seedGlobalProperty(sharedOwner, "Чужая дача", "active")
	h.seedMembership(shared, h.owner, sharedOwner, "viewer", "active")

	h.seedGlobalOperation(h.propID, h.owner, "2026-08-10", opPaid, "income", "Аренда", 5650000,
		testIntegrationSlugRent, "Арендная плата")
	h.seedGlobalOperation(h.propID, h.owner, "2026-08-12", opPaid, "expense", "ЖКУ", 1050000,
		testIntegrationSlugUtilities, "Коммунальные услуги")
	h.seedGlobalOperation(h.propID, h.owner, "2026-08-20", opPlanned, "expense", "ЖКУ", 300000,
		testIntegrationSlugUtilities, "Коммунальные услуги")
	h.seedGlobalOperation(h.propID, h.owner, "2026-08-14", opCancelled, "expense", "Клининг", 900000,
		"cleaning", "Клининг")
	h.seedGlobalOperation(ownB, h.owner, "2026-08-18", opPaid, "expense", "Охрана", 250000,
		searchSlugSecurity, "Охрана")
	h.seedGlobalOperation(archived, h.owner, "2026-08-15", opPaid, "expense", "ЖКУ", 777000,
		testIntegrationSlugUtilities, "Коммунальные услуги")
	h.seedGlobalOperation(shared, sharedOwner, "2026-08-11", opPaid, "expense", "Аренда", 4200000,
		testIntegrationSlugRent, "Арендная плата")
	return ownB, shared, archived, foreign
}

// feedTitles projects a listed page onto titles in feed order — the
// ordering assertions read best over them.
func feedTitles(items []paymentsapp.OperationListItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Operation.Title)
	}
	return out
}

func titlesEqual(got []string, want ...string) bool {
	return slices.Equal(got, want)
}

func TestListGlobalOperations_MergedFeed(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	ownB, shared, _, _ := h.globalFeedFixture()

	items, err := h.ops.ListGlobalOperations(h.ctx(), h.owner, paymentsapp.GlobalOperationsListQuery{})
	if err != nil {
		t.Fatalf("list global: %v", err)
	}
	if !titlesEqual(feedTitles(items), "Охрана", "ЖКУ", "Аренда", "Аренда") {
		t.Fatalf("feed = %v, want the four paid rows newest-first (planned, cancelled, archived, foreign cut)", feedTitles(items))
	}
	nameOf := map[uuid.UUID]string{h.propID: "Квартира", ownB: "Дом", shared: "Чужая дача"}
	for _, item := range items {
		if item.ViewStatus != domain.ViewStatusPaid {
			t.Errorf("row %q status = %s, want paid — the feed is paid-only", item.Operation.Title, item.ViewStatus)
		}
		if item.PropertyName != nameOf[item.Operation.PropertyID] {
			t.Errorf("row %q propertyName = %q, want %q", item.Operation.Title, item.PropertyName, nameOf[item.Operation.PropertyID])
		}
	}
}

func TestListGlobalOperations_SuspendedMemberSeesNothing(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	member := uuid.Must(uuid.NewV7())
	h.seedActor(member)
	h.seedMembership(h.propID, member, h.owner, "viewer", "suspended")
	h.seedGlobalOperation(h.propID, h.owner, "2026-08-10", opPaid, "expense", "ЖКУ", 1050000,
		testIntegrationSlugUtilities, "Коммунальные услуги")

	items, err := h.ops.ListGlobalOperations(h.ctx(), member, paymentsapp.GlobalOperationsListQuery{})
	if err != nil {
		t.Fatalf("list global: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("suspended member's feed = %+v, want empty — a suspended membership grants no read", items)
	}
}

func TestListGlobalOperations_PropertyIdsFilter(t *testing.T) {
	t.Parallel()
	// The propertyIds entries resolve through the production membership
	// policy — the shared entry reads through the member's view gate, the
	// foreign one gets the privacy 404, exactly as the wire runs.
	h := newPaymentsHarnessWithRealPolicy(t).withOwner("Europe/Moscow")
	ownB, shared, archived, foreign := h.globalFeedFixture()

	one, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{PropertyIDs: []uuid.UUID{ownB}})
	if err != nil {
		t.Fatalf("filter by ownB: %v", err)
	}
	if !titlesEqual(feedTitles(one), "Охрана") {
		t.Fatalf("propertyIds=[ownB] = %v, want the single Дом row", feedTitles(one))
	}

	both, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{PropertyIDs: []uuid.UUID{h.propID, shared}})
	if err != nil {
		t.Fatalf("filter by ownA+shared: %v", err)
	}
	if !titlesEqual(feedTitles(both), "ЖКУ", "Аренда", "Аренда") {
		t.Fatalf("propertyIds=[ownA,shared] = %v, want the two properties' paid rows", feedTitles(both))
	}

	// Privacy: a foreign or unknown id is the privacy 404 — the filter never
	// widens the feed, and no row leaks that the property exists.
	if _, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{PropertyIDs: []uuid.UUID{foreign}}); !errors.Is(err, paymentsapp.ErrNotFound) {
		t.Errorf("foreign propertyIds err = %v, want the privacy ErrNotFound", err)
	}
	if _, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{PropertyIDs: []uuid.UUID{uuid.Must(uuid.NewV7())}}); !errors.Is(err, paymentsapp.ErrNotFound) {
		t.Errorf("unknown propertyIds err = %v, want the privacy ErrNotFound", err)
	}

	// An archived own property stays visible — the feed's archive cut simply
	// yields an empty page, not an error.
	empty, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{PropertyIDs: []uuid.UUID{archived}})
	if err != nil {
		t.Fatalf("filter by archived: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("propertyIds=[archived] = %+v, want empty — the archive cut holds under an explicit filter", empty)
	}

	// The cut holds in a mixed selection too.
	mixed, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{PropertyIDs: []uuid.UUID{archived, ownB}})
	if err != nil {
		t.Fatalf("filter by archived+ownB: %v", err)
	}
	if !titlesEqual(feedTitles(mixed), "Охрана") {
		t.Fatalf("propertyIds=[archived,ownB] = %v, want only the Дом row", feedTitles(mixed))
	}
}

func TestListGlobalOperations_FiltersOrderPagination(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	h.globalFeedFixture()

	from := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	window, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{DateFrom: &from, DateTo: &to})
	if err != nil {
		t.Fatalf("period filter: %v", err)
	}
	if !titlesEqual(feedTitles(window), "Охрана", "ЖКУ") {
		t.Fatalf("period [12–18] = %v, want the two in-window rows", feedTitles(window))
	}

	// The amount search reads the display amount's digits — «2 500» finds
	// 2 500,00 ₽ (ticket #476's predicate).
	amount, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{Search: globalAmountQuery})
	if err != nil {
		t.Fatalf("amount search: %v", err)
	}
	if !titlesEqual(feedTitles(amount), "Охрана") {
		t.Fatalf("search «2 500» = %v, want the single Охрана row", feedTitles(amount))
	}

	income := domain.TypeIncome
	incomes, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{Type: &income})
	if err != nil {
		t.Fatalf("type filter: %v", err)
	}
	if !titlesEqual(feedTitles(incomes), "Аренда") || incomes[0].Operation.AmountKopecks != 5650000 {
		t.Fatalf("type=income = %+v, want the own book's rent row only", feedTitles(incomes))
	}

	utilities, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{Categories: []string{testIntegrationSlugUtilities}})
	if err != nil {
		t.Fatalf("categories filter: %v", err)
	}
	if !titlesEqual(feedTitles(utilities), "ЖКУ") {
		t.Fatalf("categories=[utilities] = %v, want the own ЖКУ row — the archived one stays cut", feedTitles(utilities))
	}

	asc, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{Asc: true})
	if err != nil {
		t.Fatalf("asc: %v", err)
	}
	if !titlesEqual(feedTitles(asc), "Аренда", "Аренда", "ЖКУ", "Охрана") {
		t.Fatalf("asc feed = %v, want oldest-first", feedTitles(asc))
	}

	page, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{Limit: 2, Offset: 1})
	if err != nil {
		t.Fatalf("pagination: %v", err)
	}
	if !titlesEqual(feedTitles(page), "ЖКУ", "Аренда") {
		t.Fatalf("page [limit 2, offset 1] = %v, want the middle of the desc feed", feedTitles(page))
	}
}

func TestSummarizeGlobalOperations_TotalsAndCategories(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	h.globalFeedFixture()

	summary, err := h.ops.SummarizeGlobalOperations(h.ctx(), h.owner, paymentsapp.GlobalOperationsSummaryQuery{})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if summary.IncomeTotalKopecks != 5650000 {
		t.Errorf("income total = %d, want 5 650 000", summary.IncomeTotalKopecks)
	}
	if summary.ExpenseTotalKopecks != 5500000 {
		t.Errorf("expense total = %d, want 5 500 000 (own + shared, planned/cancelled/archived never count)", summary.ExpenseTotalKopecks)
	}
	if len(summary.Categories) != 4 {
		t.Fatalf("categories = %d rows (%+v), want 4 — rent splits by direction", len(summary.Categories), summary.Categories)
	}
	wantOrder := []struct {
		slug  string
		typ   domain.PaymentType
		total int64
	}{
		{testIntegrationSlugRent, domain.TypeIncome, 5650000},
		{testIntegrationSlugRent, domain.TypeExpense, 4200000},
		{testIntegrationSlugUtilities, domain.TypeExpense, 1050000},
		{searchSlugSecurity, domain.TypeExpense, 250000},
	}
	for i, want := range wantOrder {
		got := summary.Categories[i]
		if got.Slug != want.slug || got.Type != want.typ || got.TotalKopecks != want.total {
			t.Errorf("categories[%d] = %s/%s/%d, want %s/%s/%d",
				i, got.Slug, got.Type, got.TotalKopecks, want.slug, want.typ, want.total)
		}
	}
}

func TestSummarizeGlobalOperations_CategoryAndTypeNarrowBreakdownOnly(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	h.globalFeedFixture()

	rent, err := h.ops.SummarizeGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsSummaryQuery{Categories: []string{testIntegrationSlugRent}})
	if err != nil {
		t.Fatalf("categories summary: %v", err)
	}
	if rent.IncomeTotalKopecks != 5650000 || rent.ExpenseTotalKopecks != 5500000 {
		t.Errorf("totals = %d/%d, want 5 650 000/5 500 000 — the category filter narrows only the breakdown",
			rent.IncomeTotalKopecks, rent.ExpenseTotalKopecks)
	}
	if len(rent.Categories) != 2 {
		t.Fatalf("categories = %+v, want the two rent rows", rent.Categories)
	}

	expense := domain.TypeExpense
	expenses, err := h.ops.SummarizeGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsSummaryQuery{Type: &expense})
	if err != nil {
		t.Fatalf("type summary: %v", err)
	}
	if expenses.IncomeTotalKopecks != 5650000 || expenses.ExpenseTotalKopecks != 5500000 {
		t.Errorf("totals = %d/%d, want 5 650 000/5 500 000 — the type filter narrows only the breakdown",
			expenses.IncomeTotalKopecks, expenses.ExpenseTotalKopecks)
	}
	for _, row := range expenses.Categories {
		if row.Type != domain.TypeExpense {
			t.Errorf("category %s leaked an income row into the breakdown", row.Slug)
		}
	}
}

func TestSummarizeGlobalOperations_PropertyIdsAndPeriodNarrowEverything(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarnessWithRealPolicy(t).withOwner("Europe/Moscow")
	ownB, _, _, foreign := h.globalFeedFixture()

	one, err := h.ops.SummarizeGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsSummaryQuery{PropertyIDs: []uuid.UUID{ownB}})
	if err != nil {
		t.Fatalf("propertyIds summary: %v", err)
	}
	if one.IncomeTotalKopecks != 0 || one.ExpenseTotalKopecks != 250000 {
		t.Errorf("totals = %d/%d, want 0/250 000 — the property filter narrows the totals too",
			one.IncomeTotalKopecks, one.ExpenseTotalKopecks)
	}

	from := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	to := from
	period, err := h.ops.SummarizeGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsSummaryQuery{DateFrom: &from, DateTo: &to})
	if err != nil {
		t.Fatalf("period summary: %v", err)
	}
	if period.IncomeTotalKopecks != 0 || period.ExpenseTotalKopecks != 1050000 {
		t.Errorf("totals = %d/%d, want 0/1 050 000 — the period narrows the totals too",
			period.IncomeTotalKopecks, period.ExpenseTotalKopecks)
	}

	amount, err := h.ops.SummarizeGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsSummaryQuery{Search: globalAmountQuery})
	if err != nil {
		t.Fatalf("search summary: %v", err)
	}
	if amount.IncomeTotalKopecks != 0 || amount.ExpenseTotalKopecks != 250000 || len(amount.Categories) != 1 {
		t.Errorf("searched summary = %d/%d (%+v), want 0/250 000 with the single security chip",
			amount.IncomeTotalKopecks, amount.ExpenseTotalKopecks, amount.Categories)
	}

	// The filter's privacy travels to the summary as well.
	if _, err := h.ops.SummarizeGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsSummaryQuery{PropertyIDs: []uuid.UUID{foreign}}); !errors.Is(err, paymentsapp.ErrNotFound) {
		t.Errorf("foreign propertyIds err = %v, want the privacy ErrNotFound", err)
	}
}

func TestSummarizeGlobalOperations_EmptyBookIsZeroSummary(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	stranger := uuid.Must(uuid.NewV7())
	h.seedActor(stranger)

	summary, err := h.ops.SummarizeGlobalOperations(h.ctx(), stranger, paymentsapp.GlobalOperationsSummaryQuery{})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if summary.IncomeTotalKopecks != 0 || summary.ExpenseTotalKopecks != 0 || len(summary.Categories) != 0 {
		t.Fatalf("summary = %+v, want the zero summary — the feed is actor-scoped, a stranger's book is empty",
			summary)
	}
}

func TestListGlobalOperations_ViewerReadsSharedFeed(t *testing.T) {
	t.Parallel()
	// The real membership policy: the owner's viewer membership on the
	// shared property is what lets its rows through — the same authorization
	// the wire runs.
	h := newPaymentsHarnessWithRealPolicy(t).withOwner("Europe/Moscow")
	h.globalFeedFixture()

	items, err := h.ops.ListGlobalOperations(h.ctx(), h.owner, paymentsapp.GlobalOperationsListQuery{})
	if err != nil {
		t.Fatalf("viewer list: %v", err)
	}
	if len(items) != 4 {
		t.Fatalf("viewer feed = %+v, want the same four paid rows — a viewer reads the merged feed", items)
	}
}
