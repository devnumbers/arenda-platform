package application

import (
	"context"
	"errors"
	"fmt"

	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// txStores bundles the properties repositories, the cross-context ports and
// the audit recorder all bound to the same transaction. It is the only handle
// a use case receives inside runInTx, so it is impossible to forget WithTx or
// to record audit outside the transaction (ADR 0033, ADR 0020).
//
// The optional stores (photos, limiter) cover what the property service uses
// beyond the main repository; runInTx skips binding an unwired optional store
// instead of panicking on a nil WithTx — same shape as the billing lifecycle
// bridges — which keeps a test or a future wiring free to supply only the
// stores its use cases reach.
//
// The tx field additionally exposes the raw transaction handle. Two
// cross-context ports take a transaction.Tx because the access context
// implements them over the caller's transaction: RecipientSlotPolicy
// (slot enforcement and recovery) and SharedMembersDeleteMailer
// (CollectFormerMemberEmails inside the delete transaction) — a use case
// passes stores.tx through when it calls either in its transaction.
type txStores struct {
	repo    PropertyRepository
	photos  PropertyPhotoRepository
	limiter SubscriptionLimiter
	audit   auditapp.Recorder
	tx      transaction.Tx
}

// txStoreFactory holds the non-transactional properties repositories, the
// cross-context ports and the audit recorder plus the Unit-of-Work, and builds
// a transactional txStores from each runInTx call. It is embedded anonymously
// by the property service so the context shares one canonical transactional
// shape (ADR 0033 γ-factory): a use case only sees runInTx(ctx, work) and the
// *txStores it hands out.
//
// Build it once with NewTxStoreFactory at the wire layer, so adding an Nth
// repository is a change to one constructor call, not several.
type txStoreFactory struct {
	repo    PropertyRepository
	photos  PropertyPhotoRepository
	limiter SubscriptionLimiter
	audit   auditapp.Recorder
	uow     transaction.UoW
}

// NewTxStoreFactory bundles the properties repositories, the cross-context
// ports, the audit recorder, and the Unit-of-Work into the single
// txStoreFactory the property service embeds (ADR 0033 γ-factory). A nil
// audit defaults to a Noop recorder so a caller that does not care about audit
// still gets a safe factory; the optional stores may be nil (see txStores).
// The type stays unexported; callers use := to hold it (standard Go pattern
// for a factory returning an unexported type).
func NewTxStoreFactory(
	repo PropertyRepository,
	photos PropertyPhotoRepository,
	limiter SubscriptionLimiter,
	audit auditapp.Recorder,
	uow transaction.UoW,
) txStoreFactory {
	if audit == nil {
		audit = auditapp.Noop{}
	}
	return txStoreFactory{
		repo:    repo,
		photos:  photos,
		limiter: limiter,
		audit:   audit,
		uow:     uow,
	}
}

// runInTx opens a Unit-of-Work, builds the properties transactional stores
// from the transaction, and runs work with them. UoW commits on nil error and
// rolls back otherwise; a panic in work rolls back and re-panics (see
// transaction.UoW).
//
// The subscription limiter is the only store with a fallible WithTx; its bind
// error is wrapped so a limiter that fails to bind aborts the transaction with
// a clear cause rather than a silent fallthrough. The audit store is bound
// last: its WithTx is infallible (auditapp.Recorder.WithTx returns no error),
// so it cannot mask a prior bind failure.
//
// An error is returned if the factory's UoW was not configured — a service
// without a UoW has no business calling this method. This keeps the call
// sites free of nil checks while making a wiring mistake loud and immediate.
func (f *txStoreFactory) runInTx(ctx context.Context, work func(*txStores) error) error {
	if f.uow == nil {
		return errors.New("properties runInTx: Unit-of-Work is not configured")
	}
	return f.uow.Do(ctx, func(tx transaction.Tx) error {
		stores := &txStores{
			repo:  f.repo.WithTx(tx),
			audit: f.audit.WithTx(tx),
			tx:    tx,
		}
		if f.photos != nil {
			stores.photos = f.photos.WithTx(tx)
		}
		var err error
		if f.limiter != nil {
			stores.limiter, err = f.limiter.WithTx(tx)
			if err != nil {
				return fmt.Errorf("bind subscription limiter to tx: %w", err)
			}
		}
		return work(stores)
	})
}
