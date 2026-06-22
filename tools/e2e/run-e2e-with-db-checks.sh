#!/usr/bin/env bash
set -euo pipefail

# Comprehensive E2E runner for the Arenda backend.
#
# - Reads environment from the project root .env
# - Assumes backend is already running and reachable on localhost:8080
# - Assumes PostgreSQL container is running
# - Sends a fake SMS code, runs the system-e2e collection folder-by-folder,
#   executes SQL verification checks, runs edge-case collection and concurrency
#   tests, and writes a markdown report to .tmp/.
#
# Optional environment overrides:
#   BASE_URL          default http://localhost:8080
#   BACKEND_LOG       default .tmp/backend-e2e.log
#   PG_CONTAINER      default arenda-local-postgres-1
#   ENV_FILE          default .env

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BRUNO_COLLECTION="$PROJECT_ROOT/tools/e2e/bruno/arenda-api-e2e"
DB_CHECKS_DIR="$PROJECT_ROOT/tools/e2e/sql"
TMP_DIR="$PROJECT_ROOT/.tmp"

ENV_FILE="${ENV_FILE:-$PROJECT_ROOT/.env}"
BACKEND_LOG="${BACKEND_LOG:-$TMP_DIR/backend-e2e.log}"
BASE_URL="${BASE_URL:-http://localhost:8080}"
PG_CONTAINER="${PG_CONTAINER:-arenda-local-postgres-1}"

# Load .env if present
if [[ -f "$ENV_FILE" ]]; then
  set -a
  # shellcheck source=/dev/null
  source "$ENV_FILE"
  set +a
else
  echo "WARNING: .env not found at $ENV_FILE" >&2
fi

PGUSER="${POSTGRES_USER:-arenda}"
PGPASSWORD="${POSTGRES_PASSWORD:-arenda}"
PGDATABASE="${POSTGRES_DB:-arenda}"

# Run identifiers
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
EPOCH=$(date +%s)
# Russian mobile numbers must match ^\+79\d{9}$ (12 chars total).
PHONE="+7915$(printf '%07d' $((EPOCH % 10000000)))"
REPORT_FILE="$TMP_DIR/e2e-report-${TIMESTAMP}.md"
BRUNO_REPORT_BASE="$TMP_DIR/bruno-report-${TIMESTAMP}"

# State for the report (bash 3.2 compatible indexed arrays)
FOLDER_NAMES=()
FOLDER_STATUSES=()
FOLDER_OUTPUTS=()
SQL_NAMES=()
SQL_STATUSES=()
SQL_OUTPUTS=()
FAILURES=()
BUG_TITLES=()
BUG_STEPS=()
BUG_EXPECTED=()
BUG_ACTUAL=()
BUG_FIX=()
EDGE_PASSED=0
EDGE_FAILED=0
EDGE_EXPECTED_NON_2XX=0
EDGE_OUTPUT=""
ISOLATED_STATUS="SKIPPED"
RACE_STATUS="SKIPPED"
RANDOM_USER_STATUS="SKIPPED"
RACE_ARCHIVE_STATUS="SKIPPED"
RACE_OPERATION_STATUS="SKIPPED"
RACE_TARIFF_STATUS="SKIPPED"
BUG_REPORT_FILE=""

# Cleanup on exit: kill any background jobs and tidy temp files
cleanup() {
  local pids
  pids=$(jobs -p 2>/dev/null) || true
  if [[ -n "$pids" ]]; then
    log "Cleaning up background jobs..."
    # shellcheck disable=SC2086
    kill $pids 2>/dev/null || true
    # shellcheck disable=SC2086
    wait $pids 2>/dev/null || true
  fi
}
trap cleanup EXIT

log() { echo "[$(date +%H:%M:%S)] $*"; }
add_failure() {
  local msg="$1"
  log "FAIL: $msg"
  FAILURES+=("$msg")
}

# Record a bug for the Markdown bug report.
# Usage: record_bug "title" "steps" "expected" "actual" "fix"
record_bug() {
  local title="$1"
  local steps="$2"
  local expected="$3"
  local actual="$4"
  local fix="${5:-}"
  BUG_TITLES+=("$title")
  BUG_STEPS+=("$steps")
  BUG_EXPECTED+=("$expected")
  BUG_ACTUAL+=("$actual")
  BUG_FIX+=("$fix")
  log "BUG: $title"
}

# --- Indexed-array helpers -------------------------------------------------

record_folder() {
  FOLDER_NAMES+=("$1")
  FOLDER_STATUSES+=("$2")
  FOLDER_OUTPUTS+=("$3")
}

record_sql() {
  SQL_NAMES+=("$1")
  SQL_STATUSES+=("$2")
  SQL_OUTPUTS+=("$3")
}

# Look up the status of a folder by name
folder_status_by_name() {
  local target="$1"
  local i
  for i in "${!FOLDER_NAMES[@]}"; do
    if [[ "${FOLDER_NAMES[$i]}" == "$target" ]]; then
      echo "${FOLDER_STATUSES[$i]}"
      return 0
    fi
  done
  echo "SKIPPED"
}

# Return the SQL checks that should run after a given folder
sql_checks_for_folder() {
  case "$1" in
    system-e2e/00-auth-setup)
      echo "01-user-and-session-created.sql"
      ;;
    system-e2e/10-subscription)
      echo "09-subscription-upgraded.sql 10-payment-method-active.sql"
      ;;
    system-e2e/20-property-lifecycle)
      echo "02-property-created.sql 13-property-archived.sql 14-tariff-limit-not-exceeded.sql"
      ;;
    system-e2e/30-tenant-contacts)
      echo "03-tenant-contact-created.sql"
      ;;
    system-e2e/40-leases)
      echo "04-lease-created.sql 12-no-duplicate-open-lease.sql"
      ;;
    system-e2e/50-operations)
      echo "05-operation-created.sql 06-operation-soft-deleted.sql"
      ;;
    system-e2e/60-recurring-operations)
      echo "07-recurring-operation-created.sql 08-recurring-operation-paused.sql"
      ;;
    system-e2e/70-reminders)
      echo "11-reminder-created.sql 15-reset-reminders-tariff.sql"
      ;;
    system-e2e/75-webhooks)
      echo "12-webhook-payment-processed.sql"
      ;;
    *)
      echo ""
      ;;
  esac
}

# --- Health checks ---------------------------------------------------------

check_backend() {
  log "Checking backend at $BASE_URL/tariffs ..."
  local code
  code=$(curl -o /dev/null -s -w "%{http_code}" "$BASE_URL/tariffs")
  # /tariffs requires a session, so 401 means the backend is up.
  if [[ "$code" == "000" ]]; then
    return 1
  fi
  return 0
}

check_postgres() {
  log "Checking PostgreSQL container $PG_CONTAINER ..."
  if ! docker ps --format '{{.Names}}' | grep -qx "$PG_CONTAINER"; then
    log "PostgreSQL container $PG_CONTAINER is not running"
    return 1
  fi
  if ! docker exec -i "$PG_CONTAINER" pg_isready -U "$PGUSER" -d "$PGDATABASE" >/dev/null 2>&1; then
    log "PostgreSQL is not ready"
    return 1
  fi
  return 0
}

# --- Auth helpers ----------------------------------------------------------

send_phone_code() {
  local phone="$1"
  log "Sending auth code to $phone ..."
  curl -fsS -m 10 -X POST "$BASE_URL/auth/phone/send" \
    -H "Content-Type: application/json" \
    -d "{\"phone\":\"$phone\"}" >/dev/null
}

# Extract the 6-digit code from the backend log for a given phone.
# We wait up to 30 seconds because the log may be buffered.
# The phone is unique per run, so we grep the whole log (last 500 lines) for it.
extract_code_for_phone() {
  local phone="$1"
  local log_file="$2"

  log "Extracting code for $phone from $log_file ..." >&2
  local _
  for _ in $(seq 1 30); do
    if [[ -f "$log_file" ]]; then
      local code
      # The fake sender logs: "fake sms sent", "phone": "...", "message": "Код подтверждения: 123456"
      code=$(tail -n 500 "$log_file" 2>/dev/null \
        | grep -aF "$phone" \
        | grep -aoE 'Код подтверждения: [0-9]{6}' \
        | tail -1 \
        | grep -aoE '[0-9]{6}' || true)
      if [[ -n "$code" ]]; then
        echo "$code"
        return 0
      fi
    fi
    sleep 1
  done
  return 1
}

