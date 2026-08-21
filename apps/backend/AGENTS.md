# apps/backend/AGENTS.md

## Scope

Rules for the Go backend in `apps/backend`. Also follow the root `AGENTS.md`, the relevant per-context `CONTEXT.md` (index in `CONTEXT-MAP.md`), relevant product docs, and ADRs.

## Mandatory Backend Tools

Verify the following skill and MCP server before any backend work.

- **`use-modern-go` skill** — must be invoked before any backend planning or implementation. It detects the project's Go version from `go.mod` and instructs to use modern Go idioms up to and including that version. Do not write Go code without first invoking this skill.
- **`serena` MCP server** — must be active, with its `mcp__serena__*` tools present in the agent tool set, before any backend implementation or verification. It provides semantic navigation, symbol references, rename, diagnostics, and symbol-level editing for Go through one project at the repo root (`.serena/project.yml`).

If the `mcp__serena__*` tools are not available:
- Stop backend work immediately.
- Tell the user that `serena` is required and unavailable (the user can check server status with `/mcp`).
- Do not continue with implementation, lint, or test commands until `serena` is running.

## Stack & References

- Target Go: 1.26.
- For Go syntax, semantics, packages, modules, and workspace decisions, use official Go documentation first: `https://go.dev/ref/spec`, `https://go.dev/doc/modules/layout`, `https://go.dev/doc/tutorial/workspaces`, and `pkg.go.dev`.
- Official Go tools and docs (`go`, `gopls`, `go test`, `go vet`, `go fix`, `pkg.go.dev`, and `go.dev`) take precedence over any Go skill or model memory.
- Do not install or require community Go/Golang skills automatically. If a future official Go-team, OpenAI-curated, or clearly verified vendor Go skill appears, review its source repository, license, and contents before installing or making it mandatory.
- Before relying on non-obvious third-party behavior or changing third-party integrations/configuration, use `context7` for current docs. For security-sensitive behavior such as credentials, auth, payments, storage, Docker, and CI/CD, verify against official vendor docs.

## Required Skills

Invoke skills by their exact name through the harness's native skill mechanism.

- For all Go backend work, invoke `go`. Use it for idiomatic Go, clean architecture, context propagation, error handling, and review of package boundaries.
- Before writing or reviewing any Go code, invoke `use-modern-go` to detect the target Go version from `go.mod` and apply modern idioms up to that version.
- For database schema, migrations, SQL, sqlc queries, indexes, transactions, locks, or financial invariants, invoke `postgresql-best-practices`.
- For Dockerfiles, Docker Compose, image security, container health checks, or container build/runtime behavior, invoke `docker`.
- These skills support the repository rules; official Go/PostgreSQL/Docker/vendor documentation and project ADRs remain authoritative when there is a conflict.

## MCP Servers

- `serena` — mandatory for every backend task (see stop-procedure in Mandatory Backend Tools above). Treat it as navigation and diagnostics, not as the source of truth — the source of truth is the repository code plus `go test`, `go vet`, `make backend-lint`, and relevant official docs. Use `get_diagnostics_for_file` as a required quality gate before claiming backend work complete. Go contract note: a symbol's body excludes the leading doc comment — retrieve with `find_symbol` + `include_body` before `replace_symbol_body`, or the comment gets duplicated.
- `lean-ctx` — read/output compression, semantic search, dependency graph, session intelligence: broad package exploration, generated code maps, large SQL/OpenAPI files, and noisy command output. Code semantics (symbols, references, rename, diagnostics) is Serena's, not lean-ctx's. Before editing exact Go code, migrations, SQL, or OpenAPI, read the target ranges in raw/full form.

## Backend Workflow

Follow the workflow from the root `AGENTS.md`. For backend tasks, additionally:

