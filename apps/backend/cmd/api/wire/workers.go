package wire

import (
	"context"
	"log/slog"
	"sync"

	"github.com/jackc/pgx/v5"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	identitypg "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/postgres"
	identityscheduler "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/scheduler"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
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
	_ scheduler.NotificationsScanRunner  = (*notificationsapp.ScanGroup)(nil)
)

// Workers bundles the six background workers and exposes Wait (block until
// they exit). NewWorkers builds the identity data cleaner, the billing
// worker, the payment reconciliation worker, the payments tick worker, the
// tasks tick worker, the notifications scan worker and starts all six
// goroutines.
type Workers struct {
	wg sync.WaitGroup
	// Billing is the billing worker shell the time-travel rig's admin tick
	// endpoint triggers (issue #665): the same leader-elected pass the loop
	// below runs on its interval. Exposed so the HTTP wiring can mount the
	// tick route only when the rig is enabled.
	Billing *scheduler.BillingWorker
}

// NewWorkers builds and starts the six background workers: the identity
// data cleaner, the billing worker, the payment reconciliation worker, the
// payments tick worker, the tasks tick worker and the notifications scan
// worker (карта #734, #748–#749). A non-nil riverClient adds
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
	notificationsScan *notificationsapp.ScanGroup,
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
	// The scan cadence is the ticks' hourly domain decision (ADR 0048 p.3 —
	// idempotent between the zones' midnights through the dedup key),
	// deliberately not an operational knob: interval 0 is the shell's hour.
	notificationsScanWorker := scheduler.NewNotificationsScanWorker(
		notificationsScan, p.Pool, p.Clock, 0, p.Logger)

	dataCleaner := identityscheduler.NewCleaner(
		sessionRepo, codeRepo, attemptRepo, p.Clock, p.Cfg.IdentityCleanerInterval, p.Cfg.IdentityCleanerRetention, p.Logger)

	w := &Workers{Billing: billingWorker}
	goroutines := 6
	if riverClient != nil {
		goroutines++
	}
	w.wg.Add(goroutines)
	go func() { defer w.wg.Done(); dataCleaner.Run(ctx) }()
	go func() { defer w.wg.Done(); billingWorker.Run(ctx) }()
	go func() { defer w.wg.Done(); paymentReconciliationWorker.Run(ctx) }()
	go func() { defer w.wg.Done(); paymentsTickWorker.Run(ctx) }()
	go func() { defer w.wg.Done(); tasksTickWorker.Run(ctx) }()
	go func() { defer w.wg.Done(); notificationsScanWorker.Run(ctx) }()
	if riverClient != nil {
		go func() {
			defer w.wg.Done()
			// Start returns immediately (the client runs in its own
			// goroutines; the only error it can return is a startup failure
			// — logged, matching the scheduler loops' log-and-serve
			// behaviour). Cancelling the lifecycle ctx begins the soft stop
			// (SoftStopTimeout); Stopped() closes only when it has fully
			// finished, so Workers.Wait keeps covering in-flight delivery
			// jobs (research #735 §3).
			if err := riverClient.Start(ctx); err != nil {
				p.Logger.ErrorContext(ctx, "delivery queue client stopped with error", slog.String("error", err.Error()))
			}
			<-riverClient.Stopped()
		}()
	}

	return w
}

// Wait blocks until all six worker goroutines have exited.
func (w *Workers) Wait() { w.wg.Wait() }
