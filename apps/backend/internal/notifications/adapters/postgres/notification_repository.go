// Package postgres holds the notifications persistence adapters: the stored
// feed and push-subscription repositories and the owner contact resolver.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// The adapter satisfies the consumer-declared port (CODING_STANDARDS).
var _ application.NotificationRepository = (*NotificationRepository)(nil)

// NotificationRepository persists the stored notification feed (карта #734,
// модель — решение #737) using generated sqlc queries.
type NotificationRepository struct {
	db postgres.DBTX
}

// NewNotificationRepository creates a new notification feed repository.
func NewNotificationRepository(db postgres.DBTX) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *NotificationRepository) WithTx(tx transaction.Tx) (application.NotificationRepository, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("notifications.NotificationRepository.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewNotificationRepository(dbtx), nil
}

// GetByID returns one feed row by id; ErrNotFound when absent.
func (r *NotificationRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Notification, error) {
	row, err := r.q().GetNotification(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Notification{}, application.ErrNotFound
		}
		return domain.Notification{}, fmt.Errorf("get notification: %w", err)
	}
	return notificationToDomain(row)
}

// Insert publishes one recipient's feed row; a repeat publication with the
// same (recipient, dedup key) inserts nothing and reports false.
func (r *NotificationRepository) Insert(ctx context.Context, n domain.Notification) (bool, error) {
	payload, err := json.Marshal(n.Payload)
	if err != nil {
		return false, fmt.Errorf("marshal notification payload: %w", err)
	}
	rows, err := r.q().InsertNotification(ctx, postgres.InsertNotificationParams{
		ID:           pgconv.UUIDToPgtype(n.ID),
		UserID:       pgconv.UUIDToPgtype(n.UserID),
		Category:     postgres.NotificationCategory(n.Category),
		EventType:    postgres.NotificationEventType(n.EventType),
		Title:        n.Title,
		Body:         n.Body,
		ContextLabel: stringToPgtypeText(n.ContextLabel),
		Payload:      payload,
		DedupKey:     string(n.DedupKey),
	})
	if err != nil {
		return false, fmt.Errorf("insert notification: %w", err)
	}
	return rows > 0, nil
}

// ListPage walks the user's feed newest-first by the (created_at, id) keyset.
func (r *NotificationRepository) ListPage(
	ctx context.Context,
	userID uuid.UUID,
	unreadOnly bool,
	afterCreatedAt *time.Time,
	afterID uuid.UUID,
	limit int,
) ([]domain.Notification, error) {
	// The cursor pair travels together: a half-set pair (timestamp without
	// id) falls back to the beginning, so a lost id can never silently empty
	// the page — the SQL checks only the timestamp for NULL.
	if afterID == uuid.Nil {
		afterCreatedAt = nil
	}
	rows, err := r.q().ListNotifications(ctx, postgres.ListNotificationsParams{
		UserID:         pgconv.UUIDToPgtype(userID),
		UnreadOnly:     unreadOnly,
		AfterCreatedAt: pgconv.TimePtrToPgtype(afterCreatedAt),
		AfterID:        pgconv.UUIDToPgtype(afterID),
		PageLimit:      int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	feed := make([]domain.Notification, 0, len(rows))
	for _, row := range rows {
		n, err := notificationToDomain(row)
		if err != nil {
			return nil, err
		}
		feed = append(feed, n)
	}
	return feed, nil
}

// CountUnread counts the user's unread, not-deleted rows.
func (r *NotificationRepository) CountUnread(ctx context.Context, userID uuid.UUID) (int64, error) {
	count, err := r.q().CountUnreadNotifications(ctx, pgconv.UUIDToPgtype(userID))
	if err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return count, nil
}

// MarkRead marks one row read; false means nothing matched.
func (r *NotificationRepository) MarkRead(ctx context.Context, userID, id uuid.UUID) (bool, error) {
	rows, err := r.q().MarkNotificationRead(ctx, postgres.MarkNotificationReadParams{
		ID:     pgconv.UUIDToPgtype(id),
		UserID: pgconv.UUIDToPgtype(userID),
	})
	if err != nil {
		return false, fmt.Errorf("mark notification read: %w", err)
	}
	return rows > 0, nil
}

// MarkAllRead marks every unread not-deleted row of the user read.
func (r *NotificationRepository) MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error) {
	rows, err := r.q().MarkAllNotificationsRead(ctx, pgconv.UUIDToPgtype(userID))
	if err != nil {
		return 0, fmt.Errorf("mark all notifications read: %w", err)
	}
	return rows, nil
}

// Delete soft-deletes one row; false means nothing matched (deleted, gone,
// or not the user's).
func (r *NotificationRepository) Delete(ctx context.Context, userID, id uuid.UUID) (bool, error) {
	rows, err := r.q().DeleteNotification(ctx, postgres.DeleteNotificationParams{
		ID:     pgconv.UUIDToPgtype(id),
		UserID: pgconv.UUIDToPgtype(userID),
	})
	if err != nil {
		return false, fmt.Errorf("delete notification: %w", err)
	}
	return rows > 0, nil
}

// DeleteAll soft-deletes every not-deleted row of the user.
func (r *NotificationRepository) DeleteAll(ctx context.Context, userID uuid.UUID) (int64, error) {
	rows, err := r.q().DeleteAllNotifications(ctx, pgconv.UUIDToPgtype(userID))
	if err != nil {
		return 0, fmt.Errorf("delete all notifications: %w", err)
	}
	return rows, nil
}

func notificationToDomain(row postgres.Notification) (domain.Notification, error) {
	var payload domain.Payload
	if len(row.Payload) > 0 {
		if err := json.Unmarshal(row.Payload, &payload); err != nil {
			return domain.Notification{}, fmt.Errorf("unmarshal notification payload %s: %w", row.ID.String(), err)
		}
	}
	return domain.Notification{
		ID:           pgconv.UUIDFromPgtype(row.ID),
		UserID:       pgconv.UUIDFromPgtype(row.UserID),
		Category:     domain.Category(row.Category),
		EventType:    domain.EventType(row.EventType),
		Title:        row.Title,
		Body:         row.Body,
		ContextLabel: pgconv.TextToString(row.ContextLabel),
		Payload:      payload,
		DedupKey:     domain.DedupKey(row.DedupKey),
		ReadAt:       pgconv.TimestamptzToPtrTime(row.ReadAt),
		DeletedAt:    pgconv.TimestamptzToPtrTime(row.DeletedAt),
		CreatedAt:    pgconv.TimestamptzToTime(row.CreatedAt),
	}, nil
}

// stringToPgtypeText maps the empty context label to SQL NULL.
func stringToPgtypeText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}
