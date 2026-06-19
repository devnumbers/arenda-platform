# Backend Load Testing (k6) Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a local, repeatable k6-based load-testing harness for `apps/backend` that finds the maximum RPS each endpoint can sustain with p95(successful) < 500 ms.

**Architecture:** A Go seeder (`cmd/perfseed`) resets and seeds an isolated perf PostgreSQL database. A single k6 script (`perf/scripts/endpoint_benchmark.js`) drives load per endpoint using `constant-arrival-rate`. A Go orchestrator (`cmd/perfmaxrps`) re-seeds, runs k6, parses `summary.json`, and binary-searches the max RPS for each endpoint.

**Tech Stack:** Go 1.26, k6 (Docker), PostgreSQL 17, Make.

---

## Task 1: Bootstrap perf infrastructure

**Files:**
- Create: `docker-compose.perf.yml`
- Create: `.env.perf.example`
- Modify: `Makefile`

**Step 1: Create isolated perf PostgreSQL compose file**

Create `docker-compose.perf.yml`:

```yaml
services:
  postgres:
    image: postgres:17-alpine
    environment:
      POSTGRES_USER: ${PERF_POSTGRES_USER:-arenda}
      POSTGRES_PASSWORD: ${PERF_POSTGRES_PASSWORD:-arenda}
      POSTGRES_DB: ${PERF_POSTGRES_DB:-arenda}
    ports:
      - "${PERF_POSTGRES_PORT:-5433}:5432"
    volumes:
      - postgres-perf-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${PERF_POSTGRES_USER:-arenda} -d ${PERF_POSTGRES_DB:-arenda}"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  postgres-perf-data:
```

**Step 2: Create perf env example with placeholders only**

Create `.env.perf.example`:

```bash
APP_ENV=local
HTTP_ADDR=:8081
APP_BASE_URL=http://localhost:8081
DATABASE_URL=postgres://arenda:arenda@localhost:5433/arenda?sslmode=disable
MIGRATIONS_DIR=apps/backend/db/migrations
SMS_SENDER=fake
PAYMENT_PROVIDER=fake
COOKIE_SECURE=false
ENCRYPTION_KEY=<256-bit-hex-key>
RATE_LIMIT_IP_RPS=100000
RATE_LIMIT_IP_BURST=100000
RATE_LIMIT_PHONE_SEND_PER_HOUR=100000
RATE_LIMIT_PHONE_VERIFY_PER_15MIN=100000
```

**Step 3: Add perf Makefile targets**

Append to `Makefile`:

```makefile
PERF_HTTP_ADDR ?= :8081
PERF_APP_BASE_URL ?= http://localhost:8081
PERF_BASE_URL ?= http://localhost:8081
PERF_DATABASE_URL ?= postgres://arenda:arenda@localhost:5433/arenda?sslmode=disable
PERF_POSTGRES_PORT ?= 5433

PERF_ENV ?= \
	HTTP_ADDR=$(PERF_HTTP_ADDR) \
	APP_BASE_URL=$(PERF_APP_BASE_URL) \
	DATABASE_URL=$(PERF_DATABASE_URL) \
	MIGRATIONS_DIR=apps/backend/db/migrations \
	SMS_SENDER=fake \
	PAYMENT_PROVIDER=fake \
	COOKIE_SECURE=false \
	RATE_LIMIT_IP_RPS=100000 \
	RATE_LIMIT_IP_BURST=100000 \
	RATE_LIMIT_PHONE_SEND_PER_HOUR=100000 \
	RATE_LIMIT_PHONE_VERIFY_PER_15MIN=100000

perf-db-up:
	PERF_POSTGRES_PORT=$(PERF_POSTGRES_PORT) docker compose -f docker-compose.perf.yml up -d

perf-db-down:
	docker compose -f docker-compose.perf.yml down

perf-db-reset:
	docker compose -f docker-compose.perf.yml down -v
	PERF_POSTGRES_PORT=$(PERF_POSTGRES_PORT) docker compose -f docker-compose.perf.yml up -d

perf-backend-run:
	cd apps/backend && $(PERF_ENV) go run ./cmd/api

perf-seed:
	cd apps/backend && $(PERF_ENV) go run ./cmd/perfseed

perf-endpoint-max-rps:
	cd apps/backend && $(PERF_ENV) API_BASE_URL=$(PERF_BASE_URL) go run ./cmd/perfmaxrps -endpoint=$(ENDPOINT)

perf-endpoints-max-rps:
	cd apps/backend && $(PERF_ENV) API_BASE_URL=$(PERF_BASE_URL) go run ./cmd/perfmaxrps -suite

perf-results-clean:
	rm -rf apps/backend/perf/results/endpoints/*
```

