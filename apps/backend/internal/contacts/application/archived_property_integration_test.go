//go:build integration

package application_test

// The contacts exception of the archived-property immutability audit
// (ticket #618, карта #611): the owner's book stays editable even when the
// card is bound to an archived property (ADR 0054) — create, update and
// delete all pass where every property-data context would answer 409.

import (
	"errors"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/contacts/application"
	"github.com/stretchr/testify/require"
)

func TestContactsIntegration_ArchivedBindingStaysEditable(t *testing.T) {
	t.Parallel()

	h := newContactsHarness(t)
	h.owner = h.seedUser()
	propID := h.seedProperty(h.owner)
	if _, err := h.pool.Exec(t.Context(),
		`UPDATE properties SET status = 'archived' WHERE id = $1`, propID,
	); err != nil {
		t.Fatalf("archive property: %v", err)
	}
	ctx := t.Context()

	bound := propID
	cmd := createCmd()
	cmd.PropertyID = &bound
	created := h.create(cmd)
	require.NotNil(t, created.PropertyID)
	require.Equal(t, propID, *created.PropertyID)

	newRole := "прораб"
	updated, err := h.svc.UpdateContact(ctx, h.owner, created.ID, application.UpdateContactCommand{
		Role: &newRole,
	})
	require.NoError(t, err)
	require.Equal(t, newRole, updated.Role)

	require.NoError(t, h.svc.DeleteContact(ctx, h.owner, created.ID))
	if _, err := h.svc.GetContact(ctx, h.owner, created.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("get after delete: want ErrNotFound, got %v", err)
	}

	want := []string{"contact.created", "contact.updated", "contact.deleted"}
	actions := h.contactActions(t, created.ID)
	require.Len(t, actions, len(want))
	for i := range want {
		require.Equal(t, want[i], actions[i])
	}
}
