//go:build integration

package application_test

// The realtime seam's integration family of the rentals context (карта
// #714, #716; ADR 0062): the rental lifecycle's frames carry both pairs the
// step wrote — the rental's own view and its managed payment rule's — plus
// the journal row's history piggyback, dispatched strictly post-commit.

import (
	"context"
	"testing"

	rentalsapp "github.com/nambers/arenda-planform/apps/backend/internal/rentals/application"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateRentalPublishesRentalAndPaymentFrames(t *testing.T) {
	t.Parallel()

	h := newRentalsHarness(t)
	h.seedOwner()

	view, err := h.svc.CreateRental(context.Background(), h.owner, h.propID, h.createCmd())
	require.NoError(t, err)
	_ = view

	require.NotEmpty(t, h.realtime.Publications)
	last := h.realtime.Publications[len(h.realtime.Publications)-1]
	assert.Equal(t, h.owner, last.Actor)
	pairs := h.realtime.Pairs()
	assert.Contains(t, pairs, "rentals:"+h.propID.String())
	assert.Contains(t, pairs, "payments:"+h.propID.String(), "the managed payment is born with the rental")
	assert.Contains(t, pairs, "history:"+h.propID.String())
}

func TestCompleteRentalPublishesRentalAndPaymentFrames(t *testing.T) {
	t.Parallel()

	h := newRentalsHarness(t)
	h.seedOwner()

	view := h.createRental()
	h.realtime.Publications = nil

	_, err := h.svc.CompleteRental(context.Background(), h.owner, h.propID, view.Rental.ID,
		rentalsapp.CompleteRentalCommand{CompletedDate: intToday})
	require.NoError(t, err)

	pairs := h.realtime.Pairs()
	assert.Contains(t, pairs, "rentals:"+h.propID.String())
	assert.Contains(t, pairs, "payments:"+h.propID.String(), "the managed payment stopped at the completion date")
	assert.Contains(t, pairs, "history:"+h.propID.String())
}
