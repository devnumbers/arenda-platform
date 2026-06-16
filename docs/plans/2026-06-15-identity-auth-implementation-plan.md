# Identity Authentication Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the first vertical slice: SMS login/registration, server-side sessions, and default Basic subscription for new owners.

**Architecture:** DDD modular monolith with three bounded contexts (`identity`, `billing`, `platform`). Layer direction: transport/adapters → application → domain. OpenAPI-first HTTP contract, sqlc + pgx for persistence, golang-migrate for migrations.

**Tech Stack:** Go 1.26, pgx/v5, sqlc, oapi-codegen, golang-migrate, PostgreSQL 18, Docker Compose.

---

## Important Note on Testing

The user explicitly requested **no automated tests in this slice**. Manual verification via Postman/curl is required after each task group. Tests will be added in a later slice.

---

## Task 1: Module Skeleton

**Files:**
- Create: `go.work`
- Create: `apps/backend/go.mod`
- Modify: `apps/backend/AGENTS.md` (no changes needed)

**Step 1: Create workspace file**

Create `go.work` at repo root:

```go
go 1.26

use ./apps/backend
```

**Step 2: Create backend module**

Create `apps/backend/go.mod`:

```go
module github.com/nambers/arenda-planform/apps/backend

go 1.26

require (
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.7.6
	github.com/oapi-codegen/runtime v1.4.1
)
```

**Step 3: Download dependencies**

Run:

```bash
cd apps/backend && go mod tidy
```

Expected: `go.sum` created, no errors.

**Step 4: Verify build**

Run:

```bash
cd apps/backend && go build ./...
```

Expected: `no packages to build` or success.

---

## Task 2: Local Infrastructure and Migrations

**Files:**
- Create: `apps/backend/db/migrations/000001_init_schema.up.sql`
- Create: `apps/backend/db/migrations/000001_init_schema.down.sql`
- Modify: `.env.example`
- Modify: `Makefile`

**Step 1: Update `.env.example`**

Append / update values:

```bash
APP_ENV=local
HTTP_ADDR=:8080
LOG_LEVEL=info

POSTGRES_PORT=5433
DATABASE_URL=postgres://arenda:arenda@localhost:5433/arenda?sslmode=disable
MIGRATIONS_DIR=apps/backend/db/migrations

COOKIE_SECURE=false
SMS_SENDER=fake
```

**Step 2: Add migration commands to `Makefile`**

Add targets:

```makefile
migrate-up:
	set -a; . ./.env; set +a; go run github.com/golang-migrate/migrate/v4/cmd/migrate@latest -database "$$DATABASE_URL" -path "$$MIGRATIONS_DIR" up

migrate-down:
	set -a; . ./.env; set +a; go run github.com/golang-migrate/migrate/v4/cmd/migrate@latest -database "$$DATABASE_URL" -path "$$MIGRATIONS_DIR" down 1
```

**Step 3: Write first migration**

Create `apps/backend/db/migrations/000001_init_schema.up.sql`:

```sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone TEXT NOT NULL UNIQUE,
    role TEXT NOT NULL CHECK (role IN ('owner', 'admin')),
    name TEXT,
    surname TEXT,
    patronymic TEXT,
    email TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sms_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    phone TEXT NOT NULL,
    code_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sms_codes_phone_created ON sms_codes(phone, created_at DESC);

CREATE TABLE login_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone TEXT NOT NULL UNIQUE,
    failures INT NOT NULL DEFAULT 0,
    first_failure_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_failure_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sessions_token_hash ON sessions(token_hash);

CREATE TABLE tariffs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    active_property_limit INT NOT NULL,
    monthly_price NUMERIC(14,2) NOT NULL,
    yearly_price NUMERIC(14,2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE user_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    tariff_id UUID NOT NULL REFERENCES tariffs(id),
    source TEXT NOT NULL CHECK (source IN ('paid', 'service')),
    status TEXT NOT NULL CHECK (status IN ('active', 'blocked', 'cancelled')),
    valid_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seed basic tariff
INSERT INTO tariffs (id, name, active_property_limit, monthly_price, yearly_price)
VALUES (gen_random_uuid(), 'basic', 1, 0, 0);
```

**Step 4: Write down migration**

Create `apps/backend/db/migrations/000001_init_schema.down.sql`:

```sql
DROP TABLE IF EXISTS user_subscriptions;
DROP TABLE IF EXISTS tariffs;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS login_attempts;
DROP TABLE IF EXISTS sms_codes;
DROP TABLE IF EXISTS users;
```

**Step 5: Start infrastructure and run migration**

Run:

```bash
cp .env.example .env
make local-infra-up
sleep 5
make migrate-up
```

Expected: PostgreSQL container running, migration applied with no errors.

---

## Task 3: sqlc Setup and Queries

**Files:**
- Create: `apps/backend/sqlc.yaml`
- Create: `apps/backend/db/queries/identity.sql`
- Create: `apps/backend/db/queries/billing.sql`

**Step 1: Create sqlc config**

Create `apps/backend/sqlc.yaml`:

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "db/queries"
    schema: "db/migrations"
    gen:
      go:
        package: "postgres"
        out: "internal/generated/postgres"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_interface: true
        emit_prepared_queries: false
        emit_exact_table_names: false
        emit_empty_slices: true
