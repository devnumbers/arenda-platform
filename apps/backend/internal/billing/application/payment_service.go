package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// FakePaymentConfirmer completes a payment at the provider from the local dev
// endpoint (issue #250). Only the fake adapter implements it; the confirm flow
// type-asserts the active provider, so nothing fake-specific leaks into the
// port.
type FakePaymentConfirmer interface {
	ConfirmPayment(ctx context.Context, internalPaymentID string) (WebhookEvent, error)
}

// paymentFinalizerProvider is the narrow provider slice the payment service
// needs: parsing and acknowledging webhooks, querying the provider-side
// payment status, and the provider identity. Declared here, at the consumer,
// per ADR 0035.
type paymentFinalizerProvider interface {
	WebhookParser
	WebhookResponder
	PaymentStatusReader
	ProviderNamer
}

// PaymentService finalizes subscription payments: it processes provider
// webhooks synchronously (issue #250, ADR 0039),
// serves the local fake-payment confirmation endpoint and lists the user's
// payments. The payment itself is started by SubscriptionService.ChangeTariff;
// this service owns everything that happens after the provider reports an
// outcome.
type PaymentService struct {
	txStoreFactory
	provider paymentFinalizerProvider
	clock    clock.Clock
	log      *slog.Logger
}

// PaymentServiceConfig carries the non-transactional dependencies of the
// payment service.
type PaymentServiceConfig struct {
	Clock clock.Clock
	Log   *slog.Logger
}

// NewPaymentService creates a payment service over the shared factory.
func NewPaymentService(factory txStoreFactory, provider paymentFinalizerProvider, cfg PaymentServiceConfig) *PaymentService {
	if cfg.Clock == nil {
		cfg.Clock = clock.Real{}
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	return &PaymentService{
		txStoreFactory: factory,
		provider:       provider,
		clock:          cfg.Clock,
		log:            cfg.Log,
	}
}

// WebhookAck returns the fixed success body the provider expects as the
// acknowledgement of a processed webhook.
func (s *PaymentService) WebhookAck() []byte {
	return s.provider.WebhookAck()
}

// ListPayments returns the user's subscription payments with their tariffs
// resolved, newest first (GET /subscription/payments, issue #250).
func (s *PaymentService) ListPayments(ctx context.Context, userID uuid.UUID) ([]SubscriptionPaymentView, error) {
	payments, err := s.payments.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
	}
	tariffs, err := s.tariffs.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tariffs: %w", err)
	}
	tariffByID := make(map[uuid.UUID]domain.Tariff, len(tariffs))
	for _, t := range tariffs {
		tariffByID[t.ID] = t
	}

	views := make([]SubscriptionPaymentView, 0, len(payments))
	for _, p := range payments {
		tariff, ok := tariffByID[p.TariffID]
		if !ok {
			return nil, fmt.Errorf("payment %s references unknown tariff %s", p.ID, p.TariffID)
		}
		views = append(views, SubscriptionPaymentView{Payment: p, Tariff: tariff})
	}
	return views, nil
}

// HandleWebhook parses, verifies and synchronously applies one provider
// notification (issue #250). It returns nil only when the notification is
// fully processed — including the idempotent no-op of a repeated delivery;
// any error means the caller must answer non-200 so the provider retries.
func (s *PaymentService) HandleWebhook(ctx context.Context, providerName string, payload []byte) error {
	if domain.PaymentProvider(providerName) != s.provider.Name() {
		return fmt.Errorf("%w: got %q, active provider is %q", ErrWebhookProviderMismatch, providerName, s.provider.Name())
	}
	event, err := s.provider.ParseWebhook(ctx, payload)
	if err != nil {
		return fmt.Errorf("%w: parse webhook: %w", ErrWebhookRejected, err)
	}
	switch {
	case event.Payment != nil:
		return s.handlePaymentNotification(ctx, event.Payment)
	case event.MethodBound != nil:
		// The card-binding flow lands with issue #251; until then no binding
		// can be initiated, so such a notification cannot be legitimate.
		return fmt.Errorf("%w: method-bound notification", ErrWebhookUnsupported)
	default:
		return fmt.Errorf("%w: empty event", ErrWebhookUnsupported)
	}
}

// handlePaymentNotification applies one payment-status notification. Terminal
// statuses finalize the payment and apply its subscription effects in a single
// transaction; a pending status carries no outcome and is at most a chance to
// backfill a lost provider reference.
func (s *PaymentService) handlePaymentNotification(ctx context.Context, n *PaymentNotification) error {
	switch n.Status {
	case domain.PaymentStatusPending:
		return s.applyPendingNotification(ctx, n)
	case domain.PaymentStatusSucceeded, domain.PaymentStatusFailed:
		return s.applyFinalNotification(ctx, n)
	case domain.PaymentStatusRefunded:
		return s.applyRefundNotification(ctx, n)
	default:
		return fmt.Errorf("%w: status %q", ErrWebhookUnsupported, n.Status)
	}
}

