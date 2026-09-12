//go:build integration

package application_test

// The global feed integration tests (ticket #540): the actor-scoped
// cross-property read over paid facts — own book plus active-membership
// properties (ADR 0028), the archived ones excluded, the propertyIds
// multi-select behind the view gate — and the global summary twin, against
// real PostgreSQL through the shared harness.

import (
	"errors"
	"fmt"
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
	if !titlesEqual(feedTitles(items.Items), "Охрана", "ЖКУ", "Аренда", "Аренда") {
		t.Fatalf("feed = %v, want the four paid rows newest-first (planned, cancelled, archived, foreign cut)", feedTitles(items.Items))
	}
	nameOf := map[uuid.UUID]string{h.propID: "Квартира", ownB: "Дом", shared: "Чужая дача"}
	for _, item := range items.Items {
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
	if len(items.Items) != 0 {
		t.Fatalf("suspended member's feed = %+v, want empty — a suspended membership grants no read", items.Items)
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
	if !titlesEqual(feedTitles(one.Items), "Охрана") {
		t.Fatalf("propertyIds=[ownB] = %v, want the single Дом row", feedTitles(one.Items))
	}

	both, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{PropertyIDs: []uuid.UUID{h.propID, shared}})
	if err != nil {
		t.Fatalf("filter by ownA+shared: %v", err)
	}
	if !titlesEqual(feedTitles(both.Items), "ЖКУ", "Аренда", "Аренда") {
		t.Fatalf("propertyIds=[ownA,shared] = %v, want the two properties' paid rows", feedTitles(both.Items))
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
	if len(empty.Items) != 0 {
		t.Fatalf("propertyIds=[archived] = %+v, want empty — the archive cut holds under an explicit filter", empty.Items)
	}

	// The cut holds in a mixed selection too.
	mixed, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{PropertyIDs: []uuid.UUID{archived, ownB}})
	if err != nil {
		t.Fatalf("filter by archived+ownB: %v", err)
	}
	if !titlesEqual(feedTitles(mixed.Items), "Охрана") {
		t.Fatalf("propertyIds=[archived,ownB] = %v, want only the Дом row", feedTitles(mixed.Items))
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
	if !titlesEqual(feedTitles(window.Items), "Охрана", "ЖКУ") {
		t.Fatalf("period [12–18] = %v, want the two in-window rows", feedTitles(window.Items))
	}

	// The amount search reads the display amount's digits — «2 500» finds
	// 2 500,00 ₽ (ticket #476's predicate).
	amount, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{Search: globalAmountQuery})
	if err != nil {
		t.Fatalf("amount search: %v", err)
	}
	if !titlesEqual(feedTitles(amount.Items), "Охрана") {
		t.Fatalf("search «2 500» = %v, want the single Охрана row", feedTitles(amount.Items))
	}

	income := domain.TypeIncome
	incomes, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{Type: &income})
	if err != nil {
		t.Fatalf("type filter: %v", err)
	}
	if !titlesEqual(feedTitles(incomes.Items), "Аренда") || incomes.Items[0].Operation.AmountKopecks != 5650000 {
		t.Fatalf("type=income = %+v, want the own book's rent row only", feedTitles(incomes.Items))
	}

	utilities, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{Categories: []string{testIntegrationSlugUtilities}})
	if err != nil {
		t.Fatalf("categories filter: %v", err)
	}
	if !titlesEqual(feedTitles(utilities.Items), "ЖКУ") {
		t.Fatalf("categories=[utilities] = %v, want the own ЖКУ row — the archived one stays cut", feedTitles(utilities.Items))
	}

	asc, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{Asc: true})
	if err != nil {
		t.Fatalf("asc: %v", err)
	}
	if !titlesEqual(feedTitles(asc.Items), "Аренда", "Аренда", "ЖКУ", "Охрана") {
		t.Fatalf("asc feed = %v, want oldest-first", feedTitles(asc.Items))
	}
}

