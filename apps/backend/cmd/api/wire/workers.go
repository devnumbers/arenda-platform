package wire

import (
	"context"
	"sync"

	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	identitypg "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/postgres"
	identityscheduler "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/scheduler"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/scheduler"
)

// Compile-time checks that the billing worker phases satisfy the scheduler's
// consumer-side ports (ADR 0035). The phases are the issue #252 lifecycle
// workers; conformance is checked here, at the wiring site.
var (
	_ scheduler.ScheduledChangeProcessor = (*billingapp.Workers)(nil)
	_ scheduler.RenewalProcessor         = (*billingapp.Workers)(nil)
	_ scheduler.PaymentReconciler        = (*billingapp.Workers)(nil)
)

// Workers bundles the three background workers and exposes Wait (block until
// they exit). NewWorkers builds the identity data cleaner, the billing worker,
// the payment reconciliation worker and starts all three goroutines.
type Workers struct {
	wg sync.WaitGroup
}

// NewWorkers builds and starts the three background workers: the identity data
// cleaner, the billing worker and the payment reconciliation worker. It must be
// called with the still-active request context so the workers shut down when
// cancellation propagates.
func NewWorkers(
	ctx context.Context,
	p platformDeps,
	sessionRepo *identitypg.SessionRepository,
	codeRepo *identitypg.LoginCodeRepository,
	attemptRepo *identitypg.AttemptRepository,
	billingWorkers *billingapp.Workers,
) *Workers {
	billingWorker := scheduler.NewBillingWorker(
		billingWorkers, billingWorkers, billingWorkers,
		p.Pool, p.Clock, p.Cfg.BillingWorkerInterval, p.Logger)
	paymentReconciliationWorker := scheduler.NewPaymentReconciliationWorker(
		billingWorkers, p.Pool, p.Clock, p.Cfg.PaymentReconciliationWorkerInterval, p.Logger)

	dataCleaner := identityscheduler.NewCleaner(
		sessionRepo, codeRepo, attemptRepo, p.Clock, p.Cfg.IdentityCleanerInterval, p.Cfg.IdentityCleanerRetention, p.Logger)

	w := &Workers{}
	w.wg.Add(3)
	go func() { defer w.wg.Done(); dataCleaner.Run(ctx) }()
	go func() { defer w.wg.Done(); billingWorker.Run(ctx) }()
	go func() { defer w.wg.Done(); paymentReconciliationWorker.Run(ctx) }()

	return w
}

// Wait blocks until all three worker goroutines have exited.
func (w *Workers) Wait() { w.wg.Wait() }