**Step 4: Verify compose starts**

Run:

```bash
make perf-db-up
```

Expected: PostgreSQL container starts and becomes healthy on port 5433.

**Step 5: Commit**

```bash
git add docker-compose.perf.yml .env.perf.example Makefile
git commit -m "feat(perf): bootstrap isolated perf infrastructure"
```

---

## Task 2: Implement perfseed base fixtures

**Files:**
- Create: `apps/backend/cmd/perfseed/main.go`
- Create: `apps/backend/cmd/perfseed/seed.go`

**Step 1: Create main.go with CLI and config**

Create `apps/backend/cmd/perfseed/main.go`:

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	endpoint := flag.String("endpoint", "", "endpoint key to seed extra fixtures for")
	flag.Parse()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	db, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		logger.Error("connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	if err := resetDB(ctx, db); err != nil {
		logger.Error("reset database", slog.String("error", err.Error()))
		os.Exit(1)
	}

	cfg := seedConfig{
		ownerCount: envInt("PERF_OWNERS", 1000),
	}

	state, err := seedBase(ctx, db, cfg)
	if err != nil {
		logger.Error("seed base fixtures", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if *endpoint != "" {
		if err := seedEndpoint(ctx, db, state, *endpoint); err != nil {
			logger.Error("seed endpoint fixtures", slog.String("endpoint", *endpoint), slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	logger.Info("seed complete", slog.Int("owners", len(state.owners)))
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		panic(fmt.Sprintf("invalid %s: %s", key, v))
	}
	return n
}
```

**Step 2: Implement reset and base seed**

Create `apps/backend/cmd/perfseed/seed.go`:

```go
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type seedConfig struct {
	ownerCount int
}

type ownerState struct {
	id        uuid.UUID
	phone     string
	propertyID uuid.UUID
	leaseID   uuid.UUID
	operationID uuid.UUID
	recurringOperationID uuid.UUID
	reminderID uuid.UUID
	tenantContactID uuid.UUID
	sessionToken string
}

type seedState struct {
	owners []ownerState
}

func resetDB(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, `
		TRUNCATE TABLE 
			sent_sms_reminders,
			subscription_payments,
			payment_methods,
			user_subscriptions,
			tariffs,
			reminders,
			recurring_operations,
			operations,
			leases,
			tenant_contacts,
			properties,
			sessions,
			login_attempts,
			sms_codes,
			users
		CASCADE
	`)
	return err
}

func seedBase(ctx context.Context, db *pgxpool.Pool, cfg seedConfig) (*seedState, error) {
	state := &seedState{owners: make([]ownerState, cfg.ownerCount)}

	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Seed basic tariff.
	if _, err := tx.Exec(ctx, `
		INSERT INTO tariffs (id, name, active_property_limit, monthly_price, yearly_price)
		VALUES ($1, 'basic', 100, 0, 0)
		ON CONFLICT (name) DO UPDATE SET active_property_limit = EXCLUDED.active_property_limit
	`, uuid.New()); err != nil {
		return nil, fmt.Errorf("seed tariff: %w", err)
	}

	for i := 0; i < cfg.ownerCount; i++ {
		owner, err := seedOwner(ctx, tx, i)
		if err != nil {
			return nil, fmt.Errorf("seed owner %d: %w", i, err)
		}
		state.owners[i] = *owner
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return state, nil
}

func seedOwner(ctx context.Context, tx pgx.Tx, index int) (*ownerState, error) {
	ownerID := uuid.New()
	phone := fmt.Sprintf("+7999%07d", index)
	token := deterministicToken(index)
	tokenHash := hashToken(token)
	now := time.Now().UTC()

	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id, phone, role, created_at, updated_at)
		VALUES ($1, $2, 'owner', $3, $3)
	`, ownerID, phone, now); err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO user_subscriptions (id, user_id, tariff_id, source, status, valid_until, created_at, updated_at)
		VALUES ($1, $2, (SELECT id FROM tariffs WHERE name = 'basic' LIMIT 1), 'service', 'active', $3, $4, $4)
	`, uuid.New(), ownerID, now.Add(365*24*time.Hour), now); err != nil {
		return nil, fmt.Errorf("insert subscription: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, uuid.New(), ownerID, tokenHash, now.Add(30*24*time.Hour), now); err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}

	propertyID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO properties (id, user_id, name, address, type, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'apartment', 'active', $5, $5)
	`, propertyID, ownerID, fmt.Sprintf("Property %d", index), fmt.Sprintf("Address %d", index), now); err != nil {
		return nil, fmt.Errorf("insert property: %w", err)
	}

	leaseID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO leases (id, user_id, property_id, tenant_name, start_date, end_date, monthly_amount, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, 50000, 'active', $7, $7)
	`, leaseID, ownerID, propertyID, fmt.Sprintf("Tenant %d", index), now.Add(-30*24*time.Hour), now.Add(335*24*time.Hour), now); err != nil {
		return nil, fmt.Errorf("insert lease: %w", err)
	}

	opID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO operations (id, user_id, property_id, category, amount, operation_date, description, deleted_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'repair', 1000, $4, $5, NULL, $6, $6)
	`, opID, ownerID, propertyID, now, fmt.Sprintf("Operation %d", index), now); err != nil {
		return nil, fmt.Errorf("insert operation: %w", err)
	}

	recID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO recurring_operations (id, user_id, property_id, category, amount, start_date, day_of_month, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'utility', 2000, $4, 1, 'active', $5, $5)
	`, recID, ownerID, propertyID, now.Add(-60*24*time.Hour), now); err != nil {
		return nil, fmt.Errorf("insert recurring operation: %w", err)
	}

	remID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO reminders (id, user_id, target_type, target_id, scheduled_at, status, offset_days, created_at, updated_at)
		VALUES ($1, $2, 'operation', $3, $4, 'scheduled', 0, $5, $5)
	`, remID, ownerID, opID, now.Add(7*24*time.Hour), now); err != nil {
		return nil, fmt.Errorf("insert reminder: %w", err)
	}

	contactID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO tenant_contacts (id, user_id, name, phone, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
	`, contactID, ownerID, fmt.Sprintf("Contact %d", index), fmt.Sprintf("+7955%07d", index), now); err != nil {
		return nil, fmt.Errorf("insert tenant contact: %w", err)
	}

	return &ownerState{
		id: ownerID,
		phone: phone,
		propertyID: propertyID,
		leaseID: leaseID,
		operationID: opID,
		recurringOperationID: recID,
		reminderID: remID,
		tenantContactID: contactID,
		sessionToken: token,
	}, nil
}

