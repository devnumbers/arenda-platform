package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// FreeReminderRepository persists free reminder templates using generated sqlc
// queries and materializes concrete reminders from them.
type FreeReminderRepository struct {
	db postgres.DBTX
}

// NewFreeReminderRepository creates a new free reminder repository.
func NewFreeReminderRepository(db postgres.DBTX) *FreeReminderRepository {
	return &FreeReminderRepository{db: db}
}

func (r *FreeReminderRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *FreeReminderRepository) WithTx(tx transaction.Tx) application.FreeReminderRepository {
	pgtx, ok := tx.(postgres.DBTX)
	if !ok {
		panic(fmt.Sprintf("notifications.FreeReminderRepository: expected postgres.DBTX, got %T", tx))
	}
	return NewFreeReminderRepository(pgtx)
}

// Create inserts a free reminder template.
func (r *FreeReminderRepository) Create(ctx context.Context, fr domain.FreeReminder) (domain.FreeReminder, error) {
	row, err := r.q().CreateFreeReminder(ctx, postgres.CreateFreeReminderParams{
		ID:          pgconv.UUIDToPgtype(fr.ID),
		OwnerID:     pgconv.UUIDToPgtype(fr.OwnerID),
		PropertyID:  pgconv.UUIDToPgtype(fr.PropertyID),
		Title:       fr.Title,
		TriggerAt:   pgtype.Timestamptz{Time: fr.TriggerAt, Valid: true},
		Periodicity: string(fr.Periodicity),
		CreatedAt:   pgtype.Timestamptz{Time: fr.CreatedAt, Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: fr.UpdatedAt, Valid: true},
	})
	if err != nil {
		return domain.FreeReminder{}, fmt.Errorf("create free reminder: %w", err)
	}
	return freeReminderToDomain(row), nil
}

// GetByID returns a free reminder by ID scoped to an owner.
func (r *FreeReminderRepository) GetByID(ctx context.Context, id, scope uuid.UUID) (domain.FreeReminder, error) {
	row, err := r.q().GetFreeReminderByIDAndOwner(ctx, postgres.GetFreeReminderByIDAndOwnerParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(scope),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.FreeReminder{}, application.ErrNotFound
		}
		return domain.FreeReminder{}, fmt.Errorf("get free reminder: %w", err)
	}
	return freeReminderToDomain(row), nil
}

// GetByIDUnscoped returns a free reminder by id without owner scoping. It
// exists so the application layer can resolve the entity's owner (scope)
// before applying the policy gate (T3, issue #166).
func (r *FreeReminderRepository) GetByIDUnscoped(ctx context.Context, id uuid.UUID) (domain.FreeReminder, error) {
	row, err := r.q().GetFreeReminderByIDUnscoped(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.FreeReminder{}, application.ErrNotFound
		}
		return domain.FreeReminder{}, fmt.Errorf("get free reminder: %w", err)
	}
	return freeReminderToDomain(row), nil
}

// Update updates a free reminder template (title, trigger_at, periodicity).
func (r *FreeReminderRepository) Update(ctx context.Context, fr domain.FreeReminder) (domain.FreeReminder, error) {
	row, err := r.q().UpdateFreeReminder(ctx, postgres.UpdateFreeReminderParams{
		ID:          pgconv.UUIDToPgtype(fr.ID),
		OwnerID:     pgconv.UUIDToPgtype(fr.OwnerID),
		Title:       fr.Title,
		TriggerAt:   pgtype.Timestamptz{Time: fr.TriggerAt, Valid: true},
		Periodicity: string(fr.Periodicity),
		UpdatedAt:   pgtype.Timestamptz{Time: fr.UpdatedAt, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.FreeReminder{}, application.ErrNotFound
		}
		return domain.FreeReminder{}, fmt.Errorf("update free reminder: %w", err)
	}
	return freeReminderToDomain(row), nil
}

// Delete removes a free reminder template. Concrete reminders are removed by
// the ON DELETE CASCADE on reminders.free_reminder_id.
func (r *FreeReminderRepository) Delete(ctx context.Context, scope, id uuid.UUID) error {
	rows, err := r.q().DeleteFreeReminderByIDAndOwner(ctx, postgres.DeleteFreeReminderByIDAndOwnerParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(scope),
	})
	if err != nil {
		return fmt.Errorf("delete free reminder: %w", err)
	}
	if rows == 0 {
		return application.ErrNotFound
	}
	return nil
}

