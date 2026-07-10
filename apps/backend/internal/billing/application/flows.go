package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// flowDeps bundles the shared dependencies used by the billing sub-services and
// the cross-cutting flow helpers in this file. It carries the narrow provider
// ports so individual helpers depend only on what they use.
type flowDeps struct {
	tariffs              TariffRepository
	subscriptions        SubscriptionRepository
	paymentMethods       PaymentMethodRepository
	subscriptionPayments SubscriptionPaymentRepository
	propertyArchiver     PropertyArchiver
	provider             Provider
	beginner             transaction.Beginner
	clock                clock.Clock
	log                  *slog.Logger
	callbackBaseURL      string
}

func newFlowDeps(
	tariffs TariffRepository,
	subscriptions SubscriptionRepository,
	paymentMethods PaymentMethodRepository,
	subscriptionPayments SubscriptionPaymentRepository,
	propertyArchiver PropertyArchiver,
	provider Provider,
	beginner transaction.Beginner,
	clk clock.Clock,
	log *slog.Logger,
	callbackBaseURL string,
) flowDeps {
	if log == nil {
		log = slog.Default()
	}
	return flowDeps{
		tariffs:              tariffs,
		subscriptions:        subscriptions,
		paymentMethods:       paymentMethods,
		subscriptionPayments: subscriptionPayments,
		propertyArchiver:     propertyArchiver,
		provider:             provider,
		beginner:             beginner,
		clock:                clk,
		log:                  log,
		callbackBaseURL:      callbackBaseURL,
	}
}

func applyPaymentResult(
	ctx context.Context,
	d flowDeps,
	tx transaction.Tx,
	payment *domain.SubscriptionPayment,
	payload WebhookPayload,
) error {
	if payment.ProviderPaymentID != nil && *payment.ProviderPaymentID != payload.ProviderPaymentID {
		return fmt.Errorf("provider payment id mismatch: expected %q, got %q", *payment.ProviderPaymentID, payload.ProviderPaymentID)
	}

	txSubscriptionPayments, err := d.subscriptionPayments.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind subscription payments transaction: %w", err)
	}
	txPaymentMethods, err := d.paymentMethods.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind payment methods transaction: %w", err)
	}

	// If the provider payment id was not persisted during Init (e.g. recovery
	// path or concurrent webhook), store it from the webhook payload before
	// finalizing the payment.
	if payment.ProviderPaymentID == nil && payload.ProviderPaymentID != "" {
		updated, err := txSubscriptionPayments.UpdateProviderPaymentID(ctx, payment.ID, payload.ProviderPaymentID)
		if err != nil {
			return fmt.Errorf("persist provider payment id from webhook: %w", err)
		}
		payment.ProviderPaymentID = updated.ProviderPaymentID
	}

	now := d.clock.Now().UTC()
	switch payload.Status {
	case domain.PaymentStatusSucceeded:
		// Out-of-order webhooks may reconcile an already-failed payment to
		// succeeded after an explicit provider-side status check.
		if payment.Status == domain.PaymentStatusFailed {
			if err := txSubscriptionPayments.MarkReconciledSucceeded(ctx, payment.ID, now); err != nil {
				return fmt.Errorf("reconcile payment to succeeded: %w", err)
			}
		} else {
			if err := txSubscriptionPayments.MarkSucceeded(ctx, payment.ID, now); err != nil {
				return fmt.Errorf("mark payment succeeded: %w", err)
			}
		}
		payment.Status = domain.PaymentStatusSucceeded
		payment.UpdatedAt = now
		payment.SucceededAt = &now
		payment.ErrorCode = nil

		// Payment method activation is part of the critical transaction: a
		// succeeded payment means the saved token is valid and should become the
		// active method. Subscription renewal/tariff change runs afterwards as a
		// best-effort step so a domain failure cannot roll back the charge.
		methodID := payment.PaymentMethodID
		if payload.RebillID != "" {
			pm, err := upsertPaymentMethodFromWebhook(ctx, d, tx, payment.UserID, payload)
			if err != nil {
				return err
			}
			methodID = &pm.ID
			if payment.PaymentMethodID == nil {
				if _, updateErr := txSubscriptionPayments.UpdatePaymentMethodAndProviderID(ctx, payment.ID, pm.ID, payload.ProviderPaymentID); updateErr != nil {
					return fmt.Errorf("update payment method id: %w", updateErr)
				}
			}
		}

		if methodID != nil {
			if err := txPaymentMethods.SetActive(ctx, payment.UserID, *methodID); err != nil {
				return fmt.Errorf("activate payment method: %w", err)
			}
		}

	case domain.PaymentStatusFailed:
		if err := txSubscriptionPayments.MarkFailed(ctx, payment.ID, payload.ErrorCode, now); err != nil {
			return fmt.Errorf("mark payment failed: %w", err)
		}
		payment.Status = domain.PaymentStatusFailed
		payment.ErrorCode = payload.ErrorCode
		payment.UpdatedAt = now

	case domain.PaymentStatusRefunded, domain.PaymentStatusPartialRefunded:
		// The system no longer initiates partial refunds. If a provider reports
		// one anyway (external anomaly), record a full refund and warn.
		if payload.Status == domain.PaymentStatusPartialRefunded {
			d.log.WarnContext(ctx, "provider reported partial refund, which the system no longer initiates",
				slog.String("payment_id", payment.ID.String()),
				slog.String("provider_payment_id", payload.ProviderPaymentID),
				slog.Int64("payload_amount_kopecks", payload.AmountKopecks))
		}
		// Out-of-order webhooks may reconcile an already-failed payment to
		// refunded after an explicit provider-side status check.
		if payment.Status == domain.PaymentStatusFailed {
			if err := txSubscriptionPayments.MarkReconciledRefunded(ctx, payment.ID, now); err != nil {
				return fmt.Errorf("reconcile payment to refunded: %w", err)
			}
		} else {
			if err := txSubscriptionPayments.MarkRefunded(ctx, payment.ID, now); err != nil {
				return fmt.Errorf("mark payment refunded: %w", err)
			}
		}
		payment.Status = domain.PaymentStatusRefunded
		payment.RefundedAmountKopecks = &payment.AmountKopecks
		payment.UpdatedAt = now

	default:
		return fmt.Errorf("unsupported webhook status: %s", payload.Status)
	}

	return nil
}

