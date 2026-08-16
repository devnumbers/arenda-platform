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
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// The billing worker phases (issue #252, ADR 0008 lifecycle). The scheduler
// shells (platform/scheduler) call these under their advisory locks and stay
// thin: every decision lives here, behind the module's stores and provider
// port.

// ExcessPropertyArchiver archives the owner's active properties beyond a tariff
// limit — force-completing the open leases on them — inside the caller's
// transaction. Declared here, at the consumer (the worker phases), per
// ADR 0035; the composition root adapts the properties context to it.
type ExcessPropertyArchiver interface {
	ArchiveExcess(ctx context.Context, ownerID uuid.UUID, limit int) error
}

// ExcessPropertyArchiverSource produces transaction-bound archivers.
type ExcessPropertyArchiverSource interface {
	WithTx(tx transaction.Tx) (ExcessPropertyArchiver, error)
}

// RecipientSlotEnforcer suspends the shared memberships whose recipient's
// tariff limit is exceeded after a billing limit drop. Declared here, at the
// consumer, per ADR 0035.
type RecipientSlotEnforcer interface {
	Enforce(ctx context.Context, userID uuid.UUID, trigger string) error
}

// RecipientSlotEnforcerSource produces transaction-bound enforcers.
type RecipientSlotEnforcerSource interface {
	WithTx(tx transaction.Tx) (RecipientSlotEnforcer, error)
}

// renewalProvider is the narrow provider slice the renewal worker needs:
// starting merchant-initiated payments, charging saved methods, reading the
// provider-side status, and the provider identity. Declared here, at the
// consumer, per ADR 0035.
type renewalProvider interface {
	PaymentInitiator
	PaymentCharger
	PaymentStatusReader
	ProviderNamer
}

// Workers drives the subscription lifecycle phases of the billing background
// workers: deferred tariff changes, renewal charges, lost-webhook
// reconciliation and the expiry downgrades to basic (issue #252). Every phase
// batches over the worker listings, re-reads and locks each row inside its own
// transaction, and stops a batch loop that makes no progress — the next tick
// retries what failed.
type Workers struct {
	txStoreFactory
	// payments finalizes reconciled payments through the same synchronous
	// notification application the webhook flow uses.
	payments *PaymentService
	provider renewalProvider
	clock    clock.Clock
	config   Config
	log      *slog.Logger
	// publisher emits the grace lifecycle events best-effort (issue #253);
	// nil keeps the pre-#253 behaviour of no grace notifications.
	publisher EventPublisher
	// archiverSource and slotSource bridge the expiry and downgrade phases to
	// the properties and access contexts; nil until SetLifecycleBridges wires
	// them (the subscription-side phases still run without the bridges).
	archiverSource ExcessPropertyArchiverSource
	slotSource     RecipientSlotEnforcerSource
}

// WorkersConfig carries the non-transactional dependencies of the workers.
type WorkersConfig struct {
	// Provider charges renewals and reads provider-side payment status. Nil
	// keeps the pre-#252 behaviour: the charge-driven phases answer
	// ErrPaymentUnavailable instead of charging blindly.
	Provider renewalProvider
	Payments *PaymentService
	Clock    clock.Clock
	Config   Config
	Logger   *slog.Logger
	// Publisher emits the grace lifecycle events (issue #253); nil keeps the
	// pre-#253 behaviour.
	Publisher EventPublisher
}

