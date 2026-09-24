//go:build integration

package wire

// The reverse half of the rentals↔payments seam (ADR 0053 §3, ticket #818):
// the payments rule mutations of a rental-managed payment are rejected — the
// rent payment is created, edited and deleted only through the rental
// (rentals/CONTEXT.md «Платёж арендной платы»). The gate never touches the
// payment facts («Оплатить» stays the canon of marking a rent month paid)
// nor the favorite star, and the rental pipeline's own sync (the
// RentPaymentGateway) runs past it by construction — the same green pair the
// seam family above proves.

import (
	"context"
	"testing"

	"github.com/google/uuid"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	paymentsdomain "github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	rentalsapp "github.com/nambers/arenda-planform/apps/backend/internal/rentals/application"
	rentalsdomain "github.com/nambers/arenda-planform/apps/backend/internal/rentals/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createGateRental starts on seamToday with the payment day on the start day:
// the in-transaction tick materializes the due occurrence, so the pay-fact
// canon has its target right away.
func (h *seamHarness) createGateRental() rentalsapp.RentalView {
	t := h.t
	t.Helper()
	view, err := h.svc.CreateRental(context.Background(), h.owner, h.propID,
		rentalsapp.CreateRentalCommand{
			AmountKopecks: 5_000_000,
			PaymentDay:    rentalsdomain.MustPaymentDay(seamToday.Day()),
			StartDate:     seamToday,
			Utilities:     rentalsdomain.UtilitiesIncluded,
			AutoPay:       true,
		})
	require.NoError(t, err)
	return view
}

// createOrdinaryPayment is the plain payments rule of the same property —
// no rental behind it.
func (h *seamHarness) createOrdinaryPayment() paymentsdomain.Payment {
	t := h.t
	t.Helper()
	slug := "internet"
	created, err := h.paySvc.CreatePayment(context.Background(), h.owner, h.propID,
		paymentsapp.CreatePaymentCommand{
			Type:          paymentsdomain.TypeExpense,
			Title:         "Интернет",
			AmountKopecks: 50_000,
			Recurrence:    paymentsdomain.NewDailyRecurrence(),
			PaymentForm:   paymentsdomain.FormTransfer,
			CategorySlug:  slug,
		})
	require.NoError(t, err)
	return created
}

func TestGate_RentalManagedPaymentRejectsRuleMutations(t *testing.T) {
	t.Parallel()
	h := newSeamHarness(t)
	h.seedOwner()
	paymentID := h.createGateRental().Rental.PaymentID
	ctx := context.Background()

	// The domain rule precedes the rule's own state: resume of a rule that
	// was never paused is still the rental-managed 409, not ErrNotPaused.
	title := "Новое название"
	_, err := h.paySvc.UpdatePayment(ctx, h.owner, h.propID, paymentID,
		paymentsapp.UpdatePaymentCommand{Title: &title})
	require.ErrorIs(t, err, paymentsapp.ErrRentManagedPayment)
	_, err = h.paySvc.PausePayment(ctx, h.owner, h.propID, paymentID)
	require.ErrorIs(t, err, paymentsapp.ErrRentManagedPayment)
	_, err = h.paySvc.ResumePayment(ctx, h.owner, h.propID, paymentID)
	require.ErrorIs(t, err, paymentsapp.ErrRentManagedPayment)
	err = h.paySvc.DeletePayment(ctx, h.owner, h.propID, paymentID, true)
	require.ErrorIs(t, err, paymentsapp.ErrRentManagedPayment)

	// Nothing happened: the rule is whole, no pause row exists.
	payment, err := h.paySvc.GetPayment(ctx, h.owner, h.propID, paymentID)
	require.NoError(t, err)
	assert.Equal(t, "Арендная плата", payment.Title)
	assert.Empty(t, payment.Pauses)

	// The read flag names the managed rule; the ordinary one stays false.
	managed, err := h.paySvc.RentalManagedStatus(ctx, h.owner, h.propID, paymentID)
	require.NoError(t, err)
	assert.True(t, managed)
}

func TestGate_ManagedPaymentKeepsFactsAndFavorite(t *testing.T) {
	t.Parallel()
	h := newSeamHarness(t)
	h.seedOwner()
	paymentID := h.createGateRental().Rental.PaymentID
	ctx := context.Background()

	// «Оплатить» — канон отметки оплаты месяца аренды: платёжный факт
	// гасится мимо гейта.
	planned := paymentsdomain.ViewStatusPlanned
	ops, err := h.ops.ListPaymentOperations(ctx, h.owner, h.propID, paymentID,
		paymentsapp.OperationsListQuery{Status: &planned, Today: seamToday, Limit: 1})
	require.NoError(t, err)
	require.Len(t, ops, 1)
	_, err = h.ops.PayOperation(ctx, h.owner, h.propID, ops[0].Operation.ID)
	require.NoError(t, err)

	// Звезда избранного — не условие аренды: гейт её не трогает.
	_, err = h.paySvc.SetPaymentFavorite(ctx, h.owner, h.propID, paymentID, true)
	require.NoError(t, err)

	// The list read carries the flag per rule.
	_, statuses, err := h.paySvc.PaymentFlags(ctx, h.owner, h.propID)
	require.NoError(t, err)
	assert.Equal(t, map[uuid.UUID]bool{paymentID: true}, statuses)
}

func TestGate_CompletedRentalKeepsThePaymentGated(t *testing.T) {
	t.Parallel()
	h := newSeamHarness(t)
	h.seedOwner()
	view := h.createGateRental()
	paymentID := view.Rental.PaymentID
	ctx := context.Background()

	// Completion is the rental pipeline's own legal sync — it lands past
	// the gate (Stop writes through the gateway).
	_, err := h.svc.CompleteRental(ctx, h.owner, h.propID, view.Rental.ID,
		rentalsapp.CompleteRentalCommand{CompletedDate: seamToday})
	require.NoError(t, err)

	// The rental is final — and with it its payment: no edit path exists
	// anywhere, so the rule mutations stay rejected (the honest 409 the
	// RESTRICT FK always implied for deletion).
	title := "Новое название"
	_, err = h.paySvc.UpdatePayment(ctx, h.owner, h.propID, paymentID,
		paymentsapp.UpdatePaymentCommand{Title: &title})
	require.ErrorIs(t, err, paymentsapp.ErrRentManagedPayment)
	_, err = h.paySvc.PausePayment(ctx, h.owner, h.propID, paymentID)
	require.ErrorIs(t, err, paymentsapp.ErrRentManagedPayment)
	err = h.paySvc.DeletePayment(ctx, h.owner, h.propID, paymentID, true)
	require.ErrorIs(t, err, paymentsapp.ErrRentManagedPayment)

	managed, err := h.paySvc.RentalManagedStatus(ctx, h.owner, h.propID, paymentID)
	require.NoError(t, err)
	assert.True(t, managed, "a completed rental still owns its payment's fate")

	// The legal deletion path: delete the rental — the payment goes with it
	// (keep_overdue semantics), the gate never saw it.
	require.NoError(t, h.svc.DeleteRental(ctx, h.owner, h.propID, view.Rental.ID))
	_, err = h.paySvc.GetPayment(ctx, h.owner, h.propID, paymentID)
	require.ErrorIs(t, err, paymentsapp.ErrNotFound)
}

func TestGate_OrdinaryPaymentStaysMutable(t *testing.T) {
	t.Parallel()
	h := newSeamHarness(t)
	h.seedOwner()
	created := h.createOrdinaryPayment()
	ctx := context.Background()

	title := "Интернет по тарифу"
	updated, err := h.paySvc.UpdatePayment(ctx, h.owner, h.propID, created.ID,
		paymentsapp.UpdatePaymentCommand{Title: &title})
	require.NoError(t, err)
	assert.Equal(t, title, updated.Title)
	_, err = h.paySvc.PausePayment(ctx, h.owner, h.propID, created.ID)
	require.NoError(t, err)
	_, err = h.paySvc.ResumePayment(ctx, h.owner, h.propID, created.ID)
	require.NoError(t, err)

	// The flag read names the ordinary rule false.
	_, statuses, err := h.paySvc.PaymentFlags(ctx, h.owner, h.propID)
	require.NoError(t, err)
	assert.Equal(t, map[uuid.UUID]bool{created.ID: false}, statuses)

	require.NoError(t, h.paySvc.DeletePayment(ctx, h.owner, h.propID, created.ID, true))
}
