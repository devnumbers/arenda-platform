package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// Publication is one event's notification content addressed to its
// recipients (карта #734): the catalog event type, the text snapshot, the
// payload links and the publication dedup key shared by every recipient's
// row. Recipients are the final list the publishing context resolved (владелец
// + участники for object events, the addressee for access events); Actor is
// the optional initiator who never receives a row for their own action
// (actor-skip, решение #737).
type Publication struct {
	EventType    domain.EventType
	DedupKey     domain.DedupKey
	Title        string
	Body         string
	ContextLabel string
	Payload      domain.Payload
	Recipients   []uuid.UUID
	Actor        uuid.UUID
}

// Publisher is the creation service of the delivery pipeline (#740): it
// fans a publication out into one feed row per recipient and enqueues the
// channel deliveries for the rows it actually created. The feed row is the
// in-app delivery — it exists the moment Publish commits.
//
// The transactional seam follows the grace-events canon (ADR 0033,
// решение #740): publishers capture the publication while their business
// transaction runs and call Publish strictly after it commits; Publish then
// runs its own transaction, where a feed row and its delivery jobs commit
// together — a job never exists without its row and vice versa. Dedup is
// durable: a repeat publication with the same (recipient, dedup key) inserts
// nothing and enqueues nothing. Delivery is at-least-once (ADR 0057): the
// queue may redeliver in an crash window, a duplicate message beats a lost
// one.
type Publisher struct {
	feed   NotificationRepository
	queue  DeliveryQueue
	stream StreamPublisher
	uow    transaction.UoW
	log    *slog.Logger
}

// NewPublisher creates the notification publication service. The stream port
// is the optional SSE transport hook (nil disables live pushes — tests, or a
// wiring without the stream); a nil logger defaults to the standard one.
func NewPublisher(
	feed NotificationRepository,
	queue DeliveryQueue,
	stream StreamPublisher,
	uow transaction.UoW,
	log *slog.Logger,
) *Publisher {
	if log == nil {
		log = slog.Default()
	}
	return &Publisher{feed: feed, queue: queue, stream: stream, uow: uow, log: log}
}

// Publish fans the publication out to every recipient. An empty recipient
// list (the actor-skip filtered everyone out) is a no-op. The error is
// returned to the caller — the publisher decides whether a failed
// publication can fail its flow; the post-commit wrapper at the call site
// swallows it per the grace canon.
func (p *Publisher) Publish(ctx context.Context, pub Publication) error {
	rows, err := p.buildRows(pub)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	var created []*domain.Notification
	err = p.uow.Do(ctx, func(tx transaction.Tx) error {
		feed, err := p.feed.WithTx(tx)
		if err != nil {
			return fmt.Errorf("bind notification repository: %w", err)
		}
		for _, n := range rows {
			inserted, err := feed.Insert(ctx, *n)
			if err != nil {
				return err
			}
			if !inserted {
				// The (recipient, dedup key) row already exists: the repeat
				// publication must not re-deliver the channels either.
				continue
			}
			created = append(created, n)
			if err := p.queue.EnqueueEmail(ctx, tx, n.ID); err != nil {
				return fmt.Errorf("enqueue email delivery %s: %w", n.ID, err)
			}
			if err := p.queue.EnqueuePush(ctx, tx, n.ID); err != nil {
				return fmt.Errorf("enqueue push delivery %s: %w", n.ID, err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Strictly after the commit: the frames must not run ahead of the rows
	// they announce, and a rollback streams nothing.
	p.streamCreated(ctx, created)
	return nil
}

// streamCreated pushes the live frames for the rows the publication created.
// The SSE transport is best-effort (ADR 0058): a failed unread count drops
// only the badge frame — the created frame never depends on it — and nothing
// here can fail the publication.
func (p *Publisher) streamCreated(ctx context.Context, created []*domain.Notification) {
	if p.stream == nil || len(created) == 0 {
		return
	}
	for _, n := range created {
		p.stream.NotificationCreated(ctx, *n)
		count, err := p.feed.CountUnread(ctx, n.UserID)
		if err != nil {
			p.log.WarnContext(ctx, "unread count for the stream failed",
				slog.String("user_id", n.UserID.String()), slog.String("error", err.Error()))
			continue
		}
		p.stream.UnreadCount(ctx, n.UserID, count)
	}
}

// buildRows validates the publication against the catalog and builds one
// row per recipient, dropping the actor and any duplicates. The per-row id
// is app-generated here: callers name recipients and content, not database
// identities.
func (p *Publisher) buildRows(pub Publication) ([]*domain.Notification, error) {
	recipients := make([]uuid.UUID, 0, len(pub.Recipients))
	seen := make(map[uuid.UUID]bool, len(pub.Recipients))
	for _, r := range pub.Recipients {
		if r == pub.Actor || r == uuid.Nil || seen[r] {
			continue
		}
		seen[r] = true
		recipients = append(recipients, r)
	}

	rows := make([]*domain.Notification, 0, len(recipients))
	for _, recipient := range recipients {
		id, err := uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("generate notification id: %w", err)
		}
		// NewNotification validates the id/recipient pair, maps the event
		// type through the catalog and rejects blank texts: an error here is
		// the publisher's bug, reported as is.
		n, err := domain.NewNotification(
			id, recipient,
			pub.EventType,
			pub.Title, pub.Body, pub.ContextLabel,
			pub.Payload,
			pub.DedupKey,
		)
		if err != nil {
			return nil, err
		}
		rows = append(rows, n)
	}
	return rows, nil
}