// applyRefundToSubscription downgrades the subscription to basic after a refund.
func applyRefundToSubscription(ctx context.Context, d flowDeps, tx transaction.Tx, subscriptionID uuid.UUID) error {
	txSubscriptions, err := d.subscriptions.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind subscriptions transaction: %w", err)
	}

	sub, err := txSubscriptions.GetByIDForUpdate(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("get subscription for refund: %w", err)
	}
	basicTariff, err := d.tariffs.GetByName(ctx, domain.TariffBasic)
	if err != nil {
		return fmt.Errorf("get basic tariff for refund: %w", err)
	}
	sub.DowngradeToBasic(basicTariff.ID)
	if err := txSubscriptions.Update(ctx, sub); err != nil {
		return fmt.Errorf("downgrade subscription to basic after refund: %w", err)
	}
	if d.propertyArchiver != nil {
		if err := d.propertyArchiver.ArchiveExcessProperties(ctx, tx, sub.UserID, basicTariff.ActivePropertyLimit); err != nil {
			return fmt.Errorf("archive excess properties after refund: %w", err)
		}
	}
	return nil
}

// applySubscriptionRenewalAndArchive runs the best-effort subscription renewal
// and then archives excess properties when the tariff changed to a lower limit.
func applySubscriptionRenewalAndArchive(ctx context.Context, d flowDeps, payment domain.SubscriptionPayment) {
	now := d.clock.Now().UTC()
	sub, renewalTariff, oldTariffID, ok := applySubscriptionRenewalBestEffort(ctx, d, payment.SubscriptionID, payment, now)
	if !ok {
		return
	}
	// Archiving is only relevant when the tariff changed, because only then can
	// the property limit decrease.
	if oldTariffID == sub.TariffID {
		return
	}
	archiveExcessPropertiesBestEffort(ctx, d, sub.UserID, renewalTariff.ActivePropertyLimit)
}

