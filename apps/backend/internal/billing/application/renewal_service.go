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
	deps     renewalServiceDeps
	provider RenewalProvider
}

// NewRenewalService creates a RenewalService.
func NewRenewalService(deps renewalServiceDeps, provider RenewalProvider) *RenewalService {
	return &RenewalService{deps: deps, provider: provider}
}

// ProcessRenewals processes all subscriptions whose validity period has ended.
// For subscriptions with auto-renew enabled it attempts to charge and renew them;
// failed charges move the subscription to a grace period. For subscriptions with
// auto-renew disabled it downgrades them to the free basic tariff. Returns the
// total number of subscriptions processed.
func (r *RenewalService) ProcessRenewals(ctx context.Context, now time.Time) (int, error) {
	processed := 0

	n, err := r.processSubscriptionBatch(ctx, now, "renew",
		r.deps.subscriptions.ListUpForRenewal,
		r.renewSubscription)
	if err != nil {
		return processed, err
	}
	processed += n

	basicTariff, err := r.deps.tariffs.GetByName(ctx, domain.TariffBasic)
	if err != nil {
		return processed, fmt.Errorf("get basic tariff for non-renewing cleanup: %w", err)
	}

	n, err = r.processSubscriptionBatch(ctx, now, "expire non-renewing",
		r.deps.subscriptions.ListExpiredNonRenewing,
		func(ctx context.Context, sub domain.Subscription, now time.Time) error {
			return r.expireNonRenewingSubscription(ctx, sub, basicTariff, now)
		})
	if err != nil {
		return processed, err
	}
	processed += n

	n, err = r.processSubscriptionBatch(ctx, now, "expire cancelled",
		r.deps.subscriptions.ListExpiredCancelled,
		func(ctx context.Context, sub domain.Subscription, now time.Time) error {
			return r.expireNonRenewingSubscription(ctx, sub, basicTariff, now)
		})
	if err != nil {
		return processed, err
	}
	processed += n

	return processed, nil
}

func (r *RenewalService) processSubscriptionBatch(
	ctx context.Context,
	now time.Time,
	op string,
	list func(context.Context, time.Time, int32) ([]domain.Subscription, error),
	process func(context.Context, domain.Subscription, time.Time) error,
) (int, error) {
	processed := 0
	for {
		subs, err := list(ctx, now.UTC(), renewalBatchSize)
		if err != nil {
			return processed, fmt.Errorf("list subscriptions for %s: %w", op, err)
		}
		if len(subs) == 0 {
			break
		}
		batchProcessed := 0
		for _, sub := range subs {
			if err := process(ctx, sub, now.UTC()); err != nil {
				r.deps.log.ErrorContext(ctx, op+" subscription failed",
					slog.String("subscription_id", sub.ID.String()),
					slog.String("user_id", sub.UserID.String()),
					slog.String("error", sanitize.Error(err)))
				continue
			}
			batchProcessed++
		}
		processed += batchProcessed
		if len(subs) < int(renewalBatchSize) {
			break
		}
		if batchProcessed == 0 {
			// A full batch with zero progress means every item failed and
			// stays in the selection; defer to the next tick instead of
			// spinning in an infinite retry loop.
			r.deps.log.WarnContext(ctx, "batch made no progress; deferring to next tick",
				slog.String("op", op))
			break
		}
	}
	return processed, nil
}

func (r *RenewalService) renewSubscription(ctx context.Context, sub domain.Subscription, now time.Time) error {
	payment, pm, amount, period, renewalTariff, err := r.prepareRenewalPayment(ctx, sub, now)
	if err != nil {
		return err
	}
	if payment.ID == uuid.Nil {
		return nil
	}
	return r.finalizeRenewalCharge(ctx, sub, payment, pm, amount, period, renewalTariff, now)
}

