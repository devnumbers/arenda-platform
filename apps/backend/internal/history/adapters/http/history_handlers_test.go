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
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// historyReaderFake is the http package's in-memory journal: whatever List
// returns, plus the query the handler folded for it.
type historyReaderFake struct {
	rows    []domain.FeedEntry
	gotFeed *historyapp.JournalQuery
	gotIDs  *[]uuid.UUID
}

func (f *historyReaderFake) List(_ context.Context, _ uuid.UUID, q historyapp.JournalQuery) ([]domain.FeedEntry, error) {
	f.gotFeed = &q
	return f.rows, nil
}

func (f *historyReaderFake) FilterParticipants(_ context.Context, _ uuid.UUID, ids []uuid.UUID) ([]domain.FilterParticipant, error) {
	f.gotIDs = &ids
	return []domain.FilterParticipant{{ID: uuid.Must(uuid.NewV7()), Name: "Иван Иванов", Email: "ivan@example.com"}}, nil
}

func (f *historyReaderFake) FilterObjects(_ context.Context, _ uuid.UUID, _ []uuid.UUID) ([]domain.FilterObject, error) {
	return []domain.FilterObject{{ID: uuid.Must(uuid.NewV7()), Name: "Гараж", Address: "Москва", PhotoURL: ""}}, nil
}

// historyPolicyAllow grants the owner role everywhere — the privacy-404
// proof belongs to the service unit tests, here the wire mapping matters.
type historyPolicyAllow struct{}

func (historyPolicyAllow) Role(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error) {
	return sharedpolicy.RoleOwner, nil
}

func (historyPolicyAllow) RoleForProperty(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error) {
	return sharedpolicy.RoleOwner, nil
}

func newHistoryTestHandlers() (*HistoryHandlers, *historyReaderFake) {
	reader := &historyReaderFake{}
	svc := historyapp.NewHistoryReadService(reader, historyPolicyAllow{})
	return NewHistoryHandlers(svc, slog.Default()), reader
}

func historyRequest(userID *uuid.UUID, target string) *http.Request {
	ctx := context.Background()
	if userID != nil {
		ctx = httpsupport.WithUserID(ctx, *userID)
	}
	return httptest.NewRequestWithContext(ctx, http.MethodGet, target, nil)
}

var historyReaderID = uuid.Must(uuid.NewV7())