// applySubscriptionRenewalBestEffort applies the subscription side of a
// successful payment in a separate transaction. This keeps MarkSucceeded in the
// critical transaction path: if the renewal/tariff change fails, the payment
// stays succeeded and the error is logged for manual review.
func applySubscriptionRenewalBestEffort(ctx context.Context, d flowDeps, subscriptionID uuid.UUID, payment domain.SubscriptionPayment, now time.Time) (domain.Subscription, domain.Tariff, uuid.UUID, bool) {
	tx, err := d.beginner.Begin(ctx)
	if err != nil {
		d.log.ErrorContext(ctx, "failed to begin transaction for best-effort subscription renewal",
			slog.String("subscription_id", subscriptionID.String()),
			slog.String("payment_id", payment.ID.String()),
			slog.String("error", sanitize.Error(err)))
		return domain.Subscription{}, domain.Tariff{}, uuid.UUID{}, false
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txSubscriptions, err := d.subscriptions.WithTx(tx)
	if err != nil {
		d.log.ErrorContext(ctx, "failed to bind subscription transaction for best-effort renewal",
			slog.String("subscription_id", subscriptionID.String()),
			slog.String("payment_id", payment.ID.String()),
			slog.String("error", sanitize.Error(err)))
		return domain.Subscription{}, domain.Tariff{}, uuid.UUID{}, false
	}

	sub, err := txSubscriptions.GetByIDForUpdate(ctx, subscriptionID)
	if err != nil {
		d.log.ErrorContext(ctx, "failed to load subscription for best-effort renewal",
			slog.String("subscription_id", subscriptionID.String()),
			slog.String("payment_id", payment.ID.String()),
			slog.String("error", sanitize.Error(err)))
		return domain.Subscription{}, domain.Tariff{}, uuid.UUID{}, false
	}
	oldTariffID := sub.TariffID

	// Idempotency: if the subscription already reflects this payment, skip.
	if isSubscriptionRenewalApplied(sub, payment) {
		return sub, domain.Tariff{}, oldTariffID, true
	}

	if payment.PaymentMethodID != nil {
		sub.ActivePaymentMethodID = payment.PaymentMethodID
	}

	sub, renewalTariff, err := applyRenewalChanges(ctx, d, tx, sub, payment, now)
	if err != nil {
		d.log.ErrorContext(ctx, "best-effort subscription renewal failed",
			slog.String("subscription_id", subscriptionID.String()),
			slog.String("payment_id", payment.ID.String()),
			slog.String("error", sanitize.Error(err)))
		return domain.Subscription{}, domain.Tariff{}, uuid.UUID{}, false
	}

	if err := tx.Commit(ctx); err != nil {
		d.log.ErrorContext(ctx, "failed to commit best-effort subscription renewal transaction",
			slog.String("subscription_id", subscriptionID.String()),
			slog.String("payment_id", payment.ID.String()),
			slog.String("error", sanitize.Error(err)))
		return domain.Subscription{}, domain.Tariff{}, uuid.UUID{}, false
	}

	return sub, renewalTariff, oldTariffID, true
}

func applyRenewalChanges(ctx context.Context, d flowDeps, tx transaction.Tx, sub domain.Subscription, payment domain.SubscriptionPayment, now time.Time) (domain.Subscription, domain.Tariff, error) {
	txTariffs, err := d.tariffs.WithTx(tx)
	if err != nil {
		return domain.Subscription{}, domain.Tariff{}, fmt.Errorf("bind tariffs transaction: %w", err)
	}
	txSubscriptions, err := d.subscriptions.WithTx(tx)
	if err != nil {
		return domain.Subscription{}, domain.Tariff{}, fmt.Errorf("bind subscriptions transaction: %w", err)
	}

	renewalTariff, err := txTariffs.GetByID(ctx, payment.TariffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Subscription{}, domain.Tariff{}, ErrTariffNotFound
		}
		return domain.Subscription{}, domain.Tariff{}, fmt.Errorf("get renewal tariff: %w", err)
	}

	if sub.TariffID == renewalTariff.ID {
		if err := sub.ApplyRenewal(payment.ID, payment.Period, now); err != nil {
			return domain.Subscription{}, domain.Tariff{}, fmt.Errorf("apply renewal: %w", err)
		}
	} else {
		currentTariff, err := txTariffs.GetByID(ctx, sub.TariffID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return domain.Subscription{}, domain.Tariff{}, ErrTariffNotFound
			}
			return domain.Subscription{}, domain.Tariff{}, fmt.Errorf("get current tariff for change: %w", err)
		}
		if err := sub.ApplyTariffChange(payment.ID, currentTariff, renewalTariff, payment.Period, now); err != nil {
			return domain.Subscription{}, domain.Tariff{}, fmt.Errorf("apply tariff change: %w", err)
		}
	}

	if err := txSubscriptions.Update(ctx, sub); err != nil {
		return domain.Subscription{}, domain.Tariff{}, fmt.Errorf("update subscription after renewal: %w", err)
	}
	return sub, renewalTariff, nil
}

