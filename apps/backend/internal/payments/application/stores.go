package application

import (
	"context"
	"errors"
	"fmt"

	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// txStores bundles the payments repositories, the property serialization
// store and the audit and history recorders bound to the same transaction.
// It is the only handle a use case receives inside runInTx, so it is
// impossible to forget WithTx, to write outside the transaction or to record
// audit or history outside it (ADR 0033, ADR 0020, ADR 0061).
type txStores struct {
	tick           TickStore
	payments       PaymentStore
	operations     OperationStore
	properties     PropertyStore
	favoriteOrders GlobalPaymentOrderStore
	rentalManaged  RentalManagedReader
	changeLog      PaymentChangeLogStore
	audit          auditapp.Recorder
	history        historyapp.Recorder
}

// txStoreFactory holds the non-transactional payments repositories plus the
// Unit-of-Work and builds a transactional txStores from each runInTx call
// (ADR 0033 γ-factory). Build it once with NewTxStoreFactory at the wire
// layer and pass the same value to every payments service, so adding the Nth
// repository is a change to one constructor call. The non-transactional
// stores also serve the read side of the use cases.
type txStoreFactory struct {
	tick           TickStore
	payments       PaymentStore
	operations     OperationStore
	properties     PropertyStore
	favoriteOrders GlobalPaymentOrderStore
	rentalManaged  RentalManagedReader
	changeLog      PaymentChangeLogStore
	audit          auditapp.Recorder
	history        historyapp.Recorder
	uow            transaction.UoW
}

// NewTxStoreFactory bundles the payments repositories, the recorders and the
// Unit-of-Work into the single txStoreFactory every payments service embeds.
// A nil recorder defaults to a Noop so a caller that does not care about
// audit, history or the change log still gets a safe factory. The type stays
// unexported; callers use := to hold it (standard Go pattern for a factory
// returning an unexported type).
func NewTxStoreFactory(
	tick TickStore, payments PaymentStore, operations OperationStore,
	properties PropertyStore, favoriteOrders GlobalPaymentOrderStore,
	rentalManaged RentalManagedReader, changeLog PaymentChangeLogStore,
	audit auditapp.Recorder, history historyapp.Recorder, uow transaction.UoW,
) txStoreFactory {
	if audit == nil {
		audit = auditapp.Noop{}
	}
	if history == nil {
		history = historyapp.Noop{}
	}
	if changeLog == nil {
		changeLog = NoopChangeLogStore{}
	}
	return txStoreFactory{
		tick:           tick,
		payments:       payments,
		operations:     operations,
		properties:     properties,
		favoriteOrders: favoriteOrders,
		rentalManaged:  rentalManaged,
		changeLog:      changeLog,
		audit:          audit,
		history:        history,
		uow:            uow,
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
		operations, err := f.operations.WithTx(tx)
		if err != nil {
			return fmt.Errorf("bind operation store to tx: %w", err)
		}
		properties, err := f.properties.WithTx(tx)
		if err != nil {
			return fmt.Errorf("bind property store to tx: %w", err)
		}
		favoriteOrders, err := f.favoriteOrders.WithTx(tx)
		if err != nil {
			return fmt.Errorf("bind favorite order store to tx: %w", err)
		}
		rentalManaged, err := f.rentalManaged.WithTx(tx)
		if err != nil {
			return fmt.Errorf("bind rental-managed reader to tx: %w", err)
		}
		changeLog, err := f.changeLog.WithTx(tx)
		if err != nil {
			return fmt.Errorf("bind change log store to tx: %w", err)
		}
		return work(&txStores{
			tick:           tick,
			payments:       payments,
			operations:     operations,
			properties:     properties,
			favoriteOrders: favoriteOrders,
			rentalManaged:  rentalManaged,
			changeLog:      changeLog,
			audit:          f.audit.WithTx(tx),
			history:        f.history.WithTx(tx),
		})
	})
}