```

**Step 2: Write identity queries**

Create `apps/backend/db/queries/identity.sql`:

```sql
-- name: GetUserByPhone :one
SELECT * FROM users WHERE phone = $1;

-- name: CreateUser :one
INSERT INTO users (phone, role) VALUES ($1, $2) RETURNING *;

-- name: GetLatestSMSCodeByPhone :one
SELECT * FROM sms_codes
WHERE phone = $1 AND used = false
ORDER BY created_at DESC
LIMIT 1;

-- name: CreateSMSCode :one
INSERT INTO sms_codes (user_id, phone, code_hash, expires_at)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: MarkSMSCodeUsed :exec
UPDATE sms_codes SET used = true WHERE id = $1;

-- name: GetLoginAttemptByPhone :one
SELECT * FROM login_attempts WHERE phone = $1;

-- name: UpsertLoginAttempt :exec
INSERT INTO login_attempts (phone, failures, first_failure_at, last_failure_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (phone) DO UPDATE SET
    failures = EXCLUDED.failures,
    first_failure_at = EXCLUDED.first_failure_at,
    last_failure_at = EXCLUDED.last_failure_at;

-- name: CreateSession :one
INSERT INTO sessions (user_id, token_hash, expires_at)
VALUES ($1, $2, $3) RETURNING *;

-- name: DeleteSessionByTokenHash :exec
DELETE FROM sessions WHERE token_hash = $1;

-- name: GetSessionByTokenHash :one
SELECT s.*, u.id as user_id, u.phone, u.role, u.name, u.surname, u.patronymic, u.email
FROM sessions s
JOIN users u ON s.user_id = u.id
WHERE s.token_hash = $1 AND s.expires_at > now();
```

**Step 3: Write billing queries**

Create `apps/backend/db/queries/billing.sql`:

```sql
-- name: GetTariffByName :one
SELECT * FROM tariffs WHERE name = $1;

-- name: CreateSubscription :one
INSERT INTO user_subscriptions (user_id, tariff_id, source, status)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetSubscriptionByUserID :one
SELECT * FROM user_subscriptions WHERE user_id = $1;
```

**Step 4: Generate sqlc code**

Run:

```bash
cd apps/backend && go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
```

Expected: `internal/generated/postgres` created with `.go` files.

---

## Task 4: Platform Config and Database

**Files:**
- Create: `apps/backend/internal/platform/config/config.go`
- Create: `apps/backend/internal/platform/database/database.go`

**Step 1: Config**

Create `apps/backend/internal/platform/config/config.go`:

```go
package config

import (
	"fmt"
	"os"
)

type Config struct {
	AppEnv        string
	HTTPAddr      string
	LogLevel      string
	DatabaseURL   string
	MigrationsDir string
	CookieSecure  bool
	SMSSender     string
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:        os.Getenv("APP_ENV"),
		HTTPAddr:      os.Getenv("HTTP_ADDR"),
		LogLevel:      os.Getenv("LOG_LEVEL"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		MigrationsDir: os.Getenv("MIGRATIONS_DIR"),
		SMSSender:     os.Getenv("SMS_SENDER"),
	}

	if cfg.AppEnv == "" {
		cfg.AppEnv = "local"
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	cookieSecure := os.Getenv("COOKIE_SECURE")
	cfg.CookieSecure = cookieSecure == "true" || cookieSecure == "1"

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.MigrationsDir == "" {
		return Config{}, fmt.Errorf("MIGRATIONS_DIR is required")
	}

	return cfg, nil
}
```

**Step 2: Database pool and migrations**

Create `apps/backend/internal/platform/database/database.go`:

```go
package database

import (
	"context"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

func MigrateUp(databaseURL, migrationsDir string) error {
	m, err := migrate.New(
		"file://"+migrationsDir,
		databaseURL,
	)
	if err != nil {
		return fmt.Errorf("create migrate: %w", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
```

**Step 3: Verify build**

Run:

```bash
cd apps/backend && go build ./...
```

Expected: success.

---

## Task 5: Identity Domain

**Files:**
- Create: `apps/backend/internal/identity/domain/phone.go`
- Create: `apps/backend/internal/identity/domain/sms.go`
- Create: `apps/backend/internal/identity/domain/user.go`
- Create: `apps/backend/internal/identity/domain/session.go`

**Step 1: Phone value object**

Create `apps/backend/internal/identity/domain/phone.go`:

```go
package domain

import (
	"errors"
	"regexp"
	"strings"
)

var ErrInvalidPhone = errors.New("invalid russian phone number")

var phoneRegex = regexp.MustCompile(`^(?:\+7|7|8)(\d{10})$`)

type Phone string

func NewPhone(raw string) (Phone, error) {
	digits := phoneRegex.FindStringSubmatch(strings.TrimSpace(raw))
	if digits == nil {
		return "", ErrInvalidPhone
	}
	return Phone("+7" + digits[1]), nil
}

func (p Phone) String() string {
	return string(p)
}
```

**Step 2: SMS code and attempt window**

Create `apps/backend/internal/identity/domain/sms.go`:

```go
package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
)

const (
	SMSCodeTTL          = 5 * time.Minute
	SMSAttemptWindowTTL = 30 * time.Minute
	MaxSMSFailures      = 5
)

var (
	ErrSMSCodeInvalid  = errors.New("sms code invalid")
	ErrTooManyAttempts = errors.New("too many attempts")
)

type SMSCode struct {
	Phone     Phone
	CodeHash  string
	ExpiresAt time.Time
	Used      bool
}

func NewSMSCode(phone Phone, code string, now time.Time) SMSCode {
	return SMSCode{
		Phone:     phone,
		CodeHash:  hashCode(phone.String(), code),
		ExpiresAt: now.Add(SMSCodeTTL),
		Used:      false,
	}
}

func (c *SMSCode) Verify(code string, now time.Time) error {
	if c.Used || now.After(c.ExpiresAt) || hashCode(c.Phone.String(), code) != c.CodeHash {
		return ErrSMSCodeInvalid
	}
	c.Used = true
	return nil
}

func hashCode(phone, code string) string {
	sum := sha256.Sum256([]byte(phone + ":" + code))
	return hex.EncodeToString(sum[:])
}

type AttemptWindow struct {
	Failures       int
	FirstFailureAt time.Time
	LastFailureAt  time.Time
}

func NewAttemptWindow(now time.Time) AttemptWindow {
	return AttemptWindow{FirstFailureAt: now, LastFailureAt: now}
}

func (w *AttemptWindow) RecordFailure(now time.Time) error {
	if w.Failures == 0 || now.Sub(w.FirstFailureAt) > SMSAttemptWindowTTL {
		w.FirstFailureAt = now
		w.Failures = 0
	}
	w.Failures++
	w.LastFailureAt = now
	if w.Failures >= MaxSMSFailures {
		return ErrTooManyAttempts
	}
	return nil
}

func (w *AttemptWindow) Blocked(now time.Time) bool {
	if w.Failures < MaxSMSFailures {
		return false
	}
	return now.Before(w.FirstFailureAt.Add(SMSAttemptWindowTTL))
}
```

**Step 3: User and Role**

Create `apps/backend/internal/identity/domain/user.go`:

```go
package domain

import "github.com/google/uuid"

type Role string

const (
	RoleOwner Role = "owner"
	RoleAdmin Role = "admin"
)

type User struct {
	ID            uuid.UUID
	Phone         Phone
	Role          Role
	Name          *string
	Surname       *string
	Patronymic    *string
	Email         *string
}

func NewOwner(phone Phone) User {
	return User{
		ID:    uuid.Must(uuid.NewRandom()),
		Phone: phone,
		Role:  RoleOwner,
	}
}
```

**Step 4: Session**

Create `apps/backend/internal/identity/domain/session.go`:

```go
package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const SessionTTL = 30 * 24 * time.Hour

type Session struct {
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
}

type RawSession struct {
	Token   string
	Session Session
}

func NewSession(userID uuid.UUID, now time.Time) (RawSession, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return RawSession{}, fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(token))
	return RawSession{
		Token: token,
		Session: Session{
			UserID:    userID,
			TokenHash: hex.EncodeToString(sum[:]),
			ExpiresAt: now.Add(SessionTTL),
		},
	}, nil
}
```

---

## Task 6: Billing Domain

**Files:**
- Create: `apps/backend/internal/billing/domain/tariff.go`
- Create: `apps/backend/internal/billing/domain/subscription.go`

**Step 1: Tariff**

Create `apps/backend/internal/billing/domain/tariff.go`:

```go
package domain

import (
	"errors"

	"github.com/google/uuid"
)

var ErrTariffNotFound = errors.New("tariff not found")

type TariffName string

const TariffBasic TariffName = "basic"

type Tariff struct {
	ID                   uuid.UUID
	Name                 TariffName
	ActivePropertyLimit  int
	MonthlyPrice         int64 // kopecks
	YearlyPrice          int64 // kopecks
}
```

**Step 2: Subscription**

Create `apps/backend/internal/billing/domain/subscription.go`:

```go
package domain

import (
	"github.com/google/uuid"
)

type SubscriptionSource string

const (
	SubscriptionSourcePaid   SubscriptionSource = "paid"
	SubscriptionSourceService SubscriptionSource = "service"
)

type SubscriptionStatus string

const (
	SubscriptionStatusActive    SubscriptionStatus = "active"
	SubscriptionStatusBlocked   SubscriptionStatus = "blocked"
	SubscriptionStatusCancelled SubscriptionStatus = "cancelled"
)

type Subscription struct {
	ID       uuid.UUID
	UserID   uuid.UUID
	TariffID uuid.UUID
	Source   SubscriptionSource
	Status   SubscriptionStatus
}

func NewOwnerSubscription(userID, tariffID uuid.UUID) Subscription {
	return Subscription{
		UserID:   userID,
		TariffID: tariffID,
		Source:   SubscriptionSourcePaid,
		Status:   SubscriptionStatusActive,
	}
}
```

---

## Task 7: Identity Application

**Files:**
- Create: `apps/backend/internal/identity/application/ports.go`
- Create: `apps/backend/internal/identity/application/service.go`

**Step 1: Ports**

Create `apps/backend/internal/identity/application/ports.go`:

```go
package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

type Clock interface {
	Now() time.Time
}

type Sender interface {
	Send(ctx context.Context, phone domain.Phone, message string) error
}

type UserRepository interface {
	GetByPhone(ctx context.Context, phone domain.Phone) (domain.User, error)
	Create(ctx context.Context, user domain.User) error
}

type SMSCodeRepository interface {
	Save(ctx context.Context, code domain.SMSCode) error
	GetLatestByPhone(ctx context.Context, phone domain.Phone) (domain.SMSCode, error)
	MarkUsed(ctx context.Context, phone domain.Phone) error
}

type AttemptRepository interface {
	GetByPhone(ctx context.Context, phone domain.Phone) (domain.AttemptWindow, error)
	Save(ctx context.Context, phone domain.Phone, window domain.AttemptWindow) error
}

type SessionRepository interface {
	Create(ctx context.Context, session domain.Session) error
	DeleteByTokenHash(ctx context.Context, tokenHash string) error
}
```

**Step 2: Service**

Create `apps/backend/internal/identity/application/service.go`:

```go
package application

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

var ErrUserBlocked = errors.New("user is temporarily blocked")

const minSendInterval = 1 * time.Minute

type BillingService interface {
	CreateDefaultSubscriptionForOwner(ctx context.Context, userID uuid.UUID) error
}

type AuthService struct {
	users    UserRepository
	codes    SMSCodeRepository
	attempts AttemptRepository
	sessions SessionRepository
	sender   Sender
	clock    Clock
	billing  BillingService
}

func NewAuthService(users UserRepository, codes SMSCodeRepository, attempts AttemptRepository, sessions SessionRepository, sender Sender, clock Clock, billing BillingService) AuthService {
	return AuthService{
		users:    users,
		codes:    codes,
		attempts: attempts,
		sessions: sessions,
		sender:   sender,
		clock:    clock,
		billing:  billing,
	}
}

func (s *AuthService) SendCode(ctx context.Context, phone domain.Phone) error {
	now := s.clock.Now()

	window, err := s.attempts.GetByPhone(ctx, phone)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("get attempts: %w", err)
	}
	if window.Blocked(now) {
		return ErrUserBlocked
	}

	latest, err := s.codes.GetLatestByPhone(ctx, phone)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("get latest code: %w", err)
	}
	if !latest.Used && latest.ExpiresAt.After(now) && now.Sub(latest.ExpiresAt.Add(-domain.SMSCodeTTL)) < minSendInterval {
		return errors.New("code sent too recently")
	}

	code := generateCode()
	sms := domain.NewSMSCode(phone, code, now)
	if err := s.codes.Save(ctx, sms); err != nil {
		return fmt.Errorf("save code: %w", err)
	}

	return s.sender.Send(ctx, phone, fmt.Sprintf("Your code: %s", code))
}

func (s *AuthService) VerifyCode(ctx context.Context, phone domain.Phone, code string) (domain.RawSession, domain.User, error) {
	now := s.clock.Now()

	window, err := s.attempts.GetByPhone(ctx, phone)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("get attempts: %w", err)
	}
	if window.Blocked(now) {
		return domain.RawSession{}, domain.User{}, ErrUserBlocked
	}

	sms, err := s.codes.GetLatestByPhone(ctx, phone)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			_ = s.recordFailure(ctx, phone, window, now)
			return domain.RawSession{}, domain.User{}, domain.ErrSMSCodeInvalid
		}
		return domain.RawSession{}, domain.User{}, fmt.Errorf("get code: %w", err)
	}

	if err := sms.Verify(code, now); err != nil {
		_ = s.recordFailure(ctx, phone, window, now)
		return domain.RawSession{}, domain.User{}, err
	}

	user, err := s.users.GetByPhone(ctx, phone)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			user = domain.NewOwner(phone)
			if err := s.users.Create(ctx, user); err != nil {
				return domain.RawSession{}, domain.User{}, fmt.Errorf("create user: %w", err)
			}
			if err := s.billing.CreateDefaultSubscriptionForOwner(ctx, user.ID); err != nil {
				return domain.RawSession{}, domain.User{}, fmt.Errorf("create subscription: %w", err)
			}
		} else {
			return domain.RawSession{}, domain.User{}, fmt.Errorf("get user: %w", err)
		}
	}

	if err := s.codes.MarkUsed(ctx, phone); err != nil {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("mark code used: %w", err)
	}

	raw, err := domain.NewSession(user.ID, now)
	if err != nil {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("create session: %w", err)
	}
	if err := s.sessions.Create(ctx, raw.Session); err != nil {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("save session: %w", err)
	}

	return raw, user, nil
}

