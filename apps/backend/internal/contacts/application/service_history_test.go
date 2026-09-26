package application

// The action journal of the contact mutations (карта #704, тикет #707,
// ADR 0061): a property-bound card journals with the ФИО snapshot — never
// the phone — a card without any binding journals nothing (the row anchors
// to a property), a no-op update journals nothing (§3), a same-binding
// detail edit carries the old → new ФИО, and a binding change anchors on
// the ends it touches (§4, тикет #856): a cross-property move writes the
// contact.moved pair — one row per end — a bind or an unbind owns a single
// end and writes contact.bound / contact.unbound there; a mixed unbind +
// details PATCH folds into the one source row.

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// createdContactFullName is the ФИО the canonical create fixture produces:
// «Пётр Иванов» (the trimmed first name plus the last name).
const createdContactFullName = contactFirstName + " " + "Иванов"

// rebindLastName is the shared rename literal of the rebind fixtures
// (goconst: one home).
const rebindLastName = "Сидоров"

// reboundContactFullName is the ФИО after the rebind fixtures' rename.
const reboundContactFullName = contactFirstName + " " + rebindLastName

func TestHistory_BoundContactRows(t *testing.T) {
	t.Parallel()
	h := newServiceHarness(t, sharedpolicy.RoleOwner)

	created, err := h.svc.CreateContact(context.Background(), h.owner, createCmd(&h.property))
	if err != nil {
		t.Fatalf("CreateContact: %v", err)
	}

	// A same-form PATCH without a move is a no-op: it journals nothing
	// (ADR 0061 §3) — only the create row stands before the delete.
	sameName := contactFirstName
	if _, err := h.svc.UpdateContact(context.Background(), h.owner, created.ID, UpdateContactCommand{
		FirstName: &sameName,
	}); err != nil {
		t.Fatalf("UpdateContact (same form): %v", err)
	}
	if len(h.history.Entries) != 1 {
		t.Fatalf("journal entries after the no-op update = %d, want 1", len(h.history.Entries))
	}

	// A real change carries the old → new ФИО snapshot.
	newLastName := rebindLastName
	if _, err := h.svc.UpdateContact(context.Background(), h.owner, created.ID, UpdateContactCommand{
		LastName: &newLastName,
	}); err != nil {
		t.Fatalf("UpdateContact: %v", err)
	}
	if err := h.svc.DeleteContact(context.Background(), h.owner, created.ID); err != nil {
		t.Fatalf("DeleteContact: %v", err)
	}

	if len(h.history.Entries) != 3 {
		t.Fatalf("journal entries = %d, want 3", len(h.history.Entries))
	}
	want := []struct{ action, text string }{
		{string(historydomain.ActionContactCreated), "Добавлен контакт: " + createdContactFullName},
		{string(historydomain.ActionContactUpdated), "Контакт изменён: " + createdContactFullName + " → " + reboundContactFullName},
		{string(historydomain.ActionContactDeleted), "Контакт удалён: " + reboundContactFullName},
	}
	for i, w := range want {
		e := h.history.Entries[i]
		if string(e.Action) != w.action {
			t.Errorf("entry %d action = %s, want %s", i, e.Action, w.action)
		}
		if e.Segments.PlainText() != w.text {
			t.Errorf("entry %d text = %q, want %q", i, e.Segments.PlainText(), w.text)
		}
		if e.PropertyID != h.property {
			t.Errorf("entry %d property = %s, want the card's binding", i, e.PropertyID)
		}
	}
	// The label snapshot is the ФИО — the phone never enters the row text.
	for _, e := range h.history.Entries {
		if text := e.Segments.PlainText(); strings.Contains(text, "89161234567") || strings.Contains(text, "+79161234567") {
			t.Errorf("row text %q must never carry the phone", text)
		}
	}
}

// rebindWant is one expected row of the rebind family: the action id, the
// row text, the anchor and the card link.
type rebindWant struct {
	action string
	text   string
	prop   uuid.UUID
	cardID uuid.UUID
}

