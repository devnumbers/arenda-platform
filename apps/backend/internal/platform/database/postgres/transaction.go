// Package postgres implements the transaction port over pgx: the Beginner and the Unit-of-Work behind every
// context's runInTx (ADR 0033).
package postgres

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type beginner struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewBeginner creates a transaction beginner from a pgx connection pool.
func NewBeginner(pool *pgxpool.Pool, logger *slog.Logger) transaction.Beginner {
	return &beginner{pool: pool, logger: logger}
}

// Begin starts a new transaction.
func (b *beginner) Begin(ctx context.Context) (transaction.Tx, error) {
	tx, err := b.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if b.logger == nil {
		return tx, nil
	}
	return database.NewInstrumentedTx(tx, b.logger), nil
}
