package application

// The archive bridge (тикет #1342): the system pause the properties context
// places on a property's payment rules when the object enters the archive,
// and its reverse leg when the object returns. The bridge writes the
// payments stores directly — past the mutation conveyor — by construction:
// the archived property fails the conveyor's lockActiveProperty gate, and
// the pause is the payments side's reaction to the object's lifecycle, not
// a rule edit (ADR 0053 §3 — the rent-managed gate does not apply either).
// The port is consumer-declared on the properties side (the
// RentalDeletionGuard shape: both methods take the caller's transaction).
// Scope note (тикет #1342): the pause leg rides the owner's ArchiveProperty
// only — the billing excess-archive (ADR 0055) keeps its pre-ticket
// behavior; the resume leg runs on BOTH restore paths (the manual unarchive
// and the grace restore), so a property restored with stranded auto_archive
// pauses cannot happen regardless of who archived it.

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// Compile-time conformance of the bridge to the consumer-declared port
// (CODING_STANDARDS: ports get static conformance assertions).
var _ propertiesapp.ArchivePausesBridge = (*ArchivePauseBridge)(nil)

// ArchivePauseBridge is the properties context's seam to the payments
// stores. The today comes from the data owner's calendar (ADR 0048) — the
// same boundary the manual pause mutations use.
type ArchivePauseBridge struct {
	factory  txStoreFactory
	calendar OwnerCalendar
}

// NewArchivePauseBridge builds the bridge over the shared payments factory
// and the owner calendar.
func NewArchivePauseBridge(factory txStoreFactory, calendar OwnerCalendar) *ArchivePauseBridge {
	return &ArchivePauseBridge{factory: factory, calendar: calendar}
}

// PauseAllForArchive places the open-ended auto_archive pause on every live
// rule of the property inside the caller's transaction: the completed ones
// are terminal (nothing left to pause), the rules already holding an open
// pause keep theirs — a manual pause outranks the archive round-trip.
// Rent-managed rules pause like every other: the rental's materialization
// stops with the archived object all the same (the tick skips archived
// properties). The change-log rows and the action journal rows carry the
// origin mark; a failure rolls the whole archive back with it.
func (b *ArchivePauseBridge) PauseAllForArchive(
	ctx context.Context, tx transaction.Tx, scope, propertyID, actorID uuid.UUID, role sharedpolicy.Role,
) error {
	leg, err := b.bind(tx)
	if err != nil {
		return err
	}
	return leg.pauseAll(ctx, scope, propertyID, actorID, role)
}

// ResumeArchivePauses closes only the open auto_archive pauses inside the
// caller's transaction — the property returned from the archive; the
// owner's manual pauses stay. The change-log rows and the action journal
// rows carry the origin mark.
func (b *ArchivePauseBridge) ResumeArchivePauses(
	ctx context.Context, tx transaction.Tx, scope, propertyID, actorID uuid.UUID, role sharedpolicy.Role,
) error {
	leg, err := b.bind(tx)
	if err != nil {
		return err
	}
	return leg.resumeAuto(ctx, scope, propertyID, actorID, role)
}

// bind wires the bridge's transaction-bound stores.
func (b *ArchivePauseBridge) bind(tx transaction.Tx) (*archivePauseLeg, error) {
	payments, err := b.factory.payments.WithTx(tx)
	if err != nil {
		return nil, fmt.Errorf("bind payment store to tx: %w", err)
	}
	changeLog, err := b.factory.changeLog.WithTx(tx)
	if err != nil {
		return nil, fmt.Errorf("bind change log store to tx: %w", err)
	}
	return &archivePauseLeg{
		payments:  payments,
		changeLog: changeLog,
		history:   b.factory.history.WithTx(tx),
		calendar:  b.calendar,
	}, nil
}

// archivePauseLeg carries the transaction-bound stores of one bridge call.
// The audit stays silent: the archive entry the properties context records
// is the audit trail of the whole change.
type archivePauseLeg struct {
	payments  PaymentStore
	changeLog PaymentChangeLogStore
	history   historyapp.Recorder
	calendar  OwnerCalendar
}

func (t *archivePauseLeg) pauseAll(
	ctx context.Context, scope, propertyID, actorID uuid.UUID, role sharedpolicy.Role,
) error {
	today, err := ownerToday(t.calendar, ctx, scope)
	if err != nil {
		return err
	}
	rules, err := t.payments.ListByProperty(ctx, scope, propertyID, "")
	if err != nil {
		return fmt.Errorf("list property payments: %w", err)
	}
	historyRole := sharedpolicy.HistoryActorRole(role)
	for _, rule := range rules {
		// The completed rule generates nothing already (тикет #1340) — no
		// pause to place; ListByProperty filters them out anyway.
		if _, active := domain.ActivePause(rule.Pauses); active {
			// An open pause keeps its origin: the owner's manual pause
			// outranks the archive round-trip, an auto one cannot exist
			// here (the bridge closes its own on every return).
			continue
		}
		if err := t.payments.InsertPause(ctx, rule.ID, today, domain.PauseOriginAutoArchive); err != nil {
			return fmt.Errorf("insert auto-archive pause of payment %s: %w", rule.ID, err)
		}
		// The system row of the change log: the paused flip with the origin
		// mark, the empty diff is the content (ADR 0065 §4).
		if err := t.changeLog.Record(ctx, scope, propertyID, actorID, ChangeLogWrite{
			PaymentID: rule.ID,
			Action:    domain.ChangePaused,
			Origin:    domain.PauseOriginAutoArchive,
		}); err != nil {
			return fmt.Errorf("record auto-archive pause change log: %w", err)
		}
		if err := historyapp.RecordScoped(ctx, t.history, propertyID, actorID, historyRole,
			historydomain.PaymentPausedByArchive(rule.ID, rule.Title)); err != nil {
			return err
		}
	}
	return nil
}

func (t *archivePauseLeg) resumeAuto(
	ctx context.Context, scope, propertyID, actorID uuid.UUID, role sharedpolicy.Role,
) error {
	today, err := ownerToday(t.calendar, ctx, scope)
	if err != nil {
		return err
	}
	rules, err := t.payments.ListByProperty(ctx, scope, propertyID, "")
	if err != nil {
		return fmt.Errorf("list property payments: %w", err)
	}
	historyRole := sharedpolicy.HistoryActorRole(role)
	for _, rule := range rules {
		active, has := domain.ActivePause(rule.Pauses)
		if !has || active.Origin != domain.PauseOriginAutoArchive {
			// No open pause or the owner's manual one — the manual pause
			// stays open across the archive round-trip (тикет #1342).
			continue
		}
		if err := t.payments.CloseAutoArchivePause(ctx, rule.ID, today); err != nil {
			return fmt.Errorf("close auto-archive pause of payment %s: %w", rule.ID, err)
		}
		if err := t.changeLog.Record(ctx, scope, propertyID, actorID, ChangeLogWrite{
			PaymentID: rule.ID,
			Action:    domain.ChangeResumed,
			Origin:    domain.PauseOriginAutoArchive,
		}); err != nil {
			return fmt.Errorf("record auto-archive resume change log: %w", err)
		}
		if err := historyapp.RecordScoped(ctx, t.history, propertyID, actorID, historyRole,
			historydomain.PaymentResumedByArchive(rule.ID, rule.Title)); err != nil {
			return err
		}
	}
	return nil
}
