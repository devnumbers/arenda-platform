//go:build integration

package application_test

// The integration tests of the second payments contracts slice (ticket #461):
// pay-now semantics, the paginated/filtered listings, computed overdue by the
// owner's timezone, the atomic favorite toggle and the role matrix — against
// real PostgreSQL through the same harness as tickets #457/#458.

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// Shared fixtures of this family. The audit-action names are hoisted because
// several files of the package assert against them; day27 joins the harness
// date consts (day26 stays unhoisted there: its meaning is TZ-dependent).
const (
	day24                  = "2026-08-24"
	day27                  = "2026-08-27"
	actionPaymentCreated   = "payment.created"
	actionPaymentUpdated   = "payment.updated"
	actionPaymentDeleted   = "payment.deleted"
	actionPaymentPaused    = "payment.paused"
	actionPaymentResumed   = "payment.resumed"
	actionOperationPaidStr = "operation.paid"
)

// wantAppError asserts err wraps the expected application sentinel.
func wantAppError(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("err = %v, want %v", err, target)
	}
}

// oldestPlannedOperation returns the earliest planned operation of a rule.
func oldestPlannedOperation(t *testing.T, h *paymentsHarness, paymentID uuid.UUID) uuid.UUID {
	t.Helper()
	rows, err := h.pool.Query(h.ctx(),
		`SELECT id FROM operations WHERE payment_id = $1 AND status = 'planned' ORDER BY date`, paymentID)
	if err != nil {
		t.Fatalf("list planned ids: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate planned ids: %v", err)
		}
		t.Fatal("fixture has no planned operations")
	}
	var id uuid.UUID
	if err := rows.Scan(&id); err != nil {
		t.Fatalf("scan planned id: %v", err)
	}
	return id
}

