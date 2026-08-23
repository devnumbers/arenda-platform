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

// paymentFinalizerProvider is the narrow provider slice the payment service
// needs: parsing and acknowledging webhooks, querying the provider-side
// payment status, refunding a payment, and the provider identity. Declared
// here, at the consumer, per ADR 0035.
type paymentFinalizerProvider interface {
	WebhookParser
	WebhookResponder
	PaymentStatusReader
	PaymentRefunder
	ProviderNamer
}

// PaymentService finalizes subscription payments: it processes provider
// webhooks synchronously (issue #250, ADR 0039), applies parsed provider
// events of every delivery channel, lists the user's payments, and runs the
// admin payment operations of issue #254 — the three-phase refund saga, the
// manual provider sync and the admin listing. The payment itself is started
// by SubscriptionService.ChangeTariff; this service owns everything that
// happens after the provider reports an outcome.
type PaymentService struct {
	txStoreFactory
	provider paymentFinalizerProvider
	clock    clock.Clock
	config   Config
	log      *slog.Logger
	// Publisher emits the grace-entered event best-effort when an
	// asynchronously failed renewal charge moves the subscription into grace
	// (issue #253); nil keeps the pre-#253 behaviour.
	publisher EventPublisher
	// AdminPayments reads the cross-user payment rows behind the admin views
	// (issue #254); nil keeps the admin read methods answered by an explicit
	// wiring error.
	adminPayments AdminPaymentListing
	// ArchiverSource and slotSource bridge the refund's subscription
	// downgrade to the properties and access contexts (issue #254); nil until
	// SetLifecycleBridges wires them (the payment-side refund still applies
	// without the bridges, only the excess archiving waits).
	archiverSource ExcessPropertyArchiverSource
	slotSource     RecipientSlotEnforcerSource
}

// PaymentServiceConfig carries the non-transactional dependencies of the
// payment service.
type PaymentServiceConfig struct {
	Clock clock.Clock
	Log   *slog.Logger
	// Config carries the operational parameters; the grace duration backs the
	// grace entry of an asynchronously failed renewal charge (issue #252).
	Config Config
	// Publisher emits the grace lifecycle events (issue #253); nil keeps the
	// pre-#253 behaviour.
	Publisher EventPublisher
	// AdminPayments reads the cross-user payment rows behind the admin views
	// (issue #254); nil keeps the admin read methods answered by an explicit
	// wiring error.
	AdminPayments AdminPaymentListing
}

// NewPaymentService creates a payment service over the shared factory.
func NewPaymentService(factory txStoreFactory, provider paymentFinalizerProvider, cfg PaymentServiceConfig) *PaymentService {
	if cfg.Clock == nil {
		cfg.Clock = clock.Real{}
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.Config == (Config{}) {
		cfg.Config = DefaultConfig()
	}
	return &PaymentService{
		txStoreFactory: factory,
		provider:       provider,
		clock:          cfg.Clock,
		config:         cfg.Config,
		log:            cfg.Log,
		publisher:      cfg.Publisher,
		adminPayments:  cfg.AdminPayments,
	}
}

// SetLifecycleBridges wires the cross-context lifecycle bridges the refund
// finalization calls in its transaction: excess-property archiving
// (properties context) and recipient-slot enforcement (access context). The
// composition root calls it with the same bridge sources the workers use —
// billing is built before them. Without bridges the refund still marks the
// payment and downgrades the subscription; only the excess properties and
// suspended memberships wait for the next wired run.
func (s *PaymentService) SetLifecycleBridges(archiver ExcessPropertyArchiverSource, slots RecipientSlotEnforcerSource) {
	s.archiverSource = archiver
	s.slotSource = slots
}

// WebhookAck returns the fixed success body the provider expects as the
// acknowledgement of a processed webhook.
func (s *PaymentService) WebhookAck() []byte {
	return s.provider.WebhookAck()
}

// GetPayment returns one subscription payment by its id — the read behind the
// flows that address a single payment directly (the dev-only local provider
// confirmation, issue #287). A missing payment answers ErrPaymentNotFound.
func (s *PaymentService) GetPayment(ctx context.Context, paymentID uuid.UUID) (domain.SubscriptionPayment, error) {
	payment, err := s.payments.GetByID(ctx, paymentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.SubscriptionPayment{}, ErrPaymentNotFound
		}
		return domain.SubscriptionPayment{}, fmt.Errorf("get payment: %w", err)
	}
	return payment, nil
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
	return s.ApplyProviderEvent(ctx, event)
}

