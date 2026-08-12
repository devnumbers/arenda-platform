# ADR 0034: Actor Identity Shared Kernel

## Status

Accepted

## Context

The platform layer (`internal/platform/httpsupport`) depends on a concrete bounded context — `internal/identity/domain` — in four files:

| File | What it imports from `identity/domain` |
|---|---|
| `context.go` | `User` (stored in and read from the request context) |
| `admin_middleware.go` | `User`, `RoleAdmin` (role check) |
| `request.go` | `Email`, `NewEmail` (`ParseOptionalEmail`) |
| `problem.go` | `ErrTooManyAttempts` (error → HTTP status mapping) |

This is an inverted dependency: shared platform infrastructure depends on one context's domain. A context is supposed to depend on the platform, not the other way around. It also creates a circular-ish coupling: changing `identity.User` ripples into platform code that every other context's handlers share.

The actual need is narrow. A fact-finding pass over the three callers of `httpsupport.UserFromContext` showed:

- `AdminOnlyMiddleware` — reads **only `.Role`**.
- `recordAuthAudit` (identity's own handler) — reads **only `.Role`**.
- `GetMe` (identity's own handler) — wants the **full profile**, but already has a fallback to `Profile.Me(userID)` for the full `User`; the context value is only a cache.

Two of three callers need nothing but the role; the third legitimately fetches the full profile from the identity service. `ParseOptionalEmail` and `ErrTooManyAttempts` are consumed only by identity's own auth handlers, not by platform code generally. The platform does not need the `identity.User` aggregate (with Phone, Timezone, Email, names) — it needs the **actor identity**: who is acting, and with which role.

`internal/shared` already exists as the shared kernel home (`clock`, `policy`, `sanitize`, `tzresolver`, …) and is imported by all nine contexts. `CONTEXT-MAP.md` already documents role vocabulary in its shared-kernel section.

## Decision

Introduce a minimal **actor identity** shared kernel in `internal/shared/actor`:

```go
// Package actor holds the shared identity kernel: the roles that thread through
// every context and the actor identity carried in request contexts.
package actor

type Role string

const (
    RoleOwner Role = "owner"
    RoleAdmin Role = "admin"
)

func NewRole(raw string) (Role, error) { /* validate */ }
func (r Role) String() string          { return string(r) }
```

`identity/domain` keeps `Role` as a domain type that wraps the shared one, or re-exports it, so identity's ubiquitous language is preserved. The canonical home for role values is `shared/actor`.

### Platform changes

- `httpsupport` request context carries **`(uuid.UUID, actor.Role)`** instead of `identitydomain.User`. `UserFromContext` becomes `ActorFromContext` (returns `(uuid.UUID, actor.Role, bool)`); `UserFromContext` is removed.
- `AdminOnlyMiddleware` checks `actor.RoleAdmin`.
- `ParseOptionalEmail` moves into the identity HTTP adapter (it is only used by auth handlers); `httpsupport` stops parsing domain `Email`.
- `problem.go` stops referencing `identitydomain.ErrTooManyAttempts`; the identity handler maps that error to a status itself.
- `httpsupport` no longer imports `identity/domain` at all.

`GetMe` (the one caller wanting the full profile) stops reading a cached `User` from context and always fetches via `Profile.Me(userID)` — its existing fallback path becomes the only path.

### Lint enforcement

A new `depguard` rule `platform-clean` denies `internal/platform/**` from importing any `internal/identity/**`. The four-file violation is fixed by this ADR's changes; the rule then keeps the dependency from returning.

### What is *not* shared

`User`, `Phone`, `Email`, `Session`, `LoginCode` stay in `identity/domain`. They are identity's aggregate and value objects, not platform concerns. Only `Role` — the one value that every context's authorization and audit needs — is lifted into the shared kernel. If a future context needs `Email` as a shared value object, that is a separate decision and a separate ADR.

## Consequences

- (+) The platform no longer depends on any bounded context's domain. The dependency direction is context → platform, as DDD layering requires.
- (+) `depguard` `platform-clean` makes the inversion structurally impossible to reintroduce.
- (+) The request context carries exactly what platform middleware needs (actor identity), not an aggregate it has no business holding.
- (-) `GetMe` always hits `Profile.Me` instead of a context cache. This is one extra read per `/me` request; acceptable for an endpoint that returns the profile anyway, and it removes a stale-cache risk.
- (-) `identity/domain` gains a dependency on `shared/actor` for the role type (or a re-export). This is the intended direction (context → shared kernel).
- `CONTEXT-MAP.md` shared-kernel section is updated to point at `internal/shared/actor` as the canonical role home.

## See also

- [`docs/adr/0033-unit-of-work-transactional-seam.md`](./0033-unit-of-work-transactional-seam.md) — the companion transactional-seam ADR; both remove `platform → identity` coupling from different angles.
- [`docs/adr/0028-object-data-access-model.md`](./0028-object-data-access-model.md) — defines the policy `Role` values that `actor.Role` mirrors; the strings are identical.