func (s *AuthService) recordFailure(ctx context.Context, phone domain.Phone, window domain.AttemptWindow, now time.Time) error {
	if err := window.RecordFailure(now); err != nil {
		return err
	}
	return s.attempts.Save(ctx, phone, window)
}

func generateCode() string {
	return fmt.Sprintf("%06d", rand.IntN(1000000))
}
```

**Note:** Add `ErrNotFound` to `application` package or use domain errors. Define it as:

```go
var ErrNotFound = errors.New("not found")
```

Also import `time` in `ports.go` if needed.

---

## Task 8: Billing Application

**Files:**
- Create: `apps/backend/internal/billing/application/ports.go`
- Create: `apps/backend/internal/billing/application/service.go`

**Step 1: Ports**

Create `apps/backend/internal/billing/application/ports.go`:

```go
package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

var ErrNotFound = errors.New("not found")

type TariffRepository interface {
	GetByName(ctx context.Context, name domain.TariffName) (domain.Tariff, error)
}

type SubscriptionRepository interface {
	Create(ctx context.Context, sub domain.Subscription) error
}
```

**Step 2: Service**

Create `apps/backend/internal/billing/application/service.go`:

```go
package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

type Service struct {
	tariffs        TariffRepository
	subscriptions  SubscriptionRepository
}

