package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	realtimeapp "github.com/nambers/arenda-planform/apps/backend/internal/realtime/application"
	realtimedom "github.com/nambers/arenda-planform/apps/backend/internal/realtime/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// Audit and log context keys shared by the access use cases: the same keys
// appear in audit Entry.Context maps and in slog attribute lists.
const (
	auditKeyPropertyID = "property_id"
	auditKeyRole       = "role"
	auditKeyTrigger    = "trigger"
	auditKeyUserID     = "user_id"
)

// userLabel resolves the row-text label of a participant: the display name
// (the audit's masking canon — «Name Surname», иначе замаскированный телефон,
// никогда email или сырой телефон). Loud on failure: a broken lookup is a
// database trouble, not a missing label.
func userLabel(ctx context.Context, users UserLookup, userID uuid.UUID) (string, error) {
	return lookupLabel(ctx, users, userID, "resolve history member label")
}

// Member is the application-level projection of a property participant used by
// ListMembers. The owner is synthesized from properties.owner_id and never
// stored as a membership row, so it carries IsOwner = true and no membership id.
// A pending email invitation (issue #161, T5) is projected as a Member with
// Pending = true, the invitation id in ID, a zero UserID, and Email/LastSentAt
// filled. Email is resolved for every row: participant emails are visible to
// all readers of the list (owner decision 2026-09-20, #758 walkthrough).
type Member struct {
	ID          uuid.UUID // Membership id; invitation id when Pending (zero uuid for the owner).
	UserID      uuid.UUID
	Role        sharedpolicy.Role // Owner | full_access | viewer.
	IsOwner     bool
	DisplayName string
	Status      domain.MemberStatus // Active | suspended (always active for the owner; meaningless when Pending).
	SuspendedAt *time.Time          // When the membership was suspended; nil when active.
	// Pending marks a pending email invitation row: the invitee is not
	// registered yet, so UserID is zero.
	Pending    bool
	Email      *string    // User address; nil when unresolved or the user has none.
	LastSentAt *time.Time // When the invite email was last sent; set only when Pending.
}

// AccessService implements the property membership use cases (issue #156, T3):
// adding, listing, changing roles, revoking and self-exit. Authorization goes
// through the policy port; persistence and audit share the same transaction
// through the embedded txStoreFactory (ADR 0033). The lifecycle notifications
// leave the context as events (карта #734, #751) on the grace-events canon:
// captured at the transition, published strictly after the commit,
// best-effort.
type AccessService struct {
	txStoreFactory
	members  MembershipRepository
	owners   PropertyOwnerResolver
	statuses PropertyStatusResolver
	users    UserLookup
	policy   sharedpolicy.Policy
	slots    *SlotCoordinator
	events   AccessEventPublisher
	logger   *slog.Logger
	// Realtime is the late-bound carrier the transitions' frames dispatch
	// through after the commit (карта #714, #716; ADR 0062); nil keeps the
	// pre-#716 silence. The invitation and participant-mutation services
	// publish through it too — they run this service's transactional cores.
	realtime realtimeapp.Publisher
}

// SetRealtimePublisher late-binds the realtime carrier (карта #714, #716;
// ADR 0062): the frames of the committed transitions dispatch through it —
// the grace-events canon, best-effort, a broken carrier never fails the
// transition.
func (s *AccessService) SetRealtimePublisher(p realtimeapp.Publisher) {
	s.realtime = p
}

// publishRealtime hands the committed transition's frames to the realtime
// carrier (карта #714, #716; ADR 0062) — strictly post-commit, best-effort:
// a rolled-back transition dispatches nothing, a nil carrier keeps the
// pre-#716 silence.
func (s *AccessService) publishRealtime(ctx context.Context, actor uuid.UUID, changes []realtimedom.Change) {
	if s.realtime == nil || len(changes) == 0 {
		return
	}
	s.realtime.EntityChanged(ctx, actor, changes...)
}

