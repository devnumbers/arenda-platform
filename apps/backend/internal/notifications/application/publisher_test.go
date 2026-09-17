package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeUoW reproduces the production UoW semantics (ADR 0033): begin, run
// work, commit on nil error, roll back otherwise. The commit/rollback facts
// are what the publisher tests assert, so the fixture records them.
type fakeUoW struct {
	committed bool
	rolled    bool
	workErr   error // When set, work fails with it.
}

func (u *fakeUoW) Do(ctx context.Context, work func(tx transaction.Tx) error) error {
	if u.workErr != nil {
		u.rolled = true
		return u.workErr
	}
	if err := work(&fakeTx{}); err != nil {
		u.rolled = true
		return err
	}
	u.committed = true
	return nil
}

// fakeTx is the transaction handle the fake UoW hands out.
type fakeTx struct{ transaction.Tx }

// fakeFeedRepo counts inserts and reports which (user, dedup key) pairs are
// already published.
type fakeFeedRepo struct {
	inserted []domain.Notification
	// Existing holds "user_id|dedup_key" pairs Insert reports as duplicates.
	existing map[string]bool

	withTxErr error
	lastTx    transaction.Tx
}

func (r *fakeFeedRepo) WithTx(tx transaction.Tx) (NotificationRepository, error) {
	if r.withTxErr != nil {
		return nil, r.withTxErr
	}
	r.lastTx = tx
	return r, nil
}

func (r *fakeFeedRepo) Insert(ctx context.Context, n domain.Notification) (bool, error) {
	if r.existing[n.UserID.String()+"|"+string(n.DedupKey)] {
		return false, nil
	}
	r.inserted = append(r.inserted, n)
	return true, nil
}

// Stubs for the read-side methods the publisher never calls; the interface
// is consumed whole, the publisher uses WithTx/Insert only.
func (r *fakeFeedRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Notification, error) {
	return domain.Notification{}, ErrNotFound
}

func (r *fakeFeedRepo) ListPage(
	ctx context.Context, userID uuid.UUID, unreadOnly bool,
	afterCreatedAt *time.Time, afterID uuid.UUID, limit int,
) ([]domain.Notification, error) {
	return nil, ErrNotFound
}

func (r *fakeFeedRepo) CountUnread(ctx context.Context, userID uuid.UUID) (int64, error) {
	return 0, ErrNotFound
}

func (r *fakeFeedRepo) MarkRead(ctx context.Context, userID, id uuid.UUID) (bool, error) {
	return false, ErrNotFound
}

func (r *fakeFeedRepo) MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error) {
	return 0, ErrNotFound
}

// fakeQueue records the enqueue calls per channel.
type fakeQueue struct {
	emails   []uuid.UUID
	pushes   []uuid.UUID
	emailErr error
	pushErr  error
	lastTx   transaction.Tx
}

func (q *fakeQueue) EnqueueEmail(ctx context.Context, tx transaction.Tx, notificationID uuid.UUID) error {
	if q.emailErr != nil {
		return q.emailErr
	}
	q.lastTx = tx
	q.emails = append(q.emails, notificationID)
	return nil
}

func (q *fakeQueue) EnqueuePush(ctx context.Context, tx transaction.Tx, notificationID uuid.UUID) error {
	if q.pushErr != nil {
		return q.pushErr
	}
	q.pushes = append(q.pushes, notificationID)
	return nil
}

func publication() Publication {
	return Publication{
		EventType:  domain.EventPaymentDue,
		DedupKey:   domain.DedupKey("payment_due:prop-1:2026-10-01"),
		Title:      "Оплатите платёж",
		Body:       "Платёж по объекту «Объект»: 2000000. Срок оплаты: 1 октября",
		Recipients: []uuid.UUID{uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())},
	}
}

func TestPublisherPublishFansOutRowPerRecipient(t *testing.T) {
	t.Parallel()

	feed := &fakeFeedRepo{}
	queue := &fakeQueue{}
	p := NewPublisher(feed, queue, &fakeUoW{}, nil)
	pub := publication()

	require.NoError(t, p.Publish(context.Background(), pub))

	require.Len(t, feed.inserted, 2)
	require.Len(t, queue.emails, 2)
	require.Len(t, queue.pushes, 2)

	seen := map[uuid.UUID]bool{}
	for i, n := range feed.inserted {
		assert.False(t, seen[n.UserID], "one row per recipient, no duplicates")
		seen[n.UserID] = true
		assert.Contains(t, pub.Recipients, n.UserID)
		assert.Equal(t, pub.DedupKey, n.DedupKey)
		assert.Equal(t, pub.Title, n.Title)
		assert.Equal(t, domain.CategoryPaymentsOperations, n.Category, "category derived from the event type")
		assert.NotEqual(t, uuid.Nil, n.ID, "row id is app-generated")
		// Both channel jobs travel with exactly this row's id.
		assert.Equal(t, n.ID, queue.emails[i])
		assert.Equal(t, n.ID, queue.pushes[i])
	}
}

