#!/usr/bin/env bash
set -euo pipefail

# Frontend Playwright e2e orchestrator (ticket #456, spec #453).
#
# Brings up a deterministic, disposable stack and runs apps/frontend/e2e:
#   1. postgres 18 — compose project arenda-e2e (slot 0), port 5436
#      (apps/backend/docker-compose.e2e.yml), wiped on exit;
#   2. backend — compiled from apps/backend, auto-migration on boot (the
#      seed waits for the schema), APP_ENV=local, EMAIL_SENDER=fake
#      (the login code lands in the JSON log), PAYMENT_PROVIDER=fake;
#   3. frontend — production build served by the next standalone server
#      (the exact runtime of apps/frontend/Dockerfile), /api proxied to the
#      backend via BACKEND_URL;
#   4. seed — tools/e2e/frontend/seed.sql: owner user, pre-authenticated
#      session (raw token exported to Playwright as E2E_SESSION_TOKEN),
#      two active properties; the postgres container is also exported
#      (E2E_PG_CONTAINER) so specs can seed mid-test data over SQL
#      (the #468 lifecycle overdue leg);
#   5. npx playwright test (apps/frontend/playwright.config.ts) — skipped when
#      E2E_LIVE=1: the stack stays up seeded for a live UI walkthrough
#      (.agents/skills/ui-walkthrough), the connection facts print at the end.
#
# Entry points: `make frontend-e2e` (local), `make frontend-e2e-headed`
# (visible browser windows), `make frontend-e2e-live-up` / `-live-down`
# (walkthrough stack), and the frontend-e2e CI job.
#
# Parallel worktree sessions (docs/agents/parallel-dev.md): the make targets
# source the checkout's root .env, so a worktree's E2E_* arrive here via the
# process env and each slot gets its own ports and compose project —
# overriding any of the defaults below.
#
# Overrides (rarely needed): E2E_PG_PORT (5436), E2E_BACKEND_PORT (8081),
# E2E_FRONTEND_PORT (3010), E2E_COMPOSE_PROJECT (arenda-e2e; the postgres
# container name follows it: <project>-postgres-1), E2E_ENCRYPTION_KEY
# (64-hex test key; the seeded session hash must match the backend HMAC).
# E2E_KEEP_STACK=1 leaves the stack up after the run for manual inspection.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
FRONTEND_DIR="$PROJECT_ROOT/apps/frontend"
SEED_SQL="$SCRIPT_DIR/seed.sql"
WORK_DIR="$PROJECT_ROOT/.tmp/e2e-frontend"
BACKEND_LOG="$WORK_DIR/backend.log"
FRONTEND_LOG="$WORK_DIR/frontend.log"
BACKEND_BIN="$WORK_DIR/arenda-api"

export E2E_PG_PORT="${E2E_PG_PORT:-5436}"
export E2E_BACKEND_PORT="${E2E_BACKEND_PORT:-8081}"
export E2E_FRONTEND_PORT="${E2E_FRONTEND_PORT:-3010}"
export E2E_COMPOSE_PROJECT="${E2E_COMPOSE_PROJECT:-arenda-e2e}"

# E2E_LIVE=1 raises the seeded stack without running Playwright — the
# walkthrough's playground. The stack must outlive this invocation.
if [ "${E2E_LIVE:-0}" = "1" ]; then
  E2E_KEEP_STACK=1
fi
# Test-only AES-256 key (hex). Not a secret: it pins the backend token HMAC
# so the seeded session row matches the raw token Playwright puts in a cookie.
E2E_ENCRYPTION_KEY="${E2E_ENCRYPTION_KEY:-0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef}"

COMPOSE=(docker compose -p "$E2E_COMPOSE_PROJECT" -f "$PROJECT_ROOT/apps/backend/docker-compose.e2e.yml")
# Compose names default containers <project>-<service>-<index>; the project
# name is the slot's isolation boundary, so the container name follows it.
PG_CONTAINER="${E2E_COMPOSE_PROJECT}-postgres-1"
DATABASE_URL="postgres://arenda:arenda@localhost:${E2E_PG_PORT}/arenda?sslmode=disable"
BACKEND_URL="http://127.0.0.1:${E2E_BACKEND_PORT}"
FRONTEND_URL="http://127.0.0.1:${E2E_FRONTEND_PORT}"