// applyPendingNotification backfills the provider reference of a payment whose
// initiation result was lost — the webhook that arrives before the atomic save
// commits. There is no outcome to apply.
func (s *PaymentService) applyPendingNotification(ctx context.Context, n *PaymentNotification) error {
	return s.runInTx(ctx, func(stores *txStores) error {
		payment, err := stores.paymentForUpdate(ctx, n.InternalPaymentID)
		if err != nil {
			return err
		}
		if err := s.checkProviderPaymentID(payment, n); err != nil {
			return err
		}
		if payment.HasProviderReference() || payment.IsFinalized() || n.ProviderPaymentID == "" {
			return nil
		}
		if err := payment.SaveProviderReference(n.ProviderPaymentID, "", s.clock.Now().UTC()); err != nil {
			return err
		}
		if err := stores.payments.Update(ctx, payment); err != nil {
			return fmt.Errorf("backfill provider reference: %w", err)
		}
		return nil
	})
}

// applyFinalNotification finalizes the payment to the notification's terminal
// status (succeeded or failed) and applies the subscription effects of a
// success — atomically, so a mid-flight failure rolls everything back and the
// provider's retry redelivers the notification (ADR "synchronous payment
// webhooks"). Repeated deliveries of the same outcome are idempotent no-ops;
// a succeeded notification for a payment already marked failed is resolved
// against the provider as the source of truth (ADR 0010) before anything is
// applied.
func (s *PaymentService) applyFinalNotification(ctx context.Context, n *PaymentNotification) error {
	if n.Status == domain.PaymentStatusSucceeded {
		payment, err := s.payments.GetByID(ctx, n.InternalPaymentID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrPaymentNotFound
			}
			return fmt.Errorf("get payment: %w", err)
		}
		if payment.Status == domain.PaymentStatusFailed {
			return s.reconcileOutOfOrderSuccess(ctx, payment, n)
		}
	}
	return s.finalizePayment(ctx, n, false)
}

// reconcileOutOfOrderSuccess resolves a succeeded notification for a payment
// already marked failed by asking the provider for its current status — the
// source of truth (ADR 0010) — before any state is changed. The status query
// runs outside any transaction, so no row locks are held across the external
// call. A provider that still reports the payment failed or not yet settled
// makes the notification a no-op; only a provider-confirmed success is applied.
func (s *PaymentService) reconcileOutOfOrderSuccess(ctx context.Context, payment domain.SubscriptionPayment, n *PaymentNotification) error {
	if !payment.HasProviderReference() {
		// Without a provider reference there is nothing to reconcile against;
		// a success we cannot verify is not applied.
		s.log.WarnContext(ctx, "cannot reconcile failed payment: missing provider reference",
			slog.String("payment_id", payment.ID.String()))
		return nil
	}
	status, err := s.provider.PaymentStatus(ctx, payment.ID, *payment.ProviderPaymentID)
	if err != nil {
		return fmt.Errorf("provider status for out-of-order success: %w", err)
	}
	if status.Status != domain.PaymentStatusSucceeded {
		s.log.InfoContext(ctx, "out-of-order success notification contradicted by provider status; skipping",
			slog.String("payment_id", payment.ID.String()),
			slog.String("provider_status", string(status.Status)))
		return nil
	}
	return s.finalizePayment(ctx, n, true)
}

