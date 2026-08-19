package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	domain "github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// SlotCoordinator is the single engine that enforces the recipient tariff slot
// invariant across all sources of suspended access: an activation without a free
// slot, a tariff downgrade / grace expiry (billing limit drop), and the unarchive
// of a shared object. It also recovers suspended memberships FIFO whenever a slot
// frees up (revoke, self-exit, owner archive/delete, recipient upgrade).
//
// The recipient tariff limit is a single pool: the recipient's own active
// properties plus the active shared memberships they hold (across all owners).
// When the pool exceeds the limit, the worst-fit candidates are evicted: own
// objects are auto-archived by the properties PropertyArchiver in the same
// transaction, shared memberships are suspended by this coordinator. When a slot
// frees, the oldest suspended shared memberships are reactivated.
//
// See issue #158 (T4) and PRD #153.
type SlotCoordinator struct {
	members    MembershipRepository
	owners     PropertyOwnerResolver
	limiter    RecipientLimiter
	occupancy  OccupancyPort
	ownedProps OwnedActivePropertiesPort
	lifecycle  *LifecycleMailer
	audit      auditapp.Recorder
	db         txBeginner
}

// NewSlotCoordinator creates a SlotCoordinator. The recorder and limiter are
// expected to be transaction-aware (the coordinator binds them to the caller's
// tx per operation). Lifecycle is the sharing lifecycle mailer (issue #162,
// T6); it may be nil to disable the lifecycle emails. The coordinator runs
// inside the caller's transaction, so its emails are sent in-transaction: a
// send failure is logged and never rolls the operation back.
func NewSlotCoordinator(
	members MembershipRepository,
	owners PropertyOwnerResolver,
	limiter RecipientLimiter,
	occupancy OccupancyPort,
	ownedProps OwnedActivePropertiesPort,
	lifecycle *LifecycleMailer,
	audit auditapp.Recorder,
	db txBeginner,
) *SlotCoordinator {
	return &SlotCoordinator{
		members:    members,
		owners:     owners,
		limiter:    limiter,
		occupancy:  occupancy,
		ownedProps: ownedProps,
		lifecycle:  lifecycle,
		audit:      audit,
		db:         db,
	}
}

// EnforceRecipientLimit enforces the recipient tariff slot invariant after a
// billing limit drop of userID (a tariff downgrade, grace expiry, or
// subscription cancellation). Billing calls it with the downgrading user's own
// id (sub.UserID), with the billing transaction already open
// (PropertyArchiver.ArchiveExcessProperties runs in the same tx to handle the
// recipient's own excess objects).
//
// Two groups are affected by the limit drop and both are enforced here:
//   - the downgrading user himself, as a recipient of shared memberships on
//     other owners' objects (the owner is never a membership row of his own
//     object, so he never appears in the member listing below);
//   - every recipient that holds an active shared membership on one of the
//     downgrading user's properties.
//
// For each affected recipient the coordinator recomputes the full pool (own
// active properties + all active shared memberships across every owner) and
// suspends the excess shared memberships. Own objects are skipped here: they
// are archived by the PropertyArchiver in the same transaction.
//
// Trigger is a low-cardinality label recorded in the audit context (e.g.
// "downgrade", "grace_expired", "subscription_cancelled").
func (c *SlotCoordinator) EnforceRecipientLimit(ctx context.Context, tx transaction.Tx, userID uuid.UUID, trigger string) error {
	txMembers := c.members.WithTx(tx)

	affected, err := txMembers.ListActiveByPropertyOwner(ctx, userID)
	if err != nil {
		return fmt.Errorf("list affected memberships: %w", err)
	}

	// Group affected memberships by recipient — a recipient with several
	// memberships on this owner's properties is evaluated once. The downgrading
	// user is a recipient too: his shared memberships on other owners' objects
	// compete for his own tariff slots.
	recipients := make(map[uuid.UUID]struct{}, len(affected)+1)
	for _, m := range affected {
		recipients[m.UserID] = struct{}{}
	}
	recipients[userID] = struct{}{}

	txLimiter, err := c.limiter.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind limiter tx: %w", err)
	}

	for recipientID := range recipients {
		if err := c.enforceRecipient(ctx, tx, txMembers, txLimiter, recipientID, trigger); err != nil {
			return err
		}
	}
	return nil
}

