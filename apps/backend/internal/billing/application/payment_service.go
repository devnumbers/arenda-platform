package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

// PaymentService lists and acts on subscription payments.
type PaymentService struct {
	deps flowDeps
}

// NewPaymentService creates a PaymentService.
func NewPaymentService(deps flowDeps) *PaymentService {
	return &PaymentService{deps: deps}
}

// ListPayments returns all subscription payments for the user with their tariffs.
func (s *PaymentService) ListPayments(ctx context.Context, userID uuid.UUID) ([]SubscriptionPaymentView, error) {
	payments, err := s.deps.subscriptionPayments.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
	}

	tariffs, err := s.deps.tariffs.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tariffs: %w", err)
	}
	tariffByID := make(map[uuid.UUID]domain.Tariff, len(tariffs))
	for _, t := range tariffs {
		tariffByID[t.ID] = t
	}

	views := make([]SubscriptionPaymentView, 0, len(payments))
	for _, p := range payments {
		t, ok := tariffByID[p.TariffID]
		if !ok {
			return nil, fmt.Errorf("payment %s references unknown tariff %s", p.ID, p.TariffID)
		}
		views = append(views, SubscriptionPaymentView{Payment: p, Tariff: t})
	}
	return views, nil
}

// GetPayment returns a single subscription payment for admin view.
func (s *PaymentService) GetPayment(ctx context.Context, paymentID uuid.UUID) (AdminSubscriptionPaymentView, error) {
	p, err := s.deps.subscriptionPayments.GetByIDAdmin(ctx, paymentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return AdminSubscriptionPaymentView{}, ErrPaymentNotFound
		}
		return AdminSubscriptionPaymentView{}, fmt.Errorf("get payment: %w", err)
	}

	tariff, err := s.deps.tariffs.GetByID(ctx, p.Payment.TariffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return AdminSubscriptionPaymentView{}, ErrTariffNotFound
		}
		return AdminSubscriptionPaymentView{}, fmt.Errorf("get payment tariff: %w", err)
	}

	return AdminSubscriptionPaymentView{
		Payment:   p.Payment,
		Tariff:    tariff,
		UserPhone: p.UserPhone,
	}, nil
}

// ListAllPayments returns all subscription payments for admin view.
func (s *PaymentService) ListAllPayments(ctx context.Context, filters ListAllPaymentsFilters) ([]AdminSubscriptionPaymentView, int64, error) {
	if filters.Limit <= 0 {
		filters.Limit = 20
	}
	if filters.Limit > 100 {
		filters.Limit = 100
	}
	if filters.Offset < 0 {
		filters.Offset = 0
	}

	if filters.Status != "" &&
		filters.Status != string(domain.PaymentStatusPending) &&
		filters.Status != string(domain.PaymentStatusSucceeded) &&
		filters.Status != string(domain.PaymentStatusFailed) &&
		filters.Status != string(domain.PaymentStatusRefunded) &&
		filters.Status != string(domain.PaymentStatusPartialRefunded) {
		return nil, 0, fmt.Errorf("%w: invalid status filter", ErrInvalidFilter)
	}

	payments, total, err := s.deps.subscriptionPayments.ListAll(ctx, filters.Status, filters.UserID, filters.Limit, filters.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list all payments: %w", err)
	}

	tariffs, err := s.deps.tariffs.List(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list tariffs: %w", err)
	}
	tariffByID := make(map[uuid.UUID]domain.Tariff, len(tariffs))
	for _, t := range tariffs {
		tariffByID[t.ID] = t
	}

	views := make([]AdminSubscriptionPaymentView, 0, len(payments))
	for _, p := range payments {
		t, ok := tariffByID[p.Payment.TariffID]
		if !ok {
			return nil, 0, fmt.Errorf("payment %s references unknown tariff %s", p.Payment.ID, p.Payment.TariffID)
		}
		views = append(views, AdminSubscriptionPaymentView{
			Payment:   p.Payment,
			Tariff:    t,
			UserPhone: p.UserPhone,
		})
	}

	return views, total, nil
}

// ConfirmFakePayment confirms a previously initialized fake payment and applies its result.
func (s *PaymentService) ConfirmFakePayment(ctx context.Context, paymentID uuid.UUID) error {
	tx, err := s.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	payment, err := s.deps.subscriptionPayments.WithTx(tx).GetByIDForUpdate(ctx, paymentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrPaymentNotFound
		}
		return fmt.Errorf("get payment: %w", err)
	}

	if payment.Status == domain.PaymentStatusSucceeded || payment.Status == domain.PaymentStatusFailed {
		if payment.Status == domain.PaymentStatusSucceeded {
			applySubscriptionRenewalAndArchive(ctx, s.deps, payment)
		}
		return nil
	}

	cp, ok := s.deps.provider.(ConfirmableProvider)
	if !ok {
		return ErrProviderNotConfirmable
	}

	payload, err := cp.ConfirmPayment(ctx, payment.ID.String())
	if err != nil {
		return fmt.Errorf("confirm fake payment: %w", err)
	}

	if err := applyPaymentResult(ctx, s.deps, tx, &payment, payload); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit fake payment confirmation transaction: %w", err)
	}

	applySubscriptionRenewalAndArchive(ctx, s.deps, payment)
	return nil
}

