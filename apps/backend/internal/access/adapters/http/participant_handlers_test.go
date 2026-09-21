package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
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

// fakeParticipantMutations is the func-backed ParticipantMutations double
// (ADR 0035): an unset use case fails the test loudly.
type fakeParticipantMutations struct {
	invite func(ctx context.Context, actor uuid.UUID, email string,
		role domain.Role, ids []uuid.UUID) ([]accessapp.ParticipantGrantResult, error)
	addProperties func(ctx context.Context, actor uuid.UUID, id string,
		role domain.Role, ids []uuid.UUID) ([]accessapp.ParticipantGrantResult, error)
	remove func(ctx context.Context, actor uuid.UUID, id string) error
}

func (f *fakeParticipantMutations) Invite(
	ctx context.Context, actor uuid.UUID, email string, role domain.Role, ids []uuid.UUID,
) ([]accessapp.ParticipantGrantResult, error) {
	if f.invite == nil {
		return nil, errors.New("unexpected Invite call")
	}
	return f.invite(ctx, actor, email, role, ids)
}

func (f *fakeParticipantMutations) AddProperties(
	ctx context.Context, actor uuid.UUID, id string, role domain.Role, ids []uuid.UUID,
) ([]accessapp.ParticipantGrantResult, error) {
	if f.addProperties == nil {
		return nil, errors.New("unexpected AddProperties call")
	}
	return f.addProperties(ctx, actor, id, role, ids)
}

func (f *fakeParticipantMutations) Remove(ctx context.Context, actor uuid.UUID, id string) error {
	if f.remove == nil {
		return errors.New("unexpected Remove call")
	}
	return f.remove(ctx, actor, id)
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
	h := NewParticipantHandlers(svc, &fakeParticipantMutations{}, slog.New(slog.DiscardHandler))

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
	h := NewParticipantHandlers(svc, &fakeParticipantMutations{}, slog.New(slog.DiscardHandler))

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
	h2 := NewParticipantHandlers(missing, &fakeParticipantMutations{}, slog.New(slog.DiscardHandler))
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
	h := NewParticipantHandlers(svc, &fakeParticipantMutations{}, slog.New(slog.DiscardHandler))

	w := httptest.NewRecorder()
	h.GetParticipantsSummary(w, newParticipantRequest(t, "/participants/summary", &actor))
	require.Equal(t, http.StatusOK, w.Code)
	var body openapi.ParticipantsSummaryResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, 3, body.ParticipantsCount)
	assert.Equal(t, 1, body.AccessiblePropertiesCount)
}