// prepareRenewalPayment locks the subscription, resolves the renewal terms, and
// persists a pending payment. The transaction is committed *before* returning
// so the database connection is not held during the unbounded external provider
// call. All early-exit paths return a zero payment and a nil error.
func (r *RenewalService) prepareRenewalPayment(
	ctx context.Context,
	sub domain.Subscription,
	now time.Time,
) (payment domain.SubscriptionPayment, pm domain.PaymentMethod, amount int64, period domain.SubscriptionPeriod, renewalTariff domain.Tariff, err error) {
	tx, err := r.deps.beginner.Begin(ctx)
	if err != nil {
		return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txSubscriptions, err := r.deps.subscriptions.WithTx(tx)
	if err != nil {
		return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, fmt.Errorf("bind subscriptions transaction: %w", err)
	}
	txSubscriptionPayments, err := r.deps.subscriptionPayments.WithTx(tx)
	if err != nil {
		return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, fmt.Errorf("bind subscription payments transaction: %w", err)
	}
	txPaymentMethods, err := r.deps.paymentMethods.WithTx(tx)
	if err != nil {
		return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, fmt.Errorf("bind payment methods transaction: %w", err)
	}

	sub, err = txSubscriptions.GetByIDForUpdate(ctx, sub.ID)
	if err != nil {
		return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, fmt.Errorf("get subscription for update: %w", err)
	}

	if sub.Status != domain.SubscriptionStatusActive || !sub.AutoRenewEnabled || sub.ValidUntil == nil || sub.ValidUntil.After(now) {
		return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, nil
	}

	renewalTariff, period, amount, err = r.resolveRenewalTariffAndAmount(ctx, tx, sub)
	if err != nil {
		return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, err
	}

	// If a previous succeeded payment for the same renewal tariff was not fully
	// applied to the subscription (e.g. the best-effort renewal step failed),
	// reconcile it now instead of creating a duplicate payment.
	lastSucceeded, err := txSubscriptionPayments.GetLastSucceededBySubscriptionID(ctx, sub.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, fmt.Errorf("get last succeeded payment: %w", err)
	}
	if err == nil && lastSucceeded.TariffID == renewalTariff.ID && !isSubscriptionRenewalApplied(sub, lastSucceeded) {
		_ = tx.Rollback(ctx)
		applySubscriptionRenewalBestEffort(ctx, renewalBestEffortDeps{
			beginner:      r.deps.beginner,
			subscriptions: r.deps.subscriptions,
			tariffs:       r.deps.tariffs,
			log:           r.deps.log,
		}, sub.ID, lastSucceeded, now)
		return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, nil
	}

	// Free tariff changes (e.g. downgrade to basic) do not require a charge.
	if amount <= 0 {
		if err := applyFreeRenewalOrDowngrade(ctx, freeRenewalDeps{
			subscriptions:         r.deps.subscriptions,
			tariffs:               r.deps.tariffs,
			propertyArchiver:      r.deps.propertyArchiver,
			recipientSlotEnforcer: r.deps.recipientSlotEnforcer,
		}, tx, &sub, renewalTariff, period, uuid.Nil, now); err != nil {
			return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, fmt.Errorf("commit free renewal transaction: %w", err)
		}
		return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, nil
	}

	if sub.ActivePaymentMethodID == nil {
		sub.EnterGrace(now)
		if err := txSubscriptions.Update(ctx, sub); err != nil {
			return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, fmt.Errorf("transition to grace: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, fmt.Errorf("commit grace transition transaction: %w", err)
		}
		return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, nil
	}

	pm, err = txPaymentMethods.GetByID(ctx, *sub.ActivePaymentMethodID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			sub.EnterGrace(now)
			if err := txSubscriptions.Update(ctx, sub); err != nil {
				return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, fmt.Errorf("transition to grace: %w", err)
			}
			if err := tx.Commit(ctx); err != nil {
				return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, fmt.Errorf("commit grace transition transaction: %w", err)
			}
			return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, nil
		}
		return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, fmt.Errorf("get active payment method: %w", err)
	}

	// Look for an existing pending renewal payment for the same terms. If a
	// previous run created the pending row but crashed before saving the provider
	// reference, recover it idempotently instead of creating a duplicate.
	pendingPayments, err := txSubscriptionPayments.ListPendingSubscriptionPaymentsByUserID(ctx, sub.UserID)
	if err != nil {
		return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, fmt.Errorf("list pending renewal payments: %w", err)
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
		payment, err = txSubscriptionPayments.UpdatePaymentMethodID(ctx, payment.ID, pm.ID)
		if err != nil {
			return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, fmt.Errorf("update pending renewal payment method: %w", err)
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
			r.provider.Name(),
			now,
		)
		if err != nil {
			return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, fmt.Errorf("create renewal payment: %w", err)
		}

		payment, err = txSubscriptionPayments.Create(ctx, payment)
		if err != nil {
			if errors.Is(err, ErrAlreadyExists) {
				// A concurrent run created the pending payment first. Return the
				// existing one instead of failing.
				pending, listErr := txSubscriptionPayments.ListPendingSubscriptionPaymentsByUserID(ctx, sub.UserID)
				if listErr != nil {
					return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, fmt.Errorf("list pending subscription payments: %w", listErr)
				}
				for _, p := range pending {
					if p.SubscriptionID == sub.ID && p.TariffID == renewalTariff.ID && p.Period == period {
						_ = tx.Rollback(ctx)
						return p, pm, amount, period, renewalTariff, nil
					}
				}
			}
			return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, fmt.Errorf("save renewal payment: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.SubscriptionPayment{}, domain.PaymentMethod{}, 0, "", domain.Tariff{}, fmt.Errorf("commit pending renewal payment transaction: %w", err)
	}

	return payment, pm, amount, period, renewalTariff, nil
}

// finalizeRenewalCharge idempotently initializes the provider payment, charges
// the active payment method, and applies the result in a second transaction.
func (r *RenewalService) finalizeRenewalCharge(
	ctx context.Context,
	sub domain.Subscription,
	payment domain.SubscriptionPayment,
	pm domain.PaymentMethod,
	amount int64,
	period domain.SubscriptionPeriod,
	renewalTariff domain.Tariff,
	now time.Time,
) error {
	// Recover a missing provider reference idempotently before charging.
	if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID == "" {
		notification, successURL, failURL := tkassaCallbackURLs(r.deps.callbackBaseURL, payment.ID)
		initRes, err := r.provider.Init(ctx, InitRequest{
			PaymentID:     payment.ID,
			AmountKopecks: amount,
			Period:        period,
			UserID:        sub.UserID,
			CustomerKey:   sub.UserID.String(),
			// Child MIT payment: Recurrent=Y marks only the parent payment and
			// must be omitted here — combined with a child
			// OperationInitiatorType it is rejected by T-Kassa (error 1126).
			Recurrent:              false,
			OperationInitiatorType: InitiatorTypeMITRecurring,
			NotificationURL:        notification,
			SuccessURL:             successURL,
			FailURL:                failURL,
			Description:            truncateTkassaDescription(renewalPaymentDescription(renewalTariff.Name, period)),
		})
		if err != nil {
			if recErr := r.recoverRenewalFailure(ctx, payment.ID, sub.ID, "", err, now); recErr != nil {
				return fmt.Errorf("provider init error: %w; recovery failed: %w", err, recErr)
			}
			r.deps.log.ErrorContext(ctx, "provider init failed; subscription moved to grace",
				slog.String("subscription_id", sub.ID.String()),
				slog.String("payment_id", payment.ID.String()),
				slog.String("error", sanitize.Error(err)))
			return nil
		}

		payment, err = saveProviderInitResult(ctx, saveProviderInitDeps{
			beginner:             r.deps.beginner,
			subscriptionPayments: r.deps.subscriptionPayments,
			paymentMethods:       r.deps.paymentMethods,
			provider:             r.provider,
			log:                  r.deps.log,
		}, payment, initRes, sub.UserID, now)
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
		statusResult, statusErr := r.provider.Status(ctx, payment.ID, chargeProviderPaymentID)
		if statusErr != nil {
			// The provider status is unknown: the previous charge may have
			// succeeded. Do not blindly re-charge; leave the payment pending
			// until a later tick can determine the status.
			r.deps.log.WarnContext(ctx, "provider status unavailable; skipping charge attempt this tick",
				slog.String("payment_id", payment.ID.String()),
				slog.String("error", sanitize.Error(statusErr)))
			return nil
		}
		status := statusResult.Status
		switch status {
		case domain.PaymentStatusSucceeded:
			return r.markRenewalSucceededAndApply(ctx, payment, sub.ID, now)
		case domain.PaymentStatusFailed:
			return r.markRenewalFailedAndGrace(ctx, payment.ID, sub.ID, providerResultErrorCode(statusResult.ErrorCode, nil), now)
		default:
			// Any other provider status (still pending, refunded) leaves the
			// payment unresolved, so fall through to a fresh charge attempt.
		}
	}

	chargeResult, chargeErr := r.provider.Charge(ctx, ChargeRequest{
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
		if recErr := r.recoverRenewalFailure(ctx, payment.ID, sub.ID, hint, chargeErr, now); recErr != nil {
			return fmt.Errorf("provider charge error: %w; recovery failed: %w", chargeErr, recErr)
		}
		r.deps.log.ErrorContext(ctx, "provider charge failed; running renewal recovery",
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

	txResultSubscriptionPayments, err := r.deps.subscriptionPayments.WithTx(resultTx)
	if err != nil {
		return fmt.Errorf("bind subscription payments transaction: %w", err)
	}
	txResultSubscriptions, err := r.deps.subscriptions.WithTx(resultTx)
	if err != nil {
		return fmt.Errorf("bind subscriptions transaction: %w", err)
	}

	// Recovery opens its own transaction, so it must never run while resultTx
	// is still open. This helper rolls back resultTx first; the deferred
	// rollback will then return ErrTxDone, which is safely ignored.
	recoverWithClosedTx := func() error {
		_ = resultTx.Rollback(ctx)
		return r.recoverRenewalFailure(ctx, payment.ID, sub.ID, chargeResult.ProviderPaymentID, nil, now)
	}

	payment, err = txResultSubscriptionPayments.GetByIDForUpdate(ctx, payment.ID)
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
		if _, updateErr := txResultSubscriptionPayments.UpdateProviderPaymentID(ctx, payment.ID, chargeResult.ProviderPaymentID); updateErr != nil {
			if recErr := recoverWithClosedTx(); recErr != nil {
				return fmt.Errorf("update renewal provider payment id: %w; recovery failed: %w", updateErr, recErr)
			}
			return fmt.Errorf("update renewal provider payment id: %w", updateErr)
		}
	}

	switch chargeResult.Status {
	case domain.PaymentStatusSucceeded:
		if err := txResultSubscriptionPayments.MarkSucceeded(ctx, payment.ID, now); err != nil {
			if recErr := recoverWithClosedTx(); recErr != nil {
				return fmt.Errorf("mark renewal payment succeeded: %w; recovery failed: %w", err, recErr)
			}
			return fmt.Errorf("mark renewal payment succeeded: %w", err)
		}
		if err := payment.MarkSucceeded(now); err != nil {
			return fmt.Errorf("apply renewal succeeded transition: %w", err)
		}
		if err := resultTx.Commit(ctx); err != nil {
			return fmt.Errorf("commit post-charge transaction: %w", err)
		}
		// Subscription renewal/tariff change and property archiving are
		// best-effort compensating operations: they must not roll back a payment
		// that has already been charged.
		sub, renewalTariff, oldTariffID, ok := applySubscriptionRenewalBestEffort(ctx, renewalBestEffortDeps{
			beginner:      r.deps.beginner,
			subscriptions: r.deps.subscriptions,
			tariffs:       r.deps.tariffs,
			log:           r.deps.log,
		}, sub.ID, payment, now)
		if ok && oldTariffID != sub.TariffID {
			archiveExcessPropertiesBestEffort(ctx, archiveDeps{
				propertyArchiver:      r.deps.propertyArchiver,
				recipientSlotEnforcer: r.deps.recipientSlotEnforcer,
				beginner:              r.deps.beginner,
				log:                   r.deps.log,
			}, sub.UserID, renewalTariff.ActivePropertyLimit, "renewal_downgrade")
		}
		return nil
	case domain.PaymentStatusFailed:
		if err := txResultSubscriptionPayments.MarkFailed(ctx, payment.ID, providerResultErrorCode(chargeResult.ErrorCode, chargeErr), now); err != nil {
			if recErr := recoverWithClosedTx(); recErr != nil {
				return fmt.Errorf("mark renewal payment failed: %w; recovery failed: %w", err, recErr)
			}
			return fmt.Errorf("mark renewal payment failed: %w", err)
		}
		sub, err = txResultSubscriptions.GetByIDForUpdate(ctx, sub.ID)
		if err != nil {
			if recErr := recoverWithClosedTx(); recErr != nil {
				return fmt.Errorf("get subscription for update after failed charge: %w; recovery failed: %w", err, recErr)
			}
			return fmt.Errorf("get subscription for update after failed charge: %w", err)
		}
		sub.EnterGrace(now)
		if err := txResultSubscriptions.Update(ctx, sub); err != nil {
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
// if necessary, move the subscription to grace. cause is the provider error
// that triggered the recovery (nil when the outcome is uncertain for other
// reasons); its provider error code is persisted on a failed payment.
func (r *RenewalService) recoverRenewalFailure(ctx context.Context, paymentID, subscriptionID uuid.UUID, providerPaymentIDHint string, cause error, now time.Time) error {
	errorCode := providerErrorCode(cause)

	switch {
	case errors.Is(cause, ErrProviderChargeBlocked):
		// T-Kassa error 10: charging is disabled on the terminal. Renewals
		// cannot succeed until COF/recurring operations are enabled.
		r.deps.log.ErrorContext(ctx, "renewal charge blocked by provider: COF/recurring operations are not enabled on the terminal; contact the T-Bank manager to enable them",
			slog.String("subscription_id", subscriptionID.String()),
			slog.String("payment_id", paymentID.String()),
			slog.String("error", sanitize.Error(cause)))
	case errors.Is(cause, ErrProviderInvalidOperation):
		// T-Kassa errors 1125/1126: the operation parameters are inconsistent.
		// This does not fix itself and needs an integration change.
		r.deps.log.ErrorContext(ctx, "renewal rejected by provider as an invalid operation: integration misconfiguration (for example a mismatched OperationInitiatorType); investigate the provider integration",
			slog.String("subscription_id", subscriptionID.String()),
			slog.String("payment_id", paymentID.String()),
			slog.String("error", sanitize.Error(cause)))
	}

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

	if payment.Status != domain.PaymentStatusPending {
		// Nothing more to do for finalized or unexpected statuses.
		return nil
	}

	if payment.ProviderPaymentID == nil {
		return r.markRenewalFailedAndGrace(ctx, paymentID, subscriptionID, errorCode, now)
	}

	statusResult, err := r.provider.Status(ctx, paymentID, *payment.ProviderPaymentID)
	if err != nil {
		// The money status is unknown, so this must not count as a charge
		// attempt: the payment may already be charged on the provider side.
		r.deps.log.WarnContext(ctx, "provider status unknown during renewal recovery; leaving subscription active and payment pending",
			slog.String("subscription_id", subscriptionID.String()),
			slog.String("payment_id", paymentID.String()),
			slog.String("error", sanitize.Error(err)))
		return nil
	}
	status := statusResult.Status

	if status == domain.PaymentStatusFailed {
		return r.markRenewalFailedAndGrace(ctx, paymentID, subscriptionID, providerResultErrorCode(statusResult.ErrorCode, cause), now)
	}

	if status == domain.PaymentStatusSucceeded {
		return r.markRenewalSucceededAndApply(ctx, payment, subscriptionID, now)
	}

	// The charge is still pending at the provider or the status is unexpected.
	// Count the attempt and give up after maxRenewalChargeAttempts so that a
	// permanently failing charge (e.g. blocked recurrent payments) does not
	// keep the expired subscription active forever.
	attempts, incErr := r.deps.subscriptionPayments.IncrementChargeAttempts(ctx, paymentID)
	if incErr != nil {
		return fmt.Errorf("increment charge attempts during renewal recovery: %w", incErr)
	}
	if attempts >= maxRenewalChargeAttempts {
		return r.markRenewalFailedAndGrace(ctx, paymentID, subscriptionID, errorCode, now)
	}

	logAttrs := []any{
		slog.String("subscription_id", subscriptionID.String()),
		slog.String("payment_id", paymentID.String()),
		slog.String("status", string(status)),
		slog.Int("charge_attempts", attempts),
	}
	if status == domain.PaymentStatusPending {
		r.deps.log.WarnContext(ctx, "provider status pending during renewal recovery; leaving subscription active and payment pending", logAttrs...)
		return nil
	}
	r.deps.log.WarnContext(ctx, "unexpected provider status during renewal recovery; leaving subscription active and payment pending", logAttrs...)
	return nil
}

func (r *RenewalService) persistProviderPaymentIDHint(ctx context.Context, paymentID uuid.UUID, providerPaymentIDHint string) error {
	tx, err := r.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin provider payment id hint transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txSubscriptionPayments, err := r.deps.subscriptionPayments.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind subscription payments transaction: %w", err)
	}

	payment, err := txSubscriptionPayments.GetByIDForUpdate(ctx, paymentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return fmt.Errorf("get payment for provider payment id hint: %w", err)
	}
	if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID != providerPaymentIDHint {
		if _, updateErr := txSubscriptionPayments.UpdateProviderPaymentID(ctx, paymentID, providerPaymentIDHint); updateErr != nil {
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

	txSubscriptionPayments, err := r.deps.subscriptionPayments.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind subscription payments transaction: %w", err)
	}
	txSubscriptions, err := r.deps.subscriptions.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind subscriptions transaction: %w", err)
	}

	if err := txSubscriptionPayments.MarkFailed(ctx, paymentID, errorCode, now); err != nil {
		return fmt.Errorf("mark payment failed in recovery: %w", err)
	}

	sub, err := txSubscriptions.GetByIDForUpdate(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("get subscription for recovery: %w", err)
	}
	sub.EnterGrace(now)
	if err := txSubscriptions.Update(ctx, sub); err != nil {
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

	txSubscriptionPayments, err := r.deps.subscriptionPayments.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind subscription payments transaction: %w", err)
	}

	if err := txSubscriptionPayments.MarkSucceeded(ctx, payment.ID, now); err != nil {
		return fmt.Errorf("mark payment succeeded in recovery: %w", err)
	}
	if err := payment.MarkSucceeded(now); err != nil {
		return fmt.Errorf("apply recovery succeeded transition: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit recovery transaction: %w", err)
	}

	sub, renewalTariff, oldTariffID, ok := applySubscriptionRenewalBestEffort(ctx, renewalBestEffortDeps{
		beginner:      r.deps.beginner,
		subscriptions: r.deps.subscriptions,
		tariffs:       r.deps.tariffs,
		log:           r.deps.log,
	}, subscriptionID, payment, now)
	if ok && oldTariffID != sub.TariffID {
		archiveExcessPropertiesBestEffort(ctx, archiveDeps{
			propertyArchiver:      r.deps.propertyArchiver,
			recipientSlotEnforcer: r.deps.recipientSlotEnforcer,
			beginner:              r.deps.beginner,
			log:                   r.deps.log,
		}, sub.UserID, renewalTariff.ActivePropertyLimit, "renewal_downgrade")
	}
	return nil
}

func (r *RenewalService) transitionSubscriptionToGrace(ctx context.Context, subscriptionID uuid.UUID, now time.Time) error {
	tx, err := r.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin recovery transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txSubscriptions, err := r.deps.subscriptions.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind subscriptions transaction: %w", err)
	}

	sub, err := txSubscriptions.GetByIDForUpdate(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("get subscription for recovery: %w", err)
	}
	sub.EnterGrace(now)
	if err := txSubscriptions.Update(ctx, sub); err != nil {
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
	txTariffs, err := r.deps.tariffs.WithTx(tx)
	if err != nil {
		return domain.Tariff{}, "", 0, fmt.Errorf("bind tariffs transaction: %w", err)
	}

	if sub.PendingTariffID != nil && sub.PendingPeriod != nil {
		pendingTariff, err := txTariffs.GetByID(ctx, *sub.PendingTariffID)
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

	currentTariff, err := txTariffs.GetByID(ctx, sub.TariffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Tariff{}, "", 0, ErrTariffNotFound
		}
		return domain.Tariff{}, "", 0, fmt.Errorf("get current tariff: %w", err)
	}

	// The renewal period is subscription state, not payment history: right
	// after a scheduled downgrade the last succeeded payment still references
	// the old tariff's period, so it cannot be used here.
	period := domain.PeriodMonth
	if sub.CurrentPeriod != nil {
		period = *sub.CurrentPeriod
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
		batchProcessed := 0
		for _, sub := range subs {
			if err := r.downgradeToBasic(ctx, sub, basicTariff, now.UTC()); err != nil {
				r.deps.log.ErrorContext(ctx, "downgrade to basic after grace failed",
					slog.String("subscription_id", sub.ID.String()),
					slog.String("user_id", sub.UserID.String()),
					slog.String("error", sanitize.Error(err)))
				continue
			}
			batchProcessed++
		}
		processed += batchProcessed
		if len(subs) < graceBatchSize {
			break
		}
		if batchProcessed == 0 {
			// A full batch with zero progress means every item failed and
			// stays in the selection; defer to the next tick instead of
			// spinning in an infinite retry loop.
			r.deps.log.WarnContext(ctx, "batch made no progress; deferring to next tick",
				slog.String("op", "expire grace"))
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

	txSubscriptions, err := r.deps.subscriptions.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind subscriptions transaction: %w", err)
	}

	sub, err = txSubscriptions.GetByIDForUpdate(ctx, sub.ID)
	if err != nil {
		return fmt.Errorf("get subscription for update: %w", err)
	}

	if sub.Status != domain.SubscriptionStatusGrace || sub.ValidUntil == nil || sub.ValidUntil.After(now) {
		return nil
	}

	sub.DowngradeToBasic(basicTariff.ID)
	if err := txSubscriptions.Update(ctx, sub); err != nil {
		return fmt.Errorf("update subscription after grace downgrade: %w", err)
	}

	if r.deps.propertyArchiver != nil {
		if err := r.deps.propertyArchiver.ArchiveExcessProperties(ctx, tx, sub.UserID, basicTariff.ActivePropertyLimit); err != nil {
			return fmt.Errorf("archive excess properties after grace downgrade: %w", err)
		}
	}
	if r.deps.recipientSlotEnforcer != nil {
		if err := r.deps.recipientSlotEnforcer.EnforceRecipientLimit(ctx, tx, sub.UserID, "grace_expired"); err != nil {
			return fmt.Errorf("enforce recipient slot limit after grace downgrade: %w", err)
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

	txSubscriptions, err := r.deps.subscriptions.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind subscriptions transaction: %w", err)
	}

	sub, err = txSubscriptions.GetByIDForUpdate(ctx, sub.ID)
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

	sub.DowngradeToBasic(basicTariff.ID)
	if err := txSubscriptions.Update(ctx, sub); err != nil {
		return fmt.Errorf("update subscription after non-renewing expiry: %w", err)
	}

	if r.deps.propertyArchiver != nil {
		if err := r.deps.propertyArchiver.ArchiveExcessProperties(ctx, tx, sub.UserID, basicTariff.ActivePropertyLimit); err != nil {
			return fmt.Errorf("archive excess properties after non-renewing expiry: %w", err)
		}
	}
	if r.deps.recipientSlotEnforcer != nil {
		if err := r.deps.recipientSlotEnforcer.EnforceRecipientLimit(ctx, tx, sub.UserID, "non_renewing_expired"); err != nil {
			return fmt.Errorf("enforce recipient slot limit after non-renewing expiry: %w", err)
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

		finalized := 0
		for _, payment := range payments {
			if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID == "" {
				continue
			}

			statusResult, statusErr := r.provider.Status(ctx, payment.ID, *payment.ProviderPaymentID)
			processed++

			if statusErr != nil {
				r.deps.log.WarnContext(ctx, "failed to query provider status for pending upgrade payment",
					slog.String("payment_id", payment.ID.String()),
					slog.String("error", sanitize.Error(statusErr)))
				continue
			}

			switch statusResult.Status {
			case domain.PaymentStatusSucceeded:
				// Recover the saved-card token when the AUTHORIZED webhook that
				// delivers it was lost; no-op when the payment is already linked
				// or the provider does not report a RebillId.
				recoverPaymentMethodRebillID(ctx, recoverPaymentMethodDeps{
					beginner:             r.deps.beginner,
					subscriptionPayments: r.deps.subscriptionPayments,
					paymentMethods:       r.deps.paymentMethods,
					provider:             r.provider,
					log:                  r.deps.log,
				}, &payment, statusResult.RebillID, now.UTC())
				if err := r.markRenewalSucceededAndApply(ctx, payment, payment.SubscriptionID, now.UTC()); err != nil {
					r.deps.log.ErrorContext(ctx, "failed to apply succeeded upgrade payment",
						slog.String("payment_id", payment.ID.String()),
						slog.String("error", sanitize.Error(err)))
					continue
				}
				finalized++
			case domain.PaymentStatusFailed:
				markPaymentFailedBestEffort(ctx, markFailedDeps{
					beginner:             r.deps.beginner,
					subscriptionPayments: r.deps.subscriptionPayments,
					log:                  r.deps.log,
				}, payment.ID, now.UTC())
				finalized++
			case domain.PaymentStatusPending:
				// Provider has not finalized the payment yet; leave it pending.
			default:
				r.deps.log.WarnContext(ctx, "unexpected provider status for pending upgrade payment",
					slog.String("payment_id", payment.ID.String()),
					slog.String("status", string(statusResult.Status)))
			}
		}

		if len(payments) < renewalBatchSize {
			break
		}
		if finalized == 0 {
			// A full batch with zero finalized payments means every item
			// stays in the selection; defer to the next tick instead of
			// spinning in an infinite retry loop.
			r.deps.log.WarnContext(ctx, "batch made no progress; deferring to next tick",
				slog.String("op", "pending upgrade payments"))
			break
		}
	}

	return processed, nil
}
