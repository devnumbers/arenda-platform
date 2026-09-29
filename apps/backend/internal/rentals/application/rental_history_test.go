package application

// The action journal of the rentals mutations (карта #704, тикет #707,
// ADR 0061): the conveyor journals every manual rental action inside the
// transaction, with the tenant's display name as the label snapshot; the
// period lives in context only (аудит #876).

import (
	"testing"

	"github.com/google/uuid"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	rentalsdomain "github.com/nambers/arenda-planform/apps/backend/internal/rentals/domain"
)

func TestHistory_RentalCreatedCarriesTenantAndPeriod(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	contact := uuid.Must(uuid.NewV7())
	cmd := createCmd()
	cmd.ContactID = &contact

	if _, err := h.svc.CreateRental(t.Context(), h.owner, h.property, cmd); err != nil {
		t.Fatalf("CreateRental: %v", err)
	}

	if len(h.history.Entries) != 1 {
		t.Fatalf("journal entries = %d, want 1", len(h.history.Entries))
	}
	e := h.history.Entries[0]
	if e.Action != historydomain.ActionRentalCreated {
		t.Fatalf("action = %s, want rental.created", e.Action)
	}
	// The label snapshot: the tenant display name (the contacts seam);
	// the period lives in context only (аудит #876 — голые даты из строк
	// убраны). The fixture plans 2026-09-04 (today) → 2027-09-01.
	want := "Добавлена аренда: Иван Tenant"
	if e.Segments.PlainText() != want {
		t.Errorf("row text = %q, want %q", e.Segments.PlainText(), want)
	}
	// The context carries the period as the RFC3339 snapshot of the planned
	// dates — the promise of the test's name beyond the label.
	if v, ok := e.Context["period_from"]; !ok || v != "2026-09-04T00:00:00Z" {
		t.Errorf("context period_from = %v, want the RFC3339 snapshot", v)
	}
	if v, ok := e.Context["period_to"]; !ok || v != "2027-09-01T00:00:00Z" {
		t.Errorf("context period_to = %v, want the RFC3339 snapshot", v)
	}
	if e.PropertyID != h.property || e.ActorID == nil || *e.ActorID != h.owner {
		t.Errorf("entry must carry the scope and the actor, got %s/%v", e.PropertyID, e.ActorID)
	}
}

func TestHistory_RentalDeletedKeepsLabelWithoutLink(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rentalID := h.seedRental(func(r *rentalsdomain.Rental) {
		// Только не начатая аренда удаляется (ErrRentalStarted иначе).
		future := today.AddDate(0, 0, 1)
		r.StartDate = future
	})

	if err := h.svc.DeleteRental(t.Context(), h.owner, h.property, rentalID); err != nil {
		t.Fatalf("DeleteRental: %v", err)
	}
	if len(h.history.Entries) == 0 {
		t.Fatal("the deletion wrote no journal row")
	}
	e := h.history.Entries[len(h.history.Entries)-1]
	if e.Action != historydomain.ActionRentalDeleted {
		t.Fatalf("action = %s, want rental.deleted", e.Action)
	}
	for _, seg := range e.Segments {
		if seg.Link != nil {
			t.Errorf("a deleted rental's row must leave no link, got %+v", seg.Link)
		}
	}
}