func applyFreeRenewalOrDowngrade(ctx context.Context, d flowDeps, tx transaction.Tx, sub *domain.Subscription, renewalTariff domain.Tariff, period domain.SubscriptionPeriod, paymentID uuid.UUID, now time.Time) error {
	txSubscriptions, err := d.subscriptions.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind subscriptions transaction: %w", err)
	}
	txTariffs, err := d.tariffs.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind tariffs transaction: %w", err)
	}

	// The free basic tariff has no validity period and cannot be auto-renewed.
	if renewalTariff.Name == domain.TariffBasic {
		sub.DowngradeToBasic(renewalTariff.ID)
		if paymentID != uuid.Nil {
			sub.LastAppliedPaymentID = &paymentID
		}
		if err := txSubscriptions.Update(ctx, *sub); err != nil {
			return fmt.Errorf("update subscription after free downgrade to basic: %w", err)
		}
		if d.propertyArchiver != nil {
			if err := d.propertyArchiver.ArchiveExcessProperties(ctx, tx, sub.UserID, renewalTariff.ActivePropertyLimit); err != nil {
				return fmt.Errorf("archive excess properties after free downgrade to basic: %w", err)
			}
		}
		return nil
	}

	if sub.TariffID == renewalTariff.ID {
		if err := sub.ApplyRenewal(paymentID, period, now); err != nil {
			return fmt.Errorf("apply free renewal: %w", err)
		}
	} else {
		currentTariff, err := txTariffs.GetByID(ctx, sub.TariffID)
		if err != nil {
			return fmt.Errorf("get current tariff for free change: %w", err)
		}
		if err := sub.ApplyTariffChange(paymentID, currentTariff, renewalTariff, period, now); err != nil {
			return fmt.Errorf("apply free tariff change: %w", err)
		}
		if d.propertyArchiver != nil {
			if err := d.propertyArchiver.ArchiveExcessProperties(ctx, tx, sub.UserID, renewalTariff.ActivePropertyLimit); err != nil {
				return fmt.Errorf("archive excess properties after free downgrade: %w", err)
			}
		}
	}
	if err := txSubscriptions.Update(ctx, *sub); err != nil {
		return fmt.Errorf("update subscription after free renewal: %w", err)
	}
	return nil
}

