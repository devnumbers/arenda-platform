package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/generated/postgres"
)

func setupSubscriptionRepositoryIntegrationTest(t *testing.T) (context.Context, pgx.Tx, func(), *SubscriptionRepository, uuid.UUID, uuid.UUID) {
	t.Helper()
	pool := setupIntegrationDB(t)
	ctx, tx, cleanup := beginTx(t, pool)

	q := genpostgres.New(tx)
	userID := createTestUser(t, ctx, q)
	tariff := getTestTariff(t, ctx, q, "pro")
	tariffID := uuid.UUID(tariff.ID.Bytes)

	repo := NewSubscriptionRepository(tx)
	return ctx, tx, cleanup, repo, userID, tariffID
}

func TestSubscriptionRepositoryIntegration_GetByID(t *testing.T) {
	ctx, _, cleanup, repo, userID, tariffID := setupSubscriptionRepositoryIntegrationTest(t)
	defer cleanup()

	sub, err := domain.NewOwnerSubscription(userID, tariffID)
	if err != nil {
		t.Fatalf("new subscription: %v", err)
	}
	created, err := repo.Create(ctx, sub)
	if err != nil {
		t.Fatalf("Create error = %v", err)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID error = %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("GetByID ID = %v, want %v", got.ID, created.ID)
	}
	if got.UserID != userID {
		t.Errorf("GetByID UserID = %v, want %v", got.UserID, userID)
	}

	randomID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	_, err = repo.GetByID(ctx, randomID)
	if !errors.Is(err, application.ErrNotFound) {
		t.Errorf("GetByID not found error = %v, want ErrNotFound", err)
	}
}
