# apps/backend/AGENTS.md

## Scope

Rules for the Go backend in `apps/backend`. Also follow the root `AGENTS.md`, `CONTEXT.md`, relevant product docs, and ADRs.

## Mandatory Backend Tools

The following skill and MCP server are mandatory for every backend task. The Orchestrator must verify them before dispatching implementation subagents.

- **`use-modern-go` skill** — must be invoked through the Kimi `Skill` tool before any backend planning or implementation. It detects the project's Go version from `go.mod` and instructs subagents to use modern Go idioms up to and including that version. Do not write Go code without first invoking this skill.
- **`gopls` MCP server** — must be active, with its `mcp__gopls__*` tools present in the agent tool set, before any backend implementation or verification. It provides semantic navigation, diagnostics, definitions, references, and workspace analysis.

If the `mcp__gopls__*` tools are not available:
- Stop backend work immediately.
- Tell the user that `gopls` is required and unavailable (the user can check server status with `/mcp`).
- Do not continue with implementation, lint, or test commands until `gopls` is running.

## Stack & References

- Target Go: 1.26.
- For Go syntax, semantics, packages, modules, and workspace decisions, use official Go documentation first: `https://go.dev/ref/spec`, `https://go.dev/doc/modules/layout`, `https://go.dev/doc/tutorial/workspaces`, and `pkg.go.dev`.
- Official Go tools and docs (`go`, `gopls`, `go test`, `go vet`, `go fix`, `pkg.go.dev`, and `go.dev`) take precedence over any Go skill or model memory.
- Do not install or require community Go/Golang skills automatically. If a future official Go-team, OpenAI-curated, or clearly verified vendor Go skill appears, review its source repository, license, and contents before installing or making it mandatory.
- Before relying on non-obvious third-party behavior or changing third-party integrations/configuration, use `context7` for current docs. For security-sensitive behavior such as credentials, auth, payments, storage, Docker, and CI/CD, verify against official vendor docs.

## Required Skills

Invoke skills through the Kimi `Skill` tool using the exact skill name.

- For all Go backend work, invoke `go`. Use it for idiomatic Go, clean architecture, context propagation, error handling, and review of package boundaries.
- Before writing or reviewing any Go code, invoke `use-modern-go` to detect the target Go version from `go.mod` and apply modern idioms up to that version.
- For database schema, migrations, SQL, sqlc queries, indexes, transactions, locks, or financial invariants, invoke `postgresql-best-practices`.
- For deployment, CI/CD, runtime configuration, infrastructure, observability rollout, production operations, or Docker Compose changes, invoke `devops-engineer`.
- For Dockerfiles, Docker Compose, image security, container health checks, or container build/runtime behavior, invoke `docker`.
- These skills support the repository rules; official Go/PostgreSQL/Docker/vendor documentation and project ADRs remain authoritative when there is a conflict.

## MCP Servers

- `gopls` — **mandatory** for every backend task. Use it for Go semantic navigation, definitions, references, diagnostics, package APIs, and impact checks. Treat `gopls` as a navigation and diagnostics tool, not as the source of truth. The source of truth is the repository code plus `go test`, `go vet`, `make backend-lint`, generated code checks, and relevant official docs.
  - Before starting backend implementation, verify the `mcp__gopls__*` tools are available in the agent tool set.
  - If the `mcp__gopls__*` tools are not available, stop and tell the user. Do not continue implementation, lint, or tests until `gopls` is running.
  - Use `gopls` diagnostics as a required quality gate before claiming backend work complete.
- `lean-ctx` — use for broad package exploration, generated code maps, large SQL/OpenAPI files, and noisy command output. Before editing exact Go code, migrations, SQL, or OpenAPI, read the target ranges in raw/full form.
- `context7` — use for current official docs on third-party libraries when needed.

Before adding new interfaces, repositories, DTO mappings, application services, domain services, or use cases, search existing backend patterns with `Grep`/`lean-ctx` and inspect semantic references with `gopls`.

## Backend Workflow

Follow the Orchestrator Mode from the root `AGENTS.md`. For backend tasks, the Orchestrator additionally:

1. **Before exploration** — invoke `use-modern-go` so the target Go version and modern idioms are known to all subagents.
2. **Before implementation** — verify the `mcp__gopls__*` tools are available. If they are not, stop and report to the user.
3. **During implementation** — ensure the coder subagent applies modern idioms from `use-modern-go` to every new or changed Go file.
4. **Before final verification** — run `gopls` diagnostics on changed packages and fix reported issues before running `make backend-lint`, `go test`, or `go vet`.

## Architecture Rules

