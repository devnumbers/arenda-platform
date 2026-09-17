package wire

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	notificationsjob "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/notificationsjob"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
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
	// ProviderLimiter is the global email-provider budget spent inside the
	// email worker.
	ProviderLimiter *httpsupport.RateLimiter
}

// WireRiverQueue builds the River client with the two delivery queues and
// the email/push workers, plus the publisher over the transactional
// enqueuer. The client is not started here: NewWorkers runs it in the
// workers phase and Workers.Wait waits for its full stop. Push jobs are
// enqueued only when a push sender could be built (VAPID keys configured);
// without them the pipeline runs in the email-only local mode.
func WireRiverQueue(
	ctx context.Context,
	p platformDeps,
	notificationsMod *Notifications,
	resolver notificationsapp.ContactResolver,
	emailer notificationsapp.DirectEmailSender,
	pushSender notificationsapp.PushSender,
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

	workers := river.NewWorkers()
	river.AddWorker(workers, notificationsjob.NewDeliverEmailWorker(
		notificationsMod.NotificationRepo,
		resolver,
		emailer,
		providerLimiter,
		emailMetrics,
		cfg.AppBaseURL,
		p.Logger,
	))
	river.AddWorker(workers, notificationsjob.NewDeliverPushWorker(
		notificationsMod.NotificationRepo,
		pushSender,
		notificationsMod.PushSubscriptionRepo,
		p.Logger,
	))

	client, err := river.NewClient(riverpgxv5.New(p.Pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			notificationsjob.QueueEmail: {MaxWorkers: cfg.NotificationsEmailMaxWorkers},
			notificationsjob.QueuePush:  {MaxWorkers: cfg.NotificationsPushMaxWorkers},
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

	return &RiverQueue{
		Client:          client,
		Publisher:       notificationsapp.NewPublisher(notificationsMod.NotificationRepo, queue, p.UoW, p.Logger),
		ProviderLimiter: providerLimiter,
	}, nil
}