func NewService(tariffs TariffRepository, subscriptions SubscriptionRepository) Service {
	return Service{tariffs: tariffs, subscriptions: subscriptions}
}

func (s *Service) CreateDefaultSubscriptionForOwner(ctx context.Context, userID uuid.UUID) error {
	tariff, err := s.tariffs.GetByName(ctx, domain.TariffBasic)
	if err != nil {
		return fmt.Errorf("get basic tariff: %w", err)
	}
	sub := domain.NewOwnerSubscription(userID, tariff.ID)
	if err := s.subscriptions.Create(ctx, sub); err != nil {
		return fmt.Errorf("save subscription: %w", err)
	}
	return nil
}
```

---

## Task 9: Postgres Adapters

**Files:**
- Create: `apps/backend/internal/identity/adapters/postgres/repository.go`
- Create: `apps/backend/internal/billing/adapters/postgres/repository.go`

**Step 1: Identity adapter**

Create `apps/backend/internal/identity/adapters/postgres/repository.go`:

```go
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/nambers/arenda-planform/apps/backend/internal/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

type Repository struct {
	q *postgres.Queries
}

func NewRepository(q *postgres.Queries) *Repository {
	return &Repository{q: q}
}

func (r *Repository) GetByPhone(ctx context.Context, phone domain.Phone) (domain.User, error) {
	row, err := r.q.GetUserByPhone(ctx, phone.String())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, err
	}
	return domain.User{
		ID:     row.ID,
		Phone:  domain.Phone(row.Phone),
		Role:   domain.Role(row.Role),
		Name:   nullString(row.Name),
		Surname: nullString(row.Surname),
		Patronymic: nullString(row.Patronymic),
		Email:  nullString(row.Email),
	}, nil
}

