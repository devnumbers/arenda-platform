package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
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
	payments      SubscriptionPaymentRepository
	methods       PaymentMethodRepository
	bindings      CardBindingSessionRepository
	audit         auditapp.Recorder
	// Archiver and slots are the cross-context lifecycle bridges (issue #252),
	// bound to the transaction by runLifecycleTx when the worker phases were
	// wired with them; nil keeps the helpers below no-ops.
	archiver ExcessPropertyArchiver
	slots    RecipientSlotEnforcer
}

// archiveExcessProperties archives the owner's active properties beyond the
// limit inside the current transaction and returns the archived ids in
// restoration-priority order (newest first). When keepPropertyID is set
// (issue #617), that property survives as long as it is one of the owner's
// active ones. Without a wired bridge it is a no-op: the subscription-side
// phase still applies, only the excess properties wait (issue #252).
func (s *txStores) archiveExcessProperties(
	ctx context.Context, ownerID uuid.UUID, limit int, keepPropertyID *uuid.UUID,
) ([]uuid.UUID, error) {
	if s.archiver == nil {
		return nil, nil
	}
	return s.archiver.ArchiveExcess(ctx, ownerID, limit, keepPropertyID)
}

// archiveExcessForGraceEntry is the grace-entry tail of ADR 0055: the owner's
// active properties beyond the grace limit of one are archived — the survivor
// is the most recently updated — and the affected recipients' excess shared
// memberships are suspended, inside the caller's transaction. The archived ids
// (restoration-priority order) become the subscription's snapshot: the
// restoration debt the next successful payment settles.
func (s *txStores) archiveExcessForGraceEntry(ctx context.Context, ownerID uuid.UUID) ([]uuid.UUID, error) {
	ids, err := s.archiveExcessProperties(ctx, ownerID, gracePropertyLimit, nil)
	if err != nil {
		return nil, err
	}
	if err := s.enforceRecipientSlots(ctx, ownerID, triggerGraceEntry); err != nil {
		return nil, err
	}
	return ids, nil
}

// restoreGraceArchive unarchives the grace snapshot inside the caller's
// transaction, respecting the given (possibly changed) tariff limit: the ids
// restore in snapshot order while the owner's active count stays under the
// limit, and the affected recipients' memberships follow the properties
// bridge. It returns the ids the bridge could not restore — the debt
// remainder (nil when the snapshot settled). Without a wired bridge the
// snapshot is returned untouched (issue #252 wiring shape).
func (s *txStores) restoreGraceArchive(ctx context.Context, ownerID uuid.UUID, ids []uuid.UUID, limit int) ([]uuid.UUID, error) {
	if s.archiver == nil || len(ids) == 0 {
		return ids, nil
	}
	return s.archiver.RestoreGraceArchive(ctx, ownerID, ids, limit)
}

// enforceRecipientSlots suspends the excess shared memberships after a billing
// limit drop. Without a wired bridge it is a no-op.
func (s *txStores) enforceRecipientSlots(ctx context.Context, userID uuid.UUID, trigger string) error {
	if s.slots == nil {
		return nil
	}
	return s.slots.Enforce(ctx, userID, trigger)
}

// recoverRecipientSlots reactivates the recipient's oldest suspended shared
// memberships FIFO after the tariff limit grew (a recipient upgrade, issue
// #695). Without a wired bridge it is a no-op.
func (s *txStores) recoverRecipientSlots(ctx context.Context, userID uuid.UUID) error {
	if s.slots == nil {
		return nil
	}
	return s.slots.Recover(ctx, userID)
}

// enforceTariffLimit is the shared tail of every worker phase that lowers a
// tariff limit: the owner's excess active properties are archived — keeping
// the keepPropertyID choice when one is given (issue #617) — and the
// affected recipients' excess shared memberships suspended, inside the
// caller's transaction so a bridge failure rolls the whole phase back
// (issue #252).
func (s *txStores) enforceTariffLimit(ctx context.Context, ownerID uuid.UUID, limit int, trigger string, keepPropertyID *uuid.UUID) error {
	if _, err := s.archiveExcessProperties(ctx, ownerID, limit, keepPropertyID); err != nil {
		return err
	}
	return s.enforceRecipientSlots(ctx, ownerID, trigger)
}

// subscriptionForUpdate loads the user's subscription under the row lock and
// narrows the repository miss to ErrSubscriptionNotFound — the shared first
// step of every subscription lifecycle mutation (issue #249).
func (s *txStores) subscriptionForUpdate(ctx context.Context, userID uuid.UUID) (domain.Subscription, error) {
	sub, err := s.subscriptions.GetByUserIDForUpdate(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Subscription{}, ErrSubscriptionNotFound
		}
		return domain.Subscription{}, fmt.Errorf("get subscription: %w", err)
	}
	return sub, nil
}

// lockInSelection locks the user's subscription and reports whether it still
// matches the worker selection that listed it (issue #286). The batch
// predicate lives in SQL, so the under-lock re-check re-lists the selection
// narrowed to the locked user instead of re-stating eligibility in Go; the
// Listing runs in the caller's transaction and therefore observes the locked
// row. An ok=false result means the state the listing saw is gone — the phase's no-op
// signal.
func (s *txStores) lockInSelection(ctx context.Context, userID uuid.UUID, sel SubscriptionSelection) (domain.Subscription, bool, error) {
	sub, err := s.subscriptionForUpdate(ctx, userID)
	if err != nil {
		return domain.Subscription{}, false, err
	}
	sel.UserID = &userID
	sel.Limit = 1
	matching, err := s.subscriptions.List(ctx, sel)
	if err != nil {
		return domain.Subscription{}, false, fmt.Errorf("re-check subscription selection: %w", err)
	}
	if len(matching) == 0 {
		return domain.Subscription{}, false, nil
	}
	return sub, true, nil
}