// accessFrames are one transition's frames on one object: the access pair —
// every membership or invitation transition dirties the object's participants
// view — plus the history pair when the transaction journaled the transition
// (a written journal row is a history change for the object's feed).
func accessFrames(propertyID uuid.UUID, journaled bool) []realtimedom.Change {
	changed := []realtimedom.Change{realtimedom.On(realtimedom.EntityAccess, propertyID)}
	if journaled {
		changed = append(changed, realtimedom.HistoryOn(propertyID))
	}
	return changed
}

// NewAccessService creates an AccessService. Slots is the recipient tariff slot
// coordinator (issue #158, T4); it may be nil to disable slot enforcement
// (pre-T4 behaviour, e.g. in tests that don't exercise the limit). Events is
// the lifecycle event publisher (карта #734, #751); it may be nil to disable
// the publications (tests that don't exercise them). Statuses reports the
// archived flag of a property (issue #163); it may be nil to skip the
// archived-property checks. Factory bundles the membership repository, the
// audit and history recorders, and the Unit-of-Work every mutating use case
// runs through (ADR 0033 γ-factory).
func NewAccessService(
	members MembershipRepository,
	owners PropertyOwnerResolver,
	statuses PropertyStatusResolver,
	users UserLookup,
	policy sharedpolicy.Policy,
	slots *SlotCoordinator,
	events AccessEventPublisher,
	factory txStoreFactory,
	logger *slog.Logger,
) *AccessService {
	if logger == nil {
		logger = slog.Default()
	}
	return &AccessService{
		txStoreFactory: factory,
		members:        members,
		owners:         owners,
		statuses:       statuses,
		users:          users,
		policy:         policy,
		slots:          slots,
		events:         events,
		logger:         logger,
	}
}

// AddMember grants a registered user a role on a property. Any participant with
// the manage-members capability (owner or full) may add anyone, including
// another full member — full members are equal in member management. Adding a
// member to an archived property is rejected (issue #163).
func (s *AccessService) AddMember(ctx context.Context, actor, propertyID, userID uuid.UUID, role domain.Role) (domain.Membership, error) {
	_, actorRole, err := s.authorizeNewMember(ctx, actor, propertyID, userID)
	if err != nil {
		return domain.Membership{}, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return domain.Membership{}, fmt.Errorf("generate membership id: %w", err)
	}
	membership := domain.Membership{
		ID:         id,
		PropertyID: propertyID,
		UserID:     userID,
		Role:       role,
		GrantedBy:  actor,
	}

	var created domain.Membership
	var suspended bool
	err = s.runInTx(ctx, func(stores *txStores) error {
		var err error
		created, suspended, err = s.createMembershipInTx(ctx, stores, actor, membership, actorRole)
		return err
	})
	if err != nil {
		return domain.Membership{}, err
	}

	// The landing state picks the member's row (карта #734, #751, #829): an
	// active grant speaks the «Приглашение в объект» event (the granter is the
	// actor); a grant created without a free tariff slot pauses the new
	// member's access (issue #158, T4) and speaks the system «Доступ
	// приостановлен» instead. Both are the event subscriber's — published
	// post-commit, best-effort; the direct lifecycle emails they replaced are
	// gone.
	if suspended {
		publishSuspendedEvents(ctx, s.events, s.logger, []domain.Membership{created})
	} else {
		publishGrantedEvents(ctx, s.events, s.logger, actor, []domain.Membership{created})
	}
	s.publishRealtime(ctx, actor, accessFrames(propertyID, true))
	return created, nil
}

