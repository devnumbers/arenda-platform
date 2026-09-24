package application

import (
	"context"
	"errors"
	"fmt"

	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// txStores bundles the contact repository and the audit and history recorders
// bound to the same transaction. It is the only handle a use case receives
// inside runInTx, so it is impossible to forget WithTx, to write outside the
// transaction or to record audit or history outside it (ADR 0033, ADR 0020,
// ADR 0061).
type txStores struct {
	contacts ContactStore
	audit    auditapp.Recorder
	history  historyapp.Recorder
}

// txStoreFactory holds the non-transactional repositories, the audit and
// history recorders plus the Unit-of-Work, and builds a transactional
// txStores from each runInTx call (ADR 0033 γ-factory). The non-transactional
// contact and property stores also serve the pre-transaction reads the
// authorization needs; the property store never enters a contact transaction.
type txStoreFactory struct {
	contacts   ContactStore
	properties PropertyStore
	audit      auditapp.Recorder
	history    historyapp.Recorder
	uow        transaction.UoW
}

// NewTxStoreFactory bundles the contact and property repositories, the audit
// and history recorders, and the Unit-of-Work into the single factory the
// contacts service embeds. A nil recorder defaults to a Noop so a caller that
// does not care about audit or history still gets a safe factory. The type
// stays unexported; callers use := to hold it (standard Go pattern for a
// factory returning an unexported type).
func NewTxStoreFactory(
	contacts ContactStore, properties PropertyStore,
	audit auditapp.Recorder, history historyapp.Recorder, uow transaction.UoW,
) txStoreFactory {
	if audit == nil {
		audit = auditapp.Noop{}
	}
	if history == nil {
		history = historyapp.Noop{}
	}
	return txStoreFactory{contacts: contacts, properties: properties, audit: audit, history: history, uow: uow}
}

// runInTx opens a Unit-of-Work, builds the transactional stores from the
// transaction, and runs work with them. UoW commits on nil error and rolls
// back otherwise; a panic in work rolls back and re-panics. A missing UoW is
// a wiring mistake and fails loudly and immediately.
func (f *txStoreFactory) runInTx(ctx context.Context, work func(*txStores) error) error {
	if f.uow == nil {
		return errors.New("contacts runInTx: Unit-of-Work is not configured")
	}
	return f.uow.Do(ctx, func(tx transaction.Tx) error {
		contacts, err := f.contacts.WithTx(tx)
		if err != nil {
			return fmt.Errorf("bind contact store to tx: %w", err)
		}
		return work(&txStores{contacts: contacts, audit: f.audit.WithTx(tx), history: f.history.WithTx(tx)})
	})
}
