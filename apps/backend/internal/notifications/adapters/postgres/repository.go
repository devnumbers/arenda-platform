package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared"
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
		MessageTitle:         rm.MessageTitle,
		MessageBody:          rm.MessageBody,
		CreatedAt:            pgtype.Timestamptz{Time: rm.CreatedAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("create reminder: %w", err)
	}
	return nil
}

// SaveOrReplaceOperationReminder atomically replaces an existing pending/sending
// reminder for the same owner, target and event with the new one. If no active
// reminder exists, it simply inserts the new reminder.
func (r *ReminderRepository) SaveOrReplaceOperationReminder(ctx context.Context, rm domain.Reminder) error {
	rows, err := r.q().SaveOrReplaceOperationReminder(ctx, postgres.SaveOrReplaceOperationReminderParams{
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
		MessageTitle:         rm.MessageTitle,
		MessageBody:          rm.MessageBody,
		CreatedAt:            pgtype.Timestamptz{Time: rm.CreatedAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("save or replace operation reminder: %w", err)
	}
	switch rows {
	case 1:
		return nil
	case 0:
		return application.ErrConcurrentUpdate
	default:
		return fmt.Errorf("save or replace operation reminder: unexpected rows affected %d", rows)
	}
}

// UpdateScheduledAt updates the scheduled time of a pending reminder.
func (r *ReminderRepository) UpdateScheduledAt(ctx context.Context, scope, id uuid.UUID, scheduledAt time.Time) error {
	rows, err := r.q().UpdateReminderScheduledAt(ctx, postgres.UpdateReminderScheduledAtParams{
		ScheduledAt: pgtype.Timestamptz{Time: scheduledAt, Valid: true},
		ID:          pgconv.UUIDToPgtype(id),
		OwnerID:     pgconv.UUIDToPgtype(scope),
	})
	if err != nil {
		return fmt.Errorf("update reminder scheduled at: %w", err)
	}
	if rows == 0 {
		return application.ErrReminderNotPending
	}
	return nil
}

// ReschedulePendingRemindersByOwner recalculates the scheduled_at of all pending
// reminders for an owner using wall-clock timezone conversion: the local date
// and time-of-day seen in oldTZ are re-applied in newTZ.
func (r *ReminderRepository) ReschedulePendingRemindersByOwner(ctx context.Context, scope uuid.UUID, oldTZ, newTZ string) error {
	if _, err := r.q().ReschedulePendingRemindersByOwner(ctx, postgres.ReschedulePendingRemindersByOwnerParams{
		OwnerID: pgconv.UUIDToPgtype(scope),
		OldTz:   oldTZ,
		NewTz:   newTZ,
	}); err != nil {
		return fmt.Errorf("reschedule pending reminders by owner: %w", err)
	}
	return nil
}

// GetByID returns a reminder by ID scoped to an owner.
func (r *ReminderRepository) GetByID(ctx context.Context, id, scope uuid.UUID) (domain.Reminder, error) {
	row, err := r.q().GetReminderByIDAndOwner(ctx, postgres.GetReminderByIDAndOwnerParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(scope),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Reminder{}, application.ErrNotFound
		}
		return domain.Reminder{}, fmt.Errorf("get reminder: %w", err)
	}
	return toDomain(row), nil
}

// CancelByIDAndOwner cancels a reminder if it belongs to the owner and is still
// pending or sending. It returns true when a row was updated.
func (r *ReminderRepository) CancelByIDAndOwner(ctx context.Context, scope, reminderID uuid.UUID) (bool, error) {
	rows, err := r.q().CancelByIDAndOwner(ctx, postgres.CancelByIDAndOwnerParams{
		ID:      pgconv.UUIDToPgtype(reminderID),
		OwnerID: pgconv.UUIDToPgtype(scope),
	})
	if err != nil {
		return false, fmt.Errorf("cancel reminder by id and owner: %w", err)
	}
	return rows > 0, nil
}

// GetByIDUnscoped returns a reminder by ID without owner scoping for internal/worker use.
func (r *ReminderRepository) GetByIDUnscoped(ctx context.Context, id uuid.UUID) (domain.Reminder, error) {
	row, err := r.q().GetReminderByIDUnscoped(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Reminder{}, application.ErrNotFound
		}
		return domain.Reminder{}, fmt.Errorf("get reminder unscoped: %w", err)
	}
	return toDomain(row), nil
}

