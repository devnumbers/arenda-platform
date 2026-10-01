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
