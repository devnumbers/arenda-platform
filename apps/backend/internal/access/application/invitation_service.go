package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
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
// email sent by this service itself; the lifecycle notifications (the
// activation notice to the inviter, the paused access on a suspended
// activation) leave the context as events (карта #734, #751). Persistence,
// the audit entries and the action journal rows (ADR 0061) share the same
// transaction through the embedded txStoreFactory (ADR 0033); the invitee
// email is PII and never appears in audit context (ADR 0020).
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
	events      AccessEventPublisher
	titles      PropertyTitleResolver
	emails      UserEmailResolver
	clock       clock.Clock
	logger      *slog.Logger
}

// NewInvitationService creates an InvitationService. Access is the membership
// service used to delegate instant activation for registered emails; slots may
// be nil to disable slot enforcement (mirrors NewAccessService); mailer may be
// nil to skip sending (e.g. in tests that do not exercise the mail path);
// events is the lifecycle event publisher (карта #734, #751) and may be nil to
// disable the publications; emails may be nil to skip the member-email
// enrichment of ListMembers. Statuses reports the archived flag of a
// property (issue #163); it may be nil to skip the archived-property checks.
// Factory bundles the repositories, the audit and history recorders, and the
// Unit-of-Work every mutating use case runs through (ADR 0033 γ-factory).
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
	events AccessEventPublisher,
	titles PropertyTitleResolver,
	factory txStoreFactory,
	clk clock.Clock,
	logger *slog.Logger,
	emails UserEmailResolver,
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
		events:         events,
		titles:         titles,
		emails:         emails,
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
	// The pending row dirties the object's participants view (карта #714,
	// #716; ADR 0062); its creation journals MemberInvited — the history
	// pair piggybacks.
	s.access.publishRealtime(ctx, actor, accessFrame(propertyID), propertyID)

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
// The row machinery is the shared createInvitationInTx core (the same core
// the batch grant uses).
func (s *InvitationService) createPendingInvitation(
	ctx context.Context, invitation domain.Invitation, actor uuid.UUID, actorRole sharedpolicy.Role,
) (domain.Invitation, error) {
	var (
		created domain.Invitation
		err     error
	)
	err = s.runInTx(ctx, func(stores *txStores) error {
		created, err = s.access.createInvitationInTx(ctx, stores, invitation, actor, actorRole)
		return err
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
// The change journals in the same transaction over the member.role_changed
// vocabulary — the pending invitee is named by the invitation's email; a
// same-role re-set is not a state change and writes no row (ADR 0061 §3).
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
	var journaled bool
	err = s.runInTx(ctx, func(stores *txStores) error {
		// Confirm the invitation exists and belongs to this property before
		// updating; a missing row is a not-found outcome rather than a silent no-op.
		existing, err := stores.invitations.GetByID(ctx, invitationID, propertyID)
		if err != nil {
			return err
		}
		roleChanged := existing.Role != role

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

		// The action journal row (ADR 0061): the pending invitee is known by
		// the invitation's email — the row survives the later acceptance or
		// cancellation. The invitee has no user id yet, so the row carries no
		// member link.
		if roleChanged {
			if err := historyapp.RecordScoped(ctx, stores.history, propertyID, actor,
				sharedpolicy.HistoryActorRole(actorRole),
				historydomain.MemberRoleChanged(uuid.Nil, existing.Email,
					sharedpolicy.HistoryActorRole(toSharedRole(existing.Role)),
					sharedpolicy.HistoryActorRole(toSharedRole(role)))); err != nil {
				return err
			}
			journaled = true
		}
		return nil
	})
	if err != nil {
		return domain.Invitation{}, err
	}
	// The role change's frame rides the journal anchor; a same-role no-op
	// journals no row and dispatches nothing — no change, no frame.
	if journaled {
		s.access.publishRealtime(ctx, actor, accessFrame(propertyID), propertyID)
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

	err = s.runInTx(ctx, func(stores *txStores) error {
		invitation, err := stores.invitations.GetByID(ctx, invitationID, propertyID)
		if err != nil {
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

		// The action journal row (ADR 0061): the invitee is known by the
		// invitation's email — the row survives the cancelled invitation.
		if err := historyapp.RecordScoped(ctx, stores.history, propertyID, actor,
			sharedpolicy.HistoryActorRole(actorRole),
			historydomain.MemberInvitationCancelled(invitation.Email)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.access.publishRealtime(ctx, actor, accessFrame(propertyID), propertyID)
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
	var created domain.Membership
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
		created, err = s.insertActivationMembership(ctx, stores, userID, invitation, suspend)
		return err
	})
	if err != nil {
		return err
	}

	// The lifecycle notifications are the event subscriber's (карта #734,
	// #751) — the activation becomes the «Приглашение принято» row for the
	// inviter and, when the access landed suspended, the system
	// «Доступ приостановлен» row for the new member instead of the
	// invitation one. Published post-commit, best-effort; the direct
	// lifecycle emails this replaces are gone.
	// The transition instant is the membership row's own suspension instant
	// when the landing is suspended (the same source AddMember's pause uses),
	// the activation moment otherwise.
	at := s.clock.Now()
	if suspend {
		at = deref(created.SuspendedAt)
	}
	publishAccessEvent(ctx, s.events, s.logger, "invitation_activated", func() error {
		return s.events.PublishInvitationActivated(ctx, InvitationActivated{
			MembershipID: created.ID,
			PropertyID:   invitation.PropertyID,
			InviterID:    invitation.InvitedBy,
			InviteeID:    userID,
			Suspended:    suspend,
			At:           at,
		})
	})
	// The landed activation dirties the object's participants view (карта
	// #714, #716; ADR 0062); the activation journals no row — the actor is
	// the registration, not a member manager.
	s.access.publishRealtime(ctx, userID, accessFrame(invitation.PropertyID))
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
// invitation and records the audit entry in the same transaction; the created
// membership travels back for the post-commit event (#751).
func (s *InvitationService) insertActivationMembership(
	ctx context.Context, stores *txStores, userID uuid.UUID, invitation domain.Invitation, suspend bool,
) (domain.Membership, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return domain.Membership{}, fmt.Errorf("generate membership id: %w", err)
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
		return domain.Membership{}, fmt.Errorf("create membership: %w", err)
	}

	if err := stores.invitations.Delete(ctx, invitation.ID, invitation.PropertyID); err != nil {
		return domain.Membership{}, fmt.Errorf("delete invitation: %w", err)
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
		return domain.Membership{}, fmt.Errorf("record audit: %w", err)
	}
	return created, nil
}

// ListMembers returns the property participants (owner first, then membership
// rows) with the pending email invitations appended after the members. Every
// reader with the view capability (gated by AccessService.ListMembers) sees
// the same rows, including member emails and pending invitations (owner
// decision 2026-09-20, #758 walkthrough: nothing is hidden from viewers).
// Email resolution degrades to a nil email when the resolver is not wired or
// a lookup fails — the row stays.
func (s *InvitationService) ListMembers(ctx context.Context, actor, propertyID uuid.UUID) ([]Member, error) {
	members, err := s.access.ListMembers(ctx, actor, propertyID)
	if err != nil {
		return nil, err
	}

	for i := range members {
		m := &members[i]
		if m.Pending || m.UserID == uuid.Nil || s.emails == nil {
			continue
		}
		email, err := s.emails.GetEmail(ctx, m.UserID)
		if err != nil {
			s.logger.WarnContext(ctx, "access: member email lookup failed",
				slog.String(auditKeyUserID, m.UserID.String()),
				slog.String("error", err.Error()))
			continue
		}
		if email != "" {
			m.Email = &email
		}
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
// property title degrades to the generic text (an empty title list, the
// shared resolveInviteTitles) rather than failing the send.
func (s *InvitationService) sendInviteEmail(ctx context.Context, email string, propertyID uuid.UUID, role domain.Role) error {
	if s.mailer == nil {
		return nil
	}
	titles := resolveInviteTitles(ctx, s.titles, s.logger, []uuid.UUID{propertyID})
	return s.mailer.SendInvite(ctx, email, titles, role)
}
