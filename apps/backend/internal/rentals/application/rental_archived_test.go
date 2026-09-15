package application

// The archived-property immutability audit (ticket #618, карта #611): every
// rentals mutation on an archived property is the read-only state's 409
// (ErrArchivedProperty) with nothing written, synced or ticked; the reads
// stay open. The conveyor's lock is the single guard — these tests pin it
// per use case.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newArchivedHarness is newHarness over an archived property: the property
// store answers Archived=true from the first mutation lock on.
func newArchivedHarness(t *testing.T) *harness {
	t.Helper()
	return newHarnessWithArchivedProperty(t, true)
}

func TestArchivedProperty_RejectsEveryMutation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(t *testing.T, h *harness) error
	}{
		{"create", func(t *testing.T, h *harness) error {
			t.Helper()
			_, err := h.svc.CreateRental(t.Context(), h.owner, h.property, createCmd())
			return err
		}},
		{"update", func(t *testing.T, h *harness) error {
			t.Helper()
			amount := int64(6_000_000)
			_, err := h.svc.UpdateRental(t.Context(), h.owner, h.property, h.seedRental(nil),
				UpdateRentalCommand{AmountKopecks: &amount})
			return err
		}},
		{"complete", func(t *testing.T, h *harness) error {
			t.Helper()
			_, err := h.svc.CompleteRental(t.Context(), h.owner, h.property, h.seedRental(nil),
				CompleteRentalCommand{CompletedDate: today})
			return err
		}},
		{"delete", func(t *testing.T, h *harness) error {
			t.Helper()
			return h.svc.DeleteRental(t.Context(), h.owner, h.property, h.seedRental(nil))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newArchivedHarness(t)

			require.ErrorIs(t, tt.mutate(t, h), ErrArchivedProperty)
			assert.Empty(t, h.journal.events, "the 409 fires before any write")
		})
	}
}

func TestArchivedProperty_ReadsStayOpen(t *testing.T) {
	t.Parallel()
	h := newArchivedHarness(t)
	rentalID := h.seedRental(nil)

	view, err := h.svc.GetRental(t.Context(), h.owner, h.property, rentalID)

	require.NoError(t, err)
	assert.Equal(t, rentalID, view.Rental.ID)
}
