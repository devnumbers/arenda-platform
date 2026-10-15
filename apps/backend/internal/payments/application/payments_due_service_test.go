package application

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
)

// The «На оплату» read's unit tests (ticket #1346): the application layer's
// own business — the owner→today threading and the rows' pass-through —
// against func-backed fakes; the window predicate, the ordering and the
// counters' aggregation are the store's SQL (integration tests). The due
// list is a pure read: the rows arrive window-filtered and ordered from the
// store, the service folds them into the response with the scope count.

// The due fixture's recurring literals; goconst wants named constants.
const (
	duePropertyName = "Квартира на_due"
	dueUtilities    = "utilities"
)

// The «На оплату» rows pass through with every field the wire needs: the
// (rule, operation) pair identity, the category reference, the owner's
// today, the due date and the days-left the SQL computed, the reminder
// lead. The response count is the rows' own — the read has no window, the
// whole section comes back in one read.
func TestListPaymentDueReturnsRows(t *testing.T) {
	t.Parallel()

	owner := uuid.Must(uuid.NewV7())
	property := uuid.Must(uuid.NewV7())
	today := utcDate(2026, time.September, 8)
	dueDate := utcDate(2026, time.September, 10)
	slug := dueUtilities
	ruleID := uuid.Must(uuid.NewV7())
	operationID := uuid.Must(uuid.NewV7())

	item := PaymentDueItem{
		ID:                 ruleID,
		OperationID:        operationID,
		PropertyID:         property,
		PropertyName:       duePropertyName,
		Type:               domain.TypeExpense,
		Title:              feedInsuranceTitle,
		AmountKopecks:      3200000,
		Category:           domain.CategoryRef{Slug: &slug},
		Today:              today,
		DueDate:            dueDate,
		DaysLeft:           2,
		ReminderOffsetDays: 3,
	}
	reader := &fakeGlobalReader{
		owners: []uuid.UUID{owner},
		due:    []PaymentDueItem{item},
	}
	service := NewGlobalPaymentService(reader, fakeGlobalCalendar{todays: map[uuid.UUID]time.Time{
		owner: today,
	}}, txStoreFactory{})

	due, err := service.ListPaymentDue(t.Context(), uuid.Must(uuid.NewV7()))
	if err != nil {
		t.Fatalf("ListPaymentDue: %v", err)
	}
	if len(due.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(due.Items))
	}
	if due.DueOperationsCount != 1 {
		t.Errorf("dueOperationsCount = %d, want 1 — the read returns the whole section", due.DueOperationsCount)
	}
	if len(reader.gotTodays) != 1 || reader.gotTodays[owner].IsZero() {
		t.Errorf("reader got todays = %v, want the owner's today", reader.gotTodays)
	}
	assertDueItem(t, due.Items[0], item)
}

// assertDueItem checks the row against the fixture: the pass-through keeps
// every field the wire needs (the identity pair, the labels, the window
// figures, the category reference).
func assertDueItem(t *testing.T, got, want PaymentDueItem) {
	t.Helper()
	if got.ID != want.ID || got.OperationID != want.OperationID || got.PropertyID != want.PropertyID {
		t.Errorf("ids = (%s, %s, %s), want the seeded pair", got.ID, got.OperationID, got.PropertyID)
	}
	if got.PropertyName != want.PropertyName || got.Title != want.Title || got.AmountKopecks != want.AmountKopecks {
		t.Errorf("labels = (%q, %q, %d), want the rule's own", got.PropertyName, got.Title, got.AmountKopecks)
	}
	if got.Category.Slug == nil || *got.Category.Slug != dueUtilities {
		t.Errorf("category = %+v, want the utilities slug", got.Category)
	}
	if !got.Today.Equal(want.Today) || !got.DueDate.Equal(want.DueDate) {
		t.Errorf("dates = (%v, %v), want (%v, %v)", got.Today, got.DueDate, want.Today, want.DueDate)
	}
	if got.DaysLeft != want.DaysLeft || got.ReminderOffsetDays != want.ReminderOffsetDays {
		t.Errorf("window = (daysLeft %d, offset %d), want (%d, %d)",
			got.DaysLeft, got.ReminderOffsetDays, want.DaysLeft, want.ReminderOffsetDays)
	}
}

// An empty visible scope (no properties to read) answers the empty section
// without reaching the store.
func TestListPaymentDueEmptyScope(t *testing.T) {
	t.Parallel()

	reader := &fakeGlobalReader{}
	service := NewGlobalPaymentService(reader, fakeGlobalCalendar{}, txStoreFactory{})

	due, err := service.ListPaymentDue(t.Context(), uuid.Must(uuid.NewV7()))
	if err != nil {
		t.Fatalf("ListPaymentDue: %v", err)
	}
	if len(due.Items) != 0 || due.DueOperationsCount != 0 {
		t.Fatalf("due = %+v, want the empty section", due)
	}
	if reader.dueCalled {
		t.Fatal("the due read reached the store on an empty scope")
	}
}

// The feed's due counters (ticket #1346) ride the existing scope counters:
// the «На оплату» card's number and the nearest days-left pass through to
// the response beside the favorites and the overdue.
func TestListGlobalPaymentsDueCounters(t *testing.T) {
	t.Parallel()

	owner := uuid.Must(uuid.NewV7())
	today := utcDate(2026, time.September, 8)
	nearest := 1
	reader := &fakeGlobalReader{
		owners: []uuid.UUID{owner},
		counters: GlobalPaymentCounters{
			FavoriteCount:          2,
			OverdueOperationsCount: 5,
			DueOperationsCount:     3,
			DueNearestDays:         &nearest,
		},
	}
	service := NewGlobalPaymentService(reader, fakeGlobalCalendar{todays: map[uuid.UUID]time.Time{
		owner: today,
	}}, txStoreFactory{})

	feed, err := service.ListGlobalPayments(t.Context(), uuid.Must(uuid.NewV7()))
	if err != nil {
		t.Fatalf("ListGlobalPayments: %v", err)
	}
	if feed.DueOperationsCount != 3 {
		t.Errorf("dueOperationsCount = %d, want 3", feed.DueOperationsCount)
	}
	if feed.DueNearestDays == nil || *feed.DueNearestDays != 1 {
		t.Errorf("dueNearestDays = %v, want 1 — «Завтра»", feed.DueNearestDays)
	}
	if feed.FavoriteCount != 2 || feed.OverdueOperationsCount != 5 {
		t.Errorf("old counters = (%d, %d), want (2, 5) — the due adds, never replaces", feed.FavoriteCount, feed.OverdueOperationsCount)
	}
}
