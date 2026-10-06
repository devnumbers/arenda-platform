//go:build integration

package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

func TestPaymentCRUD_FullLifecycle(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	// Since is the owner's today (Moscow), never a client value.
	if created.Since.Format(time.DateOnly) != day25 {
		t.Fatalf("since = %s, want %s — the server sets the owner's today", created.Since.Format(time.DateOnly), day25)
	}
	if created.OwnerID != h.owner {
		t.Fatalf("owner_id = %s, want the property owner (denormalized)", created.OwnerID)
	}
	// The in-transaction tick materialized today's occurrence and stood the
	// single future planned.
	ops := opStatuses(h.opsOf(t, created.ID))
	if ops[day25] != opPlanned || ops[day26] != opPlanned {
		t.Fatalf("after create: today=%q tomorrow=%q, want planned/planned", ops[day25], ops[day26])
	}
	if len(ops) != 2 {
		t.Fatalf("after create: %d operations, want 2 (due today + one future)", len(ops))
	}

	assertPaymentReadBack(t, h, created)

	if err := h.svc.DeletePayment(h.ctx(), h.owner, h.propID, created.ID, true); err != nil {
		t.Fatalf("delete payment: %v", err)
	}
	if _, err := h.svc.GetPayment(h.ctx(), h.owner, h.propID, created.ID); !errors.Is(err, paymentsapp.ErrNotFound) {
		t.Fatalf("get after delete = %v, want ErrNotFound", err)
	}
	actions := h.auditActions(t, created.ID)
	want := []string{actionPaymentCreated, actionPaymentDeleted}
	if len(actions) != len(want) {
		t.Fatalf("audit actions = %v, want %v", actions, want)
	}
	for i, action := range want {
		if actions[i] != action {
			t.Fatalf("audit actions = %v, want %v", actions, want)
		}
	}
}

// assertPaymentReadBack checks the read side: the single rule reads back
// whole (with its category reference) and lists first in creation order.
func assertPaymentReadBack(t *testing.T, h *paymentsHarness, created domain.Payment) {
	t.Helper()
	got, err := h.svc.GetPayment(h.ctx(), h.owner, h.propID, created.ID)
	if err != nil {
		t.Fatalf("get payment: %v", err)
	}
	if got.Title != "Аренда" || got.Category.SlugString() != testIntegrationSlugRent {
		t.Fatalf("get payment = %+v", got)
	}
	list, err := h.svc.ListPayments(h.ctx(), h.owner, h.propID, "")
	if err != nil {
		t.Fatalf("list payments: %v", err)
	}
	if len(list) != 1 || list[0].Payment.ID != created.ID {
		t.Fatalf("list = %d items, want the created one", len(list))
	}
}

func TestPaymentUpdate_RebuildsScheduleAndAudits(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}

	// Partial update: the amount grows and the schedule moves to monthly on
	// the 10th. The materialized operations keep their snapshots; the future
	// planned is rebuilt by the in-transaction tick.
	newTitle := "Аренда 2027"
	newAmount := int64(6000000)
	recurrence := domain.Recurrence{}
	{
		monthly, err := domain.NewMonthlyRecurrence([]int{10}, false)
		if err != nil {
			t.Fatalf("fixture recurrence: %v", err)
		}
		recurrence = monthly
	}
	updated, err := h.svc.UpdatePayment(h.ctx(), h.owner, h.propID, created.ID, paymentsapp.UpdatePaymentCommand{
		Title:         &newTitle,
		AmountKopecks: &newAmount,
		Recurrence:    &recurrence,
	})
	if err != nil {
		t.Fatalf("update payment: %v", err)
	}
	if updated.Title != newTitle || updated.AmountKopecks != newAmount {
		t.Fatalf("updated = %+v", updated)
	}
	if updated.Since.Format(time.DateOnly) != day25 {
		t.Fatalf("since changed by update: %s", updated.Since.Format(time.DateOnly))
	}
	ops := opStatuses(h.opsOf(t, created.ID))
	if ops["2026-09-10"] != opPlanned {
		t.Fatalf("after update: Sep 10 = %q, want planned (rebuilt future)", ops["2026-09-10"])
	}
	if _, still := ops[day26]; still {
		t.Fatalf("after update: the stale daily future planned %s must be gone", day26)
	}

	// Delete with the debt kept (the default): future planned always goes,
	// paid history stays marked «платёж удалён».
	if err := h.svc.DeletePayment(h.ctx(), h.owner, h.propID, created.ID, true); err != nil {
		t.Fatalf("delete payment: %v", err)
	}
	if _, err := h.svc.GetPayment(h.ctx(), h.owner, h.propID, created.ID); !errors.Is(err, paymentsapp.ErrNotFound) {
		t.Fatalf("get after delete = %v, want ErrNotFound", err)
	}

	actions := h.auditActions(t, created.ID)
	want := []string{actionPaymentCreated, actionPaymentUpdated, actionPaymentDeleted}
	if len(actions) != len(want) {
		t.Fatalf("audit actions = %v, want %v", actions, want)
	}
	for i, action := range want {
		if actions[i] != action {
			t.Fatalf("audit actions = %v, want %v", actions, want)
		}
	}
}

