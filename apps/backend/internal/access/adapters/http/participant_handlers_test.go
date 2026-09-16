package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeParticipantsManager is the func-backed ParticipantsManager double
// (ADR 0035): an unset use case fails the test loudly.
type fakeParticipantsManager struct {
	list    func(ctx context.Context, actor uuid.UUID) ([]accessapp.Participant, error)
	get     func(ctx context.Context, actor uuid.UUID, id string) (accessapp.Participant, error)
	summary func(ctx context.Context, actor uuid.UUID) (accessapp.ParticipantsSummary, error)
}

func (f *fakeParticipantsManager) ListParticipants(ctx context.Context, actor uuid.UUID) ([]accessapp.Participant, error) {
	if f.list == nil {
		return nil, errors.New("unexpected ListParticipants call")
	}
	return f.list(ctx, actor)
}

func (f *fakeParticipantsManager) GetParticipant(ctx context.Context, actor uuid.UUID, id string) (accessapp.Participant, error) {
	if f.get == nil {
		return accessapp.Participant{}, errors.New("unexpected GetParticipant call")
	}
	return f.get(ctx, actor, id)
}

func (f *fakeParticipantsManager) Summary(ctx context.Context, actor uuid.UUID) (accessapp.ParticipantsSummary, error) {
	if f.summary == nil {
		return accessapp.ParticipantsSummary{}, errors.New("unexpected Summary call")
	}
	return f.summary(ctx, actor)
}

// participantFixtureUser is the registered-user fixture of the wire tests.
func participantFixtureUser() accessapp.Participant {
	propID := uuid.Must(uuid.NewV7())
	return accessapp.Participant{
		UserID:          uuid.Must(uuid.NewV7()),
		Email:           "u1@x.ru",
		DisplayName:     "Иван Иванов",
		AggregateStatus: domain.ParticipantStatusPartial,
		AccessibleCount: 1,
		Properties: []accessapp.ParticipantProperty{
			{PropertyID: propID, Title: "Квартира", Role: domain.RoleViewer, Status: domain.ParticipantEntryActive},
		},
	}
}

// newParticipantRequest builds a GET request with the user context set.
func newParticipantRequest(t *testing.T, target string, userID *uuid.UUID) *http.Request {
	t.Helper()
	ctx := context.Background()
	if userID != nil {
		ctx = httpsupport.WithUserID(ctx, *userID)
	}
	return httptest.NewRequestWithContext(ctx, http.MethodGet, target, nil)
}

// TestParticipantResponseWireShape checks the response contract: a registered
// user carries id = user uuid, display name and email; a pending row carries
// id = invitee email with a null user id and display name; the property legs
// carry title, role and status.
func TestParticipantResponseWireShape(t *testing.T) {
	t.Parallel()

	user := participantFixtureUser()
	resp := participantResponse(user)
	assert.Equal(t, user.UserID.String(), resp.Id)
	assert.Equal(t, user.UserID.String(), resp.UserId.String())
	require.NotNil(t, resp.Email)
	assert.Equal(t, "u1@x.ru", string(*resp.Email))
	require.NotNil(t, resp.DisplayName)
	assert.Equal(t, "Иван Иванов", *resp.DisplayName)
	assert.Equal(t, "partial", string(resp.AggregateStatus))
	assert.Equal(t, 1, resp.AccessiblePropertiesCount)
	require.Len(t, resp.Properties, 1)
	assert.Equal(t, user.Properties[0].PropertyID.String(), resp.Properties[0].PropertyId.String())
	assert.Equal(t, "Квартира", resp.Properties[0].Title)
	assert.Equal(t, "viewer", string(resp.Properties[0].Role))
	assert.Equal(t, "active", string(resp.Properties[0].Status))

	pending := accessapp.Participant{
		Email:           "pending@x.ru",
		AggregateStatus: domain.ParticipantStatusAllProperties,
		AccessibleCount: 2,
	}
	pendingResp := participantResponse(pending)
	assert.Equal(t, "pending@x.ru", pendingResp.Id)
	assert.Nil(t, pendingResp.UserId)
	require.NotNil(t, pendingResp.Email)
	assert.Equal(t, "pending@x.ru", string(*pendingResp.Email))
	assert.Nil(t, pendingResp.DisplayName)
	assert.Equal(t, "all_properties", string(pendingResp.AggregateStatus))
}

