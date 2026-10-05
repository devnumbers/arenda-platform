// Package application holds the access bounded context use cases and ports
// (issue #156, T3): managing property memberships and the membership-aware
// authorization policy.
package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// MembershipRepository persists property membership records. Membership rows
// are not owner-scoped at the SQL level: access to management operations is
// regulated by the policy port in the application layer.
type MembershipRepository interface {
	Create(ctx context.Context, membership domain.Membership) (domain.Membership, error)
	GetByID(ctx context.Context, id, propertyID uuid.UUID) (domain.Membership, error)
	GetByPropertyAndUser(ctx context.Context, propertyID, userID uuid.UUID) (domain.Membership, error)
	GetRole(ctx context.Context, propertyID, userID uuid.UUID) (domain.Role, error)
	ListByProperty(ctx context.Context, propertyID uuid.UUID) ([]domain.Membership, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Membership, error)
	// MaxRoleByOwner returns the strongest role the user holds across all
	// properties of the given owner (scope). Returns RoleNone when the user has no
	// membership on any of the owner's properties. Used by the policy port to
	// derive owner-wide access (issue #157).
	MaxRoleByOwner(ctx context.Context, userID, ownerID uuid.UUID) (domain.Role, error)
	UpdateRole(ctx context.Context, id, propertyID uuid.UUID, role domain.Role) (domain.Membership, error)
	Delete(ctx context.Context, id, propertyID uuid.UUID) error
	// Suspend marks a membership as suspended: it no longer occupies a tariff
	// slot and grants no access until reactivated (issue #158, T4). Existence is
	// established by the service in the same transaction.
	Suspend(ctx context.Context, id, propertyID uuid.UUID) error
	// Reactivate marks a suspended membership as active again, restoring its
	// slot and access. Returns ErrMemberNotFound when no row matches.
	Reactivate(ctx context.Context, id, propertyID uuid.UUID) (domain.Membership, error)
	// ListSuspendedByUser returns the user's suspended memberships ordered for
	// FIFO recovery (oldest suspended_at first).
	ListSuspendedByUser(ctx context.Context, userID uuid.UUID) ([]domain.Membership, error)
	// CountActiveByUser returns the number of active memberships held by the
	// user — i.e. the number of occupied tariff slots.
	CountActiveByUser(ctx context.Context, userID uuid.UUID) (int, error)
	// ListActiveByPropertyOwner returns the active memberships across all of the
	// owner's properties.
	ListActiveByPropertyOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.Membership, error)
	// ListActiveByUser returns the user's active memberships with full rows
	// (id, updated_at, ...), unlike ListByUser which returns a lightweight
	// property-id+role projection. Used by the slot coordinator to build the
	// recipient's shared-property pool. See issue #158 (T4).
	ListActiveByUser(ctx context.Context, userID uuid.UUID) ([]domain.Membership, error)
	// ListForRemovalByUser returns the person's memberships (any status, with
	// ids) on the properties the given actor manages — owner or active
	// full_access member — including archived properties: revoking keeps
	// working on archived objects (issue #163), so «Отозвать и удалить»
	// (issue #694) must not leave archived legs behind. The scope predicate
	// is the authorization (issue #693).
	ListForRemovalByUser(ctx context.Context, personID, actorID uuid.UUID) ([]domain.Membership, error)
	// CreateWithStatus inserts a membership with an explicit status, used to
	// create a suspended grant directly (so it can be activated later without
	// occupying a slot until a slot frees up).
	CreateWithStatus(ctx context.Context, m domain.Membership) (domain.Membership, error)
	WithTx(tx transaction.Tx) MembershipRepository
}

// PropertyOwnerResolver resolves the data owner of a property. Used by the
// policy and the access service to turn a property id into the owner id
// (scope) without leaking the property's existence to unauthorized actors.
type PropertyOwnerResolver interface {
	GetOwnerID(ctx context.Context, propertyID uuid.UUID) (uuid.UUID, error)
}

// MemberUser describes a user for member display purposes. The display name
// composes from these fields («Name Surname», иначе «Пользователь» — карта
// #1105, аменд #1123); phones never go into audit context (ADR 0020). Member emails for
// the participants list are resolved through the narrow UserEmailResolver
// port.
type MemberUser struct {
	ID      uuid.UUID
	Name    *string
	Surname *string
	Phone   string
}

// UserLookup resolves registered users by id for member display.
type UserLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (MemberUser, error)
	GetByEmail(ctx context.Context, email string) (MemberUser, error)
}

// txBeginner begins a database transaction. Mirrors the local interface used
// by every other bounded context's service.
type txBeginner interface {
	Begin(ctx context.Context) (transaction.Tx, error)
}

// RecipientLimiter reports the active-property tariff limit of a recipient —
// i.e. the maximum number of objects (own + shared) a recipient may have
// active at once. Implemented by a bridge adapter over the billing
// SubscriptionPropertyLimiter. Returns math.MaxInt32 for unlimited tariffs.
// See issue #158 (T4), PRD #153.
type RecipientLimiter interface {
	ActivePropertyLimit(ctx context.Context, recipientID uuid.UUID) (int, error)
	WithTx(tx transaction.Tx) (RecipientLimiter, error)
}

