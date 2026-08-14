package application

import (
	"context"
	"time"
)

// Workers reconnects the scheduler worker shells (BillingWorker,
// PaymentReconciliationWorker) to the rewritten billing module (issue #245).
// Every phase is a deliberate no-op for now: until the payment flows land
// (#250) and the lifecycle worker returns (#252), no flow in the module can
// create an expiring, auto-renewing or charging subscription, so there is
// genuinely nothing to process. The scheduler shells keep ticking under their
// advisory locks; the phases fill in with their tickets.
type Workers struct{}

// ProcessScheduledChanges applies due deferred tariff changes (issue #252).
func (Workers) ProcessScheduledChanges(context.Context, time.Time) (int, error) { return 0, nil }

// ProcessRenewals charges auto-renewals and expires non-renewing
// subscriptions (issue #252).
func (Workers) ProcessRenewals(context.Context, time.Time) (int, error) { return 0, nil }

// ProcessPendingUpgradePayments finalizes pending upgrade payments (issue #252).
func (Workers) ProcessPendingUpgradePayments(context.Context, time.Time) (int, error) {
	return 0, nil
}

// ProcessExpiredGrace downgrades subscriptions whose grace period ended
// (issue #252).
func (Workers) ProcessExpiredGrace(context.Context, time.Time) (int, error) { return 0, nil }

// ReconcilePendingPayments pulls lost payment webhooks from the provider
// (issue #252).
func (Workers) ReconcilePendingPayments(context.Context, time.Time) (int, error) { return 0, nil }

// ReconcileStaleRefunds resolves payments stuck in the refunding state
// (issue #252).
func (Workers) ReconcileStaleRefunds(context.Context, time.Time) (int, error) { return 0, nil }
