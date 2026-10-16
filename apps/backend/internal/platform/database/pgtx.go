package database

import (
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// PgxTxOf returns the pgx transaction behind a transaction.Tx so
// infrastructure that must join the caller's transaction with driver-level
// semantics — the River queue's transactional enqueue (InsertTx) — can. An
// instrumented transaction unwraps to its driver transaction, a bare pgx.Tx
// passes through, anything else is a wiring error.
func PgxTxOf(tx transaction.Tx) (pgx.Tx, error) {
	switch t := tx.(type) {
	case *InstrumentedTx:
		if t.raw == nil {
			return nil, fmt.Errorf("instrumented tx wraps %T, not a pgx.Tx", t.tx)
		}
		return t.raw, nil
	case pgx.Tx:
		return t, nil
	default:
		return nil, fmt.Errorf("%T is not a pgx.Tx", tx)
	}
}