func TestCreatePayment_RejectsEndDateBeforeToday(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	past := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	cmd := h.createCmd()
	cmd.EndDate = &past
	if _, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, cmd); !errors.Is(err, paymentsapp.ErrInvalidInput) {
		t.Fatalf("create with past endDate = %v, want ErrInvalidInput", err)
	}
	same := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	cmd.EndDate = &same
	if _, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, cmd); err != nil {
		t.Fatalf("create with today endDate: %v", err)
	}
}

// The schedule window's conveyor sentinels (ticket #1154): create rejects an
// end date before the first occurrence with its own sentinel; endDate on the
// first occurrence itself stays valid (the UNTIL semantics).
func TestCreatePayment_RejectsEndDateBeforeFirstOccurrence(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	// The owner's today is Tuesday 2026-08-25: the weekly Friday rule's
	// first occurrence is 2026-08-28, so the Thursday end leaves the rule
	// zero occurrences.
	friday, err := domain.NewWeeklyRecurrence([]time.Weekday{time.Friday})
	if err != nil {
		t.Fatalf("fixture recurrence: %v", err)
	}
	cmd := h.createCmd()
	cmd.Recurrence = friday
	thursday := time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)
	cmd.EndDate = &thursday
	if _, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, cmd); !errors.Is(err, paymentsapp.ErrEndDateBeforeFirstOccurrence) {
		t.Fatalf("create with a hole = %v, want ErrEndDateBeforeFirstOccurrence", err)
	}

	firstFriday := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	cmd.EndDate = &firstFriday
	if _, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, cmd); err != nil {
		t.Fatalf("create with endDate on the first occurrence: %v", err)
	}
}