// finalizePayment runs the finalizing transaction: it locks the payment,
// verifies the notification against the persisted provider reference, moves
// the payment to the notification's status and — for a success — applies the
// tariff or renewal to the subscription with its transition-log entry and
// audit record. allowReconcile marks callers that verified an out-of-order
// success against the provider; without it a failed payment is never
// overwritten by this path.
func (s *PaymentService) finalizePayment(ctx context.Context, n *PaymentNotification, allowReconcile bool) error {
	return s.runInTx(ctx, func(stores *txStores) error {
		payment, err := stores.paymentForUpdate(ctx, n.InternalPaymentID)
		if err != nil {
			return err
		}
		if err := s.checkProviderPaymentID(payment, n); err != nil {
			return err
		}
		if !payment.HasProviderReference() && n.ProviderPaymentID != "" && payment.Status == domain.PaymentStatusPending {
			if err := payment.SaveProviderReference(n.ProviderPaymentID, "", s.clock.Now().UTC()); err != nil {
				return err
			}
			if err := stores.payments.Update(ctx, payment); err != nil {
				return fmt.Errorf("persist provider reference: %w", err)
			}
		}

		now := s.clock.Now().UTC()
		switch n.Status {
		case domain.PaymentStatusFailed:
			if payment.Status != domain.PaymentStatusPending {
				// Duplicate delivery, or a terminal state that must not be
				// overridden by a late failure.
				return nil
			}
			if err := payment.MarkFailed(n.ErrorCode, now); err != nil {
				return err
			}
			if err := stores.payments.Update(ctx, payment); err != nil {
				return fmt.Errorf("mark payment failed: %w", err)
			}
			if err := stores.audit.Record(ctx, auditdomain.Entry{
				ActorRole:  auditdomain.ActorRoleSystem,
				Action:     auditdomain.ActionSubscriptionPaymentFailed,
				EntityType: auditdomain.EntitySubscriptionPayment,
				EntityID:   &payment.ID,
				Context:    map[string]any{"payment_id": payment.ID, "provider": string(s.provider.Name())},
			}); err != nil {
				return fmt.Errorf("record audit: %w", err)
			}
			return nil

		case domain.PaymentStatusSucceeded:
			switch payment.Status {
			case domain.PaymentStatusSucceeded:
				return nil // duplicate delivery
			case domain.PaymentStatusFailed:
				if !allowReconcile {
					// The payment became failed after the caller's check;
					// refuse so the provider retries and the verified
					// reconciliation path runs instead.
					return fmt.Errorf("payment %s turned failed during webhook processing", payment.ID)
				}
				if err := payment.ReconcileToSucceeded(now); err != nil {
					return err
				}
			case domain.PaymentStatusPending:
				if err := payment.MarkSucceeded(now); err != nil {
					return err
				}
			default:
				// refunded (and the internal refunding reservation): a refund
				// is a later, deliberate state that a payment notification
				// does not override.
				return nil
			}
			if err := stores.payments.Update(ctx, payment); err != nil {
				return fmt.Errorf("mark payment succeeded: %w", err)
			}
			if err := s.applySucceededPayment(ctx, stores, payment, now); err != nil {
				return err
			}
			if err := stores.audit.Record(ctx, auditdomain.Entry{
				ActorRole:  auditdomain.ActorRoleSystem,
				Action:     auditdomain.ActionSubscriptionPaymentSucceeded,
				EntityType: auditdomain.EntitySubscriptionPayment,
				EntityID:   &payment.ID,
				Context:    map[string]any{"payment_id": payment.ID, "provider": string(s.provider.Name()), "amount_kopecks": payment.AmountKopecks},
			}); err != nil {
				return fmt.Errorf("record audit: %w", err)
			}
			return nil

		default:
			return fmt.Errorf("%w: status %q", ErrWebhookUnsupported, n.Status)
		}
	})
}

// applySucceededPayment applies the subscription effects of a succeeded
// payment inside the finalizing transaction: an upgrade switches the tariff at
// the full price of the new plan with the period counted from the payment
// moment and auto-renew on; a same-tariff payment is a renewal. The transition
// log records the applied change with the payment that caused it. A payment
// the subscription already reflects is a no-op, so duplicate deliveries and
// reconciliations stay idempotent.
func (s *PaymentService) applySucceededPayment(ctx context.Context, stores *txStores, payment domain.SubscriptionPayment, now time.Time) error {
	sub, err := stores.subscriptionForUpdate(ctx, payment.UserID)
	if err != nil {
		return err
	}
	if sub.LastAppliedPaymentID != nil && *sub.LastAppliedPaymentID == payment.ID {
		return nil
	}

	paymentTariff, err := stores.tariffs.GetByID(ctx, payment.TariffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return fmt.Errorf("payment %s references missing tariff %s: %w", payment.ID, payment.TariffID, ErrTariffNotFound)
		}
		return fmt.Errorf("get payment tariff: %w", err)
	}

	fromStatus := sub.Status
	fromTariffID := sub.TariffID
	if sub.TariffID == payment.TariffID {
		if err := sub.ApplyRenewal(payment.ID, payment.Period, now); err != nil {
			return err
		}
	} else {
		currentTariff, err := stores.tariffs.GetByID(ctx, sub.TariffID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return fmt.Errorf("subscription %s references missing tariff %s: %w", sub.ID, sub.TariffID, ErrTariffNotFound)
			}
			return fmt.Errorf("get current tariff: %w", err)
		}
		if err := sub.ApplyTariffChange(payment.ID, currentTariff, paymentTariff, payment.Period, now); err != nil {
			return err
		}
	}
	if err := stores.subscriptions.Update(ctx, sub); err != nil {
		return fmt.Errorf("apply payment to subscription: %w", err)
	}

	transition, err := domain.NewAppliedPaymentTransition(sub, &fromStatus, &fromTariffID, payment.ID)
	if err != nil {
		return fmt.Errorf("build payment transition: %w", err)
	}
	if err := stores.transitions.Append(ctx, transition); err != nil {
		return fmt.Errorf("append payment transition: %w", err)
	}
	return nil
}

