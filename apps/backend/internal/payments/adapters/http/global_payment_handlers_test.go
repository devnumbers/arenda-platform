package http

// The contract tests of the global payment rules endpoints (ticket #575):
// the wire shape of the feed with the counters, the search's matched
// categories and the «Объекты» stacks — against func-backed doubles,
// mirroring operation_handlers_test.go.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

// The fixtures' recurring literals; goconst wants named constants.
const wirePropertyName = "Моя квартира"

var (
	wireToday   = time.Date(2026, time.September, 8, 0, 0, 0, 0, time.UTC)
	wireNearest = time.Date(2026, time.September, 15, 0, 0, 0, 0, time.UTC)
)

// wireFeedItem is the one fixture row of the wire shape tests: the overdue
// favorite with a stored next date.
func wireFeedItem() application.GlobalPaymentItem {
	slug := "insurance"
	overdueDays := 10
	nearest := wireNearest
	return application.GlobalPaymentItem{
		ID:            uuid.Must(uuid.NewV7()),
		PropertyID:    uuid.Must(uuid.NewV7()),
		PropertyName:  wirePropertyName,
		Type:          "expense",
		Title:         "Страхование",
		AmountKopecks: 3200000,
		Category:      defaultCategoryRef(slug),
		IsFavorite:    true,
		Today:         wireToday,
		NearestDate:   &nearest,
		OverdueCount:  2,
		OverdueDays:   &overdueDays,
	}
}

// fakeGlobalPaymentsManager is the func-backed GlobalPaymentsManager double
// (ADR 0035 test doubles): an unset use case fails the test loudly.
type fakeGlobalPaymentsManager struct {
	list   func(ctx context.Context, actor uuid.UUID) (application.GlobalPaymentFeed, error)
	search func(
		ctx context.Context, actor uuid.UUID, query string, page application.GlobalPaymentSearchPage,
	) (application.GlobalPaymentSearch, error)
	objects func(ctx context.Context, actor uuid.UUID, query string) ([]application.GlobalPaymentObjectCard, error)
	save    func(ctx context.Context, actor uuid.UUID, paymentIDs []uuid.UUID) error
	due     func(ctx context.Context, actor uuid.UUID) (application.PaymentDueList, error)
}

func (f *fakeGlobalPaymentsManager) ListGlobalPayments(
	ctx context.Context, actor uuid.UUID,
) (application.GlobalPaymentFeed, error) {
	if f.list == nil {
		return application.GlobalPaymentFeed{}, errors.New("unexpected ListGlobalPayments call")
	}
	return f.list(ctx, actor)
}

func (f *fakeGlobalPaymentsManager) SearchGlobalPayments(
	ctx context.Context, actor uuid.UUID, query string, page application.GlobalPaymentSearchPage,
) (application.GlobalPaymentSearch, error) {
	if f.search == nil {
		return application.GlobalPaymentSearch{}, errors.New("unexpected SearchGlobalPayments call")
	}
	return f.search(ctx, actor, query, page)
}

func (f *fakeGlobalPaymentsManager) ListGlobalPaymentObjects(
	ctx context.Context, actor uuid.UUID, query string,
) ([]application.GlobalPaymentObjectCard, error) {
	if f.objects == nil {
		return nil, errors.New("unexpected ListGlobalPaymentObjects call")
	}
	return f.objects(ctx, actor, query)
}

func (f *fakeGlobalPaymentsManager) SaveFavoriteOrder(
	ctx context.Context, actor uuid.UUID, paymentIDs []uuid.UUID,
) error {
	if f.save == nil {
		return errors.New("unexpected SaveFavoriteOrder call")
	}
	return f.save(ctx, actor, paymentIDs)
}

func (f *fakeGlobalPaymentsManager) ListPaymentDue(
	ctx context.Context, actor uuid.UUID,
) (application.PaymentDueList, error) {
	if f.due == nil {
		return application.PaymentDueList{}, errors.New("unexpected ListPaymentDue call")
	}
	return f.due(ctx, actor)
}

// globalRequest builds an authenticated request for the parameterless global
// reads.
func globalRequest(ctx context.Context, target string) *http.Request {
	return httptest.NewRequestWithContext(
		httpsupport.WithUserID(ctx, uuid.Must(uuid.NewV7())), http.MethodGet, target, nil,
	)
}

