//go:build integration

package application_test

// The realtime seam's integration family of the tasks context (карта #714,
// #716; ADR 0062): the committed mutations dispatch their tasks frames —
// the object pair, the owner-book pair for the property-less slice, the
// per-property pairs of the book-wide journal clear — strictly after the
// commit, with the journal rows piggybacking their history pairs.

import (
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateRulePublishesTasksAndHistoryFrames(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwner("Europe/Moscow")

	_, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, h.createCmd())
	require.NoError(t, err)

	require.Len(t, h.realtime.Publications, 1)
	assert.Equal(t, h.owner, h.realtime.Publications[0].Actor)
	assert.Equal(t,
		[]string{"tasks:" + h.propID.String(), "history:" + h.propID.String()},
		h.realtime.Pairs())
}

// TestOwnerBookMutationPublishesNullPropertyFrame pins the owner-book pair:
// the property-less rule's frame carries no property — its audience is the
// actor alone (ADR 0052, ADR 0062 §2).
func TestOwnerBookMutationPublishesNullPropertyFrame(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwnerWithoutProperty()

	_, err := h.rules.CreateRuleWithoutProperty(h.ctx(), h.owner, h.createCmd())
	require.NoError(t, err)

	require.Len(t, h.realtime.Publications, 1)
	assert.Equal(t, []string{"tasks:"}, h.realtime.Pairs(),
		"the owner-book frame's propertyId is null")
}

// TestBookWideClearPublishesPerPropertyFrames pins the bulk canon on the
// «Удалить все выполненные» book-wide clear: one owner-book tasks frame plus
// a tasks+history pair per touched object — one frame per pair, not N.
func TestBookWideClearPublishesPerPropertyFrames(t *testing.T) {
	t.Parallel()

	h := newTasksHarness(t).withOwner(taskMoscowTZ)

	// A dead rule's completed journal is the clear's target (ADR 0051): the
	// rule dies, its completed rows lose the rule_id and the book-wide clear
	// removes them.
	rule := h.seedRule(aug27, "", string(domain.RepeatOnce), "Полить")
	h.runTick()
	h.completeAllBut(rule)
	require.NoError(t, h.rules.DeleteRule(h.ctx(), h.owner, h.propID, rule))
	h.realtime.Publications = nil

	_, err := h.tasks.ClearCompletedJournalOwnerBook(h.ctx(), h.owner)
	require.NoError(t, err)

	pairs := h.realtime.Pairs()
	assert.Contains(t, pairs, "tasks:", "the owner's book list is dirty too")
	assert.Contains(t, pairs, "tasks:"+h.propID.String())
	assert.Contains(t, pairs, "history:"+h.propID.String())
}
