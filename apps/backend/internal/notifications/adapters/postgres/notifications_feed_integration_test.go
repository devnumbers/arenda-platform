package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupFeedDB prepares the minimal test pool, a fresh user and the feed
// repository — the same harness the push subscription tests use.
func setupFeedDB(t *testing.T) (*pgxpool.Pool, *NotificationRepository, uuid.UUID) {
	t.Helper()
	pool := setupPushDB(t)
	ctx := context.Background()
	q := genpostgres.New(pool)
	userID := createPushTestUser(t, ctx, q)
	return pool, NewNotificationRepository(pool), userID
}

// feedNotification builds a valid feed row for the recipient with the given
// dedup key.
func feedNotification(t *testing.T, userID uuid.UUID, dedup string) domain.Notification {
	t.Helper()
	n, err := domain.NewNotification(
		uuid.Must(uuid.NewV7()), userID,
		domain.EventPaymentDue,
		"Оплатите платёж",
		"Платёж «Аренда» по объекту «Объект»: 2000000. Срок оплаты: 1 октября",
		"Объект",
		domain.Payload{Property: &domain.EntityRef{ID: uuid.Must(uuid.NewV7()), Name: "Объект"}},
		domain.DedupKey(dedup),
	)
	require.NoError(t, err)
	return *n
}

func TestNotificationRepository_InsertListRoundTrip(t *testing.T) {
	t.Parallel()

	_, repo, userID := setupFeedDB(t)
	ctx := context.Background()
	n := feedNotification(t, userID, "payment_due:r1:2026-10-01")

	inserted, err := repo.Insert(ctx, n)
	require.NoError(t, err)
	assert.True(t, inserted)

	page, err := repo.ListPage(ctx, userID, false, nil, uuid.Nil, 0)
	require.NoError(t, err)
	require.Len(t, page, 1)

	got := page[0]
	assert.Equal(t, n.ID, got.ID)
	assert.Equal(t, n.UserID, got.UserID)
	assert.Equal(t, domain.CategoryPaymentsOperations, got.Category, "category derived from the event type")
	assert.Equal(t, n.EventType, got.EventType)
	assert.Equal(t, n.Title, got.Title)
	assert.Equal(t, n.Body, got.Body)
	assert.Equal(t, n.ContextLabel, got.ContextLabel)
	assert.Equal(t, n.Payload, got.Payload)
	assert.Equal(t, n.DedupKey, got.DedupKey)
	assert.Nil(t, got.ReadAt)
	assert.Nil(t, got.DeletedAt)
	assert.False(t, got.CreatedAt.IsZero())

	unread, err := repo.CountUnread(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), unread)
}

func TestNotificationRepository_InsertDeduplicatesByKey(t *testing.T) {
	t.Parallel()

	_, repo, userID := setupFeedDB(t)
	ctx := context.Background()
	first := feedNotification(t, userID, "payment_due:r1:2026-10-01")

	inserted, err := repo.Insert(ctx, first)
	require.NoError(t, err)
	require.True(t, inserted)

	// The same (recipient, dedup key) publication is a no-op; the row count
	// stays at one and the stored row remains the first snapshot.
	repeat := feedNotification(t, userID, "payment_due:r1:2026-10-01")
	repeat.Title = "Другой снимок"
	inserted, err = repo.Insert(ctx, repeat)
	require.NoError(t, err)
	assert.False(t, inserted)

	page, err := repo.ListPage(ctx, userID, false, nil, uuid.Nil, 0)
	require.NoError(t, err)
	require.Len(t, page, 1)
	assert.Equal(t, first.ID, page[0].ID)
	assert.Equal(t, "Оплатите платёж", page[0].Title)

	// The key is per recipient: another user with the same entity key gets
	// their own row.
	_, otherRepo, otherUser := setupFeedDB(t)
	inserted, err = otherRepo.Insert(ctx, feedNotification(t, otherUser, "payment_due:r1:2026-10-01"))
	require.NoError(t, err)
	assert.True(t, inserted)
}

