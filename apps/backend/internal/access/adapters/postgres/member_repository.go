// Package postgres holds the postgres persistence adapters for the access
// bounded context (issue #156, T3).
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// MembershipRepository persists property membership records.
type MembershipRepository struct {
	db postgres.DBTX
}

// NewMembershipRepository creates a MembershipRepository.
func NewMembershipRepository(db postgres.DBTX) *MembershipRepository {
	return &MembershipRepository{db: db}
}

func (r *MembershipRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *MembershipRepository) WithTx(tx transaction.Tx) application.MembershipRepository {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		panic(fmt.Sprintf("access.MembershipRepository.WithTx: expected postgres.DBTX, got %T", tx))
	}
	return NewMembershipRepository(dbtx)
}

// Create inserts a membership record and returns the created membership. The
// row is created with the default 'active' status.
func (r *MembershipRepository) Create(ctx context.Context, m domain.Membership) (domain.Membership, error) {
	row, err := r.q().CreatePropertyMember(ctx, postgres.CreatePropertyMemberParams{
		ID:         pgconv.UUIDToPgtype(m.ID),
		PropertyID: pgconv.UUIDToPgtype(m.PropertyID),
		UserID:     pgconv.UUIDToPgtype(m.UserID),
		Role:       m.Role.String(),
		GrantedBy:  pgconv.UUIDToPgtype(m.GrantedBy),
	})
	if err != nil {
		return domain.Membership{}, err
	}
	return membershipFromRow(row), nil
}

// CreateWithStatus inserts a membership record with an explicit status, used to
// create a suspended grant directly. A zero-value status defaults to active.
func (r *MembershipRepository) CreateWithStatus(ctx context.Context, m domain.Membership) (domain.Membership, error) {
	status := m.Status
	if status == "" {
		status = domain.MemberStatusActive
	}
	row, err := r.q().CreatePropertyMemberWithStatus(ctx, postgres.CreatePropertyMemberWithStatusParams{
		ID:         pgconv.UUIDToPgtype(m.ID),
		PropertyID: pgconv.UUIDToPgtype(m.PropertyID),
		UserID:     pgconv.UUIDToPgtype(m.UserID),
		Role:       m.Role.String(),
		GrantedBy:  pgconv.UUIDToPgtype(m.GrantedBy),
		Status:     status.String(),
	})
	if err != nil {
		return domain.Membership{}, err
	}
	return membershipFromRow(row), nil
}

// GetByID returns a membership by its id within a property.
func (r *MembershipRepository) GetByID(ctx context.Context, id, propertyID uuid.UUID) (domain.Membership, error) {
	row, err := r.q().GetPropertyMember(ctx, postgres.GetPropertyMemberParams{
		ID:         pgconv.UUIDToPgtype(id),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Membership{}, domain.ErrMemberNotFound
		}
		return domain.Membership{}, err
	}
	return membershipFromRow(row), nil
}

// GetByPropertyAndUser returns the membership of a user on a property.
func (r *MembershipRepository) GetByPropertyAndUser(ctx context.Context, propertyID, userID uuid.UUID) (domain.Membership, error) {
	row, err := r.q().GetPropertyMemberByPropertyAndUser(ctx, postgres.GetPropertyMemberByPropertyAndUserParams{
		PropertyID: pgconv.UUIDToPgtype(propertyID),
		UserID:     pgconv.UUIDToPgtype(userID),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Membership{}, domain.ErrMemberNotFound
		}
		return domain.Membership{}, err
	}
	return membershipFromRow(row), nil
}

// GetRole returns only the role of a user on a property. Used by the policy to
// avoid loading the full row.
func (r *MembershipRepository) GetRole(ctx context.Context, propertyID, userID uuid.UUID) (domain.Role, error) {
	role, err := r.q().GetPropertyMemberRole(ctx, postgres.GetPropertyMemberRoleParams{
		PropertyID: pgconv.UUIDToPgtype(propertyID),
		UserID:     pgconv.UUIDToPgtype(userID),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", domain.ErrMemberNotFound
		}
		return "", err
	}
	return domain.ParseRole(role)
}

