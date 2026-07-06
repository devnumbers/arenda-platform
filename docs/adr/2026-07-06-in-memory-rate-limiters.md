# ADR: In-memory rate limiters for auth endpoints

## Status
Accepted, known limitation.

## Context
The identity/auth endpoints use token-bucket rate limiters implemented in `apps/backend/internal/platform/httpapi/ratelimit.go`. Buckets are created at process startup (`apps/backend/cmd/api/main.go`) and stored in memory.

## Decision
Keep the current in-memory implementation for now.

## Consequences

### Positive
- Simple, no external dependency.
- Fast, no network round-trips.
- Sufficient for single-instance deployments and local development.

### Negative
- **No cross-replica sharing.** In a multi-replica deployment the effective limit is multiplied by the number of running instances.
- **State is lost on restart.** After a deploy or crash all counters reset.
- **No central observability.** Rate-limit events are local to each instance.

## Mitigation
- Document the limitation (this ADR).
- Before scaling to multiple production replicas, replace the in-memory buckets with a shared store such as Redis or move rate limiting to the ingress/gateway layer.
- Tune burst values independently of the overall per-hour/per-15-minute limits so a single attacker cannot burn the entire quota in the first few requests.