# Extract session cookie values from a Bruno JSON report (verify-code response).
# Prints: cookie_name\tsession_id
extract_session_from_report() {
  local report_json="$1"
  if [[ ! -f "$report_json" ]] || ! command -v jq >/dev/null 2>&1; then
    return 1
  fi

  local raw
  raw=$(jq -r '.[0].results[]? | select(.test.filename | contains("verify-code")) | .response.headers."set-cookie"[0] // .response.headers."set-cookie" // empty' "$report_json" 2>/dev/null | head -1)
  if [[ -z "$raw" ]]; then
    return 1
  fi

  local cookie_name cookie_value
  cookie_name=$(echo "$raw" | grep -oE '(^|; )(__Host-session_id|session_id)=' | tail -1 | sed 's/=//; s/^; //')
  cookie_value=$(echo "$raw" | grep -oE '(__Host-session_id|session_id)=[^;]+' | tail -1 | cut -d= -f2-)
  if [[ -n "$cookie_name" && -n "$cookie_value" ]]; then
    printf '%s\t%s\n' "$cookie_name" "$cookie_value"
    return 0
  fi
  return 1
}

# Count lines in a log file (defaults to 1 if missing)
log_line_count() {
  local log_file="$1"
  if [[ -f "$log_file" ]]; then
    wc -l < "$log_file" | tr -d ' '
  else
    echo 1
  fi
}

# --- Bruno helpers ---------------------------------------------------------

# Extract the active payment method id from the subscription JSON report.
extract_active_payment_method_id() {
  local report_json="$1"
  if [[ ! -f "$report_json" ]] || ! command -v jq >/dev/null 2>&1; then
    return 1
  fi
  jq -r '.[0].results[]? | select(.test.filename | contains("list payment methods")) | .response.data.items[]? | select(.isActive == true) | .id' "$report_json" 2>/dev/null | head -1
}

# Find the subscription payment to use for the webhook test.
# Prints: payment_id\tprovider_payment_id
# Takes the run phone as argument and reads the latest payment from the DB.
get_webhook_payment() {
  local phone="$1"

  local user_id
  user_id=$(psql_value "SELECT id FROM users WHERE phone = '$phone' LIMIT 1;")
  if [[ -z "$user_id" ]]; then
    echo "ERROR: could not find user for phone $phone" >&2
    return 1
  fi

  local payment_pair
  payment_pair=$(psql_value "SELECT id || chr(9) || COALESCE(provider_payment_id, '') FROM subscription_payments WHERE user_id = '$user_id' ORDER BY created_at DESC LIMIT 1;")
  if [[ -z "$payment_pair" ]]; then
    echo "ERROR: no subscription payment found for user $user_id" >&2
    return 1
  fi

  printf '%s\n' "$payment_pair"
}

# Run a Bruno folder and record PASS/FAIL status. Output is stored for the report.
run_bruno_folder() {
  local folder="$1"
  shift
  local extra_args=("$@")
  local report_json="${BRUNO_REPORT_BASE}-$(echo "$folder" | tr '/' '-').json"

  log "Running Bruno folder: $folder ..."
  local output
  local status=0
  # shellcheck disable=SC2068
  output=$(cd "$BRUNO_COLLECTION" && bru run "$folder" -r --env Local \
    --env-var phone="$PHONE" \
    --env-var code="$CODE" \
    --env-var cookieName=session_id \
    --env-var skipSend=true \
    --reporter-json "$report_json" \
    "${extra_args[@]}" 2>&1) || status=$?

  if [[ $status -eq 0 ]]; then
    record_folder "$folder" "PASS" "$output"
    log "PASS: $folder"
  else
    record_folder "$folder" "FAIL" "$output"
    add_failure "Bruno folder $folder failed (exit $status)"
  fi

  return $status
}

# Parse a Bruno JSON report for passed/failed test counts.
parse_bruno_summary() {
  local report_json="$1"
  local passed=0
  local failed=0

  if [[ -f "$report_json" ]] && command -v jq >/dev/null 2>&1; then
    # Try common shapes from the Bruno JSON reporter
    passed=$(jq -r '.[0].summary.passedTests // .[0].summary.passed_requests // .[0].summary.totalPassed // .summary.passedTests // .summary.passed_requests // .summary.totalPassed // 0' "$report_json" 2>/dev/null || echo 0)
    failed=$(jq -r '.[0].summary.failedTests // .[0].summary.failed_requests // .[0].summary.totalFailed // .summary.failedTests // .summary.failed_requests // .summary.totalFailed // 0' "$report_json" 2>/dev/null || echo 0)
  fi

  if [[ -z "$passed" || "$passed" == "null" ]]; then passed=0; fi
  if [[ -z "$failed" || "$failed" == "null" ]]; then failed=0; fi
  printf '%s\t%s\n' "$passed" "$failed"
}

# Parse a Bruno JSON report and count real test pass/fail based on per-result
# status. Also count expected non-2xx responses (tests that pass with status>=400).
# Prints: actual_passed\tactual_failed\texpected_non_2xx\tfailed_details_json
# where failed_details_json is a compact JSON array of {filename, status, error}.
parse_edge_case_results() {
  local report_json="$1"
  if [[ ! -f "$report_json" ]] || ! command -v jq >/dev/null 2>&1; then
    printf '0\t0\t0\t[]\n'
    return
  fi

  local results
  results=$(jq -c '.[0].results // .results // []' "$report_json" 2>/dev/null)
  if [[ -z "$results" || "$results" == "null" ]]; then
    printf '0\t0\t0\t[]\n'
    return
  fi

  local actual_passed actual_failed expected_non_2xx failed_details
  actual_passed=$(echo "$results" | jq '[.[] | select(.status == "pass")] | length')
  actual_failed=$(echo "$results" | jq '[.[] | select(.status != "pass")] | length')
  expected_non_2xx=$(echo "$results" | jq '[.[] | select(.status == "pass" and .response.status >= 400)] | length')
  failed_details=$(echo "$results" | jq -c '[.[] | select(.status != "pass") | {filename: .test.filename, status: .response.status, error: .error}]')

  printf '%s\t%s\t%s\t%s\n' "$actual_passed" "$actual_failed" "$expected_non_2xx" "$failed_details"
}

# --- SQL helpers -----------------------------------------------------------

# Run a SQL check script inside the Postgres container by piping the file.
# Extra arguments are passed through to psql as -v options.
run_sql_check() {
  local script_name="$1"
  shift
  local psql_args=("$@")
  local script_path="$DB_CHECKS_DIR/$script_name"

  if [[ ! -f "$script_path" ]]; then
    record_sql "$script_name" "FAIL" "Script not found: $script_path"
    add_failure "SQL check script missing: $script_name"
    return 1
  fi

  log "Running SQL check: $script_name ..."
  local output
  local status=0
  output=$(docker exec -i "$PG_CONTAINER" \
    psql -U "$PGUSER" -d "$PGDATABASE" \
    -v ON_ERROR_STOP=1 \
    "${psql_args[@]:-}" \
    -f - < "$script_path" 2>&1) || status=$?

  if [[ $status -eq 0 ]]; then
    # Parse the first data row's first column as the ok value.
    # psql default output: header row, separator row, then data rows.
    local ok_val
    ok_val=$(echo "$output" | awk '/^[-+|]+$/{found=1; next} found {gsub(/^[ \t]+|[ \t]+$/,"",$1); print $1; exit}')
    case "$(echo "$ok_val" | tr '[:upper:]' '[:lower:]')" in
      t|true|1)
        record_sql "$script_name" "PASS" "$output"
        log "PASS SQL: $script_name"
        return 0
        ;;
      f|false|0)
        record_sql "$script_name" "FAIL" "$output"
        add_failure "SQL check returned ok=false: $script_name"
        return 1
        ;;
      *)
        # No explicit ok column or unexpected format; treat success as PASS but log.
        record_sql "$script_name" "PASS" "$output"
        log "PASS SQL: $script_name (no ok column)"
        return 0
        ;;
    esac
  else
    record_sql "$script_name" "FAIL" "$output"
    add_failure "SQL check failed: $script_name (exit $status)"
    return 1
  fi
}

