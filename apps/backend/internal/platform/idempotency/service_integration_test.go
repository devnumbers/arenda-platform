package idempotency

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// Integration tests run against a real Postgres via TEST_DATABASE_URL and are
// skipped when it is unset — the same convention as the access-context
// fixtures. Each test holds its own rollback transaction (isolation), so the
// TTL cleanup test backdates created_at inside its own tx.

func setupIdempotencyDB(t *testing.T) (context.Context, *genpostgres.Queries, pgx.Tx, func()) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	cfg := database.DefaultPoolConfig()
	cfg.MinConns = 0
	cfg.MaxConns = 2
	ctx := context.Background()
	pool, err := database.NewPoolWithConfig(ctx, databaseURL, cfg)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	cleanup := func() {
		if err := tx.Rollback(ctx); err != nil {
			t.Errorf("rollback idempotency test tx: %v", err)
		}
	}
	return ctx, genpostgres.New(tx), tx, cleanup
}

func createOwnerFixture(t *testing.T, ctx context.Context, q *genpostgres.Queries) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	// The phone must be unique per call — derived from the uuid's random tail
	// (the access fixtures' recipe: V7 timestamps collide within a
	// millisecond, ON CONFLICT DO NOTHING then returns no rows).
	_, err = q.CreateUser(ctx, genpostgres.CreateUserParams{
		ID:    pgtype.UUID{Bytes: id, Valid: true},
		Phone: fmt.Sprintf("+7999%07d", binary.BigEndian.Uint32(id[12:])%10000000),
		Role:  "owner",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return id
}

func TestService_ReserveCompleteReplay(t *testing.T) {
	t.Parallel()
	ctx, q, tx, cleanup := setupIdempotencyDB(t)
	defer cleanup()

	owner := createOwnerFixture(t, ctx, q)
	svc := NewService(tx)
	const key = "0f0e0d0c-1111-4111-8111-000000000001"
	const endpoint = "/properties/{propertyId}/payments"
	const bodyHash = "hash-body-v1"

	// Первый запрос выигрывает ключ.
	won, err := svc.Reserve(ctx, owner, key, endpoint, bodyHash)
	if err != nil || !won.Won {
		t.Fatalf("first reserve: won=%v err=%v", won.Won, err)
	}

	// Параллельный дубликат с тем же ключом до Complete — бронь в полёте.
	if _, err := svc.Reserve(ctx, owner, key, endpoint, bodyHash); !errors.Is(err, ErrKeyInProgress) {
		t.Fatalf("in-flight duplicate: expected ErrKeyInProgress, got %v", err)
	}

	// Победитель сохраняет результат; повтор с тем же ключом и телом —
	// сохранённый ответ (паттерн Stripe), а не второе исполнение.
	if err := svc.Complete(ctx, owner, key, http.StatusCreated, testContentType, []byte(testBody)); err != nil {
		t.Fatalf("complete: %v", err)
	}
	replay, err := svc.Reserve(ctx, owner, key, endpoint, bodyHash)
	if err != nil {
		t.Fatalf("replay reserve: %v", err)
	}
	// JSONB нормализует текст (пробелы после двоеточий) — сравниваем
	// семантически, без форматных ожиданий.
	if replay.Won || replay.Replay.StatusCode != http.StatusCreated ||
		strings.ReplaceAll(string(replay.Replay.Body), " ", "") != testBody {
		t.Fatalf("replay = %+v", replay)
	}
	if replay.Replay.ContentType != testContentType {
		t.Fatalf("replay content type = %q", replay.Replay.ContentType)
	}

	// Тот же ключ с другим телом — клиентский баг, 409-сигнал.
	if _, err := svc.Reserve(ctx, owner, key, endpoint, "hash-body-v2"); !errors.Is(err, ErrKeyBodyMismatch) {
		t.Fatalf("mismatched body: expected ErrKeyBodyMismatch, got %v", err)
	}
}

func TestService_KeysScopedPerOwner(t *testing.T) {
	t.Parallel()
	ctx, q, tx, cleanup := setupIdempotencyDB(t)
	defer cleanup()

	ownerA := createOwnerFixture(t, ctx, q)
	ownerB := createOwnerFixture(t, ctx, q)
	svc := NewService(tx)
	const key = "0f0e0d0c-2222-4222-8222-000000000002"

	if won, err := svc.Reserve(ctx, ownerA, key, "/properties", "h"); err != nil || !won.Won {
		t.Fatalf("owner A reserve: won=%v err=%v", won.Won, err)
	}
	// Ключ скоупится владельцем: тот же строковый ключ другого владельца —
	// самостоятельная бронь, чужой результат не переигрывается.
	if won, err := svc.Reserve(ctx, ownerB, key, "/properties", "h"); err != nil || !won.Won {
		t.Fatalf("owner B reserve: won=%v err=%v", won.Won, err)
	}
}

func TestService_CleanupExpired(t *testing.T) {
	t.Parallel()
	ctx, q, tx, cleanup := setupIdempotencyDB(t)
	defer cleanup()

	owner := createOwnerFixture(t, ctx, q)
	svc := NewService(tx)
	const freshKey = "0f0e0d0c-3333-4333-8333-000000000003"
	const staleKey = "0f0e0d0c-3333-4333-8333-000000000004"

	if won, err := svc.Reserve(ctx, owner, freshKey, "/properties", "h"); err != nil || !won.Won {
		t.Fatalf("fresh reserve: won=%v err=%v", won.Won, err)
	}
	if won, err := svc.Reserve(ctx, owner, staleKey, "/properties", "h"); err != nil || !won.Won {
		t.Fatalf("stale reserve: won=%v err=%v", won.Won, err)
	}
	// Состарить staleKey за пределы 24-часового TTL внутри своей транзакции
	// (сырой SQL: бэктейт — тестовый приём, не продуктовый запрос).
	if _, err := tx.Exec(ctx,
		`UPDATE idempotency_keys SET created_at = now() - interval '25 hours'
		 WHERE owner_id = $1 AND key = $2`,
		owner, staleKey,
	); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	if err := svc.Cleanup(ctx); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	if _, err := svc.Reserve(ctx, owner, freshKey, "/properties", "h"); !errors.Is(err, ErrKeyInProgress) {
		t.Fatalf("fresh key must survive cleanup, got %v", err)
	}
	if won, err := svc.Reserve(ctx, owner, staleKey, "/properties", "h"); err != nil || !won.Won {
		t.Fatalf("stale key must be purged (re-reservable), won=%v err=%v", won.Won, err)
	}
}