func archiveExcessPropertiesBestEffort(ctx context.Context, d flowDeps, userID uuid.UUID, limit int) {
	if d.propertyArchiver == nil {
		return
	}

	tx, err := d.beginner.Begin(ctx)
	if err != nil {
		d.log.ErrorContext(ctx, "failed to begin transaction for best-effort property archiving",
			slog.String("user_id", userID.String()),
			slog.String("error", sanitize.Error(err)))
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := d.propertyArchiver.ArchiveExcessProperties(ctx, tx, userID, limit); err != nil {
		d.log.ErrorContext(ctx, "best-effort property archiving failed",
			slog.String("user_id", userID.String()),
			slog.String("error", sanitize.Error(err)))
		return
	}

	if err := tx.Commit(ctx); err != nil {
		d.log.ErrorContext(ctx, "failed to commit best-effort property archiving transaction",
			slog.String("user_id", userID.String()),
			slog.String("error", sanitize.Error(err)))
	}
}

func buildPaymentMethodFromWebhook(d flowDeps, userID uuid.UUID, payload WebhookPayload) (domain.PaymentMethod, error) {
	pm, err := domain.NewPaymentMethod(
		userID,
		d.provider.Name(),
		payload.RebillID,
		payload.Pan,
		d.clock.Now().UTC(),
	)
	if err != nil {
		return domain.PaymentMethod{}, fmt.Errorf("create payment method from webhook: %w", err)
	}
	pm.ProviderCardID = payload.CardID
	pm.ExpDate = payload.ExpDate
	return pm, nil
}

func upsertPaymentMethodFromWebhook(ctx context.Context, d flowDeps, tx transaction.Tx, userID uuid.UUID, payload WebhookPayload) (domain.PaymentMethod, error) {
	pm, err := buildPaymentMethodFromWebhook(d, userID, payload)
	if err != nil {
		return domain.PaymentMethod{}, err
	}
	txPaymentMethods, err := d.paymentMethods.WithTx(tx)
	if err != nil {
		return domain.PaymentMethod{}, fmt.Errorf("bind payment methods transaction: %w", err)
	}
	pm, err = txPaymentMethods.UpsertByTokenHash(ctx, pm)
	if err != nil {
		return domain.PaymentMethod{}, fmt.Errorf("upsert payment method from webhook: %w", err)
	}
	return pm, nil
}

// saveProviderInitResult persists the provider reference and any newly created
// payment method atomically in a single transaction. If the transaction cannot
// be committed, the pending payment is marked failed so it does not stay
// unfinished.
func saveProviderInitResult(ctx context.Context, d flowDeps, payment domain.SubscriptionPayment, initRes InitResult, userID uuid.UUID, now time.Time) (domain.SubscriptionPayment, error) {
	paymentID := payment.ID

	tx, err := d.beginner.Begin(ctx)
	if err != nil {
		markPaymentFailedBestEffort(ctx, d, paymentID, now)
		return domain.SubscriptionPayment{}, fmt.Errorf("begin provider result transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txSubscriptionPayments, err := d.subscriptionPayments.WithTx(tx)
	if err != nil {
		return domain.SubscriptionPayment{}, fmt.Errorf("bind subscription payments transaction: %w", err)
	}
	txPaymentMethods, err := d.paymentMethods.WithTx(tx)
	if err != nil {
		return domain.SubscriptionPayment{}, fmt.Errorf("bind payment methods transaction: %w", err)
	}

	// Reload the payment under lock: the provider call happened outside of a
	// transaction, so the row must be locked before the reference is persisted.
	payment, err = txSubscriptionPayments.GetByIDForUpdate(ctx, paymentID)
	if err != nil {
		rollbackAndMarkFailedBestEffort(ctx, tx, d, paymentID, now)
		return domain.SubscriptionPayment{}, fmt.Errorf("get payment for provider result: %w", err)
	}
	if payment.IsFinalized() {
		// The payment was already finalized by a concurrent webhook or another
		// request. Return the current state without overwriting it.
		return payment, nil
	}
	if payment.Status != domain.PaymentStatusPending {
		rollbackAndMarkFailedBestEffort(ctx, tx, d, paymentID, now)
		return domain.SubscriptionPayment{}, domain.ErrInvalidPaymentStatus
	}

	if initRes.SavedToken != "" {
		pm, err := domain.NewPaymentMethod(
			userID,
			d.provider.Name(),
			initRes.SavedToken,
			maskToken(initRes.SavedToken),
			now,
		)
		if err != nil {
			rollbackAndMarkFailedBestEffort(ctx, tx, d, paymentID, now)
			return domain.SubscriptionPayment{}, fmt.Errorf("create payment method: %w", err)
		}
		pm, err = txPaymentMethods.Create(ctx, pm)
		if err != nil {
			rollbackAndMarkFailedBestEffort(ctx, tx, d, paymentID, now)
			return domain.SubscriptionPayment{}, fmt.Errorf("save payment method: %w", err)
		}
		payment, err = txSubscriptionPayments.UpdatePaymentMethodAndProviderID(ctx, paymentID, pm.ID, initRes.ProviderPaymentID)
		if err != nil {
			rollbackAndMarkFailedBestEffort(ctx, tx, d, paymentID, now)
			return domain.SubscriptionPayment{}, fmt.Errorf("update payment method and provider payment id: %w", err)
		}
	} else {
		payment, err = txSubscriptionPayments.UpdateProviderPaymentID(ctx, paymentID, initRes.ProviderPaymentID)
		if err != nil {
			rollbackAndMarkFailedBestEffort(ctx, tx, d, paymentID, now)
			return domain.SubscriptionPayment{}, fmt.Errorf("update provider payment id: %w", err)
		}
	}

	if initRes.PaymentURL != "" {
		payment, err = txSubscriptionPayments.UpdatePaymentURL(ctx, paymentID, initRes.PaymentURL)
		if err != nil {
			rollbackAndMarkFailedBestEffort(ctx, tx, d, paymentID, now)
			return domain.SubscriptionPayment{}, fmt.Errorf("update payment url: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		rollbackAndMarkFailedBestEffort(ctx, tx, d, paymentID, now)
		return domain.SubscriptionPayment{}, fmt.Errorf("commit provider result transaction: %w", err)
	}

	return payment, nil
}

// rollbackAndMarkFailedBestEffort closes the still-open outer transaction (releasing
// the row lock) and only then marks the payment as failed in a separate transaction.
// Calling Rollback on an already-closed transaction returns an error that is ignored,
// so this is safe to use on every error path of saveProviderInitResult.
func rollbackAndMarkFailedBestEffort(ctx context.Context, tx transaction.Tx, d flowDeps, paymentID uuid.UUID, now time.Time) {
	_ = tx.Rollback(ctx)
	markPaymentFailedBestEffort(ctx, d, paymentID, now)
}

// markPaymentFailedBestEffort marks a pending payment as failed in a separate
// transaction. It is used when a transaction that should have finalized the
// payment has already failed and we need to avoid leaving the record pending.
func markPaymentFailedBestEffort(ctx context.Context, d flowDeps, paymentID uuid.UUID, now time.Time) {
	tx, err := d.beginner.Begin(ctx)
	if err != nil {
		d.log.ErrorContext(ctx, "failed to begin transaction for best-effort payment failure mark",
			slog.String("payment_id", paymentID.String()),
			slog.String("error", sanitize.Error(err)))
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txSubscriptionPayments, err := d.subscriptionPayments.WithTx(tx)
	if err != nil {
		d.log.ErrorContext(ctx, "failed to bind subscription payments transaction for best-effort payment failure mark",
			slog.String("payment_id", paymentID.String()),
			slog.String("error", sanitize.Error(err)))
		return
	}

	if err := txSubscriptionPayments.MarkFailed(ctx, paymentID, nil, now); err != nil {
		d.log.ErrorContext(ctx, "failed to mark payment failed in best-effort transaction",
			slog.String("payment_id", paymentID.String()),
			slog.String("error", sanitize.Error(err)))
		return
	}

	if err := tx.Commit(ctx); err != nil {
		d.log.ErrorContext(ctx, "failed to commit best-effort payment failure mark",
			slog.String("payment_id", paymentID.String()),
			slog.String("error", sanitize.Error(err)))
	}
}
