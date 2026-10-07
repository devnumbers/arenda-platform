//go:build integration

package application_test

// The operation page's live-presentation tests (ticket #1190, решение
// владельца): the detail read returns the payment rule's current title and
// category for rule-born operations — the page always shows the actual
// icon/name/category — while the listings keep the snapshots frozen at
// materialization (the «История платежа» rows stay honest) and a manual or
// orphaned operation keeps its own.

import (
	"testing"

	"github.com/google/uuid"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
)

// seededRuleTitle is the rule/operation title the harness seeds: the detail
// tests drift the rule away from it and pin the frozen rows on it.
const seededRuleTitle = "ЖКУ"

// driftRulePresentation moves the rule away from the seeded snapshots: the
// title 'ЖКУ' becomes 'Аренда квартиры' and the utilities slug is rebound to
// the rent slug (the live catalog label «Арендная плата»).
func driftRulePresentation(t *testing.T, h *paymentsHarness, pay uuid.UUID) {
	t.Helper()
	if _, err := h.pool.Exec(h.ctx(),
		`UPDATE payments SET title = 'Аренда квартиры', category_slug = 'rent' WHERE id = $1`, pay,
	); err != nil {
		t.Fatalf("drift rule: %v", err)
	}
}

func TestGetOperation_PaymentOperationReadsLiveRulePresentation(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	pay := h.seedPayment(day22, `{"kind": "monthly", "dayOfMonth": 15}`, false)
	opID := h.seedOperation(pay, day25, opPaid) // Snapshot 'ЖКУ'/'Коммунальные услуги'/utilities.

	driftRulePresentation(t, h, pay)

	detail, err := h.ops.GetOperation(h.ctx(), h.owner, h.propID, opID)
	if err != nil {
		t.Fatalf("get operation: %v", err)
	}
	if detail.Operation.Title != "Аренда квартиры" {
		t.Fatalf("detail title = %q, want the rule's live title", detail.Operation.Title)
	}
	if detail.Operation.CategoryLabel != "Арендная плата" {
		t.Fatalf("detail category label = %q, want the live catalog label", detail.Operation.CategoryLabel)
	}
	if detail.Operation.CategorySlug == nil || *detail.Operation.CategorySlug != testIntegrationSlugRent {
		t.Fatalf("detail category slug = %v, want the live rent slug", detail.Operation.CategorySlug)
	}
	if detail.ViewStatus != domain.ViewStatusPaid {
		t.Fatalf("view status = %s, want paid — the live presentation never touches the schedule math",
			detail.ViewStatus)
	}
}

// The listings stay on the materialization snapshots (решение владельца
// #1190): only the operation page reads live.
func TestListOperations_KeepFrozenSnapshots(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	pay := h.seedPayment(day22, `{"kind": "monthly", "dayOfMonth": 15}`, false)
	h.seedOperation(pay, day25, opPaid)

	driftRulePresentation(t, h, pay)

	byPayment, err := h.ops.ListPaymentOperations(h.ctx(), h.owner, h.propID, pay, h.listCmd(nil, 50, 0, false))
	if err != nil || len(byPayment) != 1 {
		t.Fatalf("payment listing = %v items (%v), want one", len(byPayment), err)
	}
	byProperty, err := h.ops.ListPropertyOperations(h.ctx(), h.owner, h.propID, h.listCmd(nil, 50, 0, false))
	if err != nil || len(byProperty) != 1 {
		t.Fatalf("property listing = %v items (%v), want one", len(byProperty), err)
	}
	for _, rows := range [][]paymentsapp.OperationListItem{byPayment, byProperty} {
		got := rows[0].Operation
		if got.Title != seededRuleTitle || got.CategoryLabel != testIntegrationLabelUtilities ||
			got.CategorySlug == nil || *got.CategorySlug != testIntegrationSlugUtilities {
			t.Fatalf("listing row = %q/%q/%v, want the frozen snapshots ЖКУ/Коммунальные услуги/utilities",
				got.Title, got.CategoryLabel, got.CategorySlug)
		}
	}
}

