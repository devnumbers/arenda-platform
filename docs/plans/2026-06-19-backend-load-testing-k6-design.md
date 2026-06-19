# Backend Load Testing (k6) — Design

**Date:** 2026-06-19
**Status:** Approved
**Decision:** Implement a local, repeatable k6-based load-testing harness with a Go orchestrator (`perfmaxrps`), a Go seeder (`perfseed`), and a single k6 script using `constant-arrival-rate` executors.

## Goal

Build a local, repeatable k6-based load-testing harness for `apps/backend` that measures the maximum RPS each HTTP endpoint can sustain while keeping **p95 latency of successful responses under 500 ms**.

## Non-goals

- No cloud/distributed execution (single-machine k6 is sufficient for expected load).
- No CI regression gate in this iteration (local `make` targets only).
- No production telemetry backend integration (results are local JSON/Markdown reports).

## Context

- Backend is a Go 1.26 DDD modular monolith (`chi` router), contract-first API defined in `apps/backend/api/openapi/openapi.yaml`.
- ~36 HTTP endpoints across auth, properties, leases, operations, recurring operations, reminders, tenant contacts, and billing/subscription.
- Auth uses opaque server-side `HttpOnly` cookies (`session_id`).
- Local dev stack: PostgreSQL via `docker-compose.local.yml`, backend runs with `make backend-run`.
- A previous uncommitted attempt was reviewed in `docs/reviews/2026-06-19-k6-perf-harness-review.md`; its critical findings inform this design.

## User decisions

| Decision | Choice |
|---|---|
| Endpoint scope | All ~36 endpoints |
| SLO latency metric | p95 of **successful responses** only (`http_req_duration{expected_response:true}`) |
| Auth/session source | Seed sessions directly into the perf database |
| Execution environment | Local `make` targets only |
| Background workers | Keep enabled for realistic load |
| Data isolation between endpoints | Re-seed the perf database before each endpoint benchmark |

## Design overview

The harness is a three-layer system:

1. **Data layer** — `cmd/perfseed` resets the perf PostgreSQL database and seeds deterministic owners, properties, leases, operations, recurring operations, reminders, tenant contacts, and active sessions. Each re-seed is idempotent and reproducible.
2. **Load layer** — `k6` runs one endpoint at a time with a `constant-arrival-rate` executor, tagged by endpoint name, and emits a machine-readable `summary.json`.
3. **Orchestration layer** — `cmd/perfmaxrps` runs a binary-search-like discovery for each endpoint: re-seed → run k6 at candidate RPS → parse `summary.json` → adjust RPS → converge on the highest RPS that satisfies p95 < 500 ms. It produces a Markdown report and a JSON artifact.

## Component architecture

```
┌─────────────────────────────────────────────────────────────┐
│  cmd/perfmaxrps (Go orchestrator)                           │
│  - loads endpoint manifest                                  │
│  - for each endpoint:                                       │
│      run perfseed --endpoint=<name>                         │
│      run k6 with candidate RPS                              │
│      parse summary.json                                     │
│      binary-search max RPS                                  │
│  - write report.md + results.json                           │
└─────────────────────────────────────────────────────────────┘
                              │
         ┌────────────────────┴────────────────────┐
         ▼                                         ▼
┌─────────────────┐                    ┌─────────────────────┐
│ cmd/perfseed    │                    │ k6 Docker/local     │
│ (Go seeder)     │                    │ (constant-arrival-  │
│ - truncate      │                    │  rate executor)     │
│ - insert        │                    │ - endpoint tags     │
│ - sessions      │                    │ - thresholds        │
└────────┬────────┘                    └──────────┬──────────┘
         │                                         │
         └────────────────────┬────────────────────┘
                              ▼
                   ┌──────────────────┐
                   │ PostgreSQL (perf)│
                   │ docker-compose   │
                   └──────────────────┘
```

## Endpoint classification

Endpoints are grouped by behavior to avoid fixture exhaustion and state conflicts:

| Class | Examples | Strategy |
|---|---|---|
| Read-only | `GET /properties`, `GET /leases`, `GET /operations` | Re-seed, then hit existing fixtures. |
| Bounded write | `PATCH /properties/{id}`, `POST /properties/{id}/archive` | Re-seed, use one fixture per VU/iteration. |
| Growth write | `POST /properties`, `POST /tenant-contacts`, `POST /operations` | Re-seed, but run only with an RPS×duration budget below fixture/constraint limits. Report warns if budget exceeded. |
| Lifecycle / destructive | `DELETE /operations/{id}`, `DELETE /reminders/{id}`, `POST /recurring-operations/{id}/pause` | Re-seed, pair operations (pause→resume), or run explicit-only with sufficient fixtures. |
| Auth | `POST /auth/phone/send`, `POST /auth/phone/verify` | Use seeded sessions for verify; send uses deterministic/fake SMS flow. |

## Data seeding strategy

- `perfseed` connects to the perf database via `PERF_DATABASE_URL`.
- Truncates tables in dependency order or uses `CASCADE`.
- Inserts `PERF_OWNERS` (default 1,000) owners, each with a fixed set of properties, leases, operations, recurring operations, reminders, and tenant contacts.
- Creates one active session per owner with deterministic `session_id` so k6 can derive the cookie from the owner index without database access.
- Per-endpoint seeding: some endpoints need extra fixtures (e.g., many reminders for `delete_reminder`). `perfseed --endpoint=<name>` can add endpoint-specific fixtures on top of the base seed.