// createInvitationInTx is the transactional core of pending-invitation
// creation: the duplicate guard, the insert and the audited record. Shared by
// the single-property invite (InvitationService.InviteByEmail) and the batch
// grant (ParticipantMutationService.grantInvitation) — the audit context and
// the PII rule live here, not in the callers. A duplicate is the
// ErrInvitationAlreadyExists signal; the caller maps it to its own outcome.
func (s *AccessService) createInvitationInTx(
	ctx context.Context, stores *txStores, invitation domain.Invitation, actor uuid.UUID, actorRole sharedpolicy.Role,
) (domain.Invitation, error) {
	if _, err := stores.invitations.GetByPropertyAndEmail(ctx, invitation.PropertyID, invitation.Email); err == nil {
		return domain.Invitation{}, domain.ErrInvitationAlreadyExists
	} else if !errors.Is(err, domain.ErrInvitationNotFound) {
		return domain.Invitation{}, fmt.Errorf("check existing invitation: %w", err)
	}

	created, err := stores.invitations.Create(ctx, invitation)
	if err != nil {
		return domain.Invitation{}, fmt.Errorf("create invitation: %w", err)
	}

	// Audit in the same transaction. Only ids and the role are recorded; the
	// invitee email is PII and must never appear in context (ADR 0020).
	if err := stores.audit.Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  sharedpolicy.AuditActorRole(actorRole),
		Action:     auditdomain.ActionPropertyMemberInvitationInvited,
		EntityType: auditdomain.EntityPropertyMemberInvitation,
		EntityID:   &created.ID,
		Context: map[string]any{
			auditKeyPropertyID: invitation.PropertyID,
			auditKeyRole:       string(invitation.Role),
		},
	}); err != nil {
		return domain.Invitation{}, fmt.Errorf("record audit: %w", err)
	}

	// The action journal row (ADR 0061 §5): an unregistered invitee is known
	// by the email only — the email is the label snapshot here.
	entry := historydomain.MemberInvited(invitation.Email)
	entry.Context[historydomain.CtxKeyRole] = string(invitation.Role)
	if err := historyapp.RecordScoped(ctx, stores.history, invitation.PropertyID, actor,
		sharedpolicy.HistoryActorRole(actorRole), entry); err != nil {
		return domain.Invitation{}, err
	}
	return created, nil
}

// authorizeNewMember applies the AddMember gates: the actor needs the
// manage-members capability and the property must not be archived (issue #163),
// and the target user must be neither the property owner nor the actor
// themselves — ownership is the only self-granted access.
func (s *AccessService) authorizeNewMember(ctx context.Context, actor, propertyID, userID uuid.UUID) (uuid.UUID, sharedpolicy.Role, error) {
	owner, actorRole, err := s.requireManage(ctx, actor, propertyID)
	if err != nil {
		return uuid.Nil, "", err
	}
	if err := requireNotArchived(ctx, s.statuses, propertyID); err != nil {
		return uuid.Nil, "", err
	}
	if userID == owner {
		return uuid.Nil, "", domain.ErrCannotAddOwner
	}
	if userID == actor {
		return uuid.Nil, "", domain.ErrCannotAddSelf
	}
	return owner, actorRole, nil
}

// createMembershipInTx performs the transactional core of AddMember: duplicate
// rejection, the recipient slot decision (issue #158, T4 — without a free slot
// the membership is created suspended so it does not occupy a slot until one
// frees up and is recovered FIFO) and the audited insert. The returned flag
// reports the suspended outcome for the post-commit notification.
func (s *AccessService) createMembershipInTx(
	ctx context.Context, stores *txStores, actor uuid.UUID, membership domain.Membership, actorRole sharedpolicy.Role,
) (created domain.Membership, suspend bool, err error) {
	if _, err := stores.members.GetByPropertyAndUser(ctx, membership.PropertyID, membership.UserID); err == nil {
		return domain.Membership{}, false, domain.ErrMemberAlreadyExists
	} else if !errors.Is(err, domain.ErrMemberNotFound) {
		return domain.Membership{}, false, fmt.Errorf("check existing membership: %w", err)
	}

	if s.slots != nil {
		var err error
		suspend, err = s.slots.EnforceOnActivation(ctx, stores.tx, membership.UserID)
		if err != nil {
			return domain.Membership{}, false, fmt.Errorf("check recipient slot: %w", err)
		}
	}
	if suspend {
		membership.Status = domain.MemberStatusSuspended
		created, err = stores.members.CreateWithStatus(ctx, membership)
	} else {
		created, err = stores.members.Create(ctx, membership)
	}
	if err != nil {
		return domain.Membership{}, false, fmt.Errorf("create membership: %w", err)
	}

	// Audit in the same transaction. Only ids and the role are recorded; the
	// member's email/phone are PII and must never appear in context (ADR 0020).
	if err := stores.audit.Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  sharedpolicy.AuditActorRole(actorRole),
		Action:     auditdomain.ActionPropertyMemberAdded,
		EntityType: auditdomain.EntityPropertyMember,
		EntityID:   &created.ID,
		Context: map[string]any{
			auditKeyPropertyID: membership.PropertyID,
			auditKeyUserID:     membership.UserID,
			auditKeyRole:       string(membership.Role),
			"status":           string(created.Status),
		},
	}); err != nil {
		return domain.Membership{}, false, fmt.Errorf("record audit: %w", err)
	}

	// The action journal row (ADR 0061): the member's display name is the
	// label snapshot — the row survives any later rename or revocation.
	label, err := userLabel(ctx, s.users, membership.UserID)
	if err != nil {
		return domain.Membership{}, false, err
	}
	entry := historydomain.MemberAdded(membership.UserID, label)
	entry.Context[historydomain.CtxKeyRole] = string(membership.Role)
	if err := historyapp.RecordScoped(ctx, stores.history, membership.PropertyID, actor,
		sharedpolicy.HistoryActorRole(actorRole), entry); err != nil {
		return domain.Membership{}, false, err
	}
	return created, suspend, nil
}

