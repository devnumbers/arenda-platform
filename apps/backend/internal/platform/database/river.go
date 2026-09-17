package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// MigrateRiverSchema applies the River queue's schema migrations through the
// rivermigrate Go API (research #735 §2): River owns its own migration chain
// (river_migration) separate from the golang-migrate chain (schema_migrations)
// that owns db/migrations. The SQL is never vendored into db/migrations — it
// changes with every River upgrade and stays under River's own tests. The
// call is idempotent: applied versions are skipped. Deploy runs a single
// instance, so no advisory lock is taken around the migrate step.
func MigrateRiverSchema(ctx context.Context, pool *pgxpool.Pool) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("river migrator: %w", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("river migrate up: %w", err)
	}
	return nil
}
