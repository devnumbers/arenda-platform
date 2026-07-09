package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

// ScheduledChangeService applies scheduled tariff changes.
type ScheduledChangeService struct {
	deps flowDeps
}

// NewScheduledChangeService creates a ScheduledChangeService.
func NewScheduledChangeService(deps flowDeps) *ScheduledChangeService {
	return &ScheduledChangeService{deps: deps}
}

// ProcessScheduledChanges applies scheduled tariff changes (usually downgrades to
// the free basic tariff) whose pending_change_at has been reached. Returns the
// number of subscriptions processed.
func (c *ScheduledChangeService) ProcessScheduledChanges(ctx context.Context, now time.Time) (int, error) {
	processed := 0
	for {
		subs, err := c.deps.subscriptions.ListPendingChanges(ctx, now.UTC(), renewalBatchSize)
		if err != nil {
			return processed, fmt.Errorf("list subscriptions with pending change: %w", err)
		}
		if len(subs) == 0 {
			break
		}
		for _, sub := range subs {
			if err := c.applyScheduledChange(ctx, sub, now.UTC()); err != nil {
				c.deps.log.ErrorContext(ctx, "apply scheduled change failed",
					slog.String("subscription_id", sub.ID.String()),
					slog.String("user_id", sub.UserID.String()),
					slog.String("error", sanitize.Error(err)))
				continue
			}
			processed++
		}
		if len(subs) < renewalBatchSize {
			break
		}
	}
	return processed, nil
}

func (c *ScheduledChangeService) applyScheduledChange(ctx context.Context, sub domain.Subscription, now time.Time) error {
	tx, err := c.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txSubscriptions, err := c.deps.subscriptions.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind subscriptions transaction: %w", err)
	}
	txTariffs, err := c.deps.tariffs.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind tariffs transaction: %w", err)
	}
	txPaymentMethods, err := c.deps.paymentMethods.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind payment methods transaction: %w", err)
	}
	txSubscriptionPayments, err := c.deps.subscriptionPayments.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind subscription payments transaction: %w", err)
	}

	sub, err = txSubscriptions.GetByIDForUpdate(ctx, sub.ID)
	if err != nil {
		return fmt.Errorf("get subscription for update: %w", err)
	}

	if sub.Status != domain.SubscriptionStatusActive || sub.PendingTariffID == nil || sub.PendingChangeAt == nil || sub.PendingChangeAt.After(now) {
		return nil
	}

	pendingTariff, err := txTariffs.GetByID(ctx, *sub.PendingTariffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrTariffNotFound
		}
		return fmt.Errorf("get pending tariff: %w", err)
	}

	period := domain.PeriodMonth
	if sub.PendingPeriod != nil {
		period = *sub.PendingPeriod
	}

	amount := pendingTariff.MonthlyPriceKopecks
	if period == domain.PeriodYear {
		amount = pendingTariff.YearlyPriceKopecks
	}

	// Free scheduled changes (e.g. a downgrade to the basic tariff) do not require
	// a charge and are applied immediately within this transaction.
	if amount <= 0 {
		if err := applyFreeRenewalOrDowngrade(ctx, c.deps, tx, &sub, pendingTariff, period, now); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	// Paid scheduled changes are charged to the active payment method using the
	// same recurrent-charge flow as renewSubscription. If there is no active
	// payment method we cannot charge, so the scheduled change is cleared and
	// left for manual handling. In every terminal branch the pending_* fields are
	// cleared, which guarantees the worker never re-lists the same row.
	if sub.ActivePaymentMethodID == nil {
		sub.PendingTariffID = nil
		sub.PendingChangeAt = nil
		sub.PendingPeriod = nil
		if err := txSubscriptions.Update(ctx, sub); err != nil {
			return fmt.Errorf("clear scheduled change without payment method: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit cleared scheduled change: %w", err)
		}
		c.deps.log.WarnContext(ctx, "paid scheduled change skipped: no active payment method",
			slog.String("subscription_id", sub.ID.String()),
			slog.String("user_id", sub.UserID.String()),
			slog.String("pending_tariff_id", pendingTariff.ID.String()))
		return nil
	}

	pm, err := txPaymentMethods.GetByID(ctx, *sub.ActivePaymentMethodID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			sub.PendingTariffID = nil
			sub.PendingChangeAt = nil
			sub.PendingPeriod = nil
			if updateErr := txSubscriptions.Update(ctx, sub); updateErr != nil {
				return fmt.Errorf("clear scheduled change with missing payment method: %w", updateErr)
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return fmt.Errorf("commit cleared scheduled change: %w", commitErr)
			}
			c.deps.log.WarnContext(ctx, "paid scheduled change skipped: active payment method not found",
				slog.String("subscription_id", sub.ID.String()),
				slog.String("user_id", sub.UserID.String()))
			return nil
		}
		return fmt.Errorf("get active payment method for scheduled change: %w", err)
	}

	// Persist the pending payment and commit *before* the external provider call
	// so the database connection is not held during an unbounded HTTP request.
	payment, err := domain.NewSubscriptionPayment(
		sub.UserID,
		sub.ID,
		pendingTariff.ID,
		&pm.ID,
		period,
		amount,
		c.deps.provider.Name(),
		now,
	)
	if err != nil {
		return fmt.Errorf("create scheduled change payment: %w", err)
	}
	payment, err = txSubscriptionPayments.Create(ctx, payment)
	if err != nil {
		return fmt.Errorf("save scheduled change payment: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit pending scheduled change payment: %w", err)
	}

	return c.applyPaidScheduledChange(ctx, sub, pendingTariff, period, amount, payment, pm, now)
}

