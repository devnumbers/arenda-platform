#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BRUNO_DIR="${REPO_ROOT}/tools/bruno/arenda-api"
LOG_FILE="${REPO_ROOT}/backend-test.log"
REPORT_FILE="${REPO_ROOT}/docs/reviews/2026-06-17-reminders-api-test-report.md"
RESULTS_FILE="${REPO_ROOT}/tools/bruno/reminders-e2e-results.json"

set -a
# shellcheck source=/dev/null
. "${REPO_ROOT}/.env"
set +a

BASE_URL="${APP_BASE_URL:-http://localhost:8080}"
PHONE="+79150380663"

mkdir -p "$(dirname "$REPORT_FILE")"

rm -f "$RESULTS_FILE"

ENV_FILE="${BRUNO_DIR}/environments/Local.bru"
if [ ! -f "$ENV_FILE" ]; then
  echo "Creating runtime Bruno environment from template..."
  cp "${BRUNO_DIR}/environments/Local.bru.example" "$ENV_FILE"
fi

echo "=== Arenda Reminders E2E Test Run ==="
echo "Sending auth code to ${PHONE}..."
curl -s -X POST "${BASE_URL}/auth/phone/send" \
  -H 'Content-Type: application/json' \
  -d "{\"phone\":\"${PHONE}\"}" \
  -o /dev/null -w '%{http_code}\n'

sleep 1

CODE=$(grep -oE 'Код подтверждения: [0-9]+' "$LOG_FILE" | tail -1 | awk '{print $3}')
if [ -z "$CODE" ]; then
  echo "ERROR: could not extract verification code from ${LOG_FILE}" >&2
  exit 1
fi
echo "Got verification code: ${CODE}"

COOKIE_JAR=$(mktemp)
trap 'rm -f "$COOKIE_JAR"' EXIT

echo "Verifying code..."
curl -s -X POST "${BASE_URL}/auth/phone/verify" \
  -H 'Content-Type: application/json' \
  -d "{\"phone\":\"${PHONE}\",\"code\":\"${CODE}\"}" \
  -c "$COOKIE_JAR" -o /dev/null -w '%{http_code}\n'

SESSION_ID=$(grep -E '^#HttpOnly_localhost' "$COOKIE_JAR" | grep 'session_id' | awk '{print $7}' | tail -1)
if [ -z "$SESSION_ID" ]; then
  SESSION_ID=$(grep 'session_id' "$COOKIE_JAR" | awk '{print $7}' | tail -1)
fi
if [ -z "$SESSION_ID" ]; then
  echo "ERROR: could not extract session_id from cookie jar" >&2
  cat "$COOKIE_JAR" >&2
  exit 1
fi
echo "Got session_id: ${SESSION_ID:0:16}..."

echo "Seeding test tariff to allow multiple properties..."
docker exec -i arenda-local-postgres-1 psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -v ON_ERROR_STOP=1 -c "UPDATE tariffs SET active_property_limit = 100 WHERE name = 'basic';" >/dev/null 2>&1 || echo "WARN: could not seed tariff"

cd "$BRUNO_DIR"

echo "Running Bruno collection reminders-e2e..."
if bru run reminders-e2e -r --env Local --env-var session_id="$SESSION_ID" -o "$RESULTS_FILE" 2>&1 | tee "${REPO_ROOT}/tools/bruno/reminders-e2e-output.txt"; then
  BRUNO_STATUS=0
else
  BRUNO_STATUS=$?
fi

echo "Polling DB for worker-dispatched SMS reminders (up to 90s)..."
SENT_COUNT=0
for i in {1..90}; do
  SENT_COUNT=$(docker exec -i arenda-local-postgres-1 psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -t -A -c "SELECT COUNT(*) FROM sent_sms_reminders;" 2>/dev/null | head -1 | tr -d ' \n' || echo 0)
  if [ "$SENT_COUNT" -gt 0 ] 2>/dev/null; then
    echo "Worker dispatched ${SENT_COUNT} reminder(s) after ${i}s"
    break
  fi
  sleep 1
done
if [ "$SENT_COUNT" -eq 0 ] 2>/dev/null; then
  echo "WARN: no SMS audit rows observed within 90s"
fi

echo "Running DB verification..."
DB_OUT=$(docker exec -i arenda-local-postgres-1 psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -v ON_ERROR_STOP=1 -f - <<'SQL'
SELECT 'total_reminders_by_status' AS metric, status, COUNT(*) AS cnt
FROM reminders
GROUP BY status
UNION ALL
SELECT 'sent_sms_audit_rows' AS metric, NULL AS status, COUNT(*) AS cnt
FROM sent_sms_reminders
UNION ALL
SELECT 'failed_reminders' AS metric, NULL AS status, COUNT(*) AS cnt
FROM reminders WHERE status = 'failed'
UNION ALL
SELECT 'cancelled_reminders' AS metric, NULL AS status, COUNT(*) AS cnt
FROM reminders WHERE status = 'cancelled'
ORDER BY metric, status;
SQL
) || DB_OUT="DB verification failed"

COMMIT_SHA=$(cd "$REPO_ROOT" && git rev-parse HEAD)

{
  echo "# Reminders API E2E Test Report"
  echo ""
  echo "- **Date:** $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "- **Commit:** ${COMMIT_SHA}"
  echo "- **Base URL:** ${BASE_URL}"
  echo "- **Phone:** ${PHONE}"
  echo ""
  echo "## Bruno CLI Results"
  echo ""
  if [ -f "$RESULTS_FILE" ]; then
    echo "Results saved to: ${RESULTS_FILE}"
    echo ""
    echo '```json'
    cat "$RESULTS_FILE"
    echo ""
    echo '```'
  else
    echo "No results file generated. CLI exit status: ${BRUNO_STATUS}"
  fi
  echo ""
  echo "## DB Verification"
  echo ""
  echo '```'
  echo "$DB_OUT"
  echo '```'
  echo ""
  echo "## Recent Backend Log Excerpt"
  echo ""
  echo '```'
  tail -n 50 "$LOG_FILE"
  echo '```'
} > "$REPORT_FILE"

echo ""
echo "Report written to: ${REPORT_FILE}"
if [ "$BRUNO_STATUS" -eq 0 ]; then
  echo "✅ Bruno run completed successfully"
else
  echo "⚠️ Bruno run exited with status ${BRUNO_STATUS}"
fi

exit "$BRUNO_STATUS"
