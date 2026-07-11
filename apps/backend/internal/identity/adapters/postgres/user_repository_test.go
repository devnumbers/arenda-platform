package postgres

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
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

func noopEncryptor(t *testing.T) encryption.Encryptor {
	t.Helper()
	enc, err := encryption.NewEncryptor("")
	if err != nil {
		t.Fatalf("create noop encryptor: %v", err)
	}
	return enc
}

func TestUserRepository_Create_ConcurrentRace(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx := context.Background()
	repo := NewUserRepository(pool, noopEncryptor(t))

	phone, err := domain.NewPhone("+79990000001")
	if err != nil {
		t.Fatalf("new phone: %v", err)
	}
	email, err := domain.NewEmail("race1@example.com")
	if err != nil {
		t.Fatalf("new email: %v", err)
	}
	now := time.Now().UTC()

	user, err := domain.NewOwner(phone)
	if err != nil {
		t.Fatalf("new owner: %v", err)
	}
	user.Email = &email
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

	count, err := pgen.New(pool).CountUsersAdmin(ctx, pgen.CountUsersAdminParams{Phone: phone.String()})
	if err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one user row, got %d", count)
	}
}

func TestUserRepository_Create_EmailAlreadyTakenDifferentPhone(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx := context.Background()
	repo := NewUserRepository(pool, noopEncryptor(t))

	email, err := domain.NewEmail("shared@example.com")
	if err != nil {
		t.Fatalf("new email: %v", err)
	}
	now := time.Now().UTC()

	phone1, err := domain.NewPhone("+79990000002")
	if err != nil {
		t.Fatalf("new phone 1: %v", err)
	}
	user1, err := domain.NewOwner(phone1)
	if err != nil {
		t.Fatalf("new owner 1: %v", err)
	}
	user1.Email = &email
	user1.EmailVerifiedAt = &now

	if _, err := repo.Create(ctx, user1); err != nil {
		t.Fatalf("create first user: %v", err)
	}

	phone2, err := domain.NewPhone("+79990000003")
	if err != nil {
		t.Fatalf("new phone 2: %v", err)
	}
	user2, err := domain.NewOwner(phone2)
	if err != nil {
		t.Fatalf("new owner 2: %v", err)
	}
	user2.Email = &email
	user2.EmailVerifiedAt = &now

	_, err = repo.Create(ctx, user2)
	if !errors.Is(err, application.ErrEmailAlreadyTaken) {
		t.Fatalf("expected ErrEmailAlreadyTaken, got %v", err)
	}
}
