package application

import (
	"context"
	"errors"

	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// txStores bundles the leases repositories, the cross-context reminder
// scheduler and the audit recorder all bound to the same transaction. It is
// the only handle a use case receives inside runInTx, so it is impossible to
// forget WithTx or to record audit outside the transaction (ADR 0033, ADR
// 0020).
//
// The optional stores (leases, properties, tenantContacts, recurringOps,
// operations, categories, scheduler) cover the three services of the context,
// which need different subsets inside their transactions: the lease service
// touches the full set, the operation service never touches
// leases/tenantContacts/recurringOps in a transaction, and the recurring
// operation service never touches leases/tenantContacts. Production wires the
// full set once and shares the factory between the three services, so every
// store is bound in every transaction there; runInTx skips binding an
// unwired optional store instead of panicking on a nil WithTx — same shape
// as the properties optional stores — which keeps a test or a future wiring
// free to supply only the stores its use cases reach.
type txStores struct {
	leases         LeaseRepository
	properties     PropertyRepository
	tenantContacts TenantContactRepository
	recurringOps   RecurringOperationRepository
	operations     OperationRepository
	categories     OperationCategoryRepository
	scheduler      ReminderScheduler
	audit          auditapp.Recorder
}

// txStoreFactory holds the non-transactional leases repositories, the
// cross-context reminder scheduler and the audit recorder plus the
// Unit-of-Work, and builds a transactional txStores from each runInTx call.
// It is embedded anonymously by the three leases services that open their own
// transactions (lease, operation, recurring operation) so they share one
// canonical transactional shape (ADR 0033 γ-factory): a use case only sees
// runInTx(ctx, work) and the *txStores it hands out.
//
// Build it once with NewTxStoreFactory at the wire layer and pass the same
// value to the three service constructors, so adding an Nth repository is a
// change to one constructor call, not several.
type txStoreFactory struct {
	leases         LeaseRepository
	properties     PropertyRepository
	tenantContacts TenantContactRepository
	recurringOps   RecurringOperationRepository
	operations     OperationRepository
	categories     OperationCategoryRepository
	scheduler      ReminderScheduler
	audit          auditapp.Recorder
	uow            transaction.UoW
}

// NewTxStoreFactory bundles the leases repositories, the reminder scheduler,
// the audit recorder, and the Unit-of-Work into the single txStoreFactory the
// leases services embed (ADR 0033 γ-factory). A nil audit defaults to a Noop
// recorder so a caller that does not care about audit still gets a safe
// factory; the other stores may be nil (see txStores). The type stays
// unexported; callers use := to hold it (standard Go pattern for a factory
// returning an unexported type).
func NewTxStoreFactory(
	leases LeaseRepository,
	properties PropertyRepository,
	tenantContacts TenantContactRepository,
	recurringOps RecurringOperationRepository,
	operations OperationRepository,
	categories OperationCategoryRepository,
	scheduler ReminderScheduler,
	audit auditapp.Recorder,
	uow transaction.UoW,
) txStoreFactory {
	if audit == nil {
		audit = auditapp.Noop{}
	}
	return txStoreFactory{
		leases:         leases,
		properties:     properties,
		tenantContacts: tenantContacts,
		recurringOps:   recurringOps,
		operations:     operations,
		categories:     categories,
		scheduler:      scheduler,
		audit:          audit,
		uow:            uow,
	}
}

// runInTx opens a Unit-of-Work, builds the leases transactional stores from
// the transaction, and runs work with them. UoW commits on nil error and
// rolls back otherwise; a panic in work rolls back and re-panics (see
// transaction.UoW).
//
// Every WithTx in the store set is infallible (the repositories, the
// scheduler and the audit recorder all return the bound port directly), so
// there are no bind errors to wrap here; audit is bound last all the same.
// Unwired optional stores stay nil (see txStores).
//
// A call to runInTx returns an error if the factory's UoW was not configured:
// a service without a UoW has no business calling it. This keeps the call
// sites free of nil checks while making a wiring mistake loud and immediate.
func (f *txStoreFactory) runInTx(ctx context.Context, work func(*txStores) error) error {
	if f.uow == nil {
		return errors.New("leases runInTx: Unit-of-Work is not configured")
	}
	return f.uow.Do(ctx, func(tx transaction.Tx) error {
		stores := &txStores{
			audit: f.audit.WithTx(tx),
		}
		if f.leases != nil {
			stores.leases = f.leases.WithTx(tx)
		}
		if f.properties != nil {
			stores.properties = f.properties.WithTx(tx)
		}
		if f.tenantContacts != nil {
			stores.tenantContacts = f.tenantContacts.WithTx(tx)
		}
		if f.recurringOps != nil {
			stores.recurringOps = f.recurringOps.WithTx(tx)
		}
		if f.operations != nil {
			stores.operations = f.operations.WithTx(tx)
		}
		if f.categories != nil {
			stores.categories = f.categories.WithTx(tx)
		}
		if f.scheduler != nil {
			stores.scheduler = f.scheduler.WithTx(tx)
		}
		return work(stores)
	})
}