// ListByOwner returns free reminders for an owner ordered by trigger_at.
func (r *FreeReminderRepository) ListByOwner(ctx context.Context, scope uuid.UUID, limit, offset int, accessiblePropertyIDs []uuid.UUID) ([]domain.FreeReminder, error) {
	rows, err := r.q().ListFreeRemindersByOwner(ctx, postgres.ListFreeRemindersByOwnerParams{
		OwnerID:               pgconv.UUIDToPgtype(scope),
		AccessiblePropertyIds: pgconv.UUIDSliceToPgtype(accessiblePropertyIDs),
		//nolint:gosec // Pagination values are bounded by the transport layer.
		Limit: int32(limit),
		//nolint:gosec // Pagination values are bounded by the transport layer.
		Offset: int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list free reminders: %w", err)
	}
	out := make([]domain.FreeReminder, len(rows))
	for i, row := range rows {
		out[i] = freeReminderToDomain(row)
	}
	return out, nil
}

// ListByProperty returns up to limit free reminders for a property ordered by trigger_at.
func (r *FreeReminderRepository) ListByProperty(ctx context.Context, scope, propertyID uuid.UUID, limit int) ([]domain.FreeReminder, error) {
	rows, err := r.q().ListFreeRemindersByProperty(ctx, postgres.ListFreeRemindersByPropertyParams{
		OwnerID:    pgconv.UUIDToPgtype(scope),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
		//nolint:gosec // Limit is bounded by the caller.
		Limit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list free reminders by property: %w", err)
	}
	out := make([]domain.FreeReminder, len(rows))
	for i, row := range rows {
		out[i] = freeReminderToDomain(row)
	}
	return out, nil
}

// ListTemplatesByOwner returns all free reminder templates for an owner with
// their resolved property name (nullable for orphans), ordered by trigger_at.
func (r *FreeReminderRepository) ListTemplatesByOwner(ctx context.Context, scope uuid.UUID, accessiblePropertyIDs []uuid.UUID) ([]domain.FreeReminderTemplate, error) {
	rows, err := r.q().ListAllFreeRemindersByOwner(ctx, postgres.ListAllFreeRemindersByOwnerParams{
		OwnerID:               pgconv.UUIDToPgtype(scope),
		AccessiblePropertyIds: pgconv.UUIDSliceToPgtype(accessiblePropertyIDs),
	})
	if err != nil {
		return nil, fmt.Errorf("list all free reminders by owner: %w", err)
	}
	out := make([]domain.FreeReminderTemplate, len(rows))
	for i, row := range rows {
		out[i] = domain.FreeReminderTemplate{
			FreeReminder: domain.FreeReminder{
				ID:          pgconv.UUIDFromPgtype(row.ID),
				OwnerID:     pgconv.UUIDFromPgtype(row.OwnerID),
				PropertyID:  pgconv.UUIDFromPgtype(row.PropertyID),
				Title:       row.Title,
				TriggerAt:   pgconv.TimestamptzToTime(row.TriggerAt),
				Periodicity: domain.FreeReminderPeriodicity(row.Periodicity),
				CreatedAt:   pgconv.TimestamptzToTime(row.CreatedAt),
				UpdatedAt:   pgconv.TimestamptzToTime(row.UpdatedAt),
			},
			PropertyName: pgconv.TextToPtrString(row.PropertyName),
		}
	}
	return out, nil
}

// SaveFreeReminder inserts a concrete reminder row materialized from a free
// reminder template (target_type='free', event_type='free_reminder').
func (r *FreeReminderRepository) SaveFreeReminder(ctx context.Context, rm domain.Reminder) error {
	_, err := r.q().SaveFreeReminder(ctx, postgres.SaveFreeReminderParams{
		ID:                   pgconv.UUIDToPgtype(rm.ID),
		OwnerID:              pgconv.UUIDToPgtype(rm.OwnerID),
		TargetType:           postgres.NotificationTargetType(rm.TargetType),
		OperationID:          pgconv.UUIDToPgtypePtr(rm.OperationID),
		RecurringOperationID: pgconv.UUIDToPgtypePtr(rm.RecurringOperationID),
		LeaseID:              pgconv.UUIDToPgtypePtr(rm.LeaseID),
		PropertyID:           pgconv.UUIDToPgtypePtr(rm.PropertyID),
		FreeReminderID:       pgconv.UUIDToPgtypePtr(rm.FreeReminderID),
		EventType:            postgres.NotificationEventType(rm.EventType),
		Status:               postgres.NotificationStatus(rm.Status),
		ScheduledAt:          pgtype.Timestamptz{Time: rm.ScheduledAt, Valid: true},
		MessageTitle:         rm.MessageTitle,
		MessageBody:          rm.MessageBody,
		CreatedAt:            pgtype.Timestamptz{Time: rm.CreatedAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("save free reminder concrete: %w", err)
	}
	return nil
}

// CancelRemindersByFreeReminderID cancels pending/sending concrete reminders
// linked to a free reminder template.
func (r *FreeReminderRepository) CancelRemindersByFreeReminderID(ctx context.Context, scope, freeReminderID uuid.UUID) error {
	_, err := r.q().CancelRemindersByFreeReminderID(ctx, postgres.CancelRemindersByFreeReminderIDParams{
		FreeReminderID: pgconv.UUIDToPgtype(freeReminderID),
		OwnerID:        pgconv.UUIDToPgtype(scope),
	})
	if err != nil {
		return fmt.Errorf("cancel reminders by free reminder id: %w", err)
	}
	return nil
}

func freeReminderToDomain(row postgres.FreeReminder) domain.FreeReminder {
	return domain.FreeReminder{
		ID:          pgconv.UUIDFromPgtype(row.ID),
		OwnerID:     pgconv.UUIDFromPgtype(row.OwnerID),
		PropertyID:  pgconv.UUIDFromPgtype(row.PropertyID),
		Title:       row.Title,
		TriggerAt:   pgconv.TimestamptzToTime(row.TriggerAt),
		Periodicity: domain.FreeReminderPeriodicity(row.Periodicity),
		CreatedAt:   pgconv.TimestamptzToTime(row.CreatedAt),
		UpdatedAt:   pgconv.TimestamptzToTime(row.UpdatedAt),
	}
}
