package postgres

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	pgen "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

var (
	migrateOnce sync.Once
	errMigrate  error
)

func setupIntegrationDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	migrateOnce.Do(func() {
		errMigrate = database.MigrateUp(databaseURL, "../../../../db/migrations")
	})
	if errMigrate != nil {
		t.Fatalf("migrate up: %v", errMigrate)
	}

	ctx := context.Background()
	pool, err := database.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestUserRepository_Create_ConcurrentRace(t *testing.T) {
	t.Parallel()
	pool := setupIntegrationDB(t)
	ctx := context.Background()
	repo := NewUserRepository(pool)

	email, err := domain.NewEmail("race1@example.com")
	if err != nil {
		t.Fatalf("new email: %v", err)
	}
	now := time.Now().UTC()

	user, err := domain.NewOwner(email)
	if err != nil {
		t.Fatalf("new owner: %v", err)
	}
	user.EmailVerifiedAt = &now

	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan struct {
		user domain.User
		err  error
	}, 2)
	var started atomic.Int32

	for range 2 {
		wg.Go(func() {
			started.Add(1)
			<-start
			created, createErr := repo.Create(ctx, user)
			results <- struct {
				user domain.User
				err  error
			}{created, createErr}
		})
	}

	for started.Load() < 2 {
		time.Sleep(time.Millisecond)
	}
	close(start)
	wg.Wait()
	close(results)

	var users []domain.User
	for res := range results {
		if res.err != nil {
			t.Fatalf("create returned error: %v", res.err)
		}
		users = append(users, res.user)
	}

	if len(users) != 2 {
		t.Fatalf("expected 2 results, got %d", len(users))
	}
	if users[0].ID != users[1].ID {
		t.Fatalf("expected both calls to return the same user, got %v and %v", users[0].ID, users[1].ID)
	}

	count, err := pgen.New(pool).CountUsersAdmin(ctx, pgen.CountUsersAdminParams{Email: email.String()})
	if err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one user row, got %d", count)
	}
}

// TestUserRepository_Create_ConflictingEmailResolvesToExisting pins the
// create-race contract on the canonical identifier: two creates carrying the
// same email but different aggregate ids converge on the one stored row —
// email is the account (ADR 0067), so there is no "taken" refusal here, the
// late insert resolves to the existing user.
func TestUserRepository_Create_ConflictingEmailResolvesToExisting(t *testing.T) {
	t.Parallel()
	pool := setupIntegrationDB(t)
	ctx := context.Background()
	repo := NewUserRepository(pool)

	email, err := domain.NewEmail("shared@example.com")
	if err != nil {
		t.Fatalf("new email: %v", err)
	}
	now := time.Now().UTC()

	user1, err := domain.NewOwner(email)
	if err != nil {
		t.Fatalf("new owner 1: %v", err)
	}
	user1.EmailVerifiedAt = &now

	if _, err := repo.Create(ctx, user1); err != nil {
		t.Fatalf("create first user: %v", err)
	}

	user2, err := domain.NewOwner(email)
	if err != nil {
		t.Fatalf("new owner 2: %v", err)
	}
	user2.EmailVerifiedAt = &now

	resolved, err := repo.Create(ctx, user2)
	if err != nil {
		t.Fatalf("create conflicting user: %v", err)
	}
	if resolved.ID != user1.ID {
		t.Fatalf("conflicting create resolved to %s, want the stored %s", resolved.ID, user1.ID)
	}
}