func deterministicToken(index int) string {
	return fmt.Sprintf("perf-session-token-%d", index)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
```

**Step 3: Verify perfseed compiles**

Run:

```bash
cd apps/backend && go build ./cmd/perfseed
```

Expected: build succeeds.

**Step 4: Commit**

```bash
git add apps/backend/cmd/perfseed
git commit -m "feat(perf): add perfseed base fixtures"
```

---

## Task 3: Implement per-endpoint fixtures in perfseed

**Files:**
- Modify: `apps/backend/cmd/perfseed/seed.go`

**Step 1: Add endpoint-specific seed function**

Append to `apps/backend/cmd/perfseed/seed.go`:

```go
func seedEndpoint(ctx context.Context, db *pgxpool.Pool, state *seedState, endpoint string) error {
	switch endpoint {
	case "delete_reminder":
		return seedManyReminders(ctx, db, state)
	case "auth_verify_code":
		return seedAuthCodes(ctx, db, state)
	default:
		return nil
	}
}

func seedManyReminders(ctx context.Context, db *pgxpool.Pool, state *seedState) error {
	now := time.Now().UTC()
	batch := &pgx.Batch{}
	// Seed 10 reminders per owner so delete_reminder does not exhaust fixtures.
	for _, owner := range state.owners {
		for j := 0; j < 10; j++ {
			batch.Queue(`
				INSERT INTO reminders (id, user_id, target_type, target_id, scheduled_at, status, offset_days, created_at, updated_at)
				VALUES ($1, $2, 'operation', $3, $4, 'scheduled', 0, $5, $5)
			`, uuid.New(), owner.id, owner.operationID, now.Add(time.Duration(j+1)*24*time.Hour), now)
		}
	}
	return db.SendBatch(ctx, batch).Close()
}

func seedAuthCodes(ctx context.Context, db *pgxpool.Pool, state *seedState) error {
	now := time.Now().UTC()
	code := "000000"
	codeHash := hashToken(code)
	batch := &pgx.Batch{}
	for _, owner := range state.owners {
		batch.Queue(`
			INSERT INTO sms_codes (id, user_id, phone, code_hash, expires_at, used, created_at)
			VALUES ($1, $2, $3, $4, $5, false, $6)
		`, uuid.New(), owner.id, owner.phone, codeHash, now.Add(2*time.Hour), now)
	}
	return db.SendBatch(ctx, batch).Close()
}
```

**Step 2: Commit**

```bash
git add apps/backend/cmd/perfseed/seed.go
git commit -m "feat(perf): add per-endpoint fixtures for delete_reminder and auth_verify_code"
```

---

## Task 4: Implement k6 endpoint_benchmark.js skeleton

**Files:**
- Create: `apps/backend/perf/scripts/endpoint_benchmark.js`

**Step 1: Create base k6 script with options and summary**

Create `apps/backend/perf/scripts/endpoint_benchmark.js`:

```javascript
import http from 'k6/http';
import { check } from 'k6';
import { textSummary } from 'https://jslib.k6.io/k6-summary/0.0.4/index.js';
import { endpoints } from './endpoints.js';

const ENDPOINT = __ENV.ENDPOINT || 'get_properties';
const RATE = parseInt(__ENV.RATE || '10', 10);
const DURATION = __ENV.DURATION || '30s';
const PRE_ALLOCATED_VUS = parseInt(__ENV.PRE_ALLOCATED_VUS || String(Math.max(RATE, 10)), 10);
const MAX_VUS = parseInt(__ENV.MAX_VUS || String(PRE_ALLOCATED_VUS * 2), 10);
const API_BASE_URL = __ENV.API_BASE_URL || 'http://localhost:8081';

export const options = {
  scenarios: {
    benchmark: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: PRE_ALLOCATED_VUS,
      maxVUs: MAX_VUS,
    },
  },
  thresholds: {
    'http_req_duration{expected_response:true}': ['p(95)<500'],
    'http_req_failed': ['rate<0.01'],
  },
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
};