// assertRebindRows checks the captured journal rows against the expectation:
// exact order (the source leg precedes the destination leg), each row
// «changed», anchored on its end, the text verbatim, the second run linked
// to the card's own page (ADR 0061 §6).
func assertRebindRows(t *testing.T, want []rebindWant, got []historydomain.Entry) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("journal entries = %d, want %d", len(got), len(want))
	}
	for i, w := range want {
		e := got[i]
		if string(e.Action) != w.action {
			t.Errorf("entry %d action = %s, want %s", i, e.Action, w.action)
		}
		if e.Segments.PlainText() != w.text {
			t.Errorf("entry %d text = %q, want %q", i, e.Segments.PlainText(), w.text)
		}
		if e.PropertyID != w.prop {
			t.Errorf("entry %d property = %s, want %s", i, e.PropertyID, w.prop)
		}
		if e.BaseAction != historydomain.BaseChanged {
			t.Errorf("entry %d base action = %s, want changed", i, e.BaseAction)
		}
		if len(e.Segments) != 2 || e.Segments[1].Link == nil ||
			e.Segments[1].Link.Kind != historydomain.KindContact || e.Segments[1].Link.ID != w.cardID {
			t.Errorf("entry %d segments = %+v, want the linked card run", i, e.Segments)
		}
	}
}

func TestHistory_MoveJournalsBothEnds(t *testing.T) {
	t.Parallel()
	h := newServiceHarness(t, sharedpolicy.RoleOwner)

	// Leg 1 — a move between properties (тикет #856): the pair of rows, one
	// per end — the source object's feed loses the card, the destination's
	// gains it. The ФИО untouched: the rows record the rebinding.
	created, err := h.svc.CreateContact(context.Background(), h.owner, createCmd(&h.property))
	if err != nil {
		t.Fatalf("CreateContact: %v", err)
	}
	h.history.Entries = nil
	if _, err := h.svc.UpdateContact(context.Background(), h.owner, created.ID, UpdateContactCommand{
		PropertyID: &PropertyIDUpdate{Value: &h.otherProp},
	}); err != nil {
		t.Fatalf("UpdateContact (move): %v", err)
	}
	assertRebindRows(t, []rebindWant{
		{string(historydomain.ActionContactMoved), "Контакт перенесён на другой объект: " + createdContactFullName, h.property, created.ID},
		{string(historydomain.ActionContactMoved), "Контакт перенесён с другого объекта: " + createdContactFullName, h.otherProp, created.ID},
	}, h.history.Entries)

	// Leg 2 — the way back with a ФИО edit in the same PATCH: still just the
	// moved pair with the action-time ФИО; a mixed rebind + details action
	// writes no extra contact.updated row (one row per user action per
	// object, ADR 0061 §3).
	newLastName := rebindLastName
	h.history.Entries = nil
	if _, err := h.svc.UpdateContact(context.Background(), h.owner, created.ID, UpdateContactCommand{
		PropertyID: &PropertyIDUpdate{Value: &h.property},
		LastName:   &newLastName,
	}); err != nil {
		t.Fatalf("UpdateContact (move + name edit): %v", err)
	}
	assertRebindRows(t, []rebindWant{
		{string(historydomain.ActionContactMoved), "Контакт перенесён на другой объект: " + reboundContactFullName, h.otherProp, created.ID},
		{string(historydomain.ActionContactMoved), "Контакт перенесён с другого объекта: " + reboundContactFullName, h.property, created.ID},
	}, h.history.Entries)
}

