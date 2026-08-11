package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// InvitationRepository persists pending property member invitations
// (issue #161, T5).
type InvitationRepository struct {
	db postgres.DBTX
}

// NewInvitationRepository creates an InvitationRepository.
func NewInvitationRepository(db postgres.DBTX) *InvitationRepository {
	return &InvitationRepository{db: db}
}

func (r *InvitationRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *InvitationRepository) WithTx(tx transaction.Tx) application.InvitationRepository {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		panic(fmt.Sprintf("access.InvitationRepository.WithTx: expected postgres.DBTX, got %T", tx))
	}
	return NewInvitationRepository(dbtx)
}

// Create inserts a pending invitation and returns it.
func (r *InvitationRepository) Create(ctx context.Context, inv domain.Invitation) (domain.Invitation, error) {
	row, err := r.q().CreatePropertyMemberInvitation(ctx, postgres.CreatePropertyMemberInvitationParams{
		ID:         pgconv.UUIDToPgtype(inv.ID),
		PropertyID: pgconv.UUIDToPgtype(inv.PropertyID),
		Email:      inv.Email,
		Role:       inv.Role.String(),
		InvitedBy:  pgconv.UUIDToPgtype(inv.InvitedBy),
		LastSentAt: pgtype.Timestamptz{Time: inv.LastSentAt.UTC(), Valid: true},
	})
	if err != nil {
		return domain.Invitation{}, err
	}
	return invitationFromRow(row), nil
}

// GetByID returns an invitation by its id within a property.
func (r *InvitationRepository) GetByID(ctx context.Context, id, propertyID uuid.UUID) (domain.Invitation, error) {
	row, err := r.q().GetPropertyMemberInvitation(ctx, postgres.GetPropertyMemberInvitationParams{
		ID:         pgconv.UUIDToPgtype(id),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Invitation{}, domain.ErrInvitationNotFound
		}
		return domain.Invitation{}, err
	}
	return invitationFromRow(row), nil
}

// GetByPropertyAndEmail returns the pending invitation for the email on the
// property. Matching is case-insensitive (the service passes a normalized
// lowercase email; the unique index is on lower(email)).
func (r *InvitationRepository) GetByPropertyAndEmail(ctx context.Context, propertyID uuid.UUID, email string) (domain.Invitation, error) {
	row, err := r.q().GetPropertyMemberInvitationByEmail(ctx, postgres.GetPropertyMemberInvitationByEmailParams{
		PropertyID: pgconv.UUIDToPgtype(propertyID),
		Email:      email,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Invitation{}, domain.ErrInvitationNotFound
		}
		return domain.Invitation{}, err
	}
	return invitationFromRow(row), nil
}

// ListByProperty returns all pending invitations of a property ordered by
// created_at.
func (r *InvitationRepository) ListByProperty(ctx context.Context, propertyID uuid.UUID) ([]domain.Invitation, error) {
	rows, err := r.q().ListPropertyMemberInvitations(ctx, pgconv.UUIDToPgtype(propertyID))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Invitation, 0, len(rows))
	for _, row := range rows {
		out = append(out, invitationFromRow(row))
	}
	return out, nil
}

// ListPendingByEmail returns all pending invitations for the email across
// properties, oldest first (FIFO activation at registration).
func (r *InvitationRepository) ListPendingByEmail(ctx context.Context, email string) ([]domain.Invitation, error) {
	rows, err := r.q().ListPendingInvitationsByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Invitation, 0, len(rows))
	for _, row := range rows {
		out = append(out, invitationFromRow(row))
	}
	return out, nil
}

// UpdateRole changes the role of a pending invitation.
func (r *InvitationRepository) UpdateRole(ctx context.Context, id, propertyID uuid.UUID, role domain.Role) (domain.Invitation, error) {
	row, err := r.q().UpdatePropertyMemberInvitationRole(ctx, postgres.UpdatePropertyMemberInvitationRoleParams{
		ID:         pgconv.UUIDToPgtype(id),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
		Role:       role.String(),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Invitation{}, domain.ErrInvitationNotFound
		}
		return domain.Invitation{}, err
	}
	return invitationFromRow(row), nil
}

// UpdateLastSentAt bumps the invite email send timestamp (the resend cooldown
// anchor). Existence is established by the service in the same transaction, so
// a zero-rows result is not treated as NotFound.
func (r *InvitationRepository) UpdateLastSentAt(ctx context.Context, id, propertyID uuid.UUID, sentAt time.Time) error {
	return r.q().UpdatePropertyMemberInvitationLastSentAt(ctx, postgres.UpdatePropertyMemberInvitationLastSentAtParams{
		ID:         pgconv.UUIDToPgtype(id),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
		LastSentAt: pgtype.Timestamptz{Time: sentAt.UTC(), Valid: true},
	})
}

// Delete removes a pending invitation (cancellation or activation). Existence
// is established by the service in the same transaction, so a zero-rows result
// is not treated as NotFound.
func (r *InvitationRepository) Delete(ctx context.Context, id, propertyID uuid.UUID) error {
	return r.q().DeletePropertyMemberInvitation(ctx, postgres.DeletePropertyMemberInvitationParams{
		ID:         pgconv.UUIDToPgtype(id),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
}

func invitationFromRow(row postgres.PropertyMemberInvitation) domain.Invitation {
	return domain.Invitation{
		ID:         pgconv.UUIDFromPgtype(row.ID),
		PropertyID: pgconv.UUIDFromPgtype(row.PropertyID),
		Email:      row.Email,
		Role:       domainRole(row.Role),
		InvitedBy:  pgconv.UUIDFromPgtype(row.InvitedBy),
		LastSentAt: pgconv.TimestamptzToTime(row.LastSentAt),
		CreatedAt:  pgconv.TimestamptzToTime(row.CreatedAt),
		UpdatedAt:  pgconv.TimestamptzToTime(row.UpdatedAt),
	}
}
