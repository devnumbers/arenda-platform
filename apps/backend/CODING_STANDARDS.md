# apps/backend/CODING_STANDARDS.md

How Go code in this backend is written and reviewed. Read before implementing or reviewing a backend change; the Standards axis of `/code-review` diffs against this file.

Not duplicated here — single sources of truth elsewhere:

- Tool-enforced invariants (money kopecks, UUIDv7, layer imports, no ORM, no stdlib log): `AGENTS.md` (same directory) + `.golangci.yml` + `make migrations-lint`. If lint catches it, review does not re-report it.
- Domain language: per-context `CONTEXT.md` (index in `CONTEXT-MAP.md`). Decisions: `docs/adr/`.
- Logging, tracing, span, and PII conventions: `docs/backend-observability.md` (slog `*Context` variants are enforced by `sloglint`).

## Architecture inside a bounded context

Canonical layout of a context (`internal/<context>/`):

- `domain/` — types and rules in domain language. Pure Go: no I/O, no clocks, no infrastructure (depguard `domain-clean` enforces imports; it cannot enforce that logic actually lives here — see review rubric).
- `application/` — one service per cohesive area, one method per use case. The service owns orchestration, the transaction boundary, and actor/scope threading (see `AGENTS.md`).
- `adapters/http`, `adapters/postgres`, … — transport and persistence. OpenAPI DTOs map to domain/application models here and nowhere else.

Rules that no linter can check:

- **Ports are declared by consumers, not providers.** The application package declares the interface it needs (repository, storage, clock); the adapter implements it. Do not declare an interface next to an implementation "just in case" — grow ports from a real second implementation or test fake.
- **Time arrives through the `clock.Clock` port** (`internal/shared/clock`), injected into services. Domain and application code never call `time.Now()` themselves; they receive the instant. Tests fake the clock (`fakeClock` precedent in identity).
- **Transactions go through `internal/transaction`** — `UoW.Do(ctx, work)` or `runInTx` (ADR 0033). Manual `Begin` in production code is blocked module-wide by forbidigo; the only exempt production path is `internal/platform` (the transaction layer itself), test fixtures are exempt per ADR 0033 («Test fixtures»). Audit records join the business operation's transaction (ADR 0020).
- **Shared kernel before new packages**: `internal/shared/` already carries `actor`, `clock`, `policy`, `pgerr`, `sanitize`, `timeutil`, `tzresolver`. Check these before inventing a parallel helper.

Adding a new bounded context — checklist:

1. `internal/<context>/{domain,application,adapters}` + `CONTEXT.md` (via `/domain-modeling`).
2. Add the context to **both** `domain-clean` and `application-clean` deny lists in `.golangci.yml` (they enumerate contexts explicitly; a missed entry silently disables the guard).
3. Decide the transaction story: UoW from the start — a production `Begin` outside `internal/platform` fails `make backend-lint` with no migration-ticket escape hatch (test fixtures are exempt, ADR 0033).

## Errors

- Sentinel errors per context live in `<context>/application/errors.go` (`Err` prefix; `identity/domain/errors.go` is the precedent for rules that are domain invariants).
- Return sentinels **unwrapped** from domain and application code. Wrap with `%w` only when adding information a caller needs (which operation, which id) — never re-wrap a sentinel into an opaque error on a path where the caller must match it.
- Matching happens with `errors.Is` **at the transport edge only** — `writeBillingError` in `billing/adapters/http` and `httpsupport.UserFacingDetail` are the patterns: one place per edge maps application errors to problem details and to the user-facing Russian message. Layer-to-layer `errors.Is` switching in the middle of the stack is a smell.
- Error text is a lowercase, unpunctuated continuation of the context (`staticcheck` ST1005 enforces form; the policy here is content: what failed + the identifying value).
- **Handle an error once.** One error, one handler: log it, map it to a response, or wrap-and-return it — never two on the same path. A returned error is the caller's to handle; logging it locally and returning it too produces double reporting and log spam (`revive unhandled-error` catches the statement-position calls; the "once" discipline above it is review's to check). The sanctioned discards are explicit: `_, _ = fmt.Fprintf(os.Stderr, ...)` in test teardown and `hash.Hash.Write` (documented never-fail) — an intentional discard says so with a comment.