// The update path validates the merged rule (ticket #1154): a recurrence
// change re-checks the standing end date, the frontend's silent reset (the
// end-date clear riding the same patch) is accepted, and a legacy rule with
// a hole accepts unrelated edits — the narrow trigger keeps the off-topic
// 400 away from title-only patches.
func TestUpdatePayment_WindowGuardsTheMergedRule(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	friday, err := domain.NewWeeklyRecurrence([]time.Weekday{time.Friday})
	if err != nil {
		t.Fatalf("fixture recurrence: %v", err)
	}
	monday, err := domain.NewWeeklyRecurrence([]time.Weekday{time.Monday})
	if err != nil {
		t.Fatalf("fixture recurrence: %v", err)
	}
	sunday, err := domain.NewWeeklyRecurrence([]time.Weekday{time.Sunday})
	if err != nil {
		t.Fatalf("fixture recurrence: %v", err)
	}

	// A valid rule: weekly Friday ending on its own first occurrence.
	cmd := h.createCmd()
	cmd.Recurrence = friday
	firstFriday := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	cmd.EndDate = &firstFriday
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, cmd)
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}

	// A title-only patch on the valid rule is accepted (research case 10).
	validTitle := "Аренда (пт)"
	if _, err := h.svc.UpdatePayment(h.ctx(), h.owner, h.propID, created.ID, paymentsapp.UpdatePaymentCommand{
		Title: &validTitle,
	}); err != nil {
		t.Fatalf("title-only patch on a valid rule: %v", err)
	}

	// PATCH recurrence → Monday: the merged rule's first occurrence (Monday
	// 2026-08-31) moves past the standing end — rejected.
	if _, err := h.svc.UpdatePayment(h.ctx(), h.owner, h.propID, created.ID, paymentsapp.UpdatePaymentCommand{
		Recurrence: &monday,
	}); !errors.Is(err, paymentsapp.ErrEndDateBeforeFirstOccurrence) {
		t.Fatalf("recurrence-only patch making a hole = %v, want ErrEndDateBeforeFirstOccurrence", err)
	}

	// The same recurrence change plus the end-date clear — the frontend's
	// silent reset — is accepted and turns the rule open-ended.
	updated, err := h.svc.UpdatePayment(h.ctx(), h.owner, h.propID, created.ID, paymentsapp.UpdatePaymentCommand{
		Recurrence: &monday,
		EndDate:    &paymentsapp.EndDateUpdate{},
	})
	if err != nil {
		t.Fatalf("update with the end-date clear: %v", err)
	}
	if updated.EndDate != nil {
		t.Fatalf("end date = %v, want cleared", updated.EndDate)
	}

	// The legacy rule with a hole cannot be created through the API
	// anymore — the end date is drilled back by direct SQL (the pre-fix
	// rows the migration never heals).
	legacyCmd := h.createCmd()
	legacyCmd.Recurrence = monday
	firstMonday := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	legacyCmd.EndDate = &firstMonday
	legacy, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, legacyCmd)
	if err != nil {
		t.Fatalf("create legacy payment: %v", err)
	}
	hole := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	if _, err := h.pool.Exec(h.ctx(), `UPDATE payments SET end_date = $1 WHERE id = $2`, hole, legacy.ID); err != nil {
		t.Fatalf("drill the legacy hole: %v", err)
	}

	// A recurrence-only patch over the hole re-validates the merged rule:
	// the first Sunday (2026-08-30) still stands past the end — rejected.
	if _, err := h.svc.UpdatePayment(h.ctx(), h.owner, h.propID, legacy.ID, paymentsapp.UpdatePaymentCommand{
		Recurrence: &sunday,
	}); !errors.Is(err, paymentsapp.ErrEndDateBeforeFirstOccurrence) {
		t.Fatalf("recurrence patch over a legacy hole = %v, want ErrEndDateBeforeFirstOccurrence", err)
	}

	// A title-only patch on the same legacy rule is accepted.
	newTitle := "Аренда (правка)"
	titleUpdated, err := h.svc.UpdatePayment(h.ctx(), h.owner, h.propID, legacy.ID, paymentsapp.UpdatePaymentCommand{
		Title: &newTitle,
	})
	if err != nil {
		t.Fatalf("title-only patch on a legacy rule: %v", err)
	}
	if titleUpdated.Title != newTitle {
		t.Fatalf("title = %q, want %q", titleUpdated.Title, newTitle)
	}
}

// The rule-level validation table lives at the validator's own fast seam
// (payment_rule_test.go, TestValidateRule); these are the conveyor
// sentinels — the create and update paths really reject an invalid rule
// through the full mutation pipeline.
func TestPaymentMutations_RejectInvalidRules(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	unknownSlug := "not-a-catalog-slug"
	cmd := h.createCmd()
	cmd.CategorySlug = unknownSlug
	if _, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, cmd); !errors.Is(err, paymentsapp.ErrInvalidInput) {
		t.Fatalf("create with unknown slug = %v, want ErrInvalidInput", err)
	}

	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	blankTitle := " "
	if _, err := h.svc.UpdatePayment(h.ctx(), h.owner, h.propID, created.ID, paymentsapp.UpdatePaymentCommand{
		Title: &blankTitle,
	}); !errors.Is(err, paymentsapp.ErrInvalidInput) {
		t.Fatalf("update with blank title = %v, want ErrInvalidInput", err)
	}
}

