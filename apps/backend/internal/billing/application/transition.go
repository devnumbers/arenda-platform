package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// transitionSpec names one subscription state change for applyTransition:
// why it happened, who caused it, the payment that caused it when there was
// one, and the audit entry that describes it. The from-side (the prior status
// and tariff) is deliberately not a field — applyTransition captures it
// itself, so the log entry cannot disagree with what the aggregate looked
// like before the mutation.
type transitionSpec struct {
	reason      domain.TransitionReason
	initiator   domain.TransitionInitiator
	initiatorID *uuid.UUID
	// paymentID references the subscription payment that caused the
	// transition; nil for the flows that change no payment.
	paymentID *uuid.UUID
	// scheduledTariffID marks a deferred tariff change (issue #249 downgrade
	// planning): the log entry records the future target as its to-side while
	// the aggregate keeps its current tariff.
	scheduledTariffID *uuid.UUID
	// auditAction names the audit entry recorded with the transition. A zero
	// action records none: the worker phases and the payment applications
	// audit their payments, not the subscription change itself.
	auditAction auditdomain.Action
	// auditContext, when set, builds the audit entry's context from the
	// post-change subscription and the appended transition — some entries
	// quote state only one of the two knows (the extended grace deadline, the
	// captured from-tariff). The transition is always freshly captured by
	// applyTransition, so its from-side is present for every update
	// transition; only the zero transition of an idempotent no-op carries a
	// nil from-side, and a no-op records no audit entry.
	auditContext func(sub domain.Subscription, transition domain.Transition) map[string]any
}

// applyTransition is the single seam every subscription state change goes
// through (issue #282): it captures the from-side, runs the mutation,
// persists the aggregate, appends the transition-log entry and — when the
// spec names an audit action — records the audit entry attributed to the
// transition's initiator. Everything runs inside the caller's transaction, so
// the invariant "no subscription state change without a transition-log entry"
// holds by construction instead of call-site discipline. It returns the
// appended transition so callers can branch on the captured from-side (the
// tariff-limit enforcement applies only when the tariff actually changed).
func (s *txStores) applyTransition(
	ctx context.Context,
	sub *domain.Subscription,
	mutate func(*domain.Subscription) error,
	spec transitionSpec,
) (domain.Transition, error) {
	if err := spec.validate(); err != nil {
		return domain.Transition{}, err
	}
	fromStatus := sub.Status
	fromTariffID := sub.TariffID
	if err := mutate(sub); err != nil {
		return domain.Transition{}, err
	}
	if err := s.subscriptions.Update(ctx, *sub); err != nil {
		return domain.Transition{}, fmt.Errorf("update subscription: %w", err)
	}
	transition, err := s.buildTransition(sub, &fromStatus, &fromTariffID, spec)
	if err != nil {
		return domain.Transition{}, err
	}
	if err := s.transitions.Append(ctx, transition); err != nil {
		return domain.Transition{}, fmt.Errorf("append transition: %w", err)
	}
	if spec.auditAction != "" {
		entry := auditdomain.Entry{
			ActorID:    spec.initiatorID,
			ActorRole:  auditRoleOfInitiator(spec.initiator),
			Action:     spec.auditAction,
			EntityType: auditdomain.EntitySubscription,
			EntityID:   &sub.ID,
		}
		if spec.auditContext != nil {
			entry.Context = spec.auditContext(*sub, transition)
		}
		if err := s.audit.Record(ctx, entry); err != nil {
			return domain.Transition{}, fmt.Errorf("record audit: %w", err)
		}
	}
	return transition, nil
}

// buildTransition assembles the log entry through the domain constructors,
// which own the construction rules: a scheduled change logs its future
// target, a payment-caused change references its payment, everything else
// records the applied state. The payment-referencing combinations were
// validated upfront by validate.
func (s *txStores) buildTransition(
	sub *domain.Subscription,
	fromStatus *domain.SubscriptionStatus,
	fromTariffID *uuid.UUID,
	spec transitionSpec,
) (domain.Transition, error) {
	switch {
	case spec.scheduledTariffID != nil:
		return domain.NewScheduledTariffTransition(*sub, *spec.scheduledTariffID, spec.reason, spec.initiator, spec.initiatorID)
	case spec.paymentID != nil && spec.reason == domain.TransitionReasonPaymentApplied:
		return domain.NewAppliedPaymentTransition(*sub, fromStatus, fromTariffID, *spec.paymentID)
	case spec.paymentID != nil && spec.reason == domain.TransitionReasonRefunded:
		return domain.NewRefundTransition(*sub, fromStatus, fromTariffID, spec.initiator, spec.initiatorID, *spec.paymentID)
	default:
		return domain.NewTransition(*sub, fromStatus, fromTariffID, spec.reason, spec.initiator, spec.initiatorID)
	}
}

// validate rejects the spec combinations the domain constructors would
// silently flatten: a payment reference belongs only to an applied payment
// (always system-initiated — the provider webhook applies it) or a refund
// (admin or system). A mix-up fails before anything is written.
func (spec transitionSpec) validate() error {
	if spec.paymentID == nil {
		return nil
	}
	switch spec.reason {
	case domain.TransitionReasonPaymentApplied:
		if spec.initiator != domain.InitiatorSystem {
			return fmt.Errorf("transition spec: reason %q is always system-initiated, got initiator %q", spec.reason, spec.initiator)
		}
		return nil
	case domain.TransitionReasonRefunded:
		return nil
	default:
		return fmt.Errorf("transition spec: paymentID requires reason %q or %q, got %q", domain.TransitionReasonPaymentApplied, domain.TransitionReasonRefunded, spec.reason)
	}
}

// auditRoleOfInitiator maps a transition initiator to the audit actor role
// that records its actions: the user audits as the subscription owner, the
// admin as the platform admin, the system as the system itself.
func auditRoleOfInitiator(initiator domain.TransitionInitiator) auditdomain.ActorRole {
	switch initiator {
	case domain.InitiatorUser:
		return auditdomain.ActorRoleOwner
	case domain.InitiatorAdmin:
		return auditdomain.ActorRoleAdmin
	default:
		return auditdomain.ActorRoleSystem
	}
}

// transitionChangedTariff reports whether the transition moved the
// subscription off the given tariff — the guard of every tariff-limit
// enforcement that follows a transition. A transition without a from-side
// (the zero value a no-op returns, a creation entry) reports false: there is
// no prior tariff the limit could have moved from.
func transitionChangedTariff(transition domain.Transition, tariffID uuid.UUID) bool {
	return transition.FromTariffID != nil && *transition.FromTariffID != tariffID
}
