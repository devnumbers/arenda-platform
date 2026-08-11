package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
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
// the same transaction; the invitee email is PII and never appears in audit
// context (ADR 0020).
type InvitationService struct {
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
	db          txBeginner
	audit       auditapp.Recorder
	clock       clock.Clock
	logger      *slog.Logger
}

// NewInvitationService creates an InvitationService. access is the membership
// service used to delegate instant activation for registered emails; slots may
// be nil to disable slot enforcement (mirrors NewAccessService); mailer may be
// nil to skip sending (e.g. in tests that do not exercise the mail path);
// lifecycle is the sharing lifecycle mailer (issue #162, T6) and may be nil to
// disable the lifecycle emails. statuses reports the archived flag of a
// property (issue #163); it may be nil to skip the archived-property checks.
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
	db txBeginner,
	audit auditapp.Recorder,
	clk clock.Clock,
	logger *slog.Logger,
) *InvitationService {
	if logger == nil {
		logger = slog.Default()
	}
	if audit == nil {
		audit = auditapp.Noop{}
	}
	if clk == nil {
		clk = clock.Real{}
	}
	return &InvitationService{
		access:      access,
		members:     members,
		invitations: invitations,
		owners:      owners,
		statuses:    statuses,
		users:       users,
		policy:      policy,
		slots:       slots,
		mailer:      mailer,
		lifecycle:   lifecycle,
		titles:      titles,
		db:          db,
		audit:       audit,
		clock:       clk,
		logger:      logger,
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
func (s *InvitationService) InviteByEmail(ctx context.Context, actor, propertyID uuid.UUID, rawEmail string, role domain.Role) (InviteOutcome, error) {
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
		if user.ID == owner {
			return InviteOutcome{}, domain.ErrCannotAddOwner
		}
		if user.ID == actor {
			return InviteOutcome{}, domain.ErrCannotAddSelf
		}
		// Registered invitee: instant activation through the regular member
		// flow, no invite email (issue #161, T5).
		m, err := s.access.AddMember(ctx, actor, propertyID, user.ID, role)
		if err != nil {
			return InviteOutcome{}, err
		}
		return InviteOutcome{Member: &m}, nil
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

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return InviteOutcome{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txInvitations := s.invitations.WithTx(tx)
	if _, err := txInvitations.GetByPropertyAndEmail(ctx, propertyID, email); err == nil {
		return InviteOutcome{}, domain.ErrInvitationAlreadyExists
	} else if !errors.Is(err, domain.ErrInvitationNotFound) {
		return InviteOutcome{}, fmt.Errorf("check existing invitation: %w", err)
	}

	created, err := txInvitations.Create(ctx, invitation)
	if err != nil {
		return InviteOutcome{}, fmt.Errorf("create invitation: %w", err)
	}

	// Audit in the same transaction. Only ids and the role are recorded; the
	// invitee email is PII and must never appear in context (ADR 0020).
	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  actorRoleFromPolicyRole(actorRole),
		Action:     auditdomain.ActionPropertyMemberInvitationInvited,
		EntityType: auditdomain.EntityPropertyMemberInvitation,
		EntityID:   &created.ID,
		Context: map[string]any{
			"property_id": propertyID,
			"role":        string(role),
		},
	}); err != nil {
		return InviteOutcome{}, fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return InviteOutcome{}, fmt.Errorf("commit tx: %w", err)
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

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.invitations.WithTx(tx).UpdateLastSentAt(ctx, invitationID, propertyID, now); err != nil {
		return fmt.Errorf("update invitation last_sent_at: %w", err)
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  actorRoleFromPolicyRole(actorRole),
		Action:     auditdomain.ActionPropertyMemberInvitationResent,
		EntityType: auditdomain.EntityPropertyMemberInvitation,
		EntityID:   &invitation.ID,
		Context: map[string]any{
			"property_id": propertyID,
		},
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// ChangeInvitationRole changes the role of a pending invitation. No new email
// is sent; the role current at registration time is applied on activation.
func (s *InvitationService) ChangeInvitationRole(ctx context.Context, actor, propertyID, invitationID uuid.UUID, role domain.Role) (domain.Invitation, error) {
	_, actorRole, err := requireManageAccess(ctx, s.policy, s.owners, actor, propertyID)
	if err != nil {
		return domain.Invitation{}, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Invitation{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txInvitations := s.invitations.WithTx(tx)
	// Confirm the invitation exists and belongs to this property before
	// updating; a missing row is a not-found outcome rather than a silent no-op.
	if _, err := txInvitations.GetByID(ctx, invitationID, propertyID); err != nil {
		return domain.Invitation{}, err
	}

	updated, err := txInvitations.UpdateRole(ctx, invitationID, propertyID, role)
	if err != nil {
		return domain.Invitation{}, fmt.Errorf("update invitation role: %w", err)
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  actorRoleFromPolicyRole(actorRole),
		Action:     auditdomain.ActionPropertyMemberInvitationRoleChanged,
		EntityType: auditdomain.EntityPropertyMemberInvitation,
		EntityID:   &updated.ID,
		Context: map[string]any{
			"property_id": propertyID,
			"role":        string(role),
		},
	}); err != nil {
		return domain.Invitation{}, fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Invitation{}, fmt.Errorf("commit tx: %w", err)
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

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txInvitations := s.invitations.WithTx(tx)
	if _, err := txInvitations.GetByID(ctx, invitationID, propertyID); err != nil {
		return err
	}

	if err := txInvitations.Delete(ctx, invitationID, propertyID); err != nil {
		return fmt.Errorf("delete invitation: %w", err)
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  actorRoleFromPolicyRole(actorRole),
		Action:     auditdomain.ActionPropertyMemberInvitationCancelled,
		EntityType: auditdomain.EntityPropertyMemberInvitation,
		EntityID:   &invitationID,
		Context: map[string]any{
			"property_id": propertyID,
		},
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
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
				slog.String("property_id", invitation.PropertyID.String()),
				slog.String("user_id", userID.String()),
				slog.String("error", err.Error()))
		}
	}
	return nil
}

// activateInvitation activates a single pending invitation in its own
// transaction.
func (s *InvitationService) activateInvitation(ctx context.Context, userID uuid.UUID, invitation domain.Invitation) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txMembers := s.members.WithTx(tx)
	txInvitations := s.invitations.WithTx(tx)

	if _, err := txMembers.GetByPropertyAndUser(ctx, invitation.PropertyID, userID); err == nil {
		// Already a member on this property (any status): drop the invitation
		// silently.
		if err := txInvitations.Delete(ctx, invitation.ID, invitation.PropertyID); err != nil {
			return fmt.Errorf("delete invitation: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit tx: %w", err)
		}
		return nil
	} else if !errors.Is(err, domain.ErrMemberNotFound) {
		return fmt.Errorf("check existing membership: %w", err)
	}

	// Enforce the recipient tariff slot invariant (issue #158, T4): without a
	// free slot the membership is created suspended until one frees up. An
	// archived property does not occupy a recipient slot (issue #163), so the
	// slot check is skipped and the membership activates read-only in the
	// active status — with the archived object excluded from slot accounting it
	// occupies nothing.
	suspend := false
	archived := false
	if s.statuses != nil {
		archived, err = s.statuses.IsArchived(ctx, invitation.PropertyID)
		if err != nil {
			return fmt.Errorf("check property archived: %w", err)
		}
	}
	if !archived && s.slots != nil {
		suspend, err = s.slots.EnforceOnActivation(ctx, tx, userID)
		if err != nil {
			return fmt.Errorf("check recipient slot: %w", err)
		}
	}

	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate membership id: %w", err)
	}
	membership := domain.Membership{
		ID:         id,
		PropertyID: invitation.PropertyID,
		UserID:     userID,
		Role:       invitation.Role, // the role current at activation time
		GrantedBy:  invitation.InvitedBy,
	}

	var created domain.Membership
	if suspend {
		membership.Status = domain.MemberStatusSuspended
		created, err = txMembers.CreateWithStatus(ctx, membership)
	} else {
		created, err = txMembers.Create(ctx, membership)
	}
	if err != nil {
		return fmt.Errorf("create membership: %w", err)
	}

	if err := txInvitations.Delete(ctx, invitation.ID, invitation.PropertyID); err != nil {
		return fmt.Errorf("delete invitation: %w", err)
	}

	// Audit in the same transaction; the invitee email is PII and must never
	// appear in context (ADR 0020).
	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		// System action at registration: actor_id is NULL (ADR 0020); the
		// activated user is carried in context, not as the actor.
		ActorRole:  auditdomain.ActorRoleSystem,
		Action:     auditdomain.ActionPropertyMemberInvitationActivated,
		EntityType: auditdomain.EntityPropertyMemberInvitation,
		EntityID:   &invitation.ID,
		Context: map[string]any{
			"trigger":       "registration",
			"property_id":   invitation.PropertyID,
			"user_id":       userID,
			"membership_id": created.ID,
			"role":          string(created.Role),
			"status":        string(created.Status),
		},
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	// Lifecycle emails post-commit (issue #162, T6): the owner is notified
	// about the activation; a membership created without a free tariff slot
	// additionally sends the "access waits for a free slot" email to the new
	// member. Send failures are logged inside the mailer and never fail the
	// activation.
	owner, err := s.owners.GetOwnerID(ctx, invitation.PropertyID)
	if err != nil {
		s.logger.WarnContext(ctx, "access: owner lookup for invitation activated email failed",
			slog.String("property_id", invitation.PropertyID.String()),
			slog.String("error", err.Error()))
	} else {
		s.lifecycle.SendInvitationActivated(ctx, owner, invitation.PropertyID, invitation.Email)
	}
	if suspend {
		s.lifecycle.SendAccessSuspended(ctx, userID, invitation.PropertyID)
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
				slog.String("property_id", propertyID.String()),
				slog.String("error", err.Error()))
		} else {
			title = resolved
		}
	}
	return s.mailer.SendInvite(ctx, email, title, role)
}
