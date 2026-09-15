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
)

// paymentInitiationProvider is the narrow provider slice the tariff-change
// payment flow needs: starting a payment and the provider identity payments
// persist (ADR 0038). Declared here, at the consumer, per ADR 0035.
type paymentInitiationProvider interface {
	PaymentInitiator
	ProviderNamer
}

// SubscriptionService serves the user's view of and lifecycle control over
// their own subscription, plus the admin subscription operations of issue
// #255 (admin_subscription_service.go).
type SubscriptionService struct {
	txStoreFactory
	provider paymentInitiationProvider
	clock    clock.Clock
	config   Config
	log      *slog.Logger
	// ArchiverSource and slotSource bridge the admin operations that lower a
	// tariff limit to the properties and access contexts (issue #255); nil
	// until SetLifecycleBridges wires them — the subscription-side change
	// still applies, only the excess archiving waits.
	archiverSource ExcessPropertyArchiverSource
	slotSource     RecipientSlotEnforcerSource
	// The stand-only time-travel railguard of the service (issue #665): the
	// second layer behind the BILLING_TIME_TRAVEL route mounting.
	timeTravelEnabled bool
}

// SubscriptionServiceConfig carries the non-transactional dependencies of the
// subscription service.
type SubscriptionServiceConfig struct {
	Clock clock.Clock
	// Provider starts payments for upgrades and same-tariff grace renewals
	// (issue #250). The tariff-change flow is the only consumer; nil keeps the
	// pre-#250 behaviour of ErrPaymentUnavailable for the paid paths.
	Provider paymentInitiationProvider
	Config   Config
	Logger   *slog.Logger
	// TimeTravelEnabled turns on the admin time-shift rig (issue #665); the
	// wiring sets it from the platform's BILLING_TIME_TRAVEL flag.
	TimeTravelEnabled bool
}

// NewSubscriptionService creates a subscription service over the shared
// factory. A nil clock defaults to the real clock, matching the identity
// module's constructor conventions.
func NewSubscriptionService(factory txStoreFactory, cfg SubscriptionServiceConfig) *SubscriptionService {
	if cfg.Clock == nil {
		cfg.Clock = clock.Real{}
	}
	if cfg.Config == (Config{}) {
		cfg.Config = DefaultConfig()
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &SubscriptionService{
		txStoreFactory:    factory,
		provider:          cfg.Provider,
		clock:             cfg.Clock,
		config:            cfg.Config,
		log:               cfg.Logger,
		timeTravelEnabled: cfg.TimeTravelEnabled,
	}
}

// GetSubscription assembles the user's subscription view: the subscription
// aggregate with its tariff, any scheduled (pending) tariff, the active
// payment method resolved (issue #251), and the live pending payment when one
// holds the user's tariff decision (issue #616).
func (s *SubscriptionService) GetSubscription(ctx context.Context, userID uuid.UUID) (SubscriptionView, error) {
	sub, err := s.subscriptions.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return SubscriptionView{}, ErrSubscriptionNotFound
		}
		return SubscriptionView{}, fmt.Errorf("get subscription: %w", err)
	}

	tariff, err := s.tariffs.GetByID(ctx, sub.TariffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return SubscriptionView{}, fmt.Errorf("subscription %s references missing tariff %s: %w", sub.ID, sub.TariffID, ErrTariffNotFound)
		}
		return SubscriptionView{}, fmt.Errorf("get tariff: %w", err)
	}

	view := SubscriptionView{
		Subscription: sub,
		Tariff:       tariff,
	}
	if sub.PendingTariffID != nil {
		pending, err := s.tariffs.GetByID(ctx, *sub.PendingTariffID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return SubscriptionView{}, fmt.Errorf(
					"subscription %s references missing pending tariff %s: %w",
					sub.ID, *sub.PendingTariffID, ErrTariffNotFound)
			}
			return SubscriptionView{}, fmt.Errorf("get pending tariff: %w", err)
		}
		view.PendingTariff = &pending
	}
	if sub.ActivePaymentMethodID != nil {
		method, err := s.methods.GetByID(ctx, *sub.ActivePaymentMethodID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return SubscriptionView{}, fmt.Errorf(
					"subscription %s references missing payment method %s: %w",
					sub.ID, *sub.ActivePaymentMethodID, ErrPaymentMethodNotFound)
			}
			return SubscriptionView{}, fmt.Errorf("get active payment method: %w", err)
		}
		view.ActivePaymentMethod = &method
	}
	pending, found, err := s.livePendingPayment(ctx, userID)
	if err != nil {
		return SubscriptionView{}, err
	}
	if found {
		view.PendingPayment = pending
	}
	return view, nil
}

