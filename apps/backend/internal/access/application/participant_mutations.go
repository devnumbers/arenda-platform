package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// ParticipantGrantOutcome is the per-property result of a participant grant
// batch (issue #694). The granted outcomes reuse the aggregate entry-status
// vocabulary (active/suspended/pending); the skipped ones explain why a
// requested property got no grant.
type ParticipantGrantOutcome string

const (
	// ParticipantGrantActive — a membership was created with a free recipient slot.
	ParticipantGrantActive ParticipantGrantOutcome = "active"
	// ParticipantGrantSuspended — a membership was created without a free
	// recipient slot (issue #158, T4).
	ParticipantGrantSuspended ParticipantGrantOutcome = "suspended"
	// ParticipantGrantPending — a pending invitation was stored; the single
	// batch invite email is sent post-commit.
	ParticipantGrantPending ParticipantGrantOutcome = "pending"
	// ParticipantGrantDuplicate — the person already holds access (any
	// status) or a pending invitation on the property. Roles are never
	// overwritten by a batch grant.
	ParticipantGrantDuplicate ParticipantGrantOutcome = "skipped_duplicate"
	// ParticipantGrantArchived — the property is archived; granting new
	// shared access is forbidden (issue #163).
	ParticipantGrantArchived ParticipantGrantOutcome = "skipped_archived"
	// ParticipantGrantOwner — the target person is the property's owner
	// themself; ownership is the only self-held access.
	ParticipantGrantOwner ParticipantGrantOutcome = "skipped_owner"
	// ParticipantGrantUnavailable — the property is not in the reading
	// actor's manage scope (unknown, foreign, or otherwise unmanageable).
	ParticipantGrantUnavailable ParticipantGrantOutcome = "skipped_unavailable"
)

// String returns the string representation of the outcome.
func (o ParticipantGrantOutcome) String() string { return string(o) }

// ParticipantGrantResult is one requested property's grant outcome. Exactly
// one of MembershipID / InvitationID is set for the granted outcomes; both
// stay zero for the skipped ones.
type ParticipantGrantResult struct {
	PropertyID   uuid.UUID
	Outcome      ParticipantGrantOutcome
	MembershipID uuid.UUID
	InvitationID uuid.UUID
}

// ParticipantMutations is the use-case interface the HTTP adapters consume
// for the mutation side of the owner's participant aggregate (issue #694,
// ADR 0035 func-backed test doubles); *ParticipantMutationService implements
// it.
type ParticipantMutations interface {
	// Invite invites an email to several objects with one role — the
	// snapshot semantics of the «Совместный доступ» hub: grants are created
	// on the objects chosen now; future objects are not shared automatically.
	Invite(ctx context.Context, actor uuid.UUID, rawEmail string, role domain.Role, propertyIDs []uuid.UUID) ([]ParticipantGrantResult, error)
	// AddProperties grants an existing participant (a user uuid or an
	// invitee email identifier) access to more objects — «Пригласить в
	// объект» on the participant page.
	AddProperties(ctx context.Context, actor uuid.UUID, participantID string,
		role domain.Role, propertyIDs []uuid.UUID) ([]ParticipantGrantResult, error)
	// Remove revokes and deletes the participant: every membership and
	// pending invitation they hold on the properties the actor manages —
	// one transaction, an audit entry per row, FIFO recovery of the person's
	// suspended accesses, no emails.
	Remove(ctx context.Context, actor uuid.UUID, participantID string) error
}

// grantTarget is the resolved person of a grant batch: a registered user's
// id, or a pending invitee's normalized email. UserID is zero for a pending
// email; email is empty for a pending target and for a registered user
// without one.
type grantTarget struct {
	userID uuid.UUID
	email  string
}

// ParticipantMutationService implements the mutation side of the «Участник
// (владельца)» aggregate (issue #694) on top of the existing per-property
// gates: manage scope (CanManageMembers), the archived-property gate (issue
// #163) and the recipient tariff slots (SlotCoordinator) are evaluated per
// property, every row is audited in the same transaction (ADR 0020/0033),
// and a batch reports per-property outcomes instead of failing as a whole.
// The only direct email is the batch invite one; the lifecycle notifications
// (the suspended grant, the revoked active legs) leave the context as events
// (карта #734, #751).
type ParticipantMutationService struct {
	txStoreFactory
	access   *AccessService
	owners   PropertyOwnerResolver
	statuses PropertyStatusResolver
	users    UserLookup
	emails   UserEmailResolver
	policy   sharedpolicy.Policy
	slots    *SlotCoordinator
	mailer   AccessMailer
	events   AccessEventPublisher
	titles   PropertyTitleResolver
	clock    clock.Clock
	logger   *slog.Logger
}

