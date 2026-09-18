package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// NotificationRepository persists and queries the stored notification feed
// (карта #734, модель — решение #737). The feed readers are the creation
// service (#740) and the feed reading API (#743).
type NotificationRepository interface {
	// WithTx binds the repository to the caller's transaction (ADR 0033):
	// the publisher inserts feed rows and enqueues deliveries in one tx.
	WithTx(tx transaction.Tx) (NotificationRepository, error)
	// Insert publishes one recipient's feed row. It reports false when a row
	// with the same (recipient, dedup key) already exists: the repeat
	// publication is a no-op, not an error.
	Insert(ctx context.Context, n domain.Notification) (bool, error)
	// GetByID returns one feed row by id, ErrNotFound when absent. The
	// delivery jobs (#740) reload the committed row instead of carrying its
	// content in job args.
	GetByID(ctx context.Context, id uuid.UUID) (domain.Notification, error)
	// ListPage walks the user's feed newest-first by the (created_at, id)
	// keyset (канон #597). Deleted rows never appear. UnreadOnly filters the
	// page to unread rows. The afterCreatedAt/afterID pair resumes strictly
	// after the previous page's last row; nil reads from the beginning, and
	// a half-set pair (timestamp without id) falls back to the beginning. A
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

// TemplateEmailSender renders a named template and sends one email through
// the platform mailer: the email delivery worker's send port (#740) — the
// worker owns subject and content, the adapter owns templates and transport.
type TemplateEmailSender interface {
	SendTemplate(ctx context.Context, to, subject, template string, data map[string]any) error
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

// DeliveryQueue schedules a stored notification's channel deliveries. The
// publisher enqueues inside its own transaction, so a delivery job commits
// together with its feed row; retries and at-least-once delivery are the
// queue's contract (ADR 0057). A channel that is not configured (push
// without VAPID keys) is a legitimate no-op implementation.
type DeliveryQueue interface {
	EnqueueEmail(ctx context.Context, tx transaction.Tx, notificationID uuid.UUID) error
	EnqueuePush(ctx context.Context, tx transaction.Tx, notificationID uuid.UUID) error
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
