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
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/cleaner"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/config"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpapi"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
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

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevelValue}))

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
		transaction.NewBeginner(pool),
	)

	dataCleaner := cleaner.New(identitySessionRepo, identitySMSRepo, identityAttemptRepo, 1*time.Hour, 7*24*time.Hour, logger)
	go dataCleaner.Run(ctx)

	handler := httpapi.New(httpapi.Deps{
		Auth:         authService,
		Sessions:     identitySessionRepo,
		CookieSecure: cfg.CookieSecure,
		Logger:       logger,
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
