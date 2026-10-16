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

// fakePrefsEmail is the http package's in-memory EmailPreferencesRepository.
type fakePrefsEmail struct {
	rows map[uuid.UUID]domain.CategoryPrefs
}

func (r *fakePrefsEmail) Get(_ context.Context, userID uuid.UUID) (domain.CategoryPrefs, error) {
	if prefs, ok := r.rows[userID]; ok {
		return prefs, nil
	}
	return domain.DefaultCategoryPrefs(), nil
}

func (r *fakePrefsEmail) Set(_ context.Context, userID uuid.UUID, prefs domain.CategoryPrefs) error {
	if r.rows == nil {
		r.rows = make(map[uuid.UUID]domain.CategoryPrefs)
	}
	r.rows[userID] = prefs
	return nil
}

// fakePrefsPush is the http package's in-memory PushSubscriptionRepository.
type fakePrefsPush struct {
	subs map[string]domain.PushSubscription
}

func (r *fakePrefsPush) Upsert(_ context.Context, sub domain.PushSubscription) (domain.PushSubscription, error) {
	return sub, nil
}

func (r *fakePrefsPush) Delete(_ context.Context, _ uuid.UUID, _ string) error {
	return notificationsapp.ErrNotFound
}

func (r *fakePrefsPush) ListByUser(_ context.Context, _ uuid.UUID) ([]domain.PushSubscription, error) {
	return nil, nil
}

func (r *fakePrefsPush) GetByEndpoint(_ context.Context, userID uuid.UUID, endpoint string) (domain.PushSubscription, error) {
	sub, ok := r.subs[endpoint]
	if !ok || sub.UserID != userID {
		return domain.PushSubscription{}, notificationsapp.ErrNotFound
	}
	return sub, nil
}

func (r *fakePrefsPush) UpdatePreferences(
	_ context.Context, userID uuid.UUID, endpoint string, prefs domain.CategoryPrefs,
) (bool, error) {
	sub, ok := r.subs[endpoint]
	if !ok || sub.UserID != userID {
		return false, nil
	}
	sub.Categories = prefs
	r.subs[endpoint] = sub
	return true, nil
}

// newPrefsTestHandlers builds the settings handlers over in-memory
// repositories.
func newPrefsTestHandlers() (*NotificationPreferencesHandlers, *fakePrefsPush) {
	email, push := &fakePrefsEmail{}, &fakePrefsPush{subs: make(map[string]domain.PushSubscription)}
	svc := notificationsapp.NewSettingsService(email, push)
	return NewNotificationPreferencesHandlers(svc, slog.Default()), push
}

// JSON keys of the device settings payload, repeated across the tests.
const (
	prefsKeyEndpoint           = "endpoint"
	prefsKeyP256dh             = "p256dh"
	prefsKeyAuth               = "auth"
	prefsAuthValue             = "auth-value"
	prefsKeyCategories         = "categories"
	prefsKeyRental             = "rental"
	prefsKeyPaymentsOperations = "payments_operations"
	prefsKeySharedAccess       = "shared_access"
	prefsKeyTasks              = "tasks"
)

func prefsRequest(t *testing.T, userID uuid.UUID, method, target string, body any) *http.Request {
	t.Helper()
	ctx := httpsupport.WithUserID(context.Background(), userID)
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	r := httptest.NewRequestWithContext(ctx, method, target, reader)
	if body != nil {
		// DecodeJSONBody (#724) отвергает непустое тело без
		// application/json — как живой клиент fetch.
		r.Header.Set("Content-Type", "application/json")
	}
	return r
}

// The email matrix answers the all-on default before any PUT and echoes the
// stored matrix after it.
func TestNotificationPreferencesHandlers_EmailRoundTrip(t *testing.T) {
	t.Parallel()

	h, _ := newPrefsTestHandlers()
	user := handlerID

	w := httptest.NewRecorder()
	h.GetNotificationPreferences(w, prefsRequest(t, user, http.MethodGet, "/notification-preferences", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"email":{"rental":true,"payments_operations":true,"tasks":true,"shared_access":true}}`,
		w.Body.String(), "the untouched account reads as all-on")

	w = httptest.NewRecorder()
	h.PutNotificationPreferences(w, prefsRequest(t, user, http.MethodPut, "/notification-preferences", map[string]any{
		"email": map[string]any{"rental": true, prefsKeyPaymentsOperations: false, "tasks": true, "shared_access": false},
	}))
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"email":{"rental":true,"payments_operations":false,"tasks":true,"shared_access":false}}`,
		w.Body.String())

	w = httptest.NewRecorder()
	h.GetNotificationPreferences(w, prefsRequest(t, user, http.MethodGet, "/notification-preferences", nil))
	assert.JSONEq(t, `{"email":{"rental":true,"payments_operations":false,"tasks":true,"shared_access":false}}`,
		w.Body.String())
}

