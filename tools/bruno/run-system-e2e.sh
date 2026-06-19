#!/usr/bin/env bash
set -euo pipefail

# Runner for the full-system Bruno E2E collection.
#
# It starts the local backend, sends an auth code to a unique phone number,
# extracts the fake SMS code from the backend logs, and then runs the entire
# system-e2e collection via the Bruno CLI.

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BACKEND_DIR="${PROJECT_ROOT}/apps/backend"
BRUNO_COLLECTION="${PROJECT_ROOT}/tools/bruno/arenda-api"
E2E_FOLDER="${BRUNO_COLLECTION}/system-e2e"

BASE_URL="${BASE_URL:-http://localhost:8080}"
ENV_NAME="${ENV_NAME:-Local}"

if ! command -v bru >/dev/null 2>&1; then
  echo "Error: Bruno CLI (bru) is not installed." >&2
  echo "Install it with: npm install -g @usebruno/cli" >&2
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "Error: curl is required." >&2
  exit 1
fi

cd "${PROJECT_ROOT}"

if [[ ! -f .env ]]; then
  echo "Error: .env not found at ${PROJECT_ROOT}/.env" >&2
  echo "Copy .env.example, fill it, and start local infra with: make local-infra-up" >&2
  exit 1
fi

# Generate a unique Russian mobile phone number for this run to avoid rate limits.
TIMESTAMP=$(date +%s)
PHONE="+7915${TIMESTAMP: -7}"

echo "Starting backend for system E2E run..."
BACKEND_LOG=$(mktemp)
# shellcheck disable=SC2064
trap "rm -f ${BACKEND_LOG}; kill_backend" EXIT

kill_backend() {
  if [[ -n "${BACKEND_PID:-}" ]] && kill -0 "${BACKEND_PID}" 2>/dev/null; then
    echo "Stopping backend (pid ${BACKEND_PID})..."
    # go run spawns a child binary; terminate children first.
    pkill -P "${BACKEND_PID}" 2>/dev/null || true
    kill "${BACKEND_PID}" 2>/dev/null || true
    wait "${BACKEND_PID}" 2>/dev/null || true
  fi
}

# Start the backend in the background and tee its output to a log file.
set -a
# shellcheck source=/dev/null
. ./.env
set +a

go run ./apps/backend/cmd/api >"${BACKEND_LOG}" 2>&1 &
BACKEND_PID=$!

echo "Backend pid: ${BACKEND_PID}; log: ${BACKEND_LOG}"

# Wait for the backend to accept connections.
echo "Waiting for backend to be ready..."
READY=false
for _ in $(seq 1 60); do
  HTTP_CODE=$(curl -s -o /dev/null -w '%{http_code}' "${BASE_URL}/tariffs" || true)
  if [[ "${HTTP_CODE}" == "401" || "${HTTP_CODE}" == "200" ]]; then
    READY=true
    break
  fi
  sleep 1
done

if [[ "${READY}" != "true" ]]; then
  echo "Error: backend did not become ready within 60 seconds." >&2
  tail -n 50 "${BACKEND_LOG}" >&2
  exit 1
fi

echo "Backend ready. Sending auth code to ${PHONE}..."
curl -s -X POST "${BASE_URL}/auth/phone/send" \
  -H 'Content-Type: application/json' \
  -d "{\"phone\":\"${PHONE}\"}" >/dev/null

# Extract the code from the backend log.
SMS_CODE=$(grep -oE 'Код подтверждения: [0-9]{6}' "${BACKEND_LOG}" | tail -1 | grep -oE '[0-9]{6}' || true)

if [[ -z "${SMS_CODE}" ]]; then
  echo "Error: could not extract SMS code from backend logs." >&2
  tail -n 50 "${BACKEND_LOG}" >&2
  exit 1
fi

echo "Extracted SMS code: ${SMS_CODE}"

echo "Running Bruno system-e2e collection..."
cd "${BRUNO_COLLECTION}"
bru run system-e2e -r \
  --env "${ENV_NAME}" \
  --env-var phone="${PHONE}" \
  --env-var code="${SMS_CODE}" \
  --env-var cookieName=session_id \
  --env-var skipSend=true \
  --bail

echo "System E2E collection completed successfully."