func TestHistory_BindAndUnbindJournalsSingleEnd(t *testing.T) {
	t.Parallel()
	h := newServiceHarness(t, sharedpolicy.RoleOwner)

	// Leg 1 — the unbind (тикет #856): the card leaves the object for the
	// owner's book, the row hangs on the source the card left. The audit
	// fact (property_cleared) alone would never surface it — the feed is
	// the members' view.
	id := h.seedBoundContact(h.property)
	if _, err := h.svc.UpdateContact(context.Background(), h.owner, id, UpdateContactCommand{
		PropertyID: &PropertyIDUpdate{},
	}); err != nil {
		t.Fatalf("UpdateContact (unbind): %v", err)
	}
	assertRebindRows(t, []rebindWant{
		{string(historydomain.ActionContactUnbound), "Контакт отвязан от объекта: " + contactFirstName, h.property, id},
	}, h.history.Entries)

	// Leg 2 — the bind: a previously unbound card lands on the object; the
	// single end is the destination (there is no source to anchor).
	unbound := h.seedUnboundContact()
	h.history.Entries = nil
	if _, err := h.svc.UpdateContact(context.Background(), h.owner, unbound, UpdateContactCommand{
		PropertyID: &PropertyIDUpdate{Value: &h.property},
	}); err != nil {
		t.Fatalf("UpdateContact (bind): %v", err)
	}
	assertRebindRows(t, []rebindWant{
		{string(historydomain.ActionContactBound), "Контакт привязан к объекту: " + contactFirstName, h.property, unbound},
	}, h.history.Entries)

	// Leg 3 — a same-property re-PATCH changes no binding and journals
	// nothing: the binding is not part of the changed-fields comparison
	// (contactChanged), a same-form PATCH stays the §3 no-op.
	h.history.Entries = nil
	if _, err := h.svc.UpdateContact(context.Background(), h.owner, unbound, UpdateContactCommand{
		PropertyID: &PropertyIDUpdate{Value: &h.property},
	}); err != nil {
		t.Fatalf("UpdateContact (same property): %v", err)
	}
	if len(h.history.Entries) != 0 {
		t.Errorf("journal entries after the same-property PATCH = %d, want 0", len(h.history.Entries))
	}
}

func TestHistory_UnbindWithNameEditJournalsSourceRow(t *testing.T) {
	t.Parallel()
	// The ticket's hole (тикет #856): the guard keyed on the card's final
	// binding, so an unbind folded the ФИО edit's row away entirely. The
	// unbind row hangs on the source and carries the action-time ФИО; no
	// extra contact.updated row — one manual action writes one row per
	// affected object (ADR 0061 §3, the mixed-PATCH canon).
	h := newServiceHarness(t, sharedpolicy.RoleOwner)
	id := h.seedBoundContact(h.property)
	newLastName := rebindLastName
	if _, err := h.svc.UpdateContact(context.Background(), h.owner, id, UpdateContactCommand{
		PropertyID: &PropertyIDUpdate{},
		LastName:   &newLastName,
	}); err != nil {
		t.Fatalf("UpdateContact (unbind + name edit): %v", err)
	}
	assertRebindRows(t, []rebindWant{
		{string(historydomain.ActionContactUnbound), "Контакт отвязан от объекта: " + reboundContactFullName, h.property, id},
	}, h.history.Entries)
}

func TestHistory_UnboundContactWritesNothing(t *testing.T) {
	t.Parallel()
	h := newServiceHarness(t, sharedpolicy.RoleOwner)

	created, err := h.svc.CreateContact(context.Background(), h.owner, createCmd(nil))
	if err != nil {
		t.Fatalf("CreateContact (unbound): %v", err)
	}
	if err := h.svc.DeleteContact(context.Background(), h.owner, created.ID); err != nil {
		t.Fatalf("DeleteContact: %v", err)
	}

	if len(h.history.Entries) != 0 {
		t.Errorf("journal entries = %d, want 0 — an unbound card has no property anchor", len(h.history.Entries))
	}
	if len(h.audit.entries) == 0 {
		t.Error("the audit trail must stay complete regardless of the journal scope")
	}
}

func TestHistory_MemberEditAttributedToRole(t *testing.T) {
	t.Parallel()
	h := newServiceHarness(t, sharedpolicy.RoleFullAccess)

	if _, err := h.svc.CreateContact(context.Background(), h.member, createCmd(&h.property)); err != nil {
		t.Fatalf("CreateContact (member): %v", err)
	}

	if len(h.history.Entries) != 1 {
		t.Fatalf("journal entries = %d, want 1", len(h.history.Entries))
	}
	if got := h.history.Entries[0].ActorRole; got != historydomain.ActorRoleFullAccess {
		t.Errorf("actor role = %s, want full_access (the audit attribution pattern)", got)
	}
}