// NewWorkers creates the billing worker phases over the shared factory. Nil
// clock, config and logger default to the real clock, DefaultConfig and the
// default logger, matching the module's constructor conventions.
func NewWorkers(factory txStoreFactory, cfg WorkersConfig) *Workers {
	if cfg.Clock == nil {
		cfg.Clock = clock.Real{}
	}
	if cfg.Config == (Config{}) {
		cfg.Config = DefaultConfig()
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Workers{
		txStoreFactory: factory,
		payments:       cfg.Payments,
		provider:       cfg.Provider,
		clock:          cfg.Clock,
		config:         cfg.Config,
		log:            cfg.Logger,
		publisher:      cfg.Publisher,
	}
}

// Worker-slot audit triggers — the access coordinator records them in its
// audit context, so they are low-cardinality labels naming the phase that
// dropped a tariff limit.
const (
	triggerScheduledDowngrade = "scheduled_downgrade"
	triggerFreeDowngrade      = "free_downgrade"
	triggerRenewalDowngrade   = "renewal_downgrade"
	triggerGraceExpired       = "grace_expired"
	triggerNonRenewingExpired = "non_renewing_expired"
	triggerCancelledExpired   = "cancelled_expired"
	triggerRefund             = "refund"
)

// SetLifecycleBridges wires the cross-context lifecycle bridges the expiry and
// downgrade phases call in their transactions: excess-property archiving
// (properties context) and recipient-slot enforcement (access context). The
// composition root calls it once those modules exist — billing is built before
// them. Without bridges the phases still apply every subscription change; only
// the excess properties and suspended memberships wait for the next wired run.
func (w *Workers) SetLifecycleBridges(archiver ExcessPropertyArchiverSource, slots RecipientSlotEnforcerSource) {
	w.archiverSource = archiver
	w.slotSource = slots
}

// runLifecycleTx runs work like runInTx plus the lifecycle bridges bound to
// the transaction — the transaction shape of every phase that lowers a tariff
// limit.
func (w *Workers) runLifecycleTx(ctx context.Context, work func(*txStores) error) error {
	return w.runInTxWithBridges(ctx, w.archiverSource, w.slotSource, work)
}

// ProcessScheduledChanges applies due deferred tariff changes whose target is
// free: the switch happens in one transaction with no provider call — the
// tariff changes, validity extends from now, auto-renew stays on and any
// excess properties are archived (ADR 0008 §3). A paid target is skipped:
// it is charged at apply time by the renewal phase, which runs right after
// this one in the same tick. Returns the number of applied changes; skipped
// paid downgrades are not counted because they stay in the selection until
// their renewal charge clears them.
func (w *Workers) ProcessScheduledChanges(ctx context.Context, now time.Time) (int, error) {
	processed := 0
	for {
		subs, err := w.subscriptions.ListPendingChanges(ctx, now, w.config.WorkerBatchSize)
		if err != nil {
			return processed, fmt.Errorf("list subscriptions with pending change: %w", err)
		}
		if len(subs) == 0 {
			break
		}
		applied := 0
		skipped := 0
		for _, sub := range subs {
			didApply, applyErr := w.applyScheduledChange(ctx, sub, now)
			if applyErr != nil {
				w.log.ErrorContext(ctx, "apply scheduled change failed",
					slog.String("subscription_id", sub.ID.String()),
					slog.String("user_id", sub.UserID.String()),
					slog.String("error", sanitize.Error(applyErr)))
				continue
			}
			if didApply {
				applied++
			} else {
				skipped++
			}
		}
		processed += applied
		if len(subs) < w.config.WorkerBatchSize {
			break
		}
		if applied == 0 {
			// A full batch with zero applies stays in the selection: failed
			// items retry next tick, skipped paid downgrades wait for their
			// renewal charge. Warn only when nothing was skipped, i.e. the
			// batch made no progress due to failures alone.
			if skipped == 0 {
				w.log.WarnContext(ctx, "batch made no progress; deferring to next tick",
					slog.String("op", "apply scheduled changes"))
			}
			break
		}
	}
	return processed, nil
}

// applyScheduledChange applies one due deferred change. It returns applied
// only for the free path; a paid target reports false so the caller leaves it
// counted as skipped, and a pending change consumed between listing and
// locking reports true — the row left the selection either way.
func (w *Workers) applyScheduledChange(ctx context.Context, listed domain.Subscription, now time.Time) (bool, error) {
	applied := false
	err := w.runLifecycleTx(ctx, func(stores *txStores) error {
		sub, err := stores.subscriptionForUpdate(ctx, listed.UserID)
		if err != nil {
			return err
		}
		if !sub.HasPendingChange() {
			applied = true
			return nil
		}
		target, err := stores.tariffs.GetByID(ctx, *sub.PendingTariffID)
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
		price, err := target.Price(period)
		if err != nil {
			return err
		}
		if price > 0 {
			// A paid target is charged at apply time; charging belongs to the
			// renewal phase running right after this one.
			return nil
		}

		transition, err := stores.applyTransition(ctx, &sub,
			func(s *domain.Subscription) error { return s.ApplyScheduledDowngrade(target, period, now) },
			transitionSpec{
				reason:    domain.TransitionReasonScheduledChangeApplied,
				initiator: domain.InitiatorSystem,
			},
		)
		if err != nil {
			return err
		}
		if transitionChangedTariff(transition, target.ID) {
			if err := stores.enforceTariffLimit(ctx, sub.UserID, target.ActivePropertyLimit, triggerScheduledDowngrade); err != nil {
				return fmt.Errorf("enforce tariff limit after scheduled downgrade: %w", err)
			}
		}
		applied = true
		return nil
	})
	return applied, err
}

// ProcessRenewals drives the expiry of paid periods (ADR 0008): subscriptions
// with auto-renew on are charged on their active payment method — a failed or
// impossible charge moves them to grace — while subscriptions with auto-renew
// off (and cancelled ones whose retained period ended) downgrade to basic with
// their excess properties archived. Returns the number of subscriptions
// processed.
func (w *Workers) ProcessRenewals(ctx context.Context, now time.Time) (int, error) {
	if w.provider == nil {
		return 0, fmt.Errorf("renewal worker requires a payment provider: %w", ErrPaymentUnavailable)
	}

	processed := 0
	n, err := w.processSubscriptionBatch(ctx, now, "renew", w.subscriptions.ListUpForRenewal, w.renewSubscription)
	if err != nil {
		return processed, err
	}
	processed += n

	basicTariff, err := w.tariffs.GetByName(ctx, domain.TariffBasic)
	if err != nil {
		return processed, fmt.Errorf("get basic tariff for non-renewing expiry: %w", err)
	}
	expire := func(eligible func(domain.Subscription, time.Time) bool, trigger string) func(context.Context, domain.Subscription, time.Time) error {
		return func(ctx context.Context, sub domain.Subscription, now time.Time) error {
			return w.expireSubscription(ctx, sub, basicTariff, now, trigger, eligible)
		}
	}

	n, err = w.processSubscriptionBatch(ctx, now, "expire non-renewing", w.subscriptions.ListExpiredNonRenewing, expire(subscriptionExpiredNonRenewing, triggerNonRenewingExpired))
	if err != nil {
		return processed, err
	}
	processed += n

	n, err = w.processSubscriptionBatch(ctx, now, "expire cancelled", w.subscriptions.ListExpiredCancelled, expire(subscriptionExpiredCancelled, triggerCancelledExpired))
	if err != nil {
		return processed, err
	}
	processed += n
	return processed, nil
}

// processSubscriptionBatch loops a worker listing in batches and processes
// every row. A full batch with zero progress stays in the selection, so the
// loop stops and defers to the next tick instead of spinning on the same
// failing rows.
func (w *Workers) processSubscriptionBatch(
	ctx context.Context,
	now time.Time,
	op string,
	list func(context.Context, time.Time, int) ([]domain.Subscription, error),
	process func(context.Context, domain.Subscription, time.Time) error,
) (int, error) {
	processed := 0
	for {
		subs, err := list(ctx, now, w.config.WorkerBatchSize)
		if err != nil {
			return processed, fmt.Errorf("list subscriptions for %s: %w", op, err)
		}
		if len(subs) == 0 {
			break
		}
		batchProcessed := 0
		for _, sub := range subs {
			if err := process(ctx, sub, now); err != nil {
				w.log.ErrorContext(ctx, op+" subscription failed",
					slog.String("subscription_id", sub.ID.String()),
					slog.String("user_id", sub.UserID.String()),
					slog.String("error", sanitize.Error(err)))
				continue
			}
			batchProcessed++
		}
		processed += batchProcessed
		if len(subs) < w.config.WorkerBatchSize {
			break
		}
		if batchProcessed == 0 {
			w.log.WarnContext(ctx, "batch made no progress; deferring to next tick",
				slog.String("op", op))
			break
		}
	}
	return processed, nil
}

// renewSubscription charges one expired auto-renewing subscription in the
// two-transaction pattern: the planning transaction locks the subscription,
// resolves the renewal terms and persists the pending payment (the durable
// record that survives a crash before the provider call); the provider calls
// run outside any transaction; a second transaction finalizes the payment and
// applies its subscription effects.
func (w *Workers) renewSubscription(ctx context.Context, sub domain.Subscription, now time.Time) error {
	var plan renewalPlan
	// The planning transaction runs with the lifecycle bridges: its free-terms
	// path can lower a tariff limit, and that archiving belongs to the same
	// commit as the subscription change. Its grace path captures the event,
	// and the grace-events module publishes it strictly after the commit
	// (issue #284).
	grace := newGraceEvents(w.publisher, w.log)
	err := grace.run(ctx, w.runLifecycleTx, func(stores *txStores) error {
		var planErr error
		plan, planErr = w.planRenewal(ctx, grace, stores, sub, now)
		return planErr
	})
	if err != nil || !plan.ready {
		return err
	}
	return w.chargeRenewal(ctx, plan, now)
}

// renewalPlan is the outcome of the renewal planning transaction: a pending
// payment, the method it charges and the tariff terms it renews. A plan that
// is not ready marks an early exit — nothing is due for a charge (the
// subscription renewed or changed since listing, its terms are free, or it
// entered grace through the grace-events module).
type renewalPlan struct {
	ready   bool
	payment domain.SubscriptionPayment
	method  domain.PaymentMethod
	tariff  domain.Tariff
}

// planRenewal runs inside the planning transaction. A plan that comes back
// not ready is an early exit: the subscription renewed or changed since
// listing, its terms are free (applied right here), or it entered grace — so
// no charge is due.
func (w *Workers) planRenewal(ctx context.Context, grace *graceEvents, stores *txStores, listed domain.Subscription, now time.Time) (renewalPlan, error) {
	sub, err := stores.subscriptionForUpdate(ctx, listed.UserID)
	if err != nil {
		return renewalPlan{}, err
	}
	if sub.Status != domain.SubscriptionStatusActive || !sub.AutoRenewEnabled || sub.ValidUntil == nil || sub.ValidUntil.After(now) {
		// Renewed or changed between listing and locking.
		return renewalPlan{}, nil
	}

	renewalTariff, period, amount, err := renewalTerms(ctx, stores, sub)
	if err != nil {
		return renewalPlan{}, err
	}
	if amount <= 0 {
		// Free terms renew without a charge: the basic tariff's renewal is the
		// permanent fall to basic; a pending free change applies in place.
		return renewalPlan{}, w.applyFreeRenewal(ctx, stores, sub, renewalTariff, period, now)
	}

	method, chargeable, err := w.chargeableMethod(ctx, stores, sub)
	if err != nil {
		return renewalPlan{}, err
	}
	if !chargeable {
		// No chargeable method: nothing to charge and nothing failed at the
		// provider — grace gives the user the window to bind a card. The
		// grace-events module captures the entry for the post-commit
		// publication.
		return renewalPlan{}, grace.enterGrace(ctx, stores, sub, now, w.config.GraceDuration)
	}

	// Reuse a pending renewal payment from a crashed run instead of initiating
	// a duplicate; the pending-payments unique index is the durable backstop.
	pending := findPendingPayment(stores, ctx, sub.UserID, renewalTariff.ID, period)
	if pending == nil {
		payment, err := domain.NewSubscriptionPayment(sub.UserID, sub.ID, renewalTariff.ID, period, amount, w.provider.Name(), now)
		if err != nil {
			return renewalPlan{}, err
		}
		payment.PaymentMethodID = &method.ID
		payment, err = stores.payments.Create(ctx, payment)
		if err != nil {
			if !errors.Is(err, ErrAlreadyExists) {
				return renewalPlan{}, fmt.Errorf("save renewal payment: %w", err)
			}
			pending = findPendingPayment(stores, ctx, sub.UserID, renewalTariff.ID, period)
			if pending == nil {
				return renewalPlan{}, fmt.Errorf("pending renewal payment lost after unique-race: %w", err)
			}
			payment = *pending
		}
		pending = &payment
	}
	// A recovered pending payment may predate a card switch: point it at the
	// method the subscription charges now, so the record matches the token
	// actually charged.
	if pending.PaymentMethodID == nil || *pending.PaymentMethodID != method.ID {
		methodID := method.ID
		pending.PaymentMethodID = &methodID
		if err := stores.payments.Update(ctx, *pending); err != nil {
			return renewalPlan{}, fmt.Errorf("update renewal payment method: %w", err)
		}
	}
	return renewalPlan{ready: true, payment: *pending, method: method, tariff: renewalTariff}, nil
}

// renewalTerms resolves what a renewal buys: a due scheduled change renews
// into its target tariff and period (charged at apply time); otherwise the
// current tariff for the subscription's current period. The period is
// subscription state, not payment history — right after a scheduled downgrade
// the last succeeded payment still references the old tariff's period.
func renewalTerms(ctx context.Context, stores *txStores, sub domain.Subscription) (domain.Tariff, domain.SubscriptionPeriod, int64, error) {
	if sub.PendingTariffID != nil && sub.PendingPeriod != nil {
		target, err := stores.tariffs.GetByID(ctx, *sub.PendingTariffID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return domain.Tariff{}, "", 0, ErrTariffNotFound
			}
			return domain.Tariff{}, "", 0, fmt.Errorf("get pending tariff: %w", err)
		}
		amount, err := target.Price(*sub.PendingPeriod)
		if err != nil {
			return domain.Tariff{}, "", 0, err
		}
		return target, *sub.PendingPeriod, amount, nil
	}
	current, err := stores.tariffs.GetByID(ctx, sub.TariffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Tariff{}, "", 0, ErrTariffNotFound
		}
		return domain.Tariff{}, "", 0, fmt.Errorf("get current tariff: %w", err)
	}
	period := domain.PeriodMonth
	if sub.CurrentPeriod != nil {
		period = *sub.CurrentPeriod
	}
	amount, err := current.Price(period)
	if err != nil {
		return domain.Tariff{}, "", 0, err
	}
	return current, period, amount, nil
}

