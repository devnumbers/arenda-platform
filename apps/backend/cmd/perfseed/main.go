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
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Error("DATABASE_URL is not set")
		os.Exit(1)
	}

	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		logger.Error("connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	if err := resetDB(ctx, db); err != nil {
		logger.Error("reset database", slog.String("error", err.Error()))
		os.Exit(1)
	}

	cfg := seedConfig{
		ownerCount: envInt("PERF_OWNERS", 1000),
	}

	state, err := seedBase(ctx, db, cfg)
	if err != nil {
		logger.Error("seed base fixtures", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if *endpoint != "" {
		if err := seedEndpoint(ctx, db, state, *endpoint); err != nil {
			logger.Error("seed endpoint fixtures", slog.String("endpoint", *endpoint), slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	logger.Info("seed complete", slog.Int("owners", len(state.owners)))
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		panic(fmt.Sprintf("invalid %s: %s", key, v))
	}
	return n
}
