//go:build integration

package wire

// Wiring guard for the identity assembly (testing-strategy, layer 3): the
// composition bug fixed in 84d14023 — EmailChange left out of the
// httpserver.Deps literal, so every /me/email/* route answered 500 and only
// live e2e caught it — is exactly the class this family closes. The test
// drives the real WireIdentity over the test pool and pins the assembly
// invariant: every service and adapter of the identity module comes out
// non-nil. Removing any wiring action inside WireIdentity turns it red.

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	auditpg "github.com/nambers/arenda-planform/apps/backend/internal/audit/adapters/postgres"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	identitydomain "github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	notificationspg "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/config"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/events"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
	"github.com/stretchr/testify/require"
)

func TestWireIdentityAssembly(t *testing.T) {
	t.Parallel()

	pool := testdb.Setup(t)
	logger := slog.New(slog.DiscardHandler)
	db := database.NewInstrumentedPool(pool, logger)

	encryptor, err := encryption.NewEncryptor("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	require.NoError(t, err)

	renderer, err := mailer.NewRenderer("../../../templates/email")
	require.NoError(t, err)

	cfg := &config.Config{
		EmailSender: providerFake,
		RateLimit: config.RateLimit{
			IPRPS:                       20,
			IPBurst:                     40,
			EmailSendPerHour:            60,
			EmailVerifyPer15Min:         30,
			PhoneChangeSendPerHour:      5,
			PhoneChangeVerifyPer15Min:   10,
			EmailChangeSendPerHour:      5,
			CodeSendPerRecipientPerHour: 5,
			CodeSendPerInitiatorPerHour: 10,
		},
	}
	limits := WireRateLimiters(cfg)
	defer limits.Stop()

	identity, err := WireIdentity(
		context.Background(),
		platformDeps{
			Cfg:           cfg,
			DB:            db,
			Logger:        logger,
			Encryptor:     encryptor,
			Renderer:      renderer,
			AuditRecorder: auditapp.NewService(auditpg.NewWriter(db), clock.Real{}),
			Clock:         clock.Real{},
			UoW:           postgres.NewUoW(pool, logger),
		},
		events.NewInProcessDispatcher(),
		limits,
	)
	require.NoError(t, err)

	require.NotNil(t, identity.UserRepo)
	require.NotNil(t, identity.CodeRepo)
	require.NotNil(t, identity.AttemptRepo)
	require.NotNil(t, identity.SessionRepo)
	require.NotNil(t, identity.SessionService)
	require.NotNil(t, identity.SessionLoader)
	require.NotNil(t, identity.EventPublisher)
	require.NotNil(t, identity.Authentication)
	require.NotNil(t, identity.PhoneChange)
	// The exact field 84d14023 left out of the composition root.
	require.NotNil(t, identity.EmailChange)
	require.NotNil(t, identity.Profile)
	require.NotNil(t, identity.Logout)
	require.NotNil(t, identity.EmailMailer)
	// The contact-change letters (решение #1207): the queue seam and its
	// delivery worker are part of the assembly.
	require.NotNil(t, identity.ContactQueue)
	require.NotNil(t, identity.ContactWorker)
}

// TestWireIdentityContactQueueBindsAndEnqueuesAtomically drives the full
// contact-letter chain over the real stack (решение #1207, testing-strategy
// layer 3): WireIdentity's queue seam, bound to the real River client
// WireRiverQueue built, schedules the letter inside the change's
// transaction — the job commits with the transaction and vanishes with its
// rollback. Removing the composition root's Bind (main.go) turns this red:
// the unbound seam refuses to schedule.
func TestWireIdentityContactQueueBindsAndEnqueuesAtomically(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pool := testdb.Setup(t)
	logger := slog.New(slog.DiscardHandler)
	db := database.NewInstrumentedPool(pool, logger)

	// The delivery queue owns its schema chain; the test pool needs it before
	// any insert (the app runs the same call at boot).
	require.NoError(t, database.MigrateRiverSchema(ctx, pool))

	encryptor, err := encryption.NewEncryptor("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	require.NoError(t, err)
	renderer, err := mailer.NewRenderer("../../../templates/email")
	require.NoError(t, err)

	cfg := &config.Config{
		EmailSender: providerFake,
		RateLimit: config.RateLimit{
			IPRPS:                       20,
			IPBurst:                     40,
			EmailSendPerHour:            60,
			EmailVerifyPer15Min:         30,
			PhoneChangeSendPerHour:      5,
			PhoneChangeVerifyPer15Min:   10,
			EmailChangeSendPerHour:      5,
			CodeSendPerRecipientPerHour: 5,
			CodeSendPerInitiatorPerHour: 10,
		},
		// The delivery-queue settings WireRiverQueue reads (positive values
		// keep the provider-rate division alive; the test never starts the
		// client, it only inserts).
		NotificationsEmailMaxWorkers:        4,
		NotificationsPushMaxWorkers:         16,
		NotificationsEmailMaxAttempts:       8,
		NotificationsPushMaxAttempts:        8,
		NotificationsRiverSoftStopTimeout:   10 * time.Second,
		NotificationsEmailProviderPerMinute: 60,
	}
	limits := WireRateLimiters(cfg)
	defer limits.Stop()

	deps := platformDeps{
		Cfg:           cfg,
		DB:            db,
		Pool:          pool,
		Logger:        logger,
		Encryptor:     encryptor,
		Renderer:      renderer,
		AuditRecorder: auditapp.NewService(auditpg.NewWriter(db), clock.Real{}),
		Clock:         clock.Real{},
		UoW:           postgres.NewUoW(pool, logger),
	}
	identity, err := WireIdentity(ctx, deps,
		events.NewInProcessDispatcher(), limits)
	require.NoError(t, err)

	notificationsMod := &Notifications{
		NotificationRepo: notificationspg.NewNotificationRepository(pool),
	}
	riverMod, err := WireRiverQueue(ctx, deps, notificationsMod,
		fakeContactResolver{}, fakeEmailSender{}, nil, // push: email-only local mode
		notificationspg.NewTaskScanStore(pool),
		notificationspg.NewPaymentScanStore(pool),
		notificationspg.NewRentalScanStore(pool),
		identity.ContactWorker)
	require.NoError(t, err)
	defer riverMod.ProviderLimiter.Stop()

	// The composition root's late bind (main.go). Without it the enqueue
	// below fails with the not-bound error.
	identity.ContactQueue.Bind(riverMod.Client)

	// A river client that never starts still accepts inserts; the worker is
	// registered, so the args kind is known.
	countJobs := func() int {
		var n int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM river_job WHERE kind = 'identity:contact_changed'`).Scan(&n))
		return n
	}
	require.Equal(t, 0, countJobs(), "fresh river schema holds no contact jobs")

	event := identityapp.ContactChangedEvent{
		Kind:      identityapp.ContactChangedEmail,
		UserID:    uuid.Must(uuid.NewV7()),
		Recipient: mustWireEmail(t, "owner@example.com"),
		ChangedAt: time.Now().UTC(),
	}

	// A committed transaction leaves exactly one job.
	require.NoError(t, deps.UoW.Do(ctx, func(tx transaction.Tx) error {
		return identity.ContactQueue.WithTx(tx).ScheduleChanged(ctx, event)
	}))
	require.Equal(t, 1, countJobs(), "committed change must carry its letter job")

	// A rolled-back transaction leaves none — the enqueue rides the change's
	// transaction, so an aborted change cannot leave a letter behind.
	err = deps.UoW.Do(ctx, func(tx transaction.Tx) error {
		if err := identity.ContactQueue.WithTx(tx).ScheduleChanged(ctx, event); err != nil {
			return err
		}
		return errors.New("force rollback")
	})
	require.Error(t, err)
	require.Equal(t, 1, countJobs(), "rolled-back change must not leave a letter job")
}

// mustWireEmail parses a test email for the wiring tests.
func mustWireEmail(t *testing.T, raw string) identitydomain.Email {
	t.Helper()
	email, err := identitydomain.NewEmail(raw)
	require.NoError(t, err)
	return email
}
