package postgres

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// These integration tests run against a real Postgres via TEST_DATABASE_URL and
// are skipped when it is unset (same convention as the reminder policy
// tests). They cover the acceptance criteria for the push-subscriptions
// infrastructure (issue #180): upsert idempotency by endpoint, update on
// re-subscribe, delete (and 404 mapping), and list-by-user.

func setupPushDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := database.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func createPushTestUser(t *testing.T, ctx context.Context, q *genpostgres.Queries) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	_, err = q.CreateUser(ctx, genpostgres.CreateUserParams{
		ID:    pgUUID(id),
		Phone: fmt.Sprintf("+7999%07d", binary.BigEndian.Uint32(id[12:])%10000000),
		Role:  "owner",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return id
}

func TestPushSubscriptionRepository_UpsertIdempotentByEndpoint(t *testing.T) {
	pool := setupPushDB(t)
	ctx := context.Background()
	q := genpostgres.New(pool)
	userID := createPushTestUser(t, ctx, q)
	repo := NewPushSubscriptionRepository(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)
	endpoint := "https://fcm.googleapis.com/fcm/send/abc-123"

	first, err := repo.Upsert(ctx, domain.PushSubscription{
		ID:        uuidMustV7(t),
		UserID:    userID,
		Endpoint:  endpoint,
		P256dh:    "p256dh-1",
		Auth:      "auth-1",
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	// Re-subscribe on the same device: same endpoint, rotated keys.
	second, err := repo.Upsert(ctx, domain.PushSubscription{
		ID:        uuidMustV7(t),
		UserID:    userID,
		Endpoint:  endpoint,
		P256dh:    "p256dh-2",
		Auth:      "auth-2",
		CreatedAt: now.Add(time.Second),
		UpdatedAt: now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	// The row keeps its original id and created_at (UPSERT updates mutable fields only).
	if second.ID != first.ID {
		t.Errorf("expected same id on upsert, got %s vs %s", second.ID, first.ID)
	}
	if second.P256dh != "p256dh-2" {
		t.Errorf("expected updated p256dh, got %q", second.P256dh)
	}
	if second.Auth != "auth-2" {
		t.Errorf("expected updated auth, got %q", second.Auth)
	}

	subs, err := repo.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(subs) != 1 {
		t.Errorf("expected 1 subscription after upsert, got %d", len(subs))
	}
}

func TestPushSubscriptionRepository_UpsertExpirationTimeNullable(t *testing.T) {
	pool := setupPushDB(t)
	ctx := context.Background()
	q := genpostgres.New(pool)
	userID := createPushTestUser(t, ctx, q)
	repo := NewPushSubscriptionRepository(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)
	exp := now.Add(time.Hour)

	// With expiration_time.
	withExp, err := repo.Upsert(ctx, domain.PushSubscription{
		ID:             uuidMustV7(t),
		UserID:         userID,
		Endpoint:       "https://fcm.googleapis.com/fcm/send/exp",
		P256dh:         "k",
		Auth:           "a",
		ExpirationTime: &exp,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if err != nil {
		t.Fatalf("upsert with exp: %v", err)
	}
	if withExp.ExpirationTime == nil {
		t.Fatal("expected non-nil expiration_time")
	}

	// Without expiration_time (NULL).
	noExp, err := repo.Upsert(ctx, domain.PushSubscription{
		ID:        uuidMustV7(t),
		UserID:    userID,
		Endpoint:  "https://fcm.googleapis.com/fcm/send/noexp",
		P256dh:    "k",
		Auth:      "a",
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("upsert without exp: %v", err)
	}
	if noExp.ExpirationTime != nil {
		t.Fatalf("expected nil expiration_time, got %v", *noExp.ExpirationTime)
	}
}

func TestPushSubscriptionRepository_Delete(t *testing.T) {
	pool := setupPushDB(t)
	ctx := context.Background()
	q := genpostgres.New(pool)
	userID := createPushTestUser(t, ctx, q)
	repo := NewPushSubscriptionRepository(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)
	endpoint := "https://fcm.googleapis.com/fcm/send/del"

	if _, err := repo.Upsert(ctx, domain.PushSubscription{
		ID: uuidMustV7(t), UserID: userID, Endpoint: endpoint,
		P256dh: "k", Auth: "a", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	if err := repo.Delete(ctx, userID, endpoint); err != nil {
		t.Fatalf("delete: %v", err)
	}

	subs, err := repo.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(subs) != 0 {
		t.Errorf("expected 0 subscriptions after delete, got %d", len(subs))
	}

	// Deleting again returns ErrNotFound (404 in the API).
	if err := repo.Delete(ctx, userID, endpoint); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("expected ErrNotFound on second delete, got %v", err)
	}
}

func TestPushSubscriptionRepository_DeleteScopedByUser(t *testing.T) {
	pool := setupPushDB(t)
	ctx := context.Background()
	q := genpostgres.New(pool)
	ownerA := createPushTestUser(t, ctx, q)
	ownerB := createPushTestUser(t, ctx, q)
	repo := NewPushSubscriptionRepository(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)
	endpoint := "https://fcm.googleapis.com/fcm/send/shared"

	// Owner A owns the subscription.
	if _, err := repo.Upsert(ctx, domain.PushSubscription{
		ID: uuidMustV7(t), UserID: ownerA, Endpoint: endpoint,
		P256dh: "k", Auth: "a", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert A: %v", err)
	}

	// Owner B cannot delete it (scoped query returns 0 rows -> ErrNotFound).
	if err := repo.Delete(ctx, ownerB, endpoint); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("expected ErrNotFound when other user deletes, got %v", err)
	}

	// It still exists for owner A.
	subs, err := repo.ListByUser(ctx, ownerA)
	if err != nil {
		t.Fatalf("list A: %v", err)
	}
	if len(subs) != 1 {
		t.Errorf("expected A's subscription to survive B's delete, got %d", len(subs))
	}
}

func TestPushSubscriptionRepository_ListByUserOnlyOwn(t *testing.T) {
	pool := setupPushDB(t)
	ctx := context.Background()
	q := genpostgres.New(pool)
	ownerA := createPushTestUser(t, ctx, q)
	ownerB := createPushTestUser(t, ctx, q)
	repo := NewPushSubscriptionRepository(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)
	for _, u := range []uuid.UUID{ownerA, ownerB} {
		if _, err := repo.Upsert(ctx, domain.PushSubscription{
			ID: uuidMustV7(t), UserID: u, Endpoint: "https://fcm.googleapis.com/fcm/send/" + u.String(),
			P256dh: "k", Auth: "a", CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}

	aSubs, err := repo.ListByUser(ctx, ownerA)
	if err != nil {
		t.Fatalf("list A: %v", err)
	}
	if len(aSubs) != 1 || aSubs[0].UserID != ownerA {
		t.Errorf("expected only A's subscription, got %+v", aSubs)
	}
}

func uuidMustV7(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	return id
}
