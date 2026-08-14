package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// SubscriptionService serves the user's view of and lifecycle control over
// their own subscription.
type SubscriptionService struct {
	txStoreFactory
	clock clock.Clock
}

// SubscriptionServiceConfig carries the non-transactional dependencies of the
// subscription service.
type SubscriptionServiceConfig struct {
	Clock clock.Clock
}

// NewSubscriptionService creates a subscription service over the shared
// factory. A nil clock defaults to the real clock, matching the identity
// module's constructor conventions.
func NewSubscriptionService(factory txStoreFactory, cfg SubscriptionServiceConfig) *SubscriptionService {
	if cfg.Clock == nil {
		cfg.Clock = clock.Real{}
	}
	return &SubscriptionService{txStoreFactory: factory, clock: cfg.Clock}
}

// GetSubscription assembles the user's subscription view: the subscription
// aggregate with its tariff and any scheduled (pending) tariff resolved.
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
				return SubscriptionView{}, fmt.Errorf("subscription %s references missing pending tariff %s: %w", sub.ID, *sub.PendingTariffID, ErrTariffNotFound)
			}
			return SubscriptionView{}, fmt.Errorf("get pending tariff: %w", err)
		}
		view.PendingTariff = &pending
	}
	return view, nil
}

// CancelSubscription cancels the user's subscription (issue #249, ADR 0008):
// the status moves to cancelled, auto-renew switches off immediately, and the
// paid tariff keeps working until valid_until — data mutations stay allowed
// for the rest of the period. The state change, its transition-log entry and
// the audit record land in one transaction.
func (s *SubscriptionService) CancelSubscription(ctx context.Context, userID uuid.UUID) error {
	return s.runInTx(ctx, func(stores *txStores) error {
		sub, err := stores.subscriptionForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		// Service subscriptions are assigned and withdrawn by an admin (#255);
		// the user cannot cancel what they do not pay for.
		if !sub.IsPaidSource() {
			return domain.ErrInvalidSubscriptionState
		}

		fromStatus := sub.Status
		fromTariffID := sub.TariffID
		if err := sub.Cancel(); err != nil {
			return err
		}
		if err := stores.subscriptions.Update(ctx, sub); err != nil {
			return fmt.Errorf("update subscription: %w", err)
		}

		transition, err := domain.NewTransition(sub, &fromStatus, &fromTariffID, domain.TransitionReasonCancelled, domain.InitiatorUser, &userID)
		if err != nil {
			return fmt.Errorf("build cancellation transition: %w", err)
		}
		if err := stores.transitions.Append(ctx, transition); err != nil {
			return fmt.Errorf("append cancellation transition: %w", err)
		}
		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &userID,
			ActorRole:  auditdomain.ActorRoleOwner,
			Action:     auditdomain.ActionSubscriptionCancelled,
			EntityType: auditdomain.EntitySubscription,
			EntityID:   &sub.ID,
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
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
// renewals need a payment, which answers with the explicit temporary
// ErrPaymentUnavailable until the payment flow lands (issue #250).
func (s *SubscriptionService) ChangeTariff(ctx context.Context, userID uuid.UUID, req ChangeTariffRequest) (ChangeTariffResult, error) {
	var result ChangeTariffResult
	err := s.runInTx(ctx, func(stores *txStores) error {
		newTariff, err := stores.tariffs.GetByName(ctx, req.TariffName)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrTariffNotFound
			}
			return fmt.Errorf("get tariff: %w", err)
		}

		sub, err := stores.subscriptionForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		if !sub.IsPaidSource() {
			return domain.ErrInvalidSubscriptionState
		}

		now := s.clock.Now().UTC()
		// A same-tariff request is a manual renewal while the subscription is
		// in grace; it shares the payment path with upgrades (issue #250).
		needsPayment := false
		if sub.TariffID == newTariff.ID {
			switch {
			case sub.IsInGrace(now):
				needsPayment = true
			case sub.Status == domain.SubscriptionStatusGrace:
				// The grace window has expired; the worker downgrade to basic
				// is due and no payment can be initiated anymore.
				return domain.ErrInvalidSubscriptionState
			default:
				return domain.ErrAlreadyOnTariff
			}
		}

		currentTariff, err := stores.tariffs.GetByID(ctx, sub.TariffID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return fmt.Errorf("subscription %s references missing tariff %s: %w", sub.ID, sub.TariffID, ErrTariffNotFound)
			}
			return fmt.Errorf("get current tariff: %w", err)
		}
		if !needsPayment && domain.ClassifyTariffChange(currentTariff, newTariff) == domain.TariffChangeUpgrade {
			needsPayment = true
		}
		if needsPayment {
			return ErrPaymentUnavailable
		}
		// A deferred change needs a paid period to defer to; the domain
		// rejects the same condition, but the valid_until read below must not
		// dereference a nil pointer first.
		if sub.ValidUntil == nil {
			return domain.ErrInvalidTariffChange
		}
		if err := sub.ScheduleDowngrade(currentTariff, newTariff, req.Period, *sub.ValidUntil); err != nil {
			return err
		}
		if err := stores.subscriptions.Update(ctx, sub); err != nil {
			return fmt.Errorf("schedule downgrade: %w", err)
		}

		transition, err := domain.NewScheduledTariffTransition(sub, newTariff.ID, domain.TransitionReasonDowngradeScheduled, domain.InitiatorUser, &userID)
		if err != nil {
			return fmt.Errorf("build downgrade-scheduling transition: %w", err)
		}
		if err := stores.transitions.Append(ctx, transition); err != nil {
			return fmt.Errorf("append downgrade-scheduling transition: %w", err)
		}
		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &userID,
			ActorRole:  auditdomain.ActorRoleOwner,
			Action:     auditdomain.ActionSubscriptionTariffChanged,
			EntityType: auditdomain.EntitySubscription,
			EntityID:   &sub.ID,
			Context:    map[string]any{"from_tariff_id": currentTariff.ID, "to_tariff_id": newTariff.ID},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return ChangeTariffResult{}, err
	}
	return result, nil
}
