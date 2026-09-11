package application

// The archived-property immutability audit (ticket #618, карта #611): every
// rentals mutation on an archived property is the read-only state's 409
// (ErrArchivedProperty) with nothing written, synced or ticked; the reads
// stay open. The conveyor's lock is the single guard — these tests pin it
// per use case.

import (
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/rentals/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newArchivedHarness is newHarness over an archived property: the property
// store answers Archived=true from the first mutation lock on. The fake is
// copied into the factory by value, so the flag must be set at construction.
func newArchivedHarness(t *testing.T) *harness {
	t.Helper()
	j := &journal{}
	owner, property := mustID(t), mustID(t)
	store := &fakeRentalStore{journal: j, rentals: map[uuid.UUID]domain.Rental{}}
	gateway := &fakeGateway{
		journal: j,
		state:   RentPaymentState{AmountKopecks: 5_000_000, PaymentDay: domain.MustPaymentDay(15)},
		paid:    3,
	}
	audit := &fakeAudit{journal: j}
	factory := NewTxStoreFactory(
		store, fakePropertyStore{owner: owner, archived: true}, gateway,
		fakeTenantReader{exists: true}, audit, fakeUoW{},
	)
	svc := NewRentalService(factory, fakeCalendar{}, fakePolicy{role: sharedpolicy.RoleOwner})
	return &harness{
		t: t, journal: j, store: store, gateway: gateway, audit: audit,
		svc: svc, owner: owner, property: property,
		propStore: fakePropertyStore{owner: owner, archived: true},
	}
}

func TestArchivedProperty_RejectsEveryMutation(t *testing.T) {
	t.Parallel()

	t.Run("create", func(t *testing.T) {
		t.Parallel()
		h := newArchivedHarness(t)

		_, err := h.svc.CreateRental(t.Context(), h.owner, h.property, createCmd())

		require.ErrorIs(t, err, ErrArchivedProperty)
		assert.Empty(t, h.journal.events, "the 409 fires before any write")
		assert.Zero(t, h.gateway.createSeed, "no managed payment seed for an archived property")
	})

	t.Run("update", func(t *testing.T) {
		t.Parallel()
		h := newArchivedHarness(t)
		rentalID := h.seedRental(nil)

		amount := int64(6_000_000)
		_, err := h.svc.UpdateRental(t.Context(), h.owner, h.property, rentalID, UpdateRentalCommand{AmountKopecks: &amount})

		require.ErrorIs(t, err, ErrArchivedProperty)
		assert.Empty(t, h.journal.events)
	})

	t.Run("complete", func(t *testing.T) {
		t.Parallel()
		h := newArchivedHarness(t)
		rentalID := h.seedRental(nil)

		_, err := h.svc.CompleteRental(t.Context(), h.owner, h.property, rentalID, CompleteRentalCommand{CompletedDate: today})

		require.ErrorIs(t, err, ErrArchivedProperty)
		assert.Empty(t, h.journal.events)
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()
		h := newArchivedHarness(t)
		rentalID := h.seedRental(nil)

		err := h.svc.DeleteRental(t.Context(), h.owner, h.property, rentalID)

		require.ErrorIs(t, err, ErrArchivedProperty)
		assert.Empty(t, h.journal.events)
		assert.False(t, h.gateway.deleted, "the managed payment stays")
	})

	t.Run("reads stay open", func(t *testing.T) {
		t.Parallel()
		h := newArchivedHarness(t)
		rentalID := h.seedRental(nil)

		view, err := h.svc.GetRental(t.Context(), h.owner, h.property, rentalID)

		require.NoError(t, err)
		assert.Equal(t, rentalID, view.Rental.ID)
	})
}
