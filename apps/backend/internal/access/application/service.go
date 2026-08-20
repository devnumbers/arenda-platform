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

// Member is the application-level projection of a property participant used by
// ListMembers. The owner is synthesized from properties.owner_id and never
// stored as a membership row, so it carries IsOwner = true and no membership id.
// A pending email invitation (issue #161, T5) is projected as a Member with
// Pending = true, the invitation id in ID, a zero UserID, and Email/LastSentAt
// filled.
type Member struct {
	ID          uuid.UUID // Membership id; invitation id when Pending (zero uuid for the owner).
	UserID      uuid.UUID
	Role        sharedpolicy.Role // Owner | full_access | viewer.
	IsOwner     bool
	DisplayName string
	HasEmail    bool
	Status      domain.MemberStatus // Active | suspended (always active for the owner; meaningless when Pending).
	SuspendedAt *time.Time          // When the membership was suspended; nil when active.
	// Pending marks a pending email invitation row: the invitee is not
	// registered yet, so UserID is zero and Email carries the invitee address
	// (visible to managers only).
	Pending    bool
	Email      *string    // Invitee email; set only when Pending.
	LastSentAt *time.Time // When the invite email was last sent; set only when Pending.
}

// AccessService implements the property membership use cases (issue #156, T3):
// adding, listing, changing roles, revoking and self-exit. Authorization goes
// through the policy port; persistence and audit share the same transaction
// through the embedded txStoreFactory (ADR 0033).
type AccessService struct {
	txStoreFactory
	members   MembershipRepository
	owners    PropertyOwnerResolver
	statuses  PropertyStatusResolver
	users     UserLookup
	policy    sharedpolicy.Policy
	slots     *SlotCoordinator
	lifecycle *LifecycleMailer
	logger    *slog.Logger
}