func TestPauseResume_Lifecycle(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}

	paused, err := h.svc.PausePayment(h.ctx(), h.owner, h.propID, created.ID)
	if err != nil {
		t.Fatalf("pause payment: %v", err)
	}
	active, ok := domain.ActivePause(paused.Pauses)
	if !ok || active.From.Format(time.DateOnly) != day25 || active.To != nil {
		t.Fatalf("active pause = %+v, want open-ended from %s", active, day25)
	}
	// The pause removes the future planned; today's occurrence stays as debt.
	ops := opStatuses(h.opsOf(t, created.ID))
	if ops[day25] != opPlanned {
		t.Fatalf("today = %q, want planned (the pre-pause occurrence stays)", ops[day25])
	}
	if _, future := ops[day26]; future {
		t.Fatalf("tomorrow = planned, want absent — the pause removes the future planned")
	}

	if _, err := h.svc.PausePayment(h.ctx(), h.owner, h.propID, created.ID); !errors.Is(err, paymentsapp.ErrAlreadyPaused) {
		t.Fatalf("second pause = %v, want ErrAlreadyPaused", err)
	}

	if actions := h.auditActions(t, created.ID); len(actions) != 2 || actions[1] != "payment.paused" {
		t.Fatalf("audit actions = %v, want [payment.created payment.paused]", actions)
	}
}

func TestResume_ClosesPauseAndStandsFuture(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if _, err := h.svc.PausePayment(h.ctx(), h.owner, h.propID, created.ID); err != nil {
		t.Fatalf("pause payment: %v", err)
	}

	// Days pass inside the pause: nothing accrues.
	h.clock.now = tickBaseTime.AddDate(0, 0, 3)
	resumed, err := h.svc.ResumePayment(h.ctx(), h.owner, h.propID, created.ID)
	if err != nil {
		t.Fatalf("resume payment: %v", err)
	}
	if _, active := domain.ActivePause(resumed.Pauses); active {
		t.Fatalf("resume left an open pause: %+v", resumed.Pauses)
	}
	if len(resumed.Pauses) != 1 || resumed.Pauses[0].To == nil || resumed.Pauses[0].To.Format(time.DateOnly) != "2026-08-28" {
		t.Fatalf("closed pause = %+v, want [2026-08-25, 2026-08-28)", resumed.Pauses)
	}
	// The resume day (the 28th in Moscow) is due and the future planned
	// stands on the 29th.
	ops := opStatuses(h.opsOf(t, created.ID))
	if ops["2026-08-28"] != opPlanned || ops["2026-08-29"] != opPlanned {
		t.Fatalf("after resume: 28th=%q 29th=%q, want planned/planned", ops["2026-08-28"], ops["2026-08-29"])
	}

	if _, err := h.svc.ResumePayment(h.ctx(), h.owner, h.propID, created.ID); !errors.Is(err, paymentsapp.ErrNotPaused) {
		t.Fatalf("resume without pause = %v, want ErrNotPaused", err)
	}
}

