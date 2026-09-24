package application

// The action journal of the contact mutations (карта #704, тикет #707,
// ADR 0061): a property-bound card journals with the ФИО snapshot — never
// the phone — an unbound card journals nothing (the row anchors to a
// property), a no-op update journals nothing (§3), and the update carries
// the old → new ФИО.

import (
	"context"
	"strings"
	"testing"

	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// createdContactFullName is the ФИО the canonical create fixture produces:
// «Пётр Иванов» (the trimmed first name plus the last name).
const createdContactFullName = contactFirstName + " " + "Иванов"

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
	if len(h.history.entries) != 1 {
		t.Fatalf("journal entries after the no-op update = %d, want 1", len(h.history.entries))
	}

	// A real change carries the old → new ФИО snapshot.
	newLastName := "Сидоров"
	if _, err := h.svc.UpdateContact(context.Background(), h.owner, created.ID, UpdateContactCommand{
		LastName: &newLastName,
	}); err != nil {
		t.Fatalf("UpdateContact: %v", err)
	}
	if err := h.svc.DeleteContact(context.Background(), h.owner, created.ID); err != nil {
		t.Fatalf("DeleteContact: %v", err)
	}

	if len(h.history.entries) != 3 {
		t.Fatalf("journal entries = %d, want 3", len(h.history.entries))
	}
	want := []struct{ action, text string }{
		{string(historydomain.ActionContactCreated), "Добавлен контакт: " + createdContactFullName},
		{string(historydomain.ActionContactUpdated), "Контакт изменён: " + createdContactFullName + " → Пётр Сидоров"},
		{string(historydomain.ActionContactDeleted), "Контакт удалён: Пётр Сидоров"},
	}
	for i, w := range want {
		e := h.history.entries[i]
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
	for _, e := range h.history.entries {
		if text := e.Segments.PlainText(); strings.Contains(text, "89161234567") || strings.Contains(text, "+79161234567") {
			t.Errorf("row text %q must never carry the phone", text)
		}
	}
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

	if len(h.history.entries) != 0 {
		t.Errorf("journal entries = %d, want 0 — an unbound card has no property anchor", len(h.history.entries))
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

	if len(h.history.entries) != 1 {
		t.Fatalf("journal entries = %d, want 1", len(h.history.entries))
	}
	if got := h.history.entries[0].ActorRole; got != historydomain.ActorRoleFullAccess {
		t.Errorf("actor role = %s, want full_access (the audit attribution pattern)", got)
	}
}