func (r *Repository) Create(ctx context.Context, user domain.User) error {
	return r.q.CreateUser(ctx, postgres.CreateUserParams{
		Phone: user.Phone.String(),
		Role:  string(user.Role),
	})
}

func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
```

**Note:** sqlc generated `CreateUser` returns a row; adjust if needed. If sqlc generates return value, use `_ = ...`.

Also implement SMSCodeRepository, AttemptRepository, SessionRepository in the same file or separate files.

For brevity, implement them in the same file:

```go
func (r *Repository) Save(ctx context.Context, code domain.SMSCode) error {
	_, err := r.q.CreateSMSCode(ctx, postgres.CreateSMSCodeParams{
		UserID:   nil, // nullable until user exists
		Phone:    code.Phone.String(),
		CodeHash: code.CodeHash,
		ExpiresAt: code.ExpiresAt,
	})
	return err
}

func (r *Repository) GetLatestByPhone(ctx context.Context, phone domain.Phone) (domain.SMSCode, error) {
	row, err := r.q.GetLatestSMSCodeByPhone(ctx, phone.String())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SMSCode{}, application.ErrNotFound
		}
		return domain.SMSCode{}, err
	}
	return domain.SMSCode{
		Phone:     domain.Phone(row.Phone),
		CodeHash:  row.CodeHash,
		ExpiresAt: row.ExpiresAt,
		Used:      row.Used,
	}, nil
}

func (r *Repository) MarkUsed(ctx context.Context, phone domain.Phone) error {
	row, err := r.q.GetLatestSMSCodeByPhone(ctx, phone.String())
	if err != nil {
		return err
	}
	return r.q.MarkSMSCodeUsed(ctx, row.ID)
}
```

But wait — `Repository` implements multiple ports. Better split into separate structs per port to keep responsibilities clear. For simplicity in this slice, use separate methods on one struct, but document which port each method belongs to.

For AttemptRepository:

```go
func (r *Repository) GetByPhone(ctx context.Context, phone domain.Phone) (domain.AttemptWindow, error) {
	row, err := r.q.GetLoginAttemptByPhone(ctx, phone.String())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AttemptWindow{}, application.ErrNotFound
		}
		return domain.AttemptWindow{}, err
	}
	return domain.AttemptWindow{
		Failures:       int(row.Failures),
		FirstFailureAt: row.FirstFailureAt,
		LastFailureAt:  row.LastFailureAt,
	}, nil
}