export default function () {
  const ctx = {
    baseUrl: API_BASE_URL,
    vu: __VU,
    iteration: __ITER,
  };
  const endpoint = endpoints[ENDPOINT];
  if (!endpoint) {
    throw new Error(`unknown endpoint: ${ENDPOINT}`);
  }

  const req = endpoint.request(ctx);
  const res = http.request(req.method, req.url, req.body || null, {
    headers: req.headers || {},
    cookies: req.cookies || {},
    tags: { name: ENDPOINT, endpoint: ENDPOINT },
  });

  check(res, {
    'status is expected': (r) => endpoint.expectedStatuses.includes(r.status),
  });
}

export function handleSummary(data) {
  const outputDir = __ENV.K6_OUTPUT_DIR;
  const result = {};
  if (outputDir) {
    result[`${outputDir}/summary.json`] = JSON.stringify(data);
  }
  result['stdout'] = textSummary(data, { indent: ' ', enableColors: false });
  return result;
}
```

**Step 2: Commit**

```bash
git add apps/backend/perf/scripts/endpoint_benchmark.js
git commit -m "feat(perf): add k6 benchmark script skeleton"
```

---

## Task 5: Implement endpoint manifest and request builders

**Files:**
- Create: `apps/backend/perf/scripts/endpoints.js`

**Step 1: Create endpoints manifest**

Create `apps/backend/perf/scripts/endpoints.js`:

```javascript
const now = new Date();
const fmtDate = (d) => d.toISOString().split('T')[0];

