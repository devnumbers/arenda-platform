package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// NotificationRepository persists and queries the stored notification feed
// (карта #734, модель — решение #737). The consumers are the creation
// service (#740) and the feed reading API (#743); transactional binding
// arrives with the creation service's transaction seam.
type NotificationRepository interface {
	// Insert publishes one recipient's feed row. It reports false when a row
	// with the same (recipient, dedup key) already exists: the repeat
	// publication is a no-op, not an error.
	Insert(ctx context.Context, n domain.Notification) (bool, error)
	// ListPage walks the user's feed newest-first by the (created_at, id)
	// keyset (канон #597). Deleted rows never appear. UnreadOnly filters the
	// page to unread rows. The afterCreatedAt/afterID pair resumes strictly
	// after the previous page's last row; nil reads from the beginning. A
	// zero limit removes the page size.
	ListPage(
		ctx context.Context,
		userID uuid.UUID,
		unreadOnly bool,
		afterCreatedAt *time.Time,
		afterID uuid.UUID,
		limit int,
	) ([]domain.Notification, error)
	// CountUnread counts the user's unread, not-deleted rows.
	CountUnread(ctx context.Context, userID uuid.UUID) (int64, error)
	// MarkRead marks one row read. It reports false when nothing matched:
	// the row is already read, deleted, or not the user's.
	MarkRead(ctx context.Context, userID, id uuid.UUID) (bool, error)
	// MarkAllRead marks every unread not-deleted row of the user read and
	// returns how many rows flipped.
	MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error)
}

// PushSubscriptionRepository persists Web Push subscriptions keyed by their
// browser-issued endpoint URL.
type PushSubscriptionRepository interface {
	// Upsert inserts a subscription keyed by endpoint, or updates its mutable
	// fields when the endpoint already exists (idempotent re-subscribe).
	Upsert(ctx context.Context, sub domain.PushSubscription) (domain.PushSubscription, error)
	// Delete removes a subscription by endpoint scoped to a user. It returns
	// ErrNotFound when no row matched.
	Delete(ctx context.Context, userID uuid.UUID, endpoint string) error
	// ListByUser returns all stored subscriptions for a user.
	ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.PushSubscription, error)
}

// DirectEmailSender renders and sends a one-off email outside any worker
// lifecycle (issue #253): the direct-notification service owns subject and
// content, the adapter owns templates and transport.
type DirectEmailSender interface {
	SendDirect(ctx context.Context, to, subject, template string, data map[string]any) error
}

// PushSender dispatches a single Web Push message to one browser subscription.
// It encrypts the payload (RFC 8291) and sends it to the push service endpoint
// identified by the subscription. Domain errors signal the outcome:
// ErrSubscriptionGone (404/410) — the subscription is dead and must be deleted;
// ErrRateLimited (429) — the push service throttled the request; a non-nil
// error otherwise means the send failed (the caller may retry on 5xx).
type PushSender interface {
	Send(ctx context.Context, subscription domain.PushSubscription, payload PushPayload) error
}

// ContactResolver resolves the delivery channel and address for an owner.
type ContactResolver interface {
	Resolve(ctx context.Context, scope uuid.UUID) (Contact, error)
}

// Contact is a resolved delivery endpoint.
type Contact struct {
	Channel Channel
	Email   string
}

// Channel identifies a delivery channel.
type Channel string

const (
	ChannelEmail Channel = "email"
	ChannelPush  Channel = "push"
)
