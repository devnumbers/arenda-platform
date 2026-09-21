package wire

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	notificationsjob "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/notificationsjob"
	notificationspg "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/postgres"
	notificationsstream "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/stream"
	taskschedule "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/taskschedule"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/sse"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"
	"github.com/riverqueue/rivercontrib/otelriver"
	"golang.org/x/time/rate"
)

// RiverQueue bundles the delivery queue (River) and the notifications
// publisher. The client is started in the workers phase; the publisher is
// the seam the grace migration (#741) and the catalog publishers
// (#748–#752) call. ProviderLimiter must be stopped by the caller.
type RiverQueue struct {
	Client *river.Client[pgx.Tx]
	// Publisher writes the feed rows and schedules the channel deliveries;
	// call it strictly after the publishing context's own transaction
	// commits (the grace-events canon, решение #740).
	Publisher *notificationsapp.Publisher
	// TasksPublisher is the tasks scan's publisher (issue #750): the hourly
	// sweep books the upcoming timed tasks' due-minute jobs through the same
	// client and sweeps the zones for the already-overdue ones; the
	// due-minute worker publishes through it (bound post-construction, see
	// DeferredTaskOverdueDeliverer).
	TasksPublisher *notificationsapp.TasksPublisher
	// PaymentsPublisher is the payments scan's publisher (issues #749,
	// #776): the hourly sweep books the upcoming operations' boundary jobs
	// — «Оплатите платёж» at 00:00 of the operation date, «Платёж
	// просрочен» at 00:00 of the day after, each in the owner's timezone —
	// and sweeps the zones as the backstop; the boundary workers publish
	// through it (bound post-construction, see
	// DeferredPaymentBoundaryDeliverer).
	PaymentsPublisher *notificationsapp.PaymentsPublisher
	// RentalsPublisher is the rental scan's publisher (issues #748, #777):
	// the hourly sweep books the unfinished rentals' completed-boundary jobs
	// — «Аренда завершена» at 00:00 of the day after the planned end, in the
	// owner's timezone — and sweeps the zones as the backstop; the boundary
	// worker publishes through it (bound post-construction, see
	// DeferredRentalBoundaryDeliverer).
	RentalsPublisher *notificationsapp.RentalCompletedPublisher
	// TasksSeam is the tasks context's scheduling seam (issue #775): the
	// rule create/edit flows hand their standing tasks' ids over post-commit
	// and the seam plans them through the tasks publisher. The composition
	// root wires it into the tasks module after this constructor — the
	// tasks module builds earlier than the queue.
	TasksSeam *taskschedule.Seam
	// Stream is the shared event stream hub (карта #734, #742; ADR 0060):
	// the publisher pushes the live frames through it post-commit and the
	// HTTP server serves GET /notifications/stream from it. The composition
	// root closes it on shutdown, so long-lived streams do not hold up the
	// graceful drain.
	Stream *sse.Hub
	// ProviderLimiter is the global email-provider budget spent inside the
	// email worker.
	ProviderLimiter *httpsupport.RateLimiter
}

// The scheduled jobs' worker ceilings: tiny jobs (one feed read and one
// write) and rare — a fixed domain decision, no env knob (the scan cadence
// canon). The payments' (#776) and rentals' (#777) queues share the tasks'
// (#750) value: a midnight across zones wakes the boundary jobs in a batch,
// each stays one feed write.
const (
	notificationsTaskMaxWorkers    = 2
	notificationsPaymentMaxWorkers = 2
	notificationsRentalMaxWorkers  = 2
)

