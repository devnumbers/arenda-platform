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
		FirstName: contactFirstName,
		LastName:  contactLastName,
		Role:      plumberRole,
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
		Role:      plumberRole,
		Phone:     "89161234568",
	}
	_, err := h.svc.CreateContact(h.t.Context(), h.owner, cmd)
	require.NoError(t, err)

	require.Len(t, h.realtime.Publications, 1)
	assert.Equal(t, []string{"contacts:"}, h.realtime.Pairs())
}

// TestMoveContactPublishesSourceAndDestinationFrames pins the move's both
// ends: the origin object's card grid loses the row and the destination's
// gains it, so one dispatch carries both contacts pairs; the journal pair
// (contact.moved, one leg per end — тикет #856) anchors the history on both
// ends of the move (ADR 0061 §4) (карта #714, #716).
func TestMoveContactPublishesSourceAndDestinationFrames(t *testing.T) {
	t.Parallel()

	h := newContactsHarness(t)
	h.owner = h.seedUser()
	h.property = h.seedProperty(h.owner)
	destination := h.seedPropertyNamed(h.owner, "Дача")

	cmd := contactsapp.CreateContactCommand{
		FirstName: contactFirstName,
		LastName:  contactLastName,
		Role:      plumberRole,
		Phone:     "89161234567",
	}
	cmd.PropertyID = &h.property
	card, err := h.svc.CreateContact(h.t.Context(), h.owner, cmd)
	require.NoError(t, err)
	h.realtime.Publications = nil

	_, err = h.svc.UpdateContact(h.t.Context(), h.owner, card.ID, contactsapp.UpdateContactCommand{
		PropertyID: &contactsapp.PropertyIDUpdate{Value: &destination},
	})
	require.NoError(t, err)

	assert.Equal(t,
		[]string{
			"contacts:" + h.property.String(),
			"contacts:" + destination.String(),
			"history:" + h.property.String(),
			"history:" + destination.String(),
		},
		h.realtime.Pairs())
}

// TestUnbindContactPublishesSourceAndOwnerBookFrames pins the unbind: the
// origin object's pair for the leaving row plus the owner-book pair of the
// now-unbound card; the unbind journals its contact.unbound row on the
// source — the single existing end (ADR 0061 §4, тикет #856) — so that one
// history pair rides along; §3's «unbound card writes no rows» stays about
// cards without any binding (карта #714, #716).
func TestUnbindContactPublishesSourceAndOwnerBookFrames(t *testing.T) {
	t.Parallel()

	h := newContactsHarness(t)
	h.owner = h.seedUser()
	h.property = h.seedProperty(h.owner)

	cmd := contactsapp.CreateContactCommand{
		FirstName: contactFirstName,
		LastName:  contactLastName,
		Role:      plumberRole,
		Phone:     "89161234569",
	}
	cmd.PropertyID = &h.property
	card, err := h.svc.CreateContact(h.t.Context(), h.owner, cmd)
	require.NoError(t, err)
	h.realtime.Publications = nil

	_, err = h.svc.UpdateContact(h.t.Context(), h.owner, card.ID, contactsapp.UpdateContactCommand{
		PropertyID: &contactsapp.PropertyIDUpdate{Value: nil},
	})
	require.NoError(t, err)

	assert.Equal(t,
		[]string{"contacts:" + h.property.String(), "contacts:", "history:" + h.property.String()},
		h.realtime.Pairs())
}
