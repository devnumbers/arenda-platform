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
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// RenewalService drives the subscription renewal background jobs.
type RenewalService struct {
	deps flowDeps
}

// NewRenewalService creates a RenewalService.
func NewRenewalService(deps flowDeps) *RenewalService {
	return &RenewalService{deps: deps}
}

// ProcessRenewals processes all subscriptions whose validity period has ended.
// For subscriptions with auto-renew enabled it attempts to charge and renew them;
// failed charges move the subscription to a grace period. For subscriptions with
// auto-renew disabled it downgrades them to the free basic tariff. Returns the
// total number of subscriptions processed.
func (r *RenewalService) ProcessRenewals(ctx context.Context, now time.Time) (int, error) {
	processed := 0

	for {
		subs, err := r.deps.subscriptions.ListUpForRenewal(ctx, now.UTC(), renewalBatchSize)
		if err != nil {
			return processed, fmt.Errorf("list subscriptions up for renewal: %w", err)
		}
		if len(subs) == 0 {
			break
		}
		for _, sub := range subs {
			if err := r.renewSubscription(ctx, sub, now.UTC()); err != nil {
				r.deps.log.ErrorContext(ctx, "renew subscription failed",
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

	basicTariff, err := r.deps.tariffs.GetByName(ctx, domain.TariffBasic)
	if err != nil {
		return processed, fmt.Errorf("get basic tariff for non-renewing cleanup: %w", err)
	}

	for {
		subs, err := r.deps.subscriptions.ListExpiredNonRenewing(ctx, now.UTC(), renewalBatchSize)
		if err != nil {
			return processed, fmt.Errorf("list expired non-renewing subscriptions: %w", err)
		}
		if len(subs) == 0 {
			break
		}
		for _, sub := range subs {
			if err := r.expireNonRenewingSubscription(ctx, sub, basicTariff, now.UTC()); err != nil {
				r.deps.log.ErrorContext(ctx, "expire non-renewing subscription failed",
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

	for {
		subs, err := r.deps.subscriptions.ListExpiredCancelled(ctx, now.UTC(), renewalBatchSize)
		if err != nil {
			return processed, fmt.Errorf("list expired cancelled subscriptions: %w", err)
		}
		if len(subs) == 0 {
			break
		}
		for _, sub := range subs {
			if err := r.expireNonRenewingSubscription(ctx, sub, basicTariff, now.UTC()); err != nil {
				r.deps.log.ErrorContext(ctx, "expire cancelled subscription failed",
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

func (r *RenewalService) renewSubscription(ctx context.Context, sub domain.Subscription, now time.Time) error {
	// First transaction: lock the subscription, resolve the renewal terms, and
	// persist a pending payment. This transaction is committed *before* the
	// external provider call so the database connection is not held during an
	// unbounded HTTP request.
	tx, err := r.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sub, err = r.deps.subscriptions.WithTx(tx).GetByIDForUpdate(ctx, sub.ID)
	if err != nil {
		return fmt.Errorf("get subscription for update: %w", err)
	}

	if sub.Status != domain.SubscriptionStatusActive || !sub.AutoRenewEnabled || sub.ValidUntil == nil || sub.ValidUntil.After(now) {
		return nil
	}

	renewalTariff, period, amount, err := r.resolveRenewalTariffAndAmount(ctx, tx, sub)
	if err != nil {
		return err
	}

	// If a previous succeeded payment for the same renewal tariff was not fully
	// applied to the subscription (e.g. the best-effort renewal step failed),
	// reconcile it now instead of creating a duplicate payment.
	lastSucceeded, err := r.deps.subscriptionPayments.WithTx(tx).GetLastSucceededBySubscriptionID(ctx, sub.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("get last succeeded payment: %w", err)
	}
	if err == nil && lastSucceeded.TariffID == renewalTariff.ID && !isSubscriptionRenewalApplied(sub, lastSucceeded) {
		_ = tx.Rollback(ctx)
		applySubscriptionRenewalBestEffort(ctx, r.deps, sub.ID, lastSucceeded, now)
		return nil
	}

	// Free tariff changes (e.g. downgrade to basic) do not require a charge.
	if amount <= 0 {
		if err := applyFreeRenewalOrDowngrade(ctx, r.deps, tx, &sub, renewalTariff, period, now); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	if sub.ActivePaymentMethodID == nil {
		transitionToGrace(&sub, now)
		if err := r.deps.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
			return fmt.Errorf("transition to grace: %w", err)
		}
		return tx.Commit(ctx)
	}

	pm, err := r.deps.paymentMethods.WithTx(tx).GetByID(ctx, *sub.ActivePaymentMethodID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			transitionToGrace(&sub, now)
			if err := r.deps.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
				return fmt.Errorf("transition to grace: %w", err)
			}
			return tx.Commit(ctx)
		}
		return fmt.Errorf("get active payment method: %w", err)
	}

	// Look for an existing pending renewal payment for the same terms. If a
	// previous run created the pending row but crashed before saving the provider
	// reference, recover it idempotently instead of creating a duplicate.
	var payment domain.SubscriptionPayment
	pendingPayments, err := r.deps.subscriptionPayments.WithTx(tx).ListPendingSubscriptionPaymentsByUserID(ctx, sub.UserID)
	if err != nil {
		return fmt.Errorf("list pending renewal payments: %w", err)
	}
	for _, p := range pendingPayments {
		if p.SubscriptionID == sub.ID && p.TariffID == renewalTariff.ID && p.Period == period {
			payment = p
			break
		}
	}

	// If we reuse a pending renewal payment but the active card has changed,
	// update the payment method reference so the payment record matches the
	// token that will actually be charged.
	if payment.ID != uuid.Nil && (payment.PaymentMethodID == nil || *payment.PaymentMethodID != pm.ID) {
		payment, err = r.deps.subscriptionPayments.WithTx(tx).UpdatePaymentMethodID(ctx, payment.ID, pm.ID)
		if err != nil {
			return fmt.Errorf("update pending renewal payment method: %w", err)
		}
	}

	if payment.ID == uuid.Nil {
		payment, err = domain.NewSubscriptionPayment(
			sub.UserID,
			sub.ID,
			renewalTariff.ID,
			&pm.ID,
			period,
			amount,
			r.deps.provider.Name(),
			now,
		)
		if err != nil {
			return fmt.Errorf("create renewal payment: %w", err)
		}

		payment, err = r.deps.subscriptionPayments.WithTx(tx).Create(ctx, payment)
		if err != nil {
			return fmt.Errorf("save renewal payment: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit pending renewal payment transaction: %w", err)
	}

	// Recover a missing provider reference idempotently before charging.
	if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID == "" {
		notification, successURL, failURL := tkassaCallbackURLs(r.deps.callbackBaseURL, payment.ID)
		initRes, err := r.deps.provider.Init(ctx, InitRequest{
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
			Description:            truncateTkassaDescription(renewalPaymentDescription(renewalTariff.Name, period)),
		})
		if err != nil {
			if recErr := r.recoverRenewalFailure(ctx, payment.ID, sub.ID, "", providerErrorCode(err), now); recErr != nil {
				return fmt.Errorf("provider init error: %w; recovery failed: %w", err, recErr)
			}
			r.deps.log.ErrorContext(ctx, "provider init failed; subscription moved to grace",
				slog.String("subscription_id", sub.ID.String()),
				slog.String("payment_id", payment.ID.String()),
				slog.String("error", sanitize.Error(err)))
			return nil
		}

		payment, err = saveProviderInitResult(ctx, r.deps, payment, initRes, sub.UserID, now)
		if err != nil {
			return fmt.Errorf("save provider init result: %w", err)
		}
	}

	chargeProviderPaymentID := ""
	if payment.ProviderPaymentID != nil {
		chargeProviderPaymentID = *payment.ProviderPaymentID
	}

	// If a previous run already registered a provider reference, query the
	// provider status before re-charging to avoid duplicate charges.
	if chargeProviderPaymentID != "" {
		status, statusErr := r.deps.provider.Status(ctx, payment.ID, chargeProviderPaymentID)
		if statusErr == nil {
			switch status {
			case domain.PaymentStatusSucceeded:
				return r.markRenewalSucceededAndApply(ctx, payment, sub.ID, now)
			case domain.PaymentStatusFailed:
				return r.markRenewalFailedAndGrace(ctx, payment.ID, sub.ID, nil, now)
			}
		}
	}

	chargeResult, chargeErr := r.deps.provider.Charge(ctx, ChargeRequest{
		PaymentID:         payment.ID,
		AmountKopecks:     amount,
		Token:             pm.ProviderToken,
		ProviderPaymentID: chargeProviderPaymentID,
	})
	if chargeErr != nil {
		hint := chargeResult.ProviderPaymentID
		if hint == "" {
			hint = chargeProviderPaymentID
		}
		if recErr := r.recoverRenewalFailure(ctx, payment.ID, sub.ID, hint, providerErrorCode(chargeErr), now); recErr != nil {
			return fmt.Errorf("provider charge error: %w; recovery failed: %w", chargeErr, recErr)
		}
		r.deps.log.ErrorContext(ctx, "provider charge failed; subscription moved to grace",
			slog.String("subscription_id", sub.ID.String()),
			slog.String("payment_id", payment.ID.String()),
			slog.String("error", sanitize.Error(chargeErr)))
		return nil
	}

	// Second transaction: reload the payment under lock and apply the provider
	// result. The lock guards against concurrent webhook updates.
	resultTx, err := r.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin post-charge transaction: %w", err)
	}
	defer func() { _ = resultTx.Rollback(ctx) }()

	// Recovery opens its own transaction, so it must never run while resultTx
	// is still open. This helper rolls back resultTx first; the deferred
	// rollback will then return ErrTxDone, which is safely ignored.
	recoverWithClosedTx := func() error {
		_ = resultTx.Rollback(ctx)
		return r.recoverRenewalFailure(ctx, payment.ID, sub.ID, chargeResult.ProviderPaymentID, nil, now)
	}

	payment, err = r.deps.subscriptionPayments.WithTx(resultTx).GetByIDForUpdate(ctx, payment.ID)
	if err != nil {
		if recErr := recoverWithClosedTx(); recErr != nil {
			return fmt.Errorf("get payment for update after charge: %w; recovery failed: %w", err, recErr)
		}
		return fmt.Errorf("get payment for update after charge: %w", err)
	}

	if payment.Status != domain.PaymentStatusPending {
		// A concurrent webhook already finalized the payment; the provider
		// response is stale relative to the database state.
		return nil
	}

	if chargeResult.ProviderPaymentID != "" {
		if _, updateErr := r.deps.subscriptionPayments.WithTx(resultTx).UpdateProviderPaymentID(ctx, payment.ID, chargeResult.ProviderPaymentID); updateErr != nil {
			if recErr := recoverWithClosedTx(); recErr != nil {
				return fmt.Errorf("update renewal provider payment id: %w; recovery failed: %w", updateErr, recErr)
			}
			return fmt.Errorf("update renewal provider payment id: %w", updateErr)
		}
	}

	switch chargeResult.Status {
	case domain.PaymentStatusSucceeded:
		if err := r.deps.subscriptionPayments.WithTx(resultTx).MarkSucceeded(ctx, payment.ID, now); err != nil {
			if recErr := recoverWithClosedTx(); recErr != nil {
				return fmt.Errorf("mark renewal payment succeeded: %w; recovery failed: %w", err, recErr)
			}
			return fmt.Errorf("mark renewal payment succeeded: %w", err)
		}
		payment.Status = domain.PaymentStatusSucceeded
		payment.UpdatedAt = now
		payment.SucceededAt = &now
		if err := resultTx.Commit(ctx); err != nil {
			return fmt.Errorf("commit post-charge transaction: %w", err)
		}
		// Subscription renewal/tariff change and property archiving are
		// best-effort compensating operations: they must not roll back a payment
		// that has already been charged.
		sub, renewalTariff, oldTariffID, ok := applySubscriptionRenewalBestEffort(ctx, r.deps, sub.ID, payment, now)
		if ok && oldTariffID != sub.TariffID {
			archiveExcessPropertiesBestEffort(ctx, r.deps, sub.UserID, renewalTariff.ActivePropertyLimit)
		}
		return nil
	case domain.PaymentStatusFailed:
		if err := r.deps.subscriptionPayments.WithTx(resultTx).MarkFailed(ctx, payment.ID, providerErrorCode(chargeErr), now); err != nil {
			if recErr := recoverWithClosedTx(); recErr != nil {
				return fmt.Errorf("mark renewal payment failed: %w; recovery failed: %w", err, recErr)
			}
			return fmt.Errorf("mark renewal payment failed: %w", err)
		}
		sub, err = r.deps.subscriptions.WithTx(resultTx).GetByIDForUpdate(ctx, sub.ID)
		if err != nil {
			if recErr := recoverWithClosedTx(); recErr != nil {
				return fmt.Errorf("get subscription for update after failed charge: %w; recovery failed: %w", err, recErr)
			}
			return fmt.Errorf("get subscription for update after failed charge: %w", err)
		}
		transitionToGrace(&sub, now)
		if err := r.deps.subscriptions.WithTx(resultTx).Update(ctx, sub); err != nil {
			if recErr := recoverWithClosedTx(); recErr != nil {
				return fmt.Errorf("transition to grace after failed renewal: %w; recovery failed: %w", err, recErr)
			}
			return fmt.Errorf("transition to grace after failed renewal: %w", err)
		}
	case domain.PaymentStatusPending:
		// The provider will finalize the charge asynchronously via a webhook.
		// The pending payment is already persisted; leave the subscription active
		// and wait for the webhook.
		return resultTx.Commit(ctx)
	default:
		if recErr := recoverWithClosedTx(); recErr != nil {
			return fmt.Errorf("unexpected charge status %s; recovery failed: %w", chargeResult.Status, recErr)
		}
		return fmt.Errorf("unexpected charge status: %s", chargeResult.Status)
	}

	return resultTx.Commit(ctx)
}

// recoverRenewalFailure resolves an uncertain renewal outcome. It persists any
// provider payment id hint, reloads the payment, and then queries the provider
// for the payment status *outside* of a database transaction. Only after the
// status is known does it open a short transaction to finalize the payment and,
// if necessary, move the subscription to grace.
func (r *RenewalService) recoverRenewalFailure(ctx context.Context, paymentID, subscriptionID uuid.UUID, providerPaymentIDHint string, errorCode *string, now time.Time) error {
	// Persist the hint in a dedicated transaction so the status query can use it,
	// even if the payment row is currently locked by another request.
	if providerPaymentIDHint != "" {
		if err := r.persistProviderPaymentIDHint(ctx, paymentID, providerPaymentIDHint); err != nil {
			return err
		}
	}

	payment, err := r.deps.subscriptionPayments.GetByID(ctx, paymentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return r.transitionSubscriptionToGrace(ctx, subscriptionID, now)
		}
		return fmt.Errorf("get payment for recovery: %w", err)
	}

	if payment.Status == domain.PaymentStatusSucceeded || payment.Status != domain.PaymentStatusPending {
		// Nothing more to do for finalized or unexpected statuses.
		return nil
	}

	if payment.ProviderPaymentID == nil {
		return r.markRenewalFailedAndGrace(ctx, paymentID, subscriptionID, errorCode, now)
	}

	status, err := r.deps.provider.Status(ctx, paymentID, *payment.ProviderPaymentID)
	if err != nil || status == domain.PaymentStatusPending {
		statusStr := string(status)
		if statusStr == "" {
			statusStr = "unknown"
		}
		logAttrs := []any{
			slog.String("subscription_id", subscriptionID.String()),
			slog.String("payment_id", paymentID.String()),
			slog.String("status", statusStr),
		}
		if err != nil {
			logAttrs = append(logAttrs, slog.String("error", sanitize.Error(err)))
		}
		r.deps.log.WarnContext(ctx, "provider status unknown during renewal recovery; leaving subscription active and payment pending", logAttrs...)
		return nil
	}

	if status == domain.PaymentStatusFailed {
		return r.markRenewalFailedAndGrace(ctx, paymentID, subscriptionID, errorCode, now)
	}

	if status != domain.PaymentStatusSucceeded {
		r.deps.log.WarnContext(ctx, "unexpected provider status during renewal recovery; leaving subscription active and payment pending",
			slog.String("subscription_id", subscriptionID.String()),
			slog.String("payment_id", paymentID.String()),
			slog.String("status", string(status)))
		return nil
	}

	return r.markRenewalSucceededAndApply(ctx, payment, subscriptionID, now)
}

func (r *RenewalService) persistProviderPaymentIDHint(ctx context.Context, paymentID uuid.UUID, providerPaymentIDHint string) error {
	tx, err := r.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin provider payment id hint transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	payment, err := r.deps.subscriptionPayments.WithTx(tx).GetByIDForUpdate(ctx, paymentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return fmt.Errorf("get payment for provider payment id hint: %w", err)
	}
	if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID != providerPaymentIDHint {
		if _, updateErr := r.deps.subscriptionPayments.WithTx(tx).UpdateProviderPaymentID(ctx, paymentID, providerPaymentIDHint); updateErr != nil {
			return fmt.Errorf("update provider payment id hint: %w", updateErr)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit provider payment id hint transaction: %w", err)
	}
	return nil
}

func (r *RenewalService) markRenewalFailedAndGrace(ctx context.Context, paymentID, subscriptionID uuid.UUID, errorCode *string, now time.Time) error {
	tx, err := r.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin recovery transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := r.deps.subscriptionPayments.WithTx(tx).MarkFailed(ctx, paymentID, errorCode, now); err != nil {
		return fmt.Errorf("mark payment failed in recovery: %w", err)
	}

	sub, err := r.deps.subscriptions.WithTx(tx).GetByIDForUpdate(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("get subscription for recovery: %w", err)
	}
	transitionToGrace(&sub, now)
	if err := r.deps.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
		return fmt.Errorf("transition subscription to grace in recovery: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit recovery transaction: %w", err)
	}

	r.deps.log.ErrorContext(ctx, "recovered from renewal failure: subscription moved to grace",
		slog.String("subscription_id", subscriptionID.String()),
		slog.String("payment_id", paymentID.String()))
	return nil
}

func (r *RenewalService) markRenewalSucceededAndApply(ctx context.Context, payment domain.SubscriptionPayment, subscriptionID uuid.UUID, now time.Time) error {
	tx, err := r.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin recovery transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := r.deps.subscriptionPayments.WithTx(tx).MarkSucceeded(ctx, payment.ID, now); err != nil {
		return fmt.Errorf("mark payment succeeded in recovery: %w", err)
	}
	payment.Status = domain.PaymentStatusSucceeded
	payment.UpdatedAt = now
	payment.SucceededAt = &now
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit recovery transaction: %w", err)
	}

	sub, renewalTariff, oldTariffID, ok := applySubscriptionRenewalBestEffort(ctx, r.deps, subscriptionID, payment, now)
	if ok && oldTariffID != sub.TariffID {
		archiveExcessPropertiesBestEffort(ctx, r.deps, sub.UserID, renewalTariff.ActivePropertyLimit)
	}
	return nil
}

func (r *RenewalService) transitionSubscriptionToGrace(ctx context.Context, subscriptionID uuid.UUID, now time.Time) error {
	tx, err := r.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin recovery transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sub, err := r.deps.subscriptions.WithTx(tx).GetByIDForUpdate(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("get subscription for recovery: %w", err)
	}
	transitionToGrace(&sub, now)
	if err := r.deps.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
		return fmt.Errorf("transition subscription to grace in recovery: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit recovery transaction: %w", err)
	}

	r.deps.log.ErrorContext(ctx, "recovered from renewal failure: subscription moved to grace",
		slog.String("subscription_id", subscriptionID.String()))
	return nil
}

func (r *RenewalService) resolveRenewalTariffAndAmount(ctx context.Context, tx transaction.Tx, sub domain.Subscription) (domain.Tariff, domain.SubscriptionPeriod, int64, error) {
	if sub.PendingTariffID != nil && sub.PendingPeriod != nil {
		pendingTariff, err := r.deps.tariffs.WithTx(tx).GetByID(ctx, *sub.PendingTariffID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return domain.Tariff{}, "", 0, ErrTariffNotFound
			}
			return domain.Tariff{}, "", 0, fmt.Errorf("get pending tariff: %w", err)
		}
		amount := pendingTariff.MonthlyPriceKopecks
		if *sub.PendingPeriod == domain.PeriodYear {
			amount = pendingTariff.YearlyPriceKopecks
		}
		return pendingTariff, *sub.PendingPeriod, amount, nil
	}

	currentTariff, err := r.deps.tariffs.WithTx(tx).GetByID(ctx, sub.TariffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Tariff{}, "", 0, ErrTariffNotFound
		}
		return domain.Tariff{}, "", 0, fmt.Errorf("get current tariff: %w", err)
	}

	period := domain.PeriodMonth
	lastPayment, err := r.deps.subscriptionPayments.WithTx(tx).GetLastSucceededBySubscriptionID(ctx, sub.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return domain.Tariff{}, "", 0, fmt.Errorf("get last succeeded payment: %w", err)
	}
	if err == nil {
		period = lastPayment.Period
	}

	amount := currentTariff.MonthlyPriceKopecks
	if period == domain.PeriodYear {
		amount = currentTariff.YearlyPriceKopecks
	}
	return currentTariff, period, amount, nil
}

// ProcessExpiredGrace downgrades subscriptions whose grace period has ended to
// the free basic tariff and archives properties that exceed the basic limit.
// Returns the number of subscriptions processed.
func (r *RenewalService) ProcessExpiredGrace(ctx context.Context, now time.Time) (int, error) {
	basicTariff, err := r.deps.tariffs.GetByName(ctx, domain.TariffBasic)
	if err != nil {
		return 0, fmt.Errorf("get basic tariff: %w", err)
	}

	processed := 0
	for {
		subs, err := r.deps.subscriptions.ListInExpiredGrace(ctx, now.UTC(), graceBatchSize)
		if err != nil {
			return processed, fmt.Errorf("list subscriptions in expired grace: %w", err)
		}
		if len(subs) == 0 {
			break
		}
		for _, sub := range subs {
			if err := r.downgradeToBasic(ctx, sub, basicTariff, now.UTC()); err != nil {
				r.deps.log.ErrorContext(ctx, "downgrade to basic after grace failed",
					slog.String("subscription_id", sub.ID.String()),
					slog.String("user_id", sub.UserID.String()),
					slog.String("error", sanitize.Error(err)))
				continue
			}
			processed++
		}
		if len(subs) < graceBatchSize {
			break
		}
	}
	return processed, nil
}

func (r *RenewalService) downgradeToBasic(ctx context.Context, sub domain.Subscription, basicTariff domain.Tariff, now time.Time) error {
	tx, err := r.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sub, err = r.deps.subscriptions.WithTx(tx).GetByIDForUpdate(ctx, sub.ID)
	if err != nil {
		return fmt.Errorf("get subscription for update: %w", err)
	}

	if sub.Status != domain.SubscriptionStatusGrace || sub.ValidUntil == nil || sub.ValidUntil.After(now) {
		return nil
	}

	applyBasicDowngrade(&sub, basicTariff.ID)
	if err := r.deps.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
		return fmt.Errorf("update subscription after grace downgrade: %w", err)
	}

	if r.deps.propertyArchiver != nil {
		if err := r.deps.propertyArchiver.ArchiveExcessProperties(ctx, tx, sub.UserID, basicTariff.ActivePropertyLimit); err != nil {
			return fmt.Errorf("archive excess properties after grace downgrade: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// expireNonRenewingSubscription downgrades an expired subscription to the free
// basic tariff. It handles both active non-renewing subscriptions and cancelled
// subscriptions whose retained validity period has ended.
func (r *RenewalService) expireNonRenewingSubscription(ctx context.Context, sub domain.Subscription, basicTariff domain.Tariff, now time.Time) error {
	tx, err := r.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sub, err = r.deps.subscriptions.WithTx(tx).GetByIDForUpdate(ctx, sub.ID)
	if err != nil {
		return fmt.Errorf("get subscription for update: %w", err)
	}

	if sub.ValidUntil == nil || sub.ValidUntil.After(now) {
		return nil
	}
	if sub.Status != domain.SubscriptionStatusActive && sub.Status != domain.SubscriptionStatusCancelled {
		return nil
	}
	if sub.Status == domain.SubscriptionStatusActive && sub.AutoRenewEnabled {
		return nil
	}

	applyBasicDowngrade(&sub, basicTariff.ID)
	if err := r.deps.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
		return fmt.Errorf("update subscription after non-renewing expiry: %w", err)
	}

	if r.deps.propertyArchiver != nil {
		if err := r.deps.propertyArchiver.ArchiveExcessProperties(ctx, tx, sub.UserID, basicTariff.ActivePropertyLimit); err != nil {
			return fmt.Errorf("archive excess properties after non-renewing expiry: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// ProcessPendingUpgradePayments queries the provider for pending upgrade
// payments that have been stale for longer than the configured threshold and
// finalizes them based on the provider status. Returns the number of payments
// that were checked with the provider.
func (r *RenewalService) ProcessPendingUpgradePayments(ctx context.Context, now time.Time) (int, error) {
	processed := 0
	createdBefore := now.Add(-pendingUpgradeStalenessThreshold).UTC()

	for {
		payments, err := r.deps.subscriptionPayments.ListPendingUpgradePayments(ctx, createdBefore, renewalBatchSize)
		if err != nil {
			return processed, fmt.Errorf("list pending upgrade payments: %w", err)
		}
		if len(payments) == 0 {
			break
		}

		for _, payment := range payments {
			if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID == "" {
				continue
			}

			status, statusErr := r.deps.provider.Status(ctx, payment.ID, *payment.ProviderPaymentID)
			processed++

			if statusErr != nil {
				r.deps.log.WarnContext(ctx, "failed to query provider status for pending upgrade payment",
					slog.String("payment_id", payment.ID.String()),
					slog.String("error", sanitize.Error(statusErr)))
				continue
			}

			switch status {
			case domain.PaymentStatusSucceeded:
				if err := r.markRenewalSucceededAndApply(ctx, payment, payment.SubscriptionID, now.UTC()); err != nil {
					r.deps.log.ErrorContext(ctx, "failed to apply succeeded upgrade payment",
						slog.String("payment_id", payment.ID.String()),
						slog.String("error", sanitize.Error(err)))
				}
			case domain.PaymentStatusFailed:
				markPaymentFailedBestEffort(ctx, r.deps, payment.ID, now.UTC())
			case domain.PaymentStatusPending:
				// Provider has not finalized the payment yet; leave it pending.
			default:
				r.deps.log.WarnContext(ctx, "unexpected provider status for pending upgrade payment",
					slog.String("payment_id", payment.ID.String()),
					slog.String("status", string(status)))
			}
		}

		if len(payments) < renewalBatchSize {
			break
		}
	}

	return processed, nil
}