// livePendingPayment resolves the user's live pending payment — a pending
// form payment whose deadline has not run out and whose confirm URL has been
// persisted (issue #616) — with its tariff resolved. Merchant-initiated
// charges carry no payer form and are never surfaced; a payment whose URL has
// not been persisted yet (the crash window between the pending row and the
// provider answer) has nothing to confirm and is not surfaced either. The
// found result is false when no pending payment qualifies.
func (s *SubscriptionService) livePendingPayment(ctx context.Context, userID uuid.UUID) (*PendingPaymentView, bool, error) {
	pending, err := s.payments.ListPendingByUserID(ctx, userID)
	if err != nil {
		return nil, false, fmt.Errorf("list pending payments: %w", err)
	}
	for i := range pending {
		p := pending[i]
		if !p.IsLiveFormPayment(s.clock.Now().UTC()) || !p.HasPaymentURL() {
			continue
		}
		tariff, err := s.tariffs.GetByID(ctx, p.TariffID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, false, fmt.Errorf("pending payment %s references missing tariff %s: %w", p.ID, p.TariffID, ErrTariffNotFound)
			}
			return nil, false, fmt.Errorf("get pending payment tariff: %w", err)
		}
		return &PendingPaymentView{Payment: p, Tariff: tariff}, true, nil
	}
	return nil, false, nil
}

// CancelSubscription cancels the user's subscription (issue #249, ADR 0008):
// the status moves to cancelled, auto-renew switches off immediately, and the
// paid tariff keeps working until valid_until — data mutations stay allowed
// for the rest of the period. The state change, its transition-log entry and
// the audit record land in one transaction. A live pending payment blocks the
// cancellation (issue #616): the tariff decision is still being paid for.
// The optional keepPropertyID (issue #617) records the property the owner
// wants to survive the fall to basic when the cancelled period expires: it
// must be one of the owner's active properties, and the validation — like the
// change itself — runs inside the lifecycle transaction over the
// properties-context bridge.
func (s *SubscriptionService) CancelSubscription(ctx context.Context, userID uuid.UUID, keepPropertyID *uuid.UUID) error {
	return s.runLifecycleTx(ctx, func(stores *txStores) error {
		sub, err := stores.subscriptionForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		// Service subscriptions are assigned and withdrawn by an admin (#255);
		// the user cannot cancel what they do not pay for.
		if !sub.IsPaidSource() {
			return domain.ErrInvalidSubscriptionState
		}
		if err := rejectLivePendingPayment(ctx, stores, userID, s.clock.Now().UTC()); err != nil {
			return err
		}
		if keepPropertyID != nil {
			if stores.archiver == nil {
				// The bridge is wired in every composition root; refusing
				// loudly beats storing an unvalidated id (issue #617).
				return errors.New("cancel keep property: properties bridge is not wired")
			}
			exists, err := stores.archiver.ActivePropertyExists(ctx, userID, *keepPropertyID)
			if err != nil {
				return fmt.Errorf("validate keep property: %w", err)
			}
			if !exists {
				return ErrInvalidKeepProperty
			}
		}

		if _, err := stores.applyTransition(ctx, &sub,
			func(s *domain.Subscription) error { return s.Cancel(keepPropertyID) },
			transitionSpec{
				reason:      domain.TransitionReasonCancelled,
				initiator:   domain.InitiatorUser,
				initiatorID: &userID,
				auditAction: auditdomain.ActionSubscriptionCancelled,
				auditContext: func(_ domain.Subscription, _ domain.Transition) map[string]any {
					if keepPropertyID == nil {
						return nil
					}
					return map[string]any{auditKeyKeepPropertyID: *keepPropertyID}
				},
			},
		); err != nil {
			return err
		}
		return nil
	})
}