// enforceRecipient rebuilds the full pool for one recipient and suspends excess
// shared memberships.
func (c *SlotCoordinator) enforceRecipient(
	ctx context.Context,
	tx transaction.Tx,
	txMembers MembershipRepository,
	txLimiter RecipientLimiter,
	recipientID uuid.UUID,
	trigger string,
) error {
	txOwned, err := c.ownedProps.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind owned-props tx: %w", err)
	}
	pool, err := c.buildRecipientPool(ctx, txOwned, txMembers, recipientID)
	if err != nil {
		return err
	}

	limit, err := txLimiter.ActivePropertyLimit(ctx, recipientID)
	if err != nil {
		return fmt.Errorf("recipient limit: %w", err)
	}

	toEvict := SelectForEviction(pool, limit)
	suspended := make([]uuid.UUID, 0, len(toEvict))
	for _, cand := range toEvict {
		// Own objects are auto-archived by PropertyArchiver in the same tx; the
		// coordinator only suspends shared memberships.
		if !cand.IsShared {
			continue
		}
		if err := txMembers.Suspend(ctx, cand.MemberID, cand.PropertyID); err != nil {
			return fmt.Errorf("suspend membership %s: %w", cand.MemberID, err)
		}
		suspended = append(suspended, cand.PropertyID)
		propertyID := cand.PropertyID
		if err := c.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
			ActorRole:  auditdomain.ActorRoleSystem,
			Action:     auditdomain.ActionPropertyMemberSuspended,
			EntityType: auditdomain.EntityPropertyMember,
			EntityID:   &cand.MemberID,
			Context: map[string]any{
				auditKeyTrigger:    trigger,
				auditKeyPropertyID: propertyID,
				auditKeyUserID:     recipientID,
			},
		}); err != nil {
			return fmt.Errorf("record suspend audit: %w", err)
		}
	}
	// One summary email per recipient lists the memberships suspended by this
	// call (issue #162, T6); it replaces the per-membership waiting email on
	// the downgrade path.
	c.lifecycle.SendDowngradeSummary(ctx, recipientID, suspended)
	return nil
}

// EnforceOnActivation reports whether a new membership for recipientID must be
// created in the suspended state. It is called by AccessService.AddMember
// before the membership row is inserted. It returns suspend=true when the
// recipient's pool is already at or above the limit, so the new grant does not
// displace an existing active object.
func (c *SlotCoordinator) EnforceOnActivation(ctx context.Context, tx transaction.Tx, recipientID uuid.UUID) (bool, error) {
	txMembers := c.members.WithTx(tx)

	txLimiter, err := c.limiter.WithTx(tx)
	if err != nil {
		return false, fmt.Errorf("bind limiter tx: %w", err)
	}
	limit, err := txLimiter.ActivePropertyLimit(ctx, recipientID)
	if err != nil {
		return false, fmt.Errorf("recipient limit: %w", err)
	}

	txOwned, err := c.ownedProps.WithTx(tx)
	if err != nil {
		return false, fmt.Errorf("bind owned-props tx: %w", err)
	}
	used, err := c.usedSlots(ctx, txOwned, txMembers, recipientID)
	if err != nil {
		return false, err
	}
	// Used >= limit means there is no free slot for the new membership.
	return used >= limit, nil
}

// poolOverLimit reports whether the recipient's current pool (own active
// properties plus active shared memberships) strictly exceeds the tariff
// limit. Unlike EnforceOnActivation, the object under evaluation is already
// part of the pool, so a pool exactly at the limit is not over.
func (c *SlotCoordinator) poolOverLimit(
	ctx context.Context,
	tx transaction.Tx,
	txMembers MembershipRepository,
	recipientID uuid.UUID,
) (bool, error) {
	txLimiter, err := c.limiter.WithTx(tx)
	if err != nil {
		return false, fmt.Errorf("bind limiter tx: %w", err)
	}
	limit, err := txLimiter.ActivePropertyLimit(ctx, recipientID)
	if err != nil {
		return false, fmt.Errorf("recipient limit: %w", err)
	}

	txOwned, err := c.ownedProps.WithTx(tx)
	if err != nil {
		return false, fmt.Errorf("bind owned-props tx: %w", err)
	}
	used, err := c.usedSlots(ctx, txOwned, txMembers, recipientID)
	if err != nil {
		return false, err
	}
	return used > limit, nil
}

