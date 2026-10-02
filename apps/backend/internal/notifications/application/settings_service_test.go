package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeEmailPrefs is an in-memory EmailPreferencesRepository: rows nil means
// the user never stored settings (the all-on default path).
type fakeEmailPrefs struct {
	rows map[uuid.UUID]domain.CategoryPrefs
}

func (r *fakeEmailPrefs) Get(_ context.Context, userID uuid.UUID) (domain.CategoryPrefs, error) {
	if prefs, ok := r.rows[userID]; ok {
		return prefs, nil
	}
	return domain.DefaultCategoryPrefs(), nil
}

func (r *fakeEmailPrefs) Set(_ context.Context, userID uuid.UUID, prefs domain.CategoryPrefs) error {
	if r.rows == nil {
		r.rows = make(map[uuid.UUID]domain.CategoryPrefs)
	}
	r.rows[userID] = prefs
	return nil
}

// fakePushSettings is an in-memory PushSubscriptionRepository reduced to the
// settings use.
type fakePushSettings struct {
	subs map[string]domain.PushSubscription // key: endpoint (single user in tests)
}

func (r *fakePushSettings) Upsert(_ context.Context, sub domain.PushSubscription) (domain.PushSubscription, error) {
	return sub, nil
}

func (r *fakePushSettings) Delete(_ context.Context, _ uuid.UUID, _ string) error {
	return ErrNotFound
}

func (r *fakePushSettings) ListByUser(_ context.Context, _ uuid.UUID) ([]domain.PushSubscription, error) {
	return nil, nil
}

func (r *fakePushSettings) GetByEndpoint(_ context.Context, _ uuid.UUID, endpoint string) (domain.PushSubscription, error) {
	sub, ok := r.subs[endpoint]
	if !ok {
		return domain.PushSubscription{}, ErrNotFound
	}
	return sub, nil
}

func (r *fakePushSettings) UpdatePreferences(
	_ context.Context, _ uuid.UUID, endpoint string, prefs domain.CategoryPrefs,
) (bool, error) {
	sub, ok := r.subs[endpoint]
	if !ok {
		return false, nil
	}
	sub.Categories = prefs
	r.subs[endpoint] = sub
	return true, nil
}

func newSettingsTestService() (*SettingsService, *fakeEmailPrefs, *fakePushSettings) {
	email, push := &fakeEmailPrefs{}, &fakePushSettings{subs: make(map[string]domain.PushSubscription)}
	return NewSettingsService(email, push), email, push
}

// The account-level email matrix defaults to all-on: a user without a
// settings row reads as «всё включено» (решение #738).
func TestSettingsService_EmailPreferencesDefaultAllOn(t *testing.T) {
	t.Parallel()

	svc, email, _ := newSettingsTestService()
	user := uuid.Must(uuid.NewV7())

	prefs, err := svc.EmailPreferences(context.Background(), user)
	require.NoError(t, err)
	assert.Equal(t, domain.DefaultCategoryPrefs(), prefs)
	assert.Nil(t, email.rows, "reading never inserts a settings row")
}

func TestSettingsService_SetEmailPreferencesReplaces(t *testing.T) {
	t.Parallel()

	svc, _, _ := newSettingsTestService()
	user := uuid.Must(uuid.NewV7())
	want := domain.CategoryPrefs{Rental: true, PaymentsOperations: false, Tasks: true, SharedAccess: false}

	require.NoError(t, svc.SetEmailPreferences(context.Background(), user, want))

	got, err := svc.EmailPreferences(context.Background(), user)
	require.NoError(t, err)
	assert.Equal(t, want, got, "PUT is a full replacement")
}

func TestSettingsService_PushPreferencesNotFound(t *testing.T) {
	t.Parallel()

	svc, _, _ := newSettingsTestService()

	_, err := svc.PushPreferences(context.Background(), uuid.Must(uuid.NewV7()), "https://push.example/unknown")
	require.ErrorIs(t, err, ErrNotFound)

	err = svc.SetPushPreferences(context.Background(), uuid.Must(uuid.NewV7()), "https://push.example/unknown",
		domain.DefaultCategoryPrefs())
	require.ErrorIs(t, err, ErrNotFound)
}

// PUT replaces the device's category flags and nothing else (спека #1028 §5:
// строгий апдейт существующей подписки — мастер-состояние это само
// существование строки, выключение выполняет DELETE).
func TestSettingsService_SetPushPreferencesReplacesCategories(t *testing.T) {
	t.Parallel()

	svc, _, push := newSettingsTestService()
	user := uuid.Must(uuid.NewV7())
	endpoint := "https://push.example/device"
	push.subs[endpoint] = domain.PushSubscription{UserID: user, Endpoint: endpoint, P256dh: "p", Auth: "a"}

	want := domain.CategoryPrefs{Rental: true, PaymentsOperations: false, Tasks: true, SharedAccess: true}
	require.NoError(t, svc.SetPushPreferences(context.Background(), user, endpoint, want))

	sub, err := svc.PushPreferences(context.Background(), user, endpoint)
	require.NoError(t, err)
	assert.Equal(t, want, sub.Categories, "PUT is a full replacement of the category flags")
	assert.Equal(t, "p", sub.P256dh, "the subscription's keys stay")
}

func TestSettingsService_BlankEndpointInvalid(t *testing.T) {
	t.Parallel()

	svc, _, _ := newSettingsTestService()
	user := uuid.Must(uuid.NewV7())

	_, err := svc.PushPreferences(context.Background(), user, "")
	require.ErrorIs(t, err, ErrInvalidPushSubscription)

	err = svc.SetPushPreferences(context.Background(), user, "", domain.DefaultCategoryPrefs())
	require.ErrorIs(t, err, ErrInvalidPushSubscription)
}