// ResumeSubscription undoes a cancellation without a charge (issue #617): a
// cancelled subscription inside its already paid period returns to active
// with auto-renew on and the paid remainder kept. The state change, its
// transition-log entry (reason "resumed") and the audit record land in one
// transaction. A live pending payment blocks the resume like every other
// tariff decision (issue #616): the reactivation payment the user already
// holds the form for would supersede it. Restoration of an expired period
// stays on the paid path (issue #429).
func (s *SubscriptionService) ResumeSubscription(ctx context.Context, userID uuid.UUID) error {
	return s.runInTx(ctx, func(stores *txStores) error {
		sub, err := stores.subscriptionForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		if err := rejectLivePendingPayment(ctx, stores, userID, s.clock.Now().UTC()); err != nil {
			return err
		}
		now := s.clock.Now().UTC()
		if _, err := stores.applyTransition(ctx, &sub,
			func(s *domain.Subscription) error { return s.Resume(now) },
			transitionSpec{
				reason:      domain.TransitionReasonResumed,
				initiator:   domain.InitiatorUser,
				initiatorID: &userID,
				auditAction: auditdomain.ActionSubscriptionResumed,
			},
		); err != nil {
			return err
		}
		return nil
	})
}

// rejectLivePendingPayment refuses the flow when a live pending payment — a
// form payment whose deadline has not run out — holds the user's tariff
// decision (issue #616). Merchant-initiated charges carry no payer form and
// no user decision, so they do not block.
func rejectLivePendingPayment(ctx context.Context, stores *txStores, userID uuid.UUID, now time.Time) error {
	pending, err := stores.payments.ListPendingByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("list pending payments: %w", err)
	}
	for _, p := range pending {
		if p.IsLiveFormPayment(now) {
			return ErrPendingPaymentExists
		}
	}
	return nil
}

// rejectConflictingPendingPayment blocks a tariff decision that conflicts
// with the user's live pending payment (issue #616): one pending payment per
// user. A request for the pending payment's own tariff and period passes
// through — the payment path returns it as-is.
func rejectConflictingPendingPayment(
	ctx context.Context, stores *txStores, userID, tariffID uuid.UUID,
	period domain.SubscriptionPeriod, now time.Time,
) error {
	pending, err := stores.payments.ListPendingByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("list pending payments: %w", err)
	}
	for _, p := range pending {
		if !p.IsLiveFormPayment(now) {
			continue
		}
		if p.TariffID == tariffID && p.Period == period {
			return nil
		}
		return ErrPendingPaymentExists
	}
	return nil
}

