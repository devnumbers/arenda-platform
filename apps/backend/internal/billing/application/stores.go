package application

import (
	"context"
	"errors"
	"fmt"

	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// txStores bundles the billing repositories and audit recorder all bound to
// the same transaction. It is the only handle a use case receives inside
// runInTx, so it is impossible to forget WithTx or to record audit outside the
// transaction (ADR 0033, ADR 0020).
type txStores struct {
	tariffs       TariffRepository
	subscriptions SubscriptionRepository
	transitions   SubscriptionTransitionRepository
	audit         auditapp.Recorder
}

// txStoreFactory holds the non-transactional billing repositories and audit
// recorder plus the Unit-of-Work, and builds a transactional txStores from each
// runInTx call. It is embedded anonymously by every billing service so they
// share one canonical transactional shape (ADR 0033 γ-factory): a use case only
// sees runInTx(ctx, work) and the *txStores it hands out.
//
// Build it once with NewTxStoreFactory at the wire layer and pass the same value
// to every billing service constructor, so adding an Nth repository is a change
// to one constructor call, not several.
type txStoreFactory struct {
	tariffs       TariffRepository
	subscriptions SubscriptionRepository
	transitions   SubscriptionTransitionRepository
	audit         auditapp.Recorder
	uow           transaction.UoW
}

// NewTxStoreFactory bundles the billing repositories, the audit recorder, and
// the Unit-of-Work into the single txStoreFactory every billing service embeds
// (ADR 0033 γ-factory). A nil audit defaults to a Noop recorder so a caller
// that does not care about audit still gets a safe factory. The type stays
// unexported; callers use := to hold it (standard Go pattern for a factory
// returning an unexported type).
func NewTxStoreFactory(
	tariffs TariffRepository,
	subscriptions SubscriptionRepository,
	transitions SubscriptionTransitionRepository,
	audit auditapp.Recorder,
	uow transaction.UoW,
) txStoreFactory {
	if audit == nil {
		audit = auditapp.Noop{}
	}
	return txStoreFactory{
		tariffs:       tariffs,
		subscriptions: subscriptions,
		transitions:   transitions,
		audit:         audit,
		uow:           uow,
	}
}

// runInTx opens a Unit-of-Work, builds the billing transactional stores from
// the transaction, and runs work with them. UoW commits on nil error and rolls
// back otherwise; a panic in work rolls back and re-panics (see
// transaction.UoW).
//
// WithTx errors are wrapped so a repository that fails to bind aborts the
// transaction with a clear cause rather than a silent fallthrough. audit is
// bound last: its WithTx is infallible (auditapp.Recorder.WithTx returns no
// error), so it cannot mask a prior repository-bind failure.
//
// runInTx returns an error if the factory's UoW was not configured — a service
// without a UoW has no business calling it. This keeps the call sites free of
// nil checks while making a wiring mistake loud and immediate.
func (f *txStoreFactory) runInTx(ctx context.Context, work func(*txStores) error) error {
	if f.uow == nil {
		return errors.New("billing runInTx: Unit-of-Work is not configured")
	}
	return f.uow.Do(ctx, func(tx transaction.Tx) error {
		tariffs, err := f.tariffs.WithTx(tx)
		if err != nil {
			return fmt.Errorf("bind tariff repository to tx: %w", err)
		}
		subscriptions, err := f.subscriptions.WithTx(tx)
		if err != nil {
			return fmt.Errorf("bind subscription repository to tx: %w", err)
		}
		transitions, err := f.transitions.WithTx(tx)
		if err != nil {
			return fmt.Errorf("bind transition repository to tx: %w", err)
		}
		stores := &txStores{
			tariffs:       tariffs,
			subscriptions: subscriptions,
			transitions:   transitions,
			audit:         f.audit.WithTx(tx),
		}
		return work(stores)
	})
}
