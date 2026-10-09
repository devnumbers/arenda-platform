package postgres

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// Static conformance assertions for the consumer-declared ports this adapter
// serves (issue #693): the participant read model and the hub summary's
// accessible-properties counter (CountActiveByUser on the membership repo).
var (
	_ application.ParticipantReadModel        = (*ParticipantRepository)(nil)
	_ application.AccessiblePropertiesCounter = (*MembershipRepository)(nil)
)

// ParticipantRepository reads the raw rows of the «Участник (владельца)»
// aggregate (issue #693) scoped to a reading actor: the non-archived
// properties the actor owns or manages as an active full_access member, plus
// every membership and pending invitation on them. The scope SQL is the
// authorization — rows outside it never leave the database.
type ParticipantRepository struct {
	db postgres.DBTX
}

// NewParticipantRepository creates a ParticipantRepository.
func NewParticipantRepository(db postgres.DBTX) *ParticipantRepository {
	return &ParticipantRepository{db: db}
}

func (r *ParticipantRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// ListScopeProperties returns the actor's participant-scope properties.
func (r *ParticipantRepository) ListScopeProperties(
	ctx context.Context, actorID uuid.UUID,
) ([]application.ParticipantScopeProperty, error) {
	rows, err := r.q().ListParticipantScopeProperties(ctx, pgconv.UUIDToPgtype(actorID))
	if err != nil {
		return nil, err
	}
	out := make([]application.ParticipantScopeProperty, 0, len(rows))
	for _, row := range rows {
		out = append(out, application.ParticipantScopeProperty{
			ID:        pgconv.UUIDFromPgtype(row.ID),
			OwnerID:   pgconv.UUIDFromPgtype(row.OwnerID),
			Title:     row.Title,
			Type:      row.Type,
			PhotoPath: row.PhotoPath,
		})
	}
	return out, nil
}

// ListMembershipsByProperties returns all memberships (any status) on the
// given properties; an empty list returns no rows (the CSV cast would reject
// an empty string).
func (r *ParticipantRepository) ListMembershipsByProperties(
	ctx context.Context, propertyIDs []uuid.UUID,
) ([]application.ParticipantMembershipRow, error) {
	if len(propertyIDs) == 0 {
		return nil, nil
	}
	rows, err := r.q().ListParticipantMembershipsByProperties(ctx, joinUUIDs(propertyIDs))
	if err != nil {
		return nil, err
	}
	out := make([]application.ParticipantMembershipRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, application.ParticipantMembershipRow{
			PropertyID: pgconv.UUIDFromPgtype(row.PropertyID),
			UserID:     pgconv.UUIDFromPgtype(row.UserID),
			Role:       domainRole(row.Role),
			Status:     domainMemberStatus(row.Status),
		})
	}
	return out, nil
}

// ListInvitationsByProperties returns all pending invitations on the given
// properties; an empty list returns no rows.
func (r *ParticipantRepository) ListInvitationsByProperties(
	ctx context.Context, propertyIDs []uuid.UUID,
) ([]application.ParticipantInvitationRow, error) {
	if len(propertyIDs) == 0 {
		return nil, nil
	}
	rows, err := r.q().ListParticipantInvitationsByProperties(ctx, joinUUIDs(propertyIDs))
	if err != nil {
		return nil, err
	}
	out := make([]application.ParticipantInvitationRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, application.ParticipantInvitationRow{
			PropertyID: pgconv.UUIDFromPgtype(row.PropertyID),
			Email:      row.Email,
			Role:       domainRole(row.Role),
		})
	}
	return out, nil
}

// joinUUIDs renders the property ids as the CSV string the sqlc queries cast
// through string_to_array → uuid[].
func joinUUIDs(ids []uuid.UUID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = id.String()
	}
	return strings.Join(parts, ",")
}