// TestListParticipantsHandler checks the 200 wire of the list and the 401
// without a session.
func TestListParticipantsHandler(t *testing.T) {
	t.Parallel()

	actor := uuid.Must(uuid.NewV7())
	svc := &fakeParticipantsManager{
		list: func(ctx context.Context, id uuid.UUID) ([]accessapp.Participant, error) {
			assert.Equal(t, actor, id)
			return []accessapp.Participant{participantFixtureUser()}, nil
		},
	}
	h := NewParticipantHandlers(svc, slog.New(slog.DiscardHandler))

	w := httptest.NewRecorder()
	h.ListParticipants(w, newParticipantRequest(t, "/participants", &actor))
	require.Equal(t, http.StatusOK, w.Code)

	var body openapi.ParticipantsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Items, 1)
	assert.Equal(t, "Иван Иванов", *body.Items[0].DisplayName)

	// No session — 401.
	w = httptest.NewRecorder()
	h.ListParticipants(w, newParticipantRequest(t, "/participants", nil))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestGetParticipantHandler checks the 200 wire, the privacy-preserving 404
// and the 401 without a session.
func TestGetParticipantHandler(t *testing.T) {
	t.Parallel()

	actor := uuid.Must(uuid.NewV7())
	participantID := uuid.Must(uuid.NewV7()).String()
	svc := &fakeParticipantsManager{
		get: func(ctx context.Context, id uuid.UUID, got string) (accessapp.Participant, error) {
			assert.Equal(t, actor, id)
			assert.Equal(t, participantID, got)
			return participantFixtureUser(), nil
		},
	}
	h := NewParticipantHandlers(svc, slog.New(slog.DiscardHandler))

	w := httptest.NewRecorder()
	h.GetParticipant(w, newParticipantRequest(t, "/participants/"+participantID, &actor), participantID)
	require.Equal(t, http.StatusOK, w.Code)
	var body openapi.ParticipantResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "partial", string(body.AggregateStatus))

	// Revoked or out of scope — the privacy-preserving 404.
	missing := &fakeParticipantsManager{
		get: func(ctx context.Context, id uuid.UUID, got string) (accessapp.Participant, error) {
			return accessapp.Participant{}, domain.ErrParticipantNotFound
		},
	}
	h2 := NewParticipantHandlers(missing, slog.New(slog.DiscardHandler))
	w = httptest.NewRecorder()
	h2.GetParticipant(w, newParticipantRequest(t, "/participants/gone", &actor), "gone")
	assert.Equal(t, http.StatusNotFound, w.Code)

	// No session — 401.
	w = httptest.NewRecorder()
	h.GetParticipant(w, newParticipantRequest(t, "/participants/"+participantID, nil), participantID)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestGetParticipantsSummaryHandler checks the hub counters wire.
func TestGetParticipantsSummaryHandler(t *testing.T) {
	t.Parallel()

	actor := uuid.Must(uuid.NewV7())
	svc := &fakeParticipantsManager{
		summary: func(ctx context.Context, id uuid.UUID) (accessapp.ParticipantsSummary, error) {
			assert.Equal(t, actor, id)
			return accessapp.ParticipantsSummary{ParticipantsCount: 3, AccessiblePropertiesCount: 1}, nil
		},
	}
	h := NewParticipantHandlers(svc, slog.New(slog.DiscardHandler))

	w := httptest.NewRecorder()
	h.GetParticipantsSummary(w, newParticipantRequest(t, "/participants/summary", &actor))
	require.Equal(t, http.StatusOK, w.Code)
	var body openapi.ParticipantsSummaryResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, 3, body.ParticipantsCount)
	assert.Equal(t, 1, body.AccessiblePropertiesCount)
}