// RecoverSuspended reactivates the oldest suspended memberships of recipientID
// when a slot has freed up. It is called whenever a recipient slot is released
// (member revoke, self-exit, owner archive/delete) or expanded (recipient
// upgrade). Reactivation is FIFO by the suspension moment.
func (c *SlotCoordinator) RecoverSuspended(ctx context.Context, tx transaction.Tx, recipientID uuid.UUID) error {
	return c.recoverSuspendedForRecipient(ctx, tx, recipientID)
}

// recoverSuspendedForRecipient is the shared recovery body, factored out so the
// per-property variant can reuse it without re-resolving the limiter per
// recipient.
func (c *SlotCoordinator) recoverSuspendedForRecipient(ctx context.Context, tx transaction.Tx, recipientID uuid.UUID) error {
	txMembers := c.members.WithTx(tx)

	suspended, err := txMembers.ListSuspendedByUser(ctx, recipientID)
	if err != nil {
		return fmt.Errorf("list suspended memberships: %w", err)
	}
	if len(suspended) == 0 {
		return nil
	}

	txLimiter, err := c.limiter.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind limiter tx: %w", err)
	}
	limit, err := txLimiter.ActivePropertyLimit(ctx, recipientID)
	if err != nil {
		return fmt.Errorf("recipient limit: %w", err)
	}

	txOwned, err := c.ownedProps.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind owned-props tx: %w", err)
	}

	used, err := c.usedSlots(ctx, txOwned, txMembers, recipientID)
	if err != nil {
		return err
	}
	freeSlots := limit - used
	if freeSlots <= 0 {
		return nil
	}

	// Occupancy for recovery: group suspended memberships' properties by their
	// data owner, fetch each owner's occupied set once, then merge.
	occupied, err := collectSharedOccupancy(ctx, suspended, c.owners, c.occupancy)
	if err != nil {
		return fmt.Errorf("collect shared occupancy: %w", err)
	}

	candidates := make([]SlotCandidate, 0, len(suspended))
	for _, m := range suspended {
		candidates = append(candidates, SlotCandidate{
			PropertyID:   m.PropertyID,
			IsShared:     true,
			MemberID:     m.ID,
			RecipientID:  recipientID,
			HasOpenLease: occupied[m.PropertyID],
			UpdatedAt:    m.UpdatedAt,
			SuspendedAt:  deref(m.SuspendedAt),
		})
	}

	toRecover := SelectForRecovery(candidates, freeSlots)
	for _, cand := range toRecover {
		if _, err := txMembers.Reactivate(ctx, cand.MemberID, cand.PropertyID); err != nil {
			return fmt.Errorf("reactivate membership %s: %w", cand.MemberID, err)
		}
		if err := c.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
			ActorRole:  auditdomain.ActorRoleSystem,
			Action:     auditdomain.ActionPropertyMemberReactivated,
			EntityType: auditdomain.EntityPropertyMember,
			EntityID:   &cand.MemberID,
			Context: map[string]any{
				auditKeyPropertyID: cand.PropertyID,
				auditKeyUserID:     recipientID,
			},
		}); err != nil {
			return fmt.Errorf("record reactivate audit: %w", err)
		}
		// The "access restored" email per reactivated membership (issue #162,
		// T6); a send failure is logged inside the mailer.
		c.lifecycle.SendAccessRestored(ctx, recipientID, cand.PropertyID)
	}
	return nil
}

// RecoverSuspendedForProperty reactivates the oldest suspended memberships of
// every recipient of a single property when the owner archives or deletes that
// shared object. It is the per-property entry point called by the properties
// PropertyService (issue #158, T4): archiving/deleting the object freed one
// slot for each recipient, so each recipient's suspended queue is recovered
// FIFO in the same transaction.
func (c *SlotCoordinator) RecoverSuspendedForProperty(ctx context.Context, tx transaction.Tx, propertyID uuid.UUID) error {
	txMembers := c.members.WithTx(tx)

	memberships, err := txMembers.ListByProperty(ctx, propertyID)
	if err != nil {
		return fmt.Errorf("list memberships by property: %w", err)
	}
	recipients := make(map[uuid.UUID]struct{}, len(memberships))
	for _, m := range memberships {
		recipients[m.UserID] = struct{}{}
	}
	for recipientID := range recipients {
		if err := c.recoverSuspendedForRecipient(ctx, tx, recipientID); err != nil {
			return err
		}
	}
	return nil
}