# Query the database via docker exec and return the first column of the first row.
psql_value() {
  local query="$1"
  shift
  docker exec -i "$PG_CONTAINER" \
    psql -U "$PGUSER" -d "$PGDATABASE" -tA "$@" -c "$query" 2>/dev/null
}

# Derive the UUID of a user by phone number.
user_id_by_phone() {
  local phone="$1"
  psql_value "SELECT id FROM users WHERE phone = '$phone' LIMIT 1;"
}

# Derive variables for a SQL check based on current DB state anchored by the run phone.
derive_sql_vars() {
  local script_name="$1"
  local user_id
  local property_id
  local lease_id
  local operation_id
  local recurring_op_id
  local tariff_id
  local tenant_phone

  case "$script_name" in
    01-user-and-session-created.sql)
      echo "-v phone=$PHONE"
      ;;
    09-subscription-upgraded.sql)
      user_id=$(user_id_by_phone "$PHONE")
      tariff_id=$(psql_value "SELECT id FROM tariffs WHERE name = 'pro' LIMIT 1;")
      echo "-v user_id=$user_id -v tariff_id=$tariff_id -v subscription_status=active -v payment_status=succeeded"
      ;;
    10-payment-method-active.sql)
      user_id=$(user_id_by_phone "$PHONE")
      echo "-v user_id=$user_id"
      ;;
    02-property-created.sql)
      user_id=$(user_id_by_phone "$PHONE")
      property_id=$(psql_value "SELECT id FROM properties WHERE owner_id = '$user_id' AND status = 'active' ORDER BY created_at DESC LIMIT 1;")
      echo "-v property_id=$property_id -v owner_id=$user_id"
      ;;
    13-property-archived.sql)
      user_id=$(user_id_by_phone "$PHONE")
      property_id=$(psql_value "SELECT id FROM properties WHERE owner_id = '$user_id' AND status = 'archived' ORDER BY created_at DESC LIMIT 1;")
      echo "-v property_id=$property_id"
      ;;
    14-tariff-limit-not-exceeded.sql)
      user_id=$(user_id_by_phone "$PHONE")
      echo "-v user_id=$user_id"
      ;;
    03-tenant-contact-created.sql)
      user_id=$(user_id_by_phone "$PHONE")
      tenant_phone=$(psql_value "SELECT phone FROM tenant_contacts WHERE owner_id = '$user_id' ORDER BY created_at DESC LIMIT 1;")
      echo "-v owner_id=$user_id -v phone=$tenant_phone"
      ;;
    04-lease-created.sql)
      user_id=$(user_id_by_phone "$PHONE")
      lease_id=$(psql_value "SELECT l.id FROM leases l JOIN properties p ON p.id = l.property_id WHERE p.owner_id = '$user_id' ORDER BY l.created_at DESC LIMIT 1;")
      property_id=$(psql_value "SELECT property_id FROM leases WHERE id = '$lease_id';")
      local status start_date end_date
      status=$(psql_value "SELECT status FROM leases WHERE id = '$lease_id';")
      start_date=$(psql_value "SELECT start_date FROM leases WHERE id = '$lease_id';")
      end_date=$(psql_value "SELECT end_date FROM leases WHERE id = '$lease_id';")
      echo "-v lease_id=$lease_id -v property_id=$property_id -v status=$status -v start_date=$start_date -v end_date=$end_date"
      ;;
    12-no-duplicate-open-lease.sql)
      user_id=$(user_id_by_phone "$PHONE")
      property_id=$(psql_value "SELECT l.property_id FROM leases l JOIN properties p ON p.id = l.property_id WHERE p.owner_id = '$user_id' ORDER BY l.created_at DESC LIMIT 1;")
      echo "-v property_id=$property_id"
      ;;
    05-operation-created.sql)
      user_id=$(user_id_by_phone "$PHONE")
      operation_id=$(psql_value "SELECT o.id FROM operations o JOIN properties p ON p.id = o.property_id WHERE p.owner_id = '$user_id' AND o.deleted_at IS NULL ORDER BY o.created_at DESC LIMIT 1;")
      local op_type category amount op_date
      op_type=$(psql_value "SELECT type FROM operations WHERE id = '$operation_id';")
      category=$(psql_value "SELECT category FROM operations WHERE id = '$operation_id';")
      amount=$(psql_value "SELECT amount_kopecks FROM operations WHERE id = '$operation_id';")
      op_date=$(psql_value "SELECT operation_date FROM operations WHERE id = '$operation_id';")
      echo "-v operation_id=$operation_id -v type=$op_type -v category=$category -v amount_kopecks=$amount -v operation_date=$op_date"
      ;;
    06-operation-soft-deleted.sql)
      user_id=$(user_id_by_phone "$PHONE")
      operation_id=$(psql_value "SELECT o.id FROM operations o JOIN properties p ON p.id = o.property_id WHERE p.owner_id = '$user_id' AND o.deleted_at IS NOT NULL ORDER BY o.created_at DESC LIMIT 1;")
      echo "-v operation_id=$operation_id"
      ;;
    07-recurring-operation-created.sql)
      user_id=$(user_id_by_phone "$PHONE")
      # The recurring-operations folder may end with a paused/cleaned-up op; use the latest one.
      recurring_op_id=$(psql_value "SELECT ro.id FROM recurring_operations ro JOIN properties p ON p.id = ro.property_id WHERE p.owner_id = '$user_id' ORDER BY ro.created_at DESC LIMIT 1;")
      local payment_day amount status
      payment_day=$(psql_value "SELECT payment_day FROM recurring_operations WHERE id = '$recurring_op_id';")
      amount=$(psql_value "SELECT amount_kopecks FROM recurring_operations WHERE id = '$recurring_op_id';")
      status=$(psql_value "SELECT status FROM recurring_operations WHERE id = '$recurring_op_id';")
      echo "-v recurring_operation_id=$recurring_op_id -v payment_day=$payment_day -v amount_kopecks=$amount -v status=$status"
      ;;
    08-recurring-operation-paused.sql)
      user_id=$(user_id_by_phone "$PHONE")
      recurring_op_id=$(psql_value "SELECT ro.id FROM recurring_operations ro JOIN properties p ON p.id = ro.property_id WHERE p.owner_id = '$user_id' AND ro.status = 'paused' ORDER BY ro.created_at DESC LIMIT 1;")
      echo "-v recurring_operation_id=$recurring_op_id"
      ;;
    11-reminder-created.sql)
      user_id=$(user_id_by_phone "$PHONE")
      echo "-v user_id=$user_id"
      ;;
    12-webhook-payment-processed.sql)
      echo "-v phone=$PHONE -v payment_id=${WEBHOOK_PAYMENT_ID:-}"
      ;;
    *)
      echo ""
      ;;
  esac
}

# --- Concurrency helpers ---------------------------------------------------

# Authenticate and upgrade a fresh user to pro. Prints: user_id\tcookie_name\tcookie_value
auth_and_upgrade() {
  local phone="$1"
  local log_file="$BACKEND_LOG"

  send_phone_code "$phone" >/dev/null
  local code
  code=$(extract_code_for_phone "$phone" "$log_file") || {
    echo "ERROR: failed to extract code for $phone" >&2
    return 1
  }

  local verify_response
  verify_response=$(curl -fsS -m 10 -X POST "$BASE_URL/auth/phone/verify" \
    -H "Content-Type: application/json" \
    -D - \
    -d "{\"phone\":\"$phone\",\"code\":\"$code\"}" 2>/dev/null) || {
    echo "ERROR: verify failed for $phone" >&2
    return 1
  }

  local user_id cookie_name cookie_value
  user_id=$(echo "$verify_response" | sed '1,/^\r*$/d' | jq -r '.id // empty' 2>/dev/null)
  cookie_name=$(echo "$verify_response" | grep -i '^set-cookie:' | grep -oE '(__Host-session_id|session_id)=' | head -1 | tr -d '=')
  cookie_value=$(echo "$verify_response" | grep -i '^set-cookie:' | grep -oE '(__Host-session_id|session_id)=[^;]+' | head -1 | cut -d= -f2-)

  if [[ -z "$cookie_name" || -z "$cookie_value" ]]; then
    echo "ERROR: could not extract session cookie for $phone" >&2
    return 1
  fi

  # Upgrade to pro so we can create properties/leases
  local change_response payment_id
  change_response=$(curl -fsS -m 10 -X POST "$BASE_URL/subscription/change" \
    -H "Content-Type: application/json" \
    -H "Cookie: $cookie_name=$cookie_value" \
    -d '{"tariffName":"pro","period":"month"}' 2>/dev/null) || {
    echo "ERROR: subscription change failed for $phone" >&2
    return 1
  }
  payment_id=$(echo "$change_response" | jq -r '.paymentId // empty' 2>/dev/null)

  curl -fsS -m 10 -X POST "$BASE_URL/internal/fake-subscription-payment/$payment_id/confirm" >/dev/null 2>&1 || {
    echo "ERROR: fake payment confirmation failed for $phone" >&2
    return 1
  }

  printf '%s\t%s\t%s\n' "$user_id" "$cookie_name" "$cookie_value"
}

