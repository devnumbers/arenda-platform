# Bruno/E2E Coverage Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make `tools/bruno/arenda-api/` cover every public backend endpoint, add a webhook scenario to the system E2E suite, and introduce a coverage gate that fails when public endpoints drift out of the collections.

**Architecture:** A small bash coverage script compares the canonical route list extracted from the OpenAPI-generated router (`generated.gen.go`) with the endpoints found in `.bru` files. Missing public endpoints are reported for both the manual collection and the E2E collection. New `.bru` requests and one E2E webhook folder close the gaps, and the gate is wired into the E2E runner.

**Tech Stack:** Bruno CLI (`bru`), bash 3.2, `awk`, `grep`, `sed`, existing Go backend.

---

## Task 0: Verify local environment

**Files:**
- Read: `apps/backend/internal/platform/openapi/generated.gen.go`
- Read: `tools/bruno/arenda-api/environments/Local.bru`
- Read: `tools/e2e/bruno/arenda-api-e2e/environments/Local.bru`

**Step 1:** Confirm backend can start locally.

Run:
```bash
make local-infra-up
make backend-run
```

Expected: backend logs show it is listening on `:8080` and PostgreSQL is reachable.

**Step 2:** Stop the backend (`Ctrl-C`) but keep Postgres running.

---

## Task 1: Create the coverage gate script

**Files:**
- Create: `tools/e2e/check-bruno-coverage.sh`
- Modify: `tools/e2e/run-e2e-with-db-checks.sh` (call the gate before the feature run)
- Modify: `Makefile` (add a convenience target)

**Step 1:** Write `tools/e2e/check-bruno-coverage.sh`.

```bash
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
)
ALLOW_UNCOVERED_E2E=(
  "/internal/fake-subscription-payment/{}/confirm"
  "/internal/perf/db-pool"
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

  local missing_manual=()
  local missing_e2e=()

  while IFS= read -r route; do
    [[ -z "$route" ]] && continue
    local path
    path=$(printf '%s' "$route" | cut -d' ' -f2-)

    # Skip internal-only routes.
    [[ "$path" == /internal/* ]] && continue

    if ! contains "$route" $manual; then
      if ! contains "$path" "${ALLOW_UNCOVERED_MANUAL[@]}"; then
        missing_manual+=("$route")
      fi
    fi

    if ! contains "$route" $e2e; then
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
```

**Step 2:** Make it executable.

Run:
```bash
chmod +x tools/e2e/check-bruno-coverage.sh
```

**Step 3:** Add a Makefile target.

Modify `Makefile` after the `backend-lint` target:

```makefile
.PHONY: check-bruno-coverage

check-bruno-coverage:
	./tools/e2e/check-bruno-coverage.sh
```

**Step 4:** Call the gate at the start of the E2E runner.

Modify `tools/e2e/run-e2e-with-db-checks.sh`, adding after the `main()` function begins and before health checks:

```bash
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
  ...
}
```

**Step 5:** Run the gate to prove it reports current gaps.

Run:
```bash
./tools/e2e/check-bruno-coverage.sh
```

Expected: FAIL with a list of missing manual endpoints (subscription/tariffs/nested GETs) and missing E2E endpoint (`POST /webhooks/payment/{}`).

**Step 6:** Commit.

```bash
git add tools/e2e/check-bruno-coverage.sh tools/e2e/run-e2e-with-db-checks.sh Makefile
git commit -m "feat(tools): add Bruno/E2E coverage gate"
```

---

## Task 2: Add subscription manual requests

**Files:**
- Create: `tools/bruno/arenda-api/subscription/get subscription.bru`
- Create: `tools/bruno/arenda-api/subscription/get tariffs.bru`
- Create: `tools/bruno/arenda-api/subscription/toggle auto-renew.bru`
- Create: `tools/bruno/arenda-api/subscription/cancel subscription.bru`
- Create: `tools/bruno/arenda-api/subscription/change tariff.bru`
- Create: `tools/bruno/arenda-api/subscription/list payment methods.bru`
- Create: `tools/bruno/arenda-api/subscription/add payment method.bru`
- Create: `tools/bruno/arenda-api/subscription/delete payment method.bru`
- Create: `tools/bruno/arenda-api/subscription/activate payment method.bru`
- Create: `tools/bruno/arenda-api/subscription/list payments.bru`