// RecoverAfterPropertyDelete recovers the oldest suspended memberships of every
// recipient of a property that the owner is deleting. It is the delete-side
// counterpart to RecoverSuspendedForProperty (archive) and must run in the same
// transaction as the property delete, before the property row is removed (the
// access context owns the membership rows, so it drops them here; the property
// delete cascade then finds nothing to cascade). Dropping the membership first
// is what frees the recipient's slot so the FIFO recovery sees it. See issue
// #158 (T4).
func (c *SlotCoordinator) RecoverAfterPropertyDelete(ctx context.Context, tx transaction.Tx, propertyID uuid.UUID) error {
	txMembers := c.members.WithTx(tx)

	memberships, err := txMembers.ListByProperty(ctx, propertyID)
	if err != nil {
		return fmt.Errorf("list memberships by property: %w", err)
	}
	// Drop every membership on this property first: the object is going away, so
	// each recipient's slot is freed. The access context owns these rows.
	for _, m := range memberships {
		if err := txMembers.Delete(ctx, m.ID, propertyID); err != nil {
			return fmt.Errorf("delete membership %s on property delete: %w", m.ID, err)
		}
	}
	// Now recover each recipient's oldest suspended membership FIFO.
	recipients := make(map[uuid.UUID]struct{}, len(memberships))
	for _, m := range memberships {
		recipients[m.UserID] = struct{}{}
	}
	for recipientID := range recipients {
		if err := c.recoverSuspendedForRecipient(ctx, tx, recipientID); err != nil {
			return err
		}
	}
	return nil
}

// EnforceOnUnarchiveForProperty suspends memberships of recipients whose
// tariff pool exceeds the limit after a shared object is unarchived (the
// object re-enters the recipient's tariff pool). For each recipient of the
// property it evaluates the full pool: the unarchived membership is already
// active and therefore counted, so it is suspended only when the pool is
// strictly over the limit — a pool exactly at the limit still fits and the
// membership stays active. This is the third source of suspended access after
// activation without a slot and a billing limit drop (issue #158, T4).
func (c *SlotCoordinator) EnforceOnUnarchiveForProperty(ctx context.Context, tx transaction.Tx, propertyID uuid.UUID) error {
	txMembers := c.members.WithTx(tx)

	memberships, err := txMembers.ListByProperty(ctx, propertyID)
	if err != nil {
		return fmt.Errorf("list memberships by property: %w", err)
	}
	for _, m := range memberships {
		if m.IsSuspended() {
			// Already suspended: leave it for FIFO recovery when a slot frees.
			continue
		}
		// Unlike EnforceOnActivation (a pre-insert check), the membership is
		// already active and counted in the pool here: suspend only when the
		// pool strictly exceeds the limit.
		over, err := c.poolOverLimit(ctx, tx, txMembers, m.UserID)
		if err != nil {
			return fmt.Errorf("check recipient slot on unarchive: %w", err)
		}
		if !over {
			continue
		}
		if err := txMembers.Suspend(ctx, m.ID, propertyID); err != nil {
			return fmt.Errorf("suspend membership %s on unarchive: %w", m.ID, err)
		}
		if err := c.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
			ActorRole:  auditdomain.ActorRoleSystem,
			Action:     auditdomain.ActionPropertyMemberSuspended,
			EntityType: auditdomain.EntityPropertyMember,
			EntityID:   &m.ID,
			Context: map[string]any{
				auditKeyTrigger:    "unarchive",
				auditKeyPropertyID: propertyID,
				auditKeyUserID:     m.UserID,
			},
		}); err != nil {
			return fmt.Errorf("record suspend audit on unarchive: %w", err)
		}
		// The "access waits for a free slot" email on the unarchive path
		// (issue #162, T6); a send failure is logged inside the mailer.
		c.lifecycle.SendAccessSuspended(ctx, m.UserID, propertyID)
	}
	return nil
}

