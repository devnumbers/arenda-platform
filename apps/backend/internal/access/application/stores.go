package application

import (
	"context"
	"errors"

	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// txStores bundles the access repositories and audit recorder all bound to the
// same transaction. It is the only handle a use case receives inside runInTx,
// so it is impossible to forget WithTx or to record audit outside the
// transaction (ADR 0033, ADR 0020).
//
// tx additionally exposes the raw transaction handle: the SlotCoordinator port
// takes a transaction.Tx because properties and billing call it inside their
// own transactions, so an access use case passes stores.tx through when it
// needs slot enforcement or recovery in its transaction.
type txStores struct {
	members     MembershipRepository
	invitations InvitationRepository
	audit       auditapp.Recorder
	tx          transaction.Tx
}

// txStoreFactory holds the non-transactional access repositories and audit
// recorder plus the Unit-of-Work, and builds a transactional txStores from each
// runInTx call. It is embedded anonymously by every access service so they
// share one canonical transactional shape (ADR 0033 γ-factory): a use case only
// sees runInTx(ctx, work) and the *txStores it hands out.
//
// Build it once with NewTxStoreFactory at the wire layer and pass the same value
// to every access service constructor, so adding an Nth repository is a change
// to one constructor call, not several.
type txStoreFactory struct {
	members     MembershipRepository
	invitations InvitationRepository
	audit       auditapp.Recorder
	uow         transaction.UoW
}

// NewTxStoreFactory bundles the access repositories, the audit recorder, and
// the Unit-of-Work into the single txStoreFactory every access service embeds
// (ADR 0033 γ-factory). A nil audit defaults to a Noop recorder so a caller
// that does not care about audit still gets a safe factory. The type stays
// unexported; callers use := to hold it (standard Go pattern for a factory
// returning an unexported type).
func NewTxStoreFactory(
	members MembershipRepository,
	invitations InvitationRepository,
	audit auditapp.Recorder,
	uow transaction.UoW,
) txStoreFactory {
	if audit == nil {
		audit = auditapp.Noop{}
	}
	return txStoreFactory{
		members:     members,
		invitations: invitations,
		audit:       audit,
		uow:         uow,
	}
}

// runInTx opens a Unit-of-Work, builds the access transactional stores from the
// transaction, and runs work with them. UoW commits on nil error and rolls back
// otherwise; a panic in work rolls back and re-panics (see transaction.UoW).
// Both repository WithTx methods are infallible, so unlike identity there are
// no bind errors to wrap here.
//
// runInTx returns an error if the factory's UoW was not configured — a service
// without a UoW has no business calling it. This keeps the call sites free of
// nil checks while making a wiring mistake loud and immediate.
func (f *txStoreFactory) runInTx(ctx context.Context, work func(*txStores) error) error {
	if f.uow == nil {
		return errors.New("access runInTx: Unit-of-Work is not configured")
	}
	return f.uow.Do(ctx, func(tx transaction.Tx) error {
		stores := &txStores{
			members:     f.members.WithTx(tx),
			invitations: f.invitations.WithTx(tx),
			audit:       f.audit.WithTx(tx),
			tx:          tx,
		}
		return work(stores)
	})
}
