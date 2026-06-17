package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// ReminderRepository persists reminders using generated sqlc queries.
type ReminderRepository struct {
	db postgres.DBTX
}

// NewReminderRepository creates a new reminder repository.
func NewReminderRepository(db postgres.DBTX) *ReminderRepository {
	return &ReminderRepository{db: db}
}

func (r *ReminderRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *ReminderRepository) WithTx(tx transaction.Tx) application.ReminderRepository {
	pgtx, ok := tx.(postgres.DBTX)
	if !ok {
		panic(fmt.Sprintf("notifications.ReminderRepository: expected postgres.DBTX, got %T", tx))
	}
	return NewReminderRepository(pgtx)
}

func eventDateToPgtype(t time.Time) pgtype.Date {
	if t.IsZero() {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}

// Save inserts a reminder.
func (r *ReminderRepository) Save(ctx context.Context, rm domain.Reminder) error {
	_, err := r.q().CreateReminder(ctx, postgres.CreateReminderParams{
		ID:                   pgconv.UUIDToPgtype(rm.ID),
		OwnerID:              pgconv.UUIDToPgtype(rm.OwnerID),
		TargetType:           postgres.NotificationTargetType(rm.TargetType),
		OperationID:          pgconv.UUIDToPgtypePtr(rm.OperationID),
		RecurringOperationID: pgconv.UUIDToPgtypePtr(rm.RecurringOperationID),
		LeaseID:              pgconv.UUIDToPgtypePtr(rm.LeaseID),
		PropertyID:           pgconv.UUIDToPgtypePtr(rm.PropertyID),
		EventType:            postgres.NotificationEventType(rm.EventType),
		Status:               postgres.NotificationStatus(rm.Status),
		ScheduledAt:          pgtype.Timestamptz{Time: rm.ScheduledAt, Valid: true},
		EventDate:            eventDateToPgtype(rm.EventDate),
		MessageTitle:         rm.MessageTitle,
		MessageBody:          rm.MessageBody,
		CreatedAt:            pgtype.Timestamptz{Time: rm.CreatedAt, Valid: true},
		UpdatedAt:            pgtype.Timestamptz{Time: rm.UpdatedAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("create reminder: %w", err)
	}
	return nil
}

// Update replaces an existing reminder.
func (r *ReminderRepository) Update(ctx context.Context, rm domain.Reminder) error {
	var sentAt, nextAttemptAt pgtype.Timestamptz
	if rm.SentAt != nil {
		sentAt = pgtype.Timestamptz{Time: *rm.SentAt, Valid: true}
	}
	if rm.NextAttemptAt != nil {
		nextAttemptAt = pgtype.Timestamptz{Time: *rm.NextAttemptAt, Valid: true}
	}
	_, err := r.q().UpdateReminder(ctx, postgres.UpdateReminderParams{
		ID:                   pgconv.UUIDToPgtype(rm.ID),
		OwnerID:              pgconv.UUIDToPgtype(rm.OwnerID),
		TargetType:           postgres.NotificationTargetType(rm.TargetType),
		OperationID:          pgconv.UUIDToPgtypePtr(rm.OperationID),
		RecurringOperationID: pgconv.UUIDToPgtypePtr(rm.RecurringOperationID),
		LeaseID:              pgconv.UUIDToPgtypePtr(rm.LeaseID),
		PropertyID:           pgconv.UUIDToPgtypePtr(rm.PropertyID),
		EventType:            postgres.NotificationEventType(rm.EventType),
		Status:               postgres.NotificationStatus(rm.Status),
		ScheduledAt:          pgtype.Timestamptz{Time: rm.ScheduledAt, Valid: true},
		EventDate:            eventDateToPgtype(rm.EventDate),
		SentAt:               sentAt,
		//nolint:gosec // FailedAttempts is bounded by retry logic in the application layer.
		FailedAttempts: int32(rm.FailedAttempts),
		NextAttemptAt:  nextAttemptAt,
		MessageTitle:   rm.MessageTitle,
		MessageBody:    rm.MessageBody,
		UpdatedAt:      pgtype.Timestamptz{Time: rm.UpdatedAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("update reminder: %w", err)
	}
	return nil
}

// GetByID returns a reminder by ID.
func (r *ReminderRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Reminder, error) {
	row, err := r.q().GetReminderByID(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Reminder{}, application.ErrNotFound
		}
		return domain.Reminder{}, fmt.Errorf("get reminder: %w", err)
	}
	return toDomain(row), nil
}

// ListByOwner returns reminders for an owner with optional status filter.
func (r *ReminderRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID, filter application.ListFilter) ([]domain.Reminder, error) {
	params := postgres.ListRemindersByOwnerParams{
		OwnerID:        pgconv.UUIDToPgtype(ownerID),
		FilterByStatus: filter.Status != nil,
		//nolint:gosec // Pagination values are bounded by the transport layer.
		Offset: int32(filter.Offset),
		//nolint:gosec // Pagination values are bounded by the transport layer.
		Limit: int32(filter.Limit),
	}
	if filter.Status != nil {
		params.Status = postgres.NotificationStatus(*filter.Status)
	}
	rows, err := r.q().ListRemindersByOwner(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	out := make([]domain.Reminder, len(rows))
	for i, row := range rows {
		out[i] = toDomain(row)
	}
	return out, nil
}

// ListDue returns pending reminders that are due before the given time.
func (r *ReminderRepository) ListDue(ctx context.Context, before time.Time, limit int) ([]domain.Reminder, error) {
	rows, err := r.q().ListDueReminders(ctx, postgres.ListDueRemindersParams{
		ScheduledAt: pgtype.Timestamptz{Time: before, Valid: true},
		//nolint:gosec // Worker batch size is configured and bounded.
		Limit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list due reminders: %w", err)
	}
	out := make([]domain.Reminder, len(rows))
	for i, row := range rows {
		out[i] = toDomain(row)
	}
	return out, nil
}

// MarkReminderSending transitions a pending reminder to sending and returns the updated row.
func (r *ReminderRepository) MarkReminderSending(ctx context.Context, id uuid.UUID) (domain.Reminder, error) {
	row, err := r.q().MarkReminderSending(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Reminder{}, application.ErrNotFound
		}
		return domain.Reminder{}, fmt.Errorf("mark reminder sending: %w", err)
	}
	return toDomain(row), nil
}

// ListStaleSendingReminders returns sending reminders whose updated_at is older than staleBefore.
func (r *ReminderRepository) ListStaleSendingReminders(ctx context.Context, staleBefore time.Time, limit int) ([]domain.Reminder, error) {
	rows, err := r.q().ListStaleSendingReminders(ctx, postgres.ListStaleSendingRemindersParams{
		UpdatedAt: pgtype.Timestamptz{Time: staleBefore, Valid: true},
		//nolint:gosec // Worker batch size is configured and bounded.
		Limit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list stale sending reminders: %w", err)
	}
	out := make([]domain.Reminder, len(rows))
	for i, row := range rows {
		out[i] = toDomain(row)
	}
	return out, nil
}

// MarkSent marks a sending reminder as sent.
func (r *ReminderRepository) MarkSent(ctx context.Context, id uuid.UUID, at time.Time) error {
	_, err := r.q().MarkReminderSent(ctx, postgres.MarkReminderSentParams{
		ID:     pgconv.UUIDToPgtype(id),
		SentAt: pgtype.Timestamptz{Time: at, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("mark reminder sent: %w", err)
	}
	return nil
}

// MarkFailed increments failed attempts and optionally marks a reminder as terminal failed.
func (r *ReminderRepository) MarkFailed(ctx context.Context, id uuid.UUID, nextAttempt *time.Time, terminal bool) error {
	var next pgtype.Timestamptz
	if nextAttempt != nil {
		next = pgtype.Timestamptz{Time: *nextAttempt, Valid: true}
	}
	_, err := r.q().MarkReminderFailed(ctx, postgres.MarkReminderFailedParams{
		ID:            pgconv.UUIDToPgtype(id),
		NextAttemptAt: next,
		MarkAsFailed:  terminal,
	})
	if err != nil {
		return fmt.Errorf("mark reminder failed: %w", err)
	}
	return nil
}

// CancelByTarget cancels pending reminders for a concrete target and event type.
func (r *ReminderRepository) CancelByTarget(ctx context.Context, ownerID uuid.UUID, targetType domain.TargetType, targetID uuid.UUID, eventType domain.EventType) error {
	_, err := r.q().CancelReminderByTarget(ctx, postgres.CancelReminderByTargetParams{
		OwnerID:    pgconv.UUIDToPgtype(ownerID),
		TargetType: postgres.NotificationTargetType(targetType),
		TargetID:   pgconv.UUIDToPgtype(targetID),
		EventType:  postgres.NotificationEventType(eventType),
	})
	if err != nil {
		return fmt.Errorf("cancel reminder by target: %w", err)
	}
	return nil
}

// CancelByRecurringOperationID cancels all pending reminders linked to a recurring operation template.
func (r *ReminderRepository) CancelByRecurringOperationID(ctx context.Context, ownerID, recID uuid.UUID) error {
	_, err := r.q().CancelRemindersByRecurringOperationID(ctx, postgres.CancelRemindersByRecurringOperationIDParams{
		OwnerID:              pgconv.UUIDToPgtype(ownerID),
		RecurringOperationID: pgconv.UUIDToPgtype(recID),
	})
	if err != nil {
		return fmt.Errorf("cancel reminders by recurring operation id: %w", err)
	}
	return nil
}

// HasReminderForLeaseEvent reports whether a non-cancelled reminder already exists
// for the given lease and event type.
func (r *ReminderRepository) HasReminderForLeaseEvent(ctx context.Context, ownerID, leaseID uuid.UUID, eventType domain.EventType) (bool, error) {
	exists, err := r.q().HasReminderForLeaseEvent(ctx, postgres.HasReminderForLeaseEventParams{
		OwnerID:   pgconv.UUIDToPgtype(ownerID),
		LeaseID:   pgconv.UUIDToPgtype(leaseID),
		EventType: postgres.NotificationEventType(eventType),
	})
	if err != nil {
		return false, fmt.Errorf("check reminder for lease event: %w", err)
	}
	return exists, nil
}

// ResetReminderSending resets a sending reminder back to pending.
func (r *ReminderRepository) ResetReminderSending(ctx context.Context, id uuid.UUID) error {
	_, err := r.q().ResetReminderSending(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		return fmt.Errorf("reset reminder sending: %w", err)
	}
	return nil
}

// SaveSentSMSReminder records a successfully sent SMS reminder for audit.
func (r *ReminderRepository) SaveSentSMSReminder(ctx context.Context, id, reminderID, ownerID uuid.UUID, phone, message, providerResponse string, sentAt time.Time) error {
	_, err := r.q().CreateSentSMSReminder(ctx, postgres.CreateSentSMSReminderParams{
		ID:               pgconv.UUIDToPgtype(id),
		ReminderID:       pgconv.UUIDToPgtype(reminderID),
		OwnerID:          pgconv.UUIDToPgtype(ownerID),
		Phone:            phone,
		Message:          message,
		ProviderResponse: pgtype.Text{String: providerResponse, Valid: providerResponse != ""},
		SentAt:           pgtype.Timestamptz{Time: sentAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("create sent sms reminder: %w", err)
	}
	return nil
}

func toDomain(row postgres.Reminder) domain.Reminder {
	return domain.Reminder{
		ID:                   pgconv.UUIDFromPgtype(row.ID),
		OwnerID:              pgconv.UUIDFromPgtype(row.OwnerID),
		TargetType:           domain.TargetType(row.TargetType),
		OperationID:          pgconv.UUIDFromPgtypePtr(row.OperationID),
		RecurringOperationID: pgconv.UUIDFromPgtypePtr(row.RecurringOperationID),
		LeaseID:              pgconv.UUIDFromPgtypePtr(row.LeaseID),
		PropertyID:           pgconv.UUIDFromPgtypePtr(row.PropertyID),
		EventType:            domain.EventType(row.EventType),
		Status:               domain.ReminderStatus(row.Status),
		ScheduledAt:          pgconv.TimestamptzToTime(row.ScheduledAt),
		EventDate:            pgconv.DateFromPgtype(row.EventDate),
		SentAt:               pgconv.TimestamptzToPtrTime(row.SentAt),
		FailedAttempts:       int(row.FailedAttempts),
		NextAttemptAt:        pgconv.TimestamptzToPtrTime(row.NextAttemptAt),
		MessageTitle:         row.MessageTitle,
		MessageBody:          row.MessageBody,
		CreatedAt:            pgconv.TimestamptzToTime(row.CreatedAt),
		UpdatedAt:            pgconv.TimestamptzToTime(row.UpdatedAt),
	}
}