// ToggleAutoRenew switches automatic renewal on or off (issue #249). Enabling
// requires a paid validity period — the never-expiring basic tariff has
// nothing to renew — and is reserved for paid subscriptions: a service
// subscription runs its fixed term without charges (billing CONTEXT.md).
func (s *SubscriptionService) ToggleAutoRenew(ctx context.Context, userID uuid.UUID, enabled bool) error {
	return s.runInTx(ctx, func(stores *txStores) error {
		sub, err := stores.subscriptionForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		if enabled && !sub.IsPaidSource() {
			return domain.ErrInvalidSubscriptionState
		}
		if err := sub.SetAutoRenew(enabled); err != nil {
			return err
		}
		if err := stores.subscriptions.Update(ctx, sub); err != nil {
			return fmt.Errorf("update subscription: %w", err)
		}
		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &userID,
			ActorRole:  auditdomain.ActorRoleOwner,
			Action:     auditdomain.ActionSubscriptionAutoRenewToggled,
			EntityType: auditdomain.EntitySubscription,
			EntityID:   &sub.ID,
			Context:    map[string]any{"enabled": enabled},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
}

// ChangeTariff serves POST /subscription/change (issue #249). A downgrade is
// scheduled for the end of the paid period and auto-renew is enabled so the
// new tariff renews on the normal cycle (ADR 0008 §3). A same-tariff request
// on an active subscription is rejected; upgrades and same-tariff grace
// renewals go through the payment flow (issue #250): the pending payment is
// persisted before the provider is called, so a crash between the provider
// initiation and the save is recoverable on retry. A same-tariff request on a
// cancelled subscription is its reactivation (issue #429, ADR 0008:
// restoration goes through paying for a tariff) and shares that payment path.
// An upgrade on top of a service subscription takes the payment path too
// (issue #255): the applied payment converts the subscription into a paid one
// from the new period.
func (s *SubscriptionService) ChangeTariff(ctx context.Context, userID uuid.UUID, req ChangeTariffRequest) (ChangeTariffResult, error) {
	var plan changeTariffPlan
	if err := s.runInTx(ctx, func(stores *txStores) error {
		p, err := s.planTariffChange(ctx, stores, userID, req)
		if err != nil {
			return err
		}
		plan = p
		return nil
	}); err != nil {
		return ChangeTariffResult{}, err
	}
	if !plan.needsPayment {
		return ChangeTariffResult{}, nil
	}
	return s.executePlannedPayment(ctx, plan)
}

// changeTariffPlan is the state captured by the planning transaction for the
// payment orchestration that runs after it commits.
type changeTariffPlan struct {
	needsPayment    bool
	paymentPurpose  PaymentPurposeKind
	paymentTariff   domain.Tariff
	createdPayment  *domain.SubscriptionPayment
	existingPayment *domain.SubscriptionPayment
}

// planTariffChange is the transactional half of ChangeTariff: it loads the
// requested tariff and the subscription, classifies the change and either
// schedules the deferred downgrade or plans the payment whose provider
// initiation runs after commit.
func (s *SubscriptionService) planTariffChange(
	ctx context.Context, stores *txStores, userID uuid.UUID, req ChangeTariffRequest,
) (changeTariffPlan, error) {
	newTariff, err := selectableTariff(ctx, stores, req.TariffName)
	if err != nil {
		return changeTariffPlan{}, err
	}

	sub, err := stores.subscriptionForUpdate(ctx, userID)
	if err != nil {
		return changeTariffPlan{}, err
	}

	now := s.clock.Now().UTC()
	// One pending payment per user (issue #616): a live pending form payment
	// locks the tariff decision to what it buys — any other selection is
	// rejected before it is classified, a request matching the pending
	// payment's own tariff and period passes through to the dedup below.
	if err := rejectConflictingPendingPayment(ctx, stores, userID, newTariff.ID, req.Period, now); err != nil {
		return changeTariffPlan{}, err
	}
	// A same-tariff request is a manual payment while the subscription is in
	// grace (a renewal, issue #250) or cancelled (its reactivation, issue
	// #429); both share the payment path with upgrades.
	manualPayment, err := sameTariffManualPayment(sub, newTariff, now)
	if err != nil {
		return changeTariffPlan{}, err
	}

	currentTariff, err := currentTariffOf(ctx, stores, sub)
	if err != nil {
		return changeTariffPlan{}, err
	}

	changeType := domain.ClassifyTariffChange(currentTariff, newTariff)
	needsPayment := manualPayment || changeType == domain.TariffChangeUpgrade
	if !sub.IsPaidSource() && !needsPayment {
		// Service subscriptions are assigned and withdrawn by an admin
		// (#255): the user controls neither their deferred changes nor
		// their cancellation. The payment path stays open — an upgrade on
		// top of a service subscription converts it into a paid one.
		return changeTariffPlan{}, domain.ErrInvalidSubscriptionState
	}
	if !needsPayment {
		return changeTariffPlan{}, scheduleDeferredDowngrade(ctx, stores, userID, sub, currentTariff, newTariff, req.Period)
	}

	if err := s.checkPaymentInitiable(sub, now); err != nil {
		return changeTariffPlan{}, err
	}
	created, existing, err := s.planPayment(ctx, stores, sub, currentTariff, newTariff, req.Period)
	if err != nil {
		return changeTariffPlan{}, err
	}
	paymentPurpose := PaymentPurposeSubscription
	if manualPayment {
		paymentPurpose = PaymentPurposeRenewal
	}
	return changeTariffPlan{
		needsPayment:    true,
		paymentPurpose:  paymentPurpose,
		paymentTariff:   newTariff,
		createdPayment:  created,
		existingPayment: existing,
	}, nil
}

// selectableTariff loads a tariff by name for user selection: a miss and a
// hidden plan (issue #256) answer alike — subscriptions already on a hidden
// plan keep renewing, the guard only blocks new selection.
func selectableTariff(ctx context.Context, stores *txStores, name domain.TariffName) (domain.Tariff, error) {
	tariff, err := stores.tariffs.GetByName(ctx, name)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Tariff{}, ErrTariffNotFound
		}
		return domain.Tariff{}, fmt.Errorf("get tariff: %w", err)
	}
	if !tariff.IsActive {
		return domain.Tariff{}, ErrTariffInactive
	}
	return tariff, nil
}