// RefundPayment cancels/refunds a succeeded or pending subscription payment
// through the provider and immediately downgrades the subscription to basic.
// The system always refunds the full payment amount. The provider HTTP call is
// made outside of any database transaction so a slow provider cannot hold a row
// lock for an unbounded time.
func (s *PaymentService) RefundPayment(ctx context.Context, paymentID uuid.UUID) error {
	// First short transaction: load the payment with a row lock, validate that it
	// can be refunded, and commit immediately.
	tx, err := s.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin refund payment transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	payment, err := s.deps.subscriptionPayments.WithTx(tx).GetByIDForUpdate(ctx, paymentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrPaymentNotFound
		}
		return fmt.Errorf("get payment for refund: %w", err)
	}

	if payment.Status != domain.PaymentStatusSucceeded && payment.Status != domain.PaymentStatusPending {
		return fmt.Errorf("%w: cannot refund payment with status %s", domain.ErrInvalidPaymentStatus, payment.Status)
	}

	// The system always refunds the full payment amount.
	refundAmount := payment.AmountKopecks

	if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID == "" {
		return fmt.Errorf("%w: payment has no provider payment id", domain.ErrInvalidPaymentStatus)
	}

	providerPaymentID := *payment.ProviderPaymentID
	userID := payment.UserID
	subscriptionID := payment.SubscriptionID

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit refund validation transaction: %w", err)
	}

	// Provider call happens outside the transaction so the database connection is
	// not held during an unbounded external HTTP request.
	cancelRes, err := s.deps.provider.Cancel(ctx, CancelRequest{
		PaymentID:         paymentID,
		ProviderPaymentID: providerPaymentID,
		AmountKopecks:     refundAmount,
	})
	if err != nil {
		s.deps.log.ErrorContext(ctx, "provider cancel failed",
			slog.String("payment_id", paymentID.String()),
			slog.String("subscription_id", subscriptionID.String()),
			slog.String("user_id", userID.String()),
			slog.Int64("refund_amount_kopecks", refundAmount),
			slog.String("error", sanitize.Error(err)))
		return fmt.Errorf("provider cancel: %w", err)
	}

	if cancelRes.Status != domain.PaymentStatusRefunded && cancelRes.Status != domain.PaymentStatusPartialRefunded {
		return fmt.Errorf("provider cancel returned non-refund status: %s", cancelRes.Status)
	}

	// Second short transaction: reload the payment under lock, re-check that it is
	// still succeeded (it may have been refunded by a concurrent webhook), apply
	// the refund and downgrade the subscription.
	resultTx, err := s.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin refund result transaction: %w", err)
	}
	defer func() { _ = resultTx.Rollback(ctx) }()

	payment, err = s.deps.subscriptionPayments.WithTx(resultTx).GetByIDForUpdate(ctx, paymentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrPaymentNotFound
		}
		return fmt.Errorf("get payment for refund result: %w", err)
	}

	if payment.Status != domain.PaymentStatusSucceeded && payment.Status != domain.PaymentStatusPending {
		return fmt.Errorf("%w: payment status changed to %s during refund", domain.ErrInvalidPaymentStatus, payment.Status)
	}

	if err := s.deps.subscriptionPayments.WithTx(resultTx).MarkRefunded(ctx, payment.ID, s.deps.clock.Now().UTC()); err != nil {
		return fmt.Errorf("mark payment refunded: %w", err)
	}

	if err := applyRefundToSubscription(ctx, s.deps, resultTx, payment.SubscriptionID); err != nil {
		return err
	}

	if err := resultTx.Commit(ctx); err != nil {
		return fmt.Errorf("commit refund payment transaction: %w", err)
	}

	s.deps.log.InfoContext(ctx, "payment refunded",
		slog.String("payment_id", paymentID.String()),
		slog.String("subscription_id", subscriptionID.String()),
		slog.String("user_id", userID.String()),
		slog.Int64("refund_amount_kopecks", cancelRes.RefundedAmountKopecks))

	return nil
}