function sessionCookie(index) {
  return `session_id=perf-session-token-${index}`;
}

function ownerIndex(ctx) {
  return (ctx.vu + ctx.iteration) % 1000;
}

export const endpoints = {
  get_me: {
    request: (ctx) => ({
      method: 'GET',
      url: `${ctx.baseUrl}/me`,
      headers: { Cookie: sessionCookie(ownerIndex(ctx)) },
    }),
    expectedStatuses: [200],
  },
  get_properties: {
    request: (ctx) => ({
      method: 'GET',
      url: `${ctx.baseUrl}/properties`,
      headers: { Cookie: sessionCookie(ownerIndex(ctx)) },
    }),
    expectedStatuses: [200],
  },
  get_property: {
    request: (ctx) => ({
      method: 'GET',
      url: `${ctx.baseUrl}/properties/${ownerIndex(ctx)}`,
      headers: { Cookie: sessionCookie(ownerIndex(ctx)) },
    }),
    expectedStatuses: [200, 404],
  },
  post_property: {
    request: (ctx) => ({
      method: 'POST',
      url: `${ctx.baseUrl}/properties`,
      headers: {
        'Content-Type': 'application/json',
        Cookie: sessionCookie(ownerIndex(ctx)),
      },
      body: JSON.stringify({
        name: `Property ${ctx.vu}-${ctx.iteration}`,
        address: `Address ${ctx.vu}-${ctx.iteration}`,
        type: 'apartment',
      }),
    }),
    expectedStatuses: [201, 402, 403],
  },
  // TODO: add remaining endpoints in Task 9
};
```

**Step 2: Verify k6 script parses**

Run:

```bash
k6 run --env ENDPOINT=get_me --env RATE=1 --env DURATION=1s --env API_BASE_URL=http://localhost:8081 apps/backend/perf/scripts/endpoint_benchmark.js 2>&1 | head -20
```

Expected: script compiles and runs (may fail on connection if backend is not running, but no JS errors).

**Step 3: Commit**

```bash
git add apps/backend/perf/scripts/endpoints.js
git commit -m "feat(perf): add initial endpoint manifest for k6"
```

---

## Task 6: Implement perfmaxrps single-endpoint runner

**Files:**
- Create: `apps/backend/cmd/perfmaxrps/main.go`

**Step 1: Create main.go with flags and k6 runner**

Create `apps/backend/cmd/perfmaxrps/main.go`:

```go
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type options struct {
	endpoint   string
	suite      bool
	startRate  int
	maxRate    int
	maxIters   int
	duration   time.Duration
	apiBaseURL string
	outputDir  string
}

