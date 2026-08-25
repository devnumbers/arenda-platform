package application

import (
	"context"
	"errors"
	"fmt"

	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// txStores bundles the payments repositories, the property serialization
// store and the audit recorder bound to the same transaction. It is the only
// handle a use case receives inside runInTx, so it is impossible to forget
// WithTx, to write outside the transaction or to record audit outside it
// (ADR 0033, ADR 0020).
type txStores struct {
	tick       TickStore
	payments   PaymentStore
	properties PropertyStore
	audit      auditapp.Recorder
}

// txStoreFactory holds the non-transactional payments repositories plus the
// Unit-of-Work and builds a transactional txStores from each runInTx call
// (ADR 0033 γ-factory). Build it once with NewTxStoreFactory at the wire
// layer and pass the same value to every payments service, so adding the Nth
// repository (ticket #461) is a change to one constructor call. The
// non-transactional stores also serve the read side of the use cases.
type txStoreFactory struct {
	tick       TickStore
	payments   PaymentStore
	properties PropertyStore
	audit      auditapp.Recorder
	uow        transaction.UoW
}

// NewTxStoreFactory bundles the payments repositories, the audit recorder and
// the Unit-of-Work into the single txStoreFactory every payments service
// embeds. A nil audit defaults to a Noop recorder so a caller that does not
// care about audit still gets a safe factory. The type stays unexported;
// callers use := to hold it (standard Go pattern for a factory returning an
// unexported type).
func NewTxStoreFactory(
	tick TickStore, payments PaymentStore, properties PropertyStore,
	audit auditapp.Recorder, uow transaction.UoW,
) txStoreFactory {
	if audit == nil {
		audit = auditapp.Noop{}
	}
	return txStoreFactory{
		tick:       tick,
		payments:   payments,
		properties: properties,
		audit:      audit,
		uow:        uow,
	}
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
		payments, err := f.payments.WithTx(tx)
		if err != nil {
			return fmt.Errorf("bind payment store to tx: %w", err)
		}
		properties, err := f.properties.WithTx(tx)
		if err != nil {
			return fmt.Errorf("bind property store to tx: %w", err)
		}
		return work(&txStores{
			tick:       tick,
			payments:   payments,
			properties: properties,
			audit:      f.audit.WithTx(tx),
		})
	})
}