// ApplyProviderEvent applies one already-parsed provider notification through
// the synchronous paths of ADR 0039: a payment notification finalizes the
// payment with its subscription effects, a method-bound notification completes
// a card-binding session. HandleWebhook lands here after parsing; the
// dev-only local confirmation at the adapter/wiring level (issue #287) lands
// here after the active provider's adapter reports the completed entry — the
// application stays provider-neutral and knows nothing about which provider
// produced the event.
func (s *PaymentService) ApplyProviderEvent(ctx context.Context, event WebhookEvent) error {
	switch {
	case event.Payment != nil:
		return s.ApplyPaymentNotification(ctx, event.Payment)
	case event.MethodBound != nil:
		return s.applyMethodBoundNotification(ctx, event.MethodBound)
	case event.MethodBindingFailed != nil:
		return s.applyMethodBindingFailedNotification(ctx, event.MethodBindingFailed)
	default:
		return fmt.Errorf("%w: empty event", ErrWebhookUnsupported)
	}
}

// applyMethodBoundNotification applies a completed payment-method binding
// synchronously (issue #251, ADR 0039): the notification resolves to the
// session started by AddPaymentMethod by its request key, and one transaction
// creates (or converges on) the payment method, activates it, links it to the
// subscription and closes the session. The whole delivery is idempotent: a
// repeated notification resolves to a no-op. A session past its TTL never
// creates a card; an unknown request key cannot be fixed by a retry and is
// answered as processed so the provider stops redelivering.
func (s *PaymentService) applyMethodBoundNotification(ctx context.Context, n *MethodBoundNotification) error {
	now := s.clock.Now().UTC()
	return s.runInTx(ctx, func(stores *txStores) error {
		session, err := stores.bindings.GetByRequestKeyForUpdate(ctx, s.provider.Name(), n.BindingID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				s.log.WarnContext(ctx, "method-bound webhook for unknown binding session; ignoring",
					slog.String("binding_id", n.BindingID),
					slog.String(auditKeyProvider, string(s.provider.Name())))
				return nil
			}
			return fmt.Errorf("get card binding session: %w", err)
		}
		if !session.IsOpen() {
			// Duplicate delivery of an already-resolved session.
			return nil
		}
		if session.IsExpired(now) {
			// The binding's lifetime is over: no card may be created from it,
			// whatever the provider reports now.
			if err := session.MarkRejected(now); err != nil {
				return err
			}
			if err := stores.bindings.UpdateStatus(ctx, session); err != nil {
				return fmt.Errorf("reject expired card binding session: %w", err)
			}
			s.log.InfoContext(ctx, "expired card binding session rejected by webhook",
				slog.String("binding_id", n.BindingID),
				slog.String("user_id", session.UserID.String()))
			return nil
		}
		if n.Method.ChargeToken == "" {
			// A completed binding without a charge token is a provider defect
			// no retry can fix; the session stays open and expires by TTL.
			return fmt.Errorf("%w: binding %s completed without a charge token", ErrWebhookRejected, n.BindingID)
		}
		method, err := domain.NewPaymentMethod(session.UserID, session.Provider, n.Method.ChargeToken, now)
		if err != nil {
			return err
		}
		method.ProviderCardID = n.Method.ProviderMethodID
		method.DisplayMask = n.Method.MaskedPan
		method.ExpDate = n.Method.ExpDate
		_, err = applyCompletedCardBinding(ctx, stores, s.log, method, &session, now)
		return err
	})
}

