package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newGracePublisher wires a GracePublisher over the real pipeline Publisher
// with the publisher_test fakes: the grace publisher's job is to build the
// right Publication — texts, dedup key, recipient — and let the pipeline
// fan it out.
func newGracePublisher(feed *fakeFeedRepo, queue *fakeQueue) *GracePublisher {
	return NewGracePublisher(NewPublisher(feed, queue, nil, &fakeUoW{}, nil))
}

// graceUser returns fresh ids for the recipient (the owner, the Тариф
// recipient slot) and the subscription that identifies the window.
func graceUser() (userID, subscriptionID uuid.UUID) {
	return uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
}

func TestGracePublisherNotifyGraceEntered(t *testing.T) {
	t.Parallel()

	feed := &fakeFeedRepo{}
	queue := &fakeQueue{}
	g := newGracePublisher(feed, queue)
	user, sub := graceUser()
	until := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

	require.NoError(t, g.NotifyGraceEntered(context.Background(), user, sub, until))

	require.Len(t, feed.inserted, 1)
	n := feed.inserted[0]
	assert.Equal(t, user, n.UserID, "the owner is the single recipient")
	assert.Equal(t, domain.EventSubscriptionGraceEntered, n.EventType)
	assert.Equal(t, domain.CategoryTariff, n.Category, "category derived from the catalog")
	assert.Equal(t, "Не удалось списание за подписку", n.Title)
	assert.Equal(t,
		"Мы не смогли списать оплату за подписку. Привяжите другую карту, чтобы тариф не прервался. Льготный период действует до 30 сентября.",
		n.Body, "the legacy direct-channel copy, deadline included")
	assert.Empty(t, n.ContextLabel)
	assert.Equal(t,
		domain.DedupKey("subscription_grace_entered:"+sub.String()+":2026-09-30"),
		n.DedupKey, "the window identity (subscription, until date) is the dedup entity")

	// The feed row carries both channel deliveries, the pipeline canon.
	require.Len(t, queue.emails, 1)
	require.Len(t, queue.pushes, 1)
	assert.Equal(t, n.ID, queue.emails[0])
	assert.Equal(t, n.ID, queue.pushes[0])

	// The key's date is taken in UTC — it must not depend on the instant's
	// own location: this 02:00 MSK instant is 2026-09-30T23:00Z, the copy
	// says "1 октября" while the key keeps 2026-09-30.
	msk := time.FixedZone("MSK", 3*60*60)
	otherUser, otherSub := graceUser()
	require.NoError(t, g.NotifyGraceEntered(context.Background(), otherUser, otherSub,
		time.Date(2026, 10, 1, 2, 0, 0, 0, msk)))
	assert.Equal(t,
		domain.DedupKey("subscription_grace_entered:"+otherSub.String()+":2026-09-30"),
		feed.inserted[1].DedupKey)
	assert.Equal(t,
		"Мы не смогли списать оплату за подписку. Привяжите другую карту, чтобы тариф не прервался. Льготный период действует до 1 октября.",
		feed.inserted[1].Body)
}

func TestGracePublisherNotifyGraceEnteredWithoutDeadline(t *testing.T) {
	t.Parallel()

	feed := &fakeFeedRepo{}
	queue := &fakeQueue{}
	g := newGracePublisher(feed, queue)
	user, sub := graceUser()

	require.NoError(t, g.NotifyGraceEntered(context.Background(), user, sub, time.Time{}))

	n := feed.inserted[0]
	assert.Equal(t, "Мы не смогли списать оплату за подписку. Привяжите другую карту, чтобы тариф не прервался.", n.Body,
		"no deadline tail without a known window end")
	assert.Equal(t, domain.DedupKey("subscription_grace_entered:"+sub.String()), n.DedupKey,
		"the key drops the date part it does not have")
}

func TestGracePublisherNotifyGraceExpiring(t *testing.T) {
	t.Parallel()

	feed := &fakeFeedRepo{}
	queue := &fakeQueue{}
	g := newGracePublisher(feed, queue)
	user, sub := graceUser()
	until := time.Date(2026, 10, 1, 0, 30, 0, 0, time.UTC)

	require.NoError(t, g.NotifyGraceExpiring(context.Background(), user, sub, until))

	n := feed.inserted[0]
	assert.Equal(t, user, n.UserID)
	assert.Equal(t, domain.EventSubscriptionGraceExpiring, n.EventType)
	assert.Equal(t, domain.CategoryTariff, n.Category)
	assert.Equal(t, "Подписка скоро истекает", n.Title)
	assert.Equal(t,
		"Льготный период подписки заканчивается — обновите способ оплаты, чтобы сохранить тариф. Он действует до 1 октября.",
		n.Body, "the legacy direct-channel copy, deadline included")
	assert.Equal(t,
		domain.DedupKey("subscription_grace_expiring:"+sub.String()+":2026-10-01"),
		n.DedupKey)
}

func TestGracePublisherRepeatWindowIsDeduped(t *testing.T) {
	t.Parallel()

	feed := &fakeFeedRepo{}
	queue := &fakeQueue{}
	g := newGracePublisher(feed, queue)
	user, sub := graceUser()
	until := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

	require.NoError(t, g.NotifyGraceEntered(context.Background(), user, sub, until))
	require.NoError(t, g.NotifyGraceEntered(context.Background(), user, sub, until))

	assert.Len(t, feed.inserted, 1, "the same window publishes one row")
	assert.Len(t, queue.emails, 1, "and re-delivers no channels")
}

func TestGracePublisherNewWindowAfterOldPublishesAgain(t *testing.T) {
	t.Parallel()

	feed := &fakeFeedRepo{}
	queue := &fakeQueue{}
	g := newGracePublisher(feed, queue)
	user, sub := graceUser()

	require.NoError(t, g.NotifyGraceEntered(context.Background(), user, sub,
		time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)))
	require.NoError(t, g.NotifyGraceEntered(context.Background(), user, sub,
		time.Date(2026, 12, 15, 9, 0, 0, 0, time.UTC)))

	assert.Len(t, feed.inserted, 2, "a new grace window is a new notification")
}

func TestGracePublisherPropagatesPipelineFailure(t *testing.T) {
	t.Parallel()

	feed := &fakeFeedRepo{}
	queue := &fakeQueue{emailErr: errors.New("river down")}
	uow := &fakeUoW{}
	g := NewGracePublisher(NewPublisher(feed, queue, nil, uow, nil))
	user, sub := graceUser()

	err := g.NotifyGraceEntered(context.Background(), user, sub, time.Time{})
	require.ErrorContains(t, err, "enqueue email delivery")
	assert.True(t, uow.rolled, "the failed publication rolls back — nothing commits")
}