func TestHistoryHandlers_GetHistoryPage(t *testing.T) {
	t.Parallel()

	h, reader := newHistoryTestHandlers()
	actor := uuid.Must(uuid.NewV7())
	linkID := uuid.Must(uuid.NewV7())
	reader.rows = []domain.FeedEntry{{
		ID:           uuid.Must(uuid.NewV7()),
		PropertyID:   uuid.Must(uuid.NewV7()),
		PropertyName: "Гараж",
		ActorID:      &actor,
		ActorName:    "Иван Иванов",
		ActorEmail:   "ivan@example.com",
		ActorRole:    domain.ActorRoleOwner,
		Kind:         domain.KindOperation,
		Action:       domain.ActionOperationPaid,
		BaseAction:   domain.BaseCompleted,
		Segments: domain.Segments{
			{Text: "Операция оплачена: "},
			{Text: "Электричество", Link: &domain.Link{Kind: domain.KindOperation, ID: linkID}},
		},
		Context:   map[string]any{"amount_kopecks": float64(150000)},
		CreatedAt: time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC),
	}}

	w := httptest.NewRecorder()
	limit := 2
	propertyID := reader.rows[0].PropertyID
	h.GetHistory(w, historyRequest(&historyReaderID, "/history"), openapi.GetHistoryParams{
		Limit:       &limit,
		Kinds:       new("operation,contact"),
		Actions:     new("completed"),
		PropertyIds: new(propertyID.String()),
		Q:           new("оплачена"),
	})
	require.Equal(t, http.StatusOK, w.Code)

	// Запрос дошёл до сервиса свёрнутым: csv разложен, поиск жив.
	require.NotNil(t, reader.gotFeed)
	assert.Equal(t, 2, reader.gotFeed.Limit)
	assert.Equal(t, []domain.Kind{"operation", "contact"}, reader.gotFeed.Kinds)
	assert.Equal(t, []domain.BaseAction{"completed"}, reader.gotFeed.BaseActions)
	assert.Equal(t, []uuid.UUID{reader.rows[0].PropertyID}, reader.gotFeed.PropertyIDs)
	assert.Equal(t, "fts", reader.gotFeed.Search.Mode)

	var page struct {
		Items []struct {
			ID           uuid.UUID `json:"id"`
			PropertyName string    `json:"property_name"`
			ActorID      *string   `json:"actor_id"`
			ActorRole    string    `json:"actor_role"`
			BaseAction   string    `json:"base_action"`
			Segments     []struct {
				Text string `json:"text"`
				Link *struct {
					Kind string `json:"kind"`
					ID   string `json:"id"`
				} `json:"link"`
			} `json:"segments"`
			Context   map[string]any `json:"context"`
			CreatedAt time.Time      `json:"created_at"`
		} `json:"items"`
		NextCursor *string `json:"next_cursor"`
		PrevCursor *string `json:"prev_cursor"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	require.Len(t, page.Items, 1)
	item := page.Items[0]
	assert.Equal(t, "Гараж", item.PropertyName)
	assert.Equal(t, "owner", item.ActorRole)
	assert.Equal(t, "completed", item.BaseAction)
	require.NotNil(t, item.ActorID)
	assert.Equal(t, actor.String(), *item.ActorID)
	require.Len(t, item.Segments, 2)
	require.NotNil(t, item.Segments[1].Link)
	assert.Equal(t, "operation", item.Segments[1].Link.Kind)
	assert.Equal(t, linkID.String(), item.Segments[1].Link.ID)
	assert.InDelta(t, 150000.0, item.Context["amount_kopecks"], 0.01)
}

func TestHistoryHandlers_GetHistoryErrors(t *testing.T) {
	t.Parallel()

	h, _ := newHistoryTestHandlers()

	// Нет сессии — 401.
	w := httptest.NewRecorder()
	h.GetHistory(w, historyRequest(nil, "/history"), openapi.GetHistoryParams{})
	require.Equal(t, http.StatusUnauthorized, w.Code)

	// Битый курсор — 400 (сервисная свёртка).
	w = httptest.NewRecorder()
	h.GetHistory(w, historyRequest(&historyReaderID, "/history"), openapi.GetHistoryParams{
		BeforeCursor: new("###"),
	})
	require.Equal(t, http.StatusBadRequest, w.Code)

	// Оба курсора — 400.
	w = httptest.NewRecorder()
	h.GetHistory(w, historyRequest(&historyReaderID, "/history"), openapi.GetHistoryParams{
		BeforeCursor: new("a"),
		AfterCursor:  new("b"),
	})
	require.Equal(t, http.StatusBadRequest, w.Code)

	// Кривой uuid в списке — 400.
	w = httptest.NewRecorder()
	h.GetHistory(w, historyRequest(&historyReaderID, "/history"), openapi.GetHistoryParams{
		ActorIds: new("nope"),
	})
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHistoryHandlers_GetHistoryFilters(t *testing.T) {
	t.Parallel()

	h, reader := newHistoryTestHandlers()
	propertyID := uuid.Must(uuid.NewV7())

	w := httptest.NewRecorder()
	h.GetHistoryFilters(w, historyRequest(&historyReaderID, "/history/filters"), openapi.GetHistoryFiltersParams{
		PropertyIds: new(propertyID.String()),
	})
	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, reader.gotIDs)
	assert.Equal(t, []uuid.UUID{propertyID}, *reader.gotIDs)

	var body struct {
		Participants []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"participants"`
		Objects []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			PhotoURL string `json:"photo_url"`
		} `json:"objects"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Participants, 1)
	assert.Equal(t, "Иван Иванов", body.Participants[0].Name)
	require.Len(t, body.Objects, 1)
	assert.Equal(t, "Гараж", body.Objects[0].Name)
}
