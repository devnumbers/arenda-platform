package wire

import (
	"context"
	"sync"

	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	identitypg "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/postgres"
	identityscheduler "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/scheduler"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/scheduler"
	tasksapp "github.com/nambers/arenda-planform/apps/backend/internal/tasks/application"
)

// Compile-time checks that the worker phases satisfy the scheduler's
// consumer-side ports (ADR 0035): the issue #252 billing lifecycle workers,
// the payments tick sweep of ticket #458 and the tasks tick sweep of ADR
// 0051. Conformance is checked here, at the wiring site.
var (
	_ scheduler.ScheduledChangeProcessor = (*billingapp.Workers)(nil)
	_ scheduler.RenewalProcessor         = (*billingapp.Workers)(nil)
	_ scheduler.PaymentReconciler        = (*billingapp.Workers)(nil)
	_ scheduler.PaymentsTicker           = (*paymentsapp.TickService)(nil)
)

// Workers bundles the five background workers and exposes Wait (block until
// they exit). NewWorkers builds the identity data cleaner, the billing
// worker, the payment reconciliation worker, the payments tick worker, the
// tasks tick worker and starts all five goroutines.
type Workers struct {
	wg sync.WaitGroup
}

// NewWorkers builds and starts the five background workers: the identity
// data cleaner, the billing worker, the payment reconciliation worker, the
// payments tick worker and the tasks tick worker. It must be called with the
// still-active request context so the workers shut down when cancellation
// propagates.
func NewWorkers(
	ctx context.Context,
	p platformDeps,
	sessionRepo *identitypg.SessionRepository,
	codeRepo *identitypg.LoginCodeRepository,
	attemptRepo *identitypg.AttemptRepository,
	billingWorkers *billingapp.Workers,
	paymentsTick *paymentsapp.TickService,
	tasksTick *tasksapp.TickService,
) *Workers {
	billingWorker := scheduler.NewBillingWorker(
		billingWorkers, billingWorkers, billingWorkers,
		p.Pool, p.Clock, p.Cfg.BillingWorkerInterval, p.Logger)
	paymentReconciliationWorker := scheduler.NewPaymentReconciliationWorker(
		billingWorkers, p.Pool, p.Clock, p.Cfg.PaymentReconciliationWorkerInterval, p.Logger)
	paymentsTickWorker := scheduler.NewPaymentsTickWorker(
		paymentsTick, p.Pool, p.Clock, p.Cfg.PaymentsTickWorkerInterval, p.Logger)
	tasksTickWorker := scheduler.NewTasksTickWorker(
		tasksTick.RunZoneTicks, p.Pool, p.Clock, p.Cfg.TasksTickWorkerInterval, p.Logger)

	dataCleaner := identityscheduler.NewCleaner(
		sessionRepo, codeRepo, attemptRepo, p.Clock, p.Cfg.IdentityCleanerInterval, p.Cfg.IdentityCleanerRetention, p.Logger)

	w := &Workers{}
	w.wg.Add(5)
	go func() { defer w.wg.Done(); dataCleaner.Run(ctx) }()
	go func() { defer w.wg.Done(); billingWorker.Run(ctx) }()
	go func() { defer w.wg.Done(); paymentReconciliationWorker.Run(ctx) }()
	go func() { defer w.wg.Done(); paymentsTickWorker.Run(ctx) }()
	go func() { defer w.wg.Done(); tasksTickWorker.Run(ctx) }()

	return w
}

// Wait blocks until all five worker goroutines have exited.
func (w *Workers) Wait() { w.wg.Wait() }
