# ADR 0013: In-Memory Token-Bucket Rate Limiters

## Status

Accepted

## Context

Auth endpoints currently use token-bucket rate limiters implemented in
`apps/backend/internal/platform/httpapi/ratelimit.go`. These limiters are
created in `cmd/api/main.go` and keyed per phone/email in the auth handlers.

## Decision

Keep the in-process limiters for the current deployment shape.

## Consequences

The implementation is simple and requires no extra infrastructure, but it has
important operational limitations:

- **No survival across restarts.** Bucket state is held only in memory, so a
  process restart resets all per-key counters.
- **No cross-replica coordination.** If the backend is scaled horizontally,
  each replica maintains its own buckets, so a client can exceed the intended
  global limit by spreading requests across instances.
- **No shared state with other components.** Edge proxies, load balancers, or
  other services cannot read or contribute to the limiter state.

For production hardening, replace or augment these in-memory limiters with a
shared store such as Redis (e.g. `golang.org/x/time/rate` backed by Redis cell
or a sliding-window counter) or move rate limiting to the gateway/load balancer
layer. A gateway-based solution is usually preferred for auth paths because it
protects the backend from expensive request handling entirely.