func main() {
	var opts options
	flag.StringVar(&opts.endpoint, "endpoint", "", "single endpoint key to benchmark")
	flag.BoolVar(&opts.suite, "suite", false, "run full endpoint suite")
	flag.IntVar(&opts.startRate, "start-rate", envInt("START_RATE", 10), "initial RPS")
	flag.IntVar(&opts.maxRate, "max-rate", envInt("MAX_RATE", 1000), "maximum RPS to try")
	flag.IntVar(&opts.maxIters, "max-iters", envInt("MAX_ITERS", 10), "max search iterations")
	flag.DurationVar(&opts.duration, "duration", envDuration("DURATION", 15*time.Second), "duration per k6 run")
	flag.StringVar(&opts.apiBaseURL, "api-base-url", os.Getenv("API_BASE_URL"), "backend base URL")
	flag.Parse()

	if opts.apiBaseURL == "" {
		opts.apiBaseURL = "http://localhost:8081"
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := run(logger, opts); err != nil {
		logger.Error("perfmaxrps failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(logger *slog.Logger, opts options) error {
	runID := time.Now().UTC().Format("20060102-150405")
	opts.outputDir = filepath.Join("perf", "results", "endpoints", runID)
	if err := os.MkdirAll(opts.outputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	if opts.suite {
		return runSuite(logger, opts)
	}
	if opts.endpoint == "" {
		return fmt.Errorf("either -endpoint or -suite is required")
	}
	result, err := benchmarkEndpoint(logger, opts, opts.endpoint)
	if err != nil {
		return err
	}
	logger.Info("benchmark complete", slog.String("endpoint", opts.endpoint), slog.Float64("max_rps", result.MaxRPS))
	return nil
}

func benchmarkEndpoint(logger *slog.Logger, opts options, endpoint string) (endpointResult, error) {
	if err := reseed(endpoint); err != nil {
		return endpointResult{}, fmt.Errorf("reseed %s: %w", endpoint, err)
	}

	low := float64(opts.startRate)
	high := float64(opts.maxRate)
	best := endpointResult{Endpoint: endpoint}

	for i := 0; i < opts.maxIters; i++ {
		if high-low < 1 {
			break
		}
		mid := math.Floor((low + high) / 2)
		passed, metrics, err := runK6(logger, opts, endpoint, int(mid))
		if err != nil {
			return endpointResult{}, fmt.Errorf("run k6 for %s at %.0f rps: %w", endpoint, mid, err)
		}
		if passed {
			best.MaxRPS = mid
			best.P95 = metrics.P95
			best.ErrorRate = metrics.ErrorRate
			best.Status = "pass"
			low = mid + 1
		} else {
			best.Status = "fail"
			high = mid - 1
		}
		logger.Info("iteration", slog.String("endpoint", endpoint), slog.Float64("rate", mid), slog.Bool("passed", passed), slog.Float64("p95", metrics.P95))
	}

	return best, nil
}

func runK6(logger *slog.Logger, opts options, endpoint string, rate int) (bool, k6Metrics, error) {
	runDir := filepath.Join(opts.outputDir, fmt.Sprintf("%s-%d-%d", endpoint, rate, time.Now().Unix()))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return false, k6Metrics{}, err
	}

	cmd := exec.Command("docker", "run", "--rm",
		"--network=host",
		"-v", fmt.Sprintf("%s:/scripts", filepath.Join("perf", "scripts")),
		"-v", fmt.Sprintf("%s:/output", runDir),
		"-e", fmt.Sprintf("ENDPOINT=%s", endpoint),
		"-e", fmt.Sprintf("RATE=%d", rate),
		"-e", fmt.Sprintf("DURATION=%s", opts.duration),
		"-e", fmt.Sprintf("API_BASE_URL=%s", opts.apiBaseURL),
		"-e", fmt.Sprintf("K6_OUTPUT_DIR=/output"),
		"grafana/k6:0.52.0",
		"run", "/scripts/endpoint_benchmark.js",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run() // k6 returns non-zero on threshold failure; we parse summary anyway.

	summaryPath := filepath.Join(runDir, "summary.json")
	return parseSummary(summaryPath)
}

type k6Metrics struct {
	P95       float64
	ErrorRate float64
}

func parseSummary(path string) (bool, k6Metrics, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, k6Metrics{}, fmt.Errorf("read summary: %w", err)
	}
	var summary struct {
		Metrics map[string]map[string]any `json:"metrics"`
	}
	if err := json.Unmarshal(data, &summary); err != nil {
		return false, k6Metrics{}, fmt.Errorf("parse summary: %w", err)
	}

	dur, ok := summary.Metrics["http_req_duration{expected_response:true}"]
	if !ok {
		dur = summary.Metrics["http_req_duration"]
	}
	p95 := dur["values"].(map[string]any)["p(95)"].(float64)

	failed := summary.Metrics["http_req_failed"]
	errorRate := failed["values"].(map[string]any)["rate"].(float64)

	return p95 < 500 && errorRate < 0.01, k6Metrics{P95: p95, ErrorRate: errorRate}, nil
}

func reseed(endpoint string) error {
	cmd := exec.Command("go", "run", "./cmd/perfseed", "-endpoint="+endpoint)
	cmd.Dir = "apps/backend"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

type endpointResult struct {
	Endpoint  string  `json:"endpoint"`
	MaxRPS    float64 `json:"max_rps"`
	P95       float64 `json:"p95_ms"`
	ErrorRate float64 `json:"error_rate"`
	Status    string  `json:"status"`
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		panic(fmt.Sprintf("invalid %s: %s", key, v))
	}
	return n
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		panic(fmt.Sprintf("invalid %s: %s", key, v))
	}
	return d
}
```

**Step 2: Verify perfmaxrps compiles**

Run:

```bash
cd apps/backend && go build ./cmd/perfmaxrps
```

Expected: build succeeds.

**Step 3: Commit**

```bash
git add apps/backend/cmd/perfmaxrps
git commit -m "feat(perf): add perfmaxrps single-endpoint benchmark runner"
```

---

## Task 7: Implement suite orchestration and reporting

**Files:**
- Modify: `apps/backend/cmd/perfmaxrps/main.go`

**Step 1: Add suite and report functions**

Append to `apps/backend/cmd/perfmaxrps/main.go`:

```go
var endpointManifest = []string{
	"get_me",
	"get_properties",
	"get_property",
	"post_property",
	// TODO: add remaining endpoints in Task 9
}

func runSuite(logger *slog.Logger, opts options) error {
	var results []endpointResult
	failed := false

	for _, endpoint := range endpointManifest {
		result, err := benchmarkEndpoint(logger, opts, endpoint)
		if err != nil {
			logger.Error("benchmark failed", slog.String("endpoint", endpoint), slog.String("error", err.Error()))
			failed = true
			continue
		}
		results = append(results, result)
		if result.Status != "pass" {
			failed = true
		}
	}

	if err := writeReport(opts.outputDir, results); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	if failed {
		return fmt.Errorf("one or more endpoints failed")
	}
	return nil
}

func writeReport(dir string, results []endpointResult) error {
	jsonPath := filepath.Join(dir, "results.json")
	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(jsonPath, jsonData, 0o644); err != nil {
		return err
	}

	mdPath := filepath.Join(dir, "report.md")
	var sb strings.Builder
	sb.WriteString("# Perf Benchmark Report\n\n")
	sb.WriteString("| Endpoint | Max RPS | p95 (ms) | Error Rate | Status |\n")
	sb.WriteString("|----------|---------|----------|------------|--------|\n")
	for _, r := range results {
		fmt.Fprintf(&sb, "| %s | %.0f | %.2f | %.4f | %s |\n", r.Endpoint, r.MaxRPS, r.P95, r.ErrorRate, r.Status)
	}
	return os.WriteFile(mdPath, []byte(sb.String()), 0o644)
}
```

**Step 2: Commit**

```bash
git add apps/backend/cmd/perfmaxrps/main.go
git commit -m "feat(perf): add suite orchestration and report generation"
```

---

## Task 8: Add health check and env validation

**Files:**
- Modify: `apps/backend/cmd/perfmaxrps/main.go`

**Step 1: Add preflight checks**

Insert before `run(logger, opts)` in `main`:

```go
if err := preflightChecks(opts); err != nil {
	logger.Error("preflight failed", slog.String("error", err.Error()))
	os.Exit(1)
}
```

Add functions:

```go
func preflightChecks(opts options) error {
	if err := checkPostgres(); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	if err := checkBackend(opts.apiBaseURL); err != nil {
		return fmt.Errorf("backend: %w", err)
	}
	return nil
}

func checkPostgres() error {
	cmd := exec.Command("docker", "compose", "-f", "docker-compose.perf.yml", "ps", "-q")
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return fmt.Errorf("perf postgres is not running; run 'make perf-db-up'")
	}
	return nil
}

func checkBackend(baseURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/me", nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized && res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", res.StatusCode)
	}
	return nil
}
```

**Step 2: Add `net/http` import**

Ensure `net/http` is imported in `apps/backend/cmd/perfmaxrps/main.go`.

**Step 3: Commit**

```bash
git add apps/backend/cmd/perfmaxrps/main.go
git commit -m "feat(perf): add preflight health checks"
```

---

## Task 9: Add all remaining endpoints to k6 manifest

**Files:**
- Modify: `apps/backend/perf/scripts/endpoints.js`

**Step 1: Map all endpoints from OpenAPI/handlers**

Add entries for every endpoint in the backend:

- Auth: `auth_send_code`, `auth_verify_code`, `auth_logout`
- Properties: `get_properties`, `post_property`, `get_property`, `patch_property`, `archive_property`, `unarchive_property`
- Operations: `list_operations`, `create_operation`, `get_operation`, `update_operation`, `delete_operation`
- Recurring operations: `list_recurring_operations`, `create_recurring_operation`, `get_recurring_operation`, `update_recurring_operation`, `pause_recurring_operation`, `resume_recurring_operation`
- Leases: `list_leases`, `create_lease`, `get_lease`, `update_lease`, `complete_lease`
- Tenant contacts: `list_tenant_contacts`, `create_tenant_contact`, `get_tenant_contact`, `update_tenant_contact`
- Reminders: `list_reminders`, `get_operation_reminders`, `create_operation_reminder`, `get_recurring_operation_reminders`, `create_recurring_operation_reminder`, `get_lease_reminders`, `update_reminder`, `delete_reminder`
- Subscription: `get_tariffs`, `get_subscription`, `patch_subscription_auto_renew`, `cancel_subscription`, `change_subscription`, `list_subscription_payments`, `list_payment_methods`, `add_payment_method`, `delete_payment_method`, `activate_payment_method`

Use deterministic IDs from `ownerIndex(ctx)` for path parameters. For `get_property`, map index to the seeded `propertyID` if k6 knows it; otherwise use a seeded UUID derived from owner index.

**Step 2: Verify all endpoints parse**

Run:

```bash
for ep in get_me get_properties post_property; do
  k6 run --env ENDPOINT=$ep --env RATE=1 --env DURATION=1s --env API_BASE_URL=http://localhost:8081 apps/backend/perf/scripts/endpoint_benchmark.js 2>&1 | head -5
