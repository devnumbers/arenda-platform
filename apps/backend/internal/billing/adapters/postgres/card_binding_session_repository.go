package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/pgerr"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// CardBindingSessionRepository persists card-binding sessions (issue #251).
type CardBindingSessionRepository struct {
	db postgres.DBTX
}

// NewCardBindingSessionRepository creates a new card-binding-session
// repository.
func NewCardBindingSessionRepository(db postgres.DBTX) *CardBindingSessionRepository {
	return &CardBindingSessionRepository{db: db}
}

func (r *CardBindingSessionRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *CardBindingSessionRepository) WithTx(tx transaction.Tx) (application.CardBindingSessionRepository, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("billing.CardBindingSessionRepository.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewCardBindingSessionRepository(dbtx), nil
}

// Create inserts a new binding session. The created_at column is written
// from the domain session (the service clock), not the database default, so
// the binding limit's sliding window is deterministic. A unique violation on
// (provider, request_key) surfaces as ErrAlreadyExists: the provider handed
// out a request key that is already tracked.
func (r *CardBindingSessionRepository) Create(ctx context.Context, session domain.CardBindingSession) (domain.CardBindingSession, error) {
	row, err := r.q().CreateCardBindingSession(ctx, postgres.CreateCardBindingSessionParams{
		ID:         pgtype.UUID{Bytes: session.ID, Valid: true},
		UserID:     pgtype.UUID{Bytes: session.UserID, Valid: true},
		Provider:   string(session.Provider),
		RequestKey: session.RequestKey,
		Status:     string(session.Status),
		ExpiresAt:  pgtype.Timestamptz{Time: session.ExpiresAt, Valid: true},
		CreatedAt:  pgtype.Timestamptz{Time: session.CreatedAt, Valid: true},
	})
	if err != nil {
		if pgerr.IsUniqueViolation(err) {
			return domain.CardBindingSession{}, application.ErrAlreadyExists
		}
		return domain.CardBindingSession{}, fmt.Errorf("create card binding session: %w", err)
	}
	return mapBindingSession(row)
}

// GetByRequestKeyForUpdate returns the provider's binding session by its
// request key, locking the row for update. Must only be called inside a
// transaction.
func (r *CardBindingSessionRepository) GetByRequestKeyForUpdate(
	ctx context.Context, provider domain.PaymentProvider, requestKey string,
) (domain.CardBindingSession, error) {
	row, err := r.q().GetCardBindingSessionByRequestKeyForUpdate(ctx, postgres.GetCardBindingSessionByRequestKeyForUpdateParams{
		Provider:   string(provider),
		RequestKey: requestKey,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.CardBindingSession{}, application.ErrNotFound
		}
		return domain.CardBindingSession{}, fmt.Errorf("get card binding session for update: %w", err)
	}
	return mapBindingSession(row)
}

// ListOpenByUserID returns the user's binding sessions still awaiting an
// outcome, newest first.
func (r *CardBindingSessionRepository) ListOpenByUserID(ctx context.Context, userID uuid.UUID) ([]domain.CardBindingSession, error) {
	rows, err := r.q().ListOpenCardBindingSessionsByUserID(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list open card binding sessions: %w", err)
	}
	sessions := make([]domain.CardBindingSession, 0, len(rows))
	for _, row := range rows {
		session, err := mapBindingSession(row)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

// CountStartedSince returns how many binding sessions the user started at or
// after the instant — the sliding window of the per-user binding limit
// (ticket #427).
func (r *CardBindingSessionRepository) CountStartedSince(ctx context.Context, userID uuid.UUID, since time.Time) (int, error) {
	count, err := r.q().CountCardBindingSessionsByUserSince(ctx, postgres.CountCardBindingSessionsByUserSinceParams{
		UserID:    pgtype.UUID{Bytes: userID, Valid: true},
		CreatedAt: pgtype.Timestamptz{Time: since, Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("count card binding sessions: %w", err)
	}
	return int(count), nil
}

// UpdateStatus persists the session's status transition. Callers hold the row
// lock (GetByRequestKeyForUpdate) in the same transaction before mutating the
// aggregate.
func (r *CardBindingSessionRepository) UpdateStatus(ctx context.Context, session domain.CardBindingSession) error {
	if _, err := r.q().UpdateCardBindingSessionStatus(ctx, postgres.UpdateCardBindingSessionStatusParams{
		ID:     pgtype.UUID{Bytes: session.ID, Valid: true},
		Status: string(session.Status),
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrNotFound
		}
		return fmt.Errorf("update card binding session status: %w", err)
	}
	return nil
}

// mapBindingSession reconstitutes the stored row; a row that fails validation
// (schema/enum drift) is an error the caller must see, not silently dropped
// state.
func mapBindingSession(row postgres.CardBindingSession) (domain.CardBindingSession, error) {
	return domain.ReconstituteCardBindingSession(domain.CardBindingSession{
		ID:         pgconv.UUIDFromPgtype(row.ID),
		UserID:     pgconv.UUIDFromPgtype(row.UserID),
		Provider:   domain.PaymentProvider(row.Provider),
		RequestKey: row.RequestKey,
		Status:     domain.CardBindingSessionStatus(row.Status),
		ExpiresAt:  pgconv.TimestamptzToTime(row.ExpiresAt),
		CreatedAt:  pgconv.TimestamptzToTime(row.CreatedAt),
		UpdatedAt:  pgconv.TimestamptzToTime(row.UpdatedAt),
	})
}