// newParticipantMutationRequest builds a POST/DELETE request with the user
// context and an optional JSON body set. A JSON body carries the
// application/json Content-Type the shared decoder requires (ADR 0056).
func newParticipantMutationRequest(t *testing.T, method, target string, userID *uuid.UUID, body any) *http.Request {
	t.Helper()
	ctx := context.Background()
	if userID != nil {
		ctx = httpsupport.WithUserID(ctx, *userID)
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequestWithContext(ctx, method, target, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// fieldEmail is the JSON field name of the invitee email in request bodies.
const fieldEmail = "email"

// testInviteeEmail is the invitee email of the invite-endpoint wire tests.
const testInviteeEmail = "new@x.ru"

// fieldPropertyIDs and fieldRole are the JSON field names of the batch
// request bodies.
const (
	fieldPropertyIDs = "property_ids"
	fieldRole        = "role"

	// The role values of the wire-test request bodies.
	roleViewerJSON     = "viewer"
	roleFullAccessJSON = "full_access"
	roleUnknownJSON    = "admin"
)

// grantFixtureResults is the shared batch result of the wire tests.
func grantFixtureResults() []accessapp.ParticipantGrantResult {
	activeProp := uuid.Must(uuid.NewV7())
	activeMember := uuid.Must(uuid.NewV7())
	suspendedProp := uuid.Must(uuid.NewV7())
	suspendedMember := uuid.Must(uuid.NewV7())
	pendingProp := uuid.Must(uuid.NewV7())
	pendingInvitation := uuid.Must(uuid.NewV7())
	duplicateProp := uuid.Must(uuid.NewV7())
	return []accessapp.ParticipantGrantResult{
		{PropertyID: activeProp, Outcome: accessapp.ParticipantGrantActive, MembershipID: activeMember},
		{PropertyID: suspendedProp, Outcome: accessapp.ParticipantGrantSuspended, MembershipID: suspendedMember},
		{PropertyID: pendingProp, Outcome: accessapp.ParticipantGrantPending, InvitationID: pendingInvitation},
		{PropertyID: duplicateProp, Outcome: accessapp.ParticipantGrantDuplicate},
	}
}

// TestInviteParticipantHandler checks the 200 wire of the batch results, the
// 400s (malformed body, role, empty objects, bad email) and the 401.
func TestInviteParticipantHandler(t *testing.T) {
	t.Parallel()

	actor := uuid.Must(uuid.NewV7())
	propID := uuid.Must(uuid.NewV7())
	mutations := &fakeParticipantMutations{
		invite: func(ctx context.Context, id uuid.UUID, email string,
			role domain.Role, ids []uuid.UUID,
		) ([]accessapp.ParticipantGrantResult, error) {
			assert.Equal(t, actor, id)
			assert.Equal(t, testInviteeEmail, email)
			assert.Equal(t, domain.RoleViewer, role)
			assert.Equal(t, []uuid.UUID{propID}, ids)
			return grantFixtureResults(), nil
		},
	}
	h := NewParticipantHandlers(&fakeParticipantsManager{}, mutations, slog.New(slog.DiscardHandler))

	w := httptest.NewRecorder()
	h.InviteParticipant(w, newParticipantMutationRequest(t, http.MethodPost, "/participants/invite", &actor,
		map[string]any{fieldEmail: testInviteeEmail, fieldRole: roleViewerJSON, fieldPropertyIDs: []string{propID.String()}}))
	require.Equal(t, http.StatusOK, w.Code)
	var body openapi.ParticipantGrantResultsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Items, 4)
	assert.Equal(t, "active", string(body.Items[0].Outcome))
	require.NotNil(t, body.Items[0].MembershipId)
	assert.Nil(t, body.Items[0].InvitationId)
	assert.Equal(t, "suspended", string(body.Items[1].Outcome))
	assert.Equal(t, "pending", string(body.Items[2].Outcome))
	require.NotNil(t, body.Items[2].InvitationId)
	assert.Nil(t, body.Items[2].MembershipId)
	assert.Equal(t, "skipped_duplicate", string(body.Items[3].Outcome))
	assert.Nil(t, body.Items[3].MembershipId)
	assert.Nil(t, body.Items[3].InvitationId)

	// The target's own email — bad request.
	w = httptest.NewRecorder()
	selfMutations := &fakeParticipantMutations{
		invite: func(context.Context, uuid.UUID, string, domain.Role, []uuid.UUID) ([]accessapp.ParticipantGrantResult, error) {
			return nil, domain.ErrCannotAddSelf
		},
	}
	h2 := NewParticipantHandlers(&fakeParticipantsManager{}, selfMutations, slog.New(slog.DiscardHandler))
	h2.InviteParticipant(w, newParticipantMutationRequest(t, http.MethodPost, "/participants/invite", &actor,
		map[string]any{fieldEmail: "me@x.ru", fieldRole: roleViewerJSON, fieldPropertyIDs: []string{propID.String()}}))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Malformed body — bad request.
	w = httptest.NewRecorder()
	h.InviteParticipant(w, newParticipantMutationRequest(t, http.MethodPost, "/participants/invite", &actor, nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Unknown role — bad request.
	w = httptest.NewRecorder()
	h.InviteParticipant(w, newParticipantMutationRequest(t, http.MethodPost, "/participants/invite", &actor,
		map[string]any{fieldEmail: testInviteeEmail, fieldRole: roleUnknownJSON, fieldPropertyIDs: []string{propID.String()}}))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Empty object list — bad request.
	w = httptest.NewRecorder()
	h.InviteParticipant(w, newParticipantMutationRequest(t, http.MethodPost, "/participants/invite", &actor,
		map[string]any{fieldEmail: testInviteeEmail, fieldRole: roleViewerJSON, fieldPropertyIDs: []string{}}))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// No session — 401.
	w = httptest.NewRecorder()
	h.InviteParticipant(w, newParticipantMutationRequest(t, http.MethodPost, "/participants/invite", nil,
		map[string]any{fieldEmail: testInviteeEmail, fieldRole: roleViewerJSON, fieldPropertyIDs: []string{propID.String()}}))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestAddParticipantPropertiesHandler checks the 200 wire and the
// privacy-preserving 404.
func TestAddParticipantPropertiesHandler(t *testing.T) {
	t.Parallel()

	actor := uuid.Must(uuid.NewV7())
	participantID := uuid.Must(uuid.NewV7()).String()
	mutations := &fakeParticipantMutations{
		addProperties: func(ctx context.Context, id uuid.UUID, got string,
			role domain.Role, ids []uuid.UUID,
		) ([]accessapp.ParticipantGrantResult, error) {
			assert.Equal(t, actor, id)
			assert.Equal(t, participantID, got)
			assert.Equal(t, domain.RoleFullAccess, role)
			return grantFixtureResults()[:1], nil
		},
	}
	h := NewParticipantHandlers(&fakeParticipantsManager{}, mutations, slog.New(slog.DiscardHandler))

	w := httptest.NewRecorder()
	h.AddParticipantProperties(w, newParticipantMutationRequest(t, http.MethodPost,
		"/participants/"+participantID+"/properties", &actor,
		map[string]any{fieldRole: roleFullAccessJSON, fieldPropertyIDs: []string{uuid.Must(uuid.NewV7()).String()}}), participantID)
	require.Equal(t, http.StatusOK, w.Code)
	var body openapi.ParticipantGrantResultsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Items, 1)

	// Unknown or out-of-scope person — the privacy-preserving 404.
	missing := &fakeParticipantMutations{
		addProperties: func(context.Context, uuid.UUID, string, domain.Role, []uuid.UUID) ([]accessapp.ParticipantGrantResult, error) {
			return nil, domain.ErrParticipantNotFound
		},
	}
	h2 := NewParticipantHandlers(&fakeParticipantsManager{}, missing, slog.New(slog.DiscardHandler))
	w = httptest.NewRecorder()
	h2.AddParticipantProperties(w, newParticipantMutationRequest(t, http.MethodPost,
		"/participants/gone/properties", &actor,
		map[string]any{fieldRole: roleViewerJSON, fieldPropertyIDs: []string{uuid.Must(uuid.NewV7()).String()}}), "gone")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestDeleteParticipantHandler checks the 204 wire, the 404 and the 401.
func TestDeleteParticipantHandler(t *testing.T) {
	t.Parallel()

	actor := uuid.Must(uuid.NewV7())
	participantID := uuid.Must(uuid.NewV7()).String()
	mutations := &fakeParticipantMutations{
		remove: func(ctx context.Context, id uuid.UUID, got string) error {
			assert.Equal(t, actor, id)
			assert.Equal(t, participantID, got)
			return nil
		},
	}
	h := NewParticipantHandlers(&fakeParticipantsManager{}, mutations, slog.New(slog.DiscardHandler))

	w := httptest.NewRecorder()
	h.DeleteParticipant(w, newParticipantMutationRequest(t, http.MethodDelete,
		"/participants/"+participantID, &actor, nil), participantID)
	assert.Equal(t, http.StatusNoContent, w.Code)

	missing := &fakeParticipantMutations{
		remove: func(context.Context, uuid.UUID, string) error { return domain.ErrParticipantNotFound },
	}
	h2 := NewParticipantHandlers(&fakeParticipantsManager{}, missing, slog.New(slog.DiscardHandler))
	w = httptest.NewRecorder()
	h2.DeleteParticipant(w, newParticipantMutationRequest(t, http.MethodDelete,
		"/participants/gone", &actor, nil), "gone")
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = httptest.NewRecorder()
	h.DeleteParticipant(w, newParticipantMutationRequest(t, http.MethodDelete,
		"/participants/"+participantID, nil, nil), participantID)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
