package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeFeedQuery is an in-memory NotificationRepository for the feed reading
// tests: rows are addressed by id and filtered by the user the way the SQL
// does.
type fakeFeedQuery struct {
	rows map[uuid.UUID]domain.Notification
	// Deleted collects the (user, id) pairs of soft-deletes and the per-user
	// delete-all row counts, for the tests to assert against.
	deleted   [][2]uuid.UUID
	deleteAll map[uuid.UUID]int64
}

func newFakeFeedQuery() *fakeFeedQuery {
	return &fakeFeedQuery{rows: make(map[uuid.UUID]domain.Notification), deleteAll: make(map[uuid.UUID]int64)}
}

func (r *fakeFeedQuery) WithTx(tx transaction.Tx) (NotificationRepository, error) { return r, nil }

func (r *fakeFeedQuery) Insert(_ context.Context, n domain.Notification) (bool, error) {
	if _, exists := r.rows[n.ID]; exists {
		return false, nil
	}
	r.rows[n.ID] = n
	return true, nil
}

func (r *fakeFeedQuery) GetByID(_ context.Context, id uuid.UUID) (domain.Notification, error) {
	n, ok := r.rows[id]
	if !ok {
		return domain.Notification{}, ErrNotFound
	}
	return n, nil
}

// GetForUser mirrors the SQL scope: a foreign or deleted row does not exist
// for the reader.
func (r *fakeFeedQuery) GetForUser(_ context.Context, userID, id uuid.UUID) (domain.Notification, error) {
	n, ok := r.rows[id]
	if !ok || n.UserID != userID || n.DeletedAt != nil {
		return domain.Notification{}, ErrNotFound
	}
	return n, nil
}

