//go:build integration

package application_test

// The payment change log's integration tests (ADR 0065, ticket #1188): the
// write seams through the real conveyor (the update diff, the pause/resume
// flips, the manual-title marker, the actions that write nothing) and the
// bidirectional keyset read with its access and contract errors.

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	paymentspg "github.com/nambers/arenda-planform/apps/backend/internal/payments/adapters/postgres"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	pgdb "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	rentalspg "github.com/nambers/arenda-planform/apps/backend/internal/rentals/adapters/postgres"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// changeLogRow is the raw stored row the fixtures inspections read.
type changeLogRow struct {
	ID        uuid.UUID
	ActorID   uuid.UUID
	Action    string
	Changes   []domain.FieldChange
	CreatedAt time.Time
}

// loadChangeLog reads a payment's change log straight from the table, newest
// first — the single SQL home of the log's fixture inspection.
func (h *paymentsHarness) loadChangeLog(t *testing.T, paymentID uuid.UUID) []changeLogRow {
	t.Helper()
	rows, err := h.pool.Query(h.ctx(), `
		SELECT id, actor_id, action, changes, created_at
		FROM payment_change_log WHERE payment_id = $1
		ORDER BY created_at DESC, id DESC`, paymentID)
	if err != nil {
		t.Fatalf("query change log: %v", err)
	}
	defer rows.Close()

	out := []changeLogRow{}
	for rows.Next() {
		var r changeLogRow
		var blob []byte
		if err := rows.Scan(&r.ID, &r.ActorID, &r.Action, &blob, &r.CreatedAt); err != nil {
			t.Fatalf("scan change log: %v", err)
		}
		if err := json.Unmarshal(blob, &r.Changes); err != nil {
			t.Fatalf("unmarshal changes %s: %v", r.ID, err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate change log: %v", err)
	}
	return out
}

// seedChangeRow inserts one log row with a controlled created_at — the
// pagination tests' clock, independent of statement timing.
func (h *paymentsHarness) seedChangeRow(t *testing.T, paymentID uuid.UUID, at time.Time, action string) {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO payment_change_log (id, payment_id, owner_id, property_id, actor_id, action, changes, created_at)
		 VALUES ($1, $2, $3, $4, $3, $5, '[]'::jsonb, $6)`,
		id, paymentID, h.owner, h.propID, action, at,
	); err != nil {
		t.Fatalf("seed change row: %v", err)
	}
}

// changeLogSeedDate builds a UTC-midnight calendar date on the harness's
// controlled seeding clock (the unit-package utcDate is out of this file's
// external test package).
func changeLogSeedDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// reminderPtr keeps the reminder lead time's tri-state shape on one line.
func reminderPtr(days int) *paymentsapp.ReminderOffsetUpdate {
	return &paymentsapp.ReminderOffsetUpdate{Value: &days}
}

func TestPaymentChangeLog_UpdateWritesDictionaryOrderedDiff(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}

	// The edit touches three fields from different ends of the dictionary —
	// the stored diff must carry them in the dictionary's order.
	newAmount := int64(6000000)
	if _, err := h.svc.UpdatePayment(h.ctx(), h.owner, h.propID, created.ID, paymentsapp.UpdatePaymentCommand{
		AmountKopecks:      new(newAmount),
		AutoPay:            new(true),
		ReminderOffsetDays: reminderPtr(3),
	}); err != nil {
		t.Fatalf("update payment: %v", err)
	}

	rows := h.loadChangeLog(t, created.ID)
	if len(rows) != 1 {
		t.Fatalf("change log = %d rows, want 1 (create writes none)", len(rows))
	}
	row := rows[0]
	if row.Action != string(domain.ChangeUpdated) {
		t.Fatalf("action = %q, want %q", row.Action, domain.ChangeUpdated)
	}
	if row.ActorID != h.owner {
		t.Fatalf("actor = %s, want the mutating owner", row.ActorID)
	}
	wantFields := []domain.ChangeField{
		domain.ChangeFieldAmount, domain.ChangeFieldAutoPay, domain.ChangeFieldReminderOffset,
	}
	if len(row.Changes) != len(wantFields) {
		t.Fatalf("diff = %d fields, want %d (%v)", len(row.Changes), len(wantFields), row.Changes)
	}
	for i, want := range wantFields {
		if row.Changes[i].Field != want {
			t.Fatalf("diff[%d].field = %q, want %q — the dictionary's order", i, row.Changes[i].Field, want)
		}
	}
	// The typed values: kopecks travel as numbers, the reminder as number or
	// null, the auto-pay as booleans.
	assertChangeValue(t, row.Changes[0].Old, json.Number("5000000"))
	assertChangeValue(t, row.Changes[0].New, json.Number("6000000"))
	assertChangeValue(t, row.Changes[1].Old, false)
	assertChangeValue(t, row.Changes[1].New, true)
	assertChangeValue(t, row.Changes[2].Old, nil)
	assertChangeValue(t, row.Changes[2].New, json.Number("3"))
}

// TestPaymentChangeLog_TitleMarkerAndRule pins ADR 0065 §3: a title in the
// diff is the one chip and the marker's one flip — the marker never resets
// and creation never raises it.
func TestPaymentChangeLog_TitleMarkerAndRule(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if created.TitleIsManual {
		t.Fatalf("create must not raise the manual-title marker")
	}

	// An amount-only edit: no title row, marker stays down.
	if _, err := h.svc.UpdatePayment(h.ctx(), h.owner, h.propID, created.ID, paymentsapp.UpdatePaymentCommand{
		AmountKopecks: new(int64(6000000)),
	}); err != nil {
		t.Fatalf("update payment: %v", err)
	}
	stored, err := h.svc.GetPayment(h.ctx(), h.owner, h.propID, created.ID)
	if err != nil {
		t.Fatalf("get payment: %v", err)
	}
	if stored.TitleIsManual {
		t.Fatalf("a non-title edit must not raise the marker")
	}
	for _, row := range h.loadChangeLog(t, created.ID) {
		if domain.HasTitleChange(row.Changes) {
			t.Fatalf("a non-title edit wrote a title row: %v", row.Changes)
		}
	}

	// The title edit: the title row appears and the marker goes up —
	// and stays up through a following non-title edit.
	if _, err := h.svc.UpdatePayment(h.ctx(), h.owner, h.propID, created.ID, paymentsapp.UpdatePaymentCommand{
		Title: new("Аренда своя"),
	}); err != nil {
		t.Fatalf("update payment: %v", err)
	}
	if _, err := h.svc.UpdatePayment(h.ctx(), h.owner, h.propID, created.ID, paymentsapp.UpdatePaymentCommand{
		AmountKopecks: new(int64(7000000)),
	}); err != nil {
		t.Fatalf("update payment: %v", err)
	}
	stored, err = h.svc.GetPayment(h.ctx(), h.owner, h.propID, created.ID)
	if err != nil {
		t.Fatalf("get payment: %v", err)
	}
	if !stored.TitleIsManual {
		t.Fatalf("the title edit must raise the marker; it never resets")
	}
	var titleRows int
	for _, row := range h.loadChangeLog(t, created.ID) {
		if domain.HasTitleChange(row.Changes) {
			titleRows++
		}
	}
	if titleRows != 1 {
		t.Fatalf("title rows = %d, want 1 (the title edit only)", titleRows)
	}
}

func TestPaymentChangeLog_PauseResumeRows(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}

	if _, err := h.svc.PausePayment(h.ctx(), h.owner, h.propID, created.ID); err != nil {
		t.Fatalf("pause payment: %v", err)
	}
	if _, err := h.svc.ResumePayment(h.ctx(), h.owner, h.propID, created.ID); err != nil {
		t.Fatalf("resume payment: %v", err)
	}

	rows := h.loadChangeLog(t, created.ID)
	if len(rows) != 2 {
		t.Fatalf("change log = %d rows, want 2 (pause + resume)", len(rows))
	}
	want := []domain.ChangeAction{domain.ChangeResumed, domain.ChangePaused}
	for i, action := range want {
		if rows[i].Action != string(action) {
			t.Fatalf("rows[%d].action = %q, want %q", i, rows[i].Action, action)
		}
		if len(rows[i].Changes) != 0 {
			t.Fatalf("rows[%d].changes = %v, want empty — the flip is the content", i, rows[i].Changes)
		}
	}
	// The stored blob is the literal [] — never null.
	var blob []byte
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT changes FROM payment_change_log WHERE payment_id = $1 LIMIT 1`, created.ID,
	).Scan(&blob); err != nil {
		t.Fatalf("read changes blob: %v", err)
	}
	if string(blob) != "[]" {
		t.Fatalf("stored changes = %s, want the literal []", blob)
	}
}

// TestPaymentChangeLog_SilentActions pins the «not edits» decisions: creation,
// the favorite star and a patch that moves nothing write no row; deletion
// takes the whole log with it (FK CASCADE).
func TestPaymentChangeLog_SilentActions(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if rows := h.loadChangeLog(t, created.ID); len(rows) != 0 {
		t.Fatalf("create wrote %d rows, want 0", len(rows))
	}

	if _, err := h.svc.SetPaymentFavorite(h.ctx(), h.owner, h.propID, created.ID, true); err != nil {
		t.Fatalf("set favorite: %v", err)
	}
	if rows := h.loadChangeLog(t, created.ID); len(rows) != 0 {
		t.Fatalf("favorite wrote %d rows, want 0 (шум, ADR 0065 §4)", len(rows))
	}

	// A no-op patch (the same amount re-sent) is not a terms edit.
	if _, err := h.svc.UpdatePayment(h.ctx(), h.owner, h.propID, created.ID, paymentsapp.UpdatePaymentCommand{
		AmountKopecks: new(created.AmountKopecks),
	}); err != nil {
		t.Fatalf("noop update: %v", err)
	}
	if rows := h.loadChangeLog(t, created.ID); len(rows) != 0 {
		t.Fatalf("noop patch wrote %d rows, want 0 — a row's content is its diff", len(rows))
	}

	// Paying an operation («Оплатить сейчас», early or late alike) writes no
	// row — the fact lives on the operation's page (ADR 0065 §4).
	ops, err := h.ops.ListPaymentOperations(h.ctx(), h.owner, h.propID, created.ID, paymentsapp.OperationsListQuery{
		Limit: 1, Today: h.clock.Now(),
	})
	if err != nil || len(ops) == 0 {
		t.Fatalf("list operations for pay: %v (%d)", err, len(ops))
	}
	if _, err := h.ops.PayOperation(h.ctx(), h.owner, h.propID, ops[0].Operation.ID); err != nil {
		t.Fatalf("pay operation: %v", err)
	}
	if rows := h.loadChangeLog(t, created.ID); len(rows) != 0 {
		t.Fatalf("pay wrote %d rows, want 0 — оплаты в журнал не пишутся", len(rows))
	}

	// One real edit, then deletion: the log dies with the rule.
	if _, err := h.svc.UpdatePayment(h.ctx(), h.owner, h.propID, created.ID, paymentsapp.UpdatePaymentCommand{
		AmountKopecks: new(int64(6000000)),
	}); err != nil {
		t.Fatalf("update payment: %v", err)
	}
	if rows := h.loadChangeLog(t, created.ID); len(rows) != 1 {
		t.Fatalf("change log = %d rows, want 1 before delete", len(rows))
	}
	if err := h.svc.DeletePayment(h.ctx(), h.owner, h.propID, created.ID, true); err != nil {
		t.Fatalf("delete payment: %v", err)
	}
	if rows := h.loadChangeLog(t, created.ID); len(rows) != 0 {
		t.Fatalf("after delete the log = %d rows, want 0 — FK CASCADE", len(rows))
	}
}

func TestPaymentChanges_KeysetPagination(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	// Five seeded rows on a controlled clock: t1..t5, one minute apart.
	base := changeLogSeedDate(2026, time.September, 1)
	for i := range 5 {
		h.seedChangeRow(t, created.ID, base.Add(time.Duration(i)*time.Minute), "updated")
	}

	read := func(q paymentsapp.ChangesQuery) paymentsapp.ChangeLogPage {
		t.Helper()
		page, err := h.svc.PaymentChanges(h.ctx(), h.owner, h.propID, created.ID, q)
		if err != nil {
			t.Fatalf("payment changes: %v", err)
		}
		return page
	}

	// The first page, newest first: t5, t4 — and the cursor pair.
	page := read(paymentsapp.ChangesQuery{Limit: 2})
	if len(page.Items) != 2 {
		t.Fatalf("first page = %d rows, want 2", len(page.Items))
	}
	if page.Items[0].CreatedAt.Before(page.Items[1].CreatedAt) {
		t.Fatalf("first page order = %v, want newest first", page.Items)
	}
	if page.NextCursor == "" || page.PrevCursor == "" {
		t.Fatalf("first page cursors: next=%q prev=%q, want both", page.NextCursor, page.PrevCursor)
	}

	// The before_cursor continues into the past: t3, t2.
	older := read(paymentsapp.ChangesQuery{BeforeCursor: page.NextCursor, Limit: 2})
	if len(older.Items) != 2 || !older.Items[0].CreatedAt.Before(page.Items[1].CreatedAt) {
		t.Fatalf("before page = %v, want t3,t2 strictly older", older.Items)
	}

	// The after_cursor reads the window strictly newer than the anchor (t2):
	// t3, t4, t5 — in the feed's DESC order even though the walk is ASC
	// under the hood (the oldest tail of a wide burst never strands).
	newer := read(paymentsapp.ChangesQuery{AfterCursor: older.NextCursor, Limit: 10})
	if len(newer.Items) != 3 {
		t.Fatalf("after page = %d rows, want the 3 newer than t2", len(newer.Items))
	}
	if !newer.Items[0].CreatedAt.After(newer.Items[2].CreatedAt) {
		t.Fatalf("after page order = %v, want the feed's DESC", newer.Items)
	}
	// The after-leg chains: from the anchor upward in bounded pages the walk
	// reaches the fresh edge without holes.
	bounded := read(paymentsapp.ChangesQuery{AfterCursor: older.NextCursor, Limit: 2})
	if len(bounded.Items) != 2 || bounded.Items[1].CreatedAt.After(bounded.Items[0].CreatedAt) {
		t.Fatalf("bounded after page = %v, want the two OLDEST eligible (t3, t4)", bounded.Items)
	}

	// A full walk lands exactly on the seeded five — no duplicates, no holes.
	if seen := walkAllPages(t, read, 2); len(seen) != 5 {
		t.Fatalf("the walk saw %d rows, want the seeded 5", len(seen))
	}

	// The contract errors: both cursors at once, a malformed blob, a size
	// past the ceiling.
	assertChangesContractErrors(t, h, created.ID, page.NextCursor, page.PrevCursor)
}

func TestPaymentChanges_AccessAndPrivacy(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if _, err := h.svc.UpdatePayment(h.ctx(), h.owner, h.propID, created.ID, paymentsapp.UpdatePaymentCommand{
		AmountKopecks: new(int64(6000000)),
	}); err != nil {
		t.Fatalf("update payment: %v", err)
	}

	// The member with the view capability reads the log (ADR 0028): the
	// same store wiring over the same pool, the policy stubbed to viewer.
	viewerFactory := paymentsapp.NewTxStoreFactory(
		paymentspg.NewTickStore(h.pool),
		paymentspg.NewPaymentStore(h.pool),
		paymentspg.NewOperationStore(h.pool),
		paymentspg.NewPropertyStore(h.pool),
		paymentspg.NewGlobalPaymentStore(h.pool),
		rentalspg.NewRentalLinkReader(h.pool),
		paymentspg.NewPaymentChangeLogStore(h.pool),
		nil, nil,
		pgdb.NewUoW(h.pool, slog.New(slog.DiscardHandler)),
	)
	viewerSvc := paymentsapp.NewPaymentService(
		viewerFactory, paymentspg.NewOwnerCalendar(h.pool, h.clock),
		stubPropertyPolicy{role: sharedpolicy.RoleViewer},
	)
	member := uuid.Must(uuid.NewV7())
	h.seedActor(member)
	page, err := viewerSvc.PaymentChanges(h.ctx(), member, h.propID, created.ID, paymentsapp.ChangesQuery{})
	if err != nil {
		t.Fatalf("viewer payment changes: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Action != domain.ChangeUpdated {
		t.Fatalf("viewer page = %+v, want the single updated row", page.Items)
	}

	// A stranger gets the privacy 404 — as for an unknown payment id.
	stranger := uuid.Must(uuid.NewV7())
	h.seedActor(stranger)
	strangerPage, err := h.svc.PaymentChanges(h.ctx(), stranger, h.propID, created.ID, paymentsapp.ChangesQuery{})
	if !errors.Is(err, paymentsapp.ErrNotFound) {
		t.Fatalf("stranger = %v, want the privacy ErrNotFound", err)
	}
	_ = strangerPage
	foreign := uuid.Must(uuid.NewV7())
	foreignPage, err := h.svc.PaymentChanges(h.ctx(), h.owner, h.propID, foreign, paymentsapp.ChangesQuery{})
	if !errors.Is(err, paymentsapp.ErrNotFound) {
		t.Fatalf("foreign payment = %v, want ErrNotFound", err)
	}
	_ = foreignPage
}

// assertChangeValue compares one decoded change value against the expected
// typed JSON (numbers stay json.Number so 5000000 never decays to float).
func assertChangeValue(t *testing.T, got json.RawMessage, want any) {
	t.Helper()
	var decoded any
	dec := json.NewDecoder(bytes.NewReader(got))
	dec.UseNumber()
	if err := dec.Decode(&decoded); err != nil {
		t.Fatalf("decode change value %s: %v", got, err)
	}
	if want == nil {
		if decoded != nil {
			t.Fatalf("change value = %s, want null", got)
		}
		return
	}
	switch w := want.(type) {
	case json.Number:
		n, ok := decoded.(json.Number)
		if !ok || n.String() != w.String() {
			t.Fatalf("change value = %v (%T), want number %s", decoded, decoded, w)
		}
	default:
		if decoded != want {
			t.Fatalf("change value = %v (%T), want %v (%T)", decoded, decoded, want, want)
		}
	}
}

// assertChangesContractErrors pins the read's 400 vocabulary: the mutually
// exclusive cursors, the malformed blob, the out-of-range page size.
func assertChangesContractErrors(
	t *testing.T, h *paymentsHarness, paymentID uuid.UUID, next, prev string,
) {
	t.Helper()
	if _, err := h.svc.PaymentChanges(h.ctx(), h.owner, h.propID, paymentID, paymentsapp.ChangesQuery{
		BeforeCursor: next, AfterCursor: prev,
	}); !errors.Is(err, paymentsapp.ErrInvalidInput) {
		t.Fatalf("both cursors = %v, want ErrInvalidInput", err)
	}
	if _, err := h.svc.PaymentChanges(h.ctx(), h.owner, h.propID, paymentID, paymentsapp.ChangesQuery{
		BeforeCursor: "не-курсор",
	}); !errors.Is(err, paymentsapp.ErrInvalidInput) {
		t.Fatalf("malformed cursor = %v, want ErrInvalidInput", err)
	}
	if _, err := h.svc.PaymentChanges(h.ctx(), h.owner, h.propID, paymentID, paymentsapp.ChangesQuery{
		Limit: 101,
	}); !errors.Is(err, paymentsapp.ErrInvalidInput) {
		t.Fatalf("limit 101 = %v, want ErrInvalidInput", err)
	}
}

// walkAllPages follows the before-cursor to the feed's end, page by page,
// and returns every visited row id — the no-duplicates-no-holes check.
func walkAllPages(
	t *testing.T, read func(paymentsapp.ChangesQuery) paymentsapp.ChangeLogPage, limit int,
) map[uuid.UUID]bool {
	t.Helper()
	seen := map[uuid.UUID]bool{}
	cursor := ""
	for pages := 0; ; pages++ {
		p := read(paymentsapp.ChangesQuery{BeforeCursor: cursor, Limit: limit})
		for _, item := range p.Items {
			if seen[item.ID] {
				t.Fatalf("row %s repeated across pages", item.ID)
			}
			seen[item.ID] = true
		}
		if p.NextCursor == "" {
			return seen
		}
		cursor = p.NextCursor
		if pages > 10 {
			t.Fatalf("the before-walk does not terminate")
		}
	}
}