// buildRecipientPool assembles the recipient's full tariff pool: own active
// properties (IsShared=false) plus all active shared memberships (IsShared=true)
// across every owner, each annotated with whether it has an open lease. The pool
// is the input to SelectForEviction.
func (c *SlotCoordinator) buildRecipientPool(
	ctx context.Context,
	txOwned OwnedActivePropertiesPort,
	txMembers MembershipRepository,
	recipientID uuid.UUID,
) ([]SlotCandidate, error) {
	// Own active properties.
	owned, err := txOwned.ListActiveWithMeta(ctx, recipientID)
	if err != nil {
		return nil, fmt.Errorf("list owned active properties: %w", err)
	}
	ownOccupied, err := c.occupancy.OccupiedPropertyIDs(ctx, recipientID)
	if err != nil {
		return nil, fmt.Errorf("own occupancy: %w", err)
	}

	// Active shared memberships across all owners.
	shared, err := txMembers.ListActiveByUser(ctx, recipientID)
	if err != nil {
		return nil, fmt.Errorf("list active shared memberships: %w", err)
	}
	sharedOccupied, err := collectSharedOccupancy(ctx, shared, c.owners, c.occupancy)
	if err != nil {
		return nil, fmt.Errorf("shared occupancy: %w", err)
	}

	pool := make([]SlotCandidate, 0, len(owned)+len(shared))
	for _, p := range owned {
		pool = append(pool, SlotCandidate{
			PropertyID:   p.ID,
			IsShared:     false,
			HasOpenLease: ownOccupied[p.ID],
			UpdatedAt:    p.UpdatedAt,
		})
	}
	for _, m := range shared {
		pool = append(pool, SlotCandidate{
			PropertyID:   m.PropertyID,
			IsShared:     true,
			MemberID:     m.ID,
			RecipientID:  recipientID,
			HasOpenLease: sharedOccupied[m.PropertyID],
			UpdatedAt:    m.UpdatedAt,
		})
	}
	return pool, nil
}

// usedSlots returns the number of tariff slots the recipient currently occupies:
// own active properties plus active shared memberships.
func (c *SlotCoordinator) usedSlots(
	ctx context.Context,
	txOwned OwnedActivePropertiesPort,
	txMembers MembershipRepository,
	recipientID uuid.UUID,
) (int, error) {
	owned, err := txOwned.ListActiveWithMeta(ctx, recipientID)
	if err != nil {
		return 0, fmt.Errorf("list owned active properties: %w", err)
	}
	shared, err := txMembers.ListActiveByUser(ctx, recipientID)
	if err != nil {
		return 0, fmt.Errorf("list active shared memberships: %w", err)
	}
	return len(owned) + len(shared), nil
}

// collectSharedOccupancy merges the open-lease state for a set of shared
// memberships. Properties are grouped by their data owner (scope) and each
// owner's occupied set is fetched once via the occupancy port, then merged into
// a single property-id -> open-lease map. This avoids one occupancy round-trip
// per membership when several memberships share a data owner.
func collectSharedOccupancy(
	ctx context.Context,
	members []domain.Membership,
	owners PropertyOwnerResolver,
	occupancy OccupancyPort,
) (map[uuid.UUID]bool, error) {
	// Group property ids by their data owner.
	byOwner := make(map[uuid.UUID]map[uuid.UUID]struct{})
	for _, m := range members {
		ownerID, err := owners.GetOwnerID(ctx, m.PropertyID)
		if err != nil {
			return nil, fmt.Errorf("resolve owner of property %s: %w", m.PropertyID, err)
		}
		set, ok := byOwner[ownerID]
		if !ok {
			set = make(map[uuid.UUID]struct{})
			byOwner[ownerID] = set
		}
		set[m.PropertyID] = struct{}{}
	}

	merged := make(map[uuid.UUID]bool, len(members))
	for ownerID, propSet := range byOwner {
		occupied, err := occupancy.OccupiedPropertyIDs(ctx, ownerID)
		if err != nil {
			return nil, fmt.Errorf("occupancy of owner %s: %w", ownerID, err)
		}
		for pid := range propSet {
			merged[pid] = occupied[pid]
		}
	}
	return merged, nil
}

// deref returns *t, or the zero time when t is nil. A nil SuspendedAt is not
// expected for status='suspended' rows (suspended_at is NOT NULL on suspend),
// but the zero value sorts "earliest" in FIFO, which is a safe fallback.
func deref(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