USER_PHONE_DIGITS="9150000001"
USER_EMAIL="e2e@example.com"

BACKEND_PID=""
FRONTEND_PID=""

log() { echo "[e2e $(date +%H:%M:%S)] $*"; }

die() { echo "ERROR: $*" >&2; exit 1; }

cleanup() {
  if [ "${E2E_KEEP_STACK:-0}" = "1" ]; then
    log "E2E_KEEP_STACK=1 — stack left running:"
    log "  backend  $BACKEND_URL/healthz  (log: $BACKEND_LOG)"
    log "  frontend $FRONTEND_URL         (log: $FRONTEND_LOG)"
    log "  postgres localhost:$E2E_PG_PORT (compose project $E2E_COMPOSE_PROJECT)"
    log "Tear down later: ${COMPOSE[*]} down -v"
    return 0
  fi
  [ -z "$FRONTEND_PID" ] || kill "$FRONTEND_PID" 2>/dev/null || true
  [ -z "$BACKEND_PID" ] || kill "$BACKEND_PID" 2>/dev/null || true
  "${COMPOSE[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

wait_for_url() {
  local url="$1" attempts="$2" label="$3" log_file="$4" i
  for i in $(seq 1 "$attempts"); do
    if curl -fsS -o /dev/null "$url"; then
      return 0
    fi
    sleep 1
  done
  echo "--- $label did not come up at $url; log tail: ---" >&2
  tail -n 50 "$log_file" >&2 || true
  return 1
}

mkdir -p "$WORK_DIR"
: > "$BACKEND_LOG"
: > "$FRONTEND_LOG"

command -v docker >/dev/null 2>&1 || die "docker is required (postgres runs via compose)"
command -v node >/dev/null 2>&1 || die "node is required (frontend build, playwright)"
command -v go >/dev/null 2>&1 || die "go is required (backend build)"

# Deterministic start: a hard-killed previous run (or E2E_KEEP_STACK=1 leftovers)
# may hold the ports — a stale frontend here would silently serve an old build.
free_port() {
  local pids
  pids="$(lsof -ti tcp:"$1" -sTCP:LISTEN 2>/dev/null || true)"
  if [ -n "$pids" ]; then
    log "Freeing port $1 (pids: $pids)"
    kill $pids 2>/dev/null || true
    sleep 1
  fi
}
free_port "$E2E_BACKEND_PORT"
free_port "$E2E_FRONTEND_PORT"

"${COMPOSE[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
log "Starting postgres ($E2E_COMPOSE_PROJECT, port $E2E_PG_PORT)"
"${COMPOSE[@]}" up -d --wait

log "Building backend"
(cd "$PROJECT_ROOT" && go build -o "$BACKEND_BIN" ./apps/backend/cmd/api)

# The e2e stack stays hermetic: the make targets source the checkout's root
# .env (worktree slots), and OTEL_* from it must not turn this disposable
# backend into a telemetry exporter pointed at the shared observability.
log "Starting backend on $BACKEND_URL"
# Подсказки адреса: по умолчанию dummy-ключ (endpoint тихо деградирует —
# спека гейтится на DADATA_BASE_URL); экспортированный настоящий ключ
# (например, из корневого .env) включает живой DaData в локальных прогонах.
E2E_DADATA_API_KEY="${DADATA_API_KEY:-e2e-dadata-dummy}"
# IP-лимитер ослаблен: сюит прогревает ~14 API на маунт хаба (#626) и живёт
# одним IP — дефолт 20 rps/burst 40 пробивается 429 на мутациях (#688).
E2E_RATE_LIMIT_IP_RPS=200
E2E_RATE_LIMIT_IP_BURST=400
(
  cd "$PROJECT_ROOT"
  APP_ENV=local \
  HTTP_ADDR="127.0.0.1:${E2E_BACKEND_PORT}" \
  DATABASE_URL="$DATABASE_URL" \
  MIGRATIONS_DIR=apps/backend/db/migrations \
  COOKIE_SECURE=false \
  EMAIL_SENDER=fake \
  PAYMENT_PROVIDER=fake \
  APP_BASE_URL="$BACKEND_URL" \
  WEB_ORIGIN="$FRONTEND_URL" \
  ENCRYPTION_KEY="$E2E_ENCRYPTION_KEY" \
  DADATA_API_KEY="$E2E_DADATA_API_KEY" \
  RATE_LIMIT_IP_RPS="$E2E_RATE_LIMIT_IP_RPS" \
  RATE_LIMIT_IP_BURST="$E2E_RATE_LIMIT_IP_BURST" \
  LOG_FORMAT=json \
  LOG_LEVEL=info \
  LOG_SUCCESSFUL_REQUESTS=false \
  OTEL_TRACES_EXPORTER=none \
  OTEL_METRICS_EXPORTER=none \
  OTEL_EXPORTER_OTLP_ENDPOINT= \
  exec "$BACKEND_BIN"
) > "$BACKEND_LOG" 2>&1 &
BACKEND_PID=$!

wait_for_url "$BACKEND_URL/healthz" 60 "backend" "$BACKEND_LOG"
log "Backend is healthy"

# healthz answers while the boot auto-migration (wire.WirePlatform) may still
# be running — the seed's INSERTs need the schema, so poll for it explicitly.
log "Waiting for migrations"
for i in $(seq 1 60); do
  if docker exec "$PG_CONTAINER" psql -U arenda -d arenda -tAc \
    "SELECT to_regclass('public.users')" 2>/dev/null | grep -q users; then
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "--- migrations did not land in 60s; backend log tail: ---" >&2
    tail -n 50 "$BACKEND_LOG" >&2 || true
    die "migrations did not land in 60s"
  fi
  sleep 1
done
log "Schema is ready"

log "Seeding database"
E2E_SESSION_TOKEN="$(node -e 'console.log(require("node:crypto").randomBytes(32).toString("hex"))')"
E2E_MEMBER_SESSION_TOKEN="$(node -e 'console.log(require("node:crypto").randomBytes(32).toString("hex"))')"
E2E_VIEWER_SESSION_TOKEN="$(node -e 'console.log(require("node:crypto").randomBytes(32).toString("hex"))')"
# token_hash mirrors encryption.hashToken (HMAC-SHA256 over the raw token);
# phone_det mirrors encryption.DeterministicEncrypt — users.phone is looked
# up by its ciphertext, so a plaintext seed row would never match.
CRYPTO="$SCRIPT_DIR/e2e-crypto.mjs"
TOKEN_HASH="$(node "$CRYPTO" hash-token "$E2E_ENCRYPTION_KEY" "$E2E_SESSION_TOKEN")"
MEMBER_TOKEN_HASH="$(node "$CRYPTO" hash-token "$E2E_ENCRYPTION_KEY" "$E2E_MEMBER_SESSION_TOKEN")"
VIEWER_TOKEN_HASH="$(node "$CRYPTO" hash-token "$E2E_ENCRYPTION_KEY" "$E2E_VIEWER_SESSION_TOKEN")"
PHONE_DET="$(node "$CRYPTO" det-phone "$E2E_ENCRYPTION_KEY" "+7$USER_PHONE_DIGITS")"
MEMBER_PHONE_DET="$(node "$CRYPTO" det-phone "$E2E_ENCRYPTION_KEY" "+79150000002")"
VIEWER_PHONE_DET="$(node "$CRYPTO" det-phone "$E2E_ENCRYPTION_KEY" "+79150000003")"
docker exec -i "$PG_CONTAINER" \
  psql -U arenda -d arenda -v ON_ERROR_STOP=1 \
  -v token_hash="$TOKEN_HASH" -v phone_det="$PHONE_DET" \
  -v member_token_hash="$MEMBER_TOKEN_HASH" -v member_phone_det="$MEMBER_PHONE_DET" \
  -v viewer_token_hash="$VIEWER_TOKEN_HASH" -v viewer_phone_det="$VIEWER_PHONE_DET" \
  < "$SEED_SQL" >/dev/null

# Поверх базового сида всегда применяется оверлей (live-overlay.sql):
# правила состояний, которых в seed.sql намеренно нет (weekly / доходные /
# endDate в будущем). Базовый сид держит точные количества для спеков,
# оверлей только добавляет правила с `since` в будущем — на просрочки и
# операции он не влияет. Идемпотентен при пересиде.
log "Applying live walkthrough overlay seed"
docker exec -i "$PG_CONTAINER" \
  psql -U arenda -d arenda -v ON_ERROR_STOP=1 \
  < "$SCRIPT_DIR/live-overlay.sql" >/dev/null

log "Building frontend (production standalone)"
if ! (cd "$FRONTEND_DIR" && NEXT_TELEMETRY_DISABLED=1 npm run build) > "$FRONTEND_LOG" 2>&1; then
  echo "--- frontend build failed; log tail: ---" >&2
  tail -n 50 "$FRONTEND_LOG" >&2 || true
  die "frontend build failed"
fi
# next build leaves public/ and .next/static outside the standalone bundle —
# stage them the same way apps/frontend/Dockerfile does for production.
rm -rf "$FRONTEND_DIR/.next/standalone/public" "$FRONTEND_DIR/.next/standalone/.next/static"
cp -R "$FRONTEND_DIR/public" "$FRONTEND_DIR/.next/standalone/public"
mkdir -p "$FRONTEND_DIR/.next/standalone/.next"
cp -R "$FRONTEND_DIR/.next/static" "$FRONTEND_DIR/.next/standalone/.next/static"

log "Starting frontend on $FRONTEND_URL"
(
  cd "$FRONTEND_DIR"
  PORT="$E2E_FRONTEND_PORT" \
  HOSTNAME=127.0.0.1 \
  BACKEND_URL="$BACKEND_URL" \
  exec node .next/standalone/server.js
) >> "$FRONTEND_LOG" 2>&1 &
FRONTEND_PID=$!
wait_for_url "$FRONTEND_URL/login" 60 "frontend" "$FRONTEND_LOG"
log "Frontend is up"

if [ "${E2E_LIVE:-0}" = "1" ]; then
  log "Live walkthrough stack is ready (kept running):"
  echo "  frontend  $FRONTEND_URL          (go here)"
  echo "  backend   $BACKEND_URL/healthz   (log: $BACKEND_LOG)"
  echo "  seed      phone +7$USER_PHONE_DIGITS, email $USER_EMAIL, session token:"
  echo "            $E2E_SESSION_TOKEN"
  echo "  roles     подмена роли — cookie session_id=<token> на $FRONTEND_URL:"
  echo "            owner       +7$USER_PHONE_DIGITS  $E2E_SESSION_TOKEN"
  echo "            full access +79150000002         $E2E_MEMBER_SESSION_TOKEN"
  echo "            viewer      +79150000003         $E2E_VIEWER_SESSION_TOKEN"
  echo "  teardown  make frontend-e2e-live-down"
  exit 0
fi

log "Installing Playwright browsers (idempotent)"
(cd "$FRONTEND_DIR" && npx playwright install chromium) >/dev/null

log "Running Playwright"
status=0
(
  cd "$FRONTEND_DIR"
  E2E_BASE_URL="$FRONTEND_URL" \
  E2E_SESSION_TOKEN="$E2E_SESSION_TOKEN" \
  E2E_MEMBER_SESSION_TOKEN="$E2E_MEMBER_SESSION_TOKEN" \
  E2E_VIEWER_SESSION_TOKEN="$E2E_VIEWER_SESSION_TOKEN" \
  E2E_USER_PHONE="$USER_PHONE_DIGITS" \
  E2E_USER_EMAIL="$USER_EMAIL" \
  E2E_BACKEND_LOG="$BACKEND_LOG" \
  E2E_PG_CONTAINER="$PG_CONTAINER" \
  npx playwright test "$@"
) || status=$?

if [ "$status" -eq 0 ]; then
  log "Playwright passed"
else
  log "Playwright failed (exit $status); reports: $FRONTEND_DIR/playwright-report, logs: $WORK_DIR"
fi
exit "$status"
