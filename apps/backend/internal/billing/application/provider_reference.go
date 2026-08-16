package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// saveProviderReference atomically persists the provider's payment id of a
// successful initiation — one implementation for both payment paths (issue
// #285): the customer-initiated tariff-change payment (CIT) and the
// merchant-initiated renewal (MIT) differ only in the initiation result they
// pass and in what they map the returned payment to, never in the write
// itself. The payment is locked, a payment that was finalized (a concurrent
// webhook won the race) or already referenced (an earlier initiation's save
// survived) wins with its persisted state — the provider's payment id, once
// known, never changes — and the reference with its payer-facing URL lands
// in the same transaction as the aggregate update, so a crash between the
// provider call and this save leaves the pending payment recoverable: the
// next run re-initiates idempotently, keyed by the internal payment id. The
// saved payment is returned for each path's own continuation — the CIT flow
// maps it to the payer's confirm URL, the MIT flow charges on it.
func saveProviderReference(
	ctx context.Context,
	runTx func(context.Context, func(*txStores) error) error,
	paymentID uuid.UUID,
	initRes InitPaymentResult,
	now time.Time,
) (domain.SubscriptionPayment, error) {
	var saved domain.SubscriptionPayment
	err := runTx(ctx, func(stores *txStores) error {
		payment, err := stores.paymentForUpdate(ctx, paymentID)
		if err != nil {
			return err
		}
		if payment.IsFinalized() || payment.HasProviderReference() {
			saved = payment
			return nil
		}
		if err := payment.SaveProviderReference(initRes.ProviderPaymentID, initRes.PaymentURL, now); err != nil {
			return err
		}
		if err := stores.payments.Update(ctx, payment); err != nil {
			return fmt.Errorf("persist provider init result: %w", err)
		}
		saved = payment
		return nil
	})
	if err != nil {
		return domain.SubscriptionPayment{}, err
	}
	return saved, nil
}
