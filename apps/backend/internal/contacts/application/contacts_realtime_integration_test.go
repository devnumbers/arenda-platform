//go:build integration

package application_test

// The realtime seam's integration family of the contacts context (карта
// #714, #716; ADR 0062): the committed card mutations dispatch their
// contacts frames — the object pair for a bound card, the null-property
// owner-book pair for an unbound one — with the journal row's history
// piggyback, strictly post-commit.

import (
	"testing"

	contactsapp "github.com/nambers/arenda-planform/apps/backend/internal/contacts/application"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateBoundContactPublishesContactsAndHistoryFrames(t *testing.T) {
	t.Parallel()

	h := newContactsHarness(t)
	h.owner = h.seedUser()
	h.property = h.seedProperty(h.owner)

	cmd := contactsapp.CreateContactCommand{
		FirstName: "Пётр",
		LastName:  "Сантехников",
		Role:      "plumber",
		Phone:     "89161234567",
	}
	cmd.PropertyID = &h.property
	_, err := h.svc.CreateContact(h.t.Context(), h.owner, cmd)
	require.NoError(t, err)

	require.Len(t, h.realtime.Publications, 1)
	assert.Equal(t, h.owner, h.realtime.Publications[0].Actor)
	assert.Equal(t,
		[]string{"contacts:" + h.property.String(), "history:" + h.property.String()},
		h.realtime.Pairs())
}

// TestCreateUnboundContactPublishesOwnerBookFrame pins the owner-book pair:
// the unbound card's frame carries no property and journals no history row
// (the schema anchors every journal row to a property).
func TestCreateUnboundContactPublishesOwnerBookFrame(t *testing.T) {
	t.Parallel()

	h := newContactsHarness(t)
	h.owner = h.seedUser()

	cmd := contactsapp.CreateContactCommand{
		FirstName: "Личная",
		LastName:  "Запись",
		Role:      "plumber",
		Phone:     "89161234568",
	}
	_, err := h.svc.CreateContact(h.t.Context(), h.owner, cmd)
	require.NoError(t, err)

	require.Len(t, h.realtime.Publications, 1)
	assert.Equal(t, []string{"contacts:"}, h.realtime.Pairs())
}
