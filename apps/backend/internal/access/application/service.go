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
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// Member is the application-level projection of a property participant used by
// ListMembers. The owner is synthesized from properties.owner_id and never
// stored as a membership row, so it carries IsOwner = true and no membership id.
type Member struct {
	ID          uuid.UUID // membership id (zero uuid for the owner)
	UserID      uuid.UUID
	Role        sharedpolicy.Role // owner | full_access | viewer
	IsOwner     bool
	DisplayName string
	HasEmail    bool
	Status      domain.MemberStatus // active | suspended (always active for the owner)
	SuspendedAt *time.Time          // when the membership was suspended; nil when active
}

// AccessService implements the property membership use cases (issue #156, T3):
// adding, listing, changing roles, revoking and self-exit. Authorization goes
// through the policy port; persistence and audit share the same transaction.
type AccessService struct {
	members MembershipRepository
	owners  PropertyOwnerResolver
	users   UserLookup
	policy  sharedpolicy.Policy
	slots   *SlotCoordinator
	db      txBeginner
	audit   auditapp.Recorder
	logger  *slog.Logger
}

// NewAccessService creates an AccessService. slots is the recipient tariff slot
// coordinator (issue #158, T4); it may be nil to disable slot enforcement
// (pre-T4 behaviour, e.g. in tests that don't exercise the limit).
func NewAccessService(
	members MembershipRepository,
	owners PropertyOwnerResolver,
	users UserLookup,
	policy sharedpolicy.Policy,
	slots *SlotCoordinator,
	db txBeginner,
	audit auditapp.Recorder,
	logger *slog.Logger,
) *AccessService {
	if logger == nil {
		logger = slog.Default()
	}
	if audit == nil {
		audit = auditapp.Noop{}
	}
	return &AccessService{
		members: members,
		owners:  owners,
		users:   users,
		policy:  policy,
		slots:   slots,
		db:      db,
		audit:   audit,
		logger:  logger,
	}
}

// AddMember grants a registered user a role on a property. Any participant with
// the manage-members capability (owner or full) may add anyone, including
// another full member — full members are equal in member management.
func (s *AccessService) AddMember(ctx context.Context, actor, propertyID, userID uuid.UUID, role domain.Role) (domain.Membership, error) {
	owner, err := s.requireManage(ctx, actor, propertyID)
	if err != nil {
		return domain.Membership{}, err
	}
	if userID == owner {
		return domain.Membership{}, domain.ErrCannotAddOwner
	}
	if userID == actor {
		// A user cannot grant themselves access via the member API; ownership
		// is the only self-granted access.
		return domain.Membership{}, domain.ErrCannotAddSelf
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

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Membership{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txMembers := s.members.WithTx(tx)
	if _, err := txMembers.GetByPropertyAndUser(ctx, propertyID, userID); err == nil {
		return domain.Membership{}, domain.ErrMemberAlreadyExists
	} else if !errors.Is(err, domain.ErrMemberNotFound) {
		return domain.Membership{}, fmt.Errorf("check existing membership: %w", err)
	}

	// Enforce the recipient tariff slot invariant (issue #158, T4): when the
	// recipient has no free slot, the membership is created suspended so it does
	// not occupy a slot until one frees up and is recovered FIFO.
	suspend := false
	if s.slots != nil {
		var err2 error
		suspend, err2 = s.slots.EnforceOnActivation(ctx, tx, userID)
		if err2 != nil {
			return domain.Membership{}, fmt.Errorf("check recipient slot: %w", err2)
		}
	}
	if suspend {
		membership.Status = domain.MemberStatusSuspended
	}

	var created domain.Membership
	if suspend {
		created, err = txMembers.CreateWithStatus(ctx, membership)
	} else {
		created, err = txMembers.Create(ctx, membership)
	}
	if err != nil {
		return domain.Membership{}, fmt.Errorf("create membership: %w", err)
	}

	// Audit in the same transaction. Only ids and the role are recorded; the
	// member's email/phone are PII and must never appear in context (ADR 0020).
	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionPropertyMemberAdded,
		EntityType: auditdomain.EntityPropertyMember,
		EntityID:   &created.ID,
		Context: map[string]any{
			"property_id": propertyID,
			"user_id":     userID,
			"role":        string(role),
			"status":      string(created.Status),
		},
	}); err != nil {
		return domain.Membership{}, fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Membership{}, fmt.Errorf("commit tx: %w", err)
	}
	return created, nil
}