- Use DDD, Clean Architecture, layered architecture, clean code, and idiomatic Go.
- Keep the backend a DDD modular monolith until an ADR records a real reason to split services.
- Current bounded contexts under `internal` include `identity`, `properties`, `leases`, `billing`, `notifications`, and `platform`. Add new contexts according to docs, glossary, and ADR boundaries.
- `internal/audit` is the audit log module: `domain` holds the `Entry` model and the action registry, `application` exposes the `Recorder` port, `adapters/postgres` writes entries. Audit records go in the business operation's transaction (fail-safe: an insert error rolls the operation back); see `docs/adr/0020-audit-log.md`.
- Layer direction is inward only: transport/adapters → application → domain.
- Domain packages contain business language and rules only. They must not import HTTP, OpenAPI generated types, `pgx`, `sqlc`, `database/sql`, config, or adapters.
- Application packages own use cases, ports, orchestration, transaction boundaries, and calls into domain code.
- Adapter/platform packages own HTTP, persistence, config, logging, external services, and generated code.
- Keep packages small, names explicit, errors intentional, and dependencies boring. Prefer simple Go over clever abstractions.

## API And Persistence

- API is contract-first: change `api/openapi/openapi.yaml` before changing HTTP behavior.
- Generated OpenAPI DTOs stay at the HTTP edge and must be mapped explicitly to application/domain models.
- Regenerate from `apps/backend` after API/schema query changes:

```bash
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.1 -config api/openapi/oapi-codegen.yaml api/openapi/openapi.yaml
```

- Do not hand-edit generated files; change the OpenAPI contract, SQL, migrations, or generator configuration, then regenerate.
- Use explicit PostgreSQL SQL with `sqlc`; do not introduce ORM models.
- Schema changes require versioned migrations in `db/migrations` and matching queries in `db/queries`.
- Keep database invariants in PostgreSQL with `NOT NULL`, foreign keys, `CHECK` constraints, indexes, and triggers where they protect durable rules.
- Use `date` for domain dates, `timestamptz` for system timestamps, and `BIGINT` (kopecks) for money.
- Identifiers are UUIDv7, generated in the application via `uuid.NewV7()`; `id` columns have no `DEFAULT` in the database. Set `id` explicitly in new migrations and seeds (in SQL, use PostgreSQL 18 `uuidv7()`). See `docs/adr/0019-uuid-v7-app-generated-ids.md`.
- Browser auth uses opaque server-side sessions with `HttpOnly` cookies. Do not replace this with browser-readable JWT/session storage without a new ADR.
- Store property photos through a storage port backed by REG.RU S3-compatible storage. Do not add MinIO as a local dependency.

## Observability

- Use standard-library `log/slog` with JSON output for backend logs. Prefer `InfoContext`, `WarnContext`, and `ErrorContext` so request-scoped context can carry correlation fields.
- Keep logging middleware, request IDs, OpenTelemetry setup, exporters, and metrics in `cmd/api` or `internal/platform`. Domain packages must not import loggers, OpenTelemetry, HTTP middleware, or platform observability helpers.
- Every HTTP request must have an `X-Request-ID`. Preserve an incoming request ID when present; otherwise generate one. Include the same value in `Problem.requestId`, the response `X-Request-ID` header, and request outcome logs.
- Request outcome logs must include low-cardinality operational fields: `request_id`, method, route or operation, status, duration, and problem/error code when available.
- Do not log session tokens, cookies, authorization headers, S3 secrets, raw DaData payloads, raw request/response bodies, or other PII/secrets. Do not log SMS codes or full phone numbers in production integrations. The local/dev fake SMS sender is the only exception: it may log the SMS code and phone number so developers can complete manual login without a real SMS provider.
- OpenTelemetry is the preferred tracing/metrics foundation. Start with HTTP, PostgreSQL/pgx, S3, and external API spans/metrics; keep exporters vendor-neutral through OTLP-compatible configuration. Do not add a concrete telemetry backend, Collector topology, production alerts, or SLO policy without a separate ADR.
- Keep metric labels low-cardinality. Do not use user IDs, property IDs, phone numbers, addresses, object storage keys, or raw paths as labels.
- OpenTelemetry Logs are not a required backend signal for now; structured `slog` stdout logs are the logging source of truth until an ADR changes that.
- The audit log is not an observability log: business-audit records (who did what) persist to the `audit_log` table via `internal/audit` and are viewed in the admin panel, while structured `slog` stdout logs remain the source of truth for technical/operational logging. See `docs/adr/0020-audit-log.md`.

## Quality Gates

- Do not write new tests or use TDD unless the user explicitly asks for them.
- Before claiming backend work is complete, run `gopls` diagnostics on changed packages and resolve reported issues.
- Then run the relevant checks, including existing tests:

```bash
make backend-lint
cd apps/backend && go test ./...
cd apps/backend && go vet ./...
```