func TestDeletePayment_KeepOverdueMatrix(t *testing.T) {
	t.Parallel()
	// One payment accumulates overdue debt (the clock moves two days ahead
	// after creation), then is deleted with each keep_overdue value.
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}

	// Pay today's occurrence directly: paid facts must survive every delete.
	if _, err := h.pool.Exec(h.ctx(), `
		UPDATE operations SET status = 'paid', paid_date = $2
		WHERE payment_id = $1 AND date = $2`, created.ID, day25,
	); err != nil {
		t.Fatalf("pay today: %v", err)
	}
	h.clock.now = tickBaseTime.AddDate(0, 0, 2)
	// Materialize the debt of the missed days through the worker-path tick.
	if err := h.tick.RunOwnerTick(h.ctx(), h.owner); err != nil {
		t.Fatalf("run tick: %v", err)
	}

	if err := h.svc.DeletePayment(h.ctx(), h.owner, h.propID, created.ID, true); err != nil {
		t.Fatalf("delete keep_overdue=true: %v", err)
	}
	// The debt survives the rule: planned operations with payment_id set to
	// NULL (the «платёж удалён» mark), the paid fact kept.
	var keptPlanned, keptPaid int
	if err := h.pool.QueryRow(h.ctx(), `
		SELECT count(*) FILTER (WHERE status = 'planned'),
		       count(*) FILTER (WHERE status = 'paid')
		FROM operations
		WHERE origin = 'payment' AND payment_id IS NULL AND title = 'Аренда'`,
	).Scan(&keptPlanned, &keptPaid); err != nil {
		t.Fatalf("count orphan operations: %v", err)
	}
	if keptPlanned == 0 {
		t.Fatalf("keep_overdue=true removed the overdue debt")
	}
	if keptPaid != 1 {
		t.Fatalf("paid facts after keep delete = %d, want 1", keptPaid)
	}

	// The second variant: the debt goes with the rule.
	h2 := newPaymentsHarness(t).withOwner("Europe/Moscow")
	created2, err := h2.svc.CreatePayment(h2.ctx(), h2.owner, h2.propID, h2.createCmd())
	if err != nil {
		t.Fatalf("create payment 2: %v", err)
	}
	h2.clock.now = tickBaseTime.AddDate(0, 0, 2)
	if err := h2.tick.RunOwnerTick(h2.ctx(), h2.owner); err != nil {
		t.Fatalf("run tick 2: %v", err)
	}
	if err := h2.svc.DeletePayment(h2.ctx(), h2.owner, h2.propID, created2.ID, false); err != nil {
		t.Fatalf("delete keep_overdue=false: %v", err)
	}
	var leftPlanned int
	if err := h2.pool.QueryRow(h2.ctx(), `
		SELECT count(*) FROM operations
		WHERE origin = 'payment' AND payment_id IS NULL AND title = 'Аренда' AND status = 'planned'`,
	).Scan(&leftPlanned); err != nil {
		t.Fatalf("count orphan planned: %v", err)
	}
	if leftPlanned != 0 {
		t.Fatalf("keep_overdue=false left %d planned debt rows", leftPlanned)
	}
}

