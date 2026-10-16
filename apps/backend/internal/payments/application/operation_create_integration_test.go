//go:build integration

package application_test

// The integration tests of the manual operation creation (ticket #569): the
// one-off fact born paid on the owner's today, its category snapshot, the
// in-transaction audit, the tick verdict and the conveyor sentinels — against
// real PostgreSQL through the shared harness.

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

const (
	actionOperationCreatedStr = "operation.created"
	// TestIntegrationLabelUtilities is the catalog label of the utilities
	// slug — the category snapshot the manual-creation tests freeze.
	testIntegrationLabelUtilities = "Коммунальные услуги"
)

// createOpCmd is the happy-path command every case mutates.
func createOpCmd() paymentsapp.CreateOperationCommand {
	return paymentsapp.CreateOperationCommand{
		Type:          domain.TypeExpense,
		Title:         "Ремонт крана",
		AmountKopecks: 150000,
		CategorySlug:  testIntegrationSlugUtilities,
	}
}

func TestCreateOperation_ManualFactBornPaidOnOwnersToday(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	item, err := h.ops.CreateOperation(h.ctx(), h.owner, h.propID, createOpCmd())
	if err != nil {
		t.Fatalf("create manual operation: %v", err)
	}

	t.Run("response carries the born-paid manual shape", func(t *testing.T) {
		t.Parallel()
		assertBornPaidResponse(t, item)
	})
	t.Run("the stored row carries NULL payment link and form", func(t *testing.T) {
		t.Parallel()
		assertStoredManualRow(t, h, item.Operation.ID)
	})
	t.Run("the fact is a first-class paid row for the reads", func(t *testing.T) {
		t.Parallel()
		paid := domain.ViewStatusPaid
		history, err := h.ops.ListPropertyOperations(h.ctx(), h.owner, h.propID, h.listCmd(&paid, 50, 0, false))
		if err != nil || len(history) != 1 || history[0].Operation.ID != item.Operation.ID {
			t.Fatalf("paid history = %d items (%v), want the new fact", len(history), err)
		}
	})
	t.Run("the creation audits as operation.created", func(t *testing.T) {
		t.Parallel()
		if got := operationsAudit(t, h, item.Operation.ID); len(got) != 1 || got[0] != actionOperationCreatedStr {
			t.Fatalf("audit = %v, want [%s]", got, actionOperationCreatedStr)
		}
	})
}

// assertBornPaidResponse checks the returned fact: the manual origin, the
// paid status on the Moscow today, no rule behind it and the frozen category
// snapshot.
func assertBornPaidResponse(t *testing.T, item paymentsapp.OperationListItem) {
	t.Helper()
	op := item.Operation
	if op.Origin != domain.OriginManual {
		t.Errorf("origin = %s, want manual", op.Origin)
	}
	if op.Status != domain.StatusPaid || item.ViewStatus != domain.ViewStatusPaid {
		t.Errorf("status/view = %s/%s, want paid/paid", op.Status, item.ViewStatus)
	}
	if got := op.Date.Format(time.DateOnly); got != day25 {
		t.Errorf("date = %s, want the Moscow today (%s)", got, day25)
	}
	if op.PaidDate == nil || op.PaidDate.Format(time.DateOnly) != day25 {
		t.Errorf("paid_date = %v, want the Moscow today (%s)", op.PaidDate, day25)
	}
	if op.PaymentID != nil {
		t.Errorf("payment_id = %s, want nil — no rule behind a manual fact", op.PaymentID)
	}
	if op.Title != "Ремонт крана" || op.AmountKopecks != 150000 || op.Type != domain.TypeExpense {
		t.Errorf("payload drifted: %+v", op)
	}
	// The category froze as the snapshot: the catalog label for the slug.
	slugOK := op.CategorySlug != nil && *op.CategorySlug == testIntegrationSlugUtilities
	if !slugOK || op.CategoryLabel != testIntegrationLabelUtilities {
		t.Errorf("category snapshot = %v/%q, want %s/%s",
			op.CategorySlug, op.CategoryLabel, testIntegrationSlugUtilities, testIntegrationLabelUtilities)
	}
}

