// Package transaction defines a minimal, persistence-agnostic transaction port
// used by application layers. Adapters bind concrete pgx transactions to this
// port so that application code stays free of driver-specific types.
package transaction

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Tx is the application-layer view of a database transaction.
type Tx interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// Beginner starts database transactions.
type Beginner struct {
	pool *pgxpool.Pool
}

// NewBeginner creates a transaction beginner from a pgx connection pool.
func NewBeginner(pool *pgxpool.Pool) *Beginner {
	return &Beginner{pool: pool}
}

// Begin starts a new transaction.
func (b *Beginner) Begin(ctx context.Context) (Tx, error) {
	return b.pool.Begin(ctx)
}
