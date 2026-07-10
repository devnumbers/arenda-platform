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

	// Reuse an existing pending payment for the same scheduled change so a crash
	// after the pending row was committed (or a repeated worker tick) does not
	// create a duplicate payment and charge the same downgrade twice.
	var payment domain.SubscriptionPayment
	pendingPayments, err := txSubscriptionPayments.ListPendingSubscriptionPaymentsByUserID(ctx, sub.UserID)
	if err != nil {
		return fmt.Errorf("list pending scheduled change payments: %w", err)
	}
	for _, p := range pendingPayments {
		if p.SubscriptionID == sub.ID && p.TariffID == pendingTariff.ID && p.Period == period {
			payment = p
			break
		}
	}

	// If we reuse a pending payment but the active card has changed, refresh the
	// payment method reference so the record matches the token that will be charged.
	if payment.ID != uuid.Nil && (payment.PaymentMethodID == nil || *payment.PaymentMethodID != pm.ID) {
		payment, err = txSubscriptionPayments.UpdatePaymentMethodID(ctx, payment.ID, pm.ID)
		if err != nil {
			return fmt.Errorf("update pending scheduled change payment method: %w", err)
		}
	}

	// Persist the pending payment and commit *before* the external provider call
	// so the database connection is not held during an unbounded HTTP request.
	if payment.ID == uuid.Nil {
		payment, err = domain.NewSubscriptionPayment(
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
	}

	// Capture the scheduled change for the duration of the apply by pushing
	// pending_change_at into the future. ListPendingChanges only returns rows
	// whose pending_change_at <= now, so a crash after this commit or a repeated
	// worker tick will not re-list this subscription until the TTL elapses. The
	// terminal branches in applyPaidScheduledChange still clear pending_* as before.
	originalPendingChangeAt := *sub.PendingChangeAt
	inProgressUntil := now.Add(scheduledChangeInProgressTTL)
	sub.PendingChangeAt = &inProgressUntil
	if err := txSubscriptions.Update(ctx, sub); err != nil {
		return fmt.Errorf("capture scheduled change in progress: %w", err)
	}
	c.deps.log.DebugContext(ctx, "scheduled change captured in progress",
		slog.String("subscription_id", sub.ID.String()),
		slog.String("payment_id", payment.ID.String()),
		slog.Time("original_pending_change_at", originalPendingChangeAt),
		slog.Time("in_progress_until", inProgressUntil))

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit pending scheduled change payment: %w", err)
	}

	return c.applyPaidScheduledChange(ctx, sub, pendingTariff, period, amount, payment, pm, now)
}