**Step 1:** Create the subscription folder.

Run:
```bash
mkdir -p tools/bruno/arenda-api/subscription
```

**Step 2:** Write each `.bru` file.

`tools/bruno/arenda-api/subscription/get subscription.bru`:
```text
meta {
  name: Get Subscription
  type: http
  seq: 1
}

get {
  url: {{baseUrl}}/subscription
  body: none
  auth: none
}

script:pre-request {
  const sessionId = bru.getEnvVar("session_id") || bru.getVar("session_id");
  const cookieName = bru.getEnvVar("cookieName") || bru.getVar("cookieName") || "session_id";
  if (sessionId) {
    req.setHeader("Cookie", `${cookieName}=${sessionId}`);
  }
}

tests {
  test("should return 200", function() {
    expect(res.status).to.equal(200);
  });

  test("should have status", function() {
    expect(res.body.status).to.be.a("string");
  });
}
```

`tools/bruno/arenda-api/subscription/get tariffs.bru`:
```text
meta {
  name: Get Tariffs
  type: http
  seq: 2
}

get {
  url: {{baseUrl}}/tariffs
  body: none
  auth: none
}

script:pre-request {
  const sessionId = bru.getEnvVar("session_id") || bru.getVar("session_id");
  const cookieName = bru.getEnvVar("cookieName") || bru.getVar("cookieName") || "session_id";
  if (sessionId) {
    req.setHeader("Cookie", `${cookieName}=${sessionId}`);
  }
}

tests {
  test("should return 200", function() {
    expect(res.status).to.equal(200);
  });

  test("should have items", function() {
    expect(res.body.items).to.be.an("array");
  });
}
```

`tools/bruno/arenda-api/subscription/toggle auto-renew.bru`:
```text
meta {
  name: Toggle Auto Renew
  type: http
  seq: 3
}

patch {
  url: {{baseUrl}}/subscription/auto-renew
  body: json
  auth: none
}

headers {
  content-type: application/json
}

body:json {
  {
    "enabled": true
  }
}

script:pre-request {
  const sessionId = bru.getEnvVar("session_id") || bru.getVar("session_id");
  const cookieName = bru.getEnvVar("cookieName") || bru.getVar("cookieName") || "session_id";
  if (sessionId) {
    req.setHeader("Cookie", `${cookieName}=${sessionId}`);
  }
}

tests {
  test("should return 200", function() {
    expect(res.status).to.equal(200);
  });
}
```

`tools/bruno/arenda-api/subscription/cancel subscription.bru`:
```text
meta {
  name: Cancel Subscription
  type: http
  seq: 4
}

post {
  url: {{baseUrl}}/subscription/cancel
  body: none
  auth: none
}

script:pre-request {
  const sessionId = bru.getEnvVar("session_id") || bru.getVar("session_id");
  const cookieName = bru.getEnvVar("cookieName") || bru.getVar("cookieName") || "session_id";
  if (sessionId) {
    req.setHeader("Cookie", `${cookieName}=${sessionId}`);
  }
}

tests {
  test("should return 200", function() {
    expect(res.status).to.equal(200);
  });
}
```

`tools/bruno/arenda-api/subscription/change tariff.bru`:
```text
meta {
  name: Change Tariff
  type: http
  seq: 5
}

post {
  url: {{baseUrl}}/subscription/change
  body: json
  auth: none
}

headers {
  content-type: application/json
}

body:json {
  {
    "tariffName": "pro",
    "period": "month"
  }
}

script:pre-request {
  const sessionId = bru.getEnvVar("session_id") || bru.getVar("session_id");
  const cookieName = bru.getEnvVar("cookieName") || bru.getVar("cookieName") || "session_id";
  if (sessionId) {
    req.setHeader("Cookie", `${cookieName}=${sessionId}`);
  }
}

script:post-response {
  if (res.body && res.body.paymentId) {
    bru.setEnvVar("paymentId", res.body.paymentId);
    bru.setVar("paymentId", res.body.paymentId);
  }
}

tests {
  test("should return 200", function() {
    expect(res.status).to.equal(200);
  });

  test("should return payment id", function() {
    expect(res.body.paymentId).to.be.a("string");
  });
}
```