// OwnedActivePropertiesPort lists the active property ids owned by a user
// (their own tariff pool entries). Implemented by a bridge adapter over the
// properties PropertyRepository.
type OwnedActivePropertiesPort interface {
	// ListActiveWithMeta returns ids of active+maintenance properties of the
	// owner, each paired with its UpdatedAt (used by the eviction comparator).
	ListActiveWithMeta(ctx context.Context, ownerID uuid.UUID) ([]OwnedPropertyMeta, error)
	// WithTx binds the port to the caller's transaction. Required so the slot
	// coordinator reads the recipient's own-property pool from the same snapshot
	// as the operation in progress: when a recovery/enforce runs inside the
	// ArchiveProperty/DeleteProperty/UnarchiveProperty transaction, the
	// already-applied status change (e.g. own object just archived) must be
	// visible to usedSlots/buildRecipientPool. Without WithTx the read goes to a
	// separate connection in Read Committed and sees the pre-transaction state.
	WithTx(tx transaction.Tx) (OwnedActivePropertiesPort, error)
}

// OwnedPropertyMeta is a property id + UpdatedAt pair from the owner's active
// pool.
type OwnedPropertyMeta struct {
	ID        uuid.UUID
	UpdatedAt time.Time
}

// InvitationRepository persists pending property member invitations
// (issue #161, T5). Like membership rows, invitation rows are not owner-scoped
// at the SQL level: management operations are regulated by the policy port in
// the application layer.
type InvitationRepository interface {
	Create(ctx context.Context, invitation domain.Invitation) (domain.Invitation, error)
	GetByID(ctx context.Context, id, propertyID uuid.UUID) (domain.Invitation, error)
	// GetByPropertyAndEmail returns the pending invitation for the normalized
	// (lowercase) email on the property, or domain.ErrInvitationNotFound.
	GetByPropertyAndEmail(ctx context.Context, propertyID uuid.UUID, email string) (domain.Invitation, error)
	ListByProperty(ctx context.Context, propertyID uuid.UUID) ([]domain.Invitation, error)
	// ListPendingByEmail returns all pending invitations for the normalized
	// email across properties, oldest first: activation at registration is
	// FIFO.
	ListPendingByEmail(ctx context.Context, email string) ([]domain.Invitation, error)
	// ListForRemovalByEmail returns the pending invitations for the email on
	// the properties the given actor manages — owner or active full_access
	// member — including archived properties (issue #163): «Отозвать и
	// удалить» (issue #694) removes every invitation leg in the actor's
	// scope. The scope predicate is the authorization (issue #693).
	ListForRemovalByEmail(ctx context.Context, email string, actorID uuid.UUID) ([]domain.Invitation, error)
	UpdateRole(ctx context.Context, id, propertyID uuid.UUID, role domain.Role) (domain.Invitation, error)
	UpdateLastSentAt(ctx context.Context, id, propertyID uuid.UUID, sentAt time.Time) error
	Delete(ctx context.Context, id, propertyID uuid.UUID) error
	WithTx(tx transaction.Tx) InvitationRepository
}

// AccessMailer sends the transactional emails of the sharing lifecycle that
// stay direct: the invite email to an unregistered invitee (issue #161, T5 —
// until the invitee registers there is no recipient a feed row could belong
// to) and the "object deleted" notice (issue #162, T6) whose event is not in
// the notifications catalog. The rest of the lifecycle correspondence —
// revoke, suspension, recovery, activation, self-exit — is the stored
// notifications feed's (карта #734, #751, ADR 0058): written always,
// delivered over the channels per the category matrix (the direct lifecycle
// emails cut by issue #695 are replaced by exactly this feed).
type AccessMailer interface {
	// SendInvite is the invite email to an unregistered invitee. Since the
	// multi-object invitation (issue #694) one email covers the whole batch:
	// PropertyTitles are the display titles of the objects the person was
	// invited to.
	SendInvite(ctx context.Context, to string, propertyTitles []string, role domain.Role) error
	// SendPropertyDeleted is the "object deleted" notice: one letter per
	// former member of the deleted object (issue #162, T6).
	SendPropertyDeleted(ctx context.Context, to, propertyTitle string) error
}

// UserEmailResolver resolves a registered user's email address behind a narrow
// port: the access context otherwise sees only non-PII display fields. Read by
// the participant read model (issue #693), the participant mutations (issue
// #694) and the members list (owner decision 2026-09-20, #758: participant
// emails are visible to every reader of the list). An empty string means the
// user has no email (or no longer exists): the caller skips the send.
type UserEmailResolver interface {
	GetEmail(ctx context.Context, userID uuid.UUID) (string, error)
}

// PropertyTitleResolver resolves a property's display title for the invite
// email text. Kept separate from PropertyOwnerResolver so the mail path does
// not depend on authorization semantics.
type PropertyTitleResolver interface {
	GetTitle(ctx context.Context, propertyID uuid.UUID) (string, error)
}

// PropertyStatusResolver reports whether a property is archived. Used to
// forbid granting new shared access on archived objects and to activate a
// pending invitation to an archived object without a recipient tariff slot
// (issue #163).
type PropertyStatusResolver interface {
	IsArchived(ctx context.Context, propertyID uuid.UUID) (bool, error)
}
