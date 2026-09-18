package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixedClock is the deterministic clock the live-state tests inject into the
// owner calendar.
type fixedClock struct{ now time.Time }

func (c *fixedClock) Now() time.Time { return c.now }

// The account-level email matrix reads as all-on until a row exists, and PUT
// replaces the whole row (решение #738).
func TestEmailPreferencesRepository_DefaultAndRoundTrip(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()
	q := genpostgres.New(pool)
	userID := createPushTestUser(t, ctx, q)
	repo := NewEmailPreferencesRepository(pool)

	prefs, err := repo.Get(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, domain.DefaultCategoryPrefs(), prefs, "no row reads as the all-on default")

	want := domain.CategoryPrefs{Rental: true, PaymentsOperations: false, Tasks: true, SharedAccess: false}
	require.NoError(t, repo.Set(ctx, userID, want))

	got, err := repo.Get(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	// A second PUT replaces the row instead of failing on the key.
	second := domain.CategoryPrefs{Rental: false, PaymentsOperations: true, Tasks: false, SharedAccess: true}
	require.NoError(t, repo.Set(ctx, userID, second))
	got, err = repo.Get(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, second, got)
}

// The per-device settings live on the subscription: the upsert carries them,
// the preferences read and write scoped to the user, keys stay intact.
func TestPushSubscriptionRepository_SettingsRoundTrip(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()
	q := genpostgres.New(pool)
	userID := createPushTestUser(t, ctx, q)
	repo := NewPushSubscriptionRepository(pool)

	categories := domain.CategoryPrefs{Rental: true, PaymentsOperations: false, Tasks: true, SharedAccess: true}
	sub := domain.PushSubscription{
		ID:         uuid.Must(uuid.NewV7()),
		UserID:     userID,
		Endpoint:   "https://push.example/settings",
		P256dh:     "p256dh-settings",
		Auth:       "auth-settings",
		Enabled:    false,
		Categories: categories,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	stored, err := repo.Upsert(ctx, sub)
	require.NoError(t, err)
	assert.False(t, stored.Enabled)
	assert.Equal(t, categories, stored.Categories)

	// Read back by endpoint; another user's endpoint is not found.
	got, err := repo.GetByEndpoint(ctx, userID, sub.Endpoint)
	require.NoError(t, err)
	assert.False(t, got.Enabled)
	assert.Equal(t, categories, got.Categories)

	_, err = repo.GetByEndpoint(ctx, uuid.Must(uuid.NewV7()), sub.Endpoint)
	require.ErrorIs(t, err, notificationsapp.ErrNotFound, "another user's endpoint is not found")

	// The preferences update moves the master and the flags, keeps the keys.
	updated := domain.CategoryPrefs{Rental: false, PaymentsOperations: true, Tasks: true, SharedAccess: false}
	ok, err := repo.UpdatePreferences(ctx, userID, sub.Endpoint, true, updated)
	require.NoError(t, err)
	assert.True(t, ok)

	got, err = repo.GetByEndpoint(ctx, userID, sub.Endpoint)
	require.NoError(t, err)
	assert.True(t, got.Enabled)
	assert.Equal(t, updated, got.Categories)
	assert.Equal(t, sub.P256dh, got.P256dh, "the subscription's keys stay")

	// Unknown endpoint reports false.
	ok, err = repo.UpdatePreferences(ctx, userID, "https://push.example/unknown", true, domain.DefaultCategoryPrefs())
	require.NoError(t, err)
	assert.False(t, ok)
}

// Soft delete: one row, idempotent repeats, scoped to the user; «удалить
// все» hides every not-deleted row of the user only.
func TestNotificationRepository_DeleteOneAndAll(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()
	q := genpostgres.New(pool)
	userID := createPushTestUser(t, ctx, q)
	other := createPushTestUser(t, ctx, q)
	repo := NewNotificationRepository(pool)

	first := feedNotification(t, userID, "payment_due:del-1")
	second := feedNotification(t, userID, "payment_due:del-2")
	foreign := feedNotification(t, other, "payment_due:del-3")
	for _, n := range []domain.Notification{first, second, foreign} {
		inserted, err := repo.Insert(ctx, n)
		require.NoError(t, err)
		require.True(t, inserted)
	}

	ok, err := repo.Delete(ctx, userID, first.ID)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = repo.Delete(ctx, userID, first.ID)
	require.NoError(t, err)
	assert.False(t, ok, "deleting twice matches nothing")

	// The deleted row is invisible to the feed but still resolves by id
	// (the in-flight delivery keeps its row).
	page, err := repo.ListPage(ctx, userID, false, nil, uuid.Nil, 0)
	require.NoError(t, err)
	require.Len(t, page, 1)
	assert.Equal(t, second.ID, page[0].ID)
	_, err = repo.GetByID(ctx, first.ID)
	require.NoError(t, err)

	ok, err = repo.Delete(ctx, userID, foreign.ID)
	require.NoError(t, err)
	assert.False(t, ok, "another user's row is not ours to delete")

	hidden, err := repo.DeleteAll(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), hidden, "only the user's own not-deleted rows")

	page, err = repo.ListPage(ctx, userID, false, nil, uuid.Nil, 0)
	require.NoError(t, err)
	assert.Empty(t, page)

	// The foreign row survives.
	page, err = repo.ListPage(ctx, other, false, nil, uuid.Nil, 0)
	require.NoError(t, err)
	assert.Len(t, page, 1)
}