// ListPage walks the rows newest-first by (created_at, id), skipping deleted
// ones — the fake mirrors the keyset contract the SQL implements.
func (r *fakeFeedQuery) ListPage(
	_ context.Context, userID uuid.UUID, unreadOnly bool, afterCreatedAt *time.Time, afterID uuid.UUID, limit int,
) ([]domain.Notification, error) {
	var out []domain.Notification
	for _, n := range r.rows {
		if n.UserID != userID || n.DeletedAt != nil {
			continue
		}
		if unreadOnly && n.ReadAt != nil {
			continue
		}
		if afterID != uuid.Nil {
			older := n.CreatedAt.Before(*afterCreatedAt) ||
				(n.CreatedAt.Equal(*afterCreatedAt) && n.ID.String() < afterID.String())
			if !older {
				continue
			}
		}
		out = append(out, n)
	}
	// Newest-first.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].CreatedAt.After(out[j-1].CreatedAt); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *fakeFeedQuery) CountUnread(_ context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	for _, n := range r.rows {
		if n.UserID == userID && n.ReadAt == nil && n.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

func (r *fakeFeedQuery) MarkRead(_ context.Context, userID, id uuid.UUID) (bool, error) {
	n, ok := r.rows[id]
	if !ok || n.UserID != userID || n.ReadAt != nil || n.DeletedAt != nil {
		return false, nil
	}
	now := time.Now()
	n.ReadAt = &now
	r.rows[id] = n
	return true, nil
}

func (r *fakeFeedQuery) MarkAllRead(_ context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	for id, n := range r.rows {
		if n.UserID == userID && n.ReadAt == nil && n.DeletedAt == nil {
			now := time.Now()
			n.ReadAt = &now
			r.rows[id] = n
			count++
		}
	}
	return count, nil
}

func (r *fakeFeedQuery) Delete(_ context.Context, userID, id uuid.UUID) (bool, error) {
	n, ok := r.rows[id]
	if !ok || n.UserID != userID || n.DeletedAt != nil {
		return false, nil
	}
	now := time.Now()
	n.DeletedAt = &now
	r.rows[id] = n
	r.deleted = append(r.deleted, [2]uuid.UUID{userID, id})
	return true, nil
}

func (r *fakeFeedQuery) DeleteAll(_ context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	for id, n := range r.rows {
		if n.UserID == userID && n.DeletedAt == nil {
			now := time.Now()
			n.DeletedAt = &now
			r.rows[id] = n
			count++
		}
	}
	r.deleteAll[userID] = count
	return count, nil
}

// storedFeed seeds the fake with n rows for the user, created at one-minute
// steps from the base instant.
func storedFeed(t *testing.T, repo *fakeFeedQuery, user uuid.UUID, n int, base time.Time) []domain.Notification {
	t.Helper()
	rows := make([]domain.Notification, 0, n)
	for i := range n {
		created := base.Add(time.Duration(i) * time.Minute)
		row, err := domain.NewNotification(
			uuid.Must(uuid.NewV7()), user,
			domain.EventPaymentDue,
			"Оплатите платёж",
			"Платёж по объекту «Объект»: 2000000. Срок оплаты: 1 октября",
			"Объект",
			domain.Payload{},
			domain.DedupKey("payment_due:r"+uuid.Must(uuid.NewV7()).String()+":"+created.Format(time.RFC3339)),
		)
		require.NoError(t, err)
		row.CreatedAt = created
		inserted, err := repo.Insert(context.Background(), *row)
		require.NoError(t, err)
		require.True(t, inserted)
		rows = append(rows, *row)
	}
	return rows
}

var feedTestUser = uuid.Must(uuid.NewV7())

// The page walks newest-first and hands back the cursor of the last row; the
// next page resumes strictly after it — no duplicates, no gaps.
func TestFeedService_PageWalksByKeysetCursor(t *testing.T) {
	t.Parallel()

	repo := newFakeFeedQuery()
	svc := NewFeedService(repo, nil)
	base := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	rows := storedFeed(t, repo, feedTestUser, 5, base)

	first, err := svc.Page(context.Background(), feedTestUser, FeedPageParams{Limit: 2})
	require.NoError(t, err)
	require.Len(t, first.Items, 2)
	assert.Equal(t, rows[4].ID, first.Items[0].ID, "newest first")
	assert.Equal(t, rows[3].ID, first.Items[1].ID)
	assert.NotEmpty(t, first.NextCursor)

	second, err := svc.Page(context.Background(), feedTestUser, FeedPageParams{Limit: 2, Cursor: first.NextCursor})
	require.NoError(t, err)
	require.Len(t, second.Items, 2)
	assert.Equal(t, rows[2].ID, second.Items[0].ID)
	assert.Equal(t, rows[1].ID, second.Items[1].ID)
	assert.NotEmpty(t, second.NextCursor)

	third, err := svc.Page(context.Background(), feedTestUser, FeedPageParams{Limit: 2, Cursor: second.NextCursor})
	require.NoError(t, err)
	require.Len(t, third.Items, 1)
	assert.Equal(t, rows[0].ID, third.Items[0].ID)
	assert.Empty(t, third.NextCursor, "the feed's end has no next page")
}

// Rows created between the two loads do not duplicate or drop: the cursor
// resumes strictly after the previous page's last row.
func TestFeedService_PageCursorSurvivesNewRows(t *testing.T) {
	t.Parallel()

	repo := newFakeFeedQuery()
	svc := NewFeedService(repo, nil)
	base := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	rows := storedFeed(t, repo, feedTestUser, 2, base)

	first, err := svc.Page(context.Background(), feedTestUser, FeedPageParams{Limit: 1})
	require.NoError(t, err)
	require.Len(t, first.Items, 1)

	// A new row lands after the page was read.
	newer := storedFeed(t, repo, feedTestUser, 1, base.Add(time.Hour))
	_ = newer

	second, err := svc.Page(context.Background(), feedTestUser, FeedPageParams{Limit: 10, Cursor: first.NextCursor})
	require.NoError(t, err)
	require.Len(t, second.Items, 1)
	assert.Equal(t, rows[0].ID, second.Items[0].ID, "the older row arrives on the resumed page, the new one does not duplicate it")
}

func TestFeedService_PageUnreadOnly(t *testing.T) {
	t.Parallel()

	repo := newFakeFeedQuery()
	svc := NewFeedService(repo, nil)
	base := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	rows := storedFeed(t, repo, feedTestUser, 3, base)
	readAt := base
	rows[0].ReadAt = &readAt
	repo.rows[rows[0].ID] = rows[0]

	page, err := svc.Page(context.Background(), feedTestUser, FeedPageParams{UnreadOnly: true})
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	assert.Equal(t, rows[2].ID, page.Items[0].ID, "newest unread first")
	assert.Equal(t, rows[1].ID, page.Items[1].ID)
}

// The unread-only filter serves the «Непрочитанные» pill from the partial
// index; the cursor keeps working with it.
func TestFeedService_PageUnreadOnlyWithCursor(t *testing.T) {
	t.Parallel()

	repo := newFakeFeedQuery()
	svc := NewFeedService(repo, nil)
	base := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	rows := storedFeed(t, repo, feedTestUser, 3, base)
	readAt := base
	rows[1].ReadAt = &readAt // The middle row is read — the unread walk skips it.
	repo.rows[rows[1].ID] = rows[1]

	first, err := svc.Page(context.Background(), feedTestUser, FeedPageParams{UnreadOnly: true, Limit: 1})
	require.NoError(t, err)
	require.Len(t, first.Items, 1)

	second, err := svc.Page(context.Background(), feedTestUser, FeedPageParams{UnreadOnly: true, Cursor: first.NextCursor})
	require.NoError(t, err)
	require.Len(t, second.Items, 1)
	assert.Equal(t, rows[0].ID, second.Items[0].ID)
}

// The reader's scope is the page boundary: another user's rows never leak.
func TestFeedService_PageScopedToUser(t *testing.T) {
	t.Parallel()

	repo := newFakeFeedQuery()
	svc := NewFeedService(repo, nil)
	base := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	other := uuid.Must(uuid.NewV7())
	storedFeed(t, repo, other, 2, base)
	mine := storedFeed(t, repo, feedTestUser, 1, base)

	page, err := svc.Page(context.Background(), feedTestUser, FeedPageParams{})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, mine[0].ID, page.Items[0].ID)
}

