package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// InviteOutcome is the result of InviteByEmail: exactly one of the fields is
// set. Member is set when the invitee email belongs to a registered user and
// the membership was activated instantly (no invite email is sent);
// Invitation is set when a pending invitation was stored and the invite email
// was (attempted to be) sent.
type InviteOutcome struct {
	Member     *domain.Membership
	Invitation *domain.Invitation
}

// InvitationService implements the email invitation lifecycle (issue #161,
// T5): inviting an unregistered email to shared access, manual resend with a
// 24h cooldown, role change and silent cancellation of pending invitations,
// and FIFO activation when the invitee registers. The invite email is the only
// email sent by this service itself; the lifecycle emails (activation notice to
// the owner, the "waiting for a slot" email on a suspended activation) go
// through the LifecycleMailer (issue #162, T6). Persistence and audit share
// the same transaction through the embedded txStoreFactory (ADR 0033); the
// invitee email is PII and never appears in audit context (ADR 0020).
type InvitationService struct {
	txStoreFactory
	access      *AccessService
	members     MembershipRepository
	invitations InvitationRepository
	owners      PropertyOwnerResolver
	statuses    PropertyStatusResolver
	users       UserLookup
	policy      sharedpolicy.Policy
	slots       *SlotCoordinator
	mailer      AccessMailer
	lifecycle   *LifecycleMailer
	titles      PropertyTitleResolver
	clock       clock.Clock
	logger      *slog.Logger
}

// NewInvitationService creates an InvitationService. Access is the membership
// service used to delegate instant activation for registered emails; slots may
// be nil to disable slot enforcement (mirrors NewAccessService); mailer may be
// nil to skip sending (e.g. in tests that do not exercise the mail path);
// lifecycle is the sharing lifecycle mailer (issue #162, T6) and may be nil to
// disable the lifecycle emails. Statuses reports the archived flag of a
// property (issue #163); it may be nil to skip the archived-property checks.
// Factory bundles the repositories, the audit recorder, and the Unit-of-Work
// every mutating use case runs through (ADR 0033 γ-factory).
func NewInvitationService(
	access *AccessService,
	members MembershipRepository,
	invitations InvitationRepository,
	owners PropertyOwnerResolver,
	statuses PropertyStatusResolver,
	users UserLookup,
	policy sharedpolicy.Policy,
	slots *SlotCoordinator,
	mailer AccessMailer,
	lifecycle *LifecycleMailer,
	titles PropertyTitleResolver,
	factory txStoreFactory,
	clk clock.Clock,
	logger *slog.Logger,
) *InvitationService {
	if logger == nil {
		logger = slog.Default()
	}
	if clk == nil {
		clk = clock.Real{}
	}
	return &InvitationService{
		txStoreFactory: factory,
		access:         access,
		members:        members,
		invitations:    invitations,
		owners:         owners,
		statuses:       statuses,
		users:          users,
		policy:         policy,
		slots:          slots,
		mailer:         mailer,
		lifecycle:      lifecycle,
		titles:         titles,
		clock:          clk,
		logger:         logger,
	}
}