// seedOperation inserts one operation row directly; the listing and pay-now
// tests drive their fixtures without the tick.
func (h *paymentsHarness) seedOperation(paymentID uuid.UUID, date, status string) uuid.UUID {
	h.t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		h.t.Fatalf("new uuid: %v", err)
	}
	var paid any
	if status == opPaid {
		paid = date
	}
	const slug = "utilities"
	const form = "transfer"
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO operations (id, owner_id, property_id, payment_id, origin, date, paid_date,
		                       status, type, title, amount_kopecks, payment_form, category_label, category_slug)
		 VALUES ($1, $2, $3, $4, 'payment', $5, $6::date,
		         $7, 'expense', 'ЖКУ', 500000, $8, 'Коммунальные услуги', $9)`,
		id, h.owner, h.propID, paymentID, date, paid, status, form, slug,
	); err != nil {
		h.t.Fatalf("seed operation %s (%s): %v", date, status, err)
	}
	return id
}

// listCmd folds the common command shape: nil status means any, limit/offset
// carry the pagination window and asc overrides the default desc direction.
func (h *paymentsHarness) listCmd(status *domain.OperationViewStatus, limit, offset int, asc bool) paymentsapp.OperationsListQuery {
	return paymentsapp.OperationsListQuery{
		Status: status,
		Limit:  limit,
		Offset: offset,
		Asc:    asc,
	}
}

// listingFixture seeds seven operations around the Moscow today (the 25th):
// two overdue, two standing today/future and three closed history rows.
func listingFixture(t *testing.T, h *paymentsHarness, pay uuid.UUID) {
	t.Helper()
	dates := []struct {
		date   string
		status string
	}{
		{"2026-08-12", opPlanned},
		{"2026-08-19", opPlanned},
		{day25, opPlanned},
		{day27, opPlanned},
		{"2026-08-05", opPaid},
		{"2026-08-11", opPaid},
		{"2026-08-18", opPaid},
	}
	for _, row := range dates {
		h.seedOperation(pay, row.date, row.status)
	}
}

func TestPayOperation_PaysOnOwnersTodayAndConflicts(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	pay := h.seedPayment(day23, `{"kind": "daily"}`, false)
	h.runTick() // due: 23rd..25th plus one future planned on the 26th

	target := oldestPlannedOperation(t, h, pay)
	paid, err := h.ops.PayOperation(h.ctx(), h.owner, h.propID, target)
	if err != nil {
		t.Fatalf("pay now: %v", err)
	}
	gotDate := paid.Operation.PaidDate.Format(time.DateOnly)
	if paid.Operation.Status != domain.StatusPaid || gotDate != day25 {
		t.Fatalf("paid state = %s/%s, want paid on the Moscow today (%s)",
			paid.Operation.Status, gotDate, day25)
	}

	_, err = h.ops.PayOperation(h.ctx(), h.owner, h.propID, target)
	wantAppError(t, err, paymentsapp.ErrAlreadyPaid)

	gotAudit := operationsAudit(t, h, target)
	if len(gotAudit) != 1 || gotAudit[0] != actionOperationPaidStr {
		t.Fatalf("operation audit = %v, want [%s]", gotAudit, actionOperationPaidStr)
	}
}

func TestPayOperation_LeavesTheScheduleUnshifted(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	pay := h.seedPayment(day23, `{"kind": "daily"}`, false)
	h.runTick()

	target := oldestPlannedOperation(t, h, pay)
	if _, err := h.ops.PayOperation(h.ctx(), h.owner, h.propID, target); err != nil {
		t.Fatalf("pay now: %v", err)
	}

	now := statusesOf(h.operationsOf(pay))
	want := map[string]string{day23: opPaid, day24: opPlanned, day25: opPlanned, day26: opPlanned}
	if len(now) != len(want) {
		t.Fatalf("operations moved: %+v, want %+v", now, want)
	}
	for date, status := range want {
		if now[date] != status {
			t.Fatalf("operation %s = %s, want %s", date, now[date], status)
		}
	}
}

func TestPayOperation_PaidDateFollowsOwnerTimezone(t *testing.T) {
	t.Parallel()
	// At the anchor instant (2026-08-25T20:00Z) Kamchatka already lives on
	// its 26th while Moscow still counts the 25th (ADR 0048).
	h := newPaymentsHarness(t).withOwner("Asia/Kamchatka")
	pay := h.seedPayment(day26, `{"kind": "daily"}`, false)
	h.runTick()

	var todayOp uuid.UUID
	err := h.pool.QueryRow(h.ctx(),
		`SELECT id FROM operations WHERE payment_id = $1 AND date = DATE '2026-08-26'`, pay,
	).Scan(&todayOp)
	if err != nil {
		t.Fatalf("load kamchatka today occurrence: %v", err)
	}

	paid, err := h.ops.PayOperation(h.ctx(), h.owner, h.propID, todayOp)
	if err != nil {
		t.Fatalf("pay now: %v", err)
	}
	if got := paid.Operation.PaidDate.Format(time.DateOnly); got != day26 {
		t.Fatalf("paid_date = %s, want the Kamchatka today (%s)", got, day26)
	}
}

func TestPayOperation_AheadOfTimeKeepsSingleFuturePlanned(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	// Created today (the 25th): today stands plus tomorrow as the single
	// future planned.
	var tomorrow uuid.UUID
	err = h.pool.QueryRow(h.ctx(),
		`SELECT id FROM operations WHERE payment_id = $1 AND date > DATE '2026-08-25'`, created.ID,
	).Scan(&tomorrow)
	if err != nil {
		t.Fatalf("load future planned: %v", err)
	}

	if _, err := h.ops.PayOperation(h.ctx(), h.owner, h.propID, tomorrow); err != nil {
		t.Fatalf("early pay of the future planned: %v", err)
	}

	rows, err := h.pool.Query(h.ctx(),
		`SELECT date::text FROM operations WHERE payment_id = $1 AND status = 'planned'
		 AND date > DATE '2026-08-25' ORDER BY date`, created.ID)
	if err != nil {
		t.Fatalf("query remaining future planned: %v", err)
	}
	defer rows.Close()
	future := []string{}
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			t.Fatalf("scan date: %v", err)
		}
		future = append(future, d)
	}
	// No shift happened — every original date keeps standing, closed ones as
	// paid facts — but exactly one future planned remains: the next unclosed
	// day after today.
	if len(future) != 1 || future[0] != day27 {
		t.Fatalf("remaining future planned = %v, want [%s]", future, day27)
	}
}

func TestOperationsListing_PaginationAndSort(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	pay := h.seedPayment(day22, `{"kind": "monthly", "dayOfMonth": 15}`, false)
	listingFixture(t, h, pay)

	all, err := h.ops.ListPaymentOperations(h.ctx(), h.owner, h.propID, pay, h.listCmd(nil, 50, 0, false))
	if err != nil {
		t.Fatalf("default list: %v", err)
	}
	if len(all) != 7 || all[0].Operation.Date.Format(time.DateOnly) != day27 {
		t.Fatalf("all = %d items with first %s, want 7 in desc order",
			len(all), all[0].Operation.Date.Format(time.DateOnly))
	}

	firstPage, err := h.ops.ListPaymentOperations(h.ctx(), h.owner, h.propID, pay, h.listCmd(nil, 3, 0, false))
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	secondPage, err := h.ops.ListPaymentOperations(h.ctx(), h.owner, h.propID, pay, h.listCmd(nil, 3, 3, false))
	if err != nil {
		t.Fatalf("offset page: %v", err)
	}
	lastOfFirst := firstPage[len(firstPage)-1]
	firstOfSecond := secondPage[0]
	if lastOfFirst.Operation.Date.Equal(firstOfSecond.Operation.Date) {
		t.Errorf("offset page repeats the boundary row %s",
			lastOfFirst.Operation.Date.Format(time.DateOnly))
	}

	asc, err := h.ops.ListPaymentOperations(h.ctx(), h.owner, h.propID, pay, h.listCmd(nil, 50, 0, true))
	if err != nil {
		t.Fatalf("asc list: %v", err)
	}
	if asc[0].Operation.Date.Format(time.DateOnly) != "2026-08-05" {
		t.Fatalf("asc first = %s, want the oldest",
			asc[0].Operation.Date.Format(time.DateOnly))
	}
}

func TestOperationsListing_StatusFiltersSplitComputedViews(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	pay := h.seedPayment(day22, `{"kind": "monthly", "dayOfMonth": 15}`, false)
	listingFixture(t, h, pay)

	all, err := h.ops.ListPaymentOperations(h.ctx(), h.owner, h.propID, pay, h.listCmd(nil, 50, 0, false))
	if err != nil {
		t.Fatalf("default list: %v", err)
	}
	viewByDate := make(map[string]domain.OperationViewStatus, len(all))
	for _, item := range all {
		viewByDate[item.Operation.Date.Format(time.DateOnly)] = item.ViewStatus
	}
	// The two stored-planned rows before today read overdue; nothing else
	// does.
	if viewByDate["2026-08-12"] != domain.ViewStatusOverdue ||
		viewByDate["2026-08-19"] != domain.ViewStatusOverdue ||
		viewByDate[day27] != domain.ViewStatusPlanned ||
		viewByDate["2026-08-05"] != domain.ViewStatusPaid {
		t.Fatalf("view statuses drifted: %+v", viewByDate)
	}

	overdue := domain.ViewStatusOverdue
	debt, err := h.ops.ListPaymentOperations(h.ctx(), h.owner, h.propID, pay, h.listCmd(&overdue, 50, 0, false))
	if err != nil {
		t.Fatalf("overdue filter: %v", err)
	}
	if len(debt) != 2 {
		t.Fatalf("overdue = %d items, want the two past planned rows", len(debt))
	}

	planned := domain.ViewStatusPlanned
	standing, err := h.ops.ListPaymentOperations(h.ctx(), h.owner, h.propID, pay, h.listCmd(&planned, 50, 0, false))
	if err != nil {
		t.Fatalf("planned filter: %v", err)
	}
	if len(standing) != 2 {
		t.Fatalf("planned = %d items, want only the standing two", len(standing))
	}

	paidFilter := domain.ViewStatusPaid
	closed, err := h.ops.ListPaymentOperations(h.ctx(), h.owner, h.propID, pay, h.listCmd(&paidFilter, 50, 0, false))
	if err != nil {
		t.Fatalf("paid filter: %v", err)
	}
	if len(closed) != 3 {
		t.Fatalf("paid = %d items, want the three history rows", len(closed))
	}
}

func TestOperationsListing_PeriodFilterIsInclusive(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	pay := h.seedPayment(day22, `{"kind": "monthly", "dayOfMonth": 15}`, false)
	listingFixture(t, h, pay)

	from := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	cmd := paymentsapp.OperationsListQuery{DateFrom: &from, DateTo: &to, Limit: 50}
	window, err := h.ops.ListPaymentOperations(h.ctx(), h.owner, h.propID, pay, cmd)
	if err != nil {
		t.Fatalf("period filter: %v", err)
	}
	if len(window) != 4 {
		t.Fatalf("period = %d items, want 4 in [10th, 20th] inclusive", len(window))
	}
}

func TestOperationsListing_HidesMissingAndForeignRules(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	stranger := uuid.Must(uuid.NewV7())
	unknownRule := uuid.Must(uuid.NewV7())
	_, err := h.ops.ListPaymentOperations(h.ctx(), h.owner, h.propID, unknownRule, h.listCmd(nil, 50, 0, false))
	wantAppError(t, err, paymentsapp.ErrNotFound)

	_, err = h.ops.ListPropertyOperations(h.ctx(), stranger, h.propID, h.listCmd(nil, 50, 0, false))
	wantAppError(t, err, paymentsapp.ErrNotFound)
}

func TestSetFavorite_WritesFlagAcrossReadsAndBack(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	beforeOps := len(h.opsOf(t, created.ID))

	on, err := h.svc.SetPaymentFavorite(h.ctx(), h.owner, h.propID, created.ID, true)
	if err != nil {
		t.Fatalf("favorite on: %v", err)
	}
	rereadOn := favoriteReadback(t, h, created.ID)
	if !on.IsFavorite || !rereadOn {
		t.Fatalf("favorite-on readback = returned:%v stored:%v, want both true", on.IsFavorite, rereadOn)
	}

	_, err = h.svc.SetPaymentFavorite(h.ctx(), h.owner, h.propID, created.ID, true)
	if err != nil {
		t.Fatalf("idempotent repeat: %v", err)
	}
	off, err := h.svc.SetPaymentFavorite(h.ctx(), h.owner, h.propID, created.ID, false)
	if err != nil {
		t.Fatalf("favorite off: %v", err)
	}
	rereadOff := favoriteReadback(t, h, created.ID)
	if off.IsFavorite || rereadOff {
		t.Fatalf("favorite-off readback = returned:%v stored:%v, want both false", off.IsFavorite, rereadOff)
	}

	afterOps := len(h.opsOf(t, created.ID))
	if beforeOps != afterOps {
		t.Fatalf("operations moved under favorite toggles: %d -> %d", beforeOps, afterOps)
	}
}

func TestSetFavorite_AuditsEachToggleAsAnUpdate(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}

	wantActions := []string{actionPaymentCreated, actionPaymentUpdated, actionPaymentUpdated}
	for range 2 {
		if _, err := h.svc.SetPaymentFavorite(h.ctx(), h.owner, h.propID, created.ID, true); err != nil {
			t.Fatalf("favorite toggle: %v", err)
		}
	}
	got := h.auditActions(t, created.ID)
	if len(got) != len(wantActions) || got[0] != wantActions[0] || got[1] != actionPaymentUpdated {
		t.Fatalf("audit = %v, want [created updated updated] so far", got)
	}
}

// favoriteReadback loads the rule's current is_favorite through both read
// paths.
func favoriteReadback(t *testing.T, h *paymentsHarness, paymentID uuid.UUID) bool {
	t.Helper()
	got, err := h.svc.GetPayment(h.ctx(), h.owner, h.propID, paymentID)
	if err != nil {
		t.Fatalf("get payment: %v", err)
	}
	list, err := h.svc.ListPayments(h.ctx(), h.owner, h.propID, "")
	if err != nil {
		t.Fatalf("list payments: %v", err)
	}
	var listedIsFavorite bool
	for _, rule := range list {
		if rule.ID == paymentID {
			listedIsFavorite = rule.IsFavorite
		}
	}
	if got.IsFavorite != listedIsFavorite {
		t.Fatalf("read paths disagree: get=%v list=%v", got.IsFavorite, listedIsFavorite)
	}
	return got.IsFavorite
}

func TestOperationsRoleMatrix_FullAccessMemberActs(t *testing.T) {
	t.Parallel()
	member := uuid.Must(uuid.NewV7())
	full := newPaymentsHarnessWithPolicy(t, stubPropertyPolicy{role: sharedpolicy.RoleFullAccess}).withOwner("Europe/Moscow")
	full.seedActor(member)
	created, err := full.svc.CreatePayment(full.ctx(), member, full.propID, full.createCmd())
	if err != nil {
		t.Fatalf("full access create: %v", err)
	}

	paid, err := full.ops.PayOperation(full.ctx(), member, full.propID,
		oldestPlannedOperation(t, full, created.ID))
	if err != nil {
		t.Fatalf("full access pay: %v", err)
	}
	if paid.Operation.Status != domain.StatusPaid {
		full.t.Fatal("full access pay produced a non-paid operation")
	}
	if _, err := full.ops.ListPaymentOperations(full.ctx(), member, full.propID, created.ID, full.listCmd(nil, 50, 0, false)); err != nil {
		t.Fatalf("full access listing: %v", err)
	}
	if _, err := full.svc.SetPaymentFavorite(full.ctx(), member, full.propID, created.ID, true); err != nil {
		t.Fatalf("full access favorite: %v", err)
	}
}

func TestOperationsRoleMatrix_ViewerReadsButDoesNotAct(t *testing.T) {
	t.Parallel()
	viewer := newPaymentsHarnessWithPolicy(t, stubPropertyPolicy{role: sharedpolicy.RoleViewer}).withOwner("Europe/Moscow")
	seeding := viewer.seedPayment(day25, `{"kind": "daily"}`, false)
	viewer.runTick()

	_, err := viewer.ops.PayOperation(viewer.ctx(), viewer.owner, viewer.propID,
		oldestPlannedOperation(t, viewer, seeding))
	wantAppError(t, err, paymentsapp.ErrForbidden)

	_, err = viewer.svc.SetPaymentFavorite(viewer.ctx(), viewer.owner, viewer.propID, seeding, true)
	wantAppError(t, err, paymentsapp.ErrForbidden)

	lists, err := viewer.ops.ListPaymentOperations(
		viewer.ctx(), viewer.owner, viewer.propID, seeding, viewer.listCmd(nil, 50, 0, false))
	if err != nil {
		t.Fatalf("viewer listing: %v", err)
	}
	_ = lists
	if _, err := viewer.ops.ListPropertyOperations(viewer.ctx(), viewer.owner, viewer.propID, viewer.listCmd(nil, 50, 0, false)); err != nil {
		t.Fatalf("viewer property listing: %v", err)
	}
}

func TestOperationsRoleMatrix_StrangerStaysPrivate(t *testing.T) {
	t.Parallel()
	none := newPaymentsHarnessWithPolicy(t, stubPropertyPolicy{role: sharedpolicy.RoleNone}).withOwner("Europe/Moscow")
	nCreated := none.seedPayment(day25, `{"kind": "daily"}`, false)
	none.runTick()
	stranger := uuid.Must(uuid.NewV7())

	opID := oldestPlannedOperation(t, none, nCreated)
	_, err := none.ops.PayOperation(none.ctx(), stranger, none.propID, opID)
	wantAppError(t, err, paymentsapp.ErrNotFound)

	_, err = none.ops.ListPropertyOperations(none.ctx(), stranger, none.propID, none.listCmd(nil, 50, 0, false))
	wantAppError(t, err, paymentsapp.ErrNotFound)
}

func TestArchivedProperty_BlocksPayAndFavoriteButNotReads(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("archived fixture create: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`UPDATE properties SET status = 'archived' WHERE id = $1`, h.propID); err != nil {
		t.Fatalf("archive property: %v", err)
	}

	opID := oldestPlannedOperation(t, h, created.ID)
	_, err = h.ops.PayOperation(h.ctx(), h.owner, h.propID, opID)
	wantAppError(t, err, paymentsapp.ErrArchivedProperty)

	_, err = h.svc.SetPaymentFavorite(h.ctx(), h.owner, h.propID, created.ID, true)
	wantAppError(t, err, paymentsapp.ErrArchivedProperty)

	if _, err := h.ops.ListPropertyOperations(h.ctx(), h.owner, h.propID, h.listCmd(nil, 50, 0, false)); err != nil {
		t.Fatalf("listing on archived: %v", err)
	}
}

func TestPropertyOperationsListing_SpansRulesAndComputesOverdue(t *testing.T) {
	t.Parallel()
	// Kamchatka lives one day ahead at the anchor instant: today there is
	// already the 26th (ADR 0048).
	h := newPaymentsHarness(t).withOwner("Asia/Kamchatka")
	first := h.seedPayment(day22, `{"kind": "daily"}`, false)
	second := h.seedPayment(day22, `{"kind": "weekly", "weekdays": [1]}`, false)
	h.seedOperation(first, day24, opPlanned)
	h.seedOperation(first, day26, opPlanned)
	h.seedOperation(second, day24, opPaid)

	items, err := h.ops.ListPropertyOperations(h.ctx(), h.owner, h.propID, h.listCmd(nil, 50, 0, true))
	if err != nil {
		t.Fatalf("property listing: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("property listing = %d items, want the union of both rules", len(items))
	}

	overdue := domain.ViewStatusOverdue
	debt, err := h.ops.ListPropertyOperations(h.ctx(), h.owner, h.propID, h.listCmd(&overdue, 50, 0, false))
	if err != nil {
		t.Fatalf("debt listing: %v", err)
	}
	if len(debt) != 1 || debt[0].Operation.Date.Format(time.DateOnly) != day24 {
		t.Fatalf("kamchatka debt = %+v, want only the 24th (the 26th stands)",
			debt)
	}
}

// operationsAudit loads the audit actions recorded for one operation.
func operationsAudit(t *testing.T, h *paymentsHarness, entityID uuid.UUID) []string {
	t.Helper()
	rows, err := h.pool.Query(h.ctx(),
		`SELECT action FROM audit_log WHERE entity_type = 'operation' AND entity_id = $1 ORDER BY created_at, id`,
		entityID)
	if err != nil {
		t.Fatalf("query operation audit: %v", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var action string
		if err := rows.Scan(&action); err != nil {
			t.Fatalf("scan audit: %v", err)
		}
		out = append(out, action)
	}
	return out
}