// NewAccessService creates an AccessService. Slots is the recipient tariff slot
// coordinator (issue #158, T4); it may be nil to disable slot enforcement
// (pre-T4 behaviour, e.g. in tests that don't exercise the limit). Lifecycle is
// the sharing lifecycle mailer (issue #162, T6); it may be nil to disable the
// lifecycle emails. Statuses reports the archived flag of a property (issue
// #163); it may be nil to skip the archived-property checks. Factory bundles
// the membership repository, the audit recorder, and the Unit-of-Work every
// mutating use case runs through (ADR 0033 γ-factory).
func NewAccessService(
	members MembershipRepository,
	owners PropertyOwnerResolver,
	statuses PropertyStatusResolver,
	users UserLookup,
	policy sharedpolicy.Policy,
	slots *SlotCoordinator,
	lifecycle *LifecycleMailer,
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
		lifecycle:      lifecycle,
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
	suspend := false
	err = s.runInTx(ctx, func(stores *txStores) error {
		var err error
		created, suspend, err = s.createMembershipInTx(ctx, stores, actor, membership, actorRole)
		return err
	})
	if err != nil {
		return domain.Membership{}, err
	}

	// A grant created without a free tariff slot sends the "access waits for a
	// free slot" email post-commit (issue #162, T6); a send failure is logged
	// inside the mailer and never fails the add.
	if suspend {
		s.lifecycle.SendAccessSuspended(ctx, userID, propertyID)
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
		ActorRole:  actorRoleFromPolicyRole(actorRole),
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
	err = s.runInTx(ctx, func(stores *txStores) error {
		// Confirm the membership exists and belongs to this property before
		// updating; a missing row is a not-found outcome rather than a silent no-op.
		if _, err := stores.members.GetByID(ctx, memberID, propertyID); err != nil {
			return err
		}

		var err error
		updated, err = stores.members.UpdateRole(ctx, memberID, propertyID, role)
		if err != nil {
			return fmt.Errorf("update membership role: %w", err)
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  actorRoleFromPolicyRole(actorRole),
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
		return nil
	})
	if err != nil {
		return domain.Membership{}, err
	}
	return updated, nil
}

// RevokeMember removes a member's access. The owner cannot be revoked (and is
// never a membership row); this guard exists for defence in depth.
func (s *AccessService) RevokeMember(ctx context.Context, actor, propertyID, memberID uuid.UUID) error {
	owner, actorRole, err := s.requireManage(ctx, actor, propertyID)
	if err != nil {
		return err
	}

	var membership domain.Membership
	err = s.runInTx(ctx, func(stores *txStores) error {
		var err error
		membership, err = stores.members.GetByID(ctx, memberID, propertyID)
		if err != nil {
			return err
		}
		if membership.UserID == owner {
			return domain.ErrCannotRevokeOwner
		}

		if err := stores.members.Delete(ctx, memberID, propertyID); err != nil {
			return fmt.Errorf("delete membership: %w", err)
		}

		// Revoking the recipient freed one of their tariff slots: try to recover the
		// oldest suspended membership FIFO (issue #158, T4).
		if s.slots != nil {
			if err := s.slots.RecoverSuspended(ctx, stores.tx, membership.UserID); err != nil {
				return fmt.Errorf("recover suspended after revoke: %w", err)
			}
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  actorRoleFromPolicyRole(actorRole),
			Action:     auditdomain.ActionPropertyMemberRemoved,
			EntityType: auditdomain.EntityPropertyMember,
			EntityID:   &membership.ID,
			Context: map[string]any{
				auditKeyPropertyID: propertyID,
				auditKeyUserID:     membership.UserID,
			},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Revoking an active membership notifies the former member post-commit
	// (issue #162, T6); revoking a suspended one is silent (the object was
	// already hidden from them).
	if !membership.IsSuspended() {
		s.lifecycle.SendAccessRevoked(ctx, membership.UserID, propertyID)
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
	if role == sharedpolicy.RoleNone || role == sharedpolicy.RoleSuspended {
		// A suspended membership stays indistinguishable from no access here:
		// the object is hidden from the recipient, so self-exit keeps the
		// privacy-preserving not-found (T9 lifts the suspension signal only at
		// the property page entry point).
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

		// A suspended membership is already hidden from the recipient (no slot, no
		// access), so self-exit is not available: the object is invisible to them
		// (issue #158, T4 AC).
		if membership.IsSuspended() {
			return domain.ErrCannotLeaveSuspended
		}

		if err := stores.members.Delete(ctx, membership.ID, propertyID); err != nil {
			return fmt.Errorf("delete membership: %w", err)
		}

		// Self-exit freed one of the actor's tariff slots: try to recover the oldest
		// suspended membership FIFO (issue #158, T4).
		if s.slots != nil {
			if err := s.slots.RecoverSuspended(ctx, stores.tx, actor); err != nil {
				return fmt.Errorf("recover suspended after leave: %w", err)
			}
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  actorRoleFromPolicyRole(role),
			Action:     auditdomain.ActionPropertyMemberLeft,
			EntityType: auditdomain.EntityPropertyMember,
			EntityID:   &membership.ID,
			Context: map[string]any{
				auditKeyPropertyID: propertyID,
			},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Self-exit notifies the owner post-commit (issue #162, T6); the leaving
	// member receives nothing.
	owner, err := s.owners.GetOwnerID(ctx, propertyID)
	if err != nil {
		s.logger.WarnContext(ctx, "access: owner lookup for member left email failed",
			slog.String(auditKeyPropertyID, propertyID.String()),
			slog.String("error", err.Error()))
		return nil
	}
	memberName := ""
	if u, err := s.users.GetByID(ctx, actor); err != nil {
		s.logger.WarnContext(ctx, "access: member lookup for member left email failed",
			slog.String(auditKeyUserID, actor.String()),
			slog.String("error", err.Error()))
	} else {
		memberName = displayName(u)
	}
	s.lifecycle.SendMemberLeft(ctx, owner, propertyID, memberName)
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
		HasEmail:    ownerUser.HasEmail,
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
			HasEmail:    u.HasEmail,
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

// actorRoleFromPolicyRole maps a policy role to the audit actor role so
// actions of shared-access members are attributed to their real role instead
// of being masked as the owner's own (issue #166 follow-up). Roles that never
// reach a Record call through the gates (suspended, none) and any unknown role
// fall back to the historical owner attribution.
func actorRoleFromPolicyRole(role sharedpolicy.Role) auditdomain.ActorRole {
	switch role {
	case sharedpolicy.RoleFullAccess:
		return auditdomain.ActorRoleFullAccess
	case sharedpolicy.RoleViewer:
		return auditdomain.ActorRoleViewer
	default:
		return auditdomain.ActorRoleOwner
	}
}

// DisplayName resolves the public display name of a user (issue T11): "Name
// Surname" when present, otherwise a masked phone — never an email or a raw
// phone. Used cross-context (the properties sharing banner) via the
// OwnerDisplayNameResolver port.
func (s *AccessService) DisplayName(ctx context.Context, userID uuid.UUID) (string, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("lookup user: %w", err)
	}
	return displayName(user), nil
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

// maskPhone masks all but the country code and last two digits of a phone, so a
// display name never reveals the full phone or any email (PII).
func maskPhone(phone string) string {
	if len(phone) <= 4 {
		return phone
	}
	return phone[:2] + strings.Repeat("*", len(phone)-4) + phone[len(phone)-2:]
}