// applyRefundNotification records a full refund reported by the provider.
// Refunds are always full-amount (ADR 0037). The subscription-side effects of
// a refund (downgrade to basic, archiving) belong to the admin refund flow
// (issue #254); here the payment state is kept truthful while that flow is
// pending.
func (s *PaymentService) applyRefundNotification(ctx context.Context, n *PaymentNotification) error {
	return s.runInTx(ctx, func(stores *txStores) error {
		payment, err := stores.paymentForUpdate(ctx, n.InternalPaymentID)
		if err != nil {
			return err
		}
		if err := s.checkProviderPaymentID(payment, n); err != nil {
			return err
		}
		switch payment.Status {
		case domain.PaymentStatusRefunded:
			return nil // duplicate delivery
		case domain.PaymentStatusPending, domain.PaymentStatusSucceeded, domain.PaymentStatusRefunding:
		default:
			return nil // a failed charge was never captured; nothing to refund
		}
		if err := payment.MarkRefunded(s.clock.Now().UTC()); err != nil {
			return err
		}
		if err := stores.payments.Update(ctx, payment); err != nil {
			return fmt.Errorf("mark payment refunded: %w", err)
		}
		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorRole:  auditdomain.ActorRoleSystem,
			Action:     auditdomain.ActionSubscriptionPaymentRefunded,
			EntityType: auditdomain.EntitySubscriptionPayment,
			EntityID:   &payment.ID,
			Context:    map[string]any{"payment_id": payment.ID, "provider": string(s.provider.Name())},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
}

// checkProviderPaymentID rejects a notification whose provider payment id
// contradicts the reference persisted at initiation: the notification does not
// belong to this payment, and retrying it will not fix that.
func (s *PaymentService) checkProviderPaymentID(payment domain.SubscriptionPayment, n *PaymentNotification) error {
	if !payment.HasProviderReference() || n.ProviderPaymentID == "" {
		return nil
	}
	if *payment.ProviderPaymentID != n.ProviderPaymentID {
		return fmt.Errorf("%w: payment %s references %q, notification carries %q",
			ErrWebhookPaymentMismatch, payment.ID, *payment.ProviderPaymentID, n.ProviderPaymentID)
	}
	return nil
}

// ConfirmFakePayment completes a pending payment through the fake provider's
// local confirmation hook (POST /internal/fake-subscription-payment/{id}/confirm,
// APP_ENV=local only). The confirmed event flows through the same synchronous
// application path as a webhook, so local end-to-end runs exercise production
// behaviour. Confirming a finalized payment is an idempotent no-op.
func (s *PaymentService) ConfirmFakePayment(ctx context.Context, paymentID uuid.UUID) error {
	confirmer, ok := s.provider.(FakePaymentConfirmer)
	if !ok {
		return ErrPaymentNotConfirmable
	}
	payment, err := s.payments.GetByID(ctx, paymentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrPaymentNotFound
		}
		return fmt.Errorf("get payment: %w", err)
	}
	if payment.IsFinalized() {
		return nil
	}

	event, err := confirmer.ConfirmPayment(ctx, paymentID.String())
	if err != nil {
		if errors.Is(err, ErrProviderPaymentNotFound) && payment.HasProviderReference() {
			// The provider no longer tracks the payment (it purged a
			// long-pending entry or already settled it): the provider status
			// is the source of truth, so finalize from it.
			return s.confirmFromProviderStatus(ctx, payment)
		}
		return fmt.Errorf("confirm fake payment: %w", err)
	}
	if event.Payment == nil {
		return fmt.Errorf("%w: confirm returned no payment event", ErrWebhookUnsupported)
	}
	return s.handlePaymentNotification(ctx, event.Payment)
}

// confirmFromProviderStatus finalizes a payment from the provider's current
// status after its local confirmation could not resolve the entry.
func (s *PaymentService) confirmFromProviderStatus(ctx context.Context, payment domain.SubscriptionPayment) error {
	status, err := s.provider.PaymentStatus(ctx, payment.ID, *payment.ProviderPaymentID)
	if err != nil {
		return fmt.Errorf("provider status after lost confirm: %w", err)
	}
	if status.Status != domain.PaymentStatusSucceeded && status.Status != domain.PaymentStatusFailed {
		return nil
	}
	var errorCode *string
	if status.Status == domain.PaymentStatusFailed && status.ErrorCode != "" {
		errorCode = &status.ErrorCode
	}
	return s.handlePaymentNotification(ctx, &PaymentNotification{
		InternalPaymentID: payment.ID,
		ProviderPaymentID: *payment.ProviderPaymentID,
		Status:            status.Status,
		ErrorCode:         errorCode,
		AmountKopecks:     payment.AmountKopecks,
	})
}
