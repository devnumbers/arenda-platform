package wire

import (
	"context"
	"log/slog"
	"sync"

	"github.com/jackc/pgx/v5"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	identitypg "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/postgres"
	identityscheduler "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/scheduler"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/scheduler"
	tasksapp "github.com/nambers/arenda-planform/apps/backend/internal/tasks/application"
	"github.com/riverqueue/river"
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
	// Billing is the billing worker shell the time-travel rig's admin tick
	// endpoint triggers (issue #665): the same leader-elected pass the loop
	// below runs on its interval. Exposed so the HTTP wiring can mount the
	// tick route only when the rig is enabled.
	Billing *scheduler.BillingWorker
}

// NewWorkers builds and starts the five background workers: the identity
// data cleaner, the billing worker, the payment reconciliation worker, the
// payments tick worker and the tasks tick worker. A non-nil riverClient adds
// the delivery queue (карта #734, #740): Start blocks until the client has
// fully stopped — cancelling the lifecycle context begins the soft stop
// (SoftStopTimeout), so Workers.Wait also waits for in-flight delivery jobs.
// It must be called with the still-active request context so the workers
// shut down when cancellation propagates.
func NewWorkers(
	ctx context.Context,
	p platformDeps,
	sessionRepo *identitypg.SessionRepository,
	codeRepo *identitypg.LoginCodeRepository,
	attemptRepo *identitypg.AttemptRepository,
	billingWorkers *billingapp.Workers,
	paymentsTick *paymentsapp.TickService,
	tasksTick *tasksapp.TickService,
	riverClient *river.Client[pgx.Tx],
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

	w := &Workers{Billing: billingWorker}
	goroutines := 5
	if riverClient != nil {
		goroutines++
	}
	w.wg.Add(goroutines)
	go func() { defer w.wg.Done(); dataCleaner.Run(ctx) }()
	go func() { defer w.wg.Done(); billingWorker.Run(ctx) }()
	go func() { defer w.wg.Done(); paymentReconciliationWorker.Run(ctx) }()
	go func() { defer w.wg.Done(); paymentsTickWorker.Run(ctx) }()
	go func() { defer w.wg.Done(); tasksTickWorker.Run(ctx) }()
	if riverClient != nil {
		go func() {
			defer w.wg.Done()
			// Start blocks until the client has fully stopped; the only
			// error it can return is a startup failure (the database is
			// unreachable) — logged, matching the scheduler loops'
			// log-and-serve behaviour.
			if err := riverClient.Start(ctx); err != nil {
				p.Logger.ErrorContext(ctx, "delivery queue client stopped with error", slog.String("error", err.Error()))
			}
		}()
	}

	return w
}

// Wait blocks until all five worker goroutines have exited.
func (w *Workers) Wait() { w.wg.Wait() }
