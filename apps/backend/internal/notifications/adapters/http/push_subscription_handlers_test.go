package http

import (
	"bytes"
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePushRepo is an in-memory PushSubscriptionRepository for handler tests.
type fakePushRepo struct {
	subs []domain.PushSubscription
}

// testP256dh is the shared P-256dh fixture of the push handler tests.
const testP256dh = "p256dh-val"

func (r *fakePushRepo) Upsert(_ context.Context, sub domain.PushSubscription) (domain.PushSubscription, error) {
	for i, s := range r.subs {
		if s.Endpoint == sub.Endpoint {
			r.subs[i].UserID = sub.UserID
			r.subs[i].P256dh = sub.P256dh
			r.subs[i].Auth = sub.Auth
			r.subs[i].ExpirationTime = sub.ExpirationTime
			r.subs[i].UpdatedAt = sub.UpdatedAt
			return r.subs[i], nil
		}
	}
	r.subs = append(r.subs, sub)
	return sub, nil
}

func (r *fakePushRepo) Delete(_ context.Context, userID uuid.UUID, endpoint string) error {
	for i, s := range r.subs {
		if s.Endpoint == endpoint && s.UserID == userID {
			r.subs = append(r.subs[:i], r.subs[i+1:]...)
			return nil
		}
	}
	return notificationsapp.ErrNotFound
}

func (r *fakePushRepo) ListByUser(_ context.Context, userID uuid.UUID) ([]domain.PushSubscription, error) {
	var out []domain.PushSubscription
	for _, s := range r.subs {
		if s.UserID == userID {
			out = append(out, s)
		}
	}
	return out, nil
}

// pushFakeClock is a deterministic clock.Clock for push handler tests.
type pushFakeClock struct{ now time.Time }

func (c pushFakeClock) Now() time.Time { return c.now }

func newPushTestHandlers(vapidKey string) (*PushSubscriptionHandlers, *fakePushRepo) {
	repo := &fakePushRepo{}
	svc := notificationsapp.NewPushSubscriptionService(repo, pushFakeClock{now: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)})
	return NewPushSubscriptionHandlers(svc, vapidKey, slog.Default()), repo
}

// newPushJSONRequest builds a request with the given user context and JSON body.
func newPushJSONRequest(t *testing.T, method, target string, userID *uuid.UUID, body any) *http.Request {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	ctx := context.Background()
	if userID != nil {
		ctx = httpsupport.WithUserID(ctx, *userID)
	}
	return httptest.NewRequestWithContext(ctx, method, target, bytes.NewReader(raw))
}

func TestGetVapidPublicKey_ReturnsKey(t *testing.T) {
	t.Parallel()

	h, _ := newPushTestHandlers("BPubKeyXXX")
	uid := uuid.Must(uuid.NewV7())
	w := httptest.NewRecorder()
	r := newPushJSONRequest(t, http.MethodGet, "/push/vapid-public-key", &uid, nil)

	h.GetVapidPublicKey(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp openapi.VapidPublicKeyResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "BPubKeyXXX", resp.PublicKey)
}

func TestGetVapidPublicKey_EmptyReturnsServiceUnavailable(t *testing.T) {
	t.Parallel()

	h, _ := newPushTestHandlers("")
	uid := uuid.Must(uuid.NewV7())
	w := httptest.NewRecorder()
	r := newPushJSONRequest(t, http.MethodGet, "/push/vapid-public-key", &uid, nil)

	h.GetVapidPublicKey(w, r)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestCreatePushSubscription_StoresAndReturns201(t *testing.T) {
	t.Parallel()

	h, repo := newPushTestHandlers("BPubKeyXXX")
	userID := uuid.Must(uuid.NewV7())
	body := openapi.PushSubscriptionCreateRequest{
		Endpoint: "https://fcm.googleapis.com/fcm/send/abc",
		P256dh:   testP256dh,
		Auth:     "auth-val",
	}

	w := httptest.NewRecorder()
	r := newPushJSONRequest(t, http.MethodPost, "/push/subscriptions", &userID, body)

	h.CreatePushSubscription(w, r)

	require.Equal(t, http.StatusCreated, w.Code)
	var resp openapi.PushSubscriptionResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, body.Endpoint, resp.Endpoint)
	require.Len(t, repo.subs, 1)
	assert.Equal(t, userID, repo.subs[0].UserID)
}