// applyMethodBindingFailedNotification applies a refused payment-method
// binding synchronously (issue #422, ADR 0039): the delivery itself is valid,
// so it is a processed outcome — nil error, 200 to the provider — that closes
// the session without a payment method. The user sees the failure on the next
// sync and can start a new binding. Idempotent by the session state: a
// repeated delivery resolves to a no-op, and an unknown request key cannot be
// fixed by a retry, so it is answered as processed too.
func (s *PaymentService) applyMethodBindingFailedNotification(ctx context.Context, n *MethodBindingFailedNotification) error {
	now := s.clock.Now().UTC()
	return s.runInTx(ctx, func(stores *txStores) error {
		session, err := stores.bindings.GetByRequestKeyForUpdate(ctx, s.provider.Name(), n.BindingID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				s.log.WarnContext(ctx, "method-binding-failed webhook for unknown binding session; ignoring",
					slog.String("binding_id", n.BindingID),
					slog.String(auditKeyProvider, string(s.provider.Name())))
				return nil
			}
			return fmt.Errorf("get card binding session: %w", err)
		}
		if !session.IsOpen() {
			// Duplicate delivery of an already-resolved session.
			return nil
		}
		if err := session.MarkRejected(now); err != nil {
			return err
		}
		if err := stores.bindings.UpdateStatus(ctx, session); err != nil {
			return fmt.Errorf("reject card binding session: %w", err)
		}
		s.log.InfoContext(ctx, "card binding session rejected by webhook",
			slog.String("binding_id", n.BindingID),
			slog.String("user_id", session.UserID.String()),
			slog.String("error_code", n.ErrorCode))
		return nil
	})
}

// ApplyPaymentNotification applies one payment-status notification. Terminal
// statuses finalize the payment and apply its subscription effects in a single
// transaction; a pending status carries no outcome and is at most a chance to
// backfill a lost provider reference. This is the notification-application
// method of the workers' PaymentLifecycle port; the webhook flow reaches it
// through ApplyProviderEvent after parsing.
func (s *PaymentService) ApplyPaymentNotification(ctx context.Context, n *PaymentNotification) error {
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
			slog.String(auditKeyPaymentID, payment.ID.String()))
		return nil
	}
	status, err := s.provider.PaymentStatus(ctx, payment.ID, *payment.ProviderPaymentID)
	if err != nil {
		return fmt.Errorf("provider status for out-of-order success: %w", err)
	}
	if status.Status != domain.PaymentStatusSucceeded {
		s.log.InfoContext(ctx, "out-of-order success notification contradicted by provider status; skipping",
			slog.String(auditKeyPaymentID, payment.ID.String()),
			slog.String("provider_status", string(status.Status)))
		return nil
	}
	return s.finalizePayment(ctx, n, true)
}

// finalizePayment runs the finalizing transaction: it locks the payment,
// verifies the notification against the persisted provider reference, captures
// a missing reference, and moves the payment to the notification's status via
// the per-status handlers below. AllowReconcile marks callers that verified an
// out-of-order success against the provider; without it a failed payment is
// never overwritten by this path. Grace events captured along the way are
// published strictly after the commit — best-effort (issue #284).
func (s *PaymentService) finalizePayment(ctx context.Context, n *PaymentNotification, allowReconcile bool) error {
	grace := newGraceEvents(s.publisher, s.log)
	return grace.run(ctx, s.runInTx, func(stores *txStores) error {
		payment, err := stores.paymentForUpdate(ctx, n.InternalPaymentID)
		if err != nil {
			return err
		}
		if err := s.checkProviderPaymentID(payment, n); err != nil {
			return err
		}
		if err := s.captureProviderReference(ctx, stores, &payment, n); err != nil {
			return err
		}

		now := s.clock.Now().UTC()
		switch n.Status {
		case domain.PaymentStatusFailed:
			return s.finalizeFailedPayment(ctx, grace, stores, payment, n, now)
		case domain.PaymentStatusSucceeded:
			return s.finalizeSucceededPayment(ctx, stores, payment, now, allowReconcile)
		default:
			return fmt.Errorf("%w: status %q", ErrWebhookUnsupported, n.Status)
		}
	})
}