// The window walks the feed's own order by keyset (ticket #597): the cursor
// resumes strictly after the previous page's last row, and a full page
// still answers with a continuation — the walk stops on the short page it
// produces.
func TestListGlobalOperations_KeysetWindow(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	h.globalFeedFixture()

	page, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{Limit: 2})
	if err != nil {
		t.Fatalf("page one: %v", err)
	}
	if !titlesEqual(feedTitles(page.Items), "Охрана", "ЖКУ") {
		t.Fatalf("page one [limit 2] = %v, want the desc feed's head", feedTitles(page.Items))
	}
	if page.NextCursor == "" {
		t.Fatal("page one nextCursor = '', want the continuation of the four-row feed")
	}

	page, err = h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{Limit: 2, Cursor: page.NextCursor})
	if err != nil {
		t.Fatalf("page two: %v", err)
	}
	if !titlesEqual(feedTitles(page.Items), "Аренда", "Аренда") {
		t.Fatalf("page two [cursor] = %v, want the desc feed's tail", feedTitles(page.Items))
	}

	page, err = h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{Limit: 2, Cursor: page.NextCursor})
	if err != nil {
		t.Fatalf("page three: %v", err)
	}
	if len(page.Items) != 0 || page.NextCursor != "" {
		t.Fatalf("page three = %v/%q, want the empty exhausted page", feedTitles(page.Items), page.NextCursor)
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
	if len(items.Items) != 4 {
		t.Fatalf("viewer feed = %+v, want the same four paid rows — a viewer reads the merged feed", items.Items)
	}
}

func TestListGlobalOperations_IncludeArchived(t *testing.T) {
	t.Parallel()
	// The archive opt-in (ticket #549): the cut is the default, the flag
	// lifts it — the archived property's paid rows rejoin the merged feed
	// under the same visibility predicate, property name included.
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	h.globalFeedFixture()

	without, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{})
	if err != nil {
		t.Fatalf("list global: %v", err)
	}
	if !titlesEqual(feedTitles(without.Items), "Охрана", "ЖКУ", "Аренда", "Аренда") {
		t.Fatalf("feed without the flag = %v, want the archive cut to hold", feedTitles(without.Items))
	}

	with, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{IncludeArchived: true})
	if err != nil {
		t.Fatalf("list global includeArchived: %v", err)
	}
	if !titlesEqual(feedTitles(with.Items), "Охрана", "ЖКУ", "ЖКУ", "Аренда", "Аренда") {
		t.Fatalf("feed with the flag = %v, want the archived row back in date order", feedTitles(with.Items))
	}
	for _, item := range with.Items {
		if item.Operation.Title == "ЖКУ" && item.PropertyName == "Старый объект" && item.ViewStatus != domain.ViewStatusPaid {
			t.Errorf("archived row status = %s, want paid", item.ViewStatus)
		}
	}

	// The (а) semantics: an explicit selection keeps its active rows and
	// the flag adds every archived property on top — the flat book's ЖКУ
	// and Аренда plus the archived ЖКУ, the cut lifting the propertyIds
	// narrow (ownA = the harness's property).
	mixed, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{
			PropertyIDs:     []uuid.UUID{h.propID},
			IncludeArchived: true,
		})
	if err != nil {
		t.Fatalf("list global ownA+includeArchived: %v", err)
	}
	if !titlesEqual(feedTitles(mixed.Items), "ЖКУ", "ЖКУ", "Аренда") {
		t.Fatalf("propertyIds=[ownA]+flag = %v, want ownA's rows plus the archived one", feedTitles(mixed.Items))
	}
}

func TestSummarizeGlobalOperations_IncludeArchived(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	h.globalFeedFixture()

	summary, err := h.ops.SummarizeGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsSummaryQuery{IncludeArchived: true})
	if err != nil {
		t.Fatalf("summarize includeArchived: %v", err)
	}
	if summary.ExpenseTotalKopecks != 6277000 {
		t.Errorf("expense total = %d, want 6 277 000 (5 500 000 + the archived 777 000)", summary.ExpenseTotalKopecks)
	}
	if summary.IncomeTotalKopecks != 5650000 {
		t.Errorf("income total = %d, want 5 650 000 — the archive holds no income rows", summary.IncomeTotalKopecks)
	}
	for _, got := range summary.Categories {
		if got.Slug == testIntegrationSlugUtilities && got.Type == domain.TypeExpense && got.TotalKopecks != 1827000 {
			t.Errorf("utilities expense = %d, want 1 827 000 (1 050 000 + 777 000)", got.TotalKopecks)
		}
	}
}

