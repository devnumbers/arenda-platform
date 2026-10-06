//go:build integration

package postgres_test

// Интеграционное семейство гарда удаления (тикет #632): проверка
// незавершённой аренды и упорядоченный снос строк аренд до строки объекта.
// Порядок каскадов (аренды против платежей) при удалении строки объекта
// Postgres не гарантирует, а незавершённая аренда ссылается на свой платёж
// RESTRICT-ом rentals.payment_id — поэтому снос аренд делается явным, до
// строки объекта (ADR 0025 §2), и каскад проходит чисто при любом порядке
// триггеров сервера. Завершённая аренда ссылки не несёт (ревизия #1161:
// платёж удалён Завершением) и каскаду не мешает.

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	rentalspg "github.com/nambers/arenda-planform/apps/backend/internal/rentals/adapters/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rollbackQuietly гасит ErrTxClosed — после успешного коммита откат не
// нужен, остальные ошибки отката фатальны.
func rollbackQuietly(ctx context.Context, t *testing.T, tx pgx.Tx) {
	t.Helper()
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatalf("rollback: %v", err)
	}
}

func TestDeletionGuard_HasUnfinishedTracksCompletion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setupOccupancyReader(t)
	propID := f.seedOwnerWithProperty(ctx, t)
	ownerID := f.ownerOf(ctx, t, propID)
	guard := rentalspg.NewDeletionGuard()

	tx, err := f.pool.Begin(ctx)
	require.NoError(t, err)
	defer rollbackQuietly(ctx, t, tx)

	unfinished, err := guard.HasUnfinished(ctx, tx, ownerID, propID)
	require.NoError(t, err)
	assert.False(t, unfinished, "without a rental the property is free")

	end := mustOccDate("2027-01-01")
	f.seedRental(ctx, t, propID, mustOccDate("2026-01-01"), &end, nil)

	unfinished, err = guard.HasUnfinished(ctx, tx, ownerID, propID)
	require.NoError(t, err)
	assert.True(t, unfinished, "the unfinished rental occupies the property")

	completed := mustOccDate("2026-09-01")
	_, err = tx.Exec(ctx,
		`UPDATE rentals SET completed_date = $1, payment_id = NULL,
		                       rent_amount_kopecks = 5000000, rent_payment_day = 15,
		                       rent_auto_pay = false
		 WHERE property_id = $2`,
		completed, propID)
	require.NoError(t, err)

	unfinished, err = guard.HasUnfinished(ctx, tx, ownerID, propID)
	require.NoError(t, err)
	assert.False(t, unfinished, "the completed rental no longer blocks the delete")
}

func TestDeletionGuard_TeardownReleasesRestrictBeforePropertyDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setupOccupancyReader(t)
	propID := f.seedOwnerWithProperty(ctx, t)
	ownerID := f.ownerOf(ctx, t, propID)
	end := mustOccDate("2026-06-01")
	completed := mustOccDate("2026-06-01")
	f.seedRental(ctx, t, propID, mustOccDate("2026-01-01"), &end, &completed)
	guard := rentalspg.NewDeletionGuard()

	// Снос аренд до строки объекта — в транзакции удаления: строка объекта
	// уходит без встречи с RESTRICT при любом порядке триггеров, каскад
	// забирает платёж.
	tx, err := f.pool.Begin(ctx)
	require.NoError(t, err)
	defer rollbackQuietly(ctx, t, tx)
	require.NoError(t, guard.DeleteByProperty(ctx, tx, ownerID, propID))
	_, err = tx.Exec(ctx, `DELETE FROM properties WHERE id = $1`, propID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	var rentals, payments int
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM rentals WHERE property_id = $1`, propID).Scan(&rentals))
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM payments WHERE property_id = $1`, propID).Scan(&payments))
	assert.Zero(t, rentals, "the teardown removed the rental rows")
	assert.Zero(t, payments, "the managed payment cascaded with the property")
}

func TestDeletionGuard_TeardownIsScopedToTheProperty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setupOccupancyReader(t)
	propID := f.seedOwnerWithProperty(ctx, t)
	otherID := f.seedOwnerWithProperty(ctx, t)
	end := mustOccDate("2026-06-01")
	completed := mustOccDate("2026-06-01")
	f.seedRental(ctx, t, propID, mustOccDate("2026-01-01"), &end, &completed)
	f.seedRental(ctx, t, otherID, mustOccDate("2026-01-01"), &end, &completed)
	ownerID := f.ownerOf(ctx, t, propID)
	guard := rentalspg.NewDeletionGuard()

	tx, err := f.pool.Begin(ctx)
	require.NoError(t, err)
	defer rollbackQuietly(ctx, t, tx)
	require.NoError(t, guard.DeleteByProperty(ctx, tx, ownerID, propID))
	require.NoError(t, tx.Commit(ctx))

	var otherRentals int
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM rentals WHERE property_id = $1`, otherID).Scan(&otherRentals))
	assert.Equal(t, 1, otherRentals, "the other property's rental survives")
}
