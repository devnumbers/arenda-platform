package application

import (
	"context"
	"errors"

	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// txStores bundles the preference repository and the audit recorder all
// bound to the same transaction. It is the only handle a use case receives
// inside runInTx, so it is impossible to forget WithTx or to record audit
// outside the transaction (ADR 0033, ADR 0020).
type txStores struct {
	repo  PreferenceRepository
	audit auditapp.Recorder
}

// txStoreFactory holds the non-transactional preference repository and audit
// recorder plus the Unit-of-Work, and builds a transactional txStores from
// each runInTx call. It is embedded anonymously by the notifications
// services that open their own transactions so they share one canonical
// transactional shape (ADR 0033 γ-factory): a use case only sees
// runInTx(ctx, work) and the *txStores it hands out. The push subscription
// service never opens its own transactions and stays outside the factory.
//
// Build it once with NewTxStoreFactory at the wire layer and pass the same
// value to the notifications service constructors, so adding an Nth
// repository is a change to one constructor call, not several.
type txStoreFactory struct {
	repo  PreferenceRepository
	audit auditapp.Recorder
	uow   transaction.UoW
}

// NewTxStoreFactory bundles the preference repository, the audit recorder,
// and the Unit-of-Work into the single txStoreFactory the notifications
// services embed (ADR 0033 γ-factory). A nil audit defaults to a Noop recorder
// so a caller that does not care about audit still gets a safe factory. The
// type stays unexported; callers use := to hold it (standard Go pattern for a
// factory returning an unexported type).
func NewTxStoreFactory(
	repo PreferenceRepository,
	audit auditapp.Recorder,
	uow transaction.UoW,
) txStoreFactory {
	if audit == nil {
		audit = auditapp.Noop{}
	}
	return txStoreFactory{
		repo:  repo,
		audit: audit,
		uow:   uow,
	}
}

// runInTx opens a Unit-of-Work, builds the notifications transactional stores
// from the transaction, and runs work with them. UoW commits on nil error and
// rolls back otherwise; a panic in work rolls back and re-panics (see
// transaction.UoW).
//
// PreferenceRepository.WithTx is infallible (it returns the bound port
// directly, like the properties repositories), so there are no bind errors to
// wrap here; audit is bound last all the same.
//
// The runInTx method returns an error if the factory's UoW was not configured —
// a service without a UoW has no business calling it. This keeps the call
// sites free of nil checks while making a wiring mistake loud and immediate.
func (f *txStoreFactory) runInTx(ctx context.Context, work func(*txStores) error) error {
	if f.uow == nil {
		return errors.New("notifications runInTx: Unit-of-Work is not configured")
	}
	return f.uow.Do(ctx, func(tx transaction.Tx) error {
		stores := &txStores{
			repo:  f.repo.WithTx(tx),
			audit: f.audit.WithTx(tx),
		}
		return work(stores)
	})
}