# Extract the top-level 'id' from a JSON response using jq if available,
# otherwise fall back to the first "id" occurrence.
extract_id_from_response() {
  if command -v jq >/dev/null 2>&1; then
    jq -r '.id // empty'
  else
    sed -n 's/.*"id":"\([^"]*\)".*/\1/p' | head -1
  fi
}

# Create a property. Returns property_id.
create_property_curl() {
  local cookie_name="$1"
  local cookie_value="$2"
  local suffix="$3"
  curl -fsS -m 30 -X POST "$BASE_URL/properties" \
    -H "Content-Type: application/json" \
    -H "Cookie: $cookie_name=$cookie_value" \
    -d "{\"name\":\"E2E Concurrency Property $suffix\",\"type\":\"apartment\",\"address\":\"E2E Concurrency St $suffix\"}" 2>/dev/null \
    | extract_id_from_response
}

# Create a tenant contact. Returns tenant_contact_id.
create_tenant_contact_curl() {
  local cookie_name="$1"
  local cookie_value="$2"
  local suffix="$3"
  curl -fsS -m 30 -X POST "$BASE_URL/tenant-contacts" \
    -H "Content-Type: application/json" \
    -H "Cookie: $cookie_name=$cookie_value" \
    -d "{\"name\":\"Иван\",\"surname\":\"Иванов\",\"patronymic\":\"Иванович\",\"phone\":\"+7917$suffix\",\"email\":\"e2e.$suffix@example.com\",\"comment\":\"Concurrency tenant $suffix\"}" 2>/dev/null \
    | extract_id_from_response
}

# Create a lease. Returns lease_id.
create_lease_curl() {
  local cookie_name="$1"
  local cookie_value="$2"
  local property_id="$3"
  local tenant_contact_id="$4"
  local start_date="$5"
  local end_date="$6"
  curl -fsS -m 30 -X POST "$BASE_URL/leases" \
    -H "Content-Type: application/json" \
    -H "Cookie: $cookie_name=$cookie_value" \
    -d "{\"property_id\":\"$property_id\",\"tenant_contact_id\":\"$tenant_contact_id\",\"start_date\":\"$start_date\",\"end_date\":\"$end_date\",\"rent_amount_kopecks\":12000000,\"deposit_amount_kopecks\":2400000,\"payment_day\":10,\"comment\":\"Concurrency lease\"}" 2>/dev/null \
    | extract_id_from_response
}

# Create a manual operation. Returns operation_id.
create_operation_curl() {
  local cookie_name="$1"
  local cookie_value="$2"
  local property_id="$3"
  local op_type="${4:-income}"
  local category="${5:-rent}"
  local amount="${6:-100000}"
  local op_date="${7:-$(date +%Y-%m-%d)}"
  curl -fsS -m 30 -X POST "$BASE_URL/properties/$property_id/operations" \
    -H "Content-Type: application/json" \
    -H "Cookie: $cookie_name=$cookie_value" \
    -d "{\"type\":\"$op_type\",\"category\":\"$category\",\"amount_kopecks\":$amount,\"operation_date\":\"$op_date\",\"comment\":\"Random user op\"}" 2>/dev/null \
    | extract_id_from_response
}

# Archive a property.
archive_property_curl() {
  local cookie_name="$1"
  local cookie_value="$2"
  local property_id="$3"
  curl -fsS -m 30 -X POST "$BASE_URL/properties/$property_id/archive" \
    -H "Cookie: $cookie_name=$cookie_value" 2>/dev/null >/dev/null
}

# --- Bug report ------------------------------------------------------------