// The ticket's acceptance on the operations feed (map #596, ticket #597):
// 150 paid rows walk three full 50-row pages, and the mutations between the
// loads — the property rename, a new paid fact — never duplicate or drop a
// row, both in the descending default and the ascending order.
func TestListGlobalOperationsKeyset_MutationsBetweenPages(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	const total = 150
	const pageSize = 50
	// Distinct dates pin the (date, id) reading order: oldest first.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	dateOf := func(i int) string { return base.Add(time.Duration(i) * 24 * time.Hour).Format("2006-01-02") }
	for i := range total {
		h.seedGlobalOperation(h.propID, h.owner, dateOf(i), opPaid, "expense",
			fmt.Sprintf("Операция %03d", i), 100000+int64(i),
			testIntegrationSlugUtilities, "Коммунальные услуги")
	}

	page := func(cursor string, asc bool) paymentsapp.GlobalOperationsPage {
		h.t.Helper()
		result, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
			paymentsapp.GlobalOperationsListQuery{Limit: pageSize, Cursor: cursor, Asc: asc})
		if err != nil {
			h.t.Fatalf("operations page (cursor %q): %v", cursor, err)
		}
		return result
	}
	collect := func(pages []paymentsapp.GlobalOperationsPage) []string {
		h.t.Helper()
		titles := make([]string, 0, total)
		for _, page := range pages {
			titles = append(titles, feedTitles(page.Items)...)
		}
		return titles
	}

	// The descending default: newest first, three pages under the mutations.
	first := page("", false)
	if _, err := h.pool.Exec(h.ctx(),
		`UPDATE properties SET name = 'Переименовано' WHERE id = $1`, h.propID); err != nil {
		h.t.Fatalf("rename property: %v", err)
	}
	// A paid fact created between the loads carries the newest date — it
	// lands past both walks, never inside an unwalked window.
	h.seedGlobalOperation(h.propID, h.owner, "2026-12-31", opPaid, "expense",
		"Между порциями", 999000, testIntegrationSlugUtilities, "Коммунальные услуги")
	second := page(first.NextCursor, false)
	third := page(second.NextCursor, false)

	got := collect([]paymentsapp.GlobalOperationsPage{first, second, third})
	if len(got) != total {
		t.Fatalf("three pages carry %d rows, want %d — rows dropped or duplicated", len(got), total)
	}
	seen := make(map[string]bool, total)
	for _, title := range got {
		if seen[title] {
			t.Errorf("row %q arrived twice", title)
		}
		seen[title] = true
	}
	for i := range total {
		if want := fmt.Sprintf("Операция %03d", total-1-i); got[i] != want {
			t.Fatalf("desc row %d = %q, want %q — the walk broke the (date, id) order", i, got[i], want)
		}
	}
	// The third page came back full, so it still answers with a
	// continuation — the walk stops on the short page it produces.
	fourth := page(third.NextCursor, false)
	if len(fourth.Items) != 0 || fourth.NextCursor != "" {
		t.Errorf("fourth page = %d rows/%q, want the empty exhausted page", len(fourth.Items), fourth.NextCursor)
	}

	// The ascending order walks the same feed the other way with the same
	// guarantee.
	ascFirst := page("", true)
	ascSecond := page(ascFirst.NextCursor, true)
	ascThird := page(ascSecond.NextCursor, true)
	ascGot := collect([]paymentsapp.GlobalOperationsPage{ascFirst, ascSecond, ascThird})
	if len(ascGot) != total {
		t.Fatalf("ascending pages carry %d rows, want %d", len(ascGot), total)
	}
	for i := range total {
		if want := fmt.Sprintf("Операция %03d", i); ascGot[i] != want {
			t.Fatalf("asc row %d = %q, want %q", i, ascGot[i], want)
		}
	}
}