// assertStoredManualRow loads the stored row and checks the shape the reads
// see: NULL payment link, the manual paid origin and the label.
func assertStoredManualRow(t *testing.T, h *paymentsHarness, opID uuid.UUID) {
	t.Helper()
	var paymentID *string
	var origin, status, label string
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT payment_id::text, origin, status, category_label
		 FROM operations WHERE id = $1`, opID,
	).Scan(&paymentID, &origin, &status, &label); err != nil {
		t.Fatalf("load stored row: %v", err)
	}
	if paymentID != nil {
		t.Errorf("stored payment_id = %v, want NULL", paymentID)
	}
	if origin != "manual" || status != "paid" || label != testIntegrationLabelUtilities {
		t.Errorf("stored origin/status/label = %s/%s/%q, want manual/paid/%s", origin, status, label, testIntegrationLabelUtilities)
	}
}

func TestCreateOperation_TitleIsTrimmedLikeTheRuleS(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	cmd := createOpCmd()
	cmd.Title = "   Сантехник  "
	item, err := h.ops.CreateOperation(h.ctx(), h.owner, h.propID, cmd)
	if err != nil {
		t.Fatalf("create with padded title: %v", err)
	}
	if item.Operation.Title != "Сантехник" {
		t.Errorf("title = %q, want trimmed %q", item.Operation.Title, "Сантехник")
	}
}

func TestCreateOperation_PaidDateFollowsOwnerTimezone(t *testing.T) {
	t.Parallel()
	// At the anchor instant (2026-08-25T20:00Z) Kamchatka already lives on
	// its 26th while Moscow still counts the 25th (ADR 0048).
	h := newPaymentsHarness(t).withOwner("Asia/Kamchatka")

	item, err := h.ops.CreateOperation(h.ctx(), h.owner, h.propID, createOpCmd())
	if err != nil {
		t.Fatalf("create manual operation: %v", err)
	}
	if got := item.Operation.PaidDate.Format(time.DateOnly); got != day26 {
		t.Fatalf("paid_date = %s, want the Kamchatka today (%s)", got, day26)
	}
}

func TestCreateOperation_MutationRunsTheTick(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	// A rule never ticked: its due occurrences do not exist yet.
	pay := h.seedPayment(day22, `{"kind": "monthly", "dayOfMonth": 15}`, false)

	if _, err := h.ops.CreateOperation(h.ctx(), h.owner, h.propID, createOpCmd()); err != nil {
		t.Fatalf("create manual operation: %v", err)
	}

	// The in-mutation tick materialized the rule's due occurrences on the
	// same transaction's verdict.
	var count int
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT count(*) FROM operations WHERE payment_id = $1`, pay,
	).Scan(&count); err != nil {
		t.Fatalf("count rule operations: %v", err)
	}
	if count == 0 {
		t.Fatal("the tick did not run inside the creation mutation")
	}
}