// ChangeMemberRole changes the role of an existing member. The owner is never a
// membership row, so it cannot be targeted here; the membership id must belong
// to the given property.
func (s *AccessService) ChangeMemberRole(ctx context.Context, actor, propertyID, memberID uuid.UUID, role domain.Role) (domain.Membership, error) {
	if _, err := s.requireManage(ctx, actor, propertyID); err != nil {
		return domain.Membership{}, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Membership{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txMembers := s.members.WithTx(tx)
	// Confirm the membership exists and belongs to this property before
	// updating; a missing row is a not-found outcome rather than a silent no-op.
	if _, err := txMembers.GetByID(ctx, memberID, propertyID); err != nil {
		return domain.Membership{}, err
	}

	updated, err := txMembers.UpdateRole(ctx, memberID, propertyID, role)
	if err != nil {
		return domain.Membership{}, fmt.Errorf("update membership role: %w", err)
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionPropertyMemberUpdated,
		EntityType: auditdomain.EntityPropertyMember,
		EntityID:   &updated.ID,
		Context: map[string]any{
			"property_id": propertyID,
			"user_id":     updated.UserID,
			"role":        string(role),
		},
	}); err != nil {
		return domain.Membership{}, fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Membership{}, fmt.Errorf("commit tx: %w", err)
	}
	return updated, nil
}

// RevokeMember removes a member's access. The owner cannot be revoked (and is
// never a membership row); this guard exists for defence in depth.
func (s *AccessService) RevokeMember(ctx context.Context, actor, propertyID, memberID uuid.UUID) error {
	owner, err := s.requireManage(ctx, actor, propertyID)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txMembers := s.members.WithTx(tx)
	membership, err := txMembers.GetByID(ctx, memberID, propertyID)
	if err != nil {
		return err
	}
	if membership.UserID == owner {
		return domain.ErrCannotRevokeOwner
	}

	if err := txMembers.Delete(ctx, memberID, propertyID); err != nil {
		return fmt.Errorf("delete membership: %w", err)
	}

	// Revoking the recipient freed one of their tariff slots: try to recover the
	// oldest suspended membership FIFO (issue #158, T4).
	if s.slots != nil {
		if err := s.slots.RecoverSuspended(ctx, tx, membership.UserID); err != nil {
			return fmt.Errorf("recover suspended after revoke: %w", err)
		}
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionPropertyMemberRemoved,
		EntityType: auditdomain.EntityPropertyMember,
		EntityID:   &membership.ID,
		Context: map[string]any{
			"property_id": propertyID,
			"user_id":     membership.UserID,
		},
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
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

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txMembers := s.members.WithTx(tx)
	membership, err := txMembers.GetByPropertyAndUser(ctx, propertyID, actor)
	if err != nil {
		return err
	}

	// A suspended membership is already hidden from the recipient (no slot, no
	// access), so self-exit is not available: the object is invisible to them
	// (issue #158, T4 AC).
	if membership.IsSuspended() {
		return domain.ErrCannotLeaveSuspended
	}

	if err := txMembers.Delete(ctx, membership.ID, propertyID); err != nil {
		return fmt.Errorf("delete membership: %w", err)
	}

	// Self-exit freed one of the actor's tariff slots: try to recover the oldest
	// suspended membership FIFO (issue #158, T4).
	if s.slots != nil {
		if err := s.slots.RecoverSuspended(ctx, tx, actor); err != nil {
			return fmt.Errorf("recover suspended after leave: %w", err)
		}
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionPropertyMemberLeft,
		EntityType: auditdomain.EntityPropertyMember,
		EntityID:   &membership.ID,
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
				slog.String("user_id", m.UserID.String()),
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
// manage-members capability, and returns the property owner id (scope). A
// missing property or lack of access is mapped to ErrMemberNotFound to keep
// object existence private.
func (s *AccessService) requireManage(ctx context.Context, actor, propertyID uuid.UUID) (uuid.UUID, error) {
	role, err := s.policy.RoleForProperty(ctx, actor, propertyID)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("resolve role: %w", err)
	}
	if !sharedpolicy.CanManageMembers(role) {
		return uuid.UUID{}, domain.ErrMemberNotFound
	}
	owner, err := s.owners.GetOwnerID(ctx, propertyID)
	if err != nil {
		return uuid.UUID{}, domain.ErrMemberNotFound
	}
	return owner, nil
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