done
```

Expected: no JavaScript errors for each endpoint.

**Step 3: Commit**

```bash
git add apps/backend/perf/scripts/endpoints.js
git commit -m "feat(perf): add full endpoint manifest"
```

---

## Task 10: Validate single endpoint and suite

**Files:**
- Modify: any files needed to fix issues found during validation

**Step 1: Start perf stack**

Terminal 1:

```bash
make perf-db-up
make perf-seed
make perf-backend-run
```

Terminal 2:

```bash
make perf-endpoint-max-rps ENDPOINT=get_properties
```

Expected: `perfmaxrps` runs k6 several times, converges on a max RPS, and prints the result.

**Step 2: Run a small subset suite**

```bash
make perf-endpoints-max-rps
```

(First modify the manifest in `cmd/perfmaxrps/main.go` to include only 3-5 endpoints for faster validation.)

Expected: report is written to `apps/backend/perf/results/endpoints/<run-id>/report.md`.

**Step 3: Fix any fixture exhaustion or payload errors**

Iterate on `perfseed`, `endpoints.js`, and `perfmaxrps` until the subset passes cleanly.

**Step 4: Commit fixes**

```bash
git add -A
git commit -m "fix(perf): resolve validation issues"
```

---

## Task 11: Add documentation and changelog

**Files:**
- Create: `apps/backend/perf/README.md`
- Modify: `CHANGELOG.md`

**Step 1: Write perf README**

Create `apps/backend/perf/README.md`:

```markdown
# Backend Performance Harness

Local k6-based load tests for `apps/backend`.

## Quick start

```bash
make perf-db-up
make perf-seed
make perf-backend-run        # in another terminal
make perf-endpoints-max-rps  # runs all endpoints
```

## Single endpoint

```bash
make perf-endpoint-max-rps ENDPOINT=get_properties
```

## Configuration

Copy `.env.perf.example` to `.env.perf`, fill `ENCRYPTION_KEY`, and run `source .env.perf` before commands, or use the Make defaults.

## Reports

Results are written to `apps/backend/perf/results/endpoints/<run-id>/`.
```

**Step 2: Update CHANGELOG.md**

Add an entry under today's date:

```markdown
## 2026-06-19

- Added local k6 performance harness to measure max RPS per endpoint with p95 < 500 ms.
```

**Step 3: Final verification**

Run:

```bash
make backend-lint
cd apps/backend && go test ./...
cd apps/backend && go vet ./...
```

Expected: all checks pass (or only pre-existing failures).

**Step 4: Commit**

```bash
git add apps/backend/perf/README.md CHANGELOG.md
git commit -m "docs(perf): add perf harness README and changelog entry"
```
