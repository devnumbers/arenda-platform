package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// PreferenceRepository persists and queries the per-channel notification
// preferences (ADR 0030).
type PreferenceRepository interface {
	// ListChannelPreferences returns the stored per-channel preference rows of
	// a user. A missing row means the (event type, channel) pair is allowed.
	ListChannelPreferences(ctx context.Context, userID uuid.UUID) ([]domain.NotificationChannelPreference, error)
	// UpsertChannelPreference inserts or updates one per-channel preference row.
	UpsertChannelPreference(ctx context.Context, userID uuid.UUID, pref domain.NotificationChannelPreference) error
	// IsChannelAllowed reports whether the user permits sending notifications
	// of the given event type over the given channel. A missing row means
	// allowed.
	IsChannelAllowed(ctx context.Context, userID uuid.UUID, eventType domain.EventType, channel domain.NotificationChannel) (bool, error)
	WithTx(tx transaction.Tx) PreferenceRepository
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