1. **Before exploration** — invoke `use-modern-go` so the target Go version and modern idioms are known.
2. **Before implementation** — verify the `mcp__serena__*` tools are available (see Mandatory Backend Tools for the stop-procedure).
3. **During implementation** — apply modern idioms from `use-modern-go` to every new or changed Go file.
4. **Before final verification** — run `serena` diagnostics (`get_diagnostics_for_file`) on changed files and fix reported issues before running `make backend-lint`, `go test`, or `go vet`.

## Coding Standards

Before implementing or reviewing backend code, read `CODING_STANDARDS.md` (same directory): architecture inside a bounded context, error and concurrency conventions (handle an error once, exit-политика), domain constructor validation and static port assertions, testing patterns, and the review rubric used by the Standards axis of `/code-review`.

## Architecture Rules

- Use DDD, Clean Architecture, layered architecture, clean code, and idiomatic Go.
- Keep the backend a DDD modular monolith until an ADR records a real reason to split services.
- Current bounded contexts under `internal` include `identity`, `properties`, `leases`, `billing`, `notifications`, and `platform`. Add new contexts according to docs, glossary, and ADR boundaries.
- `internal/audit` is the audit log module: `domain` holds the `Entry` model and the action registry, `application` exposes the `Recorder` port, `adapters/postgres` writes entries. Audit records go in the business operation's transaction (fail-safe: an insert error rolls the operation back); see `docs/adr/0020-audit-log.md`.
- Layer direction is inward only: transport/adapters → application → domain.
- Domain packages contain business language and rules only. They must not import HTTP, OpenAPI generated types, `pgx`, `sqlc`, `database/sql`, config, or adapters (enforced by depguard `domain-clean` in `.golangci.yml`).
- Application packages own use cases, ports, orchestration, transaction boundaries, and calls into domain code; their import boundaries are enforced by depguard `application-clean` in `.golangci.yml`.
- Adapter/platform packages own HTTP, persistence, config, logging, external services, and generated code.
- Keep packages small, names explicit, errors intentional, and dependencies boring. Prefer simple Go over clever abstractions.

## API And Persistence

- API is contract-first: change `api/openapi/openapi.yaml` before changing HTTP behavior.
- Generated OpenAPI DTOs stay at the HTTP edge and must be mapped explicitly to application/domain models.
- Regenerate from `apps/backend` after API/schema query changes (freshness enforced by `make backend-openapi-check` / `make backend-sqlc-check` in `.github/workflows/ci.yml`):

```bash
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.1 -config api/openapi/oapi-codegen.yaml api/openapi/openapi.yaml
```