// sameTariffManualPayment reports whether a same-tariff request is the manual
// payment path: a renewal of a subscription currently in its grace window
// (issue #250) or the reactivation of a cancelled one (issue #429, ADR 0008:
// restoration goes through paying for a tariff). The expired grace window and
// the active same-tariff state are rejections.
func sameTariffManualPayment(sub domain.Subscription, newTariff domain.Tariff, now time.Time) (bool, error) {
	if sub.TariffID != newTariff.ID {
		return false, nil
	}
	switch {
	case sub.IsInGrace(now):
		return true, nil
	case sub.Status == domain.SubscriptionStatusCancelled:
		// Paying for the plan the subscription is already on restores it:
		// the "already on this tariff" rejection is reserved for active
		// subscriptions.
		return true, nil
	case sub.Status == domain.SubscriptionStatusGrace:
		// The grace window has expired; the worker downgrade to basic
		// is due and no payment can be initiated anymore.
		return false, domain.ErrInvalidSubscriptionState
	default:
		return false, domain.ErrAlreadyOnTariff
	}
}

// currentTariffOf loads the tariff the subscription is on, mapping a missing
// row to the tariff sentinel.
func currentTariffOf(ctx context.Context, stores *txStores, sub domain.Subscription) (domain.Tariff, error) {
	tariff, err := stores.tariffs.GetByID(ctx, sub.TariffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Tariff{}, fmt.Errorf("subscription %s references missing tariff %s: %w", sub.ID, sub.TariffID, ErrTariffNotFound)
		}
		return domain.Tariff{}, fmt.Errorf("get current tariff: %w", err)
	}
	return tariff, nil
}

// checkPaymentInitiable guards the payment path: payments start from a live
// subscription — active, in grace, or cancelled awaiting its restoration
// payment (ADR 0008: restoration goes through paying for a tariff; the
// recovery upgrade and the same-tariff reactivation of issue #429) — and a
// provider must be wired before any payment is planned (no pending row is
// left behind).
func (s *SubscriptionService) checkPaymentInitiable(sub domain.Subscription, now time.Time) error {
	if !sub.CanInitiatePayment(now) {
		return domain.ErrInvalidSubscriptionState
	}
	if s.provider == nil {
		// No provider wired (pre-#250 construction): refuse before any
		// payment is planned, so no pending row is left behind.
		return ErrPaymentUnavailable
	}
	return nil
}

// scheduleDeferredDowngrade schedules a downgrade for the end of the paid
// period with auto-renew on, so the new tariff renews on the normal cycle
// (ADR 0008 §3), recording the transition and its audit entry.
func scheduleDeferredDowngrade(
	ctx context.Context, stores *txStores, userID uuid.UUID, sub domain.Subscription,
	currentTariff, newTariff domain.Tariff, period domain.SubscriptionPeriod,
) error {
	// A deferred change needs a paid period to defer to; the domain
	// rejects the same condition, but the valid_until read below must not
	// dereference a nil pointer first.
	if sub.ValidUntil == nil {
		return domain.ErrInvalidTariffChange
	}
	_, err := stores.applyTransition(ctx, &sub,
		func(s *domain.Subscription) error {
			return s.ScheduleDowngrade(currentTariff, newTariff, period, *s.ValidUntil)
		},
		transitionSpec{
			reason:            domain.TransitionReasonDowngradeScheduled,
			initiator:         domain.InitiatorUser,
			initiatorID:       &userID,
			scheduledTariffID: &newTariff.ID,
			auditAction:       auditdomain.ActionSubscriptionTariffChanged,
			auditContext: func(_ domain.Subscription, transition domain.Transition) map[string]any {
				return map[string]any{auditKeyFromTariffID: *transition.FromTariffID, auditKeyToTariffID: transition.ToTariffID}
			},
		},
	)
	return err
}

// executePlannedPayment runs the provider initiation planned by the planning
// transaction. It runs outside it: no row locks are held across the external
// provider call.
func (s *SubscriptionService) executePlannedPayment(ctx context.Context, plan changeTariffPlan) (ChangeTariffResult, error) {
	payment := plan.createdPayment
	if plan.existingPayment != nil {
		if plan.existingPayment.HasProviderReference() && plan.existingPayment.HasPaymentURL() {
			return ChangeTariffResult{PaymentID: plan.existingPayment.ID, ConfirmURL: *plan.existingPayment.PaymentURL}, nil
		}
		// A previous initiation crashed before its provider reference was
		// persisted. Re-run the idempotent provider initiation and save the
		// reference, keeping the retry path idempotent.
		payment = plan.existingPayment
	}
	return s.initiatePaymentAtProvider(ctx, *payment, plan.paymentTariff, plan.paymentPurpose)
}

