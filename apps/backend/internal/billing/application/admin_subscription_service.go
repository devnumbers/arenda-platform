package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// The admin subscription operations of issue #255: service-subscription
// assignment, force tariff change, manual grace extension, cancellation on the
// user's behalf and the transition-history view. Everything lives on
// SubscriptionService — the owner of subscription state transitions — beside
// the user flows it shares its seams with. Every operation lands the
// subscription change, its transition-log entry and the audit record in one
// transaction, attributed to the acting admin.

// Worker-slot audit triggers of the admin operations — the access coordinator
// records them in its audit context, so they are low-cardinality labels naming
// the operation that dropped a tariff limit.
const (
	triggerServiceAssigned = "service_assigned"
	triggerForcedChange    = "forced_change"
)

// SetLifecycleBridges wires the cross-context lifecycle bridges the admin
// operations that lower a tariff limit call in their transactions:
// excess-property archiving (properties context) and recipient-slot
// enforcement (access context). The composition root calls it with the same
// bridge sources the workers and the payment service use. Without bridges the
// operations still apply every subscription change; only the excess properties
// and suspended memberships wait for the next wired run.
func (s *SubscriptionService) SetLifecycleBridges(archiver ExcessPropertyArchiverSource, slots RecipientSlotEnforcerSource) {
	s.archiverSource = archiver
	s.slotSource = slots
}

// runLifecycleTx runs work like runInTx plus the lifecycle bridges bound to
// the transaction — the transaction shape of every admin operation that can
// lower a tariff limit.
func (s *SubscriptionService) runLifecycleTx(ctx context.Context, work func(*txStores) error) error {
	return s.runInTxWithBridges(ctx, s.archiverSource, s.slotSource, work)
}

