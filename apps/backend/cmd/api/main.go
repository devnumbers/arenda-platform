package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	billingpg "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/postgres"
	identitypg "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/postgres"
	fakesms "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/sms"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	leasespg "github.com/nambers/arenda-planform/apps/backend/internal/leases/adapters/postgres"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	notificationspg "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/postgres"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	notificationsms "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/sms"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/cleaner"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/config"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	platformpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpapi"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/logger"
	platformnotifications "github.com/nambers/arenda-planform/apps/backend/internal/platform/notifications"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/scheduler"
	propertiespg "github.com/nambers/arenda-planform/apps/backend/internal/properties/adapters/postgres"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"golang.org/x/time/rate"
)

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

func main() {
	fallback := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	if err := run(fallback); err != nil {
		fallback.Error("backend stopped", "error", err)
		os.Exit(1)
	}
}

func run(fallback *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logHandler, err := logger.NewHandler(cfg.LogFormat, cfg.LogLevelValue, os.Stdout)
	if err != nil {
		return err
	}
	logger := slog.New(logHandler)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := database.MigrateUp(cfg.DatabaseURL, cfg.MigrationsDir); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database pool: %w", err)
	}
	defer pool.Close()

	tariffRepo := billingpg.NewTariffRepository(pool)
	subscriptionRepo := billingpg.NewSubscriptionRepository(pool)
	onboardingService := billingpg.NewOnboardingService(tariffRepo, subscriptionRepo)

	identityUserRepo := identitypg.NewUserRepository(pool)
	identitySMSRepo := identitypg.NewSMSCodeRepository(pool)
	identityAttemptRepo := identitypg.NewAttemptRepository(pool)
	identitySessionRepo := identitypg.NewSessionRepository(pool)

	var smsSender identityapp.Sender
	switch cfg.SMSSender {
	case "", "fake":
		if cfg.AppEnv == "production" {
			return fmt.Errorf("production environment requires a real SMS_SENDER")
		}
		smsSender = fakesms.NewFakeSender(logger)
	default:
		return fmt.Errorf("unsupported SMS_SENDER: %s", cfg.SMSSender)
	}

	authService := identityapp.NewAuthService(
		identityUserRepo,
		identitySMSRepo,
		identityAttemptRepo,
		identitySessionRepo,
		smsSender,
		realClock{},
		onboardingService,
		platformpostgres.NewBeginner(pool),
		logger,
	)

	propertyRepo := propertiespg.NewPropertyRepository(pool)
	occupancyProvider := propertiespg.NewOccupancyProvider(pool)
	limiter := billingpg.NewSubscriptionLimiter(pool)
	operationRepo := leasespg.NewOperationRepository(pool)
	recurringOpRepo := leasespg.NewRecurringOperationRepository(pool)
	reminderRepo := notificationspg.NewReminderRepository(pool)
	reminderScheduler := notificationsapp.NewReminderScheduler(reminderRepo, realClock{})
	propertyBillingLifecycle := leasespg.NewPropertyBillingLifecycle(operationRepo, recurringOpRepo, reminderScheduler, realClock{})
	propertyService := propertiesapp.NewPropertyService(
		propertyRepo,
		occupancyProvider,
		limiter,
		propertyBillingLifecycle,
		platformpostgres.NewBeginner(pool),
		realClock{},
		logger,
	)

	leaseRepo := leasespg.NewLeaseRepository(pool)
	leasePropertyRepo := leasespg.NewPropertyRepository(pool)
	tenantContactRepo := leasespg.NewTenantContactRepository(pool)

	leaseService := leasesapp.NewLeaseService(
		leaseRepo,
		leasePropertyRepo,
		tenantContactRepo,
		recurringOpRepo,
		operationRepo,
		reminderScheduler,
		platformpostgres.NewBeginner(pool),
		realClock{},
		logger,
	)
	tenantContactService := leasesapp.NewTenantContactService(tenantContactRepo, logger)
	operationService := leasesapp.NewOperationService(operationRepo, leasePropertyRepo, leaseRepo, reminderScheduler, platformpostgres.NewBeginner(pool), realClock{}, logger)
	recurringOperationService := leasesapp.NewRecurringOperationService(
		recurringOpRepo,
		operationRepo,
		leasePropertyRepo,
		reminderScheduler,
		platformpostgres.NewBeginner(pool),
		realClock{},
		logger,
	)

	reminderService := notificationsapp.NewReminderService(reminderRepo, realClock{}, platformpostgres.NewBeginner(pool))
	userContactProvider := platformnotifications.NewContactProvider(identityUserRepo)
	contactResolver := notificationspg.NewContactResolver(userContactProvider)
	smsSenderAdapter := platformnotifications.NewSMSSenderAdapter(smsSender)
	smsNotifier := notificationsms.NewNotifier(contactResolver, smsSenderAdapter, logger)
	reminderWorker := scheduler.NewReminderWorker(reminderRepo, smsNotifier, contactResolver, platformpostgres.NewBeginner(pool), realClock{}, &scheduler.ExponentialBackoff{Base: 1 * time.Minute, Max: 1 * time.Hour, Factor: 2}, 5, 1*time.Minute, 30*time.Second, logger)
	leaseReconciliationWorker := scheduler.NewLeaseReconciliationWorker(leaseService, realClock{}, 1*time.Hour, 100, logger)

	dataCleaner := cleaner.New(identitySessionRepo, identitySMSRepo, identityAttemptRepo, 1*time.Hour, 7*24*time.Hour, logger)
	go dataCleaner.Run(ctx)
	go reminderWorker.Run(ctx)
	go leaseReconciliationWorker.Run(ctx)

	ipLimiter := httpapi.NewRateLimiter(rate.Limit(cfg.RateLimit.IPRPS), cfg.RateLimit.IPBurst, 1*time.Hour)
	defer ipLimiter.Stop()
	phoneSendLimiter := httpapi.NewRateLimiter(rate.Limit(cfg.RateLimit.PhoneSendPerHour)/3600, cfg.RateLimit.PhoneSendPerHour, 1*time.Hour)
	defer phoneSendLimiter.Stop()
	phoneVerifyLimiter := httpapi.NewRateLimiter(rate.Limit(cfg.RateLimit.PhoneVerifyPer15Min)/(15*60), cfg.RateLimit.PhoneVerifyPer15Min, 1*time.Hour)
	defer phoneVerifyLimiter.Stop()

	handler := httpapi.New(httpapi.Deps{
		Auth:                authService,
		Sessions:            identitySessionRepo,
		Properties:          propertyService,
		Leases:              leaseService,
		TenantContacts:      tenantContactService,
		Operations:          operationService,
		RecurringOperations: recurringOperationService,
		Reminders:           reminderService,
		CookieSecure:        cfg.CookieSecure,
		Logger:              logger,
		Clock:               realClock{},
		IPRateLimiter:       ipLimiter,
		PhoneSendLimiter:    phoneSendLimiter,
		PhoneVerifyLimiter:  phoneVerifyLimiter,
	})

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("backend listening", "addr", cfg.HTTPAddr, "env", cfg.AppEnv)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