// captureProviderReference persists the provider payment id carried by the
// notification when the payment is still pending and has no reference yet —
// the reference of the initiation response arrives with the first webhook.
func (s *PaymentService) captureProviderReference(
	ctx context.Context, stores *txStores, payment *domain.SubscriptionPayment, n *PaymentNotification,
) error {
	if payment.HasProviderReference() || n.ProviderPaymentID == "" || payment.Status != domain.PaymentStatusPending {
		return nil
	}
	if err := payment.SaveProviderReference(n.ProviderPaymentID, "", s.clock.Now().UTC()); err != nil {
		return err
	}
	if err := stores.payments.Update(ctx, *payment); err != nil {
		return fmt.Errorf("persist provider reference: %w", err)
	}
	return nil
}

// finalizeFailedPayment marks a declined payment failed and — for a
// merchant-initiated renewal charge, which carries the charged method — moves
// the subscription into grace: the user gets the window to fix the payment
// method, and the next worker tick does not simply charge a fresh payment
// forever (ADR 0008). The freshness guard of issue #426 keeps a superseded
// charge out of grace: only a failure the subscription's current paid period
// still depends on enters it, so a subscription renewed or upgraded by a
// newer payment stays active. A customer-initiated payment has no method —
// its failure leaves the subscription untouched. The grace-entered event is
// captured by the grace-events module and published strictly after the commit
// (issue #284).
func (s *PaymentService) finalizeFailedPayment(
	ctx context.Context, grace *graceEvents, stores *txStores,
	payment domain.SubscriptionPayment, n *PaymentNotification, now time.Time,
) error {
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
	if payment.PaymentMethodID != nil {
		sub, err := stores.subscriptionForUpdate(ctx, payment.UserID)
		if err != nil {
			return err
		}
		if sub.FailedRenewalIsCurrent(payment.ID, now) {
			if err := grace.enterGrace(ctx, stores, sub, now, s.config.GraceDuration); err != nil {
				return err
			}
		}
	}
	if err := stores.audit.Record(ctx, auditdomain.Entry{
		ActorRole:  auditdomain.ActorRoleSystem,
		Action:     auditdomain.ActionSubscriptionPaymentFailed,
		EntityType: auditdomain.EntitySubscriptionPayment,
		EntityID:   &payment.ID,
		Context:    map[string]any{auditKeyPaymentID: payment.ID, auditKeyProvider: string(s.provider.Name())},
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}
	return nil
}

// finalizeSucceededPayment moves the payment to succeeded and applies the
// subscription effects of the success: the tariff change or renewal with its
// transition-log entry and audit record. The persisted-status switch keeps
// duplicate deliveries idempotent and — without allowReconcile — refuses a
// success that arrives after the payment was already marked failed, so the
// provider retries and the verified reconciliation path runs instead.
func (s *PaymentService) finalizeSucceededPayment(
	ctx context.Context, stores *txStores, payment domain.SubscriptionPayment, now time.Time, allowReconcile bool,
) error {
	switch payment.Status {
	case domain.PaymentStatusSucceeded:
		return nil // Duplicate delivery.
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
		// A refund (and the internal refunding reservation) is a
		// later, deliberate state that a payment notification does
		// not override.
		return nil
	}
	if err := stores.payments.Update(ctx, payment); err != nil {
		return fmt.Errorf("mark payment succeeded: %w", err)
	}
	if _, err := applySucceededPayment(ctx, stores, payment, now); err != nil {
		return err
	}
	if err := stores.audit.Record(ctx, auditdomain.Entry{
		ActorRole:  auditdomain.ActorRoleSystem,
		Action:     auditdomain.ActionSubscriptionPaymentSucceeded,
		EntityType: auditdomain.EntitySubscriptionPayment,
		EntityID:   &payment.ID,
		Context: map[string]any{
			auditKeyPaymentID: payment.ID, auditKeyProvider: string(s.provider.Name()),
			auditKeyAmountKopecks: payment.AmountKopecks,
		},
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}
	return nil
}

// applySucceededPayment applies the subscription effects of a succeeded
// payment inside the finalizing transaction: an upgrade switches the tariff at
// the full price of the new plan with the period counted from the payment
// moment and auto-renew on; a same-tariff payment is a renewal. The transition
// log records the applied change with the payment that caused it. A payment
// the subscription already reflects is a no-op — the zero transition it
// returns keeps duplicate deliveries, reconciliations and worker retries
// idempotent. Shared by the webhook flow and the renewal worker (issue #252).
func applySucceededPayment(
	ctx context.Context, stores *txStores, payment domain.SubscriptionPayment, now time.Time,
) (domain.Transition, error) {
	sub, err := stores.subscriptionForUpdate(ctx, payment.UserID)
	if err != nil {
		return domain.Transition{}, err
	}
	if sub.LastAppliedPaymentID != nil && *sub.LastAppliedPaymentID == payment.ID {
		return domain.Transition{}, nil
	}

	paymentTariff, err := stores.tariffs.GetByID(ctx, payment.TariffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Transition{}, fmt.Errorf("payment %s references missing tariff %s: %w", payment.ID, payment.TariffID, ErrTariffNotFound)
		}
		return domain.Transition{}, fmt.Errorf("get payment tariff: %w", err)
	}

	return stores.applyTransition(ctx, &sub,
		func(s *domain.Subscription) error {
			if s.TariffID == payment.TariffID {
				return s.ApplyRenewal(payment.ID, payment.Period, now)
			}
			currentTariff, err := stores.tariffs.GetByID(ctx, s.TariffID)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					return fmt.Errorf("subscription %s references missing tariff %s: %w", s.ID, s.TariffID, ErrTariffNotFound)
				}
				return fmt.Errorf("get current tariff: %w", err)
			}
			return s.ApplyTariffChange(payment.ID, currentTariff, paymentTariff, payment.Period, now)
		},
		transitionSpec{
			reason:    domain.TransitionReasonPaymentApplied,
			initiator: domain.InitiatorSystem,
			paymentID: new(payment.ID),
		},
	)
}

// applyRefundNotification records a full refund reported by the provider and
// applies the subscription effects of the refund (downgrade to basic with the
// excess properties archived, issue #254) atomically. Refunds are always
// full-amount (ADR 0037). The system is the initiator — a refund notification
// reports a provider-side outcome, not an admin action; the admin-triggered
// refund that landed first is visible in the transition log by its own entry.
func (s *PaymentService) applyRefundNotification(ctx context.Context, n *PaymentNotification) error {
	return s.runRefundTx(ctx, func(stores *txStores) error {
		payment, err := stores.paymentForUpdate(ctx, n.InternalPaymentID)
		if err != nil {
			return err
		}
		if err := s.checkProviderPaymentID(payment, n); err != nil {
			return err
		}
		switch payment.Status {
		case domain.PaymentStatusRefunded:
			return nil // Duplicate delivery; the effects were applied with it.
		case domain.PaymentStatusPending, domain.PaymentStatusSucceeded, domain.PaymentStatusRefunding:
		default:
			return nil // A failed charge was never captured; nothing to refund.
		}
		if err := s.applyRefundedPayment(ctx, stores, payment, s.clock.Now().UTC(), systemRefundActor()); err != nil {
			return err
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
