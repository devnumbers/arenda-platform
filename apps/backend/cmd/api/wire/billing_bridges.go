package wire

import (
	"context"

	"github.com/google/uuid"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// Composition-root glue between the billing worker phases and the properties
// and access contexts (issue #252): the expiry and downgrade phases consume
// the consumer-side bridge ports the billing application declares (ADR 0035),
// and these adapters — the only place both sides meet — fit the real services
// to them. Keeping the glue here means the billing application never imports
// properties or access.

// propertyArchiverSource adapts the properties service to the billing
// archiver source.
type propertyArchiverSource struct {
	svc *propertiesapp.PropertyService
}

// NewPropertyArchiverSource wraps the properties service as the billing
// worker's excess-property archiver source.
func NewPropertyArchiverSource(svc *propertiesapp.PropertyService) billingapp.ExcessPropertyArchiverSource {
	return propertyArchiverSource{svc: svc}
}

// WithTx binds the archiver to the worker phase's transaction, so archiving
// lands in the same commit as the subscription change.
func (s propertyArchiverSource) WithTx(tx transaction.Tx) (billingapp.ExcessPropertyArchiver, error) {
	return propertyArchiver{svc: s.svc, tx: tx}, nil
}

// propertyArchiver runs the properties context's archiver on the bound
// transaction: active properties beyond the limit are archived, keeping the
// newest, and the ids a grace entry archived are restored under the tariff
// limit (ADR 0055).
type propertyArchiver struct {
	svc *propertiesapp.PropertyService
	tx  transaction.Tx
}

// ArchiveExcess archives the owner's active properties beyond the limit and
// returns their ids, newest first.
func (a propertyArchiver) ArchiveExcess(ctx context.Context, ownerID uuid.UUID, limit int, keepPropertyID *uuid.UUID) ([]uuid.UUID, error) {
	return a.svc.ArchiveExcessProperties(ctx, a.tx, ownerID, limit, keepPropertyID)
}

// RestoreGraceArchive unarchives the grace snapshot ids while the owner's
// active count stays under the limit and returns the debt remainder.
func (a propertyArchiver) RestoreGraceArchive(ctx context.Context, ownerID uuid.UUID, ids []uuid.UUID, limit int) ([]uuid.UUID, error) {
	return a.svc.RestoreGraceArchivedProperties(ctx, a.tx, ownerID, ids, limit)
}

// recipientSlotSource adapts the access SlotCoordinator to the billing
// enforcer source.
type recipientSlotSource struct {
	coordinator *accessapp.SlotCoordinator
}

// NewRecipientSlotSource wraps the access SlotCoordinator as the billing
// worker's recipient-slot enforcer source.
func NewRecipientSlotSource(coordinator *accessapp.SlotCoordinator) billingapp.RecipientSlotEnforcerSource {
	return recipientSlotSource{coordinator: coordinator}
}

// WithTx binds the enforcer to the worker phase's transaction.
func (s recipientSlotSource) WithTx(tx transaction.Tx) (billingapp.RecipientSlotEnforcer, error) {
	return recipientSlotEnforcer{coordinator: s.coordinator, tx: tx}, nil
}

// recipientSlotEnforcer suspends the excess shared memberships after a billing
// limit drop and recovers the suspended ones FIFO when the limit expands, on
// the bound transaction.
type recipientSlotEnforcer struct {
	coordinator *accessapp.SlotCoordinator
	tx          transaction.Tx
}

// Enforce suspends the recipient's excess shared memberships.
func (e recipientSlotEnforcer) Enforce(ctx context.Context, userID uuid.UUID, trigger string) error {
	return e.coordinator.EnforceRecipientLimit(ctx, e.tx, userID, trigger)
}

// Recover reactivates the recipient's oldest suspended shared memberships
// FIFO (issue #695).
func (e recipientSlotEnforcer) Recover(ctx context.Context, userID uuid.UUID) error {
	return e.coordinator.RecoverSuspended(ctx, e.tx, userID)
}

// ActivePropertyExists reports whether the property belongs to the owner and
// still occupies a tariff slot — the keep-choice validation read of the
// cancel flow (issue #617).
func (a propertyArchiver) ActivePropertyExists(ctx context.Context, ownerID, propertyID uuid.UUID) (bool, error) {
	return a.svc.ActivePropertyExists(ctx, a.tx, ownerID, propertyID)
}