// applyFreeRenewal completes a renewal whose terms cost nothing, inside the
// planning transaction (issue #252). The basic tariff's renewal is the
// permanent fall to basic — validity cleared, auto-renew off, excess
// properties archived, the shared expiry outcome. A free non-basic target
// (none is seeded today, the branch keeps the phase total) renews in place or
// applies a pending free change without a payment.
func (w *Workers) applyFreeRenewal(ctx context.Context, stores *txStores, sub domain.Subscription, tariff domain.Tariff, period domain.SubscriptionPeriod, now time.Time) error {
	reason := domain.TransitionReasonExpired
	if tariff.Name != domain.TariffBasic {
		reason = domain.TransitionReasonScheduledChangeApplied
	}
	applied, err := stores.applyTransition(ctx, &sub,
		func(s *domain.Subscription) error {
			switch {
			case tariff.Name == domain.TariffBasic:
				s.DowngradeToBasic(tariff.ID)
				return nil
			case s.TariffID == tariff.ID:
				return s.ApplyRenewal(uuid.Nil, period, now)
			default:
				current, err := stores.tariffs.GetByID(ctx, s.TariffID)
				if err != nil {
					if errors.Is(err, ErrNotFound) {
						return fmt.Errorf("subscription %s references missing tariff %s: %w", s.ID, s.TariffID, ErrTariffNotFound)
					}
					return fmt.Errorf("get current tariff: %w", err)
				}
				return s.ApplyTariffChange(uuid.Nil, current, tariff, period, now)
			}
		},
		transitionSpec{
			reason:    reason,
			initiator: domain.InitiatorSystem,
		},
	)
	if err != nil {
		return err
	}
	if transitionChangedTariff(applied, sub.TariffID) {
		if err := stores.enforceTariffLimit(ctx, sub.UserID, tariff.ActivePropertyLimit, triggerFreeDowngrade); err != nil {
			return fmt.Errorf("enforce tariff limit after free renewal: %w", err)
		}
	}
	return nil
}

