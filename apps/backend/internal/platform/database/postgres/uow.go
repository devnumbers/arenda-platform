package postgres

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// uow runs work inside a single database transaction. It delegates transaction
// creation to a Beginner so that instrumentation is applied in one place.
type uow struct {
	beginner transaction.Beginner
}

// NewUoW creates a Unit-of-Work backed by a pgx connection pool. Each Do call
// begins a fresh transaction, runs work, and commits on nil error or rolls back
// otherwise. See transaction.UoW for the full commit/rollback/panic contract.
func NewUoW(pool *pgxpool.Pool, logger *slog.Logger) transaction.UoW {
	return &uow{beginner: NewBeginner(pool, logger)}
}

// Do runs work inside a single database transaction.
func (u *uow) Do(ctx context.Context, work func(tx transaction.Tx) error) (err error) {
	tx, err := u.beginner.Begin(ctx)
	if err != nil {
		return err
	}

	// A panic in work rolls back the transaction and re-panics so a tx never
	// leaks to GC. The deferred rollback is a pgx v5 no-op once work committed.
	defer func() {
		_ = tx.Rollback(ctx)
		if r := recover(); r != nil {
			panic(r)
		}
	}()

	if err := work(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
