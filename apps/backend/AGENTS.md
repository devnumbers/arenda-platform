# apps/backend/AGENTS.md

## Scope

Rules for the Go backend in `apps/backend`. Also follow the root `AGENTS.md`, the relevant per-context `CONTEXT.md` (index in `CONTEXT-MAP.md`), relevant product docs, and ADRs.

## Mandatory Backend Tools

Verify before any backend work:

- **`use-modern-go` skill** — invoke before any backend planning or implementation. It detects the target Go version from `go.mod` and instructs to use modern Go idioms up to and including that version. Target Go version itself: `GO_VERSION` in the root Makefile (single source).
- **`serena` MCP server** — must be active, with its `mcp__serena__*` tools present, before any backend implementation or verification (fallback rules: `docs/agents/mcp.md`). If the tools are not available, stop backend work immediately, tell the user, and do not continue until `serena` is running.

Go-specific serena rules:

- Treat serena as navigation and diagnostics, not the source of truth — the source of truth is the repository code plus `go test`, `go vet`, `make backend-lint`, and official docs.
- `get_diagnostics_for_file` is a required quality gate on changed files before claiming backend work complete.
- A symbol's body excludes the leading doc comment — retrieve with `find_symbol` + `include_body` before `replace_symbol_body`, or the comment gets duplicated.

## Stack & Skills

- For Go syntax, semantics, packages, modules, and workspace decisions, official Go documentation wins: `https://go.dev/ref/spec`, `https://go.dev/doc/modules/layout`, `https://go.dev/doc/tutorial/workspaces`, `pkg.go.dev` — over any Go skill or model memory. Do not install community Go skills automatically; review source, license, and contents first.
- Skills to invoke by exact name: `go` (idiomatic Go, clean architecture, context propagation, error handling, package boundaries), `use-modern-go` before writing or reviewing any Go code, `postgresql-best-practices` (schema, migrations, SQL, sqlc queries, indexes, transactions, locks, financial invariants), `docker` (Dockerfiles, compose, image security, health checks). Skills support the repository rules; official vendor docs and project ADRs stay authoritative.
- Before relying on non-obvious third-party behavior or changing third-party integrations, use `context7` for current docs; security-sensitive behavior (credentials, auth, payments, storage, Docker, CI/CD) verify against official vendor docs.

## Backend Workflow

1. **Before exploration** — invoke `use-modern-go` so the target Go version and modern idioms are known.
2. **Before implementation** — verify the `mcp__serena__*` tools are available (see above for the stop procedure).
3. **During implementation** — apply the modern idioms to every new or changed Go file.
4. **Before final verification** — run serena diagnostics on changed files and fix reported issues, then run the quality gates below.

Before implementing or reviewing backend code, read `CODING_STANDARDS.md` (same directory): architecture inside a bounded context, error and concurrency conventions (handle an error once, the exit policy), domain constructor validation, static port assertions, testing patterns, and the review rubric used by the Standards axis of `/code-review`.

## Architecture Rules

- Use DDD, Clean Architecture, layered architecture, clean code, and idiomatic Go. Keep the backend a DDD modular monolith until an ADR records a real reason to split services.
- Bounded contexts live in `internal/<context>/`, each with its own `CONTEXT.md`; the live index is `CONTEXT-MAP.md`. Add new contexts per the checklist in `CODING_STANDARDS.md`.
- Layer direction is inward only: transport/adapters → application → domain. Domain packages contain business language and rules only — no HTTP, OpenAPI generated types, `pgx`, `sqlc`, `database/sql`, config, or adapters (depguard `domain-clean` in `.golangci.yml`). Application packages own use cases, ports, orchestration, transaction boundaries (depguard `application-clean`); adapter/platform packages own HTTP, persistence, config, logging, external services, and generated code.
- Keep packages small, names explicit, errors intentional, and dependencies boring. Prefer simple Go over clever abstractions.

## API And Persistence