func TestCreateOperation_InvalidCommandsRejected(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	cases := []struct {
		name   string
		mutate func(cmd *paymentsapp.CreateOperationCommand)
	}{
		{"bad type enum", func(c *paymentsapp.CreateOperationCommand) { c.Type = domain.PaymentType("profit") }},
		{"blank title", func(c *paymentsapp.CreateOperationCommand) { c.Title = "   " }},
		{
			"title over 255 characters",
			func(c *paymentsapp.CreateOperationCommand) { c.Title = strings.Repeat("а", 256) },
		},
		{"zero amount", func(c *paymentsapp.CreateOperationCommand) { c.AmountKopecks = 0 }},
		{"amount over 10^9", func(c *paymentsapp.CreateOperationCommand) { c.AmountKopecks = 1_000_000_001 }},
		{"unknown category slug", func(c *paymentsapp.CreateOperationCommand) { c.CategorySlug = "not-a-catalog-slug" }},
		{"empty category slug", func(c *paymentsapp.CreateOperationCommand) { c.CategorySlug = "" }},
	}
	// Sequential on purpose: every command reaches the validator through the
	// property serialization lock, so parallel cases would queue on one row.
	for _, tc := range cases {
		cmd := createOpCmd()
		tc.mutate(&cmd)
		_, err := h.ops.CreateOperation(h.ctx(), h.owner, h.propID, cmd)
		if !errors.Is(err, paymentsapp.ErrInvalidInput) {
			t.Errorf("%s: err = %v, want ErrInvalidInput", tc.name, err)
		}
	}

	// Nothing leaked into the table.
	var count int
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT count(*) FROM operations WHERE property_id = $1`, h.propID,
	).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("rejected commands leaked %d rows", count)
	}
}

func TestCreateOperation_RoleMatrixAndArchived(t *testing.T) {
	t.Parallel()
	viewer := newPaymentsHarnessWithPolicy(t, stubPropertyPolicy{role: sharedpolicy.RoleViewer}).withOwner("Europe/Moscow")
	_, err := viewer.ops.CreateOperation(viewer.ctx(), viewer.owner, viewer.propID, createOpCmd())
	if !errors.Is(err, paymentsapp.ErrForbidden) {
		t.Fatalf("viewer create = %v, want ErrForbidden", err)
	}

	none := newPaymentsHarnessWithPolicy(t, stubPropertyPolicy{role: sharedpolicy.RoleNone}).withOwner("Europe/Moscow")
	stranger := uuid.Must(uuid.NewV7())
	_, err = none.ops.CreateOperation(none.ctx(), stranger, none.propID, createOpCmd())
	if !errors.Is(err, paymentsapp.ErrNotFound) {
		t.Fatalf("stranger create = %v, want the privacy ErrNotFound", err)
	}

	archived := newPaymentsHarness(t).withOwner("Europe/Moscow")
	if _, err := archived.pool.Exec(archived.ctx(),
		`UPDATE properties SET status = 'archived' WHERE id = $1`, archived.propID); err != nil {
		t.Fatalf("archive property: %v", err)
	}
	_, err = archived.ops.CreateOperation(archived.ctx(), archived.owner, archived.propID, createOpCmd())
	if !errors.Is(err, paymentsapp.ErrArchivedProperty) {
		t.Fatalf("archived create = %v, want ErrArchivedProperty", err)
	}
}

func TestCreateOperation_FullAccessMemberCreates(t *testing.T) {
	t.Parallel()
	member := uuid.Must(uuid.NewV7())
	full := newPaymentsHarnessWithPolicy(t, stubPropertyPolicy{role: sharedpolicy.RoleFullAccess}).withOwner("Europe/Moscow")
	full.seedActor(member)

	item, err := full.ops.CreateOperation(full.ctx(), member, full.propID, createOpCmd())
	if err != nil {
		t.Fatalf("full access create: %v", err)
	}
	if item.Operation.Status != domain.StatusPaid {
		t.Fatal("full access create produced a non-paid operation")
	}
}

// wireDate is the UTC-midnight shape the transport folds into the command:
// the generated openapi_types.Date parses the full-date at UTC midnight, and
// the owner's today from the calendar is the same shape (ADR 0048).
func wireDate(day string) time.Time {
	d, err := time.Parse(time.DateOnly, day)
	if err != nil {
		panic(err)
	}
	return d
}

// TestCreateOperation_BackdatedFactLandsOnChosenDate covers the backdating
// core (ticket #1392): the client-chosen past date lands in BOTH date
// columns of the born-paid fact — the response echo and the stored row —
// while created_at stays the record moment.
func TestCreateOperation_BackdatedFactLandsOnChosenDate(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	cmd := createOpCmd()
	chosen := wireDate(day24) // The day before the Moscow today (day25).
	cmd.Date = &chosen
	item, err := h.ops.CreateOperation(h.ctx(), h.owner, h.propID, cmd)
	if err != nil {
		t.Fatalf("create backdated operation: %v", err)
	}

	t.Run("the response carries the chosen date in both columns", func(t *testing.T) {
		t.Parallel()
		op := item.Operation
		if op.Status != domain.StatusPaid || item.ViewStatus != domain.ViewStatusPaid {
			t.Errorf("status/view = %s/%s, want paid/paid", op.Status, item.ViewStatus)
		}
		if got := op.Date.Format(time.DateOnly); got != day24 {
			t.Errorf("date = %s, want the chosen %s", got, day24)
		}
		if op.PaidDate == nil || op.PaidDate.Format(time.DateOnly) != "2026-08-24" {
			t.Errorf("paid_date = %v, want the chosen %s", op.PaidDate, day24)
		}
		if !op.UpdatedAt.After(chosen) {
			t.Errorf("updated_at = %v, want the record moment after the chosen day", op.UpdatedAt)
		}
	})
	t.Run("the stored row carries the chosen date in both columns", func(t *testing.T) {
		t.Parallel()
		var date, paidDate time.Time
		if err := h.pool.QueryRow(h.ctx(),
			`SELECT date, paid_date FROM operations WHERE id = $1`, item.Operation.ID,
		).Scan(&date, &paidDate); err != nil {
			t.Fatalf("load stored row: %v", err)
		}
		if got := date.Format(time.DateOnly); got != day24 {
			t.Errorf("stored date = %s, want %s", got, day24)
		}
		if got := paidDate.Format(time.DateOnly); got != day24 {
			t.Errorf("stored paid_date = %s, want %s", got, day24)
		}
	})
}

// TestCreateOperation_ClientDateValidation covers the future boundary on the
// owner's calendar (ticket #1392): tomorrow and far-future are the future
// sentinel, the explicit today is accepted, and rejected commands leak no
// rows.
func TestCreateOperation_ClientDateValidation(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	// The leak assertion rides t.Cleanup: parallel subtests resume only when
	// the parent function has returned, so a tail-of-parent check would read
	// the table too early.
	t.Cleanup(func() {
		// Only the accepted explicit-today fact leaked into the table.
		var count int
		if err := h.pool.QueryRow(h.ctx(),
			`SELECT count(*) FROM operations WHERE property_id = $1`, h.propID,
		).Scan(&count); err != nil {
			t.Errorf("count rows: %v", err)
			return
		}
		if count != 1 {
			t.Errorf("rejected commands leaked rows: %d stored, want 1", count)
		}
	})

	t.Run("tomorrow by the owner's calendar is rejected", func(t *testing.T) {
		t.Parallel()
		cmd := createOpCmd()
		tomorrow := wireDate(day26)
		cmd.Date = &tomorrow
		if _, err := h.ops.CreateOperation(h.ctx(), h.owner, h.propID, cmd); !errors.Is(err, paymentsapp.ErrOperationDateInFuture) {
			t.Errorf("err = %v, want ErrOperationDateInFuture", err)
		}
	})
	t.Run("a year ahead is rejected", func(t *testing.T) {
		t.Parallel()
		cmd := createOpCmd()
		yearAhead := wireDate("2027-08-25")
		cmd.Date = &yearAhead
		if _, err := h.ops.CreateOperation(h.ctx(), h.owner, h.propID, cmd); !errors.Is(err, paymentsapp.ErrOperationDateInFuture) {
			t.Errorf("err = %v, want ErrOperationDateInFuture", err)
		}
	})
	t.Run("the explicit today is accepted", func(t *testing.T) {
		t.Parallel()
		cmd := createOpCmd()
		today := wireDate(day25)
		cmd.Date = &today
		item, err := h.ops.CreateOperation(h.ctx(), h.owner, h.propID, cmd)
		if err != nil {
			t.Fatalf("create with explicit today: %v", err)
		}
		if got := item.Operation.Date.Format(time.DateOnly); got != day25 {
			t.Errorf("date = %s, want the today %s", got, day25)
		}
	})
}

// TestCreateOperation_FutureBoundaryFollowsOwnerTimezone pins the boundary to
// the data owner's calendar (ADR 0048 п.4): at the anchor instant Kamchatka
// already lives on its 26th, so its today is accepted even though Moscow
// would call it tomorrow — and Kamchatka's tomorrow is rejected.
func TestCreateOperation_FutureBoundaryFollowsOwnerTimezone(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Asia/Kamchatka")

	t.Run("the Kamchatka today is accepted", func(t *testing.T) {
		t.Parallel()
		cmd := createOpCmd()
		kamchatkaToday := wireDate(day26)
		cmd.Date = &kamchatkaToday
		if _, err := h.ops.CreateOperation(h.ctx(), h.owner, h.propID, cmd); err != nil {
			t.Fatalf("create on the Kamchatka today: %v", err)
		}
	})
	t.Run("the Kamchatka tomorrow is rejected", func(t *testing.T) {
		t.Parallel()
		cmd := createOpCmd()
		kamchatkaTomorrow := wireDate("2026-08-27")
		cmd.Date = &kamchatkaTomorrow
		if _, err := h.ops.CreateOperation(h.ctx(), h.owner, h.propID, cmd); !errors.Is(err, paymentsapp.ErrOperationDateInFuture) {
			t.Errorf("err = %v, want ErrOperationDateInFuture", err)
		}
	})
}

// TestCreateOperation_BackdatedFactReadsByPaidDate is research-checklist
// грабля №3 (ticket #1391): the feeds read the listing's date key, so the
// backdated fact must surface in the period of its paid_date — not of the
// record day.
func TestCreateOperation_BackdatedFactReadsByPaidDate(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	cmd := createOpCmd()
	chosen := wireDate("2026-08-24")
	cmd.Date = &chosen
	item, err := h.ops.CreateOperation(h.ctx(), h.owner, h.propID, cmd)
	if err != nil {
		t.Fatalf("create backdated operation: %v", err)
	}

	// The paid-date window of the chosen day finds it; the record day's
	// window does not — the fact lives on its business date.
	from, to := wireDate(day24), wireDate(day24)
	inWindow, err := h.ops.ListPropertyOperations(h.ctx(), h.owner, h.propID, paymentsapp.OperationsListQuery{
		Sort:     paymentsapp.SortByPaidDate,
		DateFrom: &from,
		DateTo:   &to,
		Limit:    50,
	})
	if err != nil {
		t.Fatalf("list in the chosen-day window: %v", err)
	}
	if len(inWindow) != 1 || inWindow[0].Operation.ID != item.Operation.ID {
		t.Fatalf("paid_date window of %s = %d items, want the backdated fact", day24, len(inWindow))
	}

	laterFrom, laterTo := wireDate(day25), wireDate(day26)
	outOfWindow, err := h.ops.ListPropertyOperations(h.ctx(), h.owner, h.propID, paymentsapp.OperationsListQuery{
		Sort:     paymentsapp.SortByPaidDate,
		DateFrom: &laterFrom,
		DateTo:   &laterTo,
		Limit:    50,
	})
	if err != nil {
		t.Fatalf("list in the record-day window: %v", err)
	}
	if len(outOfWindow) != 0 {
		t.Fatalf("record-day window returned %d items, want none", len(outOfWindow))
	}
}