// chargeableMethod resolves the subscription's active payment method as a
// charge target of the active provider. A missing method — none linked, the
// row gone — or a method saved by another provider (ADR 0038: a token is only
// meaningful together with its provider) is as good as absent: no charge is
// possible until the user binds a card of the active provider.
func (w *Workers) chargeableMethod(ctx context.Context, stores *txStores, sub domain.Subscription) (domain.PaymentMethod, bool, error) {
	if sub.ActivePaymentMethodID == nil {
		return domain.PaymentMethod{}, false, nil
	}
	method, err := stores.methods.GetByID(ctx, *sub.ActivePaymentMethodID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.PaymentMethod{}, false, nil
		}
		return domain.PaymentMethod{}, false, fmt.Errorf("get active payment method: %w", err)
	}
	if method.Provider != w.provider.Name() {
		return domain.PaymentMethod{}, false, nil
	}
	return method, true, nil
}

// chargeRenewal runs the provider half of a renewal outside any transaction:
// the idempotent MIT initiation (when the pending payment has no provider
// reference yet), the pre-charge status check that rules out a double charge,
// the charge itself, and the finalizing transaction.
func (w *Workers) chargeRenewal(ctx context.Context, plan renewalPlan, now time.Time) error {
	payment := plan.payment
	if !payment.HasProviderReference() {
		initRes, err := w.provider.InitPayment(ctx, w.mitInitRequest(payment, plan.tariff))
		if err != nil {
			// Init refused: nothing was or will be charged for this payment —
			// the outcome is certain, fail it and enter grace.
			return w.failRenewalPayment(ctx, payment.ID, providerErrorCode(err), now)
		}
		// The MIT path of the shared reference save (issue #285): the
		// initiation result carries no payer URL; the saved payment is the
		// charge target the rest of this phase runs on.
		payment, err = saveProviderReference(ctx, w.runInTx, payment.ID, initRes, w.clock.Now().UTC())
		if err != nil {
			return err
		}
	}
	if !payment.HasProviderReference() {
		// A concurrent flow finalized or re-referenced the payment; the
		// persisted state wins.
		return nil
	}

	// A referenced payment may already carry an earlier run's outcome whose
	// application crashed: the provider's state rules out a second charge.
	status, err := w.provider.PaymentStatus(ctx, payment.ID, *payment.ProviderPaymentID)
	if err != nil {
		// The previous outcome may be a success; never charge over an unknown
		// status. The payment stays pending and a later tick resolves it.
		w.log.WarnContext(ctx, "provider status unavailable; skipping charge attempt this tick",
			slog.String("payment_id", payment.ID.String()),
			slog.String("error", sanitize.Error(err)))
		return nil
	}
	switch status.Status {
	case domain.PaymentStatusSucceeded:
		return w.applyRenewalSuccess(ctx, payment, now)
	case domain.PaymentStatusFailed:
		return w.failRenewalPayment(ctx, payment.ID, errorCodeFromResult(status.ErrorCode), now)
	default:
		// Any other provider state (still pending, refunding, refunded) leaves
		// the charge unresolved — fall through to a fresh charge attempt.
	}

	charge, err := w.provider.ChargePayment(ctx, ChargeRequest{
		PaymentID:         payment.ID,
		ProviderPaymentID: *payment.ProviderPaymentID,
		AmountKopecks:     payment.AmountKopecks,
		ChargeToken:       plan.method.ProviderToken,
	})
	if err != nil {
		return w.recoverUncertainCharge(ctx, payment, err, now)
	}
	switch charge.Status {
	case domain.PaymentStatusSucceeded:
		return w.applyRenewalSuccess(ctx, payment, now)
	case domain.PaymentStatusFailed:
		return w.failRenewalPayment(ctx, payment.ID, errorCodeFromResult(charge.ErrorCode), now)
	default:
		// The provider finalizes asynchronously and delivers the webhook; the
		// pending payment and the subscription stay as they are.
		return nil
	}
}

