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
	// Новый шаблон (D3): заголовок читает срок — фикс plans 2026-09-04 →
	// 2027-09-01, это 11 полных месяцев (до 04.08.27) и 29 дней остатка.
	// Имя арендатора и период с годами — в деталях.
	want := "Добавлена аренда: 11 месяцев 29 дней"
	if e.Segments.PlainText() != want {
		t.Errorf("row text = %q, want %q", e.Segments.PlainText(), want)
	}
	// Детали: арендатор и период с годами.
	if len(e.Details) != 2 || e.Details[0] != `Арендатор: «Иван Tenant»` || e.Details[1] != "Период: с 4 сентября 2026 по 1 сентября 2027" {
		t.Errorf("details = %q, want the tenant line plus the period line", e.Details)
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

func TestHistory_RentalUpdatedCarriesTheDiffs(t *testing.T) {
	t.Parallel()
	// Единая строка изменения (решение владельца 12.10, карта #1324):
	// «Аренда изменена: [новый срок]», пояснение — дифф периода; арендатор
	// не менялся — его строки нет.
	h := newHarness(t)
	contact := uuid.Must(uuid.NewV7())
	cmd := createCmd()
	cmd.ContactID = &contact

	view, err := h.svc.CreateRental(t.Context(), h.owner, h.property, cmd)
	if err != nil {
		t.Fatalf("CreateRental: %v", err)
	}

	newEnd := mustDate("2027-10-01")
	if _, err := h.svc.UpdateRental(t.Context(), h.owner, h.property, view.Rental.ID,
		UpdateRentalCommand{PlannedEndDate: &DateUpdate{Value: &newEnd}}); err != nil {
		t.Fatalf("UpdateRental: %v", err)
	}

	if len(h.history.Entries) != 2 {
		t.Fatalf("journal entries = %d, want 2", len(h.history.Entries))
	}
	e := h.history.Entries[1]
	if e.Action != historydomain.ActionRentalUpdated {
		t.Fatalf("action = %s, want rental.updated", e.Action)
	}
	// Новый срок 04.09.2026 → 01.10.2027: 12 полных месяцев (до 04.09.27)
	// и 28 дней остатка — канон formatTerm.
	if want := "Аренда изменена: 12 месяцев 28 дней"; e.Segments.PlainText() != want {
		t.Errorf("row text = %q, want %q", e.Segments.PlainText(), want)
	}
	wantDetails := historydomain.DetailLines{
		`Период: «с 4 сентября 2026 по 1 сентября 2027» → «с 4 сентября 2026 по 1 октября 2027»`,
	}
	if len(e.Details) != 1 || e.Details[0] != wantDetails[0] {
		t.Errorf("details = %q, want %q", e.Details, wantDetails)
	}
}
