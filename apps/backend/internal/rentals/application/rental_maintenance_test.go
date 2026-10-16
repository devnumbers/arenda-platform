package application

// The maintenance guard (ticket #1050, карта #1047): every mutation of an
// unfinished rental on a property under maintenance is the honest 409
// (ErrPropertyMaintenance) with nothing written, synced or ticked — the
// front hides the CTAs, the conveyor is the backstop for the direct API
// call. The reads stay open, and the guard is scoped to the UNFINISHED
// rental: a completed one (the history) still deletes as before, and the
// rescue hatch — «Завершить ремонт» → the property turns active again —
// makes every mutation reachable without a special case.

import (
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/rentals/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMaintenanceHarness is newHarness over a maintenance property: the
// property store answers Maintenance=true from the first mutation lock on.
func newMaintenanceHarness(t *testing.T) *harness {
	t.Helper()
	return newHarnessWithPropertyFlags(t, false, true)
}

func TestMaintenanceProperty_RejectsEveryMutationOfUnfinishedRental(t *testing.T) {
	t.Parallel()

	for _, tt := range lifecycleGuardMutations() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newMaintenanceHarness(t)

			require.ErrorIs(t, tt.mutate(t, h), ErrPropertyMaintenance)
			assert.Empty(t, h.journal.events, "the 409 fires before any write")
		})
	}
}

// The history escapes the guard: a completed rental on a maintenance
// property deletes as before (просмотр и удаление завершённых аренд на
// ремонте не блокируются, решение владельца 02.10) — the deletion runs the
// full conveyor, so the discipline stays observable in the journal.
func TestMaintenanceProperty_CompletedRentalDeletesAsBefore(t *testing.T) {
	t.Parallel()
	h := newMaintenanceHarness(t)
	completed := today
	rentalID := h.seedRental(func(r *domain.Rental) { r.CompletedDate = &completed })

	require.NoError(t, h.svc.DeleteRental(t.Context(), h.owner, h.property, rentalID))

	assert.NotContains(t, h.store.rentals, rentalID)
}

func TestMaintenanceProperty_ReadsStayOpen(t *testing.T) {
	t.Parallel()
	h := newMaintenanceHarness(t)
	rentalID := h.seedRental(nil)

	view, err := h.svc.GetRental(t.Context(), h.owner, h.property, rentalID)

	require.NoError(t, err)
	assert.Equal(t, rentalID, view.Rental.ID)
}