// ChangeMemberRole changes the role of an existing member. The owner is never a
// membership row, so it cannot be targeted here; the membership id must belong
// to the given property.
func (s *AccessService) ChangeMemberRole(
	ctx context.Context,
	actor, propertyID, memberID uuid.UUID,
	role domain.Role,
) (domain.Membership, error) {
	_, actorRole, err := s.requireManage(ctx, actor, propertyID)
	if err != nil {
		return domain.Membership{}, err
	}

	var updated domain.Membership
	var wasSuspended bool
	var roleChanged bool
	err = s.runInTx(ctx, func(stores *txStores) error {
		// Confirm the membership exists and belongs to this property before
		// updating; a missing row is a not-found outcome rather than a silent no-op.
		existing, err := stores.members.GetByID(ctx, memberID, propertyID)
		if err != nil {
			return err
		}
		wasSuspended = existing.IsSuspended()
		roleChanged = existing.Role != role

		updated, err = stores.members.UpdateRole(ctx, memberID, propertyID, role)
		if err != nil {
			return fmt.Errorf("update membership role: %w", err)
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  sharedpolicy.AuditActorRole(actorRole),
			Action:     auditdomain.ActionPropertyMemberUpdated,
			EntityType: auditdomain.EntityPropertyMember,
			EntityID:   &updated.ID,
			Context: map[string]any{
				auditKeyPropertyID: propertyID,
				auditKeyUserID:     updated.UserID,
				auditKeyRole:       string(role),
			},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}

		// The journal records a state change, not an idempotent re-set of the
		// same role (ADR 0061 §3): a same-role edit writes no row — the gate
		// the role-change notice below applies to its event too.
		if roleChanged {
			label, err := userLabel(ctx, s.users, updated.UserID)
			if err != nil {
				return err
			}
			if err := historyapp.RecordScoped(ctx, stores.history, propertyID, actor,
				sharedpolicy.HistoryActorRole(actorRole),
				historydomain.MemberRoleChanged(updated.UserID, label,
					sharedpolicy.HistoryActorRole(toSharedRole(existing.Role)),
					sharedpolicy.HistoryActorRole(toSharedRole(role)))); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return domain.Membership{}, err
	}

	// Post-commit notice to the member whose role changed (карта #828, #830)
	// — best-effort (карта #734, #751). A suspended membership stays silent:
	// the object was already hidden from its holder (issue #162, T6 canon).
	// A same-role no-op stays silent too — there is no change for the holder
	// to learn about.
	if !wasSuspended && roleChanged {
		publishAccessEvent(ctx, s.events, s.logger, "membership_role_changed", func() error {
			return s.events.PublishMembershipRoleChanged(ctx, MembershipRoleChanged{
				MembershipID: updated.ID,
				PropertyID:   propertyID,
				RecipientID:  updated.UserID,
				ActorID:      actor,
				Role:         role,
				ChangedAt:    updated.UpdatedAt,
			})
		})
	}
	s.publishRealtime(ctx, actor, accessFrames(propertyID, roleChanged))
	return updated, nil
}

// removeMembershipInTx is the shared core of deleting one membership row:
// the delete and the audited record — the audit shape lives here, not in the
// callers. The caller decides the FIFO recovery of the recipient's suspended
// accesses (the single revoke recovers immediately, the batch Remove recovers
// once after all legs) and publishes the post-commit revocation event for the
// collected active rows (карта #734, #751); a suspended row publishes nothing
// — the object was already hidden from them (issue #162, T6 canon). The
// actor role comes from the caller: the single path resolves it in
// requireManage, the batch in actorRoleOn.
func removeMembershipInTx(
	ctx context.Context,
	stores *txStores,
	actor uuid.UUID,
	actorRole sharedpolicy.Role,
	membership domain.Membership,
	revokedActive *[]domain.Membership,
	historyEntry historydomain.Entry,
) error {
	if err := stores.members.Delete(ctx, membership.ID, membership.PropertyID); err != nil {
		return fmt.Errorf("delete membership: %w", err)
	}
	if !membership.IsSuspended() && revokedActive != nil {
		*revokedActive = append(*revokedActive, membership)
	}

	if err := stores.audit.Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  sharedpolicy.AuditActorRole(actorRole),
		Action:     auditdomain.ActionPropertyMemberRemoved,
		EntityType: auditdomain.EntityPropertyMember,
		EntityID:   &membership.ID,
		Context: map[string]any{
			auditKeyPropertyID: membership.PropertyID,
			auditKeyUserID:     membership.UserID,
		},
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}

	// The action journal row (ADR 0061): the caller builds the row — the
	// single revoke and the bulk participant removal write their own action
	// ids over the same core.
	historyEntry.Context[historydomain.CtxKeyUserID] = membership.UserID
	if err := historyapp.RecordScoped(ctx, stores.history, membership.PropertyID, actor,
		sharedpolicy.HistoryActorRole(actorRole), historyEntry); err != nil {
		return err
	}
	return nil
}

// publishGrantedEvents publishes the post-commit «Приглашение в объект»
// events for active landings (issue #829) — best-effort (карта #734, #751);
// the granter is the event's actor.
func publishGrantedEvents(
	ctx context.Context, publisher AccessEventPublisher, log *slog.Logger, actor uuid.UUID, granted []domain.Membership,
) {
	for _, m := range granted {
		publishAccessEvent(ctx, publisher, log, "membership_granted", func() error {
			return publisher.PublishMembershipGranted(ctx, MembershipGranted{
				MembershipID: m.ID,
				PropertyID:   m.PropertyID,
				RecipientID:  m.UserID,
				ActorID:      actor,
			})
		})
	}
}

// publishSuspendedEvents publishes the post-commit system pause events for
// suspended grants — best-effort (карта #734, #751); the system suspension
// has no actor.
func publishSuspendedEvents(
	ctx context.Context, publisher AccessEventPublisher, log *slog.Logger, suspended []domain.Membership,
) {
	for _, m := range suspended {
		publishAccessEvent(ctx, publisher, log, "membership_suspended", func() error {
			return publisher.PublishMembershipSuspended(ctx, MembershipSuspended{
				MembershipID: m.ID,
				PropertyID:   m.PropertyID,
				RecipientID:  m.UserID,
				ActorID:      uuid.Nil,
				SuspendedAt:  deref(m.SuspendedAt),
			})
		})
	}
}

// publishRevokedEvents publishes the post-commit revocation events for the
// removed active rows — best-effort (карта #734, #751).
func publishRevokedEvents(
	ctx context.Context, publisher AccessEventPublisher, log *slog.Logger, actor uuid.UUID, revoked []domain.Membership,
) {
	for _, m := range revoked {
		publishAccessEvent(ctx, publisher, log, "membership_revoked", func() error {
			return publisher.PublishMembershipRevoked(ctx, MembershipRevoked{
				MembershipID: m.ID,
				PropertyID:   m.PropertyID,
				RecipientID:  m.UserID,
				ActorID:      actor,
			})
		})
	}
}

// RevokeMember removes a member's access. The owner cannot be revoked (and is
// never a membership row); this guard exists for defence in depth.
func (s *AccessService) RevokeMember(ctx context.Context, actor, propertyID, memberID uuid.UUID) error {
	owner, actorRole, err := s.requireManage(ctx, actor, propertyID)
	if err != nil {
		return err
	}

	var membership domain.Membership
	var revoked []domain.Membership
	err = s.runInTx(ctx, func(stores *txStores) error {
		var err error
		membership, err = stores.members.GetByID(ctx, memberID, propertyID)
		if err != nil {
			return err
		}
		if membership.UserID == owner {
			return domain.ErrCannotRevokeOwner
		}

		// The delete goes before the recovery pass: the recovery counts the
		// recipient's active memberships, and the revoked row must already be
		// gone for its slot to read as freed (issue #158, T4).
		label, err := userLabel(ctx, s.users, membership.UserID)
		if err != nil {
			return err
		}
		if err := removeMembershipInTx(ctx, stores, actor, actorRole, membership, &revoked,
			historydomain.MemberRemoved(label)); err != nil {
			return err
		}

		// Revoking the recipient freed one of their tariff slots: try to recover the
		// oldest suspended membership FIFO (issue #158, T4).
		if s.slots != nil {
			if err := s.slots.RecoverSuspended(ctx, stores.tx, membership.UserID); err != nil {
				return fmt.Errorf("recover suspended after revoke: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	publishRevokedEvents(ctx, s.events, s.logger, actor, revoked)
	for _, m := range revoked {
		s.publishRealtime(ctx, actor, accessFrames(m.PropertyID, true))
	}
	return nil
}

// LeaveProperty performs a member's self-exit. The owner cannot leave their own
// object. Self-exit does not require the manage-members capability — it is a
// right of every participant.
func (s *AccessService) LeaveProperty(ctx context.Context, actor, propertyID uuid.UUID) error {
	role, err := s.policy.RoleForProperty(ctx, actor, propertyID)
	if err != nil {
		return fmt.Errorf("resolve role: %w", err)
	}
	if role == sharedpolicy.RoleNone {
		// No access stays indistinguishable from no object: self-exit keeps
		// the privacy-preserving not-found. A suspended membership proceeds —
		// the recipient sees the blur-card placeholder (ticket #702) and may
		// leave from its reason sheet (T9 lifted the signal at the property
		// page; the placeholder list completes the surface).
		return domain.ErrMemberNotFound
	}
	if role == sharedpolicy.RoleOwner {
		return domain.ErrCannotLeaveOwnProperty
	}

	var membership domain.Membership
	err = s.runInTx(ctx, func(stores *txStores) error {
		var err error
		membership, err = stores.members.GetByPropertyAndUser(ctx, propertyID, actor)
		if err != nil {
			return err
		}

		if err := stores.members.Delete(ctx, membership.ID, propertyID); err != nil {
			return fmt.Errorf("delete membership: %w", err)
		}

		// Self-exit of an ACTIVE membership freed one of the actor's tariff
		// slots: try to recover the oldest suspended membership FIFO (issue
		// #158, T4). A suspended membership holds no slot — leaving it (the
		// reason sheet's «Покинуть объект», ticket #702) frees nothing, so
		// the recovery is skipped.
		if !membership.IsSuspended() && s.slots != nil {
			if err := s.slots.RecoverSuspended(ctx, stores.tx, actor); err != nil {
				return fmt.Errorf("recover suspended after leave: %w", err)
			}
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  sharedpolicy.AuditActorRole(role),
			Action:     auditdomain.ActionPropertyMemberLeft,
			EntityType: auditdomain.EntityPropertyMember,
			EntityID:   &membership.ID,
			Context: map[string]any{
				auditKeyPropertyID: propertyID,
			},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}

		// The action journal row (ADR 0061): the leaver is their own label,
		// attributed to the membership's real role — the policy's suspended
		// verdict says the access is hidden, not that the role was the
		// owner's, so the attribution reads the row the transaction holds.
		label, err := userLabel(ctx, s.users, actor)
		if err != nil {
			return err
		}
		if err := historyapp.RecordScoped(ctx, stores.history, propertyID, actor,
			sharedpolicy.HistoryActorRole(toSharedRole(membership.Role)), historydomain.MemberLeft(label)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Self-exit notifies the owner post-commit (карта #734, #751; the direct
	// lifecycle email it replaced is gone); the leaving member receives
	// nothing. A failed owner lookup leaves nobody to notify — logged, not an
	// error.
	owner, err := s.owners.GetOwnerID(ctx, propertyID)
	if err != nil {
		s.logger.WarnContext(ctx, "access: owner lookup for member left event failed",
			slog.String(auditKeyPropertyID, propertyID.String()),
			slog.String("error", err.Error()))
		return nil
	}
	publishAccessEvent(ctx, s.events, s.logger, "member_left", func() error {
		return s.events.PublishMemberLeft(ctx, MemberLeft{
			MembershipID: membership.ID,
			PropertyID:   propertyID,
			OwnerID:      owner,
			MemberID:     actor,
		})
	})
	s.publishRealtime(ctx, actor, accessFrames(propertyID, true))
	return nil
}

// ListMembers returns the property participants: the owner first (synthesized
// from properties.owner_id, IsOwner = true), then the membership rows ordered
// by created_at. Requires the view capability; RoleNone is mapped to not-found
// to preserve object privacy.
func (s *AccessService) ListMembers(ctx context.Context, actor, propertyID uuid.UUID) ([]Member, error) {
	role, err := s.policy.RoleForProperty(ctx, actor, propertyID)
	if err != nil {
		return nil, fmt.Errorf("resolve role: %w", err)
	}
	if !sharedpolicy.CanView(role) {
		return nil, domain.ErrMemberNotFound
	}

	owner, err := s.owners.GetOwnerID(ctx, propertyID)
	if err != nil {
		return nil, domain.ErrMemberNotFound
	}

	ownerUser, err := s.users.GetByID(ctx, owner)
	if err != nil {
		return nil, fmt.Errorf("load owner user: %w", err)
	}

	out := make([]Member, 0, 1)
	out = append(out, Member{
		UserID:      owner,
		Role:        sharedpolicy.RoleOwner,
		IsOwner:     true,
		DisplayName: displayName(ownerUser),
		Status:      domain.MemberStatusActive,
	})

	memberships, err := s.members.ListByProperty(ctx, propertyID)
	if err != nil {
		return nil, fmt.Errorf("list memberships: %w", err)
	}
	for _, m := range memberships {
		u, err := s.users.GetByID(ctx, m.UserID)
		if err != nil {
			s.logger.WarnContext(ctx, "access: member user lookup failed",
				slog.String(auditKeyUserID, m.UserID.String()),
				slog.String("error", err.Error()))
			continue
		}
		out = append(out, Member{
			ID:          m.ID,
			UserID:      m.UserID,
			Role:        toSharedRole(m.Role),
			IsOwner:     false,
			DisplayName: displayName(u),
			Status:      m.Status,
			SuspendedAt: m.SuspendedAt,
		})
	}
	return out, nil
}

// requireManage resolves the actor's role on the property, requires the
// manage-members capability, and returns the property owner id (scope) and the
// actor's role. A missing property or lack of access is mapped to
// ErrMemberNotFound to keep object existence private. The role is returned so
// callers can attribute audit entries to the actor's real role (issue #166
// follow-up).
func (s *AccessService) requireManage(ctx context.Context, actor, propertyID uuid.UUID) (uuid.UUID, sharedpolicy.Role, error) {
	return requireManageAccess(ctx, s.policy, s.owners, actor, propertyID)
}

// requireManageAccess is the shared manage-members gate used by both the
// membership and the invitation services (issue #161, T5). It returns the
// property owner id (scope) and the actor's resolved role.
func requireManageAccess(
	ctx context.Context,
	policy sharedpolicy.Policy,
	owners PropertyOwnerResolver,
	actor, propertyID uuid.UUID,
) (uuid.UUID, sharedpolicy.Role, error) {
	role, err := policy.RoleForProperty(ctx, actor, propertyID)
	if err != nil {
		return uuid.UUID{}, "", fmt.Errorf("resolve role: %w", err)
	}
	if !sharedpolicy.CanManageMembers(role) {
		return uuid.UUID{}, "", domain.ErrMemberNotFound
	}
	owner, err := owners.GetOwnerID(ctx, propertyID)
	if err != nil {
		return uuid.UUID{}, "", domain.ErrMemberNotFound
	}
	return owner, role, nil
}

// requireNotArchived is the shared archived-property gate for granting new
// shared access (issue #163): adding a member or an invitation to an archived
// property is rejected with ErrPropertyArchived. Revoking, role changes and
// self-exit do not go through this gate — they keep working on archived
// objects. A nil resolver disables the check (tests without the dependency).
func requireNotArchived(ctx context.Context, statuses PropertyStatusResolver, propertyID uuid.UUID) error {
	if statuses == nil {
		return nil
	}
	archived, err := statuses.IsArchived(ctx, propertyID)
	if err != nil {
		return fmt.Errorf("check property archived: %w", err)
	}
	if archived {
		return domain.ErrPropertyArchived
	}
	return nil
}

func toSharedRole(r domain.Role) sharedpolicy.Role {
	switch r {
	case domain.RoleFullAccess:
		return sharedpolicy.RoleFullAccess
	case domain.RoleViewer:
		return sharedpolicy.RoleViewer
	default:
		return sharedpolicy.RoleNone
	}
}

// DisplayName resolves the public display name of a user (issue T11): "Name
// Surname" when present, otherwise a masked phone — never an email or a raw
// phone. Used cross-context (the properties sharing banner) via the
// OwnerDisplayNameResolver port.
func (s *AccessService) DisplayName(ctx context.Context, userID uuid.UUID) (string, error) {
	return lookupLabel(ctx, s.users, userID, "lookup user")
}

// lookupLabel resolves the user and projects the row-text label through the
// masking canon — the shared lookup plumbing of userLabel and DisplayName.
// Loud on failure: a broken lookup is a database trouble, wrapped with
// wrapText.
func lookupLabel(ctx context.Context, users UserLookup, userID uuid.UUID, wrapText string) (string, error) {
	u, err := users.GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf(wrapText+": %w", err)
	}
	return displayName(u), nil
}

func displayName(u MemberUser) string {
	parts := make([]string, 0, 2)
	if u.Name != nil && *u.Name != "" {
		parts = append(parts, *u.Name)
	}
	if u.Surname != nil && *u.Surname != "" {
		parts = append(parts, *u.Surname)
	}
	if name := strings.Join(parts, " "); name != "" {
		return name
	}
	// Fall back to a masked phone; never the raw phone or email.
	return maskPhone(u.Phone)
}

// DisplayNameOf composes the public display name from user fields ("Name
// Surname" when present, otherwise a masked phone — never an email or a raw
// phone). The exported seam for the context's read adapters implementing
// cross-context ports (ticket #702: the list cards' member names and the
// suspended placeholders' owner row): the masking rules live in one place.
func DisplayNameOf(u MemberUser) string {
	return displayName(u)
}

// maskPhone masks all but the country code and last two digits of a phone, so a
// display name never reveals the full phone or any email (PII).
func maskPhone(phone string) string {
	if len(phone) <= 4 {
		return phone
	}
	return phone[:2] + strings.Repeat("*", len(phone)-4) + phone[len(phone)-2:]
}