// AssignServiceSubscription assigns a service subscription (issue #255,
// billing CONTEXT.md): the tariff runs for the requested fixed term without
// payment, auto-renew is off, and the assignment overwrites the current
// subscription — the paid remainder does not stack. At the end of the term the
// expiry worker downgrades the subscription to basic through the common
// expiry path, archiving excess properties. When the new tariff's limit is
// lower than the overwritten one's, the excess is archived in the same
// transaction.
func (s *SubscriptionService) AssignServiceSubscription(ctx context.Context, adminID, userID uuid.UUID, req AssignServiceSubscriptionRequest) error {
	validUntil, err := serviceTermValidUntil(req, s.clock.Now().UTC())
	if err != nil {
		return err
	}
	return s.runLifecycleTx(ctx, func(stores *txStores) error {
		tariff, err := stores.tariffs.GetByName(ctx, req.TariffName)
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
		fromStatus := sub.Status
		fromTariffID := sub.TariffID
		sub.AssignService(tariff.ID, validUntil)
		if err := stores.subscriptions.Update(ctx, sub); err != nil {
			return fmt.Errorf("assign service subscription: %w", err)
		}

		transition, err := domain.NewTransition(sub, &fromStatus, &fromTariffID, domain.TransitionReasonServiceAssigned, domain.InitiatorAdmin, &adminID)
		if err != nil {
			return fmt.Errorf("build service-assignment transition: %w", err)
		}
		if err := stores.transitions.Append(ctx, transition); err != nil {
			return fmt.Errorf("append service-assignment transition: %w", err)
		}

		if fromTariffID != tariff.ID {
			if err := stores.enforceTariffLimit(ctx, sub.UserID, tariff.ActivePropertyLimit, triggerServiceAssigned); err != nil {
				return fmt.Errorf("enforce tariff limit after service assignment: %w", err)
			}
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &adminID,
			ActorRole:  auditdomain.ActorRoleAdmin,
			Action:     auditdomain.ActionSubscriptionServiceAssigned,
			EntityType: auditdomain.EntitySubscription,
			EntityID:   &sub.ID,
			Context:    map[string]any{"tariff_name": string(tariff.Name), "valid_until": validUntil},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
}

// serviceTermValidUntil resolves the fixed term of a service subscription into
// its valid_until: a month or a year counts from now; an explicit date runs
// until the end of that UTC date, inclusive, and may not lie in the past.
func serviceTermValidUntil(req AssignServiceSubscriptionRequest, now time.Time) (time.Time, error) {
	switch req.TermType {
	case ServiceTermMonth:
		return now.AddDate(0, 1, 0), nil
	case ServiceTermYear:
		return now.AddDate(1, 0, 0), nil
	case ServiceTermDate:
		if req.UntilDate == nil {
			return time.Time{}, fmt.Errorf("%w: the date term requires untilDate", domain.ErrInvalidTerm)
		}
		date := req.UntilDate.UTC()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		if date.Before(today) {
			return time.Time{}, fmt.Errorf("%w: untilDate %s is in the past", domain.ErrInvalidTerm, date.Format(time.DateOnly))
		}
		return date.AddDate(0, 0, 1), nil
	default:
		return time.Time{}, fmt.Errorf("%w: unknown term type %q", domain.ErrInvalidTerm, req.TermType)
	}
}

// ForceChangeTariff switches the subscription's tariff immediately without
// payment (issue #255): the new tariff runs for the requested period from
// now, the subscription keeps its source and auto-renew setting, and any
// pending change is dropped. Excess properties beyond the new tariff's limit
// are archived in the same transaction. The incident-repair lever — an
// ordinary compensation without payment is a service subscription.
func (s *SubscriptionService) ForceChangeTariff(ctx context.Context, adminID, userID uuid.UUID, req ForceChangeTariffRequest) error {
	now := s.clock.Now().UTC()
	return s.runLifecycleTx(ctx, func(stores *txStores) error {
		tariff, err := stores.tariffs.GetByName(ctx, req.TariffName)
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
		fromStatus := sub.Status
		fromTariffID := sub.TariffID
		if err := sub.ForceApplyTariffChange(tariff.ID, req.Period, now); err != nil {
			return err
		}
		if err := stores.subscriptions.Update(ctx, sub); err != nil {
			return fmt.Errorf("force tariff change: %w", err)
		}

		transition, err := domain.NewTransition(sub, &fromStatus, &fromTariffID, domain.TransitionReasonForcedChange, domain.InitiatorAdmin, &adminID)
		if err != nil {
			return fmt.Errorf("build forced-change transition: %w", err)
		}
		if err := stores.transitions.Append(ctx, transition); err != nil {
			return fmt.Errorf("append forced-change transition: %w", err)
		}

		if err := stores.enforceTariffLimit(ctx, sub.UserID, tariff.ActivePropertyLimit, triggerForcedChange); err != nil {
			return fmt.Errorf("enforce tariff limit after forced change: %w", err)
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &adminID,
			ActorRole:  auditdomain.ActorRoleAdmin,
			Action:     auditdomain.ActionSubscriptionTariffForced,
			EntityType: auditdomain.EntitySubscription,
			EntityID:   &sub.ID,
			Context:    map[string]any{"from_tariff_id": fromTariffID, "to_tariff_id": tariff.ID},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
}

// ExtendGrace lengthens the subscription's grace window by whole days
// (issue #255): the manual lever that gives the user extra time to fix their
// payment method. The extended window is a fresh one for the grace-expiry
// reminder (#253).
func (s *SubscriptionService) ExtendGrace(ctx context.Context, adminID, userID uuid.UUID, days int) error {
	if days < 1 || days > s.config.MaxGraceExtensionDays {
		return fmt.Errorf("%w: days must be between 1 and %d", domain.ErrInvalidGraceExtension, s.config.MaxGraceExtensionDays)
	}
	now := s.clock.Now().UTC()
	extra := time.Duration(days) * 24 * time.Hour
	return s.runInTx(ctx, func(stores *txStores) error {
		sub, err := stores.subscriptionForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		fromStatus := sub.Status
		fromTariffID := sub.TariffID
		if err := sub.ExtendGrace(now, extra); err != nil {
			return err
		}
		if err := stores.subscriptions.Update(ctx, sub); err != nil {
			return fmt.Errorf("extend grace: %w", err)
		}

		transition, err := domain.NewTransition(sub, &fromStatus, &fromTariffID, domain.TransitionReasonGraceExtended, domain.InitiatorAdmin, &adminID)
		if err != nil {
			return fmt.Errorf("build grace-extension transition: %w", err)
		}
		if err := stores.transitions.Append(ctx, transition); err != nil {
			return fmt.Errorf("append grace-extension transition: %w", err)
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &adminID,
			ActorRole:  auditdomain.ActorRoleAdmin,
			Action:     auditdomain.ActionSubscriptionGraceExtended,
			EntityType: auditdomain.EntitySubscription,
			EntityID:   &sub.ID,
			Context:    map[string]any{"days": days, "valid_until": *sub.ValidUntil},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
}

// CancelSubscriptionAsAdmin cancels the subscription on the user's behalf
// (issue #255) with the same semantics as the user's own cancellation: the
// tariff keeps working until valid_until, auto-renew switches off immediately
// and any scheduled change is dropped. Paid subscriptions only — a service
// subscription is withdrawn by reassignment, not cancellation.
func (s *SubscriptionService) CancelSubscriptionAsAdmin(ctx context.Context, adminID, userID uuid.UUID) error {
	return s.runInTx(ctx, func(stores *txStores) error {
		sub, err := stores.subscriptionForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		if !sub.IsPaidSource() {
			return domain.ErrInvalidSubscriptionState
		}
		fromStatus := sub.Status
		fromTariffID := sub.TariffID
		if err := sub.Cancel(); err != nil {
			return err
		}
		if err := stores.subscriptions.Update(ctx, sub); err != nil {
			return fmt.Errorf("cancel subscription: %w", err)
		}

		transition, err := domain.NewTransition(sub, &fromStatus, &fromTariffID, domain.TransitionReasonCancelled, domain.InitiatorAdmin, &adminID)
		if err != nil {
			return fmt.Errorf("build cancellation transition: %w", err)
		}
		if err := stores.transitions.Append(ctx, transition); err != nil {
			return fmt.Errorf("append cancellation transition: %w", err)
		}
		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &adminID,
			ActorRole:  auditdomain.ActorRoleAdmin,
			Action:     auditdomain.ActionSubscriptionCancelled,
			EntityType: auditdomain.EntitySubscription,
			EntityID:   &sub.ID,
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
}

// ListTransitions returns the user's subscription transition history for the
// admin views (issue #255): every status and tariff change with its reason,
// initiator and payment reference, newest first, the tariff names resolved.
func (s *SubscriptionService) ListTransitions(ctx context.Context, userID uuid.UUID) ([]SubscriptionTransitionView, error) {
	sub, err := s.subscriptions.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, fmt.Errorf("get subscription: %w", err)
	}
	transitions, err := s.transitions.ListBySubscriptionID(ctx, sub.ID)
	if err != nil {
		return nil, fmt.Errorf("list transitions: %w", err)
	}
	tariffs, err := s.tariffs.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tariffs: %w", err)
	}
	names := make(map[uuid.UUID]string, len(tariffs))
	for _, t := range tariffs {
		names[t.ID] = string(t.Name)
	}
	views := make([]SubscriptionTransitionView, 0, len(transitions))
	for _, transition := range transitions {
		name, ok := names[transition.ToTariffID]
		if !ok {
			return nil, fmt.Errorf("transition %s references unknown tariff %s", transition.ID, transition.ToTariffID)
		}
		view := SubscriptionTransitionView{Transition: transition, ToTariffName: name}
		if transition.FromTariffID != nil {
			from, ok := names[*transition.FromTariffID]
			if !ok {
				return nil, fmt.Errorf("transition %s references unknown tariff %s", transition.ID, *transition.FromTariffID)
			}
			view.FromTariffName = &from
		}
		views = append(views, view)
	}
	return views, nil
}