func TestPaymentRoleMatrix(t *testing.T) {
	t.Parallel()
	viewer := newPaymentsHarnessWithPolicy(t, stubPropertyPolicy{role: sharedpolicy.RoleViewer}).withOwner("Europe/Moscow")
	member := uuid.Must(uuid.NewV7())

	if _, err := viewer.svc.ListPayments(viewer.ctx(), member, viewer.propID, ""); err != nil {
		t.Fatalf("viewer list: %v", err)
	}
	if _, err := viewer.svc.CreatePayment(viewer.ctx(), member, viewer.propID, viewer.createCmd()); !errors.Is(err, paymentsapp.ErrForbidden) {
		t.Fatalf("viewer create = %v, want ErrForbidden", err)
	}

	full := newPaymentsHarnessWithPolicy(t, stubPropertyPolicy{role: sharedpolicy.RoleFullAccess}).withOwner("Europe/Moscow")
	full.seedActor(member)
	created, err := full.svc.CreatePayment(full.ctx(), member, full.propID, full.createCmd())
	if err != nil {
		t.Fatalf("full access create: %v", err)
	}
	// The member writes into the property owner's scope.
	if created.OwnerID != full.owner {
		t.Fatalf("created owner_id = %s, want the property owner's scope", created.OwnerID)
	}
	if err := full.svc.DeletePayment(full.ctx(), member, full.propID, created.ID, true); !errors.Is(err, paymentsapp.ErrForbidden) {
		t.Fatalf("full access delete = %v, want ErrForbidden — deletion is the owner's alone", err)
	}
	// The member's action is attributed to the full_access actor, not masked
	// as the owner's.
	var actorRole string
	if err := full.pool.QueryRow(full.ctx(),
		`SELECT actor_role FROM audit_log WHERE entity_type = 'payment' AND entity_id = $1`, created.ID,
	).Scan(&actorRole); err != nil {
		t.Fatalf("read audit actor role: %v", err)
	}
	if actorRole != "full_access" {
		t.Fatalf("audit actor_role = %q, want full_access", actorRole)
	}

	none := newPaymentsHarnessWithPolicy(t, stubPropertyPolicy{role: sharedpolicy.RoleNone}).withOwner("Europe/Moscow")
	stranger := uuid.Must(uuid.NewV7())
	if _, err := none.svc.GetPayment(none.ctx(), stranger, none.propID, created.ID); !errors.Is(err, paymentsapp.ErrNotFound) {
		t.Fatalf("stranger get = %v, want the privacy-preserving ErrNotFound", err)
	}

	// The owner (the policy's RoleOwner) may delete.
	ownerH := newPaymentsHarnessWithPolicy(t, stubPropertyPolicy{role: sharedpolicy.RoleOwner}).withOwner("Europe/Moscow")
	owned, err := ownerH.svc.CreatePayment(ownerH.ctx(), ownerH.owner, ownerH.propID, ownerH.createCmd())
	if err != nil {
		t.Fatalf("owner create: %v", err)
	}
	if err := ownerH.svc.DeletePayment(ownerH.ctx(), ownerH.owner, ownerH.propID, owned.ID, true); err != nil {
		t.Fatalf("owner delete: %v", err)
	}
}

func TestArchivedProperty_IsFinancialReadOnly(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}

	if _, err := h.pool.Exec(h.ctx(), `UPDATE properties SET status = 'archived' WHERE id = $1`, h.propID); err != nil {
		t.Fatalf("archive property: %v", err)
	}

	if _, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd()); !errors.Is(err, paymentsapp.ErrArchivedProperty) {
		t.Fatalf("create on archived = %v, want ErrArchivedProperty", err)
	}
	emptyCmd := paymentsapp.UpdatePaymentCommand{}
	if _, err := h.svc.UpdatePayment(h.ctx(), h.owner, h.propID, created.ID, emptyCmd); !errors.Is(err, paymentsapp.ErrArchivedProperty) {
		t.Fatalf("update on archived = %v, want ErrArchivedProperty", err)
	}
	if _, err := h.svc.PausePayment(h.ctx(), h.owner, h.propID, created.ID); !errors.Is(err, paymentsapp.ErrArchivedProperty) {
		t.Fatalf("pause on archived = %v, want ErrArchivedProperty", err)
	}
	if err := h.svc.DeletePayment(h.ctx(), h.owner, h.propID, created.ID, true); !errors.Is(err, paymentsapp.ErrArchivedProperty) {
		t.Fatalf("delete on archived = %v, want ErrArchivedProperty", err)
	}

	// Reads stay open on the archived object.
	if _, err := h.svc.GetPayment(h.ctx(), h.owner, h.propID, created.ID); err != nil {
		t.Fatalf("get on archived: %v", err)
	}
	if _, err := h.svc.ListPayments(h.ctx(), h.owner, h.propID, ""); err != nil {
		t.Fatalf("list on archived: %v", err)
	}
}