// applyPaidScheduledChange charges a scheduled tariff change to the active
// payment method and applies the result. It mirrors the renewal flow: Init to
// obtain a provider reference when missing, a Status check to deduplicate an
// already-charged payment, and a Charge that always carries ProviderPaymentID.
// All provider calls run outside of any database transaction. Both the success
// and the failure paths clear the subscription's pending_* fields so the worker
// loop terminates.
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
	var err error

	// Recover a missing provider reference idempotently before charging, mirroring
	// the renewal flow. The provider Init call is intentionally performed outside
	// of any database transaction so an unbounded HTTP request never holds a
	// connection.
	if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID == "" {
		notification, successURL, failURL := tkassaCallbackURLs(c.deps.callbackBaseURL, payment.ID)
		initRes, initErr := c.deps.provider.Init(ctx, InitRequest{
			PaymentID:              payment.ID,
			AmountKopecks:          amount,
			Period:                 period,
			UserID:                 sub.UserID,
			CustomerKey:            sub.UserID.String(),
			Recurrent:              true,
			OperationInitiatorType: "R",
			NotificationURL:        notification,
			SuccessURL:             successURL,
			FailURL:                failURL,
			Description:            truncateTkassaDescription(scheduledChangePaymentDescription(pendingTariff.Name, period)),
		})
		if initErr != nil {
			// Init failure is terminal for a scheduled change. There is no open
			// transaction here (the pending payment was committed in
			// applyScheduledChange before the provider call), so the best-effort
			// helpers each open their own short transaction safely.
			markPaymentFailedBestEffort(ctx, c.deps, payment.ID, now)
			c.clearPendingScheduledChangeBestEffort(ctx, sub.ID)
			c.deps.log.ErrorContext(ctx, "provider init failed for paid scheduled change; pending change cleared",
				slog.String("subscription_id", sub.ID.String()),
				slog.String("user_id", sub.UserID.String()),
				slog.String("payment_id", payment.ID.String()),
				slog.String("error", sanitize.Error(initErr)))
			return nil
		}
		payment, err = saveProviderInitResult(ctx, c.deps, payment, initRes, sub.UserID, now)
		if err != nil {
			return fmt.Errorf("save provider init result for scheduled change: %w", err)
		}
	}

	chargeProviderPaymentID := ""
	if payment.ProviderPaymentID != nil {
		chargeProviderPaymentID = *payment.ProviderPaymentID
	}

	var (
		chargeResult ChargeResult
		chargeErr    error
		charged      bool
	)

	// If a provider reference already exists (from this run's Init or a previous
	// crashed run), query the provider status before charging to avoid a duplicate
	// charge. A terminal succeeded/failed status is applied directly without a
	// second Charge.
	if chargeProviderPaymentID != "" {
		status, statusErr := c.deps.provider.Status(ctx, payment.ID, chargeProviderPaymentID)
		if statusErr == nil {
			switch status {
			case domain.PaymentStatusSucceeded:
				chargeResult = ChargeResult{Status: domain.PaymentStatusSucceeded}
				charged = true
			case domain.PaymentStatusFailed:
				chargeResult = ChargeResult{Status: domain.PaymentStatusFailed}
			}
		}
	}

	// Charge only when the provider has not already reported a terminal status.
	// The Charge always carries ProviderPaymentID: a real recurrent gateway (e.g.
	// T-Kassa) rejects a Charge that omits the PaymentId returned by Init.
	if chargeResult.Status == "" {
		chargeResult, chargeErr = c.deps.provider.Charge(ctx, ChargeRequest{
			PaymentID:         payment.ID,
			ProviderPaymentID: chargeProviderPaymentID,
			AmountKopecks:     amount,
			Token:             pm.ProviderToken,
		})
		charged = chargeErr == nil && chargeResult.Status == domain.PaymentStatusSucceeded
	}

	errorCode := providerErrorCode(chargeErr)

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

// clearPendingScheduledChangeBestEffort clears a subscription's pending_* fields
// in a short dedicated transaction. It is used on terminal failure paths that run
// without an open transaction (for example a provider Init failure), so the worker
// does not re-list the row after the in-progress capture TTL elapses.
func (c *ScheduledChangeService) clearPendingScheduledChangeBestEffort(ctx context.Context, subscriptionID uuid.UUID) {
	tx, err := c.deps.beginner.Begin(ctx)
	if err != nil {
		c.deps.log.ErrorContext(ctx, "failed to begin transaction to clear scheduled change",
			slog.String("subscription_id", subscriptionID.String()),
			slog.String("error", sanitize.Error(err)))
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txSubscriptions, err := c.deps.subscriptions.WithTx(tx)
	if err != nil {
		c.deps.log.ErrorContext(ctx, "failed to bind subscription transaction to clear scheduled change",
			slog.String("subscription_id", subscriptionID.String()),
			slog.String("error", sanitize.Error(err)))
		return
	}

	sub, err := txSubscriptions.GetByIDForUpdate(ctx, subscriptionID)
	if err != nil {
		c.deps.log.ErrorContext(ctx, "failed to load subscription to clear scheduled change",
			slog.String("subscription_id", subscriptionID.String()),
			slog.String("error", sanitize.Error(err)))
		return
	}
	sub.PendingTariffID = nil
	sub.PendingChangeAt = nil
	sub.PendingPeriod = nil
	if err := txSubscriptions.Update(ctx, sub); err != nil {
		c.deps.log.ErrorContext(ctx, "failed to clear scheduled change",
			slog.String("subscription_id", subscriptionID.String()),
			slog.String("error", sanitize.Error(err)))
		return
	}
	if err := tx.Commit(ctx); err != nil {
		c.deps.log.ErrorContext(ctx, "failed to commit scheduled change cleanup",
			slog.String("subscription_id", subscriptionID.String()),
			slog.String("error", sanitize.Error(err)))
	}
}
