package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
)

// The access lifecycle events (карта #734, #751): the single interface the
// membership transitions publish through, on the grace-events canon (ADR
// 0033) — an event is published strictly after the transaction that caused it
// commits, best-effort: a publication failure is logged by the caller and
// never fails or rolls back the transition. The composition root subscribes
// the notifications context's access publisher to the same events, so access
// never imports notifications.
//
// The events carry pure transition facts — ids, flags, instants; the display
// snapshots (names, addresses, emails) are the notifications side's
// vocabulary, resolved at publication from the owning tables (#745's
// snapshot-at-publication semantics).

// InvitationActivated reports a pending invitation's activation at
// registration: the invitation row is gone, the membership row was created
// (suspended when the recipient had no free tariff slot, issue #158, T4).
// The inviter learns their invitation was accepted; the invitee's own
// notification depends on the landing state.
type InvitationActivated struct {
	MembershipID uuid.UUID
	PropertyID   uuid.UUID
	// InviterID is the actor of the invitation (membership.GrantedBy) — the
	// recipient of the accepted event.
	InviterID uuid.UUID
	// InviteeID is the freshly registered user the invitation belonged to —
	// the actor of the accepted event.
	InviteeID uuid.UUID
	// Suspended reports the membership landed in the suspended state (no free
	// recipient slot at activation).
	Suspended bool
	// At is the transition instant the notifications' dedup keys stamp: the
	// membership row's suspension instant when the landing is suspended, the
	// activation moment otherwise.
	At time.Time
}

// MembershipGranted reports an active membership granted instantly to a
// registered user (issue #829): the Invite and AddProperties batches, the
// InviteByEmail registered path and the direct add-member endpoint
// (POST /properties/{propertyId}/access/members) all land through this
// transition. The granted member learns about the new access (the №5
// «Приглашение в объект» row, the
// copy promising an access they actually have); a no-slot landing speaks the
// system pause instead and publishes no granted event.
type MembershipGranted struct {
	MembershipID uuid.UUID
	PropertyID   uuid.UUID
	// RecipientID is the member the access was granted to.
	RecipientID uuid.UUID
	// ActorID is the granter (the inviting owner or full member) — the copy's
	// subject and the actor-skip anchor.
	ActorID uuid.UUID
}

// MembershipSuspended reports an active membership moving into the suspended
// state. RecipientID is the member whose access got paused; ActorID uuid.Nil
// marks the system suspension (the recipient slot enforcement has no human
// initiator) — every suspension today. SuspendedAt is the transition instant
// the notification's dedup key stamps (a membership can cycle paused →
// resumed → paused; each pause is its own fact).
type MembershipSuspended struct {
	MembershipID uuid.UUID
	PropertyID   uuid.UUID
	RecipientID  uuid.UUID
	ActorID      uuid.UUID
	SuspendedAt  time.Time
}

// MembershipResumed reports a suspended membership recovering to the active
// state (the FIFO slot recovery). ResumedAt is the transition instant the
// notification's dedup key stamps.
type MembershipResumed struct {
	MembershipID uuid.UUID
	PropertyID   uuid.UUID
	RecipientID  uuid.UUID
	ActorID      uuid.UUID
	ResumedAt    time.Time
}

// MembershipRevoked reports a manager deleting an active membership (issue
// #156, T3). RecipientID is the removed member; ActorID is the revoking
// owner or full member. A suspended membership's revoke publishes nothing —
// the object was already hidden from the recipient (issue #162, T6 canon).
type MembershipRevoked struct {
	MembershipID uuid.UUID
	PropertyID   uuid.UUID
	RecipientID  uuid.UUID
	ActorID      uuid.UUID
}

// MembershipRoleChanged reports a manager changing an active member's role
// (карта #828, #830, решение владельца 23.09 — the «изменение ваших прав»
// part of the settings matrix). RecipientID is the member whose role changed,
// ActorID the manager who changed it, Role the new role — the notifications
// side renders its display wording. ChangedAt is the change instant (the
// membership row's updated_at) the notification's dedup key stamps: every
// change is its own fact; a suspended membership's change publishes nothing
// — the object was already hidden from the recipient (issue #162, T6 canon).
type MembershipRoleChanged struct {
	MembershipID uuid.UUID
	PropertyID   uuid.UUID
	RecipientID  uuid.UUID
	ActorID      uuid.UUID
	Role         domain.Role
	ChangedAt    time.Time
}

// MemberLeft reports a member's self-exit (issue #156, T3): OwnerID is the
// recipient, MemberID the leaving member — the actor of their own exit.
type MemberLeft struct {
	MembershipID uuid.UUID
	PropertyID   uuid.UUID
	OwnerID      uuid.UUID
	MemberID     uuid.UUID
}

// AccessEventPublisher publishes the access lifecycle events. A nil
// implementation (or a nil interface) keeps the pre-#751 behaviour — the
// transitions run, no events leave the context.
type AccessEventPublisher interface {
	PublishInvitationActivated(ctx context.Context, event InvitationActivated) error
	PublishMembershipGranted(ctx context.Context, event MembershipGranted) error
	PublishMembershipSuspended(ctx context.Context, event MembershipSuspended) error
	PublishMembershipResumed(ctx context.Context, event MembershipResumed) error
	PublishMembershipRevoked(ctx context.Context, event MembershipRevoked) error
	PublishMemberLeft(ctx context.Context, event MemberLeft) error
	PublishMembershipRoleChanged(ctx context.Context, event MembershipRoleChanged) error
}

// publishAccessEvent runs one publication on the nil-tolerant port: a nil
// publisher is a no-op, a publication failure is logged and swallowed — the
// events are best-effort by canon and must never fail their transition.
func publishAccessEvent(ctx context.Context, publisher AccessEventPublisher, log *slog.Logger, kind string, publish func() error) {
	if publisher == nil {
		return
	}
	if err := publish(); err != nil {
		if log == nil {
			log = slog.Default()
		}
		log.ErrorContext(ctx, "access: publish lifecycle event failed",
			slog.String("event", kind),
			slog.String("error", err.Error()))
	}
}