// NewParticipantMutationService creates a ParticipantMutationService. Statuses
// may be nil to skip the archived-property checks (tests without the
// dependency); slots may be nil to disable slot enforcement; mailer/titles may
// be nil to skip the invite email; events may be nil to disable the
// publications; emails may be nil (registered targets then resolve without
// invitation-leg lookups).
func NewParticipantMutationService(
	access *AccessService,
	owners PropertyOwnerResolver,
	statuses PropertyStatusResolver,
	users UserLookup,
	emails UserEmailResolver,
	policy sharedpolicy.Policy,
	slots *SlotCoordinator,
	mailer AccessMailer,
	events AccessEventPublisher,
	titles PropertyTitleResolver,
	factory txStoreFactory,
	clk clock.Clock,
	logger *slog.Logger,
) *ParticipantMutationService {
	if logger == nil {
		logger = slog.Default()
	}
	if clk == nil {
		clk = clock.Real{}
	}
	return &ParticipantMutationService{
		txStoreFactory: factory,
		access:         access,
		owners:         owners,
		statuses:       statuses,
		users:          users,
		emails:         emails,
		policy:         policy,
		slots:          slots,
		mailer:         mailer,
		events:         events,
		titles:         titles,
		clock:          clk,
		logger:         logger,
	}
}

// Invite invites an email to several objects with one role (issue #694). A
// registered email activates instantly — each property is slot-checked
// individually, so one batch can mix active and suspended memberships; an
// unregistered email gets one pending invitation per property and exactly one
// invite email listing the granted objects, sent post-commit. Every requested
// property id comes back with its per-property outcome; duplicate request ids
// collapse to the first occurrence. The actor's own email is
// ErrCannotAddSelf.
func (s *ParticipantMutationService) Invite(
	ctx context.Context, actor uuid.UUID, rawEmail string, role domain.Role, propertyIDs []uuid.UUID,
) ([]ParticipantGrantResult, error) {
	email, err := domain.NormalizeEmail(rawEmail)
	if err != nil {
		return nil, err
	}
	target := grantTarget{email: email}
	user, err := s.users.GetByEmail(ctx, email)
	switch {
	case err == nil:
		target = grantTarget{userID: user.ID}
	case errors.Is(err, domain.ErrUserNotFound):
		// Unregistered invitee: fall through to the pending path.
	default:
		return nil, fmt.Errorf("lookup user by email: %w", err)
	}
	return s.grantProperties(ctx, actor, target, role, propertyIDs, false)
}

// AddProperties grants an existing participant access to more objects with
// one role (issue #694). The identifier resolves like everywhere else on the
// participant API — a registered user's uuid, or an invitee email (which may
// already have resolved to a registered user). A person with no legs in the
// actor's manage scope is the privacy-preserving ErrParticipantNotFound.
func (s *ParticipantMutationService) AddProperties(
	ctx context.Context, actor uuid.UUID, participantID string, role domain.Role, propertyIDs []uuid.UUID,
) ([]ParticipantGrantResult, error) {
	userID, email, err := s.resolveTarget(ctx, participantID)
	if err != nil {
		return nil, err
	}
	if userID == actor {
		// The actor is never their own participant (the aggregate hides them).
		return nil, domain.ErrParticipantNotFound
	}
	return s.grantProperties(ctx, actor, grantTarget{userID: userID, email: email}, role, propertyIDs, true)
}

