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
		{"created", "Добавлена аренда (01.09.2026 – 01.09.2027)", RentalCreated(id, "", from, to)},
		{"updated", "Условия аренды изменены (01.09.2026 – 01.09.2027)", RentalUpdated(id, "", from, to)},
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

func TestRentalRows_PeriodSegmentAlwaysPresent(t *testing.T) {
	t.Parallel()
	// The period segment is appended unconditionally: formatPeriod is never
	// empty (formatDate is a plain t.Format), the indefinite form included
	// (to = zero → «(01.09.2026)»).
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	e := RentalCreated(uuid.Must(uuid.NewV7()), "Иван Иванов", from, time.Time{})
	want := "Добавлена аренда: Иван Иванов (01.09.2026)"
	if got := e.Segments.PlainText(); got != want {
		t.Errorf("row text = %q, want %q", got, want)
	}
	// The named label keeps the link on the tenant segment.
	if len(e.Segments) != 3 || e.Segments[1].Link == nil {
		t.Errorf("segments = %+v, want the linked tenant run between the prefix and the period", e.Segments)
	}
}