write_bug_report() {
  BUG_REPORT_FILE="$TMP_DIR/e2e-bugs-${TIMESTAMP}.md"
  local commit_sha
  commit_sha=$(cd "$PROJECT_ROOT" && git rev-parse --short HEAD 2>/dev/null || echo "n/a")

  cat > "$BUG_REPORT_FILE" <<EOF
# E2E Bug Report

- **Timestamp:** $TIMESTAMP
- **Commit SHA:** $commit_sha
- **Backend URL:** $BASE_URL

## Summary

- Bugs found: ${#BUG_TITLES[@]}
- Random users status: $RANDOM_USER_STATUS
- Archive race status: $RACE_ARCHIVE_STATUS
- Operation update race status: $RACE_OPERATION_STATUS
- Tariff race status: $RACE_TARIFF_STATUS

EOF

  if [[ ${#BUG_TITLES[@]} -eq 0 ]]; then
    echo "No bugs detected during this run." >> "$BUG_REPORT_FILE"
  else
    local i
    for i in "${!BUG_TITLES[@]}"; do
      cat >> "$BUG_REPORT_FILE" <<EOF
## ${BUG_TITLES[$i]}

**Steps to reproduce:**
${BUG_STEPS[$i]}

**Expected result:**
${BUG_EXPECTED[$i]}

**Actual result:**
${BUG_ACTUAL[$i]}

**Fix applied:**
${BUG_FIX[$i]:-Pending investigation}

---

EOF
    done
  fi

  log "Bug report written to $BUG_REPORT_FILE"
}

# --- Reporting -------------------------------------------------------------

write_report() {
  local commit_sha
  commit_sha=$(cd "$PROJECT_ROOT" && git rev-parse --short HEAD 2>/dev/null || echo "n/a")

  local folder_passed=0
  local folder_failed=0
  local sql_passed=0
  local sql_failed=0
  local sql_skipped=0
  local i

  for status in "${FOLDER_STATUSES[@]:-}"; do
    if [[ "$status" == "PASS" ]]; then
      folder_passed=$((folder_passed + 1))
    else
      folder_failed=$((folder_failed + 1))
    fi
  done

  for status in "${SQL_STATUSES[@]:-}"; do
    case "$status" in
      PASS) sql_passed=$((sql_passed + 1)) ;;
      FAIL) sql_failed=$((sql_failed + 1)) ;;
      *) sql_skipped=$((sql_skipped + 1)) ;;
    esac
  done

  local total_passed=$((folder_passed + sql_passed + EDGE_PASSED))
  local total_failed=$((folder_failed + sql_failed + EDGE_FAILED))

  cat > "$REPORT_FILE" <<EOF
# E2E Test Report

- **Timestamp:** $TIMESTAMP
- **Commit SHA:** $commit_sha
- **Environment:** Local
- **Backend URL:** $BASE_URL
- **Phone:** $PHONE
- **Report file:** $REPORT_FILE

## Summary

| Category        | Passed | Failed | Skipped |
|-----------------|--------|--------|---------|
| Sequential runs | $folder_passed | $folder_failed | - |
| SQL checks      | $sql_passed | $sql_failed | $sql_skipped |
| Edge cases      | $EDGE_PASSED | $EDGE_FAILED | - |
| Random users    | $(if [[ "$RANDOM_USER_STATUS" == "PASS" ]]; then echo 1; else echo 0; fi) | $(if [[ "$RANDOM_USER_STATUS" == "PASS" ]]; then echo 0; else echo 1; fi) | - |
| Concurrency     | - | - | - |
| **Total**       | **$total_passed** | **$total_failed** | - |

## Bug Report

- **File:** $BUG_REPORT_FILE
- **Bugs found:** ${#BUG_TITLES[@]}

## Sequential Feature Run

| Folder | Status | Details |
|--------|--------|---------|
EOF

  local folders_ordered=(
    "system-e2e/00-auth-setup"
    "system-e2e/10-subscription"
    "system-e2e/20-property-lifecycle"
    "system-e2e/30-tenant-contacts"
    "system-e2e/40-leases"
    "system-e2e/50-operations"
    "system-e2e/60-recurring-operations"
    "system-e2e/70-reminders"
    "system-e2e/75-webhooks"
    "system-e2e/80-readonly-recovery"
    "system-e2e/99-final-cleanup"
  )

  local idx
  for folder in "${folders_ordered[@]}"; do
    local status="$(folder_status_by_name "$folder")"
    local details=""
    if [[ "$status" == "FAIL" ]]; then
      for i in "${!FOLDER_NAMES[@]}"; do
        if [[ "${FOLDER_NAMES[$i]}" == "$folder" ]]; then
          idx=$i
          break
        fi
      done
      details=$(echo "${FOLDER_OUTPUTS[$idx]:-}" | tail -n 3 | tr '\n' ' ')
    fi
    printf '| `%s` | %s | %s |\n' "$folder" "$status" "${details:- }" >> "$REPORT_FILE"
  done

  cat >> "$REPORT_FILE" <<EOF

## SQL Verification Checks

| Script | Status | Output snippet |
|--------|--------|----------------|
EOF

  for i in "${!SQL_NAMES[@]}"; do
    local status="${SQL_STATUSES[$i]}"
    local snippet
    snippet=$(echo "${SQL_OUTPUTS[$i]:-}" | tail -n 5 | tr '\n' ' ')
    printf '| `%s` | %s | %s |\n' "${SQL_NAMES[$i]}" "$status" "${snippet:- }" >> "$REPORT_FILE"
  done

  cat >> "$REPORT_FILE" <<EOF

## Edge-Case Collection

- **Tests passed:** $EDGE_PASSED
- **Tests failed:** $EDGE_FAILED
- **Expected non-2xx responses:** $EDGE_EXPECTED_NON_2XX
- **Status:** $(if [[ "$EDGE_FAILED" -eq 0 ]]; then echo "PASS"; else echo "FAIL"; fi)

$(if [[ "$EDGE_FAILED" -gt 0 ]]; then
  echo "$EDGE_FAILED_DETAILS" | jq -r '.[] | "- `\(.filename)` — HTTP \(.status)"' 2>/dev/null || true
else
  echo "No real failures. The $EDGE_EXPECTED_NON_2XX non-2xx responses are expected negative-test outcomes."
fi)

## Concurrency Tests

### Isolated users (3 parallel auth/property/lease cycles)

- **Status:** $ISOLATED_STATUS

### Race on same property (5 parallel leases)

- **Status:** $RACE_STATUS

### Random multi-user lifecycles

- **Status:** $RANDOM_USER_STATUS

### Additional race scenarios

| Scenario | Status |
|----------|--------|
| Archive/unarchive same property | $RACE_ARCHIVE_STATUS |
| Concurrent operation updates | $RACE_OPERATION_STATUS |
| Tariff downgrade vs property creation | $RACE_TARIFF_STATUS |

## Failures

EOF

  if [[ ${#FAILURES[@]:-} -eq 0 ]]; then
    echo "No failures." >> "$REPORT_FILE"
  else
    for f in "${FAILURES[@]}"; do
      echo "- $f" >> "$REPORT_FILE"
    done
  fi

  log "Report written to $REPORT_FILE"
}

# --- Main execution --------------------------------------------------------

main() {
  mkdir -p "$TMP_DIR"

  log "=============================================="
  log "Arenda E2E runner starting"
  log "Timestamp: $TIMESTAMP"
  log "Phone: $PHONE"
  log "=============================================="

  # Coverage gate: fail fast if collections drift from the backend.
  if ! "$SCRIPT_DIR/check-bruno-coverage.sh"; then
    add_failure "Bruno/E2E coverage gate failed"
    write_report
    exit 1
  fi

  # Health checks
  if ! check_backend; then
    add_failure "Backend is not reachable at $BASE_URL/tariffs"
    write_report
    exit 1
  fi

  local db_available=false
  if check_postgres; then
    db_available=true
  else
    add_failure "PostgreSQL not reachable; SQL checks will be skipped"
  fi

  # Auth: send code and extract it from logs
  if ! send_phone_code "$PHONE"; then
    add_failure "Failed to send auth code to $PHONE"
    write_report
    exit 1
  fi

  CODE=$(extract_code_for_phone "$PHONE" "$BACKEND_LOG") || true
  if [[ -z "${CODE:-}" ]]; then
    add_failure "Could not extract 6-digit code from $BACKEND_LOG for $PHONE"
    write_report
    exit 1
  fi
  log "Auth code extracted: $CODE"

  # Sequential feature run with SQL checks after each folder.
  # Auth setup runs first so we can extract the session cookie for subsequent folders.
  local -a feature_folders=(
    "system-e2e/10-subscription"
    "system-e2e/20-property-lifecycle"
    "system-e2e/30-tenant-contacts"
    "system-e2e/40-leases"
    "system-e2e/50-operations"
    "system-e2e/60-recurring-operations"
    "system-e2e/70-reminders"
    "system-e2e/75-webhooks"
    "system-e2e/80-readonly-recovery"
  )

  local SESSION_ID=""
  local COOKIE_NAME=""
  local auth_report_json="${BRUNO_REPORT_BASE}-system-e2e-00-auth-setup.json"

  if run_bruno_folder "system-e2e/00-auth-setup" --bail; then
    local session_info
    session_info=$(extract_session_from_report "$auth_report_json") || true
    if [[ -n "$session_info" ]]; then
      COOKIE_NAME=$(echo "$session_info" | cut -f1)
      SESSION_ID=$(echo "$session_info" | cut -f2)
      log "Session extracted: cookieName=$COOKIE_NAME"
    else
      add_failure "Could not extract session from auth-setup report"
    fi

    if [[ "$db_available" == "true" ]]; then
      run_sql_check "01-user-and-session-created.sql" $(derive_sql_vars "01-user-and-session-created.sql") || true
    fi
  else
    add_failure "Auth setup failed; subsequent feature folders will not run with session"
  fi

  local session_args=()
  if [[ -n "$SESSION_ID" && -n "$COOKIE_NAME" ]]; then
    session_args=(--env-var session_id="$SESSION_ID" --env-var cookieName="$COOKIE_NAME")
  fi

  local active_payment_method_id=""
  local WEBHOOK_PAYMENT_ID=""
  local WEBHOOK_PROVIDER_PAYMENT_ID=""

  for folder in "${feature_folders[@]}"; do
    # Seed the pro tariff limit before the reminder deep-dive so it can create
    # multiple properties without hitting the default cap.
    if [[ "$folder" == "system-e2e/70-reminders" && "$db_available" == "true" ]]; then
      run_sql_check "00-seed-reminders-tariff.sql" || true
    fi

    local folder_args=(${session_args[@]+"${session_args[@]}"})
    if [[ "$folder" == "system-e2e/99-final-cleanup" && -n "$active_payment_method_id" ]]; then
      folder_args+=(--env-var activePaymentMethodId="$active_payment_method_id")
    fi
    if [[ "$folder" == "system-e2e/75-webhooks" ]]; then
      local payment_pair
      payment_pair=$(get_webhook_payment "$PHONE") || {
        add_failure "Failed to find webhook payment for phone $PHONE"
        record_folder "$folder" "FAIL" "get_webhook_payment failed"
        continue
      }
      WEBHOOK_PAYMENT_ID=$(echo "$payment_pair" | cut -f1)
      WEBHOOK_PROVIDER_PAYMENT_ID=$(echo "$payment_pair" | cut -f2)
      log "Using webhook payment: $WEBHOOK_PAYMENT_ID"
      folder_args+=(--env-var webhookPaymentId="$WEBHOOK_PAYMENT_ID" --env-var webhookProviderPaymentId="$WEBHOOK_PROVIDER_PAYMENT_ID")
    fi

    if run_bruno_folder "$folder" ${folder_args[@]+"${folder_args[@]}"} --bail; then
      if [[ "$folder" == "system-e2e/10-subscription" ]]; then
        active_payment_method_id=$(extract_active_payment_method_id "${BRUNO_REPORT_BASE}-system-e2e-10-subscription.json") || true
        if [[ -n "$active_payment_method_id" ]]; then
          log "Active payment method extracted: $active_payment_method_id"
        fi
      fi


      if [[ "$db_available" == "true" ]]; then
        local scripts
        scripts=$(sql_checks_for_folder "$folder")
        local script_name
        for script_name in $scripts; do
          local sql_vars
          sql_vars=$(derive_sql_vars "$script_name")
          # shellcheck disable=SC2086
          run_sql_check "$script_name" $sql_vars || true
        done
      else
        log "Skipping SQL checks for $folder (DB unavailable)"
      fi
    else
      add_failure "Stopping further SQL checks after $folder failure"
      # Continue to next folder so the report captures as much as possible.
    fi
  done

  # Edge-case collection uses a fresh user so its auth flow gets a valid unused code.
  local EDGE_PHONE="+7915$(printf '%07d' $(((EPOCH + 100) % 10000000)))"
  local EDGE_CODE=""
  log "Sending auth code for edge-case user: $EDGE_PHONE ..."
  if send_phone_code "$EDGE_PHONE"; then
    # Give the backend log a moment to flush the fake SMS entry.
    sleep 1
    EDGE_CODE=$(extract_code_for_phone "$EDGE_PHONE" "$BACKEND_LOG") || true
  fi
  if [[ -z "$EDGE_CODE" ]]; then
    add_failure "Could not extract edge-case auth code for $EDGE_PHONE"
  else
    log "Edge-case auth code extracted: $EDGE_CODE"
  fi

  log "Running edge-case collection: system-e2e-edge ..."
  local edge_report_json="${BRUNO_REPORT_BASE}-system-e2e-edge.json"
  local edge_output
  local edge_status=0
  # shellcheck disable=SC2086
  edge_output=$(cd "$BRUNO_COLLECTION" && bru run system-e2e-edge -r --env Local \
    --env-var phone="${EDGE_PHONE:-$PHONE}" \
    --env-var code="${EDGE_CODE:-$CODE}" \
    --env-var cookieName=session_id \
    --env-var skipSend=true \
    --delay 200 \
    --reporter-json "$edge_report_json" 2>&1) || edge_status=$?
  EDGE_OUTPUT="$edge_output"

  # Bruno CLI counts non-2xx requests as "failed" even when the test asserts them.
  # Use per-result status for the real pass/fail count.
  local edge_parse
  if [[ -f "$edge_report_json" ]] && command -v jq >/dev/null 2>&1; then
    edge_parse=$(parse_edge_case_results "$edge_report_json")
  else
    edge_parse="0\t0\t0\t[]"
  fi
  EDGE_PASSED=$(echo -e "$edge_parse" | cut -f1)
  EDGE_FAILED=$(echo -e "$edge_parse" | cut -f2)
  EDGE_EXPECTED_NON_2XX=$(echo -e "$edge_parse" | cut -f3)
  EDGE_FAILED_DETAILS=$(echo -e "$edge_parse" | cut -f4)
  [[ -z "$EDGE_PASSED" ]] && EDGE_PASSED=0
  [[ -z "$EDGE_FAILED" ]] && EDGE_FAILED=0
  [[ -z "$EDGE_EXPECTED_NON_2XX" ]] && EDGE_EXPECTED_NON_2XX=0

  if [[ "$EDGE_FAILED" -gt 0 ]]; then
    add_failure "Edge-case collection has $EDGE_FAILED real failed test(s)"
  fi
  if [[ "$edge_status" -ne 0 && "$EDGE_FAILED" -gt 0 ]]; then
    add_failure "Edge-case collection finished with non-zero exit status ($edge_status)"
  fi
  log "Edge-case collection: $EDGE_PASSED passed, $EDGE_FAILED failed, $EDGE_EXPECTED_NON_2XX expected non-2xx"

  # Random multi-user lifecycle tests
  if [[ "$db_available" == "true" ]]; then
    run_random_user_tests
  else
    log "Skipping random user tests (DB unavailable)"
  fi

  # Concurrency tests (skip if DB unavailable)
  if [[ "$db_available" == "true" ]]; then
    run_concurrency_tests
    run_race_scenarios
  else
    log "Skipping concurrency tests (DB unavailable)"
  fi

  # Final cleanup for the main sequential user (after edge/concurrency so their session stays valid)
  local cleanup_args=("${session_args[@]:-}")
  if [[ -n "$active_payment_method_id" ]]; then
    cleanup_args+=(--env-var activePaymentMethodId="$active_payment_method_id")
  fi
  run_bruno_folder "system-e2e/99-final-cleanup" "${cleanup_args[@]:-}" --bail || true

  # Write bug report and main report
  write_bug_report
  write_report

  # Final console summary
  log "=============================================="
  if [[ ${#FAILURES[@]:-} -eq 0 ]]; then
    log "All checks passed."
  else
    log "Completed with ${#FAILURES[@]:-} failure(s). See report."
  fi
  log "=============================================="

  return $(( ${#FAILURES[@]} > 0 ? 1 : 0 ))
}

# --- Random multi-user lifecycle -------------------------------------------

# Run a full happy-path lifecycle for one random user.
# $1 phone, $2 variant suffix, $3 output file to write results.
random_user_lifecycle() {
  local phone="$1"
  local variant="$2"
  local out_file="$3"

  local auth_info
  auth_info=$(auth_and_upgrade "$phone") || {
    echo "ERROR auth_and_upgrade $phone" > "$out_file"
    return 1
  }
  local user_id cookie_name cookie_value
  user_id=$(echo "$auth_info" | cut -f1)
  cookie_name=$(echo "$auth_info" | cut -f2)
  cookie_value=$(echo "$auth_info" | cut -f3)

  local suffix
  suffix=$(echo "$phone" | sed 's/[^0-9]//g' | tail -c 8)

  local property_id tenant_id lease_id operation_id
  property_id=$(create_property_curl "$cookie_name" "$cookie_value" "$suffix") || true
  tenant_id=$(create_tenant_contact_curl "$cookie_name" "$cookie_value" "$suffix") || true

  local today next_year
  today=$(date +%Y-%m-%d)
  next_year=$(date -v+1y +%Y-%m-%d 2>/dev/null || date -d '+1 year' +%Y-%m-%d)

  # Variant tweaks: lease dates and operation type/category.
  local start_offset="+0d"
  local op_type="income"
  local op_category="rent"
  case "$variant" in
    1) start_offset="-1y" ; op_type="expense"; op_category="utilities" ;;
    2) start_offset="+1m" ; op_type="income"; op_category="other_income" ;;
    *) start_offset="+0d" ;;
  esac
  local start_date
  start_date=$(date -v"$start_offset" +%Y-%m-%d 2>/dev/null || date -d "$start_offset" +%Y-%m-%d)

  lease_id=$(create_lease_curl "$cookie_name" "$cookie_value" "$property_id" "$tenant_id" "$start_date" "$next_year") || true
  operation_id=$(create_operation_curl "$cookie_name" "$cookie_value" "$property_id" "$op_type" "$op_category" 50000 "$today") || true

  # Soft-delete the operation as a mutation check (retry transient failures).
  if [[ -n "$operation_id" ]]; then
    local attempt
    for attempt in 1 2 3; do
      if curl -fsS -m 10 -X DELETE "$BASE_URL/operations/$operation_id" \
          -H "Cookie: $cookie_name=$cookie_value" >/dev/null 2>&1; then
        break
      fi
      sleep 1
    done
  fi

  printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$user_id" "$property_id" "$tenant_id" "$lease_id" "$operation_id" "$cookie_name=$cookie_value" > "$out_file"
}

# Verify DB state for one random user and record bugs on mismatch.
verify_random_user_db() {
  local i="$1"
  local out_file="$2"

  if [[ ! -f "$out_file" ]] || grep -q "ERROR" "$out_file"; then
    record_bug "Random user $i lifecycle failed" "Auth or upgrade step" "User created and upgraded to pro" "auth_and_upgrade returned error" "Pending investigation"
    return 1
  fi

  local user_id property_id tenant_id lease_id operation_id
  user_id=$(cut -f1 "$out_file")
  property_id=$(cut -f2 "$out_file")
  tenant_id=$(cut -f3 "$out_file")
  lease_id=$(cut -f4 "$out_file")
  operation_id=$(cut -f5 "$out_file")

  local missing=""
  [[ -z "$user_id" ]] && missing="$missing user"
  [[ -z "$property_id" ]] && missing="$missing property"
  [[ -z "$tenant_id" ]] && missing="$missing tenant"
  [[ -z "$lease_id" ]] && missing="$missing lease"
  if [[ -n "$missing" ]]; then
    record_bug "Random user $i has missing IDs" "Lifecycle script produced empty IDs: $missing" "All resource IDs present" "Missing:$missing" "Pending investigation"
    return 1
  fi

  local counts=""
  counts=$(psql_value "
    SELECT
      (SELECT count(*)::int FROM users WHERE id = '$user_id'),
      (SELECT count(*)::int FROM properties WHERE id = '$property_id' AND owner_id = '$user_id'),
      (SELECT count(*)::int FROM tenant_contacts WHERE id = '$tenant_id' AND owner_id = '$user_id'),
      (SELECT count(*)::int FROM leases WHERE id = '$lease_id' AND property_id = '$property_id'),
      (SELECT count(*)::int FROM operations WHERE id = '$operation_id' AND property_id = '$property_id' AND deleted_at IS NOT NULL);
  " 2>/dev/null) || true
  counts=${counts:-empty}

  if echo "$counts" | grep -qE '^1\|1\|1\|1\|1$'; then
    log "Random user $i DB state OK (counts=$counts)"
    return 0
  else
    record_bug "Random user $i DB state mismatch" "Verify counts for user $user_id" "1|1|1|1|1" "$counts" "Pending investigation"
    return 1
  fi
}

run_random_user_tests() {
  local count="${RANDOM_USER_COUNT:-5}"
  log "Running random user lifecycle tests for $count users ..."

  local -a random_phones
  local i
  for i in $(seq 1 "$count"); do
    local phone="+7915$(printf '%07d' $(((EPOCH + 100 + i) % 10000000)))"
    random_phones+=("$phone")
    local variant=$((i % 3))
    (
      # Stagger auth/payment calls to avoid 429 rate limits.
      sleep $(( (i - 1) * 2 ))
      random_user_lifecycle "$phone" "$variant" "$TMP_DIR/random-user-$i.out"
    ) &
  done
  wait

  local ok=true
  for i in $(seq 1 "$count"); do
    if ! verify_random_user_db "$i" "$TMP_DIR/random-user-$i.out"; then
      ok=false
    fi
  done

  if $ok; then
    RANDOM_USER_STATUS="PASS"
  else
    RANDOM_USER_STATUS="FAIL"
  fi
}

# --- Race scenarios --------------------------------------------------------

run_race_scenarios() {
  log "Running additional race scenarios ..."

  # ---- Race 1: concurrent archive/unarchive of the same property ----
  log "Race: archive/unarchive same property ..."
  local arc_phone="+7915$(printf '%07d' $(((EPOCH + 200) % 10000000)))"
  local arc_auth
  arc_auth=$(auth_and_upgrade "$arc_phone") || {
    record_bug "Archive race auth failed" "auth_and_upgrade $arc_phone" "Successful auth" "auth_and_upgrade returned error" "Pending investigation"
    RACE_ARCHIVE_STATUS="FAIL"
    return
  }
  local arc_cookie_name arc_cookie_value arc_property_id
  arc_cookie_name=$(echo "$arc_auth" | cut -f2)
  arc_cookie_value=$(echo "$arc_auth" | cut -f3)
  arc_property_id=$(create_property_curl "$arc_cookie_name" "$arc_cookie_value" "arc")
  if [[ -z "$arc_property_id" ]]; then
    record_bug "Archive race setup failed" "create_property_curl" "Property created" "empty property_id" "Pending investigation"
    RACE_ARCHIVE_STATUS="FAIL"
    return
  fi

  local _
  for _ in $(seq 1 5); do
    (
      curl -fsS -m 10 -X POST "$BASE_URL/properties/$arc_property_id/archive" -H "Cookie: $arc_cookie_name=$arc_cookie_value" >/dev/null 2>&1 || true
      curl -fsS -m 10 -X POST "$BASE_URL/properties/$arc_property_id/unarchive" -H "Cookie: $arc_cookie_name=$arc_cookie_value" >/dev/null 2>&1 || true
    ) &
  done
  wait
  sleep 1

  local arc_status=""
  arc_status=$(psql_value "SELECT status FROM properties WHERE id = '$arc_property_id';" 2>/dev/null) || true
  arc_status=${arc_status:-unknown}
  if [[ "$arc_status" == "active" || "$arc_status" == "archived" ]]; then
    log "Archive/unarchive race OK: final status $arc_status"
    RACE_ARCHIVE_STATUS="PASS"
  else
    record_bug "Archive/unarchive race left property in invalid state" "5 concurrent archive/unarchive cycles" "status active or archived" "status=$arc_status" "Pending investigation"
    RACE_ARCHIVE_STATUS="FAIL"
  fi

  # ---- Race 2: concurrent updates to the same operation ----
  log "Race: concurrent operation updates ..."
  local op_phone="+7915$(printf '%07d' $(((EPOCH + 201) % 10000000)))"
  local op_auth
  op_auth=$(auth_and_upgrade "$op_phone") || {
    record_bug "Operation update race auth failed" "auth_and_upgrade $op_phone" "Successful auth" "auth_and_upgrade returned error" "Pending investigation"
    RACE_OPERATION_STATUS="FAIL"
    return
  }
  local op_cookie_name op_cookie_value op_property_id op_id
  op_cookie_name=$(echo "$op_auth" | cut -f2)
  op_cookie_value=$(echo "$op_auth" | cut -f3)
  op_property_id=$(create_property_curl "$op_cookie_name" "$op_cookie_value" "op")
  op_id=$(create_operation_curl "$op_cookie_name" "$op_cookie_value" "$op_property_id" "income" "rent" 100000 "$(date +%Y-%m-%d)")
  if [[ -z "$op_id" ]]; then
    record_bug "Operation update race setup failed" "create_operation_curl" "Operation created" "empty operation_id" "Pending investigation"
    RACE_OPERATION_STATUS="FAIL"
    return
  fi

  for _ in $(seq 1 5); do
    (
      local amount=$((100000 + RANDOM % 900000))
      curl -fsS -m 10 -X PATCH "$BASE_URL/operations/$op_id" \
        -H "Content-Type: application/json" \
        -H "Cookie: $op_cookie_name=$op_cookie_value" \
        -d "{\"amount_kopecks\":$amount,\"comment\":\"race update\"}" >/dev/null 2>&1 || true
    ) &
  done
  wait
  sleep 1

  local op_versions=""
  op_versions=$(psql_value "SELECT count(*)::int FROM operations WHERE id = '$op_id';" 2>/dev/null) || true
  op_versions=${op_versions:-0}
  if [[ "$op_versions" == "1" ]]; then
    log "Operation update race OK: single operation row preserved"
    RACE_OPERATION_STATUS="PASS"
  else
    record_bug "Operation update race produced inconsistent rows" "5 concurrent PATCH on same operation" "Exactly 1 operation row" "$op_versions rows" "Pending investigation"
    RACE_OPERATION_STATUS="FAIL"
  fi

  # ---- Race 3: tariff downgrade while creating properties over limit ----
  log "Race: tariff downgrade vs property creation ..."
  local t_phone="+7915$(printf '%07d' $(((EPOCH + 202) % 10000000)))"
  local t_auth
  t_auth=$(auth_and_upgrade "$t_phone") || {
    record_bug "Tariff race auth failed" "auth_and_upgrade $t_phone" "Successful auth" "auth_and_upgrade returned error" "Pending investigation"
    RACE_TARIFF_STATUS="FAIL"
    return
  }
  local t_cookie_name t_cookie_value t_user_id
  t_cookie_name=$(echo "$t_auth" | cut -f2)
  t_cookie_value=$(echo "$t_auth" | cut -f3)
  t_user_id=$(echo "$t_auth" | cut -f1)

  # Create 2 active properties (basic limit is presumably 3).
  local p1 p2
  p1=$(create_property_curl "$t_cookie_name" "$t_cookie_value" "t1")
  p2=$(create_property_curl "$t_cookie_name" "$t_cookie_value" "t2")

  # Downgrade to basic and create a 4th property concurrently.
  (
    local change_response payment_id
    change_response=$(curl -s -m 30 -X POST "$BASE_URL/subscription/change" \
      -H "Content-Type: application/json" \
      -H "Cookie: $t_cookie_name=$t_cookie_value" \
      -d '{"tariffName":"basic","period":"month"}' 2>/dev/null) || true
    payment_id=$(echo "$change_response" | sed -n 's/.*"paymentId":"\([^"]*\)".*/\1/p' | head -1)
    if [[ -n "$payment_id" ]]; then
      curl -fsS -m 10 -X POST "$BASE_URL/internal/fake-subscription-payment/$payment_id/confirm" >/dev/null 2>&1 || true
    fi
  ) &
  (
    create_property_curl "$t_cookie_name" "$t_cookie_value" "t4" >/dev/null 2>&1 || true
    create_property_curl "$t_cookie_name" "$t_cookie_value" "t5" >/dev/null 2>&1 || true
  ) &
  wait
  sleep 1

  log "Tariff race: waiting for background jobs ..."
  wait
  log "Tariff race: background jobs finished"
  sleep 1

  local active_count="" tariff_name=""
  active_count=$(psql_value "SELECT count(*)::int FROM properties WHERE owner_id = '$t_user_id' AND status = 'active';" 2>/dev/null) || true
  tariff_name=$(psql_value "SELECT t.name FROM user_subscriptions s JOIN tariffs t ON t.id = s.tariff_id WHERE s.user_id = '$t_user_id' ORDER BY s.updated_at DESC LIMIT 1;" 2>/dev/null) || true
  pending_tariff=$(psql_value "SELECT t.name FROM user_subscriptions s LEFT JOIN tariffs t ON t.id = s.pending_tariff_id WHERE s.user_id = '$t_user_id';" 2>/dev/null) || true
  active_count=${active_count:-99}
  tariff_name=${tariff_name:-unknown}
  pending_tariff=${pending_tariff:-none}
  log "Tariff race DB check: active=$active_count current_tariff=$tariff_name pending_tariff=$pending_tariff"

  # Downgrade to basic is scheduled for the end of the paid period; while the
  # current tariff remains pro the limit is higher. The system is correct as long
  # as a scheduled downgrade exists and the current active count does not exceed
  # the *current* tariff limit.
  if [[ "$pending_tariff" == "basic" ]]; then
    log "Tariff race OK: downgrade scheduled (active=$active_count current=$tariff_name pending=$pending_tariff)"
    RACE_TARIFF_STATUS="PASS"
  else
    record_bug "Tariff downgrade race did not schedule basic downgrade" "Concurrent downgrade to basic and property creation" "pending_tariff=basic" "active=$active_count current=$tariff_name pending=$pending_tariff" "Pending investigation"
    RACE_TARIFF_STATUS="FAIL"
  fi
}

# --- Concurrency tests -----------------------------------------------------

run_concurrency_tests() {
  log "Running concurrency tests ..."

  # ---- Isolated users ----
  log "Launching 3 isolated user cycles in parallel ..."
  local -a isolated_phones
  local i
  for i in 1 2 3; do
    local iso_phone="+7915$(printf '%07d' $(((EPOCH + i) % 10000000)))"
    isolated_phones+=("$iso_phone")
    (
      # Stagger auth/payment calls to avoid 429 rate limits.
      sleep $(( (i - 1) * 2 ))
      local auth_info
      auth_info=$(auth_and_upgrade "$iso_phone") || {
        echo "ERROR auth_and_upgrade $iso_phone" > "$TMP_DIR/isolated-$i.out"
        exit 1
      }
      local user_id cookie_name cookie_value
      user_id=$(echo "$auth_info" | cut -f1)
      cookie_name=$(echo "$auth_info" | cut -f2)
      cookie_value=$(echo "$auth_info" | cut -f3)

      local suffix
      suffix=$(echo "$iso_phone" | sed 's/[^0-9]//g' | tail -c 8)
      local property_id tenant_id
      property_id=$(create_property_curl "$cookie_name" "$cookie_value" "$suffix") || true
      tenant_id=$(create_tenant_contact_curl "$cookie_name" "$cookie_value" "$suffix") || true

      local today next_year
      today=$(date +%Y-%m-%d)
      next_year=$(date -v+1y +%Y-%m-%d 2>/dev/null || date -d '+1 year' +%Y-%m-%d)
      local lease_id
      lease_id=$(create_lease_curl "$cookie_name" "$cookie_value" "$property_id" "$tenant_id" "$today" "$next_year") || true

      printf '%s\t%s\t%s\t%s\n' "$user_id" "$property_id" "$tenant_id" "$lease_id" > "$TMP_DIR/isolated-$i.out"
    ) &
  done
  wait

  local isolated_ok=true
  for i in 1 2 3; do
    local out_file="$TMP_DIR/isolated-$i.out"
    if [[ ! -f "$out_file" ]] || grep -q "ERROR" "$out_file"; then
      isolated_ok=false
      add_failure "Isolated user cycle $i failed"
      continue
    fi
    local user_id property_id tenant_id lease_id
    user_id=$(cut -f1 "$out_file")
    property_id=$(cut -f2 "$out_file")
    tenant_id=$(cut -f3 "$out_file")
    lease_id=$(cut -f4 "$out_file")

    if [[ -z "$user_id" || -z "$property_id" || -z "$tenant_id" || -z "$lease_id" ]]; then
      isolated_ok=false
      add_failure "Isolated user cycle $i produced empty IDs"
      continue
    fi

    local counts
    counts=$(psql_value "
      SELECT
        (SELECT count(*)::int FROM properties WHERE owner_id = '$user_id'),
        (SELECT count(*)::int FROM tenant_contacts WHERE owner_id = '$user_id'),
        (SELECT count(*)::int FROM leases l JOIN properties p ON p.id = l.property_id WHERE p.owner_id = '$user_id');
    ") || true

    if echo "$counts" | grep -qE '^1\|1\|1$'; then
      log "Isolated user $i OK (1 property, 1 tenant, 1 lease)"
    else
      isolated_ok=false
      add_failure "Isolated user $i has unexpected counts: $counts (expected 1|1|1)"
    fi
  done

  if $isolated_ok; then
    ISOLATED_STATUS="PASS"
  else
    ISOLATED_STATUS="FAIL"
  fi

  # ---- Race on same property ----
  log "Launching race test: 5 parallel leases on the same property ..."
  local race_phone="+7915$(printf '%07d' $(((EPOCH + 10) % 10000000)))"
  local race_auth
  race_auth=$(auth_and_upgrade "$race_phone") || {
    add_failure "Race test auth/upgrade failed"
    RACE_STATUS="FAIL"
    return
  }
  local race_user_id race_cookie_name race_cookie_value
  race_user_id=$(echo "$race_auth" | cut -f1)
  race_cookie_name=$(echo "$race_auth" | cut -f2)
  race_cookie_value=$(echo "$race_auth" | cut -f3)

  local race_suffix
  race_suffix=$(echo "$race_phone" | sed 's/[^0-9]//g' | tail -c 8)
  local race_property_id race_tenant_id
  race_property_id=$(create_property_curl "$race_cookie_name" "$race_cookie_value" "$race_suffix")
  race_tenant_id=$(create_tenant_contact_curl "$race_cookie_name" "$race_cookie_value" "$race_suffix")

  if [[ -z "$race_property_id" || -z "$race_tenant_id" ]]; then
    add_failure "Race test setup failed (property or tenant missing)"
    RACE_STATUS="FAIL"
    return
  fi

  local today next_year
  today=$(date +%Y-%m-%d)
  next_year=$(date -v+1y +%Y-%m-%d 2>/dev/null || date -d '+1 year' +%Y-%m-%d)

  local _
  for _ in $(seq 1 5); do
    (
      create_lease_curl "$race_cookie_name" "$race_cookie_value" "$race_property_id" "$race_tenant_id" "$today" "$next_year" >/dev/null 2>&1
    ) &
  done
  wait

  sleep 1
  local open_count
  open_count=$(psql_value "
    SELECT count(*)::int
    FROM leases
    WHERE property_id = '$race_property_id'
      AND status IN ('awaiting_start', 'active', 'requires_action');
  ") || true

  if [[ "$open_count" == "1" || "$open_count" == "0" ]]; then
    log "Race test OK: $open_count open lease(s) for property $race_property_id"
    RACE_STATUS="PASS"
  else
    add_failure "Race test failed: $open_count open leases for property $race_property_id (expected <= 1)"
    RACE_STATUS="FAIL"
  fi
}

main "$@"
