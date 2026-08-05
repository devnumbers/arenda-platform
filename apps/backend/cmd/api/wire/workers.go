package wire

import (
	"context"
	"sync"
	"time"

	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	identitypg "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/postgres"
	identityscheduler "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/scheduler"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	emailnotifier "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/email"
	notificationspg "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/postgres"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	platformgenerated "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/scheduler"
)

// Workers bundles the six background workers and exposes Wait (block until they
// exit). NewWorkers builds the notifier infrastructure (contact resolver, email
// notifier, notifiers map), the identity data cleaner, the six workers and
// starts all six goroutines. The goroutine pattern matches the original main.go
// exactly: a single WaitGroup with Add(6) and six goroutines each calling
// defer wg.Done().
type Workers struct {
	wg sync.WaitGroup
}

// NewWorkers builds and starts the six background workers: the identity data
// cleaner, the notifications reminder worker, the lease reconciliation worker,
// the billing worker, the payment reconciliation worker and the operation
// overdue worker. It must be called with the still-active request context so
// the workers shut down when cancellation propagates.
func NewWorkers(
	ctx context.Context,
	p platformDeps,
	leaseService *leasesapp.LeaseService,
	operationService *leasesapp.OperationService,
	reminderRepo *notificationspg.ReminderRepository,
	sessionRepo *identitypg.SessionRepository,
	codeRepo *identitypg.LoginCodeRepository,
	attemptRepo *identitypg.AttemptRepository,
	emailMailer mailer.Sender,
	billingRenewals billingapp.RenewalRunner,
	billingScheduledChanges billingapp.ScheduledChangeRunner,
	billingPayments billingapp.PaymentProcessor,
) *Workers {
	queries := platformgenerated.New(p.DB)
	contactResolver := notificationspg.NewContactResolver(queries)
	emailNotifier := emailnotifier.NewNotifier(emailMailer, p.Renderer)
	notifiers := map[notificationsapp.Channel]notificationsapp.Notifier{
		notificationsapp.ChannelEmail: emailNotifier,
	}
	reminderWorker := scheduler.NewReminderWorker(reminderRepo, p.Renderer, notifiers, contactResolver, p.Beginner, p.Clock, &scheduler.ExponentialBackoff{Base: 1 * time.Minute, Max: 1 * time.Hour, Factor: 2}, 5, 1*time.Minute, 30*time.Second, p.Logger)
	leaseReconciliationWorker := scheduler.NewLeaseReconciliationWorker(leaseService, p.Clock, 1*time.Hour, 100, p.Logger, p.TZResolver)
	billingWorker := scheduler.NewBillingWorker(billingRenewals, billingScheduledChanges, p.Pool, p.Clock, p.Cfg.BillingWorkerInterval, p.Logger)
	paymentReconciliationWorker := scheduler.NewPaymentReconciliationWorker(billingPayments, p.Pool, p.Clock, p.Cfg.PaymentReconciliationWorkerInterval, p.Logger)
	operationOverdueWorker := scheduler.NewOperationOverdueWorker(operationService, p.Clock, p.Cfg.OverdueOperationWorkerInterval, 100, p.Logger, p.TZResolver)

	dataCleaner := identityscheduler.NewCleaner(sessionRepo, codeRepo, attemptRepo, p.Clock, 1*time.Hour, 7*24*time.Hour, p.Logger)

	w := &Workers{}
	w.wg.Add(6)
	go func() { defer w.wg.Done(); dataCleaner.Run(ctx) }()
	go func() { defer w.wg.Done(); reminderWorker.Run(ctx) }()
	go func() { defer w.wg.Done(); leaseReconciliationWorker.Run(ctx) }()
	go func() { defer w.wg.Done(); billingWorker.Run(ctx) }()
	go func() { defer w.wg.Done(); paymentReconciliationWorker.Run(ctx) }()
	go func() { defer w.wg.Done(); operationOverdueWorker.Run(ctx) }()

	return w
}

// Wait blocks until all six worker goroutines have exited.
func (w *Workers) Wait() { w.wg.Wait() }