func (r *Repository) Save(ctx context.Context, phone domain.Phone, window domain.AttemptWindow) error {
	return r.q.UpsertLoginAttempt(ctx, postgres.UpsertLoginAttemptParams{
		Phone:          phone.String(),
		Failures:       int32(window.Failures),
		FirstFailureAt: window.FirstFailureAt,
		LastFailureAt:  window.LastFailureAt,
	})
}
```

For SessionRepository:

```go
func (r *Repository) Create(ctx context.Context, session domain.Session) error {
	_, err := r.q.CreateSession(ctx, postgres.CreateSessionParams{
		UserID:    session.UserID,
		TokenHash: session.TokenHash,
		ExpiresAt: session.ExpiresAt,
	})
	return err
}

func (r *Repository) DeleteByTokenHash(ctx context.Context, tokenHash string) error {
	return r.q.DeleteSessionByTokenHash(ctx, tokenHash)
}
```

**Step 2: Billing adapter**

Create `apps/backend/internal/billing/adapters/postgres/repository.go`:

```go
package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/generated/postgres"
)

type Repository struct {
	q *postgres.Queries
}

func NewRepository(q *postgres.Queries) *Repository {
	return &Repository{q: q}
}

func (r *Repository) GetByName(ctx context.Context, name domain.TariffName) (domain.Tariff, error) {
	row, err := r.q.GetTariffByName(ctx, string(name))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Tariff{}, application.ErrNotFound
		}
		return domain.Tariff{}, err
	}
	return domain.Tariff{
		ID:                  row.ID,
		Name:                domain.TariffName(row.Name),
		ActivePropertyLimit: int(row.ActivePropertyLimit),
		MonthlyPrice:        numericToKopecks(row.MonthlyPrice),
		YearlyPrice:         numericToKopecks(row.YearlyPrice),
	}, nil
}

func (r *Repository) Create(ctx context.Context, sub domain.Subscription) error {
	_, err := r.q.CreateSubscription(ctx, postgres.CreateSubscriptionParams{
		UserID:   sub.UserID,
		TariffID: sub.TariffID,
		Source:   string(sub.Source),
		Status:   string(sub.Status),
	})
	return err
}
```

**Note:** `MonthlyPrice` and `YearlyPrice` are stored as `NUMERIC(14,2)` rubles in PostgreSQL and sqlc generates `pgtype.Numeric`. The billing adapter must convert between `pgtype.Numeric` (rubles) and `int64` kopecks in the domain model: multiply rubles by 100 when reading from the database, divide kopecks by 100 when writing to the database. Implement `numericToKopecks` and `kopecksToNumeric` helpers in the adapter package.

---

## Task 10: Fake SMS Sender

**Files:**
- Create: `apps/backend/internal/identity/adapters/sms/fake.go`

Create `apps/backend/internal/identity/adapters/sms/fake.go`:

```go
package sms

