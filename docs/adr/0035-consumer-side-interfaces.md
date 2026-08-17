# ADR 0035: Consumer-side interfaces (Go idiom)

## Status

Accepted

## Context

The Go idiom is "accept interfaces, return structs": an interface belongs to the package that *consumes* it, not the package that *produces* the concrete type. Defining an interface next to its implementation couples the producer package to a contract it does not need, invites interface bloat (the producer grows the interface to match every method it happens to have), and forces every consumer to depend on the same one-size-fits-all abstraction.

Identity violated this idiom. Four transport-facing service interfaces — `Authenticator`, `PhoneChanger`, `Profiler`, `Logout` — were declared in the producer package `internal/identity/application` (`interfaces.go`), but the only consumer was `internal/identity/adapters/http` (`AuthHandlers`). The test fake `fakeAuthenticator` in `auth_handlers_test.go` already duplicated the signatures by hand instead of satisfying a local interface — the classic duplicate-symptom that signals the interface is in the wrong place.

This was a codebase-wide convention carried over from an earlier centralized ports-and-adapters style. ADR 0029 already named consumer-side interfaces as the future mechanism for breaking cross-context cycles (rental super-context); this ADR extends the same principle to the intra-module case.

`SessionService` is a different shape of consumer: it is consumed *inside* `application` itself (by `AuthenticationService`, `PhoneChangeService`) and by the HTTP session middleware via a platform-neutral `httpsupport.SessionLoader` adapter (ADR 0034). It is an intra-application seam, not a transport-facing one, so it stays where its consumer lives — inside `application`.

## Decision

Transport-facing service interfaces live in the consumer package, next to the handler that calls them.

Identity is the pilot: the four interface declarations (`Authenticator`, `PhoneChanger`, `Profiler`, `Logout`) move from `application/interfaces.go` to `adapters/http/auth_handlers.go` (package `http`), exported, beside `AuthHandlers`. `application/interfaces.go` is deleted. The concrete services (`AuthenticationService`, `PhoneChangeService`, `ProfileService`, `LogoutService`) satisfy the interfaces through Go structural typing — no `var _ Interface = (*Service)(nil)` assertion is added; the assignment site (`NewAuthHandlers`, `httpserver.Deps`) is where conformance is checked at compile time.

`SessionService` stays in `application/session_service.go` — it is consumed intra-application, not by the transport adapter.

The other contexts (`billing`, `leases`, `audit`, …) are **not** changed in this ADR. They migrate to the same pattern incrementally, as each is touched for other reasons — the same "migrate by touch" discipline ADR 0029 uses for the rental super-context split.

## Consequences

- (+) `application` no longer declares a contract only one other package consumes. The interface sits next to its single caller, so changing the transport need changes one file, not two packages.
- (+) The test fake `fakeAuthenticator` now satisfies a local interface (same package) — no hand-duplicated signature, and removing the `application` qualifier from the test helper is a free side effect.
- (+) `internal/platform/httpserver` imports one fewer `application` package: `identityapp` disappears from `server.go`, leaving only `identityhttp` (the adapter that owns both the handlers and their interfaces).
- (-) A reader looking for "what is the auth contract" must now look in `adapters/http`, not `application`. This is the intended Go trade-off: the contract is defined by what the caller needs, and the caller is the HTTP adapter.
- (-) Concrete services gain no compile-time assertion that they satisfy the interfaces in the absence of a call site. In practice every service is wired through `NewAuthHandlers` / `httpserver.Deps` in production and tests, so conformance is still caught at build time; this is sufficient and avoids a redundant assertion the Go idiom discourages.
- Other contexts keep their centralized ports until they are migrated; this ADR does not mandate a sweep.

## See also

- [`docs/adr/0029-rental-superc-context-temporary.md`](./0029-rental-superc-context-temporary.md) — names consumer-side interfaces as the future mechanism for breaking cross-context (rental super-context) cycles; this ADR applies the same principle intra-module.
- [`docs/adr/0033-unit-of-work-transactional-seam.md`](./0033-unit-of-work-transactional-seam.md) — another identity-layer seam refactor; companion in cleaning up the application/adapter boundary.
- [`docs/adr/0034-actor-identity-shared-kernel.md`](./0034-actor-identity-shared-kernel.md) — moved `SessionLoader` to a platform-neutral adapter so `httpsupport` no longer imports identity; the same direction (consumer owns the contract it needs).