func TestPublisherPublishSkipsActor(t *testing.T) {
	t.Parallel()

	feed := &fakeFeedRepo{}
	queue := &fakeQueue{}
	p := NewPublisher(feed, queue, &fakeUoW{}, nil)

	actor := uuid.Must(uuid.NewV7())
	pub := publication()
	pub.Recipients = append(pub.Recipients, actor)
	pub.Actor = actor

	require.NoError(t, p.Publish(context.Background(), pub))

	for _, n := range feed.inserted {
		assert.NotEqual(t, actor, n.UserID, "the actor never receives a row for their own action")
	}
	assert.Len(t, feed.inserted, 2)
}

func TestPublisherPublishSkipsEnqueueForDedupedRows(t *testing.T) {
	t.Parallel()

	feed := &fakeFeedRepo{}
	queue := &fakeQueue{}
	p := NewPublisher(feed, queue, &fakeUoW{}, nil)

	pub := publication()
	recipient := pub.Recipients[1]
	feed.existing = map[string]bool{recipient.String() + "|" + string(pub.DedupKey): true}

	require.NoError(t, p.Publish(context.Background(), pub))

	// Only the fresh row is delivered: a repeat publication never re-enqueues
	// the channels for a feed row that already exists.
	require.Len(t, feed.inserted, 1)
	assert.Equal(t, pub.Recipients[0], feed.inserted[0].UserID)
	require.Len(t, queue.emails, 1)
	require.Len(t, queue.pushes, 1)
}

func TestPublisherPublishRunsInOneTransaction(t *testing.T) {
	t.Parallel()

	feed := &fakeFeedRepo{}
	queue := &fakeQueue{}
	uow := &fakeUoW{}
	p := NewPublisher(feed, queue, uow, nil)

	require.NoError(t, p.Publish(context.Background(), publication()))

	assert.True(t, uow.committed, "rows and jobs commit together")
	// The feed inserts and the enqueue calls travel on the same transaction:
	// a job can never exist without its feed row and vice versa.
	assert.NotNil(t, feed.lastTx)
	assert.Same(t, feed.lastTx, queue.lastTx)
}

func TestPublisherPublishRejectsOffCatalogEvent(t *testing.T) {
	t.Parallel()

	feed := &fakeFeedRepo{}
	queue := &fakeQueue{}
	p := NewPublisher(feed, queue, &fakeUoW{}, nil)

	pub := publication()
	// The direct-channel legacy value, outside the feed catalog.
	pub.EventType = domain.EventSubscriptionGrace

	err := p.Publish(context.Background(), pub)
	require.ErrorIs(t, err, domain.ErrInvalidNotification)
	assert.Empty(t, feed.inserted, "nothing is written for an invalid publication")
}

func TestPublisherPublishRejectsBlankTexts(t *testing.T) {
	t.Parallel()

	feed := &fakeFeedRepo{}
	queue := &fakeQueue{}
	p := NewPublisher(feed, queue, &fakeUoW{}, nil)

	pub := publication()
	pub.Title = ""

	require.ErrorIs(t, p.Publish(context.Background(), pub), domain.ErrInvalidNotification)
	assert.Empty(t, feed.inserted)
}

func TestPublisherPublishPropagatesQueueFailureAndRollsBack(t *testing.T) {
	t.Parallel()

	feed := &fakeFeedRepo{}
	queue := &fakeQueue{pushErr: errors.New("river down")}
	uow := &fakeUoW{}
	p := NewPublisher(feed, queue, uow, nil)

	err := p.Publish(context.Background(), publication())
	require.ErrorContains(t, err, "enqueue push delivery")

	assert.True(t, uow.rolled, "a failed enqueue rolls the whole publication back")
}

func TestPublisherPublishWithoutRecipientsIsNoop(t *testing.T) {
	t.Parallel()

	feed := &fakeFeedRepo{}
	queue := &fakeQueue{}
	p := NewPublisher(feed, queue, &fakeUoW{}, nil)

	pub := publication()
	pub.Recipients = nil

	require.NoError(t, p.Publish(context.Background(), pub))
	assert.Empty(t, feed.inserted)
	assert.Empty(t, queue.emails)
}