- The property attributes catalog (`internal/properties/domain` field maps, `ValidateAttributes`, `crossFieldErrors`, …) is generated from `tools/property-attributes/catalog.json`. Regenerate with `make attributes-gen` (or `cd tools/property-attributes && npm run generate`); the gate `make attributes-check` fails in CI if a `catalog.json` change was not committed with its regenerated artifacts.
- Do not hand-edit generated files (`spec.gen.go`, sqlc output, `zz_catalog.gen.go`); change the OpenAPI contract, SQL, migrations, `catalog.json`, or generator configuration, then regenerate (freshness enforced by `make backend-tkassa-spec-check` / `make backend-openapi-check` / `make backend-sqlc-check` / `make attributes-check` in `.github/workflows/ci.yml`).
- Use explicit PostgreSQL SQL with `sqlc`; do not introduce ORM models (enforced by depguard `no-orm` in `.golangci.yml`).
- `os.Exit`/`log.Fatal*` only in `cmd/` — internal packages return errors upward (exit-политика, enforced by forbidigo in `.golangci.yml`; prose in `CODING_STANDARDS.md`, bar #323).
- `github.com/stretchr/testify` is test-only: production code asserts with `errors.Is/As` and domain constructors (enforced by depguard `testify-test-only` in `.golangci.yml`).
- Every goroutine has an owner responsible for its exit; the two long-lived scheduler test binaries (`internal/platform/scheduler`, `internal/identity/adapters/scheduler`) fail their run on any goroutine that outlives the tests (goleak `VerifyTestMain` in `TestMain`, registry entry in `docs/agents/tooling.md`, #352; prose in `CODING_STANDARDS.md`).
- Schema changes require versioned migrations in `db/migrations` and matching queries in `db/queries`. Migrations are linted for lock-safety and the domain rules above (enforced by `make migrations-lint` — pre-commit on staged migrations, pre-push and the CI `migrations-lint` job on the full pass; config: `apps/backend/.squawk.toml`).
- Keep database invariants in PostgreSQL with `NOT NULL`, foreign keys, `CHECK` constraints, indexes, and triggers where they protect durable rules.
- Authorization goes through the policy port (`internal/shared/policy.Policy`), the single point that maps an actor and a data owner (scope) to a role. Owner-scoped repository queries filter by the data owner (`scope`), not by the actor; membership is resolved by the policy port, not in SQL. See ADR 0028 (`docs/adr/0028-object-data-access-model.md`).
- Application-layer services receive `actor` (the operation initiator) and thread `scope` (the data owner) into repository calls. For the owner's own data `actor == scope`. Do not reintroduce a bare `ownerID` parameter that conflates the two; the split is the seam for property sharing (T3).
- Use `date` for domain dates, `timestamptz` for system timestamps, and `BIGINT` (kopecks) for money. `numeric`/`decimal`/`real`/`double`/`float` column types are banned in migrations outright — money columns are structurally indistinguishable, so the ban is total (enforced by `make migrations-lint`, `tools/migration-lint/domain-rules.mjs`; lock-safety rules enforced by squawk, `apps/backend/.squawk.toml`).
- Money arithmetic is integer-only (`int64` kopecks); never use floating-point for money at any layer.
- Identifiers are UUIDv7, generated in the application via `uuid.NewV7()`; `id` columns have no `DEFAULT` in the database (enforced by `make migrations-lint`, `tools/migration-lint/domain-rules.mjs`). Set `id` explicitly in new migrations and seeds (in SQL, use PostgreSQL 18 `uuidv7()`). See `docs/adr/0019-uuid-v7-app-generated-ids.md`. `uuid.New()`/`uuid.NewV4()`/`uuid.NewString()`/`uuid.NewRandom()`/`uuid.NewRandomFromReader()` are banned at every layer, tests included (enforced by forbidigo in `.golangci.yml`); the panicking sites use `uuid.Must(uuid.NewV7())`.
- Browser auth uses opaque server-side sessions with `HttpOnly` cookies. Do not replace this with browser-readable JWT/session storage without a new ADR.
- Store property photos through a storage port backed by REG.RU S3-compatible storage. Do not add MinIO as a local dependency (enforced by depguard `no-minio` in `.golangci.yml`).

## Observability

Backend observability code conventions (slog, OpenTelemetry, request IDs, span naming, low-cardinality labels, PII redaction) — see `docs/backend-observability.md`. Infrastructure (Uptrace, Vector) — see `docs/deployment.md`, section "Observability".

## Quality Gates

- Code must be gofumpt-clean with gci import order (enforced by `make backend-lint`); autofix with `cd apps/backend && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 fmt` (config `apps/backend/.golangci.yml`, auto-discovered from the module dir).
- Before claiming backend work is complete, run `serena` diagnostics (`get_diagnostics_for_file`) on changed files and resolve reported issues.
- Then run the relevant checks, including existing tests:

```bash
make backend-lint
make backend-test
cd apps/backend && go vet ./...
```

For changes that touch adapters, repositories, or DB queries, also run:

```bash
make backend-test-integration
```

This uses testcontainers-go (requires Docker) to start a dedicated PostgreSQL 18
container per test binary. Alternatively, use an external database via
`make test-infra-up` then `TEST_DATABASE_URL=... make backend-test-integration`.

CI backstop: the `backend` job runs the unit suite and the `backend-integration`
job runs the same `-tags=integration -race` suite (testcontainers via the mounted
Docker socket) in `.github/workflows/ci.yml`.