// ListByProperty returns all memberships of a property ordered by created_at.
func (r *MembershipRepository) ListByProperty(ctx context.Context, propertyID uuid.UUID) ([]domain.Membership, error) {
	rows, err := r.q().ListPropertyMembers(ctx, pgconv.UUIDToPgtype(propertyID))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Membership, 0, len(rows))
	for _, row := range rows {
		out = append(out, membershipFromRow(row))
	}
	return out, nil
}

// ListByUser returns all memberships held by a user across properties.
func (r *MembershipRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Membership, error) {
	rows, err := r.q().ListPropertyMembersByUser(ctx, pgconv.UUIDToPgtype(userID))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Membership, 0, len(rows))
	for _, row := range rows {
		role, err := domain.ParseRole(row.Role)
		if err != nil {
			return nil, fmt.Errorf("parse membership role: %w", err)
		}
		out = append(out, domain.Membership{
			PropertyID: pgconv.UUIDFromPgtype(row.PropertyID),
			UserID:     userID,
			Role:       role,
		})
	}
	return out, nil
}

// MaxRoleByOwner returns the strongest role the user holds across all of the
// owner's properties. The SQL query encodes roles as integers (full_access=2,
// viewer=1) and aggregates them with MAX; COALESCE collapses the no-rows case
// (MAX of an empty set is NULL) into the sentinel -1, which maps to the empty
// role (RoleNone). A stored role outside the known set also maps to the empty
// role.
func (r *MembershipRepository) MaxRoleByOwner(ctx context.Context, userID, ownerID uuid.UUID) (domain.Role, error) {
	maxRole, err := r.q().GetMaxMemberRoleByOwner(ctx, postgres.GetMaxMemberRoleByOwnerParams{
		UserID:  pgconv.UUIDToPgtype(userID),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
	})
	if err != nil {
		return domain.Role(""), err
	}
	switch maxRole {
	case 2:
		return domain.RoleFullAccess, nil
	case 1:
		return domain.RoleViewer, nil
	default:
		return domain.Role(""), nil
	}
}

// UpdateRole changes the role of a membership.
func (r *MembershipRepository) UpdateRole(ctx context.Context, id, propertyID uuid.UUID, role domain.Role) (domain.Membership, error) {
	row, err := r.q().UpdatePropertyMemberRole(ctx, postgres.UpdatePropertyMemberRoleParams{
		ID:         pgconv.UUIDToPgtype(id),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
		Role:       role.String(),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Membership{}, domain.ErrMemberNotFound
		}
		return domain.Membership{}, err
	}
	return membershipFromRow(row), nil
}