// rentFeedTitle is the recurring seeded feed title (goconst).
const rentFeedTitle = "Аренда"

// The global feed's total (ticket #599): the whole scope's paid count under
// the query's filters — the same predicate as the rows — constant across
// the keyset pages, independent of the window's position.
func TestListGlobalOperations_TotalMatchesScope(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	h.globalFeedFixture()

	// The visible scope: the fixture's four paid facts — the planned, the
	// cancelled, the archived and the foreign rows stay out.
	page, err := h.ops.ListGlobalOperations(h.ctx(), h.owner, paymentsapp.GlobalOperationsListQuery{})
	if err != nil {
		t.Fatalf("list global: %v", err)
	}
	if len(page.Items) != 4 {
		t.Fatalf("items = %d, want the four paid rows", len(page.Items))
	}
	if page.Total != 4 {
		t.Fatalf("total = %d, want 4", page.Total)
	}

	// The total answers the query's scope, not the window: a half-page walk
	// counts the whole feed on both pages.
	pageOne, err := h.ops.ListGlobalOperations(h.ctx(), h.owner, paymentsapp.GlobalOperationsListQuery{Limit: 2})
	if err != nil {
		t.Fatalf("page one: %v", err)
	}
	if pageOne.NextCursor == "" {
		t.Fatal("page one nextCursor = '', want the continuation of the four-row feed")
	}
	pageTwo, err := h.ops.ListGlobalOperations(h.ctx(), h.owner, paymentsapp.GlobalOperationsListQuery{
		Limit: 2, Cursor: pageOne.NextCursor,
	})
	if err != nil {
		t.Fatalf("page two: %v", err)
	}
	if pageOne.Total != 4 || pageTwo.Total != 4 {
		t.Fatalf("totals = %d/%d, want 4/4 — the scope's count on every page", pageOne.Total, pageTwo.Total)
	}

	// The search narrows the total with the rows: the two Аренда facts.
	searched, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{Search: rentFeedTitle})
	if err != nil {
		t.Fatalf("searched: %v", err)
	}
	if searched.Total != 2 {
		t.Fatalf("searched total = %d, want 2", searched.Total)
	}

	// The direction filter: the one income (the shared «Аренда» is the
	// fixture's expense twin) against the three expenses.
	income := domain.TypeIncome
	incomes, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{Type: &income})
	if err != nil {
		t.Fatalf("incomes: %v", err)
	}
	if incomes.Total != 1 {
		t.Fatalf("income total = %d, want 1", incomes.Total)
	}
	expense := domain.TypeExpense
	expenses, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{Type: &expense})
	if err != nil {
		t.Fatalf("expenses: %v", err)
	}
	if expenses.Total != 3 {
		t.Fatalf("expense total = %d, want 3", expenses.Total)
	}
}

// The total's filter vocabulary (ticket #599): the propertyIds multi-select
// and the archive cut narrow the scope's count exactly as they narrow the
// rows.
func TestListGlobalOperations_TotalMatchesFilters(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	ownB, shared, archived, _ := h.globalFeedFixture()

	// The propertyIds multi-select: the one Дом row; the shared book counts
	// through the membership.
	byProperty, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{PropertyIDs: []uuid.UUID{ownB}})
	if err != nil {
		t.Fatalf("by property: %v", err)
	}
	if byProperty.Total != 1 {
		t.Fatalf("ownB total = %d, want 1", byProperty.Total)
	}
	byShared, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{PropertyIDs: []uuid.UUID{shared}})
	if err != nil {
		t.Fatalf("by shared: %v", err)
	}
	if byShared.Total != 1 {
		t.Fatalf("shared total = %d, want 1", byShared.Total)
	}

	// The archive cut lifts: the archived ЖКУ rejoins the scope's count.
	lifted, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{IncludeArchived: true, PropertyIDs: []uuid.UUID{archived}})
	if err != nil {
		t.Fatalf("archived: %v", err)
	}
	if lifted.Total != 1 {
		t.Fatalf("archived total = %d, want 1 — the lifted cut lets the archived row count", lifted.Total)
	}
}