// mitInitRequest builds the provider-neutral initiation of a merchant-initiated
// renewal: no payer form (no deadline) and no save-method — the credentials
// were saved by the customer-initiated parent payment; the purpose is worded
// as a renewal.
func (w *Workers) mitInitRequest(payment domain.SubscriptionPayment, tariff domain.Tariff) InitPaymentRequest {
	return InitPaymentRequest{
		PaymentID:     payment.ID,
		AmountKopecks: payment.AmountKopecks,
		Period:        payment.Period,
		CustomerRef:   payment.UserID.String(),
		Purpose: PaymentPurpose{
			Kind:       PaymentPurposeRenewal,
			TariffName: tariff.Name,
			Period:     payment.Period,
		},
		Initiator: InitiatorMerchant,
	}
}

// applyRenewalSuccess finalizes a provider-confirmed renewal charge and
// applies its subscription effects in one transaction. A payment another flow
// finalized first is a no-op — the persisted state wins. When the applied
// payment switches the tariff (a scheduled downgrade charged at apply time),
// the excess properties are archived and the recipient slots enforced in the
// same transaction.
func (w *Workers) applyRenewalSuccess(ctx context.Context, payment domain.SubscriptionPayment, now time.Time) error {
	return w.runLifecycleTx(ctx, func(stores *txStores) error {
		current, err := stores.paymentForUpdate(ctx, payment.ID)
		if err != nil {
			return err
		}
		if current.Status != domain.PaymentStatusPending {
			return nil
		}
		if err := current.MarkSucceeded(now); err != nil {
			return err
		}
		if err := stores.payments.Update(ctx, current); err != nil {
			return fmt.Errorf("mark renewal payment succeeded: %w", err)
		}

		applied, err := applySucceededPayment(ctx, stores, current, now)
		if err != nil {
			return err
		}
		if transitionChangedTariff(applied, current.TariffID) {
			target, err := stores.tariffs.GetByID(ctx, current.TariffID)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					return fmt.Errorf("payment %s references missing tariff %s: %w", current.ID, current.TariffID, ErrTariffNotFound)
				}
				return fmt.Errorf("get applied tariff: %w", err)
			}
			if err := stores.enforceTariffLimit(ctx, current.UserID, target.ActivePropertyLimit, triggerRenewalDowngrade); err != nil {
				return fmt.Errorf("enforce tariff limit after renewal downgrade: %w", err)
			}
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorRole:  auditdomain.ActorRoleSystem,
			Action:     auditdomain.ActionSubscriptionPaymentSucceeded,
			EntityType: auditdomain.EntitySubscriptionPayment,
			EntityID:   &current.ID,
			Context:    map[string]any{"payment_id": current.ID, "provider": string(w.provider.Name()), "amount_kopecks": current.AmountKopecks},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
}

// failRenewalPayment finalizes a definitively failed renewal charge and moves
// the subscription into grace in the same transaction: the pending window for
// the user to fix the payment method (ADR 0008). A payment finalized by
// another flow first is a no-op. The grace-entered event is captured by the
// grace-events module inside the transaction and published strictly after the
// commit — best-effort, a publication failure is logged and never fails the
// finalized payment (issue #284).
func (w *Workers) failRenewalPayment(ctx context.Context, paymentID uuid.UUID, errorCode *string, now time.Time) error {
	grace := newGraceEvents(w.publisher, w.log)
	return grace.run(ctx, w.runInTx, func(stores *txStores) error {
		payment, err := stores.paymentForUpdate(ctx, paymentID)
		if err != nil {
			return err
		}
		if payment.Status != domain.PaymentStatusPending {
			return nil
		}
		if err := payment.MarkFailed(errorCode, now); err != nil {
			return err
		}
		if err := stores.payments.Update(ctx, payment); err != nil {
			return fmt.Errorf("mark renewal payment failed: %w", err)
		}
		sub, err := stores.subscriptionForUpdate(ctx, payment.UserID)
		if err != nil {
			return err
		}
		if err := grace.enterGrace(ctx, stores, sub, now, w.config.GraceDuration); err != nil {
			return err
		}
		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorRole:  auditdomain.ActorRoleSystem,
			Action:     auditdomain.ActionSubscriptionPaymentFailed,
			EntityType: auditdomain.EntitySubscriptionPayment,
			EntityID:   &payment.ID,
			Context:    map[string]any{"payment_id": payment.ID, "provider": string(w.provider.Name())},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
}

// recoverUncertainCharge resolves a renewal charge whose provider call
// returned an error: the money state is unknown — the charge may have been
// captured. The provider's current status is the source of truth (ADR 0010)
// and is queried outside any transaction. A definitive outcome is applied; an
// unresolved one counts as a charge attempt and, at the configured limit, the
// payment fails and the subscription enters grace, so a permanently stuck
// charge cannot keep an expired subscription active forever.
func (w *Workers) recoverUncertainCharge(ctx context.Context, payment domain.SubscriptionPayment, cause error, now time.Time) error {
	switch {
	case errors.Is(cause, ErrProviderChargeBlocked):
		w.log.ErrorContext(ctx, "renewal charge blocked by provider: COF/recurring operations are not enabled on the terminal; contact the provider manager to enable them",
			slog.String("payment_id", payment.ID.String()),
			slog.String("error", sanitize.Error(cause)))
	case errors.Is(cause, ErrProviderInvalidOperation):
		w.log.ErrorContext(ctx, "renewal rejected by provider as an invalid operation: integration misconfiguration; investigate the provider integration",
			slog.String("payment_id", payment.ID.String()),
			slog.String("error", sanitize.Error(cause)))
	}

	if !payment.HasProviderReference() {
		// The charge never reached a provider payment: nothing can be
		// captured, the outcome is certain.
		return w.failRenewalPayment(ctx, payment.ID, providerErrorCode(cause), now)
	}
	status, err := w.provider.PaymentStatus(ctx, payment.ID, *payment.ProviderPaymentID)
	if err != nil {
		// Still unknown: this must not count as a charge attempt — the charge
		// may already be captured. Leave everything for a later tick.
		w.log.WarnContext(ctx, "provider status unknown after charge error; leaving subscription active and payment pending",
			slog.String("payment_id", payment.ID.String()),
			slog.String("error", sanitize.Error(err)))
		return nil
	}
	switch status.Status {
	case domain.PaymentStatusSucceeded:
		return w.applyRenewalSuccess(ctx, payment, now)
	case domain.PaymentStatusFailed:
		code := errorCodeFromResult(status.ErrorCode)
		if code == nil {
			code = providerErrorCode(cause)
		}
		return w.failRenewalPayment(ctx, payment.ID, code, now)
	default:
		// Pending or unexpected provider state: count the attempt below and
		// cap it.
	}

	exceeded := false
	err = w.runInTx(ctx, func(stores *txStores) error {
		current, err := stores.paymentForUpdate(ctx, payment.ID)
		if err != nil {
			return err
		}
		if current.Status != domain.PaymentStatusPending {
			return nil
		}
		current.RecordChargeAttempt()
		exceeded = current.ChargeAttempts >= w.config.ChargeAttemptLimit
		if err := stores.payments.Update(ctx, current); err != nil {
			return fmt.Errorf("record charge attempt: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if exceeded {
		w.log.ErrorContext(ctx, "renewal charge hit the attempt limit; failing payment and moving subscription to grace",
			slog.String("payment_id", payment.ID.String()),
			slog.Int("charge_attempt_limit", w.config.ChargeAttemptLimit))
		return w.failRenewalPayment(ctx, payment.ID, providerErrorCode(cause), now)
	}
	w.log.WarnContext(ctx, "renewal charge unresolved at provider; leaving subscription active and payment pending",
		slog.String("payment_id", payment.ID.String()),
		slog.String("provider_status", string(status.Status)))
	return nil
}

// ProcessExpiredGrace downgrades subscriptions whose grace window ended to the
// basic tariff and archives their excess properties (open leases on them
// force-completed) — the shared expiry path of ADR 0008. Returns the number of
// subscriptions processed.
func (w *Workers) ProcessExpiredGrace(ctx context.Context, now time.Time) (int, error) {
	basicTariff, err := w.tariffs.GetByName(ctx, domain.TariffBasic)
	if err != nil {
		return 0, fmt.Errorf("get basic tariff: %w", err)
	}
	return w.processSubscriptionBatch(ctx, now, "expire grace", w.subscriptions.ListInExpiredGrace,
		func(ctx context.Context, sub domain.Subscription, now time.Time) error {
			return w.expireSubscription(ctx, sub, basicTariff, now, triggerGraceExpired, subscriptionInExpiredGrace)
		})
}

// subscriptionInExpiredGrace reports whether the subscription is in grace and
// its grace window has ended.
func subscriptionInExpiredGrace(s domain.Subscription, now time.Time) bool {
	return s.Status == domain.SubscriptionStatusGrace && s.ValidUntil != nil && !s.ValidUntil.After(now)
}

// ProcessGraceExpiryReminders dispatches the grace-expiry reminder of every
// subscription inside its reminder window — the half-open window
// [valid_until - GraceExpiryReminderBefore, valid_until) — exactly once per
// grace window (issue #253). The reminder is never sent before the window
// opens or after the grace ends: the listing bounds both edges, and the
// processing transaction re-checks them under the subscription lock. Returns
// the number of subscriptions reminded.
func (w *Workers) ProcessGraceExpiryReminders(ctx context.Context, now time.Time) (int, error) {
	processed := 0
	for {
		subs, err := w.subscriptions.ListInGraceReminderWindow(ctx, now, w.config.GraceExpiryReminderBefore, w.config.WorkerBatchSize)
		if err != nil {
			return processed, fmt.Errorf("list subscriptions in grace reminder window: %w", err)
		}
		if len(subs) == 0 {
			break
		}
		reminded := 0
		for _, sub := range subs {
			if err := w.remindGraceExpiring(ctx, sub, now); err != nil {
				w.log.ErrorContext(ctx, "grace expiry reminder failed",
					slog.String("subscription_id", sub.ID.String()),
					slog.String("user_id", sub.UserID.String()),
					slog.String("error", sanitize.Error(err)))
				continue
			}
			reminded++
			processed++
		}
		if len(subs) < w.config.WorkerBatchSize {
			break
		}
		if reminded == 0 {
			w.log.WarnContext(ctx, "batch made no progress; deferring to next tick",
				slog.String("op", "grace expiry reminders"))
			break
		}
	}
	return processed, nil
}

// remindGraceExpiring applies one grace-expiry reminder: the transaction locks
// the subscription and hands it to the grace-events module, which re-checks
// the reminder window under the lock, marks it reminded and publishes the
// event strictly after the commit, best-effort, once per window (issue #284).
// An ineligible subscription is a no-op, not an error.
func (w *Workers) remindGraceExpiring(ctx context.Context, listed domain.Subscription, now time.Time) error {
	grace := newGraceEvents(w.publisher, w.log)
	return grace.run(ctx, w.runInTx, func(stores *txStores) error {
		sub, err := stores.subscriptionForUpdate(ctx, listed.UserID)
		if err != nil {
			return err
		}
		return grace.remindWindow(ctx, stores, sub, now, w.config.GraceExpiryReminderBefore)
	})
}

// subscriptionExpiredNonRenewing reports whether an active subscription with
// auto-renew off has run out its paid period.
func subscriptionExpiredNonRenewing(s domain.Subscription, now time.Time) bool {
	return s.Status == domain.SubscriptionStatusActive && !s.AutoRenewEnabled && s.ValidUntil != nil && !s.ValidUntil.After(now)
}

// subscriptionExpiredCancelled reports whether a cancelled subscription has
// run out its retained paid period.
func subscriptionExpiredCancelled(s domain.Subscription, now time.Time) bool {
	return s.Status == domain.SubscriptionStatusCancelled && s.ValidUntil != nil && !s.ValidUntil.After(now)
}

// expireSubscription applies the shared expiry path in one transaction: it
// locks the subscription, re-checks eligibility under the lock (the state the
// listing saw may be gone), downgrades to basic with its transition-log entry,
// archives the excess properties and enforces the recipient slots. An
// ineligible subscription is a no-op, not an error.
func (w *Workers) expireSubscription(ctx context.Context, listed domain.Subscription, basicTariff domain.Tariff, now time.Time, trigger string, eligible func(domain.Subscription, time.Time) bool) error {
	return w.runLifecycleTx(ctx, func(stores *txStores) error {
		sub, err := stores.subscriptionForUpdate(ctx, listed.UserID)
		if err != nil {
			return err
		}
		if !eligible(sub, now) {
			return nil
		}
		if _, err := stores.applyTransition(ctx, &sub,
			func(s *domain.Subscription) error { s.DowngradeToBasic(basicTariff.ID); return nil },
			transitionSpec{
				reason:    domain.TransitionReasonExpired,
				initiator: domain.InitiatorSystem,
			},
		); err != nil {
			return err
		}
		if err := stores.enforceTariffLimit(ctx, sub.UserID, basicTariff.ActivePropertyLimit, trigger); err != nil {
			return fmt.Errorf("enforce tariff limit after expiry downgrade: %w", err)
		}
		return nil
	})
}

// ProcessPendingUpgradePayments finalizes stale pending tariff-change payments
// (upgrades and recovery upgrades of the ChangeTariff flow) whose webhook was
// lost: the provider's status is the source of truth and its outcome is
// applied through the same synchronous path the webhook uses. Returns the
// number of payments checked with the provider.
func (w *Workers) ProcessPendingUpgradePayments(ctx context.Context, now time.Time) (int, error) {
	return w.reconcileStalePendingPayments(ctx, now, "pending upgrade payments", w.txStoreFactory.payments.ListStalePendingUpgrades)
}

// ReconcilePendingPayments pulls the lost webhooks of every stale pending
// payment with a provider reference (issue #252): the reconciliation worker's
// backstop beyond the provider's own redeliveries. Returns the number of
// payments checked with the provider.
func (w *Workers) ReconcilePendingPayments(ctx context.Context, now time.Time) (int, error) {
	return w.reconcileStalePendingPayments(ctx, now, "pending payments", w.txStoreFactory.payments.ListStalePending)
}

// reconcileStalePendingPayments batches a stale-pending listing and resolves
// each payment against the provider. A payment the provider has not settled
// stays pending; a full batch with nothing finalized stays in the selection,
// so the loop stops and defers to the next tick.
func (w *Workers) reconcileStalePendingPayments(ctx context.Context, now time.Time, op string, list func(context.Context, time.Time, int) ([]domain.SubscriptionPayment, error)) (int, error) {
	if w.provider == nil {
		return 0, fmt.Errorf("payment reconciliation worker requires a payment provider: %w", ErrPaymentUnavailable)
	}
	processed := 0
	createdBefore := now.Add(-w.config.PendingPaymentStaleness)
	for {
		payments, err := list(ctx, createdBefore, w.config.WorkerBatchSize)
		if err != nil {
			return processed, fmt.Errorf("list stale %s: %w", op, err)
		}
		if len(payments) == 0 {
			break
		}
		finalized := 0
		for _, payment := range payments {
			if !payment.HasProviderReference() {
				continue
			}
			processed++
			status, statusErr := w.provider.PaymentStatus(ctx, payment.ID, *payment.ProviderPaymentID)
			if statusErr != nil {
				w.log.WarnContext(ctx, "failed to query provider status for stale "+op,
					slog.String("payment_id", payment.ID.String()),
					slog.String("error", sanitize.Error(statusErr)))
				continue
			}
			switch status.Status {
			case domain.PaymentStatusSucceeded, domain.PaymentStatusFailed:
				if err := w.finalizeFromProviderStatus(ctx, payment, status); err != nil {
					w.log.ErrorContext(ctx, "failed to finalize stale "+op+" from provider status",
						slog.String("payment_id", payment.ID.String()),
						slog.String("error", sanitize.Error(err)))
					continue
				}
				finalized++
			case domain.PaymentStatusPending:
				// The provider has not settled the payment yet.
			default:
				w.log.WarnContext(ctx, "unexpected provider status for stale "+op,
					slog.String("payment_id", payment.ID.String()),
					slog.String("provider_status", string(status.Status)))
			}
		}
		if len(payments) < w.config.WorkerBatchSize {
			break
		}
		if finalized == 0 {
			w.log.WarnContext(ctx, "batch made no progress; deferring to next tick",
				slog.String("op", op))
			break
		}
	}
	return processed, nil
}

// finalizeFromProviderStatus routes a provider-confirmed outcome through the
// synchronous notification application the webhook flow uses (issue #250,
// ADR 0039): atomic finalization with the subscription effects, idempotent on
// repeats.
func (w *Workers) finalizeFromProviderStatus(ctx context.Context, payment domain.SubscriptionPayment, status PaymentStatusResult) error {
	return w.payments.handlePaymentNotification(ctx, notificationFromStatus(payment, status))
}

// ReconcileStaleRefunds resolves payments stuck in the refunding reservation
// of the refund saga (issue #254): the provider call's outcome was never
// finalized — an in-flight refund, an unanswered request, a crash between the
// phases. The provider's status is the source of truth (ADR 0010): a refunded
// payment finalizes with its subscription effects, a still-captured charge
// reverts the reservation to the payment's previous status (a pending one
// re-enters the pending-payment reconciliation), a failed provider status is
// ambiguous for a refund and stays under manual review, and anything else
// waits for the next tick. Returns the number of payments checked with the
// provider.
func (w *Workers) ReconcileStaleRefunds(ctx context.Context, now time.Time) (int, error) {
	if w.provider == nil {
		return 0, fmt.Errorf("refund reconciliation worker requires a payment provider: %w", ErrPaymentUnavailable)
	}
	if w.payments == nil {
		return 0, fmt.Errorf("refund reconciliation worker requires the payment service: %w", ErrPaymentUnavailable)
	}

	processed := 0
	updatedBefore := now.Add(-w.config.PendingPaymentStaleness)
	for {
		payments, err := w.txStoreFactory.payments.ListStaleRefunding(ctx, updatedBefore, w.config.WorkerBatchSize)
		if err != nil {
			return processed, fmt.Errorf("list stale refunding payments: %w", err)
		}
		if len(payments) == 0 {
			break
		}
		resolved := 0
		for _, payment := range payments {
			processed++
			didResolve, err := w.resolveStaleRefund(ctx, payment)
			if err != nil {
				w.log.ErrorContext(ctx, "failed to resolve stale refunding payment",
					slog.String("payment_id", payment.ID.String()),
					slog.String("error", sanitize.Error(err)))
				continue
			}
			if didResolve {
				resolved++
			}
		}
		if len(payments) < w.config.WorkerBatchSize {
			break
		}
		if resolved == 0 {
			// A full batch of unresolved refunds stays in the selection; the
			// ambiguous ones need manual review, the unsettled ones the
			// provider's settlement.
			w.log.WarnContext(ctx, "batch made no progress; deferring to next tick",
				slog.String("op", "stale refund reconciliation"))
			break
		}
	}
	return processed, nil
}

// resolveStaleRefund resolves one stuck refunding payment against the
// provider. It reports whether the payment actually left the refunding state
// (finalized or reverted) — the progress signal that keeps the batch loop from
// spinning on permanently unresolved rows.
func (w *Workers) resolveStaleRefund(ctx context.Context, payment domain.SubscriptionPayment) (bool, error) {
	status, err := w.provider.PaymentStatus(ctx, payment.ID, *payment.ProviderPaymentID)
	if err != nil {
		return false, fmt.Errorf("provider status: %w", err)
	}
	switch status.Status {
	case domain.PaymentStatusRefunded, domain.PaymentStatusSucceeded:
		return w.payments.resolveRefundingFromStatus(ctx, payment, status)
	case domain.PaymentStatusFailed:
		// A failed provider status is ambiguous for a payment being refunded:
		// whether the refund happened before the failure is unknowable from
		// the status alone, so the payment stays for manual review.
		w.log.WarnContext(ctx, "refunding payment has a failed status at the provider; manual review required",
			slog.String("payment_id", payment.ID.String()),
			slog.String("provider_payment_id", *payment.ProviderPaymentID))
		return false, nil
	default:
		// The provider has not settled the refund yet; retry on the next tick.
		return false, nil
	}
}

// providerCoder is implemented by provider errors that carry their provider's
// own error code; it is how a code survives into the failed payment row.
type providerCoder interface {
	error
	ProviderErrorCode() string
}

// providerErrorCode extracts the provider's own error code from a provider
// error for persistence on a failed payment; nil when the error carries none.
func providerErrorCode(err error) *string {
	if coder, ok := errors.AsType[providerCoder](err); ok {
		if code := coder.ProviderErrorCode(); code != "" {
			return &code
		}
	}
	return nil
}

// errorCodeFromResult converts the code reported in a provider result to the
// nullable form persisted on failed payments.
func errorCodeFromResult(code string) *string {
	if code == "" {
		return nil
	}
	return &code
}
