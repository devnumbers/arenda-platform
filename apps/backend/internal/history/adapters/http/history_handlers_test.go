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
	return []domain.FilterParticipant{
		{ID: uuid.Must(uuid.NewV7()), Name: fakeParticipantName, Email: "ivan@example.com"},
		// Участник с фото: путь стриминга доходит до провода дословно.
		{
			ID: uuid.Must(uuid.NewV7()), Name: "Мария Петрова", Email: "maria@example.com",
			PhotoURL: "/api/users/" + uuid.Must(uuid.NewV7()).String() + "/photo",
		},
	}, nil
}

func (f *historyReaderFake) FilterObjects(_ context.Context, _ uuid.UUID, _ []uuid.UUID) ([]domain.FilterObject, error) {
	return []domain.FilterObject{{ID: uuid.Must(uuid.NewV7()), Name: fakePropertyName, Address: "Москва", PhotoURL: "", Type: "garage"}}, nil
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
	// Nil-гейт выключен — тарифный вердикт пинят tariff_gate_test.go рядом.
	svc := historyapp.NewHistoryReadService(reader, historyPolicyAllow{}, nil, nil)
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

// Значения фейка, повторяющиеся в ассертах (goconst).
const (
	fakeParticipantName = "Иван Иванов"
	fakePropertyName    = "Гараж"
)

func TestHistoryHandlers_GetHistoryPage(t *testing.T) {
	t.Parallel()

	h, reader := newHistoryTestHandlers()
	actor := uuid.Must(uuid.NewV7())
	linkID := uuid.Must(uuid.NewV7())
	withPhoto := uuid.Must(uuid.NewV7())
	reader.rows = []domain.FeedEntry{{
		ID:           uuid.Must(uuid.NewV7()),
		PropertyID:   uuid.Must(uuid.NewV7()),
		PropertyName: fakePropertyName,
		ActorID:      &actor,
		ActorName:    fakeParticipantName,
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
	}, {
		// Актёр с фото: путь стриминга доходит до провода дословно.
		ID:            uuid.Must(uuid.NewV7()),
		PropertyID:    uuid.Must(uuid.NewV7()),
		PropertyName:  fakePropertyName,
		ActorID:       &withPhoto,
		ActorName:     fakeParticipantName,
		ActorPhotoURL: "/api/users/" + withPhoto.String() + "/photo",
		ActorRole:     domain.ActorRoleOwner,
		Kind:          domain.KindProperty,
		Action:        domain.ActionPropertyRenamed,
		BaseAction:    domain.BaseChanged,
		Segments:      domain.Segments{{Text: "Объект переименован"}},
		Context:       map[string]any{},
		CreatedAt:     time.Date(2026, 9, 20, 11, 0, 0, 0, time.UTC),
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
	assert.Equal(t, "оплачена", reader.gotFeed.Search)

	var page struct {
		Items []struct {
			ID           uuid.UUID `json:"id"`
			PropertyName string    `json:"property_name"`
			ActorID      *string   `json:"actor_id"`
			ActorRole    string    `json:"actor_role"`
			// Актёр без фото — null на проводе (контракт nullable, ADR 0065):
			// пустая строка на проводе ломает фолбэк заглушки клиента.
			ActorPhotoURL *string `json:"actor_photo_url"`
			BaseAction    string  `json:"base_action"`
			Segments      []struct {
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
	require.Len(t, page.Items, 2)
	item := page.Items[0]
	assert.Equal(t, fakePropertyName, item.PropertyName)
	assert.Equal(t, "owner", item.ActorRole)
	assert.Equal(t, "completed", item.BaseAction)
	require.NotNil(t, item.ActorID)
	assert.Equal(t, actor.String(), *item.ActorID)
	assert.Nil(t, item.ActorPhotoURL, "актёр без фото — null на проводе, не ''")
	require.Len(t, item.Segments, 2)
	require.NotNil(t, item.Segments[1].Link)
	assert.Equal(t, "operation", item.Segments[1].Link.Kind)
	assert.Equal(t, linkID.String(), item.Segments[1].Link.ID)
	assert.InDelta(t, 150000.0, item.Context["amount_kopecks"], 0.01)

	withPhotoItem := page.Items[1]
	require.NotNil(t, withPhotoItem.ActorPhotoURL)
	assert.Equal(t, "/api/users/"+withPhoto.String()+"/photo", *withPhotoItem.ActorPhotoURL)
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
			// Участник без фото — null на проводе (контракт nullable, ADR 0065).
			PhotoURL *string `json:"photo_url"`
		} `json:"participants"`
		Objects []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			PhotoURL string `json:"photo_url"`
			Type     string `json:"type"`
		} `json:"objects"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Participants, 2)
	assert.Equal(t, fakeParticipantName, body.Participants[0].Name)
	assert.Nil(t, body.Participants[0].PhotoURL, "участник без фото — null на проводе, не ''")
	assert.Equal(t, "Мария Петрова", body.Participants[1].Name)
	require.NotNil(t, body.Participants[1].PhotoURL)
	require.Len(t, body.Objects, 1)
	assert.Equal(t, fakePropertyName, body.Objects[0].Name)
	assert.Equal(t, "garage", body.Objects[0].Type, "the filter row's glyph type (#1244)")
}

// scopesFake is the participant-scope resolver's http test double (карта
// #1582): canned scope or error plus the id the handler passed through.
type scopesFake struct {
	scope historyapp.ParticipantScope
	err   error
	gotID string
}

func (f *scopesFake) ResolveParticipantScope(_ context.Context, _ uuid.UUID, participantID string) (historyapp.ParticipantScope, error) {
	f.gotID = participantID
	return f.scope, f.err
}

func newParticipantHistoryHandlers(scope historyapp.ParticipantScope, scopesErr error) (*HistoryHandlers, *historyReaderFake, *scopesFake) {
	reader := &historyReaderFake{}
	scopes := &scopesFake{scope: scope, err: scopesErr}
	svc := historyapp.NewHistoryReadService(reader, historyPolicyAllow{}, nil, scopes)
	return NewHistoryHandlers(svc, slog.Default()), reader, scopes
}

func TestHistoryHandlers_GetParticipantHistory(t *testing.T) {
	t.Parallel()

	userID := uuid.Must(uuid.NewV7())
	leg1, leg2 := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())

	// Зарегистрированный: скоуп едет на сервис (uuid юзера AND ноги),
	// ответ — страница ленты.
	h, reader, scopes := newParticipantHistoryHandlers(
		historyapp.ParticipantScope{UserID: userID, PropertyIDs: []uuid.UUID{leg1, leg2}},
		nil,
	)
	q := "платёж"
	w := httptest.NewRecorder()
	req := historyRequest(&historyReaderID, "/participants/x/history")
	h.GetParticipantHistory(w, req, "12111111-1111-4111-8111-111111111121", openapi.GetParticipantHistoryParams{
		Q: &q,
	})
	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, reader.gotFeed)
	assert.Equal(t, []uuid.UUID{userID}, reader.gotFeed.ActorIDs)
	assert.Equal(t, []uuid.UUID{leg1, leg2}, reader.gotFeed.PropertyIDs)
	assert.Equal(t, "платёж", reader.gotFeed.Search)
	assert.Equal(t, "12111111-1111-4111-8111-111111111121", scopes.gotID)

	var page struct {
		Items []json.RawMessage `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))

	// Pending: uuid нет — пустая страница без стора.
	h, reader, _ = newParticipantHistoryHandlers(
		historyapp.ParticipantScope{UserID: uuid.UUID{}, PropertyIDs: []uuid.UUID{leg1}},
		nil,
	)
	w = httptest.NewRecorder()
	h.GetParticipantHistory(w, req, "pending@example.com", openapi.GetParticipantHistoryParams{})
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Empty(t, page.Items)
	assert.Nil(t, reader.gotFeed, "pending must not reach the store")

	// Чужой участник: privacy-404 с формулировкой поверхности.
	h, _, _ = newParticipantHistoryHandlers(historyapp.ParticipantScope{}, historyapp.ErrNotFound)
	w = httptest.NewRecorder()
	h.GetParticipantHistory(w, req, "stranger@example.com", openapi.GetParticipantHistoryParams{})
	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Участник не найден")

	// Нет сессии — 401; битый курсор — 400.
	h, _, _ = newParticipantHistoryHandlers(
		historyapp.ParticipantScope{UserID: userID, PropertyIDs: []uuid.UUID{leg1}}, nil,
	)
	w = httptest.NewRecorder()
	h.GetParticipantHistory(w, historyRequest(nil, "/participants/x/history"), userID.String(), openapi.GetParticipantHistoryParams{})
	require.Equal(t, http.StatusUnauthorized, w.Code)

	w = httptest.NewRecorder()
	h.GetParticipantHistory(w, req, userID.String(), openapi.GetParticipantHistoryParams{
		BeforeCursor: new("###"),
	})
	require.Equal(t, http.StatusBadRequest, w.Code)
}