// WireRiverQueue builds the River client with the two delivery queues, the
// tasks boundary queue, the payment boundary queue, the rental boundary
// queue and the email/push/task/payment/rental workers, plus the publisher
// over the transactional enqueuer and the event stream hub behind it (#742,
// ADR 0060). The tasks (#750), payments (#776) and rentals (#777) scan
// publishers wire here too: their booking legs schedule the boundary jobs
// through the same client, so they bind the workers' deferred deliverers —
// the composition root passes them to the scan group. The client is not
// started here: NewWorkers runs it in the workers phase and Workers.Wait
// waits for its full stop. Push jobs are enqueued only when a push sender
// could be built (VAPID keys configured); without them the pipeline runs in
// the email-only local mode.
func WireRiverQueue(
	ctx context.Context,
	p platformDeps,
	notificationsMod *Notifications,
	resolver notificationsapp.ContactResolver,
	emailer notificationsapp.TemplateEmailSender,
	pushSender notificationsapp.PushSender,
	taskStore *notificationspg.TaskScanStore,
	paymentStore *notificationspg.PaymentScanStore,
	rentalStore *notificationspg.RentalScanStore,
) (*RiverQueue, error) {
	cfg := p.Cfg

	// The burst absorbs a short provider burst without touching the minute
	// budget; min keeps it from exceeding the whole budget (research #735 §8).
	providerLimiter := httpsupport.NewRateLimiter(
		rate.Every(time.Minute/time.Duration(cfg.NotificationsEmailProviderPerMinute)),
		min(10, cfg.NotificationsEmailProviderPerMinute),
		time.Hour,
	)

	emailMetrics, err := notificationsjob.NewEmailMetrics()
	if err != nil {
		providerLimiter.Stop()
		return nil, fmt.Errorf("wire email metrics: %w", err)
	}
	streamMetrics, err := sse.NewMetrics()
	if err != nil {
		providerLimiter.Stop()
		return nil, fmt.Errorf("wire stream metrics: %w", err)
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, notificationsjob.NewDeliverEmailWorker(
		notificationsMod.NotificationRepo,
		resolver,
		emailer,
		providerLimiter,
		emailMetrics,
		cfg.AppBaseURL,
		p.Logger,
		notificationsMod.DeliverySettingsGate,
	))
	river.AddWorker(workers, notificationsjob.NewDeliverPushWorker(
		notificationsMod.NotificationRepo,
		pushSender,
		notificationsMod.PushSubscriptionRepo,
		p.Logger,
	))
	taskDeliverer := &notificationsjob.DeferredTaskOverdueDeliverer{}
	river.AddWorker(workers, notificationsjob.NewTaskOverdueWorker(taskDeliverer, p.Logger))
	paymentDeliverer := &notificationsjob.DeferredPaymentBoundaryDeliverer{}
	river.AddWorker(workers, notificationsjob.NewPaymentDueWorker(paymentDeliverer, p.Clock, p.Logger))
	river.AddWorker(workers, notificationsjob.NewPaymentOverdueWorker(paymentDeliverer, p.Clock, p.Logger))
	rentalDeliverer := &notificationsjob.DeferredRentalBoundaryDeliverer{}
	river.AddWorker(workers, notificationsjob.NewRentalCompletedWorker(rentalDeliverer, p.Clock, p.Logger))

	client, err := river.NewClient(riverpgxv5.New(p.Pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			notificationsjob.QueueEmail:    {MaxWorkers: cfg.NotificationsEmailMaxWorkers},
			notificationsjob.QueuePush:     {MaxWorkers: cfg.NotificationsPushMaxWorkers},
			notificationsjob.QueueTasks:    {MaxWorkers: notificationsTaskMaxWorkers},
			notificationsjob.QueuePayments: {MaxWorkers: notificationsPaymentMaxWorkers},
			notificationsjob.QueueRentals:  {MaxWorkers: notificationsRentalMaxWorkers},
		},
		Workers:         workers,
		Logger:          p.Logger,
		ErrorHandler:    notificationsjob.NewErrorHandler(p.Logger),
		SoftStopTimeout: cfg.NotificationsRiverSoftStopTimeout,
		// Spans and metrics flow to the global OTel providers (Uptrace)
		// without further code.
		Plugins: []rivertype.Plugin{otelriver.NewMiddleware(nil)},
	})
	if err != nil {
		providerLimiter.Stop()
		return nil, fmt.Errorf("river client: %w", err)
	}

	queue := notificationsjob.NewRiverQueue(client, pushSender != nil,
		cfg.NotificationsEmailMaxAttempts, cfg.NotificationsPushMaxAttempts)

	p.Logger.InfoContext(ctx, "notifications delivery queue initialized",
		slog.Int("email_max_workers", cfg.NotificationsEmailMaxWorkers),
		slog.Int("push_max_workers", cfg.NotificationsPushMaxWorkers),
		slog.Int("email_provider_per_minute", cfg.NotificationsEmailProviderPerMinute),
		slog.Bool("push_enabled", pushSender != nil),
	)

	notificationsStream := sse.NewHub(streamMetrics)
	publisher := notificationsapp.NewPublisher(notificationsMod.NotificationRepo, queue,
		notificationsstream.NewPublisher(notificationsStream, p.Clock), p.UoW, p.Logger)
	tasksPublisher := notificationsapp.NewTasksPublisher(publisher, taskStore, taskStore,
		notificationsjob.NewTaskOverdueScheduler(client))
	taskDeliverer.Bind(tasksPublisher)
	paymentsPublisher := notificationsapp.NewPaymentsPublisher(publisher, paymentStore, paymentStore,
		notificationsjob.NewPaymentBoundaryScheduler(client))
	paymentDeliverer.Bind(paymentsPublisher)
	rentalsPublisher := notificationsapp.NewRentalCompletedPublisher(publisher, rentalStore, rentalStore,
		notificationsjob.NewRentalBoundaryScheduler(client))
	rentalDeliverer.Bind(rentalsPublisher)

	return &RiverQueue{
		Client:            client,
		Publisher:         publisher,
		TasksPublisher:    tasksPublisher,
		PaymentsPublisher: paymentsPublisher,
		RentalsPublisher:  rentalsPublisher,
		TasksSeam:         taskschedule.NewSeam(tasksPublisher, p.Clock),
		Stream:            notificationsStream,
		ProviderLimiter:   providerLimiter,
	}, nil
}
