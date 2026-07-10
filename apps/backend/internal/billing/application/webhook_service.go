package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

// WebhookService handles provider webhooks.
type WebhookService struct {
	deps flowDeps
}

// NewWebhookService creates a WebhookService.
func NewWebhookService(deps flowDeps) *WebhookService {
	return &WebhookService{deps: deps}
}

// WebhookResponse returns the provider-specific response body that must be sent
// back after a webhook is handled.
func (s *WebhookService) WebhookResponse() []byte {
	return s.deps.provider.WebhookResponse()
}

// HandleWebhook parses and applies a provider webhook payload.
func (s *WebhookService) HandleWebhook(ctx context.Context, providerName string, payload []byte) error {
	if providerName != string(s.deps.provider.Name()) {
		return fmt.Errorf("unexpected provider %q, expected %q", providerName, s.deps.provider.Name())
	}

	result, err := s.deps.provider.ParseWebhook(ctx, payload)
	if err != nil {
		return fmt.Errorf("parse webhook: %w", err)
	}

	s.deps.log.InfoContext(ctx, "processing payment webhook",
		slog.String("provider", providerName),
		slog.String("notification_type", result.NotificationType),
		slog.String("status", string(result.Status)),
		slog.String("provider_payment_id", result.ProviderPaymentID),
		slog.String("customer_key", result.CustomerKey),
		slog.String("request_key", result.RequestKey),
	)

	// Standalone card binding webhook: upsert the saved card and exit.
	if isAddCardNotificationType(result.NotificationType) {
		userID, err := uuid.Parse(result.CustomerKey)
		if err != nil {
			return fmt.Errorf("invalid customer key: %w", err)
		}

		pm, err := buildPaymentMethodFromWebhook(s.deps, userID, result)
		if err != nil {
			return err
		}

		tx, err := s.deps.beginner.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin transaction: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		txPaymentMethods, err := s.deps.paymentMethods.WithTx(tx)
		if err != nil {
			return fmt.Errorf("bind payment methods transaction: %w", err)
		}
		txSubscriptions, err := s.deps.subscriptions.WithTx(tx)
		if err != nil {
			return fmt.Errorf("bind subscriptions transaction: %w", err)
		}

		pm, err = txPaymentMethods.UpsertByTokenHash(ctx, pm)
		if err != nil {
			return fmt.Errorf("upsert add card payment method: %w", err)
		}

		// A successful AddCard webhook means the saved token is valid and should
		// become the active payment method for future renewals.
		if err = txPaymentMethods.SetActive(ctx, userID, pm.ID); err != nil {
			return fmt.Errorf("activate add card payment method: %w", err)
		}

		// Link the activated card to the subscription so renewals charge the
		// right method. A missing subscription is not fatal for AddCard itself:
		// failing here would make T-Kassa retry the webhook indefinitely.
		sub, err := txSubscriptions.GetByUserIDForUpdate(ctx, userID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				s.deps.log.WarnContext(ctx, "add card webhook: subscription not found; skipping active method link",
					slog.String("user_id", userID.String()),
					slog.String("payment_method_id", pm.ID.String()))
			} else {
				return fmt.Errorf("get subscription for add card: %w", err)
			}
		} else {
			sub.ActivePaymentMethodID = &pm.ID
			if err := txSubscriptions.Update(ctx, sub); err != nil {
				return fmt.Errorf("update subscription active payment method: %w", err)
			}
		}

		return tx.Commit(ctx)
	}

	tx, err := s.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txSubscriptionPayments, err := s.deps.subscriptionPayments.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind subscription payments transaction: %w", err)
	}
	txSubscriptions, err := s.deps.subscriptions.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind subscriptions transaction: %w", err)
	}

	payment, err := txSubscriptionPayments.GetByIDForUpdate(ctx, result.InternalPaymentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrPaymentNotFound
		}
		return fmt.Errorf("get payment: %w", err)
	}

	if payment.Status == domain.PaymentStatusSucceeded || payment.Status == domain.PaymentStatusFailed {
		// Refund webhooks may arrive after a payment has already succeeded.
		// In that case we must process the refund instead of ignoring it.
		if result.Status != domain.PaymentStatusRefunded && result.Status != domain.PaymentStatusPartialRefunded {
			if payment.Status == domain.PaymentStatusSucceeded {
				applySubscriptionRenewalAndArchive(ctx, s.deps, payment)
				return nil
			}
			// payment.Status == domain.PaymentStatusFailed && result.Status == domain.PaymentStatusSucceeded.
			// Out-of-order webhook: the provider may have captured the money after
			// we already marked the payment as failed. Reconcile from the provider
			// status before applying any subscription-side effects.
			if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID == "" {
				s.deps.log.WarnContext(ctx, "cannot reconcile failed payment: missing provider payment id",
					slog.String("payment_id", payment.ID.String()),
					slog.String("subscription_id", payment.SubscriptionID.String()))
				return nil
			}
			// Release the row lock before the outbound HTTP call.
			_ = tx.Rollback(ctx)
			return reconcileFailedPayment(ctx, s.deps, payment, result)
		}
	}

	switch result.Status {
	case domain.PaymentStatusPending:
		// AUTHORIZED: credentials received, payment stays pending until CONFIRMED.
		pm, err := upsertPaymentMethodFromWebhook(ctx, s.deps, tx, payment.UserID, result)
		if err != nil {
			return err
		}

		if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID == "" {
			if _, updateErr := txSubscriptionPayments.UpdateProviderPaymentID(ctx, payment.ID, result.ProviderPaymentID); updateErr != nil {
				return fmt.Errorf("update provider payment id: %w", updateErr)
			}
		}

		if payment.PaymentMethodID == nil {
			if _, updateErr := txSubscriptionPayments.UpdatePaymentMethodAndProviderID(ctx, payment.ID, pm.ID, result.ProviderPaymentID); updateErr != nil {
				return fmt.Errorf("update payment method id: %w", updateErr)
			}
		}

		return tx.Commit(ctx)

	case domain.PaymentStatusSucceeded, domain.PaymentStatusFailed, domain.PaymentStatusRefunded, domain.PaymentStatusPartialRefunded:
		if err := applyPaymentResult(ctx, s.deps, tx, &payment, result); err != nil {
			return err
		}

		switch result.Status {
		case domain.PaymentStatusFailed:
			sub, err := txSubscriptions.GetByIDForUpdate(ctx, payment.SubscriptionID)
			if err != nil {
				return fmt.Errorf("get subscription for failed webhook: %w", err)
			}
			if sub.TariffID == payment.TariffID {
				transitionToGrace(&sub, s.deps.clock.Now().UTC())
				if err := txSubscriptions.Update(ctx, sub); err != nil {
					return fmt.Errorf("transition subscription to grace after failed webhook: %w", err)
				}
			}

		case domain.PaymentStatusRefunded, domain.PaymentStatusPartialRefunded:
			if err := applyRefundToSubscription(ctx, s.deps, tx, payment.SubscriptionID); err != nil {
				return err
			}
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit webhook transaction: %w", err)
		}

		if result.Status == domain.PaymentStatusSucceeded {
			applySubscriptionRenewalAndArchive(ctx, s.deps, payment)
		}
		return nil

	default:
		return fmt.Errorf("unsupported webhook status: %s", result.Status)
	}
}

