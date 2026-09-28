package domain

// The rental rows with an unnamed tenant: a reachable state — CreateRental
// without a contact and an update nulling the contact (the rentals service
// resolves a nil contact to an empty label). The row must not keep a
// dangling colon, a double space or a link with nothing to attach it to.

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRentalRows_EmptyTenantLabel(t *testing.T) {
	t.Parallel()
	id := uuid.Must(uuid.NewV7())
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2027, 9, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		text string
		e    Entry
	}{
		{"created", "Добавлена аренда", RentalCreated(id, "", from, to)},
		{"updated", "Условия аренды изменены", RentalUpdated(id, "", from, to)},
		{"completed", "Аренда завершена", RentalCompleted(id, "")},
		{"deleted", "Аренда удалена", RentalDeleted(id, "")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.e.Segments.PlainText()
			if got != tc.text {
				t.Errorf("row text = %q, want %q", got, tc.text)
			}
			if strings.Contains(got, "  ") {
				t.Errorf("row text %q has a double space", got)
			}
			// The unnamed label leaves the row without a link — nothing to
			// attach it to (as in the deletion row).
			for _, seg := range tc.e.Segments {
				if seg.Link != nil {
					t.Errorf("segment %q carries a link on an empty label", seg.Text)
				}
			}
			// The context keeps the snapshot as is — the empty label included.
			if v, ok := tc.e.Context["tenant"]; !ok || v != "" {
				t.Errorf("context tenant = %v, want the empty snapshot", v)
			}
		})
	}
}

func TestRentalRows_PeriodLivesInContextOnly(t *testing.T) {
	t.Parallel()
	// Аудит #876: голые подписи дат убраны из строк — период аренды не
	// пишется в сегменты (ни definite, ни бессрочная «(дата)»), он живёт
	// в context. Поиск по датам ленты этим не стирается: даты операций
	// ищутся по created_at, период аренды — по context.
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	e := RentalCreated(uuid.Must(uuid.NewV7()), "Иван Иванов", from, time.Time{})
	want := "Добавлена аренда: Иван Иванов"
	if got := e.Segments.PlainText(); got != want {
		t.Errorf("row text = %q, want %q", got, want)
	}
	// The named label keeps the link on the tenant segment.
	if len(e.Segments) != 2 || e.Segments[1].Link == nil {
		t.Errorf("segments = %+v, want the linked tenant run after the prefix", e.Segments)
	}
	if v, ok := e.Context["period_from"]; !ok || v != "2026-09-01T00:00:00Z" {
		t.Errorf("context period_from = %v, want the RFC3339 snapshot", v)
	}
}

func TestContactRebindRows(t *testing.T) {
	t.Parallel()
	// The rebind family of the dictionary (ADR 0061 §4, тикет #856): a
	// cross-property move speaks of both ends — «на другой объект» from the
	// source, «с другого объекта» at the destination — the bind and the
	// unbind own a single end. All four are «changed», carry the linked ФИО
	// snapshot (never the phone, §5) and the name in the context.
	id := uuid.Must(uuid.NewV7())
	cases := []struct {
		name string
		e    Entry
		text string
	}{
		{"moved from the source", ContactMovedFrom(id, "Пётр Иванов"), "Контакт перенесён на другой объект: Пётр Иванов"},
		{"moved to the destination", ContactMovedTo(id, "Пётр Иванов"), "Контакт перенесён с другого объекта: Пётр Иванов"},
		{"bound", ContactBound(id, "Пётр Иванов"), "Контакт привязан к объекту: Пётр Иванов"},
		{"unbound", ContactUnbound(id, "Пётр Иванов"), "Контакт отвязан от объекта: Пётр Иванов"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.e.Segments.PlainText(); got != tc.text {
				t.Errorf("row text = %q, want %q", got, tc.text)
			}
			if tc.e.Kind != KindContact || tc.e.BaseAction != BaseChanged {
				t.Errorf("kind/base = %s/%s, want contact/changed", tc.e.Kind, tc.e.BaseAction)
			}
			if len(tc.e.Segments) != 2 || tc.e.Segments[1].Link == nil ||
				tc.e.Segments[1].Link.Kind != KindContact || tc.e.Segments[1].Link.ID != id {
				t.Errorf("segments = %+v, want the linked card run", tc.e.Segments)
			}
			if v, ok := tc.e.Context["name"]; !ok || v != "Пётр Иванов" {
				t.Errorf("context name = %v, want the ФИО snapshot", v)
			}
		})
	}
}
