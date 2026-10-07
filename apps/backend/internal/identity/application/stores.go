package application

import (
	"context"
	"errors"
	"fmt"

	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// txStores bundles the identity repositories and audit recorder all bound to
// the same transaction. It is the only handle a use case receives inside
// runInTx, so it is impossible to forget WithTx or to record audit outside the
// transaction (ADR 0033, ADR 0020).
type txStores struct {
	users    UserRepository
	codes    LoginCodeRepository
	attempts AttemptRepository
	sessions SessionRepository
	grants   EmailChangeGrantRepository
	audit    auditapp.Recorder
	// Notifier schedules the contact-change security letter inside this same
	// transaction (решение #1207): the job commits together with the change.
	notifier TransactionalContactChangeNotifier
}

// txStoreFactory holds the non-transactional identity repositories and audit
// recorder plus the Unit-of-Work, and builds a transactional txStores from each
// runInTx call. It is embedded anonymously by every identity service so they
// share one canonical transactional shape (ADR 0033 γ-factory): a use case only
// sees runInTx(ctx, work) and the *txStores it hands out.
//
// Build it once with NewTxStoreFactory at the wire layer and pass the same value
// to every identity service constructor, so adding an Nth repository is a change
// to one constructor call, not six.
type txStoreFactory struct {
	users    UserRepository
	codes    LoginCodeRepository
	attempts AttemptRepository
	sessions SessionRepository
	grants   EmailChangeGrantRepository
	audit    auditapp.Recorder
	// ContactNotifier is the change-letter queue seam; the River adapter binds
	// its client late (identity builds before the delivery queue — the
	// grace-events canon), the per-transaction binding happens in runInTx.
	contactNotifier ContactChangeNotifier
	uow             transaction.UoW
}

// NewTxStoreFactory bundles the four identity repositories, the audit recorder,
// the change-letter queue seam, and the Unit-of-Work into the single
// txStoreFactory every identity service embeds (ADR 0033 γ-factory). A nil
// audit defaults to a Noop recorder and a nil contactNotifier to
// ContactChangeNoop, so a caller that does not care about either still gets a
// safe factory. The type stays unexported; callers use := to hold it (standard
// Go pattern for a factory returning an unexported type).
func NewTxStoreFactory(
	users UserRepository,
	codes LoginCodeRepository,
	attempts AttemptRepository,
	sessions SessionRepository,
	grants EmailChangeGrantRepository,
	audit auditapp.Recorder,
	contactNotifier ContactChangeNotifier,
	uow transaction.UoW,
) txStoreFactory {
	if audit == nil {
		audit = auditapp.Noop{}
	}
	if contactNotifier == nil {
		contactNotifier = ContactChangeNoop{}
	}
	return txStoreFactory{
		users:           users,
		codes:           codes,
		attempts:        attempts,
		sessions:        sessions,
		grants:          grants,
		audit:           audit,
		contactNotifier: contactNotifier,
		uow:             uow,
	}
}

// runInTx opens a Unit-of-Work, builds the identity transactional stores from
// the transaction, and runs work with them. UoW commits on nil error and rolls
// back otherwise; a panic in work rolls back and re-panics (see transaction.UoW).
//
// WithTx errors are wrapped so a repository that fails to bind aborts the
// transaction with a clear cause rather than a silent fallthrough. The audit
// recorder is bound last: its WithTx is infallible (auditapp.Recorder.WithTx
// returns no error), so it cannot mask a prior repository-bind failure.
//
// The runInTx call returns an error if the factory's UoW was not configured —
// a service without a UoW has no business calling it. This keeps the call
// sites free of nil checks while making a wiring mistake loud and immediate.
func (f *txStoreFactory) runInTx(ctx context.Context, work func(*txStores) error) error {
	if f.uow == nil {
		return errors.New("identity runInTx: Unit-of-Work is not configured")
	}
	return f.uow.Do(ctx, func(tx transaction.Tx) error {
		users, err := f.users.WithTx(tx)
		if err != nil {
			return fmt.Errorf("bind user repository to tx: %w", err)
		}
		codes, err := f.codes.WithTx(tx)
		if err != nil {
			return fmt.Errorf("bind login code repository to tx: %w", err)
		}
		attempts, err := f.attempts.WithTx(tx)
		if err != nil {
			return fmt.Errorf("bind attempt repository to tx: %w", err)
		}
		sessions, err := f.sessions.WithTx(tx)
		if err != nil {
			return fmt.Errorf("bind session repository to tx: %w", err)
		}
		grants, err := f.grants.WithTx(tx)
		if err != nil {
			return fmt.Errorf("bind email change grant repository to tx: %w", err)
		}
		stores := &txStores{
			users:    users,
			codes:    codes,
			attempts: attempts,
			sessions: sessions,
			grants:   grants,
			audit:    f.audit.WithTx(tx),
			notifier: f.contactNotifier.WithTx(tx),
		}
		return work(stores)
	})
}