// The push preferences read and replace the device state; an unknown
// endpoint is a 404.
func TestNotificationPreferencesHandlers_PushPreferences(t *testing.T) {
	t.Parallel()

	h, push := newPrefsTestHandlers()
	user := handlerID
	endpoint := "https://push.example/prefs"

	w := httptest.NewRecorder()
	h.GetPushSubscriptionPreferences(w, prefsRequest(t, user, http.MethodGet,
		"/push/subscriptions/preferences?endpoint="+endpoint, nil),
		openapi.GetPushSubscriptionPreferencesParams{Endpoint: endpoint})
	assert.Equal(t, http.StatusNotFound, w.Code)

	push.subs[endpoint] = domain.PushSubscription{
		UserID:     user,
		Endpoint:   endpoint,
		Categories: domain.DefaultCategoryPrefs(),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	w = httptest.NewRecorder()
	h.GetPushSubscriptionPreferences(w, prefsRequest(t, user, http.MethodGet,
		"/push/subscriptions/preferences?endpoint="+endpoint, nil),
		openapi.GetPushSubscriptionPreferencesParams{Endpoint: endpoint})
	require.Equal(t, http.StatusOK, w.Code)
	// The fact of 200 = the row exists = the master is on (спека #1028);
	// the body carries only the category flags.
	assert.JSONEq(t, `{"endpoint":"`+endpoint+`",`+
		`"categories":{"rental":true,"payments_operations":true,"tasks":true,"shared_access":true}}`,
		w.Body.String())

	// The PUT replaces the flags; the subscription's keys stay.
	w = httptest.NewRecorder()
	h.PutPushSubscriptionPreferences(w, prefsRequest(t, user, http.MethodPut,
		"/push/subscriptions/preferences", map[string]any{
			prefsKeyEndpoint: endpoint,
			prefsKeyCategories: map[string]any{
				prefsKeyRental: false, prefsKeyPaymentsOperations: true, prefsKeyTasks: true, prefsKeySharedAccess: true,
			},
		}))
	require.Equal(t, http.StatusOK, w.Code)

	sub, err := h.settings.PushPreferences(context.Background(), user, endpoint)
	require.NoError(t, err)
	assert.False(t, sub.Categories.Rental)
	assert.True(t, sub.Categories.PaymentsOperations, "the flags travel with the same PUT")

	// The lying field is gone from the wire: an unknown key is rejected.
	w = httptest.NewRecorder()
	h.PutPushSubscriptionPreferences(w, prefsRequest(t, user, http.MethodPut,
		"/push/subscriptions/preferences", map[string]any{
			prefsKeyEndpoint: endpoint,
			"enabled":        false,
			prefsKeyCategories: map[string]any{
				prefsKeyRental: true, prefsKeyPaymentsOperations: true, prefsKeyTasks: true, prefsKeySharedAccess: true,
			},
		}))
	assert.Equal(t, http.StatusBadRequest, w.Code, "enabled is no longer part of the contract")

	// Another user's endpoint stays invisible.
	stranger := uuid.Must(uuid.NewV7())
	w = httptest.NewRecorder()
	h.PutPushSubscriptionPreferences(w, prefsRequest(t, stranger, http.MethodPut,
		"/push/subscriptions/preferences", map[string]any{
			prefsKeyEndpoint: endpoint, prefsKeyCategories: map[string]any{
				prefsKeyRental: true, prefsKeyPaymentsOperations: true, prefsKeyTasks: true, prefsKeySharedAccess: true,
			},
		}))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// The subscription response carries the device's category flags; enabled is
// no longer part of the contract (спека #1028 §5 — the row's existence is
// the master state, POSTing with enabled is a 400).
func TestPushSubscriptionResponse_IncludesSettings(t *testing.T) {
	t.Parallel()

	h, repo := newPushTestHandlers("test-vapid-key")
	user := uuid.Must(uuid.NewV7())

	r := newPushJSONRequest(t, http.MethodPost, "/push/subscriptions", &user, map[string]any{
		prefsKeyEndpoint: "https://push.example/with-settings",
		prefsKeyP256dh:   testP256dh,
		prefsKeyAuth:     prefsAuthValue,
		"categories": map[string]any{
			"rental": false, "payments_operations": true, prefsKeyTasks: true, prefsKeySharedAccess: true,
		},
	})
	w := httptest.NewRecorder()
	h.CreatePushSubscription(w, r)
	require.Equal(t, http.StatusCreated, w.Code)

	var resp struct {
		Categories struct {
			Rental             bool `json:"rental"`
			PaymentsOperations bool `json:"payments_operations"`
		} `json:"categories"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Categories.Rental)
	assert.True(t, resp.Categories.PaymentsOperations)

	// Omitted categories read as all-on (дефолт для чужих клиентов).
	r = newPushJSONRequest(t, http.MethodPost, "/push/subscriptions", &user, map[string]any{
		prefsKeyEndpoint: "https://push.example/plain",
		prefsKeyP256dh:   testP256dh,
		prefsKeyAuth:     prefsAuthValue,
	})
	w = httptest.NewRecorder()
	h.CreatePushSubscription(w, r)
	require.Equal(t, http.StatusCreated, w.Code)

	var plain struct {
		Categories struct {
			Rental bool `json:"rental"`
		} `json:"categories"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &plain))
	assert.True(t, plain.Categories.Rental, "omitted categories read as all-on")

	// The lying field is rejected on create too.
	r = newPushJSONRequest(t, http.MethodPost, "/push/subscriptions", &user, map[string]any{
		prefsKeyEndpoint: "https://push.example/enabled",
		prefsKeyP256dh:   testP256dh,
		prefsKeyAuth:     prefsAuthValue,
		"enabled":        false,
	})
	w = httptest.NewRecorder()
	h.CreatePushSubscription(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code, "enabled is no longer part of the contract")

	assert.Len(t, repo.subs, 2)
}
