// Package transaction defines a minimal, persistence-agnostic transaction port
// used by application layers. Adapters bind concrete pgx transactions to this
// port so that application code stays free of driver-specific types.
package transaction

import "context"

// Tx is the application-layer view of a database transaction.
type Tx interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// Beginner starts database transactions.
type Beginner interface {
	Begin(ctx context.Context) (Tx, error)
}
