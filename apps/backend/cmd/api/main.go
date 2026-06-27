package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	paymentfake "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment/fake"
	paymenttkassa "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment/tkassa"
	billingpg "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/postgres"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	identitypg "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/postgres"
	fakesms "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/sms"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	leasespg "github.com/nambers/arenda-planform/apps/backend/internal/leases/adapters/postgres"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	notificationspg "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/postgres"
	notificationsms "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/sms"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/cleaner"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/config"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	platformpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpapi"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/logger"
	platformnotifications "github.com/nambers/arenda-planform/apps/backend/internal/platform/notifications"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/scheduler"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/adapters/dadata"
	propertiespg "github.com/nambers/arenda-planform/apps/backend/internal/properties/adapters/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/adapters/storage"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"golang.org/x/time/rate"
)

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

func dbPoolStats(pool *pgxpool.Pool) func() httpapi.DBPoolSnapshot {
	return func() httpapi.DBPoolSnapshot {
		stat := pool.Stat()
		return httpapi.DBPoolSnapshot{
			AcquiredConns:          stat.AcquiredConns(),
			IdleConns:              stat.IdleConns(),
			TotalConns:             stat.TotalConns(),
			ConstructingConns:      stat.ConstructingConns(),
			MaxConns:               stat.MaxConns(),
			AcquireCount:           stat.AcquireCount(),
			AcquireDurationMS:      float64(stat.AcquireDuration()) / float64(time.Millisecond),
			CanceledAcquireCount:   stat.CanceledAcquireCount(),
			EmptyAcquireCount:      stat.EmptyAcquireCount(),
			EmptyAcquireWaitTimeMS: float64(stat.EmptyAcquireWaitTime()) / float64(time.Millisecond),
			NewConnsCount:          stat.NewConnsCount(),
		}
	}
}

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
	appLogger := slog.New(logHandler)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	encryptor, err := encryption.NewEncryptor(cfg.EncryptionKey)
	if err != nil {
		return fmt.Errorf("encryption: %w", err)
	}
	if cfg.EncryptionKey == "" {
		if cfg.PaymentProvider != "fake" {
			return fmt.Errorf("ENCRYPTION_KEY is required when using a real payment provider")
		}
		appLogger.WarnContext(ctx, "ENCRYPTION_KEY is empty; provider tokens will be stored without encryption (local dev only)")
	}

	if err := database.MigrateUp(cfg.DatabaseURL, cfg.MigrationsDir); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	poolConfig := database.PoolConfig{
		MaxConns:                        cfg.DBPool.MaxConns,
		MinConns:                        cfg.DBPool.MinConns,
		MaxConnLifetime:                 cfg.DBPool.MaxConnLifetime,
		MaxConnIdleTime:                 cfg.DBPool.MaxConnIdleTime,
		HealthCheckPeriod:               cfg.DBPool.HealthCheckPeriod,
		StatementTimeout:                cfg.DBPool.StatementTimeout,
		IdleInTransactionSessionTimeout: cfg.DBPool.IdleInTransactionSessionTimeout,
	}
	pool, err := database.NewPoolWithConfig(ctx, cfg.DatabaseURL, poolConfig)
	if err != nil {
		return fmt.Errorf("database pool: %w", err)
	}
	defer pool.Close()
	db := database.NewInstrumentedPool(pool, appLogger)
	appLogger.InfoContext(ctx, "database pool initialized",
		"max_conns", poolConfig.MaxConns,
		"min_conns", poolConfig.MinConns,
		"max_conn_lifetime", poolConfig.MaxConnLifetime.String(),
		"max_conn_idle_time", poolConfig.MaxConnIdleTime.String(),
		"health_check_period", poolConfig.HealthCheckPeriod.String(),
		"statement_timeout", poolConfig.StatementTimeout.String(),
		"idle_in_transaction_session_timeout", poolConfig.IdleInTransactionSessionTimeout.String(),
	)

	tariffRepo := billingpg.NewTariffRepository(db, cfg.TariffCacheTTL, clock.Real{})
	subscriptionRepo := billingpg.NewSubscriptionRepository(db)
	onboardingService := billingpg.NewOnboardingService(tariffRepo, subscriptionRepo)
	paymentMethodRepo := billingpg.NewPaymentMethodRepository(db, encryptor)
	subscriptionPaymentRepo := billingpg.NewSubscriptionPaymentRepository(db)
	appLogger.InfoContext(ctx, "billing repositories initialized",
		"payment_methods", paymentMethodRepo != nil,
		"subscription_payments", subscriptionPaymentRepo != nil)

	var paymentProvider billingapp.Provider
	switch cfg.PaymentProvider {
	case "fake":
		paymentProvider = paymentfake.NewProvider(cfg.AppBaseURL, appLogger, realClock{})
	case "tkassa":
		paymentProvider = paymenttkassa.NewProvider(cfg.TKassaBaseURL, cfg.TKassaTerminalKey, cfg.TKassaPassword, cfg.TKassaTimeout, appLogger)
	}
	appLogger.InfoContext(ctx, "payment provider initialized", "provider", cfg.PaymentProvider, "initialized", paymentProvider != nil)

	identityUserRepo := identitypg.NewUserRepository(db)
	identitySMSRepo := identitypg.NewSMSCodeRepository(db)
	identityAttemptRepo := identitypg.NewAttemptRepository(db)
	identitySessionRepo := identitypg.NewSessionRepository(db)

	var smsSender identityapp.Sender
	switch cfg.SMSSender {
	case "", "fake":
		if cfg.AppEnv != "local" && cfg.AppEnv != "dev" {
			return fmt.Errorf("SMS_SENDER=fake is only allowed in local or dev environments")
		}
		smsSender = fakesms.NewFakeSender(appLogger)
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
		platformpostgres.NewBeginner(pool, appLogger),
		appLogger,
	)

	propertyRepo := propertiespg.NewPropertyRepository(db)
	propertyPhotoRepo := propertiespg.NewPropertyPhotoRepository(db)
	occupancyProvider := propertiespg.NewOccupancyProvider(db)
	limiter := billingpg.NewSubscriptionLimiter(db)
	operationRepo := leasespg.NewOperationRepository(db)
	recurringOpRepo := leasespg.NewRecurringOperationRepository(db)
	leaseRepo := leasespg.NewLeaseRepository(db)
	reminderRepo := notificationspg.NewReminderRepository(db)
	reminderScheduler := notificationsapp.NewReminderScheduler(reminderRepo, realClock{})
	propertyBillingLifecycle := leasespg.NewPropertyBillingLifecycle(operationRepo, recurringOpRepo, reminderScheduler, realClock{})

	var photoStorage propertiesapp.PhotoStorage
	if cfg.PhotoStorageS3Enabled {
		var err error
		photoStorage, err = storage.NewS3Storage(
			cfg.PhotoStorageEndpoint,
			cfg.PhotoStorageRegion,
			cfg.PhotoStorageBucket,
			cfg.PhotoStorageAccessKey,
			cfg.PhotoStorageSecretKey,
			cfg.PhotoStoragePublicBaseURL,
			cfg.PhotoStoragePathStyle,
		)
		if err != nil {
			return fmt.Errorf("photo storage: %w", err)
		}
		appLogger.InfoContext(ctx, "photo storage initialized", "provider", "s3", "bucket", cfg.PhotoStorageBucket, "endpoint", cfg.PhotoStorageEndpoint)
	} else {
		photoStorage = storage.NewFakeStorage(cfg.PhotoStoragePublicBaseURL)
		appLogger.InfoContext(ctx, "photo storage initialized", "provider", "fake")
	}

	propertyService := propertiesapp.NewPropertyService(
		propertyRepo,
		propertyPhotoRepo,
		photoStorage,
		occupancyProvider,
		limiter,
		propertyBillingLifecycle,
		leaseRepo,
		platformpostgres.NewBeginner(pool, appLogger),
		realClock{},
		appLogger,
	)

	dadataClient := dadata.NewClient(dadata.Config{
		BaseURL:   cfg.DaDataBaseURL,
		APIKey:    cfg.DaDataAPIKey,
		SecretKey: cfg.DaDataSecretKey,
		Timeout:   cfg.DaDataTimeout,
		Logger:    appLogger,
	})

	billingService := billingapp.NewBillingService(
		tariffRepo,
		subscriptionRepo,
		paymentMethodRepo,
		subscriptionPaymentRepo,
		paymentProvider,
		platformpostgres.NewBeginner(pool, appLogger),
		realClock{},
		appLogger,
		cfg.AppBaseURL,
		propertyService,
	)

	leasePropertyRepo := leasespg.NewPropertyRepository(db)
	tenantContactRepo := leasespg.NewTenantContactRepository(db)

	leaseService := leasesapp.NewLeaseService(
		leaseRepo,
		leasePropertyRepo,
		tenantContactRepo,
		recurringOpRepo,
		operationRepo,
		reminderScheduler,
		platformpostgres.NewBeginner(pool, appLogger),
		realClock{},
		appLogger,
	)
	tenantContactService := leasesapp.NewTenantContactService(tenantContactRepo, appLogger)
	operationService := leasesapp.NewOperationService(operationRepo, leasePropertyRepo, leaseRepo, recurringOpRepo, reminderScheduler, platformpostgres.NewBeginner(pool, appLogger), realClock{}, appLogger)
	reminderService := notificationsapp.NewReminderService(reminderRepo, realClock{})
	recurringOperationService := leasesapp.NewRecurringOperationService(
		recurringOpRepo,
		operationRepo,
		leasePropertyRepo,
		reminderScheduler,
		reminderService,
		platformpostgres.NewBeginner(pool, appLogger),
		realClock{},
		appLogger,
	)
	userContactProvider := platformnotifications.NewContactProvider(identityUserRepo)
	contactResolver := notificationspg.NewContactResolver(userContactProvider)
	smsSenderAdapter := platformnotifications.NewSMSSenderAdapter(smsSender)
	smsNotifier := notificationsms.NewNotifier(contactResolver, smsSenderAdapter, appLogger)
	reminderWorker := scheduler.NewReminderWorker(reminderRepo, smsNotifier, contactResolver, platformpostgres.NewBeginner(pool, appLogger), realClock{}, &scheduler.ExponentialBackoff{Base: 1 * time.Minute, Max: 1 * time.Hour, Factor: 2}, 5, 1*time.Minute, 30*time.Second, appLogger)
	leaseReconciliationWorker := scheduler.NewLeaseReconciliationWorker(leaseService, realClock{}, 1*time.Hour, 100, appLogger)
	billingWorker := scheduler.NewBillingWorker(billingService, pool, realClock{}, cfg.BillingWorkerInterval, appLogger)
	paymentReconciliationWorker := scheduler.NewPaymentReconciliationWorker(billingService, pool, realClock{}, cfg.PaymentReconciliationWorkerInterval, appLogger)
	operationOverdueWorker := scheduler.NewOperationOverdueWorker(operationService, realClock{}, cfg.OverdueOperationWorkerInterval, 100, appLogger)

	dataCleaner := cleaner.New(identitySessionRepo, identitySMSRepo, identityAttemptRepo, realClock{}, 1*time.Hour, 7*24*time.Hour, appLogger)
	var workers sync.WaitGroup
	workers.Add(6)
	go func() { defer workers.Done(); dataCleaner.Run(ctx) }()
	go func() { defer workers.Done(); reminderWorker.Run(ctx) }()
	go func() { defer workers.Done(); leaseReconciliationWorker.Run(ctx) }()
	go func() { defer workers.Done(); billingWorker.Run(ctx) }()
	go func() { defer workers.Done(); paymentReconciliationWorker.Run(ctx) }()
	go func() { defer workers.Done(); operationOverdueWorker.Run(ctx) }()

	ipLimiter := httpapi.NewRateLimiter(rate.Limit(cfg.RateLimit.IPRPS), cfg.RateLimit.IPBurst, 1*time.Hour)
	defer ipLimiter.Stop()
	phoneSendLimiter := httpapi.NewRateLimiter(rate.Limit(cfg.RateLimit.PhoneSendPerHour)/3600, cfg.RateLimit.PhoneSendPerHour, 1*time.Hour)
	defer phoneSendLimiter.Stop()
	phoneVerifyLimiter := httpapi.NewRateLimiter(rate.Limit(cfg.RateLimit.PhoneVerifyPer15Min)/(15*60), cfg.RateLimit.PhoneVerifyPer15Min, 1*time.Hour)
	defer phoneVerifyLimiter.Stop()

	var poolStats func() httpapi.DBPoolSnapshot
	if cfg.AppEnv == "local" {
		poolStats = dbPoolStats(pool)
	}

	handler := httpapi.New(httpapi.Deps{
		Auth:                  authService,
		Billing:               billingService,
		Sessions:              identitySessionRepo,
		Properties:            propertyService,
		AddressSuggester:      dadataClient,
		Leases:                leaseService,
		TenantContacts:        tenantContactService,
		Operations:            operationService,
		RecurringOperations:   recurringOperationService,
		Reminders:             reminderService,
		CookieSecure:          cfg.CookieSecure,
		Logger:                appLogger,
		Clock:                 realClock{},
		LogSuccessfulRequests: cfg.LogSuccessfulRequests,
		IPRateLimiter:         ipLimiter,
		PhoneSendLimiter:      phoneSendLimiter,
		PhoneVerifyLimiter:    phoneVerifyLimiter,
		DBPoolStats:           poolStats,
		DevMode:               cfg.AppEnv == "local" && cfg.PaymentProvider == "fake",
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
		appLogger.Info("backend listening", "addr", cfg.HTTPAddr, "env", cfg.AppEnv)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		workers.Wait()
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
