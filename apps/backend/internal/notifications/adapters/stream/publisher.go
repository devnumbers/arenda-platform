// Package stream adapts the notifications context to the shared SSE
// transport (карта #734, #742; ADR 0058): it formats the delivery pipeline's
// live pushes into envelope frames and hands them to the platform hub. The
// transport is best-effort — the feed row is the system of record, the
// frames only wake the clients up.
package stream

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	notificationsjob "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/notificationsjob"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/sse"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// The stream's event names (ADR 0058): stable coarse names the browser
// dispatches to its addEventListener; adding new names is backward compatible
// by definition (a client without a listener ignores the frame).
const (
	EventNotificationCreated = "notification.created"
	EventUnreadCount         = "notification.unread_count"
)

// envelopeVersion is the envelope schema version (ADR 0058): breaking payload
// changes bump it, additive ones keep it.
const envelopeVersion = 1

// Publisher implements application.StreamPublisher over the platform hub.
type Publisher struct {
	hub *sse.Hub
	clk clock.Clock
}

// The adapter conforms to the consumer-declared port (CODING_STANDARDS).
var _ notificationsapp.StreamPublisher = (*Publisher)(nil)

// NewPublisher creates the stream publisher over the shared hub.
func NewPublisher(hub *sse.Hub, clk clock.Clock) *Publisher {
	return &Publisher{hub: hub, clk: clk}
}

// NotificationCreated pushes the "notification.created" frame — the toast
// payload: category, title, body and the event's deep link.
func (p *Publisher) NotificationCreated(ctx context.Context, n domain.Notification) {
	data, err := json.Marshal(envelope{
		V:          envelopeVersion,
		OccurredAt: p.clk.Now().UTC().Format(time.RFC3339),
		Payload: marshalPayload(createdPayload{
			ID:           n.ID.String(),
			Category:     string(n.Category),
			ContextLabel: n.ContextLabel,
			Title:        n.Title,
			Body:         n.Body,
			URL:          notificationsjob.DeepLinkFor(n),
		}),
	})
	if err != nil {
		return
	}
	p.hub.Publish(ctx, sse.Event{
		UserID: n.UserID,
		Frame:  sse.Frame{Event: EventNotificationCreated, Data: string(data)},
	})
}

// UnreadCount pushes the "notification.unread_count" frame — the badge value.
func (p *Publisher) UnreadCount(ctx context.Context, userID uuid.UUID, count int64) {
	data, err := json.Marshal(envelope{
		V:          envelopeVersion,
		OccurredAt: p.clk.Now().UTC().Format(time.RFC3339),
		Payload:    marshalPayload(unreadPayload{Count: count}),
	})
	if err != nil {
		return
	}
	p.hub.Publish(ctx, sse.Event{
		UserID: userID,
		Frame:  sse.Frame{Event: EventUnreadCount, Data: string(data)},
	})
}

// envelope is the JSON wrapper inside every frame's data line (ADR 0058):
// version, occurrence instant, per-type payload. The payload carries ids and
// display fields only — the client re-reads state through its API.
type envelope struct {
	V          int             `json:"v"`
	OccurredAt string          `json:"occurredAt"`
	Payload    json.RawMessage `json:"payload"`
}

type createdPayload struct {
	ID string `json:"id"`
	// ContextLabel is the optional line above the toast title — the feed
	// row's context snapshot (#747, toast mockup 2343:57307). Additive to
	// envelope v1: old clients ignore the extra field.
	ContextLabel string `json:"contextLabel,omitempty"`
	Category     string `json:"category"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	URL          string `json:"url,omitempty"`
}

type unreadPayload struct {
	Count int64 `json:"count"`
}

// marshalPayload marshals a payload struct of plain strings and numbers.
// Such a marshal cannot fail, but the frames are best-effort anyway: the
// fallback is an empty payload, never an error path.
func marshalPayload(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}