func TestNotificationRepository_ListPageKeysetWalk(t *testing.T) {
	t.Parallel()

	pool, repo, userID := setupFeedDB(t)
	ctx := context.Background()
	base := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

	// Three rows with fixed distinct created_at instants; the default
	// now() stamps would make the walk order clock-dependent.
	ids := make([]uuid.UUID, 0, 3)
	for i := range 3 {
		n := feedNotification(t, userID, fmt.Sprintf("payment_due:r%d:2026-10-01", i))
		inserted, err := repo.Insert(ctx, n)
		require.NoError(t, err)
		require.True(t, inserted)
		_, err = pool.Exec(ctx,
			"UPDATE notifications SET created_at = $2 WHERE id = $1",
			n.ID, base.Add(time.Duration(i)*time.Minute))
		require.NoError(t, err)
		ids = append(ids, n.ID)
	}

	page1, err := repo.ListPage(ctx, userID, false, nil, uuid.Nil, 2)
	require.NoError(t, err)
	require.Len(t, page1, 2)
	// Newest first: the last inserted row leads.
	assert.Equal(t, ids[2], page1[0].ID)
	assert.Equal(t, ids[1], page1[1].ID)

	cursor := page1[1].CreatedAt
	page2, err := repo.ListPage(ctx, userID, false, &cursor, page1[1].ID, 2)
	require.NoError(t, err)
	require.Len(t, page2, 1)
	assert.Equal(t, ids[0], page2[0].ID)
}

func TestNotificationRepository_UnreadFilterAndMarkRead(t *testing.T) {
	t.Parallel()

	_, repo, userID := setupFeedDB(t)
	ctx := context.Background()
	var first domain.Notification
	for i := range 3 {
		n := feedNotification(t, userID, fmt.Sprintf("payment_due:r%d:2026-10-01", i))
		inserted, err := repo.Insert(ctx, n)
		require.NoError(t, err)
		require.True(t, inserted)
		if i == 0 {
			first = n
		}
	}

	count, err := repo.CountUnread(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, int64(3), count)

	flipped, err := repo.MarkRead(ctx, userID, first.ID)
	require.NoError(t, err)
	assert.True(t, flipped)

	// Idempotent: the read row matches nothing the second time.
	flipped, err = repo.MarkRead(ctx, userID, first.ID)
	require.NoError(t, err)
	assert.False(t, flipped)

	count, err = repo.CountUnread(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	unreadOnly, err := repo.ListPage(ctx, userID, true, nil, uuid.Nil, 0)
	require.NoError(t, err)
	require.Len(t, unreadOnly, 2)
	for _, n := range unreadOnly {
		assert.NotEqual(t, first.ID, n.ID)
	}

	flippedAll, err := repo.MarkAllRead(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), flippedAll)

	count, err = repo.CountUnread(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	all, err := repo.ListPage(ctx, userID, false, nil, uuid.Nil, 0)
	require.NoError(t, err)
	require.Len(t, all, 3)
	for _, n := range all {
		assert.NotNil(t, n.ReadAt, "every row is read after «Прочитать все»")
	}
}

func TestNotificationRepository_DeletedRowsLeaveFeedAndCounter(t *testing.T) {
	t.Parallel()

	pool, repo, userID := setupFeedDB(t)
	ctx := context.Background()
	removed := feedNotification(t, userID, "payment_due:r1:2026-10-01")
	kept := feedNotification(t, userID, "payment_due:r2:2026-10-01")
	for _, n := range []domain.Notification{removed, kept} {
		inserted, err := repo.Insert(ctx, n)
		require.NoError(t, err)
		require.True(t, inserted)
	}

	// Deletion is the reading side's soft delete (#743): the row stays, the
	// feed and the unread counter drop it. Removal of an unread row drives
	// the counter down (решение #737).
	_, err := pool.Exec(ctx,
		"UPDATE notifications SET deleted_at = now() WHERE id = $1", removed.ID)
	require.NoError(t, err)

	page, err := repo.ListPage(ctx, userID, false, nil, uuid.Nil, 0)
	require.NoError(t, err)
	require.Len(t, page, 1)
	assert.Equal(t, kept.ID, page[0].ID)

	count, err := repo.CountUnread(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}
