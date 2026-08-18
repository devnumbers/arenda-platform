// Package wire holds the per-module constructors that decompose the backend's
// composition root (formerly cmd/api/main.go) into small, testable wiring
// functions. Each constructor takes a shared platformDeps bundle plus any
// cross-module dependencies as parameters and returns a struct with named,
// exported fields for everything the HTTP layer or other modules need.
//
// The construction order is owned by main.go, which calls these functions in
// dependency order, registers event subscribers, builds httpserver.Deps and
// runs the HTTP server. No behavior is changed by this split: the sequence of
// construction, the event subscriptions, the worker goroutines and the server
// lifecycle remain identical to the original monolithic run().
package wire

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	auditpg "github.com/nambers/arenda-planform/apps/backend/internal/audit/adapters/postgres"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/config"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	platformpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/logger"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/observability"
	platformpolicy "github.com/nambers/arenda-planform/apps/backend/internal/platform/policy"
	platformtz "github.com/nambers/arenda-planform/apps/backend/internal/platform/tzresolver"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// platformDeps bundles the platform-level resources shared across every module
// constructor. Fields are populated once by WirePlatform and passed by value
// (or pointer where appropriate) into the per-module wire functions.
type platformDeps struct {
	Cfg           *config.Config
	DB            *database.InstrumentedPool
	Pool          *pgxpool.Pool
	Logger        *slog.Logger
	Encryptor     encryption.Encryptor
	Renderer      *mailer.Renderer
	AuditRecorder auditapp.Recorder
	TZResolver    *platformtz.OwnerTimezone
	Policy        sharedpolicy.Policy
	Clock         clock.Clock
	Beginner      transaction.Beginner
	UoW           transaction.UoW
	OTelShutdown  func(ctx context.Context) error
	StopSignalCtx context.CancelFunc
	PoolConfigLog func() // logs the database pool config; nil-safe
}

// Platform is the result of WirePlatform. It exposes the shared platform
// resources and the cleanup function; the lifecycle context is returned
// separately because a context never lives in a struct field.
type Platform struct {
	Deps platformDeps
	// Cleanup releases platform resources (db pool, otel sdk, signal ctx) in
	// reverse-ish order. It must be called when the process exits.
	Cleanup func()
}

// NewAppLogger builds the process logger from the app config (format and
// level). WirePlatform and the migrate step share it so both emit records of
// the same shape.
func NewAppLogger(cfg *config.Config) (*slog.Logger, error) {
	logHandler, err := logger.NewHandler(cfg.LogFormat, cfg.LogLevelValue, os.Stdout)
	if err != nil {
		return nil, err
	}
	return slog.New(logHandler), nil
}

// WirePlatform builds the platform foundation: config, logger, signal context,
// OpenTelemetry SDK, email renderer, encryptor, auto-migration, database pool,
// instrumented pool, audit writer/recorder and the shared timezone resolver.
// It returns the platform bundle plus the signal-scoped lifecycle context that
// main.go selects on for shutdown.
func WirePlatform() (*Platform, context.Context, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}

	appLogger, err := NewAppLogger(&cfg)
	if err != nil {
		return nil, nil, err
	}
	slog.SetDefault(appLogger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	otelSDK, err := observability.NewSDK(ctx, observability.Config{
		ServiceName:  cfg.OTelServiceName,
		Enabled:      cfg.OTelEnabled,
		OTLPEndpoint: cfg.OTelOTLPEndpoint,
		TraceSampler: cfg.OTelTraceSampler,
	})
	if err != nil {
		stop()
		return nil, nil, fmt.Errorf("observability: %w", err)
	}

	renderer, err := mailer.NewRenderer(cfg.EmailTemplatesDir)
	if err != nil {
		stop()
		return nil, nil, fmt.Errorf("failed to load email templates: %w", err)
	}

	encryptor, err := encryption.NewEncryptor(cfg.EncryptionKey)
	if err != nil {
		stop()
		return nil, nil, fmt.Errorf("encryption: %w", err)
	}
	if cfg.EncryptionKey == "" {
		if cfg.PaymentProvider != "fake" {
			stop()
			return nil, nil, errors.New("ENCRYPTION_KEY is required when using a real payment provider")
		}
		appLogger.WarnContext(ctx, "ENCRYPTION_KEY is empty; provider tokens will be stored without encryption (local dev only)")
	}

	if cfg.AutoMigrate {
		if err := database.MigrateUp(cfg.DatabaseURL, cfg.MigrationsDir); err != nil {
			stop()
			return nil, nil, fmt.Errorf("migrate: %w", err)
		}
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
		stop()
		return nil, nil, fmt.Errorf("database pool: %w", err)
	}
	db := database.NewInstrumentedPool(pool, appLogger)
	auditWriter := auditpg.NewWriter(db)
	auditRecorder := auditapp.NewService(auditWriter, clock.Real{})
	appLogger.InfoContext(ctx, "database pool initialized",
		"max_conns", poolConfig.MaxConns,
		"min_conns", poolConfig.MinConns,
		"max_conn_lifetime", poolConfig.MaxConnLifetime.String(),
		"max_conn_idle_time", poolConfig.MaxConnIdleTime.String(),
		"health_check_period", poolConfig.HealthCheckPeriod.String(),
		"statement_timeout", poolConfig.StatementTimeout.String(),
		"idle_in_transaction_session_timeout", poolConfig.IdleInTransactionSessionTimeout.String(),
	)

	tzResolver := platformtz.NewOwnerTimezone(db)
	policy := platformpolicy.NewOwnerOnlyPolicy()

	deps := platformDeps{
		Cfg:           &cfg,
		DB:            db,
		Pool:          pool,
		Logger:        appLogger,
		Encryptor:     encryptor,
		Renderer:      renderer,
		AuditRecorder: auditRecorder,
		TZResolver:    tzResolver,
		Policy:        policy,
		Clock:         clock.Real{},
		Beginner:      platformpostgres.NewBeginner(pool, appLogger),
		UoW:           platformpostgres.NewUoW(pool, appLogger),
		OTelShutdown:  otelSDK.Shutdown,
		StopSignalCtx: stop,
	}

	cleanup := func() {
		pool.Close()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := otelSDK.Shutdown(shutdownCtx); err != nil {
			appLogger.ErrorContext(ctx, "observability shutdown failed", "error", err)
		}
		stop()
	}

	return &Platform{Deps: deps, Cleanup: cleanup}, ctx, nil
}

// DBPoolStats returns a snapshot provider for the httpserver's local pool
// diagnostics endpoint. It reads live statistics from the underlying pgx pool.
func DBPoolStats(pool *pgxpool.Pool) func() httpsupport.DBPoolSnapshot {
	return func() httpsupport.DBPoolSnapshot {
		stat := pool.Stat()
		return httpsupport.DBPoolSnapshot{
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
