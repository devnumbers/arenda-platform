package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type beginner struct {
	pool *pgxpool.Pool
}

// NewBeginner creates a transaction beginner from a pgx connection pool.
func NewBeginner(pool *pgxpool.Pool) transaction.Beginner {
	return &beginner{pool: pool}
}

// Begin starts a new transaction.
func (b *beginner) Begin(ctx context.Context) (transaction.Tx, error) {
	return b.pool.Begin(ctx)
}