`tools/bruno/arenda-api/subscription/list payment methods.bru`:
```text
meta {
  name: List Payment Methods
  type: http
  seq: 6
}

get {
  url: {{baseUrl}}/subscription/payment-methods
  body: none
  auth: none
}

script:pre-request {
  const sessionId = bru.getEnvVar("session_id") || bru.getVar("session_id");
  const cookieName = bru.getEnvVar("cookieName") || bru.getVar("cookieName") || "session_id";
  if (sessionId) {
    req.setHeader("Cookie", `${cookieName}=${sessionId}`);
  }
}

tests {
  test("should return 200", function() {
    expect(res.status).to.equal(200);
  });

  test("should have items", function() {
    expect(res.body.items).to.be.an("array");
  });
}
```

`tools/bruno/arenda-api/subscription/add payment method.bru`:
```text
meta {
  name: Add Payment Method
  type: http
  seq: 7
}

post {
  url: {{baseUrl}}/subscription/payment-methods
  body: json
  auth: none
}

headers {
  content-type: application/json
}

body:json {
  {
    "providerToken": "fake_token_manual"
  }
}

script:pre-request {
  const sessionId = bru.getEnvVar("session_id") || bru.getVar("session_id");
  const cookieName = bru.getEnvVar("cookieName") || bru.getVar("cookieName") || "session_id";
  if (sessionId) {
    req.setHeader("Cookie", `${cookieName}=${sessionId}`);
  }
}

script:post-response {
  if (res.body && res.body.id) {
    bru.setEnvVar("paymentMethodId", res.body.id);
    bru.setVar("paymentMethodId", res.body.id);
  }
}

tests {
  test("should return 201", function() {
    expect(res.status).to.equal(201);
  });

  test("should have payment method id", function() {
    expect(res.body.id).to.be.a("string");
  });
}
```

`tools/bruno/arenda-api/subscription/delete payment method.bru`:
```text
meta {
  name: Delete Payment Method
  type: http
  seq: 8
}

delete {
  url: {{baseUrl}}/subscription/payment-methods/{{paymentMethodId}}
  body: none
  auth: none
}

script:pre-request {
  const sessionId = bru.getEnvVar("session_id") || bru.getVar("session_id");
  const cookieName = bru.getEnvVar("cookieName") || bru.getVar("cookieName") || "session_id";
  if (sessionId) {
    req.setHeader("Cookie", `${cookieName}=${sessionId}`);
  }
}

tests {
  test("should return 204", function() {
    expect(res.status).to.equal(204);
  });
}
```

`tools/bruno/arenda-api/subscription/activate payment method.bru`:
```text
meta {
  name: Activate Payment Method
  type: http
  seq: 9
}

post {
  url: {{baseUrl}}/subscription/payment-methods/{{paymentMethodId}}/activate
  body: none
  auth: none
}

script:pre-request {
  const sessionId = bru.getEnvVar("session_id") || bru.getVar("session_id");
  const cookieName = bru.getEnvVar("cookieName") || bru.getVar("cookieName") || "session_id";
  if (sessionId) {
    req.setHeader("Cookie", `${cookieName}=${sessionId}`);
  }
}

tests {
  test("should return 200", function() {
    expect(res.status).to.equal(200);
  });
}
```

`tools/bruno/arenda-api/subscription/list payments.bru`:
```text
meta {
  name: List Payments
  type: http
  seq: 10
}

get {
  url: {{baseUrl}}/subscription/payments
  body: none
  auth: none
}

script:pre-request {
  const sessionId = bru.getEnvVar("session_id") || bru.getVar("session_id");
  const cookieName = bru.getEnvVar("cookieName") || bru.getVar("cookieName") || "session_id";
  if (sessionId) {
    req.setHeader("Cookie", `${cookieName}=${sessionId}`);
  }
}

tests {
  test("should return 200", function() {
    expect(res.status).to.equal(200);
  });

  test("should have items", function() {
    expect(res.body.items).to.be.an("array");
  });
}
```

