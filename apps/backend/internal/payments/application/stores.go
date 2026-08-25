package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// txStores bundles the payments repositories bound to the same transaction.
// It is the only handle a use case receives inside runInTx, so it is
// impossible to forget WithTx or to write outside the transaction
// (ADR 0033).
type txStores struct {
	tick TickStore
}

// txStoreFactory holds the non-transactional payments repositories plus the
// Unit-of-Work and builds a transactional txStores from each runInTx call
// (ADR 0033 γ-factory). Build it once with NewTxStoreFactory at the wire
// layer and pass the same value to every payments service, so adding the Nth
// repository (tickets #457, #461) is a change to one constructor call.
type txStoreFactory struct {
	tick TickStore
	uow  transaction.UoW
}

// NewTxStoreFactory bundles the payments repositories and the Unit-of-Work
// into the single txStoreFactory every payments service embeds. The type
// stays unexported; callers use := to hold it (standard Go pattern for a
// factory returning an unexported type).
func NewTxStoreFactory(tick TickStore, uow transaction.UoW) txStoreFactory {
	return txStoreFactory{tick: tick, uow: uow}
}

// runInTx opens a Unit-of-Work, builds the payments transactional stores from
// the transaction, and runs work with them. UoW commits on nil error and
// rolls back otherwise; a panic in work rolls back and re-panics. A missing
// UoW is a wiring mistake and fails loudly and immediately.
func (f *txStoreFactory) runInTx(ctx context.Context, work func(*txStores) error) error {
	if f.uow == nil {
		return errors.New("payments runInTx: Unit-of-Work is not configured")
	}
	return f.uow.Do(ctx, func(tx transaction.Tx) error {
		tick, err := f.tick.WithTx(tx)
		if err != nil {
			return fmt.Errorf("bind tick store to tx: %w", err)
		}
		return work(&txStores{tick: tick})
	})
}