import (
	"context"
	"log/slog"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

type FakeSender struct {
	logger *slog.Logger
}

func NewFakeSender(logger *slog.Logger) *FakeSender {
	return &FakeSender{logger: logger}
}

func (s *FakeSender) Send(ctx context.Context, phone domain.Phone, message string) error {
	s.logger.InfoContext(ctx, "fake sms sent", "phone", phone.String(), "message", message)
	return nil
}
```

---

## Task 11: OpenAPI Contract and Codegen

**Files:**
- Create: `apps/backend/api/openapi/openapi.yaml`
- Create: `apps/backend/api/openapi/oapi-codegen.yaml`

**Step 1: OpenAPI spec**

Create `apps/backend/api/openapi/openapi.yaml`:

```yaml
openapi: 3.0.3
info:
  title: Arenda Planform API
  version: 0.1.0
servers:
  - url: http://localhost:8080
paths:
  /auth/phone/send:
    post:
      operationId: sendPhoneCode
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/SendPhoneCodeRequest'
      responses:
        '204':
          description: Code sent
        '400':
          $ref: '#/components/responses/BadRequest'
        '429':
          $ref: '#/components/responses/TooManyRequests'
  /auth/phone/verify:
    post:
      operationId: verifyPhoneCode
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/VerifyPhoneCodeRequest'
      responses:
        '200':
          description: Authenticated
          headers:
            Set-Cookie:
              schema:
                type: string
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/MeResponse'
        '400':
          $ref: '#/components/responses/BadRequest'
        '401':
          $ref: '#/components/responses/Unauthorized'
        '429':
          $ref: '#/components/responses/TooManyRequests'
  /auth/logout:
    post:
      operationId: logout
      security:
        - sessionCookie: []
      responses:
        '204':
          description: Logged out
          headers:
            Set-Cookie:
              schema:
                type: string
        '401':
          $ref: '#/components/responses/Unauthorized'
  /me:
    get:
      operationId: getMe
      security:
        - sessionCookie: []
      responses:
        '200':
          description: Current user
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/MeResponse'
        '401':
          $ref: '#/components/responses/Unauthorized'

components:
  securitySchemes:
    sessionCookie:
      type: apiKey
      in: cookie
      name: session_id

  schemas:
    SendPhoneCodeRequest:
      type: object
      required: [phone]
      properties:
        phone:
          type: string
          example: '+79990001122'

    VerifyPhoneCodeRequest:
      type: object
      required: [phone, code]
      properties:
        phone:
          type: string
        code:
          type: string
          example: '123456'

    MeResponse:
      type: object
      required: [id, phone, role]
      properties:
        id:
          type: string
          format: uuid
        phone:
          type: string
        role:
          type: string
          enum: [owner, admin]
        name:
          type: string
          nullable: true
        surname:
          type: string
          nullable: true
        patronymic:
          type: string
          nullable: true
        email:
          type: string
          nullable: true

    Problem:
      type: object
      required: [type, title, status]
      properties:
        type:
          type: string
        title:
          type: string
        status:
          type: integer
        detail:
          type: string
        instance:
          type: string
        requestId:
          type: string

  responses:
    BadRequest:
      description: Bad request
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/Problem'
    Unauthorized:
      description: Unauthorized
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/Problem'
    TooManyRequests:
      description: Too many requests
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/Problem'
```

**Step 2: Codegen config**

Create `apps/backend/api/openapi/oapi-codegen.yaml`:

```yaml
package: openapi
output: internal/platform/openapi/generated.gen.go
generate:
  chi-server: true
  strict-server: true
  models: true
  embedded-spec: true
```

**Step 3: Generate code**

Run:

```bash
cd apps/backend && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.0 -config api/openapi/oapi-codegen.yaml api/openapi/openapi.yaml
```

Expected: `internal/platform/openapi/generated.gen.go` created.

---

## Task 12: HTTP Handlers and Server

**Files:**
- Create: `apps/backend/internal/platform/httpapi/server.go`
- Create: `apps/backend/internal/platform/httpapi/auth_handlers.go`
- Create: `apps/backend/internal/platform/httpapi/problem.go`
- Create: `apps/backend/internal/platform/httpapi/session.go`

**Step 1: Problem response helper**

Create `apps/backend/internal/platform/httpapi/problem.go`:

```go
package httpapi

import (
	"encoding/json"
	"net/http"
)

type Problem struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Detail    string `json:"detail,omitempty"`
	Instance  string `json:"instance,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

func WriteProblem(w http.ResponseWriter, status int, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Problem{
		Type:   "about:blank",
		Title:  title,
		Status: status,
		Detail: detail,
	})
}
```

**Step 2: Session cookie helpers**

Create `apps/backend/internal/platform/httpapi/session.go`:

```go
package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"
)

const sessionCookieName = "session_id"

func setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func sessionTokenFromRequest(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
```

**Step 3: Auth handlers**

Create `apps/backend/internal/platform/httpapi/auth_handlers.go`:

```go
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/nambers/arenda-planform/apps/backend/internal/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

type AuthServer struct {
	authService application.AuthService
	queries     *postgres.Queries
	cookieSecure bool
}

func NewAuthServer(authService application.AuthService, queries *postgres.Queries, cookieSecure bool) *AuthServer {
	return &AuthServer{
		authService:  authService,
		queries:      queries,
		cookieSecure: cookieSecure,
	}
}

func (s *AuthServer) SendPhoneCode(ctx context.Context, request openapi.SendPhoneCodeRequestObject) (openapi.SendPhoneCodeResponseObject, error) {
	phone, err := domain.NewPhone(request.Body.Phone)
	if err != nil {
		return openapi.SendPhoneCode400JSONResponse{}, nil
	}
	if err := s.authService.SendCode(ctx, phone); err != nil {
		switch {
		case errors.Is(err, application.ErrUserBlocked):
			return openapi.SendPhoneCode429JSONResponse{}, nil
		default:
			return openapi.SendPhoneCode400JSONResponse{}, nil
		}
	}
	return openapi.SendPhoneCode204Response{}, nil
}

func (s *AuthServer) VerifyPhoneCode(ctx context.Context, request openapi.VerifyPhoneCodeRequestObject) (openapi.VerifyPhoneCodeResponseObject, error) {
	phone, err := domain.NewPhone(request.Body.Phone)
	if err != nil {
		return openapi.VerifyPhoneCode400JSONResponse{}, nil
	}
	raw, user, err := s.authService.VerifyCode(ctx, phone, request.Body.Code)
	if err != nil {
		switch {
		case errors.Is(err, application.ErrUserBlocked), errors.Is(err, domain.ErrSMSCodeInvalid):
			return openapi.VerifyPhoneCode401JSONResponse{}, nil
		default:
			return openapi.VerifyPhoneCode400JSONResponse{}, nil
		}
	}
	setSessionCookie(nil, raw.Token, user.ID, raw.Session.ExpiresAt, s.cookieSecure)
	return openapi.VerifyPhoneCode200JSONResponse(meResponse(user)), nil
}

func (s *AuthServer) Logout(ctx context.Context, request openapi.LogoutRequestObject) (openapi.LogoutResponseObject, error) {
	token := sessionTokenFromRequest(request.Request)
	if token != "" {
		_ = s.authService.Logout(ctx, HashToken(token))
	}
	clearSessionCookie(nil, s.cookieSecure)
	return openapi.Logout204Response{}, nil
}

func (s *AuthServer) GetMe(ctx context.Context, request openapi.GetMeRequestObject) (openapi.GetMeResponseObject, error) {
	token := sessionTokenFromRequest(request.Request)
	if token == "" {
		return openapi.GetMe401JSONResponse{}, nil
	}
	row, err := s.queries.GetSessionByTokenHash(ctx, HashToken(token))
	if err != nil {
		return openapi.GetMe401JSONResponse{}, nil
	}
	return openapi.GetMe200JSONResponse(meResponse(domain.User{
		ID:         row.UserID,
		Phone:      domain.Phone(row.Phone),
		Role:       domain.Role(row.Role),
		Name:       nullString(row.Name),
		Surname:    nullString(row.Surname),
		Patronymic: nullString(row.Patronymic),
		Email:      nullString(row.Email),
	})), nil
}

func meResponse(user domain.User) openapi.MeResponse {
	return openapi.MeResponse{
		Id:         user.ID,
		Phone:      user.Phone.String(),
		Role:       openapi.MeResponseRole(user.Role),
		Name:       user.Name,
		Surname:    user.Surname,
		Patronymic: user.Patronymic,
		Email:      user.Email,
	}
}
```

**Note:** Need to add `Logout` method to `AuthService`:

```go
func (s *AuthService) Logout(ctx context.Context, tokenHash string) error {
	return s.sessions.DeleteByTokenHash(ctx, tokenHash)
}
```

Also, `setSessionCookie` signature needs adjustment. The generated response objects may not allow setting headers directly. Use a custom response wrapper or set cookies via `http.SetCookie` in a custom `Response` type.

Given strict server, you may need to return a custom response that implements the interface and sets the header. Simplified: use a regular `http.HandlerFunc` wrapper instead of strict server for cookie-setting endpoints if codegen gets in the way.

**Step 4: Server wiring**

Create `apps/backend/internal/platform/httpapi/server.go`:

```go
package httpapi

import (
	"net/http"

	"github.com/nambers/arenda-planform/apps/backend/internal/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

type Deps struct {
	AuthService  application.AuthService
	Queries      *postgres.Queries
	CookieSecure bool
}

func New(deps Deps) http.Handler {
	auth := NewAuthServer(deps.AuthService, deps.Queries, deps.CookieSecure)
	handler := openapi.HandlerFromMux(auth, http.NewServeMux())
	return handler
}
```

**Note:** The exact generated function name depends on oapi-codegen config. Adjust accordingly.

---

## Task 13: Wire Everything in cmd/api/main.go

**Files:**
- Create: `apps/backend/cmd/api/main.go`

Create `apps/backend/cmd/api/main.go`:

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	billingpg "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/generated/postgres"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	identitypg "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/postgres"
	fakesms "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/sms"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/config"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpapi"
)

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("backend stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := database.MigrateUp(cfg.DatabaseURL, cfg.MigrationsDir); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database pool: %w", err)
	}
	defer pool.Close()

	queries := postgres.New(pool)

	billingRepo := billingpg.NewRepository(queries)
	billingService := billingapp.NewService(billingRepo, billingRepo)

	identityRepo := identitypg.NewRepository(queries)
	smsSender := fakesms.NewFakeSender(logger)
	authService := identityapp.NewAuthService(identityRepo, identityRepo, identityRepo, identityRepo, smsSender, realClock{}, billingService)

	handler := httpapi.New(httpapi.Deps{
		AuthService:  authService,
		Queries:      queries,
		CookieSecure: cfg.CookieSecure,
	})

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("backend listening", "addr", cfg.HTTPAddr, "env", cfg.AppEnv)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
```

**Step 5: Build**

Run:

```bash
cd apps/backend && go build ./...
```

Fix any compile errors.

---

## Task 14: Manual Testing

**Prerequisites:**

```bash
make local-infra-up
make migrate-up
cd apps/backend && go run ./cmd/api
```

**Step 1: Request code**

```bash
curl -X POST http://localhost:8080/auth/phone/send \
  -H "Content-Type: application/json" \
  -d '{"phone":"+79990001122"}'
```

Expected: `204 No Content`. Check backend logs for fake SMS code.

**Step 2: Verify code**

```bash
curl -X POST http://localhost:8080/auth/phone/verify \
  -H "Content-Type: application/json" \
  -d '{"phone":"+79990001122","code":"123456"}' \
  -c cookies.txt
```

Expected: `200 OK` with user JSON. Cookie saved to `cookies.txt`.

**Step 3: Get current user**

```bash
curl http://localhost:8080/me -b cookies.txt
```

Expected: `200 OK` with same user JSON.

**Step 4: Logout**

```bash
curl -X POST http://localhost:8080/auth/logout -b cookies.txt -c cookies.txt
```

Expected: `204 No Content`. Cookie cleared.

**Step 5: Verify unauthorized**

```bash
curl http://localhost:8080/me -b cookies.txt
```

Expected: `401 Unauthorized`.

---

## Post-Implementation Checklist

- [ ] `go build ./...` passes.
- [ ] `make backend-lint` passes.
- [ ] Manual Postman/curl flow works end-to-end.
- [ ] No secrets logged.
- [ ] Design doc and implementation plan committed.
