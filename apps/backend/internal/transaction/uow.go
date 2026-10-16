package transaction

import "context"

// UoW runs work inside a single database transaction. The work function
// receives a Tx bound to the transaction; it must not retain the Tx past the
// call.
//
// Commit/rollback semantics:
//   - If work returns nil, UoW commits the transaction. A commit error is
//     returned to the caller as-is (context cancellation during commit is not
//     wrapped).
//   - If work returns a non-nil error, UoW rolls back the transaction and
//     returns the work error.
//   - If work panics, UoW rolls back the transaction and re-panics, so a
//     panicking use case never leaves a transaction open.
//
// A deferred rollback that runs after a successful commit is a pgx v5 no-op
// and is relied upon rather than tracking a committed flag.
type UoW interface {
	Do(ctx context.Context, work func(tx Tx) error) error
}
