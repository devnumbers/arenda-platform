package wire

import (
	"context"
	"sync"
	"time"

	accesspg "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/postgres"
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

// Compile-time check that the access member-recipient adapter satisfies the
// notifications recipient-lister port consumed by the reminder worker (issue
// #159). The assertion lives in the wiring layer so the access context never
// imports notifications.
var _ notificationsapp.PropertyRecipientLister = (*accesspg.MemberRecipientAdapter)(nil)

// Compile-time checks that the billing worker phases satisfy the scheduler's
// consumer-side ports (ADR 0035). The phases are the issue #252 lifecycle
// workers; conformance is checked here, at the wiring site.
var (
	_ scheduler.ScheduledChangeProcessor = (*billingapp.Workers)(nil)
	_ scheduler.RenewalProcessor         = (*billingapp.Workers)(nil)
	_ scheduler.PaymentReconciler        = (*billingapp.Workers)(nil)
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
// cleaner, the notifications reminder worker (with the property member
// recipient lister for the fan-out, issue #159), the lease reconciliation
// worker, the billing worker, the payment reconciliation worker and the
// operation overdue worker. It must be called with the still-active request
// context so the workers shut down when cancellation propagates.
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
	billingWorkers *billingapp.Workers,
	pushSubRepo *notificationspg.PushSubscriptionRepository,
	pushSender notificationsapp.PushSender,
) *Workers {
	queries := platformgenerated.New(p.DB)
	contactResolver := notificationspg.NewContactResolver(queries)
	emailNotifier := emailnotifier.NewNotifier(emailMailer, p.Renderer)
	notifiers := map[notificationsapp.Channel]notificationsapp.Notifier{
		notificationsapp.ChannelEmail: emailNotifier,
	}
	// The membership repository is stateless, so the recipient lister is built
	// here directly instead of being exported from WireAccess (issue #159).
	recipientLister := accesspg.NewMemberRecipientAdapter(accesspg.NewMembershipRepository(p.DB))
	reminderWorker := scheduler.NewReminderWorker(reminderRepo, p.Renderer, notifiers, contactResolver, recipientLister, pushSender, pushSubRepo, p.Beginner, p.Clock, &scheduler.ExponentialBackoff{Base: 1 * time.Minute, Max: 1 * time.Hour, Factor: 2}, 5, 1*time.Minute, 30*time.Second, p.Logger)
	leaseReconciliationWorker := scheduler.NewLeaseReconciliationWorker(leaseService, p.Clock, 1*time.Hour, 100, p.Logger, p.TZResolver)
	billingWorker := scheduler.NewBillingWorker(billingWorkers, billingWorkers, p.Pool, p.Clock, p.Cfg.BillingWorkerInterval, p.Logger)
	paymentReconciliationWorker := scheduler.NewPaymentReconciliationWorker(billingWorkers, p.Pool, p.Clock, p.Cfg.PaymentReconciliationWorkerInterval, p.Logger)
	operationOverdueWorker := scheduler.NewOperationOverdueWorker(operationService, p.Clock, 100, p.Logger, p.TZResolver)

	dataCleaner := identityscheduler.NewCleaner(sessionRepo, codeRepo, attemptRepo, p.Clock, p.Cfg.IdentityCleanerInterval, p.Cfg.IdentityCleanerRetention, p.Logger)

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