// planPayment is the transactional half of the payment path: it returns an
// existing live pending payment for the same tariff and period when there is
// one (no duplicate initiation), expires one whose form deadline has run out
// — the server-side expiry is the truth (issue #616) — or persists a fresh
// pending payment: the durable record of the user's decision that survives a
// crash before the provider call, with its form deadline anchored at creation
// so the same absolute instant is passed to the provider. Concurrency is
// guarded in two layers: the subscription row lock serializes initiations
// across different targets, while the pending-payments partial unique index
// (same user, tariff, period) durably backstops same-target duplicates.
func (s *SubscriptionService) planPayment(
	ctx context.Context,
	stores *txStores,
	sub domain.Subscription,
	currentTariff, newTariff domain.Tariff,
	period domain.SubscriptionPeriod,
) (created, existing *domain.SubscriptionPayment, err error) {
	now := s.clock.Now().UTC()
	if pending := findPendingPayment(stores, ctx, sub.UserID, newTariff.ID, period); pending != nil {
		if pending.IsLivePending(now) {
			return nil, pending, nil
		}
		// The pending payment's form deadline ran out while still pending:
		// fail it in this transaction and start fresh.
		if err := expireDeadPendingPayment(ctx, stores, pending, now); err != nil {
			return nil, nil, err
		}
	}

	amount, err := newTariff.Price(period)
	if err != nil {
		return nil, nil, err
	}
	payment, err := domain.NewSubscriptionPayment(sub.UserID, sub.ID, newTariff.ID, period, amount, s.provider.Name(), now)
	if err != nil {
		return nil, nil, err
	}
	if err := payment.AttachFormDeadline(now.Add(s.config.PaymentFormTTL), now); err != nil {
		return nil, nil, err
	}
	payment, err = stores.payments.Create(ctx, payment)
	if err != nil {
		if !errors.Is(err, ErrAlreadyExists) {
			return nil, nil, fmt.Errorf("save pending payment: %w", err)
		}
		// A concurrent request created the pending payment first; return it
		// instead of failing.
		if existing := findPendingPayment(stores, ctx, sub.UserID, newTariff.ID, period); existing != nil {
			return nil, existing, nil
		}
		return nil, nil, fmt.Errorf("pending payment lost after unique-race: %w", err)
	}

	// The pending payment this transaction commits is the persisted form of
	// the user's tariff-change decision, so the tariff change is audited
	// here; the payment itself is audited by the flow that finalizes it.
	if err := stores.audit.Record(ctx, auditdomain.Entry{
		ActorID:    &sub.UserID,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionSubscriptionTariffChanged,
		EntityType: auditdomain.EntitySubscription,
		EntityID:   &sub.ID,
		Context:    map[string]any{auditKeyFromTariffID: currentTariff.ID, auditKeyToTariffID: newTariff.ID, auditKeyPaymentID: payment.ID},
	}); err != nil {
		return nil, nil, fmt.Errorf("record audit: %w", err)
	}
	return &payment, nil, nil
}

// expireDeadPendingPayment finalizes a pending payment whose form deadline
// ran out (issue #616) — the server-side expiry is the truth — inside the
// caller's transaction. Shared by the tariff-change planner and the renewal
// planner, which both meet dead pendings when reusing the same tariff and
// period.
func expireDeadPendingPayment(
	ctx context.Context, stores *txStores, payment *domain.SubscriptionPayment, now time.Time,
) error {
	if err := payment.MarkExpired(now); err != nil {
		return err
	}
	if err := stores.payments.Update(ctx, *payment); err != nil {
		return fmt.Errorf("expire dead pending payment: %w", err)
	}
	return nil
}

// findPendingPayment returns the user's pending payment for the given tariff
// and period, or nil. Both the pre-create deduplication lookup and the
// unique-race backstop resolve through it.
func findPendingPayment(
	stores *txStores, ctx context.Context, userID, tariffID uuid.UUID, period domain.SubscriptionPeriod,
) *domain.SubscriptionPayment {
	pending, err := stores.payments.ListPendingByUserID(ctx, userID)
	if err != nil {
		return nil // The caller's Create path surfaces real repository errors.
	}
	for i := range pending {
		if pending[i].TariffID == tariffID && pending[i].Period == period {
			return &pending[i]
		}
	}
	return nil
}