func TestListGlobalPayments_WireShape(t *testing.T) {
	t.Parallel()

	svc := &fakeGlobalPaymentsManager{
		list: func(_ context.Context, _ uuid.UUID) (application.GlobalPaymentFeed, error) {
			return application.GlobalPaymentFeed{Items: []application.GlobalPaymentItem{wireFeedItem()}}, nil
		},
	}
	h := NewGlobalPaymentHandlers(svc, nil)

	w := httptest.NewRecorder()
	h.ListGlobalPayments(w, globalRequest(t.Context(), "/payments"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var body openapi.PaymentsGlobalResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(body.Items))
	}
	row := body.Items[0]
	if row.PropertyName != wirePropertyName || row.AmountKopecks != 3200000 || !row.IsFavorite {
		t.Errorf("row = (%q, %d, %v), want the label, the kopecks and the star", row.PropertyName, row.AmountKopecks, row.IsFavorite)
	}
	if row.Type != openapi.PaymentGlobalItemTypeExpense {
		t.Errorf("type = %q, want expense", row.Type)
	}
	if !row.Today.Equal(wireToday) {
		t.Errorf("today = %v, want %v", row.Today, wireToday)
	}
	if row.OverdueOperationCount != 2 || row.OverdueDays == nil || *row.OverdueDays != 10 {
		t.Errorf("overdue = (%d, %v), want (2, 10)", row.OverdueOperationCount, row.OverdueDays)
	}
}

