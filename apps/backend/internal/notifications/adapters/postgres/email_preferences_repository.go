package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// The adapter satisfies the consumer-declared port (CODING_STANDARDS).
var _ application.EmailPreferencesRepository = (*EmailPreferencesRepository)(nil)

// EmailPreferencesRepository persists the account-level email matrix
// (решение #738, ADR 0056): one row of four configurable category flags per
// user, absent row = all-on default.
type EmailPreferencesRepository struct {
	db postgres.DBTX
}

// NewEmailPreferencesRepository creates the email preferences repository.
func NewEmailPreferencesRepository(db postgres.DBTX) *EmailPreferencesRepository {
	return &EmailPreferencesRepository{db: db}
}

func (r *EmailPreferencesRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// Get returns the user's email matrix; a missing row reads as the all-on
// default without inserting anything.
func (r *EmailPreferencesRepository) Get(ctx context.Context, userID uuid.UUID) (domain.CategoryPrefs, error) {
	row, err := r.q().GetEmailPreferences(ctx, pgconv.UUIDToPgtype(userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.DefaultCategoryPrefs(), nil
		}
		return domain.CategoryPrefs{}, fmt.Errorf("get email preferences: %w", err)
	}
	return domain.CategoryPrefs{
		Rental:             row.Rental,
		PaymentsOperations: row.PaymentsOperations,
		Tasks:              row.Tasks,
		SharedAccess:       row.SharedAccess,
	}, nil
}

// Set replaces the user's email matrix (PUT is a full replacement).
func (r *EmailPreferencesRepository) Set(ctx context.Context, userID uuid.UUID, prefs domain.CategoryPrefs) error {
	_, err := r.q().UpsertEmailPreferences(ctx, postgres.UpsertEmailPreferencesParams{
		UserID:             pgconv.UUIDToPgtype(userID),
		Rental:             prefs.Rental,
		PaymentsOperations: prefs.PaymentsOperations,
		Tasks:              prefs.Tasks,
		SharedAccess:       prefs.SharedAccess,
	})
	if err != nil {
		return fmt.Errorf("set email preferences: %w", err)
	}
	return nil
}