func TestFeedService_PageDefaultsAndLimits(t *testing.T) {
	t.Parallel()

	repo := newFakeFeedQuery()
	svc := NewFeedService(repo, nil)
	base := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	storedFeed(t, repo, feedTestUser, 60, base)

	page, err := svc.Page(context.Background(), feedTestUser, FeedPageParams{})
	require.NoError(t, err)
	assert.Len(t, page.Items, 50, "no limit parameter reads as the 50-row portion (канон #597)")

	_, err = svc.Page(context.Background(), feedTestUser, FeedPageParams{Limit: 101})
	assert.ErrorIs(t, err, ErrInvalidInput, "over the ceiling is a contract 400")
}

func TestFeedService_PageMalformedCursorIsInvalid(t *testing.T) {
	t.Parallel()

	svc := NewFeedService(newFakeFeedQuery(), nil)

	_, err := svc.Page(context.Background(), feedTestUser, FeedPageParams{Cursor: "not-a-cursor"})
	assert.ErrorIs(t, err, ErrInvalidInput, "the cursor decode is total — anything malformed is the 400")
}

func TestFeedService_UnreadCount(t *testing.T) {
	t.Parallel()

	repo := newFakeFeedQuery()
	svc := NewFeedService(repo, nil)
	base := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	rows := storedFeed(t, repo, feedTestUser, 3, base)
	readAt := base
	rows[0].ReadAt = &readAt
	repo.rows[rows[0].ID] = rows[0]

	count, err := svc.UnreadCount(context.Background(), feedTestUser)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestFeedService_MarkReadIsIdempotent(t *testing.T) {
	t.Parallel()

	repo := newFakeFeedQuery()
	svc := NewFeedService(repo, nil)
	base := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	rows := storedFeed(t, repo, feedTestUser, 1, base)

	require.NoError(t, svc.MarkRead(context.Background(), feedTestUser, rows[0].ID))
	require.NoError(t, svc.MarkRead(context.Background(), feedTestUser, rows[0].ID), "re-reading is not an error")

	count, err := svc.UnreadCount(context.Background(), feedTestUser)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestFeedService_MarkAllRead(t *testing.T) {
	t.Parallel()

	repo := newFakeFeedQuery()
	svc := NewFeedService(repo, nil)
	base := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	storedFeed(t, repo, feedTestUser, 3, base)

	flipped, err := svc.MarkAllRead(context.Background(), feedTestUser)
	require.NoError(t, err)
	assert.Equal(t, int64(3), flipped)

	count, err := svc.UnreadCount(context.Background(), feedTestUser)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestFeedService_DeleteOneAndAll(t *testing.T) {
	t.Parallel()

	repo := newFakeFeedQuery()
	svc := NewFeedService(repo, nil)
	base := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	rows := storedFeed(t, repo, feedTestUser, 3, base)

	require.NoError(t, svc.Delete(context.Background(), feedTestUser, rows[0].ID))

	page, err := svc.Page(context.Background(), feedTestUser, FeedPageParams{})
	require.NoError(t, err)
	require.Len(t, page.Items, 2, "the deleted row never appears in the feed")

	count, err := svc.UnreadCount(context.Background(), feedTestUser)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count, "deleting an unread row lowers the unread counter")

	hidden, err := svc.DeleteAll(context.Background(), feedTestUser)
	require.NoError(t, err)
	assert.Equal(t, int64(2), hidden)

	page, err = svc.Page(context.Background(), feedTestUser, FeedPageParams{})
	require.NoError(t, err)
	assert.Empty(t, page.Items)
}