// ListByOwner returns reminders for an owner with optional status filter.
func (r *ReminderRepository) ListByOwner(ctx context.Context, scope uuid.UUID, filter application.ListFilter, accessiblePropertyIDs []uuid.UUID) ([]domain.Reminder, error) {
	params := postgres.ListRemindersByOwnerParams{
		OwnerID:               pgconv.UUIDToPgtype(scope),
		AccessiblePropertyIds: pgconv.UUIDSliceToPgtype(accessiblePropertyIDs),
		FilterByStatus:        filter.Status != nil,
		Offset:                shared.ToInt32Clamped(filter.Offset),
		Limit:                 shared.ToInt32Clamped(filter.Limit),
	}
	if filter.Status != nil {
		params.Status = string(postgres.NotificationStatus(*filter.Status))
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

// ListByOperation returns non-cancelled reminders linked to a concrete operation
// for the given owner.
func (r *ReminderRepository) ListByOperation(ctx context.Context, scope, operationID uuid.UUID, filter application.ListFilter) ([]domain.Reminder, error) {
	rows, err := r.q().ListRemindersByOperation(ctx, postgres.ListRemindersByOperationParams{
		OwnerID:     pgconv.UUIDToPgtype(scope),
		OperationID: pgconv.UUIDToPgtype(operationID),
		Limit:       shared.ToInt32Clamped(filter.Limit),
		Offset:      shared.ToInt32Clamped(filter.Offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	out := make([]domain.Reminder, len(rows))
	for i, row := range rows {
		out[i] = toDomain(row)
	}
	return out, nil
}

// ListByLease returns non-cancelled reminders linked to a lease for the given owner.
func (r *ReminderRepository) ListByLease(ctx context.Context, scope, leaseID uuid.UUID, filter application.ListFilter) ([]domain.Reminder, error) {
	rows, err := r.q().ListRemindersByLease(ctx, postgres.ListRemindersByLeaseParams{
		OwnerID: pgconv.UUIDToPgtype(scope),
		LeaseID: pgconv.UUIDToPgtype(leaseID),
		Limit:   shared.ToInt32Clamped(filter.Limit),
		Offset:  shared.ToInt32Clamped(filter.Offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	out := make([]domain.Reminder, len(rows))
	for i, row := range rows {
		out[i] = toDomain(row)
	}
	return out, nil
}

// ListByRecurringOperation returns non-cancelled reminders linked to a recurring
// operation template for the given owner.
func (r *ReminderRepository) ListByRecurringOperation(ctx context.Context, scope, recurringOpID uuid.UUID, filter application.ListFilter) ([]domain.Reminder, error) {
	rows, err := r.q().ListRemindersByRecurringOperation(ctx, postgres.ListRemindersByRecurringOperationParams{
		OwnerID:              pgconv.UUIDToPgtype(scope),
		RecurringOperationID: pgconv.UUIDToPgtype(recurringOpID),
		Limit:                shared.ToInt32Clamped(filter.Limit),
		Offset:               shared.ToInt32Clamped(filter.Offset),
	})
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
		Limit:       shared.ToInt32Clamped(limit),
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

// ListCalendarByOwner returns non-cancelled, non-skipped operation and system
// reminders for an owner in [from, to), each joined with its property name
// (nil for orphans). Ordered by scheduled_at ascending.
func (r *ReminderRepository) ListCalendarByOwner(ctx context.Context, scope uuid.UUID, from, to time.Time, accessiblePropertyIDs []uuid.UUID) ([]domain.CalendarReminder, error) {
	rows, err := r.q().ListCalendarRemindersByOwner(ctx, postgres.ListCalendarRemindersByOwnerParams{
		OwnerID:               pgconv.UUIDToPgtype(scope),
		AccessiblePropertyIds: pgconv.UUIDSliceToPgtype(accessiblePropertyIDs),
		FromTime:              pgtype.Timestamptz{Time: from, Valid: true},
		ToTime:                pgtype.Timestamptz{Time: to, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("list calendar reminders: %w", err)
	}

	out := make([]domain.CalendarReminder, len(rows))
	for i, row := range rows {
		rem := toDomain(row.Reminder)
		status := normalizeCalendarStatus(rem.Status)
		eventType := rem.EventType
		out[i] = domain.CalendarReminder{
			ID:           rem.ID,
			Type:         domain.CalendarReminderTypeFromTarget(rem.TargetType),
			ScheduledAt:  rem.ScheduledAt,
			Title:        rem.MessageTitle,
			PropertyID:   rem.PropertyID,
			PropertyName: pgconv.TextToPtrString(row.PropertyName),
			HasProperty:  row.PropertyName.Valid,
			Status:       status,
			EventType:    &eventType,
			OperationID:  rem.OperationID,
			LeaseID:      rem.LeaseID,
		}
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
		Limit:     shared.ToInt32Clamped(limit),
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

// MarkSent marks a sending reminder as sent. It is idempotent: if the reminder
// is already sent, it returns success.
func (r *ReminderRepository) MarkSent(ctx context.Context, id uuid.UUID, at time.Time) error {
	rows, err := r.q().MarkSendingReminderSent(ctx, postgres.MarkSendingReminderSentParams{
		ID:     pgconv.UUIDToPgtype(id),
		SentAt: pgtype.Timestamptz{Time: at, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("mark sending reminder sent: %w", err)
	}
	if rows == 0 {
		// The update was a no-op. Treat as success if the reminder is already sent.
		existing, err := r.GetByIDUnscoped(ctx, id)
		if err != nil {
			return fmt.Errorf("check reminder status after no-op sent: %w", err)
		}
		if existing.Status == domain.ReminderSent {
			return nil
		}
		return application.ErrConcurrentUpdate
	}
	return nil
}

// MarkReminderSent marks a pending or sending reminder as sent. It is
// idempotent: if the reminder is already sent, it returns success.
func (r *ReminderRepository) MarkReminderSent(ctx context.Context, id uuid.UUID, sentAt time.Time) error {
	rows, err := r.q().MarkReminderSent(ctx, postgres.MarkReminderSentParams{
		ID:     pgconv.UUIDToPgtype(id),
		SentAt: pgtype.Timestamptz{Time: sentAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("mark reminder sent: %w", err)
	}
	if rows == 0 {
		existing, err := r.GetByIDUnscoped(ctx, id)
		if err != nil {
			return fmt.Errorf("check reminder status after no-op sent: %w", err)
		}
		if existing.Status == domain.ReminderSent {
			return nil
		}
		return application.ErrConcurrentUpdate
	}
	return nil
}

// MarkFailed increments failed attempts and optionally marks a reminder as terminal failed.
func (r *ReminderRepository) MarkFailed(ctx context.Context, id uuid.UUID, nextAttempt *time.Time, terminal bool) error {
	var next pgtype.Timestamptz
	if nextAttempt != nil {
		next = pgtype.Timestamptz{Time: *nextAttempt, Valid: true}
	}
	rows, err := r.q().MarkReminderFailed(ctx, postgres.MarkReminderFailedParams{
		ID:            pgconv.UUIDToPgtype(id),
		NextAttemptAt: next,
		MarkAsFailed:  terminal,
	})
	if err != nil {
		return fmt.Errorf("mark reminder failed: %w", err)
	}
	if rows == 0 {
		return application.ErrConcurrentUpdate
	}
	return nil
}

// CancelByTarget cancels pending or sending reminders for a concrete target and event type.
func (r *ReminderRepository) CancelByTarget(ctx context.Context, scope uuid.UUID, targetType domain.TargetType, targetID uuid.UUID, eventType domain.EventType) error {
	_, err := r.q().CancelReminderByTarget(ctx, postgres.CancelReminderByTargetParams{
		OwnerID:    pgconv.UUIDToPgtype(scope),
		TargetType: postgres.NotificationTargetType(targetType),
		TargetID:   pgconv.UUIDToPgtype(targetID),
		EventType:  postgres.NotificationEventType(eventType),
	})
	if err != nil {
		return fmt.Errorf("cancel reminder by target: %w", err)
	}
	return nil
}

// CancelByRecurringOperationID cancels all pending or sending reminders linked to a recurring operation template.
func (r *ReminderRepository) CancelByRecurringOperationID(ctx context.Context, scope, recID uuid.UUID) error {
	_, err := r.q().CancelRemindersByRecurringOperationID(ctx, postgres.CancelRemindersByRecurringOperationIDParams{
		OwnerID:              pgconv.UUIDToPgtype(scope),
		RecurringOperationID: pgconv.UUIDToPgtype(recID),
	})
	if err != nil {
		return fmt.Errorf("cancel reminders by recurring operation id: %w", err)
	}
	return nil
}

// HasReminderForLeaseEvent reports whether an active reminder already exists
// for the given lease and event type.
func (r *ReminderRepository) HasReminderForLeaseEvent(ctx context.Context, scope, leaseID uuid.UUID, eventType domain.EventType) (bool, error) {
	exists, err := r.q().HasReminderForLeaseEvent(ctx, postgres.HasReminderForLeaseEventParams{
		OwnerID:   pgconv.UUIDToPgtype(scope),
		LeaseID:   pgconv.UUIDToPgtype(leaseID),
		EventType: postgres.NotificationEventType(eventType),
	})
	if err != nil {
		return false, fmt.Errorf("check reminder for lease event: %w", err)
	}
	return exists, nil
}

// HasReminderForOperationEvent reports whether an active reminder already exists
// for the given operation and event type.
func (r *ReminderRepository) HasReminderForOperationEvent(ctx context.Context, scope, operationID uuid.UUID, eventType domain.EventType) (bool, error) {
	exists, err := r.q().HasReminderForOperationEvent(ctx, postgres.HasReminderForOperationEventParams{
		OwnerID:     pgconv.UUIDToPgtype(scope),
		OperationID: pgconv.UUIDToPgtype(operationID),
		EventType:   postgres.NotificationEventType(eventType),
	})
	if err != nil {
		return false, fmt.Errorf("check reminder for operation event: %w", err)
	}
	return exists, nil
}

// ResetReminderSending resets a sending reminder back to pending.
func (r *ReminderRepository) ResetReminderSending(ctx context.Context, id uuid.UUID) error {
	rows, err := r.q().ResetReminderSending(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		return fmt.Errorf("reset reminder sending: %w", err)
	}
	if rows == 0 {
		return application.ErrConcurrentUpdate
	}
	return nil
}

// MarkSendingReminderPending resets a sending reminder back to pending with a
// given retry time and an incremented failed attempt count. It is used by the
// worker to recover from a failed finalize transaction.
func (r *ReminderRepository) MarkSendingReminderPending(ctx context.Context, id uuid.UUID, nextAttemptAt time.Time) error {
	rows, err := r.q().MarkSendingReminderPending(ctx, postgres.MarkSendingReminderPendingParams{
		ID:            pgconv.UUIDToPgtype(id),
		NextAttemptAt: pgtype.Timestamptz{Time: nextAttemptAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("mark sending reminder pending: %w", err)
	}
	if rows == 0 {
		return application.ErrConcurrentUpdate
	}
	return nil
}

// SaveSentSMSReminder records a successfully sent SMS reminder for audit. It
// returns ErrDuplicateSMSReminder when an audit row for the same reminder_id
// already exists.
func (r *ReminderRepository) SaveSentSMSReminder(ctx context.Context, id, reminderID, scope uuid.UUID, phone, message, providerResponse string, sentAt time.Time) error {
	rows, err := r.q().CreateSentSMSReminder(ctx, postgres.CreateSentSMSReminderParams{
		ID:               pgconv.UUIDToPgtype(id),
		ReminderID:       pgconv.UUIDToPgtype(reminderID),
		OwnerID:          pgconv.UUIDToPgtype(scope),
		Phone:            phone,
		Message:          message,
		ProviderResponse: pgtype.Text{String: providerResponse, Valid: providerResponse != ""},
		SentAt:           pgtype.Timestamptz{Time: sentAt, Valid: true},
	})
	if err != nil {
		if isDuplicateSMSReminderError(err) {
			return application.ErrDuplicateSMSReminder
		}
		return fmt.Errorf("create sent sms reminder: %w", err)
	}
	if rows == 0 {
		return application.ErrDuplicateSMSReminder
	}
	return nil
}

func isDuplicateSMSReminderError(err error) bool {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pgErr.Code == pgerrcode.UniqueViolation && pgErr.ConstraintName == "uq_sent_sms_reminders_reminder_id"
	}
	return false
}

// UpdateSMSProviderResponse updates the provider response for a sent SMS audit row.
func (r *ReminderRepository) UpdateSMSProviderResponse(ctx context.Context, reminderID uuid.UUID, response string) error {
	rows, err := r.q().UpdateSentSMSReminderProviderResponse(ctx, postgres.UpdateSentSMSReminderProviderResponseParams{
		ProviderResponse: pgtype.Text{String: response, Valid: response != ""},
		ReminderID:       pgconv.UUIDToPgtype(reminderID),
	})
	if err != nil {
		return fmt.Errorf("update sent sms reminder provider response: %w", err)
	}
	if rows == 0 {
		return application.ErrNotFound
	}
	return nil
}

// IsSMSReminderSent reports whether an audit row already exists for the given reminder.
func (r *ReminderRepository) IsSMSReminderSent(ctx context.Context, reminderID uuid.UUID) (bool, error) {
	exists, err := r.q().IsSMSReminderSent(ctx, pgconv.UUIDToPgtype(reminderID))
	if err != nil {
		return false, fmt.Errorf("check sent sms reminder: %w", err)
	}
	return exists, nil
}

// SaveSentEmailReminder records a successfully sent email reminder for audit. It
// returns ErrDuplicateEmailReminder when an audit row for the same reminder_id
// and recipient already exists.
func (r *ReminderRepository) SaveSentEmailReminder(ctx context.Context, arg application.SaveSentEmailReminderParams) error {
	rows, err := r.q().SaveSentEmailReminder(ctx, postgres.SaveSentEmailReminderParams{
		ID:         pgconv.UUIDToPgtype(arg.ID),
		ReminderID: pgconv.UUIDToPgtype(arg.ReminderID),
		OwnerID:    pgconv.UUIDToPgtype(arg.ScopeID),
		Email:      arg.Email,
		Subject:    arg.Subject,
		PlainBody:  arg.PlainBody,
		SentAt:     pgtype.Timestamptz{Time: arg.SentAt, Valid: true},
	})
	if err != nil {
		if isDuplicateEmailReminderError(err) {
			return application.ErrDuplicateEmailReminder
		}
		return fmt.Errorf("create sent email reminder: %w", err)
	}
	if rows == 0 {
		return application.ErrDuplicateEmailReminder
	}
	return nil
}

func isDuplicateEmailReminderError(err error) bool {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pgErr.Code == pgerrcode.UniqueViolation && pgErr.ConstraintName == "uq_sent_email_reminders_reminder_recipient"
	}
	return false
}

// IsEmailReminderSent reports whether an audit row already exists for the given
// reminder and recipient.
func (r *ReminderRepository) IsEmailReminderSent(ctx context.Context, reminderID, recipientID uuid.UUID) (bool, error) {
	exists, err := r.q().IsEmailReminderSent(ctx, postgres.IsEmailReminderSentParams{
		ReminderID: pgconv.UUIDToPgtype(reminderID),
		OwnerID:    pgconv.UUIDToPgtype(recipientID),
	})
	if err != nil {
		return false, fmt.Errorf("check sent email reminder: %w", err)
	}
	return exists, nil
}

// DeleteSentEmailReminder removes the sent email audit row for a reminder and
// recipient. It is used to roll back the audit insert when the external send fails.
func (r *ReminderRepository) DeleteSentEmailReminder(ctx context.Context, reminderID, recipientID uuid.UUID) error {
	if err := r.q().DeleteSentEmailReminder(ctx, postgres.DeleteSentEmailReminderParams{
		ReminderID: pgconv.UUIDToPgtype(reminderID),
		OwnerID:    pgconv.UUIDToPgtype(recipientID),
	}); err != nil {
		return fmt.Errorf("delete sent email reminder: %w", err)
	}
	return nil
}

// IsPushReminderSent reports whether a push audit row already exists for the
// given reminder and recipient (per-recipient deduplication for the push
// channel).
func (r *ReminderRepository) IsPushReminderSent(ctx context.Context, reminderID, recipientID uuid.UUID) (bool, error) {
	exists, err := r.q().IsPushReminderSent(ctx, postgres.IsPushReminderSentParams{
		ReminderID:  pgconv.UUIDToPgtype(reminderID),
		RecipientID: pgconv.UUIDToPgtype(recipientID),
	})
	if err != nil {
		return false, fmt.Errorf("check sent push reminder: %w", err)
	}
	return exists, nil
}

// SaveSentPushReminder records a successfully sent push reminder for audit and
// per-recipient deduplication. It returns ErrDuplicatePushReminder when an
// audit row for the same (reminder, recipient) pair already exists.
func (r *ReminderRepository) SaveSentPushReminder(ctx context.Context, arg application.SaveSentPushReminderParams) error {
	rows, err := r.q().SaveSentPushReminder(ctx, postgres.SaveSentPushReminderParams{
		ID:          pgconv.UUIDToPgtype(arg.ID),
		ReminderID:  pgconv.UUIDToPgtype(arg.ReminderID),
		RecipientID: pgconv.UUIDToPgtype(arg.RecipientID),
		SentAt:      pgtype.Timestamptz{Time: arg.SentAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("create sent push reminder: %w", err)
	}
	if rows == 0 {
		return application.ErrDuplicatePushReminder
	}
	return nil
}

// DeleteSentPushReminder removes the push audit row for a reminder and
// recipient. It is used to roll back the audit insert when the external send
// fails.
func (r *ReminderRepository) DeleteSentPushReminder(ctx context.Context, reminderID, recipientID uuid.UUID) error {
	if err := r.q().DeleteSentPushReminder(ctx, postgres.DeleteSentPushReminderParams{
		ReminderID:  pgconv.UUIDToPgtype(reminderID),
		RecipientID: pgconv.UUIDToPgtype(recipientID),
	}); err != nil {
		return fmt.Errorf("delete sent push reminder: %w", err)
	}
	return nil
}

// MarkReminderSkipped marks a pending or sending reminder as skipped. It is
// used by the worker when the owner revoked permission for the event type.
func (r *ReminderRepository) MarkReminderSkipped(ctx context.Context, id uuid.UUID) error {
	rows, err := r.q().MarkReminderSkipped(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		return fmt.Errorf("mark reminder skipped: %w", err)
	}
	if rows == 0 {
		return application.ErrConcurrentUpdate
	}
	return nil
}

// ListChannelPreferences returns the stored per-channel preference rows of a
// user (ADR 0030). A missing row means the (event type, channel) pair is
// allowed.
func (r *ReminderRepository) ListChannelPreferences(ctx context.Context, userID uuid.UUID) ([]domain.NotificationChannelPreference, error) {
	rows, err := r.q().ListNotificationChannelPreferences(ctx, pgconv.UUIDToPgtype(userID))
	if err != nil {
		return nil, fmt.Errorf("list notification channel preferences: %w", err)
	}
	out := make([]domain.NotificationChannelPreference, len(rows))
	for i, row := range rows {
		out[i] = domain.NotificationChannelPreference{
			EventType: domain.EventType(row.EventType),
			Channel:   domain.NotificationChannel(row.Channel),
			Allowed:   row.Allowed,
		}
	}
	return out, nil
}

// UpsertChannelPreference inserts or updates one per-channel preference row.
func (r *ReminderRepository) UpsertChannelPreference(ctx context.Context, userID uuid.UUID, pref domain.NotificationChannelPreference) error {
	if err := r.q().UpsertNotificationChannelPreference(ctx, postgres.UpsertNotificationChannelPreferenceParams{
		UserID:    pgconv.UUIDToPgtype(userID),
		EventType: postgres.NotificationEventType(pref.EventType),
		Channel:   postgres.NotificationChannel(pref.Channel),
		Allowed:   pref.Allowed,
	}); err != nil {
		return fmt.Errorf("upsert notification channel preference: %w", err)
	}
	return nil
}

// IsChannelAllowed reports whether the user permits sending reminders of the
// given event type over the given channel. A missing row means allowed.
func (r *ReminderRepository) IsChannelAllowed(ctx context.Context, userID uuid.UUID, eventType domain.EventType, channel domain.NotificationChannel) (bool, error) {
	allowed, err := r.q().IsNotificationChannelAllowed(ctx, postgres.IsNotificationChannelAllowedParams{
		UserID:    pgconv.UUIDToPgtype(userID),
		EventType: postgres.NotificationEventType(eventType),
		Channel:   postgres.NotificationChannel(channel),
	})
	if err != nil {
		return false, fmt.Errorf("check notification channel allowed: %w", err)
	}
	return allowed, nil
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
		SentAt:               pgconv.TimestamptzToPtrTime(row.SentAt),
		FailedAttempts:       int(row.FailedAttempts),
		NextAttemptAt:        pgconv.TimestamptzToPtrTime(row.NextAttemptAt),
		MessageTitle:         row.MessageTitle,
		MessageBody:          row.MessageBody,
		CreatedAt:            pgconv.TimestamptzToTime(row.CreatedAt),
		UpdatedAt:            pgconv.TimestamptzToTime(row.UpdatedAt),
	}
}

// normalizeCalendarStatus collapses 'sending' to 'pending' for the calendar
// view and returns nil for terminal/hidden statuses.
func normalizeCalendarStatus(s domain.ReminderStatus) *domain.ReminderStatus {
	switch s {
	case domain.ReminderPending, domain.ReminderSending:
		v := domain.ReminderPending
		return &v
	case domain.ReminderSent:
		v := domain.ReminderSent
		return &v
	default:
		return nil
	}
}