// SyncPendingPayment queries the provider for the current status of a single
// pending subscription payment and finalizes it based on the response.
//
// Succeeded payments are marked as such and the subscription renewal/tariff
// change is applied best-effort afterwards. Because T-Kassa GetState does not
// return card tokens, a synthetic WebhookPayload without card data is used;
// no new payment method is saved on this path.
//
// Failed payments are marked as failed. If the payment was for the current
// subscription tariff (a renewal), the subscription is moved to a grace period.
// Upgrade payments that fail leave the subscription on its current tariff.
//
// Payments that are not in pending status cannot be synced and return an
// invalid-status error.
func (s *PaymentService) SyncPendingPayment(ctx context.Context, paymentID uuid.UUID) error {
	payment, err := s.deps.subscriptionPayments.GetByID(ctx, paymentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrPaymentNotFound
		}
		return fmt.Errorf("get payment: %w", err)
	}

	if payment.Status != domain.PaymentStatusPending {
		return fmt.Errorf("%w: cannot sync payment with status %s", domain.ErrInvalidPaymentStatus, payment.Status)
	}
	if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID == "" {
		return fmt.Errorf("%w: payment has no provider payment id", domain.ErrInvalidPaymentStatus)
	}

	status, err := s.deps.provider.Status(ctx, paymentID, *payment.ProviderPaymentID)
	if err != nil {
		return fmt.Errorf("provider status: %w", err)
	}

	switch status {
	case domain.PaymentStatusPending:
		// Nothing to finalize. The provider.Status contract only returns the
		// status, so a missing provider payment id cannot be recovered here.
		return nil
	case domain.PaymentStatusSucceeded, domain.PaymentStatusFailed, domain.PaymentStatusRefunded, domain.PaymentStatusPartialRefunded:
		return s.finalizeSyncedPayment(ctx, payment, status)
	default:
		return fmt.Errorf("unexpected provider status: %s", status)
	}
}

func (s *PaymentService) finalizeSyncedPayment(ctx context.Context, payment domain.SubscriptionPayment, status domain.PaymentStatus) error {
	tx, err := s.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	payment, err = s.deps.subscriptionPayments.WithTx(tx).GetByIDForUpdate(ctx, payment.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrPaymentNotFound
		}
		return fmt.Errorf("get payment for update: %w", err)
	}

	if payment.Status != domain.PaymentStatusPending {
		// A concurrent webhook or another sync already finalized the payment.
		return nil
	}

	payload := WebhookPayload{
		ProviderPaymentID: *payment.ProviderPaymentID,
		InternalPaymentID: payment.ID,
		Status:            status,
		AmountKopecks:     payment.AmountKopecks,
	}

	if err := applyPaymentResult(ctx, s.deps, tx, &payment, payload); err != nil {
		return err
	}

	switch status {
	case domain.PaymentStatusFailed:
		sub, err := s.deps.subscriptions.WithTx(tx).GetByIDForUpdate(ctx, payment.SubscriptionID)
		if err != nil {
			return fmt.Errorf("get subscription for failed payment sync: %w", err)
		}
		if sub.TariffID == payment.TariffID {
			transitionToGrace(&sub, s.deps.clock.Now().UTC())
			if err := s.deps.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
				return fmt.Errorf("transition subscription to grace after failed payment sync: %w", err)
			}
		}

	case domain.PaymentStatusRefunded, domain.PaymentStatusPartialRefunded:
		if err := applyRefundToSubscription(ctx, s.deps, tx, payment.SubscriptionID); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit sync payment transaction: %w", err)
	}

	if status == domain.PaymentStatusSucceeded {
		applySubscriptionRenewalAndArchive(ctx, s.deps, payment)
	}

	return nil
}

// ReconcilePendingPayments checks all pending subscription payments that have
// been stuck longer than the staleness threshold with the provider in batches
// and finalizes them via SyncPendingPayment. Returns the number of payments
// successfully synced.
func (s *PaymentService) ReconcilePendingPayments(ctx context.Context, now time.Time) (int, error) {
	processed := 0
	createdBefore := now.Add(-pendingPaymentStalenessThreshold).UTC()

	for {
		payments, err := s.deps.subscriptionPayments.ListPendingPayments(ctx, createdBefore, renewalBatchSize)
		if err != nil {
			return processed, fmt.Errorf("list pending payments: %w", err)
		}
		if len(payments) == 0 {
			break
		}

		for _, payment := range payments {
			if err := s.SyncPendingPayment(ctx, payment.ID); err != nil {
				s.deps.log.ErrorContext(ctx, "failed to sync pending payment",
					slog.String("payment_id", payment.ID.String()),
					slog.String("error", sanitize.Error(err)))
				continue
			}
			processed++
		}

		if len(payments) < renewalBatchSize {
			break
		}
	}

	return processed, nil
}
