package application

import (
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// phaseScheduler computes the time phases of the billing workers (issue #421):
// the renewal onset, the grace phases, the notification lead window and the
// reconciliation staleness windows. It is the workers' single seam for time:
// built on the injected clock, it holds no path to the system clock, so every
// phase boundary is testable on deterministic time (phases_test.go), and the
// dunning retry schedule of spec #419 (+24/+72 h inside grace) extends this
// component instead of the workers.
//
// One worker tick owns one time snapshot: the scheduling shell reads it once
// and the phases thread it through their listing and the under-lock re-check,
// so a row cannot drift across a boundary between being listed and being
// locked. The selection methods therefore take the tick snapshot explicitly;
// the injected clock serves the scheduler's own reads (now) outside a tick —
// the seam the dunning phases will plan by.
type phaseScheduler struct {
	clock  clock.Clock
	config Config
}

// newPhaseScheduler builds the scheduler over the injected clock. A nil clock
// defaults to the real clock, matching the module's constructor conventions;
// NewWorkers passes its configured clock through.
func newPhaseScheduler(clk clock.Clock, cfg Config) *phaseScheduler {
	if clk == nil {
		clk = clock.Real{}
	}
	return &phaseScheduler{clock: clk, config: cfg}
}

// now is the scheduler's clock read — the only time source the workers reach
// outside the tick snapshot handed to them.
func (s *phaseScheduler) now() time.Time {
	return s.clock.Now().UTC()
}

// The phases' selections (issue #286): each phase describes its batch once as
// values, next to the phase itself. The listing and the under-lock re-check
// share the constructor, and the predicate behind the values lives once in
// SQL — a new phase is a new constructor, never a new port method.

// renewalDue is the renewal-charge batch: active auto-renewing subscriptions
// whose paid period has ended (issue #252).
func (s *phaseScheduler) renewalDue(now time.Time, limit int) SubscriptionSelection {
	return SubscriptionSelection{
		Status:           domain.SubscriptionStatusActive,
		AutoRenewEnabled: new(true),
		ValidUntilBefore: new(now),
		Limit:            limit,
	}
}

// graceExpired is the downgrade-to-basic batch: grace subscriptions whose
// grace window has ended (issue #252).
func (s *phaseScheduler) graceExpired(now time.Time, limit int) SubscriptionSelection {
	return SubscriptionSelection{
		Status:           domain.SubscriptionStatusGrace,
		ValidUntilBefore: new(now),
		Limit:            limit,
	}
}

// graceReminderWindow is the grace-expiry reminder batch: grace subscriptions
// inside the half-open window [valid_until - lead, valid_until) whose window
// was not reminded yet (issue #253); the lead comes from the config.
func (s *phaseScheduler) graceReminderWindow(now time.Time, limit int) SubscriptionSelection {
	return SubscriptionSelection{
		Status:           domain.SubscriptionStatusGrace,
		ValidUntilAfter:  new(now),
		ValidUntilBefore: new(now.Add(s.config.GraceExpiryReminderBefore)),
		Unreminded:       true,
		Limit:            limit,
	}
}

// nonRenewingExpired is the expiry batch of active subscriptions with
// auto-renew off whose retained period has ended (issue #252).
func (s *phaseScheduler) nonRenewingExpired(now time.Time, limit int) SubscriptionSelection {
	return SubscriptionSelection{
		Status:           domain.SubscriptionStatusActive,
		AutoRenewEnabled: new(false),
		ValidUntilBefore: new(now),
		Limit:            limit,
	}
}

// cancelledExpired is the expiry batch of cancelled subscriptions whose
// retained period has ended (issue #252).
func (s *phaseScheduler) cancelledExpired(now time.Time, limit int) SubscriptionSelection {
	return SubscriptionSelection{
		Status:           domain.SubscriptionStatusCancelled,
		ValidUntilBefore: new(now),
		Limit:            limit,
	}
}

// pendingChangesDue is the batch of active subscriptions with a deferred
// tariff change that is due (issue #252).
func (s *phaseScheduler) pendingChangesDue(now time.Time, limit int) SubscriptionSelection {
	return SubscriptionSelection{
		Status:           domain.SubscriptionStatusActive,
		PendingChangeDue: new(now),
		Limit:            limit,
	}
}

// stalePending is the lost-webhook batch: pending payments with a provider
// reference unresolved past the configured staleness (issue #252).
func (s *phaseScheduler) stalePending(now time.Time, limit int) PaymentSelection {
	return PaymentSelection{
		Status:        domain.PaymentStatusPending,
		CreatedBefore: new(now.Add(-s.config.PendingPaymentStaleness)),
		Limit:         limit,
	}
}

// stalePendingUpgrades narrows the lost-webhook batch to the tariff-change
// payments of the ChangeTariff flow (issue #252).
func (s *phaseScheduler) stalePendingUpgrades(now time.Time, limit int) PaymentSelection {
	sel := s.stalePending(now, limit)
	sel.TariffChangeOnly = true
	return sel
}

// staleRefunding is the lost-outcome batch: payments stuck in the refunding
// reservation past the configured staleness (issue #254).
func (s *phaseScheduler) staleRefunding(now time.Time, limit int) PaymentSelection {
	return PaymentSelection{
		Status:        domain.PaymentStatusRefunding,
		UpdatedBefore: new(now.Add(-s.config.PendingPaymentStaleness)),
		Limit:         limit,
	}
}