// reconcileFailedPayment queries the provider for the authoritative status of a
// failed payment and applies the corresponding subscription-side effects. It is
// used when a late "succeeded" webhook arrives after the payment has already
// been marked as failed.
func reconcileFailedPayment(ctx context.Context, d flowDeps, payment domain.SubscriptionPayment, result WebhookPayload) error {
	status, err := d.provider.Status(ctx, payment.ID, *payment.ProviderPaymentID)
	if err != nil {
		d.log.ErrorContext(ctx, "failed to query provider status for reconciling failed payment",
			slog.String("payment_id", payment.ID.String()),
			slog.String("subscription_id", payment.SubscriptionID.String()),
			slog.String("error", sanitize.Error(err)))
		return nil
	}

	switch status {
	case domain.PaymentStatusSucceeded, domain.PaymentStatusRefunded, domain.PaymentStatusPartialRefunded:
		// fall through to apply the reconciliation.
	case domain.PaymentStatusFailed, domain.PaymentStatusPending:
		return nil
	default:
		d.log.WarnContext(ctx, "unexpected provider status for reconciling failed payment; skipping",
			slog.String("payment_id", payment.ID.String()),
			slog.String("subscription_id", payment.SubscriptionID.String()),
			slog.String("provider_status", string(status)))
		return nil
	}

	tx, err := d.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin reconcile transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txSubscriptionPayments, err := d.subscriptionPayments.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind subscription payments transaction: %w", err)
	}

	payment, err = txSubscriptionPayments.GetByIDForUpdate(ctx, payment.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrPaymentNotFound
		}
		return fmt.Errorf("get payment for reconcile: %w", err)
	}
	if payment.Status != domain.PaymentStatusFailed {
		// A concurrent webhook or another reconciliation already finalized it.
		return nil
	}

	reconcileResult := result
	reconcileResult.Status = status

	if err := applyPaymentResult(ctx, d, tx, &payment, reconcileResult); err != nil {
		return err
	}

	switch status {
	case domain.PaymentStatusRefunded, domain.PaymentStatusPartialRefunded:
		if err := applyRefundToSubscription(ctx, d, tx, payment.SubscriptionID); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit reconcile transaction: %w", err)
	}

	if status == domain.PaymentStatusSucceeded {
		applySubscriptionRenewalAndArchive(ctx, d, payment)
	}
	return nil
}
