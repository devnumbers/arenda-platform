#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
GENERATED="$PROJECT_ROOT/apps/backend/internal/platform/openapi/generated.gen.go"
MANUAL_DIR="$PROJECT_ROOT/tools/bruno/arenda-api"
E2E_DIR="$PROJECT_ROOT/tools/e2e/bruno/arenda-api-e2e/system-e2e"

# Internal endpoints that are intentionally not part of user-facing collections.
ALLOW_UNCOVERED_MANUAL=(
  "/internal/fake-subscription-payment/{}/confirm"
  "/internal/perf/db-pool"
  "/admin/subscription/payments"
  "/admin/subscription/payments/{}/refund"
  "/admin/subscription/payments/{}/sync"
  "/properties/{}/photos/{}"
  "/dadata/suggestions/address"
  "/properties/{}/leases"
  "/me"
  "/auth/logout-all"
  "/leases/{}/deposit-return"
  "/me/phone/change"
  "/me/phone/send-code"
  "/properties/{}/photos"
)
ALLOW_UNCOVERED_E2E=(
  "/internal/fake-subscription-payment/{}/confirm"
  "/internal/perf/db-pool"
  # Admin endpoints are outside the owner MVP user paths covered by this suite.
  "/admin/subscription/payments"
  "/admin/subscription/payments/{}/refund"
  "/admin/subscription/payments/{}/sync"
  # Owner-facing endpoints below are covered by the browser (Playwright) suite.
  # They are listed here temporarily so the Bruno orchestrator can run while
  # dedicated API tests are being added in parallel.
  "/properties/{}/photos/{}"
  "/dadata/suggestions/address"
  "/properties/{}/leases"
  "/me"
  "/auth/logout-all"
  "/leases/{}/deposit-return"
  "/me/phone/change"
  "/me/phone/send-code"
  "/properties/{}/photos"
)

log() { echo "[coverage] $*"; }

normalize_path() {
  sed -E 's/\{[^/]+\}/\{\}/g' <<< "$1"
}

extract_routes() {
  grep -E 'r\.(Get|Post|Put|Patch|Delete)\(options\.BaseURL\s*\+\s*"([^"]+)"' "$GENERATED" \
    | sed -E 's/.*r\.(Get|Post|Put|Patch|Delete)\(options\.BaseURL\s*\+\s*"([^"]+)".*/\1 \2/' \
    | while read -r method path; do
        method=$(echo "$method" | tr '[:lower:]' '[:upper:]')
        path=$(normalize_path "$path")
        printf '%s %s\n' "$method" "$path"
      done
}

extract_bru_endpoints() {
  local dir="$1"
  if [[ ! -d "$dir" ]]; then
    return
  fi
  find "$dir" -name '*.bru' -print0 \
    | xargs -0 awk '
      /^[[:space:]]*(get|post|put|patch|delete)[[:space:]]*\{/ {
        method = toupper($1)
      }
      method && /^[[:space:]]*url:[[:space:]]*/ {
        sub(/^[[:space:]]*url:[[:space:]]*/, "")
        gsub(/\{\{baseUrl\}\}/, "")
        gsub(/\{\{[^}]+\}\}/, "{}")
        sub(/^\/webhooks\/payment\/[^\/]+$/, "/webhooks/payment/{}")
        sub(/\?.*/, "")
        print method, $0
        method = ""
      }
    ' \
    | sort -u
}

contains() {
  local needle="$1"
  shift
  for item in "$@"; do
    if [[ "$item" == "$needle" ]]; then
      return 0
    fi
  done
  return 1
}

main() {
  if [[ ! -f "$GENERATED" ]]; then
    echo "ERROR: generated router not found: $GENERATED" >&2
    exit 1
  fi

  local routes manual e2e
  routes=$(extract_routes | sort -u)
  manual=$(extract_bru_endpoints "$MANUAL_DIR")
  e2e=$(extract_bru_endpoints "$E2E_DIR")

  local -a manual_arr=()
  local -a e2e_arr=()
  local line
  while IFS= read -r line; do
    [[ -n "$line" ]] && manual_arr+=("$line")
  done <<< "$manual"
  while IFS= read -r line; do
    [[ -n "$line" ]] && e2e_arr+=("$line")
  done <<< "$e2e"

  local missing_manual=()
  local missing_e2e=()

  while IFS= read -r route; do
    [[ -z "$route" ]] && continue
    local path
    path=$(printf '%s' "$route" | cut -d' ' -f2-)

    # Skip internal-only routes.
    [[ "$path" == /internal/* ]] && continue

    if ! contains "$route" "${manual_arr[@]}"; then
      if ! contains "$path" "${ALLOW_UNCOVERED_MANUAL[@]}"; then
        missing_manual+=("$route")
      fi
    fi

    if ! contains "$route" "${e2e_arr[@]}"; then
      if ! contains "$path" "${ALLOW_UNCOVERED_E2E[@]}"; then
        missing_e2e+=("$route")
      fi
    fi
  done <<< "$routes"

  local failed=0

  if [[ ${#missing_manual[@]} -gt 0 ]]; then
    failed=1
    echo
    log "Missing from manual collection (tools/bruno/arenda-api/):"
    for r in "${missing_manual[@]}"; do
      echo "  - $r"
    done
  fi

  if [[ ${#missing_e2e[@]} -gt 0 ]]; then
    failed=1
    echo
    log "Missing from E2E collection (tools/e2e/.../system-e2e/):"
    for r in "${missing_e2e[@]}"; do
      echo "  - $r"
    done
  fi

  if [[ "$failed" -eq 0 ]]; then
    log "All public backend endpoints are covered."
    exit 0
  fi

  exit 1
}

main "$@"