**Step 3:** Commit.

```bash
git add tools/bruno/arenda-api/subscription/
git commit -m "feat(tools): add subscription endpoints to manual Bruno collection"
```

---

## Task 3: Add missing nested GET requests to manual collection

**Files:**
- Create: `tools/bruno/arenda-api/leases/get lease reminders.bru`
- Create: `tools/bruno/arenda-api/properties/get property operations.bru`
- Create: `tools/bruno/arenda-api/properties/get property recurring operations.bru`
- Modify: `tools/bruno/README.md`

**Step 1:** Write the files.

`tools/bruno/arenda-api/leases/get lease reminders.bru`:
```text
meta {
  name: Get Lease Reminders
  type: http
  seq: 6
}

get {
  url: {{baseUrl}}/leases/{{leaseId}}/reminders
  body: none
  auth: none
}

script:pre-request {
  const sessionId = bru.getEnvVar("session_id") || bru.getVar("session_id");
  const cookieName = bru.getEnvVar("cookieName") || bru.getVar("cookieName") || "session_id";
  if (sessionId) {
    req.setHeader("Cookie", `${cookieName}=${sessionId}`);
  }
}

tests {
  test("should return 200", function() {
    expect(res.status).to.equal(200);
  });

  test("should have items", function() {
    expect(res.body.items).to.be.an("array");
  });
}
```

`tools/bruno/arenda-api/properties/get property operations.bru`:
```text
meta {
  name: Get Property Operations
  type: http
  seq: 7
}

get {
  url: {{baseUrl}}/properties/{{propertyId}}/operations
  body: none
  auth: none
}

script:pre-request {
  const sessionId = bru.getEnvVar("session_id") || bru.getVar("session_id");
  const cookieName = bru.getEnvVar("cookieName") || bru.getVar("cookieName") || "session_id";
  if (sessionId) {
    req.setHeader("Cookie", `${cookieName}=${sessionId}`);
  }
}

tests {
  test("should return 200", function() {
    expect(res.status).to.equal(200);
  });

  test("should have items", function() {
    expect(res.body.items).to.be.an("array");
  });
}
```

`tools/bruno/arenda-api/properties/get property recurring operations.bru`:
```text
meta {
  name: Get Property Recurring Operations
  type: http
  seq: 8
}

get {
  url: {{baseUrl}}/properties/{{propertyId}}/recurring-operations
  body: none
  auth: none
}

script:pre-request {
  const sessionId = bru.getEnvVar("session_id") || bru.getVar("session_id");
  const cookieName = bru.getEnvVar("cookieName") || bru.getVar("cookieName") || "session_id";
  if (sessionId) {
    req.setHeader("Cookie", `${cookieName}=${sessionId}`);
  }
}

tests {
  test("should return 200", function() {
    expect(res.status).to.equal(200);
  });

  test("should have items", function() {
    expect(res.body.items).to.be.an("array");
  });
}
```

**Step 2:** Update `tools/bruno/README.md` to mention `subscription` and `webhooks` (after Task 4).

**Step 3:** Commit.

```bash
git add tools/bruno/arenda-api/leases/get\ lease\ reminders.bru \
  tools/bruno/arenda-api/properties/get\ property\ operations.bru \
  tools/bruno/arenda-api/properties/get\ property\ recurring\ operations.bru
git commit -m "feat(tools): add nested GET endpoints to manual Bruno collection"
```

---

## Task 4: Add webhook request to manual collection

**Files:**
- Create: `tools/bruno/arenda-api/webhooks/payment webhook.bru`

**Step 1:** Create folder and file.

Run:
```bash
mkdir -p tools/bruno/arenda-api/webhooks
```

`tools/bruno/arenda-api/webhooks/payment webhook.bru`:
```text
meta {
  name: Payment Webhook
  type: http
  seq: 1
}

post {
  url: {{baseUrl}}/webhooks/payment/fake
  body: json
  auth: none
}

headers {
  content-type: application/json
}

body:json {
  {
    "provider_payment_id": "fake-webhook-payment-id",
    "internal_payment_id": "{{paymentId}}",
    "status": "succeeded"
  }
}

tests {
  test("should return 200", function() {
    expect(res.status).to.equal(200);
  });

  test("should return ok status", function() {
    expect(res.body.status).to.equal("ok");
  });
}
```

