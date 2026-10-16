package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// The feed page defaults (канон #597/#633): the client reads the stored feed
// in 50-row keyset portions; anything over the ceiling is a contract 400.
const (
	feedDefaultLimit = 50
	feedMaxLimit     = 100
)

// FeedService is the stored feed's reading side (карта #734, решение #737,
// #743): the keyset page, the unread counter, the read/delete mutations and
// the single-notification view whose action buttons are computed from the
// entities' live state. Settings never hide rows — the feed is written
// regardless of them (ADR 0058).
type FeedService struct {
	feed NotificationRepository
	live FeedLiveState
}

// NewFeedService creates the feed reading service. The live-state port is
// the action computation's source; the reading use cases that need no
// actions accept a nil port.
func NewFeedService(feed NotificationRepository, live FeedLiveState) *FeedService {
	return &FeedService{feed: feed, live: live}
}

// FeedPageParams are the GET /notifications query parameters: the opaque
// continuation cursor, the unread-only filter and the page size (0 = the
// 50-row default).
type FeedPageParams struct {
	Cursor     string
	UnreadOnly bool
	Limit      int
}

// FeedPage is one keyset portion of the user's feed, newest first. An empty
// NextCursor means the walk is over.
type FeedPage struct {
	Items      []domain.Notification
	NextCursor string
}

// Page returns the user's feed page. Deleted rows never appear; the cursor
// resumes strictly after the previous page's last row, so rows created
// between loads neither duplicate nor drop.
func (s *FeedService) Page(ctx context.Context, userID uuid.UUID, params FeedPageParams) (FeedPage, error) {
	limit := params.Limit
	if limit == 0 {
		limit = feedDefaultLimit
	}
	if limit < 0 || limit > feedMaxLimit {
		return FeedPage{}, fmt.Errorf("limit %d: %w", limit, ErrInvalidInput)
	}

	var afterCreatedAt *time.Time
	var afterID uuid.UUID
	if params.Cursor != "" {
		at, id, err := decodeFeedCursor(params.Cursor)
		if err != nil {
			return FeedPage{}, err
		}
		afterCreatedAt = &at
		afterID = id
	}

	rows, err := s.feed.ListPage(ctx, userID, params.UnreadOnly, afterCreatedAt, afterID, limit)
	if err != nil {
		return FeedPage{}, fmt.Errorf("list feed page: %w", err)
	}

	page := FeedPage{Items: rows}
	if n := len(rows); n > 0 && n == limit {
		last := rows[n-1]
		page.NextCursor = encodeFeedCursor(last.CreatedAt, last.ID)
	}
	return page, nil
}

// UnreadCount returns the user's unread, not-deleted row count.
func (s *FeedService) UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	count, err := s.feed.CountUnread(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("unread count: %w", err)
	}
	return count, nil
}

// MarkRead marks one row read. Idempotent: re-reading or marking an unknown
// row is not an error — the read state is the same.
func (s *FeedService) MarkRead(ctx context.Context, userID, id uuid.UUID) error {
	if _, err := s.feed.MarkRead(ctx, userID, id); err != nil {
		return fmt.Errorf("mark read: %w", err)
	}
	return nil
}

// MarkAllRead marks every unread row of the user read and returns how many
// rows flipped.
func (s *FeedService) MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error) {
	count, err := s.feed.MarkAllRead(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("mark all read: %w", err)
	}
	return count, nil
}

// Delete soft-deletes one row: the row stays, the feed and the unread
// counter stop seeing it. Idempotent like MarkRead.
func (s *FeedService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	if _, err := s.feed.Delete(ctx, userID, id); err != nil {
		return fmt.Errorf("delete notification: %w", err)
	}
	return nil
}

// DeleteAll soft-deletes every row of the user and returns how many rows
// were hidden.
func (s *FeedService) DeleteAll(ctx context.Context, userID uuid.UUID) (int64, error) {
	count, err := s.feed.DeleteAll(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("delete all notifications: %w", err)
	}
	return count, nil
}
