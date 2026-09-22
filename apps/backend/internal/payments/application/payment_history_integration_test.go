//go:build integration

package application_test

// The action journal of the payments mutations (карта #704, тикет #707,
// ADR 0061): the conveyor records one row per manual action inside the same
// transaction, the noise action (the favorite star) writes nothing, and a
// failing journal insert rolls the whole mutation back — the fail-safe
// canon: the action and its journal row share the transaction's fate.

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	auditpg "github.com/nambers/arenda-planform/apps/backend/internal/audit/adapters/postgres"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	paymentspg "github.com/nambers/arenda-planform/apps/backend/internal/payments/adapters/postgres"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	pgdb "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// cleaningSlug is a valid default-catalog expense category for the fixtures.
const cleaningSlug = "cleaning"

// journalActions reads the recorded actions of the property, oldest first.
func journalActions(t *testing.T, h *paymentsHarness) []string {
	t.Helper()
	rows, err := h.pool.Query(h.ctx(),
		`SELECT action FROM action_journal WHERE property_id = $1 ORDER BY created_at, id`,
		h.propID,
	)
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	defer rows.Close()
	var actions []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			t.Fatalf("scan journal: %v", err)
		}
		actions = append(actions, a)
	}
	return actions
}

func TestHistory_PaymentMutationsRecordRows(t *testing.T) {
	t.Parallel()

	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	if _, err := h.svc.PausePayment(h.ctx(), h.owner, h.propID, created.ID); err != nil {
		t.Fatalf("PausePayment: %v", err)
	}
	if _, err := h.svc.ResumePayment(h.ctx(), h.owner, h.propID, created.ID); err != nil {
		t.Fatalf("ResumePayment: %v", err)
	}
	// The favorite star is the documented noise exclusion (ADR 0061 §3).
	if _, err := h.svc.SetPaymentFavorite(h.ctx(), h.owner, h.propID, created.ID, true); err != nil {
		t.Fatalf("SetPaymentFavorite: %v", err)
	}
	if err := h.svc.DeletePayment(h.ctx(), h.owner, h.propID, created.ID, true); err != nil {
		t.Fatalf("DeletePayment: %v", err)
	}

	got := journalActions(t, h)
	want := []string{"payment.created", "payment.paused", "payment.resumed", "payment.deleted"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("journal actions = %v, want %v", got, want)
	}
}

func TestHistory_OperationPaidAndDeleted(t *testing.T) {
	t.Parallel()

	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	if _, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd()); err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	ops, err := h.ops.ListPropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsListQuery{Today: h.clock.Now()})
	if err != nil {
		t.Fatalf("list operations: %v", err)
	}
	if len(ops) == 0 {
		t.Fatal("the fresh rule materialized no operations to pay")
	}
	if _, err := h.ops.PayOperation(h.ctx(), h.owner, h.propID, ops[0].Operation.ID); err != nil {
		t.Fatalf("PayOperation: %v", err)
	}
	if err := h.ops.DeleteOperation(h.ctx(), h.owner, h.propID, ops[0].Operation.ID); err != nil {
		t.Fatalf("DeleteOperation: %v", err)
	}

	got := journalActions(t, h)
	want := []string{"payment.created", "operation.paid", "operation.deleted"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("journal actions = %v, want %v", got, want)
	}
}

func TestHistory_ManualOperationCarriesSnapshots(t *testing.T) {
	t.Parallel()

	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	cmd := paymentsapp.CreateOperationCommand{
		Type:          "expense",
		Title:         "Химчистка",
		AmountKopecks: 1234500,
		CategorySlug:  cleaningSlug,
	}
	if _, err := h.ops.CreateOperation(h.ctx(), h.owner, h.propID, cmd); err != nil {
		t.Fatalf("CreateOperation: %v", err)
	}

	var searchable, contextJSON string
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT searchable, context::text
		 FROM action_journal WHERE property_id = $1 AND action = 'operation.created'`,
		h.propID,
	).Scan(&searchable, &contextJSON); err != nil {
		t.Fatalf("read journal row: %v", err)
	}
	// The harness owner has no name: the snapshot degrades to the masked
	// phone — the access canon (never the raw phone).
	if !strings.Contains(searchable, "Химчистка") {
		t.Errorf("searchable = %q, want the title", searchable)
	}
	if !strings.Contains(searchable, "+7*") {
		t.Errorf("searchable = %q, want the masked actor phone", searchable)
	}
	if !strings.Contains(contextJSON, "1234500") {
		t.Errorf("context = %s, want the kopecks amount (ADR 0061 §5)", contextJSON)
	}
}

func TestHistory_FailingInsertRollsMutationBack(t *testing.T) {
	t.Parallel()

	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	// The same wiring as the harness, but the journal recorder always fails —
	// the mutation must roll back with it (fail-safe, ADR 0061 §3).
	logger := slog.New(slog.DiscardHandler)
	clk := h.clock
	factory := paymentsapp.NewTxStoreFactory(
		paymentspg.NewTickStore(h.pool),
		paymentspg.NewPaymentStore(h.pool),
		paymentspg.NewOperationStore(h.pool),
		paymentspg.NewPropertyStore(h.pool),
		paymentspg.NewGlobalPaymentStore(h.pool),
		auditapp.NewService(auditpg.NewWriter(h.pool), clk),
		failingHistory{},
		pgdb.NewUoW(h.pool, logger),
	)
	calendar := paymentspg.NewOwnerCalendar(h.pool, clk)
	svc := paymentsapp.NewPaymentService(factory, calendar, stubPropertyPolicy{role: sharedpolicy.RoleOwner})

	if _, err := svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd()); !errors.Is(err, errHistoryDown) {
		t.Fatalf("CreatePayment error = %v, want errHistoryDown", err)
	}

	var rules, journal int
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT count(*) FROM payments WHERE property_id = $1`, h.propID,
	).Scan(&rules); err != nil {
		t.Fatalf("count rules: %v", err)
	}
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT count(*) FROM action_journal WHERE property_id = $1`, h.propID,
	).Scan(&journal); err != nil {
		t.Fatalf("count journal: %v", err)
	}
	if rules != 0 || journal != 0 {
		t.Errorf("rows after the failed mutation: payments=%d journal=%d, want 0/0 (rolled back)", rules, journal)
	}
}

// failingHistory is the recorder double whose every insert fails.
type failingHistory struct{}

var errHistoryDown = errors.New("history store down")

func (failingHistory) Record(context.Context, historydomain.Entry) error { return errHistoryDown }

func (failingHistory) WithTx(transaction.Tx) historyapp.Recorder { return failingHistory{} }
