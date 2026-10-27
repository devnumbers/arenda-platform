//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The blur frame's life in the row (ADR 0065 аменд, решение #1594, тикет
// #1603): SetPhoto writes the frame with the key, the conditional
// SetPhotoBlurIfKey lands the lazily materialized frame only under its own
// photo, and the heal clears the frame with the key.

func TestUserRepository_PhotoBlurRoundTrip(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	ctx := context.Background()
	repo := NewUserRepository(pool)
	user := seedUser(t, ctx, repo, "blur-roundtrip@example.com")

	key := "photos/01987654-3210-7abc-9def-0123456789ab.jpg"
	contentType := "image/jpeg"
	blur := "ZnJhbWU="
	updated, err := repo.SetPhoto(ctx, user.ID, &key, &contentType, &blur)
	require.NoError(t, err)
	assert.Equal(t, &key, updated.PhotoKey)
	require.NotNil(t, updated.PhotoBlur)
	assert.Equal(t, blur, *updated.PhotoBlur, "the frame lands next to the key")

	// The lazy materialization's conditional write: the row's own key takes
	// the frame, a foreign key does not.
	rows, err := repo.SetPhotoBlurIfKey(ctx, user.ID, key, "bmV3ZnJhbWU=")
	require.NoError(t, err)
	assert.Equal(t, int64(1), rows)

	rows, err = repo.SetPhotoBlurIfKey(ctx, user.ID, "photos/stale.jpg", "b3RoZXI=")
	require.NoError(t, err)
	assert.Zero(t, rows, "a foreign key must not stamp the row")

	// The heal clears the frame with the key.
	rows, err = repo.ClearPhotoIfKey(ctx, user.ID, key)
	require.NoError(t, err)
	assert.Equal(t, int64(1), rows)
	after, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Nil(t, after.PhotoKey)
	assert.Nil(t, after.PhotoBlur, "the frame shares the key's fate")
}
