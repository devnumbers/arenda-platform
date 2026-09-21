package wire

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	notificationspg "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// queueTestConfig returns the notifications queue settings the wiring needs
// (the division building the provider rate needs positive values).
func queueTestConfig() *config.Config {
	return &config.Config{
		NotificationsEmailMaxWorkers:        4,
		NotificationsPushMaxWorkers:         16,
		NotificationsEmailMaxAttempts:       8,
		NotificationsPushMaxAttempts:        8,
		NotificationsRiverSoftStopTimeout:   10 * time.Second,
		NotificationsEmailProviderPerMinute: 60,
	}
}

// The wiring test builds the River client over a lazy pool (no connection is
// made at construction): it pins the bundle shape, not the runtime.
func TestWireRiverQueueBuildsBundle(t *testing.T) {
	t.Parallel()

	pool, err := pgxpool.New(context.Background(), "postgres://user:pass@localhost:5/db")
	require.NoError(t, err)
	defer pool.Close()

	p := platformDeps{Cfg: queueTestConfig(), Pool: pool, Logger: slog.Default()}
	notificationsMod := &Notifications{
		PushSubscriptionRepo: nil,
		NotificationRepo:     notificationspg.NewNotificationRepository(nil),
	}

	var pushSender application.PushSender // nil: email-only local mode
	taskStore := notificationspg.NewTaskScanStore(nil)
	paymentStore := notificationspg.NewPaymentScanStore(nil)
	rentalStore := notificationspg.NewRentalScanStore(nil)
	riverMod, err := WireRiverQueue(context.Background(), p, notificationsMod,
		fakeContactResolver{}, fakeEmailSender{}, pushSender, taskStore, paymentStore, rentalStore)
	require.NoError(t, err)
	defer riverMod.ProviderLimiter.Stop()

	assert.NotNil(t, riverMod.Client)
	assert.NotNil(t, riverMod.Publisher)
	assert.NotNil(t, riverMod.ProviderLimiter)
	assert.NotNil(t, riverMod.Stream, "the stream hub ships with the queue bundle")
	assert.NotNil(t, riverMod.TasksPublisher, "the tasks scan publisher ships with the queue bundle")
	assert.NotNil(t, riverMod.PaymentsPublisher, "the payments scan publisher ships with the queue bundle")
	assert.NotNil(t, riverMod.RentalsPublisher, "the rentals scan publisher ships with the queue bundle")
}

type fakeContactResolver struct{}

func (fakeContactResolver) Resolve(ctx context.Context, scope uuid.UUID) (application.Contact, error) {
	return application.Contact{}, nil
}

type fakeEmailSender struct{}

func (fakeEmailSender) SendTemplate(ctx context.Context, to, subject, template string, data map[string]any) error {
	return nil
}