func TestCreatePushSubscription_IdempotentUpsertByEndpoint(t *testing.T) {
	t.Parallel()

	h, repo := newPushTestHandlers("BPubKeyXXX")
	userID := uuid.Must(uuid.NewV7())
	endpoint := "https://fcm.googleapis.com/fcm/send/idem"

	w1 := httptest.NewRecorder()
	h.CreatePushSubscription(w1, newPushJSONRequest(t, http.MethodPost, "/push/subscriptions", &userID, openapi.PushSubscriptionCreateRequest{
		Endpoint: endpoint, P256dh: testP256dh, Auth: "auth-1",
	}))
	require.Equal(t, http.StatusCreated, w1.Code)

	w2 := httptest.NewRecorder()
	h.CreatePushSubscription(w2, newPushJSONRequest(t, http.MethodPost, "/push/subscriptions", &userID, openapi.PushSubscriptionCreateRequest{
		Endpoint: endpoint, P256dh: testP256dh, Auth: "auth-2",
	}))
	require.Equal(t, http.StatusCreated, w2.Code)

	require.Len(t, repo.subs, 1, "upsert must not create a duplicate row")
	assert.Equal(t, "auth-2", repo.subs[0].Auth)
}

func TestCreatePushSubscription_InvalidReturns400(t *testing.T) {
	t.Parallel()

	h, _ := newPushTestHandlers("BPubKeyXXX")
	uid := uuid.Must(uuid.NewV7())
	body := openapi.PushSubscriptionCreateRequest{
		Endpoint: "not-a-url",
		P256dh:   testP256dh,
		Auth:     "auth-val",
	}

	w := httptest.NewRecorder()
	r := newPushJSONRequest(t, http.MethodPost, "/push/subscriptions", &uid, body)

	h.CreatePushSubscription(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeletePushSubscription_Returns204(t *testing.T) {
	t.Parallel()

	h, repo := newPushTestHandlers("BPubKeyXXX")
	userID := uuid.Must(uuid.NewV7())
	endpoint := "https://fcm.googleapis.com/fcm/send/del"
	repo.subs = []domain.PushSubscription{{ID: uuid.Must(uuid.NewV7()), UserID: userID, Endpoint: endpoint, P256dh: "k", Auth: "a"}}

	body := openapi.PushSubscriptionDeleteRequest{Endpoint: endpoint}
	w := httptest.NewRecorder()
	r := newPushJSONRequest(t, http.MethodDelete, "/push/subscriptions", &userID, body)

	h.DeletePushSubscription(w, r)
	assert.Equal(t, http.StatusNoContent, w.Code)
	require.Empty(t, repo.subs)
}

func TestDeletePushSubscription_MissingReturns404(t *testing.T) {
	t.Parallel()

	h, _ := newPushTestHandlers("BPubKeyXXX")
	uid := uuid.Must(uuid.NewV7())
	body := openapi.PushSubscriptionDeleteRequest{Endpoint: "https://fcm.googleapis.com/fcm/send/missing"}

	w := httptest.NewRecorder()
	r := newPushJSONRequest(t, http.MethodDelete, "/push/subscriptions", &uid, body)

	h.DeletePushSubscription(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCreatePushSubscription_Unauthorized(t *testing.T) {
	t.Parallel()

	h, _ := newPushTestHandlers("BPubKeyXXX")
	body := openapi.PushSubscriptionCreateRequest{
		Endpoint: "https://fcm.googleapis.com/fcm/send/x",
		P256dh:   "k",
		Auth:     "a",
	}
	w := httptest.NewRecorder()
	// No user id in context (nil userID).
	r := newPushJSONRequest(t, http.MethodPost, "/push/subscriptions", nil, body)
	h.CreatePushSubscription(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