- API is contract-first: change `api/openapi/openapi.yaml` before changing HTTP behavior. Generated OpenAPI DTOs stay at the HTTP edge and must be mapped explicitly to application/domain models.
- Regenerate after API/schema query changes — use the exact commands `make backend-sqlc-check` / `make backend-openapi-check` print when they fail (tool pins live in the root Makefile). Do not hand-edit generated files (`spec.gen.go`, sqlc output, `zz_catalog.gen.go`, `zz_categories.gen.go`); change the contract, SQL, migrations, or `catalog.json`, then regenerate — freshness gates: `make backend-tkassa-spec-check` / `make backend-openapi-check` / `make backend-sqlc-check` / `make attributes-check` / `make categories-check`. In sqlc output the `”` (U+201D) in `.sql.go` comments is sqlc's rendering of the `''` pair from the source query comment — a generate form with no behavior impact: do not raise it in review and do not hand-fix it.
- The property attributes catalog (`internal/properties/domain` field maps, `ValidateAttributes`, `crossFieldErrors`, …) is generated from `tools/property-attributes/catalog.json` (`make attributes-gen`); the default payment categories (`internal/payments/domain/zz_categories.gen.go`) follow the same pattern from `tools/payment-categories/catalog.json` (`make categories-gen`).
- Use explicit PostgreSQL SQL with `sqlc`; do not introduce ORM models (depguard `no-orm`). Schema changes require versioned migrations in `db/migrations` and matching queries in `db/queries`; migrations are linted for lock-safety and domain rules (`make migrations-lint`, config `apps/backend/.squawk.toml`). Keep database invariants in PostgreSQL with `NOT NULL`, foreign keys, `CHECK` constraints, indexes, and triggers where they protect durable rules.
- Column types: `date` for domain dates, `timestamptz` for system timestamps, `BIGINT` (kopecks) for money; `numeric`/`decimal`/`real`/`double`/`float` are banned in migrations outright (money columns are structurally indistinguishable — the ban is total; enforced by `make migrations-lint`).
- Identifiers are UUIDv7, generated in the application via `uuid.NewV7()`; `id` columns have no `DEFAULT` (enforced by `make migrations-lint`). Set `id` explicitly in new migrations and seeds (in SQL, PostgreSQL 18 `uuidv7()`). `uuid.New()`/`uuid.NewV4()`/`uuid.NewString()`/`uuid.NewRandom()`/`uuid.NewRandomFromReader()` are banned at every layer, tests included (forbidigo); panicking sites use `uuid.Must(uuid.NewV7())`. See `docs/adr/0019-uuid-v7-app-generated-ids.md`.
- `os.Exit`/`log.Fatal*` only in `cmd/` — internal packages return errors upward (the exit policy, enforced by forbidigo; prose in `CODING_STANDARDS.md`).
- `github.com/stretchr/testify` is test-only: production code asserts with `errors.Is/As` and domain constructors (depguard `testify-test-only`).
- Authorization goes through the policy port (`internal/shared/policy.Policy`), the single point mapping an actor and a data owner (scope) to a role (ADR 0028). Owner-scoped repository queries filter by the data owner (`scope`), not by the actor; membership is resolved by the policy port, not in SQL. The exceptions are actor-scoped cross-property reads where no single scope exists to resolve: the tasks global list (ADR 0052 decision 3; `db/queries/tasks_tasks.sql`), the owner's-participant aggregate — its read scope and removal scope (issues #693, #694; `db/queries/participants.sql`) — where the active full_access manage-scope predicate is called by that read's SQL — the SQL function `actor_can_manage` (migration 000135, issue #794); the scope is the authorization — and the realtime frame audience (ADR 0062 §4; `db/queries/realtime.sql`): the publication resolves the object's whole derived read access (owner + active members, ADR 0028) at the moment of publish, the audience being the read's very subject.
- Application services receive `actor` (the operation initiator) and thread `scope` (the data owner) into repository calls; for the owner's own data `actor == scope`. Do not reintroduce a bare `ownerID` conflating the two — the split is the seam for property sharing.
- Browser auth uses opaque server-side sessions with `HttpOnly` cookies. Do not replace this with browser-readable JWT/session storage without a new ADR.
- Store property photos through a storage port backed by REG.RU S3-compatible storage; do not add MinIO as a local dependency (depguard `no-minio`).

## Observability

Backend observability code conventions (slog, OpenTelemetry, request IDs, span naming, low-cardinality labels, PII redaction) — `docs/backend-observability.md`. Infrastructure (Uptrace, Vector) — `docs/deployment.md`, section "Observability".

## Quality Gates

- Code must be gofumpt-clean with gci import order (`make backend-lint`); autofix with `fmt` from the same pinned golangci-lint binary (config `apps/backend/.golangci.yml`, auto-discovered from the module dir).
- Then run `make backend-lint`, `make backend-test`, and `cd apps/backend && go vet ./...`. For changes touching adapters, repositories, or DB queries, also `make backend-test-integration` — testcontainers-go starts a dedicated PostgreSQL 18 container per test binary (requires Docker), or use an external database via `make test-infra-up` then `TEST_DATABASE_URL=... make backend-test-integration`.