func TestUpdatePayment_RebuildsFuturePlannedInTx(t *testing.T) {
	t.Parallel()
	// The future planned is rebuilt with the NEW snapshot; the already
	// materialized occurrences keep theirs (ADR 0049 §1: snapshots are frozen
	// at materialization).
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}

	newAmount := int64(7777777)
	if _, err := h.svc.UpdatePayment(h.ctx(), h.owner, h.propID, created.ID, paymentsapp.UpdatePaymentCommand{
		AmountKopecks: &newAmount,
	}); err != nil {
		t.Fatalf("update payment: %v", err)
	}

	var todayAmount, tomorrowAmount int64
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT amount_kopecks FROM operations WHERE payment_id = $1 AND date = $2`, created.ID, day25,
	).Scan(&todayAmount); err != nil {
		t.Fatalf("read today's amount: %v", err)
	}
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT amount_kopecks FROM operations WHERE payment_id = $1 AND date = $2`, created.ID, day26,
	).Scan(&tomorrowAmount); err != nil {
		t.Fatalf("read tomorrow's amount: %v", err)
	}
	if todayAmount != 5000000 {
		t.Fatalf("today's snapshot = %d, want the original amount", todayAmount)
	}
	if tomorrowAmount != newAmount {
		t.Fatalf("rebuilt future snapshot = %d, want %d", tomorrowAmount, newAmount)
	}
}