## Process lifetime

- **The exit policy: the process exits only in `cmd/`.** `os.Exit`/`log.Fatal*` live in the composition root (startup failures, signal-driven shutdown); everything under `internal/` returns errors upward so deferred cleanup and graceful shutdown stay possible (enforced by forbidigo in `.golangci.yml`). The one adjacent idiom: `TestMain` relies on the Go 1.15+ test wrapper exiting with `m.Run`'s result — use `defer` for teardown, not `os.Exit(code)`.

## Domain constructors and validation

- **Domain constructors validate at creation.** `domain.NewX(...)` either returns a valid aggregate/value or an error — an invalid domain value is unrepresentable. Do not add a `Validate()` method that a caller might forget; when a constructor grows past a field check, decompose it (the `Load` config precedent: per-section loaders), don't grow a branch monster. Mapping of constructor errors to user-facing text still happens at the transport edge only.
- **Ports get static conformance assertions.** Every consumer-declared port gets a compile-time check at the adapter side: `var _ ExpiredDeleter = (*fakeDeleter)(nil)` (see `identity/adapters/scheduler/cleaner_test.go`) — the wiring survives renames without a runtime surprise. One assertion per adapter at the assignment or test site; do not collect them in a central file.

## Concurrency and workers

- **Every goroutine has an owner responsible for its exit.** The owner passes the context that cancels it; "fire-and-forget" goroutines with no cancellation path fail review even when `contextcheck` stays quiet about them. The two long-lived scheduler test binaries (`platform/scheduler`, `identity/adapters/scheduler`) also fail their run on any goroutine that outlives the tests, via `goleak.VerifyTestMain` in `TestMain` (`docs/agents/tooling.md`).
- Periodic work lives in `platform/scheduler` workers; contexts do not hand-roll their own tick loops.
- Prefer ownership and channels over shared memory; a mutex is fine for a cache, not fine around an I/O call (see rubric).
- `make test`'s `-race` integration runs are the backstop, not the design argument.

## Tests

- Table-driven tests, `t.Parallel()`, testify (`require` for fatal preconditions, `assert` for outcomes; `testifylint` polices usage).
- **Fakes over mocks**: ports get hand-written fakes (`fakeClock`, `mutableClock` in identity); assertion-framework mocks of domain ports are not the pattern here.
- Never `time.Sleep` to synchronize — fake the clock or use a channel.
- Adapters and repositories are covered by integration tests (`-tags=integration`, testcontainers — commands and CI wiring in `AGENTS.md`, Quality Gates).
- Test the behavior through the service; a test that asserts "the mock was called" instead of an outcome tests the mock.

## Review rubric — Go mistakes lint does not catch

Judgement calls for the Standards axis (source: distilled from [100go.co](https://100go.co) filtered against `.golangci.yml`; not violations). Read each as *what it is* → *how to fix*.

- **Ownerless goroutine** — a `go` statement whose exit nobody guarantees (no ctx, no WaitGroup, no done channel). → give it an owner and a cancellation path, or delete it.
- **Sentinel erased** — a sentinel error re-wrapped/converted on a path where the caller matches it with `errors.Is`. → return it as-is, or wrap with `%w` and check the chain still matches.
- **Provider-side interface** — an interface declared beside the only implementation. → move it to the consumer that needs the seam, or drop it.
- **Observable map iteration** — output (response, file, seed) built by ranging a map, order-random. → sort keys explicitly when the output is observable.
- **Mystery buffer size** — `make(chan T, N)` with an unexplained N, or fan-in into a channel nobody bounds. → unbuffered by default; a buffer needs a stated reason (burst/backpressure).
- **Lock across I/O** — a mutex held across a network/DB call. → shrink the critical section to memory, or redesign ownership.
- **`time.After` in a tick loop** — a timer allocated per iteration. → `time.Ticker`, stopped with `defer Stop()`.
- **Boundary without timeout** — an outbound HTTP/provider call on the request's bare ctx. → derive a scoped ctx with timeout at the use-case boundary; cancellation belongs to the owner of the work.
