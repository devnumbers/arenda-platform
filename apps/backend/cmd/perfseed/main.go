package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	seedOwnerCount                     = 1000
	seedPropertiesPerOwner             = 3
	seedOperationsPerProperty          = 24
	seedRecurringOperationsPerProperty = 4
	seedRemindersPerTarget             = 3
	seedTenantContactsPerOwner         = 5
	seedPaymentMethodsPerOwner         = 2
	seedSubscriptionPaymentsPerOwner   = 6
)

func main() {
	ctx := context.Background()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.ErrorContext(ctx, "DATABASE_URL is not set")
		os.Exit(1)
	}

	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		logger.ErrorContext(ctx, "connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	if err := resetDB(ctx, db); err != nil {
		logger.ErrorContext(ctx, "reset database", slog.String("error", err.Error()))
		os.Exit(1)
	}

	cfg := seedConfig{
		ownerCount:                     seedOwnerCount,
		propertiesPerOwner:             seedPropertiesPerOwner,
		operationsPerProperty:          seedOperationsPerProperty,
		recurringOperationsPerProperty: seedRecurringOperationsPerProperty,
		remindersPerTarget:             seedRemindersPerTarget,
		tenantContactsPerOwner:         seedTenantContactsPerOwner,
		paymentMethodsPerOwner:         seedPaymentMethodsPerOwner,
		subscriptionPaymentsPerOwner:   seedSubscriptionPaymentsPerOwner,
	}

	state, err := seedBase(ctx, db, cfg)
	if err != nil {
		logger.ErrorContext(ctx, "seed base fixtures", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if err := writeFixtures(state); err != nil {
		logger.ErrorContext(ctx, "write fixtures", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.InfoContext(ctx, "seed complete", slog.Int("owners", len(state.owners)))
}