// paymentForUpdate loads a payment under the row lock and narrows the
// repository miss to ErrPaymentNotFound — the shared first step of every
// payment finalization (issue #250).
func (s *txStores) paymentForUpdate(ctx context.Context, paymentID uuid.UUID) (domain.SubscriptionPayment, error) {
	payment, err := s.payments.GetByIDForUpdate(ctx, paymentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.SubscriptionPayment{}, ErrPaymentNotFound
		}
		return domain.SubscriptionPayment{}, fmt.Errorf("get payment: %w", err)
	}
	return payment, nil
}

// methodForUpdate loads a payment method under the row lock, narrows the
// repository miss to ErrPaymentMethodNotFound and rejects a method of another
// user as a miss too — the shared first step of every payment-method mutation
// (issue #251).
func (s *txStores) methodForUpdate(ctx context.Context, userID, methodID uuid.UUID) (domain.PaymentMethod, error) {
	method, err := s.methods.GetByIDForUpdate(ctx, methodID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.PaymentMethod{}, ErrPaymentMethodNotFound
		}
		return domain.PaymentMethod{}, fmt.Errorf("get payment method: %w", err)
	}
	if method.UserID != userID {
		return domain.PaymentMethod{}, ErrPaymentMethodNotFound
	}
	return method, nil
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
	payments      SubscriptionPaymentRepository
	methods       PaymentMethodRepository
	bindings      CardBindingSessionRepository
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
	payments SubscriptionPaymentRepository,
	methods PaymentMethodRepository,
	bindings CardBindingSessionRepository,
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
		payments:      payments,
		methods:       methods,
		bindings:      bindings,
		audit:         audit,
		uow:           uow,
	}
}

// runInTx opens a Unit-of-Work, builds the billing transactional stores from
// the transaction, and runs work with them. UoW commits on nil error and rolls
// back otherwise; a panic in work rolls back and re-panics (see
// transaction.UoW).
//
// The runInTx call returns an error if the factory's UoW was not configured — a service
// without a UoW has no business calling it. This keeps the call sites free of
// nil checks while making a wiring mistake loud and immediate.
func (f *txStoreFactory) runInTx(ctx context.Context, work func(*txStores) error) error {
	return f.runInTxWithBridges(ctx, nil, nil, work)
}

// runInTxWithBridges is runInTx extended with the optional cross-context
// lifecycle bridges: the worker phases that change a tariff limit run the
// subscription change, the excess-property archiving and the recipient-slot
// enforcement in one transaction, so a bridge failure rolls the whole phase
// back and the next tick retries it (issue #252).
func (f *txStoreFactory) runInTxWithBridges(
	ctx context.Context,
	archiver ExcessPropertyArchiverSource,
	slots RecipientSlotEnforcerSource,
	work func(*txStores) error,
) error {
	if f.uow == nil {
		return errors.New("billing runInTx: Unit-of-Work is not configured")
	}
	return f.uow.Do(ctx, func(tx transaction.Tx) error {
		stores, err := f.buildTxStores(tx, archiver, slots)
		if err != nil {
			return err
		}
		return work(stores)
	})
}

// buildTxStores binds every billing repository and the audit recorder to the
// transaction, plus the lifecycle bridges when their sources are given.
// WithTx errors are wrapped so a repository that fails to bind aborts the transaction
// with a clear cause rather than a silent fallthrough. Audit is bound last:
// its WithTx is infallible (auditapp.Recorder.WithTx returns no error), so it
// cannot mask a prior repository-bind failure.
func (f *txStoreFactory) buildTxStores(
	tx transaction.Tx,
	archiver ExcessPropertyArchiverSource,
	slots RecipientSlotEnforcerSource,
) (*txStores, error) {
	tariffs, err := f.tariffs.WithTx(tx)
	if err != nil {
		return nil, fmt.Errorf("bind tariff repository to tx: %w", err)
	}
	subscriptions, err := f.subscriptions.WithTx(tx)
	if err != nil {
		return nil, fmt.Errorf("bind subscription repository to tx: %w", err)
	}
	transitions, err := f.transitions.WithTx(tx)
	if err != nil {
		return nil, fmt.Errorf("bind transition repository to tx: %w", err)
	}
	payments, err := f.payments.WithTx(tx)
	if err != nil {
		return nil, fmt.Errorf("bind payment repository to tx: %w", err)
	}
	methods, err := f.methods.WithTx(tx)
	if err != nil {
		return nil, fmt.Errorf("bind payment-method repository to tx: %w", err)
	}
	bindings, err := f.bindings.WithTx(tx)
	if err != nil {
		return nil, fmt.Errorf("bind card-binding repository to tx: %w", err)
	}
	stores := &txStores{
		tariffs:       tariffs,
		subscriptions: subscriptions,
		transitions:   transitions,
		payments:      payments,
		methods:       methods,
		bindings:      bindings,
		audit:         f.audit.WithTx(tx),
	}
	if archiver != nil {
		bound, err := archiver.WithTx(tx)
		if err != nil {
			return nil, fmt.Errorf("bind property archiver to tx: %w", err)
		}
		stores.archiver = bound
	}
	if slots != nil {
		bound, err := slots.WithTx(tx)
		if err != nil {
			return nil, fmt.Errorf("bind recipient-slot enforcer to tx: %w", err)
		}
		stores.slots = bound
	}
	return stores, nil
}