// initiatePaymentAtProvider initiates the payment at the provider — fresh or
// as the recovery of a crashed initiation, the provider's idempotent Init
// with the same internal payment id covers both — and atomically persists
// the initiation result. The pending row already exists, so a crash at any
// point leaves a recoverable state.
func (s *SubscriptionService) initiatePaymentAtProvider(
	ctx context.Context,
	payment domain.SubscriptionPayment,
	tariff domain.Tariff,
	purpose PaymentPurposeKind,
) (ChangeTariffResult, error) {
	initRes, err := s.provider.InitPayment(ctx, s.initPaymentRequest(payment, tariff, purpose))
	if err != nil {
		s.markPaymentFailedBestEffort(ctx, payment.ID)
		return ChangeTariffResult{}, fmt.Errorf("init payment at provider: %w", err)
	}
	// The CIT path of the shared reference save (issue #285): the initiation
	// result carries the payer's form URL, and the answer to the user is the
	// persisted payment's URL — the saved state, never the raw response.
	saved, err := saveProviderReference(ctx, s.runInTx, payment.ID, initRes, s.clock.Now().UTC())
	if err != nil {
		return ChangeTariffResult{}, err
	}
	result := ChangeTariffResult{PaymentID: saved.ID}
	if saved.HasPaymentURL() {
		result.ConfirmURL = *saved.PaymentURL
	}
	return result, nil
}

// initPaymentRequest builds the provider-neutral initiation request shared by
// the fresh-initiation and recovery paths: a customer-initiated payment that
// saves its method for later merchant-initiated charges. The form deadline is
// the payment's persisted expires_at (issue #616) — the same absolute instant
// the user-facing countdown and the TTL worker read — so a recovered
// initiation keeps the original deadline instead of extending it; payments
// without a persisted deadline (legacy rows) fall back to the module TTL.
func (s *SubscriptionService) initPaymentRequest(
	payment domain.SubscriptionPayment, tariff domain.Tariff, purpose PaymentPurposeKind,
) InitPaymentRequest {
	deadline := s.clock.Now().UTC().Add(s.config.PaymentFormTTL)
	if payment.ExpiresAt != nil {
		deadline = *payment.ExpiresAt
	}
	return InitPaymentRequest{
		PaymentID:     payment.ID,
		AmountKopecks: payment.AmountKopecks,
		Period:        payment.Period,
		CustomerRef:   payment.UserID.String(),
		Purpose: PaymentPurpose{
			Kind:       purpose,
			TariffName: tariff.Name,
			Period:     payment.Period,
		},
		SaveMethod:   true,
		Initiator:    InitiatorCustomer,
		FormDeadline: deadline,
	}
}

// markPaymentFailedBestEffort closes a pending payment as failed when the
// provider refused the initiation, in its own short transaction, with the
// same audit entry the webhook failure path records. It only logs errors: by
// the time it runs the initiation has already failed, and a stuck pending
// row is recoverable by the reconciliation worker (issue #252).
func (s *SubscriptionService) markPaymentFailedBestEffort(ctx context.Context, paymentID uuid.UUID) {
	err := s.runInTx(ctx, func(stores *txStores) error {
		payment, err := stores.paymentForUpdate(ctx, paymentID)
		if err != nil {
			return err
		}
		if payment.Status != domain.PaymentStatusPending {
			return nil
		}
		if err := payment.MarkFailed(nil, s.clock.Now().UTC()); err != nil {
			return err
		}
		if err := stores.payments.Update(ctx, payment); err != nil {
			return err
		}
		return stores.audit.Record(ctx, auditdomain.Entry{
			ActorRole:  auditdomain.ActorRoleSystem,
			Action:     auditdomain.ActionSubscriptionPaymentFailed,
			EntityType: auditdomain.EntitySubscriptionPayment,
			EntityID:   &payment.ID,
			Context:    map[string]any{auditKeyPaymentID: payment.ID, auditKeyProvider: string(s.provider.Name())},
		})
	})
	if err != nil {
		s.log.ErrorContext(ctx, "failed to mark payment failed after provider init error",
			slog.String(auditKeyPaymentID, paymentID.String()),
			slog.String("error", sanitize.Error(err)))
	}
}