// Delete removes a membership. Existence is established by the service in the
// same transaction, so a zero-rows result here is not treated as NotFound.
func (r *MembershipRepository) Delete(ctx context.Context, id, propertyID uuid.UUID) error {
	return r.q().DeletePropertyMember(ctx, postgres.DeletePropertyMemberParams{
		ID:         pgconv.UUIDToPgtype(id),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
}

// Suspend marks a membership as suspended. Existence is established by the
// service in the same transaction, so a zero-rows result is not treated as
// NotFound.
func (r *MembershipRepository) Suspend(ctx context.Context, id, propertyID uuid.UUID) error {
	return r.q().SuspendPropertyMember(ctx, postgres.SuspendPropertyMemberParams{
		ID:         pgconv.UUIDToPgtype(id),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
}

// Reactivate marks a suspended membership as active again.
func (r *MembershipRepository) Reactivate(ctx context.Context, id, propertyID uuid.UUID) (domain.Membership, error) {
	row, err := r.q().ReactivatePropertyMember(ctx, postgres.ReactivatePropertyMemberParams{
		ID:         pgconv.UUIDToPgtype(id),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Membership{}, domain.ErrMemberNotFound
		}
		return domain.Membership{}, err
	}
	return membershipFromRow(row), nil
}

// ListSuspendedByUser returns the user's suspended memberships ordered for FIFO
// recovery (oldest suspended_at first).
func (r *MembershipRepository) ListSuspendedByUser(ctx context.Context, userID uuid.UUID) ([]domain.Membership, error) {
	rows, err := r.q().ListSuspendedMembersByUser(ctx, pgconv.UUIDToPgtype(userID))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Membership, 0, len(rows))
	for _, row := range rows {
		out = append(out, membershipFromRow(row))
	}
	return out, nil
}

// CountActiveByUser returns the number of active memberships held by the user
// (occupied tariff slots).
func (r *MembershipRepository) CountActiveByUser(ctx context.Context, userID uuid.UUID) (int, error) {
	count, err := r.q().CountActiveMembersByUser(ctx, pgconv.UUIDToPgtype(userID))
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// ListActiveByPropertyOwner returns the active memberships across all of the
// owner's properties.
func (r *MembershipRepository) ListActiveByPropertyOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.Membership, error) {
	rows, err := r.q().ListActiveMembersByPropertyOwner(ctx, pgconv.UUIDToPgtype(ownerID))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Membership, 0, len(rows))
	for _, row := range rows {
		out = append(out, membershipFromRow(row))
	}
	return out, nil
}

// ListActiveByUser returns the user's active memberships with full rows. Unlike
// ListByUser (a property-id+role projection), it carries the membership id and
// updated_at the slot coordinator needs to build the recipient's shared pool
// and suspend/reactivate entries. See issue #158 (T4).
func (r *MembershipRepository) ListActiveByUser(ctx context.Context, userID uuid.UUID) ([]domain.Membership, error) {
	rows, err := r.q().ListActiveMembersByUser(ctx, pgconv.UUIDToPgtype(userID))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Membership, 0, len(rows))
	for _, row := range rows {
		out = append(out, membershipFromRow(row))
	}
	return out, nil
}

// ListForRemovalByUser returns the person's memberships (any status, with
// ids) on the properties the actor manages, archived included — the removal
// scope of «Отозвать и удалить» (issue #694). The SQL scope is the
// authorization (issue #693). The role is not part of the removal projection:
// revoking does not read it.
func (r *MembershipRepository) ListForRemovalByUser(ctx context.Context, personID, actorID uuid.UUID) ([]domain.Membership, error) {
	rows, err := r.q().ListParticipantMembershipsForRemoval(ctx, postgres.ListParticipantMembershipsForRemovalParams{
		PersonID: pgconv.UUIDToPgtype(personID),
		ActorID:  pgconv.UUIDToPgtype(actorID),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Membership, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Membership{
			ID:         pgconv.UUIDFromPgtype(row.ID),
			PropertyID: pgconv.UUIDFromPgtype(row.PropertyID),
			UserID:     pgconv.UUIDFromPgtype(row.UserID),
			Status:     domainMemberStatus(row.Status),
		})
	}
	return out, nil
}

func membershipFromRow(row postgres.PropertyMember) domain.Membership {
	return domain.Membership{
		ID:          pgconv.UUIDFromPgtype(row.ID),
		PropertyID:  pgconv.UUIDFromPgtype(row.PropertyID),
		UserID:      pgconv.UUIDFromPgtype(row.UserID),
		Role:        domainRole(row.Role),
		GrantedBy:   pgconv.UUIDFromPgtype(row.GrantedBy),
		Status:      domainMemberStatus(row.Status),
		SuspendedAt: pgconv.TimestamptzToPtrTime(row.SuspendedAt),
		CreatedAt:   pgconv.TimestamptzToTime(row.CreatedAt),
		UpdatedAt:   pgconv.TimestamptzToTime(row.UpdatedAt),
	}
}

func domainRole(s string) domain.Role {
	// Stored roles are validated by the CHECK constraint; a parse failure here
	// would indicate schema drift, so fall back to viewer rather than panic.
	role, err := domain.ParseRole(s)
	if err != nil {
		return domain.RoleViewer
	}
	return role
}

func domainMemberStatus(s string) domain.MemberStatus {
	// Stored statuses are validated by the CHECK constraint; a parse failure
	// here would indicate schema drift, so fall back to active rather than
	// panic.
	status, err := domain.ParseMemberStatus(s)
	if err != nil {
		return domain.MemberStatusActive
	}
	return status
}