// TestListGlobalPayments_WireCounters checks the main screen's two counters
// and the schedule fields' wire shape — the «Все X (N)» cards' data and the
// resolved category.
func TestListGlobalPayments_WireCounters(t *testing.T) {
	t.Parallel()

	svc := &fakeGlobalPaymentsManager{
		list: func(_ context.Context, _ uuid.UUID) (application.GlobalPaymentFeed, error) {
			nearest := 1
			return application.GlobalPaymentFeed{
				Items:                  []application.GlobalPaymentItem{wireFeedItem()},
				FavoriteCount:          1,
				OverdueOperationsCount: 7,
				DueOperationsCount:     3,
				DueNearestDays:         &nearest,
			}, nil
		},
	}
	h := NewGlobalPaymentHandlers(svc, nil)

	w := httptest.NewRecorder()
	h.ListGlobalPayments(w, globalRequest(t.Context(), "/payments"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var body openapi.PaymentsGlobalResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.FavoriteCount != 1 || body.OverdueOperationsCount != 7 {
		t.Errorf("counters = (%d, %d), want (1, 7)", body.FavoriteCount, body.OverdueOperationsCount)
	}
	if body.DueOperationsCount != 3 || body.DueNearestDays == nil || *body.DueNearestDays != 1 {
		t.Errorf("due counters = (%d, %v), want (3, 1) — «Завтра»", body.DueOperationsCount, body.DueNearestDays)
	}
	row := body.Items[0]
	if row.Category.Slug == nil || *row.Category.Slug != "insurance" || row.Category.Label != "Страхование" {
		t.Errorf("category = %+v, want the resolved default catalog entry", row.Category)
	}
	if row.NearestDate == nil || !row.NearestDate.Equal(wireNearest) {
		t.Errorf("nearest = %v, want %v", row.NearestDate, wireNearest)
	}
}

func TestListGlobalPayments_NullNearestAndOverdueTravelAsNulls(t *testing.T) {
	t.Parallel()

	svc := &fakeGlobalPaymentsManager{
		list: func(_ context.Context, _ uuid.UUID) (application.GlobalPaymentFeed, error) {
			return application.GlobalPaymentFeed{
				Items: []application.GlobalPaymentItem{{ID: uuid.Must(uuid.NewV7()), PropertyID: uuid.Must(uuid.NewV7()), Title: "Пауза"}},
			}, nil
		},
	}
	h := NewGlobalPaymentHandlers(svc, nil)

	w := httptest.NewRecorder()
	h.ListGlobalPayments(w, globalRequest(t.Context(), "/payments"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var body struct {
		Items []openapi.PaymentGlobalItem `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Items[0].NearestDate != nil || body.Items[0].OverdueDays != nil {
		t.Errorf("nullable fields = (%v, %v), want JSON nulls", body.Items[0].NearestDate, body.Items[0].OverdueDays)
	}
}

// The «На оплату» section's wire shape (ticket #1346): every row carries
// the (rule, operation) pair identity, the bucket source (daysLeft) and the
// window's own figures (today, dueDate, the reminder lead); the count
// repeats the rows.
func TestListPaymentDue_WireShape(t *testing.T) {
	t.Parallel()

	ruleID := uuid.Must(uuid.NewV7())
	operationID := uuid.Must(uuid.NewV7())
	propertyID := uuid.Must(uuid.NewV7())
	today := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	dueDate := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	row := dueWireRow(t, application.PaymentDueItem{
		ID:                 ruleID,
		OperationID:        operationID,
		PropertyID:         propertyID,
		PropertyName:       "Квартира",
		Type:               domain.TypeExpense,
		Title:              testTitleZhku,
		AmountKopecks:      500000,
		Category:           defaultCategoryRef(testSlugUtilities),
		Today:              today,
		DueDate:            dueDate,
		DaysLeft:           2,
		ReminderOffsetDays: 3,
	})
	if row.PaymentId != ruleID || row.OperationId != operationID || row.PropertyId != propertyID {
		t.Errorf("ids = (%s, %s, %s), want the seeded pair", row.PaymentId, row.OperationId, row.PropertyId)
	}
	if row.Title != testTitleZhku || row.AmountKopecks != 500000 || row.PropertyName != "Квартира" {
		t.Errorf("labels = (%q, %d, %q), want the rule's own", row.Title, row.AmountKopecks, row.PropertyName)
	}
	if row.DaysLeft != 2 || row.ReminderOffsetDays != 3 {
		t.Errorf("window = (daysLeft %d, offset %d), want (2, 3)", row.DaysLeft, row.ReminderOffsetDays)
	}
	if !row.Today.Equal(today) || !row.DueDate.Equal(dueDate) {
		t.Errorf("dates = (%v, %v), want (%v, %v)", row.Today, row.DueDate, today, dueDate)
	}
	if row.Category.Slug == nil || *row.Category.Slug != testSlugUtilities {
		t.Errorf("category = %+v, want the utilities slug", row.Category)
	}
}

// dueWireRow drives GET /payments/due through the handler over a canned
// single-row section and returns the decoded first row (the wire shape's
// test harness).
func dueWireRow(t *testing.T, item application.PaymentDueItem) openapi.PaymentDueItem {
	t.Helper()

	svc := &fakeGlobalPaymentsManager{
		due: func(_ context.Context, _ uuid.UUID) (application.PaymentDueList, error) {
			return application.PaymentDueList{
				Items:              []application.PaymentDueItem{item},
				DueOperationsCount: 1,
			}, nil
		},
	}
	h := NewGlobalPaymentHandlers(svc, nil)

	w := httptest.NewRecorder()
	h.ListPaymentDue(w, globalRequest(t.Context(), "/payments/due"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var body openapi.PaymentsDueResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.DueOperationsCount != 1 || len(body.Items) != 1 {
		t.Fatalf("response = (%d rows, count %d), want (1, 1)", len(body.Items), body.DueOperationsCount)
	}
	return body.Items[0]
}

// The empty window answers the empty section with the zero count — the
// card's number the feed repeats.
func TestListPaymentDue_EmptySection(t *testing.T) {
	t.Parallel()

	svc := &fakeGlobalPaymentsManager{
		due: func(_ context.Context, _ uuid.UUID) (application.PaymentDueList, error) {
			return application.PaymentDueList{Items: []application.PaymentDueItem{}}, nil
		},
	}
	h := NewGlobalPaymentHandlers(svc, nil)

	w := httptest.NewRecorder()
	h.ListPaymentDue(w, globalRequest(t.Context(), "/payments/due"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var body openapi.PaymentsDueResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Items) != 0 || body.DueOperationsCount != 0 {
		t.Errorf("empty section = (%d rows, count %d), want (0, 0)", len(body.Items), body.DueOperationsCount)
	}
}

func TestSearchGlobalPayments_FoldsQueryAndMapsChips(t *testing.T) {
	t.Parallel()

	var gotQuery string
	var gotPage application.GlobalPaymentSearchPage
	svc := &fakeGlobalPaymentsManager{
		search: func(
			_ context.Context, _ uuid.UUID, query string, page application.GlobalPaymentSearchPage,
		) (application.GlobalPaymentSearch, error) {
			gotQuery = query
			gotPage = page
			id := uuid.Must(uuid.NewV7())
			return application.GlobalPaymentSearch{
				MatchedCategories: []domain.CategoryRef{customCategoryRef(id, "Кофейни")},
			}, nil
		},
	}
	h := NewGlobalPaymentHandlers(svc, nil)

	query := "кофе"
	w := httptest.NewRecorder()
	h.SearchGlobalPayments(w, globalRequest(t.Context(), "/payments/search?search=%D0%BA%D0%BE%D1%84%D0%B5"),
		openapi.SearchGlobalPaymentsParams{Search: &query})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if gotQuery != "кофе" {
		t.Errorf("search query = %q, want the decoded parameter", gotQuery)
	}
	if gotPage.Category != "" || gotPage.Type != "" || gotPage.Limit != 0 {
		t.Errorf("page = %+v, want the zero page (the contract's defaults)", gotPage)
	}
	var body openapi.PaymentsSearchGlobalResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.MatchedCategories) != 1 {
		t.Fatalf("chips = %d, want 1", len(body.MatchedCategories))
	}
	// The chip rides out as the bare category view — no direction, no count
	// in the contract (ticket #602).
	chip := body.MatchedCategories[0]
	if chip.Source != openapi.CategoryViewSourceCustom || chip.Label != "Кофейни" {
		t.Errorf("chip = %+v, want the custom identity", chip)
	}
}

// The chip filter and the page window ride the query parameters into the
// use case (map #573 rework): category + type + limit/cursor — the keyset
// continuation echoes through as the opaque string (ticket #597).
// TestEchoedCursor is the continuation cursor the folding tests echo
// through (ticket #597).
const testEchoedCursor = "cursor-from-previous-page"

func TestSearchGlobalPayments_CarriesChipFilterAndPage(t *testing.T) {
	t.Parallel()

	var gotPage application.GlobalPaymentSearchPage
	svc := &fakeGlobalPaymentsManager{
		search: func(
			_ context.Context, _ uuid.UUID, _ string, page application.GlobalPaymentSearchPage,
		) (application.GlobalPaymentSearch, error) {
			gotPage = page
			return application.GlobalPaymentSearch{}, nil
		},
	}
	h := NewGlobalPaymentHandlers(svc, nil)

	limit := 50
	cursor := testEchoedCursor
	category := "parking"
	paymentType := openapi.SearchGlobalPaymentsParamsTypeExpense
	w := httptest.NewRecorder()
	request := "/payments/search?search=&category=parking&type=expense&limit=50&cursor=" + testEchoedCursor
	h.SearchGlobalPayments(w, globalRequest(t.Context(), request),
		openapi.SearchGlobalPaymentsParams{
			Search:   nil,
			Category: &category,
			Type:     &paymentType,
			Limit:    &limit,
			Cursor:   &cursor,
		})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if gotPage.Category != "parking" || gotPage.Type != domain.TypeExpense {
		t.Errorf("page filter = %q/%q, want parking/expense", gotPage.Category, gotPage.Type)
	}
	if gotPage.Limit != 50 || gotPage.Cursor != testEchoedCursor {
		t.Errorf("page window = %d/%q, want 50 with the echoed cursor", gotPage.Limit, gotPage.Cursor)
	}
}

// The out-of-range page numbers and the unknown direction are the
// contract's 400 (the shared payments table), not an opaque 500 — the
// openapi bounds (minimum/maximum, the enum) are enforced on the wire.
func TestSearchGlobalPayments_RejectsInvalidPage(t *testing.T) {
	t.Parallel()

	svc := &fakeGlobalPaymentsManager{
		search: func(_ context.Context, _ uuid.UUID, _ string, _ application.GlobalPaymentSearchPage) (application.GlobalPaymentSearch, error) {
			return application.GlobalPaymentSearch{}, errors.New("must not be called")
		},
	}
	h := NewGlobalPaymentHandlers(svc, nil)

	limit0, limit101 := 0, 101
	unknownType := openapi.SearchGlobalPaymentsParamsType("foobar")
	cases := []struct {
		name   string
		params openapi.SearchGlobalPaymentsParams
	}{
		{"limit below the minimum", openapi.SearchGlobalPaymentsParams{Limit: &limit0}},
		{"limit above the maximum", openapi.SearchGlobalPaymentsParams{Limit: &limit101}},
		{"unknown direction", openapi.SearchGlobalPaymentsParams{Type: &unknownType}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			h.SearchGlobalPayments(w, globalRequest(t.Context(), "/payments/search"), tc.params)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestListGlobalPaymentObjects_WireStacks(t *testing.T) {
	t.Parallel()

	var gotQuery string
	propertyID := uuid.Must(uuid.NewV7())
	autoRule := uuid.Must(uuid.NewV7())
	plainRule := uuid.Must(uuid.NewV7())
	svc := &fakeGlobalPaymentsManager{
		objects: func(_ context.Context, _ uuid.UUID, query string) ([]application.GlobalPaymentObjectCard, error) {
			gotQuery = query
			photo := "/uploads/first.jpg"
			return []application.GlobalPaymentObjectCard{{
				PropertyID:  propertyID,
				Name:        wirePropertyName,
				Address:     "Тверская 1",
				Type:        "apartment",
				PhotoURL:    &photo,
				AutoPayKeys: []application.GlobalPaymentObjectKey{{PaymentID: autoRule, HasOverdue: true}},
				OtherKeys:   []application.GlobalPaymentObjectKey{{PaymentID: plainRule, HasOverdue: false}},
			}}, nil
		},
	}
	h := NewGlobalPaymentHandlers(svc, nil)

	query := "квар"
	w := httptest.NewRecorder()
	h.ListGlobalPaymentObjects(w, globalRequest(t.Context(), "/payments/objects?search=%D0%BA%D0%B2%D0%B0%D1%80"),
		openapi.ListGlobalPaymentObjectsParams{Search: &query})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if gotQuery != "квар" {
		t.Errorf("search query = %q, want the decoded parameter", gotQuery)
	}
	var body openapi.PaymentObjectsGlobalResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("cards = %d, want 1", len(body.Items))
	}
	assertGlobalObjectCard(t, body.Items[0], propertyID, autoRule, plainRule)
}

// assertGlobalObjectCard checks one card's wire mapping: the property lines
// (id, name, address), both stacks, the avatar photo and the type (#1244).
func assertGlobalObjectCard(t *testing.T, card openapi.PaymentObjectItem, propertyID, autoRule, plainRule uuid.UUID) {
	t.Helper()

	if card.PropertyId != propertyID || card.Name != "Моя квартира" || card.Address != "Тверская 1" {
		t.Errorf("card = %+v, want the property card", card)
	}
	if len(card.AutoPayRules) != 1 || !card.AutoPayRules[0].HasOverdue || card.AutoPayRules[0].PaymentId != autoRule {
		t.Errorf("auto-pay keys = %+v, want the overdue key", card.AutoPayRules)
	}
	if len(card.OtherRules) != 1 || card.OtherRules[0].HasOverdue || card.OtherRules[0].PaymentId != plainRule {
		t.Errorf("other keys = %+v, want the clean key", card.OtherRules)
	}
	if card.PhotoUrl == nil || *card.PhotoUrl != "/uploads/first.jpg" {
		t.Errorf("photoUrl = %v, want the card's avatar photo", card.PhotoUrl)
	}
	if card.Type != "apartment" {
		t.Errorf("type = %q, want the object's type on the wire (#1244)", card.Type)
	}
}

// defaultCategoryRef builds a default-catalog category reference.
func defaultCategoryRef(slug string) domain.CategoryRef {
	return domain.CategoryRef{Slug: &slug}
}

// customCategoryRef builds a user-category reference with its current name.
func customCategoryRef(id uuid.UUID, name string) domain.CategoryRef {
	return domain.CategoryRef{UserCategoryID: &id, UserCategoryName: &name}
}

// The wire contract of the order save (ticket #576): the body's ids travel
// to the use case in order under the actor; 204 with no content on success.
func TestSaveGlobalPaymentFavoritesOrder_Wire(t *testing.T) {
	t.Parallel()

	var gotActor uuid.UUID
	var gotIDs []uuid.UUID
	svc := &fakeGlobalPaymentsManager{
		save: func(ctx context.Context, actor uuid.UUID, paymentIDs []uuid.UUID) error {
			gotActor = actor
			gotIDs = paymentIDs
			return nil
		},
	}
	h := NewGlobalPaymentHandlers(svc, nil)

	idA := uuid.Must(uuid.NewV7())
	idB := uuid.Must(uuid.NewV7())
	userID := uuid.Must(uuid.NewV7())
	body := `{"paymentIds":["` + idB.String() + `","` + idA.String() + `"]}`
	w := httptest.NewRecorder()
	h.SaveGlobalPaymentFavoritesOrder(w, paymentRequest(t, http.MethodPut, userID, body))

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", w.Code, w.Body.String())
	}
	if gotActor != userID {
		t.Errorf("actor = %s, want %s", gotActor, userID)
	}
	if len(gotIDs) != 2 || gotIDs[0] != idB || gotIDs[1] != idA {
		t.Errorf("ids = %v, want the submitted order [%s %s]", gotIDs, idB, idA)
	}
}

// The use case's verdicts map onto the wire: the privacy 404 and the
// invalid-list 400 (the payments problem table's statuses).
func TestSaveGlobalPaymentFavoritesOrder_ErrorMapping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"foreign id is the privacy 404", application.ErrNotFound, http.StatusNotFound},
		{"broken list is 400", application.ErrInvalidInput, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := &fakeGlobalPaymentsManager{
				save: func(context.Context, uuid.UUID, []uuid.UUID) error { return tc.err },
			}
			h := NewGlobalPaymentHandlers(svc, nil)

			body := `{"paymentIds":["` + uuid.Must(uuid.NewV7()).String() + `"]}`
			w := httptest.NewRecorder()
			h.SaveGlobalPaymentFavoritesOrder(w, paymentRequest(t, http.MethodPut, uuid.Must(uuid.NewV7()), body))

			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.wantStatus, w.Body.String())
			}
		})
	}
}

// A body without the required paymentIds is a 400 before the use case runs;
// an empty array is a valid no-op save.
func TestSaveGlobalPaymentFavoritesOrder_BodyValidation(t *testing.T) {
	t.Parallel()

	called := 0
	svc := &fakeGlobalPaymentsManager{
		save: func(context.Context, uuid.UUID, []uuid.UUID) error {
			called++
			return nil
		},
	}
	h := NewGlobalPaymentHandlers(svc, nil)
	userID := uuid.Must(uuid.NewV7())

	w := httptest.NewRecorder()
	h.SaveGlobalPaymentFavoritesOrder(w, paymentRequest(t, http.MethodPut, userID, `{"other":1}`))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing paymentIds: status = %d, want 400: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	h.SaveGlobalPaymentFavoritesOrder(w, paymentRequest(t, http.MethodPut, userID, `{}`))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty body: status = %d, want 400: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	h.SaveGlobalPaymentFavoritesOrder(w, paymentRequest(t, http.MethodPut, userID, `{"paymentIds":[]}`))
	if w.Code != http.StatusNoContent {
		t.Fatalf("empty array: status = %d, want 204: %s", w.Code, w.Body.String())
	}
	if called != 1 {
		t.Errorf("use case calls = %d, want 1 (the no-op save)", called)
	}
}

// The feed row carries the manual favorite order onto the wire (ticket
// #576): a saved position travels as favoriteOrder, a never-saved rule as
// null.
func TestListGlobalPayments_FavoriteOrderWire(t *testing.T) {
	t.Parallel()

	position := int64(2)
	item := wireFeedItem()
	item.FavoriteOrder = &position
	nilItem := wireFeedItem()
	svc := &fakeGlobalPaymentsManager{
		list: func(_ context.Context, _ uuid.UUID) (application.GlobalPaymentFeed, error) {
			return application.GlobalPaymentFeed{Items: []application.GlobalPaymentItem{item, nilItem}}, nil
		},
	}
	h := NewGlobalPaymentHandlers(svc, nil)

	w := httptest.NewRecorder()
	h.ListGlobalPayments(w, globalRequest(t.Context(), "/payments"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var body openapi.PaymentsGlobalResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Items[0].FavoriteOrder == nil || *body.Items[0].FavoriteOrder != 2 {
		t.Errorf("favoriteOrder = %v, want 2", body.Items[0].FavoriteOrder)
	}
	if body.Items[1].FavoriteOrder != nil {
		t.Errorf("nil favoriteOrder = %v, want null", body.Items[1].FavoriteOrder)
	}
}

// The search wire's total (ticket #599): the scope's match count travels on
// every page — the contract's required field.
func TestSearchGlobalPayments_CarriesScopeTotal(t *testing.T) {
	t.Parallel()

	svc := &fakeGlobalPaymentsManager{
		search: func(
			_ context.Context, _ uuid.UUID, _ string, _ application.GlobalPaymentSearchPage,
		) (application.GlobalPaymentSearch, error) {
			return application.GlobalPaymentSearch{Total: 7}, nil
		},
	}
	h := NewGlobalPaymentHandlers(svc, nil)

	w := httptest.NewRecorder()
	h.SearchGlobalPayments(w, globalRequest(t.Context(), "/payments/search"), openapi.SearchGlobalPaymentsParams{})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if total, ok := body["total"].(float64); !ok || total != 7 {
		t.Fatalf("total = %v, want 7 on the wire", body["total"])
	}
}
