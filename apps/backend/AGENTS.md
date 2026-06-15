# apps/backend/AGENTS.md

## Scope

Rules for the Go backend in `apps/backend`. Also follow the root `AGENTS.md`, `CONTEXT.md`, relevant product docs, and ADRs.

## References

- Target Go: 1.26.
- For Go backend work, invoke the local `$go` skill when it is available. Treat it as project help, not as an authority above official Go documentation.
- For Go syntax, semantics, packages, modules, and workspace decisions, use official Go documentation first: `https://go.dev/ref/spec`, `https://go.dev/doc/modules/layout`, `https://go.dev/doc/tutorial/workspaces`, and `pkg.go.dev`.
- Official Go tools and docs (`go`, `gopls`, `go test`, `go vet`, `go fix`, `pkg.go.dev`, and `go.dev`) take precedence over any Go skill or model memory.
- Do not install or require community Go/Golang skills automatically. If a future official Go-team, OpenAI-curated, or clearly verified vendor Go skill appears, review its source repository, license, and contents before installing or making it mandatory.
- Before relying on non-obvious third-party behavior or changing third-party integrations/configuration, use Context7 for current docs. For security-sensitive behavior such as credentials, auth, payments, storage, Docker, and CI/CD, verify against official vendor docs.

## Required Backend Skills

- For all Go backend work, invoke `$go`. Use it for idiomatic Go, clean architecture, context propagation, error handling, tests, and review of package boundaries.
- For database schema, migrations, SQL, sqlc queries, indexes, transactions, locks, or financial invariants, invoke `$postgresql-best-practices`.
- For deployment, CI/CD, runtime configuration, infrastructure, observability rollout, production operations, or Docker Compose changes, invoke `$devops-engineer`.
- For Dockerfiles, Docker Compose, image security, container health checks, or container build/runtime behavior, invoke `$docker`.
- These skills support the repository rules; official Go/PostgreSQL/Docker/vendor documentation and project ADRs remain authoritative when there is a conflict.

## Architecture Rules

- Use DDD, Clean Architecture, layered architecture, clean code, and idiomatic Go.
- Keep the backend a DDD modular monolith until an ADR records a real reason to split services.
- Current bounded contexts under `internal` include `identity`, `properties`, `billing`, and `platform`. Add new contexts according to docs, glossary, and ADR boundaries.
- Layer direction is inward only: transport/adapters -> application -> domain.
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
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.0 -config api/openapi/oapi-codegen.yaml api/openapi/openapi.yaml
```

- Do not edit generated files manually.
- Use explicit PostgreSQL SQL with `sqlc`; do not introduce ORM models.
- Schema changes require versioned migrations in `db/migrations` and matching queries in `db/queries`.
- Keep database invariants in PostgreSQL with `NOT NULL`, foreign keys, `CHECK` constraints, indexes, and triggers where they protect durable rules.
- Use `date` for domain dates, `timestamptz` for system timestamps, and `numeric(14,2)` for money.
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

## Testing

- Write backend changes through TDD: first add or adjust the smallest failing test that proves the core behavior, then implement the minimal production code to pass it.
- Keep tests to the necessary minimum. Cover main business logic, durable invariants, and previously broken behavior; do not add broad permutation tests, snapshot-style tests, or edge-case matrices unless they protect real application behavior.
- Test domain rules first with fast table-driven unit tests.
- Test application services with fakes for clocks, SMS, DaData, storage, and repositories only when the use case orchestration is the behavior being changed.
- Use repository/API integration tests only when migrations, SQL, sessions, HTTP contracts, or generated transport behavior are part of the change.
- Real REG.RU S3 upload tests are manual/separate because they require real credentials.
- Before claiming backend work is complete, run the relevant checks:

```bash
make backend-lint
cd apps/backend && go test ./...
cd apps/backend && go vet ./...
```