// InviteByEmail invites an email to shared access on a property. A registered
// email activates instantly through the regular AddMember flow (no invite
// email; a duplicate membership is ErrMemberAlreadyExists); an unregistered
// email becomes a pending invitation and the single invite email is sent after
// commit (a send failure is logged but the invitation stays — a manual resend
// is available). The owner's email is ErrCannotAddOwner, the actor's own email
// is ErrCannotAddSelf, and a duplicate pending invitation is
// ErrInvitationAlreadyExists. An archived property is ErrPropertyArchived for
// both paths (issue #163).
func (s *InvitationService) InviteByEmail(
	ctx context.Context,
	actor, propertyID uuid.UUID,
	rawEmail string,
	role domain.Role,
) (InviteOutcome, error) {
	email, err := domain.NormalizeEmail(rawEmail)
	if err != nil {
		return InviteOutcome{}, err
	}
	owner, actorRole, err := requireManageAccess(ctx, s.policy, s.owners, actor, propertyID)
	if err != nil {
		return InviteOutcome{}, err
	}
	// The archived gate runs before the user lookup so the registered and the
	// unregistered path get the uniform ErrPropertyArchived (issue #163).
	if err := requireNotArchived(ctx, s.statuses, propertyID); err != nil {
		return InviteOutcome{}, err
	}

	user, err := s.users.GetByEmail(ctx, email)
	switch {
	case err == nil:
		return s.inviteRegisteredUser(ctx, actor, propertyID, owner, user, role)
	case errors.Is(err, domain.ErrUserNotFound):
		// Unregistered invitee: fall through to the pending invitation path.
	default:
		return InviteOutcome{}, fmt.Errorf("lookup user by email: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return InviteOutcome{}, fmt.Errorf("generate invitation id: %w", err)
	}
	invitation := domain.Invitation{
		ID:         id,
		PropertyID: propertyID,
		Email:      email,
		Role:       role,
		InvitedBy:  actor,
		LastSentAt: s.clock.Now(),
	}

	created, err := s.createPendingInvitation(ctx, invitation, actor, actorRole)
	if err != nil {
		return InviteOutcome{}, err
	}

	// The invite email goes out after the commit; a send failure does not roll
	// back the invitation (a manual resend is available).
	if err := s.sendInviteEmail(ctx, email, propertyID, role); err != nil {
		s.logger.ErrorContext(ctx, "access: invite email send failed",
			slog.String("invitation_id", created.ID.String()),
			slog.String("error", err.Error()))
	}
	return InviteOutcome{Invitation: &created}, nil
}

// inviteRegisteredUser activates a registered invitee instantly through the
// regular member flow — no invite email is sent (issue #161, T5). The owner's
// own email and the actor's own email are rejected exactly as in AddMember.
func (s *InvitationService) inviteRegisteredUser(
	ctx context.Context, actor, propertyID, owner uuid.UUID, user MemberUser, role domain.Role,
) (InviteOutcome, error) {
	if user.ID == owner {
		return InviteOutcome{}, domain.ErrCannotAddOwner
	}
	if user.ID == actor {
		return InviteOutcome{}, domain.ErrCannotAddSelf
	}
	m, err := s.access.AddMember(ctx, actor, propertyID, user.ID, role)
	if err != nil {
		return InviteOutcome{}, err
	}
	return InviteOutcome{Member: &m}, nil
}

// createPendingInvitation stores the invitation and its audit entry in one
// transaction; a duplicate pending invitation is ErrInvitationAlreadyExists.
func (s *InvitationService) createPendingInvitation(
	ctx context.Context, invitation domain.Invitation, actor uuid.UUID, actorRole sharedpolicy.Role,
) (domain.Invitation, error) {
	var created domain.Invitation
	err := s.runInTx(ctx, func(stores *txStores) error {
		if _, err := stores.invitations.GetByPropertyAndEmail(ctx, invitation.PropertyID, invitation.Email); err == nil {
			return domain.ErrInvitationAlreadyExists
		} else if !errors.Is(err, domain.ErrInvitationNotFound) {
			return fmt.Errorf("check existing invitation: %w", err)
		}

		var err error
		created, err = stores.invitations.Create(ctx, invitation)
		if err != nil {
			return fmt.Errorf("create invitation: %w", err)
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
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Invitation{}, err
	}
	return created, nil
}

// ResendInvitation manually re-sends the invite email for a pending
// invitation. A resend within 24h of the last send is rejected with
// ResendCooldownError (matching ErrInvitationResendCooldown). The email is
// sent before last_sent_at is updated, so a send failure does not burn the
// cooldown window.
func (s *InvitationService) ResendInvitation(ctx context.Context, actor, propertyID, invitationID uuid.UUID) error {
	_, actorRole, err := requireManageAccess(ctx, s.policy, s.owners, actor, propertyID)
	if err != nil {
		return err
	}

	invitation, err := s.invitations.GetByID(ctx, invitationID, propertyID)
	if err != nil {
		return err
	}

	now := s.clock.Now()
	if elapsed := now.Sub(invitation.LastSentAt); elapsed < domain.InvitationResendCooldown {
		return &domain.ResendCooldownError{RetryAfter: domain.InvitationResendCooldown - elapsed}
	}

	if err := s.sendInviteEmail(ctx, invitation.Email, propertyID, invitation.Role); err != nil {
		return fmt.Errorf("send invite email: %w", err)
	}

	return s.runInTx(ctx, func(stores *txStores) error {
		if err := stores.invitations.UpdateLastSentAt(ctx, invitationID, propertyID, now); err != nil {
			return fmt.Errorf("update invitation last_sent_at: %w", err)
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  sharedpolicy.AuditActorRole(actorRole),
			Action:     auditdomain.ActionPropertyMemberInvitationResent,
			EntityType: auditdomain.EntityPropertyMemberInvitation,
			EntityID:   &invitation.ID,
			Context: map[string]any{
				auditKeyPropertyID: propertyID,
			},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
}

// ChangeInvitationRole changes the role of a pending invitation. No new email
// is sent; the role current at registration time is applied on activation.
func (s *InvitationService) ChangeInvitationRole(
	ctx context.Context,
	actor, propertyID, invitationID uuid.UUID,
	role domain.Role,
) (domain.Invitation, error) {
	_, actorRole, err := requireManageAccess(ctx, s.policy, s.owners, actor, propertyID)
	if err != nil {
		return domain.Invitation{}, err
	}

	var updated domain.Invitation
	err = s.runInTx(ctx, func(stores *txStores) error {
		// Confirm the invitation exists and belongs to this property before
		// updating; a missing row is a not-found outcome rather than a silent no-op.
		if _, err := stores.invitations.GetByID(ctx, invitationID, propertyID); err != nil {
			return err
		}

		var err error
		updated, err = stores.invitations.UpdateRole(ctx, invitationID, propertyID, role)
		if err != nil {
			return fmt.Errorf("update invitation role: %w", err)
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  sharedpolicy.AuditActorRole(actorRole),
			Action:     auditdomain.ActionPropertyMemberInvitationRoleChanged,
			EntityType: auditdomain.EntityPropertyMemberInvitation,
			EntityID:   &updated.ID,
			Context: map[string]any{
				auditKeyPropertyID: propertyID,
				auditKeyRole:       string(role),
			},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Invitation{}, err
	}
	return updated, nil
}

// CancelInvitation silently cancels a pending invitation: the row is deleted
// and no email is sent, so the same email can be invited again freely.
func (s *InvitationService) CancelInvitation(ctx context.Context, actor, propertyID, invitationID uuid.UUID) error {
	_, actorRole, err := requireManageAccess(ctx, s.policy, s.owners, actor, propertyID)
	if err != nil {
		return err
	}

	return s.runInTx(ctx, func(stores *txStores) error {
		if _, err := stores.invitations.GetByID(ctx, invitationID, propertyID); err != nil {
			return err
		}

		if err := stores.invitations.Delete(ctx, invitationID, propertyID); err != nil {
			return fmt.Errorf("delete invitation: %w", err)
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  sharedpolicy.AuditActorRole(actorRole),
			Action:     auditdomain.ActionPropertyMemberInvitationCancelled,
			EntityType: auditdomain.EntityPropertyMemberInvitation,
			EntityID:   &invitationID,
			Context: map[string]any{
				auditKeyPropertyID: propertyID,
			},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
}

// ActivatePendingInvitations turns every pending invitation for the email into
// a membership for the freshly registered user, oldest invitation first (FIFO
// across properties). Invitations whose property already grants the user a
// membership (any status) are dropped silently. When the recipient has no free
// tariff slot the membership is created suspended (issue #158, T4 mechanics);
// an invitation to an archived property activates read-only without a slot
// (issue #163). A failing activation is logged and does not block the
// remaining ones.
func (s *InvitationService) ActivatePendingInvitations(ctx context.Context, userID uuid.UUID, rawEmail string) error {
	email, err := domain.NormalizeEmail(rawEmail)
	if errors.Is(err, domain.ErrInvalidEmail) {
		// A user without a (valid) email has nothing to activate.
		return nil
	}
	if err != nil {
		return fmt.Errorf("normalize email: %w", err)
	}

	pending, err := s.invitations.ListPendingByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("list pending invitations: %w", err)
	}

	for _, invitation := range pending {
		if err := s.activateInvitation(ctx, userID, invitation); err != nil {
			s.logger.ErrorContext(ctx, "access: invitation activation failed",
				slog.String("invitation_id", invitation.ID.String()),
				slog.String(auditKeyPropertyID, invitation.PropertyID.String()),
				slog.String(auditKeyUserID, userID.String()),
				slog.String("error", err.Error()))
		}
	}
	return nil
}

// activateInvitation activates a single pending invitation in its own
// transaction.
func (s *InvitationService) activateInvitation(ctx context.Context, userID uuid.UUID, invitation domain.Invitation) error {
	suspend := false
	err := s.runInTx(ctx, func(stores *txStores) error {
		dropped, err := s.dropInvitationIfAlreadyMember(ctx, stores, userID, invitation)
		if err != nil {
			return err
		}
		if dropped {
			return nil
		}

		suspend, err = s.resolveActivationSuspend(ctx, stores.tx, userID, invitation.PropertyID)
		if err != nil {
			return err
		}
		return s.insertActivationMembership(ctx, stores, userID, invitation, suspend)
	})
	if err != nil {
		return err
	}

	// Lifecycle emails post-commit (issue #162, T6): the owner is notified
	// about the activation; a membership created without a free tariff slot
	// additionally sends the "access waits for a free slot" email to the new
	// member. Send failures are logged inside the mailer and never fail the
	// activation.
	owner, err := s.owners.GetOwnerID(ctx, invitation.PropertyID)
	if err != nil {
		s.logger.WarnContext(ctx, "access: owner lookup for invitation activated email failed",
			slog.String(auditKeyPropertyID, invitation.PropertyID.String()),
			slog.String("error", err.Error()))
	} else {
		s.lifecycle.SendInvitationActivated(ctx, owner, invitation.PropertyID, invitation.Email)
	}
	if suspend {
		s.lifecycle.SendAccessSuspended(ctx, userID, invitation.PropertyID)
	}
	return nil
}

// dropInvitationIfAlreadyMember drops the invitation silently when the invitee
// already holds a membership on the property (any status): a duplicate
// membership must not be created. The dropped result reports that the
// activation is complete.
func (s *InvitationService) dropInvitationIfAlreadyMember(
	ctx context.Context, stores *txStores, userID uuid.UUID, invitation domain.Invitation,
) (bool, error) {
	if _, err := stores.members.GetByPropertyAndUser(ctx, invitation.PropertyID, userID); err == nil {
		if err := stores.invitations.Delete(ctx, invitation.ID, invitation.PropertyID); err != nil {
			return false, fmt.Errorf("delete invitation: %w", err)
		}
		return true, nil
	} else if !errors.Is(err, domain.ErrMemberNotFound) {
		return false, fmt.Errorf("check existing membership: %w", err)
	}
	return false, nil
}

// resolveActivationSuspend decides whether the activated membership must be
// created suspended: without a free recipient tariff slot it is (issue #158,
// T4). An archived property occupies no recipient slot (issue #163), so the
// check is skipped and the membership activates read-only in the active
// status.
func (s *InvitationService) resolveActivationSuspend(
	ctx context.Context, tx transaction.Tx, userID, propertyID uuid.UUID,
) (bool, error) {
	archived := false
	if s.statuses != nil {
		var err error
		archived, err = s.statuses.IsArchived(ctx, propertyID)
		if err != nil {
			return false, fmt.Errorf("check property archived: %w", err)
		}
	}
	if archived || s.slots == nil {
		return false, nil
	}
	suspend, err := s.slots.EnforceOnActivation(ctx, tx, userID)
	if err != nil {
		return false, fmt.Errorf("check recipient slot: %w", err)
	}
	return suspend, nil
}

// insertActivationMembership creates the activated membership, consumes the
// invitation and records the audit entry in the same transaction.
func (s *InvitationService) insertActivationMembership(
	ctx context.Context, stores *txStores, userID uuid.UUID, invitation domain.Invitation, suspend bool,
) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate membership id: %w", err)
	}
	membership := domain.Membership{
		ID:         id,
		PropertyID: invitation.PropertyID,
		UserID:     userID,
		Role:       invitation.Role, // The role current at activation time.
		GrantedBy:  invitation.InvitedBy,
	}

	var created domain.Membership
	if suspend {
		membership.Status = domain.MemberStatusSuspended
		created, err = stores.members.CreateWithStatus(ctx, membership)
	} else {
		created, err = stores.members.Create(ctx, membership)
	}
	if err != nil {
		return fmt.Errorf("create membership: %w", err)
	}

	if err := stores.invitations.Delete(ctx, invitation.ID, invitation.PropertyID); err != nil {
		return fmt.Errorf("delete invitation: %w", err)
	}

	// Audit in the same transaction; the invitee email is PII and must never
	// appear in context (ADR 0020).
	if err := stores.audit.Record(ctx, auditdomain.Entry{
		// System action at registration: actor_id is NULL (ADR 0020); the
		// activated user is carried in context, not as the actor.
		ActorRole:  auditdomain.ActorRoleSystem,
		Action:     auditdomain.ActionPropertyMemberInvitationActivated,
		EntityType: auditdomain.EntityPropertyMemberInvitation,
		EntityID:   &invitation.ID,
		Context: map[string]any{
			auditKeyTrigger:    "registration",
			auditKeyPropertyID: invitation.PropertyID,
			auditKeyUserID:     userID,
			"membership_id":    created.ID,
			auditKeyRole:       string(created.Role),
			"status":           string(created.Status),
		},
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}
	return nil
}

// ListMembers returns the property participants (owner first, then membership
// rows) and, for actors with the manage-members capability, the pending email
// invitations of the property appended after the members. Pending rows expose
// the invitee email, so they are manager-only; viewers get the plain member
// list.
func (s *InvitationService) ListMembers(ctx context.Context, actor, propertyID uuid.UUID) ([]Member, error) {
	members, err := s.access.ListMembers(ctx, actor, propertyID)
	if err != nil {
		return nil, err
	}

	role, err := s.policy.RoleForProperty(ctx, actor, propertyID)
	if err != nil {
		return nil, fmt.Errorf("resolve role: %w", err)
	}
	if !sharedpolicy.CanManageMembers(role) {
		return members, nil
	}

	invitations, err := s.invitations.ListByProperty(ctx, propertyID)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	for _, inv := range invitations {
		email := inv.Email
		lastSentAt := inv.LastSentAt
		members = append(members, Member{
			ID:         inv.ID,
			Role:       toSharedRole(inv.Role),
			Pending:    true,
			Email:      &email,
			LastSentAt: &lastSentAt,
		})
	}
	return members, nil
}

// sendInviteEmail renders and sends the single invite email. A missing
// property title degrades to a generic text rather than failing the send.
func (s *InvitationService) sendInviteEmail(ctx context.Context, email string, propertyID uuid.UUID, role domain.Role) error {
	if s.mailer == nil {
		return nil
	}
	var title string
	if s.titles != nil {
		resolved, err := s.titles.GetTitle(ctx, propertyID)
		if err != nil {
			s.logger.WarnContext(ctx, "access: property title lookup for invite email failed",
				slog.String(auditKeyPropertyID, propertyID.String()),
				slog.String("error", err.Error()))
		} else {
			title = resolved
		}
	}
	return s.mailer.SendInvite(ctx, email, title, role)
}
