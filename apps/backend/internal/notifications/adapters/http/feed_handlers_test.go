package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// handlerFeedRepo is the notifications http package's in-memory feed: the
// same keyset semantics the SQL implements, reduced to what the handler
// tests exercise.
type handlerFeedRepo struct {
	rows map[uuid.UUID]domain.Notification
}

func newHandlerFeedRepo() *handlerFeedRepo {
	return &handlerFeedRepo{rows: make(map[uuid.UUID]domain.Notification)}
}

func (r *handlerFeedRepo) WithTx(tx transaction.Tx) (notificationsapp.NotificationRepository, error) {
	return r, nil
}

func (r *handlerFeedRepo) Insert(_ context.Context, n domain.Notification) (bool, error) {
	r.rows[n.ID] = n
	return true, nil
}

func (r *handlerFeedRepo) GetByID(_ context.Context, id uuid.UUID) (domain.Notification, error) {
	n, ok := r.rows[id]
	if !ok {
		return domain.Notification{}, notificationsapp.ErrNotFound
	}
	return n, nil
}

func (r *handlerFeedRepo) GetForUser(_ context.Context, userID, id uuid.UUID) (domain.Notification, error) {
	n, ok := r.rows[id]
	if !ok || n.UserID != userID || n.DeletedAt != nil {
		return domain.Notification{}, notificationsapp.ErrNotFound
	}
	return n, nil
}