## Auth/session strategy

- No SMS flow during benchmark iterations.
- k6 receives a list of `(owner_index, session_id)` pairs or derives `session_id` deterministically from `owner_index`.
- Cookie jar is used per VU; each VU picks owners round-robin or randomly from the pool.
- The `auth_verify_code` endpoint is tested with seeded SMS codes that have extended TTL (or expiry check is bypassed in perf mode via env flag `AUTH_PERF_CODE`).

## k6 script design

- Single script: `apps/backend/perf/scripts/endpoint_benchmark.js`.
- Executor: `constant-arrival-rate` to maintain a target RPS independent of response time.
- Options injected via env vars:
  - `ENDPOINT` — endpoint key from manifest.
  - `RATE` — target iterations per second.
  - `DURATION` — benchmark duration.
  - `PRE_ALLOCATED_VUS`, `MAX_VUS` — VU pool.
  - `API_BASE_URL` — backend base URL.
- Tags:
  - `name` set to endpoint key so k6 groups metrics correctly.
  - `endpoint` tag for filtering.
- Thresholds:
  - `'http_req_duration{expected_response:true}': ['p(95)<500']`
  - `'http_req_failed': ['rate<0.01']`
- `handleSummary` writes machine-readable `summary.json` and prints a human-readable `textSummary` to stdout.

## Max RPS discovery algorithm

`perfmaxrps` uses a search loop per endpoint:

1. Start with `START_RATE` and `MAX_RATE` bounds.
2. Run k6 at current candidate rate for `DURATION`.
3. Parse `summary.json`: check `http_req_duration{expected_response:true}.p(95)` and `http_req_failed.rate`.
4. If both pass, increase rate; if fail, decrease rate.
5. Stop when the interval is smaller than `RATE_STEP` or max iterations reached.
6. Record the highest passing rate, p95, error rate, and raw summary path.

Exit code is non-zero if any endpoint in the suite fails to find a passing rate or crashes, so CI can gate on it later.

## Infrastructure

- `docker-compose.perf.yml`: PostgreSQL `17` (or `17-alpine`) on a dedicated port/volume (`POSTGRES_PORT` default 5433), isolated from local dev DB.
- `.env.perf.example`: placeholders only (`ENCRYPTION_KEY=<256-bit-hex-key>`), no committed secrets.
- Backend runs against perf DB via `PERF_DATABASE_URL`.
- k6 runs in Docker (`grafana/k6:latest` pinned to a recent stable tag) or locally if `k6` is installed.

## Makefile targets

```makefile
perf-db-up          # start perf PostgreSQL
perf-db-down        # stop perf PostgreSQL
perf-db-reset       # drop and recreate perf DB volume
perf-seed           # run perfseed base fixtures
perf-endpoint-max-rps ENDPOINT=...  # benchmark single endpoint
perf-endpoints-max-rps              # benchmark all endpoints, write report
perf-results-clean  # remove old run artifacts
```

## Reporting

- Per-run directory: `apps/backend/perf/results/endpoints/<run-id>/`.
- `report.md`: table with endpoint, class, max RPS, p95, error rate, status.
- `results.json`: machine-readable array of endpoint results.
- Raw `summary.json` from each k6 run preserved for deep analysis.

## Error handling and preconditions

- `perfmaxrps` checks PostgreSQL and backend health before running.
- Invalid env vars cause a fatal error instead of silent fallback.
- If `RATE * DURATION` exceeds a fixture budget for a growth/lifecycle endpoint, `perfmaxrps` logs a warning and skips or caps the run.
- `perfmaxrps -suite` returns non-zero exit code on any failed endpoint.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Fixture exhaustion on destructive endpoints | Re-seed per endpoint; seed many fixtures for lifecycle operations; explicit-only for destructive endpoints. |
| p95 skewed by background workers | Documented as realistic load; results should be interpreted as "with normal background activity". |
| Domain date constraints cause 400s | Verify payloads against domain rules during implementation; keep date ranges valid. |
| k6 summary format changes | Pin k6 image version; parse summary defensively; support both old and new machine-readable formats. |
| PostgreSQL image version | Use `postgres:17` stable. |

## Implementation phases

1. **Bootstrap infrastructure**: `docker-compose.perf.yml`, `.env.perf.example`, Makefile targets.
2. **Implement `cmd/perfseed`**: base fixtures + per-endpoint fixtures + session seeding.
3. **Implement `perf/scripts/endpoint_benchmark.js`**: constant-arrival-rate, tagging, thresholds, summary output.
4. **Implement `cmd/perfmaxrps`**: single-endpoint binary search, summary parsing, report writing.
5. **Add suite orchestration**: all endpoints, re-seed per endpoint, lifecycle handling.
6. **Validate**: run against local backend, fix fixture/payload issues, finalize report format.
7. **Documentation**: design doc in `docs/plans/`, README in `apps/backend/perf/`, CHANGELOG entry.

## Open questions

1. Exact domain date constraints for `patch_lease`, `patch_operation`, `patch_recurring_operation`.
2. Whether `pause`/`resume` recurring operation are idempotent (or need paired fixture usage).
3. Optimal default values for `PERF_OWNERS`, `START_RATE`, `MAX_RATE`, `DURATION`, `RATE_STEP` after a baseline run.