// TestPaymentSearch_FiltersByTitle covers the contract's search parameter of
// the payments listing: case-insensitive substring match on the title,
// ILIKE metacharacters staying literals ('_' must not turn into a wildcard)
// and the empty search meaning "no filter".
func TestPaymentSearch_FiltersByTitle(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	const literalPercentTitle = "аренда гаража 100%"
	titles := []string{"Аренда квартиры", literalPercentTitle, "Электроэнергия"}
	for _, title := range titles {
		cmd := h.createCmd()
		cmd.Title = title
		if _, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, cmd); err != nil {
			t.Fatalf("create payment %q: %v", title, err)
		}
	}

	got, err := h.svc.ListPayments(h.ctx(), h.owner, h.propID, "АРЕНДА")
	if err != nil {
		t.Fatalf("list payments: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("search = %d items, want 2 (both аренда spellings)", len(got))
	}

	// The underscore is a literal here: nothing contains one.
	got, err = h.svc.ListPayments(h.ctx(), h.owner, h.propID, "квартир_")
	if err != nil {
		t.Fatalf("list payments: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("search with '_' matched %d items, want 0 (wildcard must stay literal)", len(got))
	}

	got, err = h.svc.ListPayments(h.ctx(), h.owner, h.propID, "100%")
	if err != nil {
		t.Fatalf("list payments: %v", err)
	}
	if len(got) != 1 || got[0].Payment.Title != literalPercentTitle {
		t.Fatalf("search '100%%' = %+v, want the literal-percent title", got)
	}

	all, err := h.svc.ListPayments(h.ctx(), h.owner, h.propID, "")
	if err != nil {
		t.Fatalf("list payments: %v", err)
	}
	if len(all) != len(titles) {
		t.Fatalf("empty search = %d items, want all %d", len(all), len(titles))
	}
}

// TestOperationSearch_FiltersByTitle covers the same search parameter on the
// property-wide operations listing against materialized rows.
func TestOperationSearch_FiltersByTitle(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	const wildcardProbeTitle = "Аренда квартиры"
	for _, title := range []string{wildcardProbeTitle, "Электроэнергия"} {
		cmd := h.createCmd()
		cmd.Title = title
		if _, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, cmd); err != nil {
			t.Fatalf("create payment %q: %v", title, err)
		}
	}

	searchCmd := h.listCmd(nil, 50, 0, false)
	searchCmd.Search = "аренда"
	got, err := h.ops.ListPropertyOperations(h.ctx(), h.owner, h.propID, searchCmd)
	if err != nil {
		t.Fatalf("list operations: %v", err)
	}
	for _, item := range got {
		if item.Operation.Title != wildcardProbeTitle {
			t.Fatalf("operation %q leaked through the search filter", item.Operation.Title)
		}
	}
	if len(got) == 0 {
		t.Fatalf("search found no operations, want materialized аренда rows")
	}

	searchCmd.Search = "квартир_"
	got, err = h.ops.ListPropertyOperations(h.ctx(), h.owner, h.propID, searchCmd)
	if err != nil {
		t.Fatalf("list operations: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("search with '_' matched %d operations, want 0", len(got))
	}
}

// readReminderOffset loads the rule and returns its reminder lead time.
func readReminderOffset(t *testing.T, h *paymentsHarness, paymentID uuid.UUID) *int {
	t.Helper()
	payment, err := h.svc.GetPayment(h.ctx(), h.owner, h.propID, paymentID)
	if err != nil {
		t.Fatalf("get payment: %v", err)
	}
	return payment.ReminderOffsetDays
}

// assertReminderReadBack fails unless the stored lead time equals want.
func assertReminderReadBack(t *testing.T, h *paymentsHarness, paymentID uuid.UUID, want *int) {
	t.Helper()
	got := readReminderOffset(t, h, paymentID)
	switch {
	case want == nil && got != nil:
		t.Fatalf("reminder offset = %v, want nil", got)
	case want != nil && (got == nil || *got != *want):
		t.Fatalf("reminder offset = %v, want %d", got, *want)
	}
}

// patchReminderSet applies the set/clear patch: value nil = явный null
// (отключить напоминания).
func patchReminderSet(t *testing.T, h *paymentsHarness, paymentID uuid.UUID, value *int) {
	t.Helper()
	if _, err := h.svc.UpdatePayment(h.ctx(), h.owner, h.propID, paymentID, paymentsapp.UpdatePaymentCommand{
		ReminderOffsetDays: &paymentsapp.ReminderOffsetUpdate{Value: value},
	}); err != nil {
		t.Fatalf("patch reminder offset: %v", err)
	}
}

// TestPaymentReminderOffset_Lifecycle goes the reminder field (карта #822,
// #824) through its whole contract: create persists it, reads carry it, the
// patch sets / clears it tri-state and omitting it changes nothing.
func TestPaymentReminderOffset_Lifecycle(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	offset := 3
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, func() paymentsapp.CreatePaymentCommand {
		cmd := h.createCmd()
		cmd.ReminderOffsetDays = &offset
		return cmd
	}())
	if err != nil {
		t.Fatalf("create payment with reminder: %v", err)
	}
	three := 3
	if created.ReminderOffsetDays == nil || *created.ReminderOffsetDays != three {
		t.Fatalf("created reminder offset = %v, want 3", created.ReminderOffsetDays)
	}
	assertReminderReadBack(t, h, created.ID, &three)
	listed, err := h.svc.ListPayments(h.ctx(), h.owner, h.propID, "")
	if err != nil {
		t.Fatalf("list payments: %v", err)
	}
	if len(listed) != 1 || listed[0].Payment.ReminderOffsetDays == nil || *listed[0].Payment.ReminderOffsetDays != three {
		t.Fatalf("list reminder offset = %v, want 3", listed)
	}

	// Omit — the field stays.
	if _, err := h.svc.UpdatePayment(h.ctx(), h.owner, h.propID, created.ID, paymentsapp.UpdatePaymentCommand{}); err != nil {
		t.Fatalf("update without changes: %v", err)
	}
	assertReminderReadBack(t, h, created.ID, &three)

	// A set value applies; the explicit null clears.
	seven := 7
	patchReminderSet(t, h, created.ID, &seven)
	assertReminderReadBack(t, h, created.ID, &seven)
	patchReminderSet(t, h, created.ID, nil)
	assertReminderReadBack(t, h, created.ID, nil)
}

// TestCreatePayment_RejectsBadReminderOffset: the reminder contract is the
// domain validator's — only nil or 1/3/7 pass the service seam too.
func TestCreatePayment_RejectsBadReminderOffset(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	bad := 2
	_, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, func() paymentsapp.CreatePaymentCommand {
		cmd := h.createCmd()
		cmd.ReminderOffsetDays = &bad
		return cmd
	}())
	if !errors.Is(err, paymentsapp.ErrInvalidInput) {
		t.Fatalf("create with reminder offset 2 = %v, want ErrInvalidInput", err)
	}
}