func (r *handlerFeedRepo) ListPage(
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

func (r *handlerFeedRepo) CountUnread(_ context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	for _, n := range r.rows {
		if n.UserID == userID && n.ReadAt == nil && n.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

func (r *handlerFeedRepo) MarkRead(_ context.Context, userID, id uuid.UUID) (bool, error) {
	n, ok := r.rows[id]
	if !ok || n.UserID != userID || n.ReadAt != nil || n.DeletedAt != nil {
		return false, nil
	}
	now := time.Now()
	n.ReadAt = &now
	r.rows[id] = n
	return true, nil
}

func (r *handlerFeedRepo) MarkAllRead(_ context.Context, userID uuid.UUID) (int64, error) {
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

func (r *handlerFeedRepo) Delete(_ context.Context, userID, id uuid.UUID) (bool, error) {
	n, ok := r.rows[id]
	if !ok || n.UserID != userID || n.DeletedAt != nil {
		return false, nil
	}
	now := time.Now()
	n.DeletedAt = &now
	r.rows[id] = n
	return true, nil
}

func (r *handlerFeedRepo) DeleteAll(_ context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	for id, n := range r.rows {
		if n.UserID == userID && n.DeletedAt == nil {
			now := time.Now()
			n.DeletedAt = &now
			r.rows[id] = n
			count++
		}
	}
	return count, nil
}

func newFeedTestHandlers() (*FeedHandlers, *handlerFeedRepo) {
	repo := newHandlerFeedRepo()
	return NewFeedHandlers(notificationsapp.NewFeedService(repo, nil), slog.Default()), repo
}

func feedRequest(userID *uuid.UUID, target string) *http.Request {
	ctx := context.Background()
	if userID != nil {
		ctx = httpsupport.WithUserID(ctx, *userID)
	}
	return httptest.NewRequestWithContext(ctx, http.MethodGet, target, nil)
}

// handlerID is the tests' shared reader id.
var handlerID = uuid.Must(uuid.NewV7())

// seedHandlerFeed stores n rows for the reader, one minute apart.
func seedHandlerFeed(t *testing.T, repo *handlerFeedRepo, n int, base time.Time) []domain.Notification {
	t.Helper()
	rows := make([]domain.Notification, 0, n)
	for i := range n {
		row, err := domain.NewNotification(
			uuid.Must(uuid.NewV7()), handlerID, domain.EventPaymentDue,
			"Оплатите платёж", "Платёж по объекту «Объект»: 2000000.", "Объект",
			domain.Payload{},
			domain.DedupKey("payment_due:handler:"+uuid.Must(uuid.NewV7()).String()),
		)
		require.NoError(t, err)
		row.CreatedAt = base.Add(time.Duration(i) * time.Minute)
		_, err = repo.Insert(context.Background(), *row)
		require.NoError(t, err)
		rows = append(rows, *row)
	}
	return rows
}

func TestFeedHandlers_ListPageAndCursor(t *testing.T) {
	t.Parallel()

	h, repo := newFeedTestHandlers()
	seedHandlerFeed(t, repo, 3, time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC))
	user := handlerID

	w := httptest.NewRecorder()
	h.ListNotifications(w, feedRequest(&user, "/notifications"), openapi.ListNotificationsParams{})
	require.Equal(t, http.StatusOK, w.Code)

	var page struct {
		Items      []map[string]any `json:"items"`
		NextCursor *string          `json:"next_cursor"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	require.Len(t, page.Items, 3)
	assert.Nil(t, page.NextCursor, "a full feed has no next page")

	// Limit=1 yields a cursor; echoing it resumes after the first row.
	w = httptest.NewRecorder()
	h.ListNotifications(w, feedRequest(&user, "/notifications?limit=1"), openapi.ListNotificationsParams{Limit: new(1)})
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	require.Len(t, page.Items, 1)
	require.NotNil(t, page.NextCursor)

	w = httptest.NewRecorder()
	h.ListNotifications(w, feedRequest(&user, "/notifications"),
		openapi.ListNotificationsParams{Cursor: page.NextCursor})
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	require.Len(t, page.Items, 2, "the resumed page holds the rest of the feed")
}

func TestFeedHandlers_UnreadCountReadAndDelete(t *testing.T) {
	t.Parallel()

	h, repo := newFeedTestHandlers()
	rows := seedHandlerFeed(t, repo, 2, time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC))
	user := handlerID

	w := httptest.NewRecorder()
	h.GetUnreadNotificationCount(w, feedRequest(&user, "/notifications/unread-count"))
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"count":2}`, w.Body.String())

	w = httptest.NewRecorder()
	h.MarkNotificationRead(w, feedRequest(&user, "/notifications/"+rows[0].ID.String()+"/read"), rows[0].ID)
	assert.Equal(t, http.StatusNoContent, w.Code)

	w = httptest.NewRecorder()
	h.MarkAllNotificationsRead(w, feedRequest(&user, "/notifications/read-all"))
	assert.Equal(t, http.StatusNoContent, w.Code)

	w = httptest.NewRecorder()
	h.GetUnreadNotificationCount(w, feedRequest(&user, "/notifications/unread-count"))
	assert.JSONEq(t, `{"count":0}`, w.Body.String())

	w = httptest.NewRecorder()
	h.DeleteNotification(w, feedRequest(&user, "/notifications/"+rows[0].ID.String()), rows[0].ID)
	assert.Equal(t, http.StatusNoContent, w.Code)

	w = httptest.NewRecorder()
	h.DeleteAllNotifications(w, feedRequest(&user, "/notifications"))
	assert.Equal(t, http.StatusNoContent, w.Code)

	w = httptest.NewRecorder()
	h.ListNotifications(w, feedRequest(&user, "/notifications"), openapi.ListNotificationsParams{})
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"items":[]}`, w.Body.String())
}

// The detail view returns the stored row with the live actions; a foreign
// row is a 404, a malformed cursor and an over-the-ceiling limit are 400s.
func TestFeedHandlers_DetailAndErrors(t *testing.T) {
	t.Parallel()

	repo := newHandlerFeedRepo()
	live := &handlerLiveState{
		rental: notificationsapp.RentalActionState{Exists: true, AwaitingAction: true, PropertyID: uuid.Must(uuid.NewV7())},
		access: notificationsapp.PropertyAccess{Exists: true, Manageable: true},
	}
	h := NewFeedHandlers(notificationsapp.NewFeedService(repo, live), slog.Default())
	user := handlerID

	rentalID := uuid.Must(uuid.NewV7())
	n, err := domain.NewNotification(
		uuid.Must(uuid.NewV7()), user, domain.EventRentalCompleted,
		"Аренда завершена", "Срок аренды истёк.", "Объект",
		domain.Payload{RentalID: &rentalID},
		domain.DedupKey("rental_completed:handler"),
	)
	require.NoError(t, err)
	_, err = repo.Insert(context.Background(), *n)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	h.GetNotification(w, feedRequest(&user, "/notifications/"+n.ID.String()), n.ID)
	require.Equal(t, http.StatusOK, w.Code)

	var detail struct {
		ID      string   `json:"id"`
		Actions []string `json:"actions"`
		Payload struct {
			RentalID string `json:"rental_id"`
		} `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &detail))
	assert.Equal(t, n.ID.String(), detail.ID)
	assert.Equal(t, []string{"rental_extend", "rental_complete"}, detail.Actions)
	assert.NotEmpty(t, detail.Payload.RentalID, "the payload carries the card's links")

	stranger := uuid.Must(uuid.NewV7())
	w = httptest.NewRecorder()
	h.GetNotification(w, feedRequest(&stranger, "/notifications/"+n.ID.String()), n.ID)
	assert.Equal(t, http.StatusNotFound, w.Code)

	garbage := "garbage"
	w = httptest.NewRecorder()
	h.ListNotifications(w, feedRequest(&user, "/notifications?cursor=garbage"),
		openapi.ListNotificationsParams{Cursor: &garbage})
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = httptest.NewRecorder()
	h.ListNotifications(w, feedRequest(&user, "/notifications?limit=101"), openapi.ListNotificationsParams{Limit: new(101)})
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = httptest.NewRecorder()
	h.ListNotifications(w, feedRequest(nil, "/notifications"), openapi.ListNotificationsParams{})
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	w = httptest.NewRecorder()
	h.GetUnreadNotificationCount(w, feedRequest(nil, "/notifications/unread-count"))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// handlerLiveState is the http package's fake of the live-state port.
type handlerLiveState struct {
	rental notificationsapp.RentalActionState
	access notificationsapp.PropertyAccess
}

func (f *handlerLiveState) RentalActionState(_ context.Context, _ uuid.UUID) (notificationsapp.RentalActionState, error) {
	return f.rental, nil
}

func (f *handlerLiveState) PaymentOpen(_ context.Context, _ uuid.UUID) (bool, error) {
	return true, nil
}

func (f *handlerLiveState) TaskOpen(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil }

func (f *handlerLiveState) PropertyAccess(_ context.Context, _, _ uuid.UUID) (notificationsapp.PropertyAccess, error) {
	return f.access, nil
}