func TestGetOperation_UserCategoryReadsLiveName(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	pay := h.seedPayment(day22, `{"kind": "monthly", "dayOfMonth": 15}`, false)
	opID := h.seedOperation(pay, day25, opPaid) // Snapshot slug utilities.

	catID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO payment_categories (id, owner_id, name) VALUES ($1, $2, 'Моя категория')`,
		catID, h.owner,
	); err != nil {
		t.Fatalf("seed user category: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`UPDATE payments SET category_slug = NULL, user_category_id = $1 WHERE id = $2`, catID, pay,
	); err != nil {
		t.Fatalf("rebind rule category: %v", err)
	}

	detail, err := h.ops.GetOperation(h.ctx(), h.owner, h.propID, opID)
	if err != nil {
		t.Fatalf("get operation: %v", err)
	}
	if detail.Operation.CategoryLabel != "Моя категория" {
		t.Fatalf("detail category label = %q, want the user category's live name", detail.Operation.CategoryLabel)
	}
	if detail.Operation.CategorySlug != nil {
		t.Fatalf("detail category slug = %v, want nil for a user category", detail.Operation.CategorySlug)
	}

	// The category renames — the next detail read follows it.
	if _, err := h.pool.Exec(h.ctx(),
		`UPDATE payment_categories SET name = 'Категория №2' WHERE id = $1`, catID,
	); err != nil {
		t.Fatalf("rename user category: %v", err)
	}
	renamed, err := h.ops.GetOperation(h.ctx(), h.owner, h.propID, opID)
	if err != nil {
		t.Fatalf("get operation after rename: %v", err)
	}
	if renamed.Operation.CategoryLabel != "Категория №2" {
		t.Fatalf("detail category label = %q, want the renamed user category", renamed.Operation.CategoryLabel)
	}
}

// A manual operation has no rule behind it and keeps its own data; the
// «платёж удалён» row (origin 'payment', payment_id nulled by the rule
// deletion) keeps its frozen snapshots even while a rule still exists on the
// property.
func TestGetOperation_ManualAndOrphanedRowsKeepTheirSnapshots(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	created, err := h.ops.CreateOperation(h.ctx(), h.owner, h.propID, paymentsapp.CreateOperationCommand{
		Type:          domain.TypeExpense,
		Title:         "Прочее",
		AmountKopecks: 12345,
		CategorySlug:  "other",
	})
	if err != nil {
		t.Fatalf("create manual: %v", err)
	}
	manual, err := h.ops.GetOperation(h.ctx(), h.owner, h.propID, created.Operation.ID)
	if err != nil {
		t.Fatalf("get manual: %v", err)
	}
	if manual.Operation.Title != "Прочее" || manual.Operation.CategoryLabel != "Другое" {
		t.Fatalf("manual detail = %q/%q, want its own snapshot Прочее/Другое",
			manual.Operation.Title, manual.Operation.CategoryLabel)
	}

	pay := h.seedPayment(day22, `{"kind": "monthly", "dayOfMonth": 15}`, false)
	orphanID := h.seedOperation(pay, day25, opPaid)
	if _, err := h.pool.Exec(h.ctx(),
		`UPDATE operations SET payment_id = NULL WHERE id = $1`, orphanID,
	); err != nil {
		t.Fatalf("orphan operation: %v", err)
	}
	// The rule exists and drifts — the orphaned row must not follow it.
	driftRulePresentation(t, h, pay)

	orphan, err := h.ops.GetOperation(h.ctx(), h.owner, h.propID, orphanID)
	if err != nil {
		t.Fatalf("get orphaned operation: %v", err)
	}
	if orphan.Operation.Title != seededRuleTitle || orphan.Operation.CategoryLabel != testIntegrationLabelUtilities ||
		orphan.Operation.CategorySlug == nil || *orphan.Operation.CategorySlug != testIntegrationSlugUtilities {
		t.Fatalf("orphan detail = %q/%q/%v, want the frozen snapshots ЖКУ/Коммунальные услуги/utilities",
			orphan.Operation.Title, orphan.Operation.CategoryLabel, orphan.Operation.CategorySlug)
	}
}