**Step 2:** Update `tools/bruno/README.md` to list `subscription` and `webhooks`.

```markdown
- `subscription` — tariffs, current subscription, payment methods, and payments
- `webhooks` — incoming payment provider webhooks
```

**Step 3:** Commit.

```bash
git add tools/bruno/arenda-api/webhooks/ tools/bruno/README.md
git commit -m "feat(tools): add webhook endpoint to manual Bruno collection"
```

---

## Task 5: Add webhook E2E scenario

**Files:**
- Create: `tools/e2e/bruno/arenda-api-e2e/system-e2e/85-webhooks/00-setup/01 change tariff business.bru`
- Create: `tools/e2e/bruno/arenda-api-e2e/system-e2e/85-webhooks/01-positive/01 payment webhook succeeded.bru`
- Create: `tools/e2e/bruno/arenda-api-e2e/system-e2e/85-webhooks/01-positive/02 get subscription after webhook.bru`
- Create: `tools/e2e/bruno/arenda-api-e2e/system-e2e/85-webhooks/02-negative/01 payment webhook invalid body.bru`
- Modify: `tools/e2e/run-e2e-with-db-checks.sh`
- Modify: `tools/e2e/README.md`
- Create: `tools/e2e/sql/12-webhook-payment-processed.sql`

**Step 1:** Create the folder.

Run:
```bash
mkdir -p tools/e2e/bruno/arenda-api-e2e/system-e2e/85-webhooks/{00-setup,01-positive,02-negative}
```

**Step 2:** Write setup request.

`tools/e2e/bruno/arenda-api-e2e/system-e2e/85-webhooks/00-setup/01 change tariff business.bru`:
```text
meta {
  name: Change Tariff Business
  type: http
  seq: 1
}

post {
  url: {{baseUrl}}/subscription/change
  body: json
  auth: none
}

headers {
  content-type: application/json
}

body:json {
  {
    "tariffName": "business",
    "period": "month"
  }
}

script:pre-request {
  const sessionId = bru.getEnvVar("session_id") || bru.getVar("session_id");
  const cookieName = bru.getEnvVar("cookieName") || bru.getVar("cookieName") || "session_id";
  if (sessionId) {
    req.setHeader("Cookie", `${cookieName}=${sessionId}`);
  }
}

script:post-response {
  if (res.body && res.body.paymentId) {
    bru.setEnvVar("webhookPaymentId", res.body.paymentId);
    bru.setVar("webhookPaymentId", res.body.paymentId);
    console.log("saved webhookPaymentId =", res.body.paymentId);
  }
}

tests {
  test("should return 200", function() {
    expect(res.status).to.equal(200);
  });

  test("should return payment id", function() {
    expect(res.body.paymentId).to.be.a("string");
  });
}
```

**Step 3:** Write positive webhook request.

`tools/e2e/bruno/arenda-api-e2e/system-e2e/85-webhooks/01-positive/01 payment webhook succeeded.bru`:
```text
meta {
  name: Payment Webhook Succeeded
  type: http
  seq: 1
}

post {
  url: {{baseUrl}}/webhooks/payment/fake
  body: json
  auth: none
}

headers {
  content-type: application/json
}

body:json {
  {
    "provider_payment_id": "fake-webhook-e2e-payment-id",
    "internal_payment_id": "{{webhookPaymentId}}",
    "status": "succeeded"
  }
}

tests {
  test("should return 200", function() {
    expect(res.status).to.equal(200);
  });

  test("should return ok status", function() {
    expect(res.body.status).to.equal("ok");
  });
}
```

**Step 4:** Write follow-up verification.

