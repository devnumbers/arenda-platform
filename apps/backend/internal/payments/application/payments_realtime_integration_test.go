//go:build integration

package application_test

// The realtime seam's integration family of the payments context (карта
// #714, #716; ADR 0062): the committed mutations' frames dispatch to the
// recording carrier strictly after the commit — the dispatch-time callback
// reads only committed rows — and a rolled-back mutation dispatches nothing.

import (
	"testing"

	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	realtimetest "github.com/nambers/arenda-planform/apps/backend/internal/realtime/realtimetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreatePaymentPublishesFramesAfterCommit pins the carrier canon on the
// rule creation: one post-commit dispatch carrying the payments pair, the
// tick's operations pair (the creation materializes its first planned in the
// same transaction) and the piggybacked history pair, the actor the author.
func TestCreatePaymentPublishesFramesAfterCommit(t *testing.T) {
	t.Parallel()

	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	seenCommitted := false
	h.realtime.OnPublish = func(realtimetest.Publication) {
		// The dispatch happens strictly after the commit: the created row is
		// already visible to a plain reader outside the mutation's
		// transaction.
		var count int
		if err := h.pool.QueryRow(h.ctx(),
			"SELECT count(*) FROM payments WHERE property_id = $1", h.propID).Scan(&count); err != nil {
			t.Errorf("count payments at dispatch time: %v", err)
			return
		}
		seenCommitted = count == 1
	}

	_, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	require.NoError(t, err)

	require.Len(t, h.realtime.Publications, 1)
	publication := h.realtime.Publications[0]
	assert.Equal(t, h.owner, publication.Actor)
	assert.True(t, seenCommitted, "the dispatch ran after the commit: the row was visible")
	assert.Equal(t,
		[]string{
			"payments:" + h.propID.String(),
			"operations:" + h.propID.String(),
			"history:" + h.propID.String(),
		},
		h.realtime.Pairs())
}

// TestPayOperationPublishesOperationsAndPayments pins the pay step's pairs:
// the paid fact dirties the operations view and — the schedule cursor moves —
// the bound rule's payments view; the journal row piggybacks history.
func TestPayOperationPublishesOperationsAndPayments(t *testing.T) {
	t.Parallel()

	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	require.NoError(t, err)
	h.realtime.Publications = nil

	ops, err := h.ops.ListPaymentOperations(h.ctx(), h.owner, h.propID, created.ID, operationsQueryForRule())
	require.NoError(t, err)
	require.NotEmpty(t, ops)

	_, err = h.ops.PayOperation(h.ctx(), h.owner, h.propID, ops[0].Operation.ID)
	require.NoError(t, err)

	pairs := h.realtime.Pairs()
	assert.Contains(t, pairs, "operations:"+h.propID.String())
	assert.Contains(t, pairs, "payments:"+h.propID.String())
	assert.Contains(t, pairs, "history:"+h.propID.String())
}

// operationsQueryForRule is the bounded query the pay test reads the fresh
// rule's planned operation with.
func operationsQueryForRule() paymentsapp.OperationsListQuery {
	return paymentsapp.OperationsListQuery{Limit: 1}
}

// TestRolledBackMutationPublishesNothing pins the post-commit canon: a
// mutation that fails inside its transaction dispatches no frames.
func TestRolledBackMutationPublishesNothing(t *testing.T) {
	t.Parallel()

	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	cmd := h.createCmd()
	cmd.Recurrence = domain.Recurrence{} // The zero recurrence is the validator's reject.
	_, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, cmd)
	require.Error(t, err)

	assert.Empty(t, h.realtime.Publications)
}
