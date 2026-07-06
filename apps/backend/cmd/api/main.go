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
	adminpg "github.com/nambers/arenda-planform/apps/backend/internal/admin/adapters/postgres"
	adminapp "github.com/nambers/arenda-planform/apps/backend/internal/admin/application"
	paymentfake "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment/fake"
	paymenttkassa "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment/tkassa"
	billingpg "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/postgres"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	identityemail "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/email"
	identitypg "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/postgres"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	leasespg "github.com/nambers/arenda-planform/apps/backend/internal/leases/adapters/postgres"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	emailnotifier "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/email"
	notificationspg "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/postgres"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/cleaner"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/config"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	platformpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/events"
	platformgenerated "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpapi"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/logger"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
	mailerfake "github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer/fake"
	mailersmtp "github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer/smtp"
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

	renderer, err := mailer.NewRenderer(cfg.EmailTemplatesDir)
	if err != nil {
		return fmt.Errorf("failed to load email templates: %w", err)
	}

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
	onboardingService := billingpg.NewOnboardingService(tariffRepo, subscriptionRepo, platformpostgres.NewBeginner(pool, appLogger))
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

	identityUserRepo := identitypg.NewUserRepository(db, encryptor)
	identityCodeRepo := identitypg.NewLoginCodeRepository(db, encryptor)
	identityAttemptRepo := identitypg.NewAttemptRepository(db, encryptor)
	identitySessionRepo := identitypg.NewSessionRepository(db, encryptor)
	identitySessionService := identityapp.NewSessionService(identitySessionRepo)
	userAuthenticator := identityapp.NewUserAuthenticator(identityUserRepo, identitySessionRepo, realClock{})
	eventDispatcher := events.NewInProcessDispatcher()

	if cfg.EncryptionKey != "" {
		if err := backfillPhoneEncryption(ctx, db, encryptor, appLogger); err != nil {
			return fmt.Errorf("backfill phone encryption: %w", err)
		}
	} else {
		appLogger.WarnContext(ctx, "skipping phone encryption backfill: ENCRYPTION_KEY is empty")
	}

	var emailMailer mailer.Sender
	switch cfg.EmailSender {
	case "smtp":
		emailMailer = mailersmtp.NewSender(mailersmtp.Config{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUser,
			Password: cfg.SMTPPass,
			From:     cfg.SMTPFrom,
			FromName: cfg.SMTPFromName,
			Timeout:  cfg.SMTPTimeout,
		})
	case "fake":
		emailMailer = mailerfake.NewFakeSender(appLogger)
	default:
		return fmt.Errorf("unsupported EMAIL_SENDER: %s", cfg.EmailSender)
	}

	emailSender := identityemail.NewSender(emailMailer, renderer)

	authService := identityapp.NewAuthService(
		identityUserRepo,
		identityCodeRepo,
		identityAttemptRepo,
		identitySessionRepo,
		emailSender,
		realClock{},
		userAuthenticator,
		eventDispatcher,
		platformpostgres.NewBeginner(pool, appLogger),
		appLogger,
		encryptor,
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
		if err := photoStorage.HeadBucket(ctx); err != nil {
			return fmt.Errorf("photo storage: head bucket %q: %w", cfg.PhotoStorageBucket, err)
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
		onboardingService,
	)
	eventDispatcher.OnUserRegistered(billingService.OnUserRegistered)

	adminRepo := adminpg.NewAdminRepository(db, encryptor, realClock{}, occupancyProvider)
	adminService := adminapp.NewAdminService(adminRepo, adminRepo, adminRepo, adminRepo, adminRepo, billingService, realClock{})

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
	queries := platformgenerated.New(db)
	contactResolver := notificationspg.NewContactResolver(queries)
	emailNotifier := emailnotifier.NewNotifier(emailMailer, renderer)
	notifiers := map[notificationsapp.Channel]notificationsapp.Notifier{
		notificationsapp.ChannelEmail: emailNotifier,
	}
	reminderWorker := scheduler.NewReminderWorker(reminderRepo, renderer, notifiers, contactResolver, platformpostgres.NewBeginner(pool, appLogger), realClock{}, &scheduler.ExponentialBackoff{Base: 1 * time.Minute, Max: 1 * time.Hour, Factor: 2}, 5, 1*time.Minute, 30*time.Second, appLogger)
	leaseReconciliationWorker := scheduler.NewLeaseReconciliationWorker(leaseService, realClock{}, 1*time.Hour, 100, appLogger)
	billingWorker := scheduler.NewBillingWorker(billingService, pool, realClock{}, cfg.BillingWorkerInterval, appLogger)
	paymentReconciliationWorker := scheduler.NewPaymentReconciliationWorker(billingService, pool, realClock{}, cfg.PaymentReconciliationWorkerInterval, appLogger)
	operationOverdueWorker := scheduler.NewOperationOverdueWorker(operationService, realClock{}, cfg.OverdueOperationWorkerInterval, 100, appLogger)

	dataCleaner := cleaner.New(identitySessionRepo, identityCodeRepo, identityAttemptRepo, realClock{}, 1*time.Hour, 7*24*time.Hour, appLogger)
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

	emailSendBurst := 3
	if cfg.RateLimit.EmailSendPerHour < emailSendBurst {
		emailSendBurst = cfg.RateLimit.EmailSendPerHour
	}
	emailSendLimiter := httpapi.NewRateLimiter(rate.Limit(cfg.RateLimit.EmailSendPerHour)/3600, emailSendBurst, 1*time.Hour)
	defer emailSendLimiter.Stop()

	emailVerifyBurst := 5
	if cfg.RateLimit.EmailVerifyPer15Min < emailVerifyBurst {
		emailVerifyBurst = cfg.RateLimit.EmailVerifyPer15Min
	}
	emailVerifyLimiter := httpapi.NewRateLimiter(rate.Limit(cfg.RateLimit.EmailVerifyPer15Min)/(15*60), emailVerifyBurst, 1*time.Hour)
	defer emailVerifyLimiter.Stop()

	phoneChangeSendBurst := 3
	if cfg.RateLimit.PhoneChangeSendPerHour < phoneChangeSendBurst {
		phoneChangeSendBurst = cfg.RateLimit.PhoneChangeSendPerHour
	}
	phoneChangeSendLimiter := httpapi.NewRateLimiter(rate.Every(time.Hour/time.Duration(cfg.RateLimit.PhoneChangeSendPerHour)), phoneChangeSendBurst, 1*time.Hour)
	defer phoneChangeSendLimiter.Stop()

	phoneChangeVerifyBurst := 5
	if cfg.RateLimit.PhoneChangeVerifyPer15Min < phoneChangeVerifyBurst {
		phoneChangeVerifyBurst = cfg.RateLimit.PhoneChangeVerifyPer15Min
	}
	phoneChangeVerifyLimiter := httpapi.NewRateLimiter(rate.Every(15*time.Minute/time.Duration(cfg.RateLimit.PhoneChangeVerifyPer15Min)), phoneChangeVerifyBurst, 1*time.Hour)
	defer phoneChangeVerifyLimiter.Stop()

	var poolStats func() httpapi.DBPoolSnapshot
	if cfg.AppEnv == "local" {
		poolStats = dbPoolStats(pool)
	}

	handler := httpapi.New(httpapi.Deps{
		Auth:                     authService,
		Billing:                  billingService,
		Admin:                    adminService,
		Sessions:                 identitySessionService,
		Properties:               propertyService,
		AddressSuggester:         dadataClient,
		Leases:                   leaseService,
		TenantContacts:           tenantContactService,
		Operations:               operationService,
		RecurringOperations:      recurringOperationService,
		Reminders:                reminderService,
		AppBaseURL:               cfg.AppBaseURL,
		CookieSecure:             cfg.CookieSecure,
		Logger:                   appLogger,
		Clock:                    realClock{},
		LogSuccessfulRequests:    cfg.LogSuccessfulRequests,
		IPRateLimiter:            ipLimiter,
		EmailSendLimiter:         emailSendLimiter,
		EmailVerifyLimiter:       emailVerifyLimiter,
		PhoneChangeSendLimiter:   phoneChangeSendLimiter,
		PhoneChangeVerifyLimiter: phoneChangeVerifyLimiter,
		DBPoolStats:              poolStats,
		DevMode:                  cfg.AppEnv == "local" && cfg.PaymentProvider == "fake",
		TrustedProxies:           cfg.TrustedProxies,
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

func backfillPhoneEncryption(ctx context.Context, db *database.InstrumentedPool, enc encryption.Encryptor, logger *slog.Logger) error {
	backfillTable := func(table string) (int64, error) {
		rows, err := db.Query(ctx, "SELECT id, phone FROM "+table+" WHERE phone_encrypted = false")
		if err != nil {
			return 0, fmt.Errorf("select unencrypted %s: %w", table, err)
		}
		defer rows.Close()

		var updated int64
		for rows.Next() {
			var id, phone string
			if err := rows.Scan(&id, &phone); err != nil {
				return updated, fmt.Errorf("scan %s: %w", table, err)
			}
			encrypted, err := enc.DeterministicEncrypt(ctx, phone)
			if err != nil {
				return updated, fmt.Errorf("encrypt %s phone: %w", table, err)
			}
			tag, err := db.Exec(ctx, "UPDATE "+table+" SET phone = $1, phone_encrypted = true WHERE id = $2", encrypted, id)
			if err != nil {
				return updated, fmt.Errorf("update %s: %w", table, err)
			}
			updated += tag.RowsAffected()
		}
		if err := rows.Err(); err != nil {
			return updated, fmt.Errorf("iterate %s: %w", table, err)
		}
		return updated, nil
	}

	tables := []string{"users", "sms_codes", "login_attempts"}
	for _, table := range tables {
		count, err := backfillTable(table)
		if err != nil {
			return err
		}
		logger.InfoContext(ctx, "phone encryption backfill complete", "table", table, "rows_updated", count)
	}
	return nil
}
