package database

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolConfig contains the runtime pgxpool settings controlled by app config.
type PoolConfig struct {
	MaxConns                        int32
	MinConns                        int32
	MaxConnLifetime                 time.Duration
	MaxConnIdleTime                 time.Duration
	HealthCheckPeriod               time.Duration
	StatementTimeout                time.Duration
	IdleInTransactionSessionTimeout time.Duration
}

// DefaultPoolConfig is the local/perf baseline. PostgreSQL max_connections must
// stay higher than MaxConns to leave diagnostic and maintenance headroom.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxConns:                        64,
		MinConns:                        16,
		MaxConnLifetime:                 30 * time.Minute,
		MaxConnIdleTime:                 5 * time.Minute,
		HealthCheckPeriod:               30 * time.Second,
		StatementTimeout:                30 * time.Second,
		IdleInTransactionSessionTimeout: 60 * time.Second,
	}
}

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	return NewPoolWithConfig(ctx, databaseURL, DefaultPoolConfig())
}

func NewPoolWithConfig(ctx context.Context, databaseURL string, poolConfig PoolConfig) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	cfg.MaxConns = poolConfig.MaxConns
	cfg.MinConns = poolConfig.MinConns
	cfg.MaxConnLifetime = poolConfig.MaxConnLifetime
	cfg.MaxConnIdleTime = poolConfig.MaxConnIdleTime
	cfg.HealthCheckPeriod = poolConfig.HealthCheckPeriod
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = make(map[string]string)
	}
	if poolConfig.StatementTimeout > 0 {
		cfg.ConnConfig.RuntimeParams["statement_timeout"] = fmt.Sprintf("%.0f", poolConfig.StatementTimeout.Seconds()*1000)
	}
	if poolConfig.IdleInTransactionSessionTimeout > 0 {
		cfg.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = fmt.Sprintf("%.0f", poolConfig.IdleInTransactionSessionTimeout.Seconds()*1000)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

func MigrateUp(databaseURL, migrationsDir string) error {
	absDir, err := filepath.Abs(migrationsDir)
	if err != nil {
		return fmt.Errorf("resolve migrations dir: %w", err)
	}
	// pgx/v5 driver registers itself as "pgx5", so we rewrite the scheme
	// while keeping the rest of the URL intact.
	migrateURL := databaseURL
	if strings.HasPrefix(migrateURL, "postgres://") {
		migrateURL = "pgx5://" + strings.TrimPrefix(migrateURL, "postgres://")
	} else if strings.HasPrefix(migrateURL, "postgresql://") {
		migrateURL = "pgx5://" + strings.TrimPrefix(migrateURL, "postgresql://")
	}
	m, err := migrate.New(
		"file://"+absDir,
		migrateURL,
	)
	if err != nil {
		return fmt.Errorf("create migrate: %w", err)
	}
	defer func() { _, _ = m.Close() }()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