// applyPaidScheduledChange charges a scheduled tariff change to the active
// payment method and applies the result. The provider call is performed outside
// of any database transaction. Both the success and the failure paths clear the
// subscription's pending_* fields so the worker loop terminates.
func (c *ScheduledChangeService) applyPaidScheduledChange(
	ctx context.Context,
	sub domain.Subscription,
	pendingTariff domain.Tariff,
	period domain.SubscriptionPeriod,
	amount int64,
	payment domain.SubscriptionPayment,
	pm domain.PaymentMethod,
	now time.Time,
) error {
	// The provider charge is intentionally performed outside of any database
	// transaction so an unbounded HTTP request never holds a connection.
	chargeResult, chargeErr := c.deps.provider.Charge(ctx, ChargeRequest{
		PaymentID:     payment.ID,
		AmountKopecks: amount,
		Token:         pm.ProviderToken,
	})

	errorCode := providerErrorCode(chargeErr)
	charged := chargeErr == nil && chargeResult.Status == domain.PaymentStatusSucceeded

	tx, err := c.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin scheduled change result transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txSubscriptionPayments, err := c.deps.subscriptionPayments.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind subscription payments transaction: %w", err)
	}
	txSubscriptions, err := c.deps.subscriptions.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind subscriptions transaction: %w", err)
	}

	payment, err = txSubscriptionPayments.GetByIDForUpdate(ctx, payment.ID)
	if err != nil {
		return fmt.Errorf("get scheduled change payment for update: %w", err)
	}

	if charged {
		if payment.Status == domain.PaymentStatusPending {
			if chargeResult.ProviderPaymentID != "" {
				if _, updateErr := txSubscriptionPayments.UpdateProviderPaymentID(ctx, payment.ID, chargeResult.ProviderPaymentID); updateErr != nil {
					return fmt.Errorf("update scheduled change provider payment id: %w", updateErr)
				}
			}
			if err := txSubscriptionPayments.MarkSucceeded(ctx, payment.ID, now); err != nil {
				return fmt.Errorf("mark scheduled change payment succeeded: %w", err)
			}
		}

		sub, err = txSubscriptions.GetByIDForUpdate(ctx, sub.ID)
		if err != nil {
			return fmt.Errorf("get subscription for scheduled change apply: %w", err)
		}
		oldTariffID := sub.TariffID
		sub.TariffID = pendingTariff.ID
		sub.CurrentPeriod = &period
		sub.PendingTariffID = nil
		sub.PendingChangeAt = nil
		sub.PendingPeriod = nil
		if err := txSubscriptions.Update(ctx, sub); err != nil {
			return fmt.Errorf("apply paid scheduled change: %w", err)
		}
		if c.deps.propertyArchiver != nil && oldTariffID != pendingTariff.ID {
			if err := c.deps.propertyArchiver.ArchiveExcessProperties(ctx, tx, sub.UserID, pendingTariff.ActivePropertyLimit); err != nil {
				return fmt.Errorf("archive excess properties after paid scheduled change: %w", err)
			}
		}

		return tx.Commit(ctx)
	}

	// Any non-succeeded outcome (provider error, failed, or pending) is terminal
	// for a scheduled change: mark the payment failed and clear the pending
	// fields so the worker does not charge the same row again.
	if payment.Status == domain.PaymentStatusPending {
		if err := txSubscriptionPayments.MarkFailed(ctx, payment.ID, errorCode, now); err != nil {
			return fmt.Errorf("mark scheduled change payment failed: %w", err)
		}
	}

	sub, err = txSubscriptions.GetByIDForUpdate(ctx, sub.ID)
	if err != nil {
		return fmt.Errorf("get subscription for scheduled change failure: %w", err)
	}
	sub.PendingTariffID = nil
	sub.PendingChangeAt = nil
	sub.PendingPeriod = nil
	if err := txSubscriptions.Update(ctx, sub); err != nil {
		return fmt.Errorf("clear scheduled change after failed charge: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit failed scheduled change: %w", err)
	}

	c.deps.log.ErrorContext(ctx, "paid scheduled change failed; pending change cleared",
		slog.String("subscription_id", sub.ID.String()),
		slog.String("user_id", sub.UserID.String()),
		slog.String("payment_id", payment.ID.String()),
		slog.String("error", sanitize.Error(chargeErr)))
	return nil
}