// grantProperties is the shared batch core of Invite and AddProperties. The
// whole batch runs in one transaction; a repo failure aborts it, while
// per-property gate outcomes (duplicate/archived/owner/unavailable) skip just
// their property. With requireExisting the person must already hold a leg in
// the actor's scope («Пригласить в объект» targets existing participants).
func (s *ParticipantMutationService) grantProperties(
	ctx context.Context, actor uuid.UUID, target grantTarget, role domain.Role, propertyIDs []uuid.UUID, requireExisting bool,
) ([]ParticipantGrantResult, error) {
	if target.userID != (uuid.UUID{}) && target.userID == actor {
		return nil, domain.ErrCannotAddSelf
	}
	ids := dedupePropertyIDs(propertyIDs)

	results := make([]ParticipantGrantResult, 0, len(ids))
	var suspended []domain.Membership
	err := s.runInTx(ctx, func(stores *txStores) error {
		if requireExisting {
			exists, err := s.personInScope(ctx, stores, target, actor)
			if err != nil {
				return err
			}
			if !exists {
				return domain.ErrParticipantNotFound
			}
		}
		for _, propertyID := range ids {
			result, err := s.grantOne(ctx, stores, actor, target, role, propertyID, &suspended)
			if err != nil {
				return err
			}
			results = append(results, result)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Each active landing notifies the new member post-commit (issue #829):
	// one «Приглашение в объект» event per active leg, the granter the actor.
	// A grant created without a free tariff slot pauses the new member's
	// access (issue #158, T4) and speaks the system «Доступ приостановлен»
	// instead — the event subscriber's (карта #734, #751), published
	// post-commit, best-effort, no actor.
	var grantedActive []domain.Membership
	for _, r := range results {
		if r.Outcome != ParticipantGrantActive {
			continue
		}
		grantedActive = append(grantedActive, domain.Membership{
			ID:         r.MembershipID,
			PropertyID: r.PropertyID,
			UserID:     target.userID,
		})
	}
	publishGrantedEvents(ctx, s.events, s.logger, actor, grantedActive)
	publishSuspendedEvents(ctx, s.events, s.logger, suspended)

	// The single batch invite email goes out after the commit; a send failure
	// does not roll the invitations back (a manual resend is available).
	granted := make([]uuid.UUID, 0, len(results))
	for _, r := range results {
		if r.Outcome == ParticipantGrantPending {
			granted = append(granted, r.PropertyID)
		}
	}
	if len(granted) > 0 {
		s.sendInviteEmail(ctx, target.email, granted, role)
	}
	return results, nil
}

// grantOne produces one requested property's outcome. Gate failures skip the
// property (they are outcomes, not errors); a repository failure returns an
// error and aborts the transaction. A suspended landing appends its membership
// row to the collector — the caller publishes its system pause post-commit
// (карта #734, #751).
func (s *ParticipantMutationService) grantOne(
	ctx context.Context, stores *txStores, actor uuid.UUID, target grantTarget, role domain.Role, propertyID uuid.UUID,
	suspended *[]domain.Membership,
) (ParticipantGrantResult, error) {
	owner, actorRole, err := requireManageAccess(ctx, s.policy, s.owners, actor, propertyID)
	if err != nil {
		// Not in the actor's manage scope: the gate signals it with the
		// privacy-preserving not-found — a per-property skip, not a batch
		// failure. Any other error is infrastructural and aborts the batch.
		if !errors.Is(err, domain.ErrMemberNotFound) {
			return ParticipantGrantResult{}, fmt.Errorf("resolve manage scope: %w", err)
		}
		return ParticipantGrantResult{PropertyID: propertyID, Outcome: ParticipantGrantUnavailable}, nil
	}
	if target.userID != (uuid.UUID{}) && target.userID == owner {
		return ParticipantGrantResult{PropertyID: propertyID, Outcome: ParticipantGrantOwner}, nil
	}
	if err := requireNotArchived(ctx, s.statuses, propertyID); err != nil {
		if errors.Is(err, domain.ErrPropertyArchived) {
			return ParticipantGrantResult{PropertyID: propertyID, Outcome: ParticipantGrantArchived}, nil
		}
		return ParticipantGrantResult{}, fmt.Errorf("check property archived: %w", err)
	}

	if target.userID != (uuid.UUID{}) {
		return s.grantMembership(ctx, stores, actor, target.userID, actorRole, role, propertyID, suspended)
	}
	return s.grantInvitation(ctx, stores, actor, target.email, actorRole, role, propertyID)
}

// grantMembership creates one membership through the same transactional core
// AddMember uses (duplicate check, recipient slot decision, audited insert);
// a duplicate is a skipped outcome, not an error. A suspended landing joins
// the caller's post-commit publication collector.
func (s *ParticipantMutationService) grantMembership(
	ctx context.Context, stores *txStores, actor, userID uuid.UUID, actorRole sharedpolicy.Role, role domain.Role, propertyID uuid.UUID,
	suspended *[]domain.Membership,
) (ParticipantGrantResult, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return ParticipantGrantResult{}, fmt.Errorf("generate membership id: %w", err)
	}
	created, suspend, err := s.access.createMembershipInTx(ctx, stores, actor, domain.Membership{
		ID:         id,
		PropertyID: propertyID,
		UserID:     userID,
		Role:       role,
		GrantedBy:  actor,
	}, actorRole)
	if errors.Is(err, domain.ErrMemberAlreadyExists) {
		return ParticipantGrantResult{PropertyID: propertyID, Outcome: ParticipantGrantDuplicate}, nil
	}
	if err != nil {
		return ParticipantGrantResult{}, err
	}
	if suspend {
		*suspended = append(*suspended, created)
	}
	outcome := ParticipantGrantActive
	if suspend {
		outcome = ParticipantGrantSuspended
	}
	return ParticipantGrantResult{PropertyID: propertyID, Outcome: outcome, MembershipID: created.ID}, nil
}

// grantInvitation creates one pending invitation through the shared
// createInvitationInTx core (the mirror of grantMembership over
// createMembershipInTx); a duplicate pending invitation is a skipped
// outcome, not an error.
func (s *ParticipantMutationService) grantInvitation(
	ctx context.Context, stores *txStores, actor uuid.UUID, email string, actorRole sharedpolicy.Role, role domain.Role, propertyID uuid.UUID,
) (ParticipantGrantResult, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return ParticipantGrantResult{}, fmt.Errorf("generate invitation id: %w", err)
	}
	created, err := s.access.createInvitationInTx(ctx, stores, domain.Invitation{
		ID:         id,
		PropertyID: propertyID,
		Email:      email,
		Role:       role,
		InvitedBy:  actor,
		LastSentAt: s.clock.Now(),
	}, actor, actorRole)
	if errors.Is(err, domain.ErrInvitationAlreadyExists) {
		return ParticipantGrantResult{PropertyID: propertyID, Outcome: ParticipantGrantDuplicate}, nil
	}
	if err != nil {
		return ParticipantGrantResult{}, err
	}
	return ParticipantGrantResult{PropertyID: propertyID, Outcome: ParticipantGrantPending, InvitationID: created.ID}, nil
}

// personInScope reports whether the target already holds a leg — a membership
// or a pending invitation — inside the actor's manage scope (issue #693).
func (s *ParticipantMutationService) personInScope(
	ctx context.Context, stores *txStores, target grantTarget, actor uuid.UUID,
) (bool, error) {
	if target.userID != (uuid.UUID{}) {
		legs, err := stores.members.ListForRemovalByUser(ctx, target.userID, actor)
		if err != nil {
			return false, fmt.Errorf("list participant memberships: %w", err)
		}
		if len(legs) > 0 {
			return true, nil
		}
	}
	if target.email != "" {
		legs, err := stores.invitations.ListForRemovalByEmail(ctx, target.email, actor)
		if err != nil {
			return false, fmt.Errorf("list participant invitations: %w", err)
		}
		if len(legs) > 0 {
			return true, nil
		}
	}
	return false, nil
}

// Remove revokes and deletes a participant («Отозвать и удалить», issue
// #694): every membership and pending invitation the person holds on the
// properties the actor manages — archived included (revoking keeps working
// on archived objects, issue #163) — in one transaction, with an audit entry
// per removed row mirroring the single-property flows. Freeing active slots
// recovers the person's suspended accesses FIFO (issue #158, T4). No
// lifecycle emails are sent; legs on other owners' properties stay untouched.
// A person with nothing in the actor's scope is ErrParticipantNotFound.
func (s *ParticipantMutationService) Remove(ctx context.Context, actor uuid.UUID, participantID string) error {
	userID, email, err := s.resolveTarget(ctx, participantID)
	if err != nil {
		return err
	}
	if userID == actor {
		// The actor is never their own participant (the aggregate hides them).
		return domain.ErrParticipantNotFound
	}

	freedActive := 0
	var revokedActive []domain.Membership
	err = s.runInTx(ctx, func(stores *txStores) error {
		legs, invitationLegs, err := s.listRemovalLegs(ctx, stores, userID, email, actor)
		if err != nil {
			return err
		}
		if len(legs) == 0 && len(invitationLegs) == 0 {
			return domain.ErrParticipantNotFound
		}
		freedActive, revokedActive, err = s.removeMembershipLegs(ctx, stores, actor, legs)
		if err != nil {
			return err
		}
		if err := s.removeInvitationLegs(ctx, stores, actor, invitationLegs); err != nil {
			return err
		}
		// Every freed active slot lets the oldest suspended access come back
		// FIFO; one recovery pass covers the whole batch.
		if freedActive > 0 && s.slots != nil && userID != (uuid.UUID{}) {
			if err := s.slots.RecoverSuspended(ctx, stores.tx, userID); err != nil {
				return fmt.Errorf("recover suspended after participant removal: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Every removed ACTIVE membership notifies the former member post-commit
	// (карта #734, #751); suspended legs publish nothing — the object was
	// already hidden from them (issue #162, T6 canon).
	publishRevokedEvents(ctx, s.events, s.logger, actor, revokedActive)
	return nil
}

// listRemovalLegs enumerates the person's removal scope: their memberships
// (registered targets) and pending invitations (email-known targets) on the
// properties the actor manages.
func (s *ParticipantMutationService) listRemovalLegs(
	ctx context.Context, stores *txStores, userID uuid.UUID, email string, actor uuid.UUID,
) ([]domain.Membership, []domain.Invitation, error) {
	var legs []domain.Membership
	if userID != (uuid.UUID{}) {
		var err error
		legs, err = stores.members.ListForRemovalByUser(ctx, userID, actor)
		if err != nil {
			return nil, nil, fmt.Errorf("list participant memberships: %w", err)
		}
	}
	var invitationLegs []domain.Invitation
	if email != "" {
		var err error
		invitationLegs, err = stores.invitations.ListForRemovalByEmail(ctx, email, actor)
		if err != nil {
			return nil, nil, fmt.Errorf("list participant invitations: %w", err)
		}
	}
	return legs, invitationLegs, nil
}

// actorRoleOn attributes the audit entry to the actor's real role on the
// property, like the single-property flows do. The «owner, else
// full_access» verdict mirrors the SQL manage-scope predicate of
// ListParticipantMembershipsForRemoval (db/queries/participants.sql): the
// legs only got here because that predicate passed — this attribution
// must evolve with it (the matrix test pins the predicate, not this
// mirror).
func (s *ParticipantMutationService) actorRoleOn(ctx context.Context, actor, propertyID uuid.UUID) (sharedpolicy.Role, error) {
	ownerID, err := s.owners.GetOwnerID(ctx, propertyID)
	if err != nil {
		return "", fmt.Errorf("resolve owner: %w", err)
	}
	if ownerID == actor {
		return sharedpolicy.RoleOwner, nil
	}
	return sharedpolicy.RoleFullAccess, nil
}

// removeMembershipLegs deletes the person's membership rows with an audit
// entry per row, reports how many active (slot-occupying) ones were freed and
// appends the removed active rows to the collector — the caller publishes
// their revocation events post-commit (карта #734, #751).
func (s *ParticipantMutationService) removeMembershipLegs(
	ctx context.Context, stores *txStores, actor uuid.UUID, legs []domain.Membership,
) (int, []domain.Membership, error) {
	freedActive := 0
	var revokedActive []domain.Membership
	for _, m := range legs {
		actorRole, err := s.actorRoleOn(ctx, actor, m.PropertyID)
		if err != nil {
			return 0, nil, err
		}
		if err := removeMembershipInTx(ctx, stores, actor, actorRole, m, &revokedActive); err != nil {
			return 0, nil, err
		}
		if !m.IsSuspended() {
			freedActive++
		}
	}
	return freedActive, revokedActive, nil
}

// removeInvitationLegs deletes the person's pending invitation rows with an
// audit entry per row.
func (s *ParticipantMutationService) removeInvitationLegs(
	ctx context.Context, stores *txStores, actor uuid.UUID, legs []domain.Invitation,
) error {
	for _, inv := range legs {
		if err := stores.invitations.Delete(ctx, inv.ID, inv.PropertyID); err != nil {
			return fmt.Errorf("delete invitation: %w", err)
		}
		actorRole, err := s.actorRoleOn(ctx, actor, inv.PropertyID)
		if err != nil {
			return err
		}
		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  sharedpolicy.AuditActorRole(actorRole),
			Action:     auditdomain.ActionPropertyMemberInvitationCancelled,
			EntityType: auditdomain.EntityPropertyMemberInvitation,
			EntityID:   &inv.ID,
			Context: map[string]any{
				auditKeyPropertyID: inv.PropertyID,
			},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
	}
	return nil
}

// resolveTarget resolves a participant identifier to the mutation target: a
// registered user's uuid, or an invitee email (which may already belong to a
// registered user). An unresolvable identifier is the privacy-preserving
// ErrParticipantNotFound; a resolution failure (user/email lookup) is a real
// error so the caller aborts instead of silently operating on partial data.
// The returned email is normalized: set for the pending and the registered
// targets — resolved through the email port for uuid targets; empty means the
// user has none, so the invitation legs are out of reach.
func (s *ParticipantMutationService) resolveTarget(ctx context.Context, participantID string) (uuid.UUID, string, error) {
	id := strings.TrimSpace(participantID)
	if userID, err := uuid.Parse(id); err == nil {
		if _, err := s.users.GetByID(ctx, userID); err != nil {
			if errors.Is(err, domain.ErrUserNotFound) {
				return uuid.UUID{}, "", domain.ErrParticipantNotFound
			}
			return uuid.UUID{}, "", fmt.Errorf("lookup user by id: %w", err)
		}
		email := ""
		if s.emails != nil {
			resolved, err := s.emails.GetEmail(ctx, userID)
			if err != nil {
				return uuid.UUID{}, "", fmt.Errorf("resolve participant email: %w", err)
			}
			email = strings.ToLower(resolved)
		}
		return userID, email, nil
	}
	email, err := domain.NormalizeEmail(id)
	if err != nil {
		return uuid.UUID{}, "", domain.ErrParticipantNotFound
	}
	user, err := s.users.GetByEmail(ctx, email)
	switch {
	case err == nil:
		return user.ID, email, nil
	case errors.Is(err, domain.ErrUserNotFound):
		return uuid.UUID{}, email, nil
	default:
		return uuid.UUID{}, "", fmt.Errorf("lookup user by email: %w", err)
	}
}

// resolveInviteTitles renders the display titles for the invite email: a
// missing or failed title degrades to no list entry rather than failing the
// send — the template falls back to the generic text when the list is
// empty (an empty-string entry would render broken «к объекту «»»).
func resolveInviteTitles(
	ctx context.Context, resolver PropertyTitleResolver, log *slog.Logger, propertyIDs []uuid.UUID,
) []string {
	titles := make([]string, 0, len(propertyIDs))
	for _, propertyID := range propertyIDs {
		if resolver == nil {
			continue
		}
		title, err := resolver.GetTitle(ctx, propertyID)
		if err != nil {
			log.WarnContext(ctx, "access: property title lookup for invite email failed",
				slog.String(auditKeyPropertyID, propertyID.String()),
				slog.String("error", err.Error()))
			continue
		}
		if title != "" {
			titles = append(titles, title)
		}
	}
	return titles
}

// sendInviteEmail renders and sends the single batch invite email listing the
// granted objects. A nil mailer disables the email; titles degrade per
// resolveInviteTitles.
func (s *ParticipantMutationService) sendInviteEmail(ctx context.Context, email string, propertyIDs []uuid.UUID, role domain.Role) {
	if s.mailer == nil {
		return
	}
	titles := resolveInviteTitles(ctx, s.titles, s.logger, propertyIDs)
	if err := s.mailer.SendInvite(ctx, email, titles, role); err != nil {
		// The recipient email is PII and never reaches the log (ADR 0020);
		// the granted-object count is the correlation handle.
		s.logger.ErrorContext(ctx, "access: invite email send failed",
			slog.Int("granted_properties", len(propertyIDs)),
			slog.String("error", err.Error()))
	}
}

// dedupePropertyIDs collapses duplicate request ids to the first occurrence,
// preserving the request order.
func dedupePropertyIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
