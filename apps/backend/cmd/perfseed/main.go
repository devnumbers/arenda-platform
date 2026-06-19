package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	endpoint := flag.String("endpoint", "", "endpoint key to seed extra fixtures for")
	flag.Parse()

	ctx := context.Background()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.ErrorContext(ctx, "DATABASE_URL is not set")
		os.Exit(1)
	}

	ownerCount, err := envInt("PERF_OWNERS", 1000)
	if err != nil {
		logger.ErrorContext(ctx, "invalid PERF_OWNERS", slog.String("error", err.Error()))
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
		ownerCount: ownerCount,
	}

	state, err := seedBase(ctx, db, cfg)
	if err != nil {
		logger.ErrorContext(ctx, "seed base fixtures", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if *endpoint != "" {
		if err := seedEndpoint(ctx, db, state, *endpoint); err != nil {
			logger.ErrorContext(ctx, "seed endpoint fixtures", slog.String("endpoint", *endpoint), slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	if err := writeFixtures(state); err != nil {
		logger.ErrorContext(ctx, "write fixtures", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.InfoContext(ctx, "seed complete", slog.Int("owners", len(state.owners)))
}

func envInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return 0, fmt.Errorf("invalid %s: %s", key, v)
	}
	return n, nil
}