`tools/e2e/bruno/arenda-api-e2e/system-e2e/85-webhooks/01-positive/02 get subscription after webhook.bru`:
```text
meta {
  name: Get Subscription After Webhook
  type: http
  seq: 2
}

get {
  url: {{baseUrl}}/subscription
  body: none
  auth: none
}

script:pre-request {
  const sessionId = bru.getEnvVar("session_id") || bru.getVar("session_id");
  const cookieName = bru.getEnvVar("cookieName") || bru.getVar("cookieName") || "session_id";
  if (sessionId) {
    req.setHeader("Cookie", `${cookieName}=${sessionId}`);
  }
}

tests {
  test("should return 200", function() {
    expect(res.status).to.equal(200);
  });

  test("should be active", function() {
    expect(res.body.status).to.equal("active");
  });

  test("should be on business tariff", function() {
    expect(res.body.tariff.name).to.equal("business");
  });
}
```

**Step 5:** Write negative webhook request.

`tools/e2e/bruno/arenda-api-e2e/system-e2e/85-webhooks/02-negative/01 payment webhook invalid body.bru`:
```text
meta {
  name: Payment Webhook Invalid Body
  type: http
  seq: 1
}

post {
  url: {{baseUrl}}/webhooks/payment/fake
  body: json
  auth: none
}

headers {
  content-type: application/json
}

body:json {
  {
    "internal_payment_id": "not-a-uuid",
    "status": "succeeded"
  }
}

tests {
  test("should still return 200 to avoid leaking errors", function() {
    expect(res.status).to.equal(200);
  });

  test("should return ok status", function() {
    expect(res.body.status).to.equal("ok");
  });
}
```

**Step 6:** Add SQL check.

`tools/e2e/sql/12-webhook-payment-processed.sql`:
```sql
SELECT EXISTS (
  SELECT 1
  FROM subscription_payments sp
  JOIN subscriptions s ON s.id = sp.subscription_id
  JOIN users u ON u.id = s.user_id
  WHERE u.phone = :phone
    AND sp.id = :payment_id
    AND sp.status = 'succeeded'
) AS ok;
```

**Step 7:** Wire the new folder into the runner.

Modify `tools/e2e/run-e2e-with-db-checks.sh`:

1. Add `"system-e2e/85-webhooks"` to the `feature_folders` array after `system-e2e/80-readonly-recovery` and before `system-e2e/99-final-cleanup`.
2. Add a SQL-check mapping in `sql_checks_for_folder()`:

```bash
system-e2e/85-webhooks)
  echo "12-webhook-payment-processed.sql"
  ;;
```
3. Add variable derivation in `derive_sql_vars()`:

```bash
12-webhook-payment-processed.sql)
  echo "-v phone=$PHONE -v payment_id=$(bru.getEnvVar('webhookPaymentId') || true)"
  ;;
```

Wait: `derive_sql_vars` is bash, not Bruno. It cannot read `webhookPaymentId` from env var because the Bruno runner saves it only in the report. We need to extract it from the JSON report like `active_payment_method_id`. Add a helper:

```bash
extract_webhook_payment_id() {
  local report_json="$1"
  if [[ ! -f "$report_json" ]] || ! command -v jq >/dev/null 2>&1; then
    return 1
  fi
  jq -r '.[0].results[]? | select(.test.filename | contains("change tariff business")) | .response.data.paymentId // empty' "$report_json" 2>/dev/null | head -1
}
```

Then in the main loop after running `system-e2e/85-webhooks`, capture it:

```bash
if [[ "$folder" == "system-e2e/85-webhooks" ]]; then
  webhook_payment_id=$(extract_webhook_payment_id "${BRUNO_REPORT_BASE}-system-e2e-85-webhooks.json") || true
fi
```

And in `derive_sql_vars`:

```bash
12-webhook-payment-processed.sql)
  echo "-v phone=$PHONE -v payment_id=${webhook_payment_id:-}"
  ;;
```

Actually the SQL check runs inside the same folder loop; we can capture payment id before running SQL by setting a global variable after the folder run. Because `run_bruno_folder` returns and then SQL checks run. So set `WEBHOOK_PAYMENT_ID` after the folder succeeds.

**Step 8:** Update `tools/e2e/README.md` to mention `85-webhooks` in the folder list.

**Step 9:** Commit.

