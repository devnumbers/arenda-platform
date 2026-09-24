package application

import (
	"context"
	"errors"
	"fmt"

	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// txStores bundles the rentals repository, the payments property store (the
// serialization lock) and the transaction-bound payments gateway under one
// transaction, plus the audit recorder. It is the only handle a use case
// receives inside runInTx, so it is impossible to forget WithTx, to write
// outside the transaction or to record audit outside it (ADR 0033, ADR 0020).
type txStores struct {
	rentals    RentalStore
	properties paymentsapp.PropertyStore
	pay        RentPaymentGatewayTx
	audit      auditapp.Recorder
	history    historyapp.Recorder
}

// txStoreFactory is the composite factory of the rentals context (ADR 0053
// §3): the non-transactional rentals repository, the payments property store
// and the payments gateway plus the Unit-of-Work, building a transactional
// txStores from each runInTx call (ADR 0033 γ-factory) — both contexts'
// stores over one transaction. The non-transactional stores also serve the
// read side of the use cases.
type txStoreFactory struct {
	rentals    RentalStore
	properties paymentsapp.PropertyStore
	gateway    RentPaymentGateway
	tenants    TenantReader
	audit      auditapp.Recorder
	history    historyapp.Recorder
	uow        transaction.UoW
}

// NewTxStoreFactory bundles the stores, the ports, the audit recorder and the
// Unit-of-Work into the single factory every rentals use case embeds. A nil
// audit defaults to a Noop recorder so a caller that does not care about
// audit still gets a safe factory. The type stays unexported; callers use :=
// to hold it (standard Go pattern for a factory returning an unexported
// type).
func NewTxStoreFactory(
	rentals RentalStore, properties paymentsapp.PropertyStore,
	gateway RentPaymentGateway, tenants TenantReader,
	audit auditapp.Recorder, history historyapp.Recorder, uow transaction.UoW,
) txStoreFactory {
	if audit == nil {
		audit = auditapp.Noop{}
	}
	if history == nil {
		history = historyapp.Noop{}
	}
	return txStoreFactory{
		rentals:    rentals,
		properties: properties,
		gateway:    gateway,
		tenants:    tenants,
		audit:      audit,
		history:    history,
		uow:        uow,
	}
}

// runInTx opens a Unit-of-Work, binds the rentals repository, the payments
// property store and the payments gateway to the transaction, and runs work
// with them. UoW commits on nil error and rolls back otherwise; a panic in
// work rolls back and re-panics. A missing UoW is a wiring mistake and fails
// loudly and immediately.
func (f *txStoreFactory) runInTx(ctx context.Context, work func(*txStores) error) error {
	if f.uow == nil {
		return errors.New("rentals runInTx: Unit-of-Work is not configured")
	}
	return f.uow.Do(ctx, func(tx transaction.Tx) error {
		rentals, err := f.rentals.WithTx(tx)
		if err != nil {
			return fmt.Errorf("bind rentals store to tx: %w", err)
		}
		properties, err := f.properties.WithTx(tx)
		if err != nil {
			return fmt.Errorf("bind property store to tx: %w", err)
		}
		pay, err := f.gateway.WithTx(tx)
		if err != nil {
			return fmt.Errorf("bind rent payment gateway to tx: %w", err)
		}
		return work(&txStores{
			rentals:    rentals,
			properties: properties,
			pay:        pay,
			audit:      f.audit.WithTx(tx),
			history:    f.history.WithTx(tx),
		})
	})
}