```bash
git add tools/e2e/bruno/arenda-api-e2e/system-e2e/85-webhooks/ \
  tools/e2e/sql/12-webhook-payment-processed.sql \
  tools/e2e/run-e2e-with-db-checks.sh \
  tools/e2e/README.md
git commit -m "feat(tools): add webhook E2E scenario"
```

---

## Task 6: Run the coverage gate and fix any drift

**Files:**
- Run: `tools/e2e/check-bruno-coverage.sh`

**Step 1:** Run the gate.

```bash
./tools/e2e/check-bruno-coverage.sh
```

Expected: PASS with `All public backend endpoints are covered.`

If any endpoint is still missing, add the corresponding `.bru` request and rerun until green.

---

## Task 7: Run the manual collection

**Files:**
- Run: `tools/bruno/arenda-api/`

**Step 1:** Start backend.

```bash
make local-infra-up
make backend-run
```

**Step 2:** In another terminal, run auth flow to obtain a session.

```bash
cd tools/bruno/arenda-api
bru run auth/send-code.bru --env Local
# Edit environments/Local.bru and set `code` to the code from backend logs.
bru run auth/verify-code.bru --env Local
# Copy the session_id from response Set-Cookie into environments/Local.bru as `session_id`.
```

**Step 3:** Run the whole manual collection.

```bash
bru run . -r --env Local
```

Expected: every request passes. If a request fails because of missing setup (e.g., `paymentMethodId` not set), run the prerequisite request first and save the variable.

**Step 4:** Fix any failing requests.

Common fixes:
- Wrong HTTP status: adjust the assertion.
- Missing variable: add a `script:post-response` block to save the id.
- Invalid body: compare with the OpenAPI spec in `apps/backend/api/openapi/openapi.yaml`.

---

## Task 8: Run the full E2E suite

**Files:**
- Run: `tools/e2e/run-e2e-with-db-checks.sh`

**Step 1:** Ensure backend and Postgres are running.

**Step 2:** Run the suite.

```bash
./tools/e2e/run-e2e-with-db-checks.sh
```

Expected: `All checks passed` and the new `85-webhooks` folder reports PASS.

**Step 3:** Fix any failures.

If the webhook SQL check fails, inspect the JSON report for `85-webhooks` to see the actual payment id and status.

---

## Task 9: Update docs and changelog

**Files:**
- Modify: `tools/bruno/README.md`
- Modify: `tools/e2e/README.md`
- Modify: `CHANGELOG.md`

**Step 1:** In `tools/bruno/README.md`, ensure the collection list includes `subscription` and `webhooks` and mention the coverage gate.

```markdown
## Current collection

- `arenda-api/` — everyday CRUD requests for:
  - `auth`
  - `properties`
  - `leases`
  - `operations`
  - `recurring-operations`
  - `tenant-contacts`
  - `reminders`
  - `subscription` — tariffs, subscription state, payment methods, payments
  - `webhooks` — incoming payment-provider webhooks

Run `make check-bruno-coverage` to verify that the collection covers every public backend endpoint.
```

**Step 2:** In `tools/e2e/README.md`, add `85-webhooks` to the numbered list and mention the coverage gate.

```markdown
10. `80-readonly-recovery`
11. `85-webhooks`
12. `99-final-cleanup`
```

**Step 3:** Add a CHANGELOG entry under `## 2026-06-22` (or current date).

```markdown
### Добавлено
- Ручная Bruno-коллекция теперь покрывает все публичные endpoint'ы backend, включая подписки, тарифы и webhook'и.
- Системные E2E-тесты дополнены сценарием обработки платёжного webhook'а.
- Добавлен coverage-шлюз `make check-bruno-coverage`, который не даёт новым endpoint'ам выпасть из коллекций.
```

**Step 4:** Commit.

```bash
git add tools/bruno/README.md tools/e2e/README.md CHANGELOG.md
git commit -m "docs(tools): document subscription/webhook coverage and coverage gate"
```

---

## Task 10: Final verification and commit summary

**Step 1:** Run all verification commands.

```bash
make check-bruno-coverage
./tools/e2e/run-e2e-with-db-checks.sh
```

**Step 2:** Push or report completion.

```bash
git log --oneline -5
```

Expected output shows commits for the coverage gate, manual collection additions, webhook E2E, and docs.
